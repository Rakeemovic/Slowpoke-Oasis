package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
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
	db, err := openExisting()
	if err != nil {
		return err
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
