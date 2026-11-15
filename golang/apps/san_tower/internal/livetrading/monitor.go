package livetrading

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// Monitor statuses.
const (
	MonitorConnecting   = "connecting"
	MonitorConnected    = "connected"    // the Events stream is up
	MonitorReconnecting = "reconnecting" // it dropped: dialling again, the REST snapshot meanwhile
	MonitorNoConnection = "no_connection"
)

// MonitorIdle is how long a monitor keeps its streams after the last page
// watching it went away.
var MonitorIdle = 30 * time.Second

// monitorBackoff is the longest wait between two dials of a stream, and
// restEvery how often the REST snapshot is read while the Events stream is
// down.
var (
	monitorBackoff = 30 * time.Second
	restEvery      = 10 * time.Second
)

// Position is an open position with its live P/L.
type Position struct {
	ID         string
	Instrument string
	Side       string // buy, sell
	Volume     float64
	OpenPrice  float64
	OpenTime   time.Time
	StopLoss   float64 // 0: none (from the position's SL/TP order)
	TakeProfit float64
	Price      float64 // the price it would close at now: bid for a buy, ask for a sell
	Profit     float64 // floating, in Currency
	Currency   string
	Swap       float64
	Commission float64
}

// Order is a pending order (limit or stop).
type Order struct {
	ID         string
	Instrument string
	Side       string
	Type       string
	Volume     float64
	Price      float64
	StopLoss   float64
	TakeProfit float64
	Created    time.Time
}

// Account is the account's money now.
type Account struct {
	ID         string
	Name       string
	Currency   string
	Leverage   string
	Balance    float64
	Equity     float64
	Margin     float64
	FreeMargin float64
}

// MonitorState is what the monitor shows.
type MonitorState struct {
	ConnectionID uint32
	Connection   string
	Status       string
	Message      string
	Account      Account
	Positions    []Position
	Orders       []Order
	Updated      time.Time
	// Source says where the positions came from last: "stream" or "rest".
	Source string
}

// MonitorEvent is one thing a watcher gets: a state or a tick.
type MonitorEvent struct {
	State *MonitorState
	Tick  *exness.Tick
}

// Monitor watches one Exness connection, read-only.
type Monitor struct {
	id   uint32
	name string
	api  Exness

	mu        sync.Mutex
	status    string
	message   string
	account   Account
	state     exness.AccountState
	positions map[string]exness.Position
	orders    map[string]exness.Order
	ticks     map[string]exness.Tick
	conds     map[string]exness.InstrumentCondition
	source    string
	updated   time.Time
	want      map[string]int // instruments wanted for ticks, by watchers
	subbed    map[string]bool
	tickConn  *exness.Stream
	resub     chan struct{}
	watchers  map[int]chan MonitorEvent
	next      int
	cancel    context.CancelFunc
	idle      *time.Timer
	done      chan struct{}
	onStopped func()
}

func newMonitor(id uint32, name string, api Exness) *Monitor {
	return &Monitor{id: id, name: name, api: api, status: MonitorConnecting, positions: map[string]exness.Position{}, orders: map[string]exness.Order{},
		ticks: map[string]exness.Tick{}, conds: map[string]exness.InstrumentCondition{}, want: map[string]int{}, subbed: map[string]bool{},
		resub: make(chan struct{}, 1), watchers: map[int]chan MonitorEvent{}}
}

// start runs the streams until stop.
func (m *Monitor) start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel, m.done = cancel, make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); m.events(ctx) }()
	go func() { defer wg.Done(); m.tickLoop(ctx) }()
	go func() { wg.Wait(); close(m.done) }()
}

func (m *Monitor) stop() {
	if m.cancel != nil {
		m.cancel()
		<-m.done
	}
}

// Watch adds a watcher wanting ticks of instruments; the channel gets the
// state at once, then each change. Call the returned func to leave.
func (m *Monitor) Watch(instruments []string) (<-chan MonitorEvent, func()) {
	ch := make(chan MonitorEvent, 256)
	m.mu.Lock()
	id := m.next
	m.next++
	m.watchers[id] = ch
	if m.idle != nil {
		m.idle.Stop()
		m.idle = nil
	}
	m.wantLocked(instruments, 1)
	st := m.stateLocked()
	ticks := slices.Collect(maps.Values(m.ticks))
	m.mu.Unlock()
	ch <- MonitorEvent{State: &st}
	for i := range ticks {
		if len(ch) < cap(ch)-1 {
			ch <- MonitorEvent{Tick: &ticks[i]}
		}
	}
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			delete(m.watchers, id)
			m.wantLocked(instruments, -1)
			if len(m.watchers) == 0 && m.onStopped != nil {
				m.idle = time.AfterFunc(MonitorIdle, m.onStopped)
			}
		})
	}
}

