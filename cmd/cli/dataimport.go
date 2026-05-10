package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func handleImport(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub import <library|progress|manga>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  library   Import library entries from JSON or CSV")
		fmt.Println("  progress  Import reading progress from JSON or CSV")
		fmt.Println("  manga     Import manga data from JSON or CSV")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  --file <path>   Input file path (required)")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  mangahub import library --file library.json")
		fmt.Println("  mangahub import progress --file progress.csv")
		fmt.Println("  mangahub import manga --file manga.json")
		return
	}

	switch args[0] {
	case "library":
		importLibrary(args[1:])
	case "progress":
		importProgress(args[1:])
	case "manga":
		importManga(args[1:])
	default:
		fmt.Printf("✗ Unknown import target: '%s'\n", args[0])
		fmt.Println("Available: library, progress, manga")
	}
}

// importLibrary imports library entries from a JSON or CSV file.
func importLibrary(args []string) {
	filePath := parseFlag(args, "file")
	if filePath == "" {
		fmt.Println("Usage: mangahub import library --file <path>")
		fmt.Println("  Supported formats: .json, .csv")
		return
	}

	cfg := requireAuth()
	format := detectFormat(filePath)

	fmt.Printf("Importing library from %s...\n", filePath)

	type libEntry struct {
		MangaID string `json:"manga_id"`
		Status  string `json:"status"`
	}

	var entries []libEntry

	switch format {
	case "json":
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("✗ Failed to read file: %v\n", err)
			return
		}
		// Try wrapped format first: {"entries": [...]}
		var wrapped struct {
			Entries []libEntry `json:"entries"`
		}
		if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Entries) > 0 {
			entries = wrapped.Entries
		} else {
			// Try flat array
			json.Unmarshal(data, &entries)
		}

	case "csv":
		records, err := readCSV(filePath)
		if err != nil {
			fmt.Printf("✗ %v\n", err)
			return
		}
		for _, r := range records {
			status := "plan_to_read"
			if len(r) >= 3 {
				status = r[2] // status column
			}
			entries = append(entries, libEntry{MangaID: r[0], Status: status})
		}

	default:
		fmt.Printf("✗ Unsupported file format. Use .json or .csv\n")
		return
	}

	if len(entries) == 0 {
		fmt.Println("✗ No entries found in file")
		return
	}

	imported := 0
	skipped := 0
	for _, e := range entries {
		body := map[string]string{"manga_id": e.MangaID, "status": e.Status}
		resp, err := apiRequest("POST", "/users/library", body, cfg.Token)
		if err != nil {
			fmt.Printf("  ✗ %s: connection error\n", e.MangaID)
			continue
		}
		if !resp.Success {
			skipped++
			continue
		}
		imported++
	}

	fmt.Printf("\n✓ Library import complete!\n")
	fmt.Printf("  Imported: %d entries\n", imported)
	if skipped > 0 {
		fmt.Printf("  Skipped:  %d (already in library)\n", skipped)
	}
}

