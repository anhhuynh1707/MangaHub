package activity

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"mangahub/pkg/models"
)

// Repository handles all activity database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new activity repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateActivity logs a new user activity.
func (r *Repository) CreateActivity(userID, activityType, mangaID, reviewID, friendID, message string) (*models.Activity, error) {
	id := fmt.Sprintf("act_%d", time.Now().UnixNano())
	now := time.Now()

	var dbMangaID, dbReviewID, dbFriendID interface{}
	if mangaID != "" {
		dbMangaID = mangaID
	}
	if reviewID != "" {
		dbReviewID = reviewID
	}
	if friendID != "" {
		dbFriendID = friendID
	}

	_, err := r.db.Exec(
		`INSERT INTO activities (id, user_id, type, manga_id, review_id, friend_id, message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, activityType, dbMangaID, dbReviewID, dbFriendID, message, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create activity: %w", err)
	}

	return &models.Activity{
		ID:        id,
		UserID:    userID,
		Type:      activityType,
		MangaID:   mangaID,
		ReviewID:  reviewID,
		FriendID:  friendID,
		Message:   message,
		CreatedAt: now,
	}, nil
}

// GetUserActivities retrieves activities from a specific user.
func (r *Repository) GetUserActivities(userID string, limit, offset int) ([]models.Activity, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, type, manga_id, review_id, friend_id, message, created_at
		 FROM activities WHERE user_id = ?
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch activities: %w", err)
	}
	defer rows.Close()

	var activities []models.Activity
	for rows.Next() {
		var activity models.Activity
		var mangaID, reviewID, friendID sql.NullString

		err := rows.Scan(&activity.ID, &activity.UserID, &activity.Type, &mangaID,
			&reviewID, &friendID, &activity.Message, &activity.CreatedAt)
		if err != nil {
			log.Printf("Error scanning activity: %v", err)
			continue
		}

		if mangaID.Valid {
			activity.MangaID = mangaID.String
		}
		if reviewID.Valid {
			activity.ReviewID = reviewID.String
		}
		if friendID.Valid {
			activity.FriendID = friendID.String
		}

		activities = append(activities, activity)
	}

	return activities, rows.Err()
}

// GetAllActivities retrieves activities from all users globally.
func (r *Repository) GetAllActivities(limit, offset int) ([]models.Activity, error) {
	rows, err := r.db.Query(
		`SELECT id, user_id, type, manga_id, review_id, friend_id, message, created_at
		 FROM activities
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch global activities: %w", err)
	}
	defer rows.Close()

	var activities []models.Activity
	for rows.Next() {
		var activity models.Activity
		var mangaID, reviewID, friendID sql.NullString

		err := rows.Scan(&activity.ID, &activity.UserID, &activity.Type, &mangaID,
			&reviewID, &friendID, &activity.Message, &activity.CreatedAt)
		if err != nil {
			log.Printf("Error scanning activity: %v", err)
			continue
		}

		if mangaID.Valid {
			activity.MangaID = mangaID.String
		}
		if reviewID.Valid {
			activity.ReviewID = reviewID.String
		}
		if friendID.Valid {
			activity.FriendID = friendID.String
		}

		activities = append(activities, activity)
	}

	return activities, rows.Err()
}

// GetFriendsActivities retrieves activities from a user's friends.
// This is used for the activity feed.
func (r *Repository) GetFriendsActivities(userID string, limit, offset int) ([]models.Activity, error) {
	// Get friends first
	friendRows, err := r.db.Query(
		`SELECT CASE 
			WHEN user_id = ? THEN friend_id 
			ELSE user_id 
		 END as friend_id
		 FROM friendships 
		 WHERE (user_id = ? OR friend_id = ?) AND status = 'accepted'`,
		userID, userID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch friends: %w", err)
	}
	defer friendRows.Close()

	var friendIDs []string
	for friendRows.Next() {
		var friendID string
		if err := friendRows.Scan(&friendID); err != nil {
			continue
		}
		friendIDs = append(friendIDs, friendID)
	}

	if len(friendIDs) == 0 {
		return []models.Activity{}, nil
	}

	// Build query with friend IDs
	query := `SELECT id, user_id, type, manga_id, review_id, friend_id, message, created_at
	         FROM activities WHERE user_id IN (`
	args := make([]interface{}, len(friendIDs))
	for i, id := range friendIDs {
		args[i] = id
		if i > 0 {
			query += ","
		}
		query += "?"
	}
	query += `) ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch friends activities: %w", err)
	}
	defer rows.Close()

	var activities []models.Activity
	for rows.Next() {
		var activity models.Activity
		var mangaID, reviewID, friendID sql.NullString

		err := rows.Scan(&activity.ID, &activity.UserID, &activity.Type, &mangaID,
			&reviewID, &friendID, &activity.Message, &activity.CreatedAt)
		if err != nil {
			log.Printf("Error scanning activity: %v", err)
			continue
		}

		if mangaID.Valid {
			activity.MangaID = mangaID.String
		}
		if reviewID.Valid {
			activity.ReviewID = reviewID.String
		}
		if friendID.Valid {
			activity.FriendID = friendID.String
		}

		activities = append(activities, activity)
	}

	return activities, rows.Err()
}

// GetActivityByID retrieves a specific activity by ID.
func (r *Repository) GetActivityByID(activityID string) (*models.Activity, error) {
	var activity models.Activity
	var mangaID, reviewID, friendID sql.NullString

	err := r.db.QueryRow(
		`SELECT id, user_id, type, manga_id, review_id, friend_id, message, created_at
		 FROM activities WHERE id = ?`,
		activityID,
	).Scan(&activity.ID, &activity.UserID, &activity.Type, &mangaID,
		&reviewID, &friendID, &activity.Message, &activity.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("activity not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch activity: %w", err)
	}

	if mangaID.Valid {
		activity.MangaID = mangaID.String
	}
	if reviewID.Valid {
		activity.ReviewID = reviewID.String
	}
	if friendID.Valid {
		activity.FriendID = friendID.String
	}

	return &activity, nil
}

// DeleteActivity removes an activity record.
func (r *Repository) DeleteActivity(activityID string) error {
	result, err := r.db.Exec(`DELETE FROM activities WHERE id = ?`, activityID)
	if err != nil {
		return fmt.Errorf("failed to delete activity: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("activity not found")
	}

	return nil
}

// DeleteActivitiesByUser removes all activities belonging to a user and returns
// how many were deleted.
func (r *Repository) DeleteActivitiesByUser(userID string) (int64, error) {
	result, err := r.db.Exec(`DELETE FROM activities WHERE user_id = ?`, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to clear activities: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
