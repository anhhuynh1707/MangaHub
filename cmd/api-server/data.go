package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"mangahub/data"
	"mangahub/internal/auth"
	mangaPkg "mangahub/internal/manga"
	"mangahub/pkg/models"
	"mangahub/pkg/utils"

	"github.com/gin-gonic/gin"
)

// DataSeed imports a fixed batch of manga from MangaDex.
func (s *APIServer) DataSeed(c *gin.Context) {
	imported, err := mangaPkg.ImportFromMangaDex(s.MangaService, s.MangaDexClient, 100)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch from MangaDex: "+err.Error())
		return
	}
	utils.SuccessResponse(c, fmt.Sprintf("Seeded %d manga from MangaDex", imported), gin.H{"imported": imported})
}

// DataFetchMangaDex imports a configurable count (?count=N, 1..500) from MangaDex.
func (s *APIServer) DataFetchMangaDex(c *gin.Context) {
	count := 100
	if v := c.Query("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			count = n
		}
	}
	imported, err := mangaPkg.ImportFromMangaDex(s.MangaService, s.MangaDexClient, count)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to fetch from MangaDex: "+err.Error())
		return
	}
	utils.SuccessResponse(c, fmt.Sprintf("Imported %d manga from MangaDex", imported), gin.H{
		"imported":  imported,
		"requested": count,
	})
}

// DataExportJSON returns every manga as JSON (admin/full catalog dump).
func (s *APIServer) DataExportJSON(c *gin.Context) {
	allManga, err := s.MangaService.GetAll()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to export manga")
		return
	}
	utils.SuccessResponse(c, "Manga exported as JSON", allManga)
}

// ── Web scraping (educational) ───────────────────────────────────────

func (s *APIServer) DataScrapeQuotes(c *gin.Context) {
	pages, _ := strconv.Atoi(c.DefaultQuery("pages", "3"))
	scraper := data.NewScraper()
	quotes, err := scraper.ScrapeQuotes(pages)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Scraping failed: "+err.Error())
		return
	}
	if err := data.ExportQuotesToJSON(quotes, "./data/scraped_quotes.json"); err != nil {
		log.Printf("Warning: failed to save quotes to JSON: %v", err)
	}
	utils.SuccessResponse(c, fmt.Sprintf("Scraped %d quotes from %d pages", len(quotes), pages), gin.H{
		"quotes": quotes,
		"count":  len(quotes),
		"pages":  pages,
	})
}

func (s *APIServer) DataScrapedQuotes(c *gin.Context) {
	quotes, err := data.ImportQuotesFromJSON("./data/scraped_quotes.json")
	if err != nil {
		utils.NotFoundResponse(c, "No scraped quotes found. Run POST /data/scrape-quotes first.")
		return
	}
	utils.SuccessResponse(c, fmt.Sprintf("Found %d scraped quotes", len(quotes)), quotes)
}

func (s *APIServer) DataTestHTTPBin(c *gin.Context) {
	results, err := data.NewScraper().TestHTTPBin()
	if err != nil {
		utils.InternalServerErrorResponse(c, "HTTPBin test failed: "+err.Error())
		return
	}
	utils.SuccessResponse(c, "HTTPBin tests completed", results)
}

// ── JSON file storage ────────────────────────────────────────────────

func (s *APIServer) DataExportFiles(c *gin.Context) {
	allManga, err := s.MangaService.GetAll()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get manga for export")
		return
	}
	if err := data.ExportMangaToJSON(allManga, "./data/manga.json"); err != nil {
		utils.InternalServerErrorResponse(c, "Failed to export manga JSON: "+err.Error())
		return
	}
	utils.SuccessResponse(c, "Data exported to JSON files", gin.H{
		"manga_file":  "./data/manga.json",
		"manga_count": len(allManga),
	})
}

func (s *APIServer) DataImportJSON(c *gin.Context) {
	mangaList, err := data.ImportMangaFromJSON("./data/manga.json")
	if err != nil {
		utils.NotFoundResponse(c, "No manga.json found. Run POST /data/export-files first.")
		return
	}
	inserted, _ := s.MangaService.BulkCreate(mangaList)
	utils.SuccessResponse(c, fmt.Sprintf("Imported %d manga from JSON", inserted), gin.H{
		"imported": inserted,
		"total":    len(mangaList),
	})
}

