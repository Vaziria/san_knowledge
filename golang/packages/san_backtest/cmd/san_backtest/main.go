// Command san_backtest runs San's Go approaches over history: Exness
// candles through san_exness (cached on disk) or CSV files. It is a thin
// wrapper over the san_backtest package: each flag maps onto one Config or
// Portfolio field. It never trades; it only reads candles and instrument
// conditions.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/examples"
	exness "github.com/wargasipil/san_exness"
)

var version = "dev"

// Seams for tests: the HTTP client for Exness, and the runners.
var (
	httpClient   *http.Client
	runOne       = bt.Run
	runPortfolio = bt.RunPortfolio
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := app(os.Stdout, registry()).Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "san_backtest:", err)
		os.Exit(1)
	}
}

// registry is every strategy the CLI knows: add your own approaches here.
func registry() *bt.Registry {
	r := bt.NewRegistry()
	examples.Register(r)
	return r
}

// shared are the flags of both run and portfolio.
func shared() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "from", Usage: "first candle's open time, UTC: 2026-08-01 or RFC 3339 (Config.From)"},
		&cli.StringFlag{Name: "to", Usage: "end, UTC, exclusive; empty = now (Config.To)"},
		&cli.StringFlag{Name: "last", Usage: "instead of --from/--to: the last 90d, 12h, 2w, 3m (months), 1y (Last)"},
		&cli.StringFlag{Name: "tz", Value: "UTC", Usage: "where a day starts for --max-daily-loss: UTC, WIB or an IANA zone (Config.DayTZ)"},
		&cli.Float64Flag{Name: "balance", Value: bt.DefaultBalance, Usage: "starting balance (Config.Balance)"},
		&cli.StringFlag{Name: "currency", Value: bt.DefaultCurrency, Usage: "account currency (Config.Currency)"},
		&cli.IntFlag{Name: "leverage", Value: bt.DefaultLeverage, Usage: "account leverage, 200 = 1:200 (Config.Leverage)"},
		&cli.Float64Flag{Name: "spread", Usage: "fixed spread in price instead of each candle's ask - bid (Costs.Spread)"},
		&cli.Float64Flag{Name: "commission", Usage: "commission per lot per side, account currency, instead of Exness's (Costs.Commission)"},
		&cli.StringFlag{Name: "swap", Usage: "LONG,SHORT swap per lot per night, account currency, instead of Exness's (Costs.Swap)"},
		&cli.Float64Flag{Name: "slippage", Usage: "price added against you on every market fill (Costs.Slippage)"},
		&cli.Float64Flag{Name: "max-daily-loss", Usage: "percent below the day's starting equity that stops new trades for the day; 0 = off (MaxDailyLoss)"},
		&cli.BoolFlag{Name: "strict", Usage: "fail when history starts after --from (Config.Strict)"},
		&cli.StringFlag{Name: "cache", Value: bt.DefaultCache, Usage: `folder for downloaded Exness candles; "off" keeps none (Config.Cache)`},
		&cli.StringFlag{Name: "csv", Usage: "read candles from CSV files in this folder instead of Exness (see README)"},
		&cli.StringFlag{Name: "out", Usage: "write summary.txt, trades.csv, missed.csv, equity.csv, equity.png (and walk.csv) here"},
	}
}

