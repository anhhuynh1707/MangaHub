package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	grpcClient "mangahub/internal/grpc"
	pb "mangahub/internal/grpc/pb"
)

const defaultGRPCAddr = "localhost:9092"

func handleGRPC(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub grpc <manga|progress|watch>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  manga      Manga operations via gRPC (get, search, stream)")
		fmt.Println("  progress   Progress operations via gRPC (update)")
		fmt.Println("  watch      Watch real-time manga events (server-side streaming)")
		fmt.Println()
		fmt.Println("Subcommands:")
		fmt.Println("  mangahub grpc manga get --id <manga-id>")
		fmt.Println("  mangahub grpc manga search --query <term> [--genre <genre>] [--limit <n>]")
		fmt.Println("  mangahub grpc manga stream --query <term> [--genre <genre>] [--limit <n>]")
		fmt.Println("  mangahub grpc progress update --user-id <uid> --manga-id <mid> --chapter <n>")
		fmt.Println("  mangahub grpc watch [--manga-id <id>]")
		return
	}

	switch args[0] {
	case "manga":
		handleGRPCManga(args[1:])
	case "progress":
		handleGRPCProgress(args[1:])
	case "watch":
		grpcWatch(args[1:])
	default:
		fmt.Printf("✗ Unknown gRPC command: '%s'\n", args[0])
		fmt.Println("Available: manga, progress, watch")
	}
}

func handleGRPCManga(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub grpc manga <get|search|stream>")
		fmt.Println()
		fmt.Println("  get     Get a manga by ID (unary RPC)")
		fmt.Println("  search  Search manga — returns full list at once (unary RPC)")
		fmt.Println("  stream  Search manga — results streamed one by one (server-side streaming)")
		return
	}

	switch args[0] {
	case "get":
		grpcMangaGet(args[1:])
	case "search":
		grpcMangaSearch(args[1:])
	case "stream":
		grpcMangaStream(args[1:])
	default:
		fmt.Printf("✗ Unknown grpc manga command: '%s'\n", args[0])
		fmt.Println("Available: get, search, stream")
	}
}

// grpcMangaGet fetches a single manga by ID via gRPC.
func grpcMangaGet(args []string) {
	id := parseFlag(args, "id")
	if id == "" {
		fmt.Println("Usage: mangahub grpc manga get --id <manga-id>")
		fmt.Println("Example: mangahub grpc manga get --id one-piece")
		return
	}

	cfg := requireAuth()

	client, err := grpcClient.NewMangaClient(defaultGRPCAddr, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to connect to gRPC server: %v\n", err)
		fmt.Println("  Make sure the server is running: go run ./cmd/api-server/")
		return
	}
	defer client.Close()

	manga, err := client.GetManga(id)
	if err != nil {
		fmt.Printf("✗ gRPC GetManga failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Manga found via gRPC\n\n")
	fmt.Printf("  ID:          %s\n", manga.Id)
	fmt.Printf("  Title:       %s\n", manga.Title)
	fmt.Printf("  Author:      %s\n", manga.Author)
	fmt.Printf("  Status:      %s\n", manga.Status)
	fmt.Printf("  Chapters:    %d\n", manga.TotalChapters)
	if len(manga.Genres) > 0 {
		fmt.Printf("  Genres:      %s\n", strings.Join(manga.Genres, ", "))
	}
	if manga.Description != "" {
		desc := manga.Description
		if len(desc) > 200 {
			desc = desc[:200] + "..."
		}
		fmt.Printf("  Description: %s\n", desc)
	}
}

// grpcMangaSearch searches manga by title and/or genre via gRPC.
func grpcMangaSearch(args []string) {
	query := parseFlag(args, "query")
	genre := parseFlag(args, "genre")
	limitStr := parseFlag(args, "limit")

	if query == "" && genre == "" {
		fmt.Println("Usage: mangahub grpc manga search --query <term> [--genre <genre>] [--limit <n>]")
		fmt.Println("Examples:")
		fmt.Println("  mangahub grpc manga search --query \"one piece\"")
		fmt.Println("  mangahub grpc manga search --genre Shounen --limit 10")
		return
	}

	var limit int32 = 20
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	cfg := requireAuth()

	client, err := grpcClient.NewMangaClient(defaultGRPCAddr, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to connect to gRPC server: %v\n", err)
		fmt.Println("  Make sure the server is running: go run ./cmd/api-server/")
		return
	}
	defer client.Close()

	resp, err := client.SearchManga(query, genre, limit)
	if err != nil {
		fmt.Printf("✗ gRPC SearchManga failed: %v\n", err)
		return
	}

	if len(resp.Results) == 0 {
		fmt.Println("No manga found matching your query.")
		return
	}

	fmt.Printf("✓ Found %d manga (total: %d) via gRPC\n\n", len(resp.Results), resp.Total)

	// Table header
	fmt.Printf("  %-25s %-30s %-20s %-10s %s\n", "ID", "Title", "Author", "Status", "Chapters")
	fmt.Printf("  %s %s %s %s %s\n",
		strings.Repeat("─", 25),
		strings.Repeat("─", 30),
		strings.Repeat("─", 20),
		strings.Repeat("─", 10),
		strings.Repeat("─", 8))

	for _, m := range resp.Results {
		title := m.Title
		if len(title) > 28 {
			title = title[:25] + "..."
		}
		author := m.Author
		if len(author) > 18 {
			author = author[:15] + "..."
		}
		id := m.Id
		if len(id) > 23 {
			id = id[:20] + "..."
		}
		fmt.Printf("  %-25s %-30s %-20s %-10s %d\n", id, title, author, m.Status, m.TotalChapters)
	}
}

func handleGRPCProgress(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub grpc progress update --user-id <uid> --manga-id <mid> --chapter <n>")
		return
	}

	switch args[0] {
	case "update":
		grpcProgressUpdate(args[1:])
	default:
		fmt.Printf("✗ Unknown grpc progress command: '%s'\n", args[0])
		fmt.Println("Available: update")
	}
}

