package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupCopiesTheDatabase(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "live.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE note (text TEXT); INSERT INTO note VALUES ('first')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "backups")
	now := time.Date(2026, 12, 1, 8, 0, 0, 0, time.UTC)

	// A second backup of the same day replaces the first.
	for _, text := range []string{"second", "third"} {
		if _, err := db.Exec(`UPDATE note SET text = ?`, text); err != nil {
			t.Fatalf("update: %v", err)
		}
		path, err := Backup(context.Background(), db, dir, now)
		if err != nil {
			t.Fatalf("backup: %v", err)
		}

		copied, err := Open(path)
		if err != nil {
			t.Fatalf("open backup: %v", err)
		}
		var got string
		if err := copied.QueryRow(`SELECT text FROM note`).Scan(&got); err != nil {
			t.Fatalf("read backup: %v", err)
		}
		copied.Close()
		if got != text {
			t.Errorf("backup holds %q, want %q", got, text)
		}
	}

	info, err := os.Stat(BackupPath(dir, now))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
}

func TestBackupKeepsOnlyTheNewest(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	dir := t.TempDir()
	// A file of the user's own must survive the cleanup.
	own := filepath.Join(dir, "slowpoke-before-update.db")
	if err := os.WriteFile(own, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	start := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	for day := range BackupsKept + 2 {
		if _, err := Backup(context.Background(), db, dir, start.AddDate(0, 0, day)); err != nil {
			t.Fatalf("backup of day %d: %v", day, err)
		}
	}

	for day := range BackupsKept + 2 {
		_, err := os.Stat(BackupPath(dir, start.AddDate(0, 0, day)))
		if kept := err == nil; kept != (day >= 2) {
			t.Errorf("backup of day %d: kept = %v", day, kept)
		}
	}
	if _, err := os.Stat(own); err != nil {
		t.Errorf("own file: %v", err)
	}
}
