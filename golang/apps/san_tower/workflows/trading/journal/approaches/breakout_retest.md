# Breakout Retest (`breakout_retest`)

A level that capped price several times (3+ touches) breaks on a close with heavy volume (2x+), and the first retest of the level holds on a 5m close. Enter on the retest, not the breakout candle; stop under the retest low and the level by at least the stop floor (max of 15m ATR and 0.3%); target in front of the next higher-timeframe level (1h/4h EMA or swing). A close back inside the old range fails it. Covering against a crowded short side can start the break, but alone it fuels only the breakout candle (autotrade-3, autotrade-4). Against the 1h/4h trend, also require OI rising again after the break (new money) or a flat/up 1h trend. A resting limit entry must sit at or above the level the thesis depends on, never under it: a fill below it means the level already failed (autotrade-4). Don't rest a limit at the bottom of a flag that is making lower highs; wait for a higher low or a 5m close back up from the level. Manage it: if two pushes stall under the breakout high within 3-4 candles while taker flow fades to ~1.0, close near entry or move the stop to break-even; name the early-exit close in advance and act on the first close through it.

## Trades and lessons

A record of a few trades is mostly luck: weigh it by the trade count. From autotrade, 2026-10-09 to 2026-10-10: 2 closed trades, 0 won, -0.79R average, -4.87 USDT net.

- autotrade-3: BCHUSDT long @ 280.21, stop 279.33, target 282.1, 2026-10-10 09:43-10:05 UTC: -3.26 USDT (-1.04R), stopped out. Its lesson: [lessons/autotrade.md](../lessons/autotrade.md), autotrade-3.
- autotrade-4: BNBUSDT long @ 752 (limit), stop 749.7, target 756.6, 2026-10-10 15:17-15:27 UTC: -1.61 USDT (-0.53R), closed early. Its lesson: [lessons/autotrade.md](../lessons/autotrade.md), autotrade-4.
