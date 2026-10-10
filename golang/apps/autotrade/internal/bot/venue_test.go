package bot

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/autotrade/internal/binance"
)

// mtEx is the fake exchange as MetaTrader: its stops are the position's own,
// and its amounts are in USD.
type mtEx struct{ *fakeEx }

func (mtEx) StopsOnPosition()                       {}
func (mtEx) AccountCurrency(context.Context) string { return "USD" }

// downEx is a venue that does not answer: MetaTrader closed.
type downEx struct{ *fakeEx }

var errDown = errors.New("MetaTrader did not answer")

func (downEx) Positions(context.Context, string) ([]binance.Position, error) { return nil, errDown }
func (downEx) Income(context.Context, string, time.Time, time.Time) ([]binance.Income, error) {
	return nil, errDown
}
func (downEx) Account(context.Context) (binance.Account, error) { return binance.Account{}, errDown }

// newVenueBot trades BTCUSDT on ex (Binance) and XAUUSD on mt (MetaTrader).
func newVenueBot(t *testing.T, ex *fakeEx, mt Exchange) *Bot {
	b := newBot(t, ex)
	b.Cfg.Pairs = []string{"BTCUSDT", "XAUUSD"}
	b.Cfg.MT5 = MT5Config{Pairs: []string{"XAUUSD"}, Leverage: 100, MaxPosition: 5000, MaxRisk: 50}
	b.MT5 = &Venue{Name: "MetaTrader 5 demo", Market: fakeMarket{}, Ex: mt}
	return b
}

func TestForPairRoutesToItsVenue(t *testing.T) {
	ex, mt := newFake(), mtEx{newFake()}
	b := newVenueBot(t, ex, mt)
	gold, err := b.ForPair("xauusd")
	if err != nil {
		t.Fatal(err)
	}
	c := gold.Cfg
	if gold.Ex != Exchange(mt) || c.Symbol != "XAUUSD" || c.Leverage != 100 || c.MaxPositionUSDT != 5000 || c.MaxRiskUSDT != 50 {
		t.Errorf("XAUUSD: ex %T, cfg %+v", gold.Ex, c)
	}
	// A pair's bot hands out the others on their own venues and limits.
	btc, err := gold.ForPair("btc/usdt")
	if err != nil {
		t.Fatal(err)
	}
	if btc.Ex != Exchange(ex) || btc.Cfg.MaxPositionUSDT != DefaultConfig().MaxPositionUSDT || btc.Cfg.Leverage != DefaultConfig().Leverage {
		t.Errorf("BTCUSDT from the XAUUSD bot: ex %T, cfg %+v", btc.Ex, btc.Cfg)
	}
	b.MT5 = nil
	if _, err := b.ForPair("XAUUSD"); err == nil || !strings.Contains(err.Error(), "MetaTrader 5") {
		t.Errorf("no MetaTrader venue: err = %v", err)
	}
}

func TestLoadConfigMT5(t *testing.T) {
	dir := t.TempDir()
	load := func(text string) (Config, error) {
		path := filepath.Join(dir, "autotrade.yaml")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return LoadConfig(path)
	}
	cfg, err := load("pairs: [BTCUSDT]\nmt5:\n  pairs: [XAUUSD, EURUSD.m]\n  leverage: 100\n  max_position: 5000\n  max_risk: 50\n  bridge: bridge\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cfg.Pairs, ",") != "BTCUSDT,XAUUSD,EURUSD.m" || strings.Join(cfg.BinancePairs(), ",") != "BTCUSDT" || !cfg.OnMT5("EURUSD.m") || cfg.OnMT5("BTCUSDT") {
		t.Errorf("pairs %v, binance %v", cfg.Pairs, cfg.BinancePairs())
	}
	if cfg.MT5.Bridge != filepath.Join(dir, "bridge") || cfg.MT5.MaxPosition != 5000 || cfg.MT5.MaxRisk != 50 || cfg.MT5.Leverage != 100 {
		t.Errorf("mt5 %+v", cfg.MT5)
	}
	if p, ok := cfg.Resolve("eurusd.m"); !ok || p != "EURUSD.m" {
		t.Errorf("resolve eurusd.m = %q %v", p, ok)
	}
	if cfg, err := load("pairs: []\nmt5: {pairs: [XAUUSD], leverage: 100, max_position: 5000}\n"); err != nil || cfg.Symbol != "XAUUSD" {
		t.Errorf("MetaTrader only: %+v %v", cfg, err)
	}
	for bad, want := range map[string]string{
		"mt5: {pairs: [XAUUSD], max_position: 5000}\n":                              "mt5.leverage",
		"mt5: {pairs: [XAUUSD], leverage: 100}\n":                                   "mt5.max_position",
		"pairs: [XAUUSD]\nmt5: {pairs: [xauusd], leverage: 100, max_position: 1}\n": "listed twice",
		"mt5: {pairs: [XAU USD], leverage: 100, max_position: 1}\n":                 "mt5.pairs",
	} {
		if _, err := load(bad); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", bad, err, want)
		}
	}
	if cfg, err := load("pairs: [BTCUSDT]\n"); err != nil || len(cfg.MT5.Pairs) != 0 {
		t.Errorf("no mt5 section: %+v %v", cfg.MT5, err)
	}
}