// ── Data export downloads (JSON / CSV) ───────────────────────────────

func (s *APIServer) userLibraryEntries(userID string) ([]models.UserProgress, string, error) {
	userData, err := s.UserService.GetLibrary(userID)
	if err != nil {
		return nil, "", err
	}
	var entries []models.UserProgress
	entries = append(entries, userData.ReadingLists.Reading...)
	entries = append(entries, userData.ReadingLists.Completed...)
	entries = append(entries, userData.ReadingLists.PlanToRead...)
	return entries, userData.Username, nil
}

func writeProgressCSV(c *gin.Context, filename string, entries []models.UserProgress) {
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv")
	c.Writer.WriteString("manga_id,current_chapter,status,updated_at\n")
	for _, e := range entries {
		c.Writer.WriteString(fmt.Sprintf("%s,%d,%s,%s\n",
			e.MangaID, e.CurrentChapter, e.Status, e.UpdatedAt.Format(time.RFC3339)))
	}
}

func (s *APIServer) DataExportLibrary(c *gin.Context) {
	userID, _ := auth.GetUserIDFromContext(c)
	entries, username, err := s.userLibraryEntries(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get library")
		return
	}

	if c.DefaultQuery("format", "json") == "csv" {
		if err := data.ExportProgressToCSV(entries, "./data/library.csv"); err != nil {
			log.Printf("Warning: failed to save library CSV to disk: %v", err)
		}
		writeProgressCSV(c, "library.csv", entries)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=library.json")
	c.JSON(200, gin.H{
		"user_id":     userID,
		"username":    username,
		"exported_at": time.Now().Format(time.RFC3339),
		"total":       len(entries),
		"entries":     entries,
	})
}

func (s *APIServer) DataExportProgress(c *gin.Context) {
	userID, _ := auth.GetUserIDFromContext(c)
	entries, _, err := s.userLibraryEntries(userID)
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get progress")
		return
	}

	if c.DefaultQuery("format", "csv") == "csv" {
		if err := data.ExportProgressToCSV(entries, "./data/progress.csv"); err != nil {
			log.Printf("Warning: failed to save progress CSV to disk: %v", err)
		}
		writeProgressCSV(c, "progress.csv", entries)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=progress.json")
	c.JSON(200, gin.H{
		"exported_at": time.Now().Format(time.RFC3339),
		"total":       len(entries),
		"progress":    entries,
	})
}

func (s *APIServer) DataExportManga(c *gin.Context) {
	allManga, err := s.MangaService.GetAll()
	if err != nil {
		utils.InternalServerErrorResponse(c, "Failed to get manga")
		return
	}

	if c.DefaultQuery("format", "json") == "csv" {
		if err := data.ExportMangaToCSV(allManga, "./data/manga.csv"); err != nil {
			log.Printf("Warning: failed to save manga CSV to disk: %v", err)
		}
		c.Header("Content-Disposition", "attachment; filename=manga.csv")
		c.Header("Content-Type", "text/csv")
		c.Writer.WriteString("id,title,author,genres,status,total_chapters,description\n")
		for _, m := range allManga {
			c.Writer.WriteString(fmt.Sprintf("%s,%q,%q,%s,%d\n",
				m.ID, m.Title, m.Author, m.Status, m.TotalChapters))
		}
		return
	}
	if err := data.ExportMangaToJSON(allManga, "./data/manga_export.json"); err != nil {
		log.Printf("Warning: failed to save manga JSON to disk: %v", err)
	}
	c.Header("Content-Disposition", "attachment; filename=manga.json")
	c.JSON(200, allManga)
}

func (s *APIServer) DataExportFull(c *gin.Context) {
	userID, _ := auth.GetUserIDFromContext(c)
	library, _ := s.UserService.GetLibrary(userID)
	c.Header("Content-Disposition", "attachment; filename=mangahub-export.json")
	c.JSON(200, gin.H{
		"exported_at": time.Now().Format(time.RFC3339),
		"version":     "1.0",
		"user_id":     userID,
		"library":     library,
	})
}
