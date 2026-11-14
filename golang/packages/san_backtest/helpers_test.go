package san_backtest

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"
)

// memSource is an in-memory DataSource that logs its queries.
type memSource struct {
	mu      sync.Mutex
	inst    map[string]Instrument
	candles map[string][]Candle // instrument + " " + tf
	queries []Query
}

func newMem() *memSource {
	return &memSource{inst: map[string]Instrument{}, candles: map[string][]Candle{}}
}

func (m *memSource) add(in Instrument, tf string, cs []Candle) *memSource {
	m.inst[in.Name] = in
	m.candles[in.Name+" "+tf] = cs
	return m
}

func (m *memSource) Candles(_ context.Context, q Query) ([]Candle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queries = append(m.queries, q)
	return append([]Candle{}, between(m.candles[q.Instrument+" "+q.Timeframe], q.From, q.To)...), nil
}

func (m *memSource) Instrument(_ context.Context, name string) (Instrument, error) {
	return m.inst[name], nil
}

// gold is an XAUUSD-like instrument without commission or swap.
func gold() Instrument {
	return Instrument{Name: "XAUUSD", Base: "XAU", Quote: "USD", Digits: 2, ContractSize: 100, MarginPercent: 100,
		LotMin: 0.01, LotMax: 100, LotStep: 0.01, SwapDays: map[time.Weekday]int{time.Wednesday: 3}}
}

type bar struct{ o, h, l, c float64 }

func flat(p float64) bar { return bar{p, p, p, p} }

// series builds consecutive candles of tf from start, ask = bid + spread.
func candles(tf string, start time.Time, spread float64, bars ...bar) []Candle {
	var out []Candle
	t := start
	for _, b := range bars {
		out = append(out, Candle{OpenTime: t, CloseTime: closeTime(tf, t), Open: b.o, High: b.h, Low: b.l, Close: b.c,
			AskOpen: b.o + spread, AskHigh: b.h + spread, AskLow: b.l + spread, AskClose: b.c + spread, Volume: 1})
		t = closeTime(tf, t)
	}
	return out
}

func flats(n int, p float64) []bar {
	out := make([]bar, n)
	for i := range out {
		out[i] = flat(p)
	}
	return out
}

// script is a strategy driven by a function.
type script struct {
	name   string
	tfs    []string
	warm   int
	on     func(c *Context) Signal
	params map[string]float64
	starts int
}

func (s *script) Name() string               { return s.name }
func (s *script) Timeframes() []string       { return s.tfs }
func (s *script) Warmup() int                { return s.warm }
func (s *script) OnCandle(c *Context) Signal { return s.on(c) }
func (s *script) OnStart(StartInfo)          { s.starts++ }

type tunable struct{ script }

func (t *tunable) Params() map[string]float64 { return t.params }

// once buys (or sells) the first time it is flat and has not traded.
func once(sig Signal) func(c *Context) Signal {
	done := false
	return func(c *Context) Signal {
		if done || c.Position() != nil {
			return Wait
		}
		done = true
		return sig
	}
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func mustRun(t *testing.T, cfg Config) *Result {
	t.Helper()
	res, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
