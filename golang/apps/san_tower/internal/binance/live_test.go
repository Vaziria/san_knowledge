package binance

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestLiveReadOnly reads a real test network account, placing nothing:
// SAN_TOWER_BINANCE_LIVE=1 with BINANCE_TESTNET_API_KEY and
// BINANCE_TESTNET_API_SECRET (autotrade's .env).
func TestLiveReadOnly(t *testing.T) {
	if os.Getenv("SAN_TOWER_BINANCE_LIVE") != "1" {
		t.Skip("set SAN_TOWER_BINANCE_LIVE=1 and the testnet key to read a real test network account")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New(TestnetURL, os.Getenv("BINANCE_TESTNET_API_KEY"), os.Getenv("BINANCE_TESTNET_API_SECRET"))
	acc, err := c.Account(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("account: wallet %.2f, available %.2f, unrealized %.2f", acc.WalletBalance, acc.AvailableBalance, acc.UnrealizedPnL)
	hedge, err := c.DualSide(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("hedge mode: %v", hedge)
	bs, err := c.Balances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d assets held", len(bs))
	ps, err := c.Positions(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		t.Logf("position %s %s %v at %v", p.Symbol, p.Side, p.Amount, p.EntryPrice)
	}
	open, err := c.OpenOrders(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	algos, err := c.OpenAlgoOrders(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d open orders, %d stop orders", len(open), len(algos))
	hist, err := c.AllOrders(ctx, "BTCUSDT", 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range hist {
		t.Logf("order %d %s %s %s %v at %v, %s", o.OrderID, o.Side, o.Type, o.Status, o.OrigQty, o.Time.Local().Format("Jan 2 15:04"), o.Symbol)
	}
	info, err := c.SymbolInfo(ctx, "BTCUSDT")
	if err != nil {
		t.Fatal(err)
	}
	ks, err := c.Klines(ctx, "BTCUSDT", "1h", 3)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("BTCUSDT tick %v step %v; 1h closes %v %v %v", info.TickSize, info.StepSize, ks[0].Close, ks[1].Close, ks[2].Close)
}
