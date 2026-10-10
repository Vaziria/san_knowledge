package bot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// DefaultExpiry is how long an unfilled limit entry waits when the open names
// no expiry: three 5m candles.
const DefaultExpiry = 15 * time.Minute

// OpenLimit places a post-only limit entry: the order rests in the book and
// pays the maker fee when it fills, or the exchange refuses it if it would
// trade at once. Its stop-loss and take-profit go in with it as reduce-only
// orders, so the position is protected from its first fill. Nothing manages
// it between ticks except the exchange; every later action and snapshot on
// the pair reconciles it: a fill becomes the position's open_* entry, and an
// order past expire, one whose price ran to the target first, or one that
// lost its stop is cancelled. Limits are checked as for Open, with the risk
// measured from the limit price.
func (b *Bot) OpenLimit(ctx context.Context, side string, approaches []string, usdt, limit, sl, tp float64, expire time.Duration, reason string) (Entry, error) {
	if err := b.checkOpen(reason, approaches); err != nil {
		return Entry{}, err
	}
	if expire <= 0 {
		expire = DefaultExpiry
	}
	e := Entry{Time: b.now(ctx), Symbol: b.Cfg.Symbol, Action: "order_" + side, Setup: approaches[0], Approaches: approaches,
		Reason: reason, USDT: usdt, Limit: limit, StopLoss: sl, TakeProfit: tp}
	e.Expires = e.Time.Add(expire)
	// The snapshot is kept first: taking one reconciles the pair, which
	// would see the new stop orders without a recorded entry and cancel them.
	name, serr := b.keepSnapshot(ctx, e.Time)
	err := b.placeLimit(ctx, &e, side)
	if err == nil {
		if serr != nil {
			e.Warning = strings.TrimSpace(e.Warning + " the data snapshot was not saved: " + serr.Error())
		} else {
			e.Snapshot = name
		}
	}
	return b.record(e, err)
}

func (b *Bot) placeLimit(ctx context.Context, e *Entry, side string) error {
	sym := b.Cfg.Symbol
	switch {
	case side != "long" && side != "short":
		return fmt.Errorf("side must be long or short, not %q", side)
	case e.Limit <= 0:
		return errors.New("a limit price is required")
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
	e.Limit = info.RoundPrice(e.Limit)
	e.StopLoss = info.RoundPrice(e.StopLoss)
	if e.TakeProfit > 0 {
		e.TakeProfit = info.RoundPrice(e.TakeProfit)
	}
	// The levels must be on the right side of the entry, and the stop also
	// of the mark price, or it would trigger before the entry fills.
	if err := checkLevels(info, long, e.Limit, e.StopLoss, e.TakeProfit); err != nil {
		return errors.New(strings.Replace(err.Error(), "mark price", "limit price", 1))
	}
	if err := checkLevels(info, long, pi.MarkPrice, e.StopLoss, 0); err != nil {
		return err
	}
	qty := info.FloorQty(e.USDT / e.Limit)
	unit, _, _ := b.terms(ctx)
	if qty < info.MinQty || qty <= 0 {
		return fmt.Errorf("%.2f %s buys %s, under the minimum quantity %s", e.USDT, unit, info.FormatQty(qty), info.FormatQty(info.MinQty))
	}
	if qty*e.Limit < info.MinNotional {
		return fmt.Errorf("%.2f %s is under the minimum order value of %.2f %s", qty*e.Limit, unit, info.MinNotional, unit)
	}
	if err := b.checkRisk(ctx, info, e.Limit, e.StopLoss, qty); err != nil {
		return err
	}
	if err := b.prepare(ctx); err != nil {
		return err
	}

	openSide, closeSide := sides(long)
	o, err := b.Ex.LimitOrder(ctx, sym, openSide, info.FormatQty(qty), info.FormatPrice(e.Limit))
	if err != nil {
		return err
	}
	e.OrderIDs = append(e.OrderIDs, o.OrderID)
	e.Qty, e.USDT = qty, qty*e.Limit

	stop, err := b.Ex.ReduceStopOrder(ctx, sym, closeSide, binance.StopMarket, info.FormatPrice(e.StopLoss), info.FormatQty(qty))
	if err != nil {
		// Never leave an entry that can fill without its stop.
		c, cerr := b.Ex.CancelOrder(ctx, sym, o.OrderID)
		switch {
		case cerr != nil:
			return fmt.Errorf("the stop-loss was rejected (%v) and cancelling the limit entry failed (%v): CANCEL ORDER %d BY HAND", err, cerr, o.OrderID)
		case c.ExecutedQty > 0:
			if _, merr := b.Ex.MarketOrder(ctx, sym, closeSide, info.FormatQty(c.ExecutedQty), true); merr != nil {
				return fmt.Errorf("the stop-loss was rejected (%v), the entry had partly filled, and closing it failed (%v): the position is OPEN WITHOUT A STOP", err, merr)
			}
			return fmt.Errorf("the stop-loss was rejected, so the entry was cancelled and its partial fill closed: %w", err)
		}
		return fmt.Errorf("the stop-loss was rejected, so the limit entry was cancelled: %w", err)
	}
	e.OrderIDs = append(e.OrderIDs, stop.AlgoID)
	if e.TakeProfit > 0 {
		t, err := b.Ex.ReduceStopOrder(ctx, sym, closeSide, binance.TakeProfitMarket, info.FormatPrice(e.TakeProfit), info.FormatQty(qty))
		if err != nil {
			e.Warning = fmt.Sprintf("take-profit rejected: %v", err)
		} else {
			e.OrderIDs = append(e.OrderIDs, t.AlgoID)
		}
	}
	return nil
}

// ready refuses an entry while the account is in hedge mode, or the pair has
// a position or a limit entry waiting.
func (b *Bot) ready(ctx context.Context) error {
	sym := b.Cfg.Symbol
	if dual, err := b.Ex.DualSide(ctx); err != nil {
		return err
	} else if dual {
		return errors.New("the account is in hedge mode; switch it to one-way mode")
	}
	if _, err := b.reconcile(ctx); err != nil {
		return err
	}
	pos, err := b.Ex.Positions(ctx, sym)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return fmt.Errorf("a %s position of %g is already open; close it first", pos[0].Side(), math.Abs(pos[0].Amount))
	}
	if p, err := b.pending(); err != nil {
		return err
	} else if p != nil {
		return fmt.Errorf("a limit %s entry at %g is waiting to fill; `close` cancels it", strings.TrimPrefix(p.Action, "order_"), p.Limit)
	}
	return nil
}

