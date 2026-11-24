// Package web puts the API and the exported web app under one origin, so the
// session cookie (SameSite=Strict) works without CORS: the API answers under
// /api, everything else is the web app.
package web

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// APIPrefix is where the API is mounted. The web app uses the same paths as
// the API (e.g. /ledgers/…), so the API needs a prefix of its own.
const APIPrefix = "/api"

// immutablePrefix holds the bundles of the export. Their names contain a hash
// of their content, so a browser may keep them for good.
const immutablePrefix = "/_expo/static/"

// Handler serves api under /api and, if webDir is not empty, the exported web
// app (`expo export -p web`) from webDir for every other path. Without webDir
// only the API is served, e.g. for the Android app or a separate web host.
func Handler(api http.Handler, webDir string) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.Handle(APIPrefix+"/", http.StripPrefix(APIPrefix, api))
	if webDir == "" {
		return mux, nil
	}
	app, err := newApp(os.DirFS(webDir))
	if err != nil {
		return nil, err
	}
	mux.Handle("/", app)
	return mux, nil
}

// app serves the files of the web export. The app is a single-page app:
// paths like /ledgers/abc have no file of their own, so they get index.html
// and the app's router takes over.
type app struct {
	files fs.FS
}

func newApp(files fs.FS) (*app, error) {
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return nil, errors.New("web: index.html missing in WEB_DIR; was the app exported with `npx expo export -p web`?")
	}
	return &app{files: files}, nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	setSecurityHeaders(w.Header())

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	switch {
	case name != "" && isFile(a.files, name):
		if strings.HasPrefix(r.URL.Path, immutablePrefix) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFileFS(w, r, a.files, name)
	case path.Ext(name) != "":
		// A missing file (an old bundle, a typo) is a real 404; answering
		// with index.html would hand the browser HTML instead of a script.
		http.NotFound(w, r)
	default:
		// index.html must never be cached for long, or a browser would keep
		// loading the bundles of an old version after an update.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, a.files, "index.html")
	}
}

// isFile reports whether name is a regular file. path.Clean and fs.FS
// together keep requests inside the export: "../" cannot leave it.
func isFile(files fs.FS, name string) bool {
	info, err := fs.Stat(files, name)
	return err == nil && info.Mode().IsRegular()
}

// setSecurityHeaders forbids embedding the app in other sites (clickjacking)
// and guessing content types.
func setSecurityHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
}
