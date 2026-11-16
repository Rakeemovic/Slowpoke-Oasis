package ledger

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Errors for household membership.
var (
	ErrAlreadyMember  = errors.New("user is already a member of this household")
	ErrMemberNotFound = errors.New("member not found")
	ErrNotMember      = errors.New("not a member of this household")
)

// Household is a household the user belongs to, with the user's role in it.
type Household struct {
	ID   string
	Name string
	Role string
}

// CreateHousehold creates a household with its shared ledger and default pot
// and makes the creator its owner. It returns the household ID and the ledger ID.
func (s *Store) CreateHousehold(ctx context.Context, ownerID, name string, now time.Time) (string, string, error) {
	n, err := cleanName(name)
	if err != nil {
		return "", "", err
	}
	householdID, err := newID()
	if err != nil {
		return "", "", err
	}
	ledgerID, err := newID()
	if err != nil {
		return "", "", err
	}
	potID, err := newID()
	if err != nil {
		return "", "", err
	}
	ts := now.UTC().Format(time.RFC3339)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO household (id, name, created_at) VALUES (?, ?, ?)`,
		householdID, n, ts,
	); err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO household_member (household_id, user_id, role, joined_at)
		 VALUES (?, ?, 'owner', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))`,
		householdID, ownerID,
	); err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ledger (id, name, household_id, payday_day, currency, created_at)
		VALUES (?, ?, ?, 1, 'EUR', ?)`,
		ledgerID, n, householdID, ts,
	); err != nil {
		return "", "", err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pot (id, ledger_id, name, kind, is_default)
		VALUES (?, ?, 'Monthly', 'monthly', 1)`,
		potID, ledgerID,
	); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return householdID, ledgerID, nil
}

// ListHouseholds returns the households a user belongs to, sorted by name.
func (s *Store) ListHouseholds(ctx context.Context, userID string) ([]Household, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.id, h.name, m.role
		FROM household h
		JOIN household_member m ON m.household_id = h.id
		WHERE m.user_id = ?
		ORDER BY h.name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Household
	for rows.Next() {
		var h Household
		if err := rows.Scan(&h.ID, &h.Name, &h.Role); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// HouseholdRole returns the role of a user in a household, or ErrNotMember.
func (s *Store) HouseholdRole(ctx context.Context, userID, householdID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM household_member WHERE household_id = ? AND user_id = ?`,
		householdID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotMember
	}
	return role, err
}

// AddHouseholdMember adds a user to a household as a regular member.
func (s *Store) AddHouseholdMember(ctx context.Context, householdID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO household_member (household_id, user_id, role, joined_at)
		 VALUES (?, ?, 'member', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))`,
		householdID, userID,
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return ErrAlreadyMember
	}
	return err
}

// LeaveHousehold removes a user from a household. The user's bookings stay
// in the household under the placeholder (rule X12). See LeaveHouseholdTx.
func (s *Store) LeaveHousehold(ctx context.Context, householdID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if err := LeaveHouseholdTx(ctx, tx, householdID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// LeaveHouseholdTx removes a user from a household inside an existing transaction.
//
//   - The user's bookings and deletions in the household are handed over to the
//     placeholder, so they stay visible to the other members (rule X12).
//   - If the user was the last member, the household and its ledger are deleted.
//   - If the user was an owner, the longest-standing member becomes owner.
func LeaveHouseholdTx(ctx context.Context, tx *sql.Tx, householdID, userID string) error {
	var role string
	err := tx.QueryRowContext(ctx,
		`SELECT role FROM household_member WHERE household_id = ? AND user_id = ?`,
		householdID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}

	scope := `ledger_id IN (SELECT id FROM ledger WHERE household_id = ?)`
	if _, err := tx.ExecContext(ctx,
		`UPDATE transactions SET user_id = ? WHERE user_id = ? AND `+scope,
		FormerMemberID, userID, householdID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE transactions SET deleted_by = ? WHERE deleted_by = ? AND `+scope,
		FormerMemberID, userID, householdID,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM household_member WHERE household_id = ? AND user_id = ?`,
		householdID, userID,
	); err != nil {
		return err
	}

	var remaining int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM household_member WHERE household_id = ?`, householdID,
	).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		return deleteHouseholdTx(ctx, tx, householdID)
	}

	if role == "owner" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE household_member SET role = 'owner'
			WHERE household_id = ?
			  AND rowid = (SELECT rowid FROM household_member WHERE household_id = ?
			               ORDER BY joined_at, rowid LIMIT 1)`,
			householdID, householdID,
		); err != nil {
			return err
		}
	}
	return nil
}

// LeaveAllHouseholdsTx removes a user from all households. It is used when an
// account is deleted.
func LeaveAllHouseholdsTx(ctx context.Context, tx *sql.Tx, userID string) error {
	ids, err := queryIDs(ctx, tx, `SELECT household_id FROM household_member WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := LeaveHouseholdTx(ctx, tx, id, userID); err != nil {
			return err
		}
	}
	return nil
}

// deleteHouseholdTx removes a household with its ledgers. It is only used when
// the last member has left, so no one can access the data any more.
func deleteHouseholdTx(ctx context.Context, tx *sql.Tx, householdID string) error {
	ids, err := queryIDs(ctx, tx, `SELECT id FROM ledger WHERE household_id = ?`, householdID)
	if err != nil {
		return err
	}
	if err := deleteLedgersTx(ctx, tx, ids); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM household WHERE id = ?`, householdID)
	return err
}
