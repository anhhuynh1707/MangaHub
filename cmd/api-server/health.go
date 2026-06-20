package main

import (
	"net"
	"time"

	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ── Service health probes ────────────────────────────────────────────

func (s *APIServer) checkDatabase() gin.H {
	start := time.Now()
	err := s.Database.Ping()
	latency := time.Since(start)
	if err != nil {
		return gin.H{"status": "unhealthy", "error": err.Error(), "latency": latency.String()}
	}
	var mangaCount, userCount int
	s.Database.QueryRow("SELECT COUNT(*) FROM manga").Scan(&mangaCount)
	s.Database.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	return gin.H{
		"status":      "healthy",
		"latency":     latency.String(),
		"manga_count": mangaCount,
		"user_count":  userCount,
		"driver":      "sqlite3",
	}
}

func (s *APIServer) checkRedis() gin.H {
	if !s.Cache.IsAvailable() {
		return gin.H{"status": "disabled"}
	}
	return s.Cache.Stats()
}

func (s *APIServer) checkTCP() gin.H {
	if s.TCPServer == nil {
		return gin.H{"status": "disabled", "mode": "external"}
	}
	connectedUsers := s.TCPServer.GetConnectedUsers()
	return gin.H{
		"status":          "healthy",
		"mode":            "internal",
		"port":            s.TCPPort,
		"uptime":          s.TCPServer.GetUptime().String(),
		"connected_users": len(connectedUsers),
		"users":           connectedUsers,
		"strategy":        s.TCPServer.ConflictResolver.GetStrategy(),
	}
}

func (s *APIServer) checkUDP() gin.H {
	if s.UDPServer == nil {
		return gin.H{"status": "disabled", "mode": "external"}
	}
	return gin.H{
		"status":       "healthy",
		"mode":         "internal",
		"port":         s.UDPPort,
		"client_count": s.UDPServer.GetClientCount(),
		"clients":      s.UDPServer.GetClients(),
	}
}

func (s *APIServer) checkWebSocket() gin.H {
	return gin.H{
		"status":          "healthy",
		"general_clients": s.Hub.GetClientCount("general"),
		"online_users":    s.Hub.GetOnlineUsers("general"),
	}
}

func (s *APIServer) checkGRPC() gin.H {
	if !s.EnableGRPC {
		return gin.H{"status": "disabled", "mode": "external"}
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "localhost:"+s.GRPCPort, 2*time.Second)
	latency := time.Since(start)
	if err != nil {
		return gin.H{"status": "unhealthy", "port": s.GRPCPort, "error": err.Error(), "latency": latency.String()}
	}
	conn.Close()
	return gin.H{"status": "healthy", "mode": "internal", "port": s.GRPCPort, "latency": latency.String()}
}

// ── Health endpoints ─────────────────────────────────────────────────

// Health is a comprehensive health check across all services.
func (s *APIServer) Health(c *gin.Context) {
	dbHealth := s.checkDatabase()
	overallStatus := "healthy"
	if dbHealth["status"] == "unhealthy" {
		overallStatus = "degraded"
	}
	count, _ := s.MangaService.GetCount()
	utils.SuccessResponse(c, "MangaHub API is running", gin.H{
		"status":      overallStatus,
		"manga_count": count,
		"services": gin.H{
			"api":       gin.H{"status": "healthy", "port": s.Port},
			"database":  dbHealth,
			"cache":     s.checkRedis(),
			"tcp":       s.checkTCP(),
			"udp":       s.checkUDP(),
			"websocket": s.checkWebSocket(),
			"grpc":      s.checkGRPC(),
		},
	})
}

func (s *APIServer) HealthDB(c *gin.Context)    { utils.SuccessResponse(c, "Database health", s.checkDatabase()) }
func (s *APIServer) HealthCache(c *gin.Context) { utils.SuccessResponse(c, "Cache health", s.checkRedis()) }
func (s *APIServer) HealthTCP(c *gin.Context)   { utils.SuccessResponse(c, "TCP server health", s.checkTCP()) }
func (s *APIServer) HealthUDP(c *gin.Context)   { utils.SuccessResponse(c, "UDP server health", s.checkUDP()) }
func (s *APIServer) HealthWS(c *gin.Context)    { utils.SuccessResponse(c, "WebSocket hub health", s.checkWebSocket()) }
func (s *APIServer) HealthGRPC(c *gin.Context)  { utils.SuccessResponse(c, "gRPC server health", s.checkGRPC()) }

// ── Cache management ─────────────────────────────────────────────────

func (s *APIServer) CacheStats(c *gin.Context) {
	utils.SuccessResponse(c, "Cache statistics", s.Cache.Stats())
}

func (s *APIServer) CacheFlush(c *gin.Context) {
	if err := s.Cache.Flush(); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to flush cache: "+err.Error())
		return
	}
	utils.SuccessResponse(c, "Cache flushed successfully", nil)
}
