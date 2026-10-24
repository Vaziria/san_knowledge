// Package broker runs the tower's San Message broker
// (golang/packages/san_message), whose data folder is a setting that can
// change while the tower runs: changing it opens a broker in the new folder,
// switches MessageService over to it, then closes the old one. Nothing is
// moved or deleted; the topics and subscribers are those in the folder in
// use.
package broker

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wargasipil/san_message"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"

	"github.com/wargasipil/san_tower/internal/settings"
)

// DefaultDir is the broker's data folder when none is set, inside the
// tower's data folder.
const DefaultDir = "san_message"

// Broker is the tower's broker.
type Broker struct {
	towerDir string // the tower's data folder, absolute
	log      *slog.Logger
	mu       sync.Mutex // serializes SetDataDir and Close
	cur      atomic.Pointer[running]
}

// running is one san_message broker, open in one folder.
type running struct {
	b       *san_message.Broker
	h       http.Handler
	path    string // its data folder, absolute
	dataDir string // the setting that names it, as set
}

// State is the broker's setting and what it resolves to.
type State struct {
	DataDir     string // as set: empty for the default
	DataPath    string // the folder in use, absolute
	DefaultPath string // the folder an empty DataDir means
}

// Open opens the broker in the data folder its setting names, in towerDir.
func Open(towerDir string, log *slog.Logger) (*Broker, error) {
	abs, err := filepath.Abs(towerDir)
	if err != nil {
		return nil, err
	}
	s, err := settings.Load(abs)
	if err != nil {
		return nil, err
	}
	m := &Broker{towerDir: abs, log: log}
	r, err := m.open(s.Broker.DataDir)
	if err != nil {
		return nil, err
	}
	m.cur.Store(r)
	return m, nil
}

// Broker is the san_message broker in use now.
func (m *Broker) Broker() *san_message.Broker { return m.cur.Load().b }

// Handler is MessageService, served by the broker in use at each call.
func (m *Broker) Handler() (string, http.Handler) {
	return "/" + messagev1connect.MessageServiceName + "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.cur.Load().h.ServeHTTP(w, r)
	})
}

func (m *Broker) State() State {
	r := m.cur.Load()
	return State{DataDir: r.dataDir, DataPath: r.path, DefaultPath: m.resolve("")}
}

// SetDataDir moves the broker to the folder dataDir names: empty for the
// default, a relative path is inside the tower's data folder. The folder is
// created if its parent exists. When the broker cannot open there, the old
// one stays in use and the setting is not changed.
func (m *Broker) SetDataDir(dataDir string) error {
	dataDir = strings.TrimSpace(dataDir)
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.cur.Load()
	path := m.resolve(dataDir)
	if samePath(path, old.path) {
		// Another name for the same folder: keep the broker, save the name.
		if err := m.save(dataDir); err != nil {
			return err
		}
		m.cur.Store(&running{b: old.b, h: old.h, path: old.path, dataDir: dataDir})
		return nil
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return fmt.Errorf("%w: %s: the folder above it does not exist", san_message.ErrInvalid, path)
	}
	next, err := m.open(dataDir)
	if err != nil {
		return err
	}
	if err := m.save(dataDir); err != nil {
		next.b.Close()
		return err
	}
	m.cur.Store(next)
	m.log.Info("message broker moved", "from", old.path, "to", next.path)
	// Calls that already reached the old broker finish or get Unavailable;
	// its pull subscribers reconnect to the new one.
	return old.b.Close()
}

// Close closes the broker in use, which ends its open Subscribe streams.
func (m *Broker) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cur.Load().b.Close()
}

func (m *Broker) open(dataDir string) (*running, error) {
	path := m.resolve(dataDir)
	b, err := san_message.Open(san_message.Options{Dir: path, Log: m.log})
	if err != nil {
		return nil, fmt.Errorf("open the message broker in %s: %w", path, err)
	}
	_, h := b.Handler()
	return &running{b: b, h: h, path: path, dataDir: dataDir}, nil
}

func (m *Broker) save(dataDir string) error {
	s, err := settings.Load(m.towerDir)
	if err != nil {
		return err
	}
	s.Broker.DataDir = dataDir
	if err := settings.Save(m.towerDir, s); err != nil {
		return errors.Join(errors.New("save the settings"), err)
	}
	return nil
}

func (m *Broker) resolve(dataDir string) string {
	if dataDir == "" {
		dataDir = DefaultDir
	}
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(m.towerDir, dataDir)
	}
	return filepath.Clean(dataDir)
}

// samePath compares two clean absolute paths; Windows paths ignore case.
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
