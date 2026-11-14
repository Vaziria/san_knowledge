package exnesstest

import (
	"context"
	"errors"
	"testing"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// The fake keeps an account as the real API would show it, through the
// real client.
func TestFakeAccount(t *testing.T) {
	ctx := context.Background()
	s := New(t)
	c, err := exness.New(s.Config())
	if err != nil {
		t.Fatal(err)
	}
	ack, err := c.OpenPosition(ctx, exness.OpenPosition{Instrument: "XAUUSD", Side: exness.Buy, Volume: "0.10", StopLoss: "2640.000"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.WaitOperation(ctx, ack.OperationID, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	snap, err := c.Snapshot(ctx)
	if err != nil || len(snap.Positions) != 1 || snap.Positions[0].OpenPrice != "2650.200" || len(snap.Orders) != 1 ||
		snap.Orders[0].OrderType != "sltp" || snap.Orders[0].StopLossPrice != "2640.000" {
		t.Fatalf("snapshot = %+v, %v", snap, err)
	}
	pos := snap.Positions[0].PositionID

	// A step off the volume step is rejected after the ACK.
	ack, _ = c.OpenPosition(ctx, exness.OpenPosition{Instrument: "XAUUSD", Side: exness.Buy, Volume: "0.015"}, "")
	var oe *exness.OperationError
	if _, err := c.WaitOperation(ctx, ack.OperationID, time.Millisecond); !errors.As(err, &oe) || oe.Code != exness.CodeTradingRuleInvalidVolumeStep {
		t.Fatalf("off step: %v", err)
	}
	s.RejectNext(exness.CodeMarketSessionClosed)
	ack, _ = c.PlaceOrder(ctx, exness.PendingOrder{Type: exness.OrderLimit, Instrument: "EURUSD", Side: exness.Buy, Volume: "1", Price: "1.07"}, "")
	if _, err := c.WaitOperation(ctx, ack.OperationID, time.Millisecond); !errors.As(err, &oe) || oe.Code != exness.CodeMarketSessionClosed {
		t.Fatalf("rejected: %v", err)
	}

	// The price rises 10: closing makes 0.10 × 100 × (2660 - 2650.2).
	s.SetPrice("XAUUSD", 2660, 2660.2)
	ack, _ = c.ClosePosition(ctx, pos, exness.CloseOptions{}, "")
	if _, err := c.WaitOperation(ctx, ack.OperationID, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	snap, _ = c.Snapshot(ctx)
	if len(snap.Positions) != 0 || len(snap.Orders) != 0 || snap.AccountState.Balance != "10098.00" {
		t.Fatalf("after the close: %+v", snap)
	}
	deals, err := c.Deals(ctx, exness.HistoryQuery{From: time.Now().Add(-time.Hour)})
	if err != nil || len(deals.Items) != 2 || deals.Items[1].Deal.Profit.Amount != "98.00" {
		t.Fatalf("deals = %+v, %v", deals, err)
	}
	candles, err := c.Candles(ctx, exness.CandleQuery{Instrument: "XAUUSD", Timeframe: exness.M5, From: time.Now(), Count: -3, PriceType: exness.PriceBoth})
	if err != nil || len(candles) != 3 || candles[2].Ask.Close != 2660.2 {
		t.Fatalf("candles = %+v, %v", candles, err)
	}
	// A wrong key is refused, as Exness does.
	cfg := s.Config()
	cfg.APIKey = "exnsk-other"
	bad, _ := exness.New(cfg)
	var e *exness.APIError
	if _, err := bad.Account(ctx); !errors.As(err, &e) || e.Code != exness.CodeAuthInvalidAPIKey {
		t.Fatalf("wrong key: %v", err)
	}
}
