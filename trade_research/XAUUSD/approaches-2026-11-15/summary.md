# XAUUSD approaches: what failed, and three new candidates

San asked for a summary of the approach so far and a better approach in
code (the user, 2026-11-15: "Can you summary the approach and create
better approach in code ?"). This is research only: no orders were placed
and none are suggested here except as a DEMO forward-test.

- **Data:** Exness XAUUSD candles (M15, H1, H4; bid and ask) from
  san_backtest's cache, 2022-04-01 to 2026-11-13. Daily bars are built
  from H4 candles (UTC date; Saturday and Sunday candles count as Monday).
- **Account:** balance $10,000, risk 1% per trade, leverage 1:2000,
  Exness's real costs (each candle's spread, the long swap of $47.17 per
  lot per night, triple on Wednesday; shorts pay none).
- **Rule #1** (trade_research/BTCUSD/2026-11-14/session-log.md): every
  trade idea has both a stop-loss and a take-profit. Every approach here
  sets both on every entry.

## 1. Approaches tested so far

All on XAUUSD, Exness real costs, $10k, 1% risk, 1:2000, May 2022 to
Nov 2026.

### moving_average (san_backtest example)

- **Rules:** M15 only, any hour. Long when EMA20 > EMA50, the close
  crosses back above EMA20, RSI(14) > 50 and the MACD histogram rises;
  short is the mirror. Stop at the EMA50 (at least 1 M15 ATR away), target
  2R. No time exit.
- **Results:** PF 0.89, net -82%, max drawdown 85%. It lost every full
  year. Harsh costs: -97%. Walk-forward out-of-sample PF 0.96.
- **Why it failed:** an M15 trade risks about one M15 ATR (a few dollars),
  so the spread and slippage are a large slice of every stop. The EMA
  cross itself has no direction edge on M15 (the research found M15
  direction to be noise), so costs decide the result.

### big_trend (san_backtest example)

- **Rules:** steps on M15. Trade only the way H1 and H4 agree (EMA20 vs
  EMA50 and the close vs EMA50 on both). Enter on an M15 pullback that
  touched the M15 EMA20 and closed back over it. Stop 1.5 M15 ATR, target
  2R; stop to break-even at +1R; close when H1 turns against the trade.
- **Results:** PF 0.92, net -54%.
- **Why it failed:** the higher-timeframe filter is the right idea, but
  the trade itself is still an M15 trade with an M15-sized stop, so it is
  stopped by ordinary noise and pays the spread many times. The filter
  helped (it lost less than moving_average) but not enough.

### breakout_retest (san_backtest example)

- **Rules:** M15 with H1 swings. A level that capped price 3+ times in the
  last 40 candles breaks on a close with 2x average volume; enter on the
  first retest that holds within 8 candles. Stop beyond the retest and the
  level (at least max(ATR, 0.3%)); target the next H1 swing, else 2R.
- **Results:** too few trades to judge, and it lost over 6 months.
- **Why it failed:** the setup is rare, so there is no sample. A 6-month
  loss alone is not a result either way.

### asian_high_fade and asian_high_fade_overlap (san_research)

- **Rules:** M15, short only. Sell the first touch from below of today's
  Asian high (00:00-06:45 UTC, 07:00-13:45 WIB) when the candle closes
  less than 0.5 ATR above it. Touches 14:00-02:45 WIB (overlap variant:
  19:00-22:45 WIB). Stop 1 ATR above the level, target 1 ATR below. Close
  at 03:30 WIB at the latest.
- **Results** (from research-2026-11-15/report.md):

  | approach | trades | PF real | net % | max DD % | WF OOS PF | harsh net % | years + |
  | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
  | asian_high_fade | 670 | 0.80 | -42.7 | 47.8 | 0.76 | -76.1 | 0 of 5 |
  | asian_high_fade_overlap | 403 | 0.75 | -38.2 | 40.3 | 0.73 | -58.7 | 0 of 5 |

- **Why it failed:** the Asian high does turn price back more often than a
  random line (STRONG), but by only a few points of bounce share, and a
  real stop is hit by any wick while the study counted closes. Shorting a
  market that doubled also fights the trend.

### control_line_fade (benchmark)

