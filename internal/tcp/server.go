package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"mangahub/internal/auth"
	"mangahub/pkg/models"
)

// ProgressPersister is an interface for saving progress to the database.
// This decouples the TCP server from the user package (avoids circular imports).
type ProgressPersister interface {
	UpdateProgress(userID string, req *models.UpdateProgressRequest) (*models.UserProgress, error)
}

// ProgressSyncServer is the TCP server for broadcasting reading progress updates.
// Spec-required struct: accepts multiple connections and broadcasts ProgressUpdate messages.
type ProgressSyncServer struct {
	Port        string
	Connections map[string]net.Conn // user_id -> connection
	Broadcast   chan models.ProgressUpdate
	Persister   ProgressPersister // saves progress to DB
	mu          sync.RWMutex
	listener    net.Listener
}

// TCPMessage represents a message in the TCP JSON protocol.
// Messages are newline-delimited JSON.
type TCPMessage struct {
	Type      string `json:"type"`                 // connect, disconnect, progress, broadcast, welcome, error, auth
	UserID    string `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	MangaID   string `json:"manga_id,omitempty"`
	Chapter   int    `json:"chapter,omitempty"`
	Message   string `json:"message,omitempty"`
	Token     string `json:"token,omitempty"`       // JWT token for authentication
	Timestamp int64  `json:"timestamp,omitempty"`
}

// NewProgressSyncServer creates a new TCP progress sync server.
func NewProgressSyncServer(port string) *ProgressSyncServer {
	return &ProgressSyncServer{
		Port:        port,
		Connections: make(map[string]net.Conn),
		Broadcast:   make(chan models.ProgressUpdate, 100),
	}
}

// Start begins listening for TCP connections and processing broadcasts.
func (s *ProgressSyncServer) Start() error {
	listener, err := net.Listen("tcp", ":"+s.Port)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	s.listener = listener

	log.Printf("📡 TCP Progress Sync Server listening on :%s", s.Port)

	// Start the broadcast goroutine
	go s.broadcastLoop()

	// Accept connections loop
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if the listener was closed (graceful shutdown)
			select {
			default:
				log.Printf("TCP accept error: %v", err)
				continue
			}
		}

		// Handle each connection in its own goroutine
		go s.handleConnection(conn)
	}
}

// Stop gracefully shuts down the TCP server.
func (s *ProgressSyncServer) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for userID, conn := range s.Connections {
		s.sendMessage(conn, TCPMessage{
			Type:      "disconnect",
			Message:   "Server shutting down",
			Timestamp: time.Now().Unix(),
		})
		conn.Close()
		delete(s.Connections, userID)
	}

	log.Println("TCP server stopped")
}

// handleConnection processes a single TCP client connection.
// Each connection runs in its own goroutine.
func (s *ProgressSyncServer) handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	log.Printf("TCP: New connection from %s", remoteAddr)

	// Send welcome message
	s.sendMessage(conn, TCPMessage{
		Type:      "welcome",
		Message:   "Connected to MangaHub TCP Progress Sync Server. Send auth message with your JWT token.",
		Timestamp: time.Now().Unix(),
	})

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

		var msg TCPMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			s.sendMessage(conn, TCPMessage{
				Type:      "error",
				Message:   "Invalid JSON message",
				Timestamp: time.Now().Unix(),
			})
			continue
		}

		switch msg.Type {
		case "auth":
			// Authenticate with JWT token — each terminal authenticates independently
			if msg.Token == "" {
				s.sendMessage(conn, TCPMessage{
					Type:      "error",
					Message:   "Token is required for authentication",
					Timestamp: time.Now().Unix(),
				})
				continue
			}

			claims, err := auth.ValidateToken(msg.Token)
			if err != nil {
				s.sendMessage(conn, TCPMessage{
					Type:      "error",
					Message:   "Invalid or expired token: " + err.Error(),
					Timestamp: time.Now().Unix(),
				})
				continue
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

			authenticatedUserID = claims.UserID
			authenticatedUsername = claims.Username

			log.Printf("TCP: User %s (%s) authenticated from %s", authenticatedUsername, authenticatedUserID, remoteAddr)

			s.sendMessage(conn, TCPMessage{
				Type:      "auth",
				UserID:    authenticatedUserID,
				Username:  authenticatedUsername,
				Message:   fmt.Sprintf("Authenticated as %s. You will receive progress broadcasts.", authenticatedUsername),
				Timestamp: time.Now().Unix(),
			})

			// Broadcast join notification to other users
			s.broadcastToOthers(authenticatedUserID, TCPMessage{
				Type:      "user_joined",
				UserID:    authenticatedUserID,
				Username:  authenticatedUsername,
				Message:   fmt.Sprintf("%s connected to sync", authenticatedUsername),
				Timestamp: time.Now().Unix(),
			})

		case "progress":
			// Client sends a progress update — requires authentication
			if authenticatedUserID == "" {
				s.sendMessage(conn, TCPMessage{
					Type:      "error",
					Message:   "You must authenticate first. Send: {\"type\":\"auth\",\"token\":\"your-jwt-token\"}",
					Timestamp: time.Now().Unix(),
				})
				continue
			}

			if msg.MangaID == "" || msg.Chapter <= 0 {
				s.sendMessage(conn, TCPMessage{
					Type:      "error",
					Message:   "manga_id and chapter are required",
					Timestamp: time.Now().Unix(),
				})
				continue
			}

			// Persist to database
			if s.Persister != nil {
				_, err := s.Persister.UpdateProgress(authenticatedUserID, &models.UpdateProgressRequest{
					MangaID:        msg.MangaID,
					CurrentChapter: msg.Chapter,
					Status:         "reading",
				})
				if err != nil {
					log.Printf("TCP: Failed to persist progress for %s: %v", authenticatedUsername, err)
					s.sendMessage(conn, TCPMessage{
						Type:      "error",
						Message:   "Progress broadcast sent but failed to save: " + err.Error(),
						Timestamp: time.Now().Unix(),
					})
				}
			}

			// Push to broadcast channel
			update := models.ProgressUpdate{
				UserID:    authenticatedUserID,
				MangaID:   msg.MangaID,
				Chapter:   msg.Chapter,
				Timestamp: time.Now().Unix(),
			}

			s.Broadcast <- update

			log.Printf("TCP: Progress update from %s: %s ch.%d (persisted)", authenticatedUsername, msg.MangaID, msg.Chapter)

		case "disconnect":
			log.Printf("TCP: User %s requested disconnect", authenticatedUsername)
			goto cleanup

		case "ping":
			s.sendMessage(conn, TCPMessage{
				Type:      "pong",
				Message:   "pong",
				Timestamp: time.Now().Unix(),
			})

		default:
			s.sendMessage(conn, TCPMessage{
				Type:      "error",
				Message:   fmt.Sprintf("Unknown message type: %s. Valid types: auth, progress, disconnect, ping", msg.Type),
				Timestamp: time.Now().Unix(),
			})
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

// broadcastLoop listens on the Broadcast channel and sends updates to ALL connected clients.
func (s *ProgressSyncServer) broadcastLoop() {
	for update := range s.Broadcast {
		msg := TCPMessage{
			Type:      "broadcast",
			UserID:    update.UserID,
			MangaID:   update.MangaID,
			Chapter:   update.Chapter,
			Timestamp: update.Timestamp,
			Message:   fmt.Sprintf("User %s read chapter %d of %s", update.UserID, update.Chapter, update.MangaID),
		}

		s.mu.RLock()
		for userID, conn := range s.Connections {
			_ = userID // broadcast to everyone, including the sender
			s.sendMessage(conn, msg)
		}
		s.mu.RUnlock()
	}
}

// broadcastToOthers sends a message to all connected clients except the specified user.
func (s *ProgressSyncServer) broadcastToOthers(excludeUserID string, msg TCPMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for userID, conn := range s.Connections {
		if userID != excludeUserID {
			s.sendMessage(conn, msg)
		}
	}
}

// sendMessage sends a JSON message to a connection (newline-delimited).
func (s *ProgressSyncServer) sendMessage(conn net.Conn, msg TCPMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("TCP: Failed to marshal message: %v", err)
		return
	}
	data = append(data, '\n')

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(data); err != nil {
		log.Printf("TCP: Failed to write to connection: %v", err)
	}
}

// GetConnectedUsers returns a list of currently connected user IDs.
func (s *ProgressSyncServer) GetConnectedUsers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]string, 0, len(s.Connections))
	for userID := range s.Connections {
		users = append(users, userID)
	}
	return users
}

// SendProgressUpdate pushes a progress update into the broadcast channel.
// This is called from the HTTP API when PUT /users/progress is hit.
func (s *ProgressSyncServer) SendProgressUpdate(update models.ProgressUpdate) {
	select {
	case s.Broadcast <- update:
	default:
		log.Printf("TCP: Broadcast channel full, dropping update for %s", update.MangaID)
	}
}
