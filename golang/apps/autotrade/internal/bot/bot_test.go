package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// fakeEx is an in-memory exchange in one-way mode.
type fakeEx struct {
	mark     float64
	pos      float64 // signed amount
	entry    float64
	leverage int
	orders   []binance.AlgoOrder
	nextID   int64
	calls    []string
	stopErr  map[string]error // by order type; rejects the next such order
	dual     bool
	noKey    bool
	income   []binance.Income
}

func newFake() *fakeEx {
	return &fakeEx{mark: 80000, leverage: 20, nextID: 100, stopErr: map[string]error{}}
}

func (f *fakeEx) call(format string, a ...any) { f.calls = append(f.calls, fmt.Sprintf(format, a...)) }

func (f *fakeEx) HasKey() bool                           { return !f.noKey }
func (f *fakeEx) SyncTime(context.Context) error         { return nil }
func (f *fakeEx) Now() time.Time                         { return time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC) }
func (f *fakeEx) DualSide(context.Context) (bool, error) { return f.dual, nil }
func (f *fakeEx) SymbolInfo(context.Context, string) (binance.SymbolInfo, error) {
	return binance.SymbolInfo{Symbol: "BTCUSDT", TickSize: 0.1, PriceDec: 1, StepSize: 0.001, QtyDec: 3, MinQty: 0.001, MaxQty: 120, MinNotional: 100}, nil
}
func (f *fakeEx) PremiumIndex(context.Context, string) (binance.PremiumIndex, error) {
	return binance.PremiumIndex{MarkPrice: f.mark}, nil
}
func (f *fakeEx) Account(context.Context) (binance.Account, error) {
	return binance.Account{WalletBalance: 15000, AvailableBalance: 14000}, nil
}
func (f *fakeEx) Positions(context.Context, string) ([]binance.Position, error) {
	if f.pos == 0 {
		return nil, nil
	}
	return []binance.Position{{Symbol: "BTCUSDT", Amount: f.pos, EntryPrice: f.entry, MarkPrice: f.mark, Notional: f.pos * f.mark}}, nil
}
func (f *fakeEx) SymbolConfig(context.Context, string) (binance.SymbolConfig, error) {
	return binance.SymbolConfig{Leverage: f.leverage}, nil
}
func (f *fakeEx) SetLeverage(_ context.Context, _ string, l int) error {
	f.call("leverage %d", l)
	f.leverage = l
	return nil
}
func (f *fakeEx) Income(_ context.Context, _ string, start, end time.Time) ([]binance.Income, error) {
	var out []binance.Income
	for _, in := range f.income {
		if !in.Time.Before(start) && (end.IsZero() || !in.Time.After(end)) {
			out = append(out, in)
		}
	}
	return out, nil
}

func (f *fakeEx) MarketOrder(_ context.Context, _, side, qty string, reduceOnly bool) (binance.Order, error) {
	f.call("market %s %s reduce=%v", side, qty, reduceOnly)
	var q float64
	fmt.Sscan(qty, &q)
	if side == "SELL" {
		q = -q
	}
	if f.pos == 0 {
		f.entry = f.mark
	}
	f.pos = math.Round((f.pos+q)*1000) / 1000
	f.nextID++
	return binance.Order{OrderID: f.nextID, Status: "FILLED", AvgPrice: f.mark, ExecutedQty: math.Abs(q)}, nil
}

func (f *fakeEx) StopOrder(_ context.Context, _, side, typ, trigger string) (binance.AlgoOrder, error) {
	f.call("stop %s %s %s", typ, side, trigger)
	if err := f.stopErr[typ]; err != nil {
		delete(f.stopErr, typ) // rejects once
		return binance.AlgoOrder{}, err
	}
	var p float64
	fmt.Sscan(trigger, &p)
	f.nextID++
	o := binance.AlgoOrder{AlgoID: f.nextID, Type: typ, Side: side, TriggerPrice: p, ClosePosition: true, Status: "NEW"}
	f.orders = append(f.orders, o)
	return o, nil
}

func (f *fakeEx) OpenAlgoOrders(context.Context, string) ([]binance.AlgoOrder, error) {
	return append([]binance.AlgoOrder(nil), f.orders...), nil
}

