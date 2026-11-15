package livetrading

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	exness "github.com/wargasipil/san_exness"

	"github.com/wargasipil/san_tower/internal/livetrading/livefake"
	"github.com/wargasipil/san_tower/internal/store"
)

// market makes M15 candles from start for n candles (a random walk with
// slow trends, Saturdays and Sundays left out) and the H1 and H4 ones
// built from them.
func market(start time.Time, n int) map[string][]livefake.Candle {
	r := rand.New(rand.NewPCG(7, 11))
	out := map[string][]livefake.Candle{}
	p := 2400.0
	for i, t := 0, start; i < n; t = t.Add(15 * time.Minute) {
		if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		o := p
		p += 0.6*math.Sin(float64(i)/400) + r.NormFloat64()*2.5
		out["M15"] = append(out["M15"], livefake.Candle{Time: t, Open: o, High: math.Max(o, p) + r.Float64()*1.5, Low: math.Min(o, p) - r.Float64()*1.5,
			Close: p, Spread: 0.2, Volume: 100})
		i++
	}
	agg := func(d time.Duration) []livefake.Candle {
		var cs []livefake.Candle
		for _, c := range out["M15"] {
			b := c.Time.Truncate(d)
			if k := len(cs); k > 0 && cs[k-1].Time.Equal(b) {
				x := &cs[k-1]
				x.High, x.Low, x.Close, x.Volume = math.Max(x.High, c.High), math.Min(x.Low, c.Low), c.Close, x.Volume+c.Volume
				continue
			}
			cs = append(cs, livefake.Candle{Time: b, Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Spread: c.Spread, Volume: c.Volume})
		}
		return cs
	}
	out["H1"], out["H4"] = agg(time.Hour), agg(4*time.Hour)
	return out
}

type rig struct {
	t     *testing.T
	fake  *livefake.Fake
	st    *store.Store
	dir   string
	clock *Clock
	api   Exness
	mu    sync.Mutex
	calls int
}

// newRig is a fake Exness with a market, a store and a client, on a
// virtual clock at speed (0: real time).
func newRig(t *testing.T, start time.Time, n int, now time.Time, speed float64) *rig {
	t.Helper()
	f, err := livefake.New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	rg := &rig{t: t, fake: f, dir: t.TempDir()}
	if speed > 0 {
		rg.clock = &Clock{start: now, anchor: time.Now(), speed: speed}
		f.Clock = rg.clock.Now
	}
	for tf, cs := range market(start, n) {
		f.SetCandles("XAUUSD", tf, cs)
	}
	rg.st, err = store.Open(rg.dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rg.st.Close() })
	c, err := exness.New(f.Config())
	if err != nil {
		t.Fatal(err)
	}
	rg.api = ReadOnly(c)
	return rg
}

func (rg *rig) manager() *Manager {
	m := New(Config{Store: rg.st, DataDir: rg.dir, Clock: rg.clock, Settle: time.Second, Retry: 20 * time.Second,
		Client: func(context.Context, uint32) (Exness, uint32, string, error) { return rg.api, 1, "fake", nil }})
	rg.t.Cleanup(m.Close)
	return m
}

