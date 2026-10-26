package engine

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// fakeBinance is a USD-M futures API: BTCUSDT (tick 0.1, step 0.001), a
// book at 80000/80000.1, and what each test sets. calls are the requests
// it got, "METHOD path?query" without the signing.
type fakeBinance struct {
	mu        sync.Mutex
	calls     []string
	hedge     bool
	positions string // positionRisk's answer
	refuseTP  bool   // a take profit is refused
	klines    string
}

func (f *fakeBinance) did(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeBinance) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	for _, k := range []string{"timestamp", "recvWindow", "signature", "newOrderRespType"} {
		q.Del(k)
	}
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path+"?"+q.Encode())
	f.mu.Unlock()
	switch r.Method + " " + r.URL.Path {
	case "GET /fapi/v1/time":
		fmt.Fprintf(w, `{"serverTime":%d}`, time.Now().UnixMilli())
	case "GET /fapi/v1/positionSide/dual":
		fmt.Fprintf(w, `{"dualSidePosition":%v}`, f.hedge)
	case "GET /fapi/v1/exchangeInfo":
		fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","status":"TRADING","filters":[
			{"filterType":"PRICE_FILTER","tickSize":"0.10"},
			{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"1000"},
			{"filterType":"MIN_NOTIONAL","notional":"100"}]}]}`)
	case "GET /fapi/v1/ticker/bookTicker":
		fmt.Fprint(w, `{"bidPrice":"80000.0","askPrice":"80000.1"}`)
	case "POST /fapi/v1/leverage":
		fmt.Fprint(w, `{"leverage":5}`)
	case "POST /fapi/v1/order":
		status, avg, exec := "FILLED", "80000.1", q.Get("quantity")
		if q.Get("type") == "LIMIT" {
			status, avg, exec = "NEW", "0", "0"
		}
		fmt.Fprintf(w, `{"orderId":11,"symbol":"%s","status":"%s","side":"%s","type":"%s","price":"%s","origQty":"%s","executedQty":"%s","avgPrice":"%s","reduceOnly":%v}`,
			q.Get("symbol"), status, q.Get("side"), q.Get("type"), q.Get("price"), q.Get("quantity"), exec, avg, q.Get("reduceOnly") == "true")
	case "DELETE /fapi/v1/order":
		fmt.Fprint(w, `{"orderId":11,"symbol":"BTCUSDT","status":"CANCELED","type":"LIMIT","executedQty":"0"}`)
	case "POST /fapi/v1/algoOrder":
		if f.refuseTP && q.Get("type") == "TAKE_PROFIT_MARKET" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"code":-2021,"msg":"Order would immediately trigger."}`)
			return
		}
		fmt.Fprintf(w, `{"algoId":%d,"symbol":"BTCUSDT","orderType":"%s","side":"%s","triggerPrice":"%s","quantity":"%s","algoStatus":"NEW"}`,
			map[string]int{"STOP_MARKET": 21, "TAKE_PROFIT_MARKET": 22}[q.Get("type")], q.Get("type"), q.Get("side"), q.Get("triggerPrice"), q.Get("quantity"))
	case "DELETE /fapi/v1/algoOrder", "DELETE /fapi/v1/algoOpenOrders":
		fmt.Fprint(w, `{"code":200,"msg":"The operation of cancel all open order is done."}`)
	case "GET /fapi/v1/klines":
		fmt.Fprint(w, f.klines)
	case "GET /fapi/v3/account":
		fmt.Fprint(w, `{"totalWalletBalance":"5000","availableBalance":"4900","totalMarginBalance":"5010","totalUnrealizedProfit":"10"}`)
	case "GET /fapi/v3/balance":
		fmt.Fprint(w, `[{"asset":"USDT","balance":"5000","availableBalance":"4900","crossUnPnl":"10"}]`)
	case "GET /fapi/v3/positionRisk":
		fmt.Fprint(w, f.positions)
	case "GET /fapi/v1/allOrders", "GET /fapi/v1/openOrders":
		fmt.Fprint(w, `[{"orderId":1,"symbol":"BTCUSDT","status":"FILLED","time":1791621000000},{"orderId":2,"symbol":"BTCUSDT","status":"NEW","time":1791622000000}]`)
	case "GET /fapi/v1/openAlgoOrders":
		fmt.Fprint(w, `[{"algoId":21,"symbol":"BTCUSDT","orderType":"STOP_MARKET","triggerPrice":"79000"}]`)
	default:
		w.WriteHeader(404)
		fmt.Fprintf(w, `{"code":-1,"msg":"no %s %s"}`, r.Method, r.URL.Path)
	}
}

