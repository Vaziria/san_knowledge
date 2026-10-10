package bot

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Entry is one decision and what came of it.
type Entry struct {
	Time       time.Time `json:"time"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"`               // open_long, open_short, order_long, order_short, cancel, close, protect, hold, note
	Setup      string    `json:"setup,omitempty"`      // open: label of the idea (the first approach)
	Approaches []string  `json:"approaches,omitempty"` // open: the approaches the analysis used
	Snapshot   string    `json:"snapshot,omitempty"`   // open: the saved data snapshot (data/snapshots/<name>.json)
	Position   int       `json:"position,omitempty"`   // note: the position (1 = the first open) it is about
	Reason     string    `json:"reason"`
	Price      float64   `json:"price,omitempty"` // open: the position's entry price; hold/protect: the mark price
	Qty        float64   `json:"qty,omitempty"`   // filled; for hold/protect the position (negative = short)
	USDT       float64   `json:"usdt,omitempty"`
	// Currency is what USDT and PnL are in when it is not USDT: the
	// MetaTrader account's currency (USD).
	Currency   string  `json:"currency,omitempty"`
	StopLoss   float64 `json:"stop_loss,omitempty"`
	TakeProfit float64 `json:"take_profit,omitempty"`
	PnL        float64 `json:"pnl,omitempty"` // close: (exit - entry) x qty, before fees
	// Limit is the price of a post-only limit entry: on its order_* entry,
	// and on the open_* written when it fills. Expires is when an unfilled
	// one is cancelled.
	Limit   float64   `json:"limit,omitempty"`
	Expires time.Time `json:"expires,omitempty"`
	// Ordered is when the limit entry of an open_* was placed: the fees of
	// its first fills can come before the open's time (its last fill).
	Ordered  time.Time `json:"ordered,omitempty"`
	OrderIDs []int64   `json:"order_ids,omitempty"`
	Warning  string    `json:"warning,omitempty"`
	Error    string    `json:"error,omitempty"` // refused or failed
}

// Journal is an append-only JSON-lines file.
type Journal struct{ Path string }

func (j Journal) Append(e Entry) error {
	if err := os.MkdirAll(filepath.Dir(j.Path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(j.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(e)
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Last returns the last n entries, oldest first. A missing file is empty.
func (j Journal) Last(n int) ([]Entry, error) {
	f, err := os.Open(j.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		out = append(out, e)
		if len(out) > n {
			out = out[1:]
		}
	}
	return out, sc.Err()
}
