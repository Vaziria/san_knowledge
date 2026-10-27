// Package ownview is the knowledge graph's own web UI: the graph, an explain
// query form, a detail panel, summary/keyword/domain editing, and sync/AI
// actions. All writes go through the kb package, so the schema is enforced.
package ownview

import (
	"compress/gzip"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"san_knowledge/internal/kb"
)

// The page is built from san_knowledge/ui (React + TypeScript + Chakra UI) into
// dist by the build scripts. dist/.gitkeep keeps the embed valid before the
// first build; the server then says the UI is not built.
//
//go:embed all:dist
var dist embed.FS

// Opener runs fn with an open store. The view opens the database per request
// so it never holds the lock while idle.
type Opener func(fn func(*kb.Store) error) error

// Options configures the UI server.
type Options struct {
	Dir       string // knowledge_data directory (for the AI step)
	Open      Opener
	Author    string       // recorded on edits made in the UI
	Annotator kb.Annotator // used by "Summarize with AI"; nil disables it
	Fetcher   kb.Fetcher   // used by "Fetch URL"; nil means kb.DefaultFetcher
}

// Handler serves the UI and its JSON API.
func Handler(o Options) http.Handler {
	if o.Fetcher == nil {
		o.Fetcher = kb.DefaultFetcher
	}
	h := &api{o: o}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/schema", h.schema)
	mux.HandleFunc("GET /api/graph", h.graph)
	mux.HandleFunc("GET /api/explain", h.explain)
	mux.HandleFunc("GET /api/node", h.getNode)
	mux.HandleFunc("PUT /api/node", h.annotate)
	mux.HandleFunc("DELETE /api/node", h.deleteNode)
	mux.HandleFunc("POST /api/domains", h.addDomain)
	mux.HandleFunc("POST /api/assign", h.assign)
	mux.HandleFunc("DELETE /api/assign", h.unassign)
	mux.HandleFunc("POST /api/sync", h.sync)
	mux.HandleFunc("POST /api/ai", h.ai)
	mux.HandleFunc("POST /api/fetch", h.fetch)
	mux.HandleFunc("GET /api/todos", h.todos)
	mux.HandleFunc("POST /api/todos", h.editTodo)

	root, _ := fs.Sub(dist, "dist")
	files := http.FileServer(http.FS(root))
	_, err := fs.Stat(root, "index.html")
	built := err == nil
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if !built {
			http.Error(w, "The Knowledge UI is not built: run \"npm ci\" and \"npm run build\" in san_knowledge/ui (needs Node.js), then build the binary again.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	return gzipAPI(mux)
}

// gzipAPI compresses the JSON API for clients that accept it: the graph is
// mostly repeated keys and summaries, about a sixth of its size gzipped.
func gzipAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(gzipWriter{w, gz}, r)
	})
}

type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g gzipWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }

type api struct {
	o    Options
	aiMu sync.Mutex // one AI run at a time
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, kb.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, kb.ErrLocked):
		status = http.StatusServiceUnavailable
	case errors.Is(err, kb.ErrTodoChanged):
		status = http.StatusConflict // the board is out of date
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// do runs fn against the store and writes its result (or error) as JSON.
func (h *api) do(w http.ResponseWriter, fn func(*kb.Store) (any, error)) {
	var out any
	err := h.o.Open(func(s *kb.Store) error {
		var err error
		out, err = fn(s)
		return err
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func decode(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errors.New("invalid JSON body: " + err.Error())
	}
	return nil
}

func (h *api) schema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"node_types": kb.NodeTypes, "relations": kb.Relations, "ai": h.o.Annotator != nil,
	})
}

// graphNode and graphLink are what the graph, table and filters read; the
// panel loads the rest of a node from /api/node.
type graphNode struct {
	Key          string   `json:"key"`
	NodeType     string   `json:"node_type"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary,omitempty"`
	Keyword      []string `json:"keyword,omitempty"`
	Loc          string   `json:"loc,omitempty"`
	LineLoc      int      `json:"line_loc,omitempty"`
	NeedsSummary bool     `json:"needs_summary,omitempty"`
}

type graphLink struct {
	From string `json:"from"`
	Rel  string `json:"rel"`
	To   string `json:"to"`
}

// graph returns all nodes and links, plus a Leiden community number per node
// key (?resolution= tunes the community size, default 1).
func (h *api) graph(w http.ResponseWriter, r *http.Request) {
	resolution, _ := strconv.ParseFloat(r.URL.Query().Get("resolution"), 64)
	h.do(w, func(s *kb.Store) (any, error) {
		snap, err := s.Export()
		if err != nil {
			return nil, err
		}
		keys := make([]string, len(snap.Nodes))
		nodes := make([]graphNode, len(snap.Nodes))
		for i, n := range snap.Nodes {
			keys[i] = n.Key
			nodes[i] = graphNode{n.Key, n.NodeType, n.Title, n.Summary, n.Keyword, n.Loc, n.LineLoc, n.NeedsSummary}
		}
		links := make([]graphLink, len(snap.Links))
		for i, l := range snap.Links {
			links[i] = graphLink{l.From, l.Rel, l.To}
		}
		return map[string]any{"nodes": nodes, "links": links, "communities": kb.Leiden(keys, snap.Links, resolution)}, nil
	})
}

