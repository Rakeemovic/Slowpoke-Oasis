// Command server runs the Slowpoke-Oasis HTTP API.
//
// Usage:
//
//	server                           start the API
//	server reset-password <username> give a user a new password and recovery code
//	server backup                    write a backup of the database now
package main

import (
	"context"
	"database/sql"
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
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/web"
)

const (
	// cleanupInterval is how often expired data is removed: soft-deleted
	// transactions past their retention and old sessions.
	cleanupInterval = time.Hour
	// shutdownGrace is how long running requests may finish after a stop signal.
	shutdownGrace = 10 * time.Second
)

func main() {
	var err error
	switch {
	case len(os.Args) == 1:
		err = run()
	case os.Args[1] == "reset-password":
		err = runResetPassword(os.Args[2:], os.Stdout)
	case os.Args[1] == "backup":
		err = runBackup(os.Args[2:], os.Stdout)
	default:
		err = fmt.Errorf("unknown command %q\n%s", os.Args[1], usage)
	}
	if err != nil {
		log.Fatal(err)
	}
}

const usage = `usage:
  server                            start the API
  server reset-password <username>  give a user a new password and recovery code
  server backup                     write a backup of the database to BACKUP_DIR`

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
	registrationOpen, err := registrationOpenFromEnv()
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backupDir := os.Getenv("BACKUP_DIR")
	go runMaintenance(ctx, ledgers, accounts, func(now time.Time) { backupDaily(ctx, db, backupDir, now) })

	// Only the web build of the frontend needs CORS; native apps work without it.
	corsOrigins := api.ParseOrigins(os.Getenv("CORS_ORIGINS"))
	if len(corsOrigins) > 0 {
		log.Printf("CORS allowed for %v", corsOrigins)
	}

	server := api.NewServer(ledgers, accounts)
	if !registrationOpen {
		server.CloseRegistration()
		log.Printf("registration is closed")
	}
	// With WEB_DIR the server also delivers the web app, so app and API share
	// one origin and the web login needs no CORS.
	webDir := os.Getenv("WEB_DIR")
	handler, err := web.Handler(api.WithCORS(corsOrigins, server.Routes()), webDir)
	if err != nil {
		return err
	}
	if webDir != "" {
		log.Printf("serving the web app from %s", webDir)
	}
	if !secureCookies {
		server.AllowInsecureCookies()
		log.Printf("WARNING: COOKIE_SECURE=false: the web login cookie is sent over plain HTTP. " +
			"Anyone on the network can read it and take over the session. Only use this in a trusted home network.")
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
		// Limits for slow or stalled clients, so they cannot hold connections open forever.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Printf("listening on %s (API under %s), database %s", addr, web.APIPrefix, dbPath)

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

// runMaintenance removes expired data and then backs up the rest, once right
// away and then at every cleanupInterval, until ctx is cancelled.
func runMaintenance(ctx context.Context, ledgers *ledger.Store, accounts *auth.Service, backup func(time.Time)) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		if n, err := ledgers.PurgeDeletedTransactions(ctx, time.Now()); err != nil {
			log.Printf("purge deleted transactions: %v", err)
		} else if n > 0 {
			log.Printf("purged %d deleted transactions", n)
		}
		if n, err := accounts.PurgeExpired(ctx); err != nil {
			log.Printf("purge expired sessions: %v", err)
		} else if n > 0 {
			log.Printf("purged %d expired sessions", n)
		}

		backup(time.Now())

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// openExisting opens the database the server uses, for commands that work on
// it directly. Opening a missing file would create an empty database, which
// only hides a wrong DB_PATH.
func openExisting() (*sql.DB, error) {
	dbPath := envOr("DB_PATH", "slowpoke.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("database %s not found, set DB_PATH to the file the server uses: %w", dbPath, err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}

// envOr returns the value of an environment variable, or fallback if it is not set.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// cookieSecureFromEnv reads COOKIE_SECURE. It defaults to true; only an
// explicit false turns the Secure flag off.
func cookieSecureFromEnv() (bool, error) {
	return boolFromEnv("COOKIE_SECURE", true)
}

// registrationOpenFromEnv reads REGISTRATION_OPEN. It defaults to true, so a
// fresh installation can create its first account.
func registrationOpenFromEnv() (bool, error) {
	return boolFromEnv("REGISTRATION_OPEN", true)
}

// boolFromEnv reads a boolean environment variable, or fallback if it is not
// set. Anything that is not a boolean is an error, so a typo cannot silently
// weaken a setting.
func boolFromEnv(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, v)
	}
	return value, nil
}
