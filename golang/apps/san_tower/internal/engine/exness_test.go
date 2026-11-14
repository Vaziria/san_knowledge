package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	exness "github.com/wargasipil/san_exness"
	"github.com/wargasipil/san_exness/exnesstest"
	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// exnessEnv is an Exness connection (1) on a fake Access Point, allowed to
// trade or not.
func exnessEnv(t *testing.T, allow bool) (*exnesstest.Server, Env) {
	t.Helper()
	f := exnesstest.New(t)
	return f, Env{
		Exness: NewExnessClients(f.Client()),
		Connection: func(id uint32) (store.Connection, error) {
			if id != 1 {
				return store.Connection{}, store.ErrNotFound
			}
			return store.Connection{ID: 1, Name: "Exness test", Type: store.Exness, Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Exness{Exness: &v1.ExnessSettings{
				ApiKey: exnesstest.APIKey, PrivateKey: exnesstest.PrivateKey, AccountId: exnesstest.Account, AccessPoint: f.URL, AllowTrading: allow,
			}}}}, nil
		},
	}
}

// exnessClient is a client of the fake, to set an account up as by hand.
func exnessClient(t *testing.T, f *exnesstest.Server) *exness.Client {
	t.Helper()
	c, err := exness.New(f.Config())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// exnessStep runs one step of kind on connection 1 with an Order object,
// as an AI's order through a Workflow Tool gives it.
func exnessStep(t *testing.T, env Env, kind string, order map[string]any, values ...string) (*call, error) {
	t.Helper()
	st := step("s", kind, append([]string{"Connection", "1"}, values...)...)
	c := &call{ctx: context.Background(), env: env, step: st, inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
	for _, s := range byKind[kind].Inputs {
		if s.Field {
			c.inputs[s.Name] = wrapperspb.String(st.Values[s.Name])
		}
	}
	v, err := structpb.NewValue(order)
	if err != nil {
		t.Fatal(err)
	}
	c.inputs["Order"] = v
	return c, byKind[kind].run(c)
}

// A market buy: the volume is rounded down to the step, the stops to the
// instrument's digits, checked against the limits at the ask, and the step
// waits until Exness confirms.
func TestExnessMarketOrder(t *testing.T) {
	f, env := exnessEnv(t, true)
	steps, err := run(t, env, "exness-order", "Connection", "1", "Instrument", " XAUUSD ", "Volume", "0.109", "Stop Loss", "2640.0004", "Take Profit", "2700",
		"Require Stop", "Yes", "Max Volume", "1", "Max Loss", "150")
	if err != nil {
		t.Fatal(err)
	}
	snap := f.Snapshot()
	if len(snap.Positions) != 1 || snap.Positions[0].Volume != "0.10" || snap.Positions[0].Direction != exness.Buy || snap.Positions[0].Instrument != "XAUUSD" ||
		len(snap.Orders) != 1 || snap.Orders[0].StopLossPrice != "2640.000" || snap.Orders[0].TakeProfitPrice != "2700.000" {
		t.Fatalf("account = %+v", snap)
	}
	if id := textOut(t, steps[1], "Operation ID"); id == "" {
		t.Error("no Operation ID")
	}
	if o := objectOut(t, steps[1], "Order").GetStructValue().GetFields(); o["volume"].GetStringValue() != "0.10" || o["status"].GetStringValue() != "confirmed" ||
		o["side"].GetStringValue() != "buy" || o["type"].GetStringValue() != "market" || o["price"].GetStringValue() != "2650.200" {
		t.Errorf("Order = %v", o)
	}
	want := "Exness 123456: buy 0.10 XAUUSD at market (2650.200), stop loss 2640.000, take profit 2700.000: confirmed"
	if l := logsOf(steps[1]); !strings.HasPrefix(l, want) {
		t.Errorf("logs = %q\nwant %q", l, want)
	}
}

// Everything that would make an order unsafe or wrong is refused before it
// reaches Exness, and Exness's own refusal fails the step with its code.
func TestExnessOrderRefused(t *testing.T) {
	f, env := exnessEnv(t, true)
	gbp := exness.InstrumentCondition{Instrument: "EURGBP", BaseCurrency: "EUR", QuoteCurrency: "GBP", PointDigits: 5, ContractSize: "100000",
		VolumeMin: "0.01", VolumeMax: "200", VolumeStep: "0.01", TradeMode: "enabled"}
	f.AddInstrument(gbp, 0.86000, 0.86010)
	for _, c := range []struct {
		values []string
		want   string
	}{
		// 0.10 lot of gold is 10 ounces: 10.2 lower loses 102 USD.
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Require Stop", "Yes"}, "Require Stop: the order needs a Stop Loss"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Max Loss", "500"}, "Require Stop: the order needs a Stop Loss"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Max Volume", "0.05"}, "Max Volume: 0.10 lots is over 0.05"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Stop Loss", "2640", "Max Loss", "50"},
			"Max Loss: 0.10 lots lose 102.00 USD at the stop 2640.000 (10.200 from 2650.200), over 50.00; at most 0.04"},
		// USDJPY's profit is in yen: 0.5 yen on 1 lot (100000 USD) is 50000 yen, 332.23 USD at the stop.
		{[]string{"Instrument", "USDJPY", "Side", "Sell", "Volume", "1", "Stop Loss", "150.5", "Max Loss", "300"},
			"Max Loss: 1.00 lots lose 332.23 USD at the stop 150.500 (0.500 from 150.000), over 300.00; at most 0.90"},
		{[]string{"Instrument", "EURGBP", "Volume", "1", "Stop Loss", "0.85", "Max Loss", "300"}, "Max Loss: EURGBP's profit is in GBP"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Stop Loss", "2660"}, "Stop Loss 2660.000 is on the wrong side of the entry, 2650.200: below it for a buy"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Side", "Sell", "Take Profit", "2660"}, "Take Profit 2660.000 is on the wrong side of the entry, 2650.000: below it for a sell"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.009"}, "Volume 0.009 is below XAUUSD's least, 0.01 lots (step 0.01)"},
		{[]string{"Instrument", "XAUUSD", "Volume", "201"}, "Volume 201.00 is over XAUUSD's most, 200 lots"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Type", "Limit"}, "Price is empty"},
		{[]string{"Instrument", "XAU USD", "Volume", "0.1"}, `Instrument "XAU USD" is not an Exness instrument`},
		{[]string{"Instrument", "GOLD", "Volume", "0.1"}, "MARKET_INSTRUMENT_NOT_FOUND"},
		{[]string{"Instrument", "XAUUSD", "Volume", "0.1", "Max Loss", "-5"}, `Max Loss "-5": give USD, a number above 0`},
	} {
		values := append([]string{"Connection", "1"}, c.values...)
		if _, err := run(t, env, "exness-order", values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v\nwant %q", c.values, err, c.want)
		}
	}
	if got := f.Calls("POST"); len(got) != 0 {
		t.Errorf("orders sent: %v", got)
	}
	// Exness refuses after the ACK: the step fails, saying why.
	f.RejectNext(exness.CodeTradingRuleInsufficientMargin)
	if _, err := run(t, env, "exness-order", "Connection", "1", "Instrument", "EURUSD", "Volume", "1"); err == nil ||
		!strings.Contains(err.Error(), "Exness 123456: buy 1.00 EURUSD refused: TRADING_RULE_INSUFFICIENT_MARGIN (5006)") {
		t.Errorf("rejected: %v", err)
	}
	if n := len(f.Snapshot().Positions); n != 0 {
		t.Errorf("%d positions after a rejection", n)
	}

	// A connection without Allow Trading only reads: no order is sent.
	f2, readOnly := exnessEnv(t, false)
	for _, kind := range []string{"exness-order", "exness-close-position", "exness-move-stop"} {
		_, err := run(t, readOnly, kind, "Connection", "1", "Instrument", "XAUUSD", "Volume", "0.1", "Stop Loss", "2640")
		if err == nil || !strings.Contains(err.Error(), "Exness test: Allow Trading is off") {
			t.Errorf("%s read-only: %v", kind, err)
		}
	}
	if got := f2.Calls(""); len(got) != 0 {
		t.Errorf("read-only reached Exness: %v", got)
	}
}

