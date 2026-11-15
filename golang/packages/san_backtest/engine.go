package san_backtest

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Run backtests one strategy on one instrument (a one-leg portfolio, so it
// shares RunPortfolio's engine). With Config.Walk it also runs the
// walk-forward windows (Result.Walk).
func Run(ctx context.Context, cfg Config) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	p := cfg.portfolio().withDefaults()
	ld, err := load(ctx, p)
	if err != nil {
		return nil, err
	}
	res, err := ld.run(p.Balance, ld.from, ld.to, nil, true)
	if err != nil {
		return nil, err
	}
	if cfg.Walk != nil {
		if res.Walk, err = ld.walk(*cfg.Walk); err != nil {
			return nil, err
		}
		w := *cfg.Walk
		res.Run.Walk = &w
	}
	res.Run.Kind = KindRun
	return res, nil
}

// RunPortfolio backtests several legs on one shared account.
func RunPortfolio(ctx context.Context, p Portfolio) (*Result, error) {
	p.walk = nil
	if err := p.Validate(); err != nil {
		return nil, err
	}
	p = p.withDefaults()
	ld, err := load(ctx, p)
	if err != nil {
		return nil, err
	}
	res, err := ld.run(p.Balance, ld.from, ld.to, nil, true)
	if err != nil {
		return nil, err
	}
	res.Run.Kind = KindPortfolio
	return res, nil
}

// loaded is every leg's candles and conditions, fetched once; runs and
// walk-forward trials replay it.
type loaded struct {
	p        Portfolio
	legs     []*legData
	from, to time.Time // what really runs
	notes    []string
}

type legData struct {
	i         int
	leg       Leg
	name      string
	inst      Instrument
	tfs       []string
	candles   map[string][]Candle
	start     int // first step candle that may run (warm-up satisfied)
	costs     Costs
	swapMoney *[2]float64
	report    CostReport
	params    map[string]float64 // Params() defaults with Leg.Params on top
	ccy       string
}

// conv turns quote-currency money into the account currency at price.
func (d *legData) conv(price float64) float64 {
	if d.inst.Quote == d.ccy {
		return 1
	}
	return 1 / price // base == account currency, checked at load
}

func (d *legData) margin(lots, price float64, leverage int) float64 {
	return lots * d.inst.ContractSize * price * d.conv(price) * d.inst.MarginPercent / 100 / float64(leverage)
}

func load(ctx context.Context, p Portfolio) (*loaded, error) {
	src := p.Data
	if c, ok := src.(Cacheable); ok && !strings.EqualFold(p.Cache, "off") {
		src = NewCache(c, p.Cache)
	}
	ld := &loaded{p: p}
	for i, l := range p.Legs {
		d, err := loadLeg(ctx, src, p, i, l)
		if err != nil {
			return nil, err
		}
		ld.legs = append(ld.legs, d)
	}
	for _, d := range ld.legs {
		step := d.candles[d.tfs[0]]
		from := step[d.start].OpenTime
		last := len(step) - 1
		for last >= 0 && step[last].CloseTime.After(p.To) {
			last--
		}
		if last < d.start {
			return nil, fmt.Errorf("san_backtest: %s %s: no closed candle between %s and %s", d.inst.Name, d.tfs[0], from.Format(time.RFC3339), p.To.Format(time.RFC3339))
		}
		to := step[last].CloseTime
		if ld.from.IsZero() || from.Before(ld.from) {
			ld.from = from
		}
		if to.After(ld.to) {
			ld.to = to
		}
		if p.To.Sub(to) > 4*24*time.Hour {
			ld.notes = append(ld.notes, fmt.Sprintf("%s %s history ends at %s, before To %s", d.inst.Name, d.tfs[0], to.Format(time.RFC3339), p.To.Format(time.RFC3339)))
		}
	}
	return ld, nil
}

func loadLeg(ctx context.Context, src DataSource, p Portfolio, i int, l Leg) (*legData, error) {
	d, err := newLegData(ctx, src, p, i, l)
	if err != nil {
		return nil, err
	}
	return loadCandles(ctx, src, p, d)
}

// newLegData reads a leg's instrument and settles its params and costs.
// usdStable are dollar stablecoins, as Exness names them; profit in one
// is taken 1:1 as USD (an approximation: their price drifts by a fraction
// of a percent).
var usdStable = map[string]bool{"USDT": true, "TET": true, "USDC": true}

func newLegData(ctx context.Context, src DataSource, p Portfolio, i int, l Leg) (*legData, error) {
	inst, err := src.Instrument(ctx, l.Instrument)
	if err != nil {
		return nil, fmt.Errorf("san_backtest: instrument %s: %w", l.Instrument, err)
	}
	// A USD account counts a dollar stablecoin as a dollar (the user,
	// 2026-11-16: BTCUSDT on Exness, whose profit currency is "TET").
	if p.Currency == "USD" && usdStable[inst.Quote] {
		inst.Quote = "USD"
	}
	if inst.Quote != p.Currency && inst.Base != p.Currency {
		return nil, fmt.Errorf("san_backtest: %s makes profit in %s and the account is in %s: only instruments quoted in or based on the account currency convert (see README, known limits)",
			inst.Name, inst.Quote, p.Currency)
	}
	d := &legData{i: i, leg: l, name: l.Strategy.Name(), inst: inst, tfs: l.Timeframes, candles: map[string][]Candle{}, ccy: p.Currency, params: map[string]float64{}}
	if t, ok := l.Strategy.(Tunable); ok {
		for k, v := range t.Params() {
			d.params[k] = v
		}
	}
	for k, v := range l.Params {
		d.params[k] = v
	}
	d.costs = p.Costs
	if l.Costs != nil {
		d.costs = *l.Costs
	}
	d.report = costReport(d, p.Currency)
	if d.costs.Commission != nil {
		d.inst.Commission, d.inst.CommissionCurrency = *d.costs.Commission, p.Currency
	}
	d.swapMoney = d.costs.Swap
	return d, nil
}

