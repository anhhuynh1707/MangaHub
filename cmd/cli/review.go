package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func handleReview(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub review <add|list|mine|stats|update|delete|helpful>")
		return
	}

	switch args[0] {
	case "add":
		reviewAdd(args[1:])
	case "list":
		reviewList(args[1:])
	case "mine":
		reviewMine(args[1:])
	case "stats":
		reviewStats(args[1:])
	case "update":
		reviewUpdate(args[1:])
	case "delete":
		reviewDelete(args[1:])
	case "helpful":
		reviewHelpful(args[1:])
	default:
		fmt.Printf("✗ Unknown review command: '%s'\n", args[0])
		fmt.Println("Available: add, list, mine, stats, update, delete, helpful")
	}
}

func reviewAdd(args []string) {
	mangaID := parseFlag(args, "manga-id")
	ratingStr := parseFlag(args, "rating")
	text := parseFlag(args, "text")

	if mangaID == "" || ratingStr == "" {
		fmt.Println("Usage: mangahub review add --manga-id <id> --rating <1-10> [--text <review_text>]")
		return
	}

	rating, err := strconv.Atoi(ratingStr)
	if err != nil || rating < 1 || rating > 10 {
		fmt.Println("✗ Rating must be an integer between 1 and 10")
		return
	}

	body := map[string]interface{}{
		"rating": rating,
		"text":   text,
	}

	resp, err := apiPost("/manga/"+mangaID+"/reviews", body)
	if err != nil {
		fmt.Printf("✗ Failed to add review: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Printf("✓ Review added for manga '%s'!\n", mangaID)
}

func reviewList(args []string) {
	mangaID := parseFlag(args, "manga-id")
	if mangaID == "" {
		fmt.Println("Usage: mangahub review list --manga-id <id>")
		return
	}

	resp, err := apiGet("/manga/" + mangaID + "/reviews")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve reviews: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		Reviews []struct {
			ID        string `json:"id"`
			Username  string `json:"username"`
			Rating    int    `json:"rating"`
			Text      string `json:"text"`
			Helpful   int    `json:"helpful"`
			CreatedAt string `json:"created_at"`
		} `json:"reviews"`
		Total         int     `json:"total"`
		AverageRating float64 `json:"average_rating"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("⭐ Reviews for '%s' (Total: %d, Avg: %.1f)\n\n", mangaID, data.Total, data.AverageRating)

	if len(data.Reviews) == 0 {
		fmt.Println("No reviews found.")
		return
	}

	for _, rev := range data.Reviews {
		fmt.Printf("[%s] ID: %s | %s - Rating: %d/10 (Helpful: %d)\n", rev.CreatedAt, rev.ID, rev.Username, rev.Rating, rev.Helpful)
		if rev.Text != "" {
			fmt.Printf("  \"%s\"\n", rev.Text)
		}
		fmt.Println()
	}
}

func reviewMine(args []string) {
	resp, err := apiGet("/users/reviews")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve your reviews: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		Reviews []struct {
			MangaID   string `json:"manga_id"`
			Rating    int    `json:"rating"`
			Text      string `json:"text"`
			CreatedAt string `json:"created_at"`
		} `json:"reviews"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("📚 Your Reviews (%d)\n\n", len(data.Reviews))

	if len(data.Reviews) == 0 {
		fmt.Println("You haven't written any reviews.")
		return
	}

	for _, rev := range data.Reviews {
		// Note: The /users/reviews API might need to return the ID as well to be manageable from `mine` view.
		// Assuming we just print what we have.
		fmt.Printf("Manga: %s | Rating: %d/10 | %s\n", rev.MangaID, rev.Rating, rev.CreatedAt)
		if rev.Text != "" {
			fmt.Printf("  \"%s\"\n", rev.Text)
		}
		fmt.Println()
	}
}

func reviewStats(args []string) {
	mangaID := parseFlag(args, "manga-id")
	if mangaID == "" {
		fmt.Println("Usage: mangahub review stats --manga-id <id>")
		return
	}

	resp, err := apiGet("/manga/" + mangaID + "/rating-stats")
	if err != nil {
		fmt.Printf("✗ Failed to retrieve rating stats: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var data struct {
		MangaID       string  `json:"manga_id"`
		AverageRating float64 `json:"avg_rating"`
		ReviewCount   int     `json:"review_count"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		fmt.Printf("✗ Failed to parse response: %v\n", err)
		return
	}

	fmt.Printf("📊 Rating Stats for '%s'\n", data.MangaID)
	fmt.Printf("Average Rating: %.1f/10\n", data.AverageRating)
	fmt.Printf("Total Reviews: %d\n", data.ReviewCount)
}

func reviewUpdate(args []string) {
	reviewID := parseFlag(args, "review-id")
	ratingStr := parseFlag(args, "rating")
	text := parseFlag(args, "text")

	if reviewID == "" || ratingStr == "" {
		fmt.Println("Usage: mangahub review update --review-id <id> --rating <1-10> [--text <review_text>]")
		return
	}

	rating, err := strconv.Atoi(ratingStr)
	if err != nil || rating < 1 || rating > 10 {
		fmt.Println("✗ Rating must be an integer between 1 and 10")
		return
	}

	body := map[string]interface{}{
		"rating": rating,
		"text":   text,
	}

	resp, err := apiPut("/reviews/"+reviewID, body)
	if err != nil {
		fmt.Printf("✗ Failed to update review: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Println("✓ Review updated successfully!")
}

func reviewDelete(args []string) {
	reviewID := parseFlag(args, "review-id")
	if reviewID == "" {
		fmt.Println("Usage: mangahub review delete --review-id <id>")
		return
	}

	resp, err := apiDelete("/reviews/" + reviewID)
	if err != nil {
		fmt.Printf("✗ Failed to delete review: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Println("✓ Review deleted successfully!")
}

func reviewHelpful(args []string) {
	reviewID := parseFlag(args, "review-id")
	if reviewID == "" {
		fmt.Println("Usage: mangahub review helpful --review-id <id>")
		return
	}

	body := map[string]interface{}{
		"review_id": reviewID,
	}

	resp, err := apiPost("/reviews/"+reviewID+"/helpful", body)
	if err != nil {
		fmt.Printf("✗ Failed to mark review helpful: %v\n", err)
		return
	}

	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	fmt.Println("✓ Review marked as helpful!")
}
