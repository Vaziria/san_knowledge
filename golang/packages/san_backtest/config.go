package san_backtest

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Config is one backtest of one strategy on one instrument, set up in code
// (the user, 2026-11-14: "I mean configuration in code"). Zero values mean
// the defaults.
type Config struct {
	Strategy   Strategy
	Instrument string     // e.g. "XAUUSDm" on Exness
	Data       DataSource // e.g. NewExness(client) or &CSV{Dir: ...}
	// From is the first candle's open time; To (exclusive, zero = now) the
	// last candle's close time at the latest. Last(d) gives both.
	From, To time.Time
	// Strict: history that starts after From (too few warm-up candles
	// before it) is an error. Without it the run starts where the history
	// allows and Result.Range says so.
	Strict bool

	Balance  float64 // starting balance, default 1000
	Currency string  // account currency, default "USD"
	Risk     float64 // percent of the balance risked per trade, default 1
	Leverage int     // default 200
	Costs    Costs   // nil fields: Exness's real costs
	// DayTZ is where a day starts for MaxDailyLoss (default UTC).
	DayTZ *time.Location
	// MaxDailyLoss: no new trades for the rest of the day once equity is
	// this percent below the day's starting equity. 0 = off.
	MaxDailyLoss float64
	// Walk runs walk-forward windows inside From-To besides the full run.
	Walk *Walk
	// Params override the strategy's Params() defaults.
	Params map[string]float64
	// Cache is the folder for downloaded candles (default
	// "./backtest_cache"; "off" keeps nothing). Only remote sources
	// (Cacheable, like Exness) are cached.
	Cache string
	// Timeframes override the strategy's own Timeframes() (CLI --tf); the
	// first is the step.
	Timeframes []string
}

// Costs overrides Exness's real costs; a nil field keeps the real one.
type Costs struct {
	Spread     *float64    // fixed spread in price (0.20 = 20 cents on XAUUSD), instead of each candle's ask - bid
	Commission *float64    // per lot per side, account currency
	Swap       *[2]float64 // long, short: per lot per night, account currency (negative = charge)
	Slippage   float64     // price added against you on every market fill (entries, closes, stops)
}

// Walk is walk-forward testing: Train tunes the strategy's Params() on a
// window, Test checks the pick on the next window, then both roll on by
// Test.
type Walk struct{ Train, Test time.Duration }

// Portfolio runs several legs on one account (the user, 2026-11-14: "Can
// we have simulated portfolio too"): one shared balance and margin, one
// clock, and account limits. A single Run is a one-leg portfolio.
type Portfolio struct {
	Data         DataSource
	From, To     time.Time
	Strict       bool
	Balance      float64
	Currency     string
	Leverage     int
	MaxOpen      int     // open positions at once, all legs; 0 = no limit
	MaxOpenRisk  float64 // percent of the balance at risk in open positions (to their stops); 0 = no limit
	MaxDailyLoss float64 // as Config.MaxDailyLoss
	DayTZ        *time.Location
	Costs        Costs // for legs without their own
	Cache        string
	Legs         []Leg

	walk *Walk // Config.Walk, checked by Validate
}

// Leg is one strategy on one instrument inside a Portfolio.
type Leg struct {
	Strategy   Strategy
	Instrument string
	Risk       float64 // percent per trade, default 1
	Costs      *Costs  // nil: the portfolio's
	Params     map[string]float64
	Timeframes []string // nil: the strategy's
}

// Defaults.
const (
	DefaultBalance  = 1000.0
	DefaultCurrency = "USD"
	DefaultRisk     = 1.0
	DefaultLeverage = 200
	DefaultCache    = "./backtest_cache"
)

// FieldError is a Config or Portfolio field that is wrong.
type FieldError struct {
	Field   string // e.g. "Config.Risk", "Portfolio.Legs[1].Instrument"
	Problem string
}

func (e *FieldError) Error() string { return "san_backtest: " + e.Field + ": " + e.Problem }

var currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)

