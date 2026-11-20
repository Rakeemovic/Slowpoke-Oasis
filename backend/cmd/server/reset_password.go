package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

// runResetPassword handles "server reset-password <username>". It is the way
// back in for a user who lost both their password and their recovery code:
// the person running the server gives them a temporary password and a new
// code. It works on the database directly, so it needs access to the server
// itself and is not reachable over the network.
func runResetPassword(args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New(usage)
	}
	dbPath := envOr("DB_PATH", "slowpoke.db")
	// Opening a missing file would create an empty database, which only hides
	// a wrong DB_PATH behind a confusing "user not found".
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("database %s not found, set DB_PATH to the file the server uses: %w", dbPath, err)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	password, code, err := auth.New(db).ResetByOperator(context.Background(), args[0])
	if err != nil {
		return fmt.Errorf("reset password of %q: %w", args[0], err)
	}
	fmt.Fprintf(out, `Password of %q has been reset. All of their logins have ended.

  Temporary password: %s
  New recovery code:  %s

Give both to the user in person. They should change the password after
logging in and keep the recovery code somewhere safe.
`, args[0], password, code)
	return nil
}
