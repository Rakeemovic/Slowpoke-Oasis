// Package mail sends transactional emails, such as password reset codes.
//
// The application talks to any SMTP relay. A hosted email provider and a
// self-hosted mail server are configured the same way: host, port, login and
// sender address.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// sendTimeout limits how long delivering one message may take, so a mail
// server that stops answering cannot block a request forever.
const sendTimeout = 30 * time.Second

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

// Send delivers one message. It gives up when ctx is cancelled or after
// sendTimeout, whichever comes first.
func (s SMTPSender) Send(ctx context.Context, to, subject, body string) error {
	// These values are written into the message header. A line break would
	// let the caller add header lines of their own.
	if strings.ContainsAny(s.From+to+subject, "\r\n") {
		return errors.New("header line contains a line break")
	}

	ctx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.Host, s.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	// net/smtp does not know about contexts. Closing the connection is what
	// interrupts it when the context ends.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	err = s.deliver(conn, to, buildMessage(s.From, to, subject, body, time.Now()))
	if err != nil && ctx.Err() != nil {
		return ctx.Err() // more useful than "use of closed network connection"
	}
	return err
}

// deliver runs the SMTP conversation for one message on an open connection.
func (s SMTPSender) deliver(conn net.Conn, to string, msg []byte) error {
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return err
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
			return err
		}
	}
	if s.Username != "" {
		// PlainAuth refuses to send the password over an unencrypted
		// connection, unless the server is on this machine.
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(s.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMessage returns a plain text message with its header. The subject is
// encoded, because header lines may only contain ASCII.
func buildMessage(from, to, subject, body string, now time.Time) []byte {
	header := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + now.Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return []byte(strings.Join(header, "\r\n") + "\r\n\r\n" + body)
}