func app(out io.Writer, reg *bt.Registry) *cli.Command {
	exness.Version = version
	return &cli.Command{
		Name:    "san_backtest",
		Usage:   "backtest Go approaches on Exness (or CSV) candles; reads only, never trades",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "api-key", Usage: "Exness API key identifier (exnsk-...)", Sources: cli.EnvVars("EXN_API_KEY")},
			&cli.StringFlag{Name: "private-key-file", Usage: "file with the private key; or set EXN_PRIVATE_KEY", Sources: cli.EnvVars("EXN_PRIVATE_KEY_FILE")},
			&cli.StringFlag{Name: "account", Usage: "trading account id", Sources: cli.EnvVars("EXN_ACCOUNT_ID")},
			&cli.StringFlag{Name: "gateway", Usage: "Gateway host from Key Management's DNS widget (https://...)", Sources: cli.EnvVars("EXN_DISCOVERY_BASE_URL")},
			&cli.StringFlag{Name: "access-point", Usage: "the account's Access Point, to skip discovery", Sources: cli.EnvVars("EXN_API_BASE_URL")},
		},
		Commands: []*cli.Command{
			{
				Name:      "run",
				Usage:     "backtest one strategy, or several (comma-separated) side by side",
				ArgsUsage: "NAME[,NAME...]",
				Flags: append(shared(),
					&cli.StringFlag{Name: "instrument", Usage: "e.g. XAUUSDm (Config.Instrument)", Required: true},
					&cli.StringFlag{Name: "tf", Usage: "timeframes, step first, e.g. M15,H1,H4; empty = the strategy's (Config.Timeframes)"},
					&cli.Float64Flag{Name: "risk", Value: bt.DefaultRisk, Usage: "percent of the balance risked per trade (Config.Risk)"},
					&cli.StringFlag{Name: "walk", Usage: "walk-forward TRAIN:TEST, e.g. 3m:1m (Config.Walk)"},
					&cli.StringSliceFlag{Name: "param", Usage: "NAME=VALUE overriding a strategy number, repeatable (Config.Params)"},
				),
				Action: func(ctx context.Context, cmd *cli.Command) error { return runCmd(ctx, cmd, out, reg) },
			},
			{
				Name:  "portfolio",
				Usage: "backtest several legs on one shared account",
				Flags: append(shared(),
					&cli.StringSliceFlag{Name: "leg", Usage: "NAME:INSTRUMENT[:RISK], repeatable (Portfolio.Legs)", Required: true},
					&cli.IntFlag{Name: "max-open", Usage: "open positions at once, all legs; 0 = no limit (Portfolio.MaxOpen)"},
					&cli.Float64Flag{Name: "max-open-risk", Usage: "percent of the balance at risk in open positions; 0 = no limit (Portfolio.MaxOpenRisk)"},
				),
				Action: func(ctx context.Context, cmd *cli.Command) error { return portfolioCmd(ctx, cmd, out, reg) },
			},
			{
				Name:  "list",
				Usage: "list the strategies: timeframes, warm-up, params",
				Action: func(_ context.Context, _ *cli.Command) error {
					for _, n := range reg.Names() {
						s, _ := reg.New(n)
						line := fmt.Sprintf("%s\ttimeframes %s\twarmup %d", n, strings.Join(s.Timeframes(), ","), s.Warmup())
						if t, ok := s.(bt.Tunable); ok {
							var p []string
							for k, v := range t.Params() {
								p = append(p, fmt.Sprintf("%s=%g", k, v))
							}
							sort.Strings(p)
							line += "\tparams " + strings.Join(p, " ")
						}
						fmt.Fprintln(out, line)
					}
					return nil
				},
			},
		},
	}
}

// common holds the flags both commands share, parsed.
type common struct {
	from, to time.Time
	tz       *time.Location
	costs    bt.Costs
	data     bt.DataSource
}

func parseWhen(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("time %q (2026-08-01, 2026-08-01T10:00 or RFC 3339, UTC)", s)
}

