package main

import (
	"log"
	"os"

	"mangahub/internal/auth"
	grpcServer "mangahub/internal/grpc"
	mangaPkg "mangahub/internal/manga"
	userPkg "mangahub/internal/user"
	"mangahub/pkg/config"
	"mangahub/pkg/database"
)

func main() {
	config.LoadDotEnv(".env")
	if err := auth.InitSecret(); err != nil {
		log.Fatalf("JWT secret error: %v", err)
	}

	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "9092"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	mangaRepo := mangaPkg.NewRepository(db)
	mangaService := mangaPkg.NewService(mangaRepo)

	userRepo := userPkg.NewRepository(db)
	userService := userPkg.NewService(userRepo)

	log.Printf("Starting standalone gRPC server on :%s", port)
	if err := grpcServer.StartGRPCServer(port, mangaService, userService); err != nil {
		log.Fatalf("gRPC server failed: %v", err)
	}
}
