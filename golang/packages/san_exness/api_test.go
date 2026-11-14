package san_exness

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigurationAndMarketData(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	f.json("GET /v1/configuration/accounts/123456/instruments/XAUUSDm/conditions", 200, `{"instrument":"XAUUSDm","point_digits":3,"contract_size":"100","volume_min":"0.01","volume_max":"200","volume_step":"0.01","exec_mode":"market","trade_mode":"enabled","order_types":["market","limit","stop"],"trade_sessions":[{"trade_mode":"enabled","week_start_time":"1 00:05:00","week_end_time":"5 23:55:00"}],"commission_settings":{"commission_type":"per_lot_per_side","commission_value":{"amount":"3.50","currency":"USD"}}}`)
	in, err := c.Instrument(ctx, "XAUUSDm")
	if err != nil || in.PointDigits != 3 || in.VolumeStep != "0.01" || in.ContractSize != "100" || len(in.OrderTypes) != 3 ||
		in.CommissionSettings.CommissionValue.Amount != "3.50" || in.TradeSessions[0].WeekStartTime != "1 00:05:00" {
		t.Fatalf("instrument = %+v, %v", in, err)
	}

	f.json("GET /v1/market-data/accounts/123456/candles", 200, `{"candles":[{"time":"2026-11-14T08:00:00+00:00","open":2650.1,"high":2655,"low":2649.5,"close":2654.25,"volume":1200}]}`)
	from := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)
	candles, err := c.Candles(ctx, CandleQuery{Instrument: "XAUUSDm", Timeframe: M15, From: from, Count: 3})
	if err != nil || len(candles) != 1 || candles[0].Close != 2654.25 || !candles[0].Time.Equal(from) {
		t.Fatalf("candles = %+v, %v", candles, err)
	}
	if got := f.last().URI; got != "/v1/market-data/accounts/123456/candles?count=3&from=2026-11-14T08%3A00%3A00Z&instrument=XAUUSDm&timeframe=M15" {
		t.Errorf("candles URI %s", got)
	}
	f.json("GET /v1/market-data/accounts/123456/candles", 200, `{"candles":[{"time":"2026-11-14T08:00:00+00:00","bid":{"open":1,"high":2,"low":0.5,"close":1.5},"ask":{"open":1.1,"high":2.1,"low":0.6,"close":1.6},"volume":3}]}`)
	to := from.Add(time.Hour)
	candles, err = c.Candles(ctx, CandleQuery{Instrument: "EURUSD", Timeframe: H1, PriceType: PriceBoth, From: from, To: to})
	if err != nil || len(candles) != 1 || candles[0].Bid == nil || candles[0].Ask.Close != 1.6 || candles[0].Bid.Low != 0.5 {
		t.Fatalf("bid/ask candles = %+v, %v", candles, err)
	}
	if got := f.last().URI; !strings.Contains(got, "price_type=both") || !strings.Contains(got, "to=2026-11-14T09%3A00%3A00Z") || strings.Contains(got, "count=") {
		t.Errorf("candles URI %s", got)
	}
	for name, q := range map[string]CandleQuery{
		"no instrument":   {Timeframe: M1, From: from, Count: 1},
		"bad timeframe":   {Instrument: "EURUSD", Timeframe: "M2", From: from, Count: 1},
		"no from":         {Instrument: "EURUSD", Timeframe: M1, Count: 1},
		"count and to":    {Instrument: "EURUSD", Timeframe: M1, From: from, To: to, Count: 1},
		"neither":         {Instrument: "EURUSD", Timeframe: M1, From: from},
		"count too big":   {Instrument: "EURUSD", Timeframe: M1, From: from, Count: 1001},
		"to before from":  {Instrument: "EURUSD", Timeframe: M1, From: to, To: from},
		"bad price type":  {Instrument: "EURUSD", Timeframe: M1, From: from, Count: 1, PriceType: "mid"},
		"instrument path": {Instrument: "EUR/USD", Timeframe: M1, From: from, Count: 1},
	} {
		n := len(f.requests())
		if _, err := c.Candles(ctx, q); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
		if len(f.requests()) != n {
			t.Errorf("%s: sent anyway", name)
		}
	}
	if _, err := c.Candles(ctx, CandleQuery{Instrument: "EURUSD", Timeframe: M1, From: from, Count: -1000}); err != nil {
		t.Errorf("count -1000 (backwards) refused: %v", err)
	}
}

