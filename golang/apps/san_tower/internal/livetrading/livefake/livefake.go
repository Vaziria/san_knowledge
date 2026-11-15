// Package livefake is a fake Exness Access Point for the Live Trading
// tests and replay checks: REST (account, instrument conditions, candles,
// the trading snapshot) and the two WebSocket streams (events: the
// snapshot, transactions and the account state every AccountEvery; ticks:
// bid and ask of the subscribed instruments every TickEvery). Its candles
// are given (real cached ones in a replay check) and served on its clock,
// so the forming candle is left out. It refuses every trading route with
// 403 and counts them in Refused: the Live Trading page must never send
// one. It does not check signatures (exnesstest does).
package livefake

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	exness "github.com/wargasipil/san_exness"
)

// The fake's account, and the credentials a connection to it uses (the
// RFC 8032 test key, as exnesstest's: not a credential).
const (
	Account    = "123456"
	APIKey     = "exnsk-livefake"
	PrivateKey = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60" +
		"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"
)

// Candle is one candle of the fake's market (bid and ask).
type Candle struct {
	Time                   time.Time // open
	Open, High, Low, Close float64   // bid
	Spread                 float64   // ask - bid
	Volume                 float64
}

// Fake is the fake Access Point.
type Fake struct {
	URL string

	// Clock is the fake's time (default time.Now); candles and ticks follow it.
	Clock        func() time.Time
	AccountEvery time.Duration // default 2 s
	TickEvery    time.Duration // default 500 ms

	srv *httptest.Server

	mu        sync.Mutex
	candles   map[string]map[string][]Candle // instrument, timeframe
	conds     map[string]exness.InstrumentCondition
	balance   float64
	positions []exness.Position
	orders    []exness.Order
	calls     []string
	refused   []string
	conns     map[*websocket.Conn]context.CancelFunc
	events    []chan []byte // the open events streams' queues
	down      bool          // REST candles answer 503 (a network drop)
}

// SetDown makes the candle requests fail (true) or work again.
func (f *Fake) SetDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

// New starts a fake (stop it with Close) with an XAUUSD instrument and
// 10000 USD; addr is where it listens ("" any loopback port).
func New(addr string) (*Fake, error) {
	f := &Fake{Clock: time.Now, candles: map[string]map[string][]Candle{}, conds: map[string]exness.InstrumentCondition{}, balance: 10000,
		conns: map[*websocket.Conn]context.CancelFunc{}}
	f.conds["XAUUSD"] = exness.InstrumentCondition{Instrument: "XAUUSD", BaseCurrency: "XAU", QuoteCurrency: "USD", MarginCurrency: "XAU", PointDigits: 3,
		ContractSize: "100", MarginPercent: "100", VolumeMin: "0.01", VolumeMax: "200", VolumeStep: "0.01", SwapLong: "-471.7", SwapShort: "0",
		SwapMultipliers: []exness.SwapMultiplier{{Day: "wednesday", Multiplier: 3}}, ExecMode: "market", TradeMode: "enabled",
		OrderTypes: []string{"market", "limit", "stop"}, CommissionSettings: exness.CommissionSettings{}}
	f.srv = httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	if addr != "" {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		f.srv.Listener = ln
	}
	f.srv.StartTLS()
	f.URL = f.srv.URL
	return f, nil
}

// Close stops it.
func (f *Fake) Close() {
	f.DropStreams()
	f.srv.Close()
}

// Client trusts the fake's certificate.
func (f *Fake) Client() *http.Client { return f.srv.Client() }

// CertPEM is the fake's certificate (for SAN_TOWER_EXNESS_CA).
func (f *Fake) CertPEM() []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.srv.Certificate().Raw})
}

// TLS is the fake's TLS config, for a client of its own.
func (f *Fake) TLS() *tls.Config { return f.srv.Client().Transport.(*http.Transport).TLSClientConfig }

// Config is a san_exness config for the fake.
func (f *Fake) Config() exness.Config {
	return exness.Config{APIKey: APIKey, PrivateKey: PrivateKey, AccountID: Account, AccessPoint: f.URL, HTTPClient: f.Client()}
}

// SetCandles gives an instrument's candles of a timeframe, oldest first.
func (f *Fake) SetCandles(instrument, tf string, cs []Candle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.candles[instrument] == nil {
		f.candles[instrument] = map[string][]Candle{}
	}
	f.candles[instrument][tf] = cs
	if _, ok := f.conds[instrument]; !ok {
		c := f.conds["XAUUSD"]
		c.Instrument = instrument
		f.conds[instrument] = c
	}
}

