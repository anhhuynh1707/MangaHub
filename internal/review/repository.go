package review

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"mangahub/pkg/models"
)

// Repository handles all review database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new review repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateReview adds a new review to the database.
func (r *Repository) CreateReview(userID, mangaID string, rating int, text string) (*models.Review, error) {
	id := fmt.Sprintf("rev_%d_%s_%s", time.Now().UnixNano(), userID, mangaID)
	now := time.Now()

	_, err := r.db.Exec(
		`INSERT INTO reviews (id, user_id, manga_id, rating, text, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, userID, mangaID, rating, text, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}

	return &models.Review{
		ID:        id,
		UserID:    userID,
		MangaID:   mangaID,
		Rating:    rating,
		Text:      text,
		Helpful:   0,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetReviewByID retrieves a review by its ID.
func (r *Repository) GetReviewByID(reviewID string) (*models.Review, error) {
	var review models.Review
	err := r.db.QueryRow(
		`SELECT r.id, r.user_id, COALESCE(u.username,''), r.manga_id, r.rating, r.text, r.helpful, r.created_at, r.updated_at
		 FROM reviews r LEFT JOIN users u ON r.user_id = u.id
		 WHERE r.id = ?`,
		reviewID,
	).Scan(&review.ID, &review.UserID, &review.Username, &review.MangaID, &review.Rating, &review.Text,
		&review.Helpful, &review.CreatedAt, &review.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("review not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch review: %w", err)
	}

	return &review, nil
}

// GetReviewByUserAndManga retrieves a user's review for a specific manga.
func (r *Repository) GetReviewByUserAndManga(userID, mangaID string) (*models.Review, error) {
	var review models.Review
	err := r.db.QueryRow(
		`SELECT r.id, r.user_id, COALESCE(u.username,''), r.manga_id, r.rating, r.text, r.helpful, r.created_at, r.updated_at
		 FROM reviews r LEFT JOIN users u ON r.user_id = u.id
		 WHERE r.user_id = ? AND r.manga_id = ?`,
		userID, mangaID,
	).Scan(&review.ID, &review.UserID, &review.Username, &review.MangaID, &review.Rating, &review.Text,
		&review.Helpful, &review.CreatedAt, &review.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil // No review exists
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch review: %w", err)
	}

	return &review, nil
}

// GetReviewsByManga retrieves all reviews for a manga.
func (r *Repository) GetReviewsByManga(mangaID string, limit, offset int) ([]models.Review, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM reviews WHERE manga_id = ?`, mangaID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count reviews: %w", err)
	}

	// Get paginated results
	rows, err := r.db.Query(
		`SELECT r.id, r.user_id, COALESCE(u.username,''), r.manga_id, r.rating, r.text, r.helpful, r.created_at, r.updated_at
		 FROM reviews r LEFT JOIN users u ON r.user_id = u.id
		 WHERE r.manga_id = ?
		 ORDER BY r.created_at DESC LIMIT ? OFFSET ?`,
		mangaID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch reviews: %w", err)
	}
	defer rows.Close()

	var reviews []models.Review
	for rows.Next() {
		var review models.Review
		err := rows.Scan(&review.ID, &review.UserID, &review.Username, &review.MangaID, &review.Rating, &review.Text,
			&review.Helpful, &review.CreatedAt, &review.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning review: %v", err)
			continue
		}
		reviews = append(reviews, review)
	}

	return reviews, total, rows.Err()
}

// GetReviewsByUser retrieves all reviews written by a user.
func (r *Repository) GetReviewsByUser(userID string, limit, offset int) ([]models.Review, int, error) {
	// Get total count
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM reviews WHERE user_id = ?`, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count reviews: %w", err)
	}

	// Get paginated results
	rows, err := r.db.Query(
		`SELECT r.id, r.user_id, COALESCE(u.username,''), r.manga_id, r.rating, r.text, r.helpful, r.created_at, r.updated_at
		 FROM reviews r LEFT JOIN users u ON r.user_id = u.id
		 WHERE r.user_id = ?
		 ORDER BY r.created_at DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to fetch reviews: %w", err)
	}
	defer rows.Close()

	var reviews []models.Review
	for rows.Next() {
		var review models.Review
		err := rows.Scan(&review.ID, &review.UserID, &review.Username, &review.MangaID, &review.Rating, &review.Text,
			&review.Helpful, &review.CreatedAt, &review.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning review: %v", err)
			continue
		}
		reviews = append(reviews, review)
	}

	return reviews, total, rows.Err()
}

// UpdateReview updates an existing review.
func (r *Repository) UpdateReview(reviewID string, rating *int, text *string) error {
	now := time.Now()

	if rating != nil {
		_, err := r.db.Exec(
			`UPDATE reviews SET rating = ?, updated_at = ? WHERE id = ?`,
			*rating, now, reviewID,
		)
		if err != nil {
			return fmt.Errorf("failed to update review rating: %w", err)
		}
	}

	if text != nil {
		_, err := r.db.Exec(
			`UPDATE reviews SET text = ?, updated_at = ? WHERE id = ?`,
			*text, now, reviewID,
		)
		if err != nil {
			return fmt.Errorf("failed to update review text: %w", err)
		}
	}

	return nil
}

// DeleteReview removes a review from the database.
func (r *Repository) DeleteReview(reviewID string) error {
	result, err := r.db.Exec(`DELETE FROM reviews WHERE id = ?`, reviewID)
	if err != nil {
		return fmt.Errorf("failed to delete review: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("review not found")
	}

	return nil
}

// MarkHelpful increments the helpful count for a review.
func (r *Repository) MarkHelpful(reviewID string) error {
	_, err := r.db.Exec(
		`UPDATE reviews SET helpful = helpful + 1 WHERE id = ?`,
		reviewID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark review helpful: %w", err)
	}

	return nil
}

// GetAverageRating returns the average rating for a manga.
func (r *Repository) GetAverageRating(mangaID string) (float64, int, error) {
	var avgRating sql.NullFloat64
	var count int

	err := r.db.QueryRow(
		`SELECT AVG(rating), COUNT(*) FROM reviews WHERE manga_id = ?`,
		mangaID,
	).Scan(&avgRating, &count)

	if err != nil {
		return 0, 0, fmt.Errorf("failed to get average rating: %w", err)
	}

	if !avgRating.Valid {
		return 0, 0, nil
	}

	return avgRating.Float64, count, nil
}
