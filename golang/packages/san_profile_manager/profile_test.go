package san_profile_manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDir(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(filepath.Join("some", "folder"))
	for _, c := range []struct{ in, want string }{
		{"", filepath.Join(root, "default")},
		{"default", filepath.Join(root, "default")},
		{"work", filepath.Join(root, "work")},
		{filepath.Join("some", "folder"), abs},
		{abs, abs},
	} {
		got, err := Dir(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("Dir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFindUp(t *testing.T) {
	ws := t.TempDir()
	deep := filepath.Join(ws, "golang", "packages", "san_x", "bin")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok := findUp(deep, RootName); ok {
		t.Fatal("found a folder that is not there")
	}
	if err := os.Mkdir(filepath.Join(ws, RootName), 0o700); err != nil {
		t.Fatal(err)
	}
	if got, ok := findUp(deep, RootName); !ok || got != filepath.Join(ws, RootName) {
		t.Errorf("findUp = %q, %v", got, ok)
	}
	// A file of that name is not the folder.
	if err := os.WriteFile(filepath.Join(deep, RootName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := findUp(deep, RootName); got != filepath.Join(ws, RootName) {
		t.Errorf("findUp = %q", got)
	}
}

func TestReadPort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, portFile)
	if _, ok := readPort(path); ok {
		t.Error("a missing file gave a port")
	}
	os.WriteFile(path, []byte("53117\n/devtools/browser/0f9c\n"), 0o600)
	if p, ok := readPort(path); !ok || p != 53117 {
		t.Errorf("readPort = %d, %v", p, ok)
	}
	os.WriteFile(path, []byte(""), 0o600) // Chrome is still writing it
	if _, ok := readPort(path); ok {
		t.Error("an empty file gave a port")
	}
}

func readPrefs(t *testing.T, profile string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(profile, "Default", "Preferences"))
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNoBrowserSignin(t *testing.T) {
	// A new profile gets a Preferences file with only the setting.
	fresh := t.TempDir()
	if err := noBrowserSignin(fresh); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"allowed", "allowed_on_next_startup"} {
		if got := readPrefs(t, fresh)["signin"].(map[string]any)[k]; got != false {
			t.Errorf("fresh profile: signin.%s = %v", k, got)
		}
	}

	// An existing profile keeps everything else, including other signin keys.
	used := t.TempDir()
	if err := os.MkdirAll(filepath.Join(used, "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"signin":{"allowed":true,"other":1},"profile":{"name":"x"}}`
	if err := os.WriteFile(filepath.Join(used, "Default", "Preferences"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := noBrowserSignin(used); err != nil {
		t.Fatal(err)
	}
	p := readPrefs(t, used)
	s := p["signin"].(map[string]any)
	if s["allowed"] != false || s["allowed_on_next_startup"] != false || s["other"] != float64(1) {
		t.Errorf("signin = %v", s)
	}
	if p["profile"].(map[string]any)["name"] != "x" {
		t.Errorf("other preferences lost: %v", p)
	}

	// A profile Chrome has open is left alone.
	open := t.TempDir()
	if err := os.WriteFile(filepath.Join(open, "lockfile"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := noBrowserSignin(open); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(open, "Default", "Preferences")); !os.IsNotExist(err) {
		t.Errorf("wrote into a profile in use: %v", err)
	}
}

// A Plain start has chromedp-undetected's flags and the debugging port,
// nothing else; the usual start keeps its background flags.
func TestLaunchArgs(t *testing.T) {
	got := strings.Join(launchArgs(Config{Plain: true}, `D:\p`), " ")
	want := `--lang=en-US --no-first-run --no-default-browser-check --log-level=0 --remote-debugging-host=127.0.0.1 --user-data-dir=D:\p --profile-directory=Default --remote-debugging-port=0 about:blank`
	if got != want {
		t.Errorf("Plain:\n got %s\nwant %s", got, want)
	}
	usual := strings.Join(launchArgs(Config{}, `D:\p`), " ")
	for _, f := range []string{"--disable-renderer-backgrounding", "--disable-sync", "--disable-extensions"} {
		if !strings.Contains(usual, f) {
			t.Errorf("usual start lost %s: %s", f, usual)
		}
	}
	if h := launchArgs(Config{Plain: true, Headless: true}, `D:\p`); h[len(h)-2] != "--headless=new" {
		t.Errorf("Plain and Headless: %v", h)
	}
}
