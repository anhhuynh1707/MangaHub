package main

import (
	"encoding/json"
	"fmt"
)

func handleFriend(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub friend <add|accept|list|pending>")
		return
	}

	switch args[0] {
	case "add":
		friendAdd(args[1:])
	case "accept":
		friendAccept(args[1:])
	case "list":
		friendList(args[1:])
	case "pending":
		friendPending(args[1:])
	case "remove":
		friendRemove(args[1:])
	default:
		fmt.Printf("✗ Unknown friend command: '%s'\n", args[0])
		fmt.Println("Available: add, accept, list, pending, remove")
	}
}

func friendAdd(args []string) {
	friendID := parseFlag(args, "id")
	if friendID == "" {
		fmt.Println("Usage: mangahub friend add --id <user_id>")
		return
	}

	body := map[string]interface{}{
		"friend_id": friendID,
	}

	resp, err := apiPost("/friends/add", body)
	if err != nil {
		fmt.Printf("✗ Failed to send friend request: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Friend request sent to '%s'!\n", friendID)
}

func friendAccept(args []string) {
	friendID := parseFlag(args, "id")
	if friendID == "" {
		fmt.Println("Usage: mangahub friend accept --id <user_id>")
		return
	}

	resp, err := apiPost("/friends/"+friendID+"/accept", nil)
	if err != nil {
		fmt.Printf("✗ Failed to accept friend request: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Friend request from '%s' accepted!\n", friendID)
}

func friendList(args []string) {
	resp, err := apiGet("/users/friends")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve friends: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		Friends []struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
			Status   string `json:"status"`
		} `json:"friends"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("👥 Your Friends (Total: %d)\n\n", data.Total)

	if len(data.Friends) == 0 {
		fmt.Println("You don't have any friends added yet.")
		return
	}

	headers := []string{"User ID", "Username", "Status"}
	var rows [][]string
	for _, f := range data.Friends {
		rows = append(rows, []string{f.UserID, f.Username, f.Status})
	}
	printTable(headers, rows)
}

func friendPending(args []string) {
	resp, err := apiGet("/users/friends/pending")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve pending requests: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		PendingRequests []string `json:"pending_requests"`
		Count           int      `json:"count"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("📬 Pending Friend Requests (Total: %d)\n\n", data.Count)

	if data.Count == 0 {
		fmt.Println("No pending requests.")
		return
	}

	for _, id := range data.PendingRequests {
		fmt.Printf("- %s (Accept with: mangahub friend accept --id %s)\n", id, id)
	}
}

func friendRemove(args []string) {
	friendID := parseFlag(args, "id")
	if friendID == "" {
		fmt.Println("Usage: mangahub friend remove --id <user_id>")
		return
	}

	resp, err := apiDelete("/friends/" + friendID)
	if err != nil {
		fmt.Printf("✗ Failed to remove friend: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Friend '%s' has been removed.\n", friendID)
}
