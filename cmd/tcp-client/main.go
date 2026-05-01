package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// TCPMessage matches the server's message format
type TCPMessage struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	MangaID   string `json:"manga_id,omitempty"`
	Chapter   int    `json:"chapter,omitempty"`
	Message   string `json:"message,omitempty"`
	Token     string `json:"token,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run test_tcp_client.go <username> <jwt-token>")
		fmt.Println("  or:  go run test_tcp_client.go <username> token-file:<path>")
		os.Exit(1)
	}

	username := os.Args[1]
	token := os.Args[2]

	// Support reading token from file
	if strings.HasPrefix(token, "token-file:") {
		filePath := strings.TrimPrefix(token, "token-file:")
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Failed to read token file: %v\n", err)
			os.Exit(1)
		}
		token = strings.TrimSpace(string(data))
	}

	fmt.Printf("[%s] Connecting to TCP server at localhost:9090...\n", username)

	conn, err := net.DialTimeout("tcp", "localhost:9090", 5*time.Second)
	if err != nil {
		fmt.Printf("[%s] Failed to connect: %v\n", username, err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("[%s] Connected!\n", username)

	// Start reader goroutine
	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var msg TCPMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				fmt.Printf("[%s] Raw: %s\n", username, scanner.Text())
				continue
			}

			switch msg.Type {
			case "welcome":
				fmt.Printf("[%s] 🟢 %s\n", username, msg.Message)
			case "auth":
				fmt.Printf("[%s] ✅ Authenticated as %s (ID: %s)\n", username, msg.Username, msg.UserID)
			case "broadcast":
				fmt.Printf("[%s] 📢 BROADCAST: %s → %s ch.%d\n", username, msg.UserID, msg.MangaID, msg.Chapter)
			case "user_joined":
				fmt.Printf("[%s] 👋 %s joined sync\n", username, msg.Username)
			case "user_left":
				fmt.Printf("[%s] 👋 %s left sync\n", username, msg.Username)
			case "error":
				fmt.Printf("[%s] ❌ Error: %s\n", username, msg.Message)
			case "pong":
				fmt.Printf("[%s] 🏓 Pong!\n", username)
			default:
				fmt.Printf("[%s] [%s] %s\n", username, msg.Type, msg.Message)
			}
		}
		fmt.Printf("[%s] Connection closed\n", username)
	}()

	// Wait for welcome message
	time.Sleep(200 * time.Millisecond)

	// Authenticate
	sendMessage(conn, username, TCPMessage{
		Type:  "auth",
		Token: token,
	})

	time.Sleep(300 * time.Millisecond)

	// Interactive mode: read from stdin
	fmt.Printf("\n[%s] Commands: progress <manga_id> <chapter> | ping | quit\n", username)
	stdinScanner := bufio.NewScanner(os.Stdin)
	for stdinScanner.Scan() {
		line := strings.TrimSpace(stdinScanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		switch parts[0] {
		case "progress":
			if len(parts) < 3 {
				fmt.Println("Usage: progress <manga_id> <chapter>")
				continue
			}
			ch := 0
			fmt.Sscanf(parts[2], "%d", &ch)
			sendMessage(conn, username, TCPMessage{
				Type:    "progress",
				MangaID: parts[1],
				Chapter: ch,
			})
		case "ping":
			sendMessage(conn, username, TCPMessage{Type: "ping"})
		case "quit":
			sendMessage(conn, username, TCPMessage{Type: "disconnect"})
			time.Sleep(200 * time.Millisecond)
			return
		default:
			fmt.Printf("Unknown command: %s\n", parts[0])
		}
	}
}

func sendMessage(conn net.Conn, username string, msg TCPMessage) {
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		fmt.Printf("[%s] Failed to send: %v\n", username, err)
	}
}
