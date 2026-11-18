package api

import (
	"net/http"
	"strings"
)

// corsMaxAge is how long (in seconds) a browser may cache a preflight answer.
const corsMaxAge = "600"

// WithCORS lets browser apps served from one of the allowed origins call the API.
//
// Native apps do not need CORS; only the web build of the frontend does. Origins
// must match exactly (scheme, host and port), so there is no wildcard: browsers
// refuse a wildcard together with cookies, and the exact list is also what keeps
// other sites from sending the CSRF header. With no allowed origins, next is
// returned as is.
func WithCORS(allowed []string, next http.Handler) http.Handler {
	if len(allowed) == 0 {
		return next
	}
	origins := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		origins[o] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// The answer depends on the Origin header, so caches must keep them apart.
		w.Header().Add("Vary", "Origin")
		if !origins[origin] {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		// Lets the web app send the session cookie (see session.go).
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if isPreflight(r) {
			// The router only knows the real methods and would answer OPTIONS with
			// 405, so the preflight is answered here without calling next.
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, "+csrfHeader)
			w.Header().Set("Access-Control-Max-Age", corsMaxAge)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isPreflight reports whether r is a browser's CORS preflight request.
func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

// ParseOrigins splits a comma-separated list of origins, as given in CORS_ORIGINS.
// Spaces and a trailing slash are removed, because browsers send origins without one.
func ParseOrigins(list string) []string {
	var origins []string
	for _, o := range strings.Split(list, ",") {
		o = strings.TrimSuffix(strings.TrimSpace(o), "/")
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}
