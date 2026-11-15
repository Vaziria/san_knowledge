package san_backtest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Live runs a strategy one closed candle at a time, as the candles come:
// paper trading on live prices (San Tower's Live Trading, the user,
// 2026-11-15: "realtime trading menu in tower", first stage "Monitor +
// Signals (paper trading)"). It never sends an order anywhere: it is the
// backtest engine fed step by step. The fills, spread, swap, the worst
// case on a fill candle, pending orders, several positions, MaxOpen, the
// decision log and rule #1 are the same code as Run, so a Live over a past
// range gives exactly Run's trades and stats over that range (live_test.go
// and san_research's live_test.go prove it).
//
// A candle's open is only known once the candle has closed (Live reads
// closed candles only), so a signal at one close fills at the next
// candle's open price, as in a backtest, but Live learns it one candle
// later. The candle still forming is never fed to the strategy.
//
// The strategy's own state (indicators, chain closures) cannot be saved,
// so a Live is not serialized: to continue one after a restart, make a new
// Live with the same Config (the same From) and it replays from From to
// now, which gives the same run as if it had never stopped.
type Live struct {
	mu  sync.Mutex
	cfg LiveConfig
	p   Portfolio
	src DataSource // the raw source, for the candles after the history
	ld  *loaded
	d   *legData
	s   *sim // nil until the first step candle with its warm-up behind it
	l   *legRun

	// through is, per timeframe, the time up to which every closed candle
	// has been fed.
	through map[string]time.Time
	next    int // the next step candle to run
	s0      int // the first step candle at or after From
	done    bool
	ended   bool // Finish closed what was open
	err     error
	// What Update has handed out so far.
	nDec, nTrades, nMissed, nEq int
}

// LiveConfig is a Live's settings: a Config (Strategy, Instrument, Data,
// From, Balance, Risk, Leverage, Costs, MaxOpen, Params...; Walk is not
// allowed) and the clock.
type LiveConfig struct {
	Config
	// Now is the clock (default: Data's Now when it is Cacheable, such as
	// Exness's clock; else time.Now).
	Now func() time.Time
	// Grace is how long a candle of a higher timeframe that should have
	// closed is waited for before the step candles go on without it (a
	// broker can publish a candle a few seconds late). Default 2 minutes.
	Grace time.Duration
}

// DefaultLiveGrace is LiveConfig.Grace's default.
const DefaultLiveGrace = 2 * time.Minute

// Update is what one Advance or Step did.
type Update struct {
	Candles   []Candle // the step candles run, oldest first
	Decisions []Decision
	Trades    []Trade // closed in this update, in close order
	Missed    []Missed
	Equity    []EquityPoint
}

// LiveState is where a Live is now.
type LiveState struct {
	Started   bool      // the first step candle has run
	Done      bool      // To was reached, or Finish was called
	Candles   int       // step candles run
	First     time.Time // the first step candle's open
	LastClose time.Time // the last step candle's close
	// NextClose is when the next step candle closes (it may come later,
	// e.g. over a weekend).
	NextClose time.Time
	Balance   float64
	Equity    float64
	Margin    float64
	Positions []Position
	Orders    []Order
}

// ErrLiveDone is returned by Advance and Step after Finish, or once To
// was reached.
var ErrLiveDone = errors.New("san_backtest: the live run is done")

// NewLive loads the warm-up history (and any candles from From to now,
// which it runs at once: a catch-up) and returns the Live, ready for
// Advance. Data's history goes through the Cache like Run's; the candles
// after it are asked from Data itself, so a candle published late is
// never taken as missing for good.
func NewLive(ctx context.Context, cfg LiveConfig) (*Live, error) {
	if cfg.Walk != nil {
		return nil, &FieldError{Field: "LiveConfig.Walk", Problem: "walk-forward has no meaning for a live run"}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
		if c, ok := cfg.Data.(Cacheable); ok {
			cfg.Now = c.Now
		}
	}
	if cfg.Grace <= 0 {
		cfg.Grace = DefaultLiveGrace
	}
	openEnded := cfg.To.IsZero()
	p := cfg.portfolio().withDefaults()
	if openEnded {
		p.To = time.Time{}
	}
	now := cfg.Now().UTC()
	hist := DataSource(p.Data)
	if c, ok := p.Data.(Cacheable); ok && !strings.EqualFold(p.Cache, "off") {
		hist = NewCache(c, p.Cache)
	}
	d, err := newLegData(ctx, hist, p, 0, p.Legs[0])
	if err != nil {
		return nil, err
	}
	// The history ends an hour back, so the cache only keeps candles that
	// are surely published; the last hour comes from Data in Advance.
	histTo := now.Add(-time.Hour)
	if !p.To.IsZero() && p.To.Before(histTo) {
		histTo = p.To
	}
	if err := d.fetch(ctx, hist, p.From, histTo); err != nil {
		return nil, err
	}
	lv := &Live{cfg: cfg, p: p, src: p.Data, d: d, ld: &loaded{p: p, legs: []*legData{d}}, through: map[string]time.Time{}}
	for _, tf := range d.tfs {
		lv.through[tf] = histTo
	}
	step := d.candles[d.tfs[0]]
	lv.s0 = sort.Search(len(step), func(k int) bool { return !step[k].OpenTime.Before(p.From) })
	lv.next = lv.s0
	if _, err := lv.advance(ctx, now); err != nil {
		return nil, err
	}
	return lv, nil
}

