# san_research

Data-first market research on Exness candles, starting with gold
(XAUUSD) (the user, 2026-11-15: "Let's true research about xauusd"). Go
programs on san_backtest's data and cache, plus a markdown report with
charts (the user, 2026-11-15: "1a"). It never trades. The research commands
read the candle cache only and make no Exness calls; only `fetch` (below)
calls Exness, for read-only candles.

## Run

```sh
go build -o <scratch>/san_research.exe ./cmd/san_research
<scratch>/san_research.exe xauusd --out D:/research/marketing/trade_research/XAUUSD/research-2026-11-15 --backtests
```

Flags: `--cache` (default `%LOCALAPPDATA%\hermes\cache\scratch\sbt\backtest_cache`,
san_backtest's cache), `--instrument XAUUSD`, `--from 2022-05-02`, `--to`
(default: the cache's newest `covered_to`), `--backtests` (also rerun the
approaches; without it the results section is rebuilt from
`backtests/summary.json`). A full run takes about 15 seconds.

Output in `--out`: `report.md`, `charts/*.png`, and with `--backtests`
`backtests/<approach>/<run>/` (san_backtest report folders: real, harsh,
2022 ... 2026; `real` carries the walk-forward) and `backtests/summary.json`.

The trend approaches (the user, 2026-11-15: "Can you summary the approach
and create better approach in code ?") have their own command:

```sh
<scratch>/san_research.exe approaches --out D:/research/marketing/trade_research/XAUUSD/approaches-2026-11-15
```

Flags: `--cache`, `--instrument`, `--from 2022-05-02`, `--to`, `--only
daily_trend,pullback_trend` and `--param name=value,...` (overrides the
candidates' defaults for sensitivity checks; send those to a scratch
`--out`) and `--chains` (runs the candidates' san_backtest chain versions,
`trend_chains.go`, instead of the hand-written ones: same trades, so the
same summary.json and results.md; only the decision notes differ, since a
chain logs the path each eventful candle took), and `--demo` (runs
`pullback_trend` next to `pullback_limit`, `trend_limit.go`: the same
rules with a buy limit at the H4 EMA20 that expires after 4 H1 candles and
up to two positions, a demonstration of san_backtest's pending orders;
research only, write it to a scratch `--out`). It writes `backtests/<approach>/<run>/` (real with walk-forward,
harsh, each year, and `real-100k`, information only), `backtests/summary.json`
and `backtests/results.md`, where the pass rule (`trend_rule.go`) is applied.
About 10 seconds; the summary and verdict are in that folder's `summary.md`.

### Scalping in San's window (M1)

The scalping study (the user, 2026-11-15: "So usually, I take 8 pm and
leave 12") works on M1 inside 20:00-24:00 WIB:

```sh
set -a; . D:/research/marketing/.env.agent; set +a   # only for fetch
<scratch>/san_research.exe fetch --tf M1 --from 2024-05        # resumable, read-only candle calls, counts them
<scratch>/san_research.exe fetch --tf S15 --from 2026-10       # optional, for the stop-first check
<scratch>/san_research.exe scalping --out D:/research/marketing/trade_research/XAUUSD/scalping-2026-11-15 --backtests
```

- `fetch` fills san_backtest's cache month by month through its Exness
  source and `Cache` (whole months; a covered month costs no call), paces
  calls (`--pace 150ms`) and waits out HTTP 429 itself. Exness keeps M1 only
  from 2024-05-01 and S15 for about 2.5 weeks; a month before that is written
  as an empty, covered file, so start at 2024-05. M1 is about 3 MB a month.
- `scalping` (about 5 s, plus about 70 s with `--backtests`) runs the studies
  of `scalp_study1.go` .. `scalp_study5.go` (activity and spread by WIB
  minute, pushes and autocorrelation, the 20:00 opening range against a
  random-walk baseline, the US data minute at 08:30 New York with DST via
  America/New_York, with- vs against-trend pushes), the one-position scalp
  simulator (`scalp_sim.go`, `scalp_simreport.go`: real per-minute spread,
  stop first, flat at 00:00 WIB, +$0.10 slippage harsh run, S15 check of
  candles that touch SL and TP) and, with `--backtests`, the closest rule as
  a san_backtest chain on M1 (`scalp_backtest.go`: real with walk-forward
  3m:1m, harsh, each year). Without `--backtests` the backtest section is
  rebuilt from `backtests/summary.json`. Data files: `data/push_cells.csv`,
  `data/sim_results.csv`.
- `Offline`'s clock and the approaches' first month ignore the short
  timeframes (M1, seconds) when M15 or longer ones are cached, so adding M1
  changes nothing in the other commands.

## What it does

- `data.go`: `Offline`, a san_backtest `Cacheable` that serves only the
  cache (its clock is frozen at the newest `covered_to`, so the cache never
  tops up, and a missing candle is `ErrNotCached` instead of a call);
  `Load` (M15, H1 through `bt.NewCache`), `Daily` (trading days = UTC
  date, Sunday evening counts as Monday), `Resample`.
- `stats.go`: mean, SD, quantiles (type 7), t-stat with a normal p-value,
  seeded bootstrap CI, Bonferroni, Benjamini-Hochberg, autocorrelation,
  proportion z-tests, Wilder ATR, `GroupBy`.
- `study1.go` ... `study5.go`: the studies (volatility by WIB hour, session
  and weekday; direction with corrections; trend vs mean reversion, the
  Asian range break, autocorrelation; key levels against a control; spread
  and swap). `levels.go`: touch detection and break/bounce resolution.
- `report.go`, `sections.go`: findings with STRONG/WEAK/NOISE labels, the
  summary, the approach ideas and the verdict.
- `chart.go`, `heatmap.go`: PNG bar charts, heatmaps and histograms with the
  standard library. The one dependency, `golang.org/x/image` (already used
  in the repo), gives the basicfont bitmap font for labels.
- `approaches.go`: the approaches as san_backtest strategies
  (`asian_high_fade`, `asian_high_fade_overlap`) and the benchmark
  `control_line_fade`; `Register` adds them to a `bt.Registry`.
  `backtest.go` runs them (real, harsh 0.40/0.10, walk-forward 3m:1m, by
  year; balance 10000, risk 1%, leverage 2000) and keeps the folders small
  (daily equity points, candles only around trades).
- `trend.go`: daily bars built from H4 (`dailyBars`, incremental; UTC days,
  weekend candles count as Monday), daily mean/channel/ATR, the daily trend
  (close vs the 50-day mean, mean vs 10 days ago), the overlap window
  (19:00-23:00 WIB) and the active hours (14:00-23:00 WIB).
- `trend_approaches.go` (`daily_trend`, with the shared position tracking
  and the 1R trailing stop) and `trend_others.go` (`overlap_momentum`,
  `pullback_trend`, the benchmark `buy_and_hold_xau` run at 50% risk with a
  50% stop, about 1x the balance). All step on H1 with H4 context.
- `trend_chains.go`: `daily_trend`, `overlap_momentum` and
  `pullback_trend` written as san_backtest chains (built-in steps plus a
  small adapter for the daily bars), `TrendChains`. `trend_chains_test.go`
  checks they trade exactly like the hand-written ones on made markets
  and, when san_backtest's cache is on the machine, on the whole cached
  history (with san_backtest's three examples and their chains).
- `trend_run.go`: `RunTrendApproaches`, with `floorSource`, which clamps
  queries to the first cached month so a warm-up that reaches before the
  cache starts the run later (noted) instead of failing. `trend_rule.go`:
  the pass rule fixed before the first run, and `results.md`.

## Conventions

- Times of day in the report are WIB (the user, 2026-11-15: "3 my Timezone
  is jakarta"); candles stay UTC inside.
- Label rule: STRONG = survives FDR within its family and has the same
  sign in at least 4 of 5 years; WEAK = one of those or raw p < 0.01;
  NOISE = the rest.
- Every trade idea carries a stop-loss and a take-profit (San's rule #1).
