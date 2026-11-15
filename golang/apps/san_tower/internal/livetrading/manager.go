package livetrading

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"sync"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
	research "github.com/wargasipil/san_research"

	"github.com/wargasipil/san_tower/internal/store"
)

// Dir is the Live Trading folder in the tower's data folder: the candle
// cache (cache/), the report folders Backtest Analysis reads a session from
// (view/<id>, made again when the session changed) and the default place
// of exports (exports/).
const Dir = "live_trading"

// ClientFunc gives the read-only client of an Exness connection (0: the
// first one) and the connection's id and name.
type ClientFunc func(ctx context.Context, connectionID uint32) (Exness, uint32, string, error)

// Config is what a Manager needs.
type Config struct {
	Store   *store.Store
	DataDir string // the tower's data folder
	Client  ClientFunc
	Log     *slog.Logger
	// Registry is the approaches (default: Approaches()).
	Registry *bt.Registry
	// Settle is how long after a candle's close Exness is asked for it
	// (default 5 s).
	Settle time.Duration
	// Retry is the wait after a failed candle read (default 30 s).
	Retry time.Duration
	// Clock is a virtual clock for replay checks (ClockEnv); nil: real
	// time.
	Clock *Clock
}

// Approaches is every approach a paper session may run: san_backtest's
// examples (and their chains) and san_research's approaches (the user,
// 2026-11-15: "2a": the live approach runs inside the tower using
// san_backtest's chain code).
func Approaches() *bt.Registry {
	r := bt.NewRegistry()
	examples.Register(r)
	research.RegisterAll(r)
	return r
}

// Manager runs the monitors and the paper sessions.
type Manager struct {
	cfg Config
	log *slog.Logger

	mu       sync.Mutex
	monitors map[uint32]*Monitor
	runs     map[uint32]*runner
	subs     map[int]*sessionSub
	nextSub  int
	closed   bool
	views    sync.Mutex
}

// New makes the Manager. Every session that was going when the tower
// stopped is marked stopped: it waits to be started by hand (the user,
// 2026-11-15: "3b": after a tower restart, sessions stay stopped until
// restarted by hand).
func New(cfg Config) *Manager {
	if cfg.Registry == nil {
		cfg.Registry = Approaches()
	}
	if cfg.Settle <= 0 {
		cfg.Settle = 5 * time.Second
	}
	if cfg.Retry <= 0 {
		cfg.Retry = 30 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	m := &Manager{cfg: cfg, log: cfg.Log, monitors: map[uint32]*Monitor{}, runs: map[uint32]*runner{}, subs: map[int]*sessionSub{}}
	if n, err := cfg.Store.StopPaperSessions(RestartMessage); err != nil {
		m.log.Error("live trading: marking the paper sessions stopped", "err", err)
	} else if n > 0 {
		m.log.Info("live trading: paper sessions stopped by the restart wait to be started by hand", "sessions", n)
	}
	return m
}

// RestartMessage is a session's message after a tower restart.
const RestartMessage = "the tower restarted: start the session again to continue it"

// Close stops every session (recorded as stopped, so they wait for a
// start by hand) and every monitor.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	runs := make([]*runner, 0, len(m.runs))
	for _, r := range m.runs {
		runs = append(runs, r)
	}
	mons := make([]*Monitor, 0, len(m.monitors))
	for _, mo := range m.monitors {
		mons = append(mons, mo)
	}
	m.monitors = map[uint32]*Monitor{}
	subs := m.subs
	m.subs = map[int]*sessionSub{}
	m.mu.Unlock()
	for _, r := range runs {
		r.stop(RestartMessage)
	}
	for _, mo := range mons {
		mo.stop()
	}
	for _, s := range subs {
		close(s.ch)
	}
}

func (m *Manager) dir(parts ...string) string {
	return filepath.Join(append([]string{m.cfg.DataDir, Dir}, parts...)...)
}

// Approach is one approach as the New session dialog shows it.
type Approach struct {
	Name       string
	Timeframes []string
	Warmup     int
	Params     map[string]float64
	MaxOpen    int // the approach's own; 0 = one at a time
}

// Approaches lists the registry's approaches.
func (m *Manager) Approaches() []Approach {
	var out []Approach
	for _, n := range m.cfg.Registry.Names() {
		s, err := m.cfg.Registry.New(n)
		if err != nil {
			continue
		}
		a := Approach{Name: n, Timeframes: s.Timeframes(), Warmup: s.Warmup(), Params: map[string]float64{}}
		if t, ok := s.(bt.Tunable); ok {
			a.Params = t.Params()
		}
		if mp, ok := s.(bt.MultiPosition); ok {
			a.MaxOpen = mp.MaxOpen()
		}
		out = append(out, a)
	}
	return out
}

