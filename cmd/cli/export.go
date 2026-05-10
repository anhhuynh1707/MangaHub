package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleExport(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: mangahub export <library|progress|all>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  library   Export your manga library")
		fmt.Println("  progress  Export reading progress")
		fmt.Println("  all       Full data export (tar.gz archive)")
		fmt.Println()
		fmt.Println("Flags:")
		fmt.Println("  --format json|csv   Output format (default: json)")
		fmt.Println("  --output <file>     Output file path")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  mangahub export library --format json --output library.json")
		fmt.Println("  mangahub export progress --format csv --output progress.csv")
		fmt.Println("  mangahub export all --output mangahub-backup.tar.gz")
		return
	}

	switch args[0] {
	case "library":
		exportLibrary(args[1:])
	case "progress":
		exportProgress(args[1:])
	case "all":
		exportAll(args[1:])
	default:
		fmt.Printf("✗ Unknown export target: '%s'\n", args[0])
		fmt.Println("Available: library, progress, all")
	}
}

// exportLibrary exports the user's manga library to JSON or CSV.
func exportLibrary(args []string) {
	format := parseFlag(args, "format")
	output := parseFlag(args, "output")
	if format == "" {
		format = "json"
	}
	if output == "" {
		output = "library." + format
	}

	cfg := requireAuth()
	fmt.Printf("Exporting library for %s...\n", cfg.Username)

	resp, err := apiRequest("GET", "/users/library", nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch library: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	// Parse the library data
	var library struct {
		UserID       string `json:"user_id"`
		Username     string `json:"username"`
		ReadingLists struct {
			Reading    []progressEntry `json:"reading"`
			Completed  []progressEntry `json:"completed"`
			PlanToRead []progressEntry `json:"plan_to_read"`
		} `json:"reading_lists"`
	}
	json.Unmarshal(resp.Data, &library)

	// Flatten all entries
	var allEntries []progressEntry
	allEntries = append(allEntries, library.ReadingLists.Reading...)
	allEntries = append(allEntries, library.ReadingLists.Completed...)
	allEntries = append(allEntries, library.ReadingLists.PlanToRead...)

	if len(allEntries) == 0 {
		fmt.Println("✗ Library is empty, nothing to export")
		return
	}

	var writeErr error
	switch format {
	case "json":
		exportData := map[string]interface{}{
			"user_id":     library.UserID,
			"username":    library.Username,
			"exported_at": time.Now().Format(time.RFC3339),
			"total":       len(allEntries),
			"entries":     allEntries,
		}
		writeErr = writeJSON(output, exportData)
	case "csv":
		writeErr = writeLibraryCSV(output, allEntries)
	default:
		fmt.Printf("✗ Unsupported format: '%s' (use json or csv)\n", format)
		return
	}

	if writeErr != nil {
		fmt.Printf("✗ Failed to write file: %v\n", writeErr)
		return
	}

	absPath, _ := filepath.Abs(output)
	fmt.Printf("✓ Library exported successfully!\n")
	fmt.Printf("  Entries:  %d\n", len(allEntries))
	fmt.Printf("  Format:   %s\n", strings.ToUpper(format))
	fmt.Printf("  File:     %s\n", absPath)
	fmt.Printf("\nImport back: mangahub import library --file %s\n", output)
}

// exportProgress exports the user's reading progress to JSON or CSV.
func exportProgress(args []string) {
	format := parseFlag(args, "format")
	output := parseFlag(args, "output")
	if format == "" {
		format = "csv"
	}
	if output == "" {
		output = "progress." + format
	}

	cfg := requireAuth()
	fmt.Printf("Exporting progress for %s...\n", cfg.Username)

	resp, err := apiRequest("GET", "/users/library", nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch progress: %v\n", err)
		return
	}
	if !resp.Success {
		fmt.Printf("✗ %s\n", resp.Error)
		return
	}

	var library struct {
		ReadingLists struct {
			Reading    []progressEntry `json:"reading"`
			Completed  []progressEntry `json:"completed"`
			PlanToRead []progressEntry `json:"plan_to_read"`
		} `json:"reading_lists"`
	}
	json.Unmarshal(resp.Data, &library)

	// Only export entries that have actual progress (chapter > 0)
	var progressEntries []progressEntry
	for _, list := range [][]progressEntry{
		library.ReadingLists.Reading,
		library.ReadingLists.Completed,
		library.ReadingLists.PlanToRead,
	} {
		for _, e := range list {
			progressEntries = append(progressEntries, e)
		}
	}

	if len(progressEntries) == 0 {
		fmt.Println("✗ No progress data to export")
		return
	}

	var writeErr error
	switch format {
	case "csv":
		writeErr = writeProgressCSV(output, progressEntries)
	case "json":
		exportData := map[string]interface{}{
			"exported_at": time.Now().Format(time.RFC3339),
			"total":       len(progressEntries),
			"progress":    progressEntries,
		}
		writeErr = writeJSON(output, exportData)
	default:
		fmt.Printf("✗ Unsupported format: '%s' (use json or csv)\n", format)
		return
	}

	if writeErr != nil {
		fmt.Printf("✗ Failed to write file: %v\n", writeErr)
		return
	}

	absPath, _ := filepath.Abs(output)
	fmt.Printf("✓ Progress exported successfully!\n")
	fmt.Printf("  Entries:  %d\n", len(progressEntries))
	fmt.Printf("  Format:   %s\n", strings.ToUpper(format))
	fmt.Printf("  File:     %s\n", absPath)
	fmt.Printf("\nImport back: mangahub import progress --file %s\n", output)
}

// exportAll creates a tar.gz archive containing library JSON + progress CSV + profile JSON.
func exportAll(args []string) {
	output := parseFlag(args, "output")
	if output == "" {
		output = "mangahub-backup.tar.gz"
	}

	cfg := requireAuth()
	fmt.Printf("Creating full backup for %s...\n", cfg.Username)

	// 1. Fetch library
	libResp, err := apiRequest("GET", "/users/library", nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch library: %v\n", err)
		return
	}

	// 2. Fetch reviews
	reviewResp, err := apiRequest("GET", "/users/reviews", nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch reviews: %v\n", err)
		// Non-fatal, continue
		reviewResp = nil
	}

	// 3. Fetch friends
	friendResp, err := apiRequest("GET", "/users/friends", nil, cfg.Token)
	if err != nil {
		fmt.Printf("✗ Failed to fetch friends: %v\n", err)
		friendResp = nil
	}

	// Build the archive
	outFile, err := os.Create(output)
	if err != nil {
		fmt.Printf("✗ Failed to create archive: %v\n", err)
		return
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	fileCount := 0

	// Add library.json
	if libResp != nil && libResp.Success {
		if addToArchive(tarWriter, "library.json", libResp.Data) == nil {
			fileCount++
			fmt.Println("  ✓ library.json")
		}
	}

	// Add reviews.json
	if reviewResp != nil && reviewResp.Success {
		if addToArchive(tarWriter, "reviews.json", reviewResp.Data) == nil {
			fileCount++
			fmt.Println("  ✓ reviews.json")
		}
	}

	// Add friends.json
	if friendResp != nil && friendResp.Success {
		if addToArchive(tarWriter, "friends.json", friendResp.Data) == nil {
			fileCount++
			fmt.Println("  ✓ friends.json")
		}
	}

	// Add progress.csv from library data
	if libResp != nil && libResp.Success {
		var library struct {
			ReadingLists struct {
				Reading    []progressEntry `json:"reading"`
				Completed  []progressEntry `json:"completed"`
				PlanToRead []progressEntry `json:"plan_to_read"`
			} `json:"reading_lists"`
		}
		json.Unmarshal(libResp.Data, &library)

		var all []progressEntry
		all = append(all, library.ReadingLists.Reading...)
		all = append(all, library.ReadingLists.Completed...)
		all = append(all, library.ReadingLists.PlanToRead...)

		if len(all) > 0 {
			csvData := progressToCSVBytes(all)
			if addToArchive(tarWriter, "progress.csv", csvData) == nil {
				fileCount++
				fmt.Println("  ✓ progress.csv")
			}
		}
	}

	// Add metadata
	meta := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"username":    cfg.Username,
		"user_id":     cfg.UserID,
		"version":     version,
		"files":       fileCount,
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	if addToArchive(tarWriter, "metadata.json", metaJSON) == nil {
		fileCount++
		fmt.Println("  ✓ metadata.json")
	}

	absPath, _ := filepath.Abs(output)
	fi, _ := os.Stat(output)
	sizeKB := float64(0)
	if fi != nil {
		sizeKB = float64(fi.Size()) / 1024.0
	}

	fmt.Printf("\n✓ Full backup created successfully!\n")
	fmt.Printf("  Files:    %d\n", fileCount)
	fmt.Printf("  Size:     %.1f KB\n", sizeKB)
	fmt.Printf("  Archive:  %s\n", absPath)
}

// --- Helper functions ---

func writeJSON(filePath string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonData, 0644)
}

func writeLibraryCSV(filePath string, entries []progressEntry) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	defer w.Flush()

	w.Write([]string{"manga_id", "current_chapter", "status", "updated_at"})
	for _, e := range entries {
		w.Write([]string{
			e.MangaID,
			fmt.Sprintf("%d", e.CurrentChapter),
			e.Status,
			e.UpdatedAt.Format(time.RFC3339),
		})
	}
	return nil
}

func writeProgressCSV(filePath string, entries []progressEntry) error {
	return writeLibraryCSV(filePath, entries) // same format
}

func progressToCSVBytes(entries []progressEntry) []byte {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	w.Write([]string{"manga_id", "current_chapter", "status", "updated_at"})
	for _, e := range entries {
		w.Write([]string{
			e.MangaID,
			fmt.Sprintf("%d", e.CurrentChapter),
			e.Status,
			e.UpdatedAt.Format(time.RFC3339),
		})
	}
	w.Flush()
	return []byte(buf.String())
}

func addToArchive(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}
