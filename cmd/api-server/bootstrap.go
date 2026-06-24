package main

import (
	"log"
	"os"
	"strings"

	grpcServer "mangahub/internal/grpc"
	mangaPkg "mangahub/internal/manga"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	userPkg "mangahub/internal/user"
)

// startTCP starts the internal TCP progress-sync server, or connects to a remote
// one in standalone mode.
func startTCP(s *APIServer, enabled bool, persister *userPkg.Service) {
	if enabled {
		tcpServer := tcp.NewProgressSyncServer(s.TCPPort)
		tcpServer.Persister = persister
		s.TCPServer = tcpServer
		log.Println("Starting internal TCP server...")
		go func() {
			if err := tcpServer.Start(); err != nil {
				log.Printf("TCP server error: %v", err)
			}
		}()
		return
	}
	log.Printf("TCP server disabled - using remote service at %s", s.TCPPort)
	if client, err := tcp.NewProgressSyncClient(s.TCPPort); err != nil {
		log.Printf("Warning: Failed to connect to remote TCP server: %v", err)
	} else {
		s.TCPClient = client
	}
}

// startUDP starts the internal UDP notification server, or connects to a remote one.
func startUDP(s *APIServer, enabled bool) {
	if enabled {
		udpServer := udp.NewNotificationServer(s.UDPPort)
		s.UDPServer = udpServer
		log.Println("Starting internal UDP server...")
		go func() {
			if err := udpServer.Start(); err != nil {
				log.Printf("UDP server error: %v", err)
			}
		}()
		return
	}
	log.Printf("UDP server disabled - using remote service at %s", s.UDPPort)
	if client, err := udp.NewNotificationClient(s.UDPPort); err != nil {
		log.Printf("Warning: Failed to connect to remote UDP server: %v", err)
	} else {
		s.UDPClient = client
	}
}

// startGRPC starts the internal gRPC service.
func startGRPC(s *APIServer, enabled bool, mangaService *mangaPkg.Service, userService *userPkg.Service) {
	if !enabled {
		log.Println("gRPC server disabled (using external service)")
		return
	}
	log.Println("Starting internal gRPC server...")
	gms := grpcServer.NewMangaServer(mangaService, userService)
	s.GRPCMangaServer = gms
	go func() {
		if err := grpcServer.ServeGRPC(s.GRPCPort, gms); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()
}

// seedDatabase populates the database from MangaDex API on first run. Runs in a
// background goroutine so the HTTP server can start listening immediately.
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
	log.Println("Fetching manga from MangaDex API...")
	if imported, err := mangaPkg.ImportFromMangaDex(mangaService, mangaDexClient, 200); err != nil {
		log.Printf("Warning: failed to import from MangaDex: %v", err)
	} else {
		log.Printf("✅ Successfully imported %d manga from MangaDex API", imported)
	}
}

// corsOrigins returns the allowed CORS origins from FRONTEND_ORIGINS (comma
// separated), defaulting to the Vite dev server and the Docker frontend port.
func corsOrigins() []string {
	var origins []string
	for _, o := range strings.Split(os.Getenv("FRONTEND_ORIGINS"), ",") {
		if t := strings.TrimSpace(o); t != "" {
			origins = append(origins, t)
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:5173", "http://localhost:3000"}
	}
	return origins
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
