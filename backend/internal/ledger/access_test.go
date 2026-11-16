package ledger

import (
	"context"
	"testing"
)

func TestCanAccessLedgerForOwnersAndHouseholdMembers(t *testing.T) {
	db, s := setup(t)
	ctx := context.Background()

	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('bob', 'Bob', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO user (id, name, created_at) VALUES ('carol', 'Carol', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO household (id, name, created_at) VALUES ('h1', 'Flat', '2026-01-01T00:00:00Z')`)
	mustExec(t, db, `INSERT INTO household_member (household_id, user_id) VALUES ('h1', 'bob')`)
	mustExec(t, db, `INSERT INTO ledger (id, name, household_id, created_at) VALUES ('lh', 'Shared', 'h1', '2026-01-01T00:00:00Z')`)

	cases := []struct {
		name     string
		user     string
		ledger   string
		expected bool
	}{
		{"owner of a personal ledger", "u1", "l1", true},
		{"other user, personal ledger", "bob", "l1", false},
		{"household member, household ledger", "bob", "lh", true},
		{"non-member, household ledger", "carol", "lh", false},
		{"unknown ledger", "u1", "nope", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.CanAccessLedger(ctx, tc.user, tc.ledger)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if got != tc.expected {
				t.Errorf("CanAccessLedger(%s, %s) = %v, want %v", tc.user, tc.ledger, got, tc.expected)
			}
		})
	}
}
