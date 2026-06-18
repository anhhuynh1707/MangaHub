package websocket

import (
	"log"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

// ChatClient wraps a WebSocket connection with a buffered write channel
// to avoid concurrent write issues with gorilla/websocket.
type ChatClient struct {
	Conn     *ws.Conn
	Username string
	UserID   string
	Room     string
	Send     chan ChatMessage // Buffered channel for outgoing messages
}

// ChatHub is the central coordinator for all WebSocket chat connections.
// It manages client registration, unregistration, and message broadcasting.
// Hub.Run() must be started as a goroutine — it is the single central event loop.
type ChatHub struct {
	Clients    map[*ChatClient]bool // set of active clients
	Broadcast  chan ChatMessage
	Register   chan *ChatClient
	Unregister chan *ChatClient
	mu         sync.RWMutex

	// In-memory chat history (last N messages per room)
	History    map[string][]ChatMessage
	maxHistory int
}

// ChatMessage represents a single chat message.
type ChatMessage struct {
	Type      string   `json:"type"`                // "message", "system", "pm", "join", "leave", "users", "history", "error"
	UserID    string   `json:"user_id,omitempty"`
	Username  string   `json:"username,omitempty"`
	Message   string   `json:"message"`
	Recipient string   `json:"recipient,omitempty"` // For private messages
	Room      string   `json:"room,omitempty"`      // Chat room (e.g., "general", "one-piece")
	Users     []string `json:"users,omitempty"`     // For user list responses
	Timestamp int64    `json:"timestamp"`
}

// ClientConnection represents a new client connecting to the hub.
type ClientConnection struct {
	Conn     *ws.Conn
	Username string
	UserID   string
}

// NewChatHub creates a new ChatHub with initialized channels.
func NewChatHub() *ChatHub {
	return &ChatHub{
		Clients:    make(map[*ChatClient]bool),
		Broadcast:  make(chan ChatMessage, 256),
		Register:   make(chan *ChatClient),
		Unregister: make(chan *ChatClient),
		History:    make(map[string][]ChatMessage),
		maxHistory: 50,
	}
}

// Run is the central goroutine that processes all hub events.
// It listens for register, unregister, and broadcast events
// and dispatches them to the appropriate handlers.
// Must be started with: go hub.Run()
func (h *ChatHub) Run() {
	log.Println("💬 WebSocket ChatHub started")
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client] = true
			h.mu.Unlock()

			log.Printf("WS: %s joined room %s (total in room: %d)", client.Username, client.Room, h.GetClientCount(client.Room))

			// Send join notification to all OTHER clients in the SAME room
			joinMsg := ChatMessage{
				Type:      "join",
				Username:  client.Username,
				Message:   client.Username + " joined the chat",
				Room:      client.Room,
				Timestamp: time.Now().Unix(),
			}
			h.mu.RLock()
			for c := range h.Clients {
				if c != client && c.Room == client.Room {
					select {
					case c.Send <- joinMsg:
					default:
					}
				}
			}
			h.mu.RUnlock()

			// Broadcast the authoritative (deduped) online-user list to the room
			h.broadcastUserList(client.Room)

			// Send recent history to the newly joined client for their room
			h.mu.RLock()
			roomHistory := h.History[client.Room]
			if len(roomHistory) > 0 {
				select {
				case client.Send <- ChatMessage{
					Type:      "history",
					Message:   "Recent messages:",
					Timestamp: time.Now().Unix(),
				}:
				default:
				}
				for _, msg := range roomHistory {
					select {
					case client.Send <- msg:
					default:
					}
				}
			}
			h.mu.RUnlock()

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.mu.Unlock()

			log.Printf("WS: %s left room %s (total in room: %d)", client.Username, client.Room, h.GetClientCount(client.Room))

			// Notify remaining clients in the SAME room
			leaveMsg := ChatMessage{
				Type:      "leave",
				Username:  client.Username,
				Message:   client.Username + " left the chat",
				Room:      client.Room,
				Timestamp: time.Now().Unix(),
			}
			h.mu.RLock()
			for c := range h.Clients {
				if c.Room == client.Room {
					select {
					case c.Send <- leaveMsg:
					default:
					}
				}
			}
			h.mu.RUnlock()

			// Broadcast the updated (deduped) online-user list to the room
			h.broadcastUserList(client.Room)

		case msg := <-h.Broadcast:
			// Store in history (for regular messages only)
			if msg.Type == "message" {
				h.mu.Lock()
				if h.History[msg.Room] == nil {
					h.History[msg.Room] = make([]ChatMessage, 0)
				}
				h.History[msg.Room] = append(h.History[msg.Room], msg)
				if len(h.History[msg.Room]) > h.maxHistory {
					h.History[msg.Room] = h.History[msg.Room][len(h.History[msg.Room])-h.maxHistory:]
				}
				h.mu.Unlock()
			}

			// Broadcast to all connected clients in the SAME room
			h.mu.RLock()
			for c := range h.Clients {
				if c.Room == msg.Room {
					select {
					case c.Send <- msg:
					default:
						// Client's send buffer is full; skip
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// GetClientCount returns the number of connected clients in a specific room.
func (h *ChatHub) GetClientCount(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for c := range h.Clients {
		if c.Room == room {
			count++
		}
	}
	return count
}

// GetOnlineUsers returns a list of all connected usernames in a specific room.
func (h *ChatHub) GetOnlineUsers(room string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Dedupe by username — a user may hold several connections (tabs, reconnects)
	// but should appear only once in the online list.
	seen := make(map[string]bool)
	users := make([]string, 0)
	for c := range h.Clients {
		if c.Room == room && !seen[c.Username] {
			seen[c.Username] = true
			users = append(users, c.Username)
		}
	}
	return users
}

// broadcastUserList sends the current deduped online-user list to every client
// in the room as a "users" message, giving clients an authoritative presence
// snapshot on each join/leave (robust against multiple connections per user).
func (h *ChatHub) broadcastUserList(room string) {
	users := h.GetOnlineUsers(room) // locks internally; call before taking RLock below

	msg := ChatMessage{
		Type:      "users",
		Room:      room,
		Users:     users,
		Timestamp: time.Now().Unix(),
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.Clients {
		if c.Room == room {
			select {
			case c.Send <- msg:
			default:
			}
		}
	}
}

// SendToClient sends a message to a specific client via its channel.
func (h *ChatHub) SendToClient(client *ChatClient, msg ChatMessage) {
	select {
	case client.Send <- msg:
	default:
	}
}

// SendPrivateMessage sends a message to a specific user by username.
func (h *ChatHub) SendPrivateMessage(from, to, message string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.Clients {
		if c.Username == to {
			select {
			case c.Send <- ChatMessage{
				Type:      "pm",
				Username:  from,
				Recipient: to,
				Message:   message,
				Room:      "pm",
				Timestamp: time.Now().Unix(),
			}:
			default:
			}
			return true
		}
	}
	return false
}

// GetHistory returns a copy of the chat history for a room.
func (h *ChatHub) GetHistory(room string, limit int) []ChatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()

	roomHistory := h.History[room]
	if limit <= 0 || limit > len(roomHistory) {
		limit = len(roomHistory)
	}

	start := len(roomHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]ChatMessage, limit)
	copy(result, roomHistory[start:])
	return result
}
