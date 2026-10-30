package engine

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_tower/internal/binance"
)

// Trades are rebuilt from flat to flat: a scaled-out close, a flip from
// long to short in one fill, and a trade under way before the fills, which
// is left out.
func TestRoundTrips(t *testing.T) {
	at := func(m int) time.Time { return time.Date(2026, 10, 30, 13, m, 0, 0, time.UTC) }
	fill := func(m int, side string, qty, price, pnl, fee float64) binance.Fill {
		return binance.Fill{Side: side, Qty: qty, Price: price, RealizedPnL: pnl, Commission: fee, Time: at(m)}
	}
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-9 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	trips := roundTrips("BTCUSDT", []binance.Fill{
		fill(0, "BUY", 0.01, 100, 0, 0.04),
		fill(5, "SELL", 0.004, 110, 0.04, 0.02),
		fill(9, "SELL", 0.006, 120, 0.12, 0.03),
		// Long 0.01 at 100, then a sell of 0.02 closes it and opens a short.
		fill(10, "BUY", 0.01, 100, 0, 0.01),
		fill(12, "SELL", 0.02, 90, -0.1, 0.02),
		fill(15, "BUY", 0.01, 80, 0.1, 0.01),
	}, 0)
	if len(trips) != 3 {
		t.Fatalf("%d trips: %+v", len(trips), trips)
	}
	a := trips[0]
	if a.ID != "BTCUSDT-20261030-130900" || a.Side != "long" || a.Fills != 3 || !a.Opened.Equal(at(0)) {
		t.Errorf("first: %+v", a)
	}
	near("qty", a.Qty, 0.01)
	near("entry", a.Entry, 100)
	near("exit (0.004 at 110, 0.006 at 120)", a.Exit, 116)
	near("pnl", a.PnL, 0.16)
	near("fees", a.Fees, 0.09)
	near("net", a.Net, 0.07)
	b, s := trips[1], trips[2]
	if b.Side != "long" || s.Side != "short" || !b.Closed.Equal(at(12)) || !s.Opened.Equal(at(12)) {
		t.Errorf("the flip: %+v, %+v", b, s)
	}
	near("the flip's pnl goes to the trade it closes", b.PnL, -0.1)
	near("the flip's fee is shared", b.Fees, 0.02)
	near("the short's fees", s.Fees, 0.02)
	near("the short's exit", s.Exit, 80)

	// Long 0.01 before the fills: its close is not a trade, the next is.
	trips = roundTrips("ETHUSDT", []binance.Fill{fill(0, "SELL", 0.01, 100, 1, 0), fill(1, "SELL", 0.01, 100, 0, 0), fill(2, "BUY", 0.01, 90, 0.1, 0)}, 0.01)
	if len(trips) != 1 || trips[0].Side != "short" || trips[0].Entry != 100 {
		t.Errorf("under way at the start: %+v", trips)
	}
}

// One Per Symbol refuses an order while the symbol has a position or an
// entry waiting, and otherwise clears stops left from a closed position.
// Max Leverage sets the leverage when the order gives none, and a limit
// entry's expiry goes into its client order id.
func TestBinanceOrderOnePerSymbol(t *testing.T) {
	f, env := binanceEnv(t)
	order := []string{"Connection", "1", "Symbol", "BTCUSDT", "Type", "Limit", "Time In Force", "Post Only", "Price", "79000",
		"Quantity", "0.01", "Stop Loss", "78000", "One Per Symbol", "Yes", "Max Leverage", "5", "Limit Expiry", "15"}
	f.positions = `[{"symbol":"BTCUSDT","positionAmt":"0.01","entryPrice":"80000"}]`
	if _, err := run(t, env, "binance-order", order...); err == nil || !strings.Contains(err.Error(), "already has a LONG position") {
		t.Errorf("with a position: %v", err)
	}
	f.positions = `[]`
	f.open = `[{"orderId":5,"symbol":"BTCUSDT","status":"NEW","type":"LIMIT","side":"BUY","price":"79500","origQty":"0.01"}]`
	if _, err := run(t, env, "binance-order", order...); err == nil || !strings.Contains(err.Error(), "a limit BUY entry of 0.01 BTCUSDT at 79500.0 is waiting") {
		t.Errorf("with an entry: %v", err)
	}
	f.open = `[]`
	if _, err := run(t, env, "binance-order", append(order, "Leverage", "10")...); err == nil || !strings.Contains(err.Error(), "Max Leverage: 10x is over 5x") {
		t.Errorf("leverage over the most: %v", err)
	}
	f.calls = nil
	steps, err := run(t, env, "binance-order", order...)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.did("DELETE /fapi/v1/algoOpenOrders"); len(got) != 1 {
		t.Errorf("the stop left from a closed position: %v", got)
	}
	if got := f.did("POST /fapi/v1/leverage"); len(got) != 1 || !strings.Contains(got[0], "leverage=5") {
		t.Errorf("leverage: %v", got)
	}
	placed := f.did("POST /fapi/v1/order")
	if len(placed) != 1 || !strings.Contains(placed[0], "newClientOrderId=twx-") {
		t.Fatalf("order: %v", placed)
	}
	logs := logsOf(steps[1])
	if !strings.Contains(logs, "cancelled 1 stop orders left from a closed BTCUSDT position") || !strings.Contains(logs, "expires in 15m") {
		t.Errorf("logs:\n%s", logs)
	}
	id := placed[0][strings.Index(placed[0], "twx-"):]
	id = id[:strings.IndexAny(id+"&", "&")]
	if exp := expiryOf(binance.Order{ClientID: id}); exp.Sub(time.Now()) < 14*time.Minute || exp.Sub(time.Now()) > 16*time.Minute {
		t.Errorf("expiry of %s: %v", id, exp)
	}

	// An AI's order object gives the expiry too; a market order has none.
	f.calls = nil
	if _, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "BTCUSDT", "Quantity", "0.01", "Stop Loss", "78000", "Limit Expiry", "15"); err != nil {
		t.Fatal(err)
	}
	if got := f.did("POST /fapi/v1/order"); len(got) != 1 || strings.Contains(got[0], "newClientOrderId") {
		t.Errorf("a market order: %v", got)
	}
}