// Want adds instruments to the ticks of a watcher already there (the
// chart's instrument changed).
func (m *Monitor) Want(instruments []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wantLocked(instruments, 1)
}

func (m *Monitor) wantLocked(instruments []string, d int) {
	grew := false
	for _, n := range instruments {
		if n = strings.TrimSpace(n); n == "" {
			continue
		}
		m.want[n] += d
		if m.want[n] <= 0 {
			delete(m.want, n)
		} else if !m.subbed[n] {
			grew = true
		}
	}
	if grew {
		select {
		case m.resub <- struct{}{}:
		default:
		}
	}
}

// State is the state now.
func (m *Monitor) State() MonitorState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateLocked()
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func (m *Monitor) stateLocked() MonitorState {
	st := MonitorState{ConnectionID: m.id, Connection: m.name, Status: m.status, Message: m.message, Account: m.account, Updated: m.updated, Source: m.source}
	st.Account.Balance, st.Account.Equity, st.Account.Margin = num(m.state.Balance), num(m.state.Equity), num(m.state.UsedMargin)
	st.Account.FreeMargin = st.Account.Equity - st.Account.Margin
	sl := map[string][2]float64{}
	for _, o := range m.orders {
		if o.OrderType == "sltp" && o.PositionID != "" {
			sl[o.PositionID] = [2]float64{num(o.StopLossPrice), num(o.TakeProfitPrice)}
		}
	}
	for _, p := range m.positions {
		if p.State == "closed" {
			continue
		}
		x := Position{ID: p.PositionID, Instrument: p.Instrument, Side: string(p.Direction), Volume: num(p.Volume), OpenPrice: num(p.OpenPrice),
			OpenTime: p.CreateTime, Swap: num(p.Swap), Commission: num(p.Commission), Currency: m.account.Currency}
		x.StopLoss, x.TakeProfit = sl[p.PositionID][0], sl[p.PositionID][1]
		if t, ok := m.ticks[p.Instrument]; ok {
			x.Price, x.Profit, x.Currency = m.profit(p, x, t)
		}
		st.Positions = append(st.Positions, x)
	}
	sort.Slice(st.Positions, func(i, j int) bool { return st.Positions[i].OpenTime.Before(st.Positions[j].OpenTime) })
	for _, o := range m.orders {
		if o.OrderType == "sltp" || (o.State != "placed" && o.State != "pending_new" && o.State != "") {
			continue
		}
		st.Orders = append(st.Orders, Order{ID: o.OrderID, Instrument: o.Instrument, Side: string(o.Direction), Type: o.OrderType, Volume: num(o.Volume),
			Price: num(o.Price), StopLoss: num(o.StopLossPrice), TakeProfit: num(o.TakeProfitPrice), Created: o.CreateTime})
	}
	sort.Slice(st.Orders, func(i, j int) bool { return st.Orders[i].Created.Before(st.Orders[j].Created) })
	return st
}

// profit is a position's floating P/L at tick t: the price move times the
// volume and the contract size, in the quote currency, converted when the
// account currency is the base (USDJPY in USD); left in the quote currency
// otherwise (the currency says which).
func (m *Monitor) profit(p exness.Position, x Position, t exness.Tick) (price, profit float64, ccy string) {
	price, sign := t.Bid, 1.0
	if p.Direction == exness.Sell {
		price, sign = t.Ask, -1
	}
	c, ok := m.conds[p.Instrument]
	if !ok {
		return price, 0, m.account.Currency
	}
	profit = sign * (price - x.OpenPrice) * x.Volume * num(c.ContractSize)
	ccy = c.QuoteCurrency
	if ccy != m.account.Currency && c.BaseCurrency == m.account.Currency && price > 0 {
		profit, ccy = profit/price, m.account.Currency
	}
	return price, profit, ccy
}

