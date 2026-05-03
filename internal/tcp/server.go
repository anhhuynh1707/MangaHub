package tcp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

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
	startTime   time.Time
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
	s.startTime = time.Now()

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

// broadcastLoop listens on the Broadcast channel and sends updates to ALL connected clients.
func (s *ProgressSyncServer) broadcastLoop() {
	for update := range s.Broadcast {
		msg := NewBroadcastMessage(update.UserID, update.MangaID, update.Chapter)

		s.mu.RLock()
		for _, conn := range s.Connections {
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
	SendMessage(conn, msg)
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

// GetConnectionCount returns the number of active connections.
func (s *ProgressSyncServer) GetConnectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Connections)
}

// GetUptime returns how long the server has been running.
func (s *ProgressSyncServer) GetUptime() time.Duration {
	return time.Since(s.startTime)
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
