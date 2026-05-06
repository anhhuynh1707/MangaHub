package main

import (
	"fmt"
	"strings"

	grpcClient "mangahub/internal/grpc"
)

const defaultGRPCAddr = "localhost:9092"

func handleGRPC(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub grpc <manga|progress>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  manga      Manga operations via gRPC")
		fmt.Println("  progress   Progress operations via gRPC")
		fmt.Println()
		fmt.Println("Subcommands:")
		fmt.Println("  mangahub grpc manga get --id <manga-id>")
		fmt.Println("  mangahub grpc manga search --query <term> [--genre <genre>] [--limit <n>]")
		fmt.Println("  mangahub grpc progress update --user-id <uid> --manga-id <mid> --chapter <n>")
		return
	}

	switch args[0] {
	case "manga":
		handleGRPCManga(args[1:])
	case "progress":
		handleGRPCProgress(args[1:])
	default:
		fmt.Printf("✗ Unknown gRPC command: '%s'\n", args[0])
		fmt.Println("Available: manga, progress")
	}
}

func handleGRPCManga(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub grpc manga <get|search>")
		fmt.Println()
		fmt.Println("  get       Get a manga by ID")
		fmt.Println("  search    Search manga by title/genre")
		return
	}

	switch args[0] {
	case "get":
		grpcMangaGet(args[1:])
	case "search":
		grpcMangaSearch(args[1:])
	default:
		fmt.Printf("✗ Unknown grpc manga command: '%s'\n", args[0])
		fmt.Println("Available: get, search")
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
