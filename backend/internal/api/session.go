package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Rakeemovic/slowpoke-oasis/backend/internal/auth"
)

// A login token reaches the API in one of two ways:
//
//   - Native apps send it as "Authorization: Bearer <token>" and keep it in the
//     device's secure storage.
//   - The web app asks for a cookie at login. The cookie is HttpOnly, so
//     JavaScript never sees the token and injected scripts cannot steal it.
//
// Browsers send cookies on their own, which allows cross-site request forgery
// (CSRF). Two independent layers prevent it: SameSite=Strict keeps the browser
// from sending the cookie with requests started by other sites, and requests
// authenticated by cookie must carry the csrfHeader. Browsers let other origins
// set such a header only after a CORS preflight, which WithCORS refuses for
// origins that are not allowed.
const (
	sessionCookieName = "session"
	csrfHeader        = "X-Requested-With"
)

// ErrCSRFHeaderMissing is returned when a cookie-authenticated request lacks csrfHeader.
var ErrCSRFHeaderMissing = errors.New("requests with a session cookie need the X-Requested-With header")

// sessionToken returns the login token of the request and whether it came from
// the cookie. A bearer token takes precedence over a cookie.
func sessionToken(r *http.Request) (token string, fromCookie bool) {
	if token := bearerToken(r); token != "" {
		return token, false
	}
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return c.Value, true
	}
	return "", false
}

// bearerToken returns the token from "Authorization: Bearer <token>", or "".
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// setSessionCookie stores the login token in an HttpOnly cookie that lives as
// long as the session. Secure is always set: browsers accept it on
// http://localhost, and everywhere else the API must be served over HTTPS.
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookie tells the browser to delete the session cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookieIfUsed deletes the cookie after logout or account deletion,
// but only for clients that logged in with one.
func clearSessionCookieIfUsed(w http.ResponseWriter, r *http.Request) {
	if _, fromCookie := sessionToken(r); fromCookie {
		clearSessionCookie(w)
	}
}
