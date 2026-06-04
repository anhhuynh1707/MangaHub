package recommendation

import (
	"math"
	"sort"

	"mangahub/pkg/models"
)

// RecommendationEngine implements basic collaborative filtering based on user reading patterns.
// Spec-required struct.
type RecommendationEngine struct {
	// UserSimilarity holds Jaccard similarity scores to other users for a target user.
	// Key: other_user_id, Value: similarity score (0.0 – 1.0)
	UserSimilarity map[string]float64

	// MangaSimilarity holds the most similar manga for each manga ID (co-occurrence based).
	// Key: manga_id, Value: list of similar manga IDs (ordered by similarity desc)
	MangaSimilarity map[string][]string

	// UserProfiles holds reading profiles for all users, keyed by user_id.
	UserProfiles map[string]models.UserProfile
}

// NewEngine creates an empty RecommendationEngine.
func NewEngine() *RecommendationEngine {
	return &RecommendationEngine{
		UserSimilarity:  make(map[string]float64),
		MangaSimilarity: make(map[string][]string),
		UserProfiles:    make(map[string]models.UserProfile),
	}
}

// ─── Profile Building ───────────────────────────────────────────────────────

// LoadProfiles populates UserProfiles from a map of user-library entries.
// allProgress: map[userID] -> list of UserProgress records
// allRatings:  map[userID] -> map[mangaID] -> rating (from reviews)
// allManga:    map[mangaID] -> Manga (for genre data)
func (e *RecommendationEngine) LoadProfiles(
	allProgress map[string][]models.UserProgress,
	allRatings map[string]map[string]int,
	allManga map[string]models.Manga,
) {
	for userID, progressList := range allProgress {
		profile := models.UserProfile{
			UserID:         userID,
			ReadManga:      make([]string, 0),
			CompletedManga: make([]string, 0),
			Ratings:        make(map[string]int),
			GenreScores:    make(map[string]float64),
		}

		for _, p := range progressList {
			profile.ReadManga = append(profile.ReadManga, p.MangaID)
			if p.Status == "completed" {
				profile.CompletedManga = append(profile.CompletedManga, p.MangaID)
			}

			// Accumulate genre scores from read manga
			if m, ok := allManga[p.MangaID]; ok {
				weight := 1.0
				if p.Status == "completed" {
					weight = 2.0 // completed manga count more toward genre preference
				}
				for _, g := range m.Genres {
					profile.GenreScores[g] += weight
				}
			}
		}

		// Merge review ratings into profile
		if ratings, ok := allRatings[userID]; ok {
			for mangaID, rating := range ratings {
				profile.Ratings[mangaID] = rating
			}
		}

		e.UserProfiles[userID] = profile
	}
}

// ─── User-User Similarity ────────────────────────────────────────────────────

// ComputeUserSimilarity fills UserSimilarity for targetUserID vs all other users.
// Uses Jaccard similarity on the set of read manga.
func (e *RecommendationEngine) ComputeUserSimilarity(targetUserID string) {
	e.UserSimilarity = make(map[string]float64)

	target, ok := e.UserProfiles[targetUserID]
	if !ok {
		return
	}

	targetSet := toSet(target.ReadManga)

	for userID, profile := range e.UserProfiles {
		if userID == targetUserID {
			continue
		}
		otherSet := toSet(profile.ReadManga)
		e.UserSimilarity[userID] = jaccard(targetSet, otherSet)
	}
}

// ─── Manga-Manga Similarity ──────────────────────────────────────────────────

