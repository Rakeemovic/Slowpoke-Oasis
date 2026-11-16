package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
)

// fakeMailer records the bodies of all sent emails instead of sending them.
type fakeMailer struct {
	bodies []string
}

func (f *fakeMailer) Send(_ context.Context, _, _, body string) error {
	f.bodies = append(f.bodies, body)
	return nil
}

// tokenFromMail returns the reset code written into the email body.
func tokenFromMail(t *testing.T, body string) string {
	t.Helper()
	const marker = "Dein Code: "
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("no reset code in mail: %q", body)
	}
	return strings.Fields(body[i+len(marker):])[0]
}

// newResetService returns a service with a fake mailer and a clock the test controls.
func newResetService(t *testing.T) (*Service, *fakeMailer, *time.Time) {
	t.Helper()
	_, s := newService(t)
	clock := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	mailer := &fakeMailer{}
	s.SetMailer(mailer)
	return s, mailer, &clock
}

func TestRegisterRequiresValidEmail(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()

	for _, email := range []string{"", "not-an-email", "bob@", "bob smith@example.com"} {
		if _, err := s.Register(ctx, "bob", email, "correct horse battery", ""); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("email %q: got %v, want ErrInvalidEmail", email, err)
		}
	}
}

func TestEmailMustBeUniqueIgnoringCase(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := s.Register(ctx, "carol", "BOB@example.com", "correct horse battery", ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("got %v, want ErrEmailTaken", err)
	}
}

func TestPasswordResetWorksOnceAndEndsOldLogins(t *testing.T) {
	s, mailer, _ := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	oldSession, err := s.Login(ctx, "bob", "correct horse battery")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := s.RequestPasswordReset(ctx, "BOB@example.com"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	if len(mailer.bodies) != 1 {
		t.Fatalf("got %d mails, want 1", len(mailer.bodies))
	}
	token := tokenFromMail(t, mailer.bodies[0])

	if err := s.ConfirmPasswordReset(ctx, token, "a new password"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := s.Login(ctx, "bob", "correct horse battery"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password: got %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Login(ctx, "bob", "a new password"); err != nil {
		t.Errorf("new password: %v", err)
	}
	if _, err := s.UserForToken(ctx, oldSession); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("old session: got %v, want ErrUnauthenticated", err)
	}
	if err := s.ConfirmPasswordReset(ctx, token, "another password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("reused code: got %v, want ErrInvalidResetToken", err)
	}
}

func TestResetCodeExpiresAfterThirtyMinutes(t *testing.T) {
	s, mailer, clock := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	token := tokenFromMail(t, mailer.bodies[0])

	*clock = clock.Add(31 * time.Minute)
	if err := s.ConfirmPasswordReset(ctx, token, "a new password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("got %v, want ErrInvalidResetToken", err)
	}
}

func TestNewResetRequestReplacesTheOldCode(t *testing.T) {
	s, mailer, _ := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
		t.Fatalf("second request: %v", err)
	}
	first := tokenFromMail(t, mailer.bodies[0])
	second := tokenFromMail(t, mailer.bodies[1])

	if err := s.ConfirmPasswordReset(ctx, first, "a new password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("old code: got %v, want ErrInvalidResetToken", err)
	}
	if err := s.ConfirmPasswordReset(ctx, second, "a new password"); err != nil {
		t.Errorf("newest code: %v", err)
	}
}

func TestUnknownEmailGetsNoErrorAndNoMail(t *testing.T) {
	s, mailer, _ := newResetService(t)
	if err := s.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("got %v, want nil so that registered addresses stay hidden", err)
	}
	if len(mailer.bodies) != 0 {
		t.Fatalf("got %d mails for an unknown address, want 0", len(mailer.bodies))
	}
}

func TestResetWithoutMailSetupFails(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); !errors.Is(err, mail.ErrNotConfigured) {
		t.Fatalf("got %v, want mail.ErrNotConfigured", err)
	}
}

func TestResetCodeIsStoredOnlyAsHash(t *testing.T) {
	db, s := newService(t)
	mailer := &fakeMailer{}
	s.SetMailer(mailer)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	token := tokenFromMail(t, mailer.bodies[0])

	var stored string
	if err := db.QueryRow(`SELECT token_hash FROM password_reset`).Scan(&stored); err != nil {
		t.Fatalf("read stored code: %v", err)
	}
	if stored == token || stored != hashToken(token) {
		t.Fatalf("stored value %q is not the hash of the code", stored)
	}
}