// Advance asks Data for the candles that closed since the last call, on
// every timeframe, and runs the step candles that are ready, oldest first.
// Missed candles (a sleeping computer, a dropped network) are fetched in
// order: Advance always asks from the last candle it has.
func (lv *Live) Advance(ctx context.Context) (Update, error) {
	return lv.advance(ctx, lv.cfg.Now().UTC())
}

func (lv *Live) advance(ctx context.Context, now time.Time) (Update, error) {
	lv.mu.Lock()
	if err := lv.usable(); err != nil {
		lv.mu.Unlock()
		return Update{}, err
	}
	d := lv.d
	type got struct {
		tf      string
		cs      []Candle
		through time.Time
	}
	var all []got
	for _, tf := range d.tfs {
		have := d.candles[tf]
		from := lv.through[tf]
		if n := len(have); n > 0 {
			from = have[n-1].CloseTime
		}
		all = append(all, got{tf: tf, through: lv.through[tf]})
		if !from.Before(now) {
			continue
		}
		lv.mu.Unlock()
		cs, err := lv.src.Candles(ctx, Query{Instrument: d.inst.Name, Timeframe: tf, From: from, To: now})
		lv.mu.Lock()
		if err != nil {
			lv.mu.Unlock()
			return Update{}, fmt.Errorf("san_backtest: live %s %s: %w", d.inst.Name, tf, err)
		}
		var keep []Candle
		for _, c := range cs {
			if c.CloseTime.After(now) {
				continue // still forming
			}
			keep = append(keep, c)
		}
		all[len(all)-1].cs = keep
		all[len(all)-1].through = lv.coverage(tf, keep, now)
	}
	for _, g := range all {
		if err := lv.feedLocked(g.tf, g.through, g.cs); err != nil {
			lv.mu.Unlock()
			return Update{}, err
		}
	}
	lv.mu.Unlock()
	return lv.Step()
}

// coverage is how far tf's candles are complete after a fetch at now:
// now, unless the candle that should have closed last is not there while
// the market was open and Grace has not passed (then: up to the newest
// candle there).
func (lv *Live) coverage(tf string, fresh []Candle, now time.Time) time.Time {
	have := lv.d.candles[tf]
	var last Candle
	switch {
	case len(fresh) > 0:
		last = fresh[len(fresh)-1]
	case len(have) > 0:
		last = have[len(have)-1]
	default:
		return now
	}
	anchor := last.OpenTime
	if len(have) > 0 {
		anchor = have[0].OpenTime
	}
	forming := bucket(tf, anchor, now)
	var expected time.Time // the open of the candle that closed last
	if tf == "MN" {
		expected = forming.AddDate(0, -1, 0)
	} else {
		expected = forming.Add(-timeframeDur[tf])
	}
	if expected.After(last.OpenTime) && lv.d.inst.Open(expected) && now.Sub(closeTime(tf, expected)) < lv.cfg.Grace {
		return last.CloseTime
	}
	return now
}

// Feed gives Live closed candles of tf, oldest first, and says that every
// closed candle of tf up to through is now in (through may be later than
// the last candle: a weekend has none). Candles it already has are
// skipped. Step then runs what is ready. Advance does this from Data;
// Feed is for callers with candles of their own.
func (lv *Live) Feed(tf string, through time.Time, cs ...Candle) error {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	if err := lv.usable(); err != nil {
		return err
	}
	return lv.feedLocked(tf, through, cs)
}

func (lv *Live) feedLocked(tf string, through time.Time, cs []Candle) error {
	d := lv.d
	have, ok := d.candles[tf]
	if !ok {
		return fmt.Errorf("san_backtest: live %s: timeframe %s is not one of %s", d.inst.Name, tf, strings.Join(d.tfs, ", "))
	}
	var add []Candle
	for _, c := range cs {
		if c.CloseTime.IsZero() {
			c.CloseTime = closeTime(tf, c.OpenTime)
		}
		last := time.Time{}
		if n := len(have) + len(add); n > 0 {
			if len(add) > 0 {
				last = add[len(add)-1].OpenTime
			} else {
				last = have[len(have)-1].OpenTime
			}
		}
		if !last.IsZero() && !c.OpenTime.After(last) {
			continue
		}
		add = append(add, c)
	}
	if len(add) > 0 {
		d.candles[tf] = append(have, d.adjust(add)...)
		if lv.l != nil {
			lv.l.frames[tf].all = d.candles[tf]
		}
	}
	if through.After(lv.through[tf]) {
		lv.through[tf] = through
	}
	return nil
}

