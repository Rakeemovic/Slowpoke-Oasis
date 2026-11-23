package ledger

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Errors for managing categories.
var (
	ErrCategoryNotFound    = errors.New("category not found")
	ErrCategoryLimit       = errors.New("a ledger can have at most 50 categories, archived ones included")
	ErrCategoryArchived    = errors.New("this category is archived and accepts no new transactions")
	ErrFallbackProtected   = errors.New("the catch-all categories cannot be archived or deleted")
	ErrFallbackNameTaken   = errors.New("another category already uses the name of the catch-all category; rename it first")
	ErrInvalidMoveTarget   = errors.New("bookings can only move to another active category of the same kind")
	ErrInvalidPurpose      = errors.New("purpose must be need, want, saving or empty")
	ErrPurposeNeedsExpense = errors.New("only expense categories can have a purpose")
)

// maxCategoriesPerLedger limits the categories of a ledger, archived ones
// included, so pickers and reports stay manageable (rule K7).
const maxCategoriesPerLedger = 50

// Purpose says what kind of spending an expense category stands for (rule K5).
// The empty Purpose means none.
type Purpose string

const (
	PurposeNeed   Purpose = "need"
	PurposeWant   Purpose = "want"
	PurposeSaving Purpose = "saving"
)

// valid reports whether p is one of the known purposes or empty.
func (p Purpose) valid() bool {
	switch p {
	case "", PurposeNeed, PurposeWant, PurposeSaving:
		return true
	}
	return false
}

// Names of the fallback categories that take over the transactions of deleted
// categories (rule K6). They are only used when a ledger has no fallback
// category yet; afterwards the category is found by its flag, so renaming it
// is fine.
const (
	fallbackExpenseName = "Other"
	fallbackIncomeName  = "Other income"
)

// fallbackName returns the name a new fallback category of the kind gets.
func fallbackName(isIncome bool) string {
	if isIncome {
		return fallbackIncomeName
	}
	return fallbackExpenseName
}

// Category is a category of a ledger. Archived categories are still listed,
// because older transactions refer to them and need their names (rule K2).
type Category struct {
	ID         string
	Name       string
	IsIncome   bool
	Archived   bool
	Purpose    Purpose
	IsFallback bool
	// TransactionCount is the number of transactions that are not deleted.
	// The app shows it before deleting the category.
	TransactionCount int
}

// NewCategory is the input for CreateCategory.
type NewCategory struct {
	LedgerID string
	Name     string
	IsIncome bool
	Purpose  Purpose
}

// CategoryUpdate changes an existing category. Nil fields stay unchanged.
// An empty Purpose removes the purpose.
type CategoryUpdate struct {
	Name     *string
	Purpose  *Purpose
	Archived *bool
}

// validatePurpose checks rule K5.
func validatePurpose(p Purpose, isIncome bool) error {
	if !p.valid() {
		return ErrInvalidPurpose
	}
	if p != "" && isIncome {
		return ErrPurposeNeedsExpense
	}
	return nil
}

