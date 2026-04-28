package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"mangahub/data"
	"mangahub/internal/auth"
	mangaPkg "mangahub/internal/manga"
	userPkg "mangahub/internal/user"
	"mangahub/pkg/database"
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

	// --- Seed data on first run ---
	seedDatabase(mangaService)

	// --- Handlers ---
	userHandler := userPkg.NewHandler(userService)
	mangaHandler := mangaPkg.NewHandler(mangaService)

	// --- MangaDex Client ---
	mangaDexClient := mangaPkg.NewMangaDexClient()

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
		users.PUT("/progress", userHandler.UpdateProgress)
	}

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
	}

	// --- Start server ---
	log.Printf("🚀 MangaHub API server starting on port %s", port)
	log.Printf("📖 Health check: http://localhost:%s/health", port)
	log.Printf("📚 Endpoints: POST /auth/register, POST /auth/login, GET /manga, GET /manga/:id")
	log.Printf("📚 Endpoints: POST /users/library, GET /users/library, PUT /users/progress")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// seedDatabase seeds the database with initial manga data if empty.
func seedDatabase(mangaService *mangaPkg.Service) {
	count, err := mangaService.GetCount()
	if err != nil {
		log.Printf("Warning: could not check manga count: %v", err)
		return
	}
	if count > 0 {
		log.Printf("Database already has %d manga, skipping seed", count)
		return
	}

	log.Println("Seeding database with initial manga data...")
	seedManga := data.GetSeedManga()
	inserted, err := mangaService.BulkCreate(seedManga)
	if err != nil {
		log.Printf("Warning: failed to seed database: %v", err)
		return
	}
	log.Printf("✅ Successfully seeded %d manga series", inserted)
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

