package steps

import (
	"fmt"
	"math"

	bt "github.com/wargasipil/san_backtest"
)

// The triggers: each looks for an entry on Dir's side, or on both sides
// while Dir is not set (then it sets Dir to the side that fired; both at
// once stops the chain). A trigger that fires calls next; one that does
// not stops the chain. The built-in ones keep no state, so the chain may
// also run them on a candle an earlier step blocked, to log "would fire".

// fire runs a trigger: up/down say whether it fires long/short.
func fire(c *bt.Context, next bt.Next, what string, up, down func() bool) bt.Signal {
	switch c.Dir {
	case bt.Long:
		if !up() {
			return block("no %s long", what)
		}
	case bt.Short:
		if !down() {
			return block("no %s short", what)
		}
	default:
		u, d := up(), down()
		switch {
		case u && d:
			return block("%s fired both ways", what)
		case u:
			c.Dir = bt.Long
		case d:
			c.Dir = bt.Short
		default:
			return block("no %s", what)
		}
	}
	c.Explain("fired %s", c.Dir)
	return next(c)
}

func trigger(name string, run bt.Step, needs ...V) bt.Link {
	return bt.Define(name, bt.RoleTrigger, run, func(d *bt.Decl) {
		d.Pure()
		for _, v := range needs {
			v.Setup(d)
		}
	})
}

// Pullback is PullbackTo's trigger; CameFrom adds the "came from" rule.
type Pullback struct {
	tf         string
	level      V
	bars       V
	dist       V
	withCameTo bool
}

// PullbackTo fires long when tf's candle dipped to level (low <= level),
// closed back above it and closed up (close > open); short is the mirror
// (high >= level, close below it, down candle). level is a value, e.g.
// EMA("H4", 20).
func PullbackTo(tf string, level any) *Pullback { return &Pullback{tf: tf, level: num(level)} }

// CameFrom also asks that, within the bars candles before this one, price
// closed at least dist beyond the level (above it for a long): a real
// pullback, not a market sitting on the level.
func (p *Pullback) CameFrom(bars, dist any) *Pullback {
	p.bars, p.dist, p.withCameTo = num(bars), num(dist), true
	return p
}

func (p *Pullback) Setup(d *bt.Decl) {
	d.Name("pullback_to")
	d.Role(bt.RoleTrigger)
	d.Pure()
	p.level.Setup(d)
	if p.withCameTo {
		p.bars.count(d)
		p.dist.Setup(d)
		if n := p.bars.Default(); !math.IsNaN(n) {
			d.Bars(p.tf, int(n)+1)
		}
	}
}

func (p *Pullback) Run(c *bt.Context, next bt.Next) bt.Signal {
	cs := c.Candles(c.TF(p.tf))
	lvl := p.level.Value(c)
	if len(cs) == 0 || math.IsNaN(lvl) {
		return block("no %s yet", p.level)
	}
	k := cs[len(cs)-1]
	far, dist := math.NaN(), 0.0
	var before []bt.Candle
	if p.withCameTo {
		n, ok := countOf(c, p.bars)
		dist = p.dist.Value(c)
		if !ok || len(cs) < n+1 || math.IsNaN(dist) {
			return block("not enough %s candles", p.tf)
		}
		before = cs[len(cs)-1-n : len(cs)-1]
	}
	came := func(up bool) bool {
		if !p.withCameTo {
			return true
		}
		far = math.Inf(-1)
		if !up {
			far = math.Inf(1)
		}
		for _, x := range before {
			if up {
				far = math.Max(far, x.Close)
			} else {
				far = math.Min(far, x.Close)
			}
		}
		if up {
			return far >= lvl+dist
		}
		return far <= lvl-dist
	}
	sig := fire(c, next, "pullback to "+p.level.name,
		func() bool { return k.Low <= lvl && k.Close > lvl && k.Close > k.Open && came(true) },
		func() bool { return k.High >= lvl && k.Close < lvl && k.Close < k.Open && came(false) })
	return sig
}

