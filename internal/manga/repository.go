package manga

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"mangahub/pkg/models"
)

// Repository handles database operations for manga using raw SQL.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new manga repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create inserts a new manga into the database.
func (r *Repository) Create(manga *models.Manga) error {
	genresJSON, _ := json.Marshal(manga.Genres)
	_, err := r.db.Exec(
		`INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		manga.ID, manga.Title, manga.Author, string(genresJSON),
		manga.Status, manga.TotalChapters, manga.Description, manga.CoverURL,
	)
	return err
}

// FindByID retrieves a manga by ID.
func (r *Repository) FindByID(id string) (*models.Manga, error) {
	var manga models.Manga
	var genresJSON string
	err := r.db.QueryRow(
		"SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga WHERE id = ?",
		id,
	).Scan(&manga.ID, &manga.Title, &manga.Author, &genresJSON,
		&manga.Status, &manga.TotalChapters, &manga.Description, &manga.CoverURL)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse genres JSON array
	json.Unmarshal([]byte(genresJSON), &manga.Genres)
	return &manga, nil
}

// FindByTitle retrieves a manga by title (exact match).
func (r *Repository) FindByTitle(title string) (*models.Manga, error) {
	var manga models.Manga
	var genresJSON string
	err := r.db.QueryRow(
		"SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga WHERE title = ?",
		title,
	).Scan(&manga.ID, &manga.Title, &manga.Author, &genresJSON,
		&manga.Status, &manga.TotalChapters, &manga.Description, &manga.CoverURL)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(genresJSON), &manga.Genres)
	return &manga, nil
}

// Search retrieves manga with filtering, advanced search, and sorting.
func (r *Repository) Search(query *models.MangaSearchQuery) ([]models.Manga, int, error) {
	whereClauses := []string{}
	args := []interface{}{}

	// Full-text search across title, author, description
	if query.Search != "" {
		whereClauses = append(whereClauses,
			"(LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(description) LIKE ?)")
		s := "%" + strings.ToLower(query.Search) + "%"
		args = append(args, s, s, s)
	}

	// Single-genre filter (legacy) — merged with multi-genre below
	allGenres := make([]string, 0, len(query.Genres)+1)
	if query.Genre != "" {
		allGenres = append(allGenres, query.Genre)
	}
	allGenres = append(allGenres, query.Genres...)

	// Multi-genre OR filter — each specified genre must appear in the genres JSON
	for _, g := range allGenres {
		if g != "" {
			whereClauses = append(whereClauses, "LOWER(genres) LIKE ?")
			args = append(args, "%"+strings.ToLower(g)+"%")
		}
	}

	if query.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, query.Status)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Rating filter: wrap the base table in a subquery joined with reviews
	useRatingJoin := query.MinRating > 0 || query.SortBy == "rating"

	var baseSQL string
	if useRatingJoin {
		// Subquery that computes avg rating per manga
		baseSQL = fmt.Sprintf(`
			SELECT m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url,
			       COALESCE(AVG(r.rating), 0) AS avg_rating
			FROM manga m
			LEFT JOIN reviews r ON r.manga_id = m.id
			%s
			GROUP BY m.id`, whereSQL)
		if query.MinRating > 0 {
			baseSQL += fmt.Sprintf(" HAVING avg_rating >= %.1f", query.MinRating)
		}
	}

	// ORDER BY based on sort_by
	orderSQL := "ORDER BY title ASC"
	switch query.SortBy {
	case "popularity":
		orderSQL = "ORDER BY total_chapters DESC"
	case "rating":
		orderSQL = "ORDER BY avg_rating DESC"
	case "recent":
		orderSQL = "ORDER BY rowid DESC"
	}

	// Pagination
	if query.Page < 1 {
		query.Page = 1
	}
	if query.Limit < 1 || query.Limit > 100 {
		query.Limit = 20
	}
	offset := (query.Page - 1) * query.Limit

	var total int
	var selectSQL string

	if useRatingJoin {
		countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s)", baseSQL)
		r.db.QueryRow(countSQL, args...).Scan(&total)

		selectSQL = fmt.Sprintf("%s %s LIMIT ? OFFSET ?", baseSQL, orderSQL)
	} else {
		countSQL := "SELECT COUNT(*) FROM manga" + whereSQL
		r.db.QueryRow(countSQL, args...).Scan(&total)

		selectSQL = fmt.Sprintf(
			"SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga%s %s LIMIT ? OFFSET ?",
			whereSQL, orderSQL,
		)
	}

	queryArgs := append(args, query.Limit, offset)
	rows, err := r.db.Query(selectSQL, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var mangaList []models.Manga
	for rows.Next() {
		var m models.Manga
		var genresJSON string
		if useRatingJoin {
			var avgRating float64
			if err := rows.Scan(&m.ID, &m.Title, &m.Author, &genresJSON,
				&m.Status, &m.TotalChapters, &m.Description, &m.CoverURL, &avgRating); err != nil {
				return nil, 0, err
			}
		} else {
			if err := rows.Scan(&m.ID, &m.Title, &m.Author, &genresJSON,
				&m.Status, &m.TotalChapters, &m.Description, &m.CoverURL); err != nil {
				return nil, 0, err
			}
		}
		json.Unmarshal([]byte(genresJSON), &m.Genres)
		mangaList = append(mangaList, m)
	}

	return mangaList, total, rows.Err()
}

// SearchByFilters converts a SearchFilters struct into a MangaSearchQuery and delegates to Search.
func (r *Repository) SearchByFilters(f *models.SearchFilters) ([]models.Manga, int, error) {
	q := &models.MangaSearchQuery{
		Search:    f.Search,
		Genres:    f.Genres,
		Status:    f.Status,
		MinRating: f.MinRating,
		SortBy:    f.SortBy,
		Page:      f.Page,
		Limit:     f.Limit,
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 20
	}
	// YearRange mapped to total_chapters range (as a proxy — manga data has no year field)
	// If YearRange is provided and non-zero, we store min/max chapters in the query struct.
	// This could be a dedicated chapter-range field in future.
	return r.Search(q)
}

// Update updates a manga record.
func (r *Repository) Update(manga *models.Manga) error {
	genresJSON, _ := json.Marshal(manga.Genres)
	result, err := r.db.Exec(
		`UPDATE manga SET title=?, author=?, genres=?, status=?, total_chapters=?, description=?, cover_url=?
		 WHERE id=?`,
		manga.Title, manga.Author, string(genresJSON),
		manga.Status, manga.TotalChapters, manga.Description, manga.CoverURL, manga.ID,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes a manga by ID.
func (r *Repository) Delete(id string) error {
	result, err := r.db.Exec("DELETE FROM manga WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Count returns the total number of manga in the database.
func (r *Repository) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	return count, err
}

// BulkCreate inserts multiple manga records. Skips duplicates.
func (r *Repository) BulkCreate(mangaList []models.Manga) (int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO manga (id, title, author, genres, status, total_chapters, description, cover_url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	inserted := 0
	for _, m := range mangaList {
		genresJSON, _ := json.Marshal(m.Genres)
		result, err := stmt.Exec(
			m.ID, m.Title, m.Author, string(genresJSON),
			m.Status, m.TotalChapters, m.Description, m.CoverURL,
		)
		if err != nil {
			continue
		}
		rows, _ := result.RowsAffected()
		inserted += int(rows)
	}

	return inserted, tx.Commit()
}

// GetAll retrieves all manga (for JSON export).
func (r *Repository) GetAll() ([]models.Manga, error) {
	rows, err := r.db.Query(
		"SELECT id, title, author, genres, status, total_chapters, description, cover_url FROM manga ORDER BY title ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mangaList []models.Manga
	for rows.Next() {
		var m models.Manga
		var genresJSON string
		if err := rows.Scan(&m.ID, &m.Title, &m.Author, &genresJSON,
			&m.Status, &m.TotalChapters, &m.Description, &m.CoverURL); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(genresJSON), &m.Genres)
		mangaList = append(mangaList, m)
	}

	return mangaList, rows.Err()
}
