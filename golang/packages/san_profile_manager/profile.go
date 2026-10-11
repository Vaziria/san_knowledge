// Package san_profile_manager keeps the Chrome profiles that the san_* tools
// (san_medium, san_tiktok, and those to come) drive, in one place, and starts
// Chrome on them the way those tools need it.
//
// A profile is a Chrome user-data folder. One profile can be signed in to
// Medium, TikTok and any other site at once, so every tool uses the same one
// unless told otherwise: sign in to each site once, in any tool, and all tools
// see it. Profiles live under Root, the workspace's browser_profiles folder,
// by name; DefaultName is the one used when none is given.
//
// Only one Chrome can have a profile open. When a tool already has Chrome
// running on it, Start attaches to that Chrome and works in a tab of its own,
// so two tools can run at once. The last one to close its tab closes Chrome.
package san_profile_manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultName is the profile used when none is given.
const DefaultName = "default"

// ErrProfileInUse means a Chrome that no tool can attach to already has the
// profile open: the window Login opened, or one a person started. Close that
// window, or start it with --remote-debugging-port and give that as Remote.
var ErrProfileInUse = errors.New("san_profile_manager: the profile is already open in another Chrome window")

// Config says which profile to use, and how to reach its Chrome.
type Config struct {
	// Profile is a name, such as "default" or "work", kept under Root, or a
	// folder (anything with a path separator in it). Empty is DefaultName.
	Profile string

	// Remote attaches to a Chrome already running with
	// --remote-debugging-port, such as "http://127.0.0.1:9222", instead of
	// using Profile. That Chrome is never closed.
	Remote string

	// ExecPath is the Chrome, Edge or Chromium binary. Empty looks in the
	// usual places (FindChrome).
	ExecPath string

	// Headless starts Chrome without a window, for reading cookies only. A
	// Chrome a tool already has running on the profile is used instead.
	Headless bool

	// Plain starts Chrome with chromedp-undetected's flags (plainFlags) and
	// nothing else, and leaves the profile's settings as they are. On
	// 2026-10-11 Medium's Cloudflare blocked every Chrome started with the
	// sync, extension and background flags of the usual start, and let in
	// the same profile started the chromedp-undetected way (cmd/example).
	// Without the background flags Chrome freezes a page whose window is
	// minimized or covered, so that window has to stay in view while a tool
	// works in it.
	Plain bool
}

// RootName is the name of the workspace folder that holds the profiles. It
// sits at the workspace root, and git ignores what is in it: the profiles
// hold signed-in sessions.
const RootName = "browser_profiles"

// ErrNoRoot means no browser_profiles folder was found, neither in the
// working directory or above it, nor beside the program or above it.
var ErrNoRoot = errors.New("san_profile_manager: no " + RootName + " folder here or above; run inside the workspace, or give the profile as a folder")

// Root is the folder that holds the profiles: the nearest browser_profiles
// folder, looking up from the working directory, then from the running
// program's folder, so a tool built into the workspace finds it from
// anywhere.
func Root() (string, error) {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, s := range starts {
		if dir, ok := findUp(s, RootName); ok {
			return dir, nil
		}
	}
	return "", ErrNoRoot
}