// loadLeg's candles and their first step candle, after newLegData.
func loadCandles(ctx context.Context, src DataSource, p Portfolio, d *legData) (*legData, error) {
	inst, l := d.inst, d.leg
	if err := d.fetch(ctx, src, p.From, p.To); err != nil {
		return nil, err
	}
	step := d.candles[d.tfs[0]]
	s0 := sort.Search(len(step), func(k int) bool { return !step[k].OpenTime.Before(p.From) })
	if s0 == len(step) {
		return nil, fmt.Errorf("san_backtest: no %s %s candles between %s and %s", inst.Name, d.tfs[0], p.From.Format(time.RFC3339), p.To.Format(time.RFC3339))
	}
	s := s0
	for s < len(step) && !d.ready(s) {
		s++
	}
	if s == len(step) {
		return nil, fmt.Errorf("san_backtest: %s: not enough history for %d warm-up candles on %s", inst.Name, l.Strategy.Warmup(), strings.Join(d.tfs, ", "))
	}
	if err := d.lateStart(p, s0, s); err != nil {
		return nil, err
	}
	d.start = s
	return d, nil
}

// fetch loads every timeframe's candles of [from, to) with the warm-up
// before from.
func (d *legData) fetch(ctx context.Context, src DataSource, from, to time.Time) error {
	w := d.leg.Strategy.Warmup()
	for j, tf := range d.tfs {
		need := w
		if j == 0 {
			need = max(w, 1)
		}
		cs, err := fetchWarm(ctx, src, d.inst.Name, tf, from, to, need)
		if err != nil {
			return fmt.Errorf("san_backtest: %w", err)
		}
		d.candles[tf] = d.adjust(cs)
	}
	return nil
}

// adjust applies Costs.Spread (a fixed ask over the bid) to new candles.
func (d *legData) adjust(cs []Candle) []Candle {
	if d.costs.Spread == nil {
		return cs
	}
	sp := *d.costs.Spread
	out := make([]Candle, len(cs))
	for k, c := range cs {
		c.AskOpen, c.AskHigh, c.AskLow, c.AskClose = c.Open+sp, c.High+sp, c.Low+sp, c.Close+sp
		out[k] = c
	}
	return out
}

// ready says whether step candle s has the warm-up behind it on every
// timeframe.
func (d *legData) ready(s int) bool {
	w := d.leg.Strategy.Warmup()
	if s < max(w, 1) {
		return false
	}
	step := d.candles[d.tfs[0]]
	for _, tf := range d.tfs[1:] {
		cs := d.candles[tf]
		n := sort.Search(len(cs), func(k int) bool { return cs[k].CloseTime.After(step[s].OpenTime) })
		if n < w {
			return false
		}
	}
	return true
}

// lateStart notes (or with Strict refuses) a run that starts at step
// candle s instead of s0, the first at or after From, for want of warm-up.
func (d *legData) lateStart(p Portfolio, s0, s int) error {
	if s <= s0 {
		return nil
	}
	step := d.candles[d.tfs[0]]
	msg := fmt.Sprintf("%s history starts too late for From %s (%d warm-up candles of %s needed before it): the run starts at %s",
		d.inst.Name, p.From.Format(time.RFC3339), d.leg.Strategy.Warmup(), strings.Join(d.tfs, "/"), step[s].OpenTime.Format(time.RFC3339))
	if p.Strict {
		return &FieldError{Field: "Strict", Problem: msg}
	}
	d.report.Notes = append(d.report.Notes, msg)
	return nil
}

// fetchWarm gets [from, to) plus need candles before from, looking further
// back while there are too few (weekends and holidays have none).
func fetchWarm(ctx context.Context, src DataSource, name, tf string, from, to time.Time, need int) ([]Candle, error) {
	lookback := time.Duration(need)*longest(tf)*3/2 + 4*24*time.Hour
	start := from.Add(-lookback)
	cs, err := src.Candles(ctx, Query{Instrument: name, Timeframe: tf, From: start, To: to})
	if err != nil {
		return nil, err
	}
	empty := 0
	for tries := 0; tries < 8 && empty < 2; tries++ {
		before := sort.Search(len(cs), func(k int) bool { return !cs[k].OpenTime.Before(from) })
		if before >= need || lookback > 20*365*24*time.Hour {
			break
		}
		lookback *= 2
		older, err := src.Candles(ctx, Query{Instrument: name, Timeframe: tf, From: from.Add(-lookback), To: start})
		if err != nil {
			return nil, err
		}
		if len(older) == 0 {
			empty++
		}
		start = from.Add(-lookback)
		cs = append(older, cs...)
	}
	return cs, nil
}

func costReport(d *legData, ccy string) CostReport {
	r := CostReport{Leg: d.i, Instrument: d.inst.Name, Notes: append([]string{}, d.inst.Notes...)}
	c := d.costs
	if c.Spread != nil {
		r.Spread = fmt.Sprintf("override: %v fixed", *c.Spread)
	} else {
		r.Spread = "exness: each candle's ask - bid"
	}
	if c.Commission != nil {
		r.Commission = fmt.Sprintf("override: %v %s per lot per side", *c.Commission, ccy)
	} else {
		r.Commission = fmt.Sprintf("exness: %v %s per lot per side", d.inst.Commission, d.inst.CommissionCurrency)
		if d.inst.CommissionCurrency != "" && d.inst.CommissionCurrency != ccy && d.inst.Commission != 0 {
			r.Notes = append(r.Notes, fmt.Sprintf("commission currency %s taken as %s (no conversion)", d.inst.CommissionCurrency, ccy))
		}
	}
	if c.Swap != nil {
		r.Swap = fmt.Sprintf("override: long %v, short %v %s per lot per night", c.Swap[0], c.Swap[1], ccy)
	} else {
		r.Swap = fmt.Sprintf("exness: long %v, short %v points per lot per night", d.inst.SwapLong, d.inst.SwapShort)
	}
	r.Slippage = fmt.Sprintf("%v (0 unless set)", c.Slippage)
	if c.Slippage > 0 {
		r.Slippage = fmt.Sprintf("override: %v per market fill", c.Slippage)
	}
	return r
}

