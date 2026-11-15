package san_backtest

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// ChainBuilder collects a chain's steps; Build makes the Strategy.
type ChainBuilder struct {
	name    string
	tfs     []string
	links   []Link
	warmup  int
	warmSet bool
}

// Chain starts a chain strategy called name. The timeframes work as in
// Strategy.Timeframes: the first is the one the chain steps on.
func Chain(name string, timeframes ...string) *ChainBuilder {
	return &ChainBuilder{name: name, tfs: append([]string{}, timeframes...)}
}

// Use appends steps, in the order they run.
func (b *ChainBuilder) Use(links ...Link) *ChainBuilder {
	b.links = append(b.links, links...)
	return b
}

// Warmup overrides the warm-up worked out from the steps' bars.
func (b *ChainBuilder) Warmup(n int) *ChainBuilder {
	b.warmup, b.warmSet = n, true
	return b
}

// MustBuild is Build that panics on an error, for package-level
// registrations whose chain is fixed in code.
func (b *ChainBuilder) MustBuild() Strategy {
	s, err := b.Build()
	if err != nil {
		panic(err)
	}
	return s
}

// node is one set-up link.
type node struct {
	link Link
	name string
	role Role
	pure bool
}

// list is a run of nodes: the chain itself, or a Sub (ManageOpen's).
type list struct {
	prefix string
	nodes  []*node
}

// Decl is what a step's Setup declares through.
type Decl struct {
	b *build
	n *node
}

type build struct {
	s     *chainStrategy
	seen  map[any]*node
	once  map[any]bool
	errs  []error
	depth int
}

// Name names the step in the decision log (default: "step <n>").
func (d *Decl) Name(name string) { d.n.name = name }

// Role says what kind of step this is.
func (d *Decl) Role(r Role) { d.n.role = r }

// Pure marks a trigger as keeping no state between candles, so the chain
// may run it on its own to log that it would have fired on a candle an
// earlier step blocked.
func (d *Decl) Pure() { d.n.pure = true }

// Param declares a tunable number. The same *ParamDef may be declared by
// several steps (one variable passed to each); two different ones with one
// name are an error from Build.
func (d *Decl) Param(p *ParamDef) {
	s := d.b.s
	if old, ok := s.params[p.Name]; ok {
		if old != p {
			d.b.errs = append(d.b.errs, fmt.Errorf("san_backtest: chain %s: param %q is declared twice (declare it once and pass the same value to each step)", s.name, p.Name))
		}
		return
	}
	if p.Name == "" {
		d.b.errs = append(d.b.errs, fmt.Errorf("san_backtest: chain %s: a param without a name", s.name))
		return
	}
	s.params[p.Name] = p
	s.order = append(s.order, p.Name)
}

// Bars says the step reads n closed candles of tf (an indicator period, a
// lookback). The chain's warm-up is the largest n plus a margin.
func (d *Decl) Bars(tf string, n int) {
	s := d.b.s
	if tf != "" && !contains(s.tfs, tf) {
		d.b.errs = append(d.b.errs, fmt.Errorf("san_backtest: chain %s: step %s reads %s, which is not in its timeframes %s", s.name, d.n.name, tf, strings.Join(s.tfs, ", ")))
	}
	if n > s.bars {
		s.bars = n
	}
}

// OnStart runs f before every run (walk-forward trials included): reset
// the step's state there.
func (d *Decl) OnStart(f func()) { d.b.s.starts = append(d.b.s.starts, f) }

// EveryCandle runs f at the start of every candle, before the first step,
// whether or not the chain reaches the step (e.g. to remember an
// indicator's previous value while a position is open).
func (d *Decl) EveryCandle(f func(c *Context)) { d.b.s.every = append(d.b.s.every, f) }

// Once is true the first time key is passed in this Build: a value shared
// by several steps registers its EveryCandle hook only once.
func (d *Decl) Once(key any) bool {
	if d.b.once[key] {
		return false
	}
	d.b.once[key] = true
	return true
}

