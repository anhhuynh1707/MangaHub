package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"
)

// tcpMessage matches the server's TCP message format.
type tcpMessage struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id,omitempty"`
	Username  string `json:"username,omitempty"`
	MangaID   string `json:"manga_id,omitempty"`
	Chapter   int    `json:"chapter,omitempty"`
	Message   string `json:"message,omitempty"`
	Token     string `json:"token,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

func handleSync(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub sync <connect|disconnect|status|monitor>")
		return
	}

	switch args[0] {
	case "connect":
		syncConnect()
	case "status":
		syncStatus()
	case "monitor":
		syncMonitor()
	case "disconnect":
		fmt.Println("✓ Disconnected from sync server")
	default:
		fmt.Printf("✗ Unknown sync command: '%s'\n", args[0])
		fmt.Println("Available: connect, disconnect, status, monitor")
	}
}

func syncConnect() {
	cfg := requireAuth()

	fmt.Println("Connecting to TCP sync server at localhost:9090...")

	conn, err := net.DialTimeout("tcp", "localhost:9090", 5*time.Second)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		fmt.Println("  Check server status: mangahub server status")
		return
	}
	defer conn.Close()

	// Read welcome
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)

	if scanner.Scan() {
		var msg tcpMessage
		json.Unmarshal(scanner.Bytes(), &msg)
	}

	// Authenticate
	sendTCP(conn, tcpMessage{Type: "auth", Token: cfg.Token})
	if scanner.Scan() {
		var msg tcpMessage
		json.Unmarshal(scanner.Bytes(), &msg)
		if msg.Type == "error" {
			fmt.Printf("✗ Authentication failed: %s\n", msg.Message)
			return
		}
	}

	fmt.Printf("✓ Connected successfully!\n\n")
	fmt.Println("Connection Details:")
	fmt.Printf("  Server:    localhost:9090\n")
	fmt.Printf("  User:      %s (%s)\n", cfg.Username, cfg.UserID)
	fmt.Printf("  Connected: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("\nReal-time sync is now active.")
	fmt.Println("Type 'progress <manga-id> <chapter>' to send updates, 'quit' to exit.")

	// Handle Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Reader goroutine
	done := make(chan bool)
	go func() {
		for scanner.Scan() {
			var msg tcpMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			ts := time.Unix(msg.Timestamp, 0).Format("15:04:05")
			switch msg.Type {
			case "broadcast":
				fmt.Printf("[%s] ← %s updated: %s → Chapter %d\n", ts, msg.UserID, msg.MangaID, msg.Chapter)
			case "user_joined":
				fmt.Printf("[%s] 👋 %s joined sync\n", ts, msg.Username)
			case "user_left":
				fmt.Printf("[%s] 👋 %s left sync\n", ts, msg.Username)
			case "pong":
				fmt.Printf("[%s] 🏓 Pong\n", ts)
			case "error":
				fmt.Printf("[%s] ✗ %s\n", ts, msg.Message)
			}
		}
		done <- true
	}()

	// Stdin reader
	go func() {
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
					fmt.Println("Usage: progress <manga-id> <chapter>")
					continue
				}
				ch := 0
				fmt.Sscanf(parts[2], "%d", &ch)
				sendTCP(conn, tcpMessage{Type: "progress", MangaID: parts[1], Chapter: ch})
				fmt.Printf("[%s] → Broadcasting update: %s → Chapter %d\n", time.Now().Format("15:04:05"), parts[1], ch)
			case "ping":
				sendTCP(conn, tcpMessage{Type: "ping"})
			case "quit", "exit":
				sendTCP(conn, tcpMessage{Type: "disconnect"})
				fmt.Println("✓ Disconnected from sync server")
				os.Exit(0)
			default:
				fmt.Println("Commands: progress <manga-id> <chapter> | ping | quit")
			}
		}
	}()

	select {
	case <-interrupt:
		sendTCP(conn, tcpMessage{Type: "disconnect"})
		fmt.Println("\n✓ Disconnected from sync server")
	case <-done:
		fmt.Println("Connection closed by server")
	}
}

func syncStatus() {
	cfg := requireAuth()

	// Try to connect briefly to check
	conn, err := net.DialTimeout("tcp", "localhost:9090", 2*time.Second)
	if err != nil {
		fmt.Println("TCP Sync Status:")
		fmt.Println("  Connection: ✗ Not connected")
		fmt.Println("  Server:     localhost:9090 (unreachable)")
		return
	}
	conn.Close()

	fmt.Println("TCP Sync Status:")
	fmt.Println("  Connection: ✓ Server available")
	fmt.Println("  Server:     localhost:9090")
	fmt.Printf("  User:       %s\n", cfg.Username)
	fmt.Println("\nConnect with: mangahub sync connect")
}

func syncMonitor() {
	cfg := requireAuth()

	fmt.Println("Connecting to monitor sync updates...")

	conn, err := net.DialTimeout("tcp", "localhost:9090", 5*time.Second)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	scanner.Scan() // welcome

	sendTCP(conn, tcpMessage{Type: "auth", Token: cfg.Token})
	scanner.Scan() // auth response

	fmt.Println("Monitoring real-time sync updates... (Press Ctrl+C to exit)\n")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	go func() {
		for scanner.Scan() {
			var msg tcpMessage
			if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
				continue
			}
			ts := time.Unix(msg.Timestamp, 0).Format("15:04:05")
			switch msg.Type {
			case "broadcast":
				fmt.Printf("[%s] ← %s updated: %s → Chapter %d\n", ts, msg.UserID, msg.MangaID, msg.Chapter)
			case "user_joined":
				fmt.Printf("[%s] 👋 %s joined\n", ts, msg.Username)
			case "user_left":
				fmt.Printf("[%s] 👋 %s left\n", ts, msg.Username)
			}
		}
	}()

	<-interrupt
	sendTCP(conn, tcpMessage{Type: "disconnect"})
	fmt.Println("\n✓ Stopped monitoring")
}

func sendTCP(conn net.Conn, msg tcpMessage) {
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	conn.Write(data)
}

func handleServer(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub server <status|start>")
		return
	}

	switch args[0] {
	case "status":
		resp, err := apiRequest("GET", "/health", nil, "")
		if err != nil {
			fmt.Println("Server Status: ✗ Offline")
			fmt.Printf("  Error: %v\n", err)
			fmt.Println("  Start server: go run ./cmd/api-server/")
			return
		}
		fmt.Println("Server Status: ✓ Online")
		fmt.Printf("  %s\n", resp.Message)
	case "start":
		fmt.Println("Start the server in a separate terminal:")
		fmt.Println("  cd c:\\Users\\Dell\\Documents\\Go\\mangahub")
		fmt.Println("  go run ./cmd/api-server/")
	}
}
