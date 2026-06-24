package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"mangahub/internal/auth"
	"mangahub/internal/sse"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// EventStream is the browser-facing SSE endpoint (GET /events/stream?token=).
// EventSource cannot send an Authorization header, so the JWT is passed as a
// query param and validated here — the same approach as the chat WebSocket.
func (s *APIServer) EventStream(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		utils.UnauthorizedResponse(c, "Missing token query parameter")
		return
	}
	if _, err := auth.ValidateToken(token); err != nil {
		utils.UnauthorizedResponse(c, "Invalid or expired token")
		return
	}

	client := &sse.Client{Send: make(chan sse.Event, 16)}
	s.SSE.Register <- client
	defer func() { s.SSE.Unregister <- client }()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering (nginx)
	c.Writer.Flush()

	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	// c.Stream auto-flushes after each callback; returning false ends the stream.
	c.Stream(func(w io.Writer) bool {
		select {
		case ev, ok := <-client.Send:
			if !ok {
				return false
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				return true
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			return true
		case <-keepalive.C:
			fmt.Fprint(w, ": ping\n\n") // comment line keeps the connection alive
			return true
		case <-c.Request.Context().Done():
			return false // client disconnected
		}
	})
}
