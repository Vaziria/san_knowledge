package bot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

var errNoKey = errors.New("no API key: set BINANCE_TESTNET_API_KEY and BINANCE_TESTNET_API_SECRET")

var setupLabel = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// Open opens a long or short position worth usdt at the market price, with a
// stop-loss and an optional take-profit (tp = 0). It refuses when a position
// is already open, when the size or the loss at the stop is over the
// configured limits, or when a level is on the wrong side of the mark price.
// If the exchange rejects the stop-loss, the position is closed again.
//
// approaches are the ways of reading the market the analysis used, from the
// approach catalog or the coded setups. The first one labels the trade for
// `review`; a coded setup's name also picks its management rules. The
// snapshot the analysis was made on is kept with the open, for the
// knowledge graph.
func (b *Bot) Open(ctx context.Context, side string, approaches []string, usdt, sl, tp float64, reason string) (Entry, error) {
	if err := b.checkOpen(reason, approaches); err != nil {
		return Entry{}, err
	}
	e := Entry{Time: b.now(ctx), Symbol: b.Cfg.Symbol, Action: "open_" + side, Setup: approaches[0], Approaches: approaches,
		Reason: reason, USDT: usdt, StopLoss: sl, TakeProfit: tp}
	err := b.open(ctx, &e, side)
	if err == nil {
		if name, serr := b.keepSnapshot(ctx, e.Time); serr != nil {
			e.Warning = strings.TrimSpace(e.Warning + " the data snapshot was not saved: " + serr.Error())
		} else {
			e.Snapshot = name
		}
	}
	return b.record(e, err)
}

// checkOpen checks what every open needs: a reason and known approaches.
func (b *Bot) checkOpen(reason string, approaches []string) error {
	if reason == "" {
		return errors.New("a reason is required")
	}
	if len(approaches) == 0 {
		return errors.New("name the approaches the analysis used (at least one)")
	}
	for _, a := range approaches {
		if !setupLabel.MatchString(a) {
			return fmt.Errorf("approach %q must be a short lowercase label such as big_trend", a)
		}
	}
	return b.checkApproaches(approaches)
}

// Close closes the whole position at the market price and cancels its
// stop-loss and take-profit. With no position but a limit entry waiting, it
// cancels the entry instead.
func (b *Bot) Close(ctx context.Context, reason string) (Entry, error) {
	if reason == "" {
		return Entry{}, errors.New("a reason is required")
	}
	e := Entry{Time: b.now(ctx), Symbol: b.Cfg.Symbol, Action: "close", Reason: reason}
	if b.Ex.HasKey() {
		if _, err := b.reconcile(ctx); err != nil {
			return b.record(e, err)
		}
		p, err := b.pending()
		if err != nil {
			return b.record(e, err)
		}
		if p != nil {
			if _, err := b.cancelEntry(ctx, p, reason); err != nil {
				return b.record(e, err)
			}
			pos, err := b.Ex.Positions(ctx, b.Cfg.Symbol)
			if err != nil {
				return b.record(e, err)
			}
			if len(pos) == 0 {
				all, err := b.Journal.Last(1)
				if err != nil || len(all) == 0 {
					return Entry{}, err
				}
				return all[0], nil // the cancel
			}
			// It filled while being cancelled: close that position too.
		}
	}
	return b.record(e, b.close(ctx, &e))
}

// Protect replaces the open position's stop-loss and/or take-profit (0 =
// leave it). A stop-loss may only be tightened, never moved further away.
func (b *Bot) Protect(ctx context.Context, sl, tp float64, reason string) (Entry, error) {
	if reason == "" {
		return Entry{}, errors.New("a reason is required")
	}
	e := Entry{Time: b.now(ctx), Symbol: b.Cfg.Symbol, Action: "protect", Reason: reason, StopLoss: sl, TakeProfit: tp}
	return b.record(e, b.protect(ctx, &e))
}

