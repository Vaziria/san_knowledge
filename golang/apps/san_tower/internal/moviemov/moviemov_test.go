package moviemov_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/san_tower/internal/moviemov"
	"github.com/wargasipil/san_tower/internal/moviemov/moviemovtest"
)

func site(t *testing.T) *moviemovtest.Server {
	t.Helper()
	s := moviemovtest.New("admin@moviemov.test", "rahasia")
	t.Cleanup(s.Close)
	return s
}

func login(s *moviemovtest.Server) moviemov.Login {
	return moviemov.Login{Base: s.API(), Email: "admin@moviemov.test", Password: "rahasia"}
}

// One login serves every call of a connection, also from several runs at
// once; the token goes as a bearer, the login as JSON with the tower's
// device name, and the password nowhere else.
func TestTokenKept(t *testing.T) {
	s := site(t)
	sessions := moviemov.NewSessions()
	var lines []string
	var mu sync.Mutex
	log := func(l string) { mu.Lock(); lines = append(lines, l); mu.Unlock() }
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if u, err := sessions.Client(7, login(s), log).Me(context.Background()); err != nil || u.Name != "Admin" {
				t.Errorf("Me = %v, %v", u, err)
			}
		}()
	}
	wg.Wait()
	if _, err := sessions.Client(7, login(s), log).Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := s.Logins(); n != 1 {
		t.Errorf("logins = %d, want 1", n)
	}
	reqs := s.Requests()
	first := reqs[0]
	if first.Path != "/api/login" || first.JSON["email"] != "admin@moviemov.test" || first.JSON["password"] != "rahasia" || first.JSON["device_name"] != "san-tower" || first.Auth != "" {
		t.Errorf("login = %+v", first)
	}
	for _, r := range reqs[1:] {
		if r.Auth != "Bearer 1|tok1" || r.Path != "/api/me" {
			t.Errorf("call = %+v", r)
		}
	}
	if strings.Join(lines, "\n") != "logged in to "+s.API()+" as admin@moviemov.test (a new token, san-tower)" {
		t.Errorf("log = %q", lines)
	}
	for _, l := range lines {
		if strings.Contains(l, "rahasia") || strings.Contains(l, "tok1") {
			t.Errorf("a secret in the log: %q", l)
		}
	}

	// Another connection, or this one with another password, logs in anew.
	if _, err := sessions.Client(8, login(s), nil).Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Password = "baru"
	l := login(s)
	l.Password = "baru"
	if _, err := sessions.Client(7, l, nil).Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := s.Logins(); n != 3 {
		t.Errorf("logins = %d, want 3", n)
	}
}

