package livetrading

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// Approach plugins: approaches built as programs of their own (san_backtest
// package plugin), dropped in the plugins folder of the tower's data folder
// (the user, 2026-11-16: "Yes I'm thinking loading like dll plugin"). The
// tower reads each file's list at start and on Reload plugins; a paper
// session of a plugin approach runs the plugin in a process of its own,
// started with the session, and keeps it until the session stops, also
// when the file is rebuilt meanwhile (the user, 2026-11-16: San picked "a
// running paper session keeps its old plugin process until the user
// restarts that session"). A plugin gets only candles, positions and the
// account over its stdin: no Exness client, no connection settings, and
// a clean environment.

// PluginsDir is the plugins folder in the tower's data folder.
const PluginsDir = "plugins"

// PluginTimeout is the longest a plugin may take to answer one step.
var PluginTimeout = bt.DefaultPluginTimeout

// pluginEnv is added to a plugin's clean environment (tests).
var pluginEnv []string

// PluginFile is one file of the plugins folder as the Plugins panel shows
// it.
type PluginFile struct {
	Name       string // the file's name
	Path       string
	ModTime    time.Time
	Approaches []string
	// Error is why the file, or some of its approaches, cannot be used.
	Error string
}

type pluginApproach struct {
	info bt.PluginApproach
}

type plugins struct {
	mu    sync.Mutex
	files []PluginFile
	by    map[string]pluginApproach // approach name -> its plugin
}

// pluginsDir is the plugins folder.
func (m *Manager) pluginsDir() string { return filepath.Join(m.cfg.DataDir, PluginsDir) }

// ReloadPlugins reads the plugins folder again: every *.exe file's
// approaches (each file is started, asked and stopped). Running sessions
// keep their own plugin process.
func (m *Manager) ReloadPlugins() []PluginFile {
	dir := m.pluginsDir()
	paths, _ := filepath.Glob(filepath.Join(dir, "*.exe"))
	sort.Strings(paths)
	taken := map[string]string{}
	for _, n := range m.cfg.Registry.Names() {
		taken[n] = "compiled into the tower"
	}
	var files []PluginFile
	by := map[string]pluginApproach{}
	for _, p := range paths {
		f := PluginFile{Name: filepath.Base(p), Path: p}
		if fi, err := os.Stat(p); err == nil {
			f.ModTime = fi.ModTime()
		}
		list, err := bt.ListPlugin(p, m.pluginOptions(filepath.Base(p)))
		if err != nil {
			f.Error = err.Error()
			files = append(files, f)
			m.log.Warn("live trading: plugin", "file", p, "err", err)
			continue
		}
		var clash []string
		for _, a := range list {
			if who, ok := taken[a.Name]; ok {
				clash = append(clash, fmt.Sprintf("%s (%s)", a.Name, who))
				continue
			}
			taken[a.Name] = "served by " + f.Name
			by[a.Name] = pluginApproach{info: a}
			f.Approaches = append(f.Approaches, a.Name)
		}
		if len(clash) > 0 {
			f.Error = "skipped, the name is taken: " + strings.Join(clash, ", ")
		}
		if len(list) == 0 {
			f.Error = "serves no approach"
		}
		files = append(files, f)
	}
	m.plugins.mu.Lock()
	m.plugins.files, m.plugins.by = files, by
	m.plugins.mu.Unlock()
	// Sessions running a plugin are sent again, so a page shows at once
	// whether their file was rebuilt.
	m.mu.Lock()
	var ids []uint32
	for id, r := range m.runs {
		r.mu.Lock()
		if r.plugin != nil {
			ids = append(ids, id)
		}
		r.mu.Unlock()
	}
	m.mu.Unlock()
	for _, id := range ids {
		if p, err := m.cfg.Store.GetPaperSession(id); err == nil {
			m.publish(SessionEvent{Session: &p})
		}
	}
	return files
}

// Plugins are the plugins folder's files as last read.
func (m *Manager) Plugins() (dir string, files []PluginFile) {
	m.plugins.mu.Lock()
	defer m.plugins.mu.Unlock()
	return m.pluginsDir(), append([]PluginFile{}, m.plugins.files...)
}

func (m *Manager) pluginOf(name string) (bt.PluginApproach, bool) {
	m.plugins.mu.Lock()
	defer m.plugins.mu.Unlock()
	a, ok := m.plugins.by[name]
	return a.info, ok
}

// pluginOptions run a plugin with its log in the tower's (one line per
// line of its stderr) and without the tower's environment.
func (m *Manager) pluginOptions(name string) bt.PluginOptions {
	return bt.PluginOptions{Stderr: &logWriter{log: m.log, plugin: name}, Timeout: PluginTimeout, CleanEnv: true, Env: pluginEnv}
}

// strategy makes approach name: compiled in, or from its plugin in a
// process of its own (Close it to stop the process).
func (m *Manager) strategy(name string) (bt.Strategy, error) {
	if s, err := m.cfg.Registry.New(name); err == nil {
		return s, nil
	} else if _, ok := m.pluginOf(name); !ok {
		return nil, err
	}
	a, _ := m.pluginOf(name)
	return bt.LoadApproach(a.File, name, m.pluginOptions(filepath.Base(a.File)))
}

// PluginRun is the plugin a running session uses: its file, the file's
// modification time when the session started, and whether the file has
// changed since (a rebuild: the session uses it after a restart).
type PluginRun struct {
	File    string
	ModTime time.Time
	Changed bool
}

// PluginRun is session id's plugin while it runs; false for a stopped
// session or a compiled-in approach.
func (m *Manager) PluginRun(id uint32) (PluginRun, bool) {
	m.mu.Lock()
	r := m.runs[id]
	m.mu.Unlock()
	if r == nil {
		return PluginRun{}, false
	}
	r.mu.Lock()
	pr := r.plugin
	r.mu.Unlock()
	if pr == nil {
		return PluginRun{}, false
	}
	out := *pr
	if fi, err := os.Stat(out.File); err != nil || !fi.ModTime().Equal(out.ModTime) {
		out.Changed = true
	}
	return out, true
}

// logWriter turns a plugin's stderr into log lines.
type logWriter struct {
	log    *slog.Logger
	plugin string
	mu     sync.Mutex
	buf    []byte
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		if line := strings.TrimRight(string(w.buf[:i]), "\r"); line != "" {
			w.log.Info("plugin", "plugin", w.plugin, "line", line)
		}
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
}
