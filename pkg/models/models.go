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

// ChangePasswordRequest is the payload for changing a user's password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=100"`
}

// ProgressUpdate represents a real-time progress update broadcast over TCP.
// This is the spec-required struct for the TCP Progress Sync Server.
type ProgressUpdate struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Timestamp int64  `json:"timestamp"`
}

// ScrapedQuote represents a quote scraped from practice sites.
type ScrapedQuote struct {
	Text   string   `json:"text"`
	Author string   `json:"author"`
	Tags   []string `json:"tags"`
}

// --- Social & Community Features ---

// Review represents a user's review and rating for a manga.
type Review struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"` // Denormalized for display
	MangaID   string    `json:"manga_id"`
	Rating    int       `json:"rating"` // 1-10
	Text      string    `json:"text"`
	Helpful   int       `json:"helpful"` // Helpful votes count
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateReviewRequest is the payload for creating a review.
type CreateReviewRequest struct {
	MangaID string `json:"manga_id" binding:"required"`
	Rating  int    `json:"rating" binding:"required,min=1,max=10"`
	Text    string `json:"text" binding:"required,max=5000"`
}

// UpdateReviewRequest is the payload for updating a review.
type UpdateReviewRequest struct {
	Rating int    `json:"rating" binding:"omitempty,min=1,max=10"`
	Text   string `json:"text" binding:"omitempty,max=5000"`
}

// Friendship represents a relationship between two users.
type Friendship struct {
	UserID    string    `json:"user_id"`
	FriendID  string    `json:"friend_id"`
	Status    string    `json:"status"` // "pending", "accepted", "blocked"
	CreatedAt time.Time `json:"created_at"`
}

// FriendInfo represents friend information with their reading activity.
type FriendInfo struct {
	UserID        string         `json:"user_id"`
	Username      string         `json:"username"`
	Status        string         `json:"status"` // online, away, offline
	LastActive    time.Time      `json:"last_active"`
	RecentReads   []UserProgress `json:"recent_reads"`
	RecentReviews []Review       `json:"recent_reviews"`
}

// SharedReadingList represents a reading list shared with friends.
type SharedReadingList struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`   // True = public, False = shared with specific friends
	MangaList   []string  `json:"manga_list"`  // Array of manga IDs
	SharedWith  []string  `json:"shared_with"` // Array of user IDs (if not public)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateSharedListRequest is the payload for creating a shared list.
type CreateSharedListRequest struct {
	Title       string   `json:"title" binding:"required,max=100"`
	Description string   `json:"description" binding:"max=500"`
	IsPublic    bool     `json:"is_public"`
	MangaList   []string `json:"manga_list" binding:"required"`
	SharedWith  []string `json:"shared_with"` // Friend user IDs
}

// Activity represents a user activity for the activity feed.
type Activity struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Type       string    `json:"type"` // "started_manga", "completed_manga", "wrote_review", "added_friend"
	MangaID    string    `json:"manga_id,omitempty"`
	MangaTitle string    `json:"manga_title,omitempty"`
	ReviewID   string    `json:"review_id,omitempty"`
	FriendID   string    `json:"friend_id,omitempty"`
	Message    string    `json:"message"` // Human-readable activity description
	CreatedAt  time.Time `json:"created_at"`
}

// ActivityFeed represents the feed of activities from friends.
type ActivityFeed struct {
	Activities []Activity `json:"activities"`
	Total      int        `json:"total"`
}

// CreateActivityRequest is the payload for creating a custom activity post.
type CreateActivityRequest struct {
	Message string `json:"message" binding:"required,max=500"`
}
