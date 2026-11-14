# San Backtest

A backtest framework for trading approaches written in Go. It replays
Exness candles (through `golang/packages/san_exness`, cached on disk) or
CSV files, charges Exness's real costs, sizes every trade from a risk
percent, and reports stats, a trade list, an equity chart, walk-forward
windows and a comparison table. One strategy at a time or a portfolio of
legs on one shared account.

Status: built (design agreed with San on 2026-11-14). Tested on fakes and
hand-made candles with known answers; **never run against the real Exness
API** (see [First real run](#first-real-run)).

Module: `github.com/wargasipil/san_backtest` (Go 1.26). Dependencies:
`github.com/wargasipil/san_exness` (replace `../san_exness`) and
`github.com/urfave/cli/v3` for the CLI only. The PNG chart uses only the
standard library.

## Design

San's choices, one decision at a time:

- **Approaches are Go code** (the user, 2026-11-14: "a"): a `Strategy`
  interface, one type per approach, registered by name for the CLI. Not
  YAML rules, not text a bot translates.
- **Several timeframes, no look-ahead** (the user, 2026-11-14: "Is approach
  can read multiple timeframe ?"): the strategy steps on its first
  timeframe and sees only candles closed at that moment on every
  timeframe; `Forming(tf)` gives the higher-timeframe candle still going
  on, built from the closed step candles inside it.
- **Timestamps** (the user, 2026-11-14: "And timestamp ?"): UTC everywhere,
  `Local()` in WIB, `Session()` for Asia/London/New York, `Gap()` for
  weekends and holes. Gaps are kept, never filled with made-up candles.
  The broker's server time is ignored.
- **Data from Exness candles** (the user, 2026-11-14: "a"): bid and ask
  candles through san_exness, cached on disk, so a period downloads once.
  `CSV` is the second built-in source; any `DataSource` plugs in from code.
  Warm-up candles are fetched before `From`; `Strict` decides what happens
  when the history starts later.
- **Costs** (the user, 2026-11-14: "C."): Exness's real costs by default
  (each candle's bid/ask spread; commission, swap, contract size and
  trading sessions from the instrument's conditions), with `Costs`
  overrides and the CLI flags `--spread --commission --swap --slippage`.
- **Sizing** (the user, 2026-11-14: "b"): risk percent per trade, default
  balance 1000 and risk 1%. The lot comes from the stop distance, rounded
  down to the lot step; a trade under the minimum lot is skipped; margin is
  checked; the balance compounds.
- **Portfolio** (the user, 2026-11-14: "Can we have simulated portfolio
  too"): `RunPortfolio` with legs, one shared balance and margin, one clock,
  `MaxOpen`, `MaxOpenRisk` and `MaxDailyLoss`. A single `Run` is a one-leg
  portfolio on the same engine.
- **Report** (the user, 2026-11-14: "D"): stats, a trade CSV (UTC and WIB),
  an equity PNG, walk-forward `Walk{Train, Test}`, and a comparison table.
- **Library first** (the user, 2026-11-14: "And backtest can be used as code
  library right ?"): `san_backtest.Run(ctx, Config{...})`; the CLI is a thin
  wrapper whose flags map onto `Config` fields.
- **Configuration in code** (the user, 2026-11-14: "I mean configuration in
  code"): a `Config` struct whose zero values are the defaults, `Last(d)`
  for "the last 90 days", and `Config.Validate()` naming every wrong field.

## Use

```go
import (
	"context"
	"fmt"
	"time"

	bt "github.com/wargasipil/san_backtest"
	exness "github.com/wargasipil/san_exness"
)

client, err := exness.New(exness.Config{ /* key, account: see san_exness */ })
if err != nil {
	return err
}
from, to := bt.Last(90 * 24 * time.Hour)
res, err := bt.Run(context.Background(), bt.Config{
	Strategy:   &MyApproach{},
	Instrument: "XAUUSDm",
	Data:       bt.NewExness(client), // or &bt.CSV{Dir: "data"}
	From:       from,
	To:         to,
	Risk:       1,                    // % per trade (default 1)
})
if err != nil {
	return err
}
fmt.Println(res.Summary())
paths, err := res.Save("out/my_approach") // summary.txt, trades.csv, equity.png, ...
```

A strategy:

```go
type MyApproach struct{}

func (*MyApproach) Name() string         { return "my_approach" }
func (*MyApproach) Timeframes() []string { return []string{"M15", "H1"} } // step first
func (*MyApproach) Warmup() int          { return 60 }                    // closed candles per timeframe

func (*MyApproach) OnCandle(c *bt.Context) bt.Signal {
	if c.Position() != nil {
		return bt.Wait
	}
	if c.EMA("H1", 20) > c.EMA("H1", 50) && c.Close("M15") > c.EMA("M15", 20) {
		atr := c.ATR("M15", 14)
		price := c.Close("M15")
		return bt.Buy(price-1.5*atr, price+3*atr) // stop and target are prices
	}
	return bt.Wait
}
```

Three example approaches, modelled on San's approach notes in
`golang/apps/san_tower/workflows/trading/journal/approaches/`, are in
`examples/` (`big_trend`, `breakout_retest`, `moving_average`); the CLI
registers them. Add your own in `cmd/san_backtest/main.go`'s `registry()`.

## API

### Strategy

```go
type Strategy interface {
	Name() string
	Timeframes() []string // the first is the step: OnCandle runs once per closed candle of it
	Warmup() int          // closed candles each timeframe needs before the first OnCandle
	OnCandle(c *Context) Signal
}

// optional
type Starter interface{ OnStart(info StartInfo) } // before every run, walk-forward trials too: reset state here
type Tunable interface{ Params() map[string]float64 } // numbers and defaults; read them with c.Param(name)
```

Timeframes: S1 S5 S15 M1 M3 M5 M10 M15 M30 H1 H2 H4 D1 W1 MN, none shorter
than the first. A strategy that reads its timeframes from `c.Timeframes()`
instead of naming `"M15"` keeps working when `--tf` changes them.

Signals (prices, not distances; all act at the **next** candle's open):

| Signal | Does |
|---|---|
| `Wait` | nothing |
| `Buy(stop, target)` | open a long at the ask; `target` 0 = none |
| `Sell(stop, target)` | open a short at the bid |
| `Close(reason)` | close the open position; the trade's reason is `close: <reason>` |
| `MoveStop(price)` | move the stop, only closer to the price (up for a long, down for a short); a move the other way is ignored |

A `Buy` or `Sell` while the leg already has a position is ignored (one
position per leg, no pyramiding or reversing in one signal).

### Context

| Method | Gives |
|---|---|
| `Time()` | close time of the step candle, UTC |
| `Local()` | the same in WIB (UTC+7) |
| `Step()`, `Timeframes()` | the step timeframe; all timeframes in force, step first |
| `Candles(tf)`, `Last(tf)` | closed candles only, oldest first (on a higher timeframe: closed at `Time()`) |
| `Close(tf)`, `High(tf)`, `Low(tf)` | newest closed candle's bid prices (NaN before any) |
| `Forming(tf)` | the higher-timeframe candle still going on, from the closed step candles inside it |
| `Session()` | `Session{Asia, London, NewYork}` at `Time()`, daylight saving followed |
| `Gap()` | previous step close to this step's open: 0, ~2 days after a weekend, more after a hole |
| `Position()` | copy of the open position (side, entry, stop, target, lots, risk, floating profit), nil when flat |
| `Account()` | currency, balance, equity, used margin, open positions (all legs) |
| `Instrument()` | name, currencies, digits, point, contract size, lot min/max/step, spread now |
| `Param(name)` | a `Params()` number in force (default, `Config.Params`, or the walk-forward pick) |
| `EMA SMA RSI ATR (tf, period)` | on tf's closed bid closes, incremental; NaN until enough candles |
| `MACD(tf, fast, slow, signal)` | line, signal, histogram |
| `Bollinger(tf, period, k)` | middle, upper, lower |
| `SwingHighs(tf, strength)`, `SwingLows` | confirmed swings only (`strength` candles after them have closed) |

Asking for a timeframe not in `Timeframes()` panics inside the strategy;
the run returns it as an error.

### Config

```go
type Config struct {
	Strategy     Strategy
	Instrument   string         // "XAUUSDm"
	Data         DataSource     // NewExness(client) or &CSV{Dir: ...}
	From, To     time.Time      // To exclusive, zero = now; Last(d) gives both
	Strict       bool           // history starting after From is an error (else the run starts later, noted)
	Balance      float64        // default 1000
	Currency     string         // default "USD"
	Risk         float64        // % of the balance per trade, default 1
	Leverage     int            // default 200
	Costs        Costs          // nil fields = Exness's real costs
	DayTZ        *time.Location // where a day starts for MaxDailyLoss, default UTC
	MaxDailyLoss float64        // % below the day's starting equity: no new trades that day; 0 = off
	Walk         *Walk          // walk-forward windows besides the full run
	Params       map[string]float64 // override Params() defaults
	Cache        string         // default "./backtest_cache"; "off" keeps nothing
	Timeframes   []string       // override Timeframes() (CLI --tf)
}

type Costs struct {
	Spread     *float64    // fixed spread in price instead of each candle's ask - bid
	Commission *float64    // per lot per side, account currency
	Swap       *[2]float64 // long, short per lot per night, account currency (negative = charge)
	Slippage   float64     // price against you on every market fill
}

type Walk struct{ Train, Test time.Duration }
```

`Config.Validate()` (and `Portfolio.Validate()`) return every wrong field
at once, joined, each a `*FieldError` such as `Config.Risk: must be a
percent from 0 (= 1) to 100, got 150`. `Run` calls it first.

### Portfolio

```go
res, err := bt.RunPortfolio(ctx, bt.Portfolio{
	Data: bt.NewExness(client), From: from, To: to,
	MaxOpen:     2,   // positions open at once, all legs
	MaxOpenRisk: 3,   // % of the balance at risk to the stops, all open positions
	MaxDailyLoss: 4,
	Legs: []bt.Leg{
		{Strategy: &examples.BigTrend{}, Instrument: "XAUUSDm", Risk: 1},
		{Strategy: &examples.MovingAverage{}, Instrument: "EURUSDm", Risk: 0.5, Costs: &bt.Costs{Slippage: 0.00002}},
	},
})
```

`Portfolio` has the account fields of `Config` (Data, From, To, Strict,
Balance, Currency, Leverage, MaxDailyLoss, DayTZ, Costs, Cache) plus
`MaxOpen`, `MaxOpenRisk` and `Legs`. A `Leg` is `Strategy, Instrument,
Risk, Costs *Costs (nil = the portfolio's), Params, Timeframes`. Every leg
runs on one clock: at the same moment, candle closes come before opens,
then legs in order. `Result.Legs` gives each leg's own stats and its share
of the net profit. Walk-forward is for `Run` only.

### Data sources

```go
type DataSource interface {
	Candles(ctx context.Context, q Query) ([]Candle, error) // closed candles opening in [From, To), oldest first
	Instrument(ctx context.Context, name string) (Instrument, error)
}
```

- `NewExness(api)`: any `ExnessAPI` (`*san_exness.Client` is one): bid and
  ask (`price_type` both), 1000 candles a request, the candle still forming
  is dropped, rate limits (429 / `Retry-After`) retried up to 4 times.
  `FromExness` reads the instrument conditions (see
  [Known limits](#known-limits-and-decisions)).
- `CSV{Dir}`: `<Dir>/<instrument>_<TF>.csv` with the header
  `time,open,high,low,close,ask_open,ask_high,ask_low,ask_close,volume`
  (time = open time, RFC 3339 or Unix seconds; the ask columns and volume
  may be left out, then ask = bid and only `--spread` gives a spread).
  Instruments from `CSV.Instruments` or `<Dir>/<instrument>.json` (an
  `Instrument` as JSON).
- Cache: a `Cacheable` source (Exness) is wrapped in `Config.Cache`:
  `<Cache>/exness/<instrument>/<TF>/<YYYY-MM>.csv`, each month file
  headed `# covered_to <time>`. A range already covered makes no request;
  the month still going on is topped up from `covered_to`. Instrument
  conditions are kept a day (`instrument.json`). Delete the folder to
  download again.

### Result

`Result{Name, Stats, Trades, Missed, Equity, Range, Costs, Legs, Params,
Walk}`. `Range` is what really ran (first step candle's open, last one's
close) with the requested range and notes on why they differ. `Summary()`
is the text report, `Save(dir)` writes the files, `WriteTrades`,
`WriteMissed`, `WriteEquity`, `WriteWalk` and `WritePNG` write one each,
and `Compare(results...)` is the side-by-side table.

Helpers: `Last(d)`, `ParseSpan("3m")` (m = months of 30 days),
`ParseZone("WIB")`, `ParseTimeframe("h1")`, `TimeframeDuration`,
`SessionAt(t)`, `WIB`, `NewRegistry()` / `Register` / `New` / `Names`.

## CLI

`cmd/san_backtest` reads candles and conditions only; it never trades.
Each flag maps onto one `Config` or `Portfolio` field (shown in `--help`).

```bash
# list the registered strategies, their timeframes, warm-up and params
san_backtest list

# CSV data, no Exness key needed
san_backtest run moving_average --csv data --instrument XAUUSD --from 2026-08-01 --to 2026-11-01 --out out/ma

# Exness, last 90 days, overrides and walk-forward (3 months train, 1 month test)
san_backtest run big_trend --instrument XAUUSDm --last 1y --walk 3m:1m --risk 0.5 --commission 3.5 --param fast=30 --out out/bt

# several strategies side by side: one folder each plus the comparison table
san_backtest run big_trend,breakout_retest,moving_average --instrument XAUUSDm --last 180d --out out/cmp

# a portfolio on one account
san_backtest portfolio --leg big_trend:XAUUSDm:1 --leg moving_average:EURUSDm:0.5 --max-open 2 --max-open-risk 3 \
  --max-daily-loss 4 --tz WIB --last 90d --out out/pf
```

```text
COMMANDS:
   run        backtest one strategy, or several (comma-separated) side by side
   portfolio  backtest several legs on one shared account
   list       list the strategies: timeframes, warm-up, params

GLOBAL OPTIONS:
   --api-key string           Exness API key identifier (exnsk-...) [%EXN_API_KEY%]
   --private-key-file string  file with the private key; or set EXN_PRIVATE_KEY [%EXN_PRIVATE_KEY_FILE%]
   --account string           trading account id [%EXN_ACCOUNT_ID%]
   --gateway string           Gateway host from Key Management's DNS widget (https://...) [%EXN_DISCOVERY_BASE_URL%]
   --access-point string      the account's Access Point, to skip discovery [%EXN_API_BASE_URL%]

run and portfolio:
   --from string            first candle's open time, UTC: 2026-08-01 or RFC 3339 (Config.From)
   --to string              end, UTC, exclusive; empty = now (Config.To)
   --last string            instead of --from/--to: the last 90d, 12h, 2w, 3m (months), 1y (Last)
   --tz string              where a day starts for --max-daily-loss: UTC, WIB or an IANA zone (Config.DayTZ) (default: "UTC")
   --balance float          starting balance (Config.Balance) (default: 1000)
   --currency string        account currency (Config.Currency) (default: "USD")
   --leverage int           account leverage, 200 = 1:200 (Config.Leverage) (default: 200)
   --spread float           fixed spread in price instead of each candle's ask - bid (Costs.Spread)
   --commission float       commission per lot per side, account currency, instead of Exness's (Costs.Commission)
   --swap string            LONG,SHORT swap per lot per night, account currency, instead of Exness's (Costs.Swap)
   --slippage float         price added against you on every market fill (Costs.Slippage)
   --max-daily-loss float   percent below the day's starting equity that stops new trades for the day; 0 = off (MaxDailyLoss)
   --strict                 fail when history starts after --from (Config.Strict)
   --cache string           folder for downloaded Exness candles; "off" keeps none (Config.Cache) (default: "./backtest_cache")
   --csv string             read candles from CSV files in this folder instead of Exness (see README)
   --out string             write summary.txt, trades.csv, missed.csv, equity.csv, equity.png (and walk.csv) here

run only:
   --instrument string      e.g. XAUUSDm (Config.Instrument)
   --tf string              timeframes, step first, e.g. M15,H1,H4; empty = the strategy's (Config.Timeframes)
   --risk float             percent of the balance risked per trade (Config.Risk) (default: 1)
   --walk string            walk-forward TRAIN:TEST, e.g. 3m:1m (Config.Walk)
   --param NAME=VALUE       overriding a strategy number, repeatable (Config.Params)

portfolio only:
   --leg NAME:INSTRUMENT[:RISK]  repeatable (Portfolio.Legs)
   --max-open int                open positions at once, all legs; 0 = no limit (Portfolio.MaxOpen)
   --max-open-risk float         percent of the balance at risk in open positions; 0 = no limit (Portfolio.MaxOpenRisk)
```

The Exness key is read from `EXN_PRIVATE_KEY` or `--private-key-file`,
never from a flag value, as in san_exness's own CLI. With `--csv` no key is
needed. Spans: `h`, `d`, `w`, `m` (30-day months, not minutes) and `y`.

## Report

`Summary()` / `summary.txt`:

- the name, the range that ran (UTC with WIB), notes on why it differs
  from the request (history starts later or ends earlier);
- per leg, where each cost comes from (`exness: ...` or `override: ...`)
  and how the conditions were read;
- the params in force;
- trades (won, lost), win rate, profit factor (`inf` with no loss),
  average R, net profit and return %, commission, swap, start and end
  balance, max drawdown (money and % of the equity high), the longest
  losing streak, the first half against the second half of the range, and
  the skipped signals by reason;
- with several legs: a table per leg with its share of the net;
- with walk-forward: one row per window (test dates, picked params, train
  net, test trades, test net, test win %) and the out-of-sample totals.

Files from `Save(dir)` / `--out`:

| File | Content |
|---|---|
| `summary.txt` | the text above |
| `trades.csv` | one row a trade: leg, strategy, instrument, side, signal/open/close times in UTC and WIB, entry, exit, stop, final stop, target, lots, spread, risk, gross, commission, swap, net, R, reason (`stop`, `target`, `close: ...`, `closed at end`) |
| `missed.csv` | signals that did not open, with time (UTC, WIB) and reason |
| `equity.csv` | balance and equity after every step |
| `equity.png` | 1200x600: equity (blue) and balance (grey), money on the left, UTC dates below |
| `walk.csv` | with walk-forward: each window's dates, params, trials, train and test results |

Skip reasons (`Stats.Skipped`): `too small` (the risk buys less than the
minimum lot), `margin`, `max open`, `max open risk`, `daily loss`,
`market closed` (outside the instrument's sessions) and `invalid levels`
(stop or target on the wrong side of the fill).

Walk-forward: windows of `Train` then `Test` roll by `Test` over the range
that ran. On each train window the strategy's `Params()` are tuned by one
pass of coordinate search: each number (by name) tries its default x0.5,
0.75, 1, 1.25 and 1.5 (whole numbers stay whole) with the others held at
the best so far, keeping the highest train net profit. The pick then runs
on the test window. Test windows chain their balance; their trades make
the out-of-sample stats. A strategy without `Params()` gets the same
windows with no tuning (a stability check).

## Costs and fills

- **Prices.** `Open/High/Low/Close` are the bid (what an MT5 chart draws),
  `Ask*` the ask. A long opens at the ask and closes at the bid; a short
  opens at the bid and closes at the ask. So the spread paid is each
  candle's own ask - bid, wider where Exness's candles are wider.
- **Entry** at the next candle's open after the signal (the signal sees
  the closed candle; it cannot fill at that candle's close).
- **Stop and target** are checked on each candle's range on the side that
  closes: a long's on the bid high/low, a short's on the ask high/low,
  from the entry candle on. When both are inside one candle the **stop
  wins** (a candle does not say which came first). A candle that opens past
  a level fills at its open: worse than the stop, better than the target.
- **Close(reason)** fills at the next open. Positions still open at `To`
  are closed at the last close and marked `closed at end`.
- **Sizing.** lots = balance x risk% / (|fill - stop| x contract size, in
  the account currency), rounded **down** to the lot step and capped at the
  maximum lot. Under the minimum lot: skipped (`too small`). The margin
  (lots x contract size x price / leverage x margin percent) must fit in
  the free margin (equity - used margin), else `margin`. The balance
  compounds: each trade sizes from the balance at its fill.
- **Commission** per lot per side: charged at the open and again at the
  close.
- **Swap** at every rollover, 17:00 New York time (21:00 UTC in summer,
  22:00 in winter), Monday to Friday, in points x point x contract size x
  lots. The instrument's swap multipliers make one weekday count three
  (Wednesday when Exness lists none, the forex convention).
- **Sessions.** Outside the instrument's trading sessions no trade opens
  (`market closed`), a `Close` waits for the market to open, and stops and
  targets are not checked.
- **Slippage** (0 unless set) moves every market fill against you: entries,
  `Close`, stops. Targets are limit orders and get none.
- **Daily loss.** Once equity (floating included) is `MaxDailyLoss`%
  below the day's starting equity, no new trade opens until the next day
  in `DayTZ`. Open positions keep running.
- **Overrides.** `Costs.Spread` replaces the ask with bid + spread;
  `Costs.Commission` and `Costs.Swap` are in the account currency. The
  report says which costs are Exness's and which are overridden.

## First real run

Nothing here has run against the real Exness API: every Exness test uses a
fake HTTP server (`data_test.go`, `cmd/san_backtest/main_test.go`) or a
stub `ExnessAPI`. What a first run needs, from san_exness's docs
(`golang/packages/san_exness/docs/readme.md`):

1. An Exness trading account, **fully verified** (proof of identity and of
   address); standard MetaTrader accounts cannot connect, and the API is
   offered only in some regions.
2. An API key made in the Personal Area's Key Management: `EXN_API_KEY`,
   the private key in `EXN_PRIVATE_KEY_FILE` (or `EXN_PRIVATE_KEY`),
   `EXN_ACCOUNT_ID`, and the gateway from the DNS widget
   (`EXN_DISCOVERY_BASE_URL`) or the Access Point (`EXN_API_BASE_URL`).
3. A small first run, read-only like every run:

   ```bash
   san_backtest run moving_average --instrument XAUUSDm --last 7d --out out/first
   ```

   Then check in `summary.txt` the `costs` lines and notes (how the
   conditions were read: commission type, swap days, sessions), that
   `trades.csv` spreads look like Exness's, and that a second identical
   run is instant (served from `backtest_cache`).

Exness's API reference gives field names, not units, so the readings in
[Known limits](#known-limits-and-decisions) are assumptions to check on
that first run; a wrong one shows as a note or as odd costs, and
`--commission` / `--swap` / `--spread` override it meanwhile.

## Known limits and decisions

- **Exness condition readings** (`FromExness`): `swap_long`/`swap_short`
  are read as points per lot per night (MT5's swap-in-points mode).
  `commission_settings` with type `per_lot_per_side` (or empty) is per
  lot per side; a type naming a round turn (`round`, `per_trade`) is
  halved per side; any other type is read as per lot per side, with a
  note. `swap_multipliers` days may be names or numbers; with none listed,
  Wednesday counts 3. `trade_sessions` bounds are read as `"1 00:05:00"`
  (weekday number, then UTC time) or `"Monday 00:05"`; sessions whose
  trade mode is not empty/`enabled`/`full` are closed for new trades; a
  bound that cannot be read drops the session limit, with a note.
  `margin_percent` empty = 100.
- **Commission currency** is taken as the account currency, with a note
  when Exness names another one (no conversion).
- **Non-USD quote currencies.** Money converts only for instruments quoted
  in the account currency (XAUUSD, EURUSD with a USD account) or based on
  it (USDJPY: profit in JPY / price). A cross such as EURGBP on a USD
  account is refused with an error, since converting needs a third
  instrument's prices; a later version could load that pair as well.
- **One position per leg.** No pyramiding, partial closes or pending
  orders; reversing is `Close` then the opposite signal.
- **Candle resolution.** Fills are judged from OHLC only; the stop wins
  when stop and target are in one candle. A lower step timeframe gives
  finer fills.
- **MN** candles close at the next calendar month's start; `ParseSpan`'s
  `m` is 30 days.
- **Broker server time** is ignored: all times are UTC as Exness sends
  them. Sessions in `Session()` are Tokyo 09-18, London 08-17 and New York
  08-17 local time.
- **Walk-forward** tunes leg 0 of a `Run` only, by one coordinate-search
  pass over a fixed x0.5..x1.5 grid (not a full grid search).
- **Cache** holds candles only for the Exness source; CSV is read as is.
  A run whose `To` is now tops up the current month on every run (a few
  requests); a range fully in the past makes none.
- Additions beyond the agreed field list: `Config.Timeframes` /
  `Leg.Timeframes` (for `--tf`), `Leg.Params`, `Result.Missed` with
  `missed.csv`, and `CostReport` notes.

## Files

| File | What |
|---|---|
| `strategy.go` | `Strategy`, `Starter`, `Tunable`, signals, `Registry` |
| `context.go` | `Context`: candles, forming candle, position, account, indicators |
| `indicators.go` | incremental EMA, SMA/Bollinger, RSI, ATR, MACD, swings |
| `config.go` | `Config`, `Costs`, `Walk`, `Portfolio`, `Leg`, defaults, `Validate` |
| `engine.go` | loading with warm-up, the event-clock simulation, fills, sizing, costs |
| `walk.go` | walk-forward windows and tuning |
| `data.go` | `DataSource`, the Exness source, CSV |
| `cache.go` | the disk cache |
| `instrument.go` | `Instrument`, `FromExness`, sessions |
| `candle.go`, `clock.go` | timeframes, WIB, sessions, rollovers, `Last`, `ParseSpan` |
| `result.go`, `report.go`, `chart.go` | stats, text/CSV reports, the PNG |
| `examples/` | three example approaches |
| `cmd/san_backtest/` | the CLI |

## Build and test

```bash
go vet ./...
go test -count=1 -cover ./...
bash build.sh          # vet, test, then bin/san_backtest.exe and bin/san_backtest (CGO off)
```

Tests run on in-memory sources, CSV files in `t.TempDir()` and a fake
Exness HTTP server; nothing touches the network or leaves files behind.
They pin by hand-worked numbers: fills on the right side, stop before
target, gaps filled at the open, lot rounding, too-small and margin skips,
commission, swap with the triple day, no look-ahead and the forming
candle, warm-up before `From`, `Strict`, sessions, `MoveStop`, the daily
loss by time zone, the portfolio limits, walk-forward windows, the stats,
the report files and PNG, the cache (a second run makes no requests), and
the CLI flags mapping onto `Config`.
