package main

import (
	"fmt"
	"log"
	"time"

	"mangahub/internal/auth"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Recommendations returns collaborative-filtering recommendations for the user.
func (s *APIServer) Recommendations(c *gin.Context) {
	userID, err := auth.GetUserIDFromContext(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Unauthorized")
		return
	}
	limit := 10
	fmt.Sscanf(c.DefaultQuery("limit", "10"), "%d", &limit)

	result, err := s.RecService.GetRecommendations(userID, limit)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to generate recommendations: "+err.Error())
		return
	}
	utils.SuccessResponse(c, "Recommendations generated", result)
}

// UpdateProgress wraps the user handler's UpdateProgress and, on success,
// broadcasts the change to TCP clients and the gRPC event hub.
func (s *APIServer) UpdateProgress(c *gin.Context) {
	s.UserHandler.UpdateProgress(c)
	if c.Writer.Status() != 200 {
		return
	}

	userID, _ := auth.GetUserIDFromContext(c)
	mangaID := c.GetString("progress_manga_id")
	chapter := c.GetInt("progress_chapter")
	if mangaID == "" || chapter <= 0 {
		return
	}

	update := models.ProgressUpdate{
		UserID:    userID,
		MangaID:   mangaID,
		Chapter:   chapter,
		Timestamp: time.Now().Unix(),
	}

	if s.TCPClient != nil {
		if err := s.TCPClient.SendProgressUpdate(update); err != nil {
			log.Printf("TCP client error: %v", err)
		}
	} else if s.TCPServer != nil {
		s.TCPServer.SendProgressUpdate(update)
	}

	if s.GRPCMangaServer != nil {
		s.GRPCMangaServer.EventHub.PublishProgressUpdate(userID, mangaID, int32(chapter))
	}

	// Bridge the same progress update to browser clients over SSE (Phase 2).
	s.SSE.Publish("progress", gin.H{
		"user_id":  userID,
		"username": c.GetString("username"),
		"manga_id": mangaID,
		"chapter":  chapter,
	})
}

// SyncStatus reports the TCP progress-sync server status.
func (s *APIServer) SyncStatus(c *gin.Context) {
	userID, _ := auth.GetUserIDFromContext(c)
	var connectedUsers []string
	var uptime string
	var count int

	if s.TCPServer != nil {
		connectedUsers = s.TCPServer.GetConnectedUsers()
		uptime = s.TCPServer.GetUptime().String()
		count = len(connectedUsers)
	} else if s.TCPClient != nil {
		if status, err := s.TCPClient.RequestStatus(); err == nil {
			count = status.ConnectedUsers
			connectedUsers = []string{"(View list in TCP monitor)"}
			uptime = "Remote (See Standalone Logs)"
		} else {
			uptime = "Unreachable"
		}
	}

	utils.SuccessResponse(c, "TCP sync server status", gin.H{
		"server":          s.TCPPort,
		"uptime":          uptime,
		"connected_count": count,
		"connected_users": connectedUsers,
		"your_user_id":    userID,
	})
}

// SyncConflicts returns the conflict-resolution log.
func (s *APIServer) SyncConflicts(c *gin.Context) {
	if s.TCPServer == nil {
		utils.SuccessResponse(c, "No TCP server available", gin.H{"conflicts": []interface{}{}, "count": 0})
		return
	}
	conflicts := s.TCPServer.ConflictResolver.GetConflictLog()
	utils.SuccessResponse(c, "Conflict resolution log", gin.H{
		"conflicts": conflicts,
		"count":     len(conflicts),
		"strategy":  s.TCPServer.ConflictResolver.GetStrategy(),
	})
}

// SyncStrategy returns the current conflict-resolution strategy.
func (s *APIServer) SyncStrategy(c *gin.Context) {
	strategy := "last_write_wins"
	if s.TCPServer != nil {
		strategy = s.TCPServer.ConflictResolver.GetStrategy()
	}
	utils.SuccessResponse(c, "Current conflict resolution strategy", gin.H{
		"strategy":             strategy,
		"available_strategies": []string{"last_write_wins", "merge", "user_choice"},
	})
}

// SetSyncStrategy changes the conflict-resolution strategy at runtime.
func (s *APIServer) SetSyncStrategy(c *gin.Context) {
	var req struct {
		Strategy string `json:"strategy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request: strategy is required")
		return
	}

	validStrategies := map[string]bool{"last_write_wins": true, "merge": true, "user_choice": true}
	if !validStrategies[req.Strategy] {
		utils.BadRequestResponse(c, fmt.Sprintf("Invalid strategy '%s'. Valid: last_write_wins, merge, user_choice", req.Strategy))
		return
	}

	if s.TCPServer != nil {
		s.TCPServer.ConflictResolver.SetStrategy(req.Strategy)
	}
	utils.SuccessResponse(c, fmt.Sprintf("Strategy changed to '%s'", req.Strategy), gin.H{"strategy": req.Strategy})
}
