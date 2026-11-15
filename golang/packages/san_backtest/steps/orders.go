package steps

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
)

// Pending entries and several positions (the user, 2026-11-15: "Is we
// have simulate limit trade ?", then "1b, 2a, 3b" and "1b, 2a, 3a, 4a").

// PendingEntry is EntryLimit's or EntryStop's step; Expire and Tag modify it.
type PendingEntry struct {
	name  string
	stop  bool // a stop order (else a limit)
	level V
	bars  V
	tag   string
}

// EntryLimit turns the chain's entry into a pending limit order at level
// (a value, e.g. EMA("H4", 20)): it sets c.Entry to level before the steps
// after it, so the stop and target steps (SwingStop, RR) and rule #1 work
// from the order's price, then turns the Buy or Sell coming back into
// BuyLimit or SellLimit at level. Put it after the direction steps and
// triggers, before the stop and target. A buy limit must be below the step
// close (a sell limit above it), or the chain waits: a limit through the
// market would just fill at once.
func EntryLimit(level any) *PendingEntry {
	return &PendingEntry{name: "entry_limit", level: num(level)}
}

// EntryStop is EntryLimit with a stop order: a buy stop above the step
// close (a breakout entry), a sell stop below it.
func EntryStop(level any) *PendingEntry {
	return &PendingEntry{name: "entry_stop", stop: true, level: num(level)}
}

// Expire makes the order expire after n step candles without a fill.
func (e *PendingEntry) Expire(n any) *PendingEntry {
	e.bars = num(n)
	return e
}

// Tag labels the order (and its position and trade).
func (e *PendingEntry) Tag(label string) *PendingEntry {
	e.tag = label
	return e
}

func (e *PendingEntry) Setup(d *bt.Decl) {
	d.Name(e.name)
	d.Role(bt.RoleStep)
	e.level.Setup(d)
	if e.bars.f != nil {
		e.bars.count(d)
	}
}

func (e *PendingEntry) Run(c *bt.Context, next bt.Next) bt.Signal {
	lvl := e.level.Value(c)
	if math.IsNaN(lvl) || lvl <= 0 {
		return block("no %s yet", e.level)
	}
	cl := c.Entry
	c.Entry = lvl
	kind := "limit"
	if e.stop {
		kind = "stop"
	}
	c.Explain("%s at %.5g", kind, lvl)
	sig := next(c)
	if !sig.IsEntry() {
		return sig
	}
	n := 0
	if e.bars.f != nil {
		if k, ok := countOf(c, e.bars); ok {
			n = k
		}
	}
	var out []bt.Signal
	for _, x := range sig.List() {
		side := x.Action.Side()
		if x.Action != bt.ActBuy && x.Action != bt.ActSell {
			out = append(out, x)
			continue
		}
		// a buy limit waits below the close and a buy stop above it; sells mirror
		wantBelow := (side == bt.Long) != e.stop
		if !math.IsNaN(cl) && ((wantBelow && lvl >= cl) || (!wantBelow && lvl <= cl)) {
			return block("%s %s at %.5g is on the wrong side of the close %.5g", side, kind, lvl, cl)
		}
		var o bt.Signal
		switch {
		case side == bt.Long && e.stop:
			o = bt.BuyStop(lvl, x.Stop, x.Target)
		case side == bt.Long:
			o = bt.BuyLimit(lvl, x.Stop, x.Target)
		case e.stop:
			o = bt.SellStop(lvl, x.Stop, x.Target)
		default:
			o = bt.SellLimit(lvl, x.Stop, x.Target)
		}
		o.Reason = x.Reason
		out = append(out, o.Expire(n).Tag(e.tag))
	}
	return bt.Do(out...)
}

// MaxOpen lets the chain hold n positions at once: it declares n as the
// strategy's MaxOpen() (Config.MaxOpen overrides it) and stops the chain
// while n positions are open and pending together. Without it a chain
// enters only while flat, as before. Put it after ManageOpen.
func MaxOpen(n int) bt.Link {
	return bt.Define("max_open", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		open, pending := len(c.Positions()), len(c.Orders())
		if open+pending >= n {
			return block("%d open and %d pending, max %d", open, pending, n)
		}
		return next(c)
	}, func(d *bt.Decl) { d.MaxOpen(n) })
}

// OnePerSide drops an entry on a side that already has an open position
// or a pending order (one long and one short at most: a hedge, never two
// on one side).
func OnePerSide() bt.Link {
	return bt.Define("one_per_side", bt.RoleCheck, func(c *bt.Context, next bt.Next) bt.Signal {
		sig := next(c)
		if !sig.IsEntry() {
			return sig
		}
		taken := map[bt.Side]bool{}
		for _, p := range c.Positions() {
			taken[p.Side] = true
		}
		for _, o := range c.Orders() {
			taken[o.Side] = true
		}
		var out []bt.Signal
		var dropped bt.Side
		for _, x := range sig.List() {
			if s := x.Action.Side(); s != "" && taken[s] {
				dropped = s
				continue
			}
			out = append(out, x)
		}
		if res := bt.Do(out...); res.Action != bt.ActWait || dropped == "" {
			return res
		}
		return block("a %s is already open or pending", dropped)
	}, nil)
}