// ListCategories returns the categories of a ledger, sorted by name.
func (s *Store) ListCategories(ctx context.Context, ledgerID string) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.is_income, c.archived, COALESCE(c.purpose, ''), c.is_fallback,
		       (SELECT COUNT(*) FROM transactions t
		         WHERE t.category_id = c.id AND t.deleted_at IS NULL)
		FROM category c
		WHERE c.ledger_id = ?
		ORDER BY lower(c.name)`,
		ledgerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.IsIncome, &c.Archived, &c.Purpose,
			&c.IsFallback, &c.TransactionCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory adds a category to a ledger and returns its ID. Names are
// unique per ledger, ignoring case (rule K1), and a ledger has at most 50
// categories (rule K7).
func (s *Store) CreateCategory(ctx context.Context, in NewCategory) (string, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return "", err
	}
	if err := validatePurpose(in.Purpose, in.IsIncome); err != nil {
		return "", err
	}

	id := newID()
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM category WHERE ledger_id = ?`, in.LedgerID,
		).Scan(&count); err != nil {
			return err
		}
		if count >= maxCategoriesPerLedger {
			return ErrCategoryLimit
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO category (id, ledger_id, name, is_income, purpose) VALUES (?, ?, ?, ?, ?)`,
			id, in.LedgerID, name, in.IsIncome, nullIfEmpty(string(in.Purpose)),
		)
		return nameError(err)
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// categoryState is what the update and delete rules need to know about a
// stored category.
type categoryState struct {
	name       string
	isIncome   bool
	archived   bool
	purpose    Purpose
	isFallback bool
}

// loadCategory reads a category of the ledger in tx.
func loadCategory(ctx context.Context, tx *sql.Tx, ledgerID, categoryID string) (categoryState, error) {
	var c categoryState
	err := tx.QueryRowContext(ctx, `
		SELECT name, is_income, archived, COALESCE(purpose, ''), is_fallback
		FROM category WHERE id = ? AND ledger_id = ?`,
		categoryID, ledgerID,
	).Scan(&c.name, &c.isIncome, &c.archived, &c.purpose, &c.isFallback)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrCategoryNotFound
	}
	return c, err
}

// UpdateCategory renames, archives or restores a category, or changes its
// purpose. Whether income or expense cannot change, because the existing
// transactions depend on it (rule K3). Fallback categories may be renamed but
// not archived (rule K6).
func (s *Store) UpdateCategory(ctx context.Context, ledgerID, categoryID string, u CategoryUpdate) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		c, err := loadCategory(ctx, tx, ledgerID, categoryID)
		if err != nil {
			return err
		}
		if u.Name != nil {
			if c.name, err = cleanName(*u.Name); err != nil {
				return err
			}
		}
		if u.Purpose != nil {
			c.purpose = Purpose(strings.TrimSpace(string(*u.Purpose)))
		}
		if u.Archived != nil {
			c.archived = *u.Archived
		}

		if err := validatePurpose(c.purpose, c.isIncome); err != nil {
			return err
		}
		if c.isFallback && c.archived {
			return ErrFallbackProtected
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE category SET name = ?, purpose = ?, archived = ?
			WHERE id = ? AND ledger_id = ?`,
			c.name, nullIfEmpty(string(c.purpose)), c.archived, categoryID, ledgerID,
		)
		return nameError(err)
	})
}

// DeletedCategory reports where the transactions of a deleted category went.
type DeletedCategory struct {
	TargetID          string
	MovedTransactions int64
}

