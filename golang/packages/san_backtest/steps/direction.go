package steps

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
)

// The direction steps: each works out a side. While Dir is not set it sets
// it; when an earlier step set it, the two must agree, or the chain stops.
// No side (a flat trend, missing data) stops the chain too.

func direct(c *bt.Context, next bt.Next, side bt.Side, why string) bt.Signal {
	if side == "" {
		return block("%s", why)
	}
	if c.Dir == "" {
		c.Dir = side
	} else if c.Dir != side {
		return block("%s, the chain is on %s", side, c.Dir)
	}
	c.Explain("%s", side)
	return next(c)
}

func sideOf(up, down bool) bt.Side {
	switch {
	case up:
		return bt.Long
	case down:
		return bt.Short
	}
	return ""
}

// meanClose is the mean of tf's n closes ending back candles before the
// newest (back 0 = ending with it); NaN without enough candles.
func meanClose(cs []bt.Candle, n, back int) float64 {
	end := len(cs) - back
	if n < 1 || back < 0 || end-n < 0 {
		return math.NaN()
	}
	s := 0.0
	for _, k := range cs[end-n : end] {
		s += k.Close
	}
	return s / float64(n)
}

// TrendSMA is long when tf's close is above its n-candle SMA and that SMA
// is rising: higher than it was back candles ago (back 0 = n/5, at least
// 1). Short is the mirror.
func TrendSMA(tf string, n, back any) bt.Link {
	nv, bv := num(n), num(back)
	return bt.Define("trend_sma", bt.RoleDirection, func(c *bt.Context, next bt.Next) bt.Signal {
		p, ok := countOf(c, nv)
		if !ok {
			return block("bad period")
		}
		b := int(bv.Value(c))
		if b < 1 {
			b = max(1, p/5)
		}
		cs := c.Candles(c.TF(tf))
		now, before := meanClose(cs, p, 0), meanClose(cs, p, b)
		if math.IsNaN(now) || math.IsNaN(before) {
			return block("not enough %s candles for sma%d", tf, p)
		}
		cl := cs[len(cs)-1].Close
		return direct(c, next, sideOf(cl > now && now > before, cl < now && now < before), "no sma trend")
	}, func(d *bt.Decl) {
		nv.count(d)
		bv.count(d)
		if x := nv.Default(); !math.IsNaN(x) {
			b := bv.Default()
			if !(b >= 1) {
				b = math.Max(1, math.Floor(x/5))
			}
			d.Bars(tf, int(x+b))
		}
	})
}

// EMAAgree is long when tf's EMA(fast) is above EMA(slow) and the close is
// above EMA(slow) too; short is the mirror.
func EMAAgree(tf string, fast, slow any) bt.Link {
	f, s := EMA(tf, fast), EMA(tf, slow)
	cl := Close(tf)
	return bt.Define("ema_agree", bt.RoleDirection, func(c *bt.Context, next bt.Next) bt.Signal {
		fv, sv, x := f.Value(c), s.Value(c), cl.Value(c)
		if math.IsNaN(fv + sv + x) {
			return block("no %s ema yet", tf)
		}
		return direct(c, next, sideOf(fv > sv && x > sv, fv < sv && x < sv), tf+" emas and close disagree")
	}, func(d *bt.Decl) { f.Setup(d); s.Setup(d) })
}

// Agree is long when a is above b and short when it is below, e.g.
// Agree(EMA("", 20), EMA("", 50)), or as a confirmation after a trend:
// Agree(RSI("", 14), 50) asks RSI over 50 for longs, under 50 for shorts.
func Agree(a, b any) bt.Link {
	x, y := num(a), num(b)
	return bt.Define("agree", bt.RoleDirection, func(c *bt.Context, next bt.Next) bt.Signal {
		u, v := x.Value(c), y.Value(c)
		if math.IsNaN(u) || math.IsNaN(v) {
			return block("no %s or %s yet", x, y)
		}
		return direct(c, next, sideOf(u > v, u < v), x.name+" = "+y.name)
	}, func(d *bt.Decl) { x.Setup(d); y.Setup(d) })
}

// Momentum is long when tf's close is above the close bars candles
// before, short when below.
func Momentum(tf string, bars any) bt.Link {
	b := num(bars)
	return bt.Define("momentum", bt.RoleDirection, func(c *bt.Context, next bt.Next) bt.Signal {
		n, ok := countOf(c, b)
		cs := c.Candles(c.TF(tf))
		if !ok || len(cs) < n+1 {
			return block("not enough %s candles", tf)
		}
		now, then := cs[len(cs)-1].Close, cs[len(cs)-1-n].Close
		return direct(c, next, sideOf(now > then, now < then), "no momentum")
	}, func(d *bt.Decl) {
		b.count(d)
		if x := b.Default(); !math.IsNaN(x) {
			d.Bars(tf, int(x)+1)
		}
	})
}

// OnlyLong lets longs only: it sets Dir to Long, or stops a short.
func OnlyLong() bt.Link { return only(bt.Long) }

// OnlyShort lets shorts only.
func OnlyShort() bt.Link { return only(bt.Short) }

func only(s bt.Side) bt.Link {
	return bt.Define("only_"+string(s), bt.RoleDirection, func(c *bt.Context, next bt.Next) bt.Signal {
		return direct(c, next, s, "")
	}, nil)
}
