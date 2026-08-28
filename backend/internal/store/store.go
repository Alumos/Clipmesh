package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
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

	// The first release had no user_id column. Keep the migration deliberately
	// additive so an existing NAS volume can be upgraded in place.
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

// EnsureAdmin creates the first administrator when the database is empty. On
// later starts the existing hash is preserved, so changing an environment
// variable cannot silently invalidate or replace a database account. Legacy
// clips without an owner are assigned to this administrator exactly once.
func (s *Store) EnsureAdmin(ctx context.Context, username, password string) (model.User, error) {
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return model.User{}, err
	}
	if password == "" {
		return model.User{}, errors.New("admin password is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.User{}, fmt.Errorf("begin ensure admin: %w", err)
	}
	defer tx.Rollback()

	var user model.User
	var passwordHash string
	var createdAt string
	err = tx.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &createdAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		passwordHashBytes, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return model.User{}, fmt.Errorf("hash admin password: %w", hashErr)
		}
		user.ID, err = model.NewID()
		if err != nil {
			return model.User{}, fmt.Errorf("create admin id: %w", err)
		}
		user.Username = username
		user.Role = "admin"
		user.CreatedAt = time.Now().UTC()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, 'admin', ?)
		`, user.ID, user.Username, string(passwordHashBytes), user.CreatedAt.Format(time.RFC3339Nano))
		if err != nil {
			return model.User{}, fmt.Errorf("insert admin: %w", err)
		}
	case err != nil:
		return model.User{}, fmt.Errorf("find admin: %w", err)
	default:
		user.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if user.Role != "admin" {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id = ?`, user.ID); err != nil {
				return model.User{}, fmt.Errorf("promote configured admin: %w", err)
			}
			user.Role = "admin"
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE clips SET user_id = ? WHERE user_id = '' OR user_id IS NULL`, user.ID); err != nil {
		return model.User{}, fmt.Errorf("assign legacy clips: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.User{}, fmt.Errorf("commit ensure admin: %w", err)
	}
	return user, nil
}

func (s *Store) AuthenticateUser(ctx context.Context, username, password string) (model.User, error) {
	username = strings.TrimSpace(username)
	var user model.User
	var passwordHash string
	var createdAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return model.User{}, ErrInvalidCredentials
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return user, nil
}

func (s *Store) CreateUser(ctx context.Context, username, password, role string) (model.User, error) {
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return model.User{}, err
	}
	if len([]byte(password)) < 8 {
		return model.User{}, errors.New("password must be at least 8 characters")
	}
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		return model.User{}, errors.New("role must be user or admin")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash user password: %w", err)
	}
	id, err := model.NewID()
	if err != nil {
		return model.User{}, fmt.Errorf("create user id: %w", err)
	}
	createdAt := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)
	`, id, username, string(hash), role, createdAt.Format(time.RFC3339Nano))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return model.User{}, ErrUsernameTaken
		}
		return model.User{}, fmt.Errorf("insert user: %w", err)
	}
	return model.User{ID: id, Username: username, Role: role, CreatedAt: createdAt}, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, role, created_at
		FROM users
		ORDER BY CASE WHEN role = 'admin' THEN 0 ELSE 1 END, created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := make([]model.User, 0)
	for rows.Next() {
		var user model.User
		var createdAt string
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &createdAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		user.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

// DeleteUser returns file paths that the HTTP layer should remove after the
// transaction commits. Account deletion also deletes that account's clips and
// sessions, so data can never become visible to another user.
func (s *Store) DeleteUser(ctx context.Context, id string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin delete user: %w", err)
	}
	defer tx.Rollback()

	var role string
	if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&role); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, fmt.Errorf("find user for delete: %w", err)
	}
	if role == "admin" {
		var adminCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount); err != nil {
			return nil, fmt.Errorf("count administrators: %w", err)
		}
		if adminCount <= 1 {
			return nil, ErrLastAdmin
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT storage_path FROM clips WHERE user_id = ? AND storage_path <> ''`, id)
	if err != nil {
		return nil, fmt.Errorf("find user files: %w", err)
	}
	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan user file: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close user files: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE user_id = ?`, id); err != nil {
		return nil, fmt.Errorf("delete user clips: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return nil, fmt.Errorf("delete user sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
		return nil, fmt.Errorf("delete user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit delete user: %w", err)
	}
	return paths, nil
}

func (s *Store) SaveSession(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)
	`, tokenHash, userID, expiresAt.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (s *Store) UserForSession(ctx context.Context, tokenHash string) (model.User, error) {
	var user model.User
	var createdAt string
	now := time.Now().UTC().Format(time.RFC3339Nano)
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.role, u.created_at
		FROM sessions AS sess
		JOIN users AS u ON u.id = sess.user_id
		WHERE sess.token_hash = ? AND sess.expires_at > ?
	`, tokenHash, now).Scan(&user.ID, &user.Username, &user.Role, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
		return model.User{}, ErrUserNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find session user: %w", err)
	}
	user.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return user, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *Store) CleanupSessions(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("cleanup sessions: %w", err)
	}
	return nil
}