func (f *fakeEx) CancelAlgoOrder(_ context.Context, _ string, id int64) error {
	f.call("cancel %d", id)
	for i, o := range f.orders {
		if o.AlgoID == id {
			f.orders = append(f.orders[:i], f.orders[i+1:]...)
		}
	}
	return nil
}

func (f *fakeEx) CancelAlgoOrders(context.Context, string) error {
	f.call("cancel all")
	f.orders = nil
	return nil
}

func newBot(t *testing.T, ex *fakeEx) *Bot {
	cfg := DefaultConfig()
	cfg.Journal = filepath.Join(t.TempDir(), "journal.jsonl")
	return &Bot{Cfg: cfg, Market: fakeMarket{}, Ex: ex, Journal: Journal{Path: cfg.Journal}}
}

func calls(ex *fakeEx) string { return strings.Join(ex.calls, "; ") }

func TestOpenLong(t *testing.T) {
	ex := newFake()
	b := newBot(t, ex)
	e, err := b.Open(context.Background(), "long", []string{"trend_pullback"}, 1000, 79000.04, 82000, "trend up")
	if err != nil {
		t.Fatal(err)
	}
	// 1000/80000 = 0.0125 → floored to 0.012; leverage set to the config's 1x.
	want := "leverage 1; market BUY 0.012 reduce=false; stop STOP_MARKET SELL 79000.0; stop TAKE_PROFIT_MARKET SELL 82000.0"
	if got := calls(ex); got != want {
		t.Errorf("calls:\n got %s\nwant %s", got, want)
	}
	if e.Qty != 0.012 || e.Price != 80000 || e.StopLoss != 79000 || len(e.OrderIDs) != 3 || e.Error != "" {
		t.Errorf("entry = %+v", e)
	}
	got, _ := b.Journal.Last(10)
	if len(got) != 1 || got[0].Action != "open_long" || got[0].Reason != "trend up" {
		t.Errorf("journal = %+v", got)
	}
}

func TestOpenShortCancelsLeftOverStops(t *testing.T) {
	ex := newFake()
	ex.leverage = 1
	ex.orders = []binance.AlgoOrder{{AlgoID: 7, Type: binance.StopMarket}}
	b := newBot(t, ex)
	if _, err := b.Open(context.Background(), "short", []string{"trend_pullback"}, 500, 80500, 0, "rejected at resistance"); err != nil {
		t.Fatal(err)
	}
	want := "cancel all; market SELL 0.006 reduce=false; stop STOP_MARKET BUY 80500.0"
	if got := calls(ex); got != want {
		t.Errorf("calls:\n got %s\nwant %s", got, want)
	}
}

func TestOpenRefusals(t *testing.T) {
	cases := []struct {
		name         string
		setup        func(*fakeEx)
		side         string
		usdt, sl, tp float64
		want         string
	}{
		{"open position", func(f *fakeEx) { f.pos = 0.01 }, "long", 500, 79500, 0, "already open"},
		{"over max size", nil, "long", 1500, 79500, 0, "max_position_usdt"},
		{"no stop", nil, "long", 500, 0, 0, "stop-loss is required"},
		{"stop above long", nil, "long", 500, 80100, 0, "wrong side"},
		{"stop below short", nil, "short", 500, 79900, 0, "wrong side"},
		{"tp below long", nil, "long", 500, 79500, 79900, "take-profit"},
		{"risk", nil, "long", 1000, 78000, 0, "max_risk_usdt"}, // 0.012 x 2000 = 24 > 20
		{"too small", nil, "long", 50, 79500, 0, "minimum"},
		{"hedge mode", func(f *fakeEx) { f.dual = true }, "long", 500, 79500, 0, "hedge mode"},
		{"bad side", nil, "up", 500, 79500, 0, "long or short"},
		{"no key", func(f *fakeEx) { f.noKey = true }, "long", 500, 79500, 0, "no API key"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex := newFake()
			if c.setup != nil {
				c.setup(ex)
			}
			b := newBot(t, ex)
			e, err := b.Open(context.Background(), c.side, []string{"trend_pullback"}, c.usdt, c.sl, c.tp, "test")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
			for _, call := range ex.calls {
				if strings.HasPrefix(call, "market") || strings.HasPrefix(call, "stop") {
					t.Errorf("refused open still sent %q", call)
				}
			}
			if got, _ := b.Journal.Last(1); len(got) != 1 || got[0].Error == "" || e.Error == "" {
				t.Errorf("refusal not journaled: %+v", got)
			}
		})
	}
	if _, err := newBot(t, newFake()).Open(context.Background(), "long", []string{"trend_pullback"}, 500, 79500, 0, ""); err == nil {
		t.Error("open without a reason accepted")
	}
}