// AddPosition opens a position in the fake's account (as if placed in
// Exness's own terminal) and tells the events streams.
func (f *Fake) AddPosition(instrument string, side exness.Side, volume, openPrice, sl, tp float64) string {
	f.mu.Lock()
	id := strconv.Itoa(1000 + len(f.positions) + len(f.orders))
	r := func(v float64) float64 { return math.Round(v*1000) / 1000 }
	openPrice, sl, tp = r(openPrice), r(sl), r(tp)
	p := exness.Position{PositionID: id, Direction: side, Instrument: instrument, Volume: num(volume), InitialVolume: num(volume), OpenPrice: num(openPrice),
		State: "open", CreateTime: f.Clock().UTC()}
	f.positions = append(f.positions, p)
	var os []exness.Order
	if sl > 0 || tp > 0 {
		o := exness.Order{OrderID: "sl" + id, PositionID: id, OrderType: "sltp", State: "placed", Instrument: instrument, StopLossPrice: num(sl),
			TakeProfitPrice: num(tp), CreateTime: p.CreateTime}
		f.orders = append(f.orders, o)
		os = append(os, o)
	}
	f.mu.Unlock()
	f.transaction([]exness.Position{p}, os)
	return id
}

// AddOrder adds a pending order.
func (f *Fake) AddOrder(instrument string, side exness.Side, typ string, volume, price, sl, tp float64) string {
	f.mu.Lock()
	id := strconv.Itoa(2000 + len(f.positions) + len(f.orders))
	r := func(v float64) float64 { return math.Round(v*1000) / 1000 }
	price, sl, tp = r(price), r(sl), r(tp)
	o := exness.Order{OrderID: id, OrderType: typ, State: "placed", Direction: side, Instrument: instrument, Volume: num(volume), Price: num(price),
		StopLossPrice: num(sl), TakeProfitPrice: num(tp), CreateTime: f.Clock().UTC()}
	f.orders = append(f.orders, o)
	f.mu.Unlock()
	f.transaction(nil, []exness.Order{o})
	return id
}

// RemovePosition ends a position in the fake (as if its stop was hit in
// Exness's own terminal) and tells the events streams.
func (f *Fake) RemovePosition(id string) {
	f.mu.Lock()
	var gone []exness.Position
	keep := f.positions[:0]
	for _, p := range f.positions {
		if p.PositionID == id {
			p.State = "closed"
			gone = append(gone, p)
			continue
		}
		keep = append(keep, p)
	}
	f.positions = keep
	f.mu.Unlock()
	f.transaction(gone, nil)
}

// Calls are the requests so far ("GET /v1/..."); Refused the trading ones
// it refused.
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func (f *Fake) Refused() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.refused...)
}

