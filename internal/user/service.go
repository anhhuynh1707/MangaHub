package user

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"mangahub/internal/auth"
	"mangahub/pkg/cache"
	"mangahub/pkg/models"

	"golang.org/x/crypto/bcrypt"
)

// Service handles user business logic.
type Service struct {
	repo  *Repository
	cache *cache.RedisCache
}

// NewService creates a new user service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetCache sets the Redis cache instance for this service.
func (s *Service) SetCache(c *cache.RedisCache) {
	s.cache = c
}

// invalidateUserCaches removes cached data for a specific user.
func (s *Service) invalidateUserCaches(userID string) {
	if s.cache == nil {
		return
	}
	if err := s.cache.Delete(cache.UserLibraryKey(userID)); err != nil {
		log.Printf("cache: failed to invalidate user library %s: %v", userID, err)
	}
	if err := s.cache.Delete(cache.UserProfileKey(userID)); err != nil {
		log.Printf("cache: failed to invalidate user profile %s: %v", userID, err)
	}
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
	// Try cache first
	if s.cache != nil {
		var cached models.User
		if s.cache.Get(cache.UserProfileKey(userID), &cached) {
			return &cached, nil
		}
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.Set(cache.UserProfileKey(userID), user, cache.UserProfileTTL)
	}
	return user, nil
}

// SearchUsers returns users whose username matches the query (excluding the
// caller). Returns an empty slice for blank queries.
func (s *Service) SearchUsers(query, excludeID string) ([]models.User, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []models.User{}, nil
	}
	return s.repo.SearchByUsername(query, excludeID, 8)
}

// --- Library Service Methods ---

// GetProgressEntry returns a single library entry (or nil if not present).
// Used to detect status transitions for activity logging.
func (s *Service) GetProgressEntry(userID, mangaID string) (*models.UserProgress, error) {
	return s.repo.GetLibraryEntry(userID, mangaID)
}

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

	// Invalidate user library cache
	s.invalidateUserCaches(userID)

	return progress, nil
}

// GetLibrary returns the user's library as categorized reading lists.
func (s *Service) GetLibrary(userID string) (*models.UserData, error) {
	// Try cache first
	if s.cache != nil {
		var cached models.UserData
		if s.cache.Get(cache.UserLibraryKey(userID), &cached) {
			return &cached, nil
		}
	}

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

	userData := &models.UserData{
		UserID:       userID,
		Username:     user.Username,
		ReadingLists: *lists,
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.Set(cache.UserLibraryKey(userID), userData, cache.UserLibraryTTL)
	}

	return userData, nil
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

	// Invalidate user library cache
	s.invalidateUserCaches(userID)

	return s.repo.GetLibraryEntry(userID, req.MangaID)
}

// RemoveFromLibrary removes a manga from the user's library.
func (s *Service) RemoveFromLibrary(userID, mangaID string) error {
	if err := s.repo.RemoveFromLibrary(userID, mangaID); err != nil {
		return err
	}
	// Invalidate user library cache
	s.invalidateUserCaches(userID)
	return nil
}

// ChangePassword validates the old password and updates to a new one.
func (s *Service) ChangePassword(userID string, req *models.ChangePasswordRequest) error {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return errors.New("incorrect current password")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash new password")
	}

	return s.repo.UpdatePasswordHash(userID, string(hashedPassword))
}

// generateUserID creates a slug-style user ID from a username.
func generateUserID(username string) string {
	id := strings.ToLower(username)
	id = strings.ReplaceAll(id, " ", "-")
	return fmt.Sprintf("user-%s", id)
}


