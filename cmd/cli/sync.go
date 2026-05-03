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
	Type           string `json:"type"`
	UserID         string `json:"user_id,omitempty"`
	Username       string `json:"username,omitempty"`
	MangaID        string `json:"manga_id,omitempty"`
	Chapter        int    `json:"chapter,omitempty"`
	Message        string `json:"message,omitempty"`
	Token          string `json:"token,omitempty"`
	Timestamp      int64  `json:"timestamp,omitempty"`
	ConnectedUsers int    `json:"connected_users,omitempty"`
}

func handleSync(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub sync <connect|disconnect|status|monitor>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  connect     Connect to TCP sync server (interactive)")
		fmt.Println("  disconnect  Disconnect from sync server")
		fmt.Println("  status      Check TCP server status and connected users")
		fmt.Println("  monitor     Watch live progress updates (read-only)")
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

// syncConnect establishes an interactive TCP connection to the sync server.
// Supports: progress <manga-id> <chapter>, ping, status, quit
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

	connectedAt := time.Now()

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
	fmt.Printf("  Profile:   %s\n", getProfileName())
	fmt.Printf("  Connected: %s\n", connectedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("Real-time sync is now active.")
	fmt.Println("Commands: progress <manga-id> <chapter> | ping | status | quit")
	fmt.Println()

	// Track stats
	msgsSent := 0
	msgsReceived := 0

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
			msgsReceived++
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
			case "status":
				fmt.Printf("[%s] 📊 %s\n", ts, msg.Message)
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
				msgsSent++
				fmt.Printf("[%s] → Broadcasting update: %s → Chapter %d\n", time.Now().Format("15:04:05"), parts[1], ch)
			case "ping":
				sendTCP(conn, tcpMessage{Type: "ping"})
				msgsSent++
			case "status":
				sendTCP(conn, tcpMessage{Type: "status"})
				msgsSent++
			case "quit", "exit":
				sendTCP(conn, tcpMessage{Type: "disconnect"})
				uptime := time.Since(connectedAt).Round(time.Second)
				fmt.Println("✓ Disconnected from sync server")
				fmt.Printf("  Session: %s | Sent: %d | Received: %d\n", uptime, msgsSent, msgsReceived)
				os.Exit(0)
			default:
				fmt.Println("Commands: progress <manga-id> <chapter> | ping | status | quit")
			}
		}
	}()

	select {
	case <-interrupt:
		sendTCP(conn, tcpMessage{Type: "disconnect"})
		uptime := time.Since(connectedAt).Round(time.Second)
		fmt.Printf("\n✓ Disconnected from sync server")
		fmt.Printf("\n  Session: %s | Sent: %d | Received: %d\n", uptime, msgsSent, msgsReceived)
	case <-done:
		fmt.Println("Connection closed by server")
	}
}

// syncStatus queries the HTTP API for TCP sync server status.
func syncStatus() {
	cfg := requireAuth()

	// Query the HTTP API for sync status
	resp, err := apiRequest("GET", "/sync/status", nil, cfg.Token)
	if err != nil {
		// Fallback: try direct TCP connection check
		conn, tcpErr := net.DialTimeout("tcp", "localhost:9090", 2*time.Second)
		if tcpErr != nil {
			fmt.Println("TCP Sync Status:")
			fmt.Println("  Connection: ✗ Server unreachable")
			fmt.Println("  Server:     localhost:9090")
			fmt.Printf("  Error:      %v\n", err)
			return
		}
		conn.Close()
		fmt.Println("TCP Sync Status:")
		fmt.Println("  Connection: ✓ Server available (API unreachable)")
		fmt.Println("  Server:     localhost:9090")
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var status struct {
		Server         string   `json:"server"`
		Uptime         string   `json:"uptime"`
		ConnectedCount int      `json:"connected_count"`
		ConnectedUsers []string `json:"connected_users"`
		YourUserID     string   `json:"your_user_id"`
	}
	json.Unmarshal(resp.Data, &status)

	fmt.Println("TCP Sync Status:")
	fmt.Printf("  Connection: ✓ Active\n")
	fmt.Printf("  Server:     %s\n", status.Server)
	fmt.Printf("  Uptime:     %s\n", status.Uptime)
	fmt.Println()
	fmt.Println("Session Info:")
	fmt.Printf("  User:       %s\n", cfg.Username)
	fmt.Printf("  User ID:    %s\n", status.YourUserID)
	fmt.Printf("  Profile:    %s\n", getProfileName())
	fmt.Println()
	fmt.Printf("Connected Users: %d\n", status.ConnectedCount)
	if len(status.ConnectedUsers) > 0 {
		for _, u := range status.ConnectedUsers {
			fmt.Printf("  • %s\n", u)
		}
	} else {
		fmt.Println("  (none)")
	}
	fmt.Println()
	fmt.Println("Connect with: mangahub sync connect")
}

// syncMonitor connects to the TCP server in read-only mode to watch live updates.
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

	// Read welcome
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
		fmt.Printf("✓ Authenticated as %s\n", cfg.Username)
	}

	fmt.Printf("Monitoring real-time sync updates... (Press Ctrl+C to exit)\n\n")

	updateCount := 0

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
				updateCount++
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
	fmt.Printf("\n✓ Stopped monitoring (%d updates received)\n", updateCount)
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
		fmt.Println("  go run ./cmd/api-server/")
	}
}
