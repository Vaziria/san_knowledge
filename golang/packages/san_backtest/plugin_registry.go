package san_backtest

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// The Registry's second mode: approaches from plugin files next to the
// ones registered from code. Both come out of New the same way.
//
//	r := bt.NewRegistry()
//	examples.Register(r)                   // from code
//	err := r.Load("plugins/my_scalp.exe")  // from a plugin file
//	s, err := r.New("my_scalp")
//
// One process serves every approach of a loaded file and hosts each New
// as its own instance (a fresh maker() call in the plugin), so a run's
// walk-forward trials do not start a process each.

type loadedPlugin struct {
	proc  *pluginProc
	names []string
}

// SetPluginOptions sets how Load runs plugin processes (stderr, timeout).
func (r *Registry) SetPluginOptions(o PluginOptions) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.opts = o
}

// Load starts the plugin file and registers every approach it serves. A
// name the registry already has fails the load, and nothing of the file is
// registered then.
func (r *Registry) Load(file string) error {
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	r.mu.Lock()
	o := r.opts
	_, loaded := r.plugins[abs]
	r.mu.Unlock()
	if loaded {
		return fmt.Errorf("san_backtest: plugin %s is already loaded (Reload replaces it)", file)
	}
	p, err := startPlugin(abs, o)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var taken []string
	for _, a := range p.approaches {
		if _, ok := r.m[a.Name]; ok {
			taken = append(taken, a.Name)
		}
	}
	if len(taken) > 0 || len(p.approaches) == 0 {
		go p.stop()
		if len(taken) == 0 {
			return fmt.Errorf("san_backtest: plugin %s serves no approach", filepath.Base(abs))
		}
		return fmt.Errorf("san_backtest: plugin %s serves %s, already registered", filepath.Base(abs), strings.Join(taken, ", "))
	}
	lp := &loadedPlugin{proc: p}
	for _, a := range p.approaches {
		a := a
		lp.names = append(lp.names, a.Name)
		r.m[a.Name] = func() Strategy {
			s, _ := newRemote(p, a, false) // a failure fails its first step; New returns it
			return s
		}
	}
	r.plugins[abs] = lp
	return nil
}

// Unload stops the plugin file's process and removes its approaches.
// Strategies made from it fail from their next step on.
func (r *Registry) Unload(file string) error {
	abs, err := filepath.Abs(file)
	if err != nil {
		return err
	}
	r.mu.Lock()
	lp, ok := r.plugins[abs]
	if ok {
		delete(r.plugins, abs)
		for _, n := range lp.names {
			delete(r.m, n)
		}
	}
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("san_backtest: plugin %s is not loaded", file)
	}
	lp.proc.stop()
	return nil
}

// Reload unloads the file (when loaded) and loads it again, e.g. after a
// rebuild.
func (r *Registry) Reload(file string) error {
	if err := r.Unload(file); err != nil && !r.isNotLoaded(file) {
		return err
	}
	return r.Load(file)
}

func (r *Registry) isNotLoaded(file string) bool {
	abs, _ := filepath.Abs(file)
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.plugins[abs]
	return !ok
}

// Plugins are the loaded plugin files (absolute paths), sorted.
func (r *Registry) Plugins() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for f := range r.plugins {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// PluginOf is the plugin file serving the approach name, "" for one
// registered from code.
func (r *Registry) PluginOf(name string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for f, lp := range r.plugins {
		for _, n := range lp.names {
			if n == name {
				return f
			}
		}
	}
	return ""
}

// Close stops every loaded plugin process.
func (r *Registry) Close() error {
	for _, f := range r.Plugins() {
		r.Unload(f)
	}
	return nil
}

// ListPlugin starts the plugin file, reads what it serves and stops it.
func ListPlugin(file string, o ...PluginOptions) ([]PluginApproach, error) {
	p, err := startPlugin(file, optsOf(o))
	if err != nil {
		return nil, err
	}
	defer p.stop()
	return p.approaches, nil
}

// LoadApproach starts the plugin file in a process of its own and makes
// its approach name. Close the strategy (it is an io.Closer) to stop the
// process; San Tower gives each paper session its own this way, so one
// crash cannot stop another session.
func LoadApproach(file, name string, o ...PluginOptions) (Strategy, error) {
	p, err := startPlugin(file, optsOf(o))
	if err != nil {
		return nil, err
	}
	for _, a := range p.approaches {
		if a.Name == name {
			s, err := newRemote(p, a, true)
			if err != nil {
				p.stop()
				return nil, err
			}
			return s, nil
		}
	}
	p.stop()
	var have []string
	for _, a := range p.approaches {
		have = append(have, a.Name)
	}
	return nil, fmt.Errorf("san_backtest: plugin %s serves no approach %q (have: %s)", filepath.Base(file), name, strings.Join(have, ", "))
}

func optsOf(o []PluginOptions) PluginOptions {
	if len(o) > 0 {
		return o[0]
	}
	return PluginOptions{}
}

// PluginFileOf is the plugin file a strategy comes from, "" for one
// written in code.
func PluginFileOf(s Strategy) string {
	switch x := s.(type) {
	case *remote:
		return x.info.File
	case remoteTunable:
		return x.info.File
	}
	return ""
}

// PluginInfoOf is what a plugin strategy's process said about it when it
// started (its file, the file's ModTime then, ...); false for a strategy
// written in code.
func PluginInfoOf(s Strategy) (PluginApproach, bool) {
	switch x := s.(type) {
	case *remote:
		return x.info, true
	case remoteTunable:
		return x.info, true
	}
	return PluginApproach{}, false
}

// pluginErr is a plugin strategy's failure so far (its NewInstance).
func pluginErr(s Strategy) error {
	switch x := s.(type) {
	case *remote:
		return x.err
	case remoteTunable:
		return x.err
	}
	return nil
}

// CloseStrategy closes s when it holds a plugin instance or process (an
// io.Closer); other strategies are left alone.
func CloseStrategy(s Strategy) error {
	if c, ok := s.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
