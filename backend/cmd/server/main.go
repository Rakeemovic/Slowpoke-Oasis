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

	log.Printf("listening on %s, database %s", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, api.NewServer(ledgers, auth.New(db)).Routes()))
}