// importProgress imports reading progress from a JSON or CSV file.
func importProgress(args []string) {
	filePath := parseFlag(args, "file")
	if filePath == "" {
		fmt.Println("Usage: mangahub import progress --file <path>")
		fmt.Println("  Supported formats: .json, .csv")
		return
	}

	cfg := requireAuth()
	format := detectFormat(filePath)

	fmt.Printf("Importing progress from %s...\n", filePath)

	type progressImport struct {
		MangaID        string `json:"manga_id"`
		CurrentChapter int    `json:"current_chapter"`
		Status         string `json:"status"`
	}

	var entries []progressImport

	switch format {
	case "json":
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("✗ Failed to read file: %v\n", err)
			return
		}
		var wrapped struct {
			Progress []progressImport `json:"progress"`
		}
		if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Progress) > 0 {
			entries = wrapped.Progress
		} else {
			json.Unmarshal(data, &entries)
		}

	case "csv":
		records, err := readCSV(filePath)
		if err != nil {
			fmt.Printf("✗ %v\n", err)
			return
		}
		for _, r := range records {
			chapter := 0
			if len(r) >= 2 {
				chapter, _ = strconv.Atoi(r[1])
			}
			status := "reading"
			if len(r) >= 3 {
				status = r[2]
			}
			entries = append(entries, progressImport{
				MangaID:        r[0],
				CurrentChapter: chapter,
				Status:         status,
			})
		}

	default:
		fmt.Printf("✗ Unsupported file format. Use .json or .csv\n")
		return
	}

	if len(entries) == 0 {
		fmt.Println("✗ No entries found in file")
		return
	}

	updated := 0
	failed := 0
	for _, e := range entries {
		body := map[string]interface{}{
			"manga_id":        e.MangaID,
			"current_chapter": e.CurrentChapter,
			"status":          e.Status,
		}
		resp, err := apiRequest("PUT", "/users/progress", body, cfg.Token)
		if err != nil {
			fmt.Printf("  ✗ %s: connection error\n", e.MangaID)
			failed++
			continue
		}
		if !resp.Success {
			fmt.Printf("  ✗ %s: %s\n", e.MangaID, resp.Error)
			failed++
			continue
		}
		updated++
	}

	fmt.Printf("\n✓ Progress import complete!\n")
	fmt.Printf("  Updated: %d entries\n", updated)
	if failed > 0 {
		fmt.Printf("  Failed:  %d (manga may not be in library)\n", failed)
	}
}

// importManga imports manga data from a JSON or CSV file.
func importManga(args []string) {
	filePath := parseFlag(args, "file")
	if filePath == "" {
		fmt.Println("Usage: mangahub import manga --file <path>")
		fmt.Println("  Supported formats: .json, .csv")
		return
	}

	cfg := requireAuth()
	format := detectFormat(filePath)

	fmt.Printf("Importing manga from %s...\n", filePath)

	type mangaImport struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Author        string   `json:"author"`
		Genres        []string `json:"genres"`
		Status        string   `json:"status"`
		TotalChapters int      `json:"total_chapters"`
		Description   string   `json:"description"`
	}

	var mangaList []mangaImport

	switch format {
	case "json":
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("✗ Failed to read file: %v\n", err)
			return
		}
		json.Unmarshal(data, &mangaList)

	case "csv":
		records, err := readCSV(filePath)
		if err != nil {
			fmt.Printf("✗ %v\n", err)
			return
		}
		for _, r := range records {
			if len(r) < 3 {
				continue
			}
			var genres []string
			if len(r) >= 4 && r[3] != "" {
				genres = strings.Split(r[3], ";")
			}
			status := ""
			if len(r) >= 5 {
				status = r[4]
			}
			totalCh := 0
			if len(r) >= 6 {
				totalCh, _ = strconv.Atoi(r[5])
			}
			desc := ""
			if len(r) >= 7 {
				desc = r[6]
			}
			mangaList = append(mangaList, mangaImport{
				ID:            r[0],
				Title:         r[1],
				Author:        r[2],
				Genres:        genres,
				Status:        status,
				TotalChapters: totalCh,
				Description:   desc,
			})
		}

	default:
		fmt.Printf("✗ Unsupported file format. Use .json or .csv\n")
		return
	}

	if len(mangaList) == 0 {
		fmt.Println("✗ No manga found in file")
		return
	}

	imported := 0
	skipped := 0
	for _, m := range mangaList {
		resp, err := apiRequest("POST", "/manga", m, cfg.Token)
		if err != nil {
			fmt.Printf("  ✗ %s: connection error\n", m.ID)
			continue
		}
		if !resp.Success {
			skipped++
			continue
		}
		imported++
	}

	fmt.Printf("\n✓ Manga import complete!\n")
	fmt.Printf("  Imported: %d manga\n", imported)
	if skipped > 0 {
		fmt.Printf("  Skipped:  %d (already exist)\n", skipped)
	}
}

// --- Helpers ---

// detectFormat returns "json" or "csv" based on file extension.
func detectFormat(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return "json"
	case ".csv":
		return "csv"
	default:
		return ext
	}
}

// readCSV reads a CSV file skipping the header, returns data rows.
func readCSV(filePath string) ([][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file has no data rows")
	}

	return records[1:], nil // skip header row
}