// The simulation.

// openPos is one open position. A leg may hold several (the user,
// 2026-11-15: "1b": hedging allowed, longs and shorts open together).
type openPos struct {
	id, orderID    int
	kind, tag      string // TypeMarket/TypeLimit/TypeStop, the order's Tag
	side           Side
	entry          float64
	stop, initStop float64
	target         float64
	lots           float64
	signalTime     time.Time
	openTime       time.Time
	risk           float64
	margin         float64
	commission     float64 // charged so far (the open side)
	swap           float64
	lastRoll       time.Time
	spread         float64
}

// pendingOrder is a limit or stop order waiting for its price.
type pendingOrder struct {
	id                  int
	side                Side
	kind                string // TypeLimit or TypeStop
	price, stop, target float64
	tag                 string
	signalTime          time.Time // the close of the candle that asked for it
	placed              time.Time // the open of the candle it went live on
	bars, seen          int       // Expire's candles, the candles checked so far
	until               time.Time
}

type legRun struct {
	d          *legData
	frames     map[string]*series
	ctx        *Context
	positions  []*openPos // in open order
	orders     []*pendingOrder
	pending    []Signal // the last signal's actions, run at the next open
	signalTime time.Time
	// maxOpen is the leg's position limit: Leg.MaxOpen, else the
	// strategy's MaxOpen(); 0 = neither, one position at a time as before
	// (a Buy or Sell while one is open is ignored, unrecorded).
	maxOpen   int
	bid, ask  float64
	prevClose time.Time
	trades    []Trade
	firstIdx  int
	lastIdx   int
	// ranIdx is the last step candle whose close has run (Live's results
	// end there; a whole run reaches lastIdx).
	ranIdx int
	params map[string]float64
}

type sim struct {
	ld       *loaded
	p        Portfolio
	balance  float64
	legs     []*legRun
	trades   []Trade
	missed   []Missed
	skipped  map[string]int
	equity   []EquityPoint
	day      string
	dayStart float64
	// record keeps the decision log and the candles (the main run; never
	// the walk-forward trials).
	record    bool
	decisions []Decision
	ids       int // the last position or order number given out
	params    []map[string]float64
	// live: a Live run, whose results end at the last candle run so far.
	live bool
}

type event struct {
	t     time.Time
	close bool
	leg   int
	idx   int
}

// run replays [from, to) from balance; override replaces leg 0's params
// (walk-forward trials). record keeps the decision log and the candles.
func (ld *loaded) run(balance float64, from, to time.Time, override map[string]float64, record bool) (res *Result, err error) {
	s := ld.newSim(balance, from, to, override, record, nil)
	var events []event
	for i, l := range s.legs {
		for k := l.firstIdx; k <= l.lastIdx; k++ {
			c := l.d.candles[l.d.tfs[0]][k]
			events = append(events, event{t: c.OpenTime, leg: i, idx: k}, event{t: c.CloseTime, close: true, leg: i, idx: k})
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if !a.t.Equal(b.t) {
			return a.t.Before(b.t)
		}
		if a.close != b.close {
			return a.close // closes at T come before opens at T
		}
		return a.leg < b.leg
	})
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("san_backtest: strategy panicked: %v", r)
		}
	}()
	for _, e := range events {
		s.step(e)
	}
	s.finish()
	return s.result(balance, from, to, s.params), nil
}

// newSim sets up a run of [from, to) from balance and calls the
// strategies' OnStart. first, when set, is each leg's first step candle
// (Live gives the one it found ready); otherwise the first one at or after
// from that the warm-up allows.
func (ld *loaded) newSim(balance float64, from, to time.Time, override map[string]float64, record bool, first []int) *sim {
	s := &sim{ld: ld, p: ld.p, balance: balance, skipped: map[string]int{}, record: record}
	for i, d := range ld.legs {
		step := d.candles[d.tfs[0]]
		var firstIdx, last int
		if first != nil {
			firstIdx, last = first[i], math.MaxInt // a live run: no last candle yet
		} else {
			firstIdx = max(d.start, sort.Search(len(step), func(k int) bool { return !step[k].OpenTime.Before(from) }))
			last = sort.Search(len(step), func(k int) bool { return step[k].CloseTime.After(to) }) - 1
		}
		l := &legRun{d: d, frames: map[string]*series{}, firstIdx: firstIdx, lastIdx: last, params: map[string]float64{}, maxOpen: d.leg.MaxOpen}
		if mp, ok := d.leg.Strategy.(MultiPosition); ok && l.maxOpen == 0 {
			l.maxOpen = max(mp.MaxOpen(), 0)
		}
		for k, v := range d.params {
			l.params[k] = v
		}
		if i == 0 {
			for k, v := range override {
				l.params[k] = v
			}
		}
		s.params = append(s.params, l.params)
		anchors := map[string]time.Time{}
		for _, tf := range d.tfs {
			l.frames[tf] = newSeries(tf, d.candles[tf])
			anchors[tf] = time.Date(1970, 1, 4, 0, 0, 0, 0, time.UTC) // a Sunday, MT5's week start
			if cs := d.candles[tf]; len(cs) > 0 {
				anchors[tf] = cs[0].OpenTime
			}
		}
		in := d.inst
		l.ctx = &Context{step: d.tfs[0], tfs: d.tfs, frames: l.frames, anchors: anchors, params: l.params, record: record, inst: InstrumentInfo{Name: in.Name, Base: in.Base,
			Quote: in.Quote, Digits: in.Digits, Point: in.Point(), ContractSize: in.ContractSize, LotMin: in.LotMin, LotMax: in.LotMax, LotStep: in.LotStep}}
		s.legs = append(s.legs, l)
		if st, ok := d.leg.Strategy.(Starter); ok {
			st.OnStart(StartInfo{Instrument: l.ctx.inst, Timeframes: d.tfs, From: from, To: to, Balance: balance, Currency: ld.p.Currency, Params: l.params})
		}
	}
	return s
}

