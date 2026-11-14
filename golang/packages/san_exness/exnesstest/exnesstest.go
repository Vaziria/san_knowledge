// Package exnesstest is a fake Exness Access Point for the tests of
// programs that use san_exness. It checks every request's signature as
// Exness documents it, and keeps an account in memory: instruments with a
// bid and an ask, positions, pending orders, SL/TP orders, deals and
// operations. Trading operations are confirmed at once, or rejected when
// RejectNext says so. It has no streams.
package exnesstest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// The fake's credentials. PrivateKey is RFC 8032's public test key (Test
// 1), also Exness's documented signing test key: not a credential.
const (
	APIKey     = "exnsk-exnesstest"
	PrivateKey = "9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60" +
		"d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a"
	Account = "123456"
)

type instrument struct {
	cond     exness.InstrumentCondition
	bid, ask float64
}

// Server is the fake. Its zero value is not usable: make one with New.
type Server struct {
	URL string // the Access Point, https

	srv *httptest.Server
	pub ed25519.PublicKey

	mu          sync.Mutex
	instruments map[string]*instrument
	balance     float64
	positions   []exness.Position
	orders      []exness.Order // pending and sltp
	history     []exness.Order // filled, cancelled
	deals       []exness.Deal
	ops         map[string]exness.Operation
	next        int
	reject      []int
	calls       []string
}

// New starts a fake with 10000 USD and three instruments: EURUSD (1.08000
// / 1.08010), XAUUSD (2650.000 / 2650.200) and USDJPY (150.000 / 150.010,
// its profit in JPY). It stops when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	raw, _ := hex.DecodeString(PrivateKey)
	s := &Server{
		pub:         ed25519.PublicKey(raw[32:]),
		instruments: map[string]*instrument{},
		balance:     10000,
		ops:         map[string]exness.Operation{},
		next:        100,
	}
	forex := func(name, quote string, digits int, contract string) exness.InstrumentCondition {
		return exness.InstrumentCondition{Instrument: name, BaseCurrency: name[:3], QuoteCurrency: quote, MarginCurrency: name[:3],
			PointDigits: digits, ContractSize: contract, VolumeMin: "0.01", VolumeMax: "200", VolumeStep: "0.01",
			ExecMode: "market", TradeMode: "enabled", OrderTypes: []string{"market", "limit", "stop"}}
	}
	s.AddInstrument(forex("EURUSD", "USD", 5, "100000"), 1.08000, 1.08010)
	s.AddInstrument(forex("XAUUSD", "USD", 3, "100"), 2650.000, 2650.200)
	s.AddInstrument(forex("USDJPY", "JPY", 3, "100000"), 150.000, 150.010)
	s.srv = httptest.NewTLSServer(http.HandlerFunc(s.serve))
	s.URL = s.srv.URL
	t.Cleanup(s.srv.Close)
	return s
}

// Client is an HTTP client that trusts the fake's certificate.
func (s *Server) Client() *http.Client { return s.srv.Client() }

// Config is a san_exness config for the fake's account.
func (s *Server) Config() exness.Config {
	return exness.Config{APIKey: APIKey, PrivateKey: PrivateKey, AccountID: Account, AccessPoint: s.URL, HTTPClient: s.Client()}
}

// AddInstrument adds or replaces an instrument and its prices.
func (s *Server) AddInstrument(c exness.InstrumentCondition, bid, ask float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instruments[c.Instrument] = &instrument{cond: c, bid: bid, ask: ask}
}

// SetPrice moves an instrument's bid and ask.
func (s *Server) SetPrice(name string, bid, ask float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instruments[name].bid, s.instruments[name].ask = bid, ask
}

// RejectNext rejects the next trading operation with code (the ACK is
// given; the operation's status says rejected).
func (s *Server) RejectNext(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reject = append(s.reject, code)
}

// AddPosition opens a position directly, as if placed by hand; it returns
// its id.
func (s *Server) AddPosition(name string, side exness.Side, volume, openPrice string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openPosition(name, side, volume, openPrice, "", "")
}

// Snapshot is the account's state now.
func (s *Server) Snapshot() exness.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot()
}

