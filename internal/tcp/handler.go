package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"mangahub/internal/auth"
	"mangahub/pkg/models"
)

// handleConnection processes a single TCP client connection.
// Each connection runs in its own goroutine (one goroutine per client).
func (s *ProgressSyncServer) handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	log.Printf("TCP: New connection from %s", remoteAddr)

	// Send welcome message
	s.sendMessage(conn, NewWelcomeMessage())

	var authenticatedUserID string
	var authenticatedUsername string

	scanner := bufio.NewScanner(conn)
	// Set max token size to 64KB (for large JWT tokens)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		msg, err := DecodeMessage([]byte(line))
		if err != nil {
			s.sendMessage(conn, NewErrorMessage("Invalid JSON message"))
			continue
		}

		switch msg.Type {
		case "auth":
			authenticatedUserID, authenticatedUsername = s.handleAuth(conn, msg, remoteAddr)

		case "connect":
			// Legacy connect message — treat as auth if token is present
			if msg.Token != "" {
				authenticatedUserID, authenticatedUsername = s.handleAuth(conn, msg, remoteAddr)
			} else if msg.UserID != "" {
				// Spec protocol: {"type":"connect","user_id":"user123"}
				// Accept without JWT for backward compatibility, but log warning
				log.Printf("TCP: Warning - unauthenticated connect from %s (user_id: %s)", remoteAddr, msg.UserID)
				s.sendMessage(conn, NewErrorMessage("Authentication required. Send: {\"type\":\"auth\",\"token\":\"your-jwt-token\"}"))
			}

		case "progress":
			s.handleProgress(conn, msg, authenticatedUserID, authenticatedUsername)

		case "disconnect":
			log.Printf("TCP: User %s requested disconnect", authenticatedUsername)
			goto cleanup

		case "ping":
			s.sendMessage(conn, TCPMessage{
				Type:      "pong",
				Message:   "pong",
				Timestamp: time.Now().Unix(),
			})

		case "status":
			// Client requests connection status
			s.handleStatusRequest(conn, authenticatedUserID, authenticatedUsername)

		default:
			s.sendMessage(conn, NewErrorMessage(
				fmt.Sprintf("Unknown message type: %s. Valid types: auth, connect, progress, disconnect, ping, status", msg.Type),
			))
		}
	}

cleanup:
	// Remove connection on disconnect
	if authenticatedUserID != "" {
		s.mu.Lock()
		// Only delete if this connection is still the one mapped to the user
		if existingConn, ok := s.Connections[authenticatedUserID]; ok && existingConn == conn {
			delete(s.Connections, authenticatedUserID)
		}
		s.mu.Unlock()

		// Broadcast leave notification
		s.broadcastToOthers(authenticatedUserID, TCPMessage{
			Type:      "user_left",
			UserID:    authenticatedUserID,
			Username:  authenticatedUsername,
			Message:   fmt.Sprintf("%s disconnected from sync", authenticatedUsername),
			Timestamp: time.Now().Unix(),
		})

		log.Printf("TCP: User %s (%s) disconnected", authenticatedUsername, authenticatedUserID)
	} else {
		log.Printf("TCP: Unauthenticated connection from %s closed", remoteAddr)
	}

	conn.Close()
}

// handleAuth processes an authentication message.
// Returns the authenticated user ID and username.
func (s *ProgressSyncServer) handleAuth(conn net.Conn, msg *TCPMessage, remoteAddr string) (string, string) {
	if msg.Token == "" {
		s.sendMessage(conn, NewErrorMessage("Token is required for authentication"))
		return "", ""
	}

	claims, err := auth.ValidateToken(msg.Token)
	if err != nil {
		s.sendMessage(conn, NewErrorMessage("Invalid or expired token: "+err.Error()))
		return "", ""
	}

	// Remove old connection for this user if exists
	s.mu.Lock()
	if oldConn, exists := s.Connections[claims.UserID]; exists {
		s.sendMessage(oldConn, TCPMessage{
			Type:      "disconnect",
			Message:   "Replaced by new connection",
			Timestamp: time.Now().Unix(),
		})
		oldConn.Close()
	}
	s.Connections[claims.UserID] = conn
	s.mu.Unlock()

	log.Printf("TCP: User %s (%s) authenticated from %s", claims.Username, claims.UserID, remoteAddr)

	s.sendMessage(conn, TCPMessage{
		Type:     "auth",
		UserID:   claims.UserID,
		Username: claims.Username,
		Message:  fmt.Sprintf("Authenticated as %s. You will receive progress broadcasts.", claims.Username),
		Timestamp: time.Now().Unix(),
	})

	// Broadcast join notification to other users
	s.broadcastToOthers(claims.UserID, TCPMessage{
		Type:      "user_joined",
		UserID:    claims.UserID,
		Username:  claims.Username,
		Message:   fmt.Sprintf("%s connected to sync", claims.Username),
		Timestamp: time.Now().Unix(),
	})

	return claims.UserID, claims.Username
}

// handleProgress processes a progress update message.
func (s *ProgressSyncServer) handleProgress(conn net.Conn, msg *TCPMessage, userID, username string) {
	// Requires authentication
	if userID == "" {
		s.sendMessage(conn, NewErrorMessage(
			"You must authenticate first. Send: {\"type\":\"auth\",\"token\":\"your-jwt-token\"}",
		))
		return
	}

	if msg.MangaID == "" || msg.Chapter <= 0 {
		s.sendMessage(conn, NewErrorMessage("manga_id and chapter are required"))
		return
	}

	// Persist to database — broadcast only if save succeeds
	if s.Persister != nil {
		_, err := s.Persister.UpdateProgress(userID, &models.UpdateProgressRequest{
			MangaID:        msg.MangaID,
			CurrentChapter: msg.Chapter,
			Status:         "reading",
		})
		if err != nil {
			log.Printf("TCP: Failed to persist progress for %s: %v", username, err)
			s.sendMessage(conn, NewErrorMessage("Failed to save progress: "+err.Error()+". Make sure the manga is in your library first."))
			return
		}
	}

	// Push to broadcast channel (only reached if save succeeded)
	update := models.ProgressUpdate{
		UserID:    userID,
		MangaID:   msg.MangaID,
		Chapter:   msg.Chapter,
		Timestamp: time.Now().Unix(),
	}

	s.Broadcast <- update

	log.Printf("TCP: Progress update from %s: %s ch.%d (saved & broadcast)", username, msg.MangaID, msg.Chapter)
}

// handleStatusRequest responds to a client's status inquiry.
func (s *ProgressSyncServer) handleStatusRequest(conn net.Conn, userID, username string) {
	s.mu.RLock()
	connectedCount := len(s.Connections)
	users := make([]string, 0, connectedCount)
	for uid := range s.Connections {
		users = append(users, uid)
	}
	s.mu.RUnlock()

	statusMsg := fmt.Sprintf("Server running on :%s | %d users connected", s.Port, connectedCount)
	if userID != "" {
		statusMsg += fmt.Sprintf(" | You: %s (%s)", username, userID)
	}

	msg := TCPMessage{
		Type:           "status",
		UserID:         userID,
		Username:       username,
		Message:        statusMsg,
		ConnectedUsers: connectedCount,
		Timestamp:      time.Now().Unix(),
	}

	data, _ := json.Marshal(users)
	msg.Message += " | Users: " + string(data)

	s.sendMessage(conn, msg)
}
