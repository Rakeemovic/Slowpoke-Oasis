package auth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// registerBob creates the account "bob" and returns its ID and recovery code.
func registerBob(t *testing.T, s *Service) (userID, code string) {
	t.Helper()
	userID, code, err := s.Register(context.Background(), "bob", "correct horse battery", "", nil)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return userID, code
}

// mustLogin logs in and fails the test if that does not work.
func mustLogin(t *testing.T, s *Service, username, password string) string {
	t.Helper()
	token, err := s.Login(context.Background(), username, password)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return token
}

func TestWordlistHas7776UniqueWords(t *testing.T) {
	if len(wordlist) != 7776 {
		t.Fatalf("len = %d, want 7776", len(wordlist))
	}
	sorted := slices.Clone(wordlist)
	slices.Sort(sorted)
	if len(slices.Compact(sorted)) != len(wordlist) {
		t.Fatal("word list contains duplicates")
	}
	if wordlist[0] != "abacus" || wordlist[len(wordlist)-1] != "zoom" {
		t.Fatalf("first/last = %q/%q, the dice digits were not stripped", wordlist[0], wordlist[len(wordlist)-1])
	}
}

func TestRecoveryCodeIsSixWordsFromTheList(t *testing.T) {
	code, hash := newRecoveryCode()
	words := strings.Split(code, " ")
	if len(words) != recoveryWords {
		t.Fatalf("code %q has %d words, want %d", code, len(words), recoveryWords)
	}
	for _, w := range words {
		if !slices.Contains(wordlist, w) {
			t.Errorf("word %q is not in the list", w)
		}
	}
	if !verifyPassword(code, hash) {
		t.Error("hash does not match the code")
	}
	if other, _ := newRecoveryCode(); other == code {
		t.Error("two codes are equal")
	}
}