// step runs one event: a step candle's open or its close.
func (s *sim) step(e event) {
	s.newDay(e.t)
	l := s.legs[e.leg]
	c := l.d.candles[l.d.tfs[0]][e.idx]
	if e.close {
		s.onClose(l, e.idx, c)
		l.ranIdx = e.idx
	} else {
		s.onOpen(l, c)
	}
}

// finish ends the run: pending orders are cancelled and open positions
// closed at the last price.
func (s *sim) finish() {
	var end time.Time
	for _, l := range s.legs {
		for _, o := range l.orders {
			s.endOrder(l, o, l.prevClose, DecisionOrderCancelled, "", "end of the run")
		}
		l.orders = nil
		for len(l.positions) > 0 {
			p := l.positions[0]
			exit := l.bid
			if p.side == Short {
				exit = l.ask
			}
			s.closePos(l, p, exit, l.prevClose, "closed at end", l.prevClose)
		}
		if l.prevClose.After(end) {
			end = l.prevClose
		}
	}
	if !end.IsZero() {
		s.mark(end)
	}
}

func (s *sim) newDay(t time.Time) {
	k := t.In(s.p.DayTZ).Format("2006-01-02")
	if k != s.day {
		s.day, s.dayStart = k, s.equityNow()
	}
}

func (s *sim) floatingOf(l *legRun, p *openPos) float64 {
	exit, sign := l.bid, 1.0
	if p.side == Short {
		exit, sign = l.ask, -1
	}
	return sign*(exit-p.entry)*p.lots*l.d.inst.ContractSize*l.d.conv(exit) + p.swap
}

// floating is the leg's open positions' floating profit, swap included.
func (s *sim) floating(l *legRun) (f float64) {
	for _, p := range l.positions {
		f += s.floatingOf(l, p)
	}
	return f
}

func (s *sim) equityNow() float64 {
	e := s.balance
	for _, l := range s.legs {
		e += s.floating(l)
	}
	return e
}

// usedMargin is the margin of every open position, all legs.
func (s *sim) usedMargin() (m float64) {
	for _, l := range s.legs {
		for _, p := range l.positions {
			m += p.margin
		}
	}
	return m
}

func (s *sim) openCount() (n int) {
	for _, l := range s.legs {
		n += len(l.positions)
	}
	return n
}

// openRisk is the money open positions lose if all stops are hit now.
func (s *sim) openRisk() (r float64) {
	for _, l := range s.legs {
		for _, p := range l.positions {
			adverse := p.entry - p.stop
			if p.side == Short {
				adverse = p.stop - p.entry
			}
			r += math.Max(0, adverse) * p.lots * l.d.inst.ContractSize * l.d.conv(p.stop)
		}
	}
	return r
}

func (s *sim) newID() int {
	s.ids++
	return s.ids
}

// onOpen runs the last signal's actions at the candle's open, in order. A
// Close that waits for the market to open keeps itself and the actions
// after it for the next open.
func (s *sim) onOpen(l *legRun, c Candle) {
	l.bid, l.ask = c.Open, c.AskOpen
	for len(l.pending) > 0 {
		if !s.act(l, c, l.pending[0]) {
			return
		}
		l.pending = l.pending[1:]
	}
	l.pending = nil
}

// act runs one action; false when it waits for the market to open.
func (s *sim) act(l *legRun, c Candle, sig Signal) bool {
	switch sig.Action {
	case ActBuy, ActSell:
		if l.maxOpen == 0 && len(l.positions) > 0 {
			return true // one position at a time, as before: ignored
		}
		s.enter(l, c, sig)
	case ActClose, ActCloseAll:
		if sig.Action == ActCloseAll {
			for _, o := range l.orders {
				s.endOrder(l, o, c.OpenTime, DecisionOrderCancelled, SkipCancelled, reasonOr(sig.Reason, "close all"))
			}
			l.orders = nil
		}
		ps := l.targets(sig.ID)
		if len(ps) == 0 {
			return true
		}
		if !l.d.inst.Open(c.OpenTime) {
			return false // waits for the market to open
		}
		reason := "close"
		if sig.Reason != "" {
			reason += ": " + sig.Reason
		}
		slip := l.d.costs.Slippage
		for _, p := range ps {
			exit := c.Open - slip
			if p.side == Short {
				exit = c.AskOpen + slip
			}
			s.closePos(l, p, exit, c.OpenTime, reason, c.OpenTime)
		}
	case ActMoveStop:
		if math.IsNaN(sig.Stop) || sig.Stop <= 0 {
			return true
		}
		for _, p := range l.targets(sig.ID) {
			if (p.side == Long && sig.Stop > p.stop) || (p.side == Short && sig.Stop < p.stop) {
				s.decide(l, c.OpenTime, DecisionStopMove, p.side, posDetails(p, map[string]float64{"old_stop": p.stop, "stop": sig.Stop}), sig.Reason)
				p.stop = sig.Stop
			}
		}
	case ActBuyLimit, ActSellLimit, ActBuyStop, ActSellStop:
		s.place(l, c, sig)
	case ActCancel, ActCancelAll:
		var keep []*pendingOrder
		for _, o := range l.orders {
			if sig.Action == ActCancelAll || o.id == sig.ID {
				s.endOrder(l, o, c.OpenTime, DecisionOrderCancelled, SkipCancelled, reasonOr(sig.Reason, "cancel"))
				continue
			}
			keep = append(keep, o)
		}
		l.orders = keep
	}
	return true
}

func reasonOr(r, def string) string {
	if r != "" {
		return r
	}
	return def
}

// targets are the open positions an action is for: id's, or all for 0.
func (l *legRun) targets(id int) []*openPos {
	if id == 0 {
		return append([]*openPos{}, l.positions...)
	}
	for _, p := range l.positions {
		if p.id == id {
			return []*openPos{p}
		}
	}
	return nil
}

