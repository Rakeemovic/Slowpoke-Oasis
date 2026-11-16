package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// captureMailer keeps the bodies of sent emails instead of sending them.
type captureMailer struct {
	bodies []string
}

func (m *captureMailer) Send(_ context.Context, _, _, body string) error {
	m.bodies = append(m.bodies, body)
	return nil
}

// newResetHandler returns routes with an empty database and a mailer that records mails.
func newResetHandler(t *testing.T) (http.Handler, *captureMailer) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	accounts := auth.New(db)
	mailer := &captureMailer{}
	accounts.SetMailer(mailer)
	return NewServer(ledger.New(db), accounts).Routes(), mailer
}

// resetCodeFromMail returns the code written into the reset email.
func resetCodeFromMail(t *testing.T, body string) string {
	t.Helper()
	const marker = "Dein Code: "
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no reset code in mail: %q", body)
	}
	return strings.Fields(body[i+len(marker):])[0]
}

func TestRegisterWithoutEmailIsRejected(t *testing.T) {
	h, _ := newResetHandler(t)
	rec := doToken(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"username": "bob", "password": "correct horse battery",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestPasswordResetOverHTTP(t *testing.T) {
	h, mailer := newResetHandler(t)
	register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/auth/password-reset", "", map[string]any{"email": "bob@example.com"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: status %d, want 202", rec.Code)
	}
	code := resetCodeFromMail(t, mailer.bodies[0])

	rec = doToken(t, h, http.MethodPost, "/auth/password-reset/confirm", "", map[string]any{
		"token": code, "new_password": "a new password",
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("confirm: status %d, want 204, body %s", rec.Code, rec.Body)
	}

	rec = doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": "bob", "password": "correct horse battery",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("old password: status %d, want 401", rec.Code)
	}
	rec = doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": "bob", "password": "a new password",
	})
	if rec.Code != http.StatusOK {
		t.Errorf("new password: status %d, want 200", rec.Code)
	}

	rec = doToken(t, h, http.MethodPost, "/auth/password-reset/confirm", "", map[string]any{
		"token": code, "new_password": "another password",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("reused code: status %d, want 400", rec.Code)
	}
}

func TestPasswordResetAnswersTheSameForUnknownAddress(t *testing.T) {
	h, mailer := newResetHandler(t)

	rec := doToken(t, h, http.MethodPost, "/auth/password-reset", "", map[string]any{"email": "nobody@example.com"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, want 202 so that registered addresses stay hidden", rec.Code)
	}
	if len(mailer.bodies) != 0 {
		t.Fatalf("%d mails sent for an unknown address, want 0", len(mailer.bodies))
	}
}

func TestPasswordResetRejectsBadInput(t *testing.T) {
	h, _ := newResetHandler(t)

	rec := doToken(t, h, http.MethodPost, "/auth/password-reset", "", map[string]any{"email": "not-an-email"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed address: status %d, want 400", rec.Code)
	}
	rec = doToken(t, h, http.MethodPost, "/auth/password-reset/confirm", "", map[string]any{
		"token": "nonsense", "new_password": "a new password",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown code: status %d, want 400", rec.Code)
	}
	rec = doToken(t, h, http.MethodPost, "/auth/password-reset/confirm", "", map[string]any{
		"token": "nonsense", "new_password": "short",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("short password: status %d, want 400", rec.Code)
	}
}

func TestPasswordResetNeedsMailSetup(t *testing.T) {
	h := newTestHandler(t) // no mailer configured
	rec := doToken(t, h, http.MethodPost, "/auth/password-reset", "", map[string]any{"email": "alice@example.com"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}