func TestOpenClosesAgainWhenStopRejected(t *testing.T) {
	ex := newFake()
	ex.stopErr[binance.StopMarket] = errors.New("-2021 Order would immediately trigger")
	b := newBot(t, ex)
	_, err := b.Open(context.Background(), "long", []string{"trend_pullback"}, 500, 79500, 0, "test")
	if err == nil || !strings.Contains(err.Error(), "closed again") {
		t.Fatalf("err = %v", err)
	}
	if ex.pos != 0 {
		t.Errorf("position %v left open without a stop", ex.pos)
	}
	if got := ex.calls[len(ex.calls)-1]; got != "market SELL 0.006 reduce=true" {
		t.Errorf("last call %q", got)
	}
}

func TestClose(t *testing.T) {
	ex := newFake()
	ex.pos, ex.entry, ex.mark = -0.01, 81000, 80000
	ex.orders = []binance.AlgoOrder{{AlgoID: 7, Type: binance.StopMarket}}
	b := newBot(t, ex)
	e, err := b.Close(context.Background(), "target reached")
	if err != nil {
		t.Fatal(err)
	}
	if want := "market BUY 0.010 reduce=true; cancel all"; calls(ex) != want {
		t.Errorf("calls = %s, want %s", calls(ex), want)
	}
	// short 0.01 from 81000 to 80000: +10
	if math.Abs(e.PnL-10) > 1e-9 || ex.pos != 0 {
		t.Errorf("pnl %v pos %v", e.PnL, ex.pos)
	}

	if _, err := b.Close(context.Background(), "again"); err == nil || !strings.Contains(err.Error(), "no open position") {
		t.Errorf("close when flat: %v", err)
	}
}

func TestProtect(t *testing.T) {
	ex := newFake()
	ex.pos, ex.entry, ex.mark = 0.012, 80000, 81000
	ex.orders = []binance.AlgoOrder{{AlgoID: 7, Type: binance.StopMarket, TriggerPrice: 79000}, {AlgoID: 8, Type: binance.TakeProfitMarket, TriggerPrice: 82000}}
	b := newBot(t, ex)

	if _, err := b.Protect(context.Background(), 78500, 0, "wider"); err == nil || !strings.Contains(err.Error(), "tightened") {
		t.Errorf("widening the stop: %v", err)
	}
	if len(ex.calls) != 0 {
		t.Errorf("refused protect sent %s", calls(ex))
	}

	if _, err := b.Protect(context.Background(), 80200, 0, "trail to break-even"); err != nil {
		t.Fatal(err)
	}
	if want := "cancel 7; stop STOP_MARKET SELL 80200.0"; calls(ex) != want {
		t.Errorf("calls = %s, want %s", calls(ex), want)
	}

	// A rejected new stop puts the old one back.
	ex.calls = nil
	ex.stopErr[binance.StopMarket] = errors.New("rejected")
	if _, err := b.Protect(context.Background(), 80500, 0, "tighter"); err == nil || !strings.Contains(err.Error(), "is back") {
		t.Errorf("err = %v", err)
	}
	if !strings.HasSuffix(calls(ex), "stop STOP_MARKET SELL 80500.0; stop STOP_MARKET SELL 80200.0") {
		t.Errorf("calls = %s", calls(ex))
	}
}