func TestProtectOnMT5MovesTheStopInOneStep(t *testing.T) {
	ex, mt := newFake(), mtEx{newFake()}
	mt.pos, mt.entry, mt.mark = 1, 2650, 2660
	mt.orders = []binance.AlgoOrder{{AlgoID: 71, Type: binance.StopMarket, TriggerPrice: 2640}}
	b := newVenueBot(t, ex, mt)
	gold, err := b.ForPair("XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	e, err := gold.Protect(context.Background(), 2645, 0, "trail")
	if err != nil {
		t.Fatal(err)
	}
	if e.Currency != "USD" {
		t.Errorf("entry currency %q, want USD", e.Currency)
	}
	// No cancel first: the position is never without its stop.
	if got := calls(mt.fakeEx); got != "stop STOP_MARKET SELL 2645.0" {
		t.Errorf("MetaTrader calls = %s", got)
	}
	if len(ex.calls) != 0 {
		t.Errorf("Binance was asked: %s", calls(ex))
	}
	mt.calls = nil
	mt.stopErr[binance.StopMarket] = errors.New("invalid stops")
	if _, err := gold.Protect(context.Background(), 2650, 0, "tighter"); err == nil || !strings.Contains(err.Error(), "the old one stays") {
		t.Errorf("rejected stop: err = %v", err)
	}
}

func TestTradesWithAVenueDown(t *testing.T) {
	ex := newFake()
	b := newVenueBot(t, ex, downEx{newFake()})
	at := func(m int) time.Time { return time.Date(2026, 10, 9, 10, m, 0, 0, time.UTC) }
	for _, e := range []Entry{
		{Time: at(0), Symbol: "BTCUSDT", Action: "open_long", Setup: "trend_pullback", Price: 80000, Qty: 0.01, StopLoss: 79000, Reason: "btc"},
		{Time: at(1), Symbol: "XAUUSD", Action: "open_long", Setup: "trend_pullback", Price: 2650, Qty: 1, StopLoss: 2640, Reason: "gold"},
		{Time: at(10), Symbol: "BTCUSDT", Action: "close", Reason: "btc done"},
		{Time: at(11), Symbol: "XAUUSD", Action: "close", Reason: "gold done"},
	} {
		if err := b.Journal.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	ex.income = []binance.Income{{Type: "REALIZED_PNL", Amount: 12, Time: at(10).Add(time.Second)}}
	all, _ := b.Journal.Last(10)
	trades, unscored, err := b.trades(context.Background(), all, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 2 || !trades[0].Final || trades[0].Net != 12 || trades[1].Final || trades[1].Net != 0 {
		t.Errorf("trades %+v", trades)
	}
	if len(unscored) != 1 || !strings.Contains(unscored[0], "XAUUSD: MetaTrader did not answer") {
		t.Errorf("unscored %v", unscored)
	}
	// The knowledge sync goes on without MetaTrader: Binance's wallet only.
	in, err := b.KnowledgeInput(context.Background(), nil)
	if err != nil || in.Account == nil || in.Account.Wallet != 15000 || len(in.Positions) != 2 {
		t.Errorf("knowledge input %+v, %v", in.Account, err)
	}
}

func TestKnowledgeWalletAddsBothVenues(t *testing.T) {
	b := newVenueBot(t, newFake(), mtEx{newFake()})
	in, err := b.KnowledgeInput(context.Background(), nil)
	if err != nil || in.Account == nil || in.Account.Wallet != 30000 || in.Config.MT5.MaxPosition != 5000 {
		t.Errorf("account %+v, mt5 %+v, %v", in.Account, in.Config.MT5, err)
	}
}

func TestSnapshotOnMT5(t *testing.T) {
	mt := mtEx{newFake()}
	b := newVenueBot(t, newFake(), mt)
	gold, err := b.ForPair("XAUUSD")
	if err != nil {
		t.Fatal(err)
	}
	s, err := gold.Snapshot(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if s.Venue != "MetaTrader 5 demo" || s.Currency != "USD" || s.Positioning != nil {
		t.Errorf("venue %q, currency %q, positioning %v", s.Venue, s.Currency, s.Positioning)
	}
	var buf bytes.Buffer
	s.Render(&buf)
	out := buf.String()
	for _, want := range []string{
		"# XAUUSD 15m snapshot",
		"MetaTrader 5 demo price 80,000.0 (middle of bid and ask",
		"| tick volume |",
		"## Portfolio (MetaTrader 5 demo)",
		"Wallet 15,000.00 USD",
		"Limits: max position 5,000 USD, max loss at stop 50 USD, leverage 100x",
		"Sizes are in units of XAUUSD",
		"The account's leverage is 1:20 and mt5.leverage says 100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("render is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "funding") || strings.Contains(out, "USDT") {
		t.Errorf("a MetaTrader snapshot speaks of funding or USDT:\n%s", out)
	}
}
