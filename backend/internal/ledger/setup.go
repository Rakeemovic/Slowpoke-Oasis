package ledger

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors for creating ledgers and categories.
var (
	ErrInvalidName   = errors.New("name must have between 1 and 50 characters")
	ErrInvalidPayday = errors.New("payday_day must be a day between 1 and 31")
)

// defaultPotName is the name of the monthly pot every ledger starts with.
const defaultPotName = "Monthly"

// cleanName trims a name and checks its length.
func cleanName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if l := utf8.RuneCountInString(n); l < 1 || l > 50 {
		return "", ErrInvalidName
	}
	return n, nil
}

// validPayday reports whether day can be a payday (rule L2).
func validPayday(day int) bool {
	return day >= 1 && day <= 31
}

// ledgerOwner says who a new ledger belongs to: exactly one of the fields is
// set (rule L1).
type ledgerOwner struct {
	UserID      string
	HouseholdID string
}

// insertLedger creates a ledger together with its default monthly pot (rule
// L4) and returns the ledger ID. It runs in tx, so both exist or neither does.
func insertLedger(ctx context.Context, tx *sql.Tx, owner ledgerOwner, name string, paydayDay int, now time.Time) (string, error) {
	ledgerID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ledger (id, name, user_id, household_id, payday_day, currency, created_at)
		VALUES (?, ?, ?, ?, ?, 'EUR', ?)`,
		ledgerID, name, nullIfEmpty(owner.UserID), nullIfEmpty(owner.HouseholdID), paydayDay, timestamp(now),
	); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pot (id, ledger_id, name, kind, is_default)
		VALUES (?, ?, ?, 'monthly', 1)`,
		newID(), ledgerID, defaultPotName,
	); err != nil {
		return "", err
	}
	if err := insertDefaultCategories(ctx, tx, ledgerID); err != nil {
		return "", err
	}
	return ledgerID, nil
}

// defaultCategory is a category every new ledger starts with (rule K4).
type defaultCategory struct {
	name     string
	isIncome bool
}

// defaultCategories lets a new ledger book right away instead of starting
// with an empty category list. Users rename or archive what they do not need.
// Migration 0012 holds a copy of this list for ledgers that existed before;
// a change here does not reach those ledgers.
var defaultCategories = []defaultCategory{
	{"Salary", true},
	{"Other income", true},
	{"Groceries", false},
	{"Rent", false},
	{"Utilities", false},
	{"Transport", false},
	{"Insurance", false},
	{"Health", false},
	{"Eating out", false},
	{"Leisure", false},
	{"Clothing", false},
	{"Subscriptions", false},
	{"Household", false},
	{"Other", false},
}

// insertDefaultCategories adds the default categories to a new ledger in tx.
func insertDefaultCategories(ctx context.Context, tx *sql.Tx, ledgerID string) error {
	for _, c := range defaultCategories {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO category (id, ledger_id, name, is_income) VALUES (?, ?, ?, ?)`,
			newID(), ledgerID, c.name, c.isIncome,
		); err != nil {
			return err
		}
	}
	return nil
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
	if !validPayday(paydayDay) {
		return "", ErrInvalidPayday
	}

	var ledgerID string
	err = s.inTx(ctx, func(tx *sql.Tx) (err error) {
		ledgerID, err = insertLedger(ctx, tx, ledgerOwner{UserID: userID}, n, paydayDay, now)
		return err
	})
	return ledgerID, err
}

// personalLedgerName is the name of the ledger every new account starts with.
const personalLedgerName = "Personal"

// CreateStarterLedger creates the primary personal ledger of a new user in
// tx, as part of the registration (rule L6).
func CreateStarterLedger(ctx context.Context, tx *sql.Tx, userID string, now time.Time) error {
	ledgerID, err := insertLedger(ctx, tx, ledgerOwner{UserID: userID}, personalLedgerName, 1, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE ledger SET is_primary = 1 WHERE id = ?`, ledgerID)
	return err
}

// PersonalLedgerID returns the user's primary personal ledger, the one created
// at registration (rule L6). Further personal ledgers are extras. Accounts
// without any personal ledger have none; then it returns "".
func (s *Store) PersonalLedgerID(ctx context.Context, userID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM ledger WHERE user_id = ? AND is_primary = 1`, userID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// Category is a category of a ledger. Archived categories are still listed,
// because older transactions refer to them and need their names (rule K2).
type Category struct {
	ID       string
	Name     string
	IsIncome bool
	Archived bool
}

// ListCategories returns the categories of a ledger, sorted by name.
func (s *Store) ListCategories(ctx context.Context, ledgerID string) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, is_income, archived
		FROM category
		WHERE ledger_id = ?
		ORDER BY lower(name)`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.IsIncome, &c.Archived); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory adds a category to a ledger and returns its ID.
// Names are unique per ledger, ignoring case (rule K1).
func (s *Store) CreateCategory(ctx context.Context, ledgerID, name string, isIncome bool) (string, error) {
	n, err := cleanName(name)
	if err != nil {
		return "", err
	}
	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO category (id, ledger_id, name, is_income) VALUES (?, ?, ?, ?)`,
		id, ledgerID, n, isIncome,
	); err != nil {
		return "", nameError(err)
	}
	return id, nil
}