func parseCommon(cmd *cli.Command) (common, error) {
	var c common
	var err error
	if last := cmd.String("last"); last != "" {
		if cmd.String("from") != "" || cmd.String("to") != "" {
			return c, errors.New("--last goes without --from and --to")
		}
		d, err := bt.ParseSpan(last)
		if err != nil {
			return c, fmt.Errorf("--last: %w", err)
		}
		c.from, c.to = bt.Last(d)
	} else {
		if cmd.String("from") == "" {
			return c, errors.New("--from or --last is needed")
		}
		if c.from, err = parseWhen(cmd.String("from")); err != nil {
			return c, fmt.Errorf("--from: %w", err)
		}
		if s := cmd.String("to"); s != "" {
			if c.to, err = parseWhen(s); err != nil {
				return c, fmt.Errorf("--to: %w", err)
			}
		}
	}
	if c.tz, err = bt.ParseZone(cmd.String("tz")); err != nil {
		return c, fmt.Errorf("--tz: %w", err)
	}
	if cmd.IsSet("spread") {
		v := cmd.Float64("spread")
		c.costs.Spread = &v
	}
	if cmd.IsSet("commission") {
		v := cmd.Float64("commission")
		c.costs.Commission = &v
	}
	if s := cmd.String("swap"); s != "" {
		parts := strings.Split(s, ",")
		if len(parts) != 2 {
			return c, fmt.Errorf("--swap %q: LONG,SHORT like -7.5,2.1", s)
		}
		var v [2]float64
		for i, p := range parts {
			if v[i], err = strconv.ParseFloat(strings.TrimSpace(p), 64); err != nil {
				return c, fmt.Errorf("--swap %q: LONG,SHORT like -7.5,2.1", s)
			}
		}
		c.costs.Swap = &v
	}
	c.costs.Slippage = cmd.Float64("slippage")
	if dir := cmd.String("csv"); dir != "" {
		c.data = &bt.CSV{Dir: dir}
	}
	return c, nil
}

// source is the CSV source, or Exness through san_exness with the global
// flags (made only when needed, so CSV runs need no key).
func source(cmd *cli.Command, c common) (bt.DataSource, error) {
	if c.data != nil {
		return c.data, nil
	}
	key := os.Getenv("EXN_PRIVATE_KEY")
	if path := cmd.String("private-key-file"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("private key file: %w", err)
		}
		key = string(b)
	}
	if key == "" {
		return nil, errors.New("no private key: set EXN_PRIVATE_KEY or --private-key-file (or use --csv)")
	}
	client, err := exness.New(exness.Config{APIKey: cmd.String("api-key"), PrivateKey: key, AccountID: cmd.String("account"),
		Gateway: cmd.String("gateway"), AccessPoint: cmd.String("access-point"), HTTPClient: httpClient})
	if err != nil {
		return nil, err
	}
	return bt.NewExness(client), nil
}

func parseParams(list []string) (map[string]float64, error) {
	if len(list) == 0 {
		return nil, nil
	}
	m := map[string]float64{}
	for _, kv := range list {
		k, v, ok := strings.Cut(kv, "=")
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if !ok || err != nil || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("--param %q: NAME=VALUE like fast=20", kv)
		}
		m[strings.TrimSpace(k)] = f
	}
	return m, nil
}

// configs builds one Config per strategy name from the run flags.
func configs(cmd *cli.Command, reg *bt.Registry) ([]bt.Config, error) {
	if cmd.Args().Len() != 1 {
		return nil, errors.New("name one strategy, or several joined by commas (see list)")
	}
	c, err := parseCommon(cmd)
	if err != nil {
		return nil, err
	}
	var tfs []string
	if s := cmd.String("tf"); s != "" {
		for _, x := range strings.Split(s, ",") {
			tf, err := bt.ParseTimeframe(x)
			if err != nil {
				return nil, fmt.Errorf("--tf: %w", err)
			}
			tfs = append(tfs, tf)
		}
	}
	var walk *bt.Walk
	if s := cmd.String("walk"); s != "" {
		a, b, ok := strings.Cut(s, ":")
		train, err1 := bt.ParseSpan(a)
		test, err2 := bt.ParseSpan(b)
		if !ok || err1 != nil || err2 != nil {
			return nil, fmt.Errorf("--walk %q: TRAIN:TEST like 3m:1m", s)
		}
		walk = &bt.Walk{Train: train, Test: test}
	}
	params, err := parseParams(cmd.StringSlice("param"))
	if err != nil {
		return nil, err
	}
	var out []bt.Config
	for _, name := range strings.Split(cmd.Args().First(), ",") {
		st, err := reg.New(strings.TrimSpace(name))
		if err != nil {
			return nil, err
		}
		out = append(out, bt.Config{Strategy: st, Instrument: cmd.String("instrument"), Data: c.data, From: c.from, To: c.to, Strict: cmd.Bool("strict"),
			Balance: cmd.Float64("balance"), Currency: cmd.String("currency"), Risk: cmd.Float64("risk"), Leverage: cmd.Int("leverage"),
			Costs: c.costs, DayTZ: c.tz, MaxDailyLoss: cmd.Float64("max-daily-loss"), Walk: walk, Params: params, Cache: cmd.String("cache"),
			Timeframes: tfs})
	}
	return out, nil
}