// grpcMangaStream fetches manga results via server-side streaming (one message per manga).
func grpcMangaStream(args []string) {
	query := parseFlag(args, "query")
	genre := parseFlag(args, "genre")
	limitStr := parseFlag(args, "limit")

	if query == "" && genre == "" {
		fmt.Println("Usage: mangahub grpc manga stream --query <term> [--genre <genre>] [--limit <n>]")
		fmt.Println("Example: mangahub grpc manga stream --query \"one piece\" --limit 5")
		return
	}

	var limit int32 = 20
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	cfg := requireAuth()
	client, err := grpcClient.NewMangaClient(defaultGRPCAddr, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to connect to gRPC server: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Printf("📡 Streaming search results for \"%s\" (server-side streaming)...\n\n", query)

	count := 0
	err = client.StreamSearch(query, genre, limit, func(m *pb.MangaResponse) {
		count++
		title := m.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		genres := strings.Join(m.Genres, ", ")
		if len(genres) > 25 {
			genres = genres[:22] + "..."
		}
		fmt.Printf("  [%2d] %-32s %-20s %-12s %d ch | %s\n",
			count, title, m.Author, m.Status, m.TotalChapters, genres)
	})
	if err != nil {
		fmt.Printf("✗ Stream error: %v\n", err)
		return
	}
	fmt.Printf("\n✓ Stream complete — received %d results\n", count)
}

// grpcWatch subscribes to real-time manga update events via WatchMangaUpdates streaming RPC.
func grpcWatch(args []string) {
	mangaID := parseFlag(args, "manga-id")

	cfg := requireAuth()
	client, err := grpcClient.NewMangaClient(defaultGRPCAddr, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to connect to gRPC server: %v\n", err)
		return
	}
	defer client.Close()

	if mangaID != "" {
		fmt.Printf("📺 Watching events for manga: %s\n", mangaID)
	} else {
		fmt.Println("📺 Watching ALL manga update events (press Ctrl+C to stop)...")
	}
	fmt.Println("   Events stream live as users update progress or manga is changed.\n")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel on Ctrl+C
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		<-interrupt
		fmt.Println("\n✓ Watch stopped")
		cancel()
	}()

	err = client.WatchMangaUpdates(ctx, mangaID, cfg.UserID, func(event *pb.MangaEvent) {
		ts := time.Unix(event.Timestamp, 0).Format("15:04:05")
		switch event.EventType {
		case "connected":
			fmt.Printf("[%s] ✓ Connected — %s\n", ts, event.Message)
		case "progress_updated":
			fmt.Printf("[%s] 📖 PROGRESS  manga=%-20s  ch=%-5d  user=%s\n",
				ts, event.MangaId, event.Chapter, event.UserId)
		case "manga_updated":
			fmt.Printf("[%s] 📝 UPDATED   manga=%s  — %s\n",
				ts, event.MangaId, event.Message)
		default:
			fmt.Printf("[%s] 📨 %-15s manga=%s  %s\n",
				ts, event.EventType, event.MangaId, event.Message)
		}
	})
	if err != nil && ctx.Err() == nil {
		fmt.Printf("✗ Watch error: %v\n", err)
	}
}

// grpcProgressUpdate updates reading progress via gRPC.
func grpcProgressUpdate(args []string) {
	userID := parseFlag(args, "user-id")
	mangaID := parseFlag(args, "manga-id")
	chapterStr := parseFlag(args, "chapter")

	if userID == "" || mangaID == "" || chapterStr == "" {
		fmt.Println("Usage: mangahub grpc progress update --user-id <uid> --manga-id <mid> --chapter <n>")
		fmt.Println("Example: mangahub grpc progress update --user-id user-alice --manga-id one-piece --chapter 500")
		return
	}

	var chapter int32
	fmt.Sscanf(chapterStr, "%d", &chapter)

	cfg := requireAuth()

	client, err := grpcClient.NewMangaClient(defaultGRPCAddr, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to connect to gRPC server: %v\n", err)
		fmt.Println("  Make sure the server is running: go run ./cmd/api-server/")
		return
	}
	defer client.Close()

	resp, err := client.UpdateProgress(userID, mangaID, chapter)
	if err != nil {
		fmt.Printf("✗ gRPC UpdateProgress failed: %v\n", err)
		return
	}

	if resp.Success {
		fmt.Printf("✓ %s\n", resp.Message)
	} else {
		fmt.Printf("✗ %s\n", resp.Message)
	}
}
