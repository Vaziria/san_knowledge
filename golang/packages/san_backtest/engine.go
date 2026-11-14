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
	res, err := ld.run(p.Balance, ld.from, ld.to, nil)
	if err != nil {
		return nil, err
	}
	if cfg.Walk != nil {
		if res.Walk, err = ld.walk(*cfg.Walk); err != nil {
			return nil, err
		}
	}
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
	return ld.run(p.Balance, ld.from, ld.to, nil)
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
	inst, err := src.Instrument(ctx, l.Instrument)
	if err != nil {
		return nil, fmt.Errorf("san_backtest: instrument %s: %w", l.Instrument, err)
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
	w := l.Strategy.Warmup()
	for j, tf := range d.tfs {
		need := w
		if j == 0 {
			need = max(w, 1)
		}
		cs, err := fetchWarm(ctx, src, inst.Name, tf, p.From, p.To, need)
		if err != nil {
			return nil, fmt.Errorf("san_backtest: %w", err)
		}
		if d.costs.Spread != nil {
			sp := *d.costs.Spread
			out := make([]Candle, len(cs))
			for k, c := range cs {
				c.AskOpen, c.AskHigh, c.AskLow, c.AskClose = c.Open+sp, c.High+sp, c.Low+sp, c.Close+sp
				out[k] = c
			}
			cs = out
		}
		d.candles[tf] = cs
	}
	step := d.candles[d.tfs[0]]
	s0 := sort.Search(len(step), func(k int) bool { return !step[k].OpenTime.Before(p.From) })
	if s0 == len(step) {
		return nil, fmt.Errorf("san_backtest: no %s %s candles between %s and %s", inst.Name, d.tfs[0], p.From.Format(time.RFC3339), p.To.Format(time.RFC3339))
	}
	ready := func(s int) bool {
		if s < max(w, 1) {
			return false
		}
		for _, tf := range d.tfs[1:] {
			cs := d.candles[tf]
			n := sort.Search(len(cs), func(k int) bool { return cs[k].CloseTime.After(step[s].OpenTime) })
			if n < w {
				return false
			}
		}
		return true
	}
	s := s0
	for s < len(step) && !ready(s) {
		s++
	}
	if s == len(step) {
		return nil, fmt.Errorf("san_backtest: %s: not enough history for %d warm-up candles on %s", inst.Name, w, strings.Join(d.tfs, ", "))
	}
	if s > s0 {
		msg := fmt.Sprintf("%s history starts too late for From %s (%d warm-up candles of %s needed before it): the run starts at %s",
			inst.Name, p.From.Format(time.RFC3339), w, strings.Join(d.tfs, "/"), step[s].OpenTime.Format(time.RFC3339))
		if p.Strict {
			return nil, &FieldError{Field: "Strict", Problem: msg}
		}
		d.report.Notes = append(d.report.Notes, msg)
	}
	d.start = s
	return d, nil
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

type openPos struct {
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

type legRun struct {
	d          *legData
	frames     map[string]*series
	ctx        *Context
	pos        *openPos
	pending    *Signal
	signalTime time.Time
	bid, ask   float64
	prevClose  time.Time
	trades     []Trade
	lastIdx    int
	params     map[string]float64
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
}

type event struct {
	t     time.Time
	close bool
	leg   int
	idx   int
}

// run replays [from, to) from balance; override replaces leg 0's params
// (walk-forward trials).
func (ld *loaded) run(balance float64, from, to time.Time, override map[string]float64) (res *Result, err error) {
	s := &sim{ld: ld, p: ld.p, balance: balance, skipped: map[string]int{}}
	var events []event
	var params []map[string]float64
	for i, d := range ld.legs {
		step := d.candles[d.tfs[0]]
		first := max(d.start, sort.Search(len(step), func(k int) bool { return !step[k].OpenTime.Before(from) }))
		last := sort.Search(len(step), func(k int) bool { return step[k].CloseTime.After(to) }) - 1
		l := &legRun{d: d, frames: map[string]*series{}, lastIdx: last, params: map[string]float64{}}
		for k, v := range d.params {
			l.params[k] = v
		}
		if i == 0 {
			for k, v := range override {
				l.params[k] = v
			}
		}
		params = append(params, l.params)
		anchors := map[string]time.Time{}
		for _, tf := range d.tfs {
			l.frames[tf] = newSeries(tf, d.candles[tf])
			anchors[tf] = time.Date(1970, 1, 4, 0, 0, 0, 0, time.UTC) // a Sunday, MT5's week start
			if cs := d.candles[tf]; len(cs) > 0 {
				anchors[tf] = cs[0].OpenTime
			}
		}
		in := d.inst
		l.ctx = &Context{step: d.tfs[0], tfs: d.tfs, frames: l.frames, anchors: anchors, params: l.params, inst: InstrumentInfo{Name: in.Name, Base: in.Base,
			Quote: in.Quote, Digits: in.Digits, Point: in.Point(), ContractSize: in.ContractSize, LotMin: in.LotMin, LotMax: in.LotMax, LotStep: in.LotStep}}
		s.legs = append(s.legs, l)
		for k := first; k <= last; k++ {
			events = append(events, event{t: step[k].OpenTime, leg: i, idx: k}, event{t: step[k].CloseTime, close: true, leg: i, idx: k})
		}
		if st, ok := d.leg.Strategy.(Starter); ok {
			st.OnStart(StartInfo{Instrument: l.ctx.inst, Timeframes: d.tfs, From: from, To: to, Balance: balance, Currency: ld.p.Currency, Params: l.params})
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
		s.newDay(e.t)
		l := s.legs[e.leg]
		c := l.d.candles[l.d.tfs[0]][e.idx]
		if e.close {
			s.onClose(l, e.idx, c)
		} else {
			s.onOpen(l, c)
		}
	}
	var end time.Time
	for _, l := range s.legs {
		if l.pos != nil {
			exit := l.bid
			if l.pos.side == Short {
				exit = l.ask
			}
			s.closePos(l, exit, l.prevClose, "closed at end", l.prevClose)
		}
		if l.prevClose.After(end) {
			end = l.prevClose
		}
	}
	if !end.IsZero() {
		s.mark(end)
	}
	return s.result(balance, from, to, params), nil
}

func (s *sim) newDay(t time.Time) {
	k := t.In(s.p.DayTZ).Format("2006-01-02")
	if k != s.day {
		s.day, s.dayStart = k, s.equityNow()
	}
}

func (s *sim) floating(l *legRun) float64 {
	p := l.pos
	if p == nil {
		return 0
	}
	exit, sign := l.bid, 1.0
	if p.side == Short {
		exit, sign = l.ask, -1
	}
	return sign*(exit-p.entry)*p.lots*l.d.inst.ContractSize*l.d.conv(exit) + p.swap
}

func (s *sim) equityNow() float64 {
	e := s.balance
	for _, l := range s.legs {
		e += s.floating(l)
	}
	return e
}

func (s *sim) usedMargin() (m float64) {
	for _, l := range s.legs {
		if l.pos != nil {
			m += l.pos.margin
		}
	}
	return m
}

func (s *sim) openCount() (n int) {
	for _, l := range s.legs {
		if l.pos != nil {
			n++
		}
	}
	return n
}

// openRisk is the money open positions lose if all stops are hit now.
func (s *sim) openRisk() (r float64) {
	for _, l := range s.legs {
		if p := l.pos; p != nil {
			adverse := p.entry - p.stop
			if p.side == Short {
				adverse = p.stop - p.entry
			}
			r += math.Max(0, adverse) * p.lots * l.d.inst.ContractSize * l.d.conv(p.stop)
		}
	}
	return r
}

func (s *sim) onOpen(l *legRun, c Candle) {
	l.bid, l.ask = c.Open, c.AskOpen
	if l.pending == nil {
		return
	}
	sig := *l.pending
	slip := l.d.costs.Slippage
	switch sig.Action {
	case ActBuy, ActSell:
		l.pending = nil
		if l.pos == nil {
			s.enter(l, c, sig)
		}
	case ActClose:
		if l.pos == nil {
			l.pending = nil
			return
		}
		if !l.d.inst.Open(c.OpenTime) {
			return // waits for the market to open
		}
		l.pending = nil
		exit := c.Open - slip
		if l.pos.side == Short {
			exit = c.AskOpen + slip
		}
		reason := "close"
		if sig.Reason != "" {
			reason += ": " + sig.Reason
		}
		s.closePos(l, exit, c.OpenTime, reason, c.OpenTime)
	case ActMoveStop:
		l.pending = nil
		if p := l.pos; p != nil && !math.IsNaN(sig.Stop) && sig.Stop > 0 {
			if (p.side == Long && sig.Stop > p.stop) || (p.side == Short && sig.Stop < p.stop) {
				p.stop = sig.Stop
			}
		}
	}
}

func (s *sim) enter(l *legRun, c Candle, sig Signal) {
	d := l.d
	side := Long
	if sig.Action == ActSell {
		side = Short
	}
	miss := func(reason string) {
		s.skipped[reason]++
		s.missed = append(s.missed, Missed{Time: c.OpenTime, Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Side: side, Reason: reason})
	}
	if !d.inst.Open(c.OpenTime) {
		miss(SkipMarketClosed)
		return
	}
	fill := c.AskOpen + d.costs.Slippage
	if side == Short {
		fill = c.Open - d.costs.Slippage
	}
	st, tg := sig.Stop, sig.Target
	if math.IsNaN(st) || math.IsNaN(tg) || st <= 0 || tg < 0 ||
		(side == Long && (st >= fill || (tg > 0 && tg <= fill))) || (side == Short && (st <= fill || (tg > 0 && tg >= fill))) {
		miss(SkipLevels)
		return
	}
	eq := s.equityNow()
	if s.p.MaxDailyLoss > 0 && s.dayStart > 0 && 100*(s.dayStart-eq)/s.dayStart >= s.p.MaxDailyLoss {
		miss(SkipDailyLoss)
		return
	}
	if s.p.MaxOpen > 0 && s.openCount() >= s.p.MaxOpen {
		miss(SkipMaxOpen)
		return
	}
	perLot := math.Abs(fill-st) * d.inst.ContractSize * d.conv(fill)
	lots := lotsFor(s.balance*d.leg.Risk/100/perLot, d.inst)
	if lots == 0 {
		miss(SkipTooSmall)
		return
	}
	risk := lots * perLot
	if s.p.MaxOpenRisk > 0 && 100*(s.openRisk()+risk)/s.balance > s.p.MaxOpenRisk+1e-9 {
		miss(SkipMaxOpenRisk)
		return
	}
	margin := d.margin(lots, fill, s.p.Leverage)
	if margin > eq-s.usedMargin()+1e-9 {
		miss(SkipMargin)
		return
	}
	comm := lots * d.inst.Commission
	s.balance -= comm
	l.pos = &openPos{side: side, entry: fill, stop: st, initStop: st, target: tg, lots: lots, signalTime: l.signalTime, openTime: c.OpenTime,
		risk: risk, margin: margin, commission: comm, lastRoll: c.OpenTime, spread: c.AskOpen - c.Open}
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
	if p := l.pos; p != nil && !p.openTime.After(c.OpenTime) && l.d.inst.Open(c.OpenTime) {
		s.exits(l, c)
	}
	l.bid, l.ask = c.Close, c.AskClose
	if l.pos != nil {
		s.swap(l, c.CloseTime)
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
	x.pos = nil
	if p := l.pos; p != nil {
		x.pos = &Position{Side: p.side, Entry: p.entry, Stop: p.stop, Target: p.target, Lots: p.lots, OpenTime: p.openTime, Risk: p.risk,
			Profit: s.floating(l) - p.commission}
	}
	x.account = Account{Currency: s.p.Currency, Balance: s.balance, Equity: s.equityNow(), Margin: s.usedMargin(), Open: s.openCount()}
	sig := l.d.leg.Strategy.OnCandle(x)
	if sig.Action != ActWait {
		l.pending, l.signalTime = &sig, c.CloseTime
	}
}

// exits checks the stop and the target on c's range; the stop counts first
// when both are inside one candle. A candle opening past a level fills at
// its open.
func (s *sim) exits(l *legRun, c Candle) {
	p := l.pos
	slip := l.d.costs.Slippage
	if p.side == Long {
		switch {
		case c.Low <= p.stop:
			s.closePos(l, math.Min(p.stop, c.Open)-slip, c.CloseTime, "stop", c.OpenTime)
		case p.target > 0 && c.High >= p.target:
			s.closePos(l, math.Max(p.target, c.Open), c.CloseTime, "target", c.OpenTime)
		}
		return
	}
	switch {
	case c.AskHigh >= p.stop:
		s.closePos(l, math.Max(p.stop, c.AskOpen)+slip, c.CloseTime, "stop", c.OpenTime)
	case p.target > 0 && c.AskLow <= p.target:
		s.closePos(l, math.Min(p.target, c.AskOpen), c.CloseTime, "target", c.OpenTime)
	}
}

// swap charges the rollovers in (lastRoll, until].
func (s *sim) swap(l *legRun, until time.Time) {
	p := l.pos
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

func (s *sim) closePos(l *legRun, exit float64, at time.Time, reason string, swapUntil time.Time) {
	s.swap(l, swapUntil)
	p, d := l.pos, l.d
	sign := 1.0
	if p.side == Short {
		sign = -1
	}
	gross := sign * (exit - p.entry) * p.lots * d.inst.ContractSize * d.conv(exit)
	comm := p.lots * d.inst.Commission
	s.balance += gross + p.swap - comm
	t := Trade{Leg: d.i, Strategy: d.name, Instrument: d.inst.Name, Side: p.side, SignalTime: p.signalTime, OpenTime: p.openTime, CloseTime: at,
		Entry: p.entry, Exit: exit, Stop: p.initStop, FinalStop: p.stop, Target: p.target, Lots: p.lots, Spread: p.spread, Risk: p.risk,
		Gross: gross, Commission: p.commission + comm, Swap: p.swap, Reason: reason}
	t.Net = t.Gross - t.Commission + t.Swap
	if t.Risk > 0 {
		t.R = t.Net / t.Risk
	}
	s.trades = append(s.trades, t)
	l.trades = append(l.trades, t)
	l.pos = nil
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
	sort.SliceStable(s.trades, func(i, j int) bool { return s.trades[i].CloseTime.Before(s.trades[j].CloseTime) })
	mid := from.Add(to.Sub(from) / 2)
	res := &Result{Trades: s.trades, Missed: s.missed, Equity: s.equity, Params: params,
		Range: Range{From: from, To: to, RequestedFrom: ld.p.From, RequestedTo: ld.p.To, Notes: append([]string{}, ld.notes...)}}
	res.Stats = computeStats(s.trades, s.equity, balance, mid, s.skipped)
	var names []string
	for _, l := range s.legs {
		d := l.d
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