// posDetails adds a position's numbers to a decision's details.
func posDetails(p *openPos, det map[string]float64) map[string]float64 {
	det["position_id"] = float64(p.id)
	if p.orderID != 0 {
		det["order_id"] = float64(p.orderID)
	}
	return det
}

// entryMeta is where an entry comes from: a market Buy/Sell or an order.
type entryMeta struct {
	kind       string
	orderID    int
	tag        string
	signalTime time.Time
}

func (s *sim) miss(l *legRun, t time.Time, side Side, det map[string]float64, reason string, m entryMeta) {
	d := l.d
	s.skipped[reason]++
	s.missed = append(s.missed, Missed{Time: t, Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Side: side, Reason: reason, OrderID: m.orderID,
		OrderType: m.kind, Tag: m.tag})
	if m.orderID != 0 {
		det["order_id"] = float64(m.orderID)
	}
	s.decide(l, t, DecisionMissed, side, det, reason)
}

// enter opens a market Buy or Sell at the candle's open.
func (s *sim) enter(l *legRun, c Candle, sig Signal) {
	d := l.d
	side := sig.Action.Side()
	m := entryMeta{kind: TypeMarket, signalTime: l.signalTime}
	st, tg := sig.Stop, sig.Target
	if !d.inst.Open(c.OpenTime) {
		s.miss(l, c.OpenTime, side, map[string]float64{"stop": st, "target": tg}, SkipMarketClosed, m)
		return
	}
	fill := c.AskOpen + d.costs.Slippage
	if side == Short {
		fill = c.Open - d.costs.Slippage
	}
	if math.IsNaN(st) || math.IsNaN(tg) || st <= 0 || tg < 0 ||
		(side == Long && (st >= fill || (tg > 0 && tg <= fill))) || (side == Short && (st <= fill || (tg > 0 && tg >= fill))) {
		s.miss(l, c.OpenTime, side, map[string]float64{"stop": st, "target": tg}, SkipLevels, m)
		return
	}
	s.open(l, c, side, fill, st, tg, m)
}

// open checks the account limits and sizes a new position at fill: the
// daily loss, MaxOpen (the portfolio's, then the leg's), the risk % of the
// balance now rounded down to the lot step (the user, 2026-11-15: "sizing
// = risk % of the current balance, as now"), MaxOpenRisk and the free
// margin. nil when it is skipped (recorded).
func (s *sim) open(l *legRun, c Candle, side Side, fill, st, tg float64, m entryMeta) *openPos {
	d := l.d
	miss := func(reason string) {
		s.miss(l, c.OpenTime, side, map[string]float64{"stop": st, "target": tg}, reason, m)
	}
	eq := s.equityNow()
	if s.p.MaxDailyLoss > 0 && s.dayStart > 0 && 100*(s.dayStart-eq)/s.dayStart >= s.p.MaxDailyLoss {
		miss(SkipDailyLoss)
		return nil
	}
	if s.p.MaxOpen > 0 && s.openCount() >= s.p.MaxOpen {
		miss(SkipMaxOpen)
		return nil
	}
	if len(l.positions) >= max(l.maxOpen, 1) {
		miss(SkipMaxOpen)
		return nil
	}
	perLot := math.Abs(fill-st) * d.inst.ContractSize * d.conv(fill)
	lots := lotsFor(s.balance*d.leg.Risk/100/perLot, d.inst)
	if lots == 0 {
		miss(SkipTooSmall)
		return nil
	}
	risk := lots * perLot
	if s.p.MaxOpenRisk > 0 && 100*(s.openRisk()+risk)/s.balance > s.p.MaxOpenRisk+1e-9 {
		miss(SkipMaxOpenRisk)
		return nil
	}
	margin := d.margin(lots, fill, s.p.Leverage)
	if margin > eq-s.usedMargin()+1e-9 {
		miss(SkipMargin)
		return nil
	}
	comm := lots * d.inst.Commission
	s.balance -= comm
	p := &openPos{id: s.newID(), orderID: m.orderID, kind: m.kind, tag: m.tag, side: side, entry: fill, stop: st, initStop: st, target: tg, lots: lots,
		signalTime: m.signalTime, openTime: c.OpenTime, risk: risk, margin: margin, commission: comm, lastRoll: c.OpenTime, spread: c.AskOpen - c.Open}
	l.positions = append(l.positions, p)
	if m.orderID != 0 {
		s.decide(l, c.OpenTime, DecisionOrderFilled, side, map[string]float64{"order_id": float64(m.orderID), "position_id": float64(p.id), "price": fill,
			"type": orderTypeCode(m.kind)}, orderName(side, m.kind, m.tag))
	}
	s.decide(l, c.OpenTime, DecisionEntry, side, posDetails(p, map[string]float64{"price": fill, "stop": st, "target": tg, "lots": lots, "risk": risk,
		"spread": c.AskOpen - c.Open}), "")
	return p
}

func orderTypeCode(kind string) float64 {
	if kind == TypeStop {
		return 2
	}
	return 1
}

// orderName is e.g. "buy limit" or "sell stop (tag)".
func orderName(side Side, kind, tag string) string {
	n := string(side) + " " + kind
	if tag != "" {
		n += " (" + tag + ")"
	}
	return n
}

// orderLevels is rule #1 for a pending order at price: a stop-loss and a
// take-profit, real prices, on the right sides (below and above price for
// a buy). "" when it holds.
func orderLevels(side Side, price, st, tg float64) string {
	bad := func(x float64) bool { return math.IsNaN(x) || math.IsInf(x, 0) || x <= 0 }
	switch {
	case bad(price):
		return "no order price"
	case bad(st):
		return "no stop-loss"
	case bad(tg):
		return "no take-profit"
	case side == Long && !(st < price && price < tg):
		return fmt.Sprintf("buy at %.5g with stop %.5g and target %.5g", price, st, tg)
	case side == Short && !(tg < price && price < st):
		return fmt.Sprintf("sell at %.5g with stop %.5g and target %.5g", price, st, tg)
	}
	return ""
}

