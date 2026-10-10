package mt5

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// fakeEA answers the bridge's requests the way AutotradeBridge.mq5 does,
// from a small simulated demo account with one symbol, XAUUSD.
type fakeEA struct {
	dir string

	mu         sync.Mutex
	tradeMode  string
	profitCcy  string
	session    bool
	positions  []*fakePos
	orders     []*fakeOrder
	history    map[int64]*fakeOrder
	next       int64
	reqs       []map[string]string
	delay      time.Duration // before answering a claimed request
	ignoreReqs bool          // leave requests unclaimed, as with no EA running
}

type fakePos struct {
	ticket   int64
	long     bool
	lots     float64
	open, sl float64
	tp       float64
}

type fakeOrder struct {
	ticket          int64
	long            bool
	lots, price     float64
	sl, tp          float64
	magic           int64
	state           string
	filled, avg     float64
	setup, doneTime int64
}

const (
	bid = 2650.00
	ask = 2650.30
)

func startEA(t *testing.T) (*fakeEA, *Client) {
	t.Helper()
	ea := &fakeEA{dir: t.TempDir(), tradeMode: "demo", profitCcy: "USD", session: true, history: map[int64]*fakeOrder{}, next: 1000}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Millisecond):
			}
			ea.poll()
		}
	}()
	t.Cleanup(func() { close(stop); <-done })
	c := New(ea.dir, nil)
	c.b.wait, c.b.busy = 300*time.Millisecond, 2*time.Second
	return ea, c
}

func (ea *fakeEA) poll() {
	ea.mu.Lock()
	ignore, delay := ea.ignoreReqs, ea.delay
	ea.mu.Unlock()
	if ignore {
		return
	}
	names, _ := filepath.Glob(filepath.Join(ea.dir, "*.req"))
	for _, name := range names {
		id := strings.TrimSuffix(filepath.Base(name), ".req")
		work := filepath.Join(ea.dir, id+".work")
		if os.Rename(name, work) != nil {
			continue
		}
		data, _ := os.ReadFile(work)
		os.Remove(work)
		req := map[string]string{}
		for _, l := range strings.Split(string(data), "\r\n") {
			if k, v, ok := strings.Cut(l, "="); ok {
				req[k] = v
			}
		}
		time.Sleep(delay)
		ans := ea.answer(req)
		tmp := filepath.Join(ea.dir, id+".tmp")
		os.WriteFile(tmp, []byte(strings.ReplaceAll(ans, "\n", "\r\n")), 0o644)
		os.Rename(tmp, filepath.Join(ea.dir, id+".res"))
	}
}

// requests returns the ops asked so far, with their arguments.
func (ea *fakeEA) requests() []string {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	var out []string
	for _, r := range ea.reqs {
		s := r["op"]
		for _, k := range []string{"symbol", "side", "volume", "price", "ticket", "sl", "tp", "tf", "count"} {
			if v, ok := r[k]; ok {
				s += " " + k + "=" + v
			}
		}
		out = append(out, s)
	}
	return out
}

