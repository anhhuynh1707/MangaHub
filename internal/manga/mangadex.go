package manga

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mangahub/pkg/models"
)

// MangaDexClient handles communication with the MangaDex API.
// API docs: https://api.mangadex.org/docs/
type MangaDexClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewMangaDexClient creates a new MangaDex API client.
func NewMangaDexClient() *MangaDexClient {
	return &MangaDexClient{
		baseURL: "https://api.mangadex.org",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- MangaDex API response types ---

// mangaDexResponse represents the paginated collection response from GET /manga.
type mangaDexResponse struct {
	Result   string          `json:"result"`
	Response string          `json:"response"`
	Data     []mangaDexManga `json:"data"`
	Limit    int             `json:"limit"`
	Offset   int             `json:"offset"`
	Total    int             `json:"total"`
}

type mangaDexManga struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Title            map[string]string `json:"title"`
		AltTitles        []map[string]string `json:"altTitles"`
		Description      map[string]string `json:"description"`
		Status           string            `json:"status"`
		Year             *int              `json:"year"`
		Tags             []mangaDexTag     `json:"tags"`
		LastChapter      string            `json:"lastChapter"`
		LastVolume       string            `json:"lastVolume"`
		OriginalLanguage string            `json:"originalLanguage"`
		PublicationDemographic *string     `json:"publicationDemographic"`
	} `json:"attributes"`
	Relationships []mangaDexRelationship `json:"relationships"`
}

type mangaDexTag struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name  map[string]string `json:"name"`
		Group string            `json:"group"`
	} `json:"attributes"`
}

type mangaDexRelationship struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes *struct {
		Name     string `json:"name"`
		FileName string `json:"fileName"`
	} `json:"attributes,omitempty"`
}

// --- Public API methods ---

// FetchPopularManga fetches the most popular manga from MangaDex API.
// Uses GET /manga with order[followedCount]=desc and pagination.
// MangaDex limits to 100 per request, so we paginate with offset.
func (c *MangaDexClient) FetchPopularManga(total int) ([]models.Manga, error) {
	var allManga []models.Manga

	for offset := 0; offset < total; offset += 100 {
		batchSize := 100
		remaining := total - offset
		if remaining < batchSize {
			batchSize = remaining
		}

		// Build URL using proper URL encoding
		params := url.Values{}
		params.Set("limit", fmt.Sprintf("%d", batchSize))
		params.Set("offset", fmt.Sprintf("%d", offset))
		params.Add("includes[]", "author")
		params.Add("includes[]", "cover_art")
		params.Set("order[followedCount]", "desc")
		params.Add("availableTranslatedLanguage[]", "en")
		params.Set("contentRating[]", "safe")

		reqURL := fmt.Sprintf("%s/manga?%s", c.baseURL, params.Encode())
		log.Printf("MangaDex API: GET %s", reqURL)

		batch, err := c.fetchBatch(reqURL)
		if err != nil {
			log.Printf("MangaDex batch error at offset %d: %v", offset, err)
			break
		}

		allManga = append(allManga, batch...)
		log.Printf("MangaDex: fetched %d manga (offset %d, total so far: %d)", len(batch), offset, len(allManga))

		// Respect MangaDex rate limits (5 requests/second for anonymous)
		if offset+batchSize < total {
			time.Sleep(300 * time.Millisecond)
		}
	}

	return allManga, nil
}

// FetchMangaByTitle searches MangaDex for manga matching a title.
// Uses GET /manga with title query parameter.
func (c *MangaDexClient) FetchMangaByTitle(title string, limit int) ([]models.Manga, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	params := url.Values{}
	params.Set("title", title)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Add("includes[]", "author")
	params.Add("includes[]", "cover_art")

	reqURL := fmt.Sprintf("%s/manga?%s", c.baseURL, params.Encode())
	return c.fetchBatch(reqURL)
}

