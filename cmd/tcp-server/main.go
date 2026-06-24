package main

import (
	"log"
	"os"

	"mangahub/internal/auth"
	"mangahub/internal/tcp"
	userPkg "mangahub/internal/user"
	"mangahub/pkg/config"
	"mangahub/pkg/database"
)

func main() {
	config.LoadDotEnv(".env")
	if err := auth.InitSecret(); err != nil {
		log.Fatalf("JWT secret error: %v", err)
	}

	port := os.Getenv("TCP_PORT")
	if port == "" {
		port = "9090"
	}

	// Initialize database for progress persistence
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Printf("Warning: Failed to init DB for TCP persistence: %v", err)
		log.Println("TCP server will run WITHOUT database persistence")
	}

	server := tcp.NewProgressSyncServer(port)

	// Wire up the Persister so progress updates save to DB
	if db != nil {
		userRepo := userPkg.NewRepository(db)
		userService := userPkg.NewService(userRepo)
		server.Persister = userService
		log.Println("Database persistence enabled for TCP progress updates")
		defer db.Close()
	}

	log.Printf("Starting TCP Progress Sync Server on port %s...", port)
	if err := server.Start(); err != nil {
		log.Fatalf("TCP server failed: %v", err)
	}
}
