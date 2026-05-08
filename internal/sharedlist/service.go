package sharedlist

import (
	"fmt"

	"mangahub/pkg/models"
)

// Service handles business logic for shared reading lists.
type Service struct {
	repo *Repository
}

// NewService creates a new shared list service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateList creates a new shared reading list.
func (s *Service) CreateList(ownerID, title, description string, isPublic bool, mangaList []string, sharedWith []string) (*models.SharedReadingList, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	if len(mangaList) == 0 {
		return nil, fmt.Errorf("manga list cannot be empty")
	}

	return s.repo.CreateList(ownerID, title, description, isPublic, mangaList, sharedWith)
}

// GetListByID retrieves a shared list by ID.
func (s *Service) GetListByID(listID string) (*models.SharedReadingList, error) {
	return s.repo.GetListByID(listID)
}

// GetListsByOwner retrieves all lists created by a user.
func (s *Service) GetListsByOwner(ownerID string) ([]models.SharedReadingList, error) {
	return s.repo.GetListsByOwner(ownerID)
}

// GetPublicLists retrieves all public shared lists.
func (s *Service) GetPublicLists(limit, offset int) ([]models.SharedReadingList, int, error) {
	return s.repo.GetPublicLists(limit, offset)
}

// UpdateList updates a shared list (owner only).
func (s *Service) UpdateList(userID, listID, title, description string, isPublic bool, mangaList, sharedWith []string) error {
	// Verify ownership
	existing, err := s.repo.GetListByID(listID)
	if err != nil {
		return err
	}

	if existing.OwnerID != userID {
		return fmt.Errorf("not authorized to update this list")
	}

	if title != "" {
		existing.Title = title
	}
	if description != "" {
		existing.Description = description
	}
	if len(mangaList) > 0 {
		existing.MangaList = mangaList
	}

	return s.repo.UpdateList(listID, existing.Title, existing.Description, isPublic, existing.MangaList, sharedWith)
}

// DeleteList removes a shared list (owner only).
func (s *Service) DeleteList(userID, listID string) error {
	// Verify ownership
	existing, err := s.repo.GetListByID(listID)
	if err != nil {
		return err
	}

	if existing.OwnerID != userID {
		return fmt.Errorf("not authorized to delete this list")
	}

	return s.repo.DeleteList(listID)
}

// CanAccessList checks if a user can access a shared list.
func (s *Service) CanAccessList(userID, listID string) (bool, error) {
	list, err := s.repo.GetListByID(listID)
	if err != nil {
		return false, err
	}

	// Owner can access their own list
	if list.OwnerID == userID {
		return true, nil
	}

	// Public lists are accessible to everyone
	if list.IsPublic {
		return true, nil
	}

	// Check if user is in shared_with
	for _, sharedID := range list.SharedWith {
		if sharedID == userID {
			return true, nil
		}
	}

	return false, nil
}
