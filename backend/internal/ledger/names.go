package ledger

import (
	"errors"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// ErrNameTaken is returned when another category or active pot of the ledger
// already has the name (rules K1 and T1). Names are compared ignoring case.
var ErrNameTaken = errors.New("a category or pot with this name already exists")

// nameError turns a unique violation into ErrNameTaken. Other errors are
// returned unchanged.
//
// Callers only insert or update categories and pots with random IDs and
// without touching is_default, so the name indexes are the only unique rules
// such a statement can break.
func nameError(err error) error {
	if store.IsUniqueViolation(err) {
		return ErrNameTaken
	}
	return err
}
