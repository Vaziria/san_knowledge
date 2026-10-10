package bot

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/backtest"
	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/knowledge"
	"github.com/wargasipil/autotrade/internal/strategy"
)

// ReviewTrade is one trade from the journal, scored with what the exchange
// booked for it: realized PnL, commission and funding.
type ReviewTrade struct {
	Symbol     string    `json:"symbol"`
	Setup      string    `json:"setup"`
	Approaches []string  `json:"approaches"`
	Coded      bool      `json:"coded"` // a coded setup's signal, rather than an idea of the loop's own
	Side       string    `json:"side"`
	Opened     time.Time `json:"opened"`
	Ended      time.Time `json:"ended"`
	ClosedBy   string    `json:"closed_by"` // close (by the loop), exchange (its stop or target), open
	Entry      float64   `json:"entry"`
	Stop       float64   `json:"stop"`
	Target     float64   `json:"target"`
	Qty        float64   `json:"qty"`
	Net        float64   `json:"net"`   // USDT after fees; unrealized for an open trade
	Final      bool      `json:"final"` // Net is the exchange's figure for a closed trade
	Risk       float64   `json:"risk"`  // USDT lost at the initial stop
	R          float64   `json:"r"`
	MFE        float64   `json:"mfe"` // best excursion before the close, in R
	MAE        float64   `json:"mae"` // worst excursion before the close, in R
	Reason     string    `json:"reason"`
	Snapshot   string    `json:"snapshot,omitempty"`
	CloseNote  string    `json:"close_reason,omitempty"` // the loop's close reason
	Note       string    `json:"note,omitempty"`         // the latest note about the position
}

type Review struct {
	Trades   []ReviewTrade `json:"trades"`
	Holds    int           `json:"holds"`
	Vetoes   int           `json:"vetoes"` // holds whose reason starts with "veto"
	Refusals int           `json:"refusals"`
}

// Review scores every trade in the journal. A trade ends at the loop's
// `close`, or, when the next `open` comes without one, at that open: the
// exchange closed it at its stop or target in between.
func (b *Bot) Review(ctx context.Context) (*Review, error) {
	if !b.Ex.HasKey() {
		return nil, errNoKey
	}
	all, err := b.Journal.Last(math.MaxInt)
	if err != nil {
		return nil, err
	}
	r := &Review{}
	for _, e := range all {
		switch {
		case e.Error != "":
			r.Refusals++
		case e.Action == "hold":
			r.Holds++
			if strings.HasPrefix(strings.ToLower(e.Reason), "veto") {
				r.Vetoes++
			}
		}
	}
	r.Trades, err = b.trades(ctx, all, nil)
	return r, err
}

// Ranger downloads candles over a time range. The market client has it; the
// trades' best and worst points are measured with it.
type Ranger interface {
	KlinesRange(ctx context.Context, symbol, interval string, start, end time.Time) ([]binance.Candle, error)
}

// slack is how far a fill's time on the exchange can be from the journal
// entry around it. Entries are stamped with our estimate of the exchange's
// clock, which is off by up to the last sync's error, so a trade's income is
// taken from slack before its open to slack before the pair's next open:
// the next open's fee is the next trade's, however the clocks fall.
const slack = 2 * time.Second

// trades pairs each open in the journal with how it ended, per symbol: a
// close or an open on another pair does not end it. known holds the closes
// the knowledge graph already has final figures for, by open time (RFC
// 3339); the exchange is not asked about those again. Without an API key the
// figures stay zero and not final.
func (b *Bot) trades(ctx context.Context, all []Entry, known map[string]knowledge.Close) ([]ReviewTrade, error) {
	isOpen := func(e Entry) bool { return e.Error == "" && strings.HasPrefix(e.Action, "open_") }
	symOf := func(e Entry) string {
		if e.Symbol == "" {
			return b.Cfg.Symbol
		}
		return e.Symbol
	}
	now := time.Now().UTC()
	if b.Ex.HasKey() {
		_ = b.Ex.SyncTime(ctx)
		now = b.Ex.Now().UTC()
	}
	posBy := map[string][]binance.Position{}
	positions := func(sym string) ([]binance.Position, error) {
		if p, ok := posBy[sym]; ok || !b.Ex.HasKey() {
			return p, nil
		}
		p, err := b.Ex.Positions(ctx, sym)
		posBy[sym] = p
		return p, err
	}
	notes := map[int]string{}
	for _, e := range all {
		if e.Action == "note" && e.Error == "" && e.Position > 0 {
			notes[e.Position] = e.Reason
		}
	}

	var out []ReviewTrade
	for i, e := range all {
		if !isOpen(e) {
			continue
		}
		sym := symOf(e)
		pos, err := positions(sym)
		if err != nil {
			return nil, err
		}
		t := ReviewTrade{Symbol: sym, Setup: e.Setup, Approaches: e.Approaches, Coded: strategy.Known(e.Setup), Side: strings.TrimPrefix(e.Action, "open_"),
			Opened: e.Time, Entry: e.Price, Stop: e.StopLoss, Target: e.TakeProfit, Qty: e.Qty, Reason: e.Reason, Snapshot: e.Snapshot,
			Note: notes[len(out)+1]}
		if t.Setup == "" {
			t.Setup = "unlabeled"
		}
		if len(t.Approaches) == 0 && e.Setup != "" {
			t.Approaches = []string{e.Setup}
		}
		var closeAt, nextOpen time.Time
		for _, n := range all[i+1:] {
			if symOf(n) != sym {
				continue
			}
			if n.Error == "" && n.Action == "close" && closeAt.IsZero() {
				closeAt, t.CloseNote = n.Time, n.Reason
			}
			if isOpen(n) {
				nextOpen = n.Time
				break
			}
		}
		switch {
		case !closeAt.IsZero():
			t.ClosedBy, t.Ended = "close", closeAt.Add(time.Minute)
			if !nextOpen.IsZero() && nextOpen.Add(-slack).Before(t.Ended) {
				t.Ended = nextOpen.Add(-slack)
			}
		case !nextOpen.IsZero():
			t.ClosedBy, t.Ended = "exchange", nextOpen.Add(-slack)
		case len(pos) > 0 || !b.Ex.HasKey():
			t.ClosedBy, t.Ended = "open", now
		default:
			t.ClosedBy, t.Ended = "exchange", now
		}
		t.Risk = math.Abs(t.Entry-t.Stop) * t.Qty

		if k, ok := known[t.Opened.UTC().Format(time.RFC3339)]; ok && t.ClosedBy != "open" {
			t.Net, t.R, t.Final, t.MFE, t.MAE = k.Net, k.R, true, k.MFE, k.MAE
			if !k.Closed.IsZero() {
				t.Ended = k.Closed
			}
			out = append(out, t)
			continue
		}
		if b.Ex.HasKey() {
			end := t.Ended
			if t.ClosedBy == "open" {
				end = time.Time{} // up to now, whatever the clocks say
			}
			inc, err := b.Ex.Income(ctx, sym, t.Opened.Add(-slack), end)
			if err != nil {
				return nil, err
			}
			last := time.Time{}
			for _, in := range inc {
				switch in.Type {
				case "REALIZED_PNL", "COMMISSION", "FUNDING_FEE":
					t.Net += in.Amount
					if in.Type == "REALIZED_PNL" && in.Time.After(last) {
						last = in.Time
					}
				}
			}
			if t.ClosedBy == "open" {
				t.Net += pos[0].UnrealizedPnL
			} else {
				t.Final = true
				if t.ClosedBy == "exchange" && !last.IsZero() {
					t.Ended = last // when the stop or target filled
				}
				t.MFE, t.MAE = b.excursions(ctx, t)
			}
		}
		if t.Risk > 0 {
			t.R = t.Net / t.Risk
		}
		out = append(out, t)
	}
	return out, nil
}

