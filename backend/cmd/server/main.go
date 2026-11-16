// Command server runs the Slowpoke-Oasis HTTP API.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/api"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/ledger"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

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

	log.Printf("listening on %s, database %s", addr, dbPath)
	log.Fatal(http.ListenAndServe(addr, api.NewServer(ledger.New(db), auth.New(db)).Routes()))
}