// Hold records a decision to do nothing this tick.
func (b *Bot) Hold(ctx context.Context, reason string) (Entry, error) {
	if reason == "" {
		return Entry{}, errors.New("a reason is required")
	}
	e := Entry{Time: b.now(ctx), Symbol: b.Cfg.Symbol, Action: "hold", Reason: reason}
	if msgs, err := b.reconcile(ctx); err != nil {
		e.Warning = "checking the limit entry failed: " + err.Error()
	} else if len(msgs) > 0 {
		e.Warning = strings.Join(msgs, " ")
	}
	if pi, err := b.Ex.PremiumIndex(ctx, b.Cfg.Symbol); err == nil {
		e.Price = pi.MarkPrice
	}
	if b.Ex.HasKey() {
		if pos, err := b.Ex.Positions(ctx, b.Cfg.Symbol); err == nil && len(pos) > 0 {
			e.Qty = pos[0].Amount
		}
	}
	return b.record(e, nil)
}

func (b *Bot) now(ctx context.Context) time.Time {
	_ = b.Ex.SyncTime(ctx) // on failure the local clock will do for the journal
	return b.Ex.Now().UTC()
}

func (b *Bot) record(e Entry, err error) (Entry, error) {
	if err != nil {
		e.Error = err.Error()
	}
	if unit, _, _ := b.terms(context.Background()); e.Currency == "" && unit != "USDT" {
		e.Currency = unit
	}
	if jerr := b.Journal.Append(e); jerr != nil {
		if err == nil {
			return e, fmt.Errorf("journal: %w", jerr)
		}
		return e, fmt.Errorf("%w (and writing the journal failed: %v)", err, jerr)
	}
	return e, err
}

// sides returns the order side that opens and the one that closes.
func sides(long bool) (open, close string) {
	if long {
		return "BUY", "SELL"
	}
	return "SELL", "BUY"
}

// checkLevels checks that the stop-loss and take-profit are on the right
// side of the mark price.
func checkLevels(info binance.SymbolInfo, long bool, mark, sl, tp float64) error {
	p := info.FormatPrice
	if sl > 0 && (long && sl >= mark || !long && sl <= mark) {
		return fmt.Errorf("stop-loss %s is on the wrong side of the mark price %s", p(sl), p(mark))
	}
	if tp > 0 && (long && tp <= mark || !long && tp >= mark) {
		return fmt.Errorf("take-profit %s is on the wrong side of the mark price %s", p(tp), p(mark))
	}
	return nil
}

// checkRisk checks the loss if the stop is hit, from the mark price.
func (b *Bot) checkRisk(ctx context.Context, info binance.SymbolInfo, mark, sl, qty float64) error {
	if b.Cfg.MaxRiskUSDT <= 0 {
		return nil
	}
	dist := math.Abs(mark - sl)
	if risk := dist * qty; risk > b.Cfg.MaxRiskUSDT {
		unit, _, maxRisk := b.terms(ctx)
		return fmt.Errorf("a stop-loss at %s loses %.2f %s, over %s %.2f: use at most %.0f %s or a closer stop",
			info.FormatPrice(sl), risk, unit, maxRisk, b.Cfg.MaxRiskUSDT, math.Floor(b.Cfg.MaxRiskUSDT/dist*mark), unit)
	}
	return nil
}

// checkSize checks the size of a new position against its limit.
func (b *Bot) checkSize(ctx context.Context, usdt float64) error {
	unit, maxPos, _ := b.terms(ctx)
	switch {
	case usdt <= 0:
		return fmt.Errorf("the size in %s must be positive", unit)
	case usdt > b.Cfg.MaxPositionUSDT:
		return fmt.Errorf("%.2f %s is over %s %.2f", usdt, unit, maxPos, b.Cfg.MaxPositionUSDT)
	}
	return nil
}

