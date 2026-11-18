// Package store opens the SQLite database and applies schema migrations.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"modernc.org/sqlite" // pure-Go SQLite driver, registers "sqlite"

	"github.com/Rakeemovic/slowpoke-oasis/backend/migrations"
)

// connParams are applied by the driver to every new connection. Setting the
// pragmas here instead of with a one-time statement keeps foreign keys enforced
// even if database/sql has to replace the connection.
const connParams = "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

// Open opens (or creates) the SQLite database at path, enables foreign keys
// and applies all pending migrations. Use ":memory:" for tests.
func Open(path string) (*sql.DB, error) {
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return open(path, names)
}

// open is Open with an explicit list of migrations, so tests can stop at an
// earlier schema version.
func open(path string, migrationNames []string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+connParams)
	if err != nil {
		return nil, err
	}

	// A single connection keeps an in-memory database alive for the lifetime
	// of the handle and serializes all writes.
	db.SetMaxOpenConns(1)

	if err := migrate(db, migrationNames); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrate applies every named migration that has not been applied yet.
func migrate(db *sql.DB, names []string) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	for _, name := range names {
		if err := applyMigration(db, name); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

// applyMigration runs one migration file and records it, in one transaction.
func applyMigration(db *sql.DB, name string) error {
	var applied bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = ?)`, name).Scan(&applied); err != nil {
		return err
	}
	if applied {
		return nil
	}

	script, err := fs.ReadFile(migrations.FS, name)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if _, err := tx.Exec(string(script)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (name) VALUES (?)`, name); err != nil {
		return err
	}
	return tx.Commit()
}

// SQLite result codes for violated constraints. The primary code is in the
// low byte; the extended codes say which kind of constraint failed.
const (
	codeConstraint           = 19
	codeConstraintPrimaryKey = codeConstraint | 6<<8
	codeConstraintUnique     = codeConstraint | 8<<8
)

// IsConstraintViolation reports whether err was caused by a database rule:
// a CHECK, UNIQUE, NOT NULL or foreign key constraint, or a trigger.
func IsConstraintViolation(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && e.Code()&0xff == codeConstraint
}

// IsUniqueViolation reports whether err was caused by a value that already
// exists in a UNIQUE index or a primary key.
func IsUniqueViolation(err error) bool {
	var e *sqlite.Error
	if !errors.As(err, &e) {
		return false
	}
	return e.Code() == codeConstraintUnique || e.Code() == codeConstraintPrimaryKey
}