// FetchMangaByID fetches a single manga by its MangaDex UUID.
// Uses GET /manga/{id}.
func (c *MangaDexClient) FetchMangaByID(mangaDexID string) (*models.Manga, error) {
	params := url.Values{}
	params.Add("includes[]", "author")
	params.Add("includes[]", "cover_art")

	reqURL := fmt.Sprintf("%s/manga/%s?%s", c.baseURL, mangaDexID, params.Encode())

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("MangaDex API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MangaDex API returned %d: %s", resp.StatusCode, string(body))
	}

	var singleResp struct {
		Result string        `json:"result"`
		Data   mangaDexManga `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&singleResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	manga := c.convertSingleManga(singleResp.Data)
	return &manga, nil
}

// --- Internal helpers ---

// fetchBatch performs a single paginated GET /manga request and returns parsed manga.
func (c *MangaDexClient) fetchBatch(reqURL string) ([]models.Manga, error) {
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("MangaDex API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MangaDex API returned %d: %s", resp.StatusCode, string(body))
	}

	var mdResp mangaDexResponse
	if err := json.NewDecoder(resp.Body).Decode(&mdResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var mangaList []models.Manga
	for _, md := range mdResp.Data {
		manga := c.convertSingleManga(md)
		if manga.Title != "" {
			mangaList = append(mangaList, manga)
		}
	}

	return mangaList, nil
}

// convertSingleManga converts one MangaDex API entry to our Manga model.
func (c *MangaDexClient) convertSingleManga(md mangaDexManga) models.Manga {
	// Get English title; fallback to altTitles then first available
	title := ""
	if en, ok := md.Attributes.Title["en"]; ok {
		title = en
	} else {
		// Check altTitles for English
		for _, alt := range md.Attributes.AltTitles {
			if en, ok := alt["en"]; ok {
				title = en
				break
			}
		}
		// Fallback to any title
		if title == "" {
			for _, v := range md.Attributes.Title {
				title = v
				break
			}
		}
	}

	manga := models.Manga{
		ID:     generateSlugID(title),
		Title:  title,
		Status: normalizeStatus(md.Attributes.Status),
	}

	// English description, truncated
	if en, ok := md.Attributes.Description["en"]; ok {
		if len(en) > 500 {
			en = en[:500] + "..."
		}
		manga.Description = en
	}

	// Author name from relationships
	for _, rel := range md.Relationships {
		if rel.Type == "author" && rel.Attributes != nil && rel.Attributes.Name != "" {
			manga.Author = rel.Attributes.Name
			break
		}
	}

	// Cover art URL from relationships
	for _, rel := range md.Relationships {
		if rel.Type == "cover_art" && rel.Attributes != nil && rel.Attributes.FileName != "" {
			manga.CoverURL = fmt.Sprintf("https://uploads.mangadex.org/covers/%s/%s.256.jpg",
				md.ID, rel.Attributes.FileName)
			break
		}
	}

	// Genres from tags (only genre and theme groups)
	var genres []string
	for _, tag := range md.Attributes.Tags {
		group := tag.Attributes.Group
		if group == "genre" || group == "theme" {
			if en, ok := tag.Attributes.Name["en"]; ok {
				genres = append(genres, en)
			}
		}
	}
	// Add demographic if present
	if md.Attributes.PublicationDemographic != nil && *md.Attributes.PublicationDemographic != "" {
		demo := strings.Title(strings.ToLower(*md.Attributes.PublicationDemographic))
		genres = append(genres, demo)
	}
	manga.Genres = genres

	// Chapter count from lastChapter
	if md.Attributes.LastChapter != "" {
		fmt.Sscanf(md.Attributes.LastChapter, "%d", &manga.TotalChapters)
	}

	return manga
}

// normalizeStatus maps MangaDex status values to our internal status values.
func normalizeStatus(status string) string {
	switch strings.ToLower(status) {
	case "ongoing":
		return "ongoing"
	case "completed":
		return "completed"
	case "hiatus":
		return "hiatus"
	case "cancelled":
		return "hiatus"
	default:
		return "ongoing"
	}
}

// generateSlugID creates a URL-friendly slug from a title.
func generateSlugID(title string) string {
	slug := strings.ToLower(title)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = slug[:80]
	}
	return slug
}

// --- Import function ---

// ImportFromMangaDex fetches 100 additional manga from MangaDex and saves to database.
// Skips manga that already exist (via INSERT OR IGNORE).
func ImportFromMangaDex(service *Service, client *MangaDexClient, total int) (int, error) {
	log.Printf("Starting MangaDex import: fetching %d manga...", total)

	mangaList, err := client.FetchPopularManga(total)
	if err != nil {
		return 0, fmt.Errorf("MangaDex fetch failed: %w", err)
	}

	log.Printf("Fetched %d manga from MangaDex, inserting into database...", len(mangaList))

	inserted, err := service.BulkCreate(mangaList)
	if err != nil {
		return 0, fmt.Errorf("database insert failed: %w", err)
	}

	log.Printf("✅ Imported %d new manga from MangaDex (%d fetched, duplicates skipped)", inserted, len(mangaList))
	return inserted, nil
}
