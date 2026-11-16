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

// MultiPosition is optional: MaxOpen is how many positions the strategy
// may hold at once on its leg (a chain with steps.MaxOpen implements it).
// Leg.MaxOpen (Config.MaxOpen) overrides it; without either a leg holds
// one position at a time, as before.
type MultiPosition interface {
	MaxOpen() int
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
	// Pending orders and several positions (see BuyLimit, CloseID, Do).
	ActBuyLimit
	ActSellLimit
	ActBuyStop
	ActSellStop
	ActCancel
	ActCancelAll
	ActCloseAll
	ActDo
)

func (a Action) String() string {
	names := [...]string{"wait", "buy", "sell", "close", "move stop", "buy limit", "sell limit", "buy stop", "sell stop", "cancel", "cancel all",
		"close all", "do"}
	if a < 0 || int(a) >= len(names) {
		return fmt.Sprintf("action %d", int(a))
	}
	return names[a]
}

// Signal is a strategy's answer for one step. Prices, not distances: the
// lot comes from the risk % and the stop.
type Signal struct {
	Action Action
	Stop   float64 // Buy/Sell and orders: the stop price; MoveStop: the new stop
	Target float64 // Buy/Sell: the target price, 0 = none; orders: the target price (required)
	Reason string  // Close: why (shown in the trade list)

	// Price is a pending order's price: the limit, or the stop that
	// triggers it.
	Price float64
	// ID is the position (CloseID, MoveStopID) or the pending order
	// (Cancel) the signal is for; 0 = every open position of the leg.
	ID int
	// Bars: a pending order expires after this many step candles without
	// a fill (Expire); 0 = never.
	Bars int
	// Expires: a pending order expires at this time (Until); zero = never.
	Expires time.Time
	// Label is a pending order's tag (Tag), carried to its position and
	// trade.
	Label string
	// Batch is Do's signals, run in order.
	Batch []Signal
}

// Wait does nothing this step.
var Wait = Signal{}

// Because sets the signal's reason, e.g. bt.Wait.Because("spread %.2f too
// wide", sp) for a chain step that stops the chain. A Wait's reason goes
// to the decision log as a note; a chain keeps it in its path instead.
func (s Signal) Because(format string, args ...any) Signal {
	s.Reason = fmt.Sprintf(format, args...)
	return s
}

// Buy opens a long at the next candle's open (at the ask).
func Buy(stop, target float64) Signal { return Signal{Action: ActBuy, Stop: stop, Target: target} }

// Sell opens a short at the next candle's open (at the bid).
func Sell(stop, target float64) Signal { return Signal{Action: ActSell, Stop: stop, Target: target} }

// Close closes the leg's open positions at the next candle's open: the
// position, while one is open (as it always did), or every one of them.
// Pending orders stay; CloseAll also cancels them.
func Close(reason string) Signal { return Signal{Action: ActClose, Reason: reason} }

// MoveStop moves the open position's stop at the next candle's open. It
// only moves the stop closer to the price (up for a long, down for a
// short); a move the other way is ignored. With several positions open it
// applies to each one it moves closer (a long's stop only moves up, a
// short's only down); MoveStopID moves one.
func MoveStop(price float64) Signal { return Signal{Action: ActMoveStop, Stop: price} }

// Several positions and pending orders (the user, 2026-11-15: "1b, 2a,
// 3b": limit and stop orders, the worst case when a fill and the stop are
// both possible in one candle, several pending orders with a
// multi-position engine).

// BuyLimit places a pending buy at price or lower (below the market),
// with its stop and target. It can fill from the next candle on and fills
// once; Expire, Until and Tag modify it.
func BuyLimit(price, stop, target float64) Signal {
	return Signal{Action: ActBuyLimit, Price: price, Stop: stop, Target: target}
}

// SellLimit places a pending sell at price or higher (above the market).
func SellLimit(price, stop, target float64) Signal {
	return Signal{Action: ActSellLimit, Price: price, Stop: stop, Target: target}
}

// BuyStop places a pending buy that triggers when the ask reaches price
// (above the market: a breakout).
func BuyStop(price, stop, target float64) Signal {
	return Signal{Action: ActBuyStop, Price: price, Stop: stop, Target: target}
}

