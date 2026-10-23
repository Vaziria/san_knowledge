package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHandler(t *testing.T) {
	h := Handler(fstest.MapFS{
		"index.html":      {Data: []byte("<html>tower</html>")},
		"assets/index.js": {Data: []byte("console.log(1)")},
	})
	for _, c := range []struct {
		path string
		code int
		body string
	}{
		{"/", 200, "<html>tower</html>"},
		{"/index.html", 200, "<html>tower</html>"},
		{"/some/route", 200, "<html>tower</html>"}, // a react-router page, reloaded
		{"/assets/index.js", 200, "console.log(1)"},
		{"/assets/missing.js", 404, ""},
	} {
		rec := get(h, c.path)
		if rec.Code != c.code {
			t.Errorf("GET %s: code %d, want %d", c.path, rec.Code, c.code)
		}
		if c.body != "" && rec.Body.String() != c.body {
			t.Errorf("GET %s: body %q, want %q", c.path, rec.Body.String(), c.body)
		}
	}
}

func TestHandlerNotBuilt(t *testing.T) {
	if rec := get(Handler(fstest.MapFS{".gitkeep": {}}), "/"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code %d, want 503", rec.Code)
	}
}
