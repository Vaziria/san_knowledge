// Package moviemovtest is a fake of a web_movie site's API (MovieMov), on
// 127.0.0.1, for the tests of internal/moviemov, the engine's MovieMov steps
// and the tower's MovieMov connection. It keeps movies, tokens and reports
// in memory, answers in the site's shapes (docs/openapi.json) and records
// every request.
package moviemovtest

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Request is a request the fake got: JSON is its JSON body, Form its form
// fields (a list as several values), File the thumbnail's name and size.
type Request struct {
	Method, Path, Query, ContentType, Auth string
	JSON                                   map[string]any
	Form                                   url.Values
	File                                   string
	FileSize                               int
}

// Server is the fake site; its API is at URL + "/api".
type Server struct {
	*httptest.Server
	Email, Password string

	mu       sync.Mutex
	logins   int
	tokens   map[string]bool
	movies   []map[string]any
	nextID   int
	requests []Request
	reports  map[string]int // open reports by slug
	// LoginStatus, when not 0, is what every login answers, such as 429.
	LoginStatus int
	// RefuseTokens refuses every token, also a new one.
	RefuseTokens bool
}

// New starts a fake site whose admin is email with password.
func New(email, password string) *Server {
	s := &Server{Email: email, Password: password, tokens: map[string]bool{}, reports: map[string]int{}, nextID: 1}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// API is the base URI of the fake's API.
func (s *Server) API() string { return s.URL + "/api" }

// Logins is how many logins succeeded.
func (s *Server) Logins() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logins
}

// Requests are the requests so far, logins too.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

// Last is the last request that was not a login.
func (s *Server) Last() Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.requests) - 1; i >= 0; i-- {
		if s.requests[i].Path != "/api/login" {
			return s.requests[i]
		}
	}
	return Request{}
}

// RevokeAll revokes every token, as POST /logout or a deleted token does.
func (s *Server) RevokeAll() {
	s.mu.Lock()
	s.tokens = map[string]bool{}
	s.mu.Unlock()
}

// AddMovie adds a movie as the site has it, with slug, title and status.
func (s *Server) AddMovie(slug, title, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.newMovie()
	m["slug"], m["title"], m["status"] = slug, title, status
	if status == "draft" {
		m["published_at"] = nil
	}
	m["url"] = s.URL + "/movies/" + slug
	s.movies = append(s.movies, m)
}

// Movie is the movie slug as the fake keeps it, nil when there is none.
func (s *Server) Movie(slug string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.find(slug); i >= 0 {
		return s.movies[i]
	}
	return nil
}

// Report adds n open broken-video reports to movie slug.
func (s *Server) Report(slug string, n int) {
	s.mu.Lock()
	s.reports[slug] += n
	s.mu.Unlock()
}

func (s *Server) newMovie() map[string]any {
	id := s.nextID
	s.nextID++
	return map[string]any{"id": id, "synopsis": nil, "thumbnail_url": nil, "embed_url": "https://vidara.to/e/x",
		"backup_embed_urls": []any{}, "release_year": nil, "duration_minutes": nil, "categories": []any{}, "tags": []any{},
		"views": 0, "rating_avg": 0, "rating_count": 0, "status": "published", "published_at": "2026-11-07T07:47:08+00:00"}
}

func (s *Server) find(slug string) int {
	return slices.IndexFunc(s.movies, func(m map[string]any) bool { return m["slug"] == slug })
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]any{"message": msg})
}

// invalid answers a 422 with errors in the order given: field, message,
// field, message...
func invalid(w http.ResponseWriter, pairs ...string) {
	var b strings.Builder
	b.WriteString(`{"message":` + strconv.Quote(pairs[1]) + `,"errors":{`)
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%q:[%q]", pairs[i], pairs[i+1])
	}
	b.WriteString("}}")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(422)
	w.Write([]byte(b.String()))
}

