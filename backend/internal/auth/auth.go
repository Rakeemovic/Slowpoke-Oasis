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
)

// sessionTTL is how long a login stays valid.
const sessionTTL = 30 * 24 * time.Hour

// Errors returned by the auth service. Callers can compare them with errors.Is.
var (
	ErrInvalidUsername    = errors.New("username must be 3 to 32 characters: lowercase letters, digits or underscore")
	ErrInvalidPassword    = errors.New("password must have between 8 and 256 characters")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrInvalidCredentials = errors.New("wrong username or password")
	ErrUnauthenticated    = errors.New("not logged in or session expired")
	ErrWrongPassword      = errors.New("wrong password")
	ErrUserNotFound       = errors.New("user not found")
)

// Service handles registration, login and sessions.
type Service struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Service backed by db.
func New(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now}
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

// Register creates an account and returns the new user ID.
// The display name defaults to the username.
func (s *Service) Register(ctx context.Context, username, password, displayName string) (string, error) {
	u, err := normalizeUsername(username)
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
		INSERT INTO user (id, name, username, password_hash, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		id, name, u, hash, s.now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
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

	var userID string
	var hash sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, password_hash FROM user WHERE lower(username) = ?`, u,
	).Scan(&userID, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		verifyPassword(password, dummyHash) // equalize the response time
		return "", ErrInvalidCredentials
	}
	if err != nil {
		return "", err
	}
	if !hash.Valid || !verifyPassword(password, hash.String) {
		return "", ErrInvalidCredentials
	}
	return s.CreateSession(ctx, userID)
}

// CreateSession starts a session for a user and returns the token.
// The token is shown to the client once; only its hash is stored.
func (s *Service) CreateSession(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	now := s.now().UTC()
	_, err := s.db.ExecContext(ctx, `
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

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
