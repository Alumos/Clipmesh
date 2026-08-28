package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound           = errors.New("clip not found")
	ErrUserNotFound       = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrLastAdmin          = errors.New("cannot delete the last administrator")
	ErrInvalidCredentials = errors.New("invalid username or password")
)

type Store struct {
	db        *sql.DB
	textLimit int
}

func Open(databasePath string, textLimit int) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A small writer pool is enough for a NAS-oriented service. WAL keeps reads
	// responsive while writes remain predictable on SQLite.
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(`
		PRAGMA journal_mode = WAL;
		PRAGMA foreign_keys = ON;
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
			created_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
		CREATE TABLE IF NOT EXISTS clips (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL CHECK (kind IN ('text', 'file')),
			device_id TEXT NOT NULL DEFAULT '',
			device_name TEXT NOT NULL DEFAULT '',
			mime_type TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			size INTEGER NOT NULL DEFAULT 0,
			formats_json TEXT NOT NULL DEFAULT '{}',
			preview TEXT NOT NULL DEFAULT '',
			storage_path TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			expires_at TEXT
		);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize sqlite: %w", err)
	}

	// The first release had no user_id column. Keep the migration additive so
	// an existing NAS volume can be upgraded in place.
	hasUserID, err := hasColumn(db, "clips", "user_id")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("inspect clips schema: %w", err)
	}
	if !hasUserID {
		if _, err := db.Exec(`ALTER TABLE clips ADD COLUMN user_id TEXT NOT NULL DEFAULT ''`); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate clips user ownership: %w", err)
		}
	}
	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_clips_user_created_at ON clips(user_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_clips_kind_created_at ON clips(kind, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_clips_expires_at ON clips(expires_at);
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize sqlite indexes: %w", err)
	}
	if textLimit < 1 {
		textLimit = 100
	}
	return &Store{db: db, textLimit: textLimit}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