var moviePath = regexp.MustCompile(`^/api/movies/([^/]+)$`)
var resolvePath = regexp.MustCompile(`^/api/reports/([^/]+)/resolve$`)

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	req := Request{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, ContentType: r.Header.Get("Content-Type"), Auth: r.Header.Get("Authorization")}
	fields := map[string]any{} // the body's fields, as JSON would have them
	mt, _, _ := mime.ParseMediaType(req.ContentType)
	switch mt {
	case "application/json":
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &req.JSON)
		for k, v := range req.JSON {
			fields[k] = v
		}
	case "multipart/form-data":
		if err := r.ParseMultipartForm(8 << 20); err == nil {
			req.Form = r.MultipartForm.Value
			for k, vs := range r.MultipartForm.Value {
				if name, ok := strings.CutSuffix(k, "[]"); ok {
					list := []any{}
					for _, v := range vs {
						list = append(list, v)
					}
					fields[name] = list
				} else {
					fields[k] = vs[0]
				}
			}
			if fh := r.MultipartForm.File["thumbnail"]; len(fh) > 0 {
				req.File, req.FileSize = fh[0].Filename, int(fh[0].Size)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	if r.Header.Get("Accept") != "application/json" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(302)
		return
	}
	if r.URL.Path == "/api/login" && r.Method == "POST" {
		if s.LoginStatus != 0 {
			fail(w, s.LoginStatus, "Too Many Attempts.")
			return
		}
		if fields["email"] != s.Email || fields["password"] != s.Password {
			invalid(w, "email", "Invalid credentials.")
			return
		}
		s.logins++
		tok := fmt.Sprintf("%d|tok%d", s.logins, s.logins)
		s.tokens[tok] = true
		reply(w, 200, map[string]any{"token": tok, "token_type": "Bearer", "user": map[string]any{"id": 1, "name": "Admin", "email": s.Email}})
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(404)
		w.Write([]byte("<!DOCTYPE html><html><body>Not Found</body></html>"))
		return
	}
	admin := s.tokens[strings.TrimPrefix(req.Auth, "Bearer ")] && !s.RefuseTokens
	// GET /movies is public: a token it does not know is no token, and
	// only an admin's lists drafts and scheduled movies.
	if r.URL.Path == "/api/movies" && r.Method == "GET" {
		if st := r.URL.Query().Get("status"); st != "" && st != "published" && !admin {
			fail(w, 403, "Only admins can list drafts and scheduled movies.")
			return
		}
	} else if !admin {
		fail(w, 401, "Unauthenticated.")
		return
	}
	switch m := moviePath.FindStringSubmatch(r.URL.Path); {
	case r.URL.Path == "/api/me" && r.Method == "GET":
		reply(w, 200, map[string]any{"id": 1, "name": "Admin", "email": s.Email})
	case r.URL.Path == "/api/movies" && r.Method == "POST":
		var missing []string
		for _, f := range []string{"title", "embed_url"} {
			if v, _ := fields[f].(string); v == "" {
				missing = append(missing, f, "The "+strings.ReplaceAll(f, "_", " ")+" field is required.")
			}
		}
		if len(missing) > 0 {
			invalid(w, missing...)
			return
		}
		if e, _ := fields["embed_url"].(string); strings.HasPrefix(e, "https://bad") {
			invalid(w, "embed_url", "The embed url field must be a valid URL.")
			return
		}
		movie := s.newMovie()
		movie["slug"] = slugOf(fields["title"].(string))
		s.apply(movie, fields, req)
		if s.find(movie["slug"].(string)) >= 0 {
			invalid(w, "slug", "The slug has already been taken.")
			return
		}
		s.movies = append(s.movies, movie)
		reply(w, 201, map[string]any{"data": movie})
	case m != nil && (r.Method == "PATCH" || r.Method == "PUT" || r.Method == "POST"):
		i := s.find(m[1])
		if i < 0 {
			fail(w, 404, "No query results for model [App\\Models\\Movie].")
			return
		}
		s.apply(s.movies[i], fields, req)
		reply(w, 200, map[string]any{"data": s.movies[i]})
	case m != nil && r.Method == "DELETE":
		i := s.find(m[1])
		if i < 0 {
			fail(w, 404, "No query results for model [App\\Models\\Movie].")
			return
		}
		s.movies = slices.Delete(s.movies, i, i+1)
		w.WriteHeader(204)
	case r.URL.Path == "/api/movies" && r.Method == "GET":
		q := r.URL.Query()
		status := q.Get("status")
		if status == "" {
			status = "published"
		}
		var list []map[string]any
		for _, mv := range s.movies {
			if (status == "all" || mv["status"] == status) && strings.Contains(strings.ToLower(mv["title"].(string)), strings.ToLower(q.Get("q"))) {
				list = append(list, mv)
			}
		}
		per, _ := strconv.Atoi(q.Get("per_page"))
		if per == 0 {
			per = 24
		}
		page, _ := strconv.Atoi(q.Get("page"))
		if page == 0 {
			page = 1
		}
		last := max(1, (len(list)+per-1)/per)
		from, to := min(len(list), (page-1)*per), min(len(list), page*per)
		data := list[from:to]
		if data == nil {
			data = []map[string]any{}
		}
		reply(w, 200, map[string]any{"data": data, "links": map[string]any{},
			"meta": map[string]any{"current_page": page, "last_page": last, "per_page": per, "total": len(list)}})
	case r.URL.Path == "/api/reports" && r.Method == "GET":
		data := []any{}
		for _, mv := range s.movies {
			if n := s.reports[mv["slug"].(string)]; n > 0 {
				data = append(data, map[string]any{"movie": map[string]any{"slug": mv["slug"], "title": mv["title"]}, "open_reports": n,
					"servers": map[string]any{"1": n}, "reasons": map[string]any{"not_loading": n}, "latest_at": "2026-11-08T03:12:00+00:00"})
			}
		}
		reply(w, 200, map[string]any{"data": data})
	default:
		if m := resolvePath.FindStringSubmatch(r.URL.Path); m != nil && r.Method == "POST" {
			if s.find(m[1]) < 0 {
				fail(w, 404, "No query results for model [App\\Models\\Movie].")
				return
			}
			n := s.reports[m[1]]
			delete(s.reports, m[1])
			reply(w, 200, map[string]any{"resolved": n})
			return
		}
		fail(w, 404, "The route "+strings.TrimPrefix(r.URL.Path, "/")+" could not be found.")
	}
}

