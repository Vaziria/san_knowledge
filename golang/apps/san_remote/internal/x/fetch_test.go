package x

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A download whose connection breaks goes on where it broke; one the
// server will not give is not asked again, and leaves no file.
func TestFetchResumes(t *testing.T) {
	defer func(w time.Duration) { fetchWait = w }(fetchWait)
	fetchWait = time.Millisecond
	body := strings.Repeat("0123456789", 10000)
	var tries, ranged atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/gone.mp4" {
			tries.Add(1)
			http.NotFound(w, r)
			return
		}
		from := 0
		if rg := r.Header.Get("Range"); rg != "" {
			ranged.Add(1)
			from, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rg, "bytes="), "-"))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, len(body)-1, len(body)))
			w.Header().Set("Content-Length", strconv.Itoa(len(body)-from))
			w.WriteHeader(http.StatusPartialContent)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		}
		rest := body[from:]
		if tries.Add(1) <= 2 { // the first two break a third of the way in
			w.Write([]byte(rest[:len(rest)/3]))
			w.(http.Flusher).Flush()
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		w.Write([]byte(rest))
	}))
	defer srv.Close()
	dir := t.TempDir()

	file := filepath.Join(dir, "a.mp4")
	if err := fetch(context.Background(), srv.URL+"/a.mp4", file); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != body {
		t.Errorf("saved %d bytes, want %d", len(got), len(body))
	}
	if tries.Load() != 3 || ranged.Load() != 2 {
		t.Errorf("tries %d, of them ranged %d", tries.Load(), ranged.Load())
	}

	tries.Store(0)
	err := fetch(context.Background(), srv.URL+"/gone.mp4", filepath.Join(dir, "gone.mp4"))
	if err == nil || !strings.Contains(err.Error(), "404") || tries.Load() != 1 {
		t.Errorf("gone: %v after %d tries", err, tries.Load())
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.part")); len(left) != 0 {
		t.Errorf("left %v", left)
	}
}
