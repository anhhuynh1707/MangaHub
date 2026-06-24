// Package sse provides a tiny Server-Sent Events hub that fans backend events
// (UDP notifications, TCP progress updates) out to connected browser clients.
// Browsers cannot speak raw TCP/UDP, so the API server bridges those events to
// the SPA over SSE. The TCP/UDP servers and the CLI are unaffected.
package sse

import (
	"sync"
	"time"
)

// Event is a single server-sent event delivered to browser clients.
type Event struct {
	Type      string `json:"type"` // "notification" | "progress"
	Data      any    `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// Client is one connected browser stream.
type Client struct {
	Send chan Event
}

// Hub broadcasts events to every connected client (mirrors UDP broadcast
// semantics). Run() must be started as a goroutine: it is the single owner of
// the client set.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan Event
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan Event, 64),
	}
}

// Run is the central goroutine processing register/unregister/broadcast events.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.Register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()

		case c := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.Send)
			}
			h.mu.Unlock()

		case ev := <-h.Broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.Send <- ev:
				default: // drop for a slow/blocked client rather than stall the hub
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Publish is a non-blocking helper to broadcast an event of the given type.
func (h *Hub) Publish(eventType string, data any) {
	select {
	case h.Broadcast <- Event{Type: eventType, Data: data, Timestamp: time.Now().Unix()}:
	default:
	}
}

// ClientCount returns the number of connected SSE clients (used by /health).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
