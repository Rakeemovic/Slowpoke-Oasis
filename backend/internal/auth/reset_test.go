package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
)

// fakeMailer records the bodies of all sent emails instead of sending them.
// Sending happens in the background, so tests use waitForMails.
type fakeMailer struct {
	mu     sync.Mutex
	bodies []string
}

func (f *fakeMailer) Send(_ context.Context, _, _, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bodies = append(f.bodies, body)
	return nil
}

// waitForMails waits until n mails have arrived and returns their bodies.
func waitForMails(t *testing.T, m *fakeMailer, n int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		if len(m.bodies) >= n {
			out := append([]string(nil), m.bodies...)
			m.mu.Unlock()
			return out
		}
		m.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("got fewer than %d mails", n)
		}
		time.Sleep(5 * time.Millisecond)
	}
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
	token := tokenFromMail(t, waitForMails(t, mailer, 1)[0])

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
	token := tokenFromMail(t, waitForMails(t, mailer, 1)[0])

	*clock = clock.Add(31 * time.Minute)
	if err := s.ConfirmPasswordReset(ctx, token, "a new password"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("got %v, want ErrInvalidResetToken", err)
	}
}

// Only the newest code is valid. Mails are sent in the background, so the
// order of the two mails is not fixed; exactly one of the two codes must work.
func TestNewResetRequestReplacesTheOldCode(t *testing.T) {
	s, mailer, _ := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}

	working := 0
	for _, body := range waitForMails(t, mailer, 2) {
		err := s.ConfirmPasswordReset(ctx, tokenFromMail(t, body), "a new password")
		switch {
		case err == nil:
			working++
		case !errors.Is(err, ErrInvalidResetToken):
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if working != 1 {
		t.Fatalf("%d codes work, want exactly 1", working)
	}
}

func TestUnknownEmailGetsNoErrorAndNoMail(t *testing.T) {
	s, mailer, _ := newResetService(t)
	if err := s.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("got %v, want nil so that registered addresses stay hidden", err)
	}
	time.Sleep(50 * time.Millisecond) // give a wrong background send time to happen
	mailer.mu.Lock()
	defer mailer.mu.Unlock()
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
	token := tokenFromMail(t, waitForMails(t, mailer, 1)[0])

	var stored string
	if err := db.QueryRow(`SELECT token_hash FROM password_reset`).Scan(&stored); err != nil {
		t.Fatalf("read stored code: %v", err)
	}
	if stored == token || stored != hashToken(token) {
		t.Fatalf("stored value %q is not the hash of the code", stored)
	}
}
