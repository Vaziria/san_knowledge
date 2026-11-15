// Package livetrading is the tower's Live Trading page (the user,
// 2026-11-15: "Beside backtest analysis, can we have realtime trading menu
// in tower ?", the menu named "Live Trading"). Its first stage is "Monitor
// + Signals (paper trading)" (the user, 2026-11-15: "1a"), so it never
// trades: San's Exness account is real money.
//
//   - The monitor shows an Exness connection's account, open positions and
//     pending orders with live P/L, from the Events stream (a snapshot,
//     then the transactions and the account state every ~2 s; the REST
//     Snapshot while the stream is down), and live bid/ask from the Ticks
//     stream (monitor.go).
//   - Paper sessions run San Backtest approaches live, inside the tower
//     (the user, 2026-11-15: "2a"), on the candles Exness publishes, with
//     San Backtest's engine stepped one closed candle at a time (bt.Live),
//     several at once, journalled in the tower's database (session.go).
//
// Every Exness call goes through the read-only Exness interface below: it
// has no trading method, and safety_test.go fails the build's tests if
// any trading method of san_exness is named anywhere in this package or in
// the server's Live Trading file.
package livetrading

import (
	"context"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// Exness is the read-only part of san_exness's client: the account, the
// trading snapshot, candles, an instrument's conditions, and the Events and
// Ticks streams. Nothing here can place, change, cancel or close an order
// or a position.
type Exness interface {
	Account(ctx context.Context) (exness.AccountDetails, error)
	Snapshot(ctx context.Context) (exness.Snapshot, error)
	Candles(ctx context.Context, q exness.CandleQuery) ([]exness.Candle, error)
	Instrument(ctx context.Context, name string) (exness.InstrumentCondition, error)
	Events(ctx context.Context) (*exness.Stream, error)
	Ticks(ctx context.Context) (*exness.Stream, error)
	Now() time.Time
}

// ReadOnly wraps a client so only its read-only methods can be reached.
func ReadOnly(c *exness.Client) Exness { return readOnly{c: c} }

type readOnly struct{ c *exness.Client }

func (r readOnly) Account(ctx context.Context) (exness.AccountDetails, error) {
	return r.c.Account(ctx)
}
func (r readOnly) Snapshot(ctx context.Context) (exness.Snapshot, error) { return r.c.Snapshot(ctx) }
func (r readOnly) Candles(ctx context.Context, q exness.CandleQuery) ([]exness.Candle, error) {
	return r.c.Candles(ctx, q)
}
func (r readOnly) Instrument(ctx context.Context, name string) (exness.InstrumentCondition, error) {
	return r.c.Instrument(ctx, name)
}
func (r readOnly) Events(ctx context.Context) (*exness.Stream, error) { return r.c.Events(ctx) }
func (r readOnly) Ticks(ctx context.Context) (*exness.Stream, error)  { return r.c.Ticks(ctx) }
func (r readOnly) Now() time.Time                                     { return r.c.Now() }
