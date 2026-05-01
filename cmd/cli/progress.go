package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func handleProgress(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub progress <update|history>")
		return
	}

	switch args[0] {
	case "update":
		progressUpdate(args[1:])
	case "history":
		progressHistory(args[1:])
	default:
		fmt.Printf("✗ Unknown progress command: '%s'\n", args[0])
		fmt.Println("Available: update, history")
	}
}

func progressUpdate(args []string) {
	mangaID := parseFlag(args, "manga-id")
	chapterStr := parseFlag(args, "chapter")
	if mangaID == "" || chapterStr == "" {
		fmt.Println("Usage: mangahub progress update --manga-id <id> --chapter <number>")
		return
	}

	chapter, err := strconv.Atoi(chapterStr)
	if err != nil || chapter < 0 {
		fmt.Println("✗ Chapter must be a positive number")
		return
	}

	fmt.Println("Updating reading progress...")

	body := map[string]interface{}{
		"manga_id":        mangaID,
		"current_chapter": chapter,
		"status":          "reading",
	}
	resp, err := apiPut("/users/progress", body)
	if err != nil {
		fmt.Printf("✗ Progress update failed: %v\n", err)
		return
	}

	if !resp.Success {
		errMsg := resp.Error
		fmt.Printf("✗ Progress update failed: %s\n", errMsg)
		if errMsg == "manga not found in library" {
			fmt.Printf("  Add to library first: mangahub library add --manga-id %s --status reading\n", mangaID)
		}
		return
	}

	var progress struct {
		MangaID        string `json:"manga_id"`
		CurrentChapter int    `json:"current_chapter"`
		Status         string `json:"status"`
	}
	json.Unmarshal(resp.Data, &progress)

	fmt.Printf("✓ Progress updated successfully!\n")
	fmt.Printf("  Manga:   %s\n", progress.MangaID)
	fmt.Printf("  Chapter: %d\n", progress.CurrentChapter)
	fmt.Printf("  Status:  %s\n", progress.Status)
	fmt.Println("\nSync Status:")
	fmt.Println("  Local database: ✓ Updated")
	fmt.Println("  TCP sync:       Broadcasting to connected devices")
}

func progressHistory(args []string) {
	resp, err := apiGet("/users/library")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve progress: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var library struct {
		ReadingLists struct {
			Reading    []progressEntry `json:"reading"`
			Completed  []progressEntry `json:"completed"`
			PlanToRead []progressEntry `json:"plan_to_read"`
		} `json:"reading_lists"`
	}
	json.Unmarshal(resp.Data, &library)

	// Combine all entries
	var all []progressEntry
	all = append(all, library.ReadingLists.Reading...)
	all = append(all, library.ReadingLists.Completed...)
	all = append(all, library.ReadingLists.PlanToRead...)

	if len(all) == 0 {
		fmt.Println("No reading progress found.")
		return
	}

	fmt.Printf("Reading Progress History (%d entries):\n\n", len(all))
	headers := []string{"Manga", "Chapter", "Status", "Last Updated"}
	var rows [][]string
	for _, e := range all {
		rows = append(rows, []string{
			e.MangaID,
			fmt.Sprintf("%d", e.CurrentChapter),
			e.Status,
			e.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	printTable(headers, rows)
}