// place makes a pending order live from this candle on (the signal came at
// the previous candle's close). Rule #1 is checked here and again at the
// fill; a refused order is recorded as "invalid levels".
func (s *sim) place(l *legRun, c Candle, sig Signal) {
	side := sig.Action.Side()
	kind := TypeLimit
	if sig.Action == ActBuyStop || sig.Action == ActSellStop {
		kind = TypeStop
	}
	o := &pendingOrder{id: s.newID(), side: side, kind: kind, price: sig.Price, stop: sig.Stop, target: sig.Target, tag: sig.Label,
		signalTime: l.signalTime, placed: c.OpenTime, bars: max(sig.Bars, 0), until: sig.Expires}
	det := map[string]float64{"price": o.price, "stop": o.stop, "target": o.target, "type": orderTypeCode(kind)}
	if why := orderLevels(side, o.price, o.stop, o.target); why != "" {
		s.miss(l, c.OpenTime, side, det, SkipLevels, entryMeta{kind: kind, orderID: o.id, tag: o.tag})
		return
	}
	det["order_id"] = float64(o.id)
	if o.bars > 0 {
		det["bars"] = float64(o.bars)
	}
	l.orders = append(l.orders, o)
	s.decide(l, c.OpenTime, DecisionOrderPlaced, side, det, orderName(side, kind, o.tag))
}

// endOrder records an order that ends unfilled: kind is the decision,
// skip the Missed reason ("" = not a missed trade, e.g. the end of the
// run).
func (s *sim) endOrder(l *legRun, o *pendingOrder, t time.Time, kind, skip, reason string) {
	det := map[string]float64{"order_id": float64(o.id), "price": o.price, "type": orderTypeCode(o.kind)}
	s.decide(l, t, kind, o.side, det, reason)
	if skip != "" {
		d := l.d
		s.skipped[skip]++
		s.missed = append(s.missed, Missed{Time: t, Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Side: o.side, Reason: skip, OrderID: o.id,
			OrderType: o.kind, Tag: o.tag})
	}
}

// decide adds a line to the decision log when the run records one.
func (s *sim) decide(l *legRun, t time.Time, kind string, side Side, details map[string]float64, reason string) {
	if !s.record {
		return
	}
	s.decisions = append(s.decisions, Decision{Time: t, Leg: l.d.i, Kind: kind, Side: side, Details: details, Reason: reason})
}

