package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
)

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
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now); err != nil {
		return fmt.Errorf("cleanup sessions: %w", err)
	}
	return nil
}