func (b *Bot) open(ctx context.Context, e *Entry, side string) error {
	sym := b.Cfg.Symbol
	if side != "long" && side != "short" {
		return fmt.Errorf("side must be long or short, not %q", side)
	}
	if err := b.checkSize(ctx, e.USDT); err != nil {
		return err
	}
	switch {
	case e.StopLoss <= 0:
		return errors.New("a stop-loss is required")
	case !b.Ex.HasKey():
		return errNoKey
	}
	long := side == "long"
	if err := b.ready(ctx); err != nil {
		return err
	}
	info, err := b.Ex.SymbolInfo(ctx, sym)
	if err != nil {
		return err
	}
	pi, err := b.Ex.PremiumIndex(ctx, sym)
	if err != nil {
		return err
	}
	mark := pi.MarkPrice
	e.StopLoss = info.RoundPrice(e.StopLoss)
	if e.TakeProfit > 0 {
		e.TakeProfit = info.RoundPrice(e.TakeProfit)
	}
	if err := checkLevels(info, long, mark, e.StopLoss, e.TakeProfit); err != nil {
		return err
	}
	qty := info.FloorQty(e.USDT / mark)
	unit, _, _ := b.terms(ctx)
	if qty < info.MinQty || qty <= 0 {
		return fmt.Errorf("%.2f %s buys %s, under the minimum quantity %s", e.USDT, unit, info.FormatQty(qty), info.FormatQty(info.MinQty))
	}
	if qty*mark < info.MinNotional {
		return fmt.Errorf("%.2f %s is under the minimum order value of %.2f %s", qty*mark, unit, info.MinNotional, unit)
	}
	if err := b.checkRisk(ctx, info, mark, e.StopLoss, qty); err != nil {
		return err
	}
	if err := b.prepare(ctx); err != nil {
		return err
	}

	openSide, closeSide := sides(long)
	o, err := b.Ex.MarketOrder(ctx, sym, openSide, info.FormatQty(qty), false)
	if err != nil {
		return err
	}
	e.OrderIDs = append(e.OrderIDs, o.OrderID)
	e.Price, e.Qty = o.AvgPrice, o.ExecutedQty
	if e.Price == 0 {
		e.Price = mark
	}
	if e.Qty == 0 {
		e.Qty = qty
	}
	// The order's average price can differ from the position's entry price
	// (seen on the testnet); the management rules measure from the latter.
	if pos, err := b.Ex.Positions(ctx, sym); err == nil && len(pos) > 0 && pos[0].EntryPrice > 0 {
		e.Price = pos[0].EntryPrice
	}
	e.USDT = e.Price * e.Qty

	stop, err := b.Ex.StopOrder(ctx, sym, closeSide, binance.StopMarket, info.FormatPrice(e.StopLoss))
	if err != nil {
		// Never leave a position without its stop.
		if _, cerr := b.Ex.MarketOrder(ctx, sym, closeSide, info.FormatQty(e.Qty), true); cerr != nil {
			return fmt.Errorf("the stop-loss was rejected (%v) and closing the position failed (%v): the position is OPEN WITHOUT A STOP", err, cerr)
		}
		return fmt.Errorf("the stop-loss was rejected, so the position was closed again: %w", err)
	}
	e.OrderIDs = append(e.OrderIDs, stop.AlgoID)
	if e.TakeProfit > 0 {
		tp, err := b.Ex.StopOrder(ctx, sym, closeSide, binance.TakeProfitMarket, info.FormatPrice(e.TakeProfit))
		if err != nil {
			e.Warning = fmt.Sprintf("take-profit rejected: %v", err)
		} else {
			e.OrderIDs = append(e.OrderIDs, tp.AlgoID)
		}
	}
	return nil
}

func (b *Bot) close(ctx context.Context, e *Entry) error {
	sym := b.Cfg.Symbol
	if !b.Ex.HasKey() {
		return errNoKey
	}
	pos, err := b.Ex.Positions(ctx, sym)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		if left, err := b.Ex.OpenAlgoOrders(ctx, sym); err == nil && len(left) > 0 {
			if err := b.Ex.CancelAlgoOrders(ctx, sym); err == nil {
				e.Warning = fmt.Sprintf("cancelled %d left-over stop orders", len(left))
			}
		}
		return errors.New("there is no open position")
	}
	p := pos[0]
	info, err := b.Ex.SymbolInfo(ctx, sym)
	if err != nil {
		return err
	}
	_, closeSide := sides(p.Amount > 0)
	e.Qty = math.Abs(p.Amount)
	o, err := b.Ex.MarketOrder(ctx, sym, closeSide, info.FormatQty(e.Qty), true)
	if err != nil {
		return err
	}
	e.OrderIDs = append(e.OrderIDs, o.OrderID)
	e.Price = o.AvgPrice
	if e.Price == 0 {
		e.Price = p.MarkPrice
	}
	e.PnL = (e.Price - p.EntryPrice) * p.Amount
	if err := b.Ex.CancelAlgoOrders(ctx, sym); err != nil {
		e.Warning = fmt.Sprintf("the position is closed, but cancelling its stop orders failed: %v", err)
	}
	return nil
}