- **Rules:** the same fade on arbitrary lines (50k+17, 50k+33).
- **Results:** 1438 trades, PF 0.96, net -17.5%, max DD 41.7%, harsh
  -73.2%, WF OOS PF 0.94.
- **Why it matters:** the Asian-high fade lost MORE than fading random
  lines, so the level added nothing tradeable.

### Lessons that shape the new designs

1. **Intraday M15 patterns don't beat costs.** Every M15 approach lost,
   and harsh costs made each one far worse. A trade must aim for a move
   many times the spread: H1 at least, better days.
2. **Direction is not in the clock.** Hour-of-day and weekday direction
   effects were noise. Time of day may decide *when* to act, never *which
   way*.
3. **The swap matters for longs.** $47.17 per lot per night (x3 on
   Wednesday), about 4.4% a year of gold's price; shorts pay nothing. A
   long held for weeks must earn more than that.
4. **Volatility timing matters for sizing and stops.** 19:00-23:00 WIB
   moves about 2x the Asian hours and has the tightest spreads; 03:00-06:00
   WIB is quiet with a wide spread at the 04:00 WIB rollover and the
   Monday open. Stops sized in the active window's ATR are realistic;
   entries at the rollover are not.
5. **Trend over days and weeks is the one big thing in the data.** Gold
   went from about 1,800 to about 4,000. The only direction finding that
   held at all (WEAK) is that a big H1 move up kept going a little in 4 of
   5 years. Everything new trades WITH the multi-day trend.

## 2. The pass rule (fixed before the first run)

Written before any of the new approaches was run. A candidate is worth a
DEMO forward-test only if ALL of these hold:

| # | check | threshold |
| --- | --- | --- |
| 1 | full history, real costs | PF > 1.2 |
| 2 | full history, harsh costs (spread 0.40, slippage 0.10) | PF > 1.05 |
| 3 | walk-forward 3m train : 1m test, out-of-sample months chained | PF > 1.1 |
| 4 | years run one by one (2022 ... 2026), real costs | profitable in at least 3 of 5 |
| 5 | full history, real costs | max drawdown < 30% |
| 6 | full history, real costs | at least 60 trades |

- **The walk-forward line is the honest number.** The defaults below were
  chosen from the research facts before any run, and were not changed
  after seeing a result. The walk-forward re-tunes each parameter every
  month on the 3 months before it, over san_backtest's grid (the default x
  0.5, 0.75, 1, 1.25, 1.5; whole numbers for whole defaults), and is then
  tested on the month after. That grid is the only parameter search.
- **Sizing caveat known in advance:** at $10,000 and 1% risk the trade
  may risk $100, and the minimum lot (0.01) moves $1 per $1 of gold. So
  any stop wider than about $100 is "skipped: too small". Gold's median
  daily range was about $20 in 2022-2023, $27 in 2024, $45 in 2025 and
  $88 in 2026, so stops measured in daily ATR are untradeable on this
  account in 2026. The skipped count is reported for every run; an extra
  run with a $100,000 balance (information only, not part of the rule)
  shows what the rules do when nothing is skipped.
- **History starts 2022-04-01 in the cache.** The new approaches need
  about 55 trading days of H4 history (50-day mean), so their full runs
  start in late June 2022, not 2022-05-02. No extra Exness calls were made
  to reach further back.

## 3. The new candidates (rules fixed before the first run)