func TestTradingRequests(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	echo := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		json.NewEncoder(w).Encode(map[string]string{"operation_id": "900", "client_request_id": r.Header.Get("Exn-Idempotency-Key"), "status": "accepted"})
	}
	for _, p := range []string{"POST /v1/trading/accounts/123456/positions", "POST /v1/trading/accounts/123456/orders",
		"PATCH /v1/trading/accounts/123456/orders/41", "DELETE /v1/trading/accounts/123456/orders/41", "DELETE /v1/trading/accounts/123456/positions/52"} {
		f.route(p, echo)
	}
	cases := []struct {
		name       string
		call       func() (Ack, error)
		method     string
		uri        string
		body       string
		key        string // "" = generated
		wantKeyPfx string
	}{
		{"open", func() (Ack, error) {
			return c.OpenPosition(ctx, OpenPosition{Instrument: "XAUUSDm", Side: Sell, Volume: "0.10", StopLoss: "2700.5", TakeProfit: "2600", Comment: "t1"}, "open-0001")
		}, "POST", "/v1/trading/accounts/123456/positions",
			`{"instrument":"XAUUSDm","side":"sell","volume":"0.10","stop_loss_price":"2700.5","take_profit_price":"2600","comment":"t1"}`, "open-0001", ""},
		{"limit", func() (Ack, error) {
			return c.PlaceOrder(ctx, PendingOrder{Type: OrderLimit, Instrument: "EURUSD", Side: Buy, Volume: "1", Price: "1.0500"}, "")
		}, "POST", "/v1/trading/accounts/123456/orders", `{"type":"limit","instrument":"EURUSD","side":"buy","volume":"1","price":"1.0500"}`, "", "san-"},
		{"sltp", func() (Ack, error) {
			return c.PlaceOrder(ctx, PendingOrder{Type: OrderSLTP, PositionID: "52", StopLoss: "1.04"}, "sltp-0001")
		}, "POST", "/v1/trading/accounts/123456/orders", `{"type":"sltp","position_id":"52","stop_loss_price":"1.04"}`, "sltp-0001", ""},
		{"modify", func() (Ack, error) {
			return c.ModifyOrder(ctx, "41", ModifyOrder{Price: "1.06", TakeProfit: "1.09", RemoveStopLoss: true}, "mod-0001")
		}, "PATCH", "/v1/trading/accounts/123456/orders/41", `{"price":"1.06","stop_loss_price":null,"take_profit_price":"1.09"}`, "mod-0001", ""},
		{"cancel", func() (Ack, error) { return c.CancelOrder(ctx, "41", "cancel-0001") },
			"DELETE", "/v1/trading/accounts/123456/orders/41", "", "cancel-0001", ""},
		{"close part", func() (Ack, error) {
			return c.ClosePosition(ctx, "52", CloseOptions{Volume: "0.05", Price: "1.05", Deviation: "10"}, "close-0001")
		}, "DELETE", "/v1/trading/accounts/123456/positions/52?deviation=10&price=1.05&volume=0.05", "", "close-0001", ""},
		{"close all", func() (Ack, error) { return c.ClosePosition(ctx, "52", CloseOptions{}, "close-0002") },
			"DELETE", "/v1/trading/accounts/123456/positions/52", "", "close-0002", ""},
	}
	for _, tc := range cases {
		ack, err := tc.call()
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got := f.last()
		if got.Method != tc.method || got.URI != tc.uri || string(got.Body) != tc.body {
			t.Errorf("%s: sent %s %s %s\nwant %s %s %s", tc.name, got.Method, got.URI, got.Body, tc.method, tc.uri, tc.body)
		}
		if tc.body != "" && got.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s: content type %q", tc.name, got.Header.Get("Content-Type"))
		}
		if tc.key != "" && got.Idempotency != tc.key {
			t.Errorf("%s: idempotency key %q, want %q", tc.name, got.Idempotency, tc.key)
		}
		if tc.wantKeyPfx != "" && (!strings.HasPrefix(got.Idempotency, tc.wantKeyPfx) || !idempotentKey.MatchString(got.Idempotency)) {
			t.Errorf("%s: generated key %q", tc.name, got.Idempotency)
		}
		if ack.OperationID != "900" || ack.Status != "accepted" || ack.IdempotencyKey != got.Idempotency || ack.ClientRequestID != got.Idempotency {
			t.Errorf("%s: ack %+v", tc.name, ack)
		}
	}
	// Two generated keys differ.
	a1, _ := c.PlaceOrder(ctx, PendingOrder{Type: OrderStop, Instrument: "EURUSD", Side: Buy, Volume: "1", Price: "1.2"}, "")
	a2, _ := c.PlaceOrder(ctx, PendingOrder{Type: OrderStop, Instrument: "EURUSD", Side: Buy, Volume: "1", Price: "1.2"}, "")
	if a1.IdempotencyKey == a2.IdempotencyKey {
		t.Error("generated idempotency keys repeat")
	}
}