// Calls are the requests that reached the fake, "METHOD /path?query",
// those starting with prefix.
func (s *Server) Calls(prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, c := range s.calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func (s *Server) id() string {
	s.next++
	return strconv.Itoa(s.next)
}

func (s *Server) price(name string, v float64) string {
	return strconv.FormatFloat(v, 'f', s.instruments[name].cond.PointDigits, 64)
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (s *Server) snapshot() exness.Snapshot {
	equity := s.balance
	for _, p := range s.positions {
		equity += s.profit(p, p.Volume)
	}
	return exness.Snapshot{
		Orders:    append([]exness.Order{}, s.orders...),
		Positions: append([]exness.Position{}, s.positions...),
		AccountState: exness.AccountState{
			Balance: fmt.Sprintf("%.2f", s.balance), Equity: fmt.Sprintf("%.2f", equity), UsedMargin: fmt.Sprintf("%.2f", float64(len(s.positions))*10),
		},
	}
}

// profit is what closing volume of p at the current price makes, in the
// quote currency.
func (s *Server) profit(p exness.Position, volume string) float64 {
	in := s.instruments[p.Instrument]
	exit, sign := in.bid, 1.0
	if p.Direction == exness.Sell {
		exit, sign = in.ask, -1
	}
	return sign * (exit - num(p.OpenPrice)) * num(volume) * num(in.cond.ContractSize)
}

func (s *Server) openPosition(name string, side exness.Side, volume, openPrice, sl, tp string) string {
	id := s.id()
	now := time.Now().UTC().Truncate(time.Second)
	s.positions = append(s.positions, exness.Position{PositionID: id, Direction: side, Instrument: name, InitialVolume: volume, Volume: volume,
		OpenPrice: openPrice, State: "open", CreateTime: now})
	s.deals = append(s.deals, exness.Deal{DealID: s.id(), PositionID: id, Type: "open", Direction: side, Instrument: name, Volume: volume,
		Price: openPrice, Profit: exness.Money{Amount: "0.00", Currency: "USD"}, CreateTime: now})
	if sl != "" || tp != "" {
		s.orders = append(s.orders, exness.Order{OrderID: s.id(), PositionID: id, OrderType: "sltp", State: "placed", Instrument: name,
			StopLossPrice: sl, TakeProfitPrice: tp, CreateTime: now})
	}
	return id
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func apiError(w http.ResponseWriter, status, code int) {
	writeJSON(w, status, map[string]any{"code": code, "error_message": exness.CodeNames[code]})
}

// verify checks a request's signature as Exness documents it: 0 or the
// error code.
func (s *Server) verify(r *http.Request, body []byte) int {
	h := r.Header.Get
	if h("EXN-API-KEY") != APIKey {
		return exness.CodeAuthInvalidAPIKey
	}
	if _, ok := r.Header["Exn-Idempotency-Key"]; !ok {
		return exness.CodeAuthInvalidSignature
	}
	data, err1 := base64.RawURLEncoding.DecodeString(h("EXN-DATA"))
	sig, err2 := base64.RawURLEncoding.DecodeString(h("EXN-SIGN"))
	if err1 != nil || err2 != nil || !ed25519.Verify(s.pub, data, sig) {
		return exness.CodeAuthInvalidSignature
	}
	var d struct {
		APIKey         string `json:"api_key"`
		IdempotencyKey string `json:"idempotency_key"`
		Timestamp      int64  `json:"timestamp"`
		SignVersion    int    `json:"sign_version"`
		Method         string `json:"method"`
		Path           string `json:"path"`
		BodyHash       string `json:"body_hash"`
	}
	sum := sha256.Sum256(body)
	ts, _ := strconv.ParseInt(h("EXN-TIMESTAMP"), 10, 64)
	if json.Unmarshal(data, &d) != nil || d.APIKey != APIKey || d.Method != r.Method || d.Path != r.RequestURI || d.Timestamp != ts ||
		d.SignVersion != 1 || d.IdempotencyKey != h("EXN-IDEMPOTENCY-KEY") || d.BodyHash != base64.RawURLEncoding.EncodeToString(sum[:]) ||
		math.Abs(time.Since(time.UnixMilli(ts)).Seconds()) > 30 {
		return exness.CodeAuthInvalidSignature
	}
	if (r.Method == "GET") != (d.IdempotencyKey == "") {
		return exness.CodeRequestInvalid
	}
	return 0
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, r.Method+" "+r.RequestURI)
	if code := s.verify(r, body); code != 0 {
		apiError(w, http.StatusUnauthorized, code)
		return
	}
	prefix := "/v1/"
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
	// group/accounts/{id}/rest...
	if len(parts) < 4 || parts[1] != "accounts" {
		apiError(w, http.StatusNotFound, exness.CodeRequestInvalid)
		return
	}
	if parts[2] != Account {
		apiError(w, http.StatusNotFound, exness.CodeAccountNotFound)
		return
	}
	route := r.Method + " " + parts[0] + "/" + strings.Join(parts[3:], "/")
	q := r.URL.Query()
	switch {
	case route == "GET configuration/account":
		writeJSON(w, 200, exness.AccountDetails{AccountID: Account, AccountName: "Fake", CreateDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			Settings: exness.AccountSettings{Currency: "USD", Leverage: "200", TradeMode: "enabled", AccountStatus: "active", ExecutionMode: "market"}})
	case route == "GET configuration/instruments":
		var names []string
		for n := range s.instruments {
			names = append(names, n)
		}
		sort.Strings(names)
		writeJSON(w, 200, map[string]any{"instruments": names})
	case len(parts) == 6 && parts[0] == "configuration" && parts[3] == "instruments" && parts[5] == "conditions" && r.Method == "GET":
		in, ok := s.instruments[parts[4]]
		if !ok {
			apiError(w, 404, exness.CodeMarketInstrumentNotFound)
			return
		}
		writeJSON(w, 200, in.cond)
	case route == "GET market-data/candles":
		s.candles(w, q)
	case route == "GET trading/snapshot":
		writeJSON(w, 200, s.snapshot())
	case route == "GET history/orders":
		var items []map[string]any
		for _, o := range s.history {
			if q.Get("instrument") == "" || o.Instrument == q.Get("instrument") {
				items = append(items, map[string]any{"event_time": o.CreateTime, "order": o})
			}
		}
		writeJSON(w, 200, map[string]any{"items": orEmpty(items), "has_more": false})
	case route == "GET history/deals":
		var items []map[string]any
		for _, d := range s.deals {
			if q.Get("instrument") == "" || d.Instrument == q.Get("instrument") {
				items = append(items, map[string]any{"event_time": d.CreateTime, "deal": d})
			}
		}
		writeJSON(w, 200, map[string]any{"items": orEmpty(items), "has_more": false})
	case len(parts) == 5 && route == "GET trading/operations/"+parts[4]:
		op, ok := s.ops[parts[4]]
		if !ok {
			apiError(w, 404, exness.CodeRequestOperationNotFound)
			return
		}
		writeJSON(w, 200, op)
	case strings.HasPrefix(route, "POST trading/positions"), strings.HasPrefix(route, "POST trading/orders"),
		strings.HasPrefix(route, "PATCH trading/orders/"), strings.HasPrefix(route, "DELETE trading/orders/"),
		strings.HasPrefix(route, "DELETE trading/positions/"):
		s.trade(w, r, parts, body)
	default:
		apiError(w, 404, exness.CodeRequestInvalid)
	}
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

var timeframes = map[string]time.Duration{"S1": time.Second, "S5": 5 * time.Second, "S15": 15 * time.Second, "M1": time.Minute, "M3": 3 * time.Minute,
	"M5": 5 * time.Minute, "M10": 10 * time.Minute, "M15": 15 * time.Minute, "M30": 30 * time.Minute, "H1": time.Hour, "H2": 2 * time.Hour,
	"H4": 4 * time.Hour, "D1": 24 * time.Hour, "W1": 7 * 24 * time.Hour, "MN": 30 * 24 * time.Hour}

// candles are flat at the current price, the newest one still forming.
func (s *Server) candles(w http.ResponseWriter, q map[string][]string) {
	get := func(k string) string {
		if v := q[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	in, ok := s.instruments[get("instrument")]
	tf, tfOK := timeframes[get("timeframe")]
	from, err := time.Parse(time.RFC3339, get("from"))
	if !ok || !tfOK || err != nil {
		apiError(w, 400, exness.CodeRequestInvalidParameters)
		return
	}
	count, _ := strconv.Atoi(get("count"))
	start := from.Truncate(tf)
	n := count
	if count < 0 {
		n = -count
		start = start.Add(-time.Duration(n-1) * tf)
	}
	if get("to") != "" {
		to, _ := time.Parse(time.RFC3339, get("to"))
		n = int(to.Sub(start) / tf)
	}
	var out []map[string]any
	for i := 0; i < n; i++ {
		at := start.Add(time.Duration(i) * tf)
		if at.After(time.Now()) {
			break
		}
		c := map[string]any{"time": at.Format(time.RFC3339), "volume": 100 + i}
		side := func(p float64) map[string]float64 {
			return map[string]float64{"open": p, "high": p, "low": p, "close": p}
		}
		switch get("price_type") {
		case "both":
			c["bid"], c["ask"] = side(in.bid), side(in.ask)
		case "ask":
			c["open"], c["high"], c["low"], c["close"] = in.ask, in.ask, in.ask, in.ask
		default:
			c["open"], c["high"], c["low"], c["close"] = in.bid, in.bid, in.bid, in.bid
		}
		out = append(out, c)
	}
	writeJSON(w, 200, map[string]any{"candles": orEmpty(out)})
}

// trade runs a trading request: an ACK, and the operation at once.
func (s *Server) trade(w http.ResponseWriter, r *http.Request, parts []string, body []byte) {
	key := r.Header.Get("EXN-IDEMPOTENCY-KEY")
	opID := s.id()
	now := time.Now().UTC()
	op := exness.Operation{OperationID: opID, Status: "confirmed", ClientRequestID: key, CreatedAt: now, UpdatedAt: now}
	if len(s.reject) > 0 {
		op.Status, op.ErrorCode, op.ErrorMessage = "rejected", s.reject[0], exness.CodeNames[s.reject[0]]
		s.reject = s.reject[1:]
	} else if code := s.apply(r, parts, body); code != 0 {
		op.Status, op.ErrorCode, op.ErrorMessage = "rejected", code, exness.CodeNames[code]
	}
	s.ops[opID] = op
	writeJSON(w, 202, map[string]any{"operation_id": opID, "client_request_id": key, "status": "accepted"})
}

// checkVolume: at least the least, on the step.
func checkVolume(c exness.InstrumentCondition, v string) int {
	vol, min, step := num(v), num(c.VolumeMin), num(c.VolumeStep)
	if vol < min-1e-9 {
		return exness.CodeTradingRuleVolumeTooSmall
	}
	if r := math.Mod(math.Round((vol-min)*1e8)/1e8, step); r > 1e-9 && step-r > 1e-9 {
		return exness.CodeTradingRuleInvalidVolumeStep
	}
	return 0
}

func (s *Server) apply(r *http.Request, parts []string, body []byte) int {
	now := time.Now().UTC().Truncate(time.Second)
	switch {
	case r.Method == "POST" && parts[3] == "positions":
		var p exness.OpenPosition
		json.Unmarshal(body, &p)
		in, ok := s.instruments[p.Instrument]
		if !ok {
			return exness.CodeMarketInstrumentNotFound
		}
		if code := checkVolume(in.cond, p.Volume); code != 0 {
			return code
		}
		price := in.ask
		if p.Side == exness.Sell {
			price = in.bid
		}
		if (p.StopLoss != "" && (p.Side == exness.Buy) != (num(p.StopLoss) < price)) || (p.TakeProfit != "" && (p.Side == exness.Buy) != (num(p.TakeProfit) > price)) {
			return exness.CodeTradingRuleInvalidPriceLevels
		}
		s.openPosition(p.Instrument, p.Side, p.Volume, s.price(p.Instrument, price), p.StopLoss, p.TakeProfit)
	case r.Method == "POST" && parts[3] == "orders":
		var o exness.PendingOrder
		json.Unmarshal(body, &o)
		if o.Type == exness.OrderSLTP {
			for i := range s.orders {
				if s.orders[i].OrderType == "sltp" && s.orders[i].PositionID == o.PositionID {
					s.orders[i].StopLossPrice, s.orders[i].TakeProfitPrice = o.StopLoss, o.TakeProfit
					return 0
				}
			}
			for _, p := range s.positions {
				if p.PositionID == o.PositionID {
					s.orders = append(s.orders, exness.Order{OrderID: s.id(), PositionID: p.PositionID, OrderType: "sltp", State: "placed",
						Instrument: p.Instrument, StopLossPrice: o.StopLoss, TakeProfitPrice: o.TakeProfit, CreateTime: now})
					return 0
				}
			}
			return exness.CodeRequestPositionNotFound
		}
		in, ok := s.instruments[o.Instrument]
		if !ok {
			return exness.CodeMarketInstrumentNotFound
		}
		if code := checkVolume(in.cond, o.Volume); code != 0 {
			return code
		}
		s.orders = append(s.orders, exness.Order{OrderID: s.id(), OrderType: string(o.Type), State: "placed", Direction: o.Side, Instrument: o.Instrument,
			Price: o.Price, StopLossPrice: o.StopLoss, TakeProfitPrice: o.TakeProfit, Volume: o.Volume, CreateTime: now})
	case r.Method == "PATCH":
		var m map[string]*string
		json.Unmarshal(body, &m)
		for i := range s.orders {
			if s.orders[i].OrderID != parts[4] {
				continue
			}
			o := &s.orders[i]
			if v, ok := m["price"]; ok && v != nil {
				o.Price = *v
			}
			for k, field := range map[string]*string{"stop_loss_price": &o.StopLossPrice, "take_profit_price": &o.TakeProfitPrice} {
				if v, ok := m[k]; ok {
					*field = ""
					if v != nil {
						*field = *v
					}
				}
			}
			o.UpdateTime = now
			return 0
		}
		return exness.CodeRequestOrderNotFound
	case r.Method == "DELETE" && parts[3] == "orders":
		for i, o := range s.orders {
			if o.OrderID == parts[4] {
				o.State = "cancelled"
				s.history = append(s.history, o)
				s.orders = append(s.orders[:i], s.orders[i+1:]...)
				return 0
			}
		}
		return exness.CodeRequestOrderNotFound
	case r.Method == "DELETE" && parts[3] == "positions":
		for i, p := range s.positions {
			if p.PositionID != parts[4] {
				continue
			}
			volume := r.URL.Query().Get("volume")
			if volume == "" || num(volume) >= num(p.Volume)-1e-9 {
				volume = p.Volume
			}
			profit := s.profit(p, volume)
			s.balance += profit
			in := s.instruments[p.Instrument]
			exit := in.bid
			if p.Direction == exness.Sell {
				exit = in.ask
			}
			closeSide := exness.Sell
			if p.Direction == exness.Sell {
				closeSide = exness.Buy
			}
			s.deals = append(s.deals, exness.Deal{DealID: s.id(), PositionID: p.PositionID, Type: "close", Direction: closeSide, Instrument: p.Instrument,
				Volume: volume, Price: s.price(p.Instrument, exit), Profit: exness.Money{Amount: fmt.Sprintf("%.2f", profit), Currency: "USD"}, CreateTime: now})
			if volume == p.Volume {
				s.positions = append(s.positions[:i], s.positions[i+1:]...)
				kept := s.orders[:0]
				for _, o := range s.orders {
					if o.OrderType != "sltp" || o.PositionID != p.PositionID {
						kept = append(kept, o)
					}
				}
				s.orders = kept
			} else {
				s.positions[i].Volume = strconv.FormatFloat(num(p.Volume)-num(volume), 'f', -1, 64)
			}
			return 0
		}
		return exness.CodeRequestPositionNotFound
	}
	return 0
}
