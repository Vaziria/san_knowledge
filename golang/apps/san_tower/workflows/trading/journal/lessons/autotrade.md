# Lessons from autotrade

The four trades autotrade closed before the trade tick workflow took over
(autotrade's knowledge graph, 2026-10-30), with the lessons it wrote. The
approaches' descriptions already hold what they taught.

## autotrade-1

BTCUSDT short @ 82473.7, stop 82807.8, target 81614.1, opened 2026-10-09
10:46 UTC, closed 11:39 UTC: -6.31 USDT (-1.56R). Approaches trend_pullback,
big_trend, moving_average.

Stopped out at -1R plus slippage (-1.66R net): a 15m pullback short with the
stop only 334 away, under one 1h ATR (464), while the 1d trend was still up.
Best point only +0.29R; the 11:00 squeeze to 83,500 ran straight through the
stop, which filled ~120 worse on the thin testnet book. Lessons: on a
lower-timeframe entry place the stop beyond structure and at least one ATR of
the timeframe above; don't short into a support cluster (82,180-82,500, 4h
EMA200) while the 1d is up; expect stop slippage on spikes.

## autotrade-2

BTCUSDT short @ 82995.1, stop 83300 (trailed to 83010), target 82380, opened
2026-10-09 14:55 UTC, closed 16:54 UTC: +1.28 USDT (+0.35R). Approaches
resistance_rejection, big_trend, positioning.

Closed by hand at the user's request (close all positions in profit) at
82,803 after 119 min: +1.28 USDT net (+0.35R), +2.31 before fees; target
82,380 not reached. What worked: the rejection at the 83,245-83,275 cluster
with falling OI (-4.5% 24h) in a 1h/4h downtrend gave a clean lower-high
sequence (83,269 > 83,119 > 83,030 > 83,003 > 82,980); the trade never went
worse than -0.18R and reached +1.06R. Trailing the stop above each lower high
while keeping it at least 1x 15m ATR from price (83,300 > 83,150 > 83,010)
cut the risk to about break-even without getting clipped; the coded
break-even stop (82,912) would have sat inside the retest of the 5m EMA20/BB
mid. What to do differently: the 1.8R target lay beyond the 82,600-82,672
support, and the move stalled there at +1R with fading volume and the 4h MACD
histogram turning up. When a support cluster sits between entry and target,
either target the cluster or take profit there once momentum fades. Fees: at
1,000 USDT of notional and a 305 stop, 1R is only 3.66 USDT and round-trip
fees (~0.9 USDT) cost about 0.25R per trade, so a trade needs a clear 1.5R+
path to be worth it.

## autotrade-3

BCHUSDT long @ 280.21, stop 279.33, target 282.1, opened 2026-10-10 09:43
UTC, closed 10:05 UTC: -3.26 USDT (-1.04R). Approaches breakout_retest,
positioning.

What happened: a long on the retest of BCH's 279.8-280.14 ceiling after a
9.89x breakout close. The retest held for four 5m candles (closes
280.16-280.47), but price never got going. The best point was +0.49R, two
pushes stalled under 280.66, and taker flow cooled from 1.36 to 1.02. The
10:05 candle then flushed from 280.29 through 279.67 to the 279.33 stop (low
near 278.8, -1.65R) while BTC/ETH were flat. Why: the breakout was fuelled by
short covering (OI falling, funding back to ~0), not by new longs. Once the
covering was done, nobody was left to buy against the 1h/4h downtrend, and
the target sat at the 1h EMA50 against that trend. OI turned up on the flush:
new shorts. Lessons: (1) On a breakout against the 1h/4h trend, short
covering alone fuels only the breakout candle. Before entering the retest,
require OI to rise again after the breakout (new longs), or the 1h trend to
be flat or up. (2) Manage it: if price stalls twice under the breakout high
within 3-4 candles while taker flow fades to ~1.0, close near entry or
tighten to break-even; don't wait for the stop. (3) A single huge-volume
candle in a quiet market can be a stop run, so wait for a second candle that
holds above the level with volume.

## autotrade-4

BNBUSDT long @ 752 (a resting limit), stop 749.7, target 756.6, opened
2026-10-10 15:17 UTC, closed 15:27 UTC: -1.61 USDT (-0.53R). Approach
breakout_retest.

What happened: a resting limit long at 752.00 on BNB's flag low, after the
14:25 breakout over the 751.4-751.5 day high and the 4h EMA20/EMA200
(752.29-752.64) on 3.9x volume. Placed at the user's request for trades, at
~1.51x net, the bare minimum. It filled at 15:17 and never went green (best
+0.00R, worst -0.25R). The next candle closed 751.46, back under the 4h EMAs
and the 751.5 old high, and it was closed at 751.55 after 9 minutes: -1.61
USDT (-0.53R) after fees, instead of about -3.5 at the 749.7 stop. Why it
lost: (1) the entry sat under the level the thesis depended on: 752.00 is
below the 4h EMA cluster, so a fill there already meant the breakout over the
4h EMAs was failing. (2) The flag was not holding. It made seven lower highs
(754.16 > 753.87 > 753.50 > 753.41 > 752.66 > 752.25 > 752.00) with 5m MACD
turning negative, so the limit got filled by sellers breaking the flag. (3)
The breakout was short covering, the autotrade-3 pattern again: BTC's OI fell
-0.32%/4h in the same move and BNB's OI was only +0.03%. The 'flat 1h'
condition was met, but only just. Lessons: put a resting limit at or above
the level the thesis needs, never below it; don't rest a long at the bottom
of a flag that makes lower highs (wait for a higher low or a 5m close back up
from the level); when the 1.5x check only passes at the edge, the setup is
weak. What worked: naming the early-exit level in advance and acting on the
first close through it cut the loss to half of 1R.
