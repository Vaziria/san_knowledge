// Package bot is the trading loop of docs/basic_flow.md, minus the AI: Snapshot does
// "connect", "feature extraction" and "portfolio data"; Open, Close, Protect
// and Hold carry out the "decide position" step that Claude chooses. Every
// decision is written to the journal so the next tick can see it.
package bot

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/strategy"
)

// Config is autotrade.yaml, where all of autotrade's settings live
// (docs/readme.md). The limits are the user's, not the AI's: the loop must
// not edit this file.
type Config struct {
	// Pairs are the pairs the loop trades, by symbol (BTCUSDT; btc/usdt is
	// read as BTCUSDT): the "Pair That Traded" of docs/knowledge.md. Each is
	// traded on its own, with its own position and the limits below. The
	// file's pairs trade on Binance; LoadConfig adds MT5.Pairs after them.
	Pairs []string `json:"pairs" yaml:"pairs"`
	// Symbol is the pair a command acts on, one of Pairs: the first, or the
	// one Bot.ForPair picked. It is not read from the file.
	Symbol string `json:"symbol" yaml:"-"`

	Interval         string   `json:"interval" yaml:"interval"`
	ContextIntervals []string `json:"context_intervals" yaml:"context_intervals"`
	Candles          int      `json:"candles" yaml:"candles"` // per timeframe, for the indicators

	// TradeURL is where orders go. Only the test network is allowed.
	TradeURL string `json:"trade_url" yaml:"trade_url"`
	// MarketURL is where candles come from: public, read-only, never sent a
	// key. Mainnet by default, because testnet volume is not real.
	MarketURL string `json:"market_url" yaml:"market_url"`

	Leverage int `json:"leverage" yaml:"leverage"`
	// MaxPositionUSDT caps the notional value of a new position.
	MaxPositionUSDT float64 `json:"max_position_usdt" yaml:"max_position_usdt"`
	// MaxRiskUSDT caps the loss if the stop-loss is hit, measured from the
	// mark price. 0 = no cap.
	MaxRiskUSDT float64 `json:"max_risk_usdt" yaml:"max_risk_usdt"`

	Journal string `json:"journal" yaml:"journal"` // relative to the config file

	// Setups are the coded entry rules the snapshot checks, in priority
	// order (see internal/strategy). Empty = no signals.
	Setups []string `json:"setups" yaml:"setups"`
	// FeeRate is the cost of one fill, for the backtest and the break-even
	// stop: 0.0005 = 0.05%, the mainnet taker fee (the testnet charges 0.04%).
	FeeRate float64 `json:"fee_rate" yaml:"fee_rate"`

	// MT5 is the second venue: a MetaTrader 5 demo account.
	MT5 MT5Config `json:"mt5" yaml:"mt5"`
}

// MT5Config is the second venue: a MetaTrader 5 demo account (MIFX),
// reached through the AutotradeBridge Expert Advisor (internal/mt5). Its
// pairs trade there with its own leverage and limits, in the account
// currency; every other pair trades on Binance.
type MT5Config struct {
	// Pairs are MetaTrader symbols as the broker spells them (XAUUSD).
	// Empty: MetaTrader is not used.
	Pairs []string `json:"pairs" yaml:"pairs"`
	// Bridge is the folder the EA answers in. Empty: MetaTrader's common
	// Files\autotrade folder.
	Bridge string `json:"bridge,omitempty" yaml:"bridge"`
	// Leverage is the account's, which the broker sets: opens are refused
	// while they differ.
	Leverage int `json:"leverage" yaml:"leverage"`
	// MaxPosition caps the notional value of a new position and MaxRisk the
	// loss at its stop (0 = no cap), in the account currency.
	MaxPosition float64 `json:"max_position" yaml:"max_position"`
	MaxRisk     float64 `json:"max_risk" yaml:"max_risk"`
}

func DefaultConfig() Config {
	return Config{
		Pairs:            []string{"BTCUSDT"},
		Symbol:           "BTCUSDT",
		Interval:         "15m",
		ContextIntervals: []string{"1h", "4h"},
		Candles:          300,
		TradeURL:         binance.TestnetURL,
		MarketURL:        binance.MainnetURL,
		Leverage:         1,
		MaxPositionUSDT:  1000,
		MaxRiskUSDT:      20,
		Journal:          "data/journal.jsonl",
		Setups:           []string{strategy.TrendPullback, strategy.TrendBreakout},
		FeeRate:          0.0005,
	}
}

// testHosts are the hosts orders may be sent to. Loopback is for tests.
var testHosts = map[string]bool{
	"testnet.binancefuture.com": true,
	"demo-fapi.binance.com":     true,
	"127.0.0.1":                 true,
	"localhost":                 true,
}

