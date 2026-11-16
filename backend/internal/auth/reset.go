package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
)

// resetTTL is how long a password reset code stays valid.
const resetTTL = 30 * time.Minute

// ErrInvalidResetToken is returned for codes that are unknown, used or expired.
var ErrInvalidResetToken = errors.New("reset code is invalid or has expired")

// RequestPasswordReset emails a reset code to the address if an account uses it.
// It returns nil in both cases, so the answer does not reveal which addresses
// are registered. Only a malformed address or a missing mail setup is an error.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	e, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if s.mailer == nil {
		return mail.ErrNotConfigured
	}

	var userID string
	err = s.db.QueryRowContext(ctx, `SELECT id FROM user WHERE email = ?`, e).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	token, err := newToken()
	if err != nil {
		return err
	}
	now := s.now().UTC()

	// The mail and the stored code belong together: if sending fails, the
	// transaction is rolled back and no unusable code is left behind.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset WHERE expires_at < ? OR user_id = ?`,
		now.Format(time.RFC3339), userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO password_reset (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, now.Format(time.RFC3339), now.Add(resetTTL).Format(time.RFC3339),
	); err != nil {
		return err
	}

	body := "Hallo,\n\n" +
		"mit diesem Code kannst du dein Passwort zurücksetzen:\n\n" +
		"Dein Code: " + token + "\n\n" +
		"Der Code ist 30 Minuten gültig und kann nur einmal verwendet werden. " +
		"Wenn du kein neues Passwort angefordert hast, kannst du diese E-Mail ignorieren.\n"
	if err := s.mailer.Send(ctx, e, "Passwort zurücksetzen", body); err != nil {
		return err
	}
	return tx.Commit()
}

// ConfirmPasswordReset sets a new password if the reset code is valid. The code
// is used up, and all existing logins of the account end.
func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if l := utf8.RuneCountInString(newPassword); l < 8 || l > 256 {
		return ErrInvalidPassword
	}
	if token == "" {
		return ErrInvalidResetToken
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	var userID, expiresAt string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id, expires_at FROM password_reset WHERE token_hash = ?`,
		hashToken(token),
	).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return err
	}
	// Both times are UTC in the same RFC 3339 format, so string order is time order.
	if expiresAt <= s.now().UTC().Format(time.RFC3339) {
		return ErrInvalidResetToken
	}

	if _, err := tx.ExecContext(ctx, `UPDATE user SET password_hash = ? WHERE id = ?`, hash, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset WHERE user_id = ?`, userID); err != nil {
		return err
	}
	// Anyone who knew the old password loses access with it.
	if _, err := tx.ExecContext(ctx, `DELETE FROM session WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}