func f(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func rec(kv ...string) string {
	var fs []string
	for i := 0; i+1 < len(kv); i += 2 {
		fs = append(fs, kv[i]+"="+kv[i+1])
	}
	return strings.Join(fs, "\t") + "\n"
}

func (ea *fakeEA) answer(req map[string]string) string {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	ea.reqs = append(ea.reqs, req)
	num := func(k string) float64 { v, _ := strconv.ParseFloat(req[k], 64); return v }
	id := func(k string) int64 { v, _ := strconv.ParseInt(req[k], 10, 64); return v }
	trading := map[string]bool{"deal": true, "close": true, "limit": true, "sltp": true, "modify": true, "cancel": true}
	if trading[req["op"]] && ea.tradeMode != "demo" {
		return "error\trefused\tnot a demo account\n"
	}
	if s, ok := req["symbol"]; ok && s != "XAUUSD" && req["op"] != "positions" && req["op"] != "orders" {
		return "error\tsymbol\tunknown symbol " + s + "\n"
	}
	switch req["op"] {
	case "ping":
		return "ok\n" + rec("bridge", "1", "login", "5551234", "server", "MIFX-Demo", "company", "PT Monex Investindo Futures",
			"currency", "USD", "leverage", "100", "trade_mode", ea.tradeMode, "margin_mode", "hedging", "balance", "10000",
			"equity", "10012.5", "profit", "12.5", "margin_free", "9000", "gmt_offset", "10800", "connected", "1", "trade_refusal", "",
			"build", "4755", "magic", "20261010")
	case "symbol":
		open := "0"
		if ea.session {
			open = "1"
		}
		return "ok\n" + rec("symbol", "XAUUSD", "digits", "2", "point", "0.01", "tick_size", "0.01", "contract", "100",
			"vol_min", "0.01", "vol_max", "50", "vol_step", "0.01", "profit_ccy", ea.profitCcy, "trade_mode", "full",
			"stops_level", "20", "session_open", open, "bid", f(bid), "ask", f(ask), "tick_time", "1791763200")
	case "rates":
		// Newest first, to check the client sorts them.
		return "ok\n" + rec("t", "1791763500", "o", "2", "h", "2", "l", "2", "c", "2", "v", "40") +
			rec("t", "1791763200", "o", "1", "h", "1", "l", "1", "c", "1", "v", "30")
	case "positions":
		out := "ok\n"
		for _, p := range ea.positions {
			typ, cur := "sell", ask
			if p.long {
				typ, cur = "buy", bid
			}
			out += rec("ticket", fmt.Sprint(p.ticket), "symbol", "XAUUSD", "type", typ, "volume", f(p.lots), "price_open", f(p.open),
				"price_current", f(cur), "sl", f(p.sl), "tp", f(p.tp), "profit", "-30", "swap", "-1.5", "time", "1791763260", "magic", "20261010")
		}
		return out
	case "orders":
		out := "ok\n"
		for _, o := range ea.orders {
			typ := "sell_limit"
			if o.long {
				typ = "buy_limit"
			}
			out += rec("ticket", fmt.Sprint(o.ticket), "symbol", "XAUUSD", "type", typ, "state", "placed", "volume_initial", f(o.lots),
				"volume_current", f(o.lots), "price", f(o.price), "sl", f(o.sl), "tp", f(o.tp), "time_setup", "1791763300", "magic", fmt.Sprint(o.magic))
		}
		return out
	case "deal":
		ea.next++
		long := req["side"] == "buy"
		px := bid
		if long {
			px = ask
		}
		ea.positions = append(ea.positions, &fakePos{ticket: ea.next, long: long, lots: num("volume"), open: px})
		return "ok\n" + rec("order", fmt.Sprint(ea.next), "deal", fmt.Sprint(ea.next+1), "volume", req["volume"], "price", f(px), "time", "1791763260")
	case "close":
		closesLong := req["side"] == "sell"
		var kept []*fakePos
		closed := 0.0
		for _, p := range ea.positions {
			if p.long == closesLong {
				closed += p.lots
				continue
			}
			kept = append(kept, p)
		}
		if closed == 0 {
			return "error\tno_position\tno open position on XAUUSD\n"
		}
		ea.positions = kept
		ea.next++
		return "ok\n" + rec("order", fmt.Sprint(ea.next), "volume", f(closed), "price", f(bid), "time", "1791763320")
	case "limit":
		long := req["side"] == "buy"
		if long && num("price") >= ask || !long && num("price") <= bid {
			return "error\t10015\tInvalid price\n"
		}
		ea.next++
		ea.orders = append(ea.orders, &fakeOrder{ticket: ea.next, long: long, lots: num("volume"), price: num("price"), magic: 20261010, state: "placed", setup: 1791763300})
		return "ok\n" + rec("order", fmt.Sprint(ea.next), "time", "1791763300")
	case "sltp":
		n := 0
		for _, p := range ea.positions {
			if t := id("ticket"); t == 0 || t == p.ticket {
				p.sl, p.tp = num("sl"), num("tp")
				n++
			}
		}
		if n == 0 {
			return "error\tno_position\tno open position on XAUUSD\n"
		}
		return "ok\n"
	case "modify":
		for _, o := range ea.orders {
			if o.ticket == id("ticket") {
				o.sl, o.tp = num("sl"), num("tp")
				return "ok\n"
			}
		}
		return "error\tno_order\tno waiting order\n"
	case "order", "cancel":
		for i, o := range ea.orders {
			if o.ticket != id("ticket") {
				continue
			}
			if req["op"] == "cancel" {
				o.state, o.doneTime = "canceled", 1791763400
				ea.orders = append(ea.orders[:i], ea.orders[i+1:]...)
				ea.history[o.ticket] = o
			}
			return "ok\n" + ea.orderRec(o)
		}
		if o, ok := ea.history[id("ticket")]; ok && req["op"] == "order" {
			return "ok\n" + ea.orderRec(o)
		}
		return "error\tno_order\tno order " + req["ticket"] + "\n"
	case "deals":
		return "ok\n" +
			rec("ticket", "1", "order", "11", "time", "1791763260", "type", "buy", "entry", "in", "volume", "0.02", "price", "2650.3", "profit", "0", "commission", "-0.7", "swap", "0", "fee", "0") +
			rec("ticket", "2", "order", "12", "time", "1791766860", "type", "sell", "entry", "out", "volume", "0.02", "price", "2660.3", "profit", "20", "commission", "-0.7", "swap", "-1.5", "fee", "0") +
			rec("ticket", "3", "order", "0", "time", "1791766900", "type", "other", "entry", "in", "volume", "0", "price", "0", "profit", "500", "commission", "0", "swap", "0", "fee", "0")
	}
	return "error\top\tunknown op " + req["op"] + "\n"
}

// pos and order copy the first position and waiting order.
func (ea *fakeEA) pos() fakePos {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	return *ea.positions[0]
}

func (ea *fakeEA) order() fakeOrder {
	ea.mu.Lock()
	defer ea.mu.Unlock()
	return *ea.orders[0]
}

func (ea *fakeEA) orderRec(o *fakeOrder) string {
	typ := "sell_limit"
	if o.long {
		typ = "buy_limit"
	}
	return rec("ticket", fmt.Sprint(o.ticket), "symbol", "XAUUSD", "type", typ, "state", o.state, "volume_initial", f(o.lots),
		"volume_current", f(o.lots-o.filled), "price", f(o.price), "sl", f(o.sl), "tp", f(o.tp), "time_setup", fmt.Sprint(o.setup),
		"time_done", fmt.Sprint(o.doneTime), "filled", f(o.filled), "avg_price", f(o.avg), "magic", fmt.Sprint(o.magic))
}

var ctx = context.Background()

func TestNoBridgeAnswer(t *testing.T) {
	ea, c := startEA(t)
	ea.mu.Lock()
	ea.ignoreReqs = true
	ea.mu.Unlock()
	_, err := c.Ping(ctx)
	if !errors.Is(err, ErrNoBridge) {
		t.Fatalf("err = %v, want ErrNoBridge", err)
	}
	if left, _ := filepath.Glob(filepath.Join(ea.dir, "*")); len(left) != 0 {
		t.Fatalf("left behind: %v", left)
	}
	// Taken as down for a while: the next call does not wait again.
	start := time.Now()
	if _, err := c.Klines(ctx, "XAUUSD", "5m", 3); !errors.Is(err, ErrNoBridge) || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("second call: %v after %s", err, time.Since(start))
	}
}

