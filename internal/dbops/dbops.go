package dbops

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

var backupNamePattern = regexp.MustCompile(
	`^mangahub-[0-9]{8}T[0-9]{6}Z(?:-pre-restore)?-sha-[0-9a-f]{40}\.db$`,
)

// Metadata describes one verified, standalone SQLite backup.
type Metadata struct {
	Path      string
	SHA256    string
	SizeBytes int64
}

// ValidBackupName limits operational filenames to timestamped, full-SHA releases.
func ValidBackupName(name string) bool {
	return filepath.Base(name) == name && backupNamePattern.MatchString(name)
}

// Backup creates a consistent online copy, verifies it, and writes a checksum sidecar.
func Backup(sourcePath, destinationPath string) (Metadata, error) {
	if err := requireRegularFile(sourcePath); err != nil {
		return Metadata{}, fmt.Errorf("source database: %w", err)
	}
	if _, err := os.Lstat(destinationPath); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return Metadata{}, fmt.Errorf("destination already exists: %s", destinationPath)
		}
		return Metadata{}, fmt.Errorf("inspect destination: %w", err)
	}

	destinationDir := filepath.Dir(destinationPath)
	if err := requireDirectory(destinationDir); err != nil {
		return Metadata{}, fmt.Errorf("destination directory: %w", err)
	}

	temporary, err := os.CreateTemp(destinationDir, ".mangahub-backup-*.db")
	if err != nil {
		return Metadata{}, fmt.Errorf("create temporary backup: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return Metadata{}, fmt.Errorf("close temporary backup: %w", err)
	}
	defer os.Remove(temporaryPath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	source, err := openSQLite(sourcePath, "ro", false)
	if err != nil {
		return Metadata{}, fmt.Errorf("open source database: %w", err)
	}
	defer source.Close()

	destination, err := openSQLite(temporaryPath, "rw", false)
	if err != nil {
		return Metadata{}, fmt.Errorf("open backup destination: %w", err)
	}

	if err := onlineBackup(ctx, source, destination); err != nil {
		_ = destination.Close()
		return Metadata{}, fmt.Errorf("online backup: %w", err)
	}
	if _, err := destination.ExecContext(ctx, "PRAGMA journal_mode=DELETE"); err != nil {
		_ = destination.Close()
		return Metadata{}, fmt.Errorf("make backup standalone: %w", err)
	}
	if err := destination.Close(); err != nil {
		return Metadata{}, fmt.Errorf("close backup destination: %w", err)
	}

	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return Metadata{}, fmt.Errorf("protect backup: %w", err)
	}
	if err := VerifyDatabase(temporaryPath); err != nil {
		return Metadata{}, fmt.Errorf("verify new backup: %w", err)
	}
	if err := syncFile(temporaryPath); err != nil {
		return Metadata{}, err
	}

	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return Metadata{}, fmt.Errorf("publish backup atomically: %w", err)
	}
	if err := syncDirectory(destinationDir); err != nil {
		return Metadata{}, err
	}

	metadata, err := metadataFor(destinationPath)
	if err != nil {
		_ = os.Remove(destinationPath)
		return Metadata{}, err
	}
	if err := writeChecksum(metadata); err != nil {
		_ = os.Remove(destinationPath)
		return Metadata{}, err
	}
	return metadata, nil
}