// excursions measures how far a closed trade went for and against it, in R,
// from candles of the trading interval. Zeros when the candles can't be had.
func (b *Bot) excursions(ctx context.Context, t ReviewTrade) (mfe, mae float64) {
	rg, ok := b.Market.(Ranger)
	r := math.Abs(t.Entry - t.Stop)
	if !ok || r == 0 || !t.Ended.After(t.Opened) {
		return 0, 0
	}
	cs, err := rg.KlinesRange(ctx, t.Symbol, b.Cfg.Interval, t.Opened, t.Ended)
	if err != nil || len(cs) == 0 {
		return 0, 0
	}
	dir := 1.0
	if t.Side == "short" {
		dir = -1
	}
	for _, c := range cs {
		fav, adv := c.High, c.Low
		if dir < 0 {
			fav, adv = c.Low, c.High
		}
		mfe = math.Max(mfe, dir*(fav-t.Entry)/r)
		mae = math.Min(mae, dir*(adv-t.Entry)/r)
	}
	return mfe, mae
}

// Render writes the review: closed trades by setup, own ideas against coded
// signals, then every trade.
func (r *Review) Render(w io.Writer) {
	var closed []backtest.Trade
	bySetup := map[string][]backtest.Trade{}
	byKind := map[string][]backtest.Trade{}
	for _, t := range r.Trades {
		if t.ClosedBy == "open" {
			continue
		}
		bt := backtest.Trade{Setup: t.Setup, Side: t.Side, Opened: t.Opened, Closed: t.Ended, PnL: t.Net, R: t.R}
		closed = append(closed, bt)
		bySetup[t.Setup] = append(bySetup[t.Setup], bt)
		kind := "own ideas"
		if t.Coded {
			kind = "coded signals"
		}
		byKind[kind] = append(byKind[kind], bt)
	}

	fmt.Fprintf(w, "# Review: %d closed trades\n\n", len(closed))
	fmt.Fprintf(w, "PnL after fees and funding, from the exchange. R is measured against the initial stop.\n\n%s", backtest.Header)
	backtest.Row(w, "all", backtest.Summarize(closed))
	for _, k := range []string{"coded signals", "own ideas"} {
		backtest.Row(w, k, backtest.Summarize(byKind[k]))
	}
	setups := make([]string, 0, len(bySetup))
	for s := range bySetup {
		setups = append(setups, s)
	}
	sort.Strings(setups)
	for _, s := range setups {
		backtest.Row(w, s, backtest.Summarize(bySetup[s]))
	}
	fmt.Fprintf(w, "\nDecisions besides trades: %d holds (%d of them vetoes), %d refused or failed commands.\n", r.Holds, r.Vetoes, r.Refusals)

	if len(r.Trades) > 0 {
		fmt.Fprintf(w, "\n## Trades\n\n")
	}
	for i, t := range r.Trades {
		end := fmt.Sprintf("closed by %s by %s", t.ClosedBy, t.Ended.Format("01-02 15:04"))
		if t.ClosedBy == "open" {
			end = "still open, unrealized"
		}
		fmt.Fprintf(w, "- #%d %s %s %s %s @ %s, stop %s: %s, %s USDT (%+.2fR). %s\n", i+1, t.Opened.Format("2006-01-02 15:04"), t.Symbol, t.Setup, t.Side,
			group(t.Entry, -1), group(t.Stop, -1), end, signed(t.Net), t.R, t.Reason)
	}
}
