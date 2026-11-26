package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

func TestRunBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "live.db")
	backups := filepath.Join(dir, "backups")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("BACKUP_DIR", backups)

	// A wrong DB_PATH must not produce an empty backup.
	if err := runBackup(nil, io.Discard); err == nil {
		t.Fatal("backup of a missing database: want an error")
	}

	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.Close()
	if err := runBackup(nil, io.Discard); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if _, err := os.Stat(store.BackupPath(backups, time.Now())); err != nil {
		t.Errorf("backup of today: %v", err)
	}
}
