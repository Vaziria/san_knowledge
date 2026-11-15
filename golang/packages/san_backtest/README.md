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
| `Close(reason)` | close the open position (with several open: every one of them); the trade's reason is `close: <reason>` |
| `MoveStop(price)` | move the stop, only closer to the price (up for a long, down for a short); a move the other way is ignored. With several open: each position it moves closer |
| `BuyLimit(price, stop, target)`, `SellLimit(...)` | a pending limit order: buy at `price` or lower, sell at `price` or higher |
| `BuyStop(price, stop, target)`, `SellStop(...)` | a pending stop order: buy once the ask reaches `price`, sell once the bid falls to it |
| `.Expire(n)`, `.Until(t)`, `.Tag(s)` | on an order: expire after n step candles, or at time t; label it (the label goes to its trade) |
| `Cancel(id)`, `CancelAll(reason)` | cancel a pending order, or all of the leg's |
| `CloseID(id, reason)`, `MoveStopID(id, price)` | close or move the stop of one position |
| `CloseAll(reason)` | close every position of the leg and cancel its orders (flat) |
| `Do(sig1, sig2, ...)` | several of these in one step, run in order at the next open |

By default a leg holds one position at a time: a `Buy` or `Sell` while it
has one is ignored (unrecorded, as it always was). With `Config.MaxOpen`
/ `Leg.MaxOpen` (or a strategy's own `MaxOpen()`, below) above 1 it may
hold that many, longs and shorts together.

### Positions and pending orders

(the user, 2026-11-15: "1b, 2a, 3b" and "1b, 2a, 3a, 4a": limit and stop
orders; the worst case when a fill and the stop share a candle; several
pending orders and positions; hedging allowed; limits = max open positions
plus max total risk %; sizing = risk % of the current balance; each order
fills once.)

```go
func (s *Ladder) OnCandle(c *bt.Context) bt.Signal {
	var out []bt.Signal
	for _, p := range c.Positions() { // oldest first; Position() is the first
		if c.Close("H1") > p.Entry+2 && p.Stop < p.Entry {
			out = append(out, bt.MoveStopID(p.ID, p.Entry)) // break-even, this one only
		}
	}
	if len(c.Positions())+len(c.Orders()) < c.MaxOpen() {
		ema := c.EMA("H4", 20)
		out = append(out, bt.BuyLimit(ema, ema-5, ema+10).Expire(4).Tag("ema20"))
	}
	return bt.Do(out...)
}
// bt.Config{..., MaxOpen: 2, MaxOpenRisk: 3}
```

- Every position and order has an `ID`, unique in the run (orders and
  positions share the numbering). `Position` has `ID`, `OrderID` (0 for a
  market entry), `Type` (`market`, `limit`, `stop`) and `Tag`; `Order` has
  `ID, Side, Type, Price, Stop, Target, Placed, BarsLeft, Expires, Tag`.
- An order needs a stop-loss and a take-profit on the right sides of its
  price (rule #1); it is checked when placed and again at the fill, and
  refused as `invalid levels`. (A market `Buy`/`Sell` keeps accepting
  target 0, as before; chains always require both.)
- A strategy may say how many positions it holds by implementing
  `MultiPosition` (`MaxOpen() int`; a chain with `steps.MaxOpen(n)` does);
  `Config.MaxOpen` / `Leg.MaxOpen` override it. `c.MaxOpen()` is the limit
  in force (1 by default).
- `Config.MaxOpenRisk` (the max total risk, as `Portfolio.MaxOpenRisk`): the
  percent of the balance the open positions lose at their current stops
  (distance from entry to the stop x lots x contract; 0 once a stop is at
  break-even), the new one included, checked at each fill. Pending orders
  do not count: they are checked when they fill, since most never do.
- A fill that would break a limit (daily loss, `MaxOpen` of the portfolio
  or the leg, `MaxOpenRisk`, margin across all open positions, too small)
  is skipped with that reason and the order is gone.
- Sizing at the fill: risk % of the balance then, rounded down to the lot
  step; under the minimum lot: `too small`. Commission, swap and spread are
  per position, as for market entries; equity counts every open position.

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
| `Position()` | copy of the open position (side, entry, stop, target, lots, risk, floating profit, ID, order ID, type, tag), nil when flat; with several the first, or the one `ManageOpen` is managing |
| `Positions()`, `Orders()` | every open position and pending order of the leg, oldest first (copies) |
| `Current()` | the position a chain's `ManageOpen` is managing now (else `Position()`) |
| `MaxOpen()` | the leg's position limit in force (`Config.MaxOpen`, the strategy's `MaxOpen()`, else 1) |
| `Account()` | currency, balance, equity, used margin, open positions (all legs) |
| `Instrument()` | name, currencies, digits, point, contract size, lot min/max/step, spread now |
| `Param(name)` | a `Params()` number in force (default, `Config.Params`, or the walk-forward pick) |
| `EMA SMA RSI ATR (tf, period)` | on tf's closed bid closes, incremental; NaN until enough candles |
| `MACD(tf, fast, slow, signal)` | line, signal, histogram |
| `Bollinger(tf, period, k)` | middle, upper, lower |
| `SwingHighs(tf, strength)`, `SwingLows` | confirmed swings only (`strength` candles after them have closed) |
| `Note(format, args...)` | explains this step in the decision log: `c.Note("ema20 above ema50, rsi %.0f", r)`; the reason of the step's signal, or a `note` line when it waits; free in walk-forward trials (nothing is kept) |

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
	MaxOpen      int            // positions open at once; 0 = the strategy's MaxOpen(), else 1
	MaxOpenRisk  float64        // max total risk: % of the balance the open positions lose at their stops; 0 = no limit
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
Risk, Costs *Costs (nil = the portfolio's), Params, Timeframes, MaxOpen`
(the leg's own position limit; `Portfolio.MaxOpen` caps all legs). Every leg
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
Walk, Run, Decisions, Candles}`. `Range` is what really ran (first step
candle's open, last one's close) with the requested range and notes on why
they differ. `Run` (`RunInfo`) is what ran: kind (`run` or `portfolio`),
when, the account settings and per leg its strategy, instrument,
timeframes, risk, params and overridden costs. `Decisions` is the decision
log and `Candles` each leg's step candles over the range (both of the main
run only; walk-forward trials keep neither). `Summary()` is the text
report, `Save(dir)` writes the files, `WriteTrades`, `WriteMissed`,
`WriteEquity`, `WriteWalk`, `WritePNG`, `WriteDecisions`, `WriteCandles`
and `WriteRun` write one each, and `Compare(results...)` is the
side-by-side table.

### Live (paper trading)

`NewLive(ctx, LiveConfig{Config, Now, Grace})` runs a strategy one closed
candle at a time, as the candles come (San Tower's Live Trading paper
sessions; the user, 2026-11-15: "Monitor + Signals (paper trading)"). It
never sends an order: it is the backtest engine fed step by step, so the
fills, spread, swap, worst case, pending orders, positions, MaxOpen, the
decision log and rule #1 are Run's code, and a Live over a past range
gives exactly Run's trades and stats (`examples/live_test.go` and
san_research's `live_test.go` replay the real cached candles one at a
time, with gaps and catch-ups, against Run).

- `NewLive` loads the warm-up history through the Cache, like Run, and
  runs any candles from `From` to now at once (a catch-up).
- `Advance(ctx)` asks Data for the candles that closed since the last
  call, on every timeframe, and runs the step candles that are ready,
  oldest first; missed candles are fetched in order. `Feed(tf, through,
  candles...)` plus `Step()` do the same with candles of your own.
- Each returns an `Update{Candles, Decisions, Trades, Missed, Equity}`
  (what that call added). `State()` is where it is now (candles run, last
  and next close, balance, equity, margin, open positions and orders),
  `Result()` the run so far, `Finish()` closes what is open and ends it.
- Only closed candles: a signal at one close fills at the next candle's
  open, as in a backtest, learned one candle later; the forming candle is
  never fed. A higher-timeframe candle late from the broker is waited for
  `Grace` (default 2 minutes).
- A strategy's own state cannot be saved, so a Live is not serialized: to
  continue after a restart, make a new Live with the same `From`; it
  replays to now and gives the same run as if it had never stopped.
- `Result.Restat()` recomputes Stats from Trades and Equity (a result
  rebuilt from a journal).

Helpers: `Last(d)`, `ParseSpan("3m")` (m = months of 30 days),
`ParseZone("WIB")`, `ParseTimeframe("h1")`, `TimeframeDuration`,
`SessionAt(t)`, `WIB`, `NewRegistry()` / `Register` / `New` / `Names`.

## Chain strategies

A strategy can also be written as an ordered list of small steps, a chain
of responsibility (the user, 2026-11-15: "can we make strategy code like
chainable pipeline" and "In function like chain of responsibility"; the
design below was agreed the same day: "Okay, agree"). `Build()` returns an
ordinary `Strategy`, so the engine, the reports, `run.json`,
`decisions.jsonl` and the tower's Backtest Analysis work unchanged.

```go
type Next func(c *bt.Context) bt.Signal
type Step func(c *bt.Context, next Next) bt.Signal   // a step as a plain function

bt.Chain(name, timeframes...).Use(steps...).Warmup(n).Build() // (bt.Strategy, error); MustBuild panics instead
```

Each step gets the Context and `next` (the rest of the chain). It may stop
the chain (`return bt.Wait.Because("spread %.2f", sp)`), call `next` and
pass its answer on, change that answer on the way back, or answer itself (a
`Close`, a `MoveStop`). A step only runs when every step before it called
`next`, so order matters.

- **Shared fields, reset every candle:** `c.Dir` (`bt.Long`, `bt.Short`
  or `""` = not set), `c.Entry` (the planned entry, the step candle's
  close), `c.Stop` and `c.Target` (NaN until set). A direction step sets
  `Dir`; the steps after it are written once and mirror for shorts;
  triggers check both sides while `Dir` is not set. At the end of the
  chain, with `Dir` set and flat, the chain answers `Buy`/`Sell(c.Stop,
  c.Target)`. `c.SetStop(dist)` puts the stop `dist` from `Entry` on
  `Dir`'s side and `c.Risk()` is that 1R. These are plain fields on
  `Context` (other strategies ignore them), which keeps every existing
  method as it was and lets custom steps read and write them directly.
- **ManageOpen goes first.** While a position is open it runs its own steps
  (each acts or passes, so the first that acts wins) and never reaches the
  entry steps; while flat it calls `next`. `c.Held()` gives the open
  position's candles held, best price since the entry and first stop (1R).
- **Rule #1** (every trade has a stop-loss and a take-profit): `Build`
  always adds `RequireSLTP` as the innermost step, and the final answer is
  checked again, so an entry without both on the right sides of `Entry`
  becomes Wait with a note.
- **Decision log:** the path a candle took (each step, what it saw, which
  one blocked and why) goes into `decisions.jsonl` through `c.Note`, only
  on candles where something happened: a signal, a trigger that fired
  (and was blocked later), rule #1 blocking, or a pure trigger that would
  have fired on a candle an earlier filter blocked ("pullback_to would
  fire buy"). Quiet candles log nothing (the user's earlier "2b"). Steps
  add detail with `c.Explain(...)`.
- **Params:** `steps.P("rr", 2)` is a tunable number; the chain's
  `Params()` lists every one, so `Config.Params`, `--param` and
  walk-forward tune them. Pass the same `P` value to every step sharing
  it; two `P`s with one name are an error from `Build`. Walk-forward tries
  a `P` as a fraction (`FineGrid`: 1, 1.5, 2, 2.5, 3 for 2), or as whole
  numbers (`DefaultGrid`) when a step uses it as a period or count;
  `.Whole()` and `.Grid(v...)` choose. Strategies other than chains keep
  `DefaultGrid` unless they implement `GridTuner`.
- **Warmup:** the largest period/bars any step declares plus
  `max(5, 20%)`, or `.Warmup(n)`.
- **Timeframes:** steps name the chain's own timeframes ("H4"); `c.TF`
  maps them by position onto the timeframes in force, so `--tf` works.
- Missing data (NaN indicators, too few candles) makes a step Wait; it
  never becomes a signal.

### Example: pullback_trend as a chain

The research approach `pullback_trend` (san_research, `trend_chains.go`)
trades exactly like its hand-written version; `days` is that package's
small adapter for daily bars built from H4 (the cache has no D1).

```go
v := &days{}
trend, stopBuf := steps.P("trend", 50).Whole(), steps.P("stop_buf", 0.5)
targetR, holdDays := steps.P("target_r", 2).Whole(), steps.P("hold_days", 3).Whole()
atr := steps.ATR("H1", 14)
ema20 := steps.EMA("H4", 20)
st := bt.Chain("pullback_trend", "H1", "H4").Use(
	steps.ManageOpen(steps.TimeStop(holdDays.Times(23))), // about hold_days trading days
	steps.Hours(14, 23),                                  // the step candle opens 14:00-22:59 WIB
	steps.OncePerDay(),
	v.direction(trend),                                   // the daily trend (custom step)
	steps.Agree(ema20, steps.EMA("H4", 50)),              // H4 EMA20 above EMA50 for longs
	steps.PullbackTo("H1", ema20).CameFrom(24, atr),      // dipped to the H4 EMA20 after being 1 ATR above
	steps.SwingStop("H1", 12, atr.Times(stopBuf), atr),   // under the 12-candle low by stop_buf ATR, at least 1 ATR
	steps.MaxStop(v.atr().Times(1.5)),                    // no trade when wider than 1.5 daily ATR
	steps.RR(targetR),
).Warmup(330).MustBuild()
```

A custom step is a `bt.Step` func, or `bt.Define(name, role, run, setup)`
when it also declares params, bars or state (`d.Param`, `d.Bars`,
`d.OnStart`, `d.EveryCandle`, `d.Sub`, `d.Directions`).

### Built-in steps (package `steps`)

Numbers are `any`: a constant, `steps.P(...)`, or a value expression.

| Kind | Steps |
|---|---|
| Filters | `Hours(from, to)` (WIB, of the step candle's open; wraps past midnight), `Weekdays(days...)`, `Sessions(names...)` (`asia`, `london`, `newyork`, `overlap`), `SkipRollover()` (within 1 h of 17:00 New York), `SkipMondayOpen()` (2 h after a gap of a day or more), `MinATR(tf, n, x)`, `MaxATR(tf, n, x)`, `MaxSpread(x)`, `OncePerDay()` (WIB day; marks the day when an entry comes back through it) |
| Direction | `TrendSMA(tf, n, back)` (close above an SMA higher than `back` candles ago; 0 = n/5), `EMAAgree(tf, fast, slow)` (fast and close above slow), `Agree(a, b)` (a above b = long), `Momentum(tf, bars)`, `OnlyLong()`, `OnlyShort()` |
| Triggers | `PullbackTo(tf, level)` with `.CameFrom(bars, dist)`, `Breakout(tf, n)` (Donchian), `Impulse(tf, bars, dist)`, `Cross(tf, fast, slow)` (EMAs), `CloseCross(tf, level)`, `CloseBeyond(upper, lower)`, `Custom(name, func(c) bt.Side)` |
| Stop and target | `ATRStop(tf, n, x)`, `StopDist(dist)`, `StopAt(level, min)`, `SwingStop(tf, bars, buffer, min)`, `MaxStop(x)`, `RR(r)`, `ATRTarget(tf, n, x)`, `TargetDist(dist)`, `TargetAt(level)`, `RequireSLTP()` |
| Manage (in `ManageOpen`) | `BreakEven(atR)`, `Trail(afterR, distR)`, `TimeStop(bars)`, `CloseAt(hour, reason...)` (WIB), `CloseIf(reason, func(c) bool)`, `ExitOnTrendFlip(dirs...)` (none = the chain's direction steps); each acts on `c.Position()`, the position being managed |
| Orders and positions | `EntryLimit(level)` / `EntryStop(level)` with `.Expire(n)` and `.Tag(s)` (the entry becomes a pending order at level; Entry = level for the stop and target steps after it), `MaxOpen(n)` (hold up to n positions; stops the chain while n are open and pending), `OnePerSide()` (no second position or order on a side) |
| Values | `P`, `Const`, `EMA`, `SMA`, `RSI`, `ATR`, `MACDHist`, `Close`/`Open`/`High`/`Low`, `SwingLow`/`SwingHigh(tf, bars)`, `Entry()`, `Prev(v, tf)`, `Mirror(long, short)`, `Func(name, f, needs)`; `.Plus .Minus .Times .Div .Max .Min` |

**Several positions in a chain.** By default a chain enters only while
flat (no position open, no order pending), as before. `steps.MaxOpen(n)`
lets it hold n: `ManageOpen` then runs its steps once for each open
position, oldest first, with that position as `c.Position()` /
`c.Current()` and its own `c.Held()`, and aims a Close or MoveStop a step
returns at that position (`CloseID`, `MoveStopID`). Then, while open
positions plus pending orders are fewer than `c.MaxOpen()`, it also calls
the entry steps; every answer goes out together (`bt.Do`). `EntryLimit` /
`EntryStop` turn the entry into a pending order:

```go
bt.Chain("pullback_limit", "H1", "H4").Use(
	steps.ManageOpen(steps.TimeStop(69)), // per position
	steps.MaxOpen(2),
	steps.Hours(14, 23),
	steps.EMAAgree("H4", 20, 50),
	steps.Custom("above_ema", aboveEMA),
	steps.EntryLimit(steps.EMA("H4", 20)).Expire(4).Tag("ema20"),
	steps.SwingStop("H1", 12, steps.ATR("H1", 14).Times(0.5), steps.ATR("H1", 14)), // from the limit price
	steps.RR(2),
).Build()
```

The examples `big_trend_chain`, `breakout_retest_chain` and
`moving_average_chain` (`examples/chains.go`) are the three examples as
chains; tests show each trades exactly like its hand-written twin (count,
times, prices, P/L, walk-forward), and san_research checks the same on the
whole cached XAUUSD history.

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
   --out string             write summary.txt, trades.csv, missed.csv, equity.csv, equity.png, decisions.jsonl, candles.csv (and walk.csv) and run.json here

run only:
   --instrument string      e.g. XAUUSDm (Config.Instrument)
   --tf string              timeframes, step first, e.g. M15,H1,H4; empty = the strategy's (Config.Timeframes)
   --risk float             percent of the balance risked per trade (Config.Risk) (default: 1)
   --walk string            walk-forward TRAIN:TEST, e.g. 3m:1m (Config.Walk)
   --param NAME=VALUE       overriding a strategy number, repeatable (Config.Params)
   --max-open int           positions open at once; 0 = the strategy's own, else 1 (Config.MaxOpen)
   --max-open-risk float    max total risk: percent of the balance the open positions lose at their stops (Config.MaxOpenRisk)

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
| `trades.csv` | one row a trade: leg, strategy, instrument, side, signal/open/close times in UTC and WIB, entry, exit, stop, final stop, target, lots, spread, risk, gross, commission, swap, net, R, reason (`stop`, `target`, `close: ...`, `closed at end`), then `position_id`, `order_id` (empty for a market entry), `order_type` (`market`, `limit`, `stop`), `tag` (added at the end, so old readers keep working) |
| `missed.csv` | signals and orders that did not open, with time (UTC, WIB), side and reason, then `order_id`, `order_type`, `tag` |
| `equity.csv` | balance and equity after every step |
| `equity.png` | 1200x600: equity (blue) and balance (grey), money on the left, UTC dates below |
| `walk.csv` | with walk-forward: each window's dates, params, trials, train and test results |
| `decisions.jsonl` | the decision log: one JSON line per thing that happened (below) |
| `candles.csv` | the step candles of the range that ran: `instrument,timeframe,time_utc,open,high,low,close,volume,spread` (bid OHLC, open time in UTC, spread = ask - bid at the close); legs on one instrument and timeframe once |
| `run.json` | what ran and its stats (below); written last, so a folder with it is a finished report |

The extra files are for San Tower's Backtest Analysis page, which opens a
folder of reports read-only and charts them offline (the user, 2026-11-15:
"It can open directory that contain report log that generated by
backtest").

`run.json` (`RunSchema` = 2; snake_case; times RFC 3339 UTC; 2 added
`legs[].max_open` and the columns and kinds of several positions and
orders, nothing else moved):
`schema`, `kind` (`run` / `portfolio`), `approach` (the name), leg 0's
`instrument`, `timeframes` and `params`, `ran_at`, `requested` and `range`
(`from`, `to`, `notes`), `account` (`balance`, `currency`, `leverage`,
`max_open`, `max_open_risk`, `max_daily_loss`, `day_tz`), `costs` per leg
(the four texts as in summary.txt plus `overridden`: which are not
Exness's), `legs` (strategy, instrument, timeframes, risk, params; in a
portfolio also each leg's `stats` and `contribution` %), `stats` (the
`Stats` fields in snake_case; `profit_factor` is a number or `"inf"`),
`walk` with walk-forward (`train`, `test` as Go durations, `windows`,
`out_of_sample`) and `files`.

`decisions.jsonl` keeps only the candles where something happened (the
user, 2026-11-15: "2b"): `{"time", "leg", "kind", "side", "details",
"reason"}`, in time order.

| Kind | Time | Details | Reason |
|---|---|---|---|
| `signal` | the signal candle's close | `action` (1 buy, 2 sell, 3 close, 4 move stop), `close`, `stop`, `target` | the signal's own reason and the step's notes, joined by `; ` |
| `note` | the candle's close | none | the notes of a step that waited |
| `entry` | the fill (next open) | `price`, `stop`, `target`, `lots`, `risk`, `spread` | none |
| `stop_move` | the next open | `old_stop`, `stop` | none |
| `exit` | the close time of the trade | `price`, `entry`, `lots`, `net`, `r` | `stop`, `target`, `close: ...`, `closed at end` |
| `missed` | when it would have filled | `stop`, `target` (`order_id` for an order) | the skip reason |
| `order_placed` | the open it went live on | `order_id`, `price`, `stop`, `target`, `type` (1 limit, 2 stop), `bars` | e.g. `buy limit (tag)` |
| `order_filled` | the open of the candle it filled in | `order_id`, `position_id`, `price`, `type` | as placed |
| `order_cancelled` | when cancelled | `order_id`, `price`, `type` | the reason, or `end of the run` |
| `order_expired` | the close of its last candle, or the open at `Until` | `order_id`, `price`, `type` | `after n candles`, `until ...` |

`entry`, `exit` and `stop_move` also carry `position_id` (and `order_id`
for a position from an order); a `signal` for one position carries `id`.
A `Do` is one `signal` line per action.

Skip reasons (`Stats.Skipped`): `too small` (the risk buys less than the
minimum lot), `margin`, `max open`, `max open risk`, `daily loss`,
`market closed` (outside the instrument's sessions), `invalid levels`
(stop or target on the wrong side of the fill or the order price),
`expired` and `cancelled` (pending orders that ended unfilled; one still
pending at the end of the run is cancelled without a missed line).

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
- **Pending orders** (`BuyLimit`, `SellLimit`, `BuyStop`, `SellStop`). An
  order asked for at candle t's close goes live at t+1's open and can fill
  from t+1 on, on each candle's range, oldest order first, after the open
  positions' stops and targets in that candle:
  - **buy limit**: fills when the ask low is <= the limit, at the limit;
    a candle that opens through it (ask open <= limit) fills at the ask
    open, which is better. **Sell limit**: the mirror on the bid (bid high
    >= limit; a bid open >= limit fills at the open).
  - **buy stop**: fills when the ask high is >= the stop price, at that
    price; a gap open beyond it fills at the ask open, which is worse.
    **Sell stop**: the mirror on the bid (bid low <= price, or the open).
  - **Slippage** is added to stop-order fills (as to market fills and
    stops). Limit fills get none: a limit order never fills worse than its
    price (as targets).
  - **The fill candle, worst case** (the user, 2026-11-15: "2a"): if that
    candle's range also reaches the position's stop, the stop is hit in
    that candle (at the stop price, slippage added), and it wins over the
    target. A fill at the open has the whole candle after it, so the stop
    and the target count as for a market entry (gaps included). A stop
    order filled inside the candle may also take its target there (the
    price went through the order on its way to it); a limit filled inside
    never takes its target in its fill candle, since that side of the
    range may have come before the fill.
  - Each order **fills once** (the user, 2026-11-15: "4a"); a fill refused
    by a limit or by rule #1 at the fill price removes it too. `Expire(n)`
    counts step candles (the n after the one that asked for it), `Until(t)`
    stops it filling in a candle that opens at or after t; both are
    recorded as `expired`, `Cancel`/`CancelAll`/`CloseAll` as `cancelled`.
    Outside the sessions orders wait (no fill), and their candles still
    count.
  - A position from an order opens at the candle it filled in
    (`Trade.OpenTime` is that candle's open; its swap starts there).
- **Several positions.** Each is its own trade: own sizing at its fill,
  commission, swap, spread and stop/target checks, in open order. Equity,
  used margin and `MaxOpenRisk` sum all of them. `Close(reason)` with
  several open closes them all at the next open; `CloseID` one.
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
- **Positions.** One per leg unless `MaxOpen` allows more (then longs and
  shorts may be open together). No partial closes, repeating or grid
  orders (each order fills once); reversing is `Close` then the opposite
  signal, or both in one `Do`.
- **Inside a candle** the order of events is unknown: open positions'
  stops and targets are checked before pending orders fill, and sizing at
  a fill uses the balance after those exits. Limit fills on a touch (ask
  low = limit) are optimistic for a real market's queue.
- **Candle resolution.** Fills are judged from OHLC only; the stop wins
  when stop and target are in one candle. A lower step timeframe gives
  finer fills.
- **MN** candles close at the next calendar month's start; `ParseSpan`'s
  `m` is 30 days.
- **Broker server time** is ignored: all times are UTC as Exness sends
  them. Sessions in `Session()` are Tokyo 09-18, London 08-17 and New York
  08-17 local time.
- **Walk-forward** tunes leg 0 of a `Run` only, by one coordinate-search
  pass over a fixed x0.5..x1.5 grid (not a full grid search). The grid
  rounds whole defaults (`DefaultGrid`); a `GridTuner` strategy (every
  chain) may give a param another grid, e.g. `FineGrid` for fractions.
- **Cache** holds candles only for the Exness source; CSV is read as is.
  A run whose `To` is now tops up the current month on every run (a few
  requests); a range fully in the past makes none.
- **Report size.** `candles.csv` holds the step timeframe only (higher
  timeframes are built from it by a reader); a year of M15 is about 25k
  rows (2 MB). A strategy that calls `Note` on every candle makes a
  `note` line per candle.
- Additions beyond the agreed field list: `Config.Timeframes` /
  `Leg.Timeframes` (for `--tf`), `Leg.Params`, `Result.Missed` with
  `missed.csv`, and `CostReport` notes.

## Files

| File | What |
|---|---|
| `strategy.go` | `Strategy`, `Starter`, `Tunable`, signals, `Registry` |
| `context.go` | `Context`: candles, forming candle, position, account, indicators, `Note` |
| `decisions.go` | `Decision`, the decision log and `decisions.jsonl` |
| `runfile.go` | `RunInfo`, `CandleSet`, `run.json` and `candles.csv` |
| `indicators.go` | incremental EMA, SMA/Bollinger, RSI, ATR, MACD, swings |
| `config.go` | `Config`, `Costs`, `Walk`, `Portfolio`, `Leg`, defaults, `Validate` |
| `engine.go` | loading with warm-up, the event-clock simulation, fills, sizing, costs |
| `live.go` | `Live`: the engine stepped one closed candle at a time (paper trading) |
| `walk.go` | walk-forward windows and tuning |
| `data.go` | `DataSource`, the Exness source, CSV |
| `cache.go` | the disk cache |
| `instrument.go` | `Instrument`, `FromExness`, sessions |
| `candle.go`, `clock.go` | timeframes, WIB, sessions, rollovers, `Last`, `ParseSpan` |
| `result.go`, `report.go`, `chart.go` | stats, text/CSV reports, the PNG |
| `chain.go`, `chain_build.go`, `chain_run.go` | chain strategies: `Chain`, `Step`, `Link`, `Define`, `Decl`, `ParamDef`, `RequireSLTP` |
| `steps/` | the built-in chain steps and value expressions |
| `examples/` | three example approaches, and the same three as chains |
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
