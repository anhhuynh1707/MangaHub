package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func handleManga(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub manga <search|info|list>")
		return
	}

	switch args[0] {
	case "search":
		mangaSearch(args[1:])
	case "info":
		mangaInfo(args[1:])
	case "list":
		mangaList(args[1:])
	default:
		fmt.Printf("✗ Unknown manga command: '%s'\n", args[0])
		fmt.Println("Available: search, info, list")
	}
}

func mangaSearch(args []string) {
	// Collect query from positional args (non-flag args)
	query := ""
	genre := parseFlag(args, "genre")
	status := parseFlag(args, "status")
	limit := parseFlag(args, "limit")

	var queryParts []string
	for i, arg := range args {
		if strings.HasPrefix(arg, "--") {
			continue
		}
		if i > 0 && strings.HasPrefix(args[i-1], "--") {
			continue
		}
		queryParts = append(queryParts, arg)
	}
	query = strings.Join(queryParts, " ")

	if query == "" && genre == "" {
		fmt.Println("Usage: mangahub manga search <query> [--genre <genre>] [--status <status>] [--limit <n>]")
		return
	}

	// Build URL
	path := "/manga?search=" + urlEncode(query)
	if genre != "" {
		path += "&genre=" + urlEncode(genre)
	}
	if status != "" {
		path += "&status=" + urlEncode(status)
	}
	if limit != "" {
		path += "&limit=" + limit
	}

	fmt.Printf("Searching for \"%s\"...\n\n", query)

	resp, err := apiRequest("GET", path, nil, "")
	if err != nil {
		fmt.Printf("✗ Search failed: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	type mangaItem struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Author        string   `json:"author"`
		Genres        []string `json:"genres"`
		Status        string   `json:"status"`
		TotalChapters int      `json:"total_chapters"`
		Description   string   `json:"description"`
		CoverURL      string   `json:"cover_url"`
	}

	// API returns {manga: [...], total, page, limit}
	var searchResult struct {
		Manga []mangaItem `json:"manga"`
		Total int         `json:"total"`
	}
	json.Unmarshal(resp.Data, &searchResult)

	if len(searchResult.Manga) == 0 {
		fmt.Println("No manga found matching your search criteria.")
		fmt.Println("\nSuggestions:")
		fmt.Println("  - Check spelling and try again")
		fmt.Println("  - Use broader search terms")
		fmt.Println("  - Browse by genre: mangahub manga list --genre action")
		return
	}

	fmt.Printf("Found %d results:\n\n", len(searchResult.Manga))

	headers := []string{"ID", "Title", "Author", "Status", "Chapters"}
	var rows [][]string
	for _, m := range searchResult.Manga {
		title := m.Title
		if len(title) > 28 {
			title = title[:25] + "..."
		}
		rows = append(rows, []string{
			m.ID, title, m.Author, m.Status, fmt.Sprintf("%d", m.TotalChapters),
		})
	}
	printTable(headers, rows)

	fmt.Println("\nUse 'mangahub manga info <id>' to view details")
	fmt.Println("Use 'mangahub library add --manga-id <id>' to add to your library")
}

func mangaInfo(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub manga info <manga-id>")
		return
	}

	mangaID := args[0]
	resp, err := apiRequest("GET", "/manga/"+mangaID, nil, "")
	if err != nil {
		fmt.Printf("✗ Failed to fetch manga: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ Manga not found: '%s'\n", mangaID)
		fmt.Println("\nTry searching instead:")
		fmt.Println("  mangahub manga search \"manga title\"")
		return
	}

	var m struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Author        string   `json:"author"`
		Genres        []string `json:"genres"`
		Status        string   `json:"status"`
		TotalChapters int      `json:"total_chapters"`
		Description   string   `json:"description"`
		CoverURL      string   `json:"cover_url"`
	}
	json.Unmarshal(resp.Data, &m)

	titleUpper := strings.ToUpper(m.Title)
	border := repeat("─", len(titleUpper)+4)

	fmt.Printf("┌%s┐\n", border)
	fmt.Printf("│  %s  │\n", titleUpper)
	fmt.Printf("└%s┘\n\n", border)

	fmt.Println("Basic Information:")
	fmt.Printf("  ID:       %s\n", m.ID)
	fmt.Printf("  Title:    %s\n", m.Title)
	fmt.Printf("  Author:   %s\n", m.Author)
	fmt.Printf("  Genres:   %s\n", strings.Join(m.Genres, ", "))
	fmt.Printf("  Status:   %s\n", strings.Title(m.Status))

	fmt.Println("\nProgress:")
	fmt.Printf("  Total Chapters: %d\n", m.TotalChapters)

	if m.CoverURL != "" {
		fmt.Printf("\nCover: %s\n", m.CoverURL)
	}

	if m.Description != "" {
		fmt.Printf("\nDescription:\n  %s\n", m.Description)
	}

	fmt.Println("\nActions:")
	fmt.Printf("  Add to library:    mangahub library add --manga-id %s --status reading\n", m.ID)
	fmt.Printf("  Update progress:   mangahub progress update --manga-id %s --chapter <n>\n", m.ID)
}

func mangaList(args []string) {
	genre := parseFlag(args, "genre")
	status := parseFlag(args, "status")
	page := parseFlag(args, "page")
	limit := parseFlag(args, "limit")

	path := "/manga?"
	if genre != "" {
		path += "genre=" + urlEncode(genre) + "&"
	}
	if status != "" {
		path += "status=" + urlEncode(status) + "&"
	}
	if page != "" {
		path += "page=" + page + "&"
	}
	if limit != "" {
		path += "limit=" + limit + "&"
	}

	resp, err := apiRequest("GET", path, nil, "")
	if err != nil {
		fmt.Printf("✗ Failed to list manga: %v\n", err)
		return
	}

	// API returns {manga: [...], total, page, limit}
	type mangaListItem struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Author        string   `json:"author"`
		Status        string   `json:"status"`
		TotalChapters int      `json:"total_chapters"`
		Genres        []string `json:"genres"`
	}
	var listResult struct {
		Manga []mangaListItem `json:"manga"`
		Total int             `json:"total"`
	}
	json.Unmarshal(resp.Data, &listResult)

	if len(listResult.Manga) == 0 {
		fmt.Println("No manga found.")
		return
	}

	fmt.Printf("Manga Database (%d entries):\n\n", len(listResult.Manga))

	headers := []string{"ID", "Title", "Author", "Genres", "Status", "Chapters"}
	var rows [][]string
	for _, m := range listResult.Manga {
		title := m.Title
		if len(title) > 25 {
			title = title[:22] + "..."
		}
		genres := strings.Join(m.Genres, ", ")
		if len(genres) > 20 {
			genres = genres[:17] + "..."
		}
		rows = append(rows, []string{
			m.ID, title, m.Author, genres, m.Status, fmt.Sprintf("%d", m.TotalChapters),
		})
	}
	printTable(headers, rows)
}
