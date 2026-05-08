package sharedlist

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"mangahub/pkg/models"
)

// Repository handles all shared reading list database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new shared list repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateList creates a new shared reading list.
func (r *Repository) CreateList(ownerID, title, description string, isPublic bool, mangaList []string, sharedWith []string) (*models.SharedReadingList, error) {
	id := fmt.Sprintf("list_%d", time.Now().UnixNano())
	now := time.Now()

	// Convert arrays to JSON
	mangaJSON, _ := json.Marshal(mangaList)
	sharedJSON, _ := json.Marshal(sharedWith)

	_, err := r.db.Exec(
		`INSERT INTO shared_reading_lists (id, owner_id, title, description, is_public, manga_list, shared_with, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, ownerID, title, description, isPublic, string(mangaJSON), string(sharedJSON), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create shared list: %w", err)
	}

	return &models.SharedReadingList{
		ID:          id,
		OwnerID:     ownerID,
		Title:       title,
		Description: description,
		IsPublic:    isPublic,
		MangaList:   mangaList,
		SharedWith:  sharedWith,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetListByID retrieves a shared list by ID.
func (r *Repository) GetListByID(listID string) (*models.SharedReadingList, error) {
	var list models.SharedReadingList
	var mangaJSON, sharedJSON string

	err := r.db.QueryRow(
		`SELECT id, owner_id, title, description, is_public, manga_list, shared_with, created_at, updated_at
		 FROM shared_reading_lists WHERE id = ?`,
		listID,
	).Scan(&list.ID, &list.OwnerID, &list.Title, &list.Description, &list.IsPublic,
		&mangaJSON, &sharedJSON, &list.CreatedAt, &list.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("shared list not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch shared list: %w", err)
	}

	json.Unmarshal([]byte(mangaJSON), &list.MangaList)
	json.Unmarshal([]byte(sharedJSON), &list.SharedWith)

	return &list, nil
}

// GetListsByOwner retrieves all lists created by a user.
func (r *Repository) GetListsByOwner(ownerID string) ([]models.SharedReadingList, error) {
	rows, err := r.db.Query(
		`SELECT id, owner_id, title, description, is_public, manga_list, shared_with, created_at, updated_at
		 FROM shared_reading_lists WHERE owner_id = ?
		 ORDER BY created_at DESC`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch lists: %w", err)
	}
	defer rows.Close()

	var lists []models.SharedReadingList
	for rows.Next() {
		var list models.SharedReadingList
		var mangaJSON, sharedJSON string

		err := rows.Scan(&list.ID, &list.OwnerID, &list.Title, &list.Description, &list.IsPublic,
			&mangaJSON, &sharedJSON, &list.CreatedAt, &list.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning list: %v", err)
			continue
		}

		json.Unmarshal([]byte(mangaJSON), &list.MangaList)
		json.Unmarshal([]byte(sharedJSON), &list.SharedWith)

		lists = append(lists, list)
	}

	return lists, rows.Err()
}

// GetPublicLists retrieves all public shared lists.
func (r *Repository) GetPublicLists(limit, offset int) ([]models.SharedReadingList, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM shared_reading_lists WHERE is_public = 1`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count lists: %w", err)
	}

	rows, err := r.db.Query(
		`SELECT id, owner_id, title, description, is_public, manga_list, shared_with, created_at, updated_at
		 FROM shared_reading_lists WHERE is_public = 1
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch public lists: %w", err)
	}
	defer rows.Close()

	var lists []models.SharedReadingList
	for rows.Next() {
		var list models.SharedReadingList
		var mangaJSON, sharedJSON string

		err := rows.Scan(&list.ID, &list.OwnerID, &list.Title, &list.Description, &list.IsPublic,
			&mangaJSON, &sharedJSON, &list.CreatedAt, &list.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning list: %v", err)
			continue
		}

		json.Unmarshal([]byte(mangaJSON), &list.MangaList)
		json.Unmarshal([]byte(sharedJSON), &list.SharedWith)

		lists = append(lists, list)
	}

	return lists, total, rows.Err()
}

// UpdateList updates an existing shared list.
func (r *Repository) UpdateList(listID, title, description string, isPublic bool, mangaList, sharedWith []string) error {
	mangaJSON, _ := json.Marshal(mangaList)
	sharedJSON, _ := json.Marshal(sharedWith)

	_, err := r.db.Exec(
		`UPDATE shared_reading_lists 
		 SET title = ?, description = ?, is_public = ?, manga_list = ?, shared_with = ?, updated_at = ?
		 WHERE id = ?`,
		title, description, isPublic, string(mangaJSON), string(sharedJSON), time.Now(), listID,
	)
	if err != nil {
		return fmt.Errorf("failed to update shared list: %w", err)
	}

	return nil
}

// DeleteList removes a shared list.
func (r *Repository) DeleteList(listID string) error {
	result, err := r.db.Exec(`DELETE FROM shared_reading_lists WHERE id = ?`, listID)
	if err != nil {
		return fmt.Errorf("failed to delete shared list: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("shared list not found")
	}

	return nil
}