func (lv *Live) usable() error {
	if lv.err != nil {
		return lv.err
	}
	if lv.done {
		return ErrLiveDone
	}
	return nil
}

// Step runs every step candle fed whose higher timeframes are complete up
// to its close, oldest first. A strategy that panics ends the Live with
// the error.
func (lv *Live) Step() (up Update, err error) {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	if err := lv.usable(); err != nil {
		return Update{}, err
	}
	defer func() {
		if r := recover(); r != nil {
			lv.err = fmt.Errorf("san_backtest: strategy panicked: %v", r)
			up, err = Update{}, lv.err
		}
	}()
	d := lv.d
	step := d.candles[d.tfs[0]]
	for lv.next < len(step) {
		k := lv.next
		c := step[k]
		if !lv.p.To.IsZero() && c.CloseTime.After(lv.p.To) {
			break
		}
		if !lv.complete(c.CloseTime) {
			break
		}
		if lv.s == nil && c.OpenTime.Before(lv.p.From) {
			lv.next++ // warm-up only
			lv.s0 = lv.next
			continue
		}
		if lv.s == nil {
			if !d.ready(k) {
				lv.next++
				continue
			}
			if err := lv.begin(k); err != nil {
				lv.err = err
				return Update{}, err
			}
		}
		if !lv.p.To.IsZero() && c.CloseTime.Equal(lv.p.To) {
			lv.l.lastIdx = k // the last candle: no signal after it, as Run
		}
		lv.s.step(event{t: c.OpenTime, idx: k})
		lv.s.step(event{t: c.CloseTime, close: true, idx: k})
		up.Candles = append(up.Candles, c)
		lv.next++
		if k == lv.l.lastIdx {
			lv.done = true
			break
		}
	}
	lv.collect(&up)
	return up, nil
}

// complete says whether every timeframe is fed up to t.
func (lv *Live) complete(t time.Time) bool {
	for _, tf := range lv.d.tfs[1:] {
		if lv.through[tf].Before(t) {
			return false
		}
	}
	return true
}

// begin starts the simulation at step candle k, the first with its
// warm-up behind it, as Run's start.
func (lv *Live) begin(k int) error {
	d := lv.d
	if err := d.lateStart(lv.p, lv.s0, k); err != nil {
		return err
	}
	d.start = k
	from := d.candles[d.tfs[0]][k].OpenTime
	lv.ld.from = from
	lv.s = lv.ld.newSim(lv.p.Balance, from, lv.p.To, nil, true, []int{k})
	lv.s.live = true
	lv.l = lv.s.legs[0]
	lv.l.ranIdx = k - 1
	return nil
}

// collect hands out what happened since the last Update.
func (lv *Live) collect(up *Update) {
	s := lv.s
	if s == nil {
		return
	}
	up.Decisions = append([]Decision(nil), s.decisions[lv.nDec:]...)
	up.Trades = append([]Trade(nil), s.trades[lv.nTrades:]...)
	up.Missed = append([]Missed(nil), s.missed[lv.nMissed:]...)
	up.Equity = append([]EquityPoint(nil), s.equity[min(lv.nEq, len(s.equity)):]...)
	lv.nDec, lv.nTrades, lv.nMissed, lv.nEq = len(s.decisions), len(s.trades), len(s.missed), len(s.equity)
}

// Result is the run so far: closed trades, missed signals, the equity
// curve, the decision log and the step candles run, with stats over them;
// open positions stay open (Finish closes them). nil before the first step
// candle ran.
func (lv *Live) Result() *Result {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	return lv.resultLocked()
}

func (lv *Live) resultLocked() *Result {
	s := lv.s
	if s == nil {
		return nil
	}
	to := lv.l.d.candles[lv.d.tfs[0]][max(lv.l.ranIdx, lv.l.firstIdx)].CloseTime
	res := s.result(lv.p.Balance, lv.ld.from, to, s.params)
	res.Missed = append([]Missed(nil), res.Missed...)
	res.Equity = append([]EquityPoint(nil), res.Equity...)
	res.Decisions = append([]Decision(nil), res.Decisions...)
	for i := range res.Candles {
		res.Candles[i].Candles = append([]Candle(nil), res.Candles[i].Candles...)
	}
	res.Range.RequestedTo = lv.p.To
	res.Run.Kind = KindRun
	return res
}

