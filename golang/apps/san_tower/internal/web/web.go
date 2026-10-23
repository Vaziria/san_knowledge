// Package web serves the tower's UI: the Vite build of
// typescripts/san_tower_frontend, embedded at build time. Until the first UI
// build, dist/ holds only .gitkeep and the page answers 503.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Dist is the embedded UI build.
func Dist() fs.FS {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return files
}

// Handler serves the UI from files. A path with a file extension is a file
// (and a 404 when missing); any other path gets index.html, so the UI's own
// routes (react-router) survive a reload or a pasted link.
func Handler(files fs.FS) http.Handler {
	static := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// File names are fixed, not hashed, so a rebuild must not be cached.
		w.Header().Set("Cache-Control", "no-cache")
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if path.Ext(name) != "" && name != "index.html" {
			static.ServeHTTP(w, r)
			return
		}
		index, err := fs.ReadFile(files, "index.html")
		if err != nil {
			http.Error(w, "the UI is not built: run build.ps1 (or npm run build in typescripts/san_tower_frontend)", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}
