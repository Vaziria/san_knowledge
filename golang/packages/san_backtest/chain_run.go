package san_backtest

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// chainStrategy is what Build makes.
type chainStrategy struct {
	name   string
	tfs    []string
	warmup int
	bars   int
	params map[string]*ParamDef
	order  []string
	top    *list
	dirs   *list
	starts []func()
	every  []func(c *Context)
	count  int
	st     chainState
	// maxOpen is what steps.MaxOpen declared (MultiPosition); 0 = one
	// position at a time, as before.
	maxOpen int
}

// MaxOpen is the positions the chain may hold at once (steps.MaxOpen); 0
// when it declares none (one at a time).
func (s *chainStrategy) MaxOpen() int { return s.maxOpen }

// Held is what a chain knows about the open position: tracked from the
// first candle it saw it (see Context.Held).
type Held struct {
	OpenTime time.Time
	Bars     int     // step candles seen with the position open, this one included
	Best     float64 // highest high (long) or lowest low (short) since the entry, bid prices
	InitStop float64 // the stop when the chain first saw the position
	R        float64 // |entry - InitStop|
}

// chainState is one chain's state during a candle (and the held position
// across candles). The Context points at it while the chain runs.
type chainState struct {
	tfs    []string      // the chain's declared timeframes, for Context.TF
	held   map[int]*Held // by position ID
	record bool
	quiet  int // > 0 while a step runs on its own (probe, Directions)
	trace  []*traceEntry
	stack  []*traceEntry
	// event: something happened worth logging (a trigger fired, rule #1
	// blocked an entry)
	event bool
	would []string
	// the stop distance SetStop was given, while Stop is still what it set
	stopAt, risk float64
}

type traceEntry struct {
	name    string
	details []string
	outcome string
}

func (s *chainStrategy) Name() string         { return s.name }
func (s *chainStrategy) Timeframes() []string { return append([]string{}, s.tfs...) }
func (s *chainStrategy) Warmup() int          { return s.warmup }

// Params are every declared param's default.
func (s *chainStrategy) Params() map[string]float64 {
	out := make(map[string]float64, len(s.params))
	for k, p := range s.params {
		out[k] = p.Default
	}
	return out
}

// Grid is the param's own walk-forward grid, nil for DefaultGrid.
func (s *chainStrategy) Grid(name string, centre float64) []float64 {
	if p, ok := s.params[name]; ok && p.Grid != nil {
		return p.Grid(centre)
	}
	return nil
}

// OnStart resets the chain and every step's state.
func (s *chainStrategy) OnStart(StartInfo) {
	s.st = chainState{tfs: s.tfs, held: map[int]*Held{}}
	for _, f := range s.starts {
		f()
	}
}

// OnCandle runs the chain once.
func (s *chainStrategy) OnCandle(c *Context) Signal {
	st := &s.st
	st.tfs, st.record = s.tfs, c.record
	st.trace, st.stack, st.event, st.would, st.quiet = st.trace[:0], st.stack[:0], false, nil, 0
	c.chain = st
	c.Dir, c.Stop, c.Target = "", math.NaN(), math.NaN()
	c.Entry = c.Close(c.step)
	st.stopAt, st.risk = math.NaN(), 0
	if st.held == nil {
		st.held = map[int]*Held{}
	}
	if len(c.positions) > 0 {
		if k, ok := c.Last(c.step); ok {
			open := map[int]bool{}
			for i := range c.positions {
				p := &c.positions[i]
				open[p.ID] = true
				h := st.held[p.ID]
				if h == nil {
					h = &Held{OpenTime: p.OpenTime, InitStop: p.Stop, Best: p.Entry, R: math.Abs(p.Entry - p.Stop)}
					st.held[p.ID] = h
				}
				h.Bars++
				if p.Side == Long {
					h.Best = math.Max(h.Best, k.High)
				} else {
					h.Best = math.Min(h.Best, k.Low)
				}
			}
			for id := range st.held {
				if !open[id] {
					delete(st.held, id)
				}
			}
		}
	} else if len(st.held) > 0 {
		clear(st.held)
	}
	for _, f := range s.every {
		f(c)
	}
	sig := s.run(c, s.top, 0, func(c *Context) Signal {
		sig := s.terminal(c)
		if sig.Action == ActWait && st.quiet == 0 {
			st.trace = append(st.trace, &traceEntry{name: "end", outcome: sig.Reason})
		}
		return sig
	})
	if sig.IsEntry() {
		var why string
		if sig, why = keepValidEntries(c, sig); why != "" {
			st.trace = append(st.trace, &traceEntry{name: "rule #1", outcome: "blocked: " + why})
		}
	}
	if st.record && (sig.Action != ActWait || st.event || len(st.would) > 0) {
		c.Note("%s", st.path())
	}
	if sig.Action == ActWait {
		sig = Wait // a quiet candle's reasons stay out of the log
	}
	return sig
}

