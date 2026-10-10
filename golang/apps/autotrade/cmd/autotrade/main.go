// Command autotrade is the hands of the trading loop in docs/basic_flow.md. Claude,
// running /loop, is the head: each tick it reads `autotrade snapshot` and,
// for each pair in autotrade.yaml, decides and runs exactly one of open,
// close, protect or hold.
//
//	autotrade snapshot
//	autotrade -symbol BTCUSDT open long -approach big_trend,positioning -usdt 500 -sl 81900 -tp 83800 -reason "..."
//	autotrade -symbol BTCUSDT protect -sl 82450 -reason "..."
//	autotrade -symbol ETHUSDT close -reason "..."
//	autotrade -symbol BNBUSDT hold -reason "..."
//	autotrade journal -n 20
//	autotrade check
//	autotrade knowledge approaches|approach NAME|positions|position N|portfolio|query CYPHER|sync
//	autotrade mcp run
//	autotrade knowledge view
//
// Orders only go to the Binance test network, and for the mt5 pairs to a
// MetaTrader 5 demo account through the AutotradeBridge EA (internal/mt5).
// Settings and limits are in autotrade.yaml; the API key is read from
// BINANCE_TESTNET_API_KEY and BINANCE_TESTNET_API_SECRET, or from a .env
// file next to autotrade.yaml.
package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wargasipil/autotrade/internal/binance"
	"github.com/wargasipil/autotrade/internal/bot"
	"github.com/wargasipil/autotrade/internal/mcp"
	"github.com/wargasipil/autotrade/internal/mt5"
	"github.com/wargasipil/autotrade/internal/strategy"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `autotrade: the trading loop's hands (Binance USD-M futures, test network;
MetaTrader 5 demo for the mt5 pairs)

usage: autotrade [-config FILE] [-symbol PAIR] [-json] COMMAND [flags]

Each pair in autotrade.yaml's pairs and mt5.pairs is traded on its own. snapshot
covers every pair, or the -symbol one; open, protect, close and hold act on
-symbol, which is required when there are several pairs.

commands:
  snapshot [-recent 12]                 per pair: market, indicators, levels, positioning,
                                        signals, portfolio, last decisions
  open long|short -approach A[,B] -usdt N -sl PRICE [-tp PRICE] -reason TEXT
                                        approaches: catalog names or coded setups
  protect [-sl PRICE] [-tp PRICE] -reason TEXT
                                        move the stop-loss (tighten only) / take-profit
  close -reason TEXT                    close the position at market
  hold -reason TEXT                     record a decision to do nothing
  journal [-n 20]                       last decisions
  check                                 key, clock, position mode, balance, each pair,
                                        and the MetaTrader account with its pairs
  mt5 install                           copy the AutotradeBridge EA into each MT5
                                        terminal's MQL5/Experts and compile it
  review                                score the journal's trades with the exchange's PnL
  discover [-days 365] [-setups a,b] [-fee 0.0005]
                                        test the candidate setups on every pair, with pass criteria
  backtest [-days 365] [-setups a,b] [-fee 0.0005] [-trades]
                                        replay the coded setups over past candles
                                        (of -symbol, else the first pair)

knowledge graph (docs/knowledge.md):
  approach [-name N -title T -description D]
                                        define an approach; without flags, list them
  note -position N -text TEXT           the summary and lesson of a position
  knowledge view [-addr 127.0.0.1:7475] [-no-open]
                                        preview the knowledge graph in the browser
  knowledge approaches | approach NAME | pair SYMBOL | positions [-pair P -status S -approach A -outcome O]
          | position N [-snapshot] | portfolio | query CYPHER | sync
  mcp run                               serve trading and knowledge over MCP (stdio)
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "autotrade:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	global := flag.NewFlagSet("autotrade", flag.ContinueOnError)
	global.SetOutput(stderr)
	global.Usage = func() { fmt.Fprint(stderr, usage) }
	configPath := global.String("config", "", "config file (default: ./autotrade.yaml, then next to the binary's folder)")
	symbol := global.String("symbol", "", "the pair to act on (BTCUSDT): one of autotrade.yaml's pairs")
	asJSON := global.Bool("json", false, "print JSON")
	if err := global.Parse(args); err != nil {
		return err
	}
	if global.NArg() == 0 {
		global.Usage()
		return errors.New("no command")
	}
	cmd, rest := global.Arg(0), global.Args()[1:]

	path, err := findConfig(*configPath)
	if err != nil {
		return err
	}
	cfg, err := bot.LoadConfig(path)
	if err != nil {
		return err
	}
	key, secret := credentials(filepath.Dir(path))
	market := binance.New(cfg.MarketURL, "", "")
	b := &bot.Bot{
		Cfg:     cfg,
		Market:  market,
		Ex:      binance.New(cfg.TradeURL, key, secret),
		Journal: bot.Journal{Path: cfg.Journal},
	}
	var mc *mt5.Client
	if len(cfg.MT5.Pairs) > 0 {
		// Binance's corrected clock serves MetaTrader too.
		mc = mt5.New(cfg.MT5.Bridge, market)
		b.MT5 = &bot.Venue{Name: "MetaTrader 5 demo", Market: mc, Ex: mc}
	}

	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	reason := fs.String("reason", "", "why: the signal and the thinking behind the decision")
	usdt := fs.Float64("usdt", 0, "position size in USDT (notional)")
	sl := fs.Float64("sl", 0, "stop-loss trigger price (mark price)")
	tp := fs.Float64("tp", 0, "take-profit trigger price (mark price)")
	limit := fs.Float64("limit", 0, "open: limit price for a post-only maker entry (default: at market)")
	expires := fs.Duration("expires", bot.DefaultExpiry, "open: how long an unfilled limit entry waits")
	approachFlag := fs.String("approach", "", "open: approaches the analysis used, comma-separated: catalog names or coded setups ("+strings.Join(strategy.AllSetups, ", ")+")")
	setup := fs.String("setup", "", "open: same as -approach (older name)")
	name := fs.String("name", "", "approach: its short lowercase name")
	title := fs.String("title", "", "approach: its readable title")
	description := fs.String("description", "", "approach: what it reads and when it says to act")
	position := fs.Int("position", 0, "note: position number (1 = the first open)")
	text := fs.String("text", "", "note: the summary and lesson")
	status := fs.String("status", "", "knowledge positions: open or closed")
	outcome := fs.String("outcome", "", "knowledge positions: profit or loss")
	pair := fs.String("pair", "", "knowledge positions: only this pair (BTCUSDT)")
	withSnapshot := fs.Bool("snapshot", false, "knowledge position: include the full data snapshot")
	recent := fs.Int("recent", 12, "candles to list")
	days := fs.Int("days", 365, "backtest: days of history")
	setups := fs.String("setups", strings.Join(strategy.AllSetups, ","), "backtest: setups to test, comma-separated")
	fee := fs.Float64("fee", cfg.FeeRate, "backtest: fee per fill (0.0005 = 0.05%)")
	listTrades := fs.Bool("trades", false, "backtest: list every trade")
	n := fs.Int("n", 20, "entries")

	addr := fs.String("addr", "127.0.0.1:7475", "knowledge view: address to serve on")
	noOpen := fs.Bool("no-open", false, "knowledge view: don't open the browser")

	words, err := parseInterleaved(fs, rest)
	if err != nil {
		return err
	}
	arg := func(i int) string {
		if i < len(words) {
			return words[i]
		}
		return ""
	}
	var side string
	if cmd == "open" {
		side = arg(0)
	}

	// act is the bot of the -symbol pair, for the commands that act on one.
	act := b
	switch cmd {
	case "open", "close", "protect", "hold":
		if act, err = b.ForPair(*symbol); err != nil {
			return err
		}
	}
	srv := &mcp.Server{Bot: b, Version: version, Log: stderr}
	// sync brings the knowledge graph up to date after a command that changes
	// what it is built from. A failure is reported, not returned.
	sync := func() {
		if _, err := b.SyncKnowledge(ctx); err != nil {
			fmt.Fprintln(stderr, "autotrade: knowledge sync:", err)
		}
	}
	tool := func(name string, args map[string]any) error {
		raw, _ := json.Marshal(args)
		out, err := srv.Call(ctx, name, raw)
		if out != "" {
			fmt.Fprintln(stdout, strings.TrimRight(out, "\n"))
		}
		return err
	}
	entry := func(e bot.Entry, err error) error {
		if e.Time.IsZero() { // refused before anything was recorded
			return err
		}
		sync()
		if *asJSON {
			writeJSON(stdout, e)
		} else {
			fmt.Fprintln(stdout, e.Line(-1))
		}
		return err
	}

	switch cmd {
	case "snapshot":
		pairs := cfg.Pairs
		if *symbol != "" {
			pairs = []string{*symbol}
		}
		// One pair's failure (MetaTrader closed) leaves the others' snapshots.
		var snaps []*bot.Snapshot
		var failed []string
		for _, p := range pairs {
			pb, err := b.ForPair(p)
			if err != nil {
				return err
			}
			s, err := pb.Snapshot(ctx, *recent)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", pb.Cfg.Symbol, err))
				continue
			}
			snaps = append(snaps, s)
		}
		defer sync()
		var failure error
		if len(failed) > 0 {
			failure = fmt.Errorf("no snapshot of %s", strings.Join(failed, "; "))
		}
		switch {
		case *asJSON && len(snaps) == 1 && len(pairs) == 1:
			writeJSON(stdout, snaps[0])
			return failure
		case *asJSON:
			writeJSON(stdout, snaps)
			return failure
		}
		for i, s := range snaps {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			s.Render(stdout)
		}
		return failure
	case "open":
		var approaches []string
		for _, a := range strings.Split(*approachFlag+","+*setup, ",") {
			if a = strings.TrimSpace(a); a != "" {
				approaches = append(approaches, a)
			}
		}
		if *limit > 0 {
			return entry(act.OpenLimit(ctx, strings.ToLower(side), approaches, *usdt, *limit, *sl, *tp, *expires, *reason))
		}
		return entry(act.Open(ctx, strings.ToLower(side), approaches, *usdt, *sl, *tp, *reason))
	case "close":
		return entry(act.Close(ctx, *reason))
	case "protect":
		return entry(act.Protect(ctx, *sl, *tp, *reason))
	case "hold":
		e, err := act.Hold(ctx, *reason)
		if e.Time.IsZero() {
			return err
		}
		fmt.Fprintln(stdout, e.Line(-1))
		return err
	case "note":
		return entry(b.Note(ctx, *position, *text))
	case "approach":
		if *name == "" {
			return tool("autotrade_approaches", nil)
		}
		return tool("autotrade_approach_define", map[string]any{"name": *name, "title": *title, "description": *description})
	case "knowledge":
		sub, arg := arg(0), arg(1)
		switch sub {
		case "view":
			return serveView(ctx, stdout, stderr, b, *addr, !*noOpen)
		case "sync":
			return tool("autotrade_knowledge_sync", nil)
		case "portfolio":
			return tool("autotrade_portfolio", nil)
		case "approaches":
			return tool("autotrade_approaches", nil)
		case "approach":
			return tool("autotrade_approach", map[string]any{"name": arg})
		case "positions":
			return tool("autotrade_positions", map[string]any{"pair": strings.ToUpper(*pair), "status": *status, "approach": *approachFlag, "outcome": *outcome})
		case "pair":
			return tool("autotrade_pair", map[string]any{"pair": strings.ToUpper(arg)})
		case "position":
			n, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("knowledge position needs a number, not %q", arg)
			}
			return tool("autotrade_position", map[string]any{"n": n, "include_snapshot": *withSnapshot})
		case "query":
			return tool("autotrade_query", map[string]any{"cypher": strings.Join(words[1:], " ")})
		}
		return fmt.Errorf("knowledge needs one of: view, approaches, approach NAME, pair SYMBOL, positions, position N, portfolio, query CYPHER, sync")
	case "mcp":
		// "mcp run" (docs/readme.md); a bare "mcp" also serves, for configs
		// written before.
		if w := arg(0); w != "" && w != "run" {
			return fmt.Errorf("unknown mcp command %q: use `autotrade mcp run`", w)
		}
		fmt.Fprintf(stderr, "autotrade mcp: serving %s on stdio\n", strings.Join(b.Cfg.Pairs, ", "))
		return srv.Serve(ctx, os.Stdin, stdout)
	case "journal":
		es, err := b.Journal.Last(*n)
		if err != nil {
			return err
		}
		if *asJSON {
			return writeJSON(stdout, es)
		}
		if len(es) == 0 {
			fmt.Fprintln(stdout, "no decisions yet")
		}
		for _, e := range es {
			fmt.Fprintln(stdout, e.Line(-1))
		}
		return nil
	case "review":
		r, err := b.Review(ctx)
		if err != nil {
			return err
		}
		if *asJSON {
			return writeJSON(stdout, r)
		}
		r.Render(stdout)
		return nil
	case "check":
		err := check(ctx, stdout, cfg, path, b.Ex.(*binance.Client))
		if mc != nil {
			err = errors.Join(err, checkMT5(ctx, stdout, cfg, mc))
		}
		return err
	case "mt5":
		if arg(0) != "install" {
			return errors.New("mt5 needs a command: install")
		}
		return installMT5(stdout, cmp.Or(cfg.MT5.Bridge, mt5.DefaultDir()))
	case "backtest":
		var names []string
		for _, n := range strings.Split(*setups, ",") {
			if n = strings.TrimSpace(n); n != "" {
				if !strategy.Known(n) {
					return fmt.Errorf("unknown setup %q", n)
				}
				names = append(names, n)
			}
		}
		if len(names) == 0 || *days < 1 {
			return errors.New("backtest needs at least one setup and one day")
		}
		bb, err := b.ForPair(cmp.Or(*symbol, cfg.Pairs[0]))
		if err != nil {
			return err
		}
		if cfg.OnMT5(bb.Cfg.Symbol) {
			return fmt.Errorf("the backtest replays Binance candles; %s trades on MetaTrader", bb.Cfg.Symbol)
		}
		return runBacktest(ctx, stdout, bb.Cfg, b.Market.(*binance.Client), backtestOpts{days: *days, setups: names, fee: *fee, trades: *listTrades, json: *asJSON})
	case "discover":
		names := strategy.Candidates
		set := false
		fs.Visit(func(f *flag.Flag) { set = set || f.Name == "setups" })
		if set {
			names = nil
			for _, n := range strings.Split(*setups, ",") {
				if n = strings.TrimSpace(n); n != "" {
					if !strategy.Known(n) {
						return fmt.Errorf("unknown setup %q", n)
					}
					names = append(names, n)
				}
			}
		}
		if len(names) == 0 || *days < 1 {
			return errors.New("discover needs at least one setup and one day")
		}
		bcfg := cfg // discover replays Binance candles: the Binance pairs only
		bcfg.Pairs = cfg.BinancePairs()
		return runDiscover(ctx, stdout, bcfg, b.Market.(*binance.Client), *days, names, *fee)
	}
	global.Usage()
	return fmt.Errorf("unknown command %q", cmd)
}

func check(ctx context.Context, w io.Writer, cfg bot.Config, path string, ex *binance.Client) error {
	fmt.Fprintf(w, "config   %s\npairs    %s on %s\ntrading  %s\nmarket   %s\njournal  %s\n", path, strings.Join(cfg.BinancePairs(), ", "), cfg.Interval, cfg.TradeURL, cfg.MarketURL, cfg.Journal)
	if err := ex.SyncTime(ctx); err != nil {
		return err
	}
	off := ex.Offset()
	fmt.Fprintf(w, "clock    local clock is %s %s the exchange (corrected automatically)\n", off.Abs().Round(1e6), map[bool]string{true: "behind", false: "ahead of"}[off > 0])
	if !ex.HasKey() {
		return errors.New("no API key: set BINANCE_TESTNET_API_KEY and BINANCE_TESTNET_API_SECRET, or put them in .env next to autotrade.yaml")
	}
	dual, err := ex.DualSide(ctx)
	if err != nil {
		return fmt.Errorf("key check: %w", err)
	}
	acc, err := ex.Account(ctx)
	if err != nil {
		return err
	}
	mode := "one-way"
	if dual {
		mode = "HEDGE (switch to one-way mode before trading)"
	}
	fmt.Fprintf(w, "key      ok\nmode     %s\nwallet   %.2f USDT (available %.2f)\n", mode, acc.WalletBalance, acc.AvailableBalance)
	for _, sym := range cfg.BinancePairs() {
		info, err := ex.SymbolInfo(ctx, sym)
		if err != nil {
			return err
		}
		sc, err := ex.SymbolConfig(ctx, sym)
		if err != nil {
			return fmt.Errorf("%s: %w", sym, err)
		}
		fmt.Fprintf(w, "pair     %s %s, leverage %dx on the exchange, %dx in the config\n", sym, strings.ToLower(info.Status), sc.Leverage, cfg.Leverage)
	}
	return nil
}

// configFile holds all of autotrade's settings (docs/readme.md).
const configFile = "autotrade.yaml"

// findConfig returns the -config path, else ./autotrade.yaml, else
// autotrade.yaml in the parent of the binary's folder (bin/..).
func findConfig(flagPath string) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if _, err := os.Stat(configFile); err == nil {
		return configFile, nil
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(filepath.Dir(exe)), configFile)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no " + configFile + " found; pass -config")
}

// credentials reads the key from the environment, else from dir/.env.
func credentials(dir string) (key, secret string) {
	key, secret = os.Getenv("BINANCE_TESTNET_API_KEY"), os.Getenv("BINANCE_TESTNET_API_SECRET")
	if key != "" && secret != "" {
		return key, secret
	}
	f, err := os.Open(filepath.Join(dir, ".env"))
	if err != nil {
		return key, secret
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch strings.TrimSpace(strings.TrimPrefix(k, "export ")) {
		case "BINANCE_TESTNET_API_KEY":
			if key == "" {
				key = v
			}
		case "BINANCE_TESTNET_API_SECRET":
			if secret == "" {
				secret = v
			}
		}
	}
	return key, secret
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
