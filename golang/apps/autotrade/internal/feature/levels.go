package feature

import (
	"math"
	"sort"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// Level is a price the market has turned at.
type Level struct {
	Price    float64   `json:"price"`
	Kind     string    `json:"kind"` // swing high, swing low, prior day high, prior day low
	Interval string    `json:"interval"`
	Time     time.Time `json:"time"` // open time of the candle that made it
}

// Pivots returns the swing highs and lows of closed candles: a high above
// the k candles on each side, or a low below them.
func Pivots(interval string, cs []binance.Candle, k int) []Level {
	var out []Level
	for i := k; i < len(cs)-k; i++ {
		hi, lo := true, true
		for j := i - k; j <= i+k; j++ {
			if j == i {
				continue
			}
			hi = hi && cs[i].High > cs[j].High
			lo = lo && cs[i].Low < cs[j].Low
		}
		if hi {
			out = append(out, Level{Price: cs[i].High, Kind: "swing high", Interval: interval, Time: cs[i].OpenTime})
		}
		if lo {
			out = append(out, Level{Price: cs[i].Low, Kind: "swing low", Interval: interval, Time: cs[i].OpenTime})
		}
	}
	return out
}

// Nearest returns up to n levels above price (nearest first) and n below.
// A level within merge (a fraction of price, 0.0015 = 0.15%) of a nearer
// one is dropped; at equal distance the earlier one in levels is kept.
func Nearest(levels []Level, price float64, n int, merge float64) (above, below []Level) {
	sorted := append([]Level(nil), levels...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].Price-price) < math.Abs(sorted[j].Price-price)
	})
	var kept []Level
	for _, l := range sorted {
		dup := false
		for _, k := range kept {
			if math.Abs(k.Price-l.Price) <= merge*price {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		kept = append(kept, l)
		if l.Price > price && len(above) < n {
			above = append(above, l)
		} else if l.Price <= price && len(below) < n {
			below = append(below, l)
		}
		if len(above) == n && len(below) == n {
			break
		}
	}
	return above, below
}