// Binance Tidy cancels an entry past its expiry, one whose take profit was
// reached first, and one that lost its stop; keeps what a partial entry
// filled; and cancels stops left with nothing to protect.
func TestBinanceTidy(t *testing.T) {
	f, env := binanceEnv(t)
	past := fmt.Sprint(time.Now().Add(-time.Minute).Unix())
	later := fmt.Sprint(time.Now().Add(time.Hour).Unix())
	entry := func(id, side, price, filled string) string {
		return fmt.Sprintf(`{"orderId":7,"clientOrderId":"%s","symbol":"BTCUSDT","status":"NEW","type":"LIMIT","side":"%s","price":"%s","origQty":"0.01","executedQty":"%s","time":%d}`,
			id, side, price, filled, time.Now().Add(-20*time.Minute).UnixMilli())
	}
	stops := func(side, sl, tp string) string {
		return fmt.Sprintf(`[{"algoId":21,"symbol":"BTCUSDT","orderType":"STOP_MARKET","side":"%s","triggerPrice":"%s"},{"algoId":22,"symbol":"BTCUSDT","orderType":"TAKE_PROFIT_MARKET","side":"%s","triggerPrice":"%s"}]`, side, sl, side, tp)
	}
	for _, c := range []struct {
		name, open, algos, want string
		stopsCancelled          int
	}{
		{"expired", "[" + entry("twx-"+past+"-ab", "BUY", "79000", "0") + "]", stops("SELL", "78000", "82000"), "cancelled the limit BUY entry of 0.01 at 79000.0: not filled within 19m", 1},
		{"its target reached first (mark 80000.5)", "[" + entry("twx-"+later+"-ab", "BUY", "79000", "0") + "]", stops("SELL", "78000", "80000"), "the price reached its take profit 80000.0 before it filled", 1},
		{"its stop gone", "[" + entry("twx-"+later+"-ab", "SELL", "81000", "0") + "]", `[]`, "its stop loss is gone", 0},
		{"partly filled", "[" + entry("twx-"+later+"-ab", "BUY", "79000", "0.004") + "]", stops("SELL", "78000", "82000"), "filled 0.004; the rest was cancelled, the position keeps its stops", 0},
		{"waiting, as it should", "[" + entry("twx-"+later+"-ab", "BUY", "79000", "0") + "]", stops("SELL", "78000", "82000"), "nothing to tidy", 0},
		{"stops left over", `[]`, stops("SELL", "78000", "82000"), "cancelled 2 stop orders with no position or entry left", 1},
		{"an entry without an expiry, past Expiry", "[" + entry("web_123", "BUY", "79000", "0") + "]", stops("SELL", "78000", "82000"), "not filled within 15m", 1},
	} {
		f.open, f.algos, f.calls = c.open, c.algos, nil
		steps, err := run(t, env, "binance-tidy", "Connection", "1", "Symbols", "btc/usdt", "Expiry", "15")
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := textOut(t, steps[1], "Text"); !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
		if got := f.did("DELETE /fapi/v1/algoOpenOrders"); len(got) != c.stopsCancelled {
			t.Errorf("%s: stops cancelled %d times", c.name, len(got))
		}
	}
	if _, err := run(t, env, "binance-tidy", "Connection", "1"); err == nil {
		t.Error("no Symbols")
	}
}