func (h *api) explain(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	depth, _ := strconv.Atoi(q.Get("depth"))
	h.do(w, func(s *kb.Store) (any, error) {
		ex, err := s.Explain(q.Get("q"), limit, depth)
		if err != nil {
			return nil, err
		}
		return map[string]any{"explanation": ex, "markdown": ex.Markdown()}, nil
	})
}

func (h *api) getNode(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	h.do(w, func(s *kb.Store) (any, error) {
		n, err := s.Get(key)
		if err != nil {
			return nil, err
		}
		out, in, err := s.Links(key)
		if err != nil {
			return nil, err
		}
		res := map[string]any{"node": n, "out": out, "in": in}
		if n.Loc != "" {
			if d, err := s.ParseDoc(n.Loc); err == nil {
				text := d.SectionText(n.Key)
				if len(text) > 6000 {
					text = text[:6000] + "…"
				}
				res["text"] = text
			}
		}
		return res, nil
	})
}

// annotate writes summary/keyword (and a domain title). Absent fields are unchanged.
func (h *api) annotate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Summary *string  `json:"summary"`
		Keyword []string `json:"keyword"`
		Title   *string  `json:"title"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	key := r.URL.Query().Get("key")
	h.do(w, func(s *kb.Store) (any, error) {
		return s.Annotate(key, kb.Annotation{Summary: body.Summary, Keyword: body.Keyword, Title: body.Title, Author: h.o.Author})
	})
}

func (h *api) deleteNode(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	h.do(w, func(s *kb.Store) (any, error) {
		n, err := s.Get(key)
		if err != nil {
			return nil, err
		}
		if n.NodeType != kb.TypeDomain {
			return nil, errors.New("docs and sections come from ./docs; edit or delete the file instead")
		}
		return map[string]string{"deleted": key}, s.Delete(key)
	})
}

func (h *api) addDomain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title   string   `json:"title"`
		Summary string   `json:"summary"`
		Keyword []string `json:"keyword"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	h.do(w, func(s *kb.Store) (any, error) {
		d, created, err := s.AddDomain(body.Title, body.Summary, body.Keyword, h.o.Author)
		return map[string]any{"created": created, "node": d}, err
	})
}

func (h *api) assign(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key    string `json:"key"`
		Domain string `json:"domain"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	h.do(w, func(s *kb.Store) (any, error) {
		d, created, err := s.AssignDomain(body.Key, body.Domain, h.o.Author)
		return map[string]any{"domain": d, "domain_created": created}, err
	})
}

func (h *api) unassign(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	h.do(w, func(s *kb.Store) (any, error) {
		n, err := s.Unlink(q.Get("key"), kb.RelDomainOf, q.Get("domain"))
		return map[string]int{"removed": n}, err
	})
}

func (h *api) sync(w http.ResponseWriter, _ *http.Request) {
	h.do(w, func(s *kb.Store) (any, error) { return s.Sync() })
}

// ai runs a structural sync and then the AI step (can take minutes).
func (h *api) ai(w http.ResponseWriter, r *http.Request) {
	if h.o.Annotator == nil {
		writeErr(w, errors.New("AI step is not available in this server"))
		return
	}
	if !h.aiMu.TryLock() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "an AI run is already in progress"})
		return
	}
	defer h.aiMu.Unlock()
	var syncRep *kb.SyncReport
	if err := h.o.Open(func(s *kb.Store) error {
		var err error
		syncRep, err = s.Sync()
		return err
	}); err != nil {
		writeErr(w, err)
		return
	}
	rep, err := kb.RunAI(context.WithoutCancel(r.Context()), h.o.Dir, h.o.Annotator, kb.AIOptions{DryRun: r.URL.Query().Get("dry_run") == "1"})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync": syncRep, "ai": rep})
}

// todos lists the board's columns, read live from the tracked docs: filtered by
// ?kind=, ?label= (repeated) and ?q=, and paged per column by ?offset= and
// ?limit= (?status= for one column).
func (h *api) todos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	query := kb.TodoQuery{Status: kb.TodoStatus(q.Get("status")), Kind: q.Get("kind"), Labels: q["label"], Search: q.Get("q"), Offset: offset, Limit: limit}
	h.do(w, func(s *kb.Store) (any, error) {
		page, err := s.TodoPage(query)
		return map[string]any{"statuses": kb.TodoStatuses, "columns": page.Columns, "total": page.Total}, err
	})
}

// editTodo writes one board change (move, labels, subtask, check) into the
// card's markdown file, then syncs so the graph follows the file.
func (h *api) editTodo(w http.ResponseWriter, r *http.Request) {
	var e kb.TodoEdit
	if err := decode(r, &e); err != nil {
		writeErr(w, err)
		return
	}
	h.do(w, func(s *kb.Store) (any, error) {
		loc, err := s.EditTodo(e)
		if err != nil {
			return nil, err
		}
		rep, err := s.Sync()
		return map[string]any{"loc": loc, "sync": rep}, err
	})
}

// fetch saves a web page into docs/external_sources/web and syncs it. The
// download happens before the database is opened.
func (h *api) fetch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := decode(r, &body); err != nil {
		writeErr(w, err)
		return
	}
	page, err := h.o.Fetcher.Fetch(r.Context(), body.URL)
	if err != nil {
		writeErr(w, err)
		return
	}
	h.do(w, func(s *kb.Store) (any, error) {
		saved, err := kb.SaveWeb(s.Root, *page)
		if err != nil {
			return nil, err
		}
		rep, err := s.Sync()
		return map[string]any{"saved": saved, "sync": rep}, err
	})
}
