package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// statusByError lists the errors a client may see, grouped by the HTTP status
// they map to. The text of these errors is sent to the client, so only errors
// that are safe to show belong here. Everything else answers 500.
var statusByError = []struct {
	status int
	errs   []error
}{
	// The request itself is wrong.
	{http.StatusBadRequest, []error{
		ledger.ErrLedgerRequired,
		ledger.ErrInvalidType,
		ledger.ErrAmountNotPositive,
		ledger.ErrInvalidDate,
		ledger.ErrCategoryRequired,
		ledger.ErrCategoryForbidden,
		ledger.ErrCounterpartRequired,
		ledger.ErrCounterpartForbidden,
		ledger.ErrSamePot,
		ledger.ErrNoteTooLong,
		ledger.ErrInvalidName,
		ledger.ErrInvalidPayday,
		ledger.ErrInvalidTarget,
		ledger.ErrTargetDateNeedsTarget,
		ledger.ErrInvalidAmount,
		ledger.ErrCategoryKindMismatch,
		ledger.ErrInvalidMonth,
		ledger.ErrInvalidPeriod,
		ledger.ErrSameLedger,
		auth.ErrInvalidUsername,
		auth.ErrInvalidPassword,
		auth.ErrInvalidEmail,
		auth.ErrInvalidResetToken,
	}},
	{http.StatusUnauthorized, []error{
		auth.ErrInvalidCredentials,
		auth.ErrUnauthenticated,
	}},
	// The user is known but may not do this.
	{http.StatusForbidden, []error{
		ledger.ErrNotAllowed,
		ledger.ErrNotOwner,
		auth.ErrWrongPassword,
	}},
	{http.StatusNotFound, []error{
		ledger.ErrLedgerNotFound,
		ledger.ErrTransactionNotFound,
		ledger.ErrPotNotFound,
		ledger.ErrPotLineNotFound,
		ledger.ErrMemberNotFound,
		ledger.ErrNotMember,
		auth.ErrUserNotFound,
	}},
	// The request is valid but does not fit the current state of the data.
	{http.StatusConflict, []error{
		ledger.ErrLinkedTransaction,
		ledger.ErrPotArchived,
		ledger.ErrDefaultPotProtected,
		ledger.ErrAlreadyMember,
		ledger.ErrNameTaken,
		auth.ErrUsernameTaken,
		auth.ErrEmailTaken,
	}},
	{http.StatusTooManyRequests, []error{
		auth.ErrTooManyAttempts,
	}},
	{http.StatusServiceUnavailable, []error{
		mail.ErrNotConfigured,
	}},
}

// fail writes the response for an error from the ledger, auth or mail package.
func fail(w http.ResponseWriter, err error) {
	for _, group := range statusByError {
		for _, known := range group.errs {
			if errors.Is(err, known) {
				// The known error's own text, without details added by wrapping.
				writeError(w, group.status, known.Error())
				return
			}
		}
	}
	if store.IsConstraintViolation(err) {
		// Database rules such as foreign keys and CHECK constraints.
		writeError(w, http.StatusUnprocessableEntity, "the request violates a data rule")
		return
	}
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
