package manga

import (
	"errors"

	"mangahub/pkg/models"
)

// Service handles manga business logic.
type Service struct {
	repo *Repository
}

// NewService creates a new manga service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new manga entry.
func (s *Service) Create(manga *models.Manga) error {
	existing, err := s.repo.FindByID(manga.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("manga with this ID already exists")
	}
	return s.repo.Create(manga)
}

// GetByID retrieves a manga by ID.
func (s *Service) GetByID(id string) (*models.Manga, error) {
	manga, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if manga == nil {
		return nil, errors.New("manga not found")
	}
	return manga, nil
}

// Search retrieves manga with search and filters.
func (s *Service) Search(query *models.MangaSearchQuery) ([]models.Manga, int, error) {
	return s.repo.Search(query)
}

// Update updates an existing manga.
func (s *Service) Update(manga *models.Manga) error {
	return s.repo.Update(manga)
}

// Delete removes a manga by ID.
func (s *Service) Delete(id string) error {
	return s.repo.Delete(id)
}

// GetCount returns the total number of manga.
func (s *Service) GetCount() (int, error) {
	return s.repo.Count()
}

// BulkCreate creates multiple manga at once (used for seeding).
func (s *Service) BulkCreate(mangaList []models.Manga) (int, error) {
	return s.repo.BulkCreate(mangaList)
}

// GetAll retrieves all manga.
func (s *Service) GetAll() ([]models.Manga, error) {
	return s.repo.GetAll()
}
