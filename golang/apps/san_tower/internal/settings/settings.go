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
)

// File is the settings file's name in the tower's data folder.
const File = "tower.json"

type Settings struct {
	Broker Broker `json:"broker"`
}

// Broker is the setup of the San Message broker the tower runs.
type Broker struct {
	// DataDir is the broker's data folder: empty for the default, a relative
	// path is inside the tower's data folder.
	DataDir string `json:"data_dir,omitempty"`
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