// waitFor polls cond for up to d.
func waitFor(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("waited %s for %s", d, what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func trades(t *testing.T, st *store.Store, id uint32) []bt.Trade {
	t.Helper()
	recs, err := st.PaperRecords(id, store.PaperTrade, time.Time{}, time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []bt.Trade
	for _, r := range recs {
		var x bt.Trade
		if err := json.Unmarshal([]byte(r.Data), &x); err != nil {
			t.Fatal(err)
		}
		out = append(out, x)
	}
	return out
}

// runOf is San Backtest's Run of the session's settings over [from, to]
// on the fake's candles: what the session must have traded.
func (rg *rig) runOf(name string, from, to time.Time) *bt.Result {
	rg.t.Helper()
	s, err := Approaches().New(name)
	if err != nil {
		rg.t.Fatal(err)
	}
	api := rg.api
	if rg.clock != nil {
		api = clocked{Exness: api, c: rg.clock}
	}
	res, err := bt.Run(context.Background(), bt.Config{Strategy: s, Instrument: "XAUUSD", Data: bt.NewExness(api), From: from, To: to, Balance: 10000,
		Risk: 1, Leverage: 2000, DayTZ: bt.WIB, Cache: "off"})
	if err != nil {
		rg.t.Fatal(err)
	}
	return res
}

// closedTrades leaves out Run's "closed at end" trades (a session keeps
// those positions open).
func closedTrades(ts []bt.Trade) []bt.Trade {
	var out []bt.Trade
	for _, x := range ts {
		if x.Reason != "closed at end" {
			out = append(out, x)
		}
	}
	return out
}

func sameTrades(a, b []bt.Trade) string {
	if x, y := fmt.Sprintf("%+v", a), fmt.Sprintf("%+v", b); x != y {
		return fmt.Sprintf("%d vs %d trades:\n%.600s\n%.600s", len(a), len(b), x, y)
	}
	return ""
}

// The store keeps a session and its journal, and gives them back whole.
func TestPaperStoreRoundTrip(t *testing.T) {
	rg := newRig(t, time.Now(), 0, time.Now(), 0)
	p, err := rg.st.CreatePaperSession(store.PaperSession{Approach: "moving_average", Instrument: "XAUUSD", Balance: 10000, Risk: 1, Leverage: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "moving_average on XAUUSD" || p.Status != store.PaperStopped {
		t.Fatalf("%+v", p)
	}
	t0 := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	recs := []store.PaperRecord{{Kind: store.PaperTrade, Time: t0, Data: `{"Net":1}`}, {Kind: store.PaperTrade, Time: t0.Add(time.Hour), Data: `{"Net":2}`},
		{Kind: store.PaperCandle, Time: t0, Data: `{}`}}
	p.Status, p.Trades = store.PaperWaiting, 2
	if err := rg.st.WritePaperJournal(p, recs, false); err != nil {
		t.Fatal(err)
	}
	got, _ := rg.st.PaperRecords(p.ID, store.PaperTrade, time.Time{}, time.Time{}, 0)
	last, _ := rg.st.PaperRecords(p.ID, store.PaperTrade, time.Time{}, time.Time{}, 1)
	if len(got) != 2 || got[1].Data != `{"Net":2}` || len(last) != 1 || last[0].Data != `{"Net":2}` {
		t.Fatalf("records %+v, newest %+v", got, last)
	}
	if err := rg.st.WritePaperJournal(p, recs[:1], true); err != nil {
		t.Fatal(err)
	}
	if n, _ := rg.st.CountPaperRecords(p.ID, store.PaperTrade); n != 1 {
		t.Fatalf("after replace: %d trades", n)
	}
	if n, _ := rg.st.CountPaperRecords(p.ID, store.PaperCandle); n != 0 {
		t.Fatalf("after replace: %d candles", n)
	}
	back, _ := rg.st.GetPaperSession(p.ID)
	if back.Status != store.PaperWaiting || back.Trades != 2 {
		t.Fatalf("%+v", back)
	}
	// a tower restart leaves it stopped, with the reason
	New(Config{Store: rg.st, DataDir: rg.dir, Client: nil})
	back, _ = rg.st.GetPaperSession(p.ID)
	if back.Status != store.PaperStopped || back.Message != RestartMessage {
		t.Fatalf("after a restart: %s %q", back.Status, back.Message)
	}
	if err := rg.st.DeletePaperSession(p.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := rg.st.CountPaperRecords(p.ID, store.PaperTrade); n != 0 {
		t.Fatal("the journal outlived its session")
	}
}

// A session started again after a restart replays from its first candle:
// its journal is exactly San Backtest's Run over that range, it is
// stopped (not running) until then, and Backtest Analysis's view and the
// export are that run's report.
func TestRestartCatchUpAndView(t *testing.T) {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -60)
	rg := newRig(t, start, 60*96, now, 0)
	m := rg.manager()
	p, err := m.Create(store.PaperSession{Approach: "moving_average", Instrument: "XAUUSD", Balance: 10000, Risk: 1, Leverage: 2000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// it ran before, from 30 days ago, and the tower then stopped
	from := now.AddDate(0, 0, -30).Truncate(15 * time.Minute)
	p.From, p.Status = &from, store.PaperWaiting
	if err := rg.st.SavePaperSession(p); err != nil {
		t.Fatal(err)
	}
	m2 := rg.manager()
	back, _ := m2.Session(p.ID)
	if back.Status != store.PaperStopped || m2.Running(p.ID) {
		t.Fatalf("after the restart the session is %s (running %v), want stopped until started by hand", back.Status, m2.Running(p.ID))
	}
	if err := m2.Start(p.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 60*time.Second, "the catch-up", func() bool {
		s, _ := m2.Session(p.ID)
		return s.Status == store.PaperWaiting || s.Status == store.PaperClosed || s.Status == store.PaperError
	})
	s, _ := m2.Session(p.ID)
	if s.Status == store.PaperError {
		t.Fatal(s.Message)
	}
	got := trades(t, rg.st, p.ID)
	want := rg.runOf("moving_average", from, *s.LastClose)
	if d := sameTrades(closedTrades(want.Trades), got); d != "" {
		t.Fatal(d)
	}
	if len(got) == 0 {
		t.Fatal("no trades in 30 days: the check proves nothing")
	}
	n, _ := rg.st.CountPaperRecords(p.ID, store.PaperCandle)
	if int(n) != len(want.Candles[0].Candles) || s.Candles != uint32(n) {
		t.Fatalf("%d candles journalled, %d counted, Run had %d", n, s.Candles, len(want.Candles[0].Candles))
	}
	// Backtest Analysis's view: a report folder with the same trades
	dir, _, err := m2.View(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"run.json", "trades.csv", "decisions.jsonl", "candles.csv", "equity.csv"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("view: %v", err)
		}
	}
	res, _, err := m2.Result(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.Trades != len(got) || len(res.Decisions) == 0 || !strings.Contains(strings.Join(res.Range.Notes, " "), "PAPER") {
		t.Fatalf("result: %d trades, %d decisions, notes %v", res.Stats.Trades, len(res.Decisions), res.Range.Notes)
	}
	// export: a normal report folder; a second export into it is refused
	out := filepath.Join(t.TempDir(), "export")
	path, files, err := m2.Export(p.ID, out)
	if err != nil || path != out || len(files) < 5 {
		t.Fatalf("export: %v %s %v", err, path, files)
	}
	if _, _, err := m2.Export(p.ID, out); err == nil {
		t.Fatal("an export into a folder that is not empty was not refused")
	}
	m2.Stop(p.ID)
	s, _ = m2.Session(p.ID)
	if s.Status != store.PaperStopped || s.Message != "stopped by hand" || s.Trades != uint32(len(got)) {
		t.Fatalf("after stop: %s %q, %d trades", s.Status, s.Message, s.Trades)
	}
	if r := rg.fake.Refused(); len(r) > 0 {
		t.Fatalf("trading requests reached the fake: %v", r)
	}
}

// Live stepping on a virtual clock (one M15 candle every 1/8 s): the
// session steps each candle as it closes; a network drop makes it wait
// and say why, then it catches the missed candles up in order. Its
// journal stays exactly Run's over the range it ran.
func TestLiveSteppingAndMissedCandles(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	vnow := time.Date(2026, 10, 6, 8, 2, 0, 0, time.UTC) // a Tuesday
	rg := newRig(t, start, 50*96, vnow, 7200)
	m := rg.manager()
	p, err := m.Create(store.PaperSession{Approach: "moving_average", Instrument: "XAUUSD", Balance: 10000, Risk: 1, Leverage: 2000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch, leave := m.Watch(p.ID)
	defer leave()
	var mu sync.Mutex
	stepped := 0
	go func() {
		for ev := range ch {
			mu.Lock()
			stepped += len(ev.Candles)
			mu.Unlock()
		}
	}()
	if err := m.Start(p.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 60*time.Second, "150 candles stepped", func() bool { s, _ := m.Session(p.ID); return s.Candles >= 150 })
	rg.fake.SetDown(true)
	waitFor(t, 30*time.Second, "the session to say Exness is down", func() bool {
		s, _ := m.Session(p.ID)
		return strings.Contains(s.Message, "trying again")
	})
	down, _ := m.Session(p.ID)
	time.Sleep(2 * time.Second) // ~16 candles missed
	rg.fake.SetDown(false)
	waitFor(t, 30*time.Second, "the catch-up", func() bool { s, _ := m.Session(p.ID); return s.Candles >= down.Candles+24 })
	m.Stop(p.ID)
	s, _ := m.Session(p.ID)
	want := rg.runOf("moving_average", *s.From, *s.LastClose)
	got := trades(t, rg.st, p.ID)
	if d := sameTrades(closedTrades(want.Trades), got); d != "" {
		t.Fatal(d)
	}
	recs, _ := rg.st.PaperRecords(p.ID, store.PaperCandle, time.Time{}, time.Time{}, 0)
	for i := 1; i < len(recs); i++ {
		if !recs[i].Time.After(recs[i-1].Time) {
			t.Fatalf("candle %d out of order", i)
		}
	}
	if len(recs) != len(want.Candles[0].Candles) {
		t.Fatalf("%d candles journalled, Run had %d: a missed candle was skipped", len(recs), len(want.Candles[0].Candles))
	}
	mu.Lock()
	defer mu.Unlock()
	if stepped < len(recs)-1 {
		t.Fatalf("the watchers saw %d candles of %d", stepped, len(recs))
	}
	if len(got) == 0 {
		t.Fatal("no trades: the check proves nothing")
	}
	t.Logf("%d candles, %d trades, down at %d candles", len(recs), len(got), down.Candles)
}

// The monitor shows the account, positions with their stop and target and
// live P/L, and pending orders, from the events stream; a change comes as
// a transaction; after a drop it dials again and rebuilds from the new
// snapshot. It never sends a trading request.
func TestMonitor(t *testing.T) {
	MonitorIdle, monitorBackoff, restEvery = 50*time.Millisecond, 200*time.Millisecond, 50*time.Millisecond
	now := time.Now().UTC()
	rg := newRig(t, now.AddDate(0, 0, -5), 5*96+200, now, 0)
	rg.fake.AccountEvery, rg.fake.TickEvery = 100*time.Millisecond, 50*time.Millisecond
	id := rg.fake.AddPosition("XAUUSD", exness.Buy, 0.1, 2300, 2290, 2500)
	rg.fake.AddOrder("XAUUSD", exness.Sell, "limit", 0.05, 2600, 2650, 2500)
	m := rg.manager()
	mo, err := m.Monitor(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	ch, leave := mo.Watch([]string{"XAUUSD"})
	defer leave()
	var last MonitorState
	ticks := 0
	next := func(what string, cond func(MonitorState) bool) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for {
			select {
			case ev := <-ch:
				if ev.Tick != nil {
					ticks++
				}
				if ev.State != nil {
					last = *ev.State
					if cond(last) {
						return
					}
				}
			case <-deadline:
				t.Fatalf("waited for %s; last state %+v", what, last)
			}
		}
	}
	next("the snapshot with a priced position", func(s MonitorState) bool {
		return s.Status == MonitorConnected && len(s.Positions) == 1 && s.Positions[0].Price > 0 && len(s.Orders) == 1
	})
	ps := last.Positions[0]
	if ps.ID != id || ps.StopLoss != 2290 || ps.TakeProfit != 2500 || ps.Currency != "USD" ||
		math.Abs(ps.Profit-(ps.Price-2300)*0.1*100) > 1e-6 {
		t.Fatalf("position %+v", ps)
	}
	if last.Account.Balance != 10000 || last.Account.Equity == 0 || last.Account.Currency != "USD" || last.Source != "stream" {
		t.Fatalf("account %+v source %s", last.Account, last.Source)
	}
	if o := last.Orders[0]; o.Type != "limit" || o.Price != 2600 || o.Side != "sell" {
		t.Fatalf("order %+v", o)
	}
	rg.fake.AddPosition("XAUUSD", exness.Sell, 0.2, 2410, 0, 0)
	next("the new position", func(s MonitorState) bool { return len(s.Positions) == 2 })
	rg.fake.RemovePosition(id)
	next("the closed position gone", func(s MonitorState) bool { return len(s.Positions) == 1 && s.Positions[0].ID != id })
	rg.fake.DropStreams()
	next("the reconnect", func(s MonitorState) bool { return s.Status == MonitorReconnecting })
	next("connected again", func(s MonitorState) bool { return s.Status == MonitorConnected && len(s.Positions) == 1 })
	if ticks == 0 {
		t.Fatal("no ticks")
	}
	if r := rg.fake.Refused(); len(r) > 0 {
		t.Fatalf("trading requests: %v", r)
	}
	for _, c := range rg.fake.Calls() {
		if strings.HasPrefix(c, "GET /v1/trading/") && !strings.HasSuffix(c, "/snapshot") || strings.Contains(c, "/trading/") && !strings.HasPrefix(c, "GET ") {
			t.Fatalf("a trading route was called: %s", c)
		}
	}
}