Code: `golang/packages/san_research/trend.go` (daily bars, indicators,
windows), `trend_approaches.go` (daily_trend), `trend_others.go`
(overlap_momentum, pullback_trend, buy_and_hold_xau). All step on H1 and
read H4; days are built from H4. Every entry has a stop-loss and a
take-profit, and every signal writes a note to the decision log
(`decisions.jsonl`, shown in the tower's Backtest Analysis).

Shared definitions:

- **Daily trend up:** the last completed day closed above its 50-day mean
  of closes, and that mean is higher than 10 days ago. Down is the mirror.
  Otherwise no trend, no trade.
- **Overlap window:** H1 candles opening 19:00-22:59 WIB (12:00-15:59
  UTC); the order fills at the next candle's open.
- **Daily ATR:** mean true range of the last 14 completed days. **H1 ATR:**
  Wilder ATR(14) on H1.

### A. daily_trend (time-series momentum, swing)

- **Entry:** in a daily uptrend, an H1 candle in the overlap window closes
  above the highest high of the last 20 completed days: buy. Downtrend and
  the 20-day low: sell. At most one entry per trading day, one position at
  a time.
- **Stop-loss:** 2 daily ATR from the entry candle's close.
- **Take-profit:** 3R (3 x the stop distance).
- **Trailing:** once the best price since entry is 1R in profit, the stop
  trails 1R behind it (at +1R that is break-even).
- **Early exit:** a completed day closes back across the 50-day mean.
- **Why:** trend over days/weeks is the one big thing in the data; few
  trades with wide targets make the spread small against each trade.
  Longs pay the swap, which the backtest charges.
- **Tuned by the walk-forward:** trend 50, channel 20, stop_atr 2,
  target_r 3.

### B. overlap_momentum (intraday, with the trend)

- **Entry:** daily trend and H4 EMA20 vs EMA50 agree; an H1 candle in the
  overlap window closes at least 3 H1 ATR beyond the close 8 candles
  before, in the trend direction, and itself closes that way: enter that
  way. One entry per day.
- **Stop-loss:** 1.5 H1 ATR. **Take-profit:** 2R.
- **Exit:** at 03:00 WIB at the latest (the open after the H1 candle that
  opens 02:00 WIB), before 03:30 WIB: no swap, no rollover spread.
- **Why:** the overlap moves twice as much; study 3's WEAK finding (a big
  H1 move continued a little in 4 of 5 years), used only WITH the trend.
  3 ATR instead of the study's 4 because 4 is rare inside a 4-hour window.
- **Tuned:** trend 50, imp_atr 3, stop_atr 1.5, target_r 2.

### C. pullback_trend (swing of a few days)

- **Entry:** daily trend up and H4 EMA20 above EMA50; within the last 24
  H1 candles price closed at least 1 H1 ATR above the H4 EMA20; now an H1
  candle opening 14:00-22:59 WIB (London and New York) dips to the EMA20
  (low at or below it), closes back above it and closes up: buy. Short is
  the mirror. One entry per day.
- **Stop-loss:** the lowest low of the last 12 H1 candles (the pullback
  swing) minus 0.5 H1 ATR, at least 1 H1 ATR from the close. No trade when
  that stop is wider than 1.5 daily ATR.
- **Take-profit:** 2R.
- **Time stop:** close after 3 x 23 H1 candles (about 3 trading days).
- **Why:** buys weakness inside the daily trend, with a stop at a real
  swing instead of a fixed ATR, and a time stop that limits the swap.
- **Tuned:** trend 50, stop_buf 0.5, target_r 2, hold_days 3.

### Benchmark: buy_and_hold_xau

- Buy once at the first H4 step and hold to the end, paying the long swap
  every night. Rule #1 still applies: a disaster stop 50% below and a
  target at 3x the entry (neither is reached on this history). Run with
  risk 50%, so the 50% stop makes the position about 1x the balance in
  gold (no leverage). Not judged by the pass rule: it shows what the trend
  alone paid after swap.

### Runs

Each candidate: `real` (full history, Exness's costs, with walk-forward
3m:1m), `harsh` (spread 0.40, slippage 0.10), `2022` ... `2026` (each year
separately, real costs) and `real-100k` (information only: $100,000 so no
trade is too small). The benchmark: real, harsh and the years. Reports in
`backtests/<approach>/<run>/`, open them in the tower's Backtest Analysis;
`backtests/results.md` has every row and the rule applied by code
(`trend_rule.go`).

Rerun:

```sh
cd D:/research/marketing/golang/packages/san_research
go build -o $LOCALAPPDATA/hermes/cache/scratch/appr/sr.exe ./cmd/san_research
$LOCALAPPDATA/hermes/cache/scratch/appr/sr.exe approaches \
  --out D:/research/marketing/trade_research/XAUUSD/approaches-2026-11-15
```

## 4. Results

2022-06-16 to 2026-11-13 (the runs start once the 330 H4 warm-up
candles are there), $10,000, 1% risk, 1:2000. Every row and the rule's
check lines: `backtests/results.md` (made by code from
`backtests/summary.json`).

### Against the pass rule