// DropStreams cuts every open stream (a network drop).
func (f *Fake) DropStreams() {
	f.mu.Lock()
	for c, cancel := range f.conns {
		cancel()
		c.Close(websocket.StatusGoingAway, "dropped")
	}
	f.conns = map[*websocket.Conn]context.CancelFunc{}
	f.mu.Unlock()
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func fnum(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }

var tfs = map[string]time.Duration{"M1": time.Minute, "M5": 5 * time.Minute, "M15": 15 * time.Minute, "M30": 30 * time.Minute, "H1": time.Hour,
	"H4": 4 * time.Hour, "D1": 24 * time.Hour}

// price is the instrument's bid now: the close of the newest candle of the
// shortest timeframe that has closed, with a small wiggle so ticks move.
func (f *Fake) price(instrument string) (bid, ask float64, ok bool) {
	now := f.Clock()
	best := time.Duration(math.MaxInt64)
	for tf, cs := range f.candles[instrument] {
		d := tfs[tf]
		if d == 0 || d >= best {
			continue
		}
		i := sort.Search(len(cs), func(k int) bool { return cs[k].Time.Add(d).After(now) }) - 1
		if i < 0 {
			continue
		}
		c := cs[i]
		best = d
		w := 0.3 * math.Sin(float64(now.UnixMilli())/700)
		bid, ask, ok = c.Close+w, c.Close+w+c.Spread, true
	}
	return
}

func (f *Fake) equity() float64 {
	e := f.balance
	for _, p := range f.positions {
		bid, ask, ok := f.price(p.Instrument)
		if !ok {
			continue
		}
		c := fnum(f.conds[p.Instrument].ContractSize)
		if p.Direction == exness.Sell {
			e += (fnum(p.OpenPrice) - ask) * fnum(p.Volume) * c
		} else {
			e += (bid - fnum(p.OpenPrice)) * fnum(p.Volume) * c
		}
	}
	return e
}

func (f *Fake) accountState() exness.AccountState {
	m := 0.0
	for _, p := range f.positions {
		bid, _, _ := f.price(p.Instrument)
		m += fnum(p.Volume) * fnum(f.conds[p.Instrument].ContractSize) * bid / 2000
	}
	return exness.AccountState{Balance: num(math.Round(f.balance*100) / 100), Equity: num(math.Round(f.equity()*100) / 100), UsedMargin: num(math.Round(m*100) / 100)}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.mu.Unlock()
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")
	if len(parts) < 4 || parts[1] != "accounts" || parts[2] != Account {
		writeJSON(w, 404, map[string]any{"code": 1, "error_message": "not found"})
		return
	}
	route := parts[0] + "/" + strings.Join(parts[3:], "/")
	switch {
	case r.Method == "GET" && route == "server-events/ws/events":
		f.stream(w, r, true)
	case r.Method == "GET" && route == "server-events/ws/ticks":
		f.stream(w, r, false)
	case r.Method == "GET" && route == "configuration/account":
		writeJSON(w, 200, exness.AccountDetails{AccountID: Account, AccountName: "Fake replay", Settings: exness.AccountSettings{Currency: "USD",
			Leverage: "2000", TradeMode: "enabled", AccountStatus: "active", ExecutionMode: "market"}})
	case r.Method == "GET" && len(parts) == 6 && parts[0] == "configuration" && parts[3] == "instruments" && parts[5] == "conditions":
		f.mu.Lock()
		c, ok := f.conds[parts[4]]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, map[string]any{"code": exness.CodeMarketInstrumentNotFound, "error_message": "no such instrument"})
			return
		}
		writeJSON(w, 200, c)
	case r.Method == "GET" && route == "market-data/candles":
		f.mu.Lock()
		down := f.down
		f.mu.Unlock()
		if down {
			writeJSON(w, 503, map[string]any{"code": 1, "error_message": "the fake is down"})
			return
		}
		f.serveCandles(w, r)
	case r.Method == "GET" && route == "trading/snapshot":
		f.mu.Lock()
		s := exness.Snapshot{Positions: append([]exness.Position{}, f.positions...), Orders: append([]exness.Order{}, f.orders...), AccountState: f.accountState()}
		f.mu.Unlock()
		writeJSON(w, 200, s)
	case strings.HasPrefix(route, "trading/"):
		f.mu.Lock()
		f.refused = append(f.refused, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		writeJSON(w, 403, map[string]any{"code": 1, "error_message": "the fake takes no trading requests"})
	default:
		writeJSON(w, 404, map[string]any{"code": 1, "error_message": "not found"})
	}
}

func (f *Fake) serveCandles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	tf := q.Get("timeframe")
	d := tfs[tf]
	from, err := time.Parse(time.RFC3339, q.Get("from"))
	if d == 0 || err != nil {
		writeJSON(w, 400, map[string]any{"code": exness.CodeRequestInvalidParameters, "error_message": "bad query"})
		return
	}
	now := f.Clock()
	f.mu.Lock()
	all := f.candles[q.Get("instrument")][tf]
	f.mu.Unlock()
	// the candles that have opened by now; the newest may be forming
	open := sort.Search(len(all), func(k int) bool { return all[k].Time.After(now) })
	all = all[:open]
	var pick []Candle
	if to := q.Get("to"); to != "" {
		t, _ := time.Parse(time.RFC3339, to)
		i := sort.Search(len(all), func(k int) bool { return !all[k].Time.Before(from) })
		for ; i < len(all) && all[i].Time.Before(t); i++ {
			pick = append(pick, all[i])
		}
	} else {
		n, _ := strconv.Atoi(q.Get("count"))
		if n >= 0 {
			i := sort.Search(len(all), func(k int) bool { return !all[k].Time.Before(from) })
			pick = all[i:min(len(all), i+n)]
		} else {
			j := sort.Search(len(all), func(k int) bool { return all[k].Time.After(from) })
			pick = all[max(0, j+n):j]
		}
	}
	out := []map[string]any{}
	for _, c := range pick {
		m := map[string]any{"time": c.Time.UTC().Format(time.RFC3339), "volume": c.Volume}
		bid := map[string]float64{"open": c.Open, "high": c.High, "low": c.Low, "close": c.Close}
		ask := map[string]float64{"open": c.Open + c.Spread, "high": c.High + c.Spread, "low": c.Low + c.Spread, "close": c.Close + c.Spread}
		switch q.Get("price_type") {
		case "both":
			m["bid"], m["ask"] = bid, ask
		case "ask":
			for k, v := range ask {
				m[k] = v
			}
		default:
			for k, v := range bid {
				m[k] = v
			}
		}
		out = append(out, m)
	}
	writeJSON(w, 200, map[string]any{"candles": out})
}

