package data

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"mangahub/pkg/models"
)

// ExportProgressToCSV writes user progress entries to a CSV file.
func ExportProgressToCSV(entries []models.UserProgress, filePath string) error {
	if err := ensureDir(filePath); err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"manga_id", "current_chapter", "status", "updated_at"})
	for _, e := range entries {
		writer.Write([]string{
			e.MangaID,
			strconv.Itoa(e.CurrentChapter),
			e.Status,
			e.UpdatedAt.Format(time.RFC3339),
		})
	}

	log.Printf("✅ Exported %d progress entries to %s", len(entries), filePath)
	return nil
}

// ImportProgressFromCSV reads user progress entries from a CSV file.
func ImportProgressFromCSV(filePath string) ([]models.UserProgress, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file has no data rows")
	}

	var entries []models.UserProgress
	for _, record := range records[1:] {
		if len(record) < 3 {
			continue
		}
		chapter, _ := strconv.Atoi(record[1])
		var updatedAt time.Time
		if len(record) >= 4 {
			updatedAt, _ = time.Parse(time.RFC3339, record[3])
		}
		entries = append(entries, models.UserProgress{
			MangaID:        record[0],
			CurrentChapter: chapter,
			Status:         record[2],
			UpdatedAt:      updatedAt,
		})
	}

	log.Printf("Imported %d progress entries from %s", len(entries), filePath)
	return entries, nil
}

// ExportMangaToCSV writes manga data to a CSV file.
func ExportMangaToCSV(mangaList []models.Manga, filePath string) error {
	if err := ensureDir(filePath); err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"id", "title", "author", "genres", "status", "total_chapters", "description"})
	for _, m := range mangaList {
		writer.Write([]string{
			m.ID,
			m.Title,
			m.Author,
			strings.Join(m.Genres, ";"),
			m.Status,
			strconv.Itoa(m.TotalChapters),
			m.Description,
		})
	}

	log.Printf("✅ Exported %d manga to CSV: %s", len(mangaList), filePath)
	return nil
}

// ImportMangaFromCSV reads manga data from a CSV file.
func ImportMangaFromCSV(filePath string) ([]models.Manga, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file has no data rows")
	}

	var mangaList []models.Manga
	for _, record := range records[1:] {
		if len(record) < 6 {
			continue
		}
		totalChapters, _ := strconv.Atoi(record[5])
		var genres []string
		if record[3] != "" {
			genres = strings.Split(record[3], ";")
		}
		desc := ""
		if len(record) >= 7 {
			desc = record[6]
		}
		mangaList = append(mangaList, models.Manga{
			ID:            record[0],
			Title:         record[1],
			Author:        record[2],
			Genres:        genres,
			Status:        record[4],
			TotalChapters: totalChapters,
			Description:   desc,
		})
	}

	log.Printf("Imported %d manga from CSV: %s", len(mangaList), filePath)
	return mangaList, nil
}