// decideSignal logs what OnCandle asked for at t, with its notes as the
// reason; a step that waits but noted something is a note line. A Do is
// one signal line per action, the reason on the first.
func (s *sim) decideSignal(l *legRun, t time.Time, sig Signal, notes []string) {
	if !s.record {
		return
	}
	reason := strings.Join(notes, "; ")
	if sig.Action == ActWait {
		if sig.Reason != "" {
			reason = strings.Join(append([]string{sig.Reason}, notes...), "; ")
		}
		if reason != "" {
			s.decide(l, t, DecisionNote, "", nil, reason)
		}
		return
	}
	list := sig.List()
	if sig.Action != ActDo && sig.Reason != "" {
		reason = strings.Join(append([]string{sig.Reason}, notes...), "; ")
	}
	for i, x := range list {
		det := map[string]float64{"action": float64(x.Action)}
		side := x.Action.Side()
		switch {
		case x.Action == ActBuy || x.Action == ActSell:
			det["stop"], det["target"] = x.Stop, x.Target
			if k, ok := l.ctx.Last(l.ctx.step); ok {
				det["close"] = k.Close
			}
		case x.Action.IsOrder():
			det["price"], det["stop"], det["target"] = x.Price, x.Stop, x.Target
		case x.Action == ActMoveStop:
			det["stop"] = x.Stop
		}
		if x.ID != 0 {
			det["id"] = float64(x.ID)
		}
		if side == "" && len(l.positions) > 0 {
			side = l.positions[0].side
			for _, p := range l.positions {
				if p.id == x.ID {
					side = p.side
				}
			}
		}
		r := reason
		if sig.Action == ActDo {
			r = x.Reason
			if i == 0 {
				r = strings.Join(append(nonEmpty(x.Reason), notes...), "; ")
			}
		}
		s.decide(l, t, DecisionSignal, side, det, r)
	}
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// lotsFor rounds want down to the lot step, capped at the maximum; 0 when
// it is under the minimum.
func lotsFor(want float64, in Instrument) float64 {
	if math.IsNaN(want) || math.IsInf(want, 0) || want <= 0 {
		return 0
	}
	steps := math.Floor(want/in.LotStep + 1e-9)
	lots := math.Round(steps*in.LotStep*1e8) / 1e8
	if lots > in.LotMax {
		lots = in.LotMax
	}
	if lots < in.LotMin-1e-12 {
		return 0
	}
	return lots
}

func (s *sim) onClose(l *legRun, idx int, c Candle) {
	for _, tf := range l.d.tfs {
		f := l.frames[tf]
		if tf == l.d.tfs[0] {
			f.n = idx + 1
			continue
		}
		for f.n < len(f.all) && !f.all[f.n].CloseTime.After(c.CloseTime) {
			f.n++
		}
	}
	if l.d.inst.Open(c.OpenTime) {
		for _, p := range append([]*openPos{}, l.positions...) {
			if !p.openTime.After(c.OpenTime) {
				s.exits(l, p, c)
			}
		}
	}
	s.fills(l, c)
	l.bid, l.ask = c.Close, c.AskClose
	for _, p := range l.positions {
		s.swap(l, p, c.CloseTime)
	}
	gap := time.Duration(0)
	if !l.prevClose.IsZero() {
		gap = c.OpenTime.Sub(l.prevClose)
	}
	l.prevClose = c.CloseTime
	s.mark(c.CloseTime)
	if idx >= l.lastIdx {
		return // a signal now would fill after To
	}
	x := l.ctx
	x.time, x.gap = c.CloseTime, gap
	x.inst.Spread = c.AskClose - c.Close
	x.positions, x.cur, x.orders, x.maxOpen = x.positions[:0], nil, x.orders[:0], l.maxOpen
	for _, p := range l.positions {
		x.positions = append(x.positions, Position{Side: p.side, Entry: p.entry, Stop: p.stop, Target: p.target, Lots: p.lots, OpenTime: p.openTime,
			Risk: p.risk, Profit: s.floatingOf(l, p) - p.commission, ID: p.id, OrderID: p.orderID, Type: p.kind, Tag: p.tag})
	}
	for _, o := range l.orders {
		left := 0
		if o.bars > 0 {
			left = o.bars - o.seen
		}
		x.orders = append(x.orders, Order{ID: o.id, Side: o.side, Type: o.kind, Price: o.price, Stop: o.stop, Target: o.target, Placed: o.placed,
			BarsLeft: left, Expires: o.until, Tag: o.tag})
	}
	x.account = Account{Currency: s.p.Currency, Balance: s.balance, Equity: s.equityNow(), Margin: s.usedMargin(), Open: s.openCount()}
	x.notes = nil
	sig := l.d.leg.Strategy.OnCandle(x)
	s.decideSignal(l, c.CloseTime, sig, x.notes)
	x.notes = nil
	if list := sig.List(); len(list) > 0 {
		l.pending, l.signalTime = list, c.CloseTime
	}
}

// fills checks the pending orders on c, oldest first, after the open
// positions' stops and targets. An order fills once (the user, 2026-11-15:
// "each order fills once only"); one past its Until or its Expire candles
// expires.
func (s *sim) fills(l *legRun, c Candle) {
	if len(l.orders) == 0 {
		return
	}
	open := l.d.inst.Open(c.OpenTime)
	var keep []*pendingOrder
	for _, o := range l.orders {
		if !o.until.IsZero() && !c.OpenTime.Before(o.until) {
			s.endOrder(l, o, c.OpenTime, DecisionOrderExpired, SkipExpired, "until "+o.until.UTC().Format(time.RFC3339))
			continue
		}
		if open {
			if px, atOpen, ok := o.fillOn(c, l.d.costs.Slippage); ok {
				s.fill(l, o, c, px, atOpen)
				continue
			}
		}
		o.seen++
		if o.bars > 0 && o.seen >= o.bars {
			s.endOrder(l, o, c.CloseTime, DecisionOrderExpired, SkipExpired, fmt.Sprintf("after %d candles", o.bars))
			continue
		}
		keep = append(keep, o)
	}
	l.orders = keep
}

// fillOn says whether o fills in c and at what price. A buy fills on the
// ask, a sell on the bid. Limits: at the limit once the range reaches it,
// or at the open when the candle opens through it (better), no slippage.
// Stops: at the stop price once the range reaches it, or at the open when
// it gaps past (worse), plus slippage. atOpen: it filled at the open.
func (o *pendingOrder) fillOn(c Candle, slip float64) (px float64, atOpen, ok bool) {
	switch {
	case o.side == Long && o.kind == TypeLimit:
		if c.AskOpen <= o.price {
			return c.AskOpen, true, true
		}
		if c.AskLow <= o.price {
			return o.price, false, true
		}
	case o.side == Short && o.kind == TypeLimit:
		if c.Open >= o.price {
			return c.Open, true, true
		}
		if c.High >= o.price {
			return o.price, false, true
		}
	case o.side == Long:
		if c.AskOpen >= o.price {
			return c.AskOpen + slip, true, true
		}
		if c.AskHigh >= o.price {
			return o.price + slip, false, true
		}
	default:
		if c.Open <= o.price {
			return c.Open - slip, true, true
		}
		if c.Low <= o.price {
			return o.price - slip, false, true
		}
	}
	return 0, false, false
}

// fill opens o's position at px, then looks at the rest of the fill
// candle the worst way (the user, 2026-11-15: "2a"): if its range also
// reaches the stop, the stop is hit in this candle (and wins over the
// target). A fill at the open has the whole candle after it, so the stop
// and the target count as for a market entry. A stop order filled inside
// the candle can also reach its target there (the price went through the
// order on its way); a limit filled inside cannot, since the far side of
// the range may have come before the fill.
func (s *sim) fill(l *legRun, o *pendingOrder, c Candle, px float64, atOpen bool) {
	m := entryMeta{kind: o.kind, orderID: o.id, tag: o.tag, signalTime: o.signalTime}
	if why := orderLevels(o.side, px, o.stop, o.target); why != "" {
		det := map[string]float64{"price": px, "stop": o.stop, "target": o.target, "type": orderTypeCode(o.kind)}
		s.miss(l, c.OpenTime, o.side, det, SkipLevels, m)
		return
	}
	p := s.open(l, c, o.side, px, o.stop, o.target, m)
	if p == nil {
		return
	}
	if atOpen {
		s.exits(l, p, c)
		return
	}
	slip := l.d.costs.Slippage
	if p.side == Long {
		switch {
		case c.Low <= p.stop:
			s.closePos(l, p, p.stop-slip, c.CloseTime, "stop", c.OpenTime)
		case o.kind == TypeStop && c.High >= p.target:
			s.closePos(l, p, p.target, c.CloseTime, "target", c.OpenTime)
		}
		return
	}
	switch {
	case c.AskHigh >= p.stop:
		s.closePos(l, p, p.stop+slip, c.CloseTime, "stop", c.OpenTime)
	case o.kind == TypeStop && c.AskLow <= p.target:
		s.closePos(l, p, p.target, c.CloseTime, "target", c.OpenTime)
	}
}

// exits checks the stop and the target on c's range; the stop counts first
// when both are inside one candle. A candle opening past a level fills at
// its open.
func (s *sim) exits(l *legRun, p *openPos, c Candle) {
	slip := l.d.costs.Slippage
	if p.side == Long {
		switch {
		case c.Low <= p.stop:
			s.closePos(l, p, math.Min(p.stop, c.Open)-slip, c.CloseTime, "stop", c.OpenTime)
		case p.target > 0 && c.High >= p.target:
			s.closePos(l, p, math.Max(p.target, c.Open), c.CloseTime, "target", c.OpenTime)
		}
		return
	}
	switch {
	case c.AskHigh >= p.stop:
		s.closePos(l, p, math.Max(p.stop, c.AskOpen)+slip, c.CloseTime, "stop", c.OpenTime)
	case p.target > 0 && c.AskLow <= p.target:
		s.closePos(l, p, math.Min(p.target, c.AskOpen), c.CloseTime, "target", c.OpenTime)
	}
}

// swap charges p the rollovers in (lastRoll, until].
func (s *sim) swap(l *legRun, p *openPos, until time.Time) {
	in := l.d.inst
	for _, r := range rollovers(p.lastRoll, until) {
		mult := 1
		if m, ok := in.SwapDays[r.In(newYork).Weekday()]; ok {
			mult = m
		}
		var perLot float64
		if l.d.swapMoney != nil {
			perLot = l.d.swapMoney[0]
			if p.side == Short {
				perLot = l.d.swapMoney[1]
			}
		} else {
			pts, price := in.SwapLong, l.bid
			if p.side == Short {
				pts, price = in.SwapShort, l.ask
			}
			perLot = pts * in.Point() * in.ContractSize * l.d.conv(price)
		}
		p.swap += perLot * p.lots * float64(mult)
	}
	if until.After(p.lastRoll) {
		p.lastRoll = until
	}
}

func (s *sim) closePos(l *legRun, p *openPos, exit float64, at time.Time, reason string, swapUntil time.Time) {
	s.swap(l, p, swapUntil)
	d := l.d
	sign := 1.0
	if p.side == Short {
		sign = -1
	}
	gross := sign * (exit - p.entry) * p.lots * d.inst.ContractSize * d.conv(exit)
	comm := p.lots * d.inst.Commission
	s.balance += gross + p.swap - comm
	kind := p.kind
	if kind == "" {
		kind = TypeMarket
	}
	t := Trade{Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Side: p.side, SignalTime: p.signalTime, OpenTime: p.openTime, CloseTime: at,
		Entry: p.entry, Exit: exit, Stop: p.initStop, FinalStop: p.stop, Target: p.target, Lots: p.lots, Spread: p.spread, Risk: p.risk,
		Gross: gross, Commission: p.commission + comm, Swap: p.swap, Reason: reason, PositionID: p.id, OrderID: p.orderID, OrderType: kind, Tag: p.tag}
	t.Net = t.Gross - t.Commission + t.Swap
	if t.Risk > 0 {
		t.R = t.Net / t.Risk
	}
	s.trades = append(s.trades, t)
	l.trades = append(l.trades, t)
	for i, q := range l.positions {
		if q == p {
			l.positions = append(l.positions[:i:i], l.positions[i+1:]...)
			break
		}
	}
	s.decide(l, at, DecisionExit, t.Side, posDetails(p, map[string]float64{"price": exit, "entry": t.Entry, "lots": t.Lots, "net": t.Net, "r": t.R}), reason)
}

func (s *sim) mark(t time.Time) {
	pt := EquityPoint{Time: t, Balance: s.balance, Equity: s.equityNow()}
	if n := len(s.equity); n > 0 && s.equity[n-1].Time.Equal(t) {
		s.equity[n-1] = pt
		return
	}
	s.equity = append(s.equity, pt)
}

func (s *sim) result(balance float64, from, to time.Time, params []map[string]float64) *Result {
	ld := s.ld
	trades := append([]Trade(nil), s.trades...) // a Live run keeps adding to s.trades
	sort.SliceStable(trades, func(i, j int) bool { return trades[i].CloseTime.Before(trades[j].CloseTime) })
	mid := from.Add(to.Sub(from) / 2)
	res := &Result{Trades: trades, Missed: s.missed, Equity: s.equity, Params: params,
		Range: Range{From: from, To: to, RequestedFrom: ld.p.From, RequestedTo: ld.p.To, Notes: append([]string{}, ld.notes...)}}
	res.Stats = computeStats(trades, s.equity, balance, mid, s.skipped)
	res.Run = RunInfo{RanAt: time.Now().UTC(), Balance: balance, Currency: ld.p.Currency, Leverage: ld.p.Leverage, MaxOpen: ld.p.MaxOpen,
		MaxOpenRisk: ld.p.MaxOpenRisk, MaxDailyLoss: ld.p.MaxDailyLoss, DayTZ: ld.p.DayTZ.String()}
	if s.record {
		res.Decisions = s.decisions
	}
	var names []string
	for i, l := range s.legs {
		d := l.d
		res.Run.Legs = append(res.Run.Legs, LegInfo{Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Timeframes: append([]string{}, d.tfs...),
			Risk: d.leg.Risk, Params: cloneParams(params[i]), Overridden: overridden(d.costs), MaxOpen: max(l.maxOpen, 1)})
		last := l.lastIdx
		if s.live {
			last = l.ranIdx
		}
		if s.record && l.firstIdx <= last {
			res.Candles = append(res.Candles, CandleSet{Leg: d.i, Instrument: d.inst.Name, Timeframe: d.tfs[0],
				Candles: d.candles[d.tfs[0]][l.firstIdx : last+1]})
		}
		res.Costs = append(res.Costs, d.report)
		res.Range.Notes = append(res.Range.Notes, d.report.Notes...)
		skipped := map[string]int{}
		for _, m := range s.missed {
			if m.Leg == d.i {
				skipped[m.Reason]++
			}
		}
		lr := LegResult{Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Stats: computeStats(l.trades, curve(l.trades, balance), balance, mid, skipped)}
		if res.Stats.NetProfit != 0 {
			lr.Contribution = 100 * lr.Stats.NetProfit / res.Stats.NetProfit
		}
		res.Legs = append(res.Legs, lr)
		names = append(names, d.name)
	}
	res.Name = strings.Join(names, " + ")
	if len(s.legs) > 1 {
		names = names[:0]
		for _, l := range s.legs {
			names = append(names, l.d.name+"@"+l.d.inst.Name)
		}
		res.Name = strings.Join(names, " + ")
	}
	return res
}
