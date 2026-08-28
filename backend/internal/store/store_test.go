package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestTextLimitKeepsNewestClips(t *testing.T) {
	clips, err := Open(filepath.Join(t.TempDir(), "clipmesh.db"), 2)
	if err != nil {
		t.Fatal(err)
	}
	defer clips.Close()
	admin, err := clips.EnsureAdmin(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{"first", "second", "third"} {
		if _, err := clips.CreateTextForUser(context.Background(), admin.ID, "device", "Test device", map[string]string{"text/plain": value}); err != nil {
			t.Fatal(err)
		}
	}
	items, err := clips.ListForUser(context.Background(), admin.ID, "", "text", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d text clips, want 2", len(items))
	}
	if items[0].Preview != "third" || items[1].Preview != "second" {
		t.Fatalf("unexpected newest order: %#v", items)
	}
}

func TestUsersIsolateClipboardData(t *testing.T) {
	clips, err := Open(filepath.Join(t.TempDir(), "clipmesh.db"), 10)
	if err != nil {
		t.Fatal(err)
	}
	defer clips.Close()

	admin, err := clips.EnsureAdmin(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := clips.CreateUser(context.Background(), "other", "other-password", "user")
	if err != nil {
		t.Fatal(err)
	}
	adminClip, err := clips.CreateTextForUser(context.Background(), admin.ID, "admin-device", "Admin", map[string]string{"text/plain": "admin only"})
	if err != nil {
		t.Fatal(err)
	}
	otherClip, err := clips.CreateTextForUser(context.Background(), other.ID, "other-device", "Other", map[string]string{"text/plain": "other only"})
	if err != nil {
		t.Fatal(err)
	}

	items, err := clips.ListForUser(context.Background(), other.ID, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != otherClip.ID {
		t.Fatalf("other user saw unexpected clips: %#v", items)
	}
	if _, err := clips.GetForUser(context.Background(), other.ID, adminClip.ID); err != ErrNotFound {
		t.Fatalf("cross-user get error = %v, want ErrNotFound", err)
	}
}

func TestLegacyClipsAreAssignedDuringMigration(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = db.Exec(`
		CREATE TABLE clips (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
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
		INSERT INTO clips (id, kind, formats_json, preview, created_at) VALUES ('legacy-clip', 'text', '{"text/plain":"legacy"}', 'legacy', ?);
	`, createdAt)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	clips, err := Open(databasePath, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer clips.Close()
	admin, err := clips.EnsureAdmin(context.Background(), "admin", "admin-password")
	if err != nil {
		t.Fatal(err)
	}
	items, err := clips.ListForUser(context.Background(), admin.ID, "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "legacy-clip" {
		t.Fatalf("legacy clips were not assigned to admin: %#v", items)
	}
}
