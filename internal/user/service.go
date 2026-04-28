package user

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"mangahub/internal/auth"
	"mangahub/pkg/models"

	"golang.org/x/crypto/bcrypt"
)

// Service handles user business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new user service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Register creates a new user account.
func (s *Service) Register(req *models.RegisterRequest) (*models.User, error) {
	// Check if username already exists
	existing, err := s.repo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("username already taken")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	// Generate user ID from username (slug-style)
	userID := generateUserID(req.Username)

	user := &models.User{
		ID:           userID,
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
	}

	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns a JWT token.
func (s *Service) Login(req *models.LoginRequest) (*models.LoginResponse, error) {
	user, err := s.repo.FindByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid username or password")
	}

	// Compare passwords
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &models.LoginResponse{
		Token: token,
		User:  *user,
	}, nil
}

// GetProfile retrieves a user's profile by ID.
func (s *Service) GetProfile(userID string) (*models.User, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// --- Library Service Methods ---

// AddToLibrary adds a manga to the user's library.
func (s *Service) AddToLibrary(userID string, req *models.AddToLibraryRequest) (*models.UserProgress, error) {
	// Check if already in library
	existing, err := s.repo.GetLibraryEntry(userID, req.MangaID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("manga already in library")
	}

	status := req.Status
	if status == "" {
		status = "plan_to_read"
	}

	progress := &models.UserProgress{
		UserID:         userID,
		MangaID:        req.MangaID,
		CurrentChapter: 0,
		Status:         status,
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.AddToLibrary(progress); err != nil {
		return nil, err
	}

	return progress, nil
}

// GetLibrary returns the user's library as categorized reading lists.
func (s *Service) GetLibrary(userID string) (*models.UserData, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	lists, err := s.repo.GetUserReadingLists(userID)
	if err != nil {
		return nil, err
	}

	return &models.UserData{
		UserID:       userID,
		Username:     user.Username,
		ReadingLists: *lists,
	}, nil
}

// UpdateProgress updates reading progress for a manga.
func (s *Service) UpdateProgress(userID string, req *models.UpdateProgressRequest) (*models.UserProgress, error) {
	existing, err := s.repo.GetLibraryEntry(userID, req.MangaID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("manga not found in library")
	}

	status := req.Status
	if status == "" {
		status = existing.Status
	}

	progress := &models.UserProgress{
		UserID:         userID,
		MangaID:        req.MangaID,
		CurrentChapter: req.CurrentChapter,
		Status:         status,
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.UpdateProgress(progress); err != nil {
		return nil, err
	}

	return s.repo.GetLibraryEntry(userID, req.MangaID)
}

// RemoveFromLibrary removes a manga from the user's library.
func (s *Service) RemoveFromLibrary(userID, mangaID string) error {
	return s.repo.RemoveFromLibrary(userID, mangaID)
}

// generateUserID creates a slug-style user ID from a username.
func generateUserID(username string) string {
	id := strings.ToLower(username)
	id = strings.ReplaceAll(id, " ", "-")
	return fmt.Sprintf("user-%s", id)
}
