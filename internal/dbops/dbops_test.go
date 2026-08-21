package dbops

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupVerifyRestoreAndPrune(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	backupDir := filepath.Join(root, "backups")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(dataDir, "mangahub.db")

	database, err := sql.Open("sqlite3", databasePath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TABLE proof (value TEXT NOT NULL); INSERT INTO proof VALUES ('before');`); err != nil {
		t.Fatal(err)
	}

	backupName := "mangahub-20260822T010203Z-sha-" + strings.Repeat("a", 40) + ".db"
	backupPath := filepath.Join(backupDir, backupName)
	metadata, err := Backup(databasePath, backupPath)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	if metadata.SizeBytes == 0 || len(metadata.SHA256) != 64 {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if _, err := VerifyBackup(backupPath); err != nil {
		t.Fatalf("VerifyBackup() error = %v", err)
	}

	if _, err := database.Exec(`DELETE FROM proof; INSERT INTO proof VALUES ('after');`); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(databasePath+"-wal", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(databasePath+"-shm", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Restore(backupPath, databasePath); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if _, err := os.Stat(databasePath + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("stale WAL was not removed: %v", err)
	}
	if _, err := os.Stat(databasePath + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("stale SHM was not removed: %v", err)
	}

	restored, err := sql.Open("sqlite3", databasePath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var value string
	if err := restored.QueryRow("SELECT value FROM proof").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "before" {
		t.Fatalf("restored value = %q, want before", value)
	}

	secondName := "mangahub-20260822T010204Z-pre-restore-sha-" + strings.Repeat("b", 40) + ".db"
	secondPath := filepath.Join(backupDir, secondName)
	if _, err := Backup(databasePath, secondPath); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(backupPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	removed, err := Prune(backupDir, 1)
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if len(removed) != 1 || removed[0] != backupName {
		t.Fatalf("removed = %v, want %s", removed, backupName)
	}
	if _, err := os.Stat(backupPath + ".sha256"); !os.IsNotExist(err) {
		t.Fatalf("old checksum sidecar was not removed: %v", err)
	}
}

func TestVerifyBackupRejectsCorruptionAndUnsafeNames(t *testing.T) {
	t.Parallel()

	if ValidBackupName("../../mangahub.db") {
		t.Fatal("path traversal was accepted")
	}
	if ValidBackupName("mangahub-latest.db") {
		t.Fatal("ambiguous backup name was accepted")
	}

	root := t.TempDir()
	databasePath := filepath.Join(root, "source.db")
	database, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("CREATE TABLE proof (value TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	name := "mangahub-20260822T010203Z-sha-" + strings.Repeat("c", 40) + ".db"
	backupPath := filepath.Join(root, name)
	if _, err := Backup(databasePath, backupPath); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(backupPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("corruption")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyBackup(backupPath); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("VerifyBackup() error = %v, want checksum mismatch", err)
	}
	if _, err := Prune(root, 1); err == nil || !strings.Contains(err.Error(), "refusing to prune") {
		t.Fatalf("Prune() error = %v, want invalid backup refusal", err)
	}
}