// An AI's order as one object wins over the settings: a sell limit with a
// stop loss, checked against the limits at its own price.
func TestExnessOrderObject(t *testing.T) {
	f, env := exnessEnv(t, true)
	c, err := exnessStep(t, env, "exness-order", map[string]any{"instrument": "EURUSD", "side": "sell", "type": "limit", "volume": 0.5, "price": 1.09,
		"stop_loss": 1.095, "take_profit": 1.07}, "Side", "Buy", "Type", "Market", "Max Loss", "300")
	if err != nil {
		t.Fatal(err)
	}
	// 0.005 above 1.09 on 0.5 lot (50000 EUR) loses 250 USD: under 300.
	snap := f.Snapshot()
	if len(snap.Orders) != 1 || snap.Orders[0].OrderType != "limit" || snap.Orders[0].Direction != exness.Sell || snap.Orders[0].Price != "1.09000" ||
		snap.Orders[0].StopLossPrice != "1.09500" || snap.Orders[0].Volume != "0.50" {
		t.Fatalf("orders = %+v", snap.Orders)
	}
	if !slicesContain(c.fired, "Done") {
		t.Errorf("fired %v", c.fired)
	}
	for _, o := range []struct {
		order map[string]any
		want  string
	}{
		{map[string]any{"instrument": "EURUSD", "side": "up", "volume": 1}, `Order: side "up": buy or sell`},
		{map[string]any{"instrument": "EURUSD", "type": "trailing", "volume": 1}, `Order: type "trailing": market, limit or stop`},
		{map[string]any{"symbol": "EURUSD", "side": "buy", "quantity": 1, "stop_loss": 1.07, "price": 1.08}, "Max Loss: 1.00 lots lose 1010.00 USD"},
	} {
		if _, err := exnessStep(t, env, "exness-order", o.order, "Max Loss", "300"); err == nil || !strings.Contains(err.Error(), o.want) {
			t.Errorf("%v: %v\nwant %q", o.order, err, o.want)
		}
	}
}

