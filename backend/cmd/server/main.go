// Command server runs the Slowpoke-Oasis HTTP API.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/api"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/mail"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// purgeInterval is how often soft-deleted transactions past their retention are removed.
const purgeInterval = time.Hour

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "slowpoke.db"
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	db, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	ledgers := ledger.New(db)
	go ledgers.RunPurgeLoop(context.Background(), purgeInterval)

	accounts := auth.New(db)
	if sender, ok := smtpFromEnv(); ok {
		accounts.SetMailer(sender)
	} else {
		log.Printf("SMTP_HOST is not set: password reset is disabled")
	}

	log.Printf("listening on %s, database %s", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, api.NewServer(ledgers, accounts).Routes()))
}

// smtpFromEnv reads the mail server settings from the environment. The server
// works with a hosted provider or a self-hosted mail server, as long as it is
// reachable over SMTP. Without SMTP_HOST, password resets are disabled.
func smtpFromEnv() (mail.SMTPSender, bool) {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		return mail.SMTPSender{}, false
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	return mail.SMTPSender{
		Host:     host,
		Port:     port,
		Username: os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}, true
}
