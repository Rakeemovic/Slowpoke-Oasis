package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"errors"
	"math/big"
	"strings"
)

// recoveryWords is the length of a recovery code. Six words from a list of
// 7776 give about 77 bits of randomness, far beyond what can be guessed
// within the rate limit.
const recoveryWords = 6

// temporaryPasswordWords is the length of the password an operator hands out
// after a reset on the server. It only has to last until the user changes it.
const temporaryPasswordWords = 4

// ErrInvalidRecovery is returned for an unknown username and for a wrong
// recovery code alike, so the answer does not reveal which accounts exist.
var ErrInvalidRecovery = errors.New("wrong username or recovery code")

// effWordlist is the EFF Large Wordlist (see wordlist/README.md). Each line
// holds five dice digits, a tab and the word.
//
//go:embed wordlist/eff_large_wordlist.txt
var effWordlist string

// wordlist holds the words of effWordlist, in file order.
var wordlist = parseWordlist(effWordlist)

// parseWordlist extracts the words from the embedded file. The file is part
// of the program, so a malformed line is a programming error and panics.
func parseWordlist(raw string) []string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	words := make([]string, 0, len(lines))
	for _, line := range lines {
		_, word, ok := strings.Cut(line, "\t")
		word = strings.TrimSpace(word)
		if !ok || word == "" {
			panic("auth: malformed word list line: " + line)
		}
		words = append(words, word)
	}
	return words
}

// randomWords returns n words from the word list, chosen uniformly with a
// cryptographic random source and separated by single spaces. Spaces are
// used because a few words contain a hyphen ("t-shirt").
func randomWords(n int) string {
	max := big.NewInt(int64(len(wordlist)))
	words := make([]string, n)
	for i := range words {
		idx, _ := rand.Int(rand.Reader, max) // never fails, see newToken
		words[i] = wordlist[idx.Int64()]
	}
	return strings.Join(words, " ")
}

// newRecoveryCode returns a fresh recovery code and its hash for storage.
func newRecoveryCode() (code, hash string) {
	code = randomWords(recoveryWords)
	return code, hashPassword(code)
}

// normalizeRecoveryCode brings a typed code into the form it was hashed in:
// lowercase words separated by single spaces. Anything other than letters and
// hyphens counts as a separator, so "Apple, Banana" and "apple banana" match.
func normalizeRecoveryCode(code string) string {
	words := strings.FieldsFunc(strings.ToLower(code), func(c rune) bool {
		return (c < 'a' || c > 'z') && c != '-'
	})
	return strings.Join(words, " ")
}

// Recover sets a new password for an account with its recovery code. The code
// is used up: the answer is a new code, which the user has to keep instead.
// All logins of the account end, as after any password reset.
func (s *Service) Recover(ctx context.Context, username, recoveryCode, newPassword string) (string, error) {
	if err := validatePassword(newPassword); err != nil {
		return "", err
	}
	u := foldUsername(username)
	now := s.now()
	// Checked for every name, known or not, so the limit reveals nothing.
	if s.recoveryFailures.blocked(u, now) {
		return "", ErrTooManyAttempts
	}

	code := normalizeRecoveryCode(recoveryCode)
	var userID string
	var oldHash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, recovery_hash FROM user WHERE lower(username) = ?`, u,
	).Scan(&userID, &oldHash)
	if errors.Is(err, sql.ErrNoRows) {
		verifyPassword(code, dummyHash) // equalize the response time
		s.recoveryFailures.record(u, now)
		return "", ErrInvalidRecovery
	}
	if err != nil {
		return "", err
	}
	if !oldHash.Valid || !verifyPassword(code, oldHash.String) {
		s.recoveryFailures.record(u, now)
		return "", ErrInvalidRecovery
	}

	newCode, newHash := newRecoveryCode()
	// Matching the old hash makes the code single-use even if two requests
	// with the same code arrive at once: only the first one changes the row.
	replaced, err := s.setCredentials(ctx, userID, hashPassword(newPassword), newHash, oldHash.String)
	if err != nil {
		return "", err
	}
	if !replaced {
		return "", ErrInvalidRecovery
	}
	s.recoveryFailures.reset(u)
	s.loginFailures.reset(u) // the user can log in with the new password right away
	return newCode, nil
}

// ResetByOperator gives an account a new temporary password and a new
// recovery code and ends all its logins. It is meant for the person who runs
// the server (command "reset-password"), for users who lost both their
// password and their code. Unlike the API, it says whether a user exists:
// whoever can run it has access to the database anyway.
func (s *Service) ResetByOperator(ctx context.Context, username string) (password, recoveryCode string, err error) {
	var userID string
	err = s.db.QueryRowContext(ctx,
		// The placeholder "former-member" has no username, so it never matches.
		`SELECT id FROM user WHERE lower(username) = ?`, foldUsername(username),
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrUserNotFound
	}
	if err != nil {
		return "", "", err
	}

	password = randomWords(temporaryPasswordWords)
	recoveryCode, recoveryHash := newRecoveryCode()
	if _, err := s.setCredentials(ctx, userID, hashPassword(password), recoveryHash, ""); err != nil {
		return "", "", err
	}
	return password, recoveryCode, nil
}

// setCredentials replaces the password and recovery hash of a user and ends
// all their sessions. If expectedRecoveryHash is not empty, the row is only
// changed while it still holds that hash; replaced reports whether it did.
func (s *Service) setCredentials(ctx context.Context, userID, passwordHash, recoveryHash, expectedRecoveryHash string) (replaced bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() // no effect after a successful Commit

	query := `UPDATE user SET password_hash = ?, recovery_hash = ? WHERE id = ?`
	args := []any{passwordHash, recoveryHash, userID}
	if expectedRecoveryHash != "" {
		query += ` AND recovery_hash = ?`
		args = append(args, expectedRecoveryHash)
	}
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return false, err
	}
	// Anyone who knew the old password loses access with it.
	if _, err := tx.ExecContext(ctx, `DELETE FROM session WHERE user_id = ?`, userID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
