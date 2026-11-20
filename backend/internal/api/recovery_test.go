package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// registerWithCode creates an account and returns the recovery code from the
// registration answer.
func registerWithCode(t *testing.T, h http.Handler, username string) string {
	t.Helper()
	rec := doToken(t, h, http.MethodPost, "/auth/register", "", map[string]any{
		"username": username, "password": testPassword,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: status %d, body %s", rec.Code, rec.Body)
	}
	code := decodeField(t, rec.Body.Bytes(), "recovery_code")
	if len(strings.Fields(code)) != 6 {
		t.Fatalf("recovery code %q: want six words", code)
	}
	return code
}

// loginStatus returns the status of a login attempt.
func loginStatus(t *testing.T, h http.Handler, username, password string) int {
	t.Helper()
	return doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": username, "password": password,
	}).Code
}

func TestRecoverWithCodeOverTheAPI(t *testing.T) {
	h := newTestHandler(t)
	code := registerWithCode(t, h, "bob")

	tryRecover := func(username, code string) *httptest.ResponseRecorder {
		return doToken(t, h, http.MethodPost, "/auth/recover", "", map[string]any{
			"username": username, "recovery_code": code, "new_password": "brand new password",
		})
	}

	// Unknown name and wrong code look the same.
	for _, tc := range []struct{ username, code string }{{"nobody", code}, {"bob", "wrong words"}} {
		if rec := tryRecover(tc.username, tc.code); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s/%q: status %d, want 401", tc.username, tc.code, rec.Code)
		}
	}

	rec := tryRecover("bob", code)
	if rec.Code != http.StatusOK {
		t.Fatalf("recover: status %d, body %s", rec.Code, rec.Body)
	}
	if newCode := decodeField(t, rec.Body.Bytes(), "recovery_code"); newCode == code {
		t.Error("the answer repeats the used code, want a new one")
	}
	if got := loginStatus(t, h, "bob", "brand new password"); got != http.StatusOK {
		t.Errorf("login with new password: status %d, want 200", got)
	}
}

func TestChangePasswordOverTheAPI(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")

	change := func(current string) int {
		return doToken(t, h, http.MethodPut, "/auth/password", token, map[string]any{
			"current_password": current, "new_password": "brand new password",
		}).Code
	}
	if got := change("wrong password"); got != http.StatusForbidden {
		t.Errorf("wrong current password: status %d, want 403", got)
	}
	if got := change(testPassword); got != http.StatusNoContent {
		t.Fatalf("change: status %d, want 204", got)
	}
	// The session that made the change stays valid.
	if rec := doToken(t, h, http.MethodGet, "/me", token, nil); rec.Code != http.StatusOK {
		t.Errorf("GET /me after change: status %d, want 200", rec.Code)
	}
	if got := loginStatus(t, h, "bob", "brand new password"); got != http.StatusOK {
		t.Errorf("login with new password: status %d, want 200", got)
	}

	if rec := doToken(t, h, http.MethodPut, "/auth/password", "", map[string]any{
		"current_password": testPassword, "new_password": "brand new password",
	}); rec.Code != http.StatusUnauthorized {
		t.Errorf("without login: status %d, want 401", rec.Code)
	}
}