func runCmd(ctx context.Context, cmd *cli.Command, out io.Writer, reg *bt.Registry) error {
	cfgs, err := configs(cmd, reg)
	if err != nil {
		return err
	}
	if cfgs[0].Data == nil {
		c, _ := parseCommon(cmd)
		src, err := source(cmd, c)
		if err != nil {
			return err
		}
		for i := range cfgs {
			cfgs[i].Data = src
		}
	}
	var results []*bt.Result
	for i := range cfgs {
		res, err := runOne(ctx, cfgs[i])
		if err != nil {
			return fmt.Errorf("%s: %w", cfgs[i].Strategy.Name(), err)
		}
		results = append(results, res)
		fmt.Fprintln(out, res.Summary())
		if dir := cmd.String("out"); dir != "" {
			if len(cfgs) > 1 {
				dir = filepath.Join(dir, cfgs[i].Strategy.Name())
			}
			paths, err := res.Save(dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "wrote %s\n\n", strings.Join(paths, ", "))
		}
	}
	if len(results) > 1 {
		fmt.Fprintln(out, bt.Compare(results...))
	}
	return nil
}

// portfolioFrom builds the Portfolio from the portfolio flags.
func portfolioFrom(cmd *cli.Command, reg *bt.Registry) (bt.Portfolio, error) {
	c, err := parseCommon(cmd)
	if err != nil {
		return bt.Portfolio{}, err
	}
	p := bt.Portfolio{From: c.from, To: c.to, Strict: cmd.Bool("strict"), Balance: cmd.Float64("balance"), Currency: cmd.String("currency"),
		Leverage: cmd.Int("leverage"), MaxOpen: cmd.Int("max-open"), MaxOpenRisk: cmd.Float64("max-open-risk"), MaxDailyLoss: cmd.Float64("max-daily-loss"),
		DayTZ: c.tz, Costs: c.costs, Cache: cmd.String("cache"), Data: c.data}
	for _, s := range cmd.StringSlice("leg") {
		parts := strings.Split(s, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return p, fmt.Errorf("--leg %q: NAME:INSTRUMENT[:RISK]", s)
		}
		st, err := reg.New(parts[0])
		if err != nil {
			return p, err
		}
		leg := bt.Leg{Strategy: st, Instrument: parts[1]}
		if len(parts) == 3 {
			if leg.Risk, err = strconv.ParseFloat(parts[2], 64); err != nil {
				return p, fmt.Errorf("--leg %q: risk %q", s, parts[2])
			}
		}
		p.Legs = append(p.Legs, leg)
	}
	return p, nil
}

func portfolioCmd(ctx context.Context, cmd *cli.Command, out io.Writer, reg *bt.Registry) error {
	p, err := portfolioFrom(cmd, reg)
	if err != nil {
		return err
	}
	if p.Data == nil {
		c, _ := parseCommon(cmd)
		if p.Data, err = source(cmd, c); err != nil {
			return err
		}
	}
	res, err := runPortfolio(ctx, p)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, res.Summary())
	if dir := cmd.String("out"); dir != "" {
		paths, err := res.Save(dir)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s\n", strings.Join(paths, ", "))
	}
	return nil
}
