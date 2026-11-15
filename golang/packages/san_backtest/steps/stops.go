package steps

import (
	"math"

	bt "github.com/wargasipil/san_backtest"
)

// The stop and target steps set c.Stop and c.Target from c.Entry on Dir's
// side (written once, mirrored for shorts), then call next. They need Dir
// set (put them after the direction steps and triggers).

func ready(c *bt.Context) (sign float64, why string) {
	switch {
	case c.Dir == "":
		return 0, "no direction for the stop"
	case math.IsNaN(c.Entry) || c.Entry <= 0:
		return 0, "no entry price"
	case c.Dir == bt.Short:
		return -1, ""
	}
	return 1, ""
}

func badDist(x float64) bool { return math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 }

// StopDist puts the stop dist below the entry (above for a short).
func StopDist(dist any) bt.Link {
	dv := num(dist)
	return bt.Define("stop_dist", bt.RoleStop, func(c *bt.Context, next bt.Next) bt.Signal {
		return setStop(c, next, dv)
	}, dv.Setup)
}

func setStop(c *bt.Context, next bt.Next, dv V) bt.Signal {
	if _, why := ready(c); why != "" {
		return block("%s", why)
	}
	if d := dv.Value(c); badDist(d) || !c.SetStop(d) {
		return block("no stop distance (%s)", dv)
	}
	c.Explain("stop %.5g", c.Stop)
	return next(c)
}

// ATRStop puts the stop x ATR(tf, n) from the entry.
func ATRStop(tf string, n, x any) bt.Link {
	dv := ATR(tf, n).Times(x)
	return bt.Define("atr_stop", bt.RoleStop, func(c *bt.Context, next bt.Next) bt.Signal {
		return setStop(c, next, dv)
	}, dv.Setup)
}

// StopAt puts the stop at level, but at least min from the entry (for a
// long: the lower of level and entry - min). min may be 0.
func StopAt(level, min any) bt.Link {
	lv, mv := num(level), num(min)
	return bt.Define("stop_at", bt.RoleStop, func(c *bt.Context, next bt.Next) bt.Signal {
		sign, why := ready(c)
		if why != "" {
			return block("%s", why)
		}
		l, m := lv.Value(c), mv.Value(c)
		if math.IsNaN(l) || math.IsNaN(m) {
			return block("no %s yet", lv)
		}
		if sign > 0 {
			c.Stop = math.Min(l, c.Entry-m)
		} else {
			c.Stop = math.Max(l, c.Entry+m)
		}
		c.Explain("stop %.5g", c.Stop)
		return next(c)
	}, func(d *bt.Decl) { lv.Setup(d); mv.Setup(d) })
}

// SwingStop puts the stop beyond the swing: buffer below the lowest low of
// tf's last bars candles for a long (above the highest high for a short),
// and at least min from the entry. buffer and min are distances, e.g.
// ATR("H1", 14).Times(0.5) and ATR("H1", 14).
func SwingStop(tf string, bars, buffer, min any) bt.Link {
	lo, hi := SwingLow(tf, bars), SwingHigh(tf, bars)
	bv, mv := num(buffer), num(min)
	return bt.Define("swing_stop", bt.RoleStop, func(c *bt.Context, next bt.Next) bt.Signal {
		sign, why := ready(c)
		if why != "" {
			return block("%s", why)
		}
		b, m := bv.Value(c), mv.Value(c)
		var d, sw float64
		if sign > 0 {
			sw = lo.Value(c)
			d = math.Max(c.Entry-(sw-b), m)
		} else {
			sw = hi.Value(c)
			d = math.Max(sw+b-c.Entry, m)
		}
		if math.IsNaN(sw) || math.IsNaN(b) || badDist(d) || !c.SetStop(d) {
			return block("no swing stop yet")
		}
		c.Explain("swing %.5g, stop %.5g", sw, c.Stop)
		return next(c)
	}, func(d *bt.Decl) { lo.Setup(d); bv.Setup(d); mv.Setup(d) })
}

// MaxStop stops the chain when the stop is more than x from the entry
// (a price distance, e.g. ATR("D1", 14).Times(1.5)).
func MaxStop(x any) bt.Link {
	lim := num(x)
	return bt.Define("max_stop", bt.RoleFilter, func(c *bt.Context, next bt.Next) bt.Signal {
		l := lim.Value(c)
		d := c.Risk()
		if math.IsNaN(l) || math.IsNaN(d) {
			return block("no stop or no limit (%s)", lim)
		}
		if d > l {
			return block("stop %.5g away > %.5g (%s)", d, l, lim)
		}
		return next(c)
	}, lim.Setup)
}

// RR puts the target r times the stop distance from the entry.
func RR(r any) bt.Link {
	rv := num(r)
	return bt.Define("rr", bt.RoleTarget, func(c *bt.Context, next bt.Next) bt.Signal {
		sign, why := ready(c)
		if why != "" {
			return block("%s", why)
		}
		x := rv.Value(c)
		risk := c.Risk()
		if badDist(x) || badDist(risk) {
			return block("no stop for the target")
		}
		c.Target = c.Entry + sign*x*risk
		c.Explain("target %.5g (%gR)", c.Target, x)
		return next(c)
	}, rv.Setup)
}

// TargetDist puts the target dist from the entry.
func TargetDist(dist any) bt.Link {
	dv := num(dist)
	return bt.Define("target_dist", bt.RoleTarget, func(c *bt.Context, next bt.Next) bt.Signal {
		return setTarget(c, next, dv)
	}, dv.Setup)
}

func setTarget(c *bt.Context, next bt.Next, dv V) bt.Signal {
	sign, why := ready(c)
	if why != "" {
		return block("%s", why)
	}
	d := dv.Value(c)
	if badDist(d) {
		return block("no target distance (%s)", dv)
	}
	c.Target = c.Entry + sign*d
	c.Explain("target %.5g", c.Target)
	return next(c)
}

// ATRTarget puts the target x ATR(tf, n) from the entry.
func ATRTarget(tf string, n, x any) bt.Link {
	dv := ATR(tf, n).Times(x)
	return bt.Define("atr_target", bt.RoleTarget, func(c *bt.Context, next bt.Next) bt.Signal {
		return setTarget(c, next, dv)
	}, dv.Setup)
}

// TargetAt puts the target at level (rule #1 still checks its side).
func TargetAt(level any) bt.Link {
	lv := num(level)
	return bt.Define("target_at", bt.RoleTarget, func(c *bt.Context, next bt.Next) bt.Signal {
		x := lv.Value(c)
		if math.IsNaN(x) {
			return block("no %s yet", lv)
		}
		c.Target = x
		c.Explain("target %.5g", x)
		return next(c)
	}, lv.Setup)
}

// RequireSLTP is rule #1 as a step (bt.RequireSLTP). Build always adds it
// as the innermost step; use it earlier to also check what the steps in
// between change on the way back.
func RequireSLTP() bt.Link { return bt.RequireSLTP() }
