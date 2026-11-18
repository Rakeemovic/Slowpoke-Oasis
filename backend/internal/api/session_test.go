package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testPassword = "correct horse battery"

// cookieLogin registers a user and logs in the way the web app does. It returns
// the session cookie set by the server.
func cookieLogin(t *testing.T, h http.Handler, username string) *http.Cookie {
	t.Helper()
	register(t, h, username)
	rec := doToken(t, h, http.MethodPost, "/auth/login", "", map[string]any{
		"username": username, "password": testPassword, "cookie": true,
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("cookie login: status %d, body %s", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("cookie login must not return the token, body %s", rec.Body)
	}
	c := findCookie(rec, sessionCookieName)
	if c == nil {
		t.Fatal("cookie login: no session cookie set")
	}
	return c
}

// doCookie sends a request authenticated by the session cookie. csrf decides
// whether the CSRF header is sent.
func doCookie(t *testing.T, h http.Handler, method, path string, c *http.Cookie, csrf bool, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(c)
	if csrf {
		req.Header.Set(csrfHeader, "fetch")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestCookieLoginSetsSecureCookie(t *testing.T) {
	h := newTestHandler(t)
	c := cookieLogin(t, h, "bob")

	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie flags: HttpOnly=%v Secure=%v SameSite=%v", c.HttpOnly, c.Secure, c.SameSite)
	}
	if c.MaxAge <= 0 || c.Path != "/" {
		t.Errorf("cookie MaxAge=%d Path=%q", c.MaxAge, c.Path)
	}
}

func TestCookieRequestWithCSRFHeaderWorks(t *testing.T) {
	h := newTestHandler(t)
	c := cookieLogin(t, h, "bob")

	rec := doCookie(t, h, http.MethodGet, "/households", c, true, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestCookieRequestWithoutCSRFHeaderIsRejected(t *testing.T) {
	h := newTestHandler(t)
	c := cookieLogin(t, h, "bob")

	rec := doCookie(t, h, http.MethodPost, "/households", c, false, map[string]any{"name": "Home"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403, body %s", rec.Code, rec.Body)
	}
}

func TestBearerTokenNeedsNoCSRFHeader(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")

	rec := doToken(t, h, http.MethodGet, "/households", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
}

func TestCookieLogoutEndsSessionAndClearsCookie(t *testing.T) {
	h := newTestHandler(t)
	c := cookieLogin(t, h, "bob")

	rec := doCookie(t, h, http.MethodPost, "/auth/logout", c, true, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status %d, body %s", rec.Code, rec.Body)
	}
	if cleared := findCookie(rec, sessionCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("logout must clear the cookie, got %+v", cleared)
	}

	rec = doCookie(t, h, http.MethodGet, "/households", c, true, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: status %d, want 401", rec.Code)
	}
}

func TestBearerLogoutSetsNoCookie(t *testing.T) {
	h := newTestHandler(t)
	_, token := register(t, h, "bob")

	rec := doToken(t, h, http.MethodPost, "/auth/logout", token, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: status %d, body %s", rec.Code, rec.Body)
	}
	if c := findCookie(rec, sessionCookieName); c != nil {
		t.Errorf("bearer logout must not touch cookies, got %+v", c)
	}
}

func TestCookieAccountDeletionClearsCookie(t *testing.T) {
	h := newTestHandler(t)
	c := cookieLogin(t, h, "bob")

	rec := doCookie(t, h, http.MethodDelete, "/auth/account", c, true, map[string]any{"password": testPassword})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d, body %s", rec.Code, rec.Body)
	}
	if cleared := findCookie(rec, sessionCookieName); cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("account deletion must clear the cookie, got %+v", cleared)
	}
}
