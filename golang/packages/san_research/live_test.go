package san_research

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
)

// San Backtest's live API (bt.NewLive, the engine of San Tower's paper
// sessions) gives exactly Run's results on the real XAUUSD cache: the same
// trades, missed signals, equity, decision log and stats for every
// approach, fed one candle at a time with gaps and weekends, or all at
// once (a restart's catch-up). Skipped without the cache or with -short.

// liveClock serves the cache's candles closed by now, read once.
type liveClock struct {
	src bt.DataSource
	now *time.Time
	all map[string][]bt.Candle
}

func (c *liveClock) Instrument(ctx context.Context, name string) (bt.Instrument, error) {
	return c.src.Instrument(ctx, name)
}

func (c *liveClock) Candles(ctx context.Context, q bt.Query) ([]bt.Candle, error) {
	all, ok := c.all[q.Timeframe]
	if !ok {
		var err error
		all, err = c.src.Candles(ctx, bt.Query{Instrument: q.Instrument, Timeframe: q.Timeframe, From: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			To: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)})
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

func liveSame(a, b *bt.Result) string {
	for _, p := range []struct {
		name string
		x, y any
	}{{"trades", a.Trades, b.Trades}, {"missed", a.Missed, b.Missed}, {"equity", a.Equity, b.Equity}, {"decisions", a.Decisions, b.Decisions},
		{"candles", a.Candles, b.Candles}, {"stats", a.Stats, b.Stats}, {"legs", a.Legs, b.Legs}} {
		if x, y := fmt.Sprintf("%+v", p.x), fmt.Sprintf("%+v", p.y); x != y {
			i := 0
			for i < min(len(x), len(y)) && x[i] == y[i] {
				i++
			}
			lo := max(0, i-150)
			return fmt.Sprintf("%s differ:\n  run:  ...%s\n  live: ...%s", p.name, x[lo:min(len(x), i+150)], y[lo:min(len(y), i+150)])
		}
	}
	return ""
}

func TestLiveGivesRunsResultsOnTheCache(t *testing.T) {
	dir := cacheDir()
	if dir == "" || testing.Short() {
		t.Skip("no backtest cache on this machine (or -short)")
	}
	off, err := NewOffline(dir, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	floor, err := firstMonth(dir, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	src := floorSource{DataSource: bt.NewCache(off, dir), floor: floor}
	reg := bt.NewRegistry()
	RegisterAll(reg)
	examples.Register(reg)
	end := off.Now().Add(-24 * time.Hour).Truncate(4 * time.Hour) // an H4 (so H1 and M15) close inside the cache
	ranges := []struct {
		mode     string
		from, to time.Time
	}{
		{"all at once, 2022-05 to the end", time.Date(2022, 5, 2, 0, 0, 0, 0, time.UTC), end},
		{"one candle at a time with gaps, last 9 months", end.AddDate(0, -9, 0), end},
	}
	total := 0
	for _, name := range reg.Names() {
		if name == ApproachDipFadeM1 || name == ApproachDipFadeM1Always {
			continue // steps on M1, cached only from 2024-05; the 15-minute replay below is for M15 and up
		}
		for _, rg := range ranges {
			cfg := func(data bt.DataSource) bt.Config {
				s, _ := reg.New(name)
				return bt.Config{Strategy: s, Instrument: "XAUUSD", Data: data, From: rg.from, To: rg.to, Balance: btBalance, Risk: btRisk,
					Leverage: btLeverage, Cache: "off", DayTZ: WIB}
			}
			want, err := bt.Run(context.Background(), cfg(src))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			now := rg.to.Add(time.Hour)
			step := time.Duration(0)
			if rg.mode != ranges[0].mode {
				now, step = rg.from.Add(5*time.Minute), 15*time.Minute
			}
			lv, err := bt.NewLive(context.Background(), bt.LiveConfig{Config: cfg(&liveClock{src: src, now: &now, all: map[string][]bt.Candle{}}),
				Now: func() time.Time { return now }})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			n := 0
			for step > 0 && now.Before(rg.to.Add(time.Hour)) {
				n++
				now = now.Add(step * time.Duration(1+n%7)) // 15 min to 1 h 45 between looks
				if _, err := lv.Advance(context.Background()); err == bt.ErrLiveDone {
					break
				} else if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
			}
			got := lv.Finish()
			if d := liveSame(want, got); d != "" {
				t.Fatalf("%s, %s: %s", name, rg.mode, d)
			}
			total += len(want.Trades)
			t.Logf("%s, %s: %d trades, %d decisions, PF %.3f: identical", name, rg.mode, len(got.Trades), len(got.Decisions), got.Stats.ProfitFactor)
		}
	}
	if total == 0 {
		t.Fatal("no trades: the check proves nothing")
	}
}
