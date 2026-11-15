// Package steps holds the built-in steps for san_backtest chain strategies
// (bt.Chain): filters, direction, triggers, stop and target, and the
// position management that goes inside ManageOpen, plus the small value
// expressions they take (steps.EMA("H4", 20), SwingLow(...).Minus(...)).
//
// Numbers a step takes are "any": a float64 or int constant, or a value
// (V) such as P("rr", 2) for a tunable number or ATR("H1", 14).Times(0.5).
// A wrong type panics when the chain is written (a programmer error).
//
// Every step waits when its data is missing (NaN indicators, too few
// candles): a missing value never becomes a signal.
package steps

import (
	"fmt"
	"math"
	"time"

	bt "github.com/wargasipil/san_backtest"
)

// V is a value expression: a number read from the Context on each candle.
type V struct {
	name  string
	f     func(c *bt.Context) float64
	def   func() float64 // the value without a Context (constants, params), NaN otherwise
	needs []func(d *bt.Decl)
	param *param // set when V is a bare P
}

type param struct {
	def      *bt.ParamDef
	explicit bool // Whole or Grid chose the grid
}

// Value is v at this candle (NaN when its data is missing).
func (v V) Value(c *bt.Context) float64 {
	if v.f == nil {
		return math.NaN()
	}
	return v.f(c)
}

// Default is v without a Context: a constant, a param's default and
// arithmetic on them; NaN for anything that reads candles.
func (v V) Default() float64 {
	if v.def == nil {
		return math.NaN()
	}
	return v.def()
}

func (v V) String() string { return v.name }

// Setup declares what v needs (its params and bars) for a step that is not
// a built-in one.
func (v V) Setup(d *bt.Decl) {
	for _, f := range v.needs {
		f(d)
	}
}

// count declares v used as a count (a period, a number of bars): a bare
// param then gets the whole-number grid unless Whole or Grid chose one.
func (v V) count(d *bt.Decl) {
	if v.param != nil && !v.param.explicit {
		v.param.def.Grid = bt.DefaultGrid
	}
	v.Setup(d)
}

// num turns a step argument into a V.
func num(x any) V {
	switch t := x.(type) {
	case V:
		return t
	case *V:
		return *t
	case float64:
		return Const(t)
	case float32:
		return Const(float64(t))
	case int:
		return Const(float64(t))
	case int64:
		return Const(float64(t))
	case nil:
		return Const(0)
	}
	panic(fmt.Sprintf("steps: want a number or a steps value, got %T", x))
}

// Const is a fixed number.
func Const(x float64) V {
	return V{name: fmt.Sprint(x), f: func(*bt.Context) float64 { return x }, def: func() float64 { return x }}
}

// P is a tunable number named name with default def: the chain's Params()
// lists it, so Config.Params, --param and walk-forward change it. Pass the
// same P value to every step that shares the number; two P calls with one
// name are an error from Build. Walk-forward tries it as a fraction
// (FineGrid), or as whole numbers when a step uses it as a count.
func P(name string, def float64) V {
	p := &param{def: &bt.ParamDef{Name: name, Default: def, Grid: bt.FineGrid}}
	return V{name: name, param: p,
		f:     func(c *bt.Context) float64 { return c.Param(name) },
		def:   func() float64 { return p.def.Default },
		needs: []func(d *bt.Decl){func(d *bt.Decl) { d.Param(p.def) }}}
}

// Whole makes walk-forward try a P like a hand-written strategy's number:
// bt.DefaultGrid, whole numbers when the value is whole.
func (v V) Whole() V {
	if v.param != nil {
		v.param.def.Grid, v.param.explicit = bt.DefaultGrid, true
	}
	return v
}

// Grid makes walk-forward try exactly these values for a P.
func (v V) Grid(values ...float64) V {
	if v.param != nil {
		vs := append([]float64{}, values...)
		v.param.def.Grid, v.param.explicit = func(float64) []float64 { return vs }, true
	}
	return v
}

func join(name string, a, b V, f func(x, y float64) float64) V {
	return V{name: name,
		f: func(c *bt.Context) float64 { return f(a.Value(c), b.Value(c)) },
		def: func() float64 {
			return f(a.Default(), b.Default())
		},
		needs: append(append([]func(d *bt.Decl){}, a.needs...), b.needs...)}
}

// Plus, Minus, Times, Div, Max and Min combine values (or numbers).
func (v V) Plus(x any) V {
	w := num(x)
	return join(v.name+"+"+w.name, v, w, func(a, b float64) float64 { return a + b })
}
func (v V) Minus(x any) V {
	w := num(x)
	return join(v.name+"-"+w.name, v, w, func(a, b float64) float64 { return a - b })
}
func (v V) Times(x any) V {
	w := num(x)
	return join(v.name+"*"+w.name, v, w, func(a, b float64) float64 { return a * b })
}
func (v V) Div(x any) V {
	w := num(x)
	return join(v.name+"/"+w.name, v, w, func(a, b float64) float64 { return a / b })
}
func (v V) Max(x any) V {
	w := num(x)
	return join("max("+v.name+","+w.name+")", v, w, math.Max)
}
func (v V) Min(x any) V {
	w := num(x)
	return join("min("+v.name+","+w.name+")", v, w, math.Min)
}

// Func is a value from your own code, for steps that take values. needs
// declares its params and bars (may be nil).
func Func(name string, f func(c *bt.Context) float64, needs func(d *bt.Decl)) V {
	v := V{name: name, f: f}
	if needs != nil {
		v.needs = []func(d *bt.Decl){needs}
	}
	return v
}