// Validate says exactly which fields are wrong (all of them, joined).
func (c Config) Validate() error { return c.portfolio().validate("Config") }

func (c Config) portfolio() Portfolio {
	return Portfolio{Data: c.Data, From: c.From, To: c.To, Strict: c.Strict, Balance: c.Balance, Currency: c.Currency, Leverage: c.Leverage,
		MaxDailyLoss: c.MaxDailyLoss, DayTZ: c.DayTZ, Costs: c.Costs, Cache: c.Cache, walk: c.Walk,
		Legs: []Leg{{Strategy: c.Strategy, Instrument: c.Instrument, Risk: c.Risk, Params: c.Params, Timeframes: c.Timeframes}}}
}

// Validate says exactly which fields are wrong (all of them, joined).
func (p Portfolio) Validate() error { return p.validate("Portfolio") }

func (p Portfolio) validate(root string) error {
	var errs []error
	bad := func(field, format string, a ...any) {
		errs = append(errs, &FieldError{Field: field, Problem: fmt.Sprintf(format, a...)})
	}
	single := root == "Config"
	legField := func(i int, f string) string {
		if single {
			return root + "." + f
		}
		return fmt.Sprintf("%s.Legs[%d].%s", root, i, f)
	}
	if p.Data == nil {
		bad(root+".Data", "is needed (NewExness(client) or &CSV{Dir: ...})")
	}
	if p.From.IsZero() {
		bad(root+".From", "is needed (or use Last(90*24*time.Hour))")
	}
	if !p.To.IsZero() && !p.From.IsZero() && !p.To.After(p.From) {
		bad(root+".To", "must be after From (%s), got %s", p.From.UTC().Format(time.RFC3339), p.To.UTC().Format(time.RFC3339))
	}
	if !p.To.IsZero() && p.To.After(time.Now().Add(time.Minute)) {
		bad(root+".To", "is in the future (%s)", p.To.UTC().Format(time.RFC3339))
	}
	if p.Balance < 0 || math.IsNaN(p.Balance) || math.IsInf(p.Balance, 0) {
		bad(root+".Balance", "must be positive (0 = %v), got %v", DefaultBalance, p.Balance)
	}
	if p.Currency != "" && !currencyRe.MatchString(p.Currency) {
		bad(root+".Currency", "must be three capital letters like USD, got %q", p.Currency)
	}
	if p.Leverage < 0 {
		bad(root+".Leverage", "must be positive (0 = %d), got %d", DefaultLeverage, p.Leverage)
	}
	if p.MaxOpen < 0 {
		bad(root+".MaxOpen", "must be 0 (no limit) or more, got %d", p.MaxOpen)
	}
	if p.MaxOpenRisk < 0 || p.MaxOpenRisk > 100 {
		bad(root+".MaxOpenRisk", "must be a percent from 0 (no limit) to 100, got %v", p.MaxOpenRisk)
	}
	if p.MaxDailyLoss < 0 || p.MaxDailyLoss > 100 {
		bad(root+".MaxDailyLoss", "must be a percent from 0 (off) to 100, got %v", p.MaxDailyLoss)
	}
	checkCosts := func(field string, c Costs) {
		if c.Spread != nil && (*c.Spread < 0 || math.IsNaN(*c.Spread)) {
			bad(field+".Spread", "must be 0 or more, got %v", *c.Spread)
		}
		if c.Commission != nil && (*c.Commission < 0 || math.IsNaN(*c.Commission)) {
			bad(field+".Commission", "must be 0 or more, got %v", *c.Commission)
		}
		if c.Slippage < 0 || math.IsNaN(c.Slippage) {
			bad(field+".Slippage", "must be 0 or more, got %v", c.Slippage)
		}
	}
	checkCosts(root+".Costs", p.Costs)
	if p.walk != nil {
		w := *p.walk
		switch {
		case w.Train <= 0 || w.Test <= 0:
			bad(root+".Walk", "Train and Test must both be positive, got %s:%s", w.Train, w.Test)
		case !p.From.IsZero():
			to := p.To
			if to.IsZero() {
				to = time.Now()
			}
			if w.Train+w.Test > to.Sub(p.From) {
				bad(root+".Walk", "Train + Test (%s) is longer than From-To (%s)", w.Train+w.Test, to.Sub(p.From).Round(time.Hour))
			}
		}
	}
	if len(p.Legs) == 0 {
		bad(root+".Legs", "needs at least one leg")
	}
	for i, l := range p.Legs {
		if l.Strategy == nil {
			bad(legField(i, "Strategy"), "is needed")
		}
		if strings.TrimSpace(l.Instrument) == "" {
			bad(legField(i, "Instrument"), "is needed, e.g. XAUUSDm")
		}
		if l.Risk < 0 || l.Risk > 100 || math.IsNaN(l.Risk) {
			bad(legField(i, "Risk"), "must be a percent from 0 (= %v) to 100, got %v", DefaultRisk, l.Risk)
		}
		if l.Costs != nil {
			checkCosts(legField(i, "Costs"), *l.Costs)
		}
		if l.Strategy == nil {
			continue
		}
		tfs := l.Timeframes
		field := legField(i, "Timeframes")
		if len(tfs) == 0 {
			tfs, field = l.Strategy.Timeframes(), legField(i, "Strategy.Timeframes()")
		}
		if err := checkTimeframes(tfs); err != nil {
			bad(field, "%v", err)
		}
		if l.Strategy.Warmup() < 0 {
			bad(legField(i, "Strategy.Warmup()"), "must be 0 or more, got %d", l.Strategy.Warmup())
		}
		if len(l.Params) > 0 {
			defs := map[string]float64{}
			if t, ok := l.Strategy.(Tunable); ok {
				defs = t.Params()
			}
			var unknown []string
			for k := range l.Params {
				if _, ok := defs[k]; !ok {
					unknown = append(unknown, k)
				}
			}
			sort.Strings(unknown)
			if len(unknown) > 0 {
				bad(legField(i, "Params"), "%s not among the strategy's Params() (%s)", strings.Join(unknown, ", "), strings.Join(sortedKeys(defs), ", "))
			}
		}
	}
	return errors.Join(errs...)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// checkTimeframes: known names, no repeats, none shorter than the first.
func checkTimeframes(tfs []string) error {
	if len(tfs) == 0 {
		return errors.New("needs at least one timeframe")
	}
	seen := map[string]bool{}
	for _, tf := range tfs {
		if _, ok := timeframeDur[tf]; !ok {
			return fmt.Errorf("timeframe %q (one of %s, in capitals)", tf, strings.Join(timeframeOrder, ", "))
		}
		if seen[tf] {
			return fmt.Errorf("timeframe %s twice", tf)
		}
		seen[tf] = true
		if timeframeDur[tf] < timeframeDur[tfs[0]] {
			return fmt.Errorf("%s is shorter than the step timeframe %s (the first)", tf, tfs[0])
		}
	}
	return nil
}

// withDefaults fills the zero values.
func (p Portfolio) withDefaults() Portfolio {
	if p.Balance == 0 {
		p.Balance = DefaultBalance
	}
	if p.Currency == "" {
		p.Currency = DefaultCurrency
	}
	if p.Leverage == 0 {
		p.Leverage = DefaultLeverage
	}
	if p.DayTZ == nil {
		p.DayTZ = time.UTC
	}
	if p.Cache == "" {
		p.Cache = DefaultCache
	}
	if p.To.IsZero() {
		p.To = time.Now().UTC().Truncate(time.Minute)
	}
	p.From, p.To = p.From.UTC(), p.To.UTC()
	legs := make([]Leg, len(p.Legs))
	for i, l := range p.Legs {
		if l.Risk == 0 {
			l.Risk = DefaultRisk
		}
		if len(l.Timeframes) == 0 {
			l.Timeframes = l.Strategy.Timeframes()
		}
		legs[i] = l
	}
	p.Legs = legs
	return p
}