// apply sets a movie's fields as the site's MovieWriter does, roughly.
func (s *Server) apply(movie, fields map[string]any, req Request) {
	for k, v := range fields {
		switch k {
		case "title", "embed_url", "synopsis", "thumbnail_url", "slug":
			movie[k] = v
			if k == "slug" {
				movie["url"] = s.URL + "/movies/" + v.(string)
			}
		case "release_year", "duration_minutes":
			switch n := v.(type) {
			case float64:
				movie[k] = int(n)
			case string:
				movie[k], _ = strconv.Atoi(n)
			}
		case "backup_embed_urls":
			if l, ok := v.([]any); ok {
				movie[k] = l
			} else {
				movie[k] = []any{}
			}
		case "categories", "tags":
			list := []any{}
			if l, ok := v.([]any); ok {
				for _, name := range l {
					list = append(list, map[string]any{"name": name, "slug": slugOf(name.(string))})
				}
			}
			movie[k] = list
		case "remove_thumbnail":
			if v == true || v == "1" {
				movie["thumbnail_url"] = nil
			}
		case "status":
			movie["status"] = v
			switch v {
			case "draft":
				movie["published_at"] = nil
			case "published":
				if movie["published_at"] == nil {
					movie["published_at"] = time.Now().UTC().Format(time.RFC3339)
				}
			}
		case "published_at":
			if t, err := time.Parse(time.RFC3339, fmt.Sprint(v)); err == nil {
				movie["published_at"] = t.UTC().Format("2006-01-02T15:04:05+00:00")
			}
		}
	}
	if req.File != "" {
		movie["thumbnail_url"] = s.URL + "/uploads/thumbnails/" + req.File
	}
	if movie["url"] == nil {
		movie["url"] = s.URL + "/movies/" + movie["slug"].(string)
	}
}

func slugOf(s string) string {
	return strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(s), "-"), "-")
}