// indicator makes a value that reads tf with a period.
func indicator(kind, tf string, period any, read func(c *bt.Context, tf string, n int) float64) V {
	p := num(period)
	return V{name: fmt.Sprintf("%s(%s,%s)", kind, tf, p.name),
		f: func(c *bt.Context) float64 {
			n, ok := countOf(c, p)
			if !ok {
				return math.NaN()
			}
			return read(c, c.TF(tf), n)
		},
		needs: []func(d *bt.Decl){func(d *bt.Decl) {
			p.count(d)
			if n := p.Default(); !math.IsNaN(n) {
				d.Bars(tf, int(n)+1)
			}
		}}}
}

// countOf reads v as a count: false when it is NaN or under 1.
func countOf(c *bt.Context, v V) (int, bool) {
	x := v.Value(c)
	if math.IsNaN(x) || math.IsInf(x, 0) || x < 1 {
		return 0, false
	}
	return int(x), true
}

// EMA, SMA, RSI and ATR are the Context's indicators on tf ("" = the step
// timeframe) with period n.
func EMA(tf string, n any) V {
	return indicator("ema", tf, n, func(c *bt.Context, tf string, n int) float64 { return c.EMA(tf, n) })
}
func SMA(tf string, n any) V {
	return indicator("sma", tf, n, func(c *bt.Context, tf string, n int) float64 { return c.SMA(tf, n) })
}
func RSI(tf string, n any) V {
	return indicator("rsi", tf, n, func(c *bt.Context, tf string, n int) float64 { return c.RSI(tf, n) })
}
func ATR(tf string, n any) V {
	return indicator("atr", tf, n, func(c *bt.Context, tf string, n int) float64 { return c.ATR(tf, n) })
}

// MACDHist is the MACD histogram on tf (e.g. 12, 26, 9).
func MACDHist(tf string, fast, slow, signal int) V {
	return V{name: fmt.Sprintf("macd_hist(%s,%d,%d,%d)", tf, fast, slow, signal),
		f: func(c *bt.Context) float64 {
			_, _, h := c.MACD(c.TF(tf), fast, slow, signal)
			return h
		},
		needs: []func(d *bt.Decl){func(d *bt.Decl) { d.Bars(tf, slow+signal) }}}
}

func field(kind, tf string, f func(k bt.Candle) float64) V {
	return V{name: kind + "(" + tf + ")", f: func(c *bt.Context) float64 {
		k, ok := c.Last(c.TF(tf))
		if !ok {
			return math.NaN()
		}
		return f(k)
	}}
}

// Close, Open, High and Low are tf's newest closed candle (bid).
func Close(tf string) V { return field("close", tf, func(k bt.Candle) float64 { return k.Close }) }
func Open(tf string) V  { return field("open", tf, func(k bt.Candle) float64 { return k.Open }) }
func High(tf string) V  { return field("high", tf, func(k bt.Candle) float64 { return k.High }) }
func Low(tf string) V   { return field("low", tf, func(k bt.Candle) float64 { return k.Low }) }

// Entry is the chain's planned entry price (c.Entry).
func Entry() V {
	return V{name: "entry", f: func(c *bt.Context) float64 { return c.Entry }}
}

func extreme(kind, tf string, bars any, low bool) V {
	b := num(bars)
	return V{name: fmt.Sprintf("%s(%s,%s)", kind, tf, b.name),
		f: func(c *bt.Context) float64 {
			n, ok := countOf(c, b)
			cs := c.Candles(c.TF(tf))
			if !ok || len(cs) < n {
				return math.NaN()
			}
			x := math.Inf(1)
			if !low {
				x = math.Inf(-1)
			}
			for _, k := range cs[len(cs)-n:] {
				if low {
					x = math.Min(x, k.Low)
				} else {
					x = math.Max(x, k.High)
				}
			}
			return x
		},
		needs: []func(d *bt.Decl){func(d *bt.Decl) {
			b.count(d)
			if n := b.Default(); !math.IsNaN(n) {
				d.Bars(tf, int(n))
			}
		}}}
}

// SwingLow is the lowest low of tf's last bars candles (the newest
// included); SwingHigh the highest high.
func SwingLow(tf string, bars any) V  { return extreme("swing_low", tf, bars, true) }
func SwingHigh(tf string, bars any) V { return extreme("swing_high", tf, bars, false) }

// Mirror is long while the chain's Dir is Long (or not set) and short
// while it is Short: a level written once for both sides, e.g.
// Mirror(SwingLow("H1", 12), SwingHigh("H1", 12)).
func Mirror(long, short any) V {
	l, s := num(long), num(short)
	return V{name: "mirror(" + l.name + "," + s.name + ")",
		f: func(c *bt.Context) float64 {
			if c.Dir == bt.Short {
				return s.Value(c)
			}
			return l.Value(c)
		},
		needs: append(append([]func(d *bt.Decl){}, l.needs...), s.needs...)}
}

// Prev is v's value on tf's previous candle: v is read on every candle (a
// step that never runs while a position is open still gets it right) and
// kept per candle of tf. NaN until two candles of tf were seen.
func Prev(v any, tf string) V {
	w := num(v)
	var seen time.Time
	cur, prev := math.NaN(), math.NaN()
	key := new(int)
	return V{name: "prev(" + w.name + ")",
		f: func(*bt.Context) float64 { return prev },
		needs: append(append([]func(d *bt.Decl){}, w.needs...), func(d *bt.Decl) {
			if !d.Once(key) {
				return
			}
			d.OnStart(func() { seen, cur, prev = time.Time{}, math.NaN(), math.NaN() })
			d.EveryCandle(func(c *bt.Context) {
				k, ok := c.Last(c.TF(tf))
				if !ok || k.CloseTime.Equal(seen) {
					return
				}
				seen = k.CloseTime
				prev, cur = cur, w.Value(c)
			})
		})}
}