// ComputeMangaSimilarity fills MangaSimilarity based on co-occurrence across all users.
// Two manga are similar if many users have read both.
func (e *RecommendationEngine) ComputeMangaSimilarity() {
	e.MangaSimilarity = make(map[string][]string)

	// co[mangaA][mangaB] = number of users who have read both
	co := make(map[string]map[string]int)

	for _, profile := range e.UserProfiles {
		mangaSet := profile.ReadManga
		for i := 0; i < len(mangaSet); i++ {
			for j := i + 1; j < len(mangaSet); j++ {
				a, b := mangaSet[i], mangaSet[j]
				if co[a] == nil {
					co[a] = make(map[string]int)
				}
				if co[b] == nil {
					co[b] = make(map[string]int)
				}
				co[a][b]++
				co[b][a]++
			}
		}
	}

	// For each manga, sort its co-read manga by count and take top 10
	for mangaID, others := range co {
		type pair struct {
			id    string
			count int
		}
		pairs := make([]pair, 0, len(others))
		for otherID, cnt := range others {
			pairs = append(pairs, pair{otherID, cnt})
		}
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].count > pairs[j].count
		})
		top := 10
		if len(pairs) < top {
			top = len(pairs)
		}
		similar := make([]string, top)
		for i := 0; i < top; i++ {
			similar[i] = pairs[i].id
		}
		e.MangaSimilarity[mangaID] = similar
	}
}

// ─── Recommendation ──────────────────────────────────────────────────────────

// ScoredManga pairs a manga ID with a recommendation score.
type ScoredManga struct {
	MangaID string  `json:"manga_id"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
}

// Recommend generates up to `limit` manga recommendations for the target user.
// Strategy (applied in order):
//  1. Collaborative: manga read by similar users that the target hasn't read yet.
//  2. Content-based: manga similar (co-occurrence) to what the target has completed.
//  3. Genre-based fallback: popular genres from the user's profile.
func (e *RecommendationEngine) Recommend(targetUserID string, limit int) []ScoredManga {
	target, ok := e.UserProfiles[targetUserID]
	if !ok {
		return nil
	}

	alreadyRead := toSet(target.ReadManga)
	scores := make(map[string]float64)
	reasons := make(map[string]string)

	// 1. Collaborative filtering — weighted by user similarity
	sortedUsers := rankUsersBySimilarity(e.UserSimilarity)
	for _, uid := range sortedUsers {
		sim := e.UserSimilarity[uid]
		if sim == 0 {
			continue
		}
		other, ok := e.UserProfiles[uid]
		if !ok {
			continue
		}
		for _, mangaID := range other.CompletedManga {
			if alreadyRead[mangaID] {
				continue
			}
			scores[mangaID] += sim * 2.0 // completed manga scored higher
			if reasons[mangaID] == "" {
				reasons[mangaID] = "collaborative"
			}
		}
		for _, mangaID := range other.ReadManga {
			if alreadyRead[mangaID] {
				continue
			}
			scores[mangaID] += sim
			if reasons[mangaID] == "" {
				reasons[mangaID] = "collaborative"
			}
		}
	}

	// 2. Content-based — manga similar to what the user has completed
	for _, completedID := range target.CompletedManga {
		for _, similarID := range e.MangaSimilarity[completedID] {
			if alreadyRead[similarID] {
				continue
			}
			scores[similarID] += 1.5
			if reasons[similarID] == "" {
				reasons[similarID] = "similar to " + completedID
			}
		}
	}

	// Build sorted list
	type entry struct {
		id     string
		score  float64
		reason string
	}
	var ranked []entry
	for id, score := range scores {
		ranked = append(ranked, entry{id, score, reasons[id]})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	result := make([]ScoredManga, len(ranked))
	for i, e := range ranked {
		result[i] = ScoredManga{MangaID: e.id, Score: math.Round(e.score*100) / 100, Reason: e.reason}
	}
	return result
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, v := range items {
		s[v] = true
	}
	return s
}

// jaccard computes |A ∩ B| / |A ∪ B|.
func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// rankUsersBySimilarity returns user IDs sorted by similarity descending.
func rankUsersBySimilarity(sim map[string]float64) []string {
	type kv struct {
		k string
		v float64
	}
	pairs := make([]kv, 0, len(sim))
	for k, v := range sim {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].v > pairs[j].v
	})
	ids := make([]string, len(pairs))
	for i, p := range pairs {
		ids[i] = p.k
	}
	return ids
}