func TestHoldAndRender(t *testing.T) {
	ex := newFake()
	b := newBot(t, ex)
	if _, err := b.Hold(context.Background(), "no edge"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Open(context.Background(), "long", []string{"trend_pullback"}, 500, 79500, 0, "breakout"); err != nil {
		t.Fatal(err)
	}
	// The stop was hit: the position and its orders are gone.
	ex.pos, ex.orders = 0, []binance.AlgoOrder{{AlgoID: 9, Type: binance.TakeProfitMarket}}

	b.Market = fakeMarket{}
	s, err := b.Snapshot(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	s.Render(&buf)
	out := buf.String()
	for _, want := range []string{
		"# BTCUSDT 15m snapshot",
		"Position: none",
		"hold @ 80,000.0. Reason: no edge",
		"open_long 0.006 @ 80,000.0 (480.00 USDT) SL 79,500.0. Reason: breakout",
		"left over from a closed position",
		"stop-loss or take-profit was hit",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render is missing %q:\n%s", want, out)
		}
	}
}

func TestSnapshotPlansTheOpenTrade(t *testing.T) {
	ex := newFake()
	b := newBot(t, ex)
	b.Market = fakeMarket{}
	if _, err := b.Open(context.Background(), "long", []string{"trend_pullback"}, 500, 79500, 81000, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Open(context.Background(), "long", []string{"Bad Label!"}, 500, 79500, 0, "test"); err == nil {
		t.Error("a malformed setup label was accepted")
	}
	s, err := b.Snapshot(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if s.Plan == nil || s.Plan.Trade.Setup != "trend_pullback" || s.Plan.Trade.InitialStop != 79500 || s.Plan.Target != 81000 {
		t.Fatalf("plan = %+v", s.Plan)
	}
	if s.Signals == nil || len(s.Suggestions) != 0 {
		t.Errorf("signals %+v, suggestions %v with a position open", s.Signals, s.Suggestions)
	}
	var buf bytes.Buffer
	s.Render(&buf)
	for _, want := range []string{"## Position plan", "trend_pullback long from 80,000.0", "## Signals", "Regime: **"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("render is missing %q:\n%s", want, buf.String())
		}
	}
}

type fakeMarket struct{}

func (fakeMarket) SyncTime(context.Context) error { return nil }
func (fakeMarket) Now() time.Time                 { return time.Date(2026, 10, 9, 10, 7, 0, 0, time.UTC) }
func (fakeMarket) Klines(_ context.Context, _, interval string, limit int) ([]binance.Candle, error) {
	step := map[string]time.Duration{"15m": 15 * time.Minute, "1h": time.Hour, "4h": 4 * time.Hour}[interval]
	end := time.Date(2026, 10, 9, 10, 7, 0, 0, time.UTC).Truncate(step).Add(step) // forming candle's close
	out := make([]binance.Candle, limit)
	for i := range out {
		open := end.Add(-time.Duration(limit-i) * step)
		c := 80000 + float64(i)
		out[i] = binance.Candle{OpenTime: open, CloseTime: open.Add(step - time.Millisecond), Open: c, High: c + 5, Low: c - 5, Close: c, QuoteVolume: 1e6}
	}
	return out, nil
}

func TestLoadConfigRefusesMainnet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TradeURL = binance.MainnetURL
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "test network") {
		t.Errorf("mainnet trade_url: %v", err)
	}
	cfg.TradeURL = binance.DemoURL
	if err := cfg.Validate(); err != nil {
		t.Errorf("demo trade_url: %v", err)
	}
}