// publish sends the state to every watcher (one whose buffer is full
// misses it; the next state is whole anyway).
func (m *Monitor) publishLocked(ev MonitorEvent) {
	for _, ch := range m.watchers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (m *Monitor) changed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated = time.Now()
	st := m.stateLocked()
	m.publishLocked(MonitorEvent{State: &st})
}

func (m *Monitor) setStatus(status, message string) {
	m.mu.Lock()
	m.status, m.message = status, message
	m.mu.Unlock()
	m.changed()
}

// conditions reads the conditions of instruments not seen yet (for the
// contract size), read-only.
func (m *Monitor) conditions(ctx context.Context, names []string) {
	for _, n := range names {
		m.mu.Lock()
		_, ok := m.conds[n]
		m.mu.Unlock()
		if ok || n == "" {
			continue
		}
		c, err := m.api.Instrument(ctx, n)
		if err != nil {
			continue
		}
		m.mu.Lock()
		m.conds[n] = c
		m.mu.Unlock()
	}
}

// snapshot replaces the positions and orders with s (a new stream's
// snapshot, or the REST one).
func (m *Monitor) snapshot(ctx context.Context, s exness.Snapshot, source string) {
	m.mu.Lock()
	m.positions, m.orders = map[string]exness.Position{}, map[string]exness.Order{}
	var names []string
	for _, p := range s.Positions {
		m.positions[p.PositionID] = p
		names = append(names, p.Instrument)
	}
	for _, o := range s.Orders {
		m.orders[o.OrderID] = o
		names = append(names, o.Instrument)
	}
	if s.AccountState.Balance != "" || s.AccountState.Equity != "" {
		m.state = s.AccountState
	}
	m.source = source
	m.mu.Unlock()
	m.conditions(ctx, names)
	m.wantPositions()
	m.changed()
}

// wantPositions keeps the positions' and orders' instruments in the ticks.
func (m *Monitor) wantPositions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	grew := false
	for _, p := range m.positions {
		if !m.subbed[p.Instrument] {
			grew = true
		}
	}
	for _, o := range m.orders {
		if o.Instrument != "" && !m.subbed[o.Instrument] {
			grew = true
		}
	}
	if grew {
		select {
		case m.resub <- struct{}{}:
		default:
		}
	}
}

// apply takes one transaction event: the positions and orders it names
// are upserted, closed or finished ones removed.
func (m *Monitor) apply(ctx context.Context, e *exness.TransactionEvent) {
	if e.Err() != nil {
		return
	}
	m.mu.Lock()
	var names []string
	for _, p := range e.Payload.Positions {
		if p.State == "closed" {
			delete(m.positions, p.PositionID)
			continue
		}
		if old, ok := m.positions[p.PositionID]; ok {
			p = mergePosition(old, p)
		}
		m.positions[p.PositionID] = p
		names = append(names, p.Instrument)
	}
	for _, o := range e.Payload.Orders {
		switch o.State {
		case "filled", "cancelled", "rejected", "expired":
			delete(m.orders, o.OrderID)
		default:
			m.orders[o.OrderID] = o
			names = append(names, o.Instrument)
		}
	}
	if a := e.Payload.AccountState; a != nil {
		m.state = *a
	}
	m.mu.Unlock()
	m.conditions(ctx, names)
	m.wantPositions()
	m.changed()
}

// mergePosition keeps old's fields that an event leaves out (an event
// holds only what changed).
func mergePosition(old, p exness.Position) exness.Position {
	if p.Instrument == "" {
		p.Instrument = old.Instrument
	}
	if p.Direction == "" {
		p.Direction = old.Direction
	}
	if p.OpenPrice == "" {
		p.OpenPrice = old.OpenPrice
	}
	if p.Volume == "" {
		p.Volume = old.Volume
	}
	if p.CreateTime.IsZero() {
		p.CreateTime = old.CreateTime
	}
	return p
}

