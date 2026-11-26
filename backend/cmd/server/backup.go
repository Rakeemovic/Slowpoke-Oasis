package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// runBackup handles "server backup", for example right before an update. The
// server may keep running meanwhile.
func runBackup(args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New(usage)
	}
	dir := os.Getenv("BACKUP_DIR")
	if dir == "" {
		return errors.New("set BACKUP_DIR to the directory for backups")
	}
	db, err := openExisting()
	if err != nil {
		return err
	}
	defer db.Close()

	path, err := store.Backup(context.Background(), db, dir, time.Now())
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Backup written to %s\n", path)
	return nil
}

// backupDaily writes the backup of the day unless it exists already. An empty
// dir turns automatic backups off.
func backupDaily(ctx context.Context, db *sql.DB, dir string, now time.Time) {
	if dir == "" {
		return
	}
	if _, err := os.Stat(store.BackupPath(dir, now)); err == nil {
		return
	}
	if path, err := store.Backup(ctx, db, dir, now); err != nil {
		log.Printf("backup: %v", err)
	} else {
		log.Printf("backup written to %s", path)
	}
}
