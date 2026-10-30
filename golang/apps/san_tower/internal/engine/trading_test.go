package engine

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
	"github.com/wargasipil/san_tower/internal/store"
)

// The indicators against values worked out by hand.
func TestIndicatorMath(t *testing.T) {
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	xs := []float64{1, 2, 3, 4, 5}
	near("ema of 3 (seed 2, then 3, then 4)", ema(xs, 3), 4)
	near("rsi always up", rsi(xs, 3), 100)
	near("rsi up and down, Wilder-smoothed", rsi([]float64{1, 2, 1, 2, 1}, 2), 37.5)
	up, low := bollinger([]float64{2, 4, 4, 4, 5, 5, 7, 9}, 8, 2)
	near("bollinger upper (mean 5, sd 2)", up, 9)
	near("bollinger lower", low, 1)
	ks := make([]binance.Candle, 5)
	for i := range ks {
		ks[i] = binance.Candle{High: 12, Low: 10, Close: 11}
	}
	near("atr of a steady 2 range", atr(ks, 3), 2)

	// Swings: a high above the 3 candles each side, nearest first.
	highs := []float64{1, 2, 3, 9, 3, 2, 1, 2, 3, 7, 3, 2, 1}
	cs := make([]binance.Candle, len(highs))
	for i, h := range highs {
		cs[i] = binance.Candle{High: h, Low: h - 0.5}
	}
	above, below := swings(cs, 5, 3, 3)
	if fmt.Sprint(above) != "[7 9]" || fmt.Sprint(below) != "[0.5]" {
		t.Errorf("swings: above %v, below %v", above, below)
	}
}