func TestNormalizeRecoveryCodeIsLenientAboutFormatting(t *testing.T) {
	got := normalizeRecoveryCode("  Abacus,  ZOOM\tt-shirt\n yo-yo.")
	if want := "abacus zoom t-shirt yo-yo"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRecoveryCodeIsStoredOnlyAsHash(t *testing.T) {
	db, s := newService(t)
	_, code := registerBob(t, s)

	var stored string
	if err := db.QueryRow(`SELECT recovery_hash FROM user WHERE username = 'bob'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, strings.Split(code, " ")[0]) || !strings.HasPrefix(stored, "argon2id$") {
		t.Fatalf("stored value %q is not an Argon2id hash of the code", stored)
	}
}

func TestRecoverSetsPasswordAndReplacesTheCode(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	_, code := registerBob(t, s)
	oldSession := mustLogin(t, s, "bob", "correct horse battery")

	// Typed in capitals and with commas, as users might.
	typed := strings.ToUpper(strings.ReplaceAll(code, " ", ", "))
	newCode, err := s.Recover(ctx, "BOB", typed, "new password 1")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if newCode == code || len(strings.Split(newCode, " ")) != recoveryWords {
		t.Fatalf("new code %q: want six fresh words", newCode)
	}

	mustLogin(t, s, "bob", "new password 1")
	if _, err := s.Login(ctx, "bob", "correct horse battery"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password: got %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.UserForToken(ctx, oldSession); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("old session: got %v, want ErrUnauthenticated", err)
	}
	// The old code is used up, the new one works.
	if _, err := s.Recover(ctx, "bob", code, "new password 2"); !errors.Is(err, ErrInvalidRecovery) {
		t.Errorf("old code: got %v, want ErrInvalidRecovery", err)
	}
	if _, err := s.Recover(ctx, "bob", newCode, "new password 2"); err != nil {
		t.Errorf("new code: %v", err)
	}
}

func TestRecoverGivesTheSameErrorForUnknownUserAndWrongCode(t *testing.T) {
	db, s := newService(t)
	ctx := context.Background()
	_, code := registerBob(t, s)

	_, errWrongCode := s.Recover(ctx, "bob", "abacus zoom abacus zoom abacus zoom", "new password 1")
	_, errUnknown := s.Recover(ctx, "nobody", code, "new password 1")
	if !errors.Is(errWrongCode, ErrInvalidRecovery) || !errors.Is(errUnknown, ErrInvalidRecovery) {
		t.Fatalf("wrong code: %v, unknown user: %v; want ErrInvalidRecovery for both", errWrongCode, errUnknown)
	}

	// Accounts from before recovery codes have none and cannot be recovered.
	if _, err := db.Exec(`UPDATE user SET recovery_hash = NULL WHERE username = 'bob'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Recover(ctx, "bob", code, "new password 1"); !errors.Is(err, ErrInvalidRecovery) {
		t.Fatalf("no code stored: got %v, want ErrInvalidRecovery", err)
	}
}

func TestRecoverChecksTheNewPassword(t *testing.T) {
	_, s := newService(t)
	_, code := registerBob(t, s)
	if _, err := s.Recover(context.Background(), "bob", code, "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("got %v, want ErrInvalidPassword", err)
	}
	// The code was not used up by the failed attempt.
	if _, err := s.Recover(context.Background(), "bob", code, "new password 1"); err != nil {
		t.Fatalf("recover: %v", err)
	}
}

func TestResetByOperator(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	_, oldCode := registerBob(t, s)
	oldSession := mustLogin(t, s, "bob", "correct horse battery")

	password, code, err := s.ResetByOperator(ctx, "Bob")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if len(strings.Split(password, " ")) != temporaryPasswordWords {
		t.Errorf("password %q: want %d words", password, temporaryPasswordWords)
	}
	mustLogin(t, s, "bob", password)
	if _, err := s.UserForToken(ctx, oldSession); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("old session: got %v, want ErrUnauthenticated", err)
	}
	if _, err := s.Recover(ctx, "bob", oldCode, "new password 1"); !errors.Is(err, ErrInvalidRecovery) {
		t.Errorf("old code: got %v, want ErrInvalidRecovery", err)
	}
	if _, err := s.Recover(ctx, "bob", code, "new password 1"); err != nil {
		t.Errorf("new code: %v", err)
	}
}

func TestResetByOperatorRejectsUnknownUsers(t *testing.T) {
	_, s := newService(t)
	// The placeholder "former-member" has no username and must stay locked.
	for _, name := range []string{"nobody", "former-member", ""} {
		if _, _, err := s.ResetByOperator(context.Background(), name); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("%q: got %v, want ErrUserNotFound", name, err)
		}
	}
}

func TestChangePasswordKeepsTheCurrentSessionAndTheCode(t *testing.T) {
	_, s := newService(t)
	ctx := context.Background()
	userID, code := registerBob(t, s)
	current := mustLogin(t, s, "bob", "correct horse battery")
	other := mustLogin(t, s, "bob", "correct horse battery")

	if err := s.ChangePassword(ctx, userID, current, "wrong password", "new password 1"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong current password: got %v, want ErrWrongPassword", err)
	}
	if err := s.ChangePassword(ctx, userID, current, "correct horse battery", "short"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("short new password: got %v, want ErrInvalidPassword", err)
	}
	if err := s.ChangePassword(ctx, userID, current, "correct horse battery", "new password 1"); err != nil {
		t.Fatalf("change: %v", err)
	}

	mustLogin(t, s, "bob", "new password 1")
	if _, err := s.Login(ctx, "bob", "correct horse battery"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("old password: got %v, want ErrInvalidCredentials", err)
	}
	if _, err := s.UserForToken(ctx, current); err != nil {
		t.Errorf("current session: %v, want it to stay", err)
	}
	if _, err := s.UserForToken(ctx, other); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("other session: got %v, want ErrUnauthenticated", err)
	}
	if _, err := s.Recover(ctx, "bob", code, "new password 2"); err != nil {
		t.Errorf("recovery code after a change: %v, want it to stay valid", err)
	}
}
