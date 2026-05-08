package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func handleSharedList(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub sharedlist <create|mine|public>")
		return
	}

	switch args[0] {
	case "create":
		sharedListCreate(args[1:])
	case "mine":
		sharedListMine(args[1:])
	case "public":
		sharedListPublic(args[1:])
	default:
		fmt.Printf("✗ Unknown sharedlist command: '%s'\n", args[0])
		fmt.Println("Available: create, mine, public")
	}
}

func sharedListCreate(args []string) {
	name := parseFlag(args, "name")
	mangaIDsStr := parseFlag(args, "manga-ids")
	isPublic := hasFlag(args, "public")

	if name == "" || mangaIDsStr == "" {
		fmt.Println("Usage: mangahub sharedlist create --name <name> --manga-ids <id1,id2> [--public]")
		return
	}

	mangaIDs := []string{}
	for _, id := range strings.Split(mangaIDsStr, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			mangaIDs = append(mangaIDs, id)
		}
	}

	body := map[string]interface{}{
		"name":      name,
		"manga_ids": mangaIDs,
		"is_public": isPublic,
	}

	resp, err := apiPost("/reading-lists/create", body)
	if err != nil {
		fmt.Printf("✗ Failed to create shared list: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Shared list '%s' created successfully!\n", name)
}

func sharedListMine(args []string) {
	resp, err := apiGet("/reading-lists/mine")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve your lists: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	// Assuming a structure, customize based on the API response
	var data struct {
		Lists []struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			IsPublic bool     `json:"is_public"`
			MangaIDs []string `json:"manga_ids"`
		} `json:"lists"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		// Just print raw if unmarshal fails or is not matched
		fmt.Println(string(resp.Data))
		return
	}

	fmt.Printf("📚 Your Reading Lists (%d)\n\n", len(data.Lists))

	if len(data.Lists) == 0 {
		fmt.Println("You haven't created any shared lists.")
		return
	}

	for _, l := range data.Lists {
		pubStr := "Private"
		if l.IsPublic {
			pubStr = "Public"
		}
		fmt.Printf("- %s (ID: %s) [%s]\n", l.Name, l.ID, pubStr)
		fmt.Printf("  Manga: %s\n", strings.Join(l.MangaIDs, ", "))
	}
}

func sharedListPublic(args []string) {
	resp, err := apiGet("/reading-lists/public")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve public lists: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		Lists []struct {
			ID        string   `json:"id"`
			OwnerID   string   `json:"owner_id"`
			OwnerName string   `json:"owner_name"`
			Title     string   `json:"title"`
			Name      string   `json:"name"`
			MangaList []string `json:"manga_list"`
			MangaIDs  []string `json:"manga_ids"`
		} `json:"lists"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("🌍 Public Reading Lists (Total: %d)\n\n", data.Total)

	if len(data.Lists) == 0 {
		fmt.Println("No public lists available.")
		return
	}

	for _, l := range data.Lists {
		name := l.Title
		if name == "" {
			name = l.Name
		}
		owner := l.OwnerName
		if owner == "" {
			owner = l.OwnerID
		}
		mangas := l.MangaList
		if len(mangas) == 0 {
			mangas = l.MangaIDs
		}
		
		fmt.Printf("- %s by %s (ID: %s)\n", name, owner, l.ID)
		fmt.Printf("  Manga: %s\n", strings.Join(mangas, ", "))
	}
}