// Binance Indicators reads each interval's closed candles and the 24h.
func TestBinanceIndicators(t *testing.T) {
	f, env := binanceEnv(t)
	var rows []string
	start := time.Now().Add(-300 * time.Hour).Truncate(time.Hour)
	for i := range 300 {
		open := start.Add(time.Duration(i) * time.Hour)
		c := 100 + float64(i)/10 // a steady rise
		rows = append(rows, fmt.Sprintf(`[%d,"%g","%g","%g","%g","10",%d,"1000",5]`, open.UnixMilli(), c-0.05, c+0.5, c-0.5, c, open.Add(time.Hour).UnixMilli()-1))
	}
	f.klines = "[" + strings.Join(rows, ",") + "]"
	steps, err := run(t, env, "binance-indicators", "Connection", "1", "Symbol", "btcusdt", "Intervals", "1h")
	if err != nil {
		t.Fatal(err)
	}
	text := textOut(t, steps[1], "Text")
	for _, want := range []string{"BTCUSDT (testnet)", "1h, closed", "trend up (close > EMA20 > EMA50 > EMA200)", "RSI14 100.0", "ATR14"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
	if got := f.did("GET /fapi/v1/klines"); len(got) != 2 || !strings.Contains(got[0], "interval=1h") || !strings.Contains(got[0], "limit=300") || !strings.Contains(got[1], "limit=24") {
		t.Errorf("klines: %v", got)
	}
	if _, err := run(t, env, "binance-indicators", "Connection", "1", "Symbol", "BTCUSDT", "Intervals", "7m"); err == nil {
		t.Error("an interval Binance has not")
	}
}

// Binance Order takes an AI's order as one object, and refuses one over
// its limits, saying the most that fits; Testnet Only refuses a live
// connection.
func TestBinanceOrderLimits(t *testing.T) {
	f, env := binanceEnv(t)
	order := func(fields map[string]any, settings ...string) error {
		t.Helper()
		st := step("o", "binance-order", append([]string{"Connection", "1"}, settings...)...)
		v, _ := structpb.NewValue(fields)
		c := &call{ctx: context.Background(), env: env, step: st, inputs: map[string]proto.Message{"Order": v}, outputs: map[string]proto.Message{}}
		for _, s := range byKind["binance-order"].Inputs {
			if s.Field {
				c.inputs[s.Name] = wrapperspb.String("")
			}
		}
		return binanceOrder(c)
	}
	limits := []string{"Require Stop", "Yes", "Max Notional", "1000", "Max Loss", "100", "Testnet Only", "Yes"}
	// At 80000.1, 0.012 BTC is 960 USDT; a stop 1000 away loses 12.
	if err := order(map[string]any{"symbol": "BTCUSDT", "side": "buy", "quantity": 0.012, "stop_loss": 79000, "take_profit": 82000, "leverage": 5}, limits...); err != nil {
		t.Fatal(err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 1 || !strings.Contains(got[0], "side=BUY") || !strings.Contains(got[0], "quantity=0.012") {
		t.Errorf("order: %v", got)
	}
	for _, tc := range []struct {
		fields map[string]any
		want   string
	}{
		{map[string]any{"symbol": "BTCUSDT", "side": "buy", "quantity": 0.012}, "Require Stop: the order needs a Stop Loss"},
		{map[string]any{"symbol": "BTCUSDT", "side": "buy", "quantity": 0.02, "stop_loss": 79900}, "Max Notional: 0.020 × 80000.1 = 1600.00 USDT, over 1000.00; at most 0.012"},
		{map[string]any{"symbol": "BTCUSDT", "side": "sell", "quantity": 0.012, "stop_loss": 90000}, "Max Loss: 0.012 lost 120.00 USDT at the stop 90000.0 (10000.0 from 80000.0), over 100.00; at most 0.010"},
		{map[string]any{"symbol": "BTCUSDT", "side": "up", "quantity": 0.01}, `Order: side "up": buy or sell`},
	} {
		if err := order(tc.fields, limits...); err == nil || err.Error() != tc.want {
			t.Errorf("%v: %v\nwant %s", tc.fields, err, tc.want)
		}
	}
	// A live connection is refused with Testnet Only.
	base := env.Connection
	env.Connection = func(id uint32) (store.Connection, error) {
		c, err := base(id)
		c.Settings = proto.Clone(c.Settings).(*v1.ConnectionSettings)
		c.Settings.GetBinance().Network = "live"
		return c, err
	}
	if err := order(map[string]any{"symbol": "BTCUSDT", "side": "buy", "quantity": 0.001, "stop_loss": 79000}, limits...); err == nil || err.Error() != "Testnet Only: the connection is on the live network" {
		t.Errorf("live: %v", err)
	}
}

// Binance Move Stop places the new stop, then cancels the old one, and only
// ever moves a stop closer to the price.
func TestBinanceMoveStop(t *testing.T) {
	f, env := binanceEnv(t)
	// A long of 0.015 at 80000, marked at 81000, its stop at 79000 (the
	// fake's open algo order).
	f.positions = `[{"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"0.015","entryPrice":"80000","markPrice":"81000","unRealizedProfit":"15"}]`
	steps, err := run(t, env, "binance-move-stop", "Connection", "1", "Symbol", "BTCUSDT", "Stop Loss", "80100")
	if err != nil {
		t.Fatal(err)
	}
	placed, cancelled := f.did("POST /fapi/v1/algoOrder"), f.did("DELETE /fapi/v1/algoOrder")
	if len(placed) != 1 || !strings.Contains(placed[0], "side=SELL") || !strings.Contains(placed[0], "triggerPrice=80100.0") || !strings.Contains(placed[0], "quantity=0.015") || len(cancelled) != 1 || !strings.Contains(cancelled[0], "algoId=21") {
		t.Errorf("placed %v, cancelled %v", placed, cancelled)
	}
	if l := logsOf(steps[1]); !strings.Contains(l, "stop loss BTCUSDT at 80100.0, was 79000.0") {
		t.Errorf("logs: %s", l)
	}
	for stop, want := range map[string]string{
		"78000": "would loosen the stop at 79000.0",
		"82000": "on the wrong side of the mark price 81000.0",
	} {
		if _, err := run(t, env, "binance-move-stop", "Connection", "1", "Symbol", "BTCUSDT", "Stop Loss", stop); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("stop %s: %v", stop, err)
		}
	}
	f.positions = `[]`
	if _, err := run(t, env, "binance-move-stop", "Connection", "1", "Symbol", "BTCUSDT", "Stop Loss", "80100"); err == nil || !strings.Contains(err.Error(), "no open position in BTCUSDT") {
		t.Errorf("no position: %v", err)
	}
}
