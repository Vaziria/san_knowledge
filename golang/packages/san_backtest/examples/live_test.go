package examples

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sort"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The live API (bt.NewLive, Advance, Finish) gives exactly Run's results:
// the same trades, missed signals, equity curve, decision log, candles and
// stats, for every example and its chain, whether the candles come one at
// a time, with gaps (a computer asleep), or all at once (a catch-up).

// clocked is a source that only has the candles closed by now, like
// Exness at that moment. It reads the CSV files once.
type clocked struct {
	src bt.DataSource
	now *time.Time
	all map[string][]bt.Candle
}

func newClocked(dir string, now *time.Time) *clocked {
	return &clocked{src: &bt.CSV{Dir: dir}, now: now, all: map[string][]bt.Candle{}}
}

func (c *clocked) Instrument(ctx context.Context, name string) (bt.Instrument, error) {
	return c.src.Instrument(ctx, name)
}

func (c *clocked) Candles(ctx context.Context, q bt.Query) ([]bt.Candle, error) {
	all, ok := c.all[q.Timeframe]
	if !ok {
		var err error
		all, err = c.src.Candles(ctx, bt.Query{Instrument: q.Instrument, Timeframe: q.Timeframe, From: time.Unix(0, 0), To: time.Now()})
		if err != nil {
			return nil, err
		}
		c.all[q.Timeframe] = all
	}
	i := sort.Search(len(all), func(k int) bool { return !all[k].OpenTime.Before(q.From) })
	var out []bt.Candle
	for _, k := range all[i:] {
		if !k.OpenTime.Before(q.To) || k.CloseTime.After(*c.now) {
			break
		}
		out = append(out, k)
	}
	return out, nil
}

// sameRun is "" when a and b match in every recorded part, else the first
// part that differs. %v prints floats in their shortest exact form, so
// equal text is equal numbers (NaN included).
func sameRun(a, b *bt.Result) string {
	parts := []struct {
		name string
		x, y any
	}{
		{"trades", a.Trades, b.Trades}, {"missed", a.Missed, b.Missed}, {"equity", a.Equity, b.Equity},
		{"decisions", a.Decisions, b.Decisions}, {"candles", a.Candles, b.Candles}, {"stats", a.Stats, b.Stats},
		{"legs", a.Legs, b.Legs}, {"range", [2]time.Time{a.Range.From, a.Range.To}, [2]time.Time{b.Range.From, b.Range.To}},
		{"params", a.Params, b.Params}, {"costs", a.Costs, b.Costs},
	}
	for _, p := range parts {
		if x, y := fmt.Sprintf("%+v", p.x), fmt.Sprintf("%+v", p.y); x != y {
			n := min(len(x), len(y))
			i := 0
			for i < n && x[i] == y[i] {
				i++
			}
			lo := max(0, i-200)
			return fmt.Sprintf("%s differ at %d:\n  run:  ...%s\n  live: ...%s", p.name, i, x[lo:min(len(x), i+200)], y[lo:min(len(y), i+200)])
		}
	}
	return ""
}

func TestLiveGivesRunsResults(t *testing.T) {
	dir := t.TempDir()
	writeMarket(t, dir, 95)
	reg := bt.NewRegistry()
	Register(reg)
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	from := start.Add(20 * 24 * time.Hour)
	to := start.Add(90*24*time.Hour + 15*time.Minute*37) // a candle's close
	traded := 0
	defer func() {
		if traded < 4 {
			t.Errorf("only %d approaches traded: the check proves too little", traded)
		}
	}()
	for _, name := range reg.Names() {
		t.Run(name, func(t *testing.T) {
			cfg := func(s bt.Strategy, data bt.DataSource) bt.Config {
				return bt.Config{Strategy: s, Instrument: "XAUUSD", Data: data, From: from, To: to, Balance: 10000, Risk: 1, Leverage: 2000, Cache: "off"}
			}
			s, _ := reg.New(name)
			want, err := bt.Run(context.Background(), cfg(s, &bt.CSV{Dir: dir}))
			if err != nil {
				t.Fatal(err)
			}
			if len(want.Trades) > 0 {
				traded++
			}
			for _, mode := range []string{"one candle at a time", "with gaps", "all at once"} {
				now := from.Add(7 * time.Minute)
				if mode == "all at once" {
					now = to.Add(time.Hour)
				}
				s, _ := reg.New(name)
				lv, err := bt.NewLive(context.Background(), bt.LiveConfig{Config: cfg(s, newClocked(dir, &now)), Now: func() time.Time { return now }})
				if err != nil {
					t.Fatal(err)
				}
				r := rand.New(rand.NewPCG(3, 5))
				steps, candles := 0, 0
				for now.Before(to.Add(time.Hour)) {
					switch mode {
					case "one candle at a time":
						now = now.Add(15 * time.Minute)
					case "with gaps":
						now = now.Add(time.Duration(1+r.IntN(40)) * 15 * time.Minute)
					}
					up, err := lv.Advance(context.Background())
					if err == bt.ErrLiveDone {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					steps++
					candles += len(up.Candles)
				}
				got := lv.Finish()
				if d := sameRun(want, got); d != "" {
					t.Fatalf("%s: %s", mode, d)
				}
				t.Logf("%s: %d trades, %d decisions, %d candles in %d advances: identical", mode, len(got.Trades), len(got.Decisions), candles, steps)
			}
		})
	}
}

// Updates hand out each trade and decision once, in order, and add up to
// the final Result.
func TestLiveUpdatesAddUp(t *testing.T) {
	dir := t.TempDir()
	writeMarket(t, dir, 60)
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	from := start.Add(10 * 24 * time.Hour)
	now := from.Add(time.Minute)
	lv, err := bt.NewLive(context.Background(), bt.LiveConfig{Config: bt.Config{Strategy: BigTrendChain(), Instrument: "XAUUSD",
		Data: newClocked(dir, &now), From: from, Balance: 10000, Leverage: 2000, Cache: "off"}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	var trades, decisions, eq int
	for now.Before(start.Add(59 * 24 * time.Hour)) {
		now = now.Add(15 * time.Minute)
		up, err := lv.Advance(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		trades, decisions, eq = trades+len(up.Trades), decisions+len(up.Decisions), eq+len(up.Equity)
		if n := len(up.Candles); n > 1 {
			t.Fatalf("%d candles in one 15 minute advance", n)
		}
	}
	st := lv.State()
	res := lv.Result()
	if trades != len(res.Trades) || decisions != len(res.Decisions) || eq != len(res.Equity) {
		t.Fatalf("updates gave %d trades, %d decisions, %d equity points; the result has %d, %d, %d", trades, decisions, eq, len(res.Trades),
			len(res.Decisions), len(res.Equity))
	}
	if !st.Started || st.Candles != len(res.Candles[0].Candles) || !st.LastClose.Equal(res.Range.To) {
		t.Fatalf("state %+v vs result range %v, %d candles", st, res.Range, len(res.Candles[0].Candles))
	}
	if trades == 0 {
		t.Fatal("no trades: the check proves nothing")
	}
}
