package ledger

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors for creating users, ledgers and categories.
var (
	ErrInvalidName   = errors.New("name must have between 1 and 50 characters")
	ErrInvalidPayday = errors.New("payday_day must be a day between 1 and 31")
)

// cleanName trims a name and checks its length.
func cleanName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if l := utf8.RuneCountInString(n); l < 1 || l > 50 {
		return "", ErrInvalidName
	}
	return n, nil
}

// CreatePersonalLedger creates a ledger owned by a user together with its
// default monthly pot (rule L4). A paydayDay of 0 means the default, 1.
func (s *Store) CreatePersonalLedger(ctx context.Context, userID, name string, paydayDay int, now time.Time) (string, error) {
	n, err := cleanName(name)
	if err != nil {
		return "", err
	}
	if paydayDay == 0 {
		paydayDay = 1
	}
	if paydayDay < 1 || paydayDay > 31 {
		return "", ErrInvalidPayday
	}

	ledgerID, err := newID()
	if err != nil {
		return "", err
	}
	potID, err := newID()
	if err != nil {
		return "", err
	}
	ts := now.UTC().Format(time.RFC3339)

	// Ledger and default pot are created together or not at all.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ledger (id, name, user_id, payday_day, currency, created_at)
		VALUES (?, ?, ?, ?, 'EUR', ?)`,
		ledgerID, n, userID, paydayDay, ts,
	); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pot (id, ledger_id, name, kind, is_default)
		VALUES (?, ?, 'Monthly', 'monthly', 1)`,
		potID, ledgerID,
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return ledgerID, nil
}

// CreateCategory adds a category to a ledger and returns its ID.
// Names are unique per ledger, ignoring case (rule K1).
func (s *Store) CreateCategory(ctx context.Context, ledgerID, name string, isIncome bool) (string, error) {
	n, err := cleanName(name)
	if err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	income := 0
	if isIncome {
		income = 1
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO category (id, ledger_id, name, is_income) VALUES (?, ?, ?, ?)`,
		id, ledgerID, n, income,
	); err != nil {
		return "", err
	}
	return id, nil
}
