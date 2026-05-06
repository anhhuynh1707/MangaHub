package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"mangahub/data"
	"mangahub/internal/auth"
	mangaPkg "mangahub/internal/manga"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	wsPkg "mangahub/internal/websocket"
	userPkg "mangahub/internal/user"
	"mangahub/pkg/database"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// APIServer is the core server structure per spec requirements.
type APIServer struct {
	Router    *gin.Engine
	Database  *sql.DB
	JWTSecret string
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

	// --- Build APIServer ---
	server := &APIServer{
		Router:    gin.Default(),
		Database:  db,
		JWTSecret: string(auth.JWTSecret),
	}

	// --- Repositories ---
	userRepo := userPkg.NewRepository(db)
	mangaRepo := mangaPkg.NewRepository(db)

	// --- Services ---
	userService := userPkg.NewService(userRepo)
	mangaService := mangaPkg.NewService(mangaRepo)

	// --- MangaDex Client ---
	mangaDexClient := mangaPkg.NewMangaDexClient()

	// --- Seed data on first run ---
	seedDatabase(mangaService, mangaDexClient)

	// --- Handlers ---
	userHandler := userPkg.NewHandler(userService)
	mangaHandler := mangaPkg.NewHandler(mangaService)

	// --- TCP Progress Sync Server (runs in goroutine) ---
	tcpPort := os.Getenv("TCP_PORT")
	if tcpPort == "" {
		tcpPort = "9090"
	}
	tcpServer := tcp.NewProgressSyncServer(tcpPort)
	tcpServer.Persister = userService // Save TCP progress updates to DB
	go func() {
		if err := tcpServer.Start(); err != nil {
			log.Printf("TCP server error: %v", err)
		}
	}()

	// --- UDP Notification Server (runs in goroutine) ---
	udpPort := os.Getenv("UDP_PORT")
	if udpPort == "" {
		udpPort = "9091"
	}
	udpServer := udp.NewNotificationServer(udpPort)
	go func() {
		if err := udpServer.Start(); err != nil {
			log.Printf("UDP server error: %v", err)
		}
	}()

	// --- WebSocket Chat Hub (runs in goroutine) ---
	chatHub := wsPkg.NewChatHub()
	go chatHub.Run()

	// --- Routes ---
	r := server.Router

	// Health check
	r.GET("/health", func(c *gin.Context) {
		count, _ := mangaService.GetCount()
		utils.SuccessResponse(c, "MangaHub API is running", gin.H{
			"status":      "healthy",
			"manga_count": count,
		})
	})

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
	r.GET("/manga/:id", mangaHandler.GetByID)

	// Manga routes (authenticated write)
	mangaAuth := r.Group("/manga")
	mangaAuth.Use(auth.AuthMiddleware())
	{
		mangaAuth.POST("", mangaHandler.Create)
		mangaAuth.PUT("/:id", mangaHandler.Update)
		mangaAuth.DELETE("/:id", mangaHandler.Delete)
	}

	// User routes (authenticated)
	users := r.Group("/users")
	users.Use(auth.AuthMiddleware())
	{
		users.GET("/profile", userHandler.GetProfile)
		users.POST("/library", userHandler.AddToLibrary)
		users.GET("/library", userHandler.GetLibrary)
		users.DELETE("/library/:manga_id", userHandler.RemoveFromLibrary)
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
					tcpServer.SendProgressUpdate(models.ProgressUpdate{
						UserID:    userID,
						MangaID:   mangaID,
						Chapter:   chapter,
						Timestamp: time.Now().Unix(),
					})
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
			connectedUsers := tcpServer.GetConnectedUsers()
			uptime := tcpServer.GetUptime()

			utils.SuccessResponse(c, "TCP sync server status", gin.H{
				"server":          fmt.Sprintf("localhost:%s", tcpPort),
				"uptime":          uptime.String(),
				"connected_count": len(connectedUsers),
				"connected_users": connectedUsers,
				"your_user_id":    userID,
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
				Type:    req.Type,
				MangaID: req.MangaID,
				Message: req.Message,
			}
			sent := udpServer.BroadcastNotification(notif)
			utils.SuccessResponse(c, fmt.Sprintf("Notification sent to %d clients", sent), gin.H{
				"type":       req.Type,
				"sent_count": sent,
				"message":    req.Message,
			})
		})

		notifyRoutes.GET("/status", func(c *gin.Context) {
			utils.SuccessResponse(c, "UDP notification server status", gin.H{
				"server":       fmt.Sprintf("localhost:%s", udpPort),
				"client_count": udpServer.GetClientCount(),
				"clients":      udpServer.GetClients(),
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
	}

	// --- Start server ---
	log.Printf("🚀 MangaHub API server starting on port %s", port)
	log.Printf("📖 Health check: http://localhost:%s/health", port)
	log.Printf("📚 Endpoints: POST /auth/register, POST /auth/login, GET /manga, GET /manga/:id")
	log.Printf("📚 Endpoints: POST /users/library, GET /users/library, PUT /users/progress")
	log.Printf("📢 Endpoints: POST /notify/broadcast, GET /notify/status")
	log.Printf("💬 Endpoints: GET /ws/chat (WebSocket), GET /chat/history")

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