| approach | trades | real PF | harsh PF | WF OOS PF | years + | max DD % | net % | verdict |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| daily_trend | **22** (fail) | 4.97 | 4.90 | 1.68 | 4 of 5 | 2.9 | +16.6 | FAIL (too few trades) |
| overlap_momentum | 91 | **0.93** | **0.89** | **0.99** | 3 of 5 | 9.5 | -2.4 | FAIL |
| pullback_trend | 146 | 1.27 | 1.20 | 1.17 | 3 of 5 | 10.4 | +24.2 | **PASS (narrowly)** |
| buy_and_hold_xau (benchmark) | 1 | - | - | - | 3 of 5 | 36.2 | +60.6 | not judged |

### Year by year (net %, real costs)

| approach | 2022* | 2023 | 2024 | 2025 | 2026* |
| --- | ---: | ---: | ---: | ---: | ---: |
| daily_trend | +2.2 (3 trades) | +3.8 (7) | +6.2 (7) | +5.4 (5; 14 skipped) | 0.0 (0; 18 skipped) |
| overlap_momentum | 0.0 | -1.0 | -5.6 | +4.2 | +0.7 |
| pullback_trend | +5.5 | +12.6 | +4.6 | **-0.6** | **-2.2** |
| buy_and_hold_xau | -9.0 | +3.1 | +15.5 | +45.7 | -12.3 |

\* partial years.

### Per approach, in plain words

**daily_trend: FAIL, but the most interesting result.** Its trades are
good (PF 4.97 real, 1.68 out-of-sample over 53 walk-forward trades) and
costs barely matter (harsh PF 4.90). But it made only 22 trades in 4.5
years, and 32 more signals were "skipped: too small": with a 2-daily-ATR
stop the 1% risk ($100) is less than the 0.01 minimum lot risks, so since
mid-2025 it cannot trade at all on a $10k account (0 trades in 2026).
With $100,000 (information only) it made 34 trades, PF 3.57, +28.1%, max
DD 3.4%. 22 or 34 trades is far too small a sample to trust a PF of 3-5:
the five best trades make 1,219 of its 1,664 dollars.

**overlap_momentum: FAIL.** PF 0.93 real, 0.99 out-of-sample. Following a
big H1 move with the trend in the overlap is about break-even before
costs: the trades closed at 03:00 WIB earned (PF 5.7), but stops lost more.
Study 3's WEAK follow-through (+0.43 ATR over 16 candles) is too small for
a 1.5-ATR stop and a 2R target inside one evening.

**pullback_trend: PASS, narrowly, with clear warnings.**

- Every check passes: real PF 1.27, harsh PF 1.20, walk-forward
  out-of-sample PF 1.17 (134 trades, +13.8%), profitable 2022, 2023 and
  2024, max drawdown 10.4%, 146 trades. Both sides made money (longs PF
  1.29 after $395 of swap, shorts PF 1.22). Exits: 52 targets, 81 stops,
  13 time stops.
- **The edge is fading.** 2025 (-0.6%) and 2026 (-2.2%) lost. The longs
  in 2026 had PF 0.52. Gold's daily range quadrupled since 2023, and the
  rules may fit the calmer years better.
- **It is not statistically strong.** Mean +0.16R per trade with a
  standard deviation of 1.41R: t = 1.4. A bootstrap of its trades gives
  an 8% chance that the true mean is 0 or worse. 21 of the 50
  walk-forward test months lost.
- **The neighbourhood is mixed.** This is information only: the defaults
  were not changed after it. Moving one parameter at a time to the next
  grid value gave real PF 1.06-1.30 and out-of-sample PF 0.97-1.21. Only
  4 of the 9 neighbours pass every check: trend 62, stop_buf 0.75,
  hold_days 4 and hold_days 5. A 38-day trend mean gives PF 1.06, and
  target 1.5R or 2.5R breaks the walk-forward. So the result sits on a
  small hill, not a plateau. (Rerun one with `--only pullback_trend
  --param hold_days=4 --out <scratch dir>`.)

**buy_and_hold_xau (benchmark).** Gold doubled (1,888.78 to 3,883.08),
which was +99.7% gross on 1x exposure, but the long swap cost -39.1% of
the balance, so the net was +60.6%, with a 36% drawdown. The trend was
the big thing, and holding it as a long paid a large share of it to
the swap. pullback_trend made +24% with a third of that drawdown at 1%
risk per trade. It is a different risk level, not a better return.

## 5. Verdict

