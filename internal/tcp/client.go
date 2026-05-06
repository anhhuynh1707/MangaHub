package tcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"mangahub/pkg/models"
)

// ProgressSyncClient sends progress updates to a remote TCP server.
type ProgressSyncClient struct {
	ServerAddr string
	conn       net.Conn
}

// NewProgressSyncClient creates a TCP client that connects to a remote server.
func NewProgressSyncClient(serverAddr string) (*ProgressSyncClient, error) {
	// Dial the remote TCP server
	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TCP server %s: %w", serverAddr, err)
	}

	return &ProgressSyncClient{
		ServerAddr: serverAddr,
		conn:       conn,
	}, nil
}

// SendProgressUpdate sends a progress update to the remote TCP server.
// This is used when the API runs in microservice mode (disabled internal server).
func (c *ProgressSyncClient) SendProgressUpdate(update models.ProgressUpdate) error {
	if c.conn == nil {
		return fmt.Errorf("TCP client not connected")
	}

	msg := NewBroadcastMessage(update.UserID, update.MangaID, update.Chapter)
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal progress update: %w", err)
	}

	// Add newline delimiter
	data = append(data, '\n')

	// Set write deadline
	c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	_, err = c.conn.Write(data)
	if err != nil {
		log.Printf("TCP client: failed to send progress update: %v", err)
		return err
	}

	return nil
}

// Close closes the TCP connection.
func (c *ProgressSyncClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
