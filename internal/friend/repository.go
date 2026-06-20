package friend

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"mangahub/pkg/utils"
)

// Repository handles all friendship database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new friend repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// AddFriend sends a directional friend request: requesterID -> recipientID.
// The request is stored with user_id = requester and friend_id = recipient so
// that only the recipient sees it in their pending list.
func (r *Repository) AddFriend(requesterID, recipientID string) error {
	// Reject if a relationship already exists in EITHER direction.
	var status string
	err := r.db.QueryRow(
		`SELECT status FROM friendships
		 WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
		requesterID, recipientID, recipientID, requesterID,
	).Scan(&status)

	if err == nil {
		switch status {
		case "accepted":
			return utils.ErrConflict("already friends")
		case "blocked":
			return utils.ErrForbidden("unable to send friend request")
		default:
			return utils.ErrConflict("a friend request is already pending")
		}
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check friendship: %w", err)
	}

	_, err = r.db.Exec(
		`INSERT INTO friendships (user_id, friend_id, status, created_at)
		 VALUES (?, ?, 'pending', ?)`,
		requesterID, recipientID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to add friend: %w", err)
	}

	return nil
}

// AcceptFriend accepts a pending request that requesterID sent to userID.
func (r *Repository) AcceptFriend(userID, requesterID string) error {
	result, err := r.db.Exec(
		`UPDATE friendships SET status = 'accepted'
		 WHERE user_id = ? AND friend_id = ? AND status = 'pending'`,
		requesterID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to accept friend request: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return utils.ErrNotFound("no pending friend request found")
	}

	return nil
}

// RemoveFriend removes a friendship/request in either direction (also used to
// decline an incoming request or cancel one you sent).
func (r *Repository) RemoveFriend(userID, friendID string) error {
	result, err := r.db.Exec(
		`DELETE FROM friendships
		 WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
		userID, friendID, friendID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to remove friend: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return utils.ErrNotFound("friendship not found")
	}

	return nil
}

// BlockFriend blocks a user (directional: userID blocks friendID).
func (r *Repository) BlockFriend(userID, friendID string) error {
	// Clear any existing relationship in either direction first.
	if _, err := r.db.Exec(
		`DELETE FROM friendships
		 WHERE (user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)`,
		userID, friendID, friendID, userID,
	); err != nil {
		return err
	}

	_, err := r.db.Exec(
		`INSERT INTO friendships (user_id, friend_id, status, created_at)
		 VALUES (?, ?, 'blocked', ?)`,
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

// IsFriend checks if two users are accepted friends (either direction).
func (r *Repository) IsFriend(userID, friendID string) (bool, error) {
	var status string
	err := r.db.QueryRow(
		`SELECT status FROM friendships
		 WHERE ((user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?))
		   AND status = 'accepted'`,
		userID, friendID, friendID, userID,
	).Scan(&status)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check friendship: %w", err)
	}

	return true, nil
}

// IsBlocked checks if userID has blocked blockedID (directional).
func (r *Repository) IsBlocked(userID, blockedID string) (bool, error) {
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
