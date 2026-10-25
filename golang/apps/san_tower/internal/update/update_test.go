package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		less bool
	}{
		{"v1.1.0", "v1.2.0", true},
		{"v1.2.0", "v1.1.9", false},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.0-rc1", "v1.2.0", true},
		{"v1.2.0", "v1.2.0-rc1", false},
		{"v1.9.0", "v1.10.0", true},
		{"v2.0.0", "v1.99.99", false},
	} {
		a, okA := Parse(c.a)
		b, okB := Parse(c.b)
		if !okA || !okB || a.Less(b) != c.less {
			t.Errorf("%s < %s = %v, want %v", c.a, c.b, a.Less(b), c.less)
		}
	}
	for _, bad := range []string{"2026.10.26-0050", "dev", "1.2.0", "v1.2", "v1.x.0", ""} {
		if _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) took it", bad)
		}
	}
}

func TestAssetFor(t *testing.T) {
	r := Release{Tag: "v1.2.0", Assets: []Asset{{Name: "san_tower-v1.2.0-windows-amd64.zip"}, {Name: "san_tower-v1.2.0-linux-arm64.tar.gz"}}}
	if a, err := r.AssetFor("linux", "arm64"); err != nil || a.Name != "san_tower-v1.2.0-linux-arm64.tar.gz" {
		t.Errorf("linux/arm64: %v, %v", a, err)
	}
	if a, err := r.AssetFor("windows", "amd64"); err != nil || a.Name != "san_tower-v1.2.0-windows-amd64.zip" {
		t.Errorf("windows/amd64: %v, %v", a, err)
	}
	if _, err := r.AssetFor("linux", "arm"); err == nil || !strings.Contains(err.Error(), "no build for linux/arm") {
		t.Errorf("linux/arm: %v", err)
	}
}

// bundle is a release bundle as the release makes it, in a san_tower/
// folder, with an entry that tries to climb out, which is skipped.
func bundle(t *testing.T, zipped bool, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	if zipped {
		z := zip.NewWriter(&b)
		for name, body := range files {
			w, _ := z.Create("san_tower/" + name)
			w.Write([]byte(body))
		}
		w, _ := z.Create("san_tower/../evil.txt")
		w.Write([]byte("no"))
		z.Close()
		return b.Bytes()
	}
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "san_tower/", Typeflag: tar.TypeDir, Mode: 0o755})
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: "san_tower/" + name, Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	tw.WriteHeader(&tar.Header{Name: "san_tower/../evil.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2})
	tw.Write([]byte("no"))
	tw.Close()
	gz.Close()
	return b.Bytes()
}

// Install from a fake GitHub: the binary, the README and the extension's
// folder replace the old ones; nothing climbs out or stays behind.
func TestInstall(t *testing.T) {
	defer func(v func(context.Context, string, string) error) { verify = v }(verify)
	verified := ""
	verify = func(_ context.Context, file, tag string) error {
		b, _ := os.ReadFile(file)
		verified = tag + ":" + string(b)
		return nil
	}
	bin := binary(runtime.GOOS)
	for _, zipped := range []bool{false, true} {
		ext := ".tar.gz"
		if zipped {
			ext = ".zip"
		}
		data := bundle(t, zipped, map[string]string{
			bin:                                  "new binary",
			"README.txt":                         "new readme",
			"san_remote_extension/manifest.json": `{"version": "1.2.0"}`,
			"san_remote_extension/assets/ui.js":  "ui",
		})
		name := "san_tower-v1.2.0-" + runtime.GOOS + "-" + runtime.GOARCH + ext
		mux := http.NewServeMux()
		ts := httptest.NewServer(mux)
		mux.HandleFunc("/repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(Release{Tag: "v1.2.0", Assets: []Asset{{Name: name, URL: ts.URL + "/dl/" + name, Size: int64(len(data))}}})
		})
		mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) { w.Write(data) })
		API = ts.URL

		dir := t.TempDir()
		exe := filepath.Join(dir, bin)
		os.WriteFile(exe, []byte("old binary"), 0o755)
		os.WriteFile(filepath.Join(dir, "README.txt"), []byte("old readme"), 0o644)
		os.MkdirAll(filepath.Join(dir, "san_remote_extension"), 0o755)
		os.WriteFile(filepath.Join(dir, "san_remote_extension", "gone.js"), []byte("old"), 0o644)
		os.MkdirAll(filepath.Join(dir, "data"), 0o755)

		rel, err := Latest(context.Background(), ts.Client())
		if err != nil {
			t.Fatal(err)
		}
		a := rel.Assets[0] // both kinds of bundle here; AssetFor's choice is TestAssetFor's
		if err := Install(context.Background(), ts.Client(), a, rel.Tag, exe); err != nil {
			t.Fatalf("%s: %v", ext, err)
		}
		ts.Close()
		read := func(p ...string) string {
			b, _ := os.ReadFile(filepath.Join(append([]string{dir}, p...)...))
			return string(b)
		}
		if read(bin) != "new binary" || read("README.txt") != "new readme" || read("san_remote_extension", "assets", "ui.js") != "ui" || verified != "v1.2.0:new binary" {
			t.Errorf("%s: installed %q %q %q, verified %q", ext, read(bin), read("README.txt"), read("san_remote_extension", "assets", "ui.js"), verified)
		}
		if _, err := os.Stat(filepath.Join(dir, "san_remote_extension", "gone.js")); err == nil {
			t.Errorf("%s: the old extension's file stayed", ext)
		}
		Clean(exe)
		left, _ := os.ReadDir(dir)
		var names []string
		for _, e := range left {
			names = append(names, e.Name())
		}
		if got := strings.Join(names, ","); got != fmt.Sprintf("README.txt,data,%s,san_remote_extension", bin) && got != fmt.Sprintf("README.txt,data,san_remote_extension,%s", bin) {
			t.Errorf("%s: left %s", ext, got)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil.txt")); err == nil {
			t.Errorf("%s: an entry climbed out", ext)
		}
	}
}

// A new binary that does not run stops the update before anything moves.
func TestInstallVerifyFails(t *testing.T) {
	defer func(v func(context.Context, string, string) error) { verify = v }(verify)
	verify = func(context.Context, string, string) error { return fmt.Errorf("exec format error") }
	data := bundle(t, runtime.GOOS == "windows", map[string]string{binary(runtime.GOOS): "arm binary"})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer ts.Close()
	dir := t.TempDir()
	exe := filepath.Join(dir, binary(runtime.GOOS))
	os.WriteFile(exe, []byte("old binary"), 0o755)
	name := "x.tar.gz"
	if runtime.GOOS == "windows" {
		name = "x.zip"
	}
	if err := Install(context.Background(), ts.Client(), Asset{Name: name, URL: ts.URL}, "v1.2.0", exe); err == nil || !strings.Contains(err.Error(), "exec format") {
		t.Fatalf("err = %v", err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old binary" {
		t.Errorf("the binary was replaced: %q", b)
	}
}