// LoadConfig reads path over the defaults and resolves the journal path
// against the file's folder. An unknown key is an error, so a misspelled
// limit can't quietly fall back to its default.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) { // EOF: an empty file, all defaults
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	for i, p := range cfg.Pairs {
		cfg.Pairs[i] = PairSymbol(p)
	}
	for i, p := range cfg.MT5.Pairs {
		cfg.MT5.Pairs[i] = strings.TrimSpace(p) // as the broker spells it
	}
	cfg.Pairs = append(cfg.Pairs, cfg.MT5.Pairs...)
	if len(cfg.Pairs) > 0 {
		cfg.Symbol = cfg.Pairs[0]
	}
	if !filepath.IsAbs(cfg.Journal) {
		cfg.Journal = filepath.Join(filepath.Dir(path), cfg.Journal)
	}
	if cfg.MT5.Bridge != "" && !filepath.IsAbs(cfg.MT5.Bridge) {
		cfg.MT5.Bridge = filepath.Join(filepath.Dir(path), cfg.MT5.Bridge)
	}
	return cfg, cfg.Validate()
}

// Trades reports whether symbol is one of the pairs.
func (c Config) Trades(symbol string) bool { return slices.Contains(c.Pairs, symbol) }

// OnMT5 reports whether symbol trades on MetaTrader 5.
func (c Config) OnMT5(symbol string) bool { return slices.Contains(c.MT5.Pairs, symbol) }

// BinancePairs are the pairs that trade on Binance.
func (c Config) BinancePairs() []string {
	return slices.DeleteFunc(slices.Clone(c.Pairs), c.OnMT5)
}

// Resolve finds the pair a name means (btc/usdt, xauusd), as configured.
func (c Config) Resolve(name string) (string, bool) {
	want := PairSymbol(name)
	for _, p := range c.Pairs {
		if PairSymbol(p) == want {
			return p, true
		}
	}
	return "", false
}

// PairSymbol turns a pair as written (btc/usdt, btcusdt) into the
// exchange's symbol, BTCUSDT.
func PairSymbol(pair string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(pair), "/", ""))
}

var (
	symbolPattern = regexp.MustCompile(`^[A-Z0-9]{5,20}$`)
	// mt5Pattern allows a broker's suffixes, such as XAUUSD.m or XAUUSDc.
	mt5Pattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._#+-]{1,29}$`)
)

func (c Config) Validate() error {
	u, err := url.Parse(c.TradeURL)
	if err != nil || !testHosts[u.Hostname()] {
		return fmt.Errorf("trade_url %q is not a Binance test network; only %s or %s are allowed", c.TradeURL, binance.TestnetURL, binance.DemoURL)
	}
	seen := map[string]bool{}
	for _, p := range c.Pairs {
		if c.OnMT5(p) {
			if !mt5Pattern.MatchString(p) || seen[PairSymbol(p)] {
				return fmt.Errorf("mt5.pairs: %q is not a MetaTrader symbol such as XAUUSD, or is listed twice", p)
			}
		} else if !symbolPattern.MatchString(p) || seen[p] {
			return fmt.Errorf("pairs: %q is not a symbol such as BTCUSDT, or is listed twice", p)
		}
		seen[PairSymbol(p)] = true
	}
	if m := c.MT5; len(m.Pairs) > 0 {
		switch {
		case m.Leverage < 1 || m.Leverage > 3000:
			return fmt.Errorf("mt5.leverage %d is out of range: set it to the account's leverage (100 for 1:100)", m.Leverage)
		case m.MaxPosition <= 0:
			return fmt.Errorf("mt5.max_position must be positive: the largest notional of a new MetaTrader position, in the account currency")
		case m.MaxRisk < 0:
			return fmt.Errorf("mt5.max_risk must not be negative (0 = no cap)")
		}
	}
	switch {
	case len(c.Pairs) == 0:
		return fmt.Errorf("pairs is empty: list the pairs to trade, e.g. [BTCUSDT]")
	case !c.Trades(c.Symbol):
		return fmt.Errorf("%s is not one of the pairs (%s)", c.Symbol, strings.Join(c.Pairs, ", "))
	case c.Leverage < 1 || c.Leverage > 125:
		return fmt.Errorf("leverage %d is out of range", c.Leverage)
	case c.MaxPositionUSDT <= 0:
		return fmt.Errorf("max_position_usdt must be positive")
	case c.Candles < 50 || c.Candles > 1500:
		return fmt.Errorf("candles must be between 50 and 1500")
	case c.FeeRate < 0 || c.FeeRate > 0.01:
		return fmt.Errorf("fee_rate %v is out of range", c.FeeRate)
	case len(c.Setups) > 0 && len(c.ContextIntervals) != 2:
		return fmt.Errorf("the setups need exactly two context_intervals (mid and high), not %d", len(c.ContextIntervals))
	}
	for _, s := range c.Setups {
		if !strategy.Known(s) {
			return fmt.Errorf("unknown setup %q (known: %s)", s, strings.Join(strategy.AllSetups, ", "))
		}
	}
	return nil
}
