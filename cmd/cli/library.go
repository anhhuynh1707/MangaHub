package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func handleLibrary(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub library <add|list|remove|update>")
		return
	}

	switch args[0] {
	case "add":
		libraryAdd(args[1:])
	case "list":
		libraryList(args[1:])
	case "remove":
		libraryRemove(args[1:])
	case "update":
		libraryUpdate(args[1:])
	default:
		fmt.Printf("✗ Unknown library command: '%s'\n", args[0])
		fmt.Println("Available: add, list, remove, update")
	}
}

func libraryAdd(args []string) {
	mangaID := parseFlag(args, "manga-id")
	status := parseFlag(args, "status")
	if mangaID == "" {
		fmt.Println("Usage: mangahub library add --manga-id <id> [--status <status>]")
		fmt.Println("  Status options: reading, completed, plan-to-read, on-hold, dropped")
		return
	}
	if status == "" {
		status = "plan_to_read"
	}
	// Convert CLI-style to API-style (plan-to-read → plan_to_read)
	status = strings.ReplaceAll(status, "-", "_")

	body := map[string]string{"manga_id": mangaID, "status": status}
	resp, err := apiPost("/users/library", body)
	if err != nil {
		fmt.Printf("✗ Failed to add to library: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		if strings.Contains(resp.Error, "already") {
			fmt.Printf("  Update status: mangahub library update --manga-id %s --status reading\n", mangaID)
		}
		return
	}

	fmt.Printf("✓ Added to library!\n")
	fmt.Printf("  Manga: %s\n", mangaID)
	fmt.Printf("  Status: %s\n", status)
	fmt.Println("\nNext steps:")
	fmt.Printf("  Update progress: mangahub progress update --manga-id %s --chapter 1\n", mangaID)
	fmt.Println("  View library:    mangahub library list")
}

func libraryList(args []string) {
	statusFilter := parseFlag(args, "status")

	resp, err := apiGet("/users/library")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve library: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var library struct {
		UserID       string `json:"user_id"`
		Username     string `json:"username"`
		ReadingLists struct {
			Reading    []progressEntry `json:"reading"`
			Completed  []progressEntry `json:"completed"`
			PlanToRead []progressEntry `json:"plan_to_read"`
		} `json:"reading_lists"`
	}
	json.Unmarshal(resp.Data, &library)

	totalEntries := len(library.ReadingLists.Reading) +
		len(library.ReadingLists.Completed) +
		len(library.ReadingLists.PlanToRead)

	if totalEntries == 0 {
		fmt.Printf("Your library is empty.\n\n")
		fmt.Println("Get started by searching and adding manga:")
		fmt.Println("  mangahub manga search \"your favorite series\"")
		fmt.Println("  mangahub library add --manga-id <id> --status reading")
		return
	}

	cfg := loadConfig()
	fmt.Printf("📚 %s's Manga Library (%d entries)\n\n", cfg.Username, totalEntries)

	headers := []string{"ID", "Manga ID", "Chapter", "Status", "Updated"}

	if statusFilter == "" || statusFilter == "reading" {
		if len(library.ReadingLists.Reading) > 0 {
			fmt.Printf("Currently Reading (%d):\n", len(library.ReadingLists.Reading))
			printProgressTable(headers, library.ReadingLists.Reading)
			fmt.Println()
		}
	}

	if statusFilter == "" || statusFilter == "completed" {
		if len(library.ReadingLists.Completed) > 0 {
			fmt.Printf("Completed (%d):\n", len(library.ReadingLists.Completed))
			printProgressTable(headers, library.ReadingLists.Completed)
			fmt.Println()
		}
	}

	if statusFilter == "" || strings.ReplaceAll(statusFilter, "-", "_") == "plan_to_read" {
		if len(library.ReadingLists.PlanToRead) > 0 {
			fmt.Printf("Plan to Read (%d):\n", len(library.ReadingLists.PlanToRead))
			printProgressTable(headers, library.ReadingLists.PlanToRead)
			fmt.Println()
		}
	}

	fmt.Println("Use --status <status> to filter by specific status")
}

type progressEntry struct {
	UserID         string    `json:"user_id"`
	MangaID        string    `json:"manga_id"`
	CurrentChapter int       `json:"current_chapter"`
	Status         string    `json:"status"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func printProgressTable(headers []string, entries []progressEntry) {
	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{
			e.UserID,
			e.MangaID,
			fmt.Sprintf("%d", e.CurrentChapter),
			e.Status,
			e.UpdatedAt.Format("2006-01-02 15:04"),
		})
	}
	printTable(headers, rows)
}

func libraryRemove(args []string) {
	mangaID := parseFlag(args, "manga-id")
	if mangaID == "" {
		fmt.Println("Usage: mangahub library remove --manga-id <id>")
		return
	}

	resp, err := apiDelete("/users/library/" + mangaID)
	if err != nil {
		fmt.Printf("✗ Failed to remove: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Removed '%s' from your library\n", mangaID)
}

func libraryUpdate(args []string) {
	mangaID := parseFlag(args, "manga-id")
	status := parseFlag(args, "status")
	if mangaID == "" || status == "" {
		fmt.Println("Usage: mangahub library update --manga-id <id> --status <new-status>")
		return
	}

	status = strings.ReplaceAll(status, "-", "_")
	body := map[string]interface{}{"manga_id": mangaID, "current_chapter": 0, "status": status}
	resp, err := apiPut("/users/progress", body)
	if err != nil {
		fmt.Printf("✗ Failed to update: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Updated '%s' status to '%s'\n", mangaID, status)
}
