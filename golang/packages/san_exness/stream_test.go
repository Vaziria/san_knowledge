package san_exness

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// wsRoute serves a stream path: it accepts the (already verified) signed
// handshake, checks the first message, and sends the replies.
func wsRoute(t *testing.T, f *fakeExness, path, wantFirst string, replies ...string) {
	f.route("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Sec-WebSocket-Protocol"); got != "exness-ws-protocol" {
			t.Errorf("subprotocol %q", got)
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"exness-ws-protocol"}})
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(1 << 20)
		_, first, err := conn.Read(r.Context())
		if err != nil {
			t.Error(err)
			return
		}
		if string(first) != wantFirst {
			t.Errorf("first message %s\nwant %s", first, wantFirst)
		}
		for _, m := range replies {
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(m)); err != nil {
				return
			}
		}
		conn.Read(r.Context()) // until the client goes
	})
}

func TestEventsStream(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	big := strings.Repeat("x", 40000) // over coder/websocket's 32 KiB default read limit
	wsRoute(t, f, "/v1/server-events/accounts/123456/ws/events", `{"id":"sub-1","subscribe":{"event":"transactions"}}`,
		`{"event_type":"trading_state_snapshot","payload":{"orders":[],"positions":[{"position_id":"52","direction":"buy","instrument":"EURUSD","volume":"1","state":"open","create_time":"2026-11-14T02:00:00+00:00"}],"account_state":{"balance":"1000.00","equity":"1001.00","used_margin":"10.00"}}}`,
		`{"event_type":"transaction_event","operation_id":"900","client_request_id":"open-0001","status":"rejected","error_code":5006,"error_message":"TRADING_RULE_INSUFFICIENT_MARGIN","event_time":"2026-11-14T02:00:01+00:00","payload":{}}`,
		`{"event_type":"transaction_event","operation_id":"901","status":"success","event_time":"2026-11-14T02:00:02+00:00","payload":{"deals":[{"deal_id":"7","type":"open","create_time":"2026-11-14T02:00:02+00:00","profit":{"amount":"0","currency":"USD"}}],"account_state":{"balance":"1000.00","equity":"999.00","used_margin":"20.00","leverage":"200"}}}`,
		`{"id":"sub-2","code":3000,"error_message":"REQUEST_INVALID"}`,
		`{"event_type":"account_state_event","event_time":"2026-11-14T02:00:03+00:00","payload":{"account_state":{"balance":"1000.00","equity":"998.00","used_margin":"20.00"}}}`,
		`{"event_type":"instrument_event","event_time":"2026-11-14T02:00:04+00:00","action":"upd","payload":{"instrument":"EURUSD","point_digits":5,"description":"`+big+`"}}`,
		`{"event_type":"hmr_update","upserted_periods":[{"period_id":"3","event_type":"regular","instruments":["XAUUSD"],"leverage":"200","start_time":"2026-11-15 00:00:00","end_time":"2026-11-15 02:00:00"}],"removed_period_ids":["2"]}`,
		`{"event_type":"something_new","x":1}`,
	)
	s, err := c.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Subscribe(ctx, "sub-1", Transactions()); err != nil {
		t.Fatal(err)
	}
	next := func() Message {
		t.Helper()
		m, err := s.Next(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	if m := next(); m.Type != EventSnapshot || m.Snapshot == nil || m.Snapshot.Positions[0].PositionID != "52" || m.Snapshot.AccountState.Equity != "1001.00" {
		t.Fatalf("snapshot = %+v", m)
	}
	m := next()
	var oe *OperationError
	if m.Type != EventTransaction || m.Transaction.ClientRequestID != "open-0001" || !errors.As(m.Transaction.Err(), &oe) || oe.Code != CodeTradingRuleInsufficientMargin {
		t.Fatalf("rejected transaction = %+v", m.Transaction)
	}
	if m := next(); m.Transaction.Err() != nil || m.Transaction.Payload.Deals[0].DealID != "7" || m.Transaction.Payload.AccountState.Leverage != "200" {
		t.Fatalf("transaction = %+v", m.Transaction)
	}
	if m := next(); m.Type != EventError || m.Error.ID != "sub-2" || m.Error.Code != CodeRequestInvalid || !strings.Contains(m.Error.Error(), "REQUEST_INVALID") {
		t.Fatalf("error = %+v", m)
	}
	if m := next(); m.Type != EventAccountState || m.AccountState.State.Equity != "998.00" || m.AccountState.EventTime.Second() != 3 {
		t.Fatalf("account state = %+v", m.AccountState)
	}
	if m := next(); m.Type != EventInstrument || m.Instrument.Action != "upd" || m.Instrument.Condition.PointDigits != 5 || len(m.Instrument.Condition.Description) != len(big) {
		t.Fatalf("instrument = %+v", m.Type)
	}
	if m := next(); m.Type != EventHMRUpdate || m.HMR.Upserted[0].Leverage != "200" || m.HMR.Removed[0] != "2" {
		t.Fatalf("hmr = %+v", m.HMR)
	}
	if m := next(); m.Type != "something_new" || !strings.Contains(string(m.Raw), `"x":1`) {
		t.Fatalf("unknown event = %+v", m)
	}
}

func TestTicksStream(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	wsRoute(t, f, "/v1/server-events/accounts/123456/ws/ticks", `{"id":"t1","subscribe":{"event":"ticks","instruments":["XAUUSDm","EURUSDm"]}}`,
		`{"instrument":"XAUUSDm","bid":2650.1,"ask":2650.35,"timestamp":"2026-11-14T02:00:00.250+00:00"}`)
	s, err := c.Ticks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Ticks have their own stream: other subscriptions are refused here,
	// and ticks on the events stream.
	if err := s.Subscribe(ctx, "x", Transactions()); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("transactions on the ticks stream: %v", err)
	}
	if err := s.Subscribe(ctx, "t1", TicksOf("XAUUSDm", "EURUSDm")); err != nil {
		t.Fatal(err)
	}
	m, err := s.Next(ctx)
	if err != nil || m.Type != EventTick || m.Tick.Instrument != "XAUUSDm" || m.Tick.Ask != 2650.35 || m.Tick.Timestamp.Nanosecond() != 250e6 {
		t.Fatalf("tick = %+v, %v", m, err)
	}
	if err := (&Stream{events: true}).Subscribe(ctx, "y", TicksOf("EURUSD")); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ticks on the events stream: %v", err)
	}
	for name, sub := range map[string]Subscription{
		"ticks without instruments": TicksOf(),
		"bad instrument":            TicksOf("EUR USD"),
	} {
		if err := s.Subscribe(ctx, "z", sub); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The handshake is signed as a GET of the stream's path; a refused one is
// an *APIError, and a skewed clock is corrected as for REST.
func TestStreamHandshake(t *testing.T) {
	f := newFake(t)
	c := newTestClient(t, f)
	f.apiKey = "exnsk-other"
	_, err := c.Events(ctx)
	var e *APIError
	if !errors.As(err, &e) || e.Status != 401 || e.Code != CodeAuthInvalidAPIKey {
		t.Fatalf("err = %v", err)
	}

	g := newFake(t)
	g.clock = func() time.Time { return time.Now().Add(-5 * time.Minute) }
	wsRoute(t, g, "/v1/server-events/accounts/123456/ws/events", `{"id":"a","subscribe":{"event":"account_state"}}`)
	c2 := newTestClient(t, g)
	s, err := c2.Events(ctx)
	if err != nil {
		t.Fatalf("skewed handshake: %v", err)
	}
	defer s.Close()
	if err := s.Subscribe(ctx, "a", AccountStateUpdates()); err != nil {
		t.Fatal(err)
	}
	got := g.requests()
	if len(got) != 2 || got[1].URI != "/v1/server-events/accounts/123456/ws/events" || got[1].Method != "GET" {
		t.Fatalf("requests %+v", got)
	}
}

func TestSubscriptionMessages(t *testing.T) {
	for _, c := range []struct {
		sub  Subscription
		want string
	}{
		{Transactions(), `{"id":"i","subscribe":{"event":"transactions"}}`},
		{AccountStateUpdates(), `{"id":"i","subscribe":{"event":"account_state"}}`},
		{InstrumentUpdates(), `{"id":"i","subscribe":{"event":"instruments","instruments":["all"]}}`},
		{InstrumentUpdates("EURUSD"), `{"id":"i","subscribe":{"event":"instruments","instruments":["EURUSD"]}}`},
		{HMR("XAUUSD"), `{"id":"i","subscribe":{"event":"hmr","instruments":["XAUUSD"]}}`},
	} {
		b, _ := json.Marshal(command{ID: "i", Subscribe: &c.sub})
		if string(b) != c.want {
			t.Errorf("%s\nwant %s", b, c.want)
		}
	}
	b, _ := json.Marshal(command{ID: "u", Unsubscribe: &Subscription{Event: "transactions"}})
	if string(b) != `{"id":"u","unsubscribe":{"event":"transactions"}}` {
		t.Errorf("unsubscribe %s", b)
	}
}