// SetStop puts c.Stop dist from c.Entry on Dir's side (below for a long,
// above for a short) and remembers dist as the planned risk, so Risk (and
// RR) use the distance itself rather than entry - (entry - dist). False
// when Dir or Entry is not set or dist is not a positive price distance.
func (c *Context) SetStop(dist float64) bool {
	if math.IsNaN(dist) || math.IsInf(dist, 0) || dist <= 0 || math.IsNaN(c.Entry) {
		return false
	}
	switch c.Dir {
	case Long:
		c.Stop = c.Entry - dist
	case Short:
		c.Stop = c.Entry + dist
	default:
		return false
	}
	if c.chain != nil {
		c.chain.stopAt, c.chain.risk = c.Stop, dist
	}
	return true
}

// Risk is the planned trade's 1R: the distance from Entry to Stop (the one
// SetStop was given while Stop is unchanged since). NaN without a stop.
func (c *Context) Risk() float64 {
	if c.chain != nil && c.chain.risk > 0 && c.Stop == c.chain.stopAt {
		return c.chain.risk
	}
	return math.Abs(c.Entry - c.Stop)
}

// terminal is the end of the chain: an entry on Dir with Stop and Target,
// while the open positions and pending orders leave room under MaxOpen
// (one at a time unless steps.MaxOpen or Config.MaxOpen allow more).
func (s *chainStrategy) terminal(c *Context) Signal {
	if lim, n := c.MaxOpen(), len(c.positions)+len(c.orders); n >= lim {
		switch {
		case lim > 1:
			return Wait.Because("%d open and %d pending, max %d", len(c.positions), len(c.orders), lim)
		case len(c.positions) > 0:
			return Wait.Because("a position is open")
		}
		return Wait.Because("an order is pending")
	}
	switch c.Dir {
	case Long:
		return Buy(c.Stop, c.Target)
	case Short:
		return Sell(c.Stop, c.Target)
	}
	return Wait.Because("no direction")
}

// run runs l's nodes from i, then end.
func (s *chainStrategy) run(c *Context, l *list, i int, end Next) Signal {
	if i == len(l.nodes) {
		return end(c)
	}
	n := l.nodes[i]
	st := &s.st
	var e *traceEntry
	if st.quiet == 0 {
		name := n.name
		if l.prefix != "" {
			name = l.prefix + "/" + name
		}
		e = &traceEntry{name: name}
		st.trace = append(st.trace, e)
		st.stack = append(st.stack, e)
	}
	called := false
	var inner Signal
	sig := n.link.Run(c, func(c *Context) Signal {
		called = true
		if e != nil {
			st.stack = st.stack[:len(st.stack)-1]
		}
		inner = s.run(c, l, i+1, end)
		if e != nil {
			st.stack = append(st.stack, e)
		}
		return inner
	})
	if e == nil {
		return sig
	}
	st.stack = st.stack[:len(st.stack)-1]
	switch {
	case !called && sig.Action == ActWait:
		e.outcome = "blocked"
		if sig.Reason != "" {
			e.outcome += ": " + sig.Reason
		}
		if l == s.top && st.record {
			s.probe(c, l.nodes[i+1:])
		}
	case !called:
		e.outcome = describe(sig)
	case !sameSignal(sig, inner) && sig.Action == ActWait:
		e.outcome = "blocked on the way back"
		if sig.Reason != "" {
			e.outcome += ": " + sig.Reason
		}
	case !sameSignal(sig, inner):
		e.outcome = "changed to " + describe(sig)
	}
	if called && n.role == RoleTrigger {
		st.event = true
	}
	return sig
}

// probe runs the pure triggers after a blocking step on their own, so a
// candle where one would have fired is logged with what blocked it.
func (s *chainStrategy) probe(c *Context, rest []*node) {
	st := &s.st
	for _, n := range rest {
		if n.role != RoleTrigger || !n.pure {
			continue
		}
		dir, entry, stop, target := c.Dir, c.Entry, c.Stop, c.Target
		st.quiet++
		var side Side
		n.link.Run(c, func(c *Context) Signal { side = c.Dir; return Wait })
		st.quiet--
		c.Dir, c.Entry, c.Stop, c.Target = dir, entry, stop, target
		if side != "" {
			st.would = append(st.would, fmt.Sprintf("%s would fire %s", n.name, side))
		}
	}
}

// direction runs l on its own and reports the Dir it ends with.
func (s *chainStrategy) direction(c *Context, l *list) Side {
	if l == nil || len(l.nodes) == 0 {
		return ""
	}
	st := &s.st
	dir, entry, stop, target := c.Dir, c.Entry, c.Stop, c.Target
	c.Dir = ""
	st.quiet++
	var got Side
	s.run(c, l, 0, func(c *Context) Signal { got = c.Dir; return Wait })
	st.quiet--
	c.Dir, c.Entry, c.Stop, c.Target = dir, entry, stop, target
	return got
}

