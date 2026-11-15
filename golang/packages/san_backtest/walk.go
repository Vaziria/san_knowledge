package san_backtest

import (
	"fmt"
	"math"
	"time"
)

// WalkResult is walk-forward testing's outcome.
type WalkResult struct {
	Windows []WalkWindow
	// OutOfSample sums the test windows only, chained: each test window
	// starts with the balance the previous one ended with.
	OutOfSample Stats
	Trades      []Trade // the test windows' trades
}

// WalkWindow is one train/test pair.
type WalkWindow struct {
	TrainFrom, TrainTo time.Time
	TestFrom, TestTo   time.Time
	Params             map[string]float64 // picked on the train window
	Trials             int                // train runs tried
	Train, Test        Stats
}

// grid is the values tried for one parameter: its default times 0.5,
// 0.75, 1, 1.25 and 1.5, rounded to whole numbers when the default is
// whole (a period), at least 1.
func grid(v float64) []float64 {
	if v == 0 {
		return []float64{0}
	}
	whole := v == math.Trunc(v)
	var out []float64
	seen := map[float64]bool{}
	for _, f := range []float64{0.5, 0.75, 1, 1.25, 1.5} {
		x := v * f
		if whole {
			x = math.Round(x)
			if v >= 1 && x < 1 {
				x = 1
			}
		}
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func cloneParams(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// walk rolls Train/Test windows over the range that ran. Tuning is one
// pass of coordinate search: for each parameter (by name), try its grid
// with the others held at the best so far, keep the value whose train run
// has the highest net profit (ties keep the earlier value).
func (ld *loaded) walk(w Walk) (*WalkResult, error) {
	out := &WalkResult{}
	balance := ld.p.Balance
	start := balance
	var equity []EquityPoint
	skipped := map[string]int{}
	var oosFrom, oosTo time.Time
	defaults := ld.legs[0].params
	for s := ld.from; !s.Add(w.Train + w.Test).After(ld.to); s = s.Add(w.Test) {
		win := WalkWindow{TrainFrom: s, TrainTo: s.Add(w.Train), TestFrom: s.Add(w.Train), TestTo: s.Add(w.Train + w.Test)}
		best := cloneParams(defaults)
		r, err := ld.run(ld.p.Balance, win.TrainFrom, win.TrainTo, best, false)
		if err != nil {
			return nil, err
		}
		win.Trials = 1
		bestScore, bestStats := r.Stats.NetProfit, r.Stats
		for _, name := range sortedKeys(defaults) {
			for _, v := range grid(defaults[name]) {
				if v == best[name] {
					continue
				}
				trial := cloneParams(best)
				trial[name] = v
				r, err := ld.run(ld.p.Balance, win.TrainFrom, win.TrainTo, trial, false)
				if err != nil {
					return nil, err
				}
				win.Trials++
				if r.Stats.NetProfit > bestScore+1e-9 {
					bestScore, bestStats, best = r.Stats.NetProfit, r.Stats, trial
				}
			}
		}
		win.Params, win.Train = best, bestStats
		test, err := ld.run(balance, win.TestFrom, win.TestTo, best, false)
		if err != nil {
			return nil, err
		}
		win.Test = test.Stats
		balance = test.Stats.EndBalance
		out.Trades = append(out.Trades, test.Trades...)
		equity = append(equity, test.Equity...)
		for k, v := range test.Stats.Skipped {
			skipped[k] += v
		}
		if oosFrom.IsZero() {
			oosFrom = win.TestFrom
		}
		oosTo = win.TestTo
		out.Windows = append(out.Windows, win)
	}
	if len(out.Windows) == 0 {
		return nil, fmt.Errorf("san_backtest: Walk %s:%s does not fit in the range that ran (%s to %s)", w.Train, w.Test,
			ld.from.Format(time.RFC3339), ld.to.Format(time.RFC3339))
	}
	out.OutOfSample = computeStats(out.Trades, equity, start, oosFrom.Add(oosTo.Sub(oosFrom)/2), skipped)
	return out, nil
}
