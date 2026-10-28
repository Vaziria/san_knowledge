// Package settings keeps the tower's settings, the ones changed from the UI,
// in tower.json in the tower's data folder (--data).
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// File is the settings file's name in the tower's data folder.
const File = "tower.json"

type Settings struct {
	Broker        Broker        `json:"broker"`
	BrowserRemote BrowserRemote `json:"browser_remote"`
}

// Broker is the setup of the San Message broker the tower uses: its own,
// running inside it, or a separate san_message server.
type Broker struct {
	// DataDir is the data folder of the broker inside the tower: empty for
	// the default, a relative path is inside the tower's data folder.
	DataDir string `json:"data_dir,omitempty"`
	// Remote says the tower uses the san_message server at URI instead of
	// its own. Both settings are kept either way.
	Remote bool   `json:"remote,omitempty"`
	URI    string `json:"uri,omitempty"`
}

// BrowserRemote is the San Remote server the tower can run, for the browser
// extension and the site services (golang/apps/san_remote).
type BrowserRemote struct {
	// Active: the tower runs it, and starts it when the tower starts.
	Active bool `json:"active,omitempty"`
	// Network: it listens on every network, not only on this computer
	// (the user, 2026-10-28), so a browser or a tower on another machine of
	// the local network can reach it.
	Network bool `json:"network,omitempty"`
}

// mu serializes Update, so two parts of the tower saving their settings at
// once do not undo each other's change.
var mu sync.Mutex

// Update changes the settings in dir with change and saves them.
func Update(dir string, change func(*Settings)) error {
	mu.Lock()
	defer mu.Unlock()
	s, err := Load(dir)
	if err != nil {
		return err
	}
	change(&s)
	return Save(dir, s)
}

// Load reads the settings in dir. With no settings file yet, every setting
// is its default. A broken file is an error rather than the defaults, which
// would quietly point the broker at another folder.
func Load(dir string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(filepath.Join(dir, File))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("%s: %w", filepath.Join(dir, File), err)
	}
	return s, nil
}

// Save writes the settings in dir, creating it. The file is replaced whole,
// so a crash leaves the old settings or the new ones, never half of them.
func Save(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, File+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, File))
}