// A kept token the site refuses is replaced once; a new one it refuses is
// the error, without logging in again.
func TestLoginAgainOn401(t *testing.T) {
	s := site(t)
	sessions := moviemov.NewSessions()
	var lines []string
	c := sessions.Client(1, login(s), func(l string) { lines = append(lines, l) })
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.RevokeAll()
	if _, err := c.Me(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := s.Logins(); n != 2 {
		t.Errorf("logins = %d, want 2", n)
	}
	if len(lines) != 3 || lines[1] != "the site refused the kept token (401): logging in again" {
		t.Errorf("log = %q", lines)
	}
	if last := s.Last(); last.Auth != "Bearer 2|tok2" {
		t.Errorf("retried with %q", last.Auth)
	}

	// GET /movies is public: the site reads a revoked token as none, and
	// refuses drafts with a 403.
	s.RevokeAll()
	p, err := c.Movies(context.Background(), url.Values{"status": {"draft"}})
	if err != nil || p.Meta.Total != 0 {
		t.Fatalf("drafts: %v, %v", p, err)
	}
	if n := s.Logins(); n != 3 || lines[len(lines)-2] != "the site refused the kept token (403): logging in again" {
		t.Errorf("logins = %d, log = %q", n, lines)
	}
	// A new token refused is the error, with no other login.
	s.RefuseTokens = true
	_, err = sessions.Client(2, login(s), nil).Me(context.Background())
	if !moviemov.IsStatus(err, 401) || s.Logins() != 4 {
		t.Errorf("fresh token refused: %v, %d logins", err, s.Logins())
	}
}

// Wrong credentials, the site's limit, and the tower's own limit say what
// to do; the tower holds the sixth login in a minute back itself.
func TestLoginRefused(t *testing.T) {
	s := site(t)
	sessions := moviemov.NewSessions()
	now := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
	sessions.Now = func() time.Time { return now }
	wrong := login(s)
	wrong.Password = "salah"
	_, err := sessions.Client(1, wrong, nil).Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "MovieMov login as admin@moviemov.test at "+s.API()+": MovieMov: email: Invalid credentials. (422): check the connection's username and password") {
		t.Errorf("wrong password: %v", err)
	}
	if strings.Contains(err.Error(), "salah") {
		t.Errorf("the password in an error: %v", err)
	}

	s.LoginStatus = 429
	_, err = sessions.Client(1, login(s), nil).Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Too Many Attempts. (429): the site takes 5 logins a minute") {
		t.Errorf("429: %v", err)
	}
	s.LoginStatus = 0
	for i := range 3 {
		if _, err := sessions.Client(uint32(10+i), login(s), nil).Me(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	before := len(s.Requests())
	_, err = sessions.Client(20, login(s), nil).Me(context.Background())
	if !errors.Is(err, moviemov.ErrTooManyLogins) || !strings.Contains(err.Error(), "tried to log in to "+s.API()+" 5 times in the last minute") || !strings.Contains(err.Error(), "try again in 60s") {
		t.Errorf("sixth login: %v", err)
	}
	if len(s.Requests()) != before {
		t.Error("the sixth login reached the site")
	}
	now = now.Add(61 * time.Second)
	if _, err := sessions.Client(20, login(s), nil).Me(context.Background()); err != nil {
		t.Errorf("a minute later: %v", err)
	}
}

// A 422 names each field and its message, in the site's order; a page that
// is not the API says to check the base URI.
func TestErrors(t *testing.T) {
	s := site(t)
	sessions := moviemov.NewSessions()
	c := sessions.Client(1, login(s), nil)
	_, err := c.CreateMovie(context.Background(), moviemov.Movie{Fields: map[string]any{"synopsis": "x"}})
	if err == nil || err.Error() != "MovieMov: title: The title field is required.; embed_url: The embed url field is required. (422)" {
		t.Errorf("422 = %v", err)
	}
	var e *moviemov.Error
	if !errors.As(err, &e) || len(e.Fields) != 2 || e.Fields[0].Field != "title" || e.Fields[1].Field != "embed_url" {
		t.Errorf("fields = %+v", e)
	}

	wrong := sessions.Client(2, moviemov.Login{Base: s.URL + "/nope", Email: "admin@moviemov.test", Password: "rahasia"}, nil)
	_, err = wrong.Me(context.Background())
	if err == nil || !strings.Contains(err.Error(), "404 Not Found, not JSON (<!DOCTYPE html><html><body>Not Found</body></html>)") || !strings.Contains(err.Error(), "is the Base URI the site's API, such as https://moviemov.id/api?") {
		t.Errorf("not the API: %v", err)
	}
	if err := c.DeleteMovie(context.Background(), "gone"); !moviemov.IsStatus(err, 404) {
		t.Errorf("delete gone: %v", err)
	}
}

// A movie without a file goes as JSON, lists as arrays; with a file as a
// form: lists as name[], a bool as 1, an empty list as an empty value, and
// the file with its type. An update with a file is a POST, else a PATCH.
func TestMovieBodies(t *testing.T) {
	s := site(t)
	c := moviemov.NewSessions().Client(1, login(s), nil)
	ctx := context.Background()
	raw, err := c.CreateMovie(ctx, moviemov.Movie{Fields: map[string]any{"title": "Night Train", "embed_url": "https://vidara.to/e/a",
		"categories": []string{"Drama, Noir", "Thriller"}, "release_year": 2024, "status": "draft"}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	if m["slug"] != "night-train" {
		t.Errorf("movie = %v", m)
	}
	r := s.Last()
	if r.Method != "POST" || r.Path != "/api/movies" || r.ContentType != "application/json" {
		t.Errorf("request = %+v", r)
	}
	if b, _ := json.Marshal(r.JSON); string(b) != `{"categories":["Drama, Noir","Thriller"],"embed_url":"https://vidara.to/e/a","release_year":2024,"status":"draft","title":"Night Train"}` {
		t.Errorf("JSON = %s", b)
	}

	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 100)...)
	if _, err := c.UpdateMovie(ctx, "night-train", moviemov.Movie{Fields: map[string]any{"title": "Night Train 2", "tags": []string{}, "categories": []string{"Drama, Noir"}, "remove_thumbnail": true},
		Thumbnail: &moviemov.File{Name: "D:/x/clip_thumb.jpg", Data: jpeg}}); err != nil {
		t.Fatal(err)
	}
	r = s.Last()
	if r.Method != "POST" || r.Path != "/api/movies/night-train" || !strings.HasPrefix(r.ContentType, "multipart/form-data; boundary=") || r.File != "clip_thumb.jpg" || r.FileSize != 104 {
		t.Errorf("request = %+v", r)
	}
	if f := r.Form; f.Get("title") != "Night Train 2" || f["categories[]"][0] != "Drama, Noir" || len(f["tags"]) != 1 || f.Get("tags") != "" || f.Get("remove_thumbnail") != "1" {
		t.Errorf("form = %v", f)
	}
	if _, err := c.UpdateMovie(ctx, "night-train", moviemov.Movie{Fields: map[string]any{"thumbnail_url": nil}}); err != nil {
		t.Fatal(err)
	}
	if r := s.Last(); r.Method != "PATCH" || r.ContentType != "application/json" {
		t.Errorf("request = %+v", r)
	}
	if b, _ := json.Marshal(s.Last().JSON); string(b) != `{"thumbnail_url":null}` {
		t.Errorf("JSON = %s", b)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"night-train":                            "night-train",
		" night_train_2 ":                        "night_train_2",
		"https://moviemov.id/movies/night-train": "night-train",
		"https://moviemov.id/en/movies/night-train?server=2#player": "night-train",
		"/movies/night-train/":                        "night-train",
		"http://localhost:8000/movies/kereta%20malam": "",
		"":                                    "",
		"night train":                         "",
		"https://moviemov.id/category/sci-fi": "",
	} {
		got, err := moviemov.Slug(in)
		if (err != nil) != (want == "") || got != want {
			t.Errorf("Slug(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestImageType(t *testing.T) {
	for want, data := range map[string][]byte{
		"image/jpeg": {0xFF, 0xD8, 0xFF, 0xDB},
		"image/png":  []byte("\x89PNG\r\n\x1a\n...."),
		"image/webp": []byte("RIFF\x10\x00\x00\x00WEBPVP8 "),
		"":           []byte("GIF89a......"),
	} {
		if got := moviemov.ImageType(data); got != want {
			t.Errorf("ImageType(%q) = %q, want %q", data[:6], got, want)
		}
	}
}