func (b *Bot) protect(ctx context.Context, e *Entry) error {
	sym := b.Cfg.Symbol
	switch {
	case e.StopLoss <= 0 && e.TakeProfit <= 0:
		return errors.New("give a stop-loss, a take-profit or both")
	case !b.Ex.HasKey():
		return errNoKey
	}
	if _, err := b.reconcile(ctx); err != nil {
		return err
	}
	pos, err := b.Ex.Positions(ctx, sym)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		if p, err := b.pending(); err == nil && p != nil {
			return errors.New("the limit entry has not filled yet; to change its levels, `close` cancels it and a new open places it again")
		}
		return errors.New("there is no open position to protect")
	}
	p := pos[0]
	long := p.Amount > 0
	info, err := b.Ex.SymbolInfo(ctx, sym)
	if err != nil {
		return err
	}
	pi, err := b.Ex.PremiumIndex(ctx, sym)
	if err != nil {
		return err
	}
	mark := pi.MarkPrice
	e.Price, e.Qty = mark, p.Amount
	if e.StopLoss > 0 {
		e.StopLoss = info.RoundPrice(e.StopLoss)
	}
	if e.TakeProfit > 0 {
		e.TakeProfit = info.RoundPrice(e.TakeProfit)
	}
	if err := checkLevels(info, long, mark, e.StopLoss, e.TakeProfit); err != nil {
		return err
	}
	orders, err := b.Ex.OpenAlgoOrders(ctx, sym)
	if err != nil {
		return err
	}
	if e.StopLoss > 0 {
		old := ofType(orders, binance.StopMarket)
		if len(old) > 0 {
			cur := old[0].TriggerPrice
			if long && e.StopLoss < cur || !long && e.StopLoss > cur {
				return fmt.Errorf("the stop-loss can only be tightened: it is at %s, %s is further away", info.FormatPrice(cur), info.FormatPrice(e.StopLoss))
			}
		} else if err := b.checkRisk(ctx, info, mark, e.StopLoss, math.Abs(p.Amount)); err != nil {
			return err
		}
	}

	_, closeSide := sides(long)
	_, inOneStep := b.Ex.(PositionStops)
	replace := func(typ string, price float64) error {
		old := ofType(orders, typ)
		if inOneStep {
			// MetaTrader moves the position's own stop: no gap without one.
			n, err := b.Ex.StopOrder(ctx, sym, closeSide, typ, info.FormatPrice(price))
			if err != nil {
				return fmt.Errorf("the new %s was rejected, the old one stays: %w", typ, err)
			}
			e.OrderIDs = append(e.OrderIDs, n.AlgoID)
			return nil
		}
		// Only one close-position order per type is allowed (-4130), so the
		// old one goes first.
		for _, o := range old {
			if err := b.Ex.CancelAlgoOrder(ctx, sym, o.AlgoID); err != nil {
				return err
			}
		}
		n, err := b.Ex.StopOrder(ctx, sym, closeSide, typ, info.FormatPrice(price))
		if err == nil {
			e.OrderIDs = append(e.OrderIDs, n.AlgoID)
			return nil
		}
		if len(old) == 0 {
			return err
		}
		prev := info.FormatPrice(old[0].TriggerPrice)
		if _, rerr := b.Ex.StopOrder(ctx, sym, closeSide, typ, prev); rerr != nil {
			return fmt.Errorf("the new %s was rejected (%v) and putting back the old one at %s failed (%v)", typ, err, prev, rerr)
		}
		return fmt.Errorf("the new %s was rejected, the old one at %s is back: %w", typ, prev, err)
	}
	if e.StopLoss > 0 {
		if err := replace(binance.StopMarket, e.StopLoss); err != nil {
			return err
		}
	}
	if e.TakeProfit > 0 {
		if err := replace(binance.TakeProfitMarket, e.TakeProfit); err != nil {
			return err
		}
	}
	return nil
}

func ofType(orders []binance.AlgoOrder, typ string) []binance.AlgoOrder {
	var out []binance.AlgoOrder
	for _, o := range orders {
		if o.Type == typ {
			out = append(out, o)
		}
	}
	return out
}
