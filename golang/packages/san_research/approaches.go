package san_research

import (
	"fmt"
	"math"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// LevelFade sells the first touch from below of a level on M15 (study 4:
// the Asian high turns price back more often than an arbitrary price line,
// STRONG). Rules, all prices bid:
//
//   - the touch: the previous close and this candle's open are below the
//     level, this candle's high reaches it, and it closes less than 0.5
//     ATR(14) above it (not yet a break by study 4's definition);
//   - entries only for touches whose candle opens inside the window
//     [FromH, ToH) UTC; one trade per level per day; flat only;
//   - sell at the next candle's open; stop = level + stop_atr x ATR,
//     target = level - target_atr x ATR (San's rule #1: both, always);
//   - anything still open is closed at the open of the 20:30 UTC candle
//     (03:30 WIB): before Exness's Friday close (20:58 UTC) and the daily
//     rollover with its wide spreads.
//
// Short only, so no swap is paid (study 5).
type LevelFade struct {
	ID         string
	FromH, ToH int // entry window, UTC hours
	// Levels gives the levels to watch at this step (empty = none).
	Levels func(c *bt.Context, cs []bt.Candle) []float64
	what   string // for notes: "Asian high" or "control line"

	day  time.Time
	used map[float64]bool
}

// exitMinute: a signal after the candle opening 20:15 UTC closes the
// position at 20:30 UTC.
const exitMinute = 20*60 + 15

// Approach names (backtests/<name>).
const (
	ApproachAsianHighFade = "asian_high_fade"
	ApproachOverlapFade   = "asian_high_fade_overlap"
	BenchmarkControlFade  = "control_line_fade"
)

// NewAsianHighFade: approach 1, touches 14:00-02:45 WIB.
func NewAsianHighFade() *LevelFade {
	return &LevelFade{ID: ApproachAsianHighFade, FromH: 7, ToH: 20, Levels: asianHighLevel, what: "Asian high"}
}

// NewOverlapFade: approach 2, the same fade only in the London+New York
// overlap, 19:00-22:45 WIB (study 1: the biggest moves, so targets are
// reached inside the window more often).
func NewOverlapFade() *LevelFade {
	return &LevelFade{ID: ApproachOverlapFade, FromH: 12, ToH: 16, Levels: asianHighLevel, what: "Asian high"}
}

// NewControlFade is the benchmark, not an approach: the same rules on
// arbitrary lines (50k+17, 50k+33), as study 4's control.
func NewControlFade() *LevelFade {
	return &LevelFade{ID: BenchmarkControlFade, FromH: 7, ToH: 20, Levels: controlLevels, what: "control line"}
}

// Register adds the approaches and the benchmarks to a san_backtest
// registry: the Asian-high fades, and the trend approaches of
// approaches-2026-11-15 (trend_approaches.go, trend_others.go).
func Register(r *bt.Registry) {
	r.Register(ApproachAsianHighFade, func() bt.Strategy { return NewAsianHighFade() })
	r.Register(ApproachOverlapFade, func() bt.Strategy { return NewOverlapFade() })
	r.Register(BenchmarkControlFade, func() bt.Strategy { return NewControlFade() })
	for _, mk := range TrendApproaches() {
		r.Register(mk().Name(), mk)
	}
}

func (s *LevelFade) Name() string         { return s.ID }
func (s *LevelFade) Timeframes() []string { return []string{"M15"} }
func (s *LevelFade) Warmup() int          { return 100 }
func (s *LevelFade) Params() map[string]float64 {
	return map[string]float64{"stop_atr": 1, "target_atr": 1}
}
func (s *LevelFade) OnStart(bt.StartInfo) { s.day, s.used = time.Time{}, nil }

func (s *LevelFade) OnCandle(c *bt.Context) bt.Signal {
	tf := c.Timeframes()[0]
	cs := c.Candles(tf)
	if len(cs) < 2 {
		return bt.Wait
	}
	k, p := cs[len(cs)-1], cs[len(cs)-2]
	if pos := c.Position(); pos != nil {
		if k.OpenTime.UTC().Hour()*60+k.OpenTime.UTC().Minute() >= exitMinute {
			c.Note("closing at 03:30 WIB, before the Friday close and the daily rollover")
			return bt.Close("before rollover")
		}
		return bt.Wait
	}
	t := k.OpenTime.UTC()
	if t.Hour() < s.FromH || t.Hour() >= s.ToH {
		return bt.Wait
	}
	if day := t.Truncate(24 * time.Hour); !day.Equal(s.day) {
		s.day, s.used = day, map[float64]bool{}
	}
	atr := c.ATR(tf, 14)
	if math.IsNaN(atr) || atr <= 0 {
		return bt.Wait
	}
	for _, l := range s.Levels(c, cs) {
		if s.used[l] || !(p.Close < l && k.Open < l && k.High >= l && k.Close < l+0.5*atr) {
			continue
		}
		s.used[l] = true
		stop, target := l+c.Param("stop_atr")*atr, l-c.Param("target_atr")*atr
		c.Note("%s %.2f touched from below at %s WIB (high %.2f, close %.2f, ATR %.2f): sell, stop %.2f, target %.2f",
			s.what, l, k.OpenTime.In(WIB).Format("Mon 15:04"), k.High, k.Close, atr, stop, target)
		return bt.Sell(stop, target)
	}
	return bt.Wait
}

// asianHighLevel is today's Asian high (candles opening 00:00-06:45 UTC),
// once that range is complete (at least 24 of its 28 candles).
func asianHighLevel(_ *bt.Context, cs []bt.Candle) []float64 {
	last := cs[len(cs)-1].OpenTime.UTC()
	if last.Hour() < asiaToH {
		return nil
	}
	day := last.Truncate(24 * time.Hour)
	hi, n := math.Inf(-1), 0
	for i := len(cs) - 1; i >= 0 && !cs[i].OpenTime.Before(day); i-- {
		if cs[i].OpenTime.UTC().Hour() < asiaToH {
			hi = math.Max(hi, cs[i].High)
			n++
		}
	}
	if n < 24 {
		return nil
	}
	return []float64{hi}
}

// controlLevels are the arbitrary lines 50k+17 and 50k+33 up to the last
// candle's high.
func controlLevels(_ *bt.Context, cs []bt.Candle) []float64 {
	k, p := cs[len(cs)-1], cs[len(cs)-2]
	var out []float64
	for m := math.Floor(p.Close/50)*50 - 50; m <= k.High; m += 50 {
		for _, off := range []float64{17, 33} {
			if l := m + off; l > p.Close && l <= k.High {
				out = append(out, l)
			}
		}
	}
	return out
}

func (s *LevelFade) String() string { return fmt.Sprintf("%s [%02d-%02d UTC]", s.ID, s.FromH, s.ToH) }