// Binance Snapshot puts the account, the position with its stops, the book,
// fees, indicators, the last candles and the positioning together.
func TestBinanceSnapshot(t *testing.T) {
	f, env := binanceEnv(t)
	var rows []string
	start := time.Now().Add(-300 * time.Hour).Truncate(time.Hour)
	for i := range 300 {
		open := start.Add(time.Duration(i) * time.Hour)
		c := 100 + float64(i)/10
		rows = append(rows, fmt.Sprintf(`[%d,"%g","%g","%g","%g","10",%d,"1000",5,"5","600"]`, open.UnixMilli(), c-0.05, c+0.5, c-0.5, c, open.Add(time.Hour).UnixMilli()-1))
	}
	f.klines = "[" + strings.Join(rows, ",") + "]"
	f.positions = `[{"symbol":"BTCUSDT","positionAmt":"0.01","entryPrice":"80000","markPrice":"80100","unRealizedProfit":"1","liquidationPrice":"64000","notional":"801"}]`
	f.open = `[]`
	steps, err := run(t, env, "binance-snapshot", "Connection", "1", "Symbols", "BTCUSDT", "Intervals", "1h,4h", "Candles", "2")
	if err != nil {
		t.Fatal(err)
	}
	text := textOut(t, steps[1], "Text")
	for _, want := range []string{
		"Orders go to the testnet network; the candles are the live market's. Trading interval 1h, context 4h.",
		"losing 10.00 USDT together at their stops", // 0.01 × (80000 − 79000)
		"Position: LONG 0.01 @ 80000.0", "stop loss 79000.0 (-1.25% from the entry), no take profit",
		"Book (testnet): bid 80000.0, ask 80000.1; mark 80000.5; funding +0.0100%", "maker 0.0200%, taker 0.0400%",
		"BTCUSDT (live)", "trend up", "Last 2 closed 1h candles", "| 1000 | 60% |",
		"Open interest 1240 contracts: +3.33% over 4h, +24.00% over 24h", "1.50 last hour, 1.15 over 24h",
		"all accounts: 2.40 (24h ago 1.20)", "Funding, last settlements: +0.0100%, -0.0050%",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Unavailable") {
		t.Errorf("a part was unavailable:\n%s", text)
	}
	// Without a stop on the position, the snapshot says so first.
	f.algos = `[]`
	steps, err = run(t, env, "binance-snapshot", "Connection", "1", "Symbols", "BTCUSDT", "Positioning", "No")
	if err != nil {
		t.Fatal(err)
	}
	if text := textOut(t, steps[1], "Text"); !strings.Contains(text, "NO STOP LOSS on the position of BTCUSDT") || strings.Contains(text, "Positioning (") {
		t.Errorf("no stop:\n%s", text)
	}

	// With Tidy, an entry past its expiry is cancelled first; Notes close it.
	f.positions, f.algos = `[]`, `[{"algoId":21,"symbol":"BTCUSDT","orderType":"STOP_MARKET","side":"SELL","triggerPrice":"78000"}]`
	f.open = fmt.Sprintf(`[{"orderId":7,"clientOrderId":"twx-%d-ab","symbol":"BTCUSDT","status":"NEW","type":"LIMIT","side":"BUY","price":"79000","origQty":"0.01","executedQty":"0","time":%d}]`,
		time.Now().Add(-time.Minute).Unix(), time.Now().Add(-16*time.Minute).UnixMilli())
	steps, err = run(t, env, "binance-snapshot", "Connection", "1", "Symbols", "BTCUSDT", "Positioning", "No", "Tidy", "Yes", "Notes", "Closed trades: none.")
	if err != nil {
		t.Fatal(err)
	}
	text = textOut(t, steps[1], "Text")
	for _, want := range []string{"Tidied now: cancelled the limit BUY entry of 0.01 at 79000.0: not filled within 15m.", "\n## Notes\n\nClosed trades: none.\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}

// Binance Closed Trades lists the trades closed, leaving out those whose
// id is in Leave Out; the Binance Tool reads them too.
func TestBinanceClosedTrades(t *testing.T) {
	f, env := binanceEnv(t)
	t0 := time.Now().Add(-time.Hour).Truncate(time.Second)
	f.fills = fmt.Sprintf(`[{"id":1,"symbol":"BTCUSDT","side":"BUY","price":"80000","qty":"0.01","realizedPnl":"0","commission":"0.32","commissionAsset":"USDT","time":%d},
		{"id":2,"symbol":"BTCUSDT","side":"SELL","price":"80800","qty":"0.01","realizedPnl":"8","commission":"0.32","commissionAsset":"USDT","time":%d}]`,
		t0.UnixMilli(), t0.Add(30*time.Minute).UnixMilli())
	id := "BTCUSDT-" + t0.Add(30*time.Minute).UTC().Format("20060102-150405")
	steps, err := run(t, env, "binance-closed-trades", "Connection", "1", "Symbols", "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	text := textOut(t, steps[1], "Text")
	if want := id + ": BTCUSDT long 0.01, entry 80000.0, exit 80800.0"; !strings.Contains(text, want) || !strings.Contains(text, "PnL +8.00, fees 0.64, net +7.36 USDT") || strings.Join(steps[1].Fired, ",") != "Found,Done" {
		t.Errorf("want %q in %q (%v)", want, text, steps[1].Fired)
	}
	steps, err = run(t, env, "binance-closed-trades", "Connection", "1", "Symbols", "BTCUSDT", "Leave Out", "BTCUSDT-20261001-000000\n"+id+"\n")
	if err != nil || strings.Join(steps[1].Fired, ",") != "None,Done" || textOut(t, steps[1], "Count") != "0" {
		t.Errorf("left out: %v, %v", err, steps)
	}
	c, tool, err := toolFrom(t, env, "binance-tool", "Connection", "1")
	if err != nil {
		t.Fatal(err)
	}
	if got := tool.run(c, "closed", []string{"btcusdt", "48"}); !strings.Contains(got, id) {
		t.Errorf("the tool's closed: %q", got)
	}
}

// Binance Close Position cancels an entry waiting, and with no position
// that is all it does.
func TestBinanceClosePositionEntry(t *testing.T) {
	f, env := binanceEnv(t)
	f.open = `[{"orderId":5,"symbol":"BTCUSDT","status":"NEW","type":"LIMIT","side":"BUY","price":"79500","origQty":"0.01"}]`
	steps, err := run(t, env, "binance-close-position", "Connection", "1", "Symbol", "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(steps[1].Fired, ",") != "Done" || !strings.Contains(logsOf(steps[1]), "cancelled the limit BUY entry of 0.01 BTCUSDT at 79500.0") {
		t.Errorf("%v: %s", steps[1].Fired, logsOf(steps[1]))
	}
	if len(f.did("DELETE /fapi/v1/order")) != 1 || len(f.did("DELETE /fapi/v1/algoOpenOrders")) != 1 || len(f.did("POST /fapi/v1/order")) != 0 {
		t.Errorf("calls: %v", f.calls)
	}
	f.open = `[]`
	steps, err = run(t, env, "binance-close-position", "Connection", "1", "Symbol", "BTCUSDT")
	if err != nil || strings.Join(steps[1].Fired, ",") != "None,Done" {
		t.Errorf("nothing to close: %v, %v", err, steps)
	}
}

// A candle still forming by the exchange's clock is not a closed one, even
// when this computer's clock, minutes fast, says it has closed.
func TestClosedCandlesByExchangeTime(t *testing.T) {
	f, env := binanceEnv(t)
	f.lag = 4 * time.Minute
	var rows []string
	end := time.Now().Add(-2 * time.Minute) // closed by this clock, forming by the exchange's
	for i := range 300 {
		open := end.Add(-time.Duration(300-i) * time.Hour)
		rows = append(rows, fmt.Sprintf(`[%d,"1","2","0.5","%d","10",%d,"1000",5]`, open.UnixMilli(), 100+i, open.Add(time.Hour).UnixMilli()))
	}
	f.klines = "[" + strings.Join(rows, ",") + "]"
	steps, err := run(t, env, "binance-indicators", "Connection", "1", "Symbol", "BTCUSDT", "Intervals", "1h")
	if err != nil {
		t.Fatal(err)
	}
	if text := textOut(t, steps[1], "Text"); !strings.Contains(text, "close 398.0") {
		t.Errorf("the forming candle (close 399) was read as closed:\n%s", text)
	}
	steps, err = run(t, env, "binance-candles", "Symbol", "BTCUSDT")
	if err != nil || textOut(t, steps[1], "Close") != "398" {
		t.Errorf("Binance Candles' last closed: %v %v", err, steps)
	}
}