// DeleteCategory removes a category (rule K2). Its transactions, including
// soft-deleted ones, move to targetID, and its planned amounts are added to
// the target's. The target must be another active category of the same kind
// (rule K8). An empty targetID means the fallback category of that kind (rule
// K6). Everything happens in one database transaction.
func (s *Store) DeleteCategory(ctx context.Context, ledgerID, categoryID, targetID string) (DeletedCategory, error) {
	var out DeletedCategory
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		c, err := loadCategory(ctx, tx, ledgerID, categoryID)
		if err != nil {
			return err
		}
		if c.isFallback {
			return ErrFallbackProtected
		}
		if out.TargetID, err = resolveMoveTarget(ctx, tx, ledgerID, categoryID, targetID, c.isIncome); err != nil {
			return err
		}
		if err := movePlannedAmounts(ctx, tx, ledgerID, categoryID, out.TargetID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE transactions SET category_id = ? WHERE ledger_id = ? AND category_id = ?`,
			out.TargetID, ledgerID, categoryID,
		)
		if err != nil {
			return err
		}
		if out.MovedTransactions, err = res.RowsAffected(); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`DELETE FROM category WHERE id = ? AND ledger_id = ?`, categoryID, ledgerID)
		return err
	})
	return out, err
}

// resolveMoveTarget returns the category that takes over the transactions of
// categoryID: targetID if it is allowed, or the fallback of the kind if
// targetID is empty.
func resolveMoveTarget(ctx context.Context, tx *sql.Tx, ledgerID, categoryID, targetID string, isIncome bool) (string, error) {
	if targetID == "" {
		fallbackID, err := ensureFallback(ctx, tx, ledgerID, isIncome)
		if err == nil && fallbackID == categoryID {
			// The category to delete was just marked as the fallback because
			// it carries the fallback name.
			return "", ErrFallbackProtected
		}
		return fallbackID, err
	}
	if targetID == categoryID {
		return "", ErrInvalidMoveTarget
	}
	target, err := loadCategory(ctx, tx, ledgerID, targetID)
	if errors.Is(err, ErrCategoryNotFound) {
		return "", ErrInvalidMoveTarget
	}
	if err != nil {
		return "", err
	}
	if target.isIncome != isIncome || target.archived {
		return "", ErrInvalidMoveTarget
	}
	return targetID, nil
}

// ensureFallback returns the fallback category of the kind and creates it if
// the ledger has none yet, for example because it is older than the default
// categories. A category that already carries the fallback name becomes the
// fallback instead, and is restored if it was archived. Creating it ignores
// the limit of rule K7: deleting must always be possible.
func ensureFallback(ctx context.Context, tx *sql.Tx, ledgerID string, isIncome bool) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM category WHERE ledger_id = ? AND is_income = ? AND is_fallback = 1`,
		ledgerID, isIncome,
	).Scan(&id)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}

	name := fallbackName(isIncome)
	var sameNameIncome bool
	err = tx.QueryRowContext(ctx,
		`SELECT id, is_income FROM category WHERE ledger_id = ? AND lower(name) = lower(?)`,
		ledgerID, name,
	).Scan(&id, &sameNameIncome)
	switch {
	case err == nil && sameNameIncome != isIncome:
		return "", ErrFallbackNameTaken
	case err == nil:
		_, err = tx.ExecContext(ctx,
			`UPDATE category SET is_fallback = 1, archived = 0 WHERE id = ?`, id)
		return id, err
	case !errors.Is(err, sql.ErrNoRows):
		return "", err
	}

	id = newID()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO category (id, ledger_id, name, is_income, is_fallback) VALUES (?, ?, ?, ?, 1)`,
		id, ledgerID, name, isIncome,
	)
	return id, err
}

// movePlannedAmounts adds the planned amounts of category from to those of
// category to, pot by pot, and removes the ones of from.
func movePlannedAmounts(ctx context.Context, tx *sql.Tx, ledgerID, from, to string) error {
	// "WHERE true" lets SQLite tell the upsert clause apart from a join.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pot_line (pot_id, ledger_id, category_id, amount_cents)
		SELECT pot_id, ledger_id, ?, amount_cents
		FROM pot_line WHERE ledger_id = ? AND category_id = ? AND true
		ON CONFLICT (pot_id, category_id)
		DO UPDATE SET amount_cents = amount_cents + excluded.amount_cents`,
		to, ledgerID, from,
	); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx,
		`DELETE FROM pot_line WHERE ledger_id = ? AND category_id = ?`, ledgerID, from)
	return err
}

// AuthorizeCategoryChange checks whether a user may rename, archive or delete
// the categories of a ledger (rule K9): the owner of a personal ledger, or the
// owner of the household. Every member may still create categories.
// It returns ErrNotOwner otherwise.
func (s *Store) AuthorizeCategoryChange(ctx context.Context, userID, ledgerID string) error {
	var isOwner bool
	err := s.db.QueryRowContext(ctx,
		`SELECT `+ownsLedger("l")+` FROM ledger l WHERE l.id = ?`,
		userID, userID, ledgerID,
	).Scan(&isOwner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLedgerNotFound
	}
	if err != nil {
		return err
	}
	if !isOwner {
		return ErrNotOwner
	}
	return nil
}

// ensureCategoryFits checks that income uses income categories and expenses
// use expense categories (rule K3). When the category is newly chosen, it
// must not be archived (rule K8); existing bookings keep an archived category.
// An unknown category is left to the foreign key.
func ensureCategoryFits(ctx context.Context, q queryer, ledgerID, categoryID string, typ TxType, newlyChosen bool) error {
	var isIncome, archived bool
	err := q.QueryRowContext(ctx,
		`SELECT is_income, archived FROM category WHERE id = ? AND ledger_id = ?`, categoryID, ledgerID,
	).Scan(&isIncome, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if isIncome != (typ == TypeIncome) {
		return ErrCategoryKindMismatch
	}
	if newlyChosen && archived {
		return ErrCategoryArchived
	}
	return nil
}
