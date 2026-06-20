package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewSPAHandler serves the embedded single-page application from static.
// Existing files (the hashed Vite assets, index.html, sounds, …) are
// served directly; every other path falls back to index.html so that
// client-side routing (React Router) survives deep links and refreshes.
func NewSPAHandler(static fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			serveIndex(w, r, static)
			return
		}

		f, err := static.Open(name)
		if err != nil {
			// Unknown path → SPA route; hand it to the client router.
			serveIndex(w, r, static)
			return
		}
		info, statErr := f.Stat()
		_ = f.Close()
		if statErr != nil || info.IsDir() {
			serveIndex(w, r, static)
			return
		}

		// Vite emits content-hashed filenames under assets/, so they can
		// be cached forever; a new deploy ships new hashes.
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	})
}

// serveIndex writes the SPA shell. It is never cached so clients always
// load the latest asset hashes after a deploy.
func serveIndex(w http.ResponseWriter, _ *http.Request, static fs.FS) {
	data, err := fs.ReadFile(static, "index.html")
	if err != nil {
		http.Error(w, "frontend bundle not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
