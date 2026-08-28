package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
)

func (s *Store) CreateTextForUser(ctx context.Context, userID, deviceID, deviceName string, formats map[string]string) (model.Clip, error) {
	if formats == nil {
		formats = map[string]string{}
	}
	plain := strings.TrimSpace(formats["text/plain"])
	if plain == "" {
		return model.Clip{}, errors.New("text/plain is required")
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

func (s *Store) ListForUser(ctx context.Context, userID, query, kind string, limit int) ([]model.Clip, error) {
	if limit < 1 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	statement := `SELECT user_id, id, kind, device_id, device_name, mime_type, name, size, formats_json, preview, storage_path, created_at, expires_at FROM clips WHERE user_id = ?`
	args := make([]any, 0, 7)
	args = append(args, userID)
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

func (s *Store) GetForUser(ctx context.Context, userID, id string) (model.Clip, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, id, kind, device_id, device_name, mime_type, name, size, formats_json, preview, storage_path, created_at, expires_at
		FROM clips
		WHERE id = ? AND user_id = ?
	`, id, userID)
	clip, err := scanClip(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Clip{}, ErrNotFound
	}
	if err != nil {
		return model.Clip{}, fmt.Errorf("get clip: %w", err)
	}
	return clip, nil
}

func (s *Store) DeleteForUser(ctx context.Context, userID, id string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin delete: %w", err)
	}
	defer tx.Rollback()
	var storagePath string
	err = tx.QueryRowContext(
		ctx,
		`SELECT storage_path FROM clips WHERE id = ? AND user_id = ?`,
		id,
		userID,
	).Scan(&storagePath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find clip for delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM clips WHERE id = ? AND user_id = ?`, id, userID); err != nil {
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
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, storage_path
		FROM clips
		WHERE expires_at IS NOT NULL AND expires_at <= ?
	`, now)
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

func preview(value string) string {
	const maxRunes = 4000
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}