// SellStop places a pending sell that triggers when the bid falls to
// price (below the market).
func SellStop(price, stop, target float64) Signal {
	return Signal{Action: ActSellStop, Price: price, Stop: stop, Target: target}
}

// Cancel cancels the pending order id.
func Cancel(id int) Signal { return Signal{Action: ActCancel, ID: id} }

// CancelAll cancels the leg's pending orders.
func CancelAll(reason string) Signal { return Signal{Action: ActCancelAll, Reason: reason} }

// CloseID closes the open position id at the next candle's open.
func CloseID(id int, reason string) Signal { return Signal{Action: ActClose, ID: id, Reason: reason} }

// CloseAll closes every open position of the leg and cancels its pending
// orders (flat, e.g. at a session's end).
func CloseAll(reason string) Signal { return Signal{Action: ActCloseAll, Reason: reason} }

// MoveStopID moves the stop of the open position id, only closer to the
// price, as MoveStop.
func MoveStopID(id int, price float64) Signal {
	return Signal{Action: ActMoveStop, ID: id, Stop: price}
}

// Do runs several signals in one step, in order, at the next candle's
// open (e.g. close one position, move another's stop and place an order).
// Waits are dropped and nested Do's flattened; one signal is returned as
// it is, none as Wait.
func Do(sigs ...Signal) Signal {
	var flat []Signal
	for _, s := range sigs {
		flat = append(flat, s.List()...)
	}
	switch len(flat) {
	case 0:
		return Wait
	case 1:
		return flat[0]
	}
	return Signal{Action: ActDo, Batch: flat}
}

// List is the signal as a list of single actions: Do's batch, nothing for
// Wait, else the signal itself.
func (s Signal) List() []Signal {
	switch s.Action {
	case ActWait:
		return nil
	case ActDo:
		var out []Signal
		for _, x := range s.Batch {
			out = append(out, x.List()...)
		}
		return out
	}
	return []Signal{s}
}

// Expire makes a pending order expire after n step candles without a
// fill (it can fill in the n candles after the one that placed it).
func (s Signal) Expire(n int) Signal {
	s.Bars = n
	return s
}

// Until makes a pending order expire at t: it does not fill in a candle
// that opens at or after t.
func (s Signal) Until(t time.Time) Signal {
	s.Expires = t
	return s
}

// Tag labels a pending order; the label goes with its position into the
// trade list.
func (s Signal) Tag(label string) Signal {
	s.Label = label
	return s
}

// IsEntry is true for a signal that may open a position: Buy, Sell, a
// pending order, or a Do holding one.
func (s Signal) IsEntry() bool {
	for _, x := range s.List() {
		if x.Action.entry() {
			return true
		}
	}
	return false
}

// IsOrder is true for a pending order (limit or stop).
func (a Action) IsOrder() bool {
	return a == ActBuyLimit || a == ActSellLimit || a == ActBuyStop || a == ActSellStop
}

func (a Action) entry() bool { return a == ActBuy || a == ActSell || a.IsOrder() }

// Side is the side an entry opens: Long for Buy and the buy orders, Short
// for Sell and the sell orders, "" otherwise.
func (a Action) Side() Side {
	switch a {
	case ActBuy, ActBuyLimit, ActBuyStop:
		return Long
	case ActSell, ActSellLimit, ActSellStop:
		return Short
	}
	return ""
}

// Registry maps names to strategy makers, for the CLI.
type Registry struct {
	mu sync.Mutex
	m  map[string]func() Strategy
	// plugins are the loaded plugin files by absolute path (Load).
	plugins map[string]*loadedPlugin
	opts    PluginOptions
}

// NewRegistry makes an empty registry.
func NewRegistry() *Registry {
	return &Registry{m: map[string]func() Strategy{}, plugins: map[string]*loadedPlugin{}}
}

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
	mk, ok := r.m[name]
	if !ok {
		defer r.mu.Unlock()
		return nil, fmt.Errorf("no strategy %q (have: %s)", name, strings.Join(r.namesLocked(), ", "))
	}
	r.mu.Unlock()
	s := mk()
	if err := pluginErr(s); err != nil {
		return nil, err
	}
	return s, nil
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
