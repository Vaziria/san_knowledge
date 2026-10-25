package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// tree makes files under a new folder and returns it, with /.
func tree(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(root)
}

func TestGlobFiles(t *testing.T) {
	root := tree(t, "a.md", "b.txt", "x/c.md", "x/y/d.md", "x/y/e.mp4", "z.md/f.md")
	for pattern, want := range map[string]string{
		"*.md":        "a.md",
		"*/*.md":      "x/c.md z.md/f.md",
		"**/*.md":     "a.md x/c.md x/y/d.md z.md/f.md",
		"x/**/*":      "x/c.md x/y/d.md x/y/e.mp4",
		"x/y/?.m*":    "x/y/d.md x/y/e.mp4",
		"x/c.md":      "x/c.md",
		"x/none.md":   "",
		"x/y":         "", // a folder is no file
		"[ab].*":      "a.md b.txt",
		"**/*.mp4":    "x/y/e.mp4",
		"*/y/*.mp4":   "x/y/e.mp4",
		"x/**/y/*.md": "x/y/d.md",
	} {
		got, err := globFiles(root + "/" + pattern)
		var rel []string
		for _, f := range got {
			rel = append(rel, strings.TrimPrefix(f, root+"/"))
		}
		if err != nil || strings.Join(rel, " ") != want {
			t.Errorf("%s: %q, %v; want %q", pattern, rel, err, want)
		}
	}
	if _, err := globFiles(root + "/gone/*.md"); err == nil || !strings.Contains(err.Error(), "is not there") {
		t.Errorf("no folder: %v", err)
	}
	if _, err := globFiles(root + "/[a.md"); err == nil || !strings.Contains(err.Error(), "is not a pattern") {
		t.Errorf("bad pattern: %v", err)
	}
}

func TestGlobLoop(t *testing.T) {
	root := tree(t, "a.md", "sub/b.md", "sub/c.txt")
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("ev", TopicEvent, "Topic", "files"),
			step("g", "glob", "Pattern", root+"/**/*.md"),
			step("fmt", "format-text", "Format", "got {{ text }}"),
			step("each", "log"),
			step("count", "format-text", "Format", "{{ text }} files"),
			step("done", "log"),
		},
		Links: []*v1.Link{
			link("ev", "Event", "g", "Event"),
			link("g", "Each", "each", "Event"),
			link("g", "Item", "fmt", "Text"),
			link("fmt", "Text", "each", "Message"),
			link("g", "Done", "done", "Event"),
			link("g", "Count", "count", "Text"),
			link("count", "Text", "done", "Message"),
		},
	}
	// Each file's branch runs to its end before the next, and Format Text,
	// a pure step reading Item, is worked out again for each.
	steps, err := Execute(context.Background(), g, "ev", nil, Env{})
	want := "ev:Event: fmt:: each::got " + root + "/a.md fmt:: each::got " + root + "/sub/b.md g:Each,Done: count:: done::2 files"
	if err != nil || ran(steps) != want {
		t.Fatalf("%s, %v\nwant %s", ran(steps), err, want)
	}

	// No file: Done only.
	g.Steps[1] = step("g", "glob", "Pattern", root+"/*.mp4")
	if steps, err := Execute(context.Background(), g, "ev", nil, Env{}); err != nil || ran(steps) != "ev:Event: g:Done: count:: done::0 files" {
		t.Errorf("no file: %s, %v", ran(steps), err)
	}

	// A step of the loop that fails ends the run, even with Glob's Failed
	// linked: the failure is not Glob's.
	g.Steps[1] = step("g", "glob", "Pattern", root+"/**/*.md")
	g.Steps[2] = step("fmt", "pick-greater", "B", "1")
	g.Links[2] = link("g", "Item", "fmt", "A")
	g.Links[3] = link("fmt", "Value", "each", "Message")
	g.Steps = append(g.Steps, step("failed", "log", "Message", "glob failed"))
	g.Links = append(g.Links, link("g", "Failed", "failed", "Event"))
	steps, err = Execute(context.Background(), g, "ev", nil, Env{})
	if err == nil || !strings.Contains(err.Error(), "not two numbers or two times") || strings.Contains(ran(steps), "glob failed") {
		t.Errorf("failing loop: %s, %v", ran(steps), err)
	}

	// Glob's own failure goes by Failed.
	g.Steps[1] = step("g", "glob", "Pattern", root+"/gone/*.md")
	if steps, err := Execute(context.Background(), g, "ev", nil, Env{}); err != nil || !strings.HasSuffix(ran(steps), "failed::glob failed") {
		t.Errorf("glob fails: %s, %v", ran(steps), err)
	}
}

func TestDownloadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files/report.pdf":
			w.Write([]byte("pdf"))
		case "/named":
			w.Header().Set("Content-Disposition", `attachment; filename="a: b.txt"`)
			w.Write([]byte("named"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	data := filepath.ToSlash(t.TempDir())
	run := func(url, to string) (string, error) {
		t.Helper()
		c := &call{
			ctx: context.Background(), env: Env{DataDir: data}, step: step("d", "download-file"),
			inputs:  map[string]proto.Message{"URL": wrapperspb.String(url), "Save To": wrapperspb.String(to)},
			outputs: map[string]proto.Message{},
		}
		if err := downloadFile(c); err != nil {
			return "", err
		}
		if len(c.fired) != 1 || c.fired[0] != "Done" {
			t.Errorf("fired %v", c.fired)
		}
		return c.outputs["File"].(*wrapperspb.StringValue).Value, nil
	}
	for _, c := range []struct{ url, to, file, body string }{
		{"/files/report.pdf", "", "report.pdf", "pdf"},
		{"/files/report.pdf", "sub/x.bin", "sub/x.bin", "pdf"},
		{"/files/report.pdf", "new/", "new/report.pdf", "pdf"},
		{"/named", "sub", "sub/a_ b.txt", "named"}, // sub is a folder by now
		{"/files/report.pdf", data + "/abs.pdf", "abs.pdf", "pdf"},
	} {
		got, err := run(srv.URL+c.url, c.to)
		if err != nil || got != data+"/"+c.file {
			t.Errorf("%s to %q: %q, %v; want %s", c.url, c.to, got, err, data+"/"+c.file)
			continue
		}
		if b, err := os.ReadFile(filepath.FromSlash(got)); err != nil || string(b) != c.body {
			t.Errorf("%s: %q, %v", got, b, err)
		}
	}
	if _, err := run(srv.URL+"/gone.zip", "gone.zip"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.FromSlash(data), "*.part")); len(left) > 0 {
		t.Errorf("left behind: %v", left)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(data), "gone.zip")); err == nil {
		t.Error("a failed download left gone.zip")
	}
	if _, err := run("ftp://example.com/a", ""); err == nil || !strings.Contains(err.Error(), "http or https") {
		t.Errorf("ftp: %v", err)
	}
}
