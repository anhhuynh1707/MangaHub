package websocket

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"mangahub/internal/auth"

	ws "github.com/gorilla/websocket"
)

// upgrader configures the WebSocket upgrade from HTTP.
var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWebSocket upgrades an HTTP connection to WebSocket and
// starts the per-client read/write pumps. Authentication is done via
// the "token" query parameter.
func HandleWebSocket(hub *ChatHub, w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from query parameter: /ws/chat?token=xxx
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token query parameter", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ValidateToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Upgrade HTTP -> WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS: Upgrade failed: %v", err)
		return
	}

	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "Missing room query parameter", http.StatusBadRequest)
		return
	}

	// Create client with buffered send channel
	client := &ChatClient{
		Conn:     conn,
		Username: claims.Username,
		UserID:   claims.UserID,
		Room:     room,
		Send:     make(chan ChatMessage, 256),
	}

	// Register with the hub
	hub.Register <- client

	// Send welcome message via the send channel
	client.Send <- ChatMessage{
		Type:      "system",
		Message:   fmt.Sprintf("Welcome to MangaHub Chat, %s! Type /help for commands.", claims.Username),
		Room:      room,
		Users:     hub.GetOnlineUsers(room),
		Timestamp: time.Now().Unix(),
	}

	// Start write pump (sends messages from channel to WebSocket)
	go writePump(client)

	// Start read pump (reads messages from WebSocket, blocks until disconnect)
	readPump(hub, client)
}

// readPump reads messages from a single WebSocket client connection.
// It runs in the handler's goroutine (one per client). When the client
// disconnects or an error occurs, it unregisters from the hub.
func readPump(hub *ChatHub, client *ChatClient) {
	defer func() {
		hub.Unregister <- client
		client.Conn.Close()
	}()

	// Keepalive: 5 minute read deadline, reset on every pong or message
	const pongWait = 5 * time.Minute
	client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg ChatMessage
		if err := client.Conn.ReadJSON(&msg); err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseNormalClosure) {
				log.Printf("WS: Read error from %s: %v", client.Username, err)
			}
			break
		}

		// Reset read deadline on every message
		client.Conn.SetReadDeadline(time.Now().Add(pongWait))

		// Stamp the message with sender info
		msg.Username = client.Username
		msg.UserID = client.UserID
		msg.Timestamp = time.Now().Unix()

		// Handle commands vs regular messages
		handleClientMessage(hub, client, &msg)
	}
}

// writePump pumps messages from the client's Send channel to the WebSocket connection.
// This is the ONLY goroutine that writes to the connection, preventing concurrent writes.
func writePump(client *ChatClient) {
	// Ping every 25 seconds to keep connection alive
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				// Channel was closed (client unregistered)
				client.Conn.WriteMessage(ws.CloseMessage, []byte{})
				return
			}
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteJSON(msg); err != nil {
				log.Printf("WS: Write error to %s: %v", client.Username, err)
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(ws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleClientMessage processes an incoming message from a client.
// It handles chat commands (/help, /users, /quit, /pm) and regular messages.
func handleClientMessage(hub *ChatHub, client *ChatClient, msg *ChatMessage) {
	text := strings.TrimSpace(msg.Message)

	// Handle slash commands
	if strings.HasPrefix(text, "/") {
		parts := strings.Fields(text)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "/help":
			hub.SendToClient(client, ChatMessage{
				Type: "system",
				Message: "Chat Commands:\n" +
					"  /help           - Show this help\n" +
					"  /users          - List online users\n" +
					"  /quit           - Leave chat\n" +
					"  /pm <user> <msg> - Private message\n" +
					"  /history        - Show recent history\n" +
					"  /status         - Connection status",
				Timestamp: time.Now().Unix(),
			})

		case "/users":
			users := hub.GetOnlineUsers(client.Room)
			hub.SendToClient(client, ChatMessage{
				Type:      "users",
				Message:   fmt.Sprintf("Online Users in #%s (%d):", client.Room, len(users)),
				Users:     users,
				Timestamp: time.Now().Unix(),
			})

		case "/quit":
			hub.SendToClient(client, ChatMessage{
				Type:      "system",
				Message:   "Goodbye! Disconnecting...",
				Timestamp: time.Now().Unix(),
			})
			// Small delay to let the message flush through writePump
			time.Sleep(100 * time.Millisecond)
			client.Conn.Close()

		case "/pm":
			if len(parts) < 3 {
				hub.SendToClient(client, ChatMessage{
					Type:      "error",
					Message:   "Usage: /pm <username> <message>",
					Timestamp: time.Now().Unix(),
				})
				return
			}
			target := parts[1]
			pmMsg := strings.Join(parts[2:], " ")

			if target == client.Username {
				hub.SendToClient(client, ChatMessage{
					Type:      "error",
					Message:   "You can't PM yourself!",
					Timestamp: time.Now().Unix(),
				})
				return
			}

			found := hub.SendPrivateMessage(client.Username, target, pmMsg)
			if found {
				// Confirm to sender
				hub.SendToClient(client, ChatMessage{
					Type:      "pm",
					Username:  client.Username,
					Recipient: target,
					Message:   pmMsg,
					Room:      "pm",
					Timestamp: time.Now().Unix(),
				})
			} else {
				hub.SendToClient(client, ChatMessage{
					Type:      "error",
					Message:   fmt.Sprintf("User '%s' is not online", target),
					Timestamp: time.Now().Unix(),
				})
			}

		case "/history":
			history := hub.GetHistory(client.Room, 20)
			hub.SendToClient(client, ChatMessage{
				Type:      "system",
				Message:   fmt.Sprintf("Recent messages (%d):", len(history)),
				Timestamp: time.Now().Unix(),
			})
			for _, h := range history {
				hub.SendToClient(client, h)
			}

		case "/status":
			hub.SendToClient(client, ChatMessage{
				Type:      "system",
				Message:   fmt.Sprintf("Connected as: %s (%s)\nRoom: #%s\nUsers in room: %d", client.Username, client.UserID, client.Room, hub.GetClientCount(client.Room)),
				Timestamp: time.Now().Unix(),
			})

		default:
			hub.SendToClient(client, ChatMessage{
				Type:      "error",
				Message:   fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd),
				Timestamp: time.Now().Unix(),
			})
		}
		return
	}

	// Regular chat message — broadcast to all in the same room
	if text != "" {
		msg.Type = "message"
		msg.Room = client.Room
		hub.Broadcast <- *msg
	}
}
