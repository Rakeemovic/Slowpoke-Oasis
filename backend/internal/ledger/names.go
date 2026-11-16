package ledger

import (
	"errors"
	"strings"
)

// ErrNameTaken is returned when another category or active pot of the ledger
// already has the name (rules K1 and T1). Names are compared ignoring case.
var ErrNameTaken = errors.New("a category or pot with this name already exists")

// nameError turns a violation of the unique name indexes into ErrNameTaken.
// Other errors are returned unchanged.
func nameError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "category_name_per_ledger") || strings.Contains(msg, "pot_name_active_per_ledger") {
		return ErrNameTaken
	}
	return err
}
