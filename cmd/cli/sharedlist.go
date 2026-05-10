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
	case "view":
		sharedListView(args[1:])
	case "add-manga":
		sharedListAddManga(args[1:])
	case "remove-manga":
		sharedListRemoveManga(args[1:])
	case "subscribe":
		sharedListSubscribe(args[1:])
	case "subscribed":
		sharedListSubscribed(args[1:])
	case "unsubscribe":
		sharedListUnsubscribe(args[1:])
	default:
		fmt.Printf("✗ Unknown sharedlist command: '%s'\n", args[0])
		fmt.Println("Available: create, mine, public, view, add-manga, remove-manga, subscribe, subscribed (view subscribed lists), unsubscribe")
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

func sharedListView(args []string) {
	listID := parseFlag(args, "id")
	if listID == "" {
		fmt.Println("Usage: mangahub sharedlist view --id <list_id>")
		return
	}

	resp, err := apiGet("/reading-lists/" + listID)
	if err != nil {
		fmt.Printf("✗ Failed to retrieve list: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Println(string(resp.Data))
		return
	}

	fmt.Printf("📖 Reading List Details\n")
	for k, v := range data {
		if k == "manga_list" || k == "manga_ids" {
			fmt.Printf("  %s:\n", k)
			if mList, ok := v.([]interface{}); ok {
				for _, m := range mList {
					if mStr, ok2 := m.(string); ok2 {
						fmt.Printf("    - %s\n", mStr)
					} else {
						// If it's an object
						mBytes, _ := json.Marshal(m)
						fmt.Printf("    - %s\n", string(mBytes))
					}
				}
			}
		} else {
			fmt.Printf("  %s: %v\n", k, v)
		}
	}
}

func sharedListAddManga(args []string) {
	listID := parseFlag(args, "id")
	mangaID := parseFlag(args, "manga-id")

	if listID == "" || mangaID == "" {
		fmt.Println("Usage: mangahub sharedlist add-manga --id <list_id> --manga-id <manga_id>")
		return
	}

	body := map[string]interface{}{
		"manga_id": mangaID,
	}

	resp, err := apiPost("/reading-lists/"+listID+"/manga", body)
	if err != nil {
		fmt.Printf("✗ Failed to add manga: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}
	fmt.Println("✓ Manga added to list successfully!")
}

func sharedListRemoveManga(args []string) {
	listID := parseFlag(args, "id")
	mangaID := parseFlag(args, "manga-id")

	if listID == "" || mangaID == "" {
		fmt.Println("Usage: mangahub sharedlist remove-manga --id <list_id> --manga-id <manga_id>")
		return
	}

	resp, err := apiDelete("/reading-lists/" + listID + "/manga/" + mangaID)
	if err != nil {
		fmt.Printf("✗ Failed to remove manga: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}
	fmt.Println("✓ Manga removed from list successfully!")
}

func sharedListSubscribe(args []string) {
	listID := parseFlag(args, "id")
	if listID == "" {
		fmt.Println("Usage: mangahub sharedlist subscribe --id <list_id>")
		return
	}

	resp, err := apiPost("/reading-lists/"+listID+"/subscribe", nil)
	if err != nil {
		fmt.Printf("✗ Failed to subscribe: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}
	fmt.Println("✓ Subscribed to list successfully!")
}

func sharedListSubscribed(args []string) {
	resp, err := apiGet("/reading-lists/subscribed")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve subscribed lists: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		Lists []struct {
			ID        string `json:"id"`
			OwnerName string `json:"owner_name"`
			Title     string `json:"title"`
			Name      string `json:"name"`
		} `json:"lists"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Println(string(resp.Data))
		return
	}

	fmt.Printf("📚 Subscribed Lists (%d)\n\n", len(data.Lists))
	if len(data.Lists) == 0 {
		fmt.Println("You haven't subscribed to any lists.")
		return
	}

	for _, l := range data.Lists {
		name := l.Title
		if name == "" {
			name = l.Name
		}
		fmt.Printf("- %s by %s (ID: %s)\n", name, l.OwnerName, l.ID)
	}
}

func sharedListUnsubscribe(args []string) {
	listID := parseFlag(args, "id")
	if listID == "" {
		fmt.Println("Usage: mangahub sharedlist unsubscribe --id <list_id>")
		return
	}

	resp, err := apiDelete("/reading-lists/" + listID + "/subscribe")
	if err != nil {
		fmt.Printf("✗ Failed to unsubscribe: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}
	fmt.Println("✓ Unsubscribed from list successfully!")
}
