package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The tests are autotrade's (internal/binance), with the tower's additions.

// server answers /fapi/v1/time with a clock 5 minutes behind the local one,
// and checks every signed request's key, signature and timestamp.
func server(t *testing.T, handle func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	skew := -5 * time.Minute
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fapi/v1/time" {
			fmt.Fprintf(w, `{"serverTime":%d}`, time.Now().Add(skew).UnixMilli())
			return
		}
		if sig := r.URL.Query().Get("signature"); sig != "" {
			if r.Header.Get("X-MBX-APIKEY") != "key" {
				t.Errorf("%s: api key header %q", r.URL.Path, r.Header.Get("X-MBX-APIKEY"))
			}
			payload := strings.TrimSuffix(r.URL.RawQuery, "&signature="+sig)
			mac := hmac.New(sha256.New, []byte("secret"))
			mac.Write([]byte(payload))
			if want := hex.EncodeToString(mac.Sum(nil)); sig != want {
				t.Errorf("%s: bad signature", r.URL.Path)
			}
			var ts int64
			fmt.Sscan(r.URL.Query().Get("timestamp"), &ts)
			if d := time.Since(time.UnixMilli(ts)) + skew; d > time.Second || d < -time.Second {
				t.Errorf("%s: timestamp off the server clock by %v", r.URL.Path, d)
			}
		}
		handle(w, r)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "key", "secret")
}

func TestSignedRequestUsesServerClock(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/fapi/v3/account" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		fmt.Fprint(w, `{"totalWalletBalance":"15000.5","availableBalance":"14000","totalMarginBalance":"15010","totalUnrealizedProfit":"9.5"}`)
	})
	acc, err := c.Account(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if acc.WalletBalance != 15000.5 || acc.UnrealizedPnL != 9.5 {
		t.Errorf("account = %+v", acc)
	}
	if off := c.Offset(); off > -4*time.Minute {
		t.Errorf("offset = %v, want about -5m", off)
	}
}

func TestNoKey(t *testing.T) {
	c := New("http://127.0.0.1:1", "", "")
	if _, err := c.Account(context.Background()); !errors.Is(err, ErrNoKey) {
		t.Errorf("err = %v, want ErrNoKey", err)
	}
}

func TestAPIErrorAndTimestampRetry(t *testing.T) {
	var calls atomic.Int32
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"code":-1021,"msg":"Timestamp for this request is outside of the recvWindow."}`)
			return
		}
		w.WriteHeader(400)
		fmt.Fprint(w, `{"code":-2019,"msg":"Margin is insufficient."}`)
	})
	_, err := c.PlaceOrder(context.Background(), NewOrder{Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: "0.010"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != -2019 {
		t.Fatalf("err = %v, want -2019 after one retry", err)
	}
	if calls.Load() != 2 {
		t.Errorf("%d calls, want 2", calls.Load())
	}
}

// A market order sends no price; a limit order its price and time in
// force; a post-only one that would take is ErrWouldTake.
func TestPlaceOrder(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != http.MethodPost || r.URL.Path != "/fapi/v1/order" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		switch q.Get("type") {
		case "MARKET":
			if q.Has("price") || q.Has("timeInForce") || q.Get("reduceOnly") != "true" {
				t.Errorf("market order query %v", q)
			}
			fmt.Fprint(w, `{"orderId":41,"symbol":"BTCUSDT","status":"FILLED","side":"SELL","type":"MARKET","reduceOnly":true,"origQty":"0.006","executedQty":"0.006","avgPrice":"80010.5","updateTime":1791621317552}`)
		case "LIMIT":
			if q.Get("price") == "80100.0" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"code":-5022,"msg":"Due to the order could not be executed as maker, the Post Only order will be rejected."}`)
				return
			}
			if q.Get("timeInForce") != "GTC" || q.Get("price") != "79900.0" {
				t.Errorf("limit order query %v", q)
			}
			fmt.Fprint(w, `{"orderId":42,"symbol":"BTCUSDT","status":"NEW","side":"BUY","type":"LIMIT","timeInForce":"GTC","price":"79900.0","origQty":"0.006","avgPrice":"0.00","executedQty":"0","updateTime":1791621317552}`)
		}
	})
	ctx := context.Background()
	m, err := c.PlaceOrder(ctx, NewOrder{Symbol: "BTCUSDT", Side: "SELL", Type: "MARKET", Quantity: "0.006", ReduceOnly: true})
	if err != nil || m.Status != "FILLED" || m.AvgPrice != 80010.5 || !m.ReduceOnly {
		t.Fatalf("market = %+v, %v", m, err)
	}
	l, err := c.PlaceOrder(ctx, NewOrder{Symbol: "BTCUSDT", Side: "BUY", Type: "LIMIT", Quantity: "0.006", Price: "79900.0", TimeInForce: "GTC"})
	if err != nil || l.OrderID != 42 || l.Price != 79900 || !l.Working() || l.TimeInForce != "GTC" {
		t.Fatalf("limit = %+v, %v", l, err)
	}
	if _, err := c.PlaceOrder(ctx, NewOrder{Symbol: "BTCUSDT", Side: "BUY", Type: "LIMIT", Quantity: "0.006", Price: "80100.0", TimeInForce: "GTX"}); !errors.Is(err, ErrWouldTake) {
		t.Errorf("err = %v, want ErrWouldTake", err)
	}
}

