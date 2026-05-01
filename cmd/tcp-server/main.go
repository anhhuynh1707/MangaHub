package main

import (
	"log"
	"os"

	"mangahub/internal/tcp"
)

func main() {
	port := os.Getenv("TCP_PORT")
	if port == "" {
		port = "9090"
	}

	server := tcp.NewProgressSyncServer(port)

	log.Printf("Starting TCP Progress Sync Server on port %s...", port)
	if err := server.Start(); err != nil {
		log.Fatalf("TCP server failed: %v", err)
	}
}