// MaxOpen declares that the chain may hold n positions at once (and
// enter while fewer are open and pending): the strategy's MaxOpen(), which
// Config.MaxOpen overrides. steps.MaxOpen calls it.
func (d *Decl) MaxOpen(n int) { d.b.s.maxOpen = max(n, 0) }

// Fail makes Build return err (a bad argument found at setup).
func (d *Decl) Fail(err error) { d.b.errs = append(d.b.errs, err) }

// Sub sets up links as a nested chain and returns its runner; the nested
// chain ends in Wait. ManageOpen runs its steps this way. Their path shows
// in the decision log as "<prefix>/<step>".
func (d *Decl) Sub(prefix string, links ...Link) Next {
	l := &list{prefix: prefix}
	d.b.depth++
	for _, k := range links {
		if n := d.b.setup(k); n != nil {
			l.nodes = append(l.nodes, n)
		}
	}
	d.b.depth--
	s := d.b.s
	return func(c *Context) Signal { return s.run(c, l, 0, func(*Context) Signal { return Wait }) }
}

// Directions returns a function that runs direction steps on their own and
// reports the side they agree on ("" when they block). With no links it
// uses the chain's own top-level direction steps.
func (d *Decl) Directions(links ...Link) func(c *Context) Side {
	s := d.b.s
	var l *list
	if len(links) > 0 {
		l = &list{prefix: "dir"}
		for _, k := range links {
			if n := d.b.setup(k); n != nil {
				l.nodes = append(l.nodes, n)
			}
		}
	}
	return func(c *Context) Side {
		ls := l
		if ls == nil {
			ls = s.dirs
		}
		return s.direction(c, ls)
	}
}

// setup makes the node for k, once per link value.
func (b *build) setup(k Link) *node {
	if k == nil {
		b.errs = append(b.errs, fmt.Errorf("san_backtest: chain %s: a nil step", b.s.name))
		return nil
	}
	var id any
	if v := reflect.ValueOf(k); v.Kind() == reflect.Pointer {
		id = k
		if n, ok := b.seen[id]; ok {
			return n
		}
	}
	n := &node{link: k}
	b.s.count++
	n.name = fmt.Sprintf("step %d", b.s.count)
	if id != nil {
		b.seen[id] = n
	}
	if st, ok := k.(Setup); ok {
		st.Setup(&Decl{b: b, n: n})
	}
	return n
}

// Build checks the chain and makes the Strategy: Params() lists every
// declared param, Warmup() is the largest declared bars plus a margin
// (max(5, 20%)) unless Warmup(n) set it, and rule #1 is enforced.
func (b *ChainBuilder) Build() (Strategy, error) {
	s := &chainStrategy{name: b.name, tfs: append([]string{}, b.tfs...), params: map[string]*ParamDef{}}
	bd := &build{s: s, seen: map[any]*node{}, once: map[any]bool{}}
	if b.name == "" {
		bd.errs = append(bd.errs, errors.New("san_backtest: a chain needs a name"))
	}
	if len(b.tfs) == 0 {
		bd.errs = append(bd.errs, fmt.Errorf("san_backtest: chain %s needs at least one timeframe", b.name))
	}
	top := &list{}
	for i, k := range b.links {
		n := bd.setup(k)
		if n == nil {
			continue
		}
		if n.role == RoleManage && i > 0 {
			bd.errs = append(bd.errs, fmt.Errorf("san_backtest: chain %s: ManageOpen must be the first step (it is step %d)", b.name, i+1))
		}
		top.nodes = append(top.nodes, n)
	}
	if k := len(top.nodes); k == 0 || top.nodes[k-1].name != requireName {
		top.nodes = append(top.nodes, bd.setup(RequireSLTP()))
	}
	s.top = top
	s.dirs = &list{prefix: "dir"}
	for _, n := range top.nodes {
		if n.role == RoleDirection {
			s.dirs.nodes = append(s.dirs.nodes, n)
		}
	}
	s.warmup = s.bars + max(5, s.bars/5)
	if s.bars == 0 {
		s.warmup = 1
	}
	if b.warmSet {
		s.warmup = b.warmup
	}
	if len(bd.errs) > 0 {
		return nil, errors.Join(bd.errs...)
	}
	return s, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
