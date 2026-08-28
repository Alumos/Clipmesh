package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
	"golang.org/x/crypto/bcrypt"
)

// EnsureAdmin creates the first administrator when the configured account is
// absent. Existing password hashes are preserved, and ownerless legacy clips
// are assigned to this account during the upgrade.
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

// DeleteUser returns file paths for removal after the transaction commits.
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

func validateUsername(username string) error {
	if username == "" ||
		len([]byte(username)) > 64 ||
		strings.ContainsAny(username, " \t\r\n") {
		return errors.New("username must be 1-64 characters without spaces")
	}
	return nil
}
