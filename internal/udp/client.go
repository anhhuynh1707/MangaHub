package udp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

// NotificationClient sends notifications to a remote UDP server.
type NotificationClient struct {
	ServerAddr string
}

// NewNotificationClient creates a UDP client that sends to a remote server.
func NewNotificationClient(serverAddr string) (*NotificationClient, error) {
	// Validate the address can be resolved
	_, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve UDP address %s: %w", serverAddr, err)
	}

	return &NotificationClient{
		ServerAddr: serverAddr,
	}, nil
}

// SendNotification sends a notification to the remote UDP server.
// It wraps the notification in a broadcast message that the server will recognize.
func (c *NotificationClient) SendNotification(notif Notification) error {
	// Create a broadcast wrapper message
	broadcastMsg := map[string]interface{}{
		"type":         "api_broadcast",
		"notification": notif,
	}

	data, err := json.Marshal(broadcastMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Add newline delimiter as per spec
	data = append(data, '\n')

	// Create a UDP connection (UDP is connectionless, but we use this to send)
	addr, err := net.ResolveUDPAddr("udp", c.ServerAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve server address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	defer conn.Close()

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	_, err = conn.Write(data)
	if err != nil {
		log.Printf("UDP client: failed to send notification: %v", err)
		return err
	}

	log.Printf("UDP client: sent broadcast to %s", c.ServerAddr)
	return nil
}

// Close is a no-op for UDP client (connectionless)
func (c *NotificationClient) Close() error {
	return nil
}