func TestReduceStopOrderHasQuantity(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/fapi/v1/algoOrder" {
			t.Errorf("got %s", r.URL.Path)
		}
		for k, v := range map[string]string{"algoType": "CONDITIONAL", "type": "STOP_MARKET", "quantity": "0.006", "reduceOnly": "true", "workingType": "MARK_PRICE"} {
			if q.Get(k) != v {
				t.Errorf("%s = %q, want %q", k, q.Get(k), v)
			}
		}
		if q.Has("closePosition") {
			t.Error("closePosition sent with a quantity")
		}
		fmt.Fprint(w, `{"algoId":7,"symbol":"BTCUSDT","orderType":"STOP_MARKET","side":"SELL","triggerPrice":"79500.0","quantity":"0.006","closePosition":false,"algoStatus":"NEW"}`)
	})
	o, err := c.ReduceStopOrder(context.Background(), "BTCUSDT", "SELL", StopMarket, "79500.0", "0.006")
	if err != nil || o.AlgoID != 7 || o.Quantity != 0.006 || o.ClosePosition || o.Symbol != "BTCUSDT" {
		t.Fatalf("order = %+v, err = %v", o, err)
	}
}

func TestKlinesAndSymbolInfo(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/fapi/v1/klines":
			fmt.Fprint(w, `[[1791537300000,"82532.00","82626.90","82519.90","82621.60","14609.4266",1791538199999,"1206575105.937150",3735,"14577.1355","1203909152.594290","0"]]`)
		case "/fapi/v1/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"ETHUSDT","filters":[]},{"symbol":"BTCUSDT","status":"TRADING","filters":[
				{"filterType":"PRICE_FILTER","tickSize":"0.10","minPrice":"261.10"},
				{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"1000"},
				{"filterType":"MARKET_LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"120"},
				{"filterType":"MIN_NOTIONAL","notional":"100"}]}]}`)
		}
	})
	ks, err := c.Klines(context.Background(), "BTCUSDT", "15m", 1)
	if err != nil {
		t.Fatal(err)
	}
	k := ks[0]
	if k.Close != 82621.6 || k.QuoteVolume != 1206575105.93715 || k.Trades != 3735 || k.CloseTime.UnixMilli() != 1791538199999 {
		t.Errorf("candle = %+v", k)
	}
	info, err := c.SymbolInfo(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if info.TickSize != 0.1 || info.PriceDec != 1 || info.StepSize != 0.001 || info.QtyDec != 3 || info.MaxQty != 120 || info.MinNotional != 100 {
		t.Errorf("info = %+v", info)
	}
	if q := info.FloorQty(0.0129999); info.FormatQty(q) != "0.012" {
		t.Errorf("floor qty = %s", info.FormatQty(q))
	}
	if p := info.RoundPrice(81900.04); info.FormatPrice(p) != "81900.0" {
		t.Errorf("round price = %s", info.FormatPrice(p))
	}
	if _, err := c.SymbolInfo(context.Background(), "NOPEUSDT"); err == nil || !strings.Contains(err.Error(), "unknown symbol NOPEUSDT") {
		t.Errorf("unknown symbol: %v", err)
	}
}

// Positions and orders of every symbol send no symbol; the orders come
// newest first; empty positions and balances are left out.
func TestListsAndHistory(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/fapi/v3/positionRisk":
			if q.Has("symbol") {
				t.Errorf("positions of every symbol sent symbol %q", q.Get("symbol"))
			}
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"-0.010","entryPrice":"80000","markPrice":"79000","unRealizedProfit":"10","updateTime":1791621317552},
				{"symbol":"ETHUSDT","positionSide":"BOTH","positionAmt":"0"}]`)
		case "/fapi/v3/balance":
			fmt.Fprint(w, `[{"asset":"USDT","balance":"5000.5","availableBalance":"4900","crossUnPnl":"10"},{"asset":"BNB","balance":"0","crossUnPnl":"0"}]`)
		case "/fapi/v1/allOrders":
			if q.Get("symbol") != "BTCUSDT" || q.Get("limit") != "50" {
				t.Errorf("allOrders query %v", q)
			}
			fmt.Fprint(w, `[{"orderId":1,"symbol":"BTCUSDT","status":"FILLED","time":1791621000000},{"orderId":2,"symbol":"BTCUSDT","status":"CANCELED","time":1791622000000}]`)
		}
	})
	ctx := context.Background()
	ps, err := c.Positions(ctx, "")
	if err != nil || len(ps) != 1 || ps[0].Side != "SHORT" || ps[0].Amount != -0.01 {
		t.Errorf("positions = %+v, %v", ps, err)
	}
	bs, err := c.Balances(ctx)
	if err != nil || len(bs) != 1 || bs[0].Asset != "USDT" || bs[0].Balance != 5000.5 {
		t.Errorf("balances = %+v, %v", bs, err)
	}
	os, err := c.AllOrders(ctx, "BTCUSDT", 50)
	if err != nil || len(os) != 2 || os[0].OrderID != 2 || os[1].OrderID != 1 {
		t.Errorf("orders = %+v, %v", os, err)
	}
}

// A market order answered with an average price of 0 (as the test network
// does) is read back for its price.
func TestMarketOrderPriceReadBack(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /fapi/v1/order":
			fmt.Fprint(w, `{"orderId":9,"symbol":"XRPUSDT","status":"FILLED","side":"BUY","type":"MARKET","origQty":"10","executedQty":"10","avgPrice":"0.0000"}`)
		case "GET /fapi/v1/order":
			if r.URL.Query().Get("orderId") != "9" {
				t.Errorf("read back %v", r.URL.Query())
			}
			fmt.Fprint(w, `{"orderId":9,"symbol":"XRPUSDT","status":"FILLED","avgPrice":"1.2984","executedQty":"10","time":1791621317552,"updateTime":1791621317552}`)
		}
	})
	o, err := c.PlaceOrder(context.Background(), NewOrder{Symbol: "XRPUSDT", Side: "BUY", Type: "MARKET", Quantity: "10"})
	if err != nil || o.AvgPrice != 1.2984 || o.Time.IsZero() {
		t.Errorf("order = %+v, %v", o, err)
	}
}
