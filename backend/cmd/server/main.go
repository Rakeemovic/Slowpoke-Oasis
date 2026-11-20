// Command server runs the Slowpoke-Oasis HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/api"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

const (
	// cleanupInterval is how often expired data is removed: soft-deleted
	// transactions past their retention, old sessions and old reset codes.
	cleanupInterval = time.Hour
	// shutdownGrace is how long running requests may finish after a stop signal.
	shutdownGrace = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run starts the server and blocks until it is stopped with Ctrl+C or SIGTERM.
// Returning an error instead of exiting lets the deferred cleanup happen.
func run() error {
	dbPath := envOr("DB_PATH", "slowpoke.db")
	addr := envOr("ADDR", ":8080")
	// Checked before anything starts, so a typo stops the server cleanly.
	secureCookies, err := cookieSecureFromEnv()
	if err != nil {
		return err
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ledgers := ledger.New(db)
	accounts := auth.New(db)
	sender, err := smtpFromEnv()
	switch {
	case errors.Is(err, mail.ErrNotConfigured):
		log.Printf("SMTP_HOST is not set: password reset is disabled")
	case err != nil:
		return err
	default:
		accounts.SetMailer(sender)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go runCleanup(ctx, ledgers, accounts)

	// Only the web build of the frontend needs CORS; native apps work without it.
	corsOrigins := api.ParseOrigins(os.Getenv("CORS_ORIGINS"))
	if len(corsOrigins) > 0 {
		log.Printf("CORS allowed for %v", corsOrigins)
	}

	server := api.NewServer(ledgers, accounts)
	if !secureCookies {
		server.AllowInsecureCookies()
		log.Printf("WARNING: COOKIE_SECURE=false: the web login cookie is sent over plain HTTP. " +
			"Anyone on the network can read it and take over the session. Only use this in a trusted home network.")
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: api.WithCORS(corsOrigins, server.Routes()),
		// Limits for slow or stalled clients, so they cannot hold connections open forever.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Printf("listening on %s, database %s", addr, dbPath)

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	log.Printf("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// runCleanup removes expired data once right away and then at every
// cleanupInterval, until ctx is cancelled.
func runCleanup(ctx context.Context, ledgers *ledger.Store, accounts *auth.Service) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		if n, err := ledgers.PurgeDeletedTransactions(ctx, time.Now()); err != nil {
			log.Printf("purge deleted transactions: %v", err)
		} else if n > 0 {
			log.Printf("purged %d deleted transactions", n)
		}
		if n, err := accounts.PurgeExpired(ctx); err != nil {
			log.Printf("purge expired sessions and reset codes: %v", err)
		} else if n > 0 {
			log.Printf("purged %d expired sessions and reset codes", n)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// envOr returns the value of an environment variable, or fallback if it is not set.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// cookieSecureFromEnv reads COOKIE_SECURE. It defaults to true; only an
// explicit false turns the Secure flag off. Anything that is not a boolean is
// an error, so a typo cannot silently weaken the cookie.
func cookieSecureFromEnv() (bool, error) {
	v := os.Getenv("COOKIE_SECURE")
	if v == "" {
		return true, nil
	}
	secure, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("COOKIE_SECURE must be true or false, got %q", v)
	}
	return secure, nil
}

// smtpFromEnv reads the mail server settings from the environment. The server
// works with a hosted provider or a self-hosted mail server, as long as it is
// reachable over SMTP. Without SMTP_HOST it returns mail.ErrNotConfigured, and
// password resets are disabled.
func smtpFromEnv() (mail.SMTPSender, error) {
	s := mail.SMTPSender{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     envOr("SMTP_PORT", "587"),
		Username: os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
	if s.Host == "" {
		return mail.SMTPSender{}, mail.ErrNotConfigured
	}
	if s.From == "" {
		return mail.SMTPSender{}, errors.New("SMTP_HOST is set but SMTP_FROM is missing: mails need a sender address")
	}
	return s, nil
}
