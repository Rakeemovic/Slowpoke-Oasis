package auth

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
)

// resetTTL is how long a password reset code stays valid.
const resetTTL = 30 * time.Minute

// ErrInvalidResetToken is returned for codes that are unknown, used or expired.
var ErrInvalidResetToken = errors.New("reset code is invalid or has expired")

// RequestPasswordReset emails a reset code to the address if an account uses it.
// It returns nil in both cases, so the answer does not reveal which addresses
// are registered. Only a malformed address, too many requests or a missing mail
// setup are errors.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	e, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if s.mailer == nil {
		return mail.ErrNotConfigured
	}
	// Counted for every address, registered or not, so the limit reveals nothing.
	now := s.now()
	if s.resetRequests.blocked(e, now) {
		return ErrTooManyAttempts
	}
	s.resetRequests.record(e, now)

	var userID string
	err = s.db.QueryRowContext(ctx, `SELECT id FROM user WHERE lower(email) = ?`, e).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	// The code is stored before the mail goes out, so the database is not
	// kept busy while the mail server answers.
	token := newToken()
	if err := s.storeResetCode(ctx, userID, token); err != nil {
		return err
	}
	// Sending can take seconds. Doing it in the background keeps the response
	// time the same for registered and unknown addresses, and a mail error
	// cannot be used to tell them apart either. The sender has its own timeout.
	go s.sendResetMail(context.WithoutCancel(ctx), e, token)
	return nil
}

// sendResetMail delivers a reset code. If that fails, nobody received the code,
// so it must not stay valid: it is removed and the failure is logged without
// the address.
func (s *Service) sendResetMail(ctx context.Context, to, token string) {
	err := s.mailer.Send(ctx, to, "Passwort zurücksetzen", resetMailBody(token))
	if err == nil {
		return
	}
	_, delErr := s.db.ExecContext(ctx, `DELETE FROM password_reset WHERE token_hash = ?`, hashToken(token))
	log.Printf("send reset email: %v", errors.Join(err, delErr))
}

// storeResetCode saves the hash of a new reset code for a user. It replaces
// any earlier code of that user and clears expired codes of all users.
func (s *Service) storeResetCode(ctx context.Context, userID, token string) error {
	now := s.now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM password_reset WHERE expires_at <= ? OR user_id = ?`, timestamp(now), userID,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO password_reset (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, timestamp(now), timestamp(now.Add(resetTTL)),
	); err != nil {
		return err
	}
	return tx.Commit()
}

// resetMailBody returns the text of the email that carries a reset code.
func resetMailBody(token string) string {
	return "Hallo,\n\n" +
		"mit diesem Code kannst du dein Passwort zurücksetzen:\n\n" +
		"Dein Code: " + token + "\n\n" +
		"Der Code ist 30 Minuten gültig und kann nur einmal verwendet werden. " +
		"Wenn du kein neues Passwort angefordert hast, kannst du diese E-Mail ignorieren.\n"
}

// ConfirmPasswordReset sets a new password if the reset code is valid. The code
// is used up, and all existing logins of the account end.
func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	if token == "" {
		return ErrInvalidResetToken
	}
	hash := hashPassword(newPassword)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no effect after a successful Commit

	var userID string
	err = tx.QueryRowContext(ctx, `
		SELECT user_id FROM password_reset WHERE token_hash = ? AND expires_at > ?`,
		hashToken(token), timestamp(s.now()),
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return err
	}

	statements := []struct {
		query string
		args  []any
	}{
		{`UPDATE user SET password_hash = ? WHERE id = ?`, []any{hash, userID}},
		{`DELETE FROM password_reset WHERE user_id = ?`, []any{userID}},
		// Anyone who knew the old password loses access with it.
		{`DELETE FROM session WHERE user_id = ?`, []any{userID}},
	}
	for _, st := range statements {
		if _, err := tx.ExecContext(ctx, st.query, st.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}
