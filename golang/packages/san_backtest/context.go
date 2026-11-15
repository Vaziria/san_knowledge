package san_backtest

import (
	"fmt"
	"math"
	"time"
)

// Side of a position.
type Side string

const (
	Long  Side = "buy"
	Short Side = "sell"
)

// Position is the strategy's open position, as the strategy sees it.
type Position struct {
	Side     Side
	Entry    float64 // fill price
	Stop     float64 // current stop
	Target   float64 // 0 = none
	Lots     float64
	OpenTime time.Time
	Risk     float64 // money at risk when it opened (account currency)
	Profit   float64 // floating profit now, after commission and swap so far
}

// Account is the account's money at this step. In a portfolio it is the
// shared account.
type Account struct {
	Currency string
	Balance  float64
	Equity   float64
	Margin   float64 // used margin
	Open     int     // open positions, all legs
}

// InstrumentInfo is what a strategy may want to know about its
// instrument.
type InstrumentInfo struct {
	Name         string
	Base, Quote  string
	Digits       int
	Point        float64
	ContractSize float64
	LotMin       float64
	LotMax       float64
	LotStep      float64
	Spread       float64 // ask - bid at the current candle's close, or the Costs.Spread override
}

// Context is what OnCandle sees at one step: closed candles only, up to
// Time(). It holds no reference to the simulator, so a live runner can
// fill one the same way.
type Context struct {
	time    time.Time
	step    string
	tfs     []string
	frames  map[string]*series
	anchors map[string]time.Time
	gap     time.Duration
	pos     *Position
	account Account
	inst    InstrumentInfo
	params  map[string]float64
	// notes are this step's Note calls; nil record drops them (walk-forward
	// trials record nothing).
	notes  []string
	record bool

	// The chain fields (chain strategies, see Chain): the steps of a chain
	// share them, and the chain resets them at the start of every candle.
	// Other strategies may ignore them.

	// Dir is the side the chain is looking at: Long, Short, or "" while no
	// direction step or trigger has set it.
	Dir Side
	// Entry is the planned entry price: the step candle's close (NaN when
	// there is none). A step may change it; rule #1 and RR read it.
	Entry float64
	// Stop and Target are the planned stop-loss and take-profit prices,
	// NaN until a step sets them.
	Stop, Target float64

	chain *chainState
}

// Time is the close time of the current step candle, UTC.
func (c *Context) Time() time.Time { return c.time }

// Local is Time in WIB (UTC+7).
func (c *Context) Local() time.Time { return c.time.In(WIB) }

// Step is the timeframe the strategy steps on.
func (c *Context) Step() string { return c.step }

// Timeframes are the timeframes in force, step first: the strategy's own,
// or Config.Timeframes (CLI --tf) when set. Strategies that read them from
// here instead of naming "M15" keep working when --tf changes them.
func (c *Context) Timeframes() []string { return append([]string{}, c.tfs...) }

func (c *Context) frame(tf string) *series {
	s, ok := c.frames[tf]
	if !ok {
		panic(fmt.Sprintf("san_backtest: timeframe %q is not in the strategy's Timeframes()", tf))
	}
	return s
}

// Candles are tf's closed candles, oldest first, newest last: on a higher
// timeframe only those whose close time is <= Time(). Don't modify them.
func (c *Context) Candles(tf string) []Candle { return c.frame(tf).closed() }

// Last is tf's newest closed candle.
func (c *Context) Last(tf string) (Candle, bool) {
	cs := c.Candles(tf)
	if len(cs) == 0 {
		return Candle{}, false
	}
	return cs[len(cs)-1], true
}

func (c *Context) lastField(tf string, f func(Candle) float64) float64 {
	if k, ok := c.Last(tf); ok {
		return f(k)
	}
	return math.NaN()
}

// Close, High and Low are the newest closed candle's bid prices (NaN when
// there is none yet).
func (c *Context) Close(tf string) float64 {
	return c.lastField(tf, func(k Candle) float64 { return k.Close })
}
func (c *Context) High(tf string) float64 {
	return c.lastField(tf, func(k Candle) float64 { return k.High })
}
func (c *Context) Low(tf string) float64 {
	return c.lastField(tf, func(k Candle) float64 { return k.Low })
}

// Forming is the higher-timeframe candle still going on, built only from
// the closed step candles inside it so far (its CloseTime is when it will
// close). False on the step timeframe, and when no step candle of it has
// closed yet.
func (c *Context) Forming(tf string) (Candle, bool) {
	c.frame(tf)
	if tf == c.step {
		return Candle{}, false
	}
	start := bucket(tf, c.anchors[tf], c.time.Add(-time.Nanosecond))
	end := closeTime(tf, start)
	if !end.After(c.time) {
		return Candle{}, false
	}
	steps := c.frame(c.step).closed()
	i := len(steps)
	for i > 0 && !steps[i-1].OpenTime.Before(start) {
		i--
	}
	in := steps[i:]
	if len(in) == 0 {
		return Candle{}, false
	}
	f := Candle{OpenTime: start, CloseTime: end, Open: in[0].Open, AskOpen: in[0].AskOpen, High: in[0].High, Low: in[0].Low,
		AskHigh: in[0].AskHigh, AskLow: in[0].AskLow}
	for _, k := range in {
		f.High, f.Low = math.Max(f.High, k.High), math.Min(f.Low, k.Low)
		f.AskHigh, f.AskLow = math.Max(f.AskHigh, k.AskHigh), math.Min(f.AskLow, k.AskLow)
		f.Volume += k.Volume
	}
	last := in[len(in)-1]
	f.Close, f.AskClose = last.Close, last.AskClose
	return f, true
}

