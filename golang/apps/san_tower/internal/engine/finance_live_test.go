package engine

import (
	"os"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// TestBinanceLive trades on the real test network, in XRPUSDT (which
// autotrade does not trade), and closes what it opens:
// SAN_TOWER_BINANCE_LIVE=1 with BINANCE_TESTNET_API_KEY and
// BINANCE_TESTNET_API_SECRET.
func TestBinanceLive(t *testing.T) {
	if os.Getenv("SAN_TOWER_BINANCE_LIVE") != "1" {
		t.Skip("set SAN_TOWER_BINANCE_LIVE=1 and the testnet key to trade on the test network")
	}
	env := Env{Connection: func(id uint32) (store.Connection, error) {
		return store.Connection{ID: 1, Name: "testnet", Type: store.Binance, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_Binance{Binance: &v1.BinanceSettings{
				ApiKey: os.Getenv("BINANCE_TESTNET_API_KEY"), ApiSecret: os.Getenv("BINANCE_TESTNET_API_SECRET"), Network: "testnet"}},
		}}, nil
	}}
	closePosition := func() {
		steps, err := run(t, env, "binance-close-position", "Connection", "1", "Symbol", "XRPUSDT")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("close: %s", logsOf(steps[1]))
		if steps[1].Fired[0] != "Done" || !strings.Contains(logsOf(steps[1]), "closed the") {
			t.Errorf("close fired %v", steps[1].Fired)
		}
	}

	// A market buy: its price is read back.
	steps, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "XRPUSDT", "Quantity", "10",
		"Stop Loss", "1.0", "Take Profit", "1.6")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("market: %s", logsOf(steps[1]))
	if l := logsOf(steps[1]); strings.Contains(l, "at 0.0000") {
		t.Errorf("the market order's price was not read back: %s", l)
	}
	closePosition()

	// A limit sell below the bid fills at once (GTC), with its stops.
	steps, err = run(t, env, "binance-order", "Connection", "1", "Symbol", "XRPUSDT", "Type", "Limit", "Side", "Sell",
		"Quantity", "10", "Price", "1.27", "Stop Loss", "1.4", "Take Profit", "1.2")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("limit: %s", logsOf(steps[1]))
	closePosition()

	// Refused: a post-only buy above the ask, and a stop above a buy.
	if _, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "XRPUSDT", "Type", "Limit", "Time In Force", "Post Only",
		"Quantity", "10", "Price", "1.32"); err == nil || !strings.Contains(err.Error(), "post-only") {
		t.Errorf("post-only that would take: %v", err)
	} else {
		t.Logf("post-only refused: %v", err)
	}
	if _, err := run(t, env, "binance-order", "Connection", "1", "Symbol", "XRPUSDT", "Quantity", "10", "Stop Loss", "1.6"); err == nil || !strings.Contains(err.Error(), "wrong side") {
		t.Errorf("wrong-side stop: %v", err)
	}
	steps, err = run(t, env, "binance-orders", "Connection", "1", "Show", "Stop Orders", "Symbol", "XRPUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(objectOut(t, steps[1], "Orders").GetListValue().GetValues()); n != 0 {
		t.Errorf("%d stop orders left", n)
	}
	steps, err = run(t, env, "binance-orders", "Connection", "1", "Show", "Open", "Symbol", "XRPUSDT")
	if err != nil {
		t.Fatal(err)
	}
	if n := len(objectOut(t, steps[1], "Orders").GetListValue().GetValues()); n != 0 {
		t.Errorf("%d open orders left", n)
	}
}
