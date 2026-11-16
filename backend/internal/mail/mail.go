// Package mail sends transactional emails, such as password reset codes.
//
// The application talks to any SMTP relay. A hosted email provider and a
// self-hosted mail server are configured the same way: host, port, login and
// sender address.
package mail

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
)

// ErrNotConfigured is returned when no mail server has been set up.
var ErrNotConfigured = errors.New("email is not configured on this server")

// Sender sends one plain text email.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPSender sends mail through an SMTP server, upgrading to TLS with STARTTLS
// when the server offers it.
type SMTPSender struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

// Send delivers one message. The recipient address must already be validated
// by the caller, because it is written into the message header.
//
// net/smtp does not accept a context, so the context is only checked before
// connecting.
func (s SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return errors.New("header line contains a line break")
	}
	msg := "From: " + s.From + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	return smtp.SendMail(s.Host+":"+s.Port, auth, s.From, []string{to}, []byte(msg))
}
