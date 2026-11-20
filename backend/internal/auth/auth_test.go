package auth

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

func newService(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, New(db)
}

func TestRegisterThenLoginReturnsUsableToken(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()

	userID, err := s.Register(ctx, "Bob_1", "bob@example.com", "correct horse battery", "Bob", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	token, err := s.Login(ctx, "bob_1", "correct horse battery") // login name is case-insensitive
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	got, err := s.UserForToken(ctx, token)
	if err != nil {
		t.Fatalf("token lookup: %v", err)
	}
	if got != userID {
		t.Errorf("token maps to %q, want %q", got, userID)
	}
}

func TestWrongPasswordAndUnknownUserGiveTheSameError(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := s.Login(ctx, "bob", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password: err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.Login(ctx, "nobody", "correct horse battery"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user: err = %v, want ErrInvalidCredentials", err)
	}
}

func TestUsernamesAreUniqueIgnoringCase(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := s.Register(ctx, "BOB", "other@example.com", "another password", "", nil); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestInvalidRegistrationInput(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()

	if _, err := s.Register(ctx, "ab", "bob@example.com", "correct horse battery", "", nil); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("short username: err = %v, want ErrInvalidUsername", err)
	}
	if _, err := s.Register(ctx, "bob smith", "bob@example.com", "correct horse battery", "", nil); !errors.Is(err, ErrInvalidUsername) {
		t.Errorf("username with space: err = %v, want ErrInvalidUsername", err)
	}
	if _, err := s.Register(ctx, "bob", "bob@example.com", "short", "", nil); !errors.Is(err, ErrInvalidPassword) {
		t.Errorf("short password: err = %v, want ErrInvalidPassword", err)
	}
}

func TestPasswordIsNotStoredInPlainText(t *testing.T) {
	db, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}

	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM user WHERE username = 'bob'`).Scan(&hash); err != nil {
		t.Fatalf("read hash: %v", err)
	}
	if hash == "correct horse battery" || hash == "" {
		t.Fatalf("stored value %q does not look hashed", hash)
	}
}

func TestSessionTokenIsStoredOnlyAsHash(t *testing.T) {
	db, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	token, err := s.Login(ctx, "bob", "correct horse battery")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	var found int
	if err := db.QueryRow(`SELECT COUNT(*) FROM session WHERE token_hash = ?`, token).Scan(&found); err != nil {
		t.Fatalf("query: %v", err)
	}
	if found != 0 {
		t.Error("the raw token is stored in the database; only its hash should be")
	}
}

func TestLogoutInvalidatesToken(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	token, err := s.Login(ctx, "bob", "correct horse battery")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := s.Logout(ctx, token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := s.UserForToken(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("after logout: err = %v, want ErrUnauthenticated", err)
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	db, s := newService(t)
	ctx := context.Background()
	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	token, err := s.Login(ctx, "bob", "correct horse battery")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if _, err := db.Exec(`UPDATE session SET expires_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatalf("expire session: %v", err)
	}
	if _, err := s.UserForToken(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("expired token: err = %v, want ErrUnauthenticated", err)
	}
}

func TestEmptyTokenIsRejected(t *testing.T) {
	_, s := newService(t)
	if _, err := s.UserForToken(context.Background(), ""); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestRegisterRollsBackWhenSetupFails(t *testing.T) {
	db, s := newService(t)
	ctx := context.Background()
	setupErr := errors.New("setup failed")
	failingSetup := func(context.Context, *sql.Tx, string) error { return setupErr }

	if _, err := s.Register(ctx, "bob", "bob@example.com", "correct horse battery", "", failingSetup); !errors.Is(err, setupErr) {
		t.Fatalf("err = %v, want the setup error", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user WHERE username = 'bob'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("bob stored %d times, want 0 after a failed setup", n)
	}
}