// Finish ends the Live as Run ends: pending orders are cancelled and open
// positions closed at the last price ("closed at end"). It returns the
// final Result (nil when no step candle ran); Advance and Step refuse
// after it.
func (lv *Live) Finish() *Result {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	if lv.s == nil {
		lv.done = true
		return nil
	}
	if !lv.ended {
		lv.s.finish()
		lv.ended = true
	}
	lv.done = true
	lv.l.lastIdx = lv.l.ranIdx
	return lv.resultLocked()
}

// State is where the Live is now.
func (lv *Live) State() LiveState {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	st := LiveState{Done: lv.done}
	d := lv.d
	step := d.candles[d.tfs[0]]
	if lv.s == nil {
		st.Balance, st.Equity = lv.p.Balance, lv.p.Balance
		st.NextClose = lv.nextClose()
		return st
	}
	s, l := lv.s, lv.l
	st.Started = true
	st.Candles = l.ranIdx - l.firstIdx + 1
	st.First = step[l.firstIdx].OpenTime
	if l.ranIdx >= l.firstIdx {
		st.LastClose = step[l.ranIdx].CloseTime
	}
	st.NextClose = lv.nextClose()
	st.Balance, st.Equity, st.Margin = s.balance, s.equityNow(), s.usedMargin()
	for _, p := range l.positions {
		st.Positions = append(st.Positions, Position{Side: p.side, Entry: p.entry, Stop: p.stop, Target: p.target, Lots: p.lots, OpenTime: p.openTime,
			Risk: p.risk, Profit: s.floatingOf(l, p) - p.commission, ID: p.id, OrderID: p.orderID, Type: p.kind, Tag: p.tag})
	}
	for _, o := range l.orders {
		left := 0
		if o.bars > 0 {
			left = o.bars - o.seen
		}
		st.Orders = append(st.Orders, Order{ID: o.id, Side: o.side, Type: o.kind, Price: o.price, Stop: o.stop, Target: o.target, Placed: o.placed,
			BarsLeft: left, Expires: o.until, Tag: o.tag})
	}
	return st
}

// nextClose is the close of the step candle after the newest one fed.
func (lv *Live) nextClose() time.Time {
	d := lv.d
	tf := d.tfs[0]
	step := d.candles[tf]
	if len(step) == 0 {
		return time.Time{}
	}
	last := step[len(step)-1]
	return closeTime(tf, last.CloseTime)
}

// Instrument is the instrument as loaded (with Costs.Commission applied).
func (lv *Live) Instrument() Instrument { return lv.d.inst }

// Timeframes are the timeframes in force, the step first.
func (lv *Live) Timeframes() []string { return append([]string(nil), lv.d.tfs...) }

// Candles are tf's candles Live holds (warm-up included) from from on.
func (lv *Live) Candles(tf string, from time.Time) []Candle {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	cs := lv.d.candles[tf]
	i := sort.Search(len(cs), func(k int) bool { return !cs[k].OpenTime.Before(from) })
	return append([]Candle(nil), cs[i:]...)
}

// Restat recomputes Stats and Legs from Trades, Equity and Missed, with
// Run.Balance as the start and Range as the period: for a Result put
// together again from saved parts (San Tower keeps a paper session's
// trades, equity and missed signals in its database).
func (r *Result) Restat() {
	mid := r.Range.From.Add(r.Range.To.Sub(r.Range.From) / 2)
	skipped := map[string]int{}
	for _, m := range r.Missed {
		skipped[m.Reason]++
	}
	sort.SliceStable(r.Trades, func(i, j int) bool { return r.Trades[i].CloseTime.Before(r.Trades[j].CloseTime) })
	r.Stats = computeStats(r.Trades, r.Equity, r.Run.Balance, mid, skipped)
	r.Legs = r.Legs[:0]
	for _, li := range r.Run.Legs {
		var trades []Trade
		sk := map[string]int{}
		for _, t := range r.Trades {
			if t.Leg == li.Leg {
				trades = append(trades, t)
			}
		}
		for _, m := range r.Missed {
			if m.Leg == li.Leg {
				sk[m.Reason]++
			}
		}
		lr := LegResult{Leg: li.Leg, Strategy: li.Strategy, Instrument: li.Instrument, Stats: computeStats(trades, curve(trades, r.Run.Balance), r.Run.Balance, mid, sk)}
		if r.Stats.NetProfit != 0 && !math.IsNaN(r.Stats.NetProfit) {
			lr.Contribution = 100 * lr.Stats.NetProfit / r.Stats.NetProfit
		}
		r.Legs = append(r.Legs, lr)
	}
}