// prepare cancels conditional orders left from an earlier position, which
// would act on the new one too, and sets the leverage.
func (b *Bot) prepare(ctx context.Context) error {
	sym := b.Cfg.Symbol
	if left, err := b.Ex.OpenAlgoOrders(ctx, sym); err != nil {
		return err
	} else if len(left) > 0 {
		if err := b.Ex.CancelAlgoOrders(ctx, sym); err != nil {
			return err
		}
	}
	if sc, err := b.Ex.SymbolConfig(ctx, sym); err != nil {
		return err
	} else if sc.Leverage != b.Cfg.Leverage {
		if err := b.Ex.SetLeverage(ctx, sym, b.Cfg.Leverage); err != nil {
			return err
		}
	}
	return nil
}

// pending is the pair's limit entry still waiting, or nil.
func (b *Bot) pending() (*Entry, error) {
	all, err := b.Journal.Last(math.MaxInt)
	if err != nil {
		return nil, err
	}
	return pendingEntry(all, b.Cfg.Symbol), nil
}

// pendingEntry is the newest successful order_* entry of sym with no open_*
// or cancel after it.
func pendingEntry(all []Entry, sym string) *Entry {
	for i := len(all) - 1; i >= 0; i-- {
		e := all[i]
		if e.Error != "" || (e.Symbol != "" && e.Symbol != sym) {
			continue
		}
		switch {
		case strings.HasPrefix(e.Action, "order_"):
			if len(e.OrderIDs) == 0 {
				return nil
			}
			return &all[i]
		case strings.HasPrefix(e.Action, "open_"), e.Action == "cancel":
			return nil
		}
	}
	return nil
}

// reconcile brings the journal up to date with the exchange for the pair's
// limit entry: a fill is recorded as the position's open_* entry (at the
// fill's time, partly filled ones after cancelling the rest); an unfilled
// one is cancelled once past its expiry, when the price reached its target
// first, or when its stop is gone; one the exchange cancelled is recorded.
// With no entry waiting and no position, stop and target orders left from a
// closed position are cancelled. It returns what it did, for the notes.
func (b *Bot) reconcile(ctx context.Context) ([]string, error) {
	if !b.Ex.HasKey() {
		return nil, nil
	}
	sym := b.Cfg.Symbol
	p, err := b.pending()
	if err != nil {
		return nil, err
	}
	if p == nil {
		return b.sweep(ctx)
	}
	o, err := b.Ex.GetOrder(ctx, sym, p.OrderIDs[0])
	if err != nil {
		return nil, err
	}
	long := p.Action == "order_long"
	if o.Working() && o.ExecutedQty == 0 {
		pi, err := b.Ex.PremiumIndex(ctx, sym)
		if err != nil {
			return nil, err
		}
		algos, err := b.Ex.OpenAlgoOrders(ctx, sym)
		if err != nil {
			return nil, err
		}
		now := b.now(ctx)
		why := ""
		switch {
		case !p.Expires.IsZero() && now.After(p.Expires):
			why = fmt.Sprintf("not filled within %s", p.Expires.Sub(p.Time).Round(time.Minute))
		case p.TakeProfit > 0 && (long && pi.MarkPrice >= p.TakeProfit || !long && pi.MarkPrice <= p.TakeProfit):
			why = "the price reached the target before the entry filled"
		case !hasStop(algos):
			why = "its stop-loss order is gone"
		}
		if why == "" {
			return nil, nil
		}
		return b.cancelEntry(ctx, p, "auto: "+why)
	}
	if o.Working() { // partly filled: keep what filled, cancel the rest
		if o, err = b.Ex.CancelOrder(ctx, sym, o.OrderID); err != nil {
			return nil, err
		}
	}
	if o.ExecutedQty == 0 {
		if err := b.Ex.CancelAlgoOrders(ctx, sym); err != nil {
			return nil, err
		}
		e := Entry{Time: b.now(ctx), Symbol: sym, Action: "cancel", Limit: p.Limit, OrderIDs: []int64{o.OrderID},
			Reason: fmt.Sprintf("auto: the exchange reports the limit entry %s", strings.ToLower(o.Status))}
		_, err := b.record(e, nil)
		return []string{e.Reason}, err
	}
	return b.filled(ctx, p, o)
}

