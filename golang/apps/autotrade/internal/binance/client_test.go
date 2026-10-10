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
	_, err := c.MarketOrder(context.Background(), "BTCUSDT", "BUY", "0.010", false)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != -2019 {
		t.Fatalf("err = %v, want -2019 after one retry", err)
	}
	if calls.Load() != 2 {
		t.Errorf("%d calls, want 2", calls.Load())
	}
}

func TestStopOrderUsesAlgoAPI(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != http.MethodPost || r.URL.Path != "/fapi/v1/algoOrder" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		for k, v := range map[string]string{"algoType": "CONDITIONAL", "type": "STOP_MARKET", "side": "SELL", "triggerPrice": "81900.0", "closePosition": "true", "workingType": "MARK_PRICE"} {
			if q.Get(k) != v {
				t.Errorf("%s = %q, want %q", k, q.Get(k), v)
			}
		}
		if q.Has("quantity") {
			t.Error("quantity sent with closePosition")
		}
		fmt.Fprint(w, `{"algoId":2146760,"orderType":"STOP_MARKET","side":"SELL","triggerPrice":"81900.0","closePosition":true,"algoStatus":"NEW","quantity":"","icebergQuantity":"null"}`)
	})
	o, err := c.StopOrder(context.Background(), "BTCUSDT", "SELL", StopMarket, "81900.0")
	if err != nil {
		t.Fatal(err)
	}
	if o.AlgoID != 2146760 || o.TriggerPrice != 81900 || !o.ClosePosition {
		t.Errorf("order = %+v", o)
	}
}

func TestLimitOrderIsPostOnly(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for k, v := range map[string]string{"type": "LIMIT", "timeInForce": "GTX", "side": "BUY", "quantity": "0.006"} {
			if q.Get(k) != v {
				t.Errorf("%s = %q, want %q", k, q.Get(k), v)
			}
		}
		if q.Get("price") == "80100.0" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"code":-5022,"msg":"Due to the order could not be executed as maker, the Post Only order will be rejected."}`)
			return
		}
		fmt.Fprint(w, `{"orderId":42,"status":"NEW","side":"BUY","type":"LIMIT","price":"79900.0","origQty":"0.006","avgPrice":"0.00","executedQty":"0","updateTime":1791621317552}`)
	})
	o, err := c.LimitOrder(context.Background(), "BTCUSDT", "BUY", "0.006", "79900.0")
	if err != nil || o.OrderID != 42 || o.Price != 79900 || o.OrigQty != 0.006 || !o.Working() {
		t.Fatalf("order = %+v, err = %v", o, err)
	}
	if _, err := c.LimitOrder(context.Background(), "BTCUSDT", "BUY", "0.006", "80100.0"); !errors.Is(err, ErrWouldTake) {
		t.Errorf("err = %v, want ErrWouldTake", err)
	}
}

func TestReduceStopOrderHasQuantity(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		for k, v := range map[string]string{"type": "STOP_MARKET", "quantity": "0.006", "reduceOnly": "true", "workingType": "MARK_PRICE"} {
			if q.Get(k) != v {
				t.Errorf("%s = %q, want %q", k, q.Get(k), v)
			}
		}
		if q.Has("closePosition") {
			t.Error("closePosition sent with a quantity")
		}
		fmt.Fprint(w, `{"algoId":7,"orderType":"STOP_MARKET","side":"SELL","triggerPrice":"79500.0","quantity":"0.006","closePosition":false,"algoStatus":"NEW"}`)
	})
	o, err := c.ReduceStopOrder(context.Background(), "BTCUSDT", "SELL", StopMarket, "79500.0", "0.006")
	if err != nil || o.AlgoID != 7 || o.Quantity != 0.006 || o.ClosePosition {
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
	if q := info.FloorQty(0.012); info.FormatQty(q) != "0.012" {
		t.Errorf("floor of an exact step = %s", info.FormatQty(q))
	}
	if p := info.RoundPrice(81900.04); info.FormatPrice(p) != "81900.0" {
		t.Errorf("round price = %s", info.FormatPrice(p))
	}
}

func TestPositioning(t *testing.T) {
	c := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("period") != "1h" && r.URL.Path != "/fapi/v1/fundingRate" {
			t.Errorf("%s: period %q", r.URL.Path, r.URL.Query().Get("period"))
		}
		switch r.URL.Path {
		case "/futures/data/openInterestHist":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","sumOpenInterest":"92848.3","sumOpenInterestValue":"7674820654.2","timestamp":1791536400000}]`)
		case "/futures/data/takerlongshortRatio":
			fmt.Fprint(w, `[{"buySellRatio":"0.9239","sellVol":"954.6240","buyVol":"881.9500","timestamp":1791536400000}]`)
		case "/futures/data/topLongShortPositionRatio":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","longAccount":"0.6110","longShortRatio":"1.5704","shortAccount":"0.3890","timestamp":1791540000000}]`)
		case "/fapi/v1/fundingRate":
			fmt.Fprint(w, `[{"symbol":"BTCUSDT","fundingTime":1791532800000,"fundingRate":"0.00005156","markPrice":"82620.1"}]`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	})
	ctx := context.Background()
	oi, err := c.OpenInterestHist(ctx, "BTCUSDT", "1h", 1)
	if err != nil || oi[0].OI != 92848.3 {
		t.Errorf("oi %+v %v", oi, err)
	}
	tk, err := c.TakerRatio(ctx, "BTCUSDT", "1h", 1)
	if err != nil || tk[0].Ratio != 0.9239 || tk[0].Buy != 881.95 || tk[0].Sell != 954.624 {
		t.Errorf("taker %+v %v", tk, err)
	}
	ls, err := c.LongShortRatio(ctx, TopPositions, "BTCUSDT", "1h", 1)
	if err != nil || ls[0].Ratio != 1.5704 {
		t.Errorf("long/short %+v %v", ls, err)
	}
	fr, err := c.FundingHistory(ctx, "BTCUSDT", 1)
	if err != nil || fr[0].Rate != 0.00005156 || fr[0].Time.UnixMilli() != 1791532800000 {
		t.Errorf("funding %+v %v", fr, err)
	}
}
