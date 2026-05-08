package review

import (
	"fmt"

	"mangahub/pkg/models"
)

// Service handles business logic for reviews.
type Service struct {
	repo *Repository
}

// NewService creates a new review service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateReview creates a new review with validation.
func (s *Service) CreateReview(userID, mangaID string, rating int, text string) (*models.Review, error) {
	// Validate rating
	if rating < 1 || rating > 10 {
		return nil, fmt.Errorf("rating must be between 1 and 10")
	}

	// Check if user already reviewed this manga
	existing, err := s.repo.GetReviewByUserAndManga(userID, mangaID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("user already reviewed this manga")
	}

	return s.repo.CreateReview(userID, mangaID, rating, text)
}

// GetReviewByID retrieves a review by ID.
func (s *Service) GetReviewByID(reviewID string) (*models.Review, error) {
	return s.repo.GetReviewByID(reviewID)
}

// GetReviewsByManga retrieves reviews for a manga with pagination.
func (s *Service) GetReviewsByManga(mangaID string, limit, offset int) ([]models.Review, int, error) {
	return s.repo.GetReviewsByManga(mangaID, limit, offset)
}

// GetReviewsByUser retrieves reviews by a user.
func (s *Service) GetReviewsByUser(userID string, limit, offset int) ([]models.Review, int, error) {
	return s.repo.GetReviewsByUser(userID, limit, offset)
}

// UpdateReview updates an existing review.
func (s *Service) UpdateReview(userID, reviewID string, rating *int, text *string) error {
	// Get existing review to verify ownership
	existing, err := s.repo.GetReviewByID(reviewID)
	if err != nil {
		return err
	}

	if existing.UserID != userID {
		return fmt.Errorf("not authorized to update this review")
	}

	// Validate new rating if provided
	if rating != nil && (*rating < 1 || *rating > 10) {
		return fmt.Errorf("rating must be between 1 and 10")
	}

	return s.repo.UpdateReview(reviewID, rating, text)
}

// DeleteReview deletes a review (owner only).
func (s *Service) DeleteReview(userID, reviewID string) error {
	// Get review to verify ownership
	existing, err := s.repo.GetReviewByID(reviewID)
	if err != nil {
		return err
	}

	if existing.UserID != userID {
		return fmt.Errorf("not authorized to delete this review")
	}

	return s.repo.DeleteReview(reviewID)
}

// MarkHelpful marks a review as helpful.
func (s *Service) MarkHelpful(reviewID string) error {
	return s.repo.MarkHelpful(reviewID)
}

// GetMangaStats returns rating statistics for a manga.
func (s *Service) GetMangaStats(mangaID string) (map[string]interface{}, error) {
	avgRating, count, err := s.repo.GetAverageRating(mangaID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"manga_id":     mangaID,
		"avg_rating":   avgRating,
		"review_count": count,
	}, nil
}
