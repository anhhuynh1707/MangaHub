package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"mangahub/data"
	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/internal/friend"
	grpcServer "mangahub/internal/grpc"
	mangaPkg "mangahub/internal/manga"
	"mangahub/internal/recommendation"
	"mangahub/internal/review"
	"mangahub/internal/sharedlist"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	userPkg "mangahub/internal/user"
	wsPkg "mangahub/internal/websocket"
	"mangahub/pkg/cache"
	"mangahub/pkg/database"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "mangahub/docs"
)

// APIServer is the core server structure per spec requirements.
type APIServer struct {
	Router          *gin.Engine
	Database        *sql.DB
	JWTSecret       string
	TCPServer       *tcp.ProgressSyncServer
	TCPClient       *tcp.ProgressSyncClient
	UDPServer       *udp.NotificationServer
	UDPClient       *udp.NotificationClient
	GRPCMangaServer *grpcServer.MangaServer // kept for EventHub access
	UseClients      bool                    // true if using remote services
}

func main() {
	// --- Load .env file ---
	loadEnvFile(".env")

	// --- Configuration ---
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret != "" {
		auth.JWTSecret = []byte(jwtSecret)
	}
	ginMode := os.Getenv("GIN_MODE")
	if ginMode != "" {
		gin.SetMode(ginMode)
	}

	// --- Database (raw database/sql + mattn/go-sqlite3) ---
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// --- Redis Cache ---
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		redisDB, _ = strconv.Atoi(v)
	}
	redisCache := cache.New(redisAddr, redisPassword, redisDB)
	defer redisCache.Close()

	// --- Repositories ---
	userRepo := userPkg.NewRepository(db)
	mangaRepo := mangaPkg.NewRepository(db)

	// --- Services ---
	userService := userPkg.NewService(userRepo)
	mangaService := mangaPkg.NewService(mangaRepo)

	// --- Social Feature Services ---
	reviewRepo := review.NewRepository(db)
	reviewService := review.NewService(reviewRepo)

	friendRepo := friend.NewRepository(db)
	friendService := friend.NewService(friendRepo)

	sharedListRepo := sharedlist.NewRepository(db)
	sharedListService := sharedlist.NewService(sharedListRepo)

	activityRepo := activity.NewRepository(db)
	activityService := activity.NewService(activityRepo)

	// --- Inject Redis cache into services ---
	mangaService.SetCache(redisCache)
	userService.SetCache(redisCache)
	activityService.SetCache(redisCache)

	// --- MangaDex Client ---
	mangaDexClient := mangaPkg.NewMangaDexClient()

	// --- Seed data on first run ---
	seedDatabase(mangaService, mangaDexClient)

	// --- Handlers ---
	userHandler := userPkg.NewHandler(userService)
	mangaHandler := mangaPkg.NewHandler(mangaService)

	// --- Social Feature Handlers ---
	reviewHandler := review.NewHandler(reviewService, activityService, mangaService)
	friendHandler := friend.NewHandler(friendService, activityService)
	sharedListHandler := sharedlist.NewHandler(sharedListService, activityService)
	activityHandler := activity.NewHandler(activityService)

	// --- Service Configuration ---
	enableTCPServer := os.Getenv("ENABLE_TCP_SERVER")
	if enableTCPServer == "" {
		enableTCPServer = "true"
	}
	enableUDPServer := os.Getenv("ENABLE_UDP_SERVER")
	if enableUDPServer == "" {
		enableUDPServer = "true"
	}
	enableGRPCServer := os.Getenv("ENABLE_GRPC_SERVER")
	if enableGRPCServer == "" {
		enableGRPCServer = "true"
	}

	// --- Build APIServer (after config flags are set) ---
	server := &APIServer{
		Router:     gin.Default(),
		Database:   db,
		JWTSecret:  string(auth.JWTSecret),
		UseClients: enableUDPServer == "false" || enableTCPServer == "false",
	}

	// --- TCP Progress Sync Server (runs in goroutine) ---
	tcpPort := os.Getenv("TCP_PORT")
	if tcpPort == "" {
		tcpPort = "9090"
	}

	if enableTCPServer == "true" {
		tcpServer := tcp.NewProgressSyncServer(tcpPort)
		tcpServer.Persister = userService // Save TCP progress updates to DB
		server.TCPServer = tcpServer

		log.Println("Starting internal TCP server...")
		go func() {
			if err := tcpServer.Start(); err != nil {
				log.Printf("TCP server error: %v", err)
			}
		}()
	} else {
		log.Printf("TCP server disabled - using remote service at %s", tcpPort)
		tcpClient, err := tcp.NewProgressSyncClient(tcpPort)
		if err != nil {
			log.Printf("Warning: Failed to connect to remote TCP server: %v", err)
		} else {
			server.TCPClient = tcpClient
		}
	}

	// --- UDP Notification Server (runs in goroutine) ---
	udpPort := os.Getenv("UDP_PORT")
	if udpPort == "" {
		udpPort = "9091"
	}

	if enableUDPServer == "true" {
		udpServer := udp.NewNotificationServer(udpPort)
		server.UDPServer = udpServer

		log.Println("Starting internal UDP server...")
		go func() {
			if err := udpServer.Start(); err != nil {
				log.Printf("UDP server error: %v", err)
			}
		}()
	} else {
		log.Printf("UDP server disabled - using remote service at %s", udpPort)
		udpClient, err := udp.NewNotificationClient(udpPort)
		if err != nil {
			log.Printf("Warning: Failed to connect to remote UDP server: %v", err)
		} else {
			server.UDPClient = udpClient
		}
	}

	// --- WebSocket Chat Hub (runs in goroutine) ---
	chatHub := wsPkg.NewChatHub()
	go chatHub.Run()

	// --- gRPC Internal Service (runs in goroutine) ---
	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "9092"
	}

	if enableGRPCServer == "true" {
		log.Println("Starting internal gRPC server...")
		gms := grpcServer.NewMangaServer(mangaService, userService)
		server.GRPCMangaServer = gms
		go func() {
			if err := grpcServer.ServeGRPC(grpcPort, gms); err != nil {
				log.Printf("gRPC server error: %v", err)
			}
		}()
	} else {
		log.Println("gRPC server disabled (using external service)")
	}

	// --- Routes ---
	r := server.Router

	// ============================================================
	// HEALTH CHECK ENDPOINTS — All Services
	// ============================================================

	// Helper: check database health
	checkDatabase := func() gin.H {
		start := time.Now()
		err := db.Ping()
		latency := time.Since(start)
		if err != nil {
			return gin.H{
				"status":  "unhealthy",
				"error":   err.Error(),
				"latency": latency.String(),
			}
		}
		// Get table counts
		var mangaCount, userCount int
		db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&mangaCount)
		db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
		return gin.H{
			"status":      "healthy",
			"latency":     latency.String(),
			"manga_count": mangaCount,
			"user_count":  userCount,
			"driver":      "sqlite3",
		}
	}

	// Helper: check Redis health
	checkRedis := func() gin.H {
		if !redisCache.IsAvailable() {
			return gin.H{"status": "disabled"}
		}
		return redisCache.Stats()
	}

	// Helper: check TCP server health
	checkTCP := func() gin.H {
		if server.TCPServer == nil {
			return gin.H{"status": "disabled", "mode": "external"}
		}
		connectedUsers := server.TCPServer.GetConnectedUsers()
		return gin.H{
			"status":          "healthy",
			"mode":            "internal",
			"port":            tcpPort,
			"uptime":          server.TCPServer.GetUptime().String(),
			"connected_users": len(connectedUsers),
			"users":           connectedUsers,
			"strategy":        server.TCPServer.ConflictResolver.GetStrategy(),
		}
	}

	// Helper: check UDP server health
	checkUDP := func() gin.H {
		if server.UDPServer == nil {
			return gin.H{"status": "disabled", "mode": "external"}
		}
		return gin.H{
			"status":       "healthy",
			"mode":         "internal",
			"port":         udpPort,
			"client_count": server.UDPServer.GetClientCount(),
			"clients":      server.UDPServer.GetClients(),
		}
	}

	// Helper: check WebSocket hub health
	checkWebSocket := func() gin.H {
		generalClients := chatHub.GetClientCount("general")
		onlineUsers := chatHub.GetOnlineUsers("general")
		return gin.H{
			"status":          "healthy",
			"general_clients": generalClients,
			"online_users":    onlineUsers,
		}
	}

	// Helper: check gRPC server health
	checkGRPC := func() gin.H {
		if enableGRPCServer != "true" {
			return gin.H{"status": "disabled", "mode": "external"}
		}

		// Attempt a TCP dial to the gRPC port as a lightweight liveness probe
		start := time.Now()
		conn, err := net.DialTimeout("tcp", "localhost:"+grpcPort, 2*time.Second)
		latency := time.Since(start)
		if err != nil {
			return gin.H{
				"status":  "unhealthy",
				"port":    grpcPort,
				"error":   err.Error(),
				"latency": latency.String(),
			}
		}
		conn.Close()
		return gin.H{
			"status":  "healthy",
			"mode":    "internal",
			"port":    grpcPort,
			"latency": latency.String(),
		}
	}

	// GET /health — Comprehensive health check for all services
	r.GET("/health", func(c *gin.Context) {
		dbHealth := checkDatabase()
		redisHealth := checkRedis()
		tcpHealth := checkTCP()
		udpHealth := checkUDP()
		wsHealth := checkWebSocket()
		grpcHealth := checkGRPC()

		// Determine overall status
		overallStatus := "healthy"
		if dbHealth["status"] == "unhealthy" {
			overallStatus = "degraded"
		}

		count, _ := mangaService.GetCount()

		utils.SuccessResponse(c, "MangaHub API is running", gin.H{
			"status":      overallStatus,
			"manga_count": count,
			"services": gin.H{
				"api":       gin.H{"status": "healthy", "port": port},
				"database":  dbHealth,
				"cache":     redisHealth,
				"tcp":       tcpHealth,
				"udp":       udpHealth,
				"websocket": wsHealth,
				"grpc":      grpcHealth,
			},
		})
	})

	// GET /health/db — Database health only
	r.GET("/health/db", func(c *gin.Context) {
		utils.SuccessResponse(c, "Database health", checkDatabase())
	})

	// GET /health/cache — Redis cache health only
	r.GET("/health/cache", func(c *gin.Context) {
		utils.SuccessResponse(c, "Cache health", checkRedis())
	})

	// GET /health/tcp — TCP Progress Sync Server health only
	r.GET("/health/tcp", func(c *gin.Context) {
		utils.SuccessResponse(c, "TCP server health", checkTCP())
	})

	// GET /health/udp — UDP Notification Server health only
	r.GET("/health/udp", func(c *gin.Context) {
		utils.SuccessResponse(c, "UDP server health", checkUDP())
	})

	// GET /health/ws — WebSocket Chat Hub health only
	r.GET("/health/ws", func(c *gin.Context) {
		utils.SuccessResponse(c, "WebSocket hub health", checkWebSocket())
	})

	// GET /health/grpc — gRPC server health only
	r.GET("/health/grpc", func(c *gin.Context) {
		utils.SuccessResponse(c, "gRPC server health", checkGRPC())
	})

	// Cache management endpoints (authenticated)
	cacheRoutes := r.Group("/cache")
	cacheRoutes.Use(auth.AuthMiddleware())
	{
		// GET /cache/stats — View Redis cache statistics
		cacheRoutes.GET("/stats", func(c *gin.Context) {
			utils.SuccessResponse(c, "Cache statistics", redisCache.Stats())
		})

		// DELETE /cache/flush — Flush all cached data
		cacheRoutes.DELETE("/flush", func(c *gin.Context) {
			if err := redisCache.Flush(); err != nil {
				utils.InternalServerErrorResponse(c, "Failed to flush cache: "+err.Error())
				return
			}
			utils.SuccessResponse(c, "Cache flushed successfully", nil)
		})
	}

	// Auth routes (public)
	r.POST("/auth/register", userHandler.Register)
	r.POST("/auth/login", userHandler.Login)

	// Auth routes (authenticated)
	authRoutes := r.Group("/auth")
	authRoutes.Use(auth.AuthMiddleware())
	{
		authRoutes.GET("/status", userHandler.AuthStatus)
		authRoutes.POST("/logout", userHandler.Logout)
		authRoutes.PUT("/change-password", userHandler.ChangePassword)
	}

	// Manga routes (public read)
	r.GET("/manga", mangaHandler.Search)
	r.POST("/manga/search", mangaHandler.AdvancedSearch) // advanced search with SearchFilters body
	r.GET("/manga/:id", mangaHandler.GetByID)

	// Manga routes (authenticated write)
	mangaAuth := r.Group("/manga")
	mangaAuth.Use(auth.AuthMiddleware())
	{
		mangaAuth.POST("", mangaHandler.Create)
		mangaAuth.PUT("/:id", mangaHandler.Update)
		mangaAuth.DELETE("/:id", mangaHandler.Delete)
	}

	// Recommendation service — shared across routes
	recService := recommendation.NewService(db)

	// User routes (authenticated)
	users := r.Group("/users")
	users.Use(auth.AuthMiddleware())
	{
		users.GET("/profile", userHandler.GetProfile)
		users.POST("/library", userHandler.AddToLibrary)
		users.GET("/library", userHandler.GetLibrary)
		users.DELETE("/library/:manga_id", userHandler.RemoveFromLibrary)

		// GET /users/recommendations — collaborative filtering based on reading history
		users.GET("/recommendations", func(c *gin.Context) {
			userID, err := auth.GetUserIDFromContext(c)
			if err != nil {
				utils.UnauthorizedResponse(c, "Unauthorized")
				return
			}
			limitStr := c.DefaultQuery("limit", "10")
			limit := 10
			fmt.Sscanf(limitStr, "%d", &limit)

			result, err := recService.GetRecommendations(userID, limit)
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to generate recommendations: "+err.Error())
				return
			}
			utils.SuccessResponse(c, "Recommendations generated", result)
		})
		users.PUT("/progress", func(c *gin.Context) {
			// Call the original handler
			userHandler.UpdateProgress(c)

			// If successful, broadcast to TCP clients
			if c.Writer.Status() == 200 {
				userID, _ := auth.GetUserIDFromContext(c)
				var req models.UpdateProgressRequest
				// Re-parse isn't needed; extract from the response context
				mangaID := c.GetString("progress_manga_id")
				chapter := c.GetInt("progress_chapter")
				if mangaID == "" {
					// Fallback: try to read from request body (already consumed)
					_ = req
				}
				if mangaID != "" && chapter > 0 {
					update := models.ProgressUpdate{
						UserID:    userID,
						MangaID:   mangaID,
						Chapter:   chapter,
						Timestamp: time.Now().Unix(),
					}

					// Broadcast via TCP
					if server.TCPClient != nil {
						if err := server.TCPClient.SendProgressUpdate(update); err != nil {
							log.Printf("TCP client error: %v", err)
						}
					} else if server.TCPServer != nil {
						server.TCPServer.SendProgressUpdate(update)
					}

					// Publish to gRPC event hubâ WatchMangaUpdates streams
					if server.GRPCMangaServer != nil {
						server.GRPCMangaServer.EventHub.PublishProgressUpdate(
							userID, mangaID, int32(chapter),
						)
					}
				}
			}
		})
	}

	// Sync status endpoint (authenticated) — used by CLI `mangahub sync status`
	syncRoutes := r.Group("/sync")
	syncRoutes.Use(auth.AuthMiddleware())
	{
		syncRoutes.GET("/status", func(c *gin.Context) {
			userID, _ := auth.GetUserIDFromContext(c)
			var connectedUsers []string
			var uptime string
			var count int

			// Try local server first
			if server.TCPServer != nil {
				connectedUsers = server.TCPServer.GetConnectedUsers()
				uptime = server.TCPServer.GetUptime().String()
				count = len(connectedUsers)
			} else if server.TCPClient != nil {
				// Standalone mode: Ask the remote server for status via TCP protocol
				status, err := server.TCPClient.RequestStatus()
				if err == nil {
					count = status.ConnectedUsers
					// The protocol status message often includes user list in Message field
					// For now, we'll just report the count from the status message
					connectedUsers = []string{"(View list in TCP monitor)"}
					uptime = "Remote (See Standalone Logs)"
				} else {
					uptime = "Unreachable"
				}
			}

			utils.SuccessResponse(c, "TCP sync server status", gin.H{
				"server":          tcpPort,
				"uptime":          uptime,
				"connected_count": count,
				"connected_users": connectedUsers,
				"your_user_id":    userID,
			})
		})

		// GET /sync/conflicts — View conflict resolution log
		syncRoutes.GET("/conflicts", func(c *gin.Context) {
			if server.TCPServer == nil {
				utils.SuccessResponse(c, "No TCP server available", gin.H{
					"conflicts": []interface{}{},
					"count":     0,
				})
				return
			}

			conflicts := server.TCPServer.ConflictResolver.GetConflictLog()
			strategy := server.TCPServer.ConflictResolver.GetStrategy()

			utils.SuccessResponse(c, "Conflict resolution log", gin.H{
				"conflicts": conflicts,
				"count":     len(conflicts),
				"strategy":  strategy,
			})
		})

		// GET /sync/strategy — View current strategy
		syncRoutes.GET("/strategy", func(c *gin.Context) {
			strategy := "last_write_wins"
			if server.TCPServer != nil {
				strategy = server.TCPServer.ConflictResolver.GetStrategy()
			}

			utils.SuccessResponse(c, "Current conflict resolution strategy", gin.H{
				"strategy":             strategy,
				"available_strategies": []string{"last_write_wins", "merge", "user_choice"},
			})
		})

		// PUT /sync/strategy — Change strategy at runtime
		syncRoutes.PUT("/strategy", func(c *gin.Context) {
			var req struct {
				Strategy string `json:"strategy" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				utils.BadRequestResponse(c, "Invalid request: strategy is required")
				return
			}

			validStrategies := map[string]bool{
				"last_write_wins": true,
				"merge":           true,
				"user_choice":     true,
			}
			if !validStrategies[req.Strategy] {
				utils.BadRequestResponse(c, fmt.Sprintf("Invalid strategy '%s'. Valid: last_write_wins, merge, user_choice", req.Strategy))
				return
			}

			if server.TCPServer != nil {
				server.TCPServer.ConflictResolver.SetStrategy(req.Strategy)
			}

			utils.SuccessResponse(c, fmt.Sprintf("Strategy changed to '%s'", req.Strategy), gin.H{
				"strategy": req.Strategy,
			})
		})
	}

	// Notification endpoints (authenticated) — used by CLI `mangahub notify send`
	notifyRoutes := r.Group("/notify")
	notifyRoutes.Use(auth.AuthMiddleware())
	{
		notifyRoutes.POST("/broadcast", func(c *gin.Context) {
			var req struct {
				Type    string `json:"type"`
				MangaID string `json:"manga_id"`
				Message string `json:"message"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				utils.BadRequestResponse(c, "Invalid request body")
				return
			}

			notif := udp.Notification{
				Type:      req.Type,
				MangaID:   req.MangaID,
				Message:   req.Message,
				Timestamp: time.Now().Unix(),
			}

			var sent int
			if server.UDPClient != nil {
				// Remote mode: send to remote server
				if err := server.UDPClient.SendNotification(notif); err != nil {
					log.Printf("UDP client error: %v", err)
					utils.InternalServerErrorResponse(c, "Failed to send notification")
					return
				}
				sent = 1 // We can't know actual count in client mode
			} else if server.UDPServer != nil {
				// Local mode: broadcast locally
				sent = server.UDPServer.BroadcastNotification(notif)
			}

			utils.SuccessResponse(c, fmt.Sprintf("Notification sent to %d clients", sent), gin.H{
				"type":       req.Type,
				"sent_count": sent,
				"message":    req.Message,
			})
		})

		// POST /notify/broadcast-ack — send with delivery confirmation (waits 3s for ACKs)
		notifyRoutes.POST("/broadcast-ack", func(c *gin.Context) {
			var req struct {
				Type    string `json:"type"`
				MangaID string `json:"manga_id"`
				Message string `json:"message"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				utils.BadRequestResponse(c, "Invalid request body")
				return
			}
			if server.UDPServer == nil {
				utils.BadRequestResponse(c, "UDP server not running in local mode")
				return
			}
			notif := udp.Notification{
				Type:      req.Type,
				MangaID:   req.MangaID,
				Message:   req.Message,
				Timestamp: time.Now().Unix(),
			}
			record := server.UDPServer.BroadcastWithACK(notif)
			utils.SuccessResponse(c, "Broadcast with ACK complete", record)
		})

		// GET /notify/ack-stats — view recent delivery records
		notifyRoutes.GET("/ack-stats", func(c *gin.Context) {
			if server.UDPServer == nil {
				utils.BadRequestResponse(c, "UDP server not running in local mode")
				return
			}
			history := server.UDPServer.GetAckTracker().GetHistory()
			utils.SuccessResponse(c, fmt.Sprintf("%d delivery records", len(history)), history)
		})

		notifyRoutes.GET("/status", func(c *gin.Context) {
			var clientCount int
			var clients []string

			if server.UDPServer != nil {
				clientCount = server.UDPServer.GetClientCount()
				clients = server.UDPServer.GetClients()
			} else if server.UDPClient != nil {
				status, err := server.UDPClient.RequestStatus()
				if err == nil {
					clientCount = status.ClientCount
					clients = status.Clients
				} else {
					log.Printf("Failed to request UDP status: %v", err)
					clientCount = 0
					clients = []string{}
				}
			} else {
				clientCount = 0
				clients = []string{}
			}

			utils.SuccessResponse(c, "UDP notification server status", gin.H{
				"server":       fmt.Sprintf("localhost:%s", udpPort),
				"client_count": clientCount,
				"clients":      clients,
			})
		})
	}

	// WebSocket chat endpoint (auth via query param)
	r.GET("/ws/chat", func(c *gin.Context) {
		wsPkg.HandleWebSocket(chatHub, c.Writer, c.Request)
	})

	// Chat history endpoint (authenticated)
	r.GET("/chat/history", auth.AuthMiddleware(), func(c *gin.Context) {
		limitStr := c.DefaultQuery("limit", "20")
		limit, _ := strconv.Atoi(limitStr)
		room := c.DefaultQuery("room", "general")
		history := chatHub.GetHistory(room, limit)
		utils.SuccessResponse(c, fmt.Sprintf("%d messages", len(history)), history)
	})

	// Data collection endpoints (authenticated)
	dataRoutes := r.Group("/data")
	dataRoutes.Use(auth.AuthMiddleware())
	{
		dataRoutes.POST("/seed", func(c *gin.Context) {
			seedManga := data.GetSeedManga()
			inserted, _ := mangaService.BulkCreate(seedManga)
			utils.SuccessResponse(c, fmt.Sprintf("Seeded %d manga", inserted), gin.H{
				"imported": inserted, "total": len(seedManga),
			})
		})

		dataRoutes.POST("/fetch-mangadex", func(c *gin.Context) {
			imported, err := mangaPkg.ImportFromMangaDex(mangaService, mangaDexClient, 100)
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to fetch from MangaDex: "+err.Error())
				return
			}
			utils.SuccessResponse(c, fmt.Sprintf("Imported %d manga from MangaDex", imported), gin.H{
				"imported": imported,
			})
		})

		dataRoutes.GET("/export-json", func(c *gin.Context) {
			allManga, err := mangaService.GetAll()
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to export manga")
				return
			}
			utils.SuccessResponse(c, "Manga exported as JSON", allManga)
		})

		// --- Web Scraping (Educational Practice) ---

		// POST /data/scrape-quotes — Scrape quotes from quotes.toscrape.com
		dataRoutes.POST("/scrape-quotes", func(c *gin.Context) {
			pagesStr := c.DefaultQuery("pages", "3")
			pages, _ := strconv.Atoi(pagesStr)

			scraper := data.NewScraper()
			quotes, err := scraper.ScrapeQuotes(pages)
			if err != nil {
				utils.InternalServerErrorResponse(c, "Scraping failed: "+err.Error())
				return
			}

			// Save to JSON file
			if err := data.ExportQuotesToJSON(quotes, "./data/scraped_quotes.json"); err != nil {
				log.Printf("Warning: failed to save quotes to JSON: %v", err)
			}

			utils.SuccessResponse(c, fmt.Sprintf("Scraped %d quotes from %d pages", len(quotes), pages), gin.H{
				"quotes": quotes,
				"count":  len(quotes),
				"pages":  pages,
			})
		})

		// GET /data/scraped-quotes — View previously scraped quotes
		dataRoutes.GET("/scraped-quotes", func(c *gin.Context) {
			quotes, err := data.ImportQuotesFromJSON("./data/scraped_quotes.json")
			if err != nil {
				utils.NotFoundResponse(c, "No scraped quotes found. Run POST /data/scrape-quotes first.")
				return
			}
			utils.SuccessResponse(c, fmt.Sprintf("Found %d scraped quotes", len(quotes)), quotes)
		})

		// POST /data/test-httpbin — Test HTTP methods against httpbin.org
		dataRoutes.POST("/test-httpbin", func(c *gin.Context) {
			scraper := data.NewScraper()
			results, err := scraper.TestHTTPBin()
			if err != nil {
				utils.InternalServerErrorResponse(c, "HTTPBin test failed: "+err.Error())
				return
			}
			utils.SuccessResponse(c, "HTTPBin tests completed", results)
		})

		// --- JSON File Storage ---

		// POST /data/export-files — Export manga DB + user data to JSON files
		dataRoutes.POST("/export-files", func(c *gin.Context) {
			// Export all manga to JSON
			allManga, err := mangaService.GetAll()
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to get manga for export")
				return
			}
			if err := data.ExportMangaToJSON(allManga, "./data/manga.json"); err != nil {
				utils.InternalServerErrorResponse(c, "Failed to export manga JSON: "+err.Error())
				return
			}

			utils.SuccessResponse(c, "Data exported to JSON files", gin.H{
				"manga_file":  "./data/manga.json",
				"manga_count": len(allManga),
			})
		})

		// POST /data/import-json — Import manga from a JSON file
		dataRoutes.POST("/import-json", func(c *gin.Context) {
			mangaList, err := data.ImportMangaFromJSON("./data/manga.json")
			if err != nil {
				utils.NotFoundResponse(c, "No manga.json found. Run POST /data/export-files first.")
				return
			}
			inserted, _ := mangaService.BulkCreate(mangaList)
			utils.SuccessResponse(c, fmt.Sprintf("Imported %d manga from JSON", inserted), gin.H{
				"imported": inserted,
				"total":    len(mangaList),
			})
		})

		// ============================================================
		// DATA EXPORT ENDPOINTS — File downloads (JSON / CSV)
		// ============================================================

		// GET /data/export/library?format=json|csv — Download user library
		dataRoutes.GET("/export/library", func(c *gin.Context) {
			userID, _ := auth.GetUserIDFromContext(c)
			format := c.DefaultQuery("format", "json")

			userData, err := userService.GetLibrary(userID)
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to get library")
				return
			}

			// Flatten entries
			var entries []models.UserProgress
			entries = append(entries, userData.ReadingLists.Reading...)
			entries = append(entries, userData.ReadingLists.Completed...)
			entries = append(entries, userData.ReadingLists.PlanToRead...)

			switch format {
			case "csv":
				// Save to file using csv_storage
				filePath := "./data/library.csv"
				if err := data.ExportProgressToCSV(entries, filePath); err != nil {
					log.Printf("Warning: failed to save library CSV to disk: %v", err)
				}
				c.Header("Content-Disposition", "attachment; filename=library.csv")
				c.Header("Content-Type", "text/csv")
				c.Writer.WriteString("manga_id,current_chapter,status,updated_at\n")
				for _, e := range entries {
					c.Writer.WriteString(fmt.Sprintf("%s,%d,%s,%s\n",
						e.MangaID, e.CurrentChapter, e.Status, e.UpdatedAt.Format(time.RFC3339)))
				}
			default:
				c.Header("Content-Disposition", "attachment; filename=library.json")
				c.JSON(200, gin.H{
					"user_id":     userID,
					"username":    userData.Username,
					"exported_at": time.Now().Format(time.RFC3339),
					"total":       len(entries),
					"entries":     entries,
				})
			}
		})

		// GET /data/export/progress?format=json|csv — Download reading progress
		dataRoutes.GET("/export/progress", func(c *gin.Context) {
			userID, _ := auth.GetUserIDFromContext(c)
			format := c.DefaultQuery("format", "csv")

			userData, err := userService.GetLibrary(userID)
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to get progress")
				return
			}

			var entries []models.UserProgress
			entries = append(entries, userData.ReadingLists.Reading...)
			entries = append(entries, userData.ReadingLists.Completed...)
			entries = append(entries, userData.ReadingLists.PlanToRead...)

			switch format {
			case "csv":
				// Save to file using csv_storage
				filePath := "./data/progress.csv"
				if err := data.ExportProgressToCSV(entries, filePath); err != nil {
					log.Printf("Warning: failed to save progress CSV to disk: %v", err)
				}
				c.Header("Content-Disposition", "attachment; filename=progress.csv")
				c.Header("Content-Type", "text/csv")
				c.Writer.WriteString("manga_id,current_chapter,status,updated_at\n")
				for _, e := range entries {
					c.Writer.WriteString(fmt.Sprintf("%s,%d,%s,%s\n",
						e.MangaID, e.CurrentChapter, e.Status, e.UpdatedAt.Format(time.RFC3339)))
				}
			default:
				c.Header("Content-Disposition", "attachment; filename=progress.json")
				c.JSON(200, gin.H{
					"exported_at": time.Now().Format(time.RFC3339),
					"total":       len(entries),
					"progress":    entries,
				})
			}
		})

		// GET /data/export/manga?format=json|csv — Download manga database
		dataRoutes.GET("/export/manga", func(c *gin.Context) {
			format := c.DefaultQuery("format", "json")
			allManga, err := mangaService.GetAll()
			if err != nil {
				utils.InternalServerErrorResponse(c, "Failed to get manga")
				return
			}

			switch format {
			case "csv":
				// Save to file using csv_storage
				filePath := "./data/manga.csv"
				if err := data.ExportMangaToCSV(allManga, filePath); err != nil {
					log.Printf("Warning: failed to save manga CSV to disk: %v", err)
				}
				c.Header("Content-Disposition", "attachment; filename=manga.csv")
				c.Header("Content-Type", "text/csv")
				c.Writer.WriteString("id,title,author,genres,status,total_chapters,description\n")
				for _, m := range allManga {
					title := fmt.Sprintf("%q", m.Title)
					author := fmt.Sprintf("%q", m.Author)
					c.Writer.WriteString(fmt.Sprintf("%s,%s,%s,%s,%d\n",
						m.ID, title, author, m.Status, m.TotalChapters))
				}
			default:
				// Save to file using json_storage
				if err := data.ExportMangaToJSON(allManga, "./data/manga_export.json"); err != nil {
					log.Printf("Warning: failed to save manga JSON to disk: %v", err)
				}
				c.Header("Content-Disposition", "attachment; filename=manga.json")
				c.JSON(200, allManga)
			}
		})

		// GET /data/export/full — Download complete user data as JSON
		dataRoutes.GET("/export/full", func(c *gin.Context) {
			userID, _ := auth.GetUserIDFromContext(c)

			library, _ := userService.GetLibrary(userID)

			c.Header("Content-Disposition", "attachment; filename=mangahub-export.json")
			c.JSON(200, gin.H{
				"exported_at": time.Now().Format(time.RFC3339),
				"version":     "1.0",
				"user_id":     userID,
				"library":     library,
			})
		})
	}

	// ============================================================
	// REVIEW ROUTES
	// ============================================================
	mangaRoutes := r.Group("/manga")
	{
		// Public review endpoints
		mangaRoutes.GET("/:id/reviews", reviewHandler.GetReviews)
		mangaRoutes.GET("/:id/rating-stats", reviewHandler.GetRatingStats)

		// Authenticated review creation
		authMangaRoutes := mangaRoutes.Group("/:id/reviews")
		authMangaRoutes.Use(auth.AuthMiddleware())
		{
			authMangaRoutes.POST("", reviewHandler.CreateReview)
		}
	}

	reviewRoutes := r.Group("/reviews")
	reviewRoutes.Use(auth.AuthMiddleware())
	{
		reviewRoutes.GET("/:review_id", reviewHandler.GetReview)
		reviewRoutes.PUT("/:review_id", reviewHandler.UpdateReview)
		reviewRoutes.DELETE("/:review_id", reviewHandler.DeleteReview)
		reviewRoutes.POST("/:review_id/helpful", reviewHandler.MarkHelpful)
	}

	userReviewRoutes := r.Group("/users/reviews")
	userReviewRoutes.Use(auth.AuthMiddleware())
	{
		userReviewRoutes.GET("", reviewHandler.GetMyReviews)
	}

	// ============================================================
	// FRIEND ROUTES
	// ============================================================
	friendRoutes := r.Group("/friends")
	friendRoutes.Use(auth.AuthMiddleware())
	{
		friendRoutes.POST("/add", friendHandler.AddFriend)
		friendRoutes.POST("/:friend_id/accept", friendHandler.AcceptFriend)
		friendRoutes.POST("/:friend_id/decline", friendHandler.DeclineFriend)
		friendRoutes.DELETE("/:friend_id", friendHandler.RemoveFriend)
		friendRoutes.POST("/:friend_id/block", friendHandler.BlockFriend)
		friendRoutes.GET("/:friend_id/info", friendHandler.GetFriendInfo)
		friendRoutes.POST("/:friend_id/check", friendHandler.CheckFriendship)
	}

	userFriendRoutes := r.Group("/users")
	userFriendRoutes.Use(auth.AuthMiddleware())
	{
		userFriendRoutes.GET("/friends", friendHandler.GetFriends)
		userFriendRoutes.GET("/friends/pending", friendHandler.GetPendingRequests)
		userFriendRoutes.GET("/friends/count", friendHandler.GetFriendCount)
	}

	// ============================================================
	// SHARED READING LIST ROUTES
	// ============================================================
	readingListGroup := r.Group("/reading-lists")
	{
		// Public routes (No Auth)
		readingListGroup.GET("/public", sharedListHandler.GetPublicLists)
		readingListGroup.GET("/:list_id", sharedListHandler.GetList)

		// Authenticated routes
		authListGroup := readingListGroup.Group("")
		authListGroup.Use(auth.AuthMiddleware())
		{
			authListGroup.POST("/create", sharedListHandler.CreateList)
			authListGroup.GET("/mine", sharedListHandler.GetMyLists)
			authListGroup.GET("/subscribed", sharedListHandler.GetSubscribedLists)
			authListGroup.PUT("/:list_id", sharedListHandler.UpdateList)
			authListGroup.DELETE("/:list_id", sharedListHandler.DeleteList)
			authListGroup.POST("/:list_id/subscribe", sharedListHandler.SubscribeToList)
			authListGroup.DELETE("/:list_id/subscribe", sharedListHandler.UnsubscribeFromList)
			authListGroup.POST("/:list_id/manga", sharedListHandler.AddMangaToList)
			authListGroup.DELETE("/:list_id/manga/:manga_id", sharedListHandler.RemoveMangaFromList)
		}
	}

	// ============================================================
	// ACTIVITY FEED ROUTES
	// ============================================================
	feedAuthRoutes := r.Group("/feed")
	feedAuthRoutes.Use(auth.AuthMiddleware())
	{
		feedAuthRoutes.POST("/activities", activityHandler.PostActivity)
		feedAuthRoutes.GET("/activities", activityHandler.GetActivityFeed)
		feedAuthRoutes.GET("/timeline", activityHandler.GetTimelineView)
		feedAuthRoutes.GET("/search", activityHandler.SearchActivities)
		feedAuthRoutes.GET("/stats", activityHandler.GetActivityStats)
		feedAuthRoutes.DELETE("/clear", activityHandler.ClearActivityFeed)
		feedAuthRoutes.GET("/notifications", activityHandler.GetActivityNotifications)
		feedAuthRoutes.GET("/stream", activityHandler.FollowActivityStream)
	}

	userActivityRoutes := r.Group("/users")
	userActivityRoutes.Use(auth.AuthMiddleware())
	{
		userActivityRoutes.GET("/:user_id/activities", activityHandler.GetUserActivities)
	}

	// ============================================================
	// --- Start server ---
	// ============================================================
	log.Printf("🚀 MangaHub API server starting on port %s", port)
	log.Printf("📖 Health check: http://localhost:%s/health", port)
	log.Printf("📚 Endpoints: POST /auth/register, POST /auth/login, GET /manga, GET /manga/:id")
	log.Printf("📚 Endpoints: POST /users/library, GET /users/library, PUT /users/progress")
	log.Printf("📢 Endpoints: POST /notify/broadcast, GET /notify/status")
	log.Printf("💬 Endpoints: GET /ws/chat (WebSocket), GET /chat/history")
	log.Printf("⭐ Endpoints: POST /manga/:id/reviews, GET /manga/:id/reviews, PUT /reviews/:review_id")
	log.Printf("👥 Endpoints: POST /friends/add, GET /users/friends, POST /friends/:friend_id/accept")
	log.Printf("📚 Endpoints: POST /reading-lists/create, GET /reading-lists/public, POST /reading-lists/:list_id/subscribe")
	log.Printf("📺 Endpoints: GET /feed/activities, GET /feed/timeline, GET /users/:user_id/activities")
	log.Printf("🔴 Endpoints: GET /cache/stats, DELETE /cache/flush (Redis cache)")
	log.Printf("🏥 Health: GET /health, /health/db, /health/cache, /health/tcp, /health/udp, /health/ws, /health/grpc")

	// Swagger UI — http://localhost:8080/swagger/index.html
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// seedDatabase seeds the database with initial manga data if empty.
func seedDatabase(mangaService *mangaPkg.Service, mangaDexClient *mangaPkg.MangaDexClient) {
	count, err := mangaService.GetCount()
	if err != nil {
		log.Printf("Warning: could not check manga count: %v", err)
		return
	}
	if count >= 200 {
		log.Printf("Database already has %d manga, skipping seed", count)
		return
	}

	if count < 100 {
		log.Println("Seeding database with initial manga data...")
		seedManga := data.GetSeedManga()
		inserted, err := mangaService.BulkCreate(seedManga)
		if err != nil {
			log.Printf("Warning: failed to seed database: %v", err)
		} else {
			log.Printf("✅ Successfully seeded %d manga series from static data", inserted)
		}
	}

	// Fetch from MangaDex if we still have < 200
	count, _ = mangaService.GetCount()
	if count < 200 {
		log.Println("Fetching additional manga from MangaDex API...")
		imported, err := mangaPkg.ImportFromMangaDex(mangaService, mangaDexClient, 100)
		if err != nil {
			log.Printf("Warning: failed to import from MangaDex: %v", err)
		} else {
			log.Printf("✅ Successfully imported %d manga from MangaDex API", imported)
		}
	}
}

// loadEnvFile reads a .env file and sets environment variables.
// Variables already set in the environment are NOT overwritten,
// so real env vars always take precedence over the .env file.
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env is optional — not an error if missing
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first '='
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if present
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// Only set if not already defined in environment
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	log.Println("Loaded configuration from .env file")
}
