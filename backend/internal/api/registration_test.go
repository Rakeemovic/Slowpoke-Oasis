package api

import (
	"net/http"
	"testing"
)

func TestClosedRegistration(t *testing.T) {
	server := newTestServer(t)
	open := server.Routes()
	register(t, open, "alice")

	server.CloseRegistration()
	closed := server.Routes()

	// A taken and a free name get the same answer.
	for _, username := range []string{"alice", "mallory"} {
		rec := doToken(t, closed, http.MethodPost, "/auth/register", "", map[string]any{
			"username": username, "password": testPassword, "display_name": "Mallory",
		})
		if rec.Code != http.StatusForbidden {
			t.Errorf("register %s: status %d, want %d, body %s", username, rec.Code, http.StatusForbidden, rec.Body)
		}
	}
	// Existing accounts keep working.
	if status := loginStatus(t, closed, "alice", testPassword); status != http.StatusOK {
		t.Errorf("login: status %d, want %d", status, http.StatusOK)
	}
}