func TestClaimedRequestWaitsForItsAnswer(t *testing.T) {
	ea, c := startEA(t)
	ea.mu.Lock()
	ea.delay = 500 * time.Millisecond // past the client's 300ms wait for a claim
	ea.mu.Unlock()
	term, err := c.Ping(ctx)
	if err != nil || term.Login != 5551234 || term.Currency != "USD" || term.Leverage != 100 || term.TradeMode != "demo" || term.Magic != 20261010 {
		t.Fatalf("ping = %+v, %v", term, err)
	}
}

func TestSymbolInfoInUnits(t *testing.T) {
	ea, c := startEA(t)
	info, err := c.SymbolInfo(ctx, "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	// One lot is 100 oz: steps of 0.01 lot are 1 oz.
	if info.StepSize != 1 || info.MinQty != 1 || info.MaxQty != 5000 || info.QtyDec != 0 || info.TickSize != 0.01 || info.PriceDec != 2 || info.Status != "TRADING" {
		t.Fatalf("info = %+v", info)
	}
	if got := info.FloorQty(1234.0 / 2650); got != 0 {
		t.Fatalf("FloorQty = %v", got)
	}

	ea.mu.Lock()
	ea.session = false
	ea.mu.Unlock()
	if info, _ := c.SymbolInfo(ctx, "XAUUSD"); info.Status != "CLOSED" {
		t.Fatalf("outside its session: status %q", info.Status)
	}

	ea.mu.Lock()
	ea.profitCcy = "JPY"
	ea.mu.Unlock()
	if _, err := c.SymbolInfo(ctx, "XAUUSD"); err == nil || !strings.Contains(err.Error(), "pays its profit in JPY") {
		t.Fatalf("profit in JPY: err = %v", err)
	}
	if _, err := c.SymbolInfo(ctx, "EURJPY"); err == nil || !strings.Contains(err.Error(), "unknown symbol EURJPY") {
		t.Fatalf("unknown symbol: err = %v", err)
	}
}

func TestMarketPrices(t *testing.T) {
	_, c := startEA(t)
	pi, err := c.PremiumIndex(ctx, "XAUUSD")
	if err != nil || pi.MarkPrice != (bid+ask)/2 || pi.FundingRate != 0 {
		t.Fatalf("premium index = %+v, %v", pi, err)
	}
	bk, err := c.BookTicker(ctx, "XAUUSD")
	if err != nil || bk.Bid != bid || bk.Ask != ask {
		t.Fatalf("book = %+v, %v", bk, err)
	}
	if _, err := c.CommissionRate(ctx, "XAUUSD"); err == nil {
		t.Fatal("commission rate: want an error, MetaTrader has none")
	}
}

func TestKlinesAreUTCAndOldestFirst(t *testing.T) {
	ea, c := startEA(t)
	cs, err := c.Klines(ctx, "XAUUSD", "5m", 2)
	if err != nil {
		t.Fatal(err)
	}
	open := time.Unix(1791763200, 0).UTC()
	if len(cs) != 2 || !cs[0].OpenTime.Equal(open) || !cs[0].CloseTime.Equal(open.Add(5*time.Minute-time.Millisecond)) || cs[1].Close != 2 || cs[0].Volume != 30 || cs[0].QuoteVolume != 30 {
		t.Fatalf("candles = %+v", cs)
	}
	if got := ea.requests(); got[len(got)-1] != "rates symbol=XAUUSD tf=5m count=2" {
		t.Fatalf("request = %q", got[len(got)-1])
	}
	if _, err := c.Klines(ctx, "XAUUSD", "5x", 2); err == nil {
		t.Fatal("unknown interval: want an error")
	}
}

func TestMarketOrderTradesLots(t *testing.T) {
	ea, c := startEA(t)
	o, err := c.MarketOrder(ctx, "XAUUSD", "BUY", "2", false)
	if err != nil {
		t.Fatal(err)
	}
	if o.Status != "FILLED" || o.AvgPrice != ask || o.ExecutedQty != 2 || o.OrderID == 0 {
		t.Fatalf("order = %+v", o)
	}
	ps, err := c.Positions(ctx, "XAUUSD")
	if err != nil || len(ps) != 1 {
		t.Fatalf("positions = %+v, %v", ps, err)
	}
	if p := ps[0]; p.Amount != 2 || p.EntryPrice != ask || p.MarkPrice != bid || p.UnrealizedPnL != -31.5 || p.Notional != 2*bid {
		t.Fatalf("position = %+v", p)
	}

	if _, err := c.MarketOrder(ctx, "XAUUSD", "SELL", "2", true); err != nil {
		t.Fatal(err)
	}
	if ps, _ := c.Positions(ctx, "XAUUSD"); len(ps) != 0 {
		t.Fatalf("after the reduce-only sell: %+v", ps)
	}
	var trades []string
	for _, r := range ea.requests() {
		if strings.HasPrefix(r, "deal") || strings.HasPrefix(r, "close") {
			trades = append(trades, r)
		}
	}
	if want := "deal symbol=XAUUSD side=buy volume=0.02; close symbol=XAUUSD side=sell volume=0.02"; strings.Join(trades, "; ") != want {
		t.Fatalf("trades = %q, want %q", strings.Join(trades, "; "), want)
	}
	if _, err := c.MarketOrder(ctx, "XAUUSD", "BUY", "0.4", false); err == nil || !strings.Contains(err.Error(), "under the minimum") {
		t.Fatalf("under the minimum lot: err = %v", err)
	}
}

func TestStopsArePartOfThePosition(t *testing.T) {
	ea, c := startEA(t)
	if _, err := c.StopOrder(ctx, "XAUUSD", "SELL", binance.StopMarket, "2640"); err == nil {
		t.Fatal("a stop with no position: want an error")
	}
	if _, err := c.MarketOrder(ctx, "XAUUSD", "BUY", "1", false); err != nil {
		t.Fatal(err)
	}
	stop, err := c.StopOrder(ctx, "XAUUSD", "SELL", binance.StopMarket, "2640")
	if err != nil {
		t.Fatal(err)
	}
	tp, err := c.StopOrder(ctx, "XAUUSD", "SELL", binance.TakeProfitMarket, "2680.5")
	if err != nil {
		t.Fatal(err)
	}
	ticket := ea.pos().ticket
	if stop.AlgoID != ticket*10+1 || tp.AlgoID != ticket*10+2 || ea.pos().sl != 2640 || ea.pos().tp != 2680.5 {
		t.Fatalf("stop %+v, target %+v, position %+v", stop, tp, ea.pos())
	}
	algos, err := c.OpenAlgoOrders(ctx, "XAUUSD")
	if err != nil || len(algos) != 2 || algos[0].Type != binance.StopMarket || algos[0].TriggerPrice != 2640 || algos[0].Side != "SELL" || !algos[0].ClosePosition || algos[1].TriggerPrice != 2680.5 {
		t.Fatalf("algo orders = %+v, %v", algos, err)
	}
	// Moving the stop is one step.
	if _, err := c.StopOrder(ctx, "XAUUSD", "SELL", binance.StopMarket, "2645"); err != nil || ea.pos().sl != 2645 || ea.pos().tp != 2680.5 {
		t.Fatalf("moved stop: %+v, %v", ea.pos(), err)
	}
	if err := c.CancelAlgoOrder(ctx, "XAUUSD", tp.AlgoID); err != nil || ea.pos().sl != 2645 || ea.pos().tp != 0 {
		t.Fatalf("cancelled target: %+v, %v", ea.pos(), err)
	}
	if err := c.CancelAlgoOrders(ctx, "XAUUSD"); err != nil || ea.pos().sl != 0 {
		t.Fatalf("cancelled all: %+v, %v", ea.pos(), err)
	}
	if err := c.CancelAlgoOrder(ctx, "XAUUSD", 99991); err != nil {
		t.Fatalf("a stop whose position is gone: %v", err)
	}
}

func TestLimitEntryCarriesItsStops(t *testing.T) {
	ea, c := startEA(t)
	if _, err := c.LimitOrder(ctx, "XAUUSD", "BUY", "1", "2651"); err == nil || !strings.Contains(err.Error(), "under the ask") {
		t.Fatalf("a buy limit over the ask: err = %v", err)
	}
	o, err := c.LimitOrder(ctx, "XAUUSD", "BUY", "1", "2640")
	if err != nil || o.Status != "NEW" || o.Price != 2640 || o.OrigQty != 1 {
		t.Fatalf("limit = %+v, %v", o, err)
	}
	if _, err := c.ReduceStopOrder(ctx, "XAUUSD", "SELL", binance.StopMarket, "2630", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReduceStopOrder(ctx, "XAUUSD", "SELL", binance.TakeProfitMarket, "2670", "1"); err != nil {
		t.Fatal(err)
	}
	if w := ea.order(); w.sl != 2630 || w.tp != 2670 {
		t.Fatalf("waiting order = %+v", w)
	}
	algos, err := c.OpenAlgoOrders(ctx, "XAUUSD")
	if err != nil || len(algos) != 2 || algos[0].Quantity != 1 || algos[0].ClosePosition || algos[0].AlgoID != o.OrderID*10+1 {
		t.Fatalf("algo orders = %+v, %v", algos, err)
	}
	g, err := c.GetOrder(ctx, "XAUUSD", o.OrderID)
	if err != nil || g.Status != "NEW" || !g.Working() || g.Side != "BUY" || g.OrigQty != 1 || g.ExecutedQty != 0 {
		t.Fatalf("get = %+v, %v", g, err)
	}
	x, err := c.CancelOrder(ctx, "XAUUSD", o.OrderID)
	if err != nil || x.Status != "CANCELED" || x.Working() || x.Updated.Unix() != 1791763400 {
		t.Fatalf("cancel = %+v, %v", x, err)
	}
	if g, err := c.GetOrder(ctx, "XAUUSD", o.OrderID); err != nil || g.Status != "CANCELED" {
		t.Fatalf("get after cancel = %+v, %v", g, err)
	}

	// An order placed by hand is not autotrade's: its stops are left alone.
	ea.mu.Lock()
	ea.orders = append(ea.orders, &fakeOrder{ticket: 7, long: true, lots: 0.05, price: 2600, sl: 2590, magic: 0, state: "placed"})
	ea.mu.Unlock()
	if algos, err := c.OpenAlgoOrders(ctx, "XAUUSD"); err != nil || len(algos) != 0 {
		t.Fatalf("hand-placed order: algo orders = %+v, %v", algos, err)
	}
}

func TestDemoAccountOnly(t *testing.T) {
	ea, c := startEA(t)
	ea.mu.Lock()
	ea.tradeMode = "real"
	ea.mu.Unlock()
	if _, err := c.MarketOrder(ctx, "XAUUSD", "BUY", "1", false); err == nil || !strings.Contains(err.Error(), "demo accounts only") {
		t.Fatalf("real account: err = %v", err)
	}
	for _, r := range ea.requests() {
		if strings.HasPrefix(r, "deal") {
			t.Fatalf("an order was sent to a real account: %q", r)
		}
	}
}

func TestIncomeFromDeals(t *testing.T) {
	_, c := startEA(t)
	inc, err := c.Income(ctx, "XAUUSD", time.Unix(1791763200, 0), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	sum := map[string]float64{}
	for _, in := range inc {
		sum[in.Type] += in.Amount
		if in.Asset != "USD" {
			t.Fatalf("asset %q", in.Asset)
		}
	}
	// The balance deal (type other) is not income of the symbol.
	if len(inc) != 4 || sum["REALIZED_PNL"] != 20 || sum["COMMISSION"] != -1.4 || sum["FUNDING_FEE"] != -1.5 {
		t.Fatalf("income = %+v", inc)
	}
}

func TestLeverageIsTheAccounts(t *testing.T) {
	_, c := startEA(t)
	sc, err := c.SymbolConfig(ctx, "XAUUSD")
	if err != nil || sc.Leverage != 100 {
		t.Fatalf("symbol config = %+v, %v", sc, err)
	}
	if err := c.SetLeverage(ctx, "XAUUSD", 100); err != nil {
		t.Fatal(err)
	}
	if err := c.SetLeverage(ctx, "XAUUSD", 5); err == nil || !strings.Contains(err.Error(), "1:100") {
		t.Fatalf("other leverage: err = %v", err)
	}
	acc, err := c.Account(ctx)
	if err != nil || acc.WalletBalance != 10000 || acc.AvailableBalance != 9000 || acc.MarginBalance != 10012.5 || acc.UnrealizedPnL != 12.5 {
		t.Fatalf("account = %+v, %v", acc, err)
	}
	if c.AccountCurrency(ctx) != "USD" {
		t.Fatal("account currency")
	}
}
