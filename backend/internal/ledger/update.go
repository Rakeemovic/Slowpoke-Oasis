package ledger

import (
	"context"
	"database/sql"
	"time"
)

// TransactionPatch changes some fields of a transaction. Nil fields stay unchanged.
type TransactionPatch struct {
	Type             *TxType
	CategoryID       *string
	PotID            *string
	CounterpartPotID *string
	AmountCents      *int64
	BookedOn         *string
	Note             *string
}

// UpdateTransaction applies a patch to a transaction that is not soft-deleted.
//
// The result is checked with the same rules as a new transaction. The author
// stays the same. Archived pots reject a transaction only when it moves into
// them (rule T5), so existing bookings can still be corrected.
func (s *Store) UpdateTransaction(ctx context.Context, ledgerID, id string, p TransactionPatch, now time.Time) error {
	var (
		userID, typ, potID, counterpart string
		categoryID, note                sql.NullString
		amount                          int64
		bookedOn                        string
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, type, category_id, pot_id, COALESCE(counterpart_pot_id, ''),
		       amount_cents, booked_on, note
		FROM transactions
		WHERE id = ? AND ledger_id = ? AND deleted_at IS NULL`,
		id, ledgerID,
	).Scan(&userID, &typ, &categoryID, &potID, &counterpart, &amount, &bookedOn, &note)
	if err == sql.ErrNoRows {
		return ErrTransactionNotFound
	}
	if err != nil {
		return err
	}
	if typ == string(TypeTransferOut) || typ == string(TypeTransferIn) {
		return ErrLinkedTransaction
	}

	in := NewTransaction{
		LedgerID:         ledgerID,
		UserID:           userID,
		Type:             TxType(typ),
		CategoryID:       categoryID.String,
		PotID:            potID,
		CounterpartPotID: counterpart,
		AmountCents:      abs64(amount),
		BookedOn:         bookedOn,
		Note:             note.String,
	}
	if p.Type != nil {
		in.Type = *p.Type
	}
	if p.CategoryID != nil {
		in.CategoryID = *p.CategoryID
	}
	if p.PotID != nil {
		in.PotID = *p.PotID
	}
	if p.CounterpartPotID != nil {
		in.CounterpartPotID = *p.CounterpartPotID
	}
	if p.AmountCents != nil {
		in.AmountCents = *p.AmountCents
	}
	if p.BookedOn != nil {
		in.BookedOn = *p.BookedOn
	}
	if p.Note != nil {
		in.Note = *p.Note
	}

	if err := in.Validate(); err != nil {
		return err
	}
	if in.PotID == "" {
		if err := s.db.QueryRowContext(ctx,
			`SELECT id FROM pot WHERE ledger_id = ? AND is_default = 1`, ledgerID,
		).Scan(&in.PotID); err != nil {
			return err
		}
	}

	if in.PotID != potID {
		if err := s.ensurePotActive(ctx, ledgerID, in.PotID); err != nil {
			return err
		}
	}
	if in.CounterpartPotID != "" && in.CounterpartPotID != counterpart {
		if err := s.ensurePotActive(ctx, ledgerID, in.CounterpartPotID); err != nil {
			return err
		}
	}
	if in.Type != TypeTransfer && (in.Type != TxType(typ) || in.CategoryID != categoryID.String) {
		if err := s.ensureCategoryKind(ctx, ledgerID, in.CategoryID, in.Type); err != nil {
			return err
		}
	}

	stored := in.AmountCents
	if in.Type == TypeExpense {
		stored = -stored
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE transactions
		SET type = ?, category_id = ?, pot_id = ?, counterpart_pot_id = ?,
		    amount_cents = ?, booked_on = ?, note = ?, updated_at = ?
		WHERE id = ? AND ledger_id = ? AND deleted_at IS NULL`,
		string(in.Type), nullIfEmpty(in.CategoryID), in.PotID, nullIfEmpty(in.CounterpartPotID),
		stored, in.BookedOn, nullIfEmpty(in.Note), now.UTC().Format(time.RFC3339),
		id, ledgerID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrTransactionNotFound
	}
	return nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