// binanceEnv is a Binance connection (1) on the test network at a fake.
func binanceEnv(t *testing.T) (*fakeBinance, Env) {
	t.Helper()
	f := &fakeBinance{positions: `[]`}
	ts := httptest.NewServer(f)
	t.Cleanup(ts.Close)
	return f, Env{
		BinanceURL: ts.URL,
		Connection: func(id uint32) (store.Connection, error) {
			if id != 1 {
				return store.Connection{}, store.ErrNotFound
			}
			return store.Connection{ID: 1, Name: "Binance test", Type: store.Binance, Settings: &v1.ConnectionSettings{
				Type: &v1.ConnectionSettings_Binance{Binance: &v1.BinanceSettings{ApiKey: "key", ApiSecret: "secret", Network: "testnet"}},
			}}, nil
		},
	}
}

func logsOf(s *v1.StepRun) string { return strings.Join(s.GetLogs(), "\n") }

// A market buy with leverage, a stop loss and a take profit: the quantity
// is rounded down to the step, the stops are reduce-only sells for what
// filled, on the mark price.
func TestBinanceMarketOrder(t *testing.T) {
	f, env := binanceEnv(t)
	steps, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "btc/usdt", "Quantity", "0.0129",
		"Stop Loss", "79000.04", "Take Profit", "82000", "Leverage", "5x")
	if err != nil {
		t.Fatal(err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 1 || got[0] != "POST /fapi/v1/order?quantity=0.012&side=BUY&symbol=BTCUSDT&type=MARKET" {
		t.Errorf("orders = %v", got)
	}
	if got := f.did("POST /fapi/v1/leverage"); len(got) != 1 || !strings.Contains(got[0], "leverage=5") {
		t.Errorf("leverage = %v", got)
	}
	algo := f.did("POST /fapi/v1/algoOrder")
	if len(algo) != 2 || !strings.Contains(algo[0], "triggerPrice=79000.0") || !strings.Contains(algo[0], "type=STOP_MARKET") ||
		!strings.Contains(algo[0], "side=SELL") || !strings.Contains(algo[0], "quantity=0.012") || !strings.Contains(algo[0], "reduceOnly=true") ||
		!strings.Contains(algo[1], "type=TAKE_PROFIT_MARKET") || !strings.Contains(algo[1], "triggerPrice=82000.0") {
		t.Errorf("stops = %v", algo)
	}
	if got := textOut(t, steps[1], "Order ID"); got != "11" {
		t.Errorf("Order ID = %q", got)
	}
	if o := objectOut(t, steps[1], "Stop Loss Order").GetStructValue().GetFields(); o["algo_id"].GetNumberValue() != 21 {
		t.Errorf("Stop Loss Order = %v", o)
	}
	if l := logsOf(steps[1]); !strings.HasPrefix(l, "testnet: BUY 0.012 BTCUSDT MARKET at 80000.1") {
		t.Errorf("logs = %q", l)
	}
}

// A stop on the wrong side of the entry, a quantity below the step, hedge
// mode and a reduce-only order with stops are refused before any order.
func TestBinanceOrderRefused(t *testing.T) {
	f, env := binanceEnv(t)
	for _, c := range []struct {
		values []string
		want   string
	}{
		{[]string{"Quantity", "0.01", "Stop Loss", "80500"}, "Stop Loss 80500.0 is on the wrong side"},
		{[]string{"Quantity", "0.01", "Side", "Sell", "Take Profit", "80500"}, "Take Profit 80500.0 is on the wrong side"},
		{[]string{"Quantity", "0.0004"}, "below BTCUSDT's least"},
		{[]string{"Quantity", "0.01", "Reduce Only", "Yes", "Stop Loss", "70000"}, "reduce-only order closes"},
		{[]string{"Quantity", "0.01", "Type", "Limit"}, "Price is empty"},
		{[]string{"Quantity", "0.01", "Leverage", "200"}, "Leverage"},
	} {
		values := append([]string{"Connection", "1", "Symbol", "BTCUSDT"}, c.values...)
		if _, err := run(t, env, "binance-order", values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v, want %q", c.values, err, c.want)
		}
	}
	f.hedge = true
	if _, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "BTCUSDT", "Quantity", "0.01"); err == nil || !strings.Contains(err.Error(), "hedge mode") {
		t.Errorf("hedge mode: %v", err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 0 {
		t.Errorf("orders placed: %v", got)
	}
}

// A post-only limit sell with a stop loss and a take profit that the
// exchange refuses: the stop loss is cancelled, the limit order too, and
// the step fails saying so.
func TestBinanceOrderUndone(t *testing.T) {
	f, env := binanceEnv(t)
	f.refuseTP = true
	_, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "BTCUSDT", "Type", "Limit", "Side", "Sell",
		"Time In Force", "Post Only", "Quantity", "0.01", "Price", "81000.06", "Stop Loss", "82000", "Take Profit", "79000")
	if err == nil || !strings.Contains(err.Error(), "the take profit at 79000.0 was refused") || !strings.Contains(err.Error(), "the order was cancelled") {
		t.Fatalf("err = %v", err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 1 || !strings.Contains(got[0], "timeInForce=GTX") || !strings.Contains(got[0], "price=81000.1") {
		t.Errorf("orders = %v", got)
	}
	if got := f.did("DELETE /fapi/v1/algoOrder"); len(got) != 1 || !strings.Contains(got[0], "algoId=21") {
		t.Errorf("stop loss cancelled: %v", got)
	}
	if got := f.did("DELETE /fapi/v1/order"); len(got) != 1 {
		t.Errorf("limit cancelled: %v", got)
	}

	// A market order whose stop is refused is closed at market.
	f.calls = nil
	_, err = run(t, env, "binance-order", "Connection", "1", "Symbol", "BTCUSDT", "Quantity", "0.01", "Take Profit", "90000")
	if err == nil || !strings.Contains(err.Error(), "closed at market") {
		t.Fatalf("err = %v", err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 2 || !strings.Contains(got[1], "reduceOnly=true") || !strings.Contains(got[1], "side=SELL") {
		t.Errorf("orders = %v", got)
	}
}

// Candles come from the network picked, Testnet by default; Last Closed is the
// newest that closed.
func TestBinanceCandles(t *testing.T) {
	f, env := binanceEnv(t)
	now := time.Now()
	f.klines = fmt.Sprintf(`[[%d,"1","2","0.5","1.5","10",%d,"15",3],[%d,"1.5","3","1","2.5","10",%d,"25",4]]`,
		now.Add(-2*time.Hour).UnixMilli(), now.Add(-time.Hour).UnixMilli()-1, now.Add(-time.Hour).UnixMilli(), now.Add(time.Hour).UnixMilli())
	steps, err := run(t, env, "binance-candles", "Symbol", "BTCUSDT", "Interval", "1h", "Limit", "2")
	if err != nil {
		t.Fatal(err)
	}
	if got := textOut(t, steps[1], "Close"); got != "1.5" {
		t.Errorf("Close = %q, want the closed candle's", got)
	}
	if n := len(objectOut(t, steps[1], "Candles").GetListValue().GetValues()); n != 2 {
		t.Errorf("%d candles", n)
	}
	if got := f.did("GET /fapi/v1/klines"); len(got) != 1 || !strings.Contains(got[0], "interval=1h") || !strings.Contains(got[0], "limit=2") {
		t.Errorf("klines = %v", got)
	}
	if _, err := run(t, env, "binance-candles", "Symbol", "BTCUSDT", "Limit", "5000"); err == nil || !strings.Contains(err.Error(), "Limit") {
		t.Errorf("limit 5000: %v", err)
	}
}

func TestBinanceReads(t *testing.T) {
	f, env := binanceEnv(t)
	steps, err := run(t, env, "binance-portfolio", "Connection", "1")
	if err != nil {
		t.Fatal(err)
	}
	if a := objectOut(t, steps[1], "Account").GetStructValue().GetFields(); a["wallet_balance"].GetNumberValue() != 5000 {
		t.Errorf("Account = %v", a)
	}
	if p := objectOut(t, steps[1], "Positions"); p.GetListValue() == nil {
		t.Errorf("Positions = %v, want an empty list", p)
	}

	if _, err := run(t, env, "binance-orders", "Connection", "1"); err == nil || !strings.Contains(err.Error(), "Symbol") {
		t.Errorf("history without a symbol: %v", err)
	}
	steps, err = run(t, env, "binance-orders", "Connection", "1", "Symbol", "BTCUSDT", "Limit", "20")
	if err != nil {
		t.Fatal(err)
	}
	if l := objectOut(t, steps[1], "Last").GetStructValue().GetFields(); l["order_id"].GetNumberValue() != 2 {
		t.Errorf("Last = %v, want the newest", l)
	}
	if _, err := run(t, env, "binance-orders", "Connection", "1", "Show", "Open"); err != nil {
		t.Error(err)
	}
	if got := f.did("GET /fapi/v1/openOrders"); len(got) != 1 || strings.Contains(got[0], "symbol") {
		t.Errorf("open orders = %v", got)
	}
	if _, err := run(t, env, "binance-orders", "Connection", "1", "Show", "Stop Orders", "Symbol", "BTCUSDT"); err != nil {
		t.Error(err)
	}
}

// Positions goes on by Found or None; Close Position closes a short with a
// reduce-only buy and cancels its stops, or goes on by None.
func TestBinancePositions(t *testing.T) {
	f, env := binanceEnv(t)
	steps, err := run(t, env, "binance-positions", "Connection", "1")
	if err != nil || strings.Join(steps[1].Fired, ",") != "None,Done" {
		t.Fatalf("no position: %v, %v", steps, err)
	}
	if _, err := run(t, env, "binance-close-position", "Connection", "1", "Symbol", "BTCUSDT"); err != nil {
		t.Fatal(err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 0 {
		t.Errorf("closed nothing with an order: %v", got)
	}

	f.positions = `[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.015","entryPrice":"80000","markPrice":"79000","unRealizedProfit":"15"}]`
	steps, err = run(t, env, "binance-positions", "Connection", "1", "Symbol", "BTCUSDT")
	if err != nil || strings.Join(steps[1].Fired, ",") != "Found,Done" {
		t.Fatalf("a position: %v, %v", steps, err)
	}
	if p := objectOut(t, steps[1], "Position").GetStructValue().GetFields(); p["side"].GetStringValue() != "SHORT" {
		t.Errorf("Position = %v", p)
	}
	steps, err = run(t, env, "binance-close-position", "Connection", "1", "Symbol", "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 1 || got[0] != "POST /fapi/v1/order?quantity=0.015&reduceOnly=true&side=BUY&symbol=BTCUSDT&type=MARKET" {
		t.Errorf("close = %v", got)
	}
	if got := f.did("DELETE /fapi/v1/algoOpenOrders"); len(got) != 1 {
		t.Errorf("stops cancelled: %v", got)
	}
	if l := logsOf(steps[1]); !strings.Contains(l, "testnet: closed the SHORT 0.015 BTCUSDT") {
		t.Errorf("logs = %q", l)
	}
}

// A step on a connection that is not Binance's says so.
func TestBinanceConnection(t *testing.T) {
	_, env := binanceEnv(t)
	if _, err := run(t, env, "binance-portfolio"); err == nil || !strings.Contains(err.Error(), "pick a Binance connection") {
		t.Errorf("no connection: %v", err)
	}
	if _, err := run(t, env, "binance-portfolio", "Connection", "7"); err == nil || !strings.Contains(err.Error(), "gone") {
		t.Errorf("gone: %v", err)
	}
}