// VerifyBackup validates the checksum sidecar and SQLite file integrity.
func VerifyBackup(path string) (Metadata, error) {
	if err := requireRegularFile(path); err != nil {
		return Metadata{}, err
	}
	metadata, err := metadataFor(path)
	if err != nil {
		return Metadata{}, err
	}

	checksumPath := path + ".sha256"
	if err := requireRegularFile(checksumPath); err != nil {
		return Metadata{}, fmt.Errorf("checksum sidecar: %w", err)
	}
	checksumFile, err := os.Open(checksumPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("open checksum sidecar: %w", err)
	}
	defer checksumFile.Close()

	scanner := bufio.NewScanner(io.LimitReader(checksumFile, 1024))
	if !scanner.Scan() {
		return Metadata{}, fmt.Errorf("checksum sidecar is empty")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) != 2 || fields[1] != filepath.Base(path) {
		return Metadata{}, fmt.Errorf("checksum sidecar has an invalid format")
	}
	if fields[0] != metadata.SHA256 {
		return Metadata{}, fmt.Errorf("checksum mismatch")
	}
	if scanner.Scan() {
		return Metadata{}, fmt.Errorf("checksum sidecar contains unexpected extra data")
	}
	if err := scanner.Err(); err != nil {
		return Metadata{}, fmt.Errorf("read checksum sidecar: %w", err)
	}

	if err := VerifyDatabase(path); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

// VerifyDatabase runs SQLite's full integrity check against a standalone file.
func VerifyDatabase(path string) error {
	if err := requireRegularFile(path); err != nil {
		return err
	}
	database, err := openSQLite(path, "ro", true)
	if err != nil {
		return fmt.Errorf("open database for verification: %w", err)
	}
	defer database.Close()

	rows, err := database.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("run integrity check: %w", err)
	}
	defer rows.Close()

	resultCount := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("read integrity result: %w", err)
		}
		resultCount++
		if result != "ok" {
			return fmt.Errorf("integrity check failed: %s", result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("integrity result: %w", err)
	}
	if resultCount != 1 {
		return fmt.Errorf("integrity check returned %d rows", resultCount)
	}
	return nil
}

