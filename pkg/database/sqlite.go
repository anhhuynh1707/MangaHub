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

	// WAL mode + a busy timeout let readers and writers coexist without
	// "database is locked" errors — important now that the first-run seed
	// runs concurrently with live login/register traffic.
	dsn := dbPath + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
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

	CREATE TABLE IF NOT EXISTS reviews (
		id         TEXT PRIMARY KEY,
		user_id    TEXT NOT NULL,
		manga_id   TEXT NOT NULL,
		rating     INTEGER NOT NULL CHECK(rating >= 1 AND rating <= 10),
		text       TEXT,
		helpful    INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (manga_id) REFERENCES manga(id),
		UNIQUE(user_id, manga_id)
	);

	CREATE TABLE IF NOT EXISTS friendships (
		user_id    TEXT NOT NULL,
		friend_id  TEXT NOT NULL,
		status     TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, friend_id),
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (friend_id) REFERENCES users(id),
		CHECK (user_id < friend_id)
	);

	CREATE TABLE IF NOT EXISTS shared_reading_lists (
		id          TEXT PRIMARY KEY,
		owner_id    TEXT NOT NULL,
		title       TEXT NOT NULL,
		description TEXT,
		is_public   BOOLEAN DEFAULT 0,
		manga_list  TEXT,
		shared_with TEXT,
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (owner_id) REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS activities (
		id          TEXT PRIMARY KEY,
		user_id     TEXT NOT NULL,
		type        TEXT NOT NULL,
		manga_id    TEXT,
		review_id   TEXT,
		friend_id   TEXT,
		message     TEXT,
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (manga_id) REFERENCES manga(id)
	);

	CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
	CREATE INDEX IF NOT EXISTS idx_manga_status ON manga(status);
	CREATE INDEX IF NOT EXISTS idx_user_progress_user ON user_progress(user_id);
	CREATE INDEX IF NOT EXISTS idx_reviews_manga ON reviews(manga_id);
	CREATE INDEX IF NOT EXISTS idx_reviews_user ON reviews(user_id);
	CREATE INDEX IF NOT EXISTS idx_friendships_user ON friendships(user_id);
	CREATE INDEX IF NOT EXISTS idx_shared_lists_owner ON shared_reading_lists(owner_id);
	CREATE INDEX IF NOT EXISTS idx_activities_user ON activities(user_id);
	CREATE INDEX IF NOT EXISTS idx_activities_created ON activities(created_at DESC);
	`

	_, err := db.Exec(schema)
	return err
}

// GetDB returns the global database instance.
func GetDB() *sql.DB {
	return DB
}