// Wrong requests are refused before anything is sent: numbers must be
// decimal strings, as the API takes them.
func TestTradingValidation(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	bad := map[string]func() error{
		"no instrument": func() error { _, err := c.OpenPosition(ctx, OpenPosition{Side: Buy, Volume: "1"}, ""); return err },
		"bad side": func() error {
			_, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: "long", Volume: "1"}, "")
			return err
		},
		"float volume": func() error {
			_, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: Buy, Volume: "1e-2"}, "")
			return err
		},
		"zero volume": func() error {
			_, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: Buy, Volume: "0"}, "")
			return err
		},
		"negative stop": func() error {
			_, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: Buy, Volume: "1", StopLoss: "-1"}, "")
			return err
		},
		"bad key": func() error {
			_, err := c.OpenPosition(ctx, OpenPosition{Instrument: "EURUSD", Side: Buy, Volume: "1"}, "a b")
			return err
		},
		"limit no price": func() error {
			_, err := c.PlaceOrder(ctx, PendingOrder{Type: OrderLimit, Instrument: "EURUSD", Side: Buy, Volume: "1"}, "")
			return err
		},
		"market pending": func() error {
			_, err := c.PlaceOrder(ctx, PendingOrder{Type: "market", Instrument: "EURUSD", Side: Buy, Volume: "1"}, "")
			return err
		},
		"sltp no position": func() error {
			_, err := c.PlaceOrder(ctx, PendingOrder{Type: OrderSLTP, StopLoss: "1"}, "")
			return err
		},
		"sltp no levels": func() error {
			_, err := c.PlaceOrder(ctx, PendingOrder{Type: OrderSLTP, PositionID: "5"}, "")
			return err
		},
		"sltp with volume": func() error {
			_, err := c.PlaceOrder(ctx, PendingOrder{Type: OrderSLTP, PositionID: "5", StopLoss: "1", Volume: "1"}, "")
			return err
		},
		"modify nothing": func() error { _, err := c.ModifyOrder(ctx, "41", ModifyOrder{}, ""); return err },
		"modify both stop": func() error {
			_, err := c.ModifyOrder(ctx, "41", ModifyOrder{StopLoss: "1", RemoveStopLoss: true}, "")
			return err
		},
		"order id path": func() error { _, err := c.CancelOrder(ctx, "41/../7", ""); return err },
		"position id":   func() error { _, err := c.ClosePosition(ctx, "x", CloseOptions{}, ""); return err },
		"close by + volume": func() error {
			_, err := c.ClosePosition(ctx, "5", CloseOptions{CloseByPositionID: "6", Volume: "1"}, "")
			return err
		},
		"operation id": func() error { _, err := c.Operation(ctx, "1;2"); return err },
	}
	for name, call := range bad {
		if err := call(); !errors.Is(err, ErrInvalidRequest) && !errors.Is(err, ErrInvalidIdempotencyKey) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Fatalf("%d requests sent", n)
	}
}

