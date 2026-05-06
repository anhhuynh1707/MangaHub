package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	ws "github.com/gorilla/websocket"
)

// wsChatMessage matches the server's WebSocket message format.
type wsChatMessage struct {
	Type      string   `json:"type"`
	UserID    string   `json:"user_id,omitempty"`
	Username  string   `json:"username,omitempty"`
	Message   string   `json:"message"`
	Recipient string   `json:"recipient,omitempty"`
	Room      string   `json:"room,omitempty"`
	Users     []string `json:"users,omitempty"`
	Timestamp int64    `json:"timestamp"`
}

func handleChat(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub chat <join|send|history>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  join <room>              Join a chat room (interactive mode)")
		fmt.Println("  send <room> <message>    Send a single message to a room")
		fmt.Println("  history <room>           View recent chat history for a room")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  mangahub chat join general")
		fmt.Println("  mangahub chat join one-piece")
		fmt.Println("  mangahub chat send one-piece \"Hello One Piece fans!\"")
		fmt.Println("  mangahub chat history general --limit 30")
		fmt.Println()
		fmt.Println("Interactive commands (inside chat):")
		fmt.Println("  /help             Show chat commands")
		fmt.Println("  /users            List online users")
		fmt.Println("  /quit             Leave chat")
		fmt.Println("  /pm <user> <msg>  Private message")
		fmt.Println("  /history          Show recent history")
		fmt.Println("  /status           Connection status")
		return
	}

	switch args[0] {
	case "join":
		chatJoin(args[1:])
	case "send":
		chatSend(args[1:])
	case "history":
		chatHistory(args[1:])
	default:
		fmt.Printf("✗ Unknown chat command: '%s'\n", args[0])
		fmt.Println("Available: join, send, history")
	}
}

// chatJoin connects to the WebSocket chat server in interactive mode.
func chatJoin(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub chat join <room>")
		fmt.Println("Example: mangahub chat join general")
		fmt.Println("         mangahub chat join one-piece")
		return
	}

	cfg := requireAuth()

	room := args[0]

	// Build WebSocket URL with token auth
	wsURL := strings.Replace(cfg.ServerURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	params := url.Values{}
	params.Set("token", cfg.Token)
	params.Set("room", room)
	fullURL := wsURL + "/ws/chat?" + params.Encode()

	fmt.Printf("Connecting to WebSocket chat server at %s/ws/chat...\n", wsURL)

	conn, _, err := ws.DefaultDialer.Dial(fullURL, nil)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		fmt.Println("  Make sure the server is running: go run ./cmd/api-server/")
		return
	}
	defer conn.Close()

	connectedAt := time.Now()
	msgsSent := 0
	msgsReceived := 0

	// Set up pong handler so server pings keep the connection alive
	conn.SetPongHandler(func(string) error {
		return nil
	})

	// Handle Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Reader goroutine — receives ALL messages from the server
	done := make(chan bool)
	welcomeReceived := make(chan bool, 1)
	go func() {
		firstMessage := true
		for {
			var msg wsChatMessage
			if err := conn.ReadJSON(&msg); err != nil {
				done <- true
				return
			}
			msgsReceived++

			// First message is the welcome
			if firstMessage && msg.Type == "system" {
				firstMessage = false
				roomName := formatRoomName(room)
				fmt.Printf("✓ Connected to %s\n\n", roomName)
				fmt.Printf("  Chat Room:  #%s\n", room)
				if len(msg.Users) > 0 {
					fmt.Printf("  Online:     %d users\n", len(msg.Users))
				}
				fmt.Printf("  Your Name:  %s\n", cfg.Username)
				fmt.Printf("  Profile:    %s\n", getProfileName())
				fmt.Printf("  Connected:  %s\n", connectedAt.Format("2006-01-02 15:04:05"))

				fmt.Println("\n─────────────────────────────────────────────────────────────")
				fmt.Println("You are now in chat. Type your message and press Enter.")
				fmt.Println("Type /help for commands or /quit to leave.")
				fmt.Println()
				fmt.Printf("%s> ", cfg.Username)
				welcomeReceived <- true
				continue
			}

			// Display history messages inline
			if msg.Type == "history" {
				fmt.Printf("\r📜 %s\n", msg.Message)
				fmt.Printf("%s> ", cfg.Username)
				continue
			}

			displayChatMessage(&msg, cfg.Username)
		}
	}()

	// Wait for welcome before starting stdin reader
	select {
	case <-welcomeReceived:
	case <-time.After(5 * time.Second):
		fmt.Println("✗ Timeout waiting for server welcome")
		return
	case <-done:
		fmt.Println("Connection closed by server")
		return
	}

	// Stdin reader goroutine — reads user input
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				fmt.Printf("%s> ", cfg.Username)
				continue
			}

			// Handle /quit locally
			if strings.TrimSpace(strings.ToLower(line)) == "/quit" {
				conn.WriteJSON(wsChatMessage{
					Type:    "message",
					Message: "/quit",
				})
				uptime := time.Since(connectedAt).Round(time.Second)
				fmt.Println("\nLeaving chat...")
				fmt.Printf("✓ Disconnected from chat server\n")
				fmt.Printf("  Session: %s | Sent: %d | Received: %d\n", uptime, msgsSent, msgsReceived)
				os.Exit(0)
			}

			// Send message (or command) to server
			conn.WriteJSON(wsChatMessage{
				Type:    "message",
				Message: line,
			})
			msgsSent++
		}
	}()

	select {
	case <-interrupt:
		conn.WriteMessage(ws.CloseMessage,
			ws.FormatCloseMessage(ws.CloseNormalClosure, ""))
		uptime := time.Since(connectedAt).Round(time.Second)
		fmt.Printf("\n✓ Disconnected from chat server\n")
		fmt.Printf("  Session: %s | Sent: %d | Received: %d\n", uptime, msgsSent, msgsReceived)
	case <-done:
		fmt.Println("\nConnection closed by server")
	}
}

