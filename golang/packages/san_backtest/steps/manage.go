package steps

import (
	"fmt"
	"math"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// ManageOpen goes first in a chain. While a position is open it runs its
// steps as a small chain of their own and returns their answer without
// running the entry steps: each manage step either acts (Close, MoveStop)
// or passes to the next one, so the first that acts wins, in the order
// given. While flat it calls next. ManageOpen() with no steps simply keeps
// the entry steps from running while a position is open.
func ManageOpen(links ...bt.Link) bt.Link {
	var sub bt.Next
	return bt.Define("manage", bt.RoleManage, func(c *bt.Context, next bt.Next) bt.Signal {
		if c.Position() == nil {
			return next(c)
		}
		return sub(c)
	}, func(d *bt.Decl) { sub = d.Sub("manage", links...) })
}

func manage(name string, run func(c *bt.Context, p *bt.Position, h bt.Held) (bt.Signal, bool), setup func(d *bt.Decl)) bt.Link {
	return bt.Define(name, bt.RoleStep, func(c *bt.Context, next bt.Next) bt.Signal {
		p := c.Position()
		if p == nil {
			return next(c)
		}
		if sig, ok := run(c, p, c.Held()); ok {
			return sig
		}
		return next(c)
	}, setup)
}

// BreakEven moves the stop to the entry once the step close is atR times
// the first risk (1R, Held.R) in profit, while the stop is still on the
// losing side.
func BreakEven(atR any) bt.Link {
	rv := num(atR)
	return manage("break_even", func(c *bt.Context, p *bt.Position, h bt.Held) (bt.Signal, bool) {
		x, cl := rv.Value(c), c.Close(c.Step())
		if h.R <= 0 || math.IsNaN(x) || math.IsNaN(cl) {
			return bt.Wait, false
		}
		if (p.Side == bt.Long && p.Stop < p.Entry && cl >= p.Entry+x*h.R) || (p.Side == bt.Short && p.Stop > p.Entry && cl <= p.Entry-x*h.R) {
			c.Explain("%gR in profit", x)
			return bt.MoveStop(p.Entry).Because("break-even"), true
		}
		return bt.Wait, false
	}, rv.Setup)
}

// Trail trails the stop distR x 1R behind the best price since the entry
// once the best is afterR x 1R in profit; it only moves the stop closer.
// Trail(1, 1) is a 1R trail from +1R (break-even at +1R).
func Trail(afterR, distR any) bt.Link {
	av, dv := num(afterR), num(distR)
	return manage("trail", func(c *bt.Context, p *bt.Position, h bt.Held) (bt.Signal, bool) {
		a, d := av.Value(c), dv.Value(c)
		r := h.R
		if r <= 0 || math.IsNaN(a+d) {
			return bt.Wait, false
		}
		var ns float64
		if p.Side == bt.Long {
			if h.Best-p.Entry < a*r {
				return bt.Wait, false
			}
			if ns = h.Best - d*r; ns <= p.Stop+1e-9 {
				return bt.Wait, false
			}
		} else {
			if p.Entry-h.Best < a*r {
				return bt.Wait, false
			}
			if ns = h.Best + d*r; ns >= p.Stop-1e-9 {
				return bt.Wait, false
			}
		}
		c.Explain("best %.5g is %.1fR from entry %.5g", h.Best, math.Abs(h.Best-p.Entry)/r, p.Entry)
		return bt.MoveStop(ns).Because("trail"), true
	}, func(d *bt.Decl) { av.Setup(d); dv.Setup(d) })
}

// TimeStop closes the position once it has been open for bars step
// candles (Held.Bars, the candle it opened on included).
func TimeStop(bars any) bt.Link {
	bv := num(bars)
	return manage("time_stop", func(c *bt.Context, p *bt.Position, h bt.Held) (bt.Signal, bool) {
		n, ok := countOf(c, bv)
		if ok && h.Bars >= n {
			c.Explain("held %d candles", h.Bars)
			return bt.Close("time stop"), true
		}
		return bt.Wait, false
	}, bv.count)
}

// CloseAt closes the position on the first step candle that opens at or
// after hour:00 WIB following the entry (CloseAt(2): out by the 02:00 WIB
// candle's close, before the rollover). reason, if given, is the close's
// reason in the trade list (default "close at HH:00 WIB").
func CloseAt(hour int, reason ...string) bt.Link {
	why := fmt.Sprintf("close at %02d:00 WIB", hour)
	if len(reason) > 0 {
		why = reason[0]
	}
	return manage("close_at", func(c *bt.Context, p *bt.Position, h bt.Held) (bt.Signal, bool) {
		k, ok := last(c)
		if !ok {
			return bt.Wait, false
		}
		o := p.OpenTime.In(bt.WIB)
		at := time.Date(o.Year(), o.Month(), o.Day(), hour, 0, 0, 0, bt.WIB)
		if !at.After(o) {
			at = at.AddDate(0, 0, 1)
		}
		if !k.OpenTime.Before(at) {
			return bt.Close(why), true
		}
		return bt.Wait, false
	}, nil)
}

// CloseIf closes the position with reason when f says so.
func CloseIf(reason string, f func(c *bt.Context) bool) bt.Link {
	return manage("close_if", func(c *bt.Context, p *bt.Position, h bt.Held) (bt.Signal, bool) {
		if f(c) {
			return bt.Close(reason), true
		}
		return bt.Wait, false
	}, nil)
}

// ExitOnTrendFlip closes the position when the direction steps turn
// against it: dirs run on their own (e.g. the chain's EMAAgree passed
// again here); with none it uses the chain's own top-level direction
// steps. A flat trend (no side) is not a flip.
func ExitOnTrendFlip(dirs ...bt.Link) bt.Link {
	var dir func(c *bt.Context) bt.Side
	return bt.Define("exit_on_trend_flip", bt.RoleStep, func(c *bt.Context, next bt.Next) bt.Signal {
		p := c.Position()
		if p == nil {
			return next(c)
		}
		if s := dir(c); s != "" && s != p.Side {
			c.Explain("trend now %s", s)
			return bt.Close("trend turned")
		}
		return next(c)
	}, func(d *bt.Decl) { dir = d.Directions(dirs...) })
}
