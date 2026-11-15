package steps

import (
	"fmt"
	"math"
	"strings"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// The filters: each lets the chain go on (calls next) or stops it with a
// reason.

func block(format string, args ...any) bt.Signal { return bt.Wait.Because(format, args...) }

// last is the step candle.
func last(c *bt.Context) (bt.Candle, bool) { return c.Last(c.Step()) }

// Hours lets candles through whose step candle OPENS from from:00 to
// before to:00 WIB (so its signal fills by to:00). from > to wraps past
// midnight: Hours(22, 3) is 22:00-02:59.
func Hours(from, to int) bt.Link {
	return bt.Define("hours", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		k, ok := last(c)
		if !ok {
			return block("no candle")
		}
		h := k.OpenTime.In(bt.WIB).Hour()
		in := h >= from && h < to
		if from > to {
			in = h >= from || h < to
		}
		if !in {
			return block("%02d:00 WIB is outside %02d:00-%02d:00", h, from, to)
		}
		return next(c)
	}, nil)
}

// Weekdays lets through step candles opening on these WIB weekdays.
func Weekdays(days ...time.Weekday) bt.Link {
	return bt.Define("weekdays", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		k, ok := last(c)
		if !ok {
			return block("no candle")
		}
		d := k.OpenTime.In(bt.WIB).Weekday()
		for _, x := range days {
			if x == d {
				return next(c)
			}
		}
		return block("%s is not a trading day here", d)
	}, nil)
}

// Market names for Sessions.
const (
	Asia    = "asia"
	London  = "london"
	NewYork = "newyork"
	Overlap = "overlap" // two sessions open at once
)

// Sessions lets through candles where one of the named sessions is open
// (bt.SessionAt at the candle's last moment, the package's session hours).
func Sessions(names ...string) bt.Link {
	return bt.Define("sessions", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		s := c.Session()
		for _, n := range names {
			if (n == Asia && s.Asia) || (n == London && s.London) || (n == NewYork && s.NewYork) || (n == Overlap && s.Overlap()) {
				return next(c)
			}
		}
		return block("session %s, want %s", s, strings.Join(names, " or "))
	}, func(d *bt.Decl) {
		for _, n := range names {
			if n != Asia && n != London && n != NewYork && n != Overlap {
				d.Fail(fmt.Errorf("steps: Sessions(%q): want asia, london, newyork or overlap", n))
			}
		}
	})
}

// SkipRollover stops the chain within an hour of the daily rollover
// (17:00 New York, 04:00 or 05:00 WIB: wide spreads and the swap) around
// the fill, the step candle's close.
func SkipRollover() bt.Link {
	return bt.Define("skip_rollover", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		t := c.Time()
		if r := bt.Rollovers(t.Add(-time.Hour-time.Nanosecond), t.Add(time.Hour)); len(r) > 0 {
			return block("rollover at %s WIB", r[0].In(bt.WIB).Format("15:04"))
		}
		return next(c)
	}, nil)
}

// SkipMondayOpen stops the chain for the first two hours after the market
// reopens from a gap of a day or more (the weekend, a holiday).
func SkipMondayOpen() bt.Link {
	return bt.Define("skip_monday_open", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		cs := c.Candles(c.Step())
		if len(cs) == 0 {
			return block("no candle")
		}
		t := cs[len(cs)-1].OpenTime
		for i := len(cs) - 1; i > 0 && !cs[i].OpenTime.Before(t.Add(-2*time.Hour)); i-- {
			if cs[i].OpenTime.Sub(cs[i-1].CloseTime) >= 24*time.Hour {
				return block("within 2 hours of the reopen at %s WIB", cs[i].OpenTime.In(bt.WIB).Format("Mon 15:04"))
			}
		}
		return next(c)
	}, nil)
}

func atrRange(name string, tf string, n, x any, min bool) bt.Link {
	a, lim := ATR(tf, n), num(x)
	return bt.Define(name, bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		v, l := a.Value(c), lim.Value(c)
		if math.IsNaN(v) || math.IsNaN(l) {
			return block("no ATR yet")
		}
		if min && v < l {
			return block("%s %.5g < %.5g", a, v, l)
		}
		if !min && v > l {
			return block("%s %.5g > %.5g", a, v, l)
		}
		return next(c)
	}, func(d *bt.Decl) { a.Setup(d); lim.Setup(d) })
}

// MinATR stops the chain while ATR(tf, n) is under x (a price distance);
// MaxATR while it is over x.
func MinATR(tf string, n, x any) bt.Link { return atrRange("min_atr", tf, n, x, true) }
func MaxATR(tf string, n, x any) bt.Link { return atrRange("max_atr", tf, n, x, false) }

// MaxSpread stops the chain while the spread (ask - bid at the step
// candle's close, or the Costs.Spread override) is over x.
func MaxSpread(x any) bt.Link {
	lim := num(x)
	return bt.Define("max_spread", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		sp, l := c.Instrument().Spread, lim.Value(c)
		if math.IsNaN(l) || sp > l {
			return block("spread %.5g > %.5g", sp, l)
		}
		return next(c)
	}, lim.Setup)
}

// OncePerDay lets one entry a day through: once an entry comes back
// through it (on the way back, after the steps inside agreed), the rest of
// that WIB day (of the step candle's open) is blocked.
func OncePerDay() bt.Link {
	var lastDay string
	return bt.Define("once_per_day", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		k, ok := last(c)
		if !ok {
			return block("no candle")
		}
		day := k.OpenTime.In(bt.WIB).Format("2006-01-02")
		if day == lastDay {
			return block("already entered on %s", day)
		}
		sig := next(c)
		if sig.Action == bt.ActBuy || sig.Action == bt.ActSell {
			lastDay = day
		}
		return sig
	}, func(d *bt.Decl) { d.OnStart(func() { lastDay = "" }) })
}