func TestSnapshotAndHistory(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	f.json("GET /v1/trading/accounts/123456/snapshot", 200, `{"orders":[{"order_id":"41","order_type":"limit","state":"placed","direction":"buy","instrument":"EURUSD","price":"1.05","create_time":"2026-11-14T01:00:00+00:00","volume":"1"}],"positions":[{"position_id":"52","direction":"sell","instrument":"XAUUSDm","volume":"0.10","open_price":"2690.5","state":"open","create_time":"2026-11-14T02:00:00+00:00","swap":"-0.12"}],"account_state":{"balance":"1000.00","equity":"990.50","used_margin":"12.30"}}`)
	s, err := c.Snapshot(ctx)
	if err != nil || len(s.Orders) != 1 || s.Orders[0].State != "placed" || s.Positions[0].OpenPrice != "2690.5" || s.AccountState.Equity != "990.50" {
		t.Fatalf("snapshot = %+v, %v", s, err)
	}

	f.json("GET /v1/history/accounts/123456/deals", 200, `{"items":[{"event_time":"2026-11-13T05:00:00+00:00","deal":{"deal_id":"7","order_id":"40","position_id":"51","type":"close","direction":"buy","instrument":"EURUSD","volume":"1","price":"1.07","profit":{"amount":"-12.40","currency":"USD"},"create_time":"2026-11-13T05:00:00+00:00","commission":"-3.50"}}],"next_cursor":"abc==","has_more":true}`)
	from, to := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	page, err := c.Deals(ctx, HistoryQuery{From: from, To: to, Limit: 50, Instrument: "EURUSD", DealType: "close"})
	if err != nil || len(page.Items) != 1 || page.Items[0].Deal.Profit.Amount != "-12.40" || page.NextCursor != "abc==" || !page.HasMore {
		t.Fatalf("deals = %+v, %v", page, err)
	}
	if got := f.last().URI; got != "/v1/history/accounts/123456/deals?deal_type=close&from=2026-11-01T00%3A00%3A00Z&instrument=EURUSD&limit=50&to=2026-11-14T00%3A00%3A00Z" {
		t.Errorf("deals URI %s", got)
	}
	// A cursor goes alone, with the limit only.
	if _, err := c.Deals(ctx, HistoryQuery{From: from, Cursor: page.NextCursor, Limit: 50, DealType: "close"}); err != nil {
		t.Fatal(err)
	}
	if got := f.last().URI; got != "/v1/history/accounts/123456/deals?cursor=abc%3D%3D&limit=50" {
		t.Errorf("next page URI %s", got)
	}

	f.json("GET /v1/history/accounts/123456/orders", 200, `{"items":[{"event_time":"2026-11-13T05:00:00+00:00","order":{"order_id":"40","order_type":"market","state":"filled","create_time":"2026-11-13T05:00:00+00:00","reason":"sl"}}],"has_more":false}`)
	op, err := c.Orders(ctx, HistoryQuery{From: from, OrderState: "filled"})
	if err != nil || op.Items[0].Order.Reason != "sl" || op.HasMore {
		t.Fatalf("orders = %+v, %v", op, err)
	}
	if got := f.last().URI; got != "/v1/history/accounts/123456/orders?from=2026-11-01T00%3A00%3A00Z&order_state=filled" {
		t.Errorf("orders URI %s", got)
	}
	for name, q := range map[string]HistoryQuery{
		"no from or cursor": {},
		"limit 201":         {From: from, Limit: 201},
		"to before from":    {From: to, To: from},
		"bad state":         {From: from, OrderState: "open"},
	} {
		if _, err := c.Orders(ctx, q); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// WaitOperation polls until the operation is final; a refused one is an
// *OperationError carrying Exness's code.
func TestWaitOperation(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	var calls atomic.Int32
	f.route("GET /v1/trading/accounts/123456/operations/900", func(w http.ResponseWriter, r *http.Request) {
		status := "pending"
		if calls.Add(1) >= 3 {
			status = "confirmed"
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"operation_id":"900","status":"` + status + `","client_request_id":"open-0001","created_at":"2026-11-14T01:00:00+00:00","updated_at":"2026-11-14T01:00:01+00:00"}`))
	})
	op, err := c.WaitOperation(ctx, "900", time.Millisecond)
	if err != nil || op.Status != "confirmed" || calls.Load() != 3 || op.ClientRequestID != "open-0001" {
		t.Fatalf("op = %+v, %v after %d calls", op, err, calls.Load())
	}

	f.json("GET /v1/trading/accounts/123456/operations/901", 200, `{"operation_id":"901","status":"rejected","error_code":5006,"error_message":"TRADING_RULE_INSUFFICIENT_MARGIN","created_at":"2026-11-14T01:00:00+00:00","updated_at":"2026-11-14T01:00:01+00:00"}`)
	bounded, stop := contextWithTimeout(2 * time.Second) // a rejection that is not final would poll forever
	defer stop()
	op, err = c.WaitOperation(bounded, "901", time.Millisecond)
	var oe *OperationError
	if !errors.As(err, &oe) || oe.Code != CodeTradingRuleInsufficientMargin || oe.Status != "rejected" || op.Status != "rejected" {
		t.Fatalf("op = %+v, err = %v", op, err)
	}

	f.json("GET /v1/trading/accounts/123456/operations/902", 200, `{"operation_id":"902","status":"pending","created_at":"2026-11-14T01:00:00+00:00","updated_at":"2026-11-14T01:00:01+00:00"}`)
	short, cancel := contextWithTimeout(30 * time.Millisecond)
	defer cancel()
	if _, err := c.WaitOperation(short, "902", 5*time.Millisecond); !errors.Is(err, errDeadline) {
		t.Fatalf("err = %v, want the deadline", err)
	}
}