// Monitor gives the monitor of an Exness connection (0: the first),
// starting it when no page watched it.
func (m *Manager) Monitor(ctx context.Context, connectionID uint32) (*Monitor, error) {
	api, id, name, err := m.client(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, errors.New("the tower is stopping")
	}
	if mo := m.monitors[id]; mo != nil && mo.api == api {
		return mo, nil
	} else if mo != nil {
		go mo.stop() // the connection's settings changed
	}
	mo := newMonitor(id, name, api)
	mo.onStopped = func() {
		m.mu.Lock()
		if m.monitors[id] == mo && len(mo.watchers) == 0 {
			delete(m.monitors, id)
			m.mu.Unlock()
			mo.stop()
			return
		}
		m.mu.Unlock()
	}
	m.monitors[id] = mo
	mo.start()
	return mo, nil
}

// client is the connection's client, on the manager's clock.
func (m *Manager) client(ctx context.Context, connectionID uint32) (Exness, uint32, string, error) {
	api, id, name, err := m.cfg.Client(ctx, connectionID)
	if err == nil && m.cfg.Clock != nil {
		api = clocked{Exness: api, c: m.cfg.Clock}
	}
	return api, id, name, err
}

// SessionEvent is a change of a paper session: its row (Session) with
// what the last step added, or Removed.
type SessionEvent struct {
	Session   *store.PaperSession
	Trades    []bt.Trade
	Decisions []bt.Decision
	Equity    []bt.EquityPoint
	Candles   []bt.Candle
	Missed    []bt.Missed
	Removed   uint32
	// Reset: the journal was written again whole (a start's replay):
	// watchers read it again rather than adding the lists above.
	Reset bool
}

type sessionSub struct {
	id uint32 // 0: every session
	ch chan SessionEvent
}

// Watch gives the changes of session id (0: of every session) until the
// returned func is called. A watcher too slow to read is dropped (its
// channel closes): it reads the session again.
func (m *Manager) Watch(id uint32) (<-chan SessionEvent, func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.nextSub
	m.nextSub++
	s := &sessionSub{id: id, ch: make(chan SessionEvent, 128)}
	if m.closed {
		close(s.ch)
		return s.ch, func() {}
	}
	m.subs[n] = s
	var once sync.Once
	return s.ch, func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			if _, ok := m.subs[n]; ok {
				delete(m.subs, n)
				close(s.ch)
			}
		})
	}
}

func (m *Manager) publish(ev SessionEvent) {
	id := ev.Removed
	if ev.Session != nil {
		id = ev.Session.ID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for n, s := range m.subs {
		if s.id != 0 && s.id != id {
			continue
		}
		select {
		case s.ch <- ev:
		default:
			delete(m.subs, n)
			close(s.ch)
		}
	}
}

// Sessions lists the sessions, newest first.
func (m *Manager) Sessions() ([]store.PaperSession, error) { return m.cfg.Store.ListPaperSessions() }

// Session reads one.
func (m *Manager) Session(id uint32) (store.PaperSession, error) {
	return m.cfg.Store.GetPaperSession(id)
}

// Create saves a new session (stopped) after checking its approach, its
// params and its instrument's name.
func (m *Manager) Create(p store.PaperSession, params map[string]float64) (store.PaperSession, error) {
	s, err := m.cfg.Registry.New(p.Approach)
	if err != nil {
		return p, fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	defs := map[string]float64{}
	if t, ok := s.(bt.Tunable); ok {
		defs = t.Params()
	}
	keep := map[string]float64{}
	var unknown []string
	for k, v := range params {
		d, ok := defs[k]
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		if v != d {
			keep[k] = v
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return p, fmt.Errorf("%w: %s has no param %v", store.ErrInvalid, p.Approach, unknown)
	}
	p.Params = mustJSON(keep)
	p.Timeframes = joinTF(s.Timeframes())
	p, err = m.cfg.Store.CreatePaperSession(p)
	if err == nil {
		m.publish(SessionEvent{Session: &p})
	}
	return p, err
}

// Delete stops a session and deletes it with its journal (its view folder
// too; exports are left).
func (m *Manager) Delete(id uint32) error {
	m.Stop(id)
	if err := m.cfg.Store.DeletePaperSession(id); err != nil {
		return err
	}
	m.removeView(id)
	m.publish(SessionEvent{Removed: id})
	return nil
}

// Running says whether a session's runner is going.
func (m *Manager) Running(id uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[id] != nil
}

// Start starts a stopped session: a new one from the step candle forming
// now, a stopped one from its first candle again (a replay up to now,
// which continues it as if it had never stopped).
func (m *Manager) Start(id uint32) error {
	p, err := m.cfg.Store.GetPaperSession(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("the tower is stopping")
	}
	if m.runs[id] != nil {
		return fmt.Errorf("%w: paper session %d is running already", store.ErrExists, id)
	}
	r := newRunner(m, p)
	m.runs[id] = r
	go r.run()
	return nil
}

// Stop stops a session (it keeps its journal). Stopping a stopped one does
// nothing.
func (m *Manager) Stop(id uint32) {
	m.mu.Lock()
	r := m.runs[id]
	m.mu.Unlock()
	if r != nil {
		r.stop("stopped by hand")
	}
}

func (m *Manager) ended(r *runner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs[r.id] == r {
		delete(m.runs, r.id)
	}
}