// Session is the trading session at Time().
func (c *Context) Session() Session { return SessionAt(c.time.Add(-time.Nanosecond)) }

// Gap is the time between the previous step candle's close and this one's
// open: 0 normally, about two days after a weekend, more after a missing
// stretch. Gaps are never filled with made-up candles.
func (c *Context) Gap() time.Duration { return c.gap }

// Position is the open position, nil when flat. It is a copy.
func (c *Context) Position() *Position {
	if c.pos == nil {
		return nil
	}
	p := *c.pos
	return &p
}

// Note explains the current step in the decision log (decisions.jsonl),
// e.g. c.Note("ema20 above ema50, rsi %.0f", r). A note goes with the
// step's signal as its reason, or stands alone when the step waits. Notes
// cost nothing when the run records no decisions (walk-forward trials).
func (c *Context) Note(format string, args ...any) {
	if !c.record {
		return
	}
	c.notes = append(c.notes, fmt.Sprintf(format, args...))
}

// TF is the timeframe in force for tf, one of a chain's own timeframes:
// the chain's timeframes map by position onto Timeframes(), so a step
// written for "H4" follows a --tf override. "" is the step timeframe.
// Outside a chain (or for a name the chain does not declare) it is tf.
func (c *Context) TF(tf string) string {
	if tf == "" {
		return c.step
	}
	if c.chain != nil {
		for i, x := range c.chain.tfs {
			if x == tf && i < len(c.tfs) {
				return c.tfs[i]
			}
		}
	}
	return tf
}

// Held is what a chain has tracked about the open position: the candles
// held, the best price since the entry and the first stop (1R). Zero when
// flat or outside a chain.
func (c *Context) Held() Held {
	if c.chain == nil || c.pos == nil {
		return Held{}
	}
	return c.chain.held
}

// Explain adds detail to the running chain step's entry in the decision
// path (logged only on candles where something happened). Outside a chain
// it is Note.
func (c *Context) Explain(format string, args ...any) {
	if c.chain == nil {
		c.Note(format, args...)
		return
	}
	st := c.chain
	if !st.record || st.quiet > 0 || len(st.stack) == 0 {
		return
	}
	e := st.stack[len(st.stack)-1]
	e.details = append(e.details, fmt.Sprintf(format, args...))
}

// Account is the account at this step.
func (c *Context) Account() Account { return c.account }

// Instrument is the instrument's sizing facts and the spread now.
func (c *Context) Instrument() InstrumentInfo { return c.inst }

// Param is a strategy number in force (Params() default, Config.Params, or
// the walk-forward pick); NaN when unknown.
func (c *Context) Param(name string) float64 {
	if v, ok := c.params[name]; ok {
		return v
	}
	return math.NaN()
}

// Indicators on tf's closed candles (bid closes), computed incrementally.
// They are NaN until they have enough candles.

// EMA is the exponential moving average of the closes.
func (c *Context) EMA(tf string, period int) float64 {
	return catchUp(c.frame(tf), key("ema", period), func() *ema { return newEMA(period) }).value
}

// SMA is the simple moving average of the closes.
func (c *Context) SMA(tf string, period int) float64 {
	return catchUp(c.frame(tf), key("sma", period), func() *sma { return newSMA(period) }).value()
}

// RSI is Wilder's relative strength index.
func (c *Context) RSI(tf string, period int) float64 {
	return catchUp(c.frame(tf), key("rsi", period), func() *rsi { return newRSI(period) }).value
}

// ATR is Wilder's average true range.
func (c *Context) ATR(tf string, period int) float64 {
	return catchUp(c.frame(tf), key("atr", period), func() *atr { return newATR(period) }).value
}

// MACD is the MACD line, its signal line and the histogram (e.g. 12, 26, 9).
func (c *Context) MACD(tf string, fast, slow, signal int) (line, sig, hist float64) {
	return catchUp(c.frame(tf), key("macd", fast, slow, signal), func() *macd { return newMACD(fast, slow, signal) }).values()
}

// Bollinger is the middle band (SMA) and the bands k standard deviations
// above and below it.
func (c *Context) Bollinger(tf string, period int, k float64) (mid, upper, lower float64) {
	m := catchUp(c.frame(tf), key("sma", period), func() *sma { return newSMA(period) })
	mid, sd := m.value(), m.stddev()
	return mid, mid + k*sd, mid - k*sd
}

func (c *Context) swings(tf string, strength int) *swings {
	if strength < 1 {
		strength = 1
	}
	return catchUp(c.frame(tf), key("swing", strength), func() *swings { return &swings{strength: strength} })
}

// SwingHighs are the confirmed swing highs so far, newest last. Don't
// modify the slice.
func (c *Context) SwingHighs(tf string, strength int) []Swing { return c.swings(tf, strength).highs }

// SwingLows are the confirmed swing lows so far, newest last.
func (c *Context) SwingLows(tf string, strength int) []Swing { return c.swings(tf, strength).lws }
