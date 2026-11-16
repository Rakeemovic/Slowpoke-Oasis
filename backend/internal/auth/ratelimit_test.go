package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoginIsBlockedAfterTenFailuresAndUnblocksLater(t *testing.T) {
	s, _, clock := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
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
	s, _, _ := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
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

func TestResetRequestsAreLimitedPerAddress(t *testing.T) {
	s, _, clock := newResetService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", ""); err != nil {
		t.Fatalf("register: %v", err)
	}
	for i := 0; i < maxResetRequests; i++ {
		if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("got %v, want ErrTooManyAttempts", err)
	}

	// The limit also applies to unknown addresses, so it reveals nothing.
	for i := 0; i < maxResetRequests; i++ {
		s.RequestPasswordReset(ctx, "nobody@example.com")
	}
	if err := s.RequestPasswordReset(ctx, "nobody@example.com"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("unknown address: got %v, want ErrTooManyAttempts", err)
	}

	*clock = clock.Add(resetWindow + time.Minute)
	if err := s.RequestPasswordReset(ctx, "bob@example.com"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}
