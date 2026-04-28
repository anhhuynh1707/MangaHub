package models

import "time"

// Manga represents a manga series in the database.
// Uses string ID (slug) as primary key per spec requirements.
type Manga struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Author        string   `json:"author"`
	Genres        []string `json:"genres"`
	Status        string   `json:"status"`
	TotalChapters int      `json:"total_chapters"`
	Description   string   `json:"description"`
	CoverURL      string   `json:"cover_url"`
}

// User represents a registered user.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never exposed in JSON
	CreatedAt    time.Time `json:"created_at"`
}

// UserProgress represents a user's reading progress for a specific manga.
type UserProgress struct {
	UserID         string    `json:"user_id"`
	MangaID        string    `json:"manga_id"`
	CurrentChapter int       `json:"current_chapter"`
	Status         string    `json:"status"` // reading, completed, plan_to_read, on_hold, dropped
	UpdatedAt      time.Time `json:"updated_at"`
}

// ReadingList represents a user's categorized reading lists.
type ReadingList struct {
	Reading    []UserProgress `json:"reading"`
	Completed  []UserProgress `json:"completed"`
	PlanToRead []UserProgress `json:"plan_to_read"`
}

// UserData represents the full user data structure for JSON storage.
type UserData struct {
	UserID       string      `json:"user_id"`
	Username     string      `json:"username"`
	ReadingLists ReadingList `json:"reading_lists"`
}

// --- Request / Response DTOs ---

// RegisterRequest is the payload for user registration.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

// LoginRequest is the payload for user login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is returned after successful login.
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// AddToLibraryRequest is the payload for adding manga to user's library.
type AddToLibraryRequest struct {
	MangaID string `json:"manga_id" binding:"required"`
	Status  string `json:"status" binding:"omitempty,oneof=reading completed plan_to_read on_hold dropped"`
}

// UpdateProgressRequest is the payload for updating reading progress.
type UpdateProgressRequest struct {
	MangaID        string `json:"manga_id" binding:"required"`
	CurrentChapter int    `json:"current_chapter" binding:"min=0"`
	Status         string `json:"status" binding:"omitempty,oneof=reading completed plan_to_read on_hold dropped"`
}

// MangaSearchQuery holds query parameters for searching manga.
type MangaSearchQuery struct {
	Search string `form:"search"`
	Genre  string `form:"genre"`
	Status string `form:"status"`
	Page   int    `form:"page,default=1"`
	Limit  int    `form:"limit,default=20"`
}

// ScrapedQuote represents a quote scraped from practice sites.
type ScrapedQuote struct {
	Text   string   `json:"text"`
	Author string   `json:"author"`
	Tags   []string `json:"tags"`
}