// events keeps the Events stream up: dial, subscribe, take the snapshot
// and the changes; on a drop, read the REST snapshot and dial again (the
// stream has no replay, so the new snapshot rebuilds everything).
func (m *Monitor) events(ctx context.Context) {
	if d, err := m.api.Account(ctx); err == nil {
		m.mu.Lock()
		m.account = Account{ID: d.AccountID, Name: d.AccountName, Currency: d.Settings.Currency, Leverage: d.Settings.Leverage}
		m.mu.Unlock()
	}
	wait := time.Second
	for ctx.Err() == nil {
		st, err := m.api.Events(ctx)
		if err == nil {
			err = st.Subscribe(ctx, "transactions", exness.Transactions())
			if err == nil {
				err = st.Subscribe(ctx, "account", exness.AccountStateUpdates())
			}
			if err == nil {
				m.setStatus(MonitorConnected, "")
				wait = time.Second
				err = m.readEvents(ctx, st)
			}
			st.Close()
		}
		if ctx.Err() != nil {
			return
		}
		m.setStatus(MonitorReconnecting, fmt.Sprintf("Exness events stream: %v; dialling again, the positions come from the REST snapshot meanwhile", short(err)))
		m.rest(ctx)
		deadline := time.Now().Add(wait)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			pause(ctx, min(restEvery, time.Until(deadline)))
			if time.Now().Before(deadline) {
				m.rest(ctx)
			}
		}
		wait = min(2*wait, monitorBackoff)
	}
}

func (m *Monitor) readEvents(ctx context.Context, st *exness.Stream) error {
	for {
		msg, err := st.Next(ctx)
		if err != nil {
			return err
		}
		switch {
		case msg.Snapshot != nil:
			m.snapshot(ctx, *msg.Snapshot, "stream")
		case msg.Transaction != nil:
			m.apply(ctx, msg.Transaction)
		case msg.AccountState != nil:
			m.mu.Lock()
			m.state = msg.AccountState.State
			m.mu.Unlock()
			m.changed()
		case msg.Error != nil:
			m.setStatus(MonitorConnected, "Exness refused a subscription: "+msg.Error.Message)
		}
	}
}

// rest reads the REST snapshot (the fallback while the stream is down).
func (m *Monitor) rest(ctx context.Context) {
	s, err := m.api.Snapshot(ctx)
	if err != nil {
		return
	}
	m.snapshot(ctx, s, "rest")
}

// tickLoop keeps the Ticks stream up for the instruments wanted.
func (m *Monitor) tickLoop(ctx context.Context) {
	wait := time.Second
	for ctx.Err() == nil {
		if len(m.tickNames(false)) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-m.resub:
			}
			continue
		}
		st, err := m.api.Ticks(ctx)
		if err != nil {
			pause(ctx, wait)
			wait = min(2*wait, monitorBackoff)
			continue
		}
		wait = time.Second
		m.mu.Lock()
		m.subbed, m.tickConn = map[string]bool{}, st
		m.mu.Unlock()
		sctx, cancel := context.WithCancel(ctx)
		go func() {
			m.subscribeTicks(sctx, st)
			for {
				select {
				case <-sctx.Done():
					return
				case <-m.resub:
					m.subscribeTicks(sctx, st)
				}
			}
		}()
		for {
			msg, err := st.Next(ctx)
			if err != nil {
				break
			}
			if msg.Tick == nil {
				continue
			}
			t := *msg.Tick
			m.mu.Lock()
			m.ticks[t.Instrument] = t
			m.publishLocked(MonitorEvent{Tick: &t})
			m.mu.Unlock()
		}
		cancel()
		st.Close()
		m.mu.Lock()
		m.tickConn = nil
		m.mu.Unlock()
	}
}

// tickNames are the instruments wanted (all, or only those not subscribed
// yet).
func (m *Monitor) tickNames(newOnly bool) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := map[string]bool{}
	for n := range m.want {
		set[n] = true
	}
	for _, p := range m.positions {
		set[p.Instrument] = true
	}
	for _, o := range m.orders {
		if o.Instrument != "" {
			set[o.Instrument] = true
		}
	}
	var out []string
	for n := range set {
		if n != "" && (!newOnly || !m.subbed[n]) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func (m *Monitor) subscribeTicks(ctx context.Context, st *exness.Stream) {
	names := m.tickNames(true)
	if len(names) == 0 {
		return
	}
	if err := st.Subscribe(ctx, "ticks-"+strings.Join(names, "-"), exness.TicksOf(names...)); err != nil {
		return
	}
	m.mu.Lock()
	for _, n := range names {
		m.subbed[n] = true
	}
	m.mu.Unlock()
}

func pause(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func short(err error) string {
	if err == nil {
		return "closed"
	}
	return strings.TrimPrefix(err.Error(), "exness: ")
}