func TestLoadConfigYAML(t *testing.T) {
	dir := t.TempDir()
	write := func(text string) string {
		path := filepath.Join(dir, "autotrade.yaml")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cfg, err := LoadConfig(write("# the user's limits\npairs: [ethusdt, btc/usdt, BNBUSDT]\nmax_risk_usdt: 15 # per trade\nsetups: []\nfee_rate: 0.0004\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.Pairs, ",") != "ETHUSDT,BTCUSDT,BNBUSDT" || cfg.Symbol != "ETHUSDT" || cfg.MaxRiskUSDT != 15 || cfg.FeeRate != 0.0004 || len(cfg.Setups) != 0 {
		t.Errorf("read %+v", cfg)
	}
	if cfg.MaxPositionUSDT != DefaultConfig().MaxPositionUSDT || cfg.Journal != filepath.Join(dir, "data/journal.jsonl") {
		t.Errorf("defaults: max_position_usdt %v, journal %s", cfg.MaxPositionUSDT, cfg.Journal)
	}
	if _, err := LoadConfig(write("max_risk_usd: 5\n")); err == nil || !strings.Contains(err.Error(), "max_risk_usd") {
		t.Errorf("a misspelled limit must be refused: %v", err)
	}
	if cfg, err := LoadConfig(write("")); err != nil || cfg.Symbol != DefaultConfig().Symbol {
		t.Errorf("empty file: %+v %v", cfg, err)
	}
	for _, bad := range []string{"pairs: []\n", "pairs: [BTCUSDT, btcusdt]\n", "pairs: [BTC USDT]\n"} {
		if _, err := LoadConfig(write(bad)); err == nil || !strings.Contains(err.Error(), "pairs") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestForPair(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Pairs = []string{"BTCUSDT", "ETHUSDT"}
	b := &Bot{Cfg: cfg}
	if eth, err := b.ForPair("eth/usdt"); err != nil || eth.Cfg.Symbol != "ETHUSDT" || b.Cfg.Symbol != "BTCUSDT" {
		t.Errorf("eth/usdt: %v %v", eth, err)
	}
	if _, err := b.ForPair(""); err == nil || !strings.Contains(err.Error(), "name the pair") {
		t.Errorf("no pair with two configured: %v", err)
	}
	if _, err := b.ForPair("SOLUSDT"); err == nil || !strings.Contains(err.Error(), "not one of the pairs") {
		t.Errorf("an unlisted pair must be refused: %v", err)
	}
	b.Cfg.Pairs = []string{"BTCUSDT"}
	if one, err := b.ForPair(""); err != nil || one.Cfg.Symbol != "BTCUSDT" {
		t.Errorf("the only pair: %v %v", one, err)
	}
}

func TestGroup(t *testing.T) {
	for _, c := range []struct {
		v    float64
		dec  int
		want string
	}{{82598.69, 1, "82,598.7"}, {-1234567, 0, "-1,234,567"}, {-0.001, 2, "0.00"}, {0.012, -1, "0.012"}, {82654.55802536, -1, "82,654.558"}, {999, 2, "999.00"}} {
		if got := group(c.v, c.dec); got != c.want {
			t.Errorf("group(%v, %d) = %s, want %s", c.v, c.dec, got, c.want)
		}
	}
}

func TestReview(t *testing.T) {
	ex := newFake()
	b := newBot(t, ex)
	at := func(m int) time.Time { return time.Date(2026, 10, 9, 10, m, 0, 0, time.UTC) }
	j := []Entry{
		// Closed by the loop: +12 realized, -0.8 fees.
		{Time: at(0), Action: "open_long", Setup: "trend_pullback", Price: 80000, Qty: 0.01, StopLoss: 79000, Reason: "a"},
		{Time: at(5), Action: "hold", Reason: "veto: under resistance"},
		{Time: at(10), Action: "close", Reason: "b"},
		// Stopped out on the exchange, then the next open.
		{Time: at(20), Action: "open_short", Setup: "range_fade", Price: 81000, Qty: 0.01, StopLoss: 81500, Reason: "c"},
		{Time: at(25), Action: "open_long", Error: "refused", Reason: "d"},
		{Time: at(40), Action: "open_long", Setup: "range_fade", Price: 80500, Qty: 0.01, StopLoss: 80000, Reason: "e"},
	}
	for _, e := range j {
		if err := b.Journal.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	ex.income = []binance.Income{
		{Type: "COMMISSION", Amount: -0.4, Time: at(0).Add(time.Second)},
		{Type: "REALIZED_PNL", Amount: 12, Time: at(10).Add(time.Second)},
		{Type: "COMMISSION", Amount: -0.4, Time: at(10).Add(time.Second)},
		{Type: "COMMISSION", Amount: -0.4, Time: at(20).Add(time.Second)},
		{Type: "REALIZED_PNL", Amount: -5, Time: at(30)},
		{Type: "COMMISSION", Amount: -0.4, Time: at(30)},
		// The next open's fee, stamped a little before its journal entry: the
		// exchange's clock and ours are apart. It is the next trade's alone.
		{Type: "COMMISSION", Amount: -0.4, Time: at(40).Add(-300 * time.Millisecond)},
	}
	ex.pos, ex.entry, ex.mark = 0.01, 80500, 80600 // the last trade is still open

	r, err := b.Review(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Trades) != 3 || r.Holds != 1 || r.Vetoes != 1 || r.Refusals != 1 {
		t.Fatalf("review = %+v", r)
	}
	want := []struct {
		by  string
		net float64
		r   float64
	}{{"close", 11.2, 1.12}, {"exchange", -5.8, -1.16}, {"open", -0.4, -0.08}}
	for i, w := range want {
		got := r.Trades[i]
		if got.ClosedBy != w.by || math.Abs(got.Net-w.net) > 1e-9 || math.Abs(got.R-w.r) > 1e-9 {
			t.Errorf("trade %d = %s %.2f %.2fR, want %s %.2f %.2fR", i, got.ClosedBy, got.Net, got.R, w.by, w.net, w.r)
		}
	}
	if !r.Trades[0].Coded || r.Trades[1].Coded {
		t.Error("coded flags wrong")
	}
	var buf bytes.Buffer
	r.Render(&buf)
	for _, s := range []string{"# Review: 2 closed trades", "| coded signals | 1 |", "| own ideas | 1 |", "1 holds (1 of them vetoes)"} {
		if !strings.Contains(buf.String(), s) {
			t.Errorf("review is missing %q:\n%s", s, buf.String())
		}
	}
}

func TestKnowledgeInputRisk(t *testing.T) {
	ex := newFake()
	b := newBot(t, ex)
	at := func(m int) time.Time { return time.Date(2026, 10, 9, 10, m, 0, 0, time.UTC) }
	for _, e := range []Entry{
		{Time: at(0), Action: "open_long", Setup: "trend_pullback", Price: 80000, Qty: 0.01, StopLoss: 79000, Reason: "closed one"},
		{Time: at(2), Action: "protect", StopLoss: 79200, Reason: "tighten"},
		{Time: at(4), Action: "close", Reason: "done"},
		{Time: at(10), Action: "open_long", Setup: "trend_pullback", Price: 80500, Qty: 0.01, StopLoss: 80000, Reason: "open one"},
		{Time: at(12), Action: "protect", StopLoss: 80100, Reason: "tighten"},
		{Time: at(13), Action: "protect", StopLoss: 80600, Error: "refused", Reason: "too close"},
		{Time: at(14), Symbol: "ETHUSDT", Action: "protect", StopLoss: 3000, Reason: "another pair"},
	} {
		if err := b.Journal.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	ex.pos, ex.entry = 0.01, 80500
	in, err := b.KnowledgeInput(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Positions) != 2 || in.Positions[0].StopNow != 79200 || in.Positions[1].StopNow != 80100 {
		t.Errorf("stops now %+v", in.Positions)
	}
	c := in.Config
	if strings.Join(c.Pairs, ",") != "BTCUSDT" || c.Leverage != b.Cfg.Leverage || c.MaxRiskUSDT != b.Cfg.MaxRiskUSDT || c.MaxOpenPerPair != 1 {
		t.Errorf("config %+v", c)
	}
	if in.Account == nil || in.Account.Wallet != 15000 {
		t.Errorf("account %+v", in.Account)
	}
	ex.noKey = true
	if in, _ := b.KnowledgeInput(context.Background(), nil); in.Account != nil {
		t.Error("without a key the wallet is unknown")
	}
}

func TestTradesPairPerSymbol(t *testing.T) {
	ex := newFake()
	b := newBot(t, ex)
	at := func(m int) time.Time { return time.Date(2026, 10, 9, 10, m, 0, 0, time.UTC) }
	for _, e := range []Entry{
		{Time: at(0), Symbol: "BTCUSDT", Action: "open_long", Setup: "trend_pullback", Price: 80000, Qty: 0.01, StopLoss: 79000, Reason: "btc"},
		{Time: at(5), Symbol: "ETHUSDT", Action: "open_short", Setup: "trend_pullback", Price: 3000, Qty: 0.1, StopLoss: 3050, Reason: "eth"},
		{Time: at(10), Symbol: "ETHUSDT", Action: "close", Reason: "eth done"},
	} {
		if err := b.Journal.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	ex.pos = 0.01 // the fake has one position for any symbol: both look open
	all, _ := b.Journal.Last(10)
	trades, err := b.trades(context.Background(), all, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 2 || trades[0].Symbol != "BTCUSDT" || trades[0].ClosedBy != "open" || trades[1].Symbol != "ETHUSDT" || trades[1].ClosedBy != "close" {
		t.Errorf("trades %+v", trades)
	}
}
