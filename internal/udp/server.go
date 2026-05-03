package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// NotificationServer is the UDP server for broadcasting manga notifications.
// Spec-required struct: manages registered client addresses and sends fire-and-forget notifications.
type NotificationServer struct {
	Port    string
	Clients []net.UDPAddr // Registered client addresses
	mu      sync.RWMutex
	conn    *net.UDPConn
}

// Notification represents a UDP notification message.
type Notification struct {
	Type      string `json:"type"`                // "new_chapter", "system", "manga_update", "register", "unregister", "register_ack", "test"
	MangaID   string `json:"manga_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// NewNotificationServer creates a new UDP notification server.
func NewNotificationServer(port string) *NotificationServer {
	return &NotificationServer{
		Port:    port,
		Clients: make([]net.UDPAddr, 0),
	}
}

// Start begins listening for UDP messages (registrations) and allows broadcasting.
func (s *NotificationServer) Start() error {
	addr, err := net.ResolveUDPAddr("udp", ":"+s.Port)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to start UDP server: %w", err)
	}
	s.conn = conn

	log.Printf("📢 UDP Notification Server listening on :%s", s.Port)

	// Listen for client registrations
	buf := make([]byte, 4096)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("UDP read error: %v", err)
			continue
		}

		go s.handleMessage(buf[:n], clientAddr)
	}
}

// Stop gracefully shuts down the UDP server.
func (s *NotificationServer) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}
	log.Println("UDP notification server stopped")
}

// handleMessage processes an incoming UDP message from a client.
func (s *NotificationServer) handleMessage(data []byte, clientAddr *net.UDPAddr) {
	var msg Notification
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("UDP: Invalid message from %s: %v", clientAddr, err)
		s.sendTo(clientAddr, Notification{
			Type:      "error",
			Message:   "Invalid JSON message",
			Timestamp: time.Now().Unix(),
		})
		return
	}

	switch msg.Type {
	case "register":
		s.registerClient(clientAddr)
		s.sendTo(clientAddr, Notification{
			Type:      "register_ack",
			Message:   fmt.Sprintf("Registered for notifications. You are client #%d.", s.GetClientCount()),
			Timestamp: time.Now().Unix(),
		})
		log.Printf("UDP: Client registered: %s (total: %d)", clientAddr, s.GetClientCount())

	case "unregister":
		s.unregisterClient(clientAddr)
		s.sendTo(clientAddr, Notification{
			Type:      "register_ack",
			Message:   "Unregistered from notifications.",
			Timestamp: time.Now().Unix(),
		})
		log.Printf("UDP: Client unregistered: %s (total: %d)", clientAddr, s.GetClientCount())

	case "test":
		// Echo test — respond directly to sender
		s.sendTo(clientAddr, Notification{
			Type:      "test",
			Message:   "UDP notification system is working!",
			Timestamp: time.Now().Unix(),
		})
		log.Printf("UDP: Test message from %s", clientAddr)

	default:
		log.Printf("UDP: Unknown message type '%s' from %s", msg.Type, clientAddr)
		s.sendTo(clientAddr, Notification{
			Type:      "error",
			Message:   fmt.Sprintf("Unknown type: %s. Valid: register, unregister, test", msg.Type),
			Timestamp: time.Now().Unix(),
		})
	}
}

// registerClient adds a client address to the notification list.
func (s *NotificationServer) registerClient(addr *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already registered (same IP:Port)
	for _, client := range s.Clients {
		if client.IP.Equal(addr.IP) && client.Port == addr.Port {
			return // Already registered
		}
	}

	s.Clients = append(s.Clients, *addr)
}

// unregisterClient removes a client address from the notification list.
func (s *NotificationServer) unregisterClient(addr *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, client := range s.Clients {
		if client.IP.Equal(addr.IP) && client.Port == addr.Port {
			s.Clients = append(s.Clients[:i], s.Clients[i+1:]...)
			return
		}
	}
}

// GetClientCount returns the number of registered clients.
func (s *NotificationServer) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Clients)
}

// GetClients returns a copy of all registered client addresses.
func (s *NotificationServer) GetClients() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrs := make([]string, len(s.Clients))
	for i, c := range s.Clients {
		addrs[i] = c.String()
	}
	return addrs
}

// sendTo sends a notification to a specific client address.
func (s *NotificationServer) sendTo(addr *net.UDPAddr, notif Notification) {
	data, err := json.Marshal(notif)
	if err != nil {
		log.Printf("UDP: Failed to marshal notification: %v", err)
		return
	}
	data = append(data, '\n')

	if _, err := s.conn.WriteToUDP(data, addr); err != nil {
		log.Printf("UDP: Failed to send to %s: %v", addr, err)
	}
}
