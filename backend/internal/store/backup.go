package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// BackupsKept is how many daily backups stay in the backup directory.
const BackupsKept = 14

const (
	backupPrefix = "slowpoke-"
	backupSuffix = ".db"
)

// BackupPath is the file in dir that holds the backup of the day of now. Days
// are counted in UTC, so a change of time zone cannot skip or repeat one.
func BackupPath(dir string, now time.Time) string {
	return filepath.Join(dir, backupPrefix+now.UTC().Format(time.DateOnly)+backupSuffix)
}

// Backup writes a copy of the database as the backup of the day of now,
// replacing an earlier one of that day, removes all but the newest
// BackupsKept backups and returns the path of the new one. The server may keep
// running: SQLite takes the copy from one consistent state.
func Backup(ctx context.Context, db *sql.DB, dir string, now time.Time) (string, error) {
	// The copy holds password hashes and every booking: only for the server.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	path := BackupPath(dir, now)
	if err := copyDatabase(ctx, db, path); err != nil {
		return "", err
	}
	if err := pruneBackups(dir, BackupsKept); err != nil {
		return "", fmt.Errorf("remove old backups: %w", err)
	}
	return path, nil
}

// copyDatabase writes the database to path. It writes to a temporary file
// first, so path never holds half a copy and an older backup survives a failure.
func copyDatabase(ctx context.Context, db *sql.DB, path string) error {
	tmp := path + ".tmp"
	// VACUUM INTO refuses to overwrite, and an interrupted run may have left one.
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unfinished backup: %w", err)
	}
	defer os.Remove(tmp) // no effect after a successful rename
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, tmp); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// pruneBackups removes the oldest backups until at most keep are left. Only
// files named like a daily backup count; anything else is left alone.
func pruneBackups(dir string, keep int) error {
	// The pattern spells out the date, and ISO dates sort by age.
	names, err := filepath.Glob(filepath.Join(dir,
		backupPrefix+"[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]"+backupSuffix))
	if err != nil {
		return err
	}
	slices.Sort(names)
	for len(names) > keep {
		if err := os.Remove(names[0]); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