// Restore verifies a backup, copies it beside the destination, and atomically replaces it.
// Database clients must be stopped before this function is called.
func Restore(backupPath, destinationPath string) (Metadata, error) {
	metadata, err := VerifyBackup(backupPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("verify restore source: %w", err)
	}
	if err := requireRegularFile(destinationPath); err != nil {
		return Metadata{}, fmt.Errorf("destination database: %w", err)
	}

	destinationDir := filepath.Dir(destinationPath)
	temporary, err := os.CreateTemp(destinationDir, ".mangahub-restore-*.db")
	if err != nil {
		return Metadata{}, fmt.Errorf("create restore file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	source, err := os.Open(backupPath)
	if err != nil {
		_ = temporary.Close()
		return Metadata{}, fmt.Errorf("open restore source: %w", err)
	}
	_, copyErr := io.Copy(temporary, source)
	closeSourceErr := source.Close()
	syncErr := temporary.Sync()
	closeTemporaryErr := temporary.Close()
	if copyErr != nil {
		return Metadata{}, fmt.Errorf("copy restore source: %w", copyErr)
	}
	if closeSourceErr != nil {
		return Metadata{}, fmt.Errorf("close restore source: %w", closeSourceErr)
	}
	if syncErr != nil {
		return Metadata{}, fmt.Errorf("sync restore file: %w", syncErr)
	}
	if closeTemporaryErr != nil {
		return Metadata{}, fmt.Errorf("close restore file: %w", closeTemporaryErr)
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return Metadata{}, fmt.Errorf("protect restore file: %w", err)
	}
	if err := VerifyDatabase(temporaryPath); err != nil {
		return Metadata{}, fmt.Errorf("verify restore copy: %w", err)
	}

	for _, sidecar := range []string{destinationPath + "-wal", destinationPath + "-shm"} {
		if err := removeRegularFileIfPresent(sidecar); err != nil {
			return Metadata{}, err
		}
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return Metadata{}, fmt.Errorf("replace destination atomically: %w", err)
	}
	if err := syncDirectory(destinationDir); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

// Prune keeps the newest verified-looking backup names and removes older pairs.
func Prune(directory string, keep int) ([]string, error) {
	if keep < 1 {
		return nil, fmt.Errorf("retention must be at least 1")
	}
	if err := requireDirectory(directory); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read backup directory: %w", err)
	}

	type candidate struct {
		name    string
		modTime time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if !ValidBackupName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect backup %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("backup is not a regular file: %s", entry.Name())
		}
		if _, err := VerifyBackup(filepath.Join(directory, entry.Name())); err != nil {
			return nil, fmt.Errorf("refusing to prune with an invalid backup %s: %w", entry.Name(), err)
		}
		candidates = append(candidates, candidate{name: entry.Name(), modTime: info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].modTime.Equal(candidates[j].modTime) {
			return candidates[i].name > candidates[j].name
		}
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	if len(candidates) <= keep {
		return nil, nil
	}

	var removed []string
	for _, candidate := range candidates[keep:] {
		backupPath := filepath.Join(directory, candidate.name)
		if err := os.Remove(backupPath); err != nil {
			return removed, fmt.Errorf("remove backup %s: %w", candidate.name, err)
		}
		if err := os.Remove(backupPath + ".sha256"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return removed, fmt.Errorf("remove checksum for %s: %w", candidate.name, err)
		}
		removed = append(removed, candidate.name)
	}
	return removed, nil
}

func onlineBackup(ctx context.Context, source, destination *sql.DB) error {
	sourceConn, err := source.Conn(ctx)
	if err != nil {
		return err
	}
	defer sourceConn.Close()
	destinationConn, err := destination.Conn(ctx)
	if err != nil {
		return err
	}
	defer destinationConn.Close()

	return destinationConn.Raw(func(destinationDriver any) error {
		destinationSQLite, ok := destinationDriver.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("destination is not a SQLite connection")
		}
		return sourceConn.Raw(func(sourceDriver any) error {
			sourceSQLite, ok := sourceDriver.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("source is not a SQLite connection")
			}
			backup, err := destinationSQLite.Backup("main", sourceSQLite, "main")
			if err != nil {
				return err
			}
			finished := false
			defer func() {
				if !finished {
					_ = backup.Finish()
				}
			}()

			for {
				done, err := backup.Step(128)
				if err != nil {
					return err
				}
				if done {
					finishErr := backup.Finish()
					finished = true
					return finishErr
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
		})
	})
}

func openSQLite(path, mode string, immutable bool) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uri := &url.URL{Scheme: "file", Path: absolute}
	query := uri.Query()
	query.Set("mode", mode)
	query.Set("_busy_timeout", "5000")
	if immutable {
		query.Set("immutable", "1")
	}
	uri.RawQuery = query.Encode()

	database, err := sql.Open("sqlite3", uri.String())
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func metadataFor(path string) (Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("open backup for checksum: %w", err)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()
		return Metadata{}, fmt.Errorf("checksum backup: %w", err)
	}
	if err := file.Close(); err != nil {
		return Metadata{}, fmt.Errorf("close backup after checksum: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Metadata{}, fmt.Errorf("inspect backup: %w", err)
	}
	return Metadata{
		Path:      path,
		SHA256:    hex.EncodeToString(hash.Sum(nil)),
		SizeBytes: info.Size(),
	}, nil
}

func writeChecksum(metadata Metadata) error {
	checksumPath := metadata.Path + ".sha256"
	temporary, err := os.CreateTemp(filepath.Dir(metadata.Path), ".mangahub-checksum-*")
	if err != nil {
		return fmt.Errorf("create checksum sidecar: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect checksum sidecar: %w", err)
	}
	if _, err := fmt.Fprintf(temporary, "%s  %s\n", metadata.SHA256, filepath.Base(metadata.Path)); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write checksum sidecar: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync checksum sidecar: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close checksum sidecar: %w", err)
	}
	if err := os.Rename(temporaryPath, checksumPath); err != nil {
		return fmt.Errorf("publish checksum sidecar: %w", err)
	}
	return syncDirectory(filepath.Dir(metadata.Path))
}

func requireRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links are not allowed: %s", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", path)
	}
	return nil
}

func requireDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic links are not allowed: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	return nil
}

func removeRegularFileIfPresent(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect SQLite sidecar %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to remove unsafe SQLite sidecar: %s", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove SQLite sidecar %s: %w", path, err)
	}
	return nil
}

func syncFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