func slicesContain(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// The read steps: candles, the account, positions, and orders.
func TestExnessReads(t *testing.T) {
	f, env := exnessEnv(t, false)
	steps, err := run(t, env, "exness-candles", "Connection", "1", "Instrument", "XAUUSD", "Timeframe", "M5", "Limit", "3", "Price", "Both")
	if err != nil {
		t.Fatal(err)
	}
	candles := objectOut(t, steps[1], "Candles").GetListValue().GetValues()
	if len(candles) != 3 || candles[2].GetStructValue().GetFields()["ask"].GetStructValue().GetFields()["close"].GetNumberValue() != 2650.2 {
		t.Errorf("candles = %v", candles)
	}
	if got := textOut(t, steps[1], "Close"); got != "2650" {
		t.Errorf("Close = %q", got)
	}
	last := objectOut(t, steps[1], "Last Closed").GetStructValue().GetFields()["time"].GetStringValue()
	if want := candles[1].GetStructValue().GetFields()["time"].GetStringValue(); last != want {
		t.Errorf("Last Closed %s, want the one before the forming one, %s", last, want)
	}
	if _, err := run(t, env, "exness-candles", "Connection", "1", "Instrument", "XAUUSD", "Limit", "1001"); err == nil || !strings.Contains(err.Error(), "from 1 to 1000") {
		t.Errorf("limit: %v", err)
	}

	steps, err = run(t, env, "exness-positions", "Connection", "1", "Instrument", "XAUUSD")
	if err != nil || strings.Join(steps[1].Fired, ",") != "None,Done" {
		t.Fatalf("no positions: %v, %v", steps, err)
	}
	f.AddPosition("EURUSD", exness.Buy, "1", "1.07000")
	id := f.AddPosition("XAUUSD", exness.Sell, "0.30", "2660.000")
	steps, err = run(t, env, "exness-positions", "Connection", "1", "Instrument", "XAUUSD")
	if err != nil || strings.Join(steps[1].Fired, ",") != "Found,Done" {
		t.Fatalf("positions: %v, %v", steps, err)
	}
	if p := objectOut(t, steps[1], "Position").GetStructValue().GetFields(); p["position_id"].GetStringValue() != id || p["direction"].GetStringValue() != "sell" {
		t.Errorf("Position = %v", p)
	}
	if n := len(objectOut(t, steps[1], "Positions").GetListValue().GetValues()); n != 1 {
		t.Errorf("%d XAUUSD positions", n)
	}
	steps, _ = run(t, env, "exness-positions", "Connection", "1")
	if n := len(objectOut(t, steps[1], "Positions").GetListValue().GetValues()); n != 2 {
		t.Errorf("%d positions of every instrument", n)
	}

	steps, err = run(t, env, "exness-portfolio", "Connection", "1")
	if err != nil {
		t.Fatal(err)
	}
	// The short of 0.30 lot from 2660 at an ask of 2650.2 is 294 up; the
	// EURUSD long from 1.07 at a bid of 1.08 is 1000 up.
	a := objectOut(t, steps[1], "Account").GetStructValue().GetFields()
	if a["account_id"].GetStringValue() != "123456" || a["currency"].GetStringValue() != "USD" || a["balance"].GetStringValue() != "10000.00" ||
		a["equity"].GetStringValue() != "11294.00" || a["leverage"].GetStringValue() != "200" || a["allow_trading"].GetBoolValue() {
		t.Errorf("Account = %v", a)
	}
	if n := len(objectOut(t, steps[1], "Positions").GetListValue().GetValues()); n != 2 {
		t.Errorf("portfolio positions %d", n)
	}

	cl := exnessClient(t, f)
	ctx := context.Background()
	ack, err := cl.PlaceOrder(ctx, exness.PendingOrder{Type: exness.OrderLimit, Instrument: "EURUSD", Side: exness.Buy, Volume: "1", Price: "1.07000"}, "")
	if err != nil {
		t.Fatal(err)
	}
	cl.WaitOperation(ctx, ack.OperationID, time.Millisecond)
	steps, err = run(t, env, "exness-orders", "Connection", "1", "Show", "Open")
	if err != nil {
		t.Fatal(err)
	}
	if o := objectOut(t, steps[1], "Orders").GetListValue().GetValues(); len(o) != 1 || o[0].GetStructValue().GetFields()["order_type"].GetStringValue() != "limit" {
		t.Errorf("open orders = %v", o)
	}
	order := objectOut(t, steps[1], "Last").GetStructValue().GetFields()["order_id"].GetStringValue()
	ack, _ = cl.CancelOrder(ctx, order, "")
	cl.WaitOperation(ctx, ack.OperationID, time.Millisecond)
	steps, err = run(t, env, "exness-orders", "Connection", "1", "Show", "History", "Instrument", "EURUSD")
	if err != nil {
		t.Fatal(err)
	}
	if o := objectOut(t, steps[1], "Last").GetStructValue().GetFields(); o["order_id"].GetStringValue() != order || o["state"].GetStringValue() != "cancelled" {
		t.Errorf("history last = %v", o)
	}
	steps, err = run(t, env, "exness-orders", "Connection", "1", "Show", "Deals", "Limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	// Newest first: the XAUUSD open, the last of the two.
	if d := objectOut(t, steps[1], "Deals").GetListValue().GetValues(); len(d) != 1 || d[0].GetStructValue().GetFields()["instrument"].GetStringValue() != "XAUUSD" {
		t.Errorf("deals = %v", d)
	}
	if !strings.Contains(f.Calls("GET /v1/history/accounts/123456/deals")[0], "from=") {
		t.Errorf("deals asked without a start: %v", f.Calls("GET /v1/history"))
	}
}

// Close Position closes the instrument's positions at market, after
// cancelling its entries still waiting; or only the one Position ID names.
func TestExnessClosePosition(t *testing.T) {
	f, env := exnessEnv(t, true)
	steps, err := run(t, env, "exness-close-position", "Connection", "1", "Instrument", "XAUUSD")
	if err != nil || strings.Join(steps[1].Fired, ",") != "None,Done" {
		t.Fatalf("nothing to close: %v, %v", steps, err)
	}
	a := f.AddPosition("XAUUSD", exness.Buy, "0.10", "2640.000")
	b := f.AddPosition("XAUUSD", exness.Sell, "0.20", "2660.000")
	keep := f.AddPosition("EURUSD", exness.Buy, "1", "1.07000")
	cl := exnessClient(t, f)
	ctx := context.Background()
	ack, _ := cl.PlaceOrder(ctx, exness.PendingOrder{Type: exness.OrderLimit, Instrument: "XAUUSD", Side: exness.Buy, Volume: "0.1", Price: "2600.000"}, "")
	cl.WaitOperation(ctx, ack.OperationID, time.Millisecond)
	// a's own SL/TP order goes with it, never cancelled first.
	ack, _ = cl.PlaceOrder(ctx, exness.PendingOrder{Type: exness.OrderSLTP, PositionID: a, StopLoss: "2600.000"}, "")
	cl.WaitOperation(ctx, ack.OperationID, time.Millisecond)

	steps, err = run(t, env, "exness-close-position", "Connection", "1", "Instrument", "XAUUSD", "Position ID", b)
	if err != nil || strings.Join(steps[1].Fired, ",") != "Done" {
		t.Fatalf("close one: %v, %v", steps, err)
	}
	if snap := f.Snapshot(); len(snap.Positions) != 2 || snap.Positions[0].PositionID != a || len(snap.Orders) != 2 {
		t.Fatalf("after closing %s: %+v", b, snap)
	}
	steps, err = run(t, env, "exness-close-position", "Connection", "1", "Instrument", "XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	snap := f.Snapshot()
	if len(snap.Positions) != 1 || snap.Positions[0].PositionID != keep || len(snap.Orders) != 0 {
		t.Fatalf("after closing XAUUSD: %+v", snap)
	}
	if n := len(objectOut(t, steps[1], "Positions").GetListValue().GetValues()); n != 1 {
		t.Errorf("Positions %d", n)
	}
	if n := len(objectOut(t, steps[1], "Cancelled").GetListValue().GetValues()); n != 1 {
		t.Errorf("Cancelled %d", n)
	}
	// The long from 2640 closes at the bid, 2650: 10 × 0.10 × 100 = 100.
	if l := logsOf(steps[1]); !strings.Contains(l, "Exness 123456: cancelled the buy limit 0.1 XAUUSD at 2600.000") ||
		!strings.Contains(l, "Exness 123456: closed the buy 0.10 XAUUSD from 2640.000 (position "+a+")") {
		t.Errorf("logs = %q", l)
	}
	if _, err := run(t, env, "exness-close-position", "Connection", "1", "Instrument", "XAUUSD", "Position ID", keep); err == nil ||
		!strings.Contains(err.Error(), "no open XAUUSD position "+keep) {
		t.Errorf("another instrument's position: %v", err)
	}
}

// The clients are kept by their settings: the Access Point is asked once
// per account, and a changed setting gives a new client.
func TestExnessClientsKept(t *testing.T) {
	x := NewExnessClients(nil)
	s := &v1.ExnessSettings{ApiKey: exnesstest.APIKey, PrivateKey: exnesstest.PrivateKey, AccountId: "1", AccessPoint: "https://ap.example"}
	a, err := x.Client(s)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := x.Client(proto.Clone(s).(*v1.ExnessSettings)); b != a {
		t.Error("the same settings gave a new client")
	}
	s.AccountId = "2"
	if b, _ := x.Client(s); b == a || b.AccountID() != "2" {
		t.Error("another account kept the old client")
	}
	if _, err := x.Client(&v1.ExnessSettings{ApiKey: "k", PrivateKey: "bad", AccountId: "1", AccessPoint: "https://ap.example"}); err == nil {
		t.Error("a bad key made a client")
	}
}

// Move Stop changes a position's stop loss only ever closer to the price,
// on the right side of it, changing its SL/TP order or placing one.
func TestExnessMoveStop(t *testing.T) {
	f, env := exnessEnv(t, true)
	cl := exnessClient(t, f)
	ctx := context.Background()
	ack, err := cl.OpenPosition(ctx, exness.OpenPosition{Instrument: "XAUUSD", Side: exness.Buy, Volume: "0.10", StopLoss: "2640.000", TakeProfit: "2700.000"}, "")
	if err != nil {
		t.Fatal(err)
	}
	cl.WaitOperation(ctx, ack.OperationID, time.Millisecond)
	steps, err := run(t, env, "exness-move-stop", "Connection", "1", "Instrument", "XAUUSD", "Stop Loss", "2645")
	if err != nil {
		t.Fatal(err)
	}
	snap := f.Snapshot()
	if len(snap.Orders) != 1 || snap.Orders[0].StopLossPrice != "2645.000" || snap.Orders[0].TakeProfitPrice != "2700.000" {
		t.Fatalf("after the move: %+v", snap.Orders)
	}
	if got := f.Calls("PATCH"); len(got) != 1 {
		t.Errorf("patches = %v", got)
	}
	if l := logsOf(steps[1]); !strings.Contains(l, "Exness 123456: stop loss XAUUSD at 2645.000, was 2640.000") {
		t.Errorf("logs = %q", l)
	}
	for _, c := range []struct {
		values []string
		want   string
	}{
		{[]string{"Stop Loss", "2642"}, "Stop Loss 2642.000 would loosen the stop at 2645.000: a stop only moves closer to the price"},
		{[]string{"Stop Loss", "2651"}, "Stop Loss 2651.000 is on the wrong side of the price 2650.000"},
		{[]string{"Take Profit", "2649"}, "Take Profit 2649.000 is on the wrong side of the price 2650.000"},
		{[]string{}, "give a Stop Loss, a Take Profit, or both"},
	} {
		values := append([]string{"Connection", "1", "Instrument", "XAUUSD"}, c.values...)
		if _, err := run(t, env, "exness-move-stop", values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v\nwant %q", c.values, err, c.want)
		}
	}
	// A short without a stop gets an SL/TP order; with two positions open
	// Position ID picks one.
	short := f.AddPosition("XAUUSD", exness.Sell, "0.20", "2660.000")
	if _, err := run(t, env, "exness-move-stop", "Connection", "1", "Instrument", "XAUUSD", "Stop Loss", "2670"); err == nil ||
		!strings.Contains(err.Error(), "2 open XAUUSD positions: give Position ID") {
		t.Errorf("two positions: %v", err)
	}
	c, err := exnessStep(t, env, "exness-move-stop", map[string]any{"instrument": "XAUUSD", "position_id": short, "stop_loss": 2670}, "Instrument", "EURUSD")
	if err != nil {
		t.Fatal(err)
	}
	if !slicesContain(c.fired, "Done") {
		t.Errorf("fired %v", c.fired)
	}
	var sltp *exness.Order
	for _, o := range f.Snapshot().Orders {
		if o.PositionID == short {
			sltp = &o
		}
	}
	if sltp == nil || sltp.StopLossPrice != "2670.000" || sltp.TakeProfitPrice != "" {
		t.Errorf("the short's SL/TP = %+v", sltp)
	}
}
