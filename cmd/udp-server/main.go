package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"mangahub/internal/udp"
)

func main() {
	port := os.Getenv("UDP_PORT")
	if port == "" {
		port = "9091"
	}

	server := udp.NewNotificationServer(port)

	// Graceful shutdown on Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	shutdownComplete := make(chan struct{})

	go func() {
		<-sigChan
		log.Println("Shutting down UDP server...")
		server.Stop()
		close(shutdownComplete)
	}()

	log.Printf("Starting standalone UDP Notification Server on :%s", port)
	if err := server.Start(); err != nil {
		log.Fatalf("UDP server error: %v", err)
	}
	<-shutdownComplete
}
