package manga

import (
	"log"

	"mangahub/pkg/cache"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"
)

// Service handles manga business logic.
type Service struct {
	repo  *Repository
	cache *cache.RedisCache
}

// NewService creates a new manga service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetCache sets the Redis cache instance for this service.
func (s *Service) SetCache(c *cache.RedisCache) {
	s.cache = c
}

// invalidateMangaCaches removes all manga-related cached data.
// Called after any write operation that changes the manga dataset.
func (s *Service) invalidateMangaCaches() {
	if s.cache == nil {
		return
	}
	if err := s.cache.DeletePattern(cache.PrefixMangaSearch + "*"); err != nil {
		log.Printf("cache: failed to invalidate manga search keys: %v", err)
	}
	if err := s.cache.Delete(cache.PrefixMangaAll); err != nil {
		log.Printf("cache: failed to invalidate manga:all key: %v", err)
	}
	if err := s.cache.Delete(cache.PrefixMangaCount); err != nil {
		log.Printf("cache: failed to invalidate manga:count key: %v", err)
	}
}

// Create creates a new manga entry.
func (s *Service) Create(manga *models.Manga) error {
	existing, err := s.repo.FindByID(manga.ID)
	if err != nil {
		return err
	}
	if existing != nil {
		return utils.ErrConflict("manga with this ID already exists")
	}
	if err := s.repo.Create(manga); err != nil {
		return err
	}
	// Cache the newly created manga and invalidate list caches
	if s.cache != nil {
		_ = s.cache.Set(cache.MangaKey(manga.ID), manga, cache.MangaDetailTTL)
	}
	s.invalidateMangaCaches()
	return nil
}

// GetByID retrieves a manga by ID.
func (s *Service) GetByID(id string) (*models.Manga, error) {
	// Try cache first
	if s.cache != nil {
		var cached models.Manga
		if s.cache.Get(cache.MangaKey(id), &cached) {
			return &cached, nil
		}
	}

	manga, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if manga == nil {
		return nil, utils.ErrNotFound("manga not found")
	}

	// Populate cache
	if s.cache != nil {
		_ = s.cache.Set(cache.MangaKey(id), manga, cache.MangaDetailTTL)
	}
	return manga, nil
}

// Search retrieves manga with search and filters.
func (s *Service) Search(query *models.MangaSearchQuery) ([]models.Manga, int, error) {
	// Build cache key from query parameters
	if s.cache != nil {
		cacheKey := cache.MangaSearchKey(query.Search, query.Genre, query.Status, query.Page, query.Limit)
		var cached struct {
			Manga []models.Manga `json:"manga"`
			Total int            `json:"total"`
		}
		if s.cache.Get(cacheKey, &cached) {
			return cached.Manga, cached.Total, nil
		}
	}

	mangaList, total, err := s.repo.Search(query)
	if err != nil {
		return nil, 0, err
	}

	// Populate cache
	if s.cache != nil {
		cacheKey := cache.MangaSearchKey(query.Search, query.Genre, query.Status, query.Page, query.Limit)
		_ = s.cache.Set(cacheKey, struct {
			Manga []models.Manga `json:"manga"`
			Total int            `json:"total"`
		}{mangaList, total}, cache.MangaListTTL)
	}
	return mangaList, total, nil
}

// Update updates an existing manga.
func (s *Service) Update(manga *models.Manga) error {
	if err := s.repo.Update(manga); err != nil {
		return err
	}
	// Invalidate the specific manga and list caches
	if s.cache != nil {
		_ = s.cache.Delete(cache.MangaKey(manga.ID))
	}
	s.invalidateMangaCaches()
	return nil
}

// Delete removes a manga by ID.
func (s *Service) Delete(id string) error {
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	if s.cache != nil {
		_ = s.cache.Delete(cache.MangaKey(id))
	}
	s.invalidateMangaCaches()
	return nil
}

// GetCount returns the total number of manga.
func (s *Service) GetCount() (int, error) {
	if s.cache != nil {
		var cached int
		if s.cache.Get(cache.PrefixMangaCount, &cached) {
			return cached, nil
		}
	}

	count, err := s.repo.Count()
	if err != nil {
		return 0, err
	}

	if s.cache != nil {
		_ = s.cache.Set(cache.PrefixMangaCount, count, cache.MangaCountTTL)
	}
	return count, nil
}

// SearchByFilters performs advanced filtering using the SearchFilters struct.
func (s *Service) SearchByFilters(f *models.SearchFilters) ([]models.Manga, int, error) {
	return s.repo.SearchByFilters(f)
}

// BulkCreate creates multiple manga at once (used for seeding).
func (s *Service) BulkCreate(mangaList []models.Manga) (int, error) {
	inserted, err := s.repo.BulkCreate(mangaList)
	if err != nil {
		return inserted, err
	}
	s.invalidateMangaCaches()
	return inserted, nil
}

// GetAll retrieves all manga.
func (s *Service) GetAll() ([]models.Manga, error) {
	if s.cache != nil {
		var cached []models.Manga
		if s.cache.Get(cache.PrefixMangaAll, &cached) {
			return cached, nil
		}
	}

	mangaList, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		_ = s.cache.Set(cache.PrefixMangaAll, mangaList, cache.MangaListTTL)
	}
	return mangaList, nil
}
