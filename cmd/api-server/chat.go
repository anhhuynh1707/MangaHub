package main

import (
	"fmt"
	"strconv"

	wsPkg "mangahub/internal/websocket"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ChatWebSocket upgrades the connection to the chat hub (auth via ?token= query).
func (s *APIServer) ChatWebSocket(c *gin.Context) {
	wsPkg.HandleWebSocket(s.Hub, c.Writer, c.Request)
}

// ChatHistory returns recent messages for a room.
func (s *APIServer) ChatHistory(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	room := c.DefaultQuery("room", "general")
	history := s.Hub.GetHistory(room, limit)
	utils.SuccessResponse(c, fmt.Sprintf("%d messages", len(history)), history)
}
