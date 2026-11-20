// Package auth manages user accounts, password checks and login sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// SessionTTL is how long a login stays valid.
const SessionTTL = 30 * 24 * time.Hour

// Errors returned by the auth service. Callers can compare them with errors.Is.
var (
	ErrInvalidUsername    = errors.New("username must be 3 to 32 characters: lowercase letters, digits or underscore")
	ErrInvalidPassword    = errors.New("password must have between 8 and 256 characters")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrTooManyAttempts    = errors.New("too many attempts, please wait a while and try again")
	ErrInvalidCredentials = errors.New("wrong username or password")
	ErrUnauthenticated    = errors.New("not logged in or session expired")
	ErrWrongPassword      = errors.New("wrong password")
	ErrUserNotFound       = errors.New("user not found")
)

// Service handles registration, login, sessions and password recovery.
type Service struct {
	db               *sql.DB
	now              func() time.Time
	loginFailures    *limiter
	recoveryFailures *limiter
}

// New returns a Service backed by db.
func New(db *sql.DB) *Service {
	return &Service{
		db:               db,
		now:              time.Now,
		loginFailures:    newLimiter(maxFailedLogins, loginWindow),
		recoveryFailures: newLimiter(maxFailedRecoveries, recoveryWindow),
	}
}

// foldUsername brings a login name into the form it is stored and compared in.
func foldUsername(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// normalizeUsername lowercases and checks a login name.
// Only ASCII letters, digits and underscores are allowed.
func normalizeUsername(name string) (string, error) {
	u := foldUsername(name)
	if l := utf8.RuneCountInString(u); l < 3 || l > 32 {
		return "", ErrInvalidUsername
	}
	for _, c := range u {
		allowed := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_'
		if !allowed {
			return "", ErrInvalidUsername
		}
	}
	return u, nil
}

// validatePassword checks the length of a new password.
func validatePassword(password string) error {
	if l := utf8.RuneCountInString(password); l < 8 || l > 256 {
		return ErrInvalidPassword
	}
	return nil
}

// timestamp formats a point in time the way it is stored: RFC 3339 in UTC.
// All stored timestamps share this format, so string order is time order.
func timestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// Register creates an account and returns the new user ID and its recovery
// code. The code is the only way to reset a forgotten password, and it is not
// stored in readable form, so the caller must show it to the user now.
// The display name defaults to the username. setupData may be nil; otherwise
// it runs in the same transaction as the user insert, so the account and its
// starting data (such as the personal ledger) exist together or not at all.
func (s *Service) Register(ctx context.Context, username, password, displayName string, setupData UserDataFunc) (userID, recoveryCode string, err error) {
	u, err := normalizeUsername(username)
	if err != nil {
		return "", "", err
	}
	if err := validatePassword(password); err != nil {
		return "", "", err
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = u
	}

	// Hashed before the transaction: Argon2id takes a moment, and the
	// database should not wait for it.
	passwordHash := hashPassword(password)
	recoveryCode, recoveryHash := newRecoveryCode()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback() // no effect after a successful Commit

	id := randomHex(16)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user (id, name, username, password_hash, recovery_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, name, u, passwordHash, recoveryHash, timestamp(s.now()),
	)
	if store.IsUniqueViolation(err) {
		return "", "", ErrUsernameTaken
	}
	if err != nil {
		return "", "", err
	}
	if setupData != nil {
		if err := setupData(ctx, tx, id); err != nil {
			return "", "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, recoveryCode, nil
}

// Login checks the credentials and returns a new session token.
// Wrong usernames and wrong passwords produce the same error.
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	u := foldUsername(username)
	now := s.now()
	// Checked for every name, known or not, so the limit reveals nothing.
	if s.loginFailures.blocked(u, now) {
		return "", ErrTooManyAttempts
	}

	var userID string
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, password_hash FROM user WHERE lower(username) = ?`, u,
	).Scan(&userID, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		verifyPassword(password, dummyHash) // equalize the response time
		s.loginFailures.record(u, now)
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if !hash.Valid || !verifyPassword(password, hash.String) {
		s.loginFailures.record(u, now)
		return "", ErrInvalidCredentials
	}
	s.loginFailures.reset(u)
	return s.CreateSession(ctx, userID)
}

// CreateSession starts a session for a user and returns the token.
// The token is shown to the client once; only its hash is stored.
func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	token := newToken()
	now := s.now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO session (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, timestamp(now), timestamp(now.Add(SessionTTL)),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// UserForToken returns the user ID for a valid, unexpired token.
func (s *Service) UserForToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", ErrUnauthenticated
	}
	var userID string
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id FROM session
		WHERE token_hash = ? AND expires_at > ?`,
		hashToken(token), timestamp(s.now()),
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnauthenticated
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}

// Logout ends the session of a token. An unknown token is not an error.
func (s *Service) Logout(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE token_hash = ?`, hashToken(token))
	return err
}

// FindUserID returns the ID of the user with the given username.
func (s *Service) FindUserID(ctx context.Context, username string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM user WHERE lower(username) = ?`, foldUsername(username),
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	return id, err
}

// Profile is what a user may see about their own account.
type Profile struct {
	UserID      string
	Username    string
	DisplayName string
}

// Profile returns the account details of a user.
func (s *Service) Profile(ctx context.Context, userID string) (Profile, error) {
	p := Profile{UserID: userID}
	err := s.db.QueryRowContext(ctx,
		`SELECT username, name FROM user WHERE id = ?`, userID,
	).Scan(&p.Username, &p.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrUnauthenticated
	}
	return p, err
}

// PurgeExpired removes sessions that are no longer valid. It returns how many
// were removed.
func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE expires_at <= ?`, timestamp(s.now()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// checkPassword confirms the current password of a logged-in user before a
// sensitive change. Wrong guesses count towards the login limit of the
// account, so a stolen session cannot be used to try passwords without limit.
func (s *Service) checkPassword(ctx context.Context, userID, password string) error {
	var username sql.NullString
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT username, password_hash FROM user WHERE id = ?`, userID,
	).Scan(&username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	key := foldUsername(username.String)
	now := s.now()
	if s.loginFailures.blocked(key, now) {
		return ErrTooManyAttempts
	}
	if !hash.Valid || !verifyPassword(password, hash.String) {
		s.loginFailures.record(key, now)
		return ErrWrongPassword
	}
	return nil
}

// ChangePassword sets a new password after checking the current one. All
// other logins of the account end; the session in keepToken stays, so the
// user does not have to log in again on the device they are using. The
// recovery code stays the same.
func (s *Service) ChangePassword(ctx context.Context, userID, keepToken, currentPassword, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	if err := s.checkPassword(ctx, userID, currentPassword); err != nil {
		return err
	}
	newHash := hashPassword(newPassword)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if _, err := tx.ExecContext(ctx,
		`UPDATE user SET password_hash = ? WHERE id = ?`, newHash, userID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM session WHERE user_id = ? AND token_hash != ?`, userID, hashToken(keepToken),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// UserDataFunc creates or removes what a user owns outside the auth tables.
// It runs inside the transaction that creates or deletes the account.
type UserDataFunc func(ctx context.Context, tx *sql.Tx, userID string) error

// DeleteAccount removes a user after checking the password. removeData runs in
// the same transaction as the user deletion, so either all data is removed or
// nothing is. Sessions are removed by the database cascade.
func (s *Service) DeleteAccount(ctx context.Context, userID, password string, removeData UserDataFunc) error {
	if err := s.checkPassword(ctx, userID, password); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if err := removeData(ctx, tx, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user WHERE id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// hashToken returns the SHA-256 hash of a token. Tokens are random and
// long, so a fast hash is sufficient here.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newToken returns 32 random bytes, encoded for use in a URL or a header.
func newToken() string {
	raw := make([]byte, 32)
	rand.Read(raw) // never fails: crypto/rand aborts the program instead
	return base64.RawURLEncoding.EncodeToString(raw)
}

// randomHex returns n random bytes as hex.
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b) // never fails, see newToken
	return hex.EncodeToString(b)
}