// findUp looks for a folder called name in start and each folder above it.
func findUp(start, name string) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// Dir is the folder of a profile. "" is DefaultName, a name is a folder under
// Root, and a path (absolute, or with a separator in it) is that folder.
func Dir(profile string) (string, error) {
	if profile == "" {
		profile = DefaultName
	}
	if filepath.IsAbs(profile) || strings.ContainsAny(profile, `/\`) || profile == "." || profile == ".." {
		p, err := filepath.Abs(profile)
		if err != nil {
			return "", fmt.Errorf("san_profile_manager: %w", err)
		}
		return p, nil
	}
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, profile), nil
}

// Locked reports whether a Chrome has the profile folder open: the lock file
// Chrome keeps there while it runs (lockfile on Windows, a SingletonLock link
// elsewhere).
func Locked(dir string) bool {
	for _, name := range []string{"lockfile", "SingletonLock"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// flags are the same for Login and for Start. On Linux the cookie store is
// encrypted with a key whose source depends on flags such as --password-store,
// so a profile logged in under one set of flags may look logged out under
// another.
//
// --disable-sync keeps the profile to itself even when Chrome is signed in to
// a Google account: nothing of the user's other browsers (extensions, history,
// settings) comes in, and nothing of the tools' goes out.
func flags(dir string) []string {
	return []string{
		"--user-data-dir=" + dir,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-sync",
		"--start-maximized",
	}
}

// plainFlags are a Plain start's flags: exactly those chromedp-undetected
// gives Chrome when cmd/example starts it, which Medium's Cloudflare let in
// (2026-10-11). The debugging port is added by launchArgs.
func plainFlags(dir string) []string {
	return []string{
		"--lang=en-US",
		"--no-first-run",
		"--no-default-browser-check",
		"--log-level=0",
		"--remote-debugging-host=127.0.0.1",
		"--user-data-dir=" + dir,
		"--profile-directory=Default",
	}
}

// launchArgs are Chrome's arguments for a tool's start: Config.Plain's, or
// the usual ones, which keep a window behind others working.
func launchArgs(c Config, dir string) []string {
	var args []string
	if c.Plain {
		args = append(plainFlags(dir), "--remote-debugging-port=0")
	} else {
		args = append(flags(dir),
			"--remote-debugging-port=0",
			// A window behind other windows is otherwise marked hidden, and
			// Chrome then slows its timers and stops drawing it: menus stop
			// animating and saves and uploads wait.
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
			"--disable-background-timer-throttling",
			// The profile is a browser like any other, and may get
			// extensions; an ad blocker in it can stop a site's scripts.
			"--disable-extensions",
		)
	}
	// Headful, except for reading cookies: sites behind Cloudflare and the
	// like treat a headless browser as a bot. No --enable-automation either,
	// for the same reason Login has none.
	if c.Headless {
		args = append(args, "--headless=new")
	}
	return append(args, "about:blank")
}

// noBrowserSignin turns off "Allow Chrome sign-in" in the profile, so that
// signing in to a site with Google does not also sign Chrome in. There is no
// command-line flag for it. It is written into the profile's preferences while
// Chrome is not running, keeping everything else in the file: the setting is
// signin.allowed_on_next_startup, which Chrome copies into signin.allowed at
// each start (writing only signin.allowed is undone at the next start).
func noBrowserSignin(dir string) error {
	if Locked(dir) {
		return nil // Chrome would overwrite the file
	}
	prefsDir := filepath.Join(dir, "Default")
	if err := os.MkdirAll(prefsDir, 0o700); err != nil {
		return fmt.Errorf("san_profile_manager: %w", err)
	}
	path := filepath.Join(prefsDir, "Preferences")
	prefs := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, &prefs); err != nil {
			return nil // not ours to repair; Chrome rewrites a broken file
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("san_profile_manager: %w", err)
	}
	signin, _ := prefs["signin"].(map[string]any)
	if signin == nil {
		signin = map[string]any{}
	}
	a, _ := signin["allowed"].(bool)
	n, _ := signin["allowed_on_next_startup"].(bool)
	_, hasA := signin["allowed"]
	_, hasN := signin["allowed_on_next_startup"]
	if hasA && hasN && !a && !n {
		return nil
	}
	signin["allowed"] = false
	signin["allowed_on_next_startup"] = false
	prefs["signin"] = signin
	b, err := json.Marshal(prefs)
	if err != nil {
		return fmt.Errorf("san_profile_manager: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("san_profile_manager: %w", err)
	}
	return nil
}

// FindChrome looks for Chrome, then Edge or Chromium, in the usual places,
// and returns "" when there is none.
func FindChrome() string {
	var locations []string
	switch runtime.GOOS {
	case "windows":
		locations = []string{
			"chrome.exe",
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		}
	case "darwin":
		locations = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		locations = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "/snap/bin/chromium"}
	}
	for _, p := range locations {
		if found, err := exec.LookPath(p); err == nil {
			return found
		}
	}
	return ""
}

func (c Config) exec() (string, error) {
	if c.ExecPath != "" {
		return c.ExecPath, nil
	}
	if p := FindChrome(); p != "" {
		return p, nil
	}
	return "", errors.New("san_profile_manager: Chrome not found; give its path as ExecPath")
}
