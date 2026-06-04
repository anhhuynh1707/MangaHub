package recommendation

import (
	"database/sql"
	"encoding/json"
	"log"

	"mangahub/pkg/models"
)

// Service loads reading data from SQLite and drives the RecommendationEngine.
type Service struct {
	db *sql.DB
}

// NewService creates a recommendation service backed by the given database.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// RecommendationResult is what the HTTP handler returns to the client.
type RecommendationResult struct {
	UserID        string               `json:"user_id"`
	Recommendations []EnrichedRec      `json:"recommendations"`
	ProfileStats  ProfileStats         `json:"profile_stats"`
}

// EnrichedRec combines a score with the full manga data.
type EnrichedRec struct {
	ScoredManga
	Manga *models.Manga `json:"manga,omitempty"`
}

// ProfileStats gives the caller insight into the user's profile used for recommendations.
type ProfileStats struct {
	TotalRead      int                `json:"total_read"`
	TotalCompleted int                `json:"total_completed"`
	TopGenres      []string           `json:"top_genres"`
	SimilarUsers   int                `json:"similar_users_found"`
}

// GetRecommendations builds a full recommendation result for the given user.
func (s *Service) GetRecommendations(userID string, limit int) (*RecommendationResult, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// ── 1. Load all user reading data ───────────────────────────────────────
	allProgress, err := s.loadAllProgress()
	if err != nil {
		return nil, err
	}

	// ── 2. Load all review ratings ───────────────────────────────────────────
	allRatings, err := s.loadAllRatings()
	if err != nil {
		return nil, err
	}

	// ── 3. Load all manga metadata ───────────────────────────────────────────
	allManga, err := s.loadAllManga()
	if err != nil {
		return nil, err
	}

	if len(allProgress[userID]) == 0 {
		log.Printf("Recommendation: user %s has no reading history", userID)
		return &RecommendationResult{
			UserID:          userID,
			Recommendations: []EnrichedRec{},
			ProfileStats:    ProfileStats{},
		}, nil
	}

	// ── 4. Build engine and compute ──────────────────────────────────────────
	engine := NewEngine()
	engine.LoadProfiles(allProgress, allRatings, allManga)
	engine.ComputeUserSimilarity(userID)
	engine.ComputeMangaSimilarity()

	scored := engine.Recommend(userID, limit)

	// ── 5. Enrich with full manga data ───────────────────────────────────────
	enriched := make([]EnrichedRec, 0, len(scored))
	for _, sc := range scored {
		m, ok := allManga[sc.MangaID]
		var mp *models.Manga
		if ok {
			mp = &m
		}
		enriched = append(enriched, EnrichedRec{ScoredManga: sc, Manga: mp})
	}

	// ── 6. Build profile stats ────────────────────────────────────────────────
	profile := engine.UserProfiles[userID]
	stats := ProfileStats{
		TotalRead:      len(profile.ReadManga),
		TotalCompleted: len(profile.CompletedManga),
		TopGenres:      topGenres(profile.GenreScores, 5),
		SimilarUsers:   countNonZero(engine.UserSimilarity),
	}

	return &RecommendationResult{
		UserID:          userID,
		Recommendations: enriched,
		ProfileStats:    stats,
	}, nil
}

// ─── DB loaders ──────────────────────────────────────────────────────────────

func (s *Service) loadAllProgress() (map[string][]models.UserProgress, error) {
	rows, err := s.db.Query(
		`SELECT user_id, manga_id, current_chapter, status, updated_at FROM user_progress`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]models.UserProgress)
	for rows.Next() {
		var p models.UserProgress
		if err := rows.Scan(&p.UserID, &p.MangaID, &p.CurrentChapter, &p.Status, &p.UpdatedAt); err != nil {
			continue
		}
		result[p.UserID] = append(result[p.UserID], p)
	}
	return result, rows.Err()
}

func (s *Service) loadAllRatings() (map[string]map[string]int, error) {
	rows, err := s.db.Query(`SELECT user_id, manga_id, rating FROM reviews`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]map[string]int)
	for rows.Next() {
		var userID, mangaID string
		var rating int
		if err := rows.Scan(&userID, &mangaID, &rating); err != nil {
			continue
		}
		if result[userID] == nil {
			result[userID] = make(map[string]int)
		}
		result[userID][mangaID] = rating
	}
	return result, rows.Err()
}

func (s *Service) loadAllManga() (map[string]models.Manga, error) {
	rows, err := s.db.Query(
		`SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]models.Manga)
	for rows.Next() {
		var m models.Manga
		var genresJSON string
		if err := rows.Scan(&m.ID, &m.Title, &m.Author, &genresJSON,
			&m.Status, &m.TotalChapters, &m.Description, &m.CoverURL); err != nil {
			continue
		}
		json.Unmarshal([]byte(genresJSON), &m.Genres)
		result[m.ID] = m
	}
	return result, rows.Err()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func topGenres(scores map[string]float64, n int) []string {
	type kv struct {
		k string
		v float64
	}
	pairs := make([]kv, 0, len(scores))
	for k, v := range scores {
		pairs = append(pairs, kv{k, v})
	}
	// sort descending
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].v > pairs[i].v {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	result := make([]string, len(pairs))
	for i, p := range pairs {
		result[i] = p.k
	}
	return result
}

func countNonZero(sim map[string]float64) int {
	count := 0
	for _, v := range sim {
		if v > 0 {
			count++
		}
	}
	return count
}