// filled records the open_* entry of a limit entry that filled.
func (b *Bot) filled(ctx context.Context, p *Entry, o binance.Order) ([]string, error) {
	sym := b.Cfg.Symbol
	e := Entry{Time: o.Updated, Symbol: sym, Action: "open_" + strings.TrimPrefix(p.Action, "order_"), Setup: p.Setup,
		Approaches: p.Approaches, Snapshot: p.Snapshot, Reason: p.Reason, StopLoss: p.StopLoss, TakeProfit: p.TakeProfit,
		Limit: p.Limit, Ordered: p.Time, OrderIDs: p.OrderIDs, Qty: o.ExecutedQty, Price: o.AvgPrice}
	if e.Time.IsZero() {
		e.Time = b.now(ctx)
	}
	if e.Price == 0 {
		e.Price = p.Limit
	}
	pos, err := b.Ex.Positions(ctx, sym)
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("The limit %s entry filled %g @ %g at %s UTC.", strings.TrimPrefix(p.Action, "order_"), e.Qty, e.Price, e.Time.Format("15:04"))
	switch {
	case len(pos) == 0:
		// Its stop or target already closed it; the other one is left over.
		msg += " Its stop or target has closed it since."
		if err := b.Ex.CancelAlgoOrders(ctx, sym); err != nil {
			e.Warning = fmt.Sprintf("cancelling the left-over stop orders failed: %v", err)
		}
	case pos[0].EntryPrice > 0:
		e.Price = pos[0].EntryPrice
	}
	if p.Qty > 0 && e.Qty < p.Qty {
		e.Warning = strings.TrimSpace(fmt.Sprintf("partly filled: %g of %g; the rest was cancelled. %s", e.Qty, p.Qty, e.Warning))
		msg += fmt.Sprintf(" Partly: %g of %g, the rest cancelled.", e.Qty, p.Qty)
	}
	e.USDT = e.Price * e.Qty
	_, err = b.record(e, nil)
	return []string{msg}, err
}

// cancelEntry cancels the pair's waiting limit entry and its stop and
// target, and records a cancel. An entry that filled meanwhile is recorded
// as opened instead.
func (b *Bot) cancelEntry(ctx context.Context, p *Entry, reason string) ([]string, error) {
	sym := b.Cfg.Symbol
	o, err := b.Ex.CancelOrder(ctx, sym, p.OrderIDs[0])
	if err != nil {
		if g, gerr := b.Ex.GetOrder(ctx, sym, p.OrderIDs[0]); gerr == nil && !g.Working() {
			o = g // it filled or ended in the meantime
		} else {
			return nil, err
		}
	}
	if o.ExecutedQty > 0 {
		return b.filled(ctx, p, o)
	}
	e := Entry{Time: b.now(ctx), Symbol: sym, Action: "cancel", Limit: p.Limit, OrderIDs: []int64{o.OrderID}, Reason: reason}
	if err := b.Ex.CancelAlgoOrders(ctx, sym); err != nil {
		e.Warning = fmt.Sprintf("the entry is cancelled, but cancelling its stop orders failed: %v", err)
	}
	_, err = b.record(e, nil)
	return []string{fmt.Sprintf("Cancelled the limit %s entry at %g: %s", strings.TrimPrefix(p.Action, "order_"), p.Limit, reason)}, err
}

// sweep cancels stop and target orders left when the pair has neither a
// position nor a limit entry waiting.
func (b *Bot) sweep(ctx context.Context) ([]string, error) {
	sym := b.Cfg.Symbol
	algos, err := b.Ex.OpenAlgoOrders(ctx, sym)
	if err != nil || len(algos) == 0 {
		return nil, err
	}
	pos, err := b.Ex.Positions(ctx, sym)
	if err != nil || len(pos) > 0 {
		return nil, err
	}
	if err := b.Ex.CancelAlgoOrders(ctx, sym); err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("Cancelled %d stop/take-profit orders left from a closed position.", len(algos))}, nil
}
