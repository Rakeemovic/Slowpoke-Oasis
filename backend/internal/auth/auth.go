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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
)

// sessionTTL is how long a login stays valid.
const sessionTTL = 30 * 24 * time.Hour

// Errors returned by the auth service. Callers can compare them with errors.Is.
var (
	ErrInvalidUsername    = errors.New("username must be 3 to 32 characters: lowercase letters, digits or underscore")
	ErrInvalidPassword    = errors.New("password must have between 8 and 256 characters")
	ErrInvalidEmail       = errors.New("email address is not valid")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrEmailTaken         = errors.New("username or email address is already in use")
	ErrTooManyAttempts    = errors.New("too many attempts, please wait a while and try again")
	ErrInvalidCredentials = errors.New("wrong username or password")
	ErrUnauthenticated    = errors.New("not logged in or session expired")
	ErrWrongPassword      = errors.New("wrong password")
	ErrUserNotFound       = errors.New("user not found")
)

// emailPattern is a deliberately simple check: something@something.something.
// Whether the address really receives mail is only shown by a reset email.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Service handles registration, login, sessions and password resets.
type Service struct {
	db            *sql.DB
	now           func() time.Time
	mailer        mail.Sender
	loginFailures *limiter
	resetRequests *limiter
}

// New returns a Service backed by db. Without a mailer, password resets fail
// with mail.ErrNotConfigured.
func New(db *sql.DB) *Service {
	return &Service{
		db:            db,
		now:           time.Now,
		loginFailures: newLimiter(maxFailedLogins, loginWindow),
		resetRequests: newLimiter(maxResetRequests, resetWindow),
	}
}

// SetMailer sets the sender for password reset emails.
func (s *Service) SetMailer(m mail.Sender) {
	s.mailer = m
}

// normalizeUsername lowercases and checks a login name.
// Only ASCII letters, digits and underscores are allowed.
func normalizeUsername(name string) (string, error) {
	u := strings.ToLower(strings.TrimSpace(name))
	if l := utf8.RuneCountInString(u); l < 3 || l > 32 {
		return "", ErrInvalidUsername
	}
	for _, c := range u {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			return "", ErrInvalidUsername
		}
	}
	return u, nil
}

// normalizeEmail lowercases and checks an email address.
func normalizeEmail(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	if len(e) > 254 || !emailPattern.MatchString(e) {
		return "", ErrInvalidEmail
	}
	return e, nil
}

// Register creates an account and returns the new user ID.
// The email address is required and used for password resets.
// The display name defaults to the username.
func (s *Service) Register(ctx context.Context, username, email, password, displayName string) (string, error) {
	u, err := normalizeUsername(username)
	if err != nil {
		return "", err
	}
	e, err := normalizeEmail(email)
	if err != nil {
		return "", err
	}
	if l := utf8.RuneCountInString(password); l < 8 || l > 256 {
		return "", ErrInvalidPassword
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = u
	}

	hash, err := hashPassword(password)
	if err != nil {
		return "", err
	}
	id, err := randomHex(16)
	if err != nil {
		return "", err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user (id, name, username, email, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, name, u, e, hash, s.now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		// Both unique indexes are named, so the error says which value clashed.
		switch {
		case strings.Contains(err.Error(), "user_email_unique"):
			return "", ErrEmailTaken
		case strings.Contains(err.Error(), "user_username_unique"):
			return "", ErrUsernameTaken
		}
		return "", err
	}
	return id, nil
}

// Login checks the credentials and returns a new session token.
// Wrong usernames and wrong passwords produce the same error.
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	u := strings.ToLower(strings.TrimSpace(username))
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
	token, err := newToken()
	if err != nil {
		return "", err
	}

	now := s.now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO session (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, now.Format(time.RFC3339), now.Add(sessionTTL).Format(time.RFC3339),
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
		hashToken(token), s.now().UTC().Format(time.RFC3339),
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
	u := strings.ToLower(strings.TrimSpace(username))
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM user WHERE lower(username) = ?`, u).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	return id, err
}

// DeleteAccount removes a user after checking the password. The removeData
// callback runs in the same transaction as the user deletion, so either all
// data is removed or nothing is. Sessions are removed by the database cascade.
func (s *Service) DeleteAccount(ctx context.Context, userID, password string, removeData func(ctx context.Context, tx *sql.Tx) error) error {
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM user WHERE id = ?`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	if !hash.Valid || !verifyPassword(password, hash.String) {
		return ErrWrongPassword
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if err := removeData(ctx, tx); err != nil {
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

// newToken returns 32 random bytes, encoded for use in a URL or a mail.
func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
