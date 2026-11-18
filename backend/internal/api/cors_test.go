package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const testOrigin = "http://localhost:8081"

// okHandler answers every request with 200, so tests can see whether it was reached.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestCORSAllowedOriginGetsHeader(t *testing.T) {
	h := WithCORS([]string{testOrigin}, okHandler)
	req := httptest.NewRequest(http.MethodGet, "/households", nil)
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Errorf("Allow-Origin = %q, want %q", got, testOrigin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
}

func TestCORSUnknownOriginGetsNoHeader(t *testing.T) {
	h := WithCORS([]string{testOrigin}, okHandler)
	req := httptest.NewRequest(http.MethodGet, "/households", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want none", got)
	}
}

func TestCORSPreflightIsAnswered(t *testing.T) {
	h := WithCORS([]string{testOrigin}, okHandler)
	req := httptest.NewRequest(http.MethodOptions, "/ledgers/l1/transactions", nil)
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type, X-Requested-With" {
		t.Errorf("Allow-Headers = %q", got)
	}
}

func TestCORSPreflightWorksWithRealRoutes(t *testing.T) {
	h := WithCORS([]string{testOrigin}, newTestHandler(t))
	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestCORSDisabledWithoutOrigins(t *testing.T) {
	h := WithCORS(nil, okHandler)
	req := httptest.NewRequest(http.MethodGet, "/households", nil)
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want none", got)
	}
}

func TestParseOrigins(t *testing.T) {
	got := ParseOrigins(" http://localhost:8081/ , https://app.example.org,,")
	want := []string{"http://localhost:8081", "https://app.example.org"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseOrigins = %v, want %v", got, want)
	}
}
