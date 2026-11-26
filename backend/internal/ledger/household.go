package ledger

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// Roles of a household member. Owners may add and remove other members.
const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

// FormerMemberID is the placeholder user that replaces a departed member in
// household transactions (rule X12). It is created by migration 0003 and has
// no username or password, so nobody can log in as it.
const FormerMemberID = "former-member"

// Errors for household membership.
var (
	ErrAlreadyMember  = errors.New("user is already a member of this household")
	ErrMemberNotFound = errors.New("member not found")
	ErrNotMember      = errors.New("not a member of this household")
	ErrNotOwner       = errors.New("only the owner can do this")
)

// Household is a household the user belongs to, with the user's role in it
// and the ID of its shared ledger (each household has exactly one, rule L5).
type Household struct {
	ID       string
	Name     string
	Role     string
	LedgerID string
}

// CreateHousehold creates a household with its shared ledger and default pot
// and makes the creator its owner. It returns the household ID and the ledger ID.
func (s *Store) CreateHousehold(ctx context.Context, ownerID, name string, now time.Time) (string, string, error) {
	n, err := cleanName(name)
	if err != nil {
		return "", "", err
	}
	householdID := newID()

	var ledgerID string
	err = s.inTx(ctx, func(tx *sql.Tx) (err error) {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO household (id, name, created_at) VALUES (?, ?, ?)`,
			householdID, n, timestamp(now),
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO household_member (household_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
			householdID, ownerID, RoleOwner, timestamp(now),
		); err != nil {
			return err
		}
		ledgerID, err = insertLedger(ctx, tx, ledgerOwner{HouseholdID: householdID}, n, 1, now)
		return err
	})
	if err != nil {
		return "", "", err
	}
	return householdID, ledgerID, nil
}

// ListHouseholds returns the households a user belongs to, sorted by name.
func (s *Store) ListHouseholds(ctx context.Context, userID string) ([]Household, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.id, h.name, m.role, l.id
		FROM household h
		JOIN household_member m ON m.household_id = h.id
		JOIN ledger l ON l.household_id = h.id
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
		if err := rows.Scan(&h.ID, &h.Name, &h.Role, &h.LedgerID); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Member is a member of a household as other members see it. It deliberately
// holds no personal data such as the display name.
type Member struct {
	UserID   string
	Username string
	Role     string
}

// ListMembers returns the members of a household in the order they joined,
// which is also the order of owner succession. Accounts from before logins
// existed have no username; they are listed with an empty one.
func (s *Store) ListMembers(ctx context.Context, householdID string) ([]Member, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.user_id, COALESCE(u.username, ''), m.role
		FROM household_member m
		JOIN user u ON u.id = m.user_id
		WHERE m.household_id = ?
		ORDER BY m.joined_at, m.rowid`,
		householdID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// HouseholdRole returns the role of a user in a household, or ErrNotMember.
func (s *Store) HouseholdRole(ctx context.Context, userID, householdID string) (string, error) {
	return memberRole(ctx, s.db, householdID, userID)
}

// memberRole returns the role of a user in a household, or ErrNotMember.
func memberRole(ctx context.Context, q queryer, householdID, userID string) (string, error) {
	var role string
	err := q.QueryRowContext(ctx,
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
		 VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))`,
		householdID, userID, RoleMember,
	)
	if store.IsUniqueViolation(err) {
		return ErrAlreadyMember
	}
	return err
}

// TransferOwnership makes another member the owner of a household. The
// current owner becomes a regular member, so a household keeps exactly one
// owner. It returns ErrNotOwner if ownerID is not the owner and
// ErrMemberNotFound if newOwnerID is not a member. Transferring to oneself
// changes nothing.
func (s *Store) TransferOwnership(ctx context.Context, householdID, ownerID, newOwnerID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		// The owner check is part of the transaction, so two transfers sent
		// at the same time cannot both succeed and leave two owners.
		role, err := memberRole(ctx, tx, householdID, ownerID)
		if err != nil {
			return err
		}
		if role != RoleOwner {
			return ErrNotOwner
		}
		if _, err := memberRole(ctx, tx, householdID, newOwnerID); errors.Is(err, ErrNotMember) {
			return ErrMemberNotFound
		} else if err != nil {
			return err
		}

		for _, change := range []struct{ userID, role string }{
			{ownerID, RoleMember},
			{newOwnerID, RoleOwner},
		} {
			if _, err := tx.ExecContext(ctx,
				`UPDATE household_member SET role = ? WHERE household_id = ? AND user_id = ?`,
				change.role, householdID, change.userID,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

// LeaveHousehold removes a user from a household. It returns ErrMemberNotFound
// if the user is not a member.
//
//   - The user's bookings and deletions in the household are handed over to the
//     placeholder, so they stay visible to the other members (rule X12).
//   - If the user was the last member, the household and its ledger are deleted.
//   - If the user was an owner, the longest-standing member becomes owner.
func (s *Store) LeaveHousehold(ctx context.Context, householdID, userID string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		return leaveHousehold(ctx, tx, householdID, userID)
	})
}

// leaveHousehold is LeaveHousehold inside an existing transaction.
func leaveHousehold(ctx context.Context, tx *sql.Tx, householdID, userID string) error {
	role, err := memberRole(ctx, tx, householdID, userID)
	if errors.Is(err, ErrNotMember) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}

	// Hand the user's traces in the household over to the placeholder.
	for _, column := range []string{"user_id", "deleted_by"} {
		if _, err := tx.ExecContext(ctx, `
			UPDATE transactions SET `+column+` = ?
			WHERE `+column+` = ? AND ledger_id IN (SELECT id FROM ledger WHERE household_id = ?)`,
			FormerMemberID, userID, householdID,
		); err != nil {
			return err
		}
	}

	// Reflections are personal data and go with the membership (rule W3).
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM weekly_review
		WHERE user_id = ? AND ledger_id IN (SELECT id FROM ledger WHERE household_id = ?)`,
		userID, householdID,
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
		return deleteHousehold(ctx, tx, householdID)
	}

	if role == RoleOwner {
		// The longest-standing member takes over. Members who joined in the
		// same second keep the order in which they were added.
		_, err := tx.ExecContext(ctx, `
			UPDATE household_member SET role = ?
			WHERE household_id = ?
			  AND rowid = (SELECT rowid FROM household_member WHERE household_id = ?
			               ORDER BY joined_at, rowid LIMIT 1)`,
			RoleOwner, householdID, householdID,
		)
		return err
	}
	return nil
}

// leaveAllHouseholds removes a user from all households. It is used when an
// account is deleted.
func leaveAllHouseholds(ctx context.Context, tx *sql.Tx, userID string) error {
	ids, err := queryIDs(ctx, tx, `SELECT household_id FROM household_member WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := leaveHousehold(ctx, tx, id, userID); err != nil {
			return err
		}
	}
	return nil
}

// deleteHousehold removes a household with its ledgers. It is only used when
// the last member has left, so no one can access the data any more.
func deleteHousehold(ctx context.Context, tx *sql.Tx, householdID string) error {
	ids, err := queryIDs(ctx, tx, `SELECT id FROM ledger WHERE household_id = ?`, householdID)
	if err != nil {
		return err
	}
	if err := deleteLedgers(ctx, tx, ids); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM household WHERE id = ?`, householdID)
	return err
}
