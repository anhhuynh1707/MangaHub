package tcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

// TCPMessage represents a message in the TCP JSON protocol.
// Messages are newline-delimited JSON (\n terminated).
//
// Protocol specification:
//
//	Client → Server: {"type":"auth","token":"jwt-token"}\n
//	Client → Server: {"type":"progress","manga_id":"one-piece","chapter":1095,"device_id":"cli-alice"}\n
//	Client → Server: {"type":"connect","user_id":"user123"}\n
//	Client → Server: {"type":"disconnect"}\n
//	Client → Server: {"type":"ping"}\n
//	Client → Server: {"type":"set_strategy","strategy":"merge"}\n
//	Server → Client: {"type":"welcome","message":"Connected to TCP sync"}\n
//	Server → Client: {"type":"broadcast","user_id":"user456","manga_id":"naruto","chapter":700}\n
//	Server → Client: {"type":"auth","user_id":"user123","username":"alice"}\n
//	Server → Client: {"type":"user_joined","username":"bob"}\n
//	Server → Client: {"type":"user_left","username":"bob"}\n
//	Server → Client: {"type":"error","message":"..."}\n
//	Server → Client: {"type":"pong"}\n
//	Server → Client: {"type":"status","message":"...","connected_users":3}\n
//	Server → Client: {"type":"conflict","manga_id":"...","chapter":N,"message":"..."}\n
type TCPMessage struct {
	Type           string `json:"type"`                      // auth, connect, disconnect, progress, broadcast, welcome, error, ping, pong, user_joined, user_left, status, conflict, set_strategy
	UserID         string `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	MangaID        string `json:"manga_id,omitempty"`
	Chapter        int    `json:"chapter,omitempty"`
	Message        string `json:"message,omitempty"`
	Token          string `json:"token,omitempty"`            // JWT token for authentication
	Timestamp      int64  `json:"timestamp,omitempty"`
	ConnectedUsers int    `json:"connected_users,omitempty"`  // For status responses
	DeviceID       string `json:"device_id,omitempty"`        // Client device/session identifier for conflict resolution
	Strategy       string `json:"strategy,omitempty"`         // Conflict resolution strategy
}

// ValidMessageTypes lists all valid message types in the protocol.
var ValidMessageTypes = []string{
	"auth", "connect", "disconnect", "progress",
	"broadcast", "welcome", "error",
	"ping", "pong", "status",
	"user_joined", "user_left",
	"conflict", "set_strategy",
}

// EncodeMessage serializes a TCPMessage to newline-delimited JSON bytes.
func EncodeMessage(msg TCPMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode message: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// DecodeMessage deserializes a JSON byte slice into a TCPMessage.
func DecodeMessage(data []byte) (*TCPMessage, error) {
	var msg TCPMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}
	return &msg, nil
}

// SendMessage encodes and writes a TCPMessage to a connection with a write deadline.
func SendMessage(conn net.Conn, msg TCPMessage) error {
	data, err := EncodeMessage(msg)
	if err != nil {
		log.Printf("TCP: Failed to encode message: %v", err)
		return err
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(data); err != nil {
		log.Printf("TCP: Failed to write to connection: %v", err)
		return err
	}
	return nil
}

// NewWelcomeMessage creates a welcome message for new connections.
func NewWelcomeMessage() TCPMessage {
	return TCPMessage{
		Type:      "welcome",
		Message:   "Connected to MangaHub TCP Progress Sync Server. Send auth message with your JWT token.",
		Timestamp: time.Now().Unix(),
	}
}

// NewErrorMessage creates an error message.
func NewErrorMessage(errMsg string) TCPMessage {
	return TCPMessage{
		Type:      "error",
		Message:   errMsg,
		Timestamp: time.Now().Unix(),
	}
}

// NewBroadcastMessage creates a broadcast message from a progress update.
func NewBroadcastMessage(userID, mangaID string, chapter int) TCPMessage {
	return TCPMessage{
		Type:      "broadcast",
		UserID:    userID,
		MangaID:   mangaID,
		Chapter:   chapter,
		Timestamp: time.Now().Unix(),
		Message:   fmt.Sprintf("User %s read chapter %d of %s", userID, chapter, mangaID),
	}
}

// NewStatusMessage creates a status response message.
func NewStatusMessage(connectedUsers int, msg string) TCPMessage {
	return TCPMessage{
		Type:           "status",
		Message:        msg,
		ConnectedUsers: connectedUsers,
		Timestamp:      time.Now().Unix(),
	}
}