func (s *Store) CreateText(ctx context.Context, deviceID, deviceName string, formats map[string]string) (model.Clip, error) {
	return s.createText(ctx, "", deviceID, deviceName, formats)
}

func (s *Store) CreateTextForUser(ctx context.Context, userID, deviceID, deviceName string, formats map[string]string) (model.Clip, error) {
	return s.createText(ctx, userID, deviceID, deviceName, formats)
}

func (s *Store) createText(ctx context.Context, userID, deviceID, deviceName string, formats map[string]string) (model.Clip, error) {
	if formats == nil {
		formats = map[string]string{}
	}
	plain := strings.TrimSpace(formats["text/plain"])
	if plain == "" {
		return model.Clip{}, fmt.Errorf("text/plain is required")
	}
	if deviceName == "" {
		deviceName = "Web browser"
	}
	id, err := model.NewID()
	if err != nil {
		return model.Clip{}, err
	}
	serialized, err := json.Marshal(formats)
	if err != nil {
		return model.Clip{}, fmt.Errorf("encode formats: %w", err)
	}
	now := time.Now().UTC()
	clip := model.Clip{
		ID:         id,
		UserID:     userID,
		Kind:       "text",
		DeviceID:   deviceID,
		DeviceName: deviceName,
		Formats:    formats,
		Preview:    preview(plain),
		CreatedAt:  now,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Clip{}, fmt.Errorf("begin text clip: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO clips (id, user_id, kind, device_id, device_name, formats_json, preview, created_at)
		VALUES (?, ?, 'text', ?, ?, ?, ?, ?)
	`, clip.ID, clip.UserID, clip.DeviceID, clip.DeviceName, string(serialized), clip.Preview, now.Format(time.RFC3339Nano)); err != nil {
		return model.Clip{}, fmt.Errorf("insert text clip: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM clips
		WHERE kind = 'text' AND user_id = ? AND id NOT IN (
			SELECT id FROM clips WHERE kind = 'text' AND user_id = ? ORDER BY created_at DESC LIMIT ?
		)
	`, userID, userID, s.textLimit); err != nil {
		return model.Clip{}, fmt.Errorf("prune text clips: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Clip{}, fmt.Errorf("commit text clip: %w", err)
	}
	return clip, nil
}

type FileInput struct {
	ID          string
	UserID      string
	DeviceID    string
	DeviceName  string
	MimeType    string
	Name        string
	Size        int64
	Formats     map[string]string
	StoragePath string
	ExpiresAt   time.Time
}

func (s *Store) CreateFile(ctx context.Context, input FileInput) (model.Clip, error) {
	if input.ID == "" {
		var err error
		input.ID, err = model.NewID()
		if err != nil {
			return model.Clip{}, err
		}
	}
	if input.DeviceName == "" {
		input.DeviceName = "Web browser"
	}
	if input.Formats == nil {
		input.Formats = map[string]string{}
	}
	serialized, err := json.Marshal(input.Formats)
	if err != nil {
		return model.Clip{}, fmt.Errorf("encode file formats: %w", err)
	}
	now := time.Now().UTC()
	expiresAt := input.ExpiresAt.UTC()
	clip := model.Clip{
		ID:          input.ID,
		UserID:      input.UserID,
		Kind:        "file",
		DeviceID:    input.DeviceID,
		DeviceName:  input.DeviceName,
		MimeType:    input.MimeType,
		Name:        input.Name,
		Size:        input.Size,
		Formats:     input.Formats,
		Preview:     input.Name,
		StoragePath: input.StoragePath,
		CreatedAt:   now,
		ExpiresAt:   &expiresAt,
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO clips (id, user_id, kind, device_id, device_name, mime_type, name, size, formats_json, preview, storage_path, created_at, expires_at)
		VALUES (?, ?, 'file', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, clip.ID, clip.UserID, clip.DeviceID, clip.DeviceName, clip.MimeType, clip.Name, clip.Size, string(serialized), clip.Preview, clip.StoragePath, now.Format(time.RFC3339Nano), expiresAt.Format(time.RFC3339Nano))
	if err != nil {
		return model.Clip{}, fmt.Errorf("insert file clip: %w", err)
	}
	return clip, nil
}

func (s *Store) List(ctx context.Context, query, kind string, limit int) ([]model.Clip, error) {
	return s.list(ctx, "", false, query, kind, limit)
}

func (s *Store) ListForUser(ctx context.Context, userID, query, kind string, limit int) ([]model.Clip, error) {
	return s.list(ctx, userID, true, query, kind, limit)
}

func (s *Store) list(ctx context.Context, userID string, scoped bool, query, kind string, limit int) ([]model.Clip, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	statement := `SELECT user_id, id, kind, device_id, device_name, mime_type, name, size, formats_json, preview, storage_path, created_at, expires_at FROM clips WHERE 1 = 1`
	args := make([]any, 0, 7)
	if scoped {
		statement += ` AND user_id = ?`
		args = append(args, userID)
	}
	if kind == "text" || kind == "file" {
		statement += ` AND kind = ?`
		args = append(args, kind)
	}
	if query != "" {
		like := "%" + query + "%"
		statement += ` AND (preview LIKE ? OR name LIKE ? OR device_name LIKE ?)`
		args = append(args, like, like, like)
	}
	statement += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list clips: %w", err)
	}
	defer rows.Close()
	clips := make([]model.Clip, 0)
	for rows.Next() {
		clip, err := scanClip(rows)
		if err != nil {
			return nil, err
		}
		clips = append(clips, clip)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clips: %w", err)
	}
	return clips, nil
}

func (s *Store) Get(ctx context.Context, id string) (model.Clip, error) {
	return s.get(ctx, "", false, id)
}

func (s *Store) GetForUser(ctx context.Context, userID, id string) (model.Clip, error) {
	return s.get(ctx, userID, true, id)
}

func (s *Store) get(ctx context.Context, userID string, scoped bool, id string) (model.Clip, error) {
	statement := `SELECT user_id, id, kind, device_id, device_name, mime_type, name, size, formats_json, preview, storage_path, created_at, expires_at FROM clips WHERE id = ?`
	args := []any{id}
	if scoped {
		statement += ` AND user_id = ?`
		args = append(args, userID)
	}
	row := s.db.QueryRowContext(ctx, statement, args...)
	clip, err := scanClip(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Clip{}, ErrNotFound
	}
	if err != nil {
		return model.Clip{}, fmt.Errorf("get clip: %w", err)
	}
	return clip, nil
}

func (s *Store) Delete(ctx context.Context, id string) (string, error) {
	return s.delete(ctx, "", false, id)
}

func (s *Store) DeleteForUser(ctx context.Context, userID, id string) (string, error) {
	return s.delete(ctx, userID, true, id)
}

func (s *Store) delete(ctx context.Context, userID string, scoped bool, id string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin delete: %w", err)
	}
	defer tx.Rollback()
	statement := `SELECT storage_path FROM clips WHERE id = ?`
	args := []any{id}
	if scoped {
		statement += ` AND user_id = ?`
		args = append(args, userID)
	}
	var storagePath string
	err = tx.QueryRowContext(ctx, statement, args...).Scan(&storagePath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find clip for delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ?`, id); err != nil {
		return "", fmt.Errorf("delete clip: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit delete: %w", err)
	}
	return storagePath, nil
}

type ExpiredClip struct {
	ID          string
	UserID      string
	StoragePath string
}

func (s *Store) CleanupExpired(ctx context.Context) ([]ExpiredClip, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin cleanup: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id, user_id, storage_path FROM clips WHERE expires_at IS NOT NULL AND expires_at <= ?`, now)
	if err != nil {
		return nil, fmt.Errorf("find expired clips: %w", err)
	}
	expiredClips := make([]ExpiredClip, 0)
	for rows.Next() {
		var item ExpiredClip
		if err := rows.Scan(&item.ID, &item.UserID, &item.StoragePath); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan expired clip: %w", err)
		}
		expiredClips = append(expiredClips, item)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close expired rows: %w", err)
	}
	for _, item := range expiredClips {
		if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ?`, item.ID); err != nil {
			return nil, fmt.Errorf("delete expired clip: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit cleanup: %w", err)
	}
	return expiredClips, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanClip(row scanner) (model.Clip, error) {
	var clip model.Clip
	var formatsJSON string
	var createdAt string
	var expiresAt sql.NullString
	if err := row.Scan(
		&clip.UserID,
		&clip.ID,
		&clip.Kind,
		&clip.DeviceID,
		&clip.DeviceName,
		&clip.MimeType,
		&clip.Name,
		&clip.Size,
		&formatsJSON,
		&clip.Preview,
		&clip.StoragePath,
		&createdAt,
		&expiresAt,
	); err != nil {
		return model.Clip{}, err
	}
	if err := json.Unmarshal([]byte(formatsJSON), &clip.Formats); err != nil {
		return model.Clip{}, fmt.Errorf("decode clipboard formats: %w", err)
	}
	clip.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if expiresAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, expiresAt.String)
		if err == nil {
			clip.ExpiresAt = &parsed
		}
	}
	return clip, nil
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

func validateUsername(username string) error {
	if username == "" || len([]byte(username)) > 64 || strings.ContainsAny(username, " \t\r\n") {
		return errors.New("username must be 1-64 characters without spaces")
	}
	return nil
}

func preview(value string) string {
	const maxRunes = 4000
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
