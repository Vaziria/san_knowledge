# Autotrade tick

One pass of the loop in [docs/basic_flow.md](docs/basic_flow.md). Run it from the repo root, by hand or with `/loop`, shortly after a candle of the trading interval closes. The trading interval is the `interval` in `autotrade.yaml` and the snapshot's title. It is now 5m, with 15m, 1h and 4h as context. There are no coded signals: on 5m they lost about 0.33R per trade after fees in the backtest (0.13R on 15m), so every trade is your judgment.

The pairs are the `pairs` in `autotrade.yaml`, now four: BTCUSDT, ETHUSDT, BNBUSDT and BCHUSDT. Each pair is traded on its own: its own snapshot, its own position, its own decision each tick. The limits apply per position, so up to one position per pair can be open at once, each risking up to `max_risk_usdt`. The Risk Summary (`autotrade_portfolio`) shows what all of them have at stake together.

Pairs under `mt5.pairs` trade on a MetaTrader 5 demo account (MIFX) instead; there are none now. Their snapshot's portfolio heading says "MetaTrader 5 demo". For those pairs:
- Sizes are units of the symbol, not lots, and `usdt` is the account currency (USD). The limits are `mt5.max_position` and `mt5.max_risk`; the snapshot's Limits line shows them and the smallest position.
- There is no positioning section, so the Positioning step rests on price and volume alone. The volume is tick volume.
- The cost is the spread (the snapshot's quote line shows it) plus any broker commission, not a maker or taker fee. Count the spread in the net R check in its place.
- A limit entry is a plain limit order: a long's goes under the ask, a short's over the bid.
- Outside the symbol's trading hours (weekends, holidays) the notes say it is not trading: hold.
- When the terminal is closed, the tool reports "MetaTrader did not answer" for those pairs. Hold them and carry on with the others.

- By hand: type `/autotrade` (or send `follow golang/apps/autotrade/tick.md`) shortly after a candle closes. The snapshot's "Next close" line says when that is (in UTC; this machine's clock runs about 4 minutes fast). Text after the command, such as `/autotrade only manage the open position`, is a note for that tick.
- Scheduled: `/loop 5m /autotrade`

Between ticks nothing manages an open position. Its stop-loss and take-profit stay on the exchange, but nobody moves the stop.

**You are the strategy.** The tool's jobs:
- extract the data (`snapshot`)
- carry out your decision (`open`, `protect`, `close`, `hold`)
- keep the knowledge graph of every trade: the approaches it used, its analysis, the data snapshot, how it ended, and your lesson
- enforce the hard limits: test network only, a stop-loss on every trade, max size, max loss per trade, and stops that can only move closer

How to read the market and when to act is decided by you, fresh each tick. The snapshot's coded signals and position plan are a second opinion, not orders. Their backtest is in USAGE.md: about break-even before fees, losing after.

Use the `autotrade` MCP tools (`autotrade_snapshot`, `autotrade_open`, ...). Without them, the same commands run from the CLI: `golang/apps/autotrade/bin/autotrade.exe <command>` (on Linux, `bin/autotrade`). The actions name their pair: `pair` in the tools, `-symbol ETHUSDT` before the command on the CLI.

## Each tick

A tick has three phases, in this order ([docs/readme.md](docs/readme.md)): **Doing Analyze**, then **Doing Action**, then **Updating Autotrade Knowledge**.

### Doing Analyze

1. Take the snapshot: `autotrade_snapshot` (CLI: `snapshot`). Without a `pair` it gives one snapshot per pair, each with that pair's position and last decisions. If a pair's notes say a position closed since the last tick, its lesson is written in the last phase.
2. Form a view of each pair, top-down, and write it out before you decide:
   - **Context.** Direction and strength on the context timeframes (now 15m, 1h and 4h), from the EMAs, the momentum, and where price sits in its range. What did your last decisions say you were waiting for?
   - **Location.** Where is price against the key levels, the EMAs and the bands? Is there room to the next level in the direction you'd trade? Buying just under a level, or selling just above one, needs a reason.
   - **Positioning.**
     - Open interest rising with price means new money is behind the move. Falling means positions are closing.
     - Taker flow shows who is pushing.
     - The long/short ratios and funding show which side is crowded: a crowded side is fuel for a squeeze against it.
   - **Trigger.** What in the last closed candles of the trading interval says *now*? A move inside the forming candle is not a trigger.
   - **Second opinion.** Do the coded signals and the position plan agree? If you go against them, say why.
   - **Knowledge.** Before opening, read `autotrade_approach` for each approach you would use (CLI: `knowledge approach NAME`). It shows the approach's record and the notes of its past positions. Does a past lesson apply here? A record of a few trades is mostly luck, so weigh it by the trade count.

