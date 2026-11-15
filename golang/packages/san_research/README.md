# san_research

Data-first market research on Exness candles, starting with gold
(XAUUSD) (the user, 2026-11-15: "Let's true research about xauusd"). Go
programs on san_backtest's data and cache, plus a markdown report with
charts (the user, 2026-11-15: "1a"). It never trades: it reads the candle
cache only and makes no Exness calls at all.

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

## Conventions

- Times of day in the report are WIB (the user, 2026-11-15: "3 my Timezone
  is jakarta"); candles stay UTC inside.
- Label rule: STRONG = survives FDR within its family and has the same
  sign in at least 4 of 5 years; WEAK = one of those or raw p < 0.01;
  NOISE = the rest.
- Every trade idea carries a stop-loss and a take-profit (San's rule #1).