// Breakout fires long when tf's close is above the highest high of the n
// candles before it (a Donchian breakout), short below the lowest low.
func Breakout(tf string, n any) bt.Link {
	b := num(n)
	return bt.Define("breakout", bt.RoleTrigger, func(c *bt.Context, next bt.Next) bt.Signal {
		m, ok := countOf(c, b)
		cs := c.Candles(c.TF(tf))
		if !ok || len(cs) < m+1 {
			return block("not enough %s candles", tf)
		}
		hi, lo := math.Inf(-1), math.Inf(1)
		for _, x := range cs[len(cs)-1-m : len(cs)-1] {
			hi, lo = math.Max(hi, x.High), math.Min(lo, x.Low)
		}
		k := cs[len(cs)-1]
		return fire(c, next, fmt.Sprintf("%d-candle breakout", m), func() bool { return k.Close > hi }, func() bool { return k.Close < lo })
	}, func(d *bt.Decl) {
		d.Pure()
		b.count(d)
		if x := b.Default(); !math.IsNaN(x) {
			d.Bars(tf, int(x)+1)
		}
	})
}

// CloseBeyond fires long when the step close is above upper, short when
// it is below lower (a channel of your own, e.g. a daily range).
func CloseBeyond(upper, lower any) bt.Link {
	u, l := num(upper), num(lower)
	return trigger("close_beyond", func(c *bt.Context, next bt.Next) bt.Signal {
		x, hi, lo := c.Close(c.Step()), u.Value(c), l.Value(c)
		if math.IsNaN(x) || math.IsNaN(hi) || math.IsNaN(lo) {
			return block("no %s or %s yet", u, l)
		}
		return fire(c, next, "close beyond "+u.name+"/"+l.name, func() bool { return x > hi }, func() bool { return x < lo })
	}, u, l)
}

// Impulse fires long when tf's close is at least dist above the close
// bars candles before and the candle closed up; short is the mirror. dist
// is a value, e.g. ATR("H1", 14).Times(3).
func Impulse(tf string, bars, dist any) bt.Link {
	b, dv := num(bars), num(dist)
	return bt.Define("impulse", bt.RoleTrigger, func(c *bt.Context, next bt.Next) bt.Signal {
		n, ok := countOf(c, b)
		cs := c.Candles(c.TF(tf))
		need := dv.Value(c)
		if !ok || len(cs) < n+1 || math.IsNaN(need) || need <= 0 {
			return block("not enough %s candles", tf)
		}
		k := cs[len(cs)-1]
		move := k.Close - cs[len(cs)-1-n].Close
		return fire(c, next, "impulse", func() bool { return move >= need && k.Close > k.Open }, func() bool { return -move >= need && k.Close < k.Open })
	}, func(d *bt.Decl) {
		d.Pure()
		b.count(d)
		dv.Setup(d)
		if x := b.Default(); !math.IsNaN(x) {
			d.Bars(tf, int(x)+1)
		}
	})
}

// Cross fires long on the candle where tf's EMA(fast) crosses above
// EMA(slow) (it was at or below on the previous tf candle), short on the
// cross below.
func Cross(tf string, fast, slow any) bt.Link {
	f, s := EMA(tf, fast), EMA(tf, slow)
	pf, ps := Prev(f, tf), Prev(s, tf)
	return trigger("cross", func(c *bt.Context, next bt.Next) bt.Signal {
		a, b, pa, pb := f.Value(c), s.Value(c), pf.Value(c), ps.Value(c)
		if math.IsNaN(a + b + pa + pb) {
			return block("no %s emas yet", tf)
		}
		return fire(c, next, "ema cross", func() bool { return pa <= pb && a > b }, func() bool { return pa >= pb && a < b })
	}, pf, ps)
}

// CloseCross fires long when tf's previous close was at or below level
// and this close is above it (level read now), short on the cross down.
func CloseCross(tf string, level any) bt.Link {
	lv := num(level)
	return trigger("close_cross", func(c *bt.Context, next bt.Next) bt.Signal {
		cs := c.Candles(c.TF(tf))
		x := lv.Value(c)
		if len(cs) < 2 || math.IsNaN(x) {
			return block("no %s yet", lv)
		}
		k, before := cs[len(cs)-1], cs[len(cs)-2]
		return fire(c, next, "close across "+lv.name, func() bool { return before.Close <= x && k.Close > x },
			func() bool { return before.Close >= x && k.Close < x })
	}, lv)
}

// Custom is a trigger of your own: f returns the side it fires on, or ""
// for none. It is not run on blocked candles (it may keep state).
func Custom(name string, f func(c *bt.Context) bt.Side) bt.Link {
	return bt.Define(name, bt.RoleTrigger, func(c *bt.Context, next bt.Next) bt.Signal {
		s := f(c)
		return fire(c, next, name, func() bool { return s == bt.Long }, func() bool { return s == bt.Short })
	}, nil)
}
