package friend

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Repository handles all friendship database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new friend repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// AddFriend sends a friend request or accepts a pending one.
func (r *Repository) AddFriend(userID, friendID string) error {
	// Ensure userID < friendID for consistent ordering
	if userID > friendID {
		userID, friendID = friendID, userID
	}

	// Check if friendship already exists
	var status string
	err := r.db.QueryRow(
		`SELECT status FROM friendships WHERE user_id = ? AND friend_id = ?`,
		userID, friendID,
	).Scan(&status)

	if err == nil {
		// Friendship exists, update status to accepted
		if status != "accepted" {
			_, err := r.db.Exec(
				`UPDATE friendships SET status = 'accepted' WHERE user_id = ? AND friend_id = ?`,
				userID, friendID,
			)
			return err
		}
		return fmt.Errorf("already friends")
	}

	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check friendship: %w", err)
	}

	// Create new friendship request
	_, err = r.db.Exec(
		`INSERT INTO friendships (user_id, friend_id, status, created_at)
		 VALUES (?, ?, 'pending', ?)`,
		userID, friendID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to add friend: %w", err)
	}

	return nil
}

// AcceptFriend accepts a pending friend request.
func (r *Repository) AcceptFriend(userID, friendID string) error {
	// Ensure consistent ordering
	if userID > friendID {
		userID, friendID = friendID, userID
	}

	result, err := r.db.Exec(
		`UPDATE friendships SET status = 'accepted' WHERE user_id = ? AND friend_id = ? AND status = 'pending'`,
		userID, friendID,
	)
	if err != nil {
		return fmt.Errorf("failed to accept friend request: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no pending friend request found")
	}

	return nil
}

// RemoveFriend removes a friend relationship.
func (r *Repository) RemoveFriend(userID, friendID string) error {
	// Ensure consistent ordering
	if userID > friendID {
		userID, friendID = friendID, userID
	}

	result, err := r.db.Exec(
		`DELETE FROM friendships WHERE user_id = ? AND friend_id = ?`,
		userID, friendID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove friend: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("friendship not found")
	}

	return nil
}

// BlockFriend blocks a user.
func (r *Repository) BlockFriend(userID, friendID string) error {
	// Ensure consistent ordering
	if userID > friendID {
		userID, friendID = friendID, userID
	}

	_, err := r.db.Exec(
		`INSERT INTO friendships (user_id, friend_id, status, created_at)
		 VALUES (?, ?, 'blocked', ?)
		 ON CONFLICT(user_id, friend_id) DO UPDATE SET status = 'blocked'`,
		userID, friendID, time.Now(),
	)
	return err
}

// GetFriends retrieves all accepted friends for a user.
func (r *Repository) GetFriends(userID string) ([]string, error) {
	rows, err := r.db.Query(
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
	defer rows.Close()

	var friends []string
	for rows.Next() {
		var friendID string
		if err := rows.Scan(&friendID); err != nil {
			log.Printf("Error scanning friend: %v", err)
			continue
		}
		friends = append(friends, friendID)
	}

	return friends, rows.Err()
}

// GetPendingRequests retrieves pending friend requests for a user.
func (r *Repository) GetPendingRequests(userID string) ([]string, error) {
	rows, err := r.db.Query(
		`SELECT user_id FROM friendships 
		 WHERE friend_id = ? AND status = 'pending'`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending requests: %w", err)
	}
	defer rows.Close()

	var requesters []string
	for rows.Next() {
		var requesterID string
		if err := rows.Scan(&requesterID); err != nil {
			log.Printf("Error scanning requester: %v", err)
			continue
		}
		requesters = append(requesters, requesterID)
	}

	return requesters, rows.Err()
}

// IsFriend checks if two users are friends.
func (r *Repository) IsFriend(userID, friendID string) (bool, error) {
	// Ensure consistent ordering
	if userID > friendID {
		userID, friendID = friendID, userID
	}

	var status string
	err := r.db.QueryRow(
		`SELECT status FROM friendships WHERE user_id = ? AND friend_id = ? AND status = 'accepted'`,
		userID, friendID,
	).Scan(&status)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check friendship: %w", err)
	}

	return true, nil
}

// IsBlocked checks if a user is blocked.
func (r *Repository) IsBlocked(userID, blockedID string) (bool, error) {
	// Ensure consistent ordering
	if userID > blockedID {
		userID, blockedID = blockedID, userID
	}

	var status string
	err := r.db.QueryRow(
		`SELECT status FROM friendships WHERE user_id = ? AND friend_id = ? AND status = 'blocked'`,
		userID, blockedID,
	).Scan(&status)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check block status: %w", err)
	}

	return true, nil
}