- **One candidate, `pullback_trend`, passes the rule fixed in advance,**
  so by that rule it earns a DEMO forward-test, and nothing more. It
  passes by small margins, lost money in 2025 and 2026, and is not
  statistically significant. Treat the forward-test as the real test.
- `daily_trend` has the best trades but cannot be judged (22 trades) and
  cannot trade a $10k account at 1% in today's gold. Keep it as a research
  line (see next steps), not a plan.
- `overlap_momentum` does not work: drop it.

### DEMO forward-test plan: pullback_trend (demo account only, no real money)

- **Instrument:** XAUUSD on the Exness demo. Timeframes: daily bars (UTC
  days, weekend candles counted as Monday), H4, H1.
- **Trend filter (both must hold):**
  - Long: yesterday's daily close is above the 50-day mean of daily
    closes, and that mean is higher than 10 days ago.
  - Long: H4 EMA20 is above H4 EMA50.
  - Short: the mirror of both.
- **Setup:** within the last 24 H1 candles, an H1 close was at least 1 H1
  ATR(14) above the H4 EMA20 (short: below).
- **Trigger:** an H1 candle that opens between 14:00 and 22:59 WIB, with:
  - its low at or below the H4 EMA20 (short: high at or above);
  - its close back above the EMA20 (short: below);
  - its close above its open (short: below).

  Enter at the next H1 open. One entry a day, one position at a time.
- **Stop-loss:** the lowest low of the last 12 H1 candles minus 0.5 H1
  ATR (short: the highest high plus 0.5 ATR).
  - At least 1 H1 ATR from the entry.
  - Skip the trade when the stop is wider than 1.5 daily ATR(14).
- **Take-profit:** 2 x the stop distance (2R).
- **Time stop:** close after about 3 trading days (69 H1 candles) if
  neither level is hit.
- **Risk:** 1% of the balance per trade. A $10k account can take any
  stop up to $100 at 0.01 lot. Skip a trade that is too small; never
  round the lot up.
- **What to watch:**
  1. **Sample:** about 30 trades a year, so 3 months gives only about 8.
     Judge after at least 30 trades (about a year), not before.
  2. **Stop it early** if the demo loses more than 10% from its start
     (the backtest's worst was 10.4%), or if after 20 trades the average
     is below -0.2R.
  3. **The fills:** compare each demo fill with the backtest's next-open
     price. The backtest assumes the candle's spread and no slippage,
     and harsh costs cut the PF to 1.20.
  4. **The swap** on longs held over Wednesday (triple). Longs paid $395
     in total in the backtest.
  5. **The 2025-2026 decay:** compare the forward PF with those years
     (PF about 0.9), not with the full history (1.27).
  6. Re-run this report monthly (the command in section 3) and watch
     whether 2026's line turns positive.

### Next research directions

1. **Trend over weeks, sized for the account.** daily_trend's trades are
   the strongest thing found, but it needs either a bigger account or
   more risk than 1% at today's gold volatility. Test it on more history
   (gold back to 2010 from another data source, or Exness H4/D1 further
   back), and test a version with a narrower stop in H4 ATR, so it has
   hundreds of trades instead of 22.
2. **The same trend rules on other instruments** where Exness's swap is
   small or positive (EURUSD, indices, silver), to see whether the pullback
   edge is general or a gold-2023 accident.
3. **A news study** (US CPI, NFP, FOMC at 19:30-01:00 WIB): the overlap's
   big moves cluster on those days; check whether pullback_trend's losers
   are news days, and whether skipping them helps.
4. **Regime check:** split 2025-2026 by volatility (daily ATR as a % of
   price) to see whether the pullback edge fails only in the high-volatility
   regime. If so, it needs a volatility filter, tested out of sample.

## 6. Data and calls

- No Exness calls were made: 0 candle calls, 0 condition calls, and
  nothing trading or changing an account. Everything came from san_backtest's
  cache (covered to 2026-11-14T17:54:36Z) through san_research's offline
  source, which refuses anything not cached.
- The costs are Exness's current conditions (swap -471.7 points long, 0
  short, triple Wednesday) for the whole history; Exness changed swaps over
  the years.
- The spread is each candle's ask - bid at its close (about $0.20 on these
  entries); slippage is 0 in `real` and $0.10 per fill in `harsh`.
