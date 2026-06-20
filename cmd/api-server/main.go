package main

import (
	"log"
	"os"
	"strconv"

	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/internal/friend"
	mangaPkg "mangahub/internal/manga"
	"mangahub/internal/recommendation"
	"mangahub/internal/review"
	"mangahub/internal/sharedlist"
	userPkg "mangahub/internal/user"
	wsPkg "mangahub/internal/websocket"
	"mangahub/pkg/cache"
	"mangahub/pkg/database"

	"github.com/gin-gonic/gin"

	_ "mangahub/docs"
)

func main() {
	loadEnvFile(".env")

	// --- Configuration ---
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "./data/mangahub.db")
	tcpPort := envOr("TCP_PORT", "9090")
	udpPort := envOr("UDP_PORT", "9091")
	grpcPort := envOr("GRPC_PORT", "9092")
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
		auth.JWTSecret = []byte(jwtSecret)
	}
	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
		gin.SetMode(ginMode)
	}
	enableTCP := envOr("ENABLE_TCP_SERVER", "true") == "true"
	enableUDP := envOr("ENABLE_UDP_SERVER", "true") == "true"
	enableGRPC := envOr("ENABLE_GRPC_SERVER", "true") == "true"

	// --- Database + cache ---
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	redisDB := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		redisDB, _ = strconv.Atoi(v)
	}
	redisCache := cache.New(envOr("REDIS_ADDR", "localhost:6379"), os.Getenv("REDIS_PASSWORD"), redisDB)
	defer redisCache.Close()

	// --- Services ---
	userService := userPkg.NewService(userPkg.NewRepository(db))
	mangaService := mangaPkg.NewService(mangaPkg.NewRepository(db))
	reviewService := review.NewService(review.NewRepository(db))
	friendService := friend.NewService(friend.NewRepository(db))
	sharedListService := sharedlist.NewService(sharedlist.NewRepository(db))
	activityService := activity.NewService(activity.NewRepository(db))
	recService := recommendation.NewService(db)

	mangaService.SetCache(redisCache)
	userService.SetCache(redisCache)
	activityService.SetCache(redisCache)

	mangaDexClient := mangaPkg.NewMangaDexClient()

	// --- Server (holds every dependency the HTTP handlers need) ---
	s := &APIServer{
		Router:            gin.Default(),
		Database:          db,
		Cache:             redisCache,
		Hub:               wsPkg.NewChatHub(),
		MangaService:      mangaService,
		UserService:       userService,
		RecService:        recService,
		MangaDexClient:    mangaDexClient,
		UserHandler:       userPkg.NewHandler(userService, activityService, mangaService),
		MangaHandler:      mangaPkg.NewHandler(mangaService),
		ReviewHandler:     review.NewHandler(reviewService, activityService, mangaService),
		FriendHandler:     friend.NewHandler(friendService, activityService),
		SharedListHandler: sharedlist.NewHandler(sharedListService, activityService),
		ActivityHandler:   activity.NewHandler(activityService),
		Port:              port,
		TCPPort:           tcpPort,
		UDPPort:           udpPort,
		GRPCPort:          grpcPort,
		EnableGRPC:        enableGRPC,
		UseClients:        !enableTCP || !enableUDP,
	}

	// --- Real-time services (goroutines) ---
	startTCP(s, enableTCP, userService)
	startUDP(s, enableUDP)
	go s.Hub.Run()
	startGRPC(s, enableGRPC, mangaService, userService)

	// Seed in the background so the API listens immediately (see seedDatabase).
	go seedDatabase(mangaService, mangaDexClient)

	// --- Routes + start ---
	s.registerRoutes(corsOrigins())
	log.Printf("🚀 MangaHub API server starting on port %s", port)
	log.Printf("🏥 Health check: http://localhost:%s/health", port)
	log.Printf("📚 Swagger UI:  http://localhost:%s/swagger/index.html", port)
	if err := s.Router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
