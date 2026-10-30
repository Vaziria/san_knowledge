package engine

import (
	"os"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/binance"
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

// TestTradeLoopLive checks the trading loop's steps on the real test
// network, in XRPUSDT, with the same switch and key as TestBinanceLive: the
// snapshot (positioning from the real market), the closed trades, and a
// post-only entry far under the price that One Per Symbol then guards and
// Binance Tidy cancels once it expires.
func TestTradeLoopLive(t *testing.T) {
	if os.Getenv("SAN_TOWER_BINANCE_LIVE") != "1" {
		t.Skip("set SAN_TOWER_BINANCE_LIVE=1 and the testnet key to trade on the test network")
	}
	env := Env{Connection: func(id uint32) (store.Connection, error) {
		return store.Connection{ID: 1, Name: "testnet", Type: store.Binance, Settings: &v1.ConnectionSettings{
			Type: &v1.ConnectionSettings_Binance{Binance: &v1.BinanceSettings{
				ApiKey: os.Getenv("BINANCE_TESTNET_API_KEY"), ApiSecret: os.Getenv("BINANCE_TESTNET_API_SECRET"), Network: "testnet"}},
		}}, nil
	}}
	steps, err := run(t, env, "binance-snapshot", "Connection", "1", "Symbols", "XRPUSDT,BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	text := textOut(t, steps[1], "Text")
	t.Logf("snapshot:\n%s", text)
	if strings.Contains(text, "Unavailable") || !strings.Contains(text, "Open interest") {
		t.Error("a part of the snapshot was unavailable")
	}
	if steps, err = run(t, env, "binance-closed-trades", "Connection", "1", "Symbols", "XRPUSDT", "Hours", "168"); err != nil {
		t.Fatal(err)
	}
	t.Logf("closed: %s", textOut(t, steps[1], "Text"))

	cl := binance.New(binance.TestnetURL, "", "")
	book, err := cl.BookTicker(t.Context(), "XRPUSDT")
	if err != nil {
		t.Fatal(err)
	}
	info, err := cl.SymbolInfo(t.Context(), "XRPUSDT")
	if err != nil {
		t.Fatal(err)
	}
	limit := info.RoundPrice(book.Bid * 0.9)
	qty := info.FormatQty(info.FloorQty(25/limit) + info.StepSize)
	entry := []string{"Connection", "1", "Symbol", "XRPUSDT", "Type", "Limit", "Time In Force", "Post Only", "Price", info.FormatPrice(limit),
		"Quantity", qty, "Stop Loss", info.FormatPrice(limit * 0.95), "Take Profit", info.FormatPrice(book.Ask * 1.05),
		"One Per Symbol", "Yes", "Max Leverage", "5", "Limit Expiry", "0.05", "Testnet Only", "Yes"}
	if steps, err = run(t, env, "binance-order", entry...); err != nil {
		t.Fatal(err)
	}
	t.Logf("entry: %s", logsOf(steps[1]))
	if _, err := run(t, env, "binance-order", entry...); err == nil || !strings.Contains(err.Error(), "is waiting") {
		t.Errorf("a second entry: %v", err)
	}
	time.Sleep(4 * time.Second)
	if steps, err = run(t, env, "binance-tidy", "Connection", "1", "Symbols", "XRPUSDT"); err != nil {
		t.Fatal(err)
	}
	tidy := textOut(t, steps[1], "Text")
	t.Logf("tidy: %s", tidy)
	if !strings.Contains(tidy, "not filled within") {
		t.Errorf("the entry was not cancelled: %s", tidy)
	}
	if steps, err = run(t, env, "binance-tidy", "Connection", "1", "Symbols", "XRPUSDT"); err != nil || textOut(t, steps[1], "Text") != "nothing to tidy" {
		t.Errorf("left after tidying: %v %s", err, textOut(t, steps[1], "Text"))
	}
}