### Doing Action

3. Choose one action for each pair, naming the pair (`pair`):
   - **open**, only with all four:
     1. a thesis
     2. a trigger
     3. an invalidation level, with the stop just beyond it, and **at least one ATR of the first context timeframe from the entry, and never closer than 0.3% of price** (now 15m; read each pair's own ATR). A stop sized on the trading interval gets taken out by ordinary noise, and the fees eat a larger share of a small stop. In a quiet market the 15m ATR is only about 0.1% on BTC, so the 0.3% floor decides; that is near one 1h ATR. Time the entry on the trading interval, but place the stop by the structure of the timeframes above it.
     4. a target at a level, in front of the next major level (one that already stopped price, or a higher-timeframe EMA), at least 1.5x the risk after the round-trip fees

     **Fees.** The snapshot's portfolio shows them per fill (this testnet account: maker 0.02%, taker 0.04%). A market entry pays taker twice, about 0.08% per round trip; a limit entry pays maker in and taker out (the stop and target are market orders), about 0.06%. Count them in the check: net R = (distance to target − round-trip fees) / (distance to stop + round-trip fees), all from the entry price you will actually get.

     **Limit entries.** When the trigger is price coming to a level (a pullback, a retest, a rejection zone), enter with `limit`: a post-only order that rests in the book and pays the maker fee. A long's limit goes at or under the best bid, a short's at or over the best ask (the snapshot shows the book); one that would fill at once is refused. Its stop and target wait with it, so it is protected from its first fill. It is cancelled when unfilled after `expires_min` (default 15, three candles) or when the price reaches the target first. It shows under the portfolio as a limit entry waiting; when it fills it becomes the position (its open is dated at the fill). Each tick, keep it while the setup holds, or cancel it with `close` when the reason is gone. A partly filled entry keeps what filled and cancels the rest. Enter at market only when waiting would miss the trade, such as a breakout close that holds.

     Size it so the loss at the stop fits `max_risk_usdt`; the tool refuses anything larger (for a limit entry, measured from the limit price). Name the **approaches** the analysis used: `autotrade_open` with `approaches` (CLI: `-approach a,b`). The first one labels the trade.
     - Use the catalog (`autotrade_approaches`): `big_trend`, `moving_average`, `resistance_rejection`, `positioning`, and the coded setups when you take their signal.
     - Only for a genuinely new way of reading the market, define a new approach first with `autotrade_approach_define`: a short name, a title, and what it reads and when it says to act. Reuse names so each approach's record builds up.
   - **protect**: move the stop toward profit when the structure allows it, for example after a new swing forms or the trade reaches +1R. Moving a target is fine too.
   - **close**: the thesis is broken before the stop is hit, or price reached the target area and the move is fading. With no position, `close` cancels a waiting limit entry.
   - **hold**: the usual answer. Say what you are watching, so the next tick can continue the plan.

### Updating Autotrade Knowledge

When every pair has its action:

4. Sync the knowledge graph: `autotrade_knowledge_sync` (CLI: `knowledge sync`). It takes in this tick's actions and the exchange's final figures for trades that closed, and lists the closed positions that have no lesson yet.
5. Write the lesson of each one it lists: `autotrade_note` (CLI: `note -position N -text "..."`). That includes a position this tick closed. `autotrade_position` shows the trade with its best and worst point. Say what happened, why, and what to do differently.
6. If a lesson changes when an approach applies, update its description with `autotrade_approach_define`, so the next analysis reads it.
7. Reply with one line per pair (the pair, the action and why), then one line on the knowledge: the lessons written, or "up to date".

## Discipline

- Write each reason in this form:
  - open: `thesis: ...; trigger: ...; invalid if: ...; target: ...`
  - hold: `watching: ...`
  - against a coded signal: start with `veto: ...`
- Not trading is a result. Don't trade to be doing something, don't chase a candle that already ran, and don't revenge-trade after a stop.
- After two losing trades in a row, the next trade needs a clearly better setup than either of them.
- If a command is refused, read why. Correct it once, or hold. Never edit `autotrade.yaml` or `.env`, and never place orders any other way.
- One action per pair per tick.
- Several pairs moving together are one bet, not several: when BTC, ETH and BNB all set up the same way, count what all of them would lose together before opening more than one.
- **At the first tick after 00:00 UTC, also run `autotrade review`.** It scores your trades with the exchange's PnL after fees, by label, and your own ideas against the coded signals. Let it shape what you trust: drop a label that keeps losing, and if your own ideas do worse than the coded signals, lean on the signals.
