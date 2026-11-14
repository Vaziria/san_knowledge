package san_backtest

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Strategy is an approach written in Go (the user, 2026-11-14: "a"). The
// same interface is meant to run live later: it sees only a Context and
// answers with a Signal.
type Strategy interface {
	Name() string
	// Timeframes: the first is the one the strategy steps on (OnCandle runs
	// once per closed candle of it); the others are context, e.g.
	// {"M15", "H1", "H4"}. They must not be shorter than the first.
	Timeframes() []string
	// Warmup is how many closed candles each timeframe needs before the
	// first OnCandle. They are fetched before From automatically.
	Warmup() int
	OnCandle(c *Context) Signal
}

// Starter is optional: OnStart runs before every run, walk-forward trials
// included, so reset any state kept in the strategy there.
type Starter interface {
	OnStart(info StartInfo)
}

// Tunable is optional: Params are the strategy's numbers and their default
// values. Config.Params overrides them, walk-forward tunes them, and the
// strategy reads the values in force with Context.Param or StartInfo.Params.
type Tunable interface {
	Params() map[string]float64
}

// StartInfo is what OnStart gets.
type StartInfo struct {
	Instrument InstrumentInfo
	Timeframes []string
	From, To   time.Time
	Balance    float64
	Currency   string
	Params     map[string]float64
}

// Action is what a Signal asks for.
type Action int

const (
	ActWait Action = iota
	ActBuy
	ActSell
	ActClose
	ActMoveStop
)

func (a Action) String() string {
	return [...]string{"wait", "buy", "sell", "close", "move stop"}[a]
}

// Signal is a strategy's answer for one step. Prices, not distances: the
// lot comes from the risk % and the stop.
type Signal struct {
	Action Action
	Stop   float64 // Buy/Sell: the stop price; MoveStop: the new stop
	Target float64 // Buy/Sell: the target price, 0 = none
	Reason string  // Close: why (shown in the trade list)
}

// Wait does nothing this step.
var Wait = Signal{}

// Buy opens a long at the next candle's open (at the ask).
func Buy(stop, target float64) Signal { return Signal{Action: ActBuy, Stop: stop, Target: target} }

// Sell opens a short at the next candle's open (at the bid).
func Sell(stop, target float64) Signal { return Signal{Action: ActSell, Stop: stop, Target: target} }

// Close closes the open position at the next candle's open.
func Close(reason string) Signal { return Signal{Action: ActClose, Reason: reason} }

// MoveStop moves the open position's stop at the next candle's open. It
// only moves the stop closer to the price (up for a long, down for a
// short); a move the other way is ignored.
func MoveStop(price float64) Signal { return Signal{Action: ActMoveStop, Stop: price} }

// Registry maps names to strategy makers, for the CLI.
type Registry struct {
	mu sync.Mutex
	m  map[string]func() Strategy
}

// NewRegistry makes an empty registry.
func NewRegistry() *Registry { return &Registry{m: map[string]func() Strategy{}} }

// Register adds a strategy maker under name (a new one each run, so state
// is never shared).
func (r *Registry) Register(name string, make func() Strategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[name] = make
}

// New makes the strategy registered under name.
func (r *Registry) New(name string) (Strategy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	mk, ok := r.m[name]
	if !ok {
		return nil, fmt.Errorf("no strategy %q (have: %s)", name, strings.Join(r.namesLocked(), ", "))
	}
	return mk(), nil
}

// Names lists the registered names, sorted.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.namesLocked()
}

func (r *Registry) namesLocked() []string {
	var out []string
	for n := range r.m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
