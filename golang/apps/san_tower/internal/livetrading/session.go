package livetrading

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bt "github.com/wargasipil/san_backtest"

	"github.com/wargasipil/san_tower/internal/store"
)

// A runner runs one paper session: it loads the warm-up history (the
// tower's candle cache, then Exness's REST Candles, read-only), catches up
// from the session's first candle to now, then wakes at each close of the
// step timeframe (plus Settle), asks Exness for the candles that closed
// and steps the engine. A missed close (the PC asleep, the network down)
// is caught up at the next wake: Advance asks from the last candle it has.
// With the market closed it only waits, asking nothing.
type runner struct {
	m      *Manager
	id     uint32
	p      store.PaperSession
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu  sync.Mutex
	why string // the stop's message
	// plugin is the session's plugin while it runs (nil: compiled in).
	plugin *PluginRun
}

func newRunner(m *Manager, p store.PaperSession) *runner {
	ctx, cancel := context.WithCancel(context.Background())
	return &runner{m: m, id: p.ID, p: p, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

func (r *runner) stop(why string) {
	r.mu.Lock()
	if r.why == "" {
		r.why = why
	}
	r.mu.Unlock()
	r.cancel()
	<-r.done
}

// costsJSON is a session's cost overrides, as stored.
type costsJSON struct {
	Spread     *float64    `json:"spread,omitempty"`
	Commission *float64    `json:"commission,omitempty"`
	Swap       *[2]float64 `json:"swap,omitempty"`
	Slippage   float64     `json:"slippage,omitempty"`
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func joinTF(tfs []string) string { return strings.Join(tfs, ",") }

// Config is the session's san_backtest config.
func (r *runner) config(api Exness, s bt.Strategy, from time.Time) (bt.LiveConfig, error) {
	p := r.p
	params := map[string]float64{}
	if err := json.Unmarshal([]byte(p.Params), &params); err != nil {
		return bt.LiveConfig{}, fmt.Errorf("params: %w", err)
	}
	var c costsJSON
	if err := json.Unmarshal([]byte(p.Costs), &c); err != nil {
		return bt.LiveConfig{}, fmt.Errorf("costs: %w", err)
	}
	cfg := bt.Config{Strategy: s, Instrument: p.Instrument, Data: bt.NewExness(api), From: from, Balance: p.Balance, Risk: p.Risk,
		Leverage: int(p.Leverage), MaxOpen: int(p.MaxOpen), MaxOpenRisk: p.MaxOpenRisk, Params: params, DayTZ: bt.WIB,
		Costs: bt.Costs{Spread: c.Spread, Commission: c.Commission, Swap: c.Swap, Slippage: c.Slippage}, Cache: r.m.dir("cache")}
	return bt.LiveConfig{Config: cfg, Now: api.Now}, nil
}

func (r *runner) set(status, message string) {
	r.p.Status, r.p.Message = status, message
	if err := r.m.cfg.Store.SetPaperStatus(r.id, status, message); err != nil {
		r.m.log.Warn("paper session status", "id", r.id, "err", err)
	}
	p := r.p
	r.m.publish(SessionEvent{Session: &p})
}

func (r *runner) run() {
	defer close(r.done)
	defer r.m.ended(r)
	err := r.loop()
	r.mu.Lock()
	why := r.why
	r.mu.Unlock()
	now := time.Now()
	r.p.StoppedAt = &now
	switch {
	case why != "":
		r.p.Status, r.p.Message = store.PaperStopped, why
	case err != nil:
		r.p.Status, r.p.Message = store.PaperError, err.Error()
	default:
		r.p.Status, r.p.Message = store.PaperStopped, "ended"
	}
	if p, gerr := r.m.cfg.Store.GetPaperSession(r.id); gerr == nil {
		// keep the numbers last written; only the status moves
		p.Status, p.Message, p.StoppedAt = r.p.Status, r.p.Message, r.p.StoppedAt
		r.p = p
		_ = r.m.cfg.Store.SavePaperSession(p)
	}
	p := r.p
	r.m.publish(SessionEvent{Session: &p})
}

func (r *runner) loop() error {
	ctx := r.ctx
	now := time.Now()
	r.p.StartedAt, r.p.StoppedAt = &now, nil
	r.set(store.PaperStarting, "connecting to Exness")
	api, _, name, err := r.m.client(ctx, r.p.ConnectionID)
	if err != nil {
		return err
	}
	s, err := r.m.strategy(r.p.Approach)
	if err != nil {
		return err
	}
	// A plugin approach's process lives as long as this run of the session.
	defer bt.CloseStrategy(s)
	if a, ok := bt.PluginInfoOf(s); ok {
		pr := &PluginRun{File: a.File, ModTime: a.ModTime}
		r.mu.Lock()
		r.plugin = pr
		r.mu.Unlock()
	}
	tfs := s.Timeframes()
	step := tfs[0]
	from := time.Time{}
	if r.p.From != nil {
		from = *r.p.From
	} else {
		// a new session starts with the step candle forming now
		d := bt.TimeframeDuration(step)
		from = api.Now().UTC().Truncate(d)
		if step == "D1" || step == "W1" || step == "MN" {
			from = api.Now().UTC().Truncate(24 * time.Hour)
		}
		r.p.From = &from
	}
	cfg, err := r.config(api, s, from)
	if err != nil {
		return err
	}
	var lv *bt.Live
	for try := 1; ; try++ {
		msg := "loading the warm-up history and catching up from " + from.In(bt.WIB).Format("2006-01-02 15:04 WIB") + " on " + name
		r.set(store.PaperStarting, msg)
		lv, err = bt.NewLive(ctx, cfg)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return nil
		}
		if try == 3 {
			return fmt.Errorf("could not start: %w", err)
		}
		r.set(store.PaperStarting, fmt.Sprintf("%s: %v; trying again in %s", msg, err, r.m.cfg.Retry))
		r.m.sleep(ctx, r.m.cfg.Retry)
	}
	r.p.Timeframes = joinTF(lv.Timeframes())
	// A start writes the whole journal again from the replay: the same
	// engine on the same candles gives the same history up to the stop,
	// and the time it was stopped is caught up.
	if err := r.write(lv, nil, true); err != nil {
		return err
	}
	inst := lv.Instrument()
	d := bt.TimeframeDuration(step)
	for {
		st := lv.State()
		next := st.NextClose
		now := api.Now().UTC()
		if next.IsZero() || next.Before(now) {
			next = now.Truncate(d).Add(d)
		}
		if !inst.Open(now) && !inst.Open(next) {
			r.set(store.PaperClosed, "the market is closed; the session waits for it to open")
		} else {
			r.set(store.PaperWaiting, "waiting for the "+step+" candle closing at "+next.In(bt.WIB).Format("15:04 WIB"))
		}
		r.m.sleep(ctx, next.Add(r.m.cfg.Settle).Sub(api.Now()))
		if ctx.Err() != nil {
			return nil
		}
		now = api.Now().UTC()
		if !inst.Open(now) && !inst.Open(now.Add(-d)) {
			continue // closed for this whole candle: nothing to ask for
		}
		r.set(store.PaperRunning, "reading the candles that closed")
		up, err := lv.Advance(ctx)
		if err == bt.ErrLiveDone {
			return nil
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if strings.Contains(err.Error(), "strategy panicked") {
				return err
			}
			r.set(store.PaperWaiting, fmt.Sprintf("Exness: %v; trying again in %s", err, r.m.cfg.Retry))
			r.m.sleep(ctx, r.m.cfg.Retry)
			continue
		}
		if err := r.write(lv, &up, false); err != nil {
			return err
		}
	}
}

// write saves the session's numbers and the journal lines of up (nil: of
// the whole run, replacing the journal) and tells the watchers.
func (r *runner) write(lv *bt.Live, up *bt.Update, replace bool) error {
	res := lv.Result()
	st := lv.State()
	if up == nil {
		up = &bt.Update{}
		if res != nil {
			up.Trades, up.Decisions, up.Missed, up.Equity = res.Trades, res.Decisions, res.Missed, res.Equity
			for _, cs := range res.Candles {
				up.Candles = append(up.Candles, cs.Candles...)
			}
		}
	}
	p := r.p
	p.NowBalance, p.Equity, p.Candles = st.Balance, st.Equity, uint32(st.Candles)
	p.OpenPositions, p.PendingOrders = uint32(len(st.Positions)), uint32(len(st.Orders))
	p.LastClose, p.NextClose = timePtr(st.LastClose), timePtr(st.NextClose)
	p.State = mustJSON(stateJSON{Positions: st.Positions, Orders: st.Orders})
	if res != nil {
		p.Trades, p.NetProfit = uint32(len(res.Trades)), res.Stats.NetProfit
		p.Info = mustJSON(infoOf(res))
	} else {
		p.Trades, p.NetProfit = 0, 0
	}
	for i := len(up.Decisions) - 1; i >= 0; i-- {
		if d := up.Decisions[i]; d.Kind == bt.DecisionSignal {
			t := d.Time
			p.LastSignal, p.LastSignalAt = signalText(d), &t
			break
		}
	}
	recs := records(up)
	if err := r.m.cfg.Store.WritePaperJournal(p, recs, replace); err != nil {
		return err
	}
	fresh, err := r.m.cfg.Store.GetPaperSession(p.ID)
	if err == nil {
		p = fresh
	}
	r.p = p
	if replace {
		r.m.publish(SessionEvent{Session: &p, Reset: true})
		return nil
	}
	r.m.publish(SessionEvent{Session: &p, Trades: up.Trades, Decisions: up.Decisions, Equity: up.Equity, Candles: up.Candles, Missed: up.Missed})
	return nil
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// signalText is a signal decision as the list shows it: "buy: <reason>".
func signalText(d bt.Decision) string {
	what := bt.Action(int(d.Details["action"])).String()
	if d.Reason != "" {
		what += ": " + d.Reason
	}
	if len(what) > 300 {
		what = what[:300] + "..."
	}
	return what
}

// stateJSON is a session's open positions and orders, as stored.
type stateJSON struct {
	Positions []bt.Position `json:"positions"`
	Orders    []bt.Order    `json:"orders"`
}

// infoJSON is what a Result needs besides its journal, as stored.
type infoJSON struct {
	Name   string               `json:"name"`
	Range  bt.Range             `json:"range"`
	Costs  []bt.CostReport      `json:"costs"`
	Run    bt.RunInfo           `json:"run"`
	Params []map[string]float64 `json:"params"`
}

func infoOf(res *bt.Result) infoJSON {
	return infoJSON{Name: res.Name, Range: res.Range, Costs: res.Costs, Run: res.Run, Params: res.Params}
}

// finite drops a decision's non-finite numbers (NaN levels of a refused
// signal), which JSON cannot hold, as decisions.jsonl does.
func finite(d bt.Decision) bt.Decision {
	det := map[string]float64{}
	for k, v := range d.Details {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			det[k] = v
		}
	}
	d.Details = det
	return d
}

func cleanTrade(t bt.Trade) bt.Trade {
	for _, f := range []*float64{&t.Entry, &t.Exit, &t.Stop, &t.FinalStop, &t.Target, &t.Lots, &t.Spread, &t.Risk, &t.Gross, &t.Commission, &t.Swap,
		&t.Net, &t.R} {
		if math.IsNaN(*f) || math.IsInf(*f, 0) {
			*f = 0
		}
	}
	return t
}

func records(up *bt.Update) []store.PaperRecord {
	var out []store.PaperRecord
	add := func(kind string, t time.Time, v any) {
		out = append(out, store.PaperRecord{Kind: kind, Time: t.UTC(), Data: mustJSON(v)})
	}
	for _, c := range up.Candles {
		add(store.PaperCandle, c.OpenTime, c)
	}
	for _, d := range up.Decisions {
		add(store.PaperDecision, d.Time, finite(d))
	}
	for _, t := range up.Trades {
		add(store.PaperTrade, t.CloseTime, cleanTrade(t))
	}
	for _, x := range up.Missed {
		add(store.PaperMissed, x.Time, x)
	}
	for _, e := range up.Equity {
		add(store.PaperEquity, e.Time, e)
	}
	return out
}

// viewDir is session id's report folder for Backtest Analysis.
func (m *Manager) viewDir(id uint32) string { return m.dir("view", fmt.Sprint(id)) }

func (m *Manager) removeView(id uint32) {
	m.views.Lock()
	defer m.views.Unlock()
	_ = removeAll(m.viewDir(id))
}

// exportDir is the default export folder of a session.
func (m *Manager) exportDir(p store.PaperSession) string {
	name := strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '-'
	}, p.Approach+"-"+p.Instrument)
	return filepath.Join(m.dir("exports"), fmt.Sprintf("%d-%s-%s", p.ID, name, time.Now().UTC().Format("20060102-150405")))
}
