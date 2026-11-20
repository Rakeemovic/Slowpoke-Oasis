package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/store"
)

func TestCookieSecureFromEnv(t *testing.T) {
	tests := []struct {
		value   string
		want    bool
		wantErr bool
	}{
		{value: "", want: true},
		{value: "true", want: true},
		{value: "1", want: true},
		{value: "false", want: false},
		{value: "FALSE", want: false},
		{value: "0", want: false},
		// A typo must stop the server instead of guessing.
		{value: "nein", wantErr: true},
		{value: "off", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("COOKIE_SECURE", tt.value)
			got, err := cookieSecureFromEnv()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResetPasswordCommand(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("DB_PATH", dbPath)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	accounts := auth.New(db)
	if _, _, err := accounts.Register(context.Background(), "bob", "correct horse battery", "", nil); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := runResetPassword([]string{"bob"}, &out); err != nil {
		t.Fatalf("reset: %v", err)
	}
	_, after, _ := strings.Cut(out.String(), "Temporary password: ")
	password, _, _ := strings.Cut(after, "\n")
	if _, err := accounts.Login(context.Background(), "bob", password); err != nil {
		t.Fatalf("login with printed password %q: %v\noutput:\n%s", password, err, out.String())
	}
	db.Close()

	if err := runResetPassword([]string{"nobody"}, &out); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("unknown user: got %v, want ErrUserNotFound", err)
	}
	if err := runResetPassword(nil, &out); err == nil {
		t.Error("no username: want a usage error")
	}
}

func TestResetPasswordCommandNeedsAnExistingDatabase(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	t.Setenv("DB_PATH", missing)
	if err := runResetPassword([]string{"bob"}, io.Discard); err == nil {
		t.Fatal("want an error for a missing database")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the command created a database file")
	}
}