func (st *chainState) path() string {
	var parts []string
	for _, e := range st.trace {
		p := e.name
		if len(e.details) > 0 {
			p += " (" + strings.Join(e.details, ", ") + ")"
		}
		if e.outcome != "" {
			p += ": " + e.outcome
		}
		parts = append(parts, p)
	}
	out := strings.Join(parts, " > ")
	if len(st.would) > 0 {
		out += "; " + strings.Join(st.would, "; ")
	}
	return out
}

// sameSignal compares signals with NaN prices equal.
func sameSignal(a, b Signal) bool {
	eq := func(x, y float64) bool { return x == y || (math.IsNaN(x) && math.IsNaN(y)) }
	if a.Action != b.Action || a.Reason != b.Reason || !eq(a.Stop, b.Stop) || !eq(a.Target, b.Target) || !eq(a.Price, b.Price) || a.ID != b.ID ||
		a.Bars != b.Bars || !a.Expires.Equal(b.Expires) || a.Label != b.Label || len(a.Batch) != len(b.Batch) {
		return false
	}
	for i := range a.Batch {
		if !sameSignal(a.Batch[i], b.Batch[i]) {
			return false
		}
	}
	return true
}

func describe(s Signal) string {
	on := ""
	if s.ID != 0 {
		on = fmt.Sprintf(" #%d", s.ID)
	}
	switch s.Action {
	case ActBuy, ActSell:
		return fmt.Sprintf("%s, stop %.5g, target %.5g", s.Action, s.Stop, s.Target)
	case ActBuyLimit, ActSellLimit, ActBuyStop, ActSellStop:
		out := fmt.Sprintf("%s at %.5g, stop %.5g, target %.5g", s.Action, s.Price, s.Stop, s.Target)
		if s.Bars > 0 {
			out += fmt.Sprintf(", expires after %d candles", s.Bars)
		}
		return out
	case ActMoveStop:
		return fmt.Sprintf("move stop%s to %.5g", on, s.Stop)
	case ActClose, ActCloseAll, ActCancel, ActCancelAll:
		out := s.Action.String() + on
		if s.Reason != "" {
			out += ": " + s.Reason
		}
		return out
	case ActDo:
		var parts []string
		for _, x := range s.Batch {
			parts = append(parts, describe(x))
		}
		return strings.Join(parts, " + ")
	}
	if s.Reason != "" {
		return "wait: " + s.Reason
	}
	return "wait"
}

// keepValidEntries drops the entries of sig that break rule #1 (a Do keeps
// its other actions); why is the first broken one's reason, "" when all
// hold. A single entry that breaks it becomes Wait.
func keepValidEntries(c *Context, sig Signal) (Signal, string) {
	list := sig.List()
	var keep []Signal
	why := ""
	for _, x := range list {
		if x.Action.entry() {
			if w := checkSLTP(c, x); w != "" {
				if why == "" {
					why = w
				}
				continue
			}
		}
		keep = append(keep, x)
	}
	if len(keep) == len(list) {
		return sig, ""
	}
	return Do(keep...), why
}

// checkSLTP is rule #1 for an entry: a stop-loss and a take-profit, both
// real prices, on the right sides of c.Entry (of a pending order's price).
// "" when it holds.
func checkSLTP(c *Context, sig Signal) string {
	bad := func(x float64) bool { return math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 }
	e := c.Entry
	if sig.Action.IsOrder() {
		e = sig.Price
	}
	side := sig.Action.Side()
	switch {
	case bad(sig.Stop):
		return "no stop-loss"
	case bad(sig.Target):
		return "no take-profit"
	case bad(e):
		return "no entry price"
	case side == Long && !(sig.Stop < e && e < sig.Target):
		return fmt.Sprintf("%s with stop %.5g and target %.5g around entry %.5g", sig.Action, sig.Stop, sig.Target, e)
	case side == Short && !(sig.Target < e && e < sig.Stop):
		return fmt.Sprintf("%s with stop %.5g and target %.5g around entry %.5g", sig.Action, sig.Stop, sig.Target, e)
	}
	return ""
}

const requireName = "require_sltp"

// RequireSLTP is rule #1 as a step: an entry that comes back without a
// stop-loss and a take-profit on the right sides of Entry becomes Wait
// with the reason. Build always adds it as the innermost step (unless the
// chain already ends with it); placing it earlier also checks what the
// steps between change on the way back.
func RequireSLTP() Link {
	return Define(requireName, RoleCheck, func(c *Context, next Next) Signal {
		sig := next(c)
		if sig.IsEntry() {
			out, why := keepValidEntries(c, sig)
			if why != "" {
				if c.chain != nil {
					c.chain.event = true
				}
				if out.Action == ActWait {
					return Wait.Because("rule #1: %s", why)
				}
			}
			return out
		}
		return sig
	}, nil)
}
