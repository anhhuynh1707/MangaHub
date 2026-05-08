package main

import (
	"encoding/json"
	"fmt"
)

func handleFeed(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub feed <view|mine|post>")
		return
	}

	switch args[0] {
	case "view":
		feedView(args[1:])
	case "mine":
		feedMine(args[1:])
	case "post":
		feedPost(args[1:])
	default:
		fmt.Printf("✗ Unknown feed command: '%s'\n", args[0])
		fmt.Println("Available: view, mine, post")
	}
}

func feedView(args []string) {
	resp, err := apiGet("/feed/activities")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve activity feed: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	printFeed(resp.Data)
}

func feedMine(args []string) {
	cfg := loadConfig()
	if cfg.UserID == "" {
		// fallback to view /users/activities if there's no user id locally
		fmt.Println("No local User ID. Fetching general feed instead...")
		feedView(args)
		return
	}

	resp, err := apiGet("/users/" + cfg.UserID + "/activities")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve your activities: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	printFeed(resp.Data)
}

func printFeed(dataBytes []byte) {
	var data struct {
		Activities []struct {
			ID          string `json:"id"`
			Username    string `json:"username"`
			Type        string `json:"type"`
			MangaTitle  string `json:"manga_title"`
			Message     string `json:"message"`
			Description string `json:"description"`
			CreatedAt   string `json:"created_at"`
			Timestamp   string `json:"timestamp"`
		} `json:"activities"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(dataBytes, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("📺 Activity Feed (Total: %d)\n\n", data.Total)

	if len(data.Activities) == 0 {
		fmt.Println("No recent activities.")
		return
	}

	for _, act := range data.Activities {
		msg := act.Message
		if msg == "" {
			msg = act.Description
		}
		
		timeStr := act.CreatedAt
		if timeStr == "" {
			timeStr = act.Timestamp
		}
		
		fmt.Printf("[%s] %s\n", timeStr, msg)
	}
}

func feedPost(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub feed post <message>")
		return
	}

	// Join all arguments as the message
	message := args[0]
	for i := 1; i < len(args); i++ {
		message += " " + args[i]
	}

	reqBody := map[string]string{
		"message": message,
	}

	resp, err := apiPost("/feed/activities", reqBody)
	if err != nil {
		fmt.Printf("✗ Failed to post activity: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✅ Successfully posted to your activity feed: \"%s\"\n", message)
}
