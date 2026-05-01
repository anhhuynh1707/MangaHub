package data

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"mangahub/pkg/models"
)

// ExportMangaToJSON writes a list of manga to a JSON file.
func ExportMangaToJSON(mangaList []models.Manga, filePath string) error {
	if err := ensureDir(filePath); err != nil {
		return err
	}

	data, err := json.MarshalIndent(mangaList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manga data: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manga JSON: %w", err)
	}

	log.Printf("✅ Exported %d manga to %s (%d bytes)", len(mangaList), filePath, len(data))
	return nil
}

// ImportMangaFromJSON reads a list of manga from a JSON file.
func ImportMangaFromJSON(filePath string) ([]models.Manga, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manga JSON: %w", err)
	}

	var mangaList []models.Manga
	if err := json.Unmarshal(data, &mangaList); err != nil {
		return nil, fmt.Errorf("failed to parse manga JSON: %w", err)
	}

	log.Printf("Imported %d manga from %s", len(mangaList), filePath)
	return mangaList, nil
}

// ExportUserDataToJSON writes a user's data (profile + reading lists) to a JSON file.
func ExportUserDataToJSON(userData models.UserData, dirPath string) error {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create user data directory: %w", err)
	}

	filePath := filepath.Join(dirPath, userData.UserID+".json")

	data, err := json.MarshalIndent(userData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write user JSON: %w", err)
	}

	log.Printf("✅ Exported user data for %s to %s", userData.Username, filePath)
	return nil
}

// ImportUserDataFromJSON reads a user's data from a JSON file.
func ImportUserDataFromJSON(filePath string) (*models.UserData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user JSON: %w", err)
	}

	var userData models.UserData
	if err := json.Unmarshal(data, &userData); err != nil {
		return nil, fmt.Errorf("failed to parse user JSON: %w", err)
	}

	return &userData, nil
}

// ExportQuotesToJSON writes scraped quotes to a JSON file.
func ExportQuotesToJSON(quotes []models.ScrapedQuote, filePath string) error {
	if err := ensureDir(filePath); err != nil {
		return err
	}

	data, err := json.MarshalIndent(quotes, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal quotes: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write quotes JSON: %w", err)
	}

	log.Printf("✅ Exported %d quotes to %s", len(quotes), filePath)
	return nil
}

// ImportQuotesFromJSON reads scraped quotes from a JSON file.
func ImportQuotesFromJSON(filePath string) ([]models.ScrapedQuote, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read quotes JSON: %w", err)
	}

	var quotes []models.ScrapedQuote
	if err := json.Unmarshal(data, &quotes); err != nil {
		return nil, fmt.Errorf("failed to parse quotes JSON: %w", err)
	}

	return quotes, nil
}

// ensureDir creates the parent directory for a file path if it doesn't exist.
func ensureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}
