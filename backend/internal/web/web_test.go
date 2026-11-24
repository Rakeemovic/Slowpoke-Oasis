package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestHandler serves a small export and an API that echoes its path.
func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":               "<html>app</html>",
		"favicon.ico":              "icon",
		"_expo/static/js/app-1.js": "bundle",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "api "+r.URL.Path)
	})
	h, err := Handler(api, dir)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	return h
}

func get(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestHandlerRoutesAPIAndApp(t *testing.T) {
	h := newTestHandler(t)
	cases := []struct {
		name, path string
		status     int
		body       string
		cache      string
	}{
		{"api without prefix", "/api/ledgers/l1", 200, "api /ledgers/l1", ""},
		{"start page", "/", 200, "<html>app</html>", "no-cache"},
		{"app route", "/ledgers/l1/budget", 200, "<html>app</html>", "no-cache"},
		{"file", "/favicon.ico", 200, "icon", "no-cache"},
		{"bundle", "/_expo/static/js/app-1.js", 200, "bundle", "public, max-age=31536000, immutable"},
		{"missing bundle", "/_expo/static/js/app-0.js", 404, "", ""},
		{"directory", "/_expo/static", 200, "<html>app</html>", "no-cache"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(h, http.MethodGet, tc.path)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d (body %q)", rec.Code, tc.status, rec.Body)
			}
			if tc.body != "" && rec.Body.String() != tc.body {
				t.Errorf("body %q, want %q", rec.Body, tc.body)
			}
			if got := rec.Header().Get("Cache-Control"); tc.cache != "" && got != tc.cache {
				t.Errorf("Cache-Control %q, want %q", got, tc.cache)
			}
		})
	}
}

func TestAppSendsSecurityHeaders(t *testing.T) {
	rec := get(newTestHandler(t), http.MethodGet, "/")
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestAppOnlyAnswersReads(t *testing.T) {
	h := newTestHandler(t)
	if rec := get(h, http.MethodPost, "/ledgers"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d, want 405", rec.Code)
	}
	// The API still takes every method.
	if rec := get(h, http.MethodPost, "/api/ledgers"); rec.Code != http.StatusOK {
		t.Errorf("POST to the API: status %d, want 200", rec.Code)
	}
}

func TestHandlerWithoutWebDirServesOnlyTheAPI(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h, err := Handler(api, "")
	if err != nil {
		t.Fatal(err)
	}
	if rec := get(h, http.MethodGet, "/api/me"); rec.Code != http.StatusOK {
		t.Errorf("api: status %d, want 200", rec.Code)
	}
	if rec := get(h, http.MethodGet, "/"); rec.Code != http.StatusNotFound {
		t.Errorf("app: status %d, want 404", rec.Code)
	}
}

func TestHandlerRejectsAnExportWithoutIndex(t *testing.T) {
	_, err := Handler(http.NotFoundHandler(), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Errorf("err = %v, want a hint about index.html", err)
	}
}

func TestAppStaysInsideTheExport(t *testing.T) {
	root := t.TempDir()
	export := filepath.Join(root, "export")
	if err := os.Mkdir(export, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		filepath.Join(export, "index.html"): "<html>app</html>",
		filepath.Join(root, "secret"):       "secret",
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := newApp(os.DirFS(export))
	if err != nil {
		t.Fatal(err)
	}
	// Built by hand: the mux would clean the path before the app sees it.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.URL.Path = "/../secret"
	rec := httptest.NewRecorder()
	a.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("served a file outside the export: %q", rec.Body)
	}
}
