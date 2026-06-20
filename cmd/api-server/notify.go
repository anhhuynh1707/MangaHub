package main

import (
	"fmt"
	"log"
	"time"

	"mangahub/internal/udp"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

type notifyRequest struct {
	Type    string `json:"type"`
	MangaID string `json:"manga_id"`
	Message string `json:"message"`
}

// NotifyBroadcast sends a UDP notification to all connected clients.
func (s *APIServer) NotifyBroadcast(c *gin.Context) {
	var req notifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	notif := udp.Notification{Type: req.Type, MangaID: req.MangaID, Message: req.Message, Timestamp: time.Now().Unix()}

	var sent int
	if s.UDPClient != nil {
		if err := s.UDPClient.SendNotification(notif); err != nil {
			log.Printf("UDP client error: %v", err)
			utils.InternalServerErrorResponse(c, "Failed to send notification")
			return
		}
		sent = 1 // count unknown in client mode
	} else if s.UDPServer != nil {
		sent = s.UDPServer.BroadcastNotification(notif)
	}

	utils.SuccessResponse(c, fmt.Sprintf("Notification sent to %d clients", sent), gin.H{
		"type":       req.Type,
		"sent_count": sent,
		"message":    req.Message,
	})
}

// NotifyBroadcastACK broadcasts with delivery confirmation (waits for ACKs).
func (s *APIServer) NotifyBroadcastACK(c *gin.Context) {
	var req notifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}
	if s.UDPServer == nil {
		utils.BadRequestResponse(c, "UDP server not running in local mode")
		return
	}
	notif := udp.Notification{Type: req.Type, MangaID: req.MangaID, Message: req.Message, Timestamp: time.Now().Unix()}
	utils.SuccessResponse(c, "Broadcast with ACK complete", s.UDPServer.BroadcastWithACK(notif))
}

// NotifyAckStats returns recent delivery records.
func (s *APIServer) NotifyAckStats(c *gin.Context) {
	if s.UDPServer == nil {
		utils.BadRequestResponse(c, "UDP server not running in local mode")
		return
	}
	history := s.UDPServer.GetAckTracker().GetHistory()
	utils.SuccessResponse(c, fmt.Sprintf("%d delivery records", len(history)), history)
}

// NotifyStatus reports the UDP notification server status.
func (s *APIServer) NotifyStatus(c *gin.Context) {
	var clientCount int
	clients := []string{}

	if s.UDPServer != nil {
		clientCount = s.UDPServer.GetClientCount()
		clients = s.UDPServer.GetClients()
	} else if s.UDPClient != nil {
		if status, err := s.UDPClient.RequestStatus(); err == nil {
			clientCount = status.ClientCount
			clients = status.Clients
		} else {
			log.Printf("Failed to request UDP status: %v", err)
		}
	}

	utils.SuccessResponse(c, "UDP notification server status", gin.H{
		"server":       fmt.Sprintf("localhost:%s", s.UDPPort),
		"client_count": clientCount,
		"clients":      clients,
	})
}
