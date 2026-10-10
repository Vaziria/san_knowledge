package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/feature"
)

// Source downloads candles. *binance.Client implements it.
type Source interface {
	KlinesRange(ctx context.Context, symbol, interval string, start, end time.Time) ([]binance.Candle, error)
}

// Load returns the closed candles that open in [start, now), from a cache
// file in dir that it extends with what is missing.
func Load(ctx context.Context, src Source, dir, symbol, interval string, start, now time.Time) ([]binance.Candle, error) {
	step, err := feature.IntervalDuration(interval)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.json", symbol, interval))
	var cached []binance.Candle
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cached) // a broken cache is downloaded again
	}
	// A cache from before candles kept the taker buy volume is downloaded
	// again.
	if len(cached) == 0 || cached[0].OpenTime.After(start) || (cached[0].QuoteVolume > 0 && cached[0].TakerBuyQuote == 0) {
		cached = nil
	}
	from := start
	if len(cached) > 0 {
		from = cached[len(cached)-1].OpenTime.Add(step)
	}
	if from.Before(now) {
		more, err := src.KlinesRange(ctx, symbol, interval, from, now)
		if err != nil {
			return nil, err
		}
		cached = append(cached, feature.Closed(more, now)...)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		b, _ := json.Marshal(cached)
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return nil, err
		}
	}
	out := cached[:0:0]
	for _, c := range cached {
		if !c.OpenTime.Before(start) {
			out = append(out, c)
		}
	}
	return out, nil
}
