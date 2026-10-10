// Package view is the knowledge graph's preview in the browser: the graph of
// docs/knowledge.md, coloured by node type, with a panel that
// shows a node's properties, its readable view and its links.
//
// The page is plain HTML, CSS and JavaScript embedded in the binary (with a
// vendored cytoscape), so there is no build step. The database is opened per
// request, so the CLI and the MCP server can use it while the view runs.
package view

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"github.com/wargasipil/autotrade/internal/knowledge"
)

//go:embed static
var static embed.FS

// Options configures the server.
type Options struct {
	Dir  string                                              // the knowledge database
	Sync func(ctx context.Context) (knowledge.Result, error) // rebuilds the graph; nil hides the button
}

// Handler serves the page and its JSON API.
func Handler(o Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/graph", func(w http.ResponseWriter, r *http.Request) {
		var g *knowledge.Graph
		err := knowledge.With(o.Dir, func(s *knowledge.Store) error {
			var err error
			g, err = knowledge.All(s)
			return err
		})
		if err != nil {
			writeErr(w, err)
			return
		}
		// The graph only needs the small properties; long text comes with
		// the node.
		for _, n := range g.Nodes {
			for _, k := range []string{"markdown", "text", "note", "reason", "description"} {
				delete(n.Props, k)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"nodes": g.Nodes, "links": g.Links, "types": knowledge.Types, "labels": knowledge.Labels, "sync": o.Sync != nil})
	})
	mux.HandleFunc("GET /api/node", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		var out map[string]any
		err := knowledge.With(o.Dir, func(s *knowledge.Store) error {
			n, err := s.Get(key)
			if err != nil {
				return err
			}
			in, links, err := s.Links(key)
			if err != nil {
				return err
			}
			md, err := knowledge.NodeMarkdown(s, n)
			if err != nil {
				return err
			}
			if n.Type == knowledge.TypeSnapshot {
				delete(n.Props, "markdown") // it is the readable view already
			}
			out = map[string]any{"node": n, "in": in, "out": links, "markdown": md}
			return nil
		})
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("POST /api/sync", func(w http.ResponseWriter, r *http.Request) {
		if o.Sync == nil {
			writeErr(w, errors.New("sync is not available here"))
			return
		}
		res, err := o.Sync(r.Context())
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	root, _ := fs.Sub(static, "static")
	files := http.FileServer(http.FS(root))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, knowledge.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, knowledge.ErrLocked):
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
