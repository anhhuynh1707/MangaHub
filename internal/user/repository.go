package user

import (
	"database/sql"
	"time"

	"mangahub/pkg/models"
)

// Repository handles database operations for users using raw SQL.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new user repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new user into the database.
func (r *Repository) Create(user *models.User) error {
	_, err := r.db.Exec(
		"INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)",
		user.ID, user.Username, user.PasswordHash, user.CreatedAt,
	)
	return err
}

// FindByUsername retrieves a user by username.
func (r *Repository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID retrieves a user by ID.
func (r *Repository) FindByID(id string) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// --- User Progress / Library Repository Methods ---

// AddToLibrary adds a manga to the user's library.
func (r *Repository) AddToLibrary(progress *models.UserProgress) error {
	_, err := r.db.Exec(
		`INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		progress.UserID, progress.MangaID, progress.CurrentChapter, progress.Status, progress.UpdatedAt,
	)
	return err
}

// GetLibraryEntry retrieves a specific library entry.
func (r *Repository) GetLibraryEntry(userID, mangaID string) (*models.UserProgress, error) {
	var progress models.UserProgress
	err := r.db.QueryRow(
		"SELECT user_id, manga_id, current_chapter, status, updated_at FROM user_progress WHERE user_id = ? AND manga_id = ?",
		userID, mangaID,
	).Scan(&progress.UserID, &progress.MangaID, &progress.CurrentChapter, &progress.Status, &progress.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

// GetUserLibrary retrieves all library entries for a user.
func (r *Repository) GetUserLibrary(userID string) ([]models.UserProgress, error) {
	rows, err := r.db.Query(
		"SELECT user_id, manga_id, current_chapter, status, updated_at FROM user_progress WHERE user_id = ? ORDER BY updated_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.UserProgress
	for rows.Next() {
		var entry models.UserProgress
		if err := rows.Scan(&entry.UserID, &entry.MangaID, &entry.CurrentChapter, &entry.Status, &entry.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetUserReadingLists retrieves a user's library organized by status.
func (r *Repository) GetUserReadingLists(userID string) (*models.ReadingList, error) {
	entries, err := r.GetUserLibrary(userID)
	if err != nil {
		return nil, err
	}

	lists := &models.ReadingList{
		Reading:    []models.UserProgress{},
		Completed:  []models.UserProgress{},
		PlanToRead: []models.UserProgress{},
	}

	for _, e := range entries {
		switch e.Status {
		case "reading":
			lists.Reading = append(lists.Reading, e)
		case "completed":
			lists.Completed = append(lists.Completed, e)
		case "plan_to_read":
			lists.PlanToRead = append(lists.PlanToRead, e)
		}
	}

	return lists, nil
}

// UpdateProgress updates a user's reading progress for a manga.
func (r *Repository) UpdateProgress(progress *models.UserProgress) error {
	result, err := r.db.Exec(
		`UPDATE user_progress SET current_chapter = ?, status = ?, updated_at = ?
		 WHERE user_id = ? AND manga_id = ?`,
		progress.CurrentChapter, progress.Status, time.Now(),
		progress.UserID, progress.MangaID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RemoveFromLibrary deletes a library entry.
func (r *Repository) RemoveFromLibrary(userID, mangaID string) error {
	result, err := r.db.Exec(
		"DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?",
		userID, mangaID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdatePasswordHash updates a user's password hash.
func (r *Repository) UpdatePasswordHash(userID, newHash string) error {
	result, err := r.db.Exec(
		"UPDATE users SET password_hash = ? WHERE id = ?",
		newHash, userID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

