package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoginIsBlockedAfterTenFailuresAndUnblocksLater(t *testing.T) {
	s, clock := newClockService(t)
	ctx := context.Background()
	if _, _, err := s.Register(ctx, "bob", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}

	for i := 0; i < maxFailedLogins; i++ {
		if _, err := s.Login(ctx, "bob", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("failure %d: got %v, want ErrInvalidCredentials", i+1, err)
		}
	}
	// Even the right password is refused while the block lasts.
	if _, err := s.Login(ctx, "bob", "correct horse battery"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("got %v, want ErrTooManyAttempts", err)
	}

	*clock = clock.Add(loginWindow + time.Minute)
	if _, err := s.Login(ctx, "bob", "correct horse battery"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

func TestSuccessfulLoginResetsTheFailureCount(t *testing.T) {
	s, _ := newClockService(t)
	ctx := context.Background()
	if _, _, err := s.Register(ctx, "bob", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < maxFailedLogins-1; i++ {
		s.Login(ctx, "bob", "wrong password")
	}
	if _, err := s.Login(ctx, "bob", "correct horse battery"); err != nil {
		t.Fatalf("login: %v", err)
	}
	for i := 0; i < maxFailedLogins-1; i++ {
		s.Login(ctx, "bob", "wrong password")
	}
	if _, err := s.Login(ctx, "bob", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v, want ErrInvalidCredentials, the count should have restarted", err)
	}
}

func TestRecoveryIsBlockedAfterFiveFailuresAndUnblocksLater(t *testing.T) {
	s, clock := newClockService(t)
	ctx := context.Background()
	_, code, err := s.Register(ctx, "bob", "correct horse battery", "", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < maxFailedRecoveries; i++ {
		if _, err := s.Recover(ctx, "bob", "wrong words", "new password 1"); !errors.Is(err, ErrInvalidRecovery) {
			t.Fatalf("failure %d: got %v, want ErrInvalidRecovery", i+1, err)
		}
	}
	// Even the right code is refused while the block lasts.
	if _, err := s.Recover(ctx, "bob", code, "new password 1"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("got %v, want ErrTooManyAttempts", err)
	}

	// The limit also applies to unknown names, so it reveals nothing.
	for i := 0; i < maxFailedRecoveries; i++ {
		s.Recover(ctx, "nobody", "wrong words", "new password 1")
	}
	if _, err := s.Recover(ctx, "nobody", "wrong words", "new password 1"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("unknown name: got %v, want ErrTooManyAttempts", err)
	}

	*clock = clock.Add(recoveryWindow + time.Minute)
	if _, err := s.Recover(ctx, "bob", code, "new password 1"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

func TestWrongCurrentPasswordCountsTowardsTheLoginLimit(t *testing.T) {
	s, _ := newClockService(t)
	ctx := context.Background()
	userID, _, err := s.Register(ctx, "bob", "correct horse battery", "", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < maxFailedLogins; i++ {
		if err := s.ChangePassword(ctx, userID, "", "wrong password", "new password 1"); !errors.Is(err, ErrWrongPassword) {
			t.Fatalf("failure %d: got %v, want ErrWrongPassword", i+1, err)
		}
	}
	if err := s.ChangePassword(ctx, userID, "", "correct horse battery", "new password 1"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("got %v, want ErrTooManyAttempts", err)
	}
	if _, err := s.Login(ctx, "bob", "correct horse battery"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("login: got %v, want ErrTooManyAttempts", err)
	}
}

// newClockService returns a service with a clock the test controls.
func newClockService(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	_, s := newService(t)
	clock := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return clock }
	return s, &clock
}
