package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func handleManga(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub manga <search|advanced|info|list|recommend>")
		fmt.Println()
		fmt.Println("  search    Basic search by title/author/genre")
		fmt.Println("  advanced  Advanced search with multi-genre, rating, and sort filters")
		fmt.Println("  info      View details of a specific manga")
		fmt.Println("  list      List all manga with optional filters")
		fmt.Println("  recommend Get personalised recommendations based on your reading history")
		return
	}

	switch args[0] {
	case "search":
		mangaSearch(args[1:])
	case "advanced":
		mangaAdvancedSearch(args[1:])
	case "info":
		mangaInfo(args[1:])
	case "list":
		mangaList(args[1:])
	case "recommend":
		mangaRecommend(args[1:])
	default:
		fmt.Printf("✗ Unknown manga command: '%s'\n", args[0])
		fmt.Println("Available: search, advanced, info, list, recommend")
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

func mangaAdvancedSearch(args []string) {
	cfg := requireAuth()

	genresFlag := parseFlag(args, "genres")   // comma-separated: "action,romance"
	status := parseFlag(args, "status")
	sortBy := parseFlag(args, "sort")
	minRatingStr := parseFlag(args, "min-rating")
	limitStr := parseFlag(args, "limit")
	pageStr := parseFlag(args, "page")

	// Collect positional args as the search query
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
	query := strings.Join(queryParts, " ")

	if query == "" && genresFlag == "" && status == "" {
		fmt.Println("Usage: mangahub manga advanced [query] [--genres <g1,g2>] [--status <status>]")
		fmt.Println("                               [--sort <title|popularity|rating|recent>]")
		fmt.Println("                               [--min-rating <1-10>] [--limit <n>] [--page <n>]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  mangahub manga advanced one piece --genres action,adventure --sort rating")
		fmt.Println("  mangahub manga advanced --genres romance --min-rating 8 --sort popularity")
		fmt.Println("  mangahub manga advanced --status ongoing --sort recent --limit 5")
		return
	}

	var genres []string
	if genresFlag != "" {
		for _, g := range strings.Split(genresFlag, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				genres = append(genres, g)
			}
		}
	}

	var minRating float64
	if minRatingStr != "" {
		fmt.Sscanf(minRatingStr, "%f", &minRating)
	}

	limit := 20
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}
	page := 1
	if pageStr != "" {
		fmt.Sscanf(pageStr, "%d", &page)
	}

	body := map[string]interface{}{
		"search":     query,
		"genres":     genres,
		"status":     status,
		"min_rating": minRating,
		"sort_by":    sortBy,
		"limit":      limit,
		"page":       page,
	}

	fmt.Printf("🔍 Advanced search")
	if query != "" {
		fmt.Printf(" for \"%s\"", query)
	}
	if len(genres) > 0 {
		fmt.Printf(" | genres: %s", strings.Join(genres, ", "))
	}
	if sortBy != "" {
		fmt.Printf(" | sort: %s", sortBy)
	}
	if minRating > 0 {
		fmt.Printf(" | min rating: %.1f", minRating)
	}
	fmt.Println()

	resp, err := apiRequest("POST", "/manga/search", body, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Advanced search failed: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var result struct {
		Manga []struct {
			ID            string   `json:"id"`
			Title         string   `json:"title"`
			Author        string   `json:"author"`
			Genres        []string `json:"genres"`
			Status        string   `json:"status"`
			TotalChapters int      `json:"total_chapters"`
		} `json:"manga"`
		Total int `json:"total"`
		Page  int `json:"page"`
		Pages int `json:"pages"`
	}
	json.Unmarshal(resp.Data, &result)

	if len(result.Manga) == 0 {
		fmt.Println("\nNo manga found matching your filters.")
		fmt.Println("Try broadening your search criteria.")
		return
	}

	fmt.Printf("\nFound %d results (page %d/%d):\n\n", result.Total, result.Page, result.Pages)

	headers := []string{"ID", "Title", "Author", "Genres", "Status", "Ch."}
	var rows [][]string
	for _, m := range result.Manga {
		title := m.Title
		if len(title) > 25 {
			title = title[:22] + "..."
		}
		genres := strings.Join(m.Genres, ", ")
		if len(genres) > 18 {
			genres = genres[:15] + "..."
		}
		rows = append(rows, []string{
			m.ID, title, m.Author, genres, m.Status, fmt.Sprintf("%d", m.TotalChapters),
		})
	}
	printTable(headers, rows)

	if result.Pages > result.Page {
		fmt.Printf("\nMore results: add --page %d\n", result.Page+1)
	}
	fmt.Println("\nTip: use 'mangahub manga info <id>' to see full details")
}

func mangaRecommend(args []string) {
	cfg := requireAuth()

	limitStr := parseFlag(args, "limit")
	limit := 10
	if limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	fmt.Println("🤖 Generating personalised recommendations...")
	fmt.Println("   (based on your reading history and similar users)")
	fmt.Println()

	resp, err := apiRequest("GET", fmt.Sprintf("/users/recommendations?limit=%d", limit), nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to get recommendations: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var result struct {
		UserID string `json:"user_id"`
		Recs   []struct {
			MangaID string  `json:"manga_id"`
			Score   float64 `json:"score"`
			Reason  string  `json:"reason"`
			Manga   *struct {
				Title  string   `json:"title"`
				Author string   `json:"author"`
				Genres []string `json:"genres"`
				Status string   `json:"status"`
			} `json:"manga"`
		} `json:"recommendations"`
		Stats struct {
			TotalRead      int      `json:"total_read"`
			TotalCompleted int      `json:"total_completed"`
			TopGenres      []string `json:"top_genres"`
			SimilarUsers   int      `json:"similar_users_found"`
		} `json:"profile_stats"`
	}
	json.Unmarshal(resp.Data, &result)

	// Profile summary
	fmt.Printf("📚 Your Reading Profile:\n")
	fmt.Printf("   Read: %d manga | Completed: %d | Similar users found: %d\n",
		result.Stats.TotalRead, result.Stats.TotalCompleted, result.Stats.SimilarUsers)
	if len(result.Stats.TopGenres) > 0 {
		fmt.Printf("   Favourite genres: %s\n", strings.Join(result.Stats.TopGenres, ", "))
	}
	fmt.Println()

	if len(result.Recs) == 0 {
		fmt.Println("No recommendations yet.")
		fmt.Println("Add more manga to your library to improve suggestions:")
		fmt.Println("  mangahub library add --manga-id <id> --status reading")
		return
	}

	fmt.Printf("🌟 Top %d Recommendations for %s:\n\n", len(result.Recs), result.UserID)

	for i, rec := range result.Recs {
		title := rec.MangaID
		author := ""
		genres := ""
		status := ""
		if rec.Manga != nil {
			title = rec.Manga.Title
			author = rec.Manga.Author
			genres = strings.Join(rec.Manga.Genres, ", ")
			if len(genres) > 30 {
				genres = genres[:27] + "..."
			}
			status = rec.Manga.Status
		}

		fmt.Printf("  %2d. %-30s  score: %.2f\n", i+1, title, rec.Score)
		if author != "" {
			fmt.Printf("      Author: %-20s  Status: %s\n", author, status)
		}
		if genres != "" {
			fmt.Printf("      Genres: %s\n", genres)
		}
		fmt.Printf("      Reason: %s\n", rec.Reason)
		fmt.Printf("      ID: %s\n", rec.MangaID)
		fmt.Println()
	}

	fmt.Println("Add to library: mangahub library add --manga-id <id> --status reading")
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