type command struct {
	ID        string               `json:"id"`
	Subscribe *exness.Subscription `json:"subscribe"`
}

// transaction tells every events stream of changed positions and orders.
func (f *Fake) transaction(ps []exness.Position, os []exness.Order) {
	f.mu.Lock()
	defer f.mu.Unlock()
	msg := map[string]any{"event_type": exness.EventTransaction, "status": "success", "event_time": f.Clock().UTC(),
		"payload": map[string]any{"positions": ps, "orders": os, "account_state": f.accountState()}}
	b, _ := json.Marshal(msg)
	for _, q := range f.events {
		select {
		case q <- b:
		default:
		}
	}
}

func (f *Fake) stream(w http.ResponseWriter, r *http.Request, events bool) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{exness.Subprotocol}})
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	q := make(chan []byte, 64)
	f.mu.Lock()
	f.conns[c] = cancel
	if events {
		f.events = append(f.events, q)
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.conns, c)
		for i, x := range f.events {
			if x == q {
				f.events = append(f.events[:i], f.events[i+1:]...)
				break
			}
		}
		f.mu.Unlock()
		c.Close(websocket.StatusNormalClosure, "")
	}()
	var mu sync.Mutex
	subs := map[string]bool{}
	account := false
	go func() {
		for {
			_, b, err := c.Read(ctx)
			if err != nil {
				cancel()
				return
			}
			var cmd command
			if json.Unmarshal(b, &cmd) != nil || cmd.Subscribe == nil {
				continue
			}
			mu.Lock()
			switch cmd.Subscribe.Event {
			case "transactions":
				f.mu.Lock()
				snap := map[string]any{"event_type": exness.EventSnapshot, "payload": exness.Snapshot{Positions: append([]exness.Position{}, f.positions...),
					Orders: append([]exness.Order{}, f.orders...), AccountState: f.accountState()}}
				f.mu.Unlock()
				sb, _ := json.Marshal(snap)
				q <- sb
			case "account_state":
				account = true
			case "ticks":
				for _, n := range cmd.Subscribe.Instruments {
					subs[n] = true
				}
			}
			mu.Unlock()
		}
	}()
	acc := f.AccountEvery
	if acc <= 0 {
		acc = 2 * time.Second
	}
	tick := f.TickEvery
	if tick <= 0 {
		tick = 500 * time.Millisecond
	}
	at := time.NewTicker(acc)
	defer at.Stop()
	tt := time.NewTicker(tick)
	defer tt.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-q:
			if c.Write(ctx, websocket.MessageText, b) != nil {
				return
			}
		case <-at.C:
			mu.Lock()
			send := events && account
			mu.Unlock()
			if !send {
				continue
			}
			f.mu.Lock()
			b, _ := json.Marshal(map[string]any{"event_type": exness.EventAccountState, "event_time": f.Clock().UTC(),
				"payload": map[string]any{"account_state": f.accountState()}})
			f.mu.Unlock()
			if c.Write(ctx, websocket.MessageText, b) != nil {
				return
			}
		case <-tt.C:
			if events {
				continue
			}
			mu.Lock()
			names := make([]string, 0, len(subs))
			for n := range subs {
				names = append(names, n)
			}
			mu.Unlock()
			for _, n := range names {
				f.mu.Lock()
				bid, ask, ok := f.price(n)
				now := f.Clock().UTC()
				f.mu.Unlock()
				if !ok {
					continue
				}
				b, _ := json.Marshal(exness.Tick{Instrument: n, Bid: math.Round(bid*1000) / 1000, Ask: math.Round(ask*1000) / 1000, Timestamp: now})
				if c.Write(ctx, websocket.MessageText, b) != nil {
					return
				}
			}
		}
	}
}

// String names the fake.
func (f *Fake) String() string { return fmt.Sprintf("fake Exness at %s", f.URL) }
