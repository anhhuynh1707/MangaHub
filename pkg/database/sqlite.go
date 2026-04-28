package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB is the global database instance.
var DB *sql.DB

// InitDB initializes the SQLite database connection and creates the schema.
func InitDB(dbPath string) (*sql.DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create tables
	if err := createSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	DB = db
	log.Println("Database initialized successfully")
	return db, nil
}

// createSchema creates all required tables matching the spec exactly.
func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            TEXT PRIMARY KEY,
		username      TEXT UNIQUE,
		password_hash TEXT,
		created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS manga (
		id             TEXT PRIMARY KEY,
		title          TEXT,
		author         TEXT,
		genres         TEXT,
		status         TEXT,
		total_chapters INTEGER,
		description    TEXT,
		cover_url      TEXT
	);

	CREATE TABLE IF NOT EXISTS user_progress (
		user_id         TEXT,
		manga_id        TEXT,
		current_chapter INTEGER,
		status          TEXT,
		updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, manga_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (manga_id) REFERENCES manga(id)
	);

	CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
	CREATE INDEX IF NOT EXISTS idx_manga_status ON manga(status);
	CREATE INDEX IF NOT EXISTS idx_user_progress_user ON user_progress(user_id);
	`

	_, err := db.Exec(schema)
	return err
}

// GetDB returns the global database instance.
func GetDB() *sql.DB {
	return DB
}