// displayChatMessage formats and prints a received chat message.
func displayChatMessage(msg *wsChatMessage, myUsername string) {
	ts := time.Unix(msg.Timestamp, 0).Format("15:04")

	switch msg.Type {
	case "message":
		fmt.Printf("\r[%s] %s: %s\n", ts, msg.Username, msg.Message)
	case "pm":
		if msg.Username == myUsername {
			fmt.Printf("\r[%s] (PM → %s): %s\n", ts, msg.Recipient, msg.Message)
		} else {
			fmt.Printf("\r[%s] (PM from %s): %s\n", ts, msg.Username, msg.Message)
		}
	case "join":
		fmt.Printf("\r[%s] 👋 %s joined the chat\n", ts, msg.Username)
	case "leave":
		fmt.Printf("\r[%s] 👋 %s left the chat\n", ts, msg.Username)
	case "system":
		fmt.Printf("\r[%s] 🔔 %s\n", ts, msg.Message)
	case "users":
		fmt.Printf("\r[%s] %s\n", ts, msg.Message)
		for _, u := range msg.Users {
			marker := "●"
			if u == myUsername || strings.HasPrefix(u, myUsername+" (") {
				marker = "★"
			}
			fmt.Printf("  %s %s\n", marker, u)
		}
	case "error":
		fmt.Printf("\r[%s] ✗ %s\n", ts, msg.Message)
	case "history":
		fmt.Printf("\r[%s] 📜 %s\n", ts, msg.Message)
	}

	// Reprint the prompt
	fmt.Printf("%s> ", myUsername)
}

// formatRoomName converts a room slug to a display-friendly name.
func formatRoomName(room string) string {
	if room == "general" {
		return "General Chat"
	}
	return strings.Title(strings.ReplaceAll(room, "-", " "))
}

// chatSend sends a single message without entering interactive mode.
func chatSend(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: mangahub chat send <room> \"your message here\"")
		fmt.Println("Example: mangahub chat send general \"Hello everyone!\"")
		fmt.Println("         mangahub chat send one-piece \"New chapter is fire!\"")
		return
	}

	cfg := requireAuth()

	room := args[0]
	args = args[1:] // remaining args are the message

	wsURL := strings.Replace(cfg.ServerURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	params := url.Values{}
	params.Set("token", cfg.Token)
	params.Set("room", room)
	fullURL := wsURL + "/ws/chat?" + params.Encode()

	conn, _, err := ws.DefaultDialer.Dial(fullURL, nil)
	if err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Read welcome
	var welcome wsChatMessage
	conn.ReadJSON(&welcome)

	// Drain any history messages quickly
	func() {
		for {
			conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
			var msg wsChatMessage
			if conn.ReadJSON(&msg) != nil {
				break
			}
		}
		conn.SetReadDeadline(time.Time{})
	}()

	// Send the message
	message := strings.Join(args, " ")
	
	conn.WriteJSON(wsChatMessage{
		Type:    "message",
		Message: message,
	})

	// Brief pause to let broadcast propagate
	time.Sleep(200 * time.Millisecond)

	fmt.Printf("✓ Message sent: %s\n", message)
	conn.WriteMessage(ws.CloseMessage,
		ws.FormatCloseMessage(ws.CloseNormalClosure, ""))
}

// chatHistory fetches recent chat history via the HTTP API.
func chatHistory(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub chat history <room> [--limit <n>]")
		fmt.Println("Example: mangahub chat history general")
		fmt.Println("         mangahub chat history one-piece --limit 30")
		return
	}

	cfg := requireAuth()

	room := args[0]

	limit := parseFlag(args[1:], "limit")
	if limit == "" {
		limit = "20"
	}

	resp, err := apiRequest("GET", "/chat/history?limit="+limit+"&room="+room, nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch history: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var messages []wsChatMessage
	json.Unmarshal(resp.Data, &messages)

	if len(messages) == 0 {
		fmt.Printf("No chat messages yet. Join the chat: mangahub chat join %s\n", room)
		return
	}

	fmt.Printf("📜 Chat History (%d messages)\n\n", len(messages))
	for _, msg := range messages {
		ts := time.Unix(msg.Timestamp, 0).Format("15:04:05")
		fmt.Printf("[%s] %s: %s\n", ts, msg.Username, msg.Message)
	}
}
