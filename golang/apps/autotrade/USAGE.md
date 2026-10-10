# autotrade: usage

The Go side of the flow in [docs/basic_flow.md](docs/basic_flow.md). Claude, running `/loop`, is the "analyze" and "decide" step. The `autotrade` command does everything else:

| readme state | command |
|---|---|
| connect (Binance API, Golang library) | `internal/binance`, a small signed REST client, standard library only |
| Feature Extraction (candle) | `snapshot`: EMA 20/50/200, RSI14, MACD, ATR14, Bollinger, volume, changes, range, on 5m plus 15m, 1h and 4h; key levels (swing highs/lows, prior day); positioning (open interest, taker flow, long/short ratios, funding) |
| Portofolio Data (balance, hold position) | `snapshot`: wallet, position, its stop-loss/take-profit, last 24h PnL |
| analyze | Claude decides each tick from the snapshot (see [tick.md](tick.md)); the coded rules (`internal/strategy`) add signals and a plan for the open trade as a second opinion |
| Decide Position: new buy / new sell | `open long` / `open short`, at market or with `-limit` as a post-only maker entry |
| Decide Position: close position | `close` (and `protect` to move the stops) |
| Decide Position: nothing | `hold` |
| wait | until the next 5m close: run by hand, or `/loop 5m` |

Trading is on Binance USD-M Futures, **test network only**: `trade_url` must be the testnet or demo host, or the tool refuses to start. The pairs under `mt5.pairs` trade on a MetaTrader 5 **demo** account instead (MIFX; see [MetaTrader 5](#metatrader-5-mifx-the-second-venue)).

## Setup

1. Log in at <https://testnet.binancefuture.com> and create an API key there. Binance Demo Trading (demo.binance.com → Demo Trading API) runs on the same backend; with a demo key, set `trade_url` to `https://demo-fapi.binance.com`.
2. Copy `.env.example` to `.env` (git-ignored) and fill in the key. Alternatively, set `BINANCE_TESTNET_API_KEY` and `BINANCE_TESTNET_API_SECRET` in the environment.
3. Build: `pwsh golang/apps/autotrade/build.ps1` (or `bash .../build.sh`). The build runs vet and the tests first.
4. Check: `golang/apps/autotrade/bin/autotrade.exe check`. It shows the clock correction, position mode, wallet, and for each pair whether it trades on the test network and its leverage.

All settings live in `autotrade.yaml` ([docs/readme.md](docs/readme.md)). The binary finds it in the current folder or in the folder above `bin/`, so it can be run from the repo root; `-config FILE` points it elsewhere. A misspelled key is refused, so a limit never falls back to its default unnoticed. The API key stays in `.env` or the environment, not in `autotrade.yaml`.

`pairs` lists the pairs the loop trades, now `[BTCUSDT, ETHUSDT, BNBUSDT, BCHUSDT]` (`btc/usdt` is read as BTCUSDT). Each pair is traded on its own: `snapshot` covers every pair (or the `-symbol` one), and `open`, `protect`, `close` and `hold` act on the `-symbol` pair, which they require when there are several. A pair that is not listed is refused. The MCP tools take the pair as `pair`.

## MetaTrader 5 (MIFX), the second venue

The pairs listed under `mt5.pairs` in `autotrade.yaml` trade on a MetaTrader 5 **demo** account instead of Binance. The rest works the same for them: `snapshot`, `open`/`protect`/`close`/`hold`, limit entries, the journal, `review` and the knowledge graph. With `mt5.pairs: []` (now), MetaTrader is not used.

**How it connects.** MetaTrader can't be called from outside, so a small Expert Advisor, [internal/mt5/AutotradeBridge.mq5](internal/mt5/AutotradeBridge.mq5), runs inside the terminal and does the work:
1. autotrade writes each request as a file in MetaTrader's common Files folder (`%APPDATA%\MetaQuotes\Terminal\Common\Files\autotrade`).
2. The EA picks it up within about 20 ms and answers in a file next to it.
3. autotrade reads the answer and deletes it.

The CLI and the MCP server can ask at the same time. The terminal must stay open and logged in. When it isn't, the MetaTrader pairs report "MetaTrader did not answer" (once; it is not asked again for 30s), and the Binance pairs carry on.

**Setup:**
1. Install MetaTrader 5 from MIFX, open a demo account, and log in to it.
2. Run `autotrade mt5 install`. It copies the EA into each MT5 terminal's `MQL5\Experts` folder and compiles it with that terminal's MetaEditor. If that fails, open it in MetaEditor and press F7.
3. In the terminal:
   - Navigator → Expert Advisors → AutotradeBridge: drag it onto any chart.
   - In its Common tab, tick "Allow Algo Trading".
   - Turn on Algo Trading in the toolbar.
   - The Experts tab now says `autotrade bridge 1: answering requests ...`.
4. In `autotrade.yaml`:
   - List the symbols under `mt5.pairs` as the broker spells them in Market Watch, for example `[XAUUSD]`.
   - Set `mt5.leverage` to the account's leverage.
   - Set `mt5.max_position` and `mt5.max_risk`.
5. Run `autotrade check`. It shows the account and its leverage, and for each pair its status and smallest position. It fails when:
   - the account is not a demo account
   - Algo Trading is off
   - the leverage differs from `mt5.leverage`
   - a pair's smallest position is over `mt5.max_position`

**What differs from Binance:**
- **Demo only.** Both the EA and autotrade refuse a real or contest account.
- **Sizes are units of the symbol, not lots.** One lot is the contract size: 100 oz for XAUUSD, 100,000 EUR for EURUSD.
- **Amounts are in the account currency (USD).** That covers the `usdt` of an open, `mt5.max_position` and `mt5.max_risk`. So only symbols whose profit is paid in that currency can be traded (XAUUSD, EURUSD, US indices, ...). USDJPY, whose profit is in JPY, is refused.
- **Prices.** The "mark price" is the middle of the bid and the ask. A long fills at the ask, and its stop triggers on the bid.
- **Stops.** MetaTrader refuses a stop or target inside the broker's stop level ("invalid stops"). The open then closes the position again, as on Binance.
- **Stops belong to the position.** `protect` moves them in one step, with no moment without a stop.
- **Limit entries** are buy/sell limit orders that carry their stop and target. These pass to the position when it fills. A limit at or past the price is refused (a long's must be under the ask). There is no maker fee: the cost is the spread plus any commission.
- **Leverage** is the account's, set by the broker. `open` is refused while `mt5.leverage` differs.
- **Hedging accounts.** autotrade closes by ticket, so it keeps one position per pair. It leaves orders placed by hand alone, but a position opened by hand on a pair counts as that pair's position.
- **The snapshot** has no positioning section: MetaTrader has no open interest, funding or long/short data. Its candle volume is tick volume, and swap shows as "swap". `review` counts swap the way it counts funding.
- **Time zones.** Candle and deal times are converted from the broker's server time to UTC. Candles from before a daylight-saving switch may be an hour off.
- **Market hours.** The market closes on weekends and holidays. The snapshot then notes that the pair is not trading, and orders are refused until it reopens.
- **No backtests.** `backtest` and `discover` replay Binance candles, so they skip MetaTrader pairs.
- **Knowledge graph.** MetaTrader pairs are pairs like the others (`xau/usd`). The Risk Summary holds them to the `mt5` limits, and the wallet is both accounts together.

The EA is untested against a live terminal so far. Its Go side is tested against a simulated EA (`internal/mt5/client_test.go`).

## Commands

```
autotrade [-symbol PAIR] snapshot [-recent 12]
autotrade -symbol PAIR open long|short -approach big_trend,positioning -usdt 500 -sl 81900 [-tp 83800] [-limit 82400 [-expires 15m]] -reason "..."
autotrade -symbol PAIR protect [-sl 82450] [-tp 83900] -reason "..."
autotrade -symbol PAIR close -reason "..."
autotrade -symbol PAIR hold -reason "..."
autotrade journal [-n 20]
autotrade check
autotrade mt5 install
autotrade review
autotrade [-symbol PAIR] backtest [-days 365] [-setups trend_pullback,trend_breakout] [-fee 0.0005] [-trades]
autotrade discover [-days 730] [-setups liquidity_sweep,...] [-fee 0.0005]

autotrade approach [-name big_trend -title "Big Trend" -description "..."]
autotrade note -position 1 -text "what happened and the lesson"
autotrade knowledge view [-addr 127.0.0.1:7475] [-no-open]
autotrade knowledge approaches | approach NAME | pair BTCUSDT | positions [-pair BTCUSDT -status open -approach A -outcome loss]
                  | position N [-snapshot] | portfolio | query "MATCH ... RETURN ..." | sync
autotrade mcp run
```

Flags can come before or after the words of a command (`knowledge position 3 -snapshot`).

Global flags go before the command: `-config FILE`, `-symbol ETHUSDT` (one of the pairs), `-json` (`snapshot -json` gives an array when it covers several pairs).

### Limit entries (maker fee)

`open -limit PRICE` (MCP: `limit`, `expires_min`) places a post-only LIMIT order (`timeInForce GTX`) instead of a market order. It rests in the book and pays the maker fee when it fills; the exchange refuses it (-5022) if it would trade at once. On this testnet account the fees are 0.02% maker and 0.04% taker per fill (`GET /fapi/v1/commissionRate`, shown in the snapshot), so a maker entry with a market exit costs about 0.06% per round trip instead of 0.08%.

- **Protection from the first fill.** The stop-loss and take-profit are placed with the order as reduce-only conditional orders for its quantity. Close-position orders can't wait for a fill: the exchange refuses them without a position (-4509). A reduce-only order never opens a position.
- **Checks.** As for a market open, with the size, the minimum order and the loss at the stop measured from the limit price, and the stop on the right side of both the limit and the mark price.
- **Reconciling.** Nothing runs between ticks, so every `snapshot`, `open`, `close`, `protect` and `hold` on the pair first brings the journal up to date. An order that filled becomes the position's `open_long`/`open_short` entry, dated at the fill (`ordered` keeps when it was placed, so `review` counts its fees). A partly filled one keeps what filled and cancels the rest. An unfilled one is cancelled once past its expiry (`-expires`, default 15m), when the mark price reaches its target first, or when its stop order is gone. The journal records these as `cancel`.
- **While it waits.** The snapshot shows it under the portfolio, with the order's state, and in the notes. `close` cancels it with its stop and target. `open` refuses while it waits, and `protect` refuses until it fills.
- **Clean-up.** With no position and no entry waiting, the reconciling also cancels stop and target orders left from a position the exchange closed.

## Who decides

Claude is the strategy. Each tick has three phases ([docs/readme.md](docs/readme.md), [tick.md](tick.md)): it analyzes (the snapshot and a written view of each pair), acts (one action per pair), and updates the autotrade knowledge (a sync, then the lesson of each closed position the sync lists). The Go code doesn't decide anything. It extracts the data, carries out the decision, and enforces the limits below.

`-setup` labels each trade's idea: a coded setup's name when Claude takes that signal, or Claude's own name for the idea (`range_fade`, ...). `review` scores trades from the journal with what the exchange booked (realized PnL, commission, funding). It shows them by label, and Claude's own ideas against the coded signals. A trade ends at the loop's `close`, or, when the next `open` comes without one, at its stop or target on the exchange. Claude's judgment can't be backtested, so this is how it is measured.

**Key levels**:
- swing highs and lows of the trading interval over the last week, and of the first context timeframe over its last 180 candles (a swing is a high or low beyond the 3 candles on each side)
- the high and low of the last closed candle of the second context timeframe (the prior 4h candle in the current config)
- the nearest 4 above and below the price, with their distance in % and in ATR

**Positioning**, from Binance's public futures data (mainnet), hourly:
- open interest and its change over 4h and 24h
- taker buy/sell volume ratio
- long/short ratio of all accounts and of top traders' positions, now and 24h ago
- the last 3 funding rates

## The coded rules (`internal/strategy`), a second opinion

The snapshot and the backtest run the same code.

**Regime**, from the two context timeframes (4h and 1d):
- **up**: the high timeframe has EMA20 above EMA50 and closes above EMA50, and the mid timeframe closes above its EMA50. Only longs are allowed.
- **down**: the mirror image. Only shorts are allowed.
- **range**: the high timeframe's EMA20 and EMA50 are within half an ATR of each other. Only the range setup applies.
- **mixed**: anything else. No trades.

**Setups**, checked on the last closed candle of the trading interval:
- `trend_pullback`: within the last 6 candles, price pulled back to the EMA20 and RSI cooled off (below 45 for a long, above 55 for a short). Then a candle in the trend's color closes back across the EMA20, less than 1 ATR from it, with RSI past 50 and the MACD histogram turning.
- `trend_breakout`: a close beyond the last 32 candles' range by more than 0.1 ATR, on at least 1.5x volume, in the strong half of its candle, with RSI not exhausted.
- `range_revert`: a close back inside a Bollinger band after a close outside it, RSI past 35 or 65, with a target at the middle band at least 1.2x the stop distance away.

**Stops and targets:** the stop goes past the recent swing, but at least 1x the mid-timeframe ATR away. Trend targets are 2R. Any target closer than 0.3% is skipped, because it doesn't pay the fees. The size is the smaller of `max_position_usdt` and `max_risk_usdt` divided by the stop distance.

**Managing an open trade:**
- Once the trade has been +1R, the stop moves to break-even plus fees.
- A trade from a coded trend setup closes when the mid timeframe closes beyond its EMA50 against it. Claude's own ideas are invalidated by their own thesis.
- Any trade closes after 32 candles without reaching +0.5R.

## Backtest results (2026-10-09)

The backtest uses real BTCUSDT candles, enters at the next candle's open, and charges a 0.05% fee per fill. When the stop and the target fall inside one candle, it counts the stop first. "1st/2nd half" splits the period in two equal halves by time.

15m entries (regime from 1h and 4h), 730 days (BTC +32.8% over the period):

| setup | trades | avg R | 1st half | 2nd half | profit factor |
|---|---|---|---|---|---|
| trend_pullback | 800 | −0.128 | −0.104 | −0.152 | 0.72 |
| trend_breakout | 638 | −0.165 | −0.200 | −0.125 | 0.70 |
| range_revert | 162 | −0.307 | −0.416 | −0.213 | 0.53 |

Before fees, the same 15m setups break about even (−0.004R to +0.046R per trade over the last year). The fees decide the result: a 1h-ATR stop is about 0.5% of price, so a 0.1% round trip costs about 0.2R per trade.

1h entries (regime from 4h and 1d), 730 days:

| setup | trades | avg R | 1st half | 2nd half | profit factor |
|---|---|---|---|---|---|
| trend_pullback | 163 | −0.034 | −0.050 | −0.019 | 0.96 |
| trend_breakout | 150 | −0.054 | +0.022 | −0.141 | 0.93 |
| range_revert | 48 | −0.415 | −0.352 | −0.484 | 0.31 |

**None of the setups makes money after fees.** `trend_pullback` on 1h is the closest to break-even, so it is the only one the snapshot checks. It serves as a second opinion and a baseline that Claude's own ideas are measured against in `review`, and is no basis for real money. Only two timeframe variants were tried, deliberately. Trying many more and keeping the best would just fit the past.

### Candidate approaches (2026-10-10)

`autotrade discover` tested six more ideas, coded in `internal/strategy/candidates.go` with their rules fixed before the run: liquidity sweep, session breakout, squeeze breakout, crowded-side fade, break and retest, relative strength (against BTC). Each ran on BTC, ETH, BNB and BCH over 730 days with 15m entries, stops of at least one 1h ATR, and 2R targets. The pass bar, also set beforehand: positive after fees in both halves over all pairs, positive on all pairs but one, at least 60 trades.

Average R per trade over all pairs (1st half / 2nd half):

| candidate | trades | taker fees (0.05%) | maker fees (0.02%) | no fees |
|---|---|---|---|---|
| liquidity_sweep | 6,565 | −0.137 (−0.131 / −0.144) | −0.053 | +0.002 (+0.005 / −0.001) |
| session_breakout | 2,919 | −0.096 (−0.072 / −0.119) | −0.030 | +0.009 (+0.025 / −0.006) |
| squeeze_breakout | 1,792 | −0.119 (−0.063 / −0.171) | −0.028 | +0.032 (+0.087 / −0.021) |
| crowded_fade | 460 | −0.057 (+0.083 / −0.208) | −0.007 | +0.028 (+0.167 / −0.121) |
| break_retest | 2,161 | −0.156 (−0.181 / −0.130) | −0.069 | −0.014 (−0.048 / +0.022) |
| relative_strength | 2,281 | −0.139 (−0.147 / −0.126) | −0.086 | −0.053 (−0.069 / −0.030) |

**All six fail**, at every fee level. Before fees they sit within about ±0.05R of zero, and none stays positive in both halves even with no fees at all. Mechanically they behave like coin flips that then pay fees. The best gross results (`squeeze_breakout`, `crowded_fade`) came from the first year and turned negative in the second, when BTC fell about a third. The candidates stay coded for `discover`, but none is used in live trading. Rerun with `autotrade discover -days 730 [-fee 0]`.

Downloaded candles are cached in `data/candles/` (with the taker buy volume; older caches are downloaded again).

## Limits and safety (`autotrade.yaml`)

The limits are set by you. The loop must not edit this file.

- `max_position_usdt` (1000): the largest notional value of a new position.
- `max_risk_usdt` (100): the largest loss if the stop-loss is hit, measured from the mark price. A refusal tells you the largest size that fits.
- `leverage` (1): set on the exchange before each open.
- `setups`: the coded setups the snapshot checks. `fee_rate`: used by the backtest and the break-even stop.
- A stop-loss is required. It is a STOP_MARKET order on the mark price that closes the whole position, placed through Binance's Algo Order API (the old order endpoint has rejected stops since 2025-12). If Binance rejects the stop, the position is closed again right away.
- There is one position per pair at a time. To reverse a position, `close` it, then `open` the other side. The limits are per position, so with three pairs up to three positions, each up to `max_risk_usdt` at stake, can be open at once.
- `protect` can only tighten a stop-loss, never move it further away.
- Stop and take-profit orders left over after a closed position are cancelled before the next open, because they would close the new position.
- Signed requests use the exchange's clock. This machine's clock runs about 4 minutes fast, and Binance rejects timestamps more than 10s off.

Candles come from the **real** market (`market_url`, public and read-only, no key is ever sent there), because testnet volume is not real: one 15m candle traded about 14,600 BTC on the testnet and about 680 BTC on mainnet. Prices match closely. Orders, the mark price and the stops are on the testnet.

## Knowledge graph

[docs/knowledge.md](docs/knowledge.md) as a graph database (goraphdb, the same as san_knowledge), in `data/knowledge/`:

```
Strategy Approach ─have→ Approach ─with_approach→ Analytical Result ─have→ Position
Data Snapshot ─data_snapshot→ Position
Open Position / Close Positions ─have→ Position
Close Summary ─have_summary→ Position
Portfolio ─have_pair→ Pair ─have→ btc/usdt, eth/usdt, ...
btc/usdt ─have→ Open Position / Close Positions ─have→ Position
btc/usdt ─have→ Profit Summary / Loss Summary ─have→ Close Summary
Configuration ─have→ Pair That Traded ─have→ btc/usdt, eth/usdt, bnb/usdt, ...
Configuration ─have→ Open Position Limit / Leverage ─have→ Risk Summary
Portfolio ─have→ Risk Summary
```

Each pair has its own Open Position, Close Positions, Profit Summary and Loss Summary. A position belongs to the pair it was traded on (the journal entry's symbol). Every configured pair has its pair node, traded yet or not; a pair dropped from `pairs` keeps its node and record under Pair, but leaves Pair That Traded. Trades are paired with their closes per symbol, so a close on one pair never ends a trade on another.

Each node has a `key`, a `node_type` and a `title`. Keys are `position/1`, `approach/big_trend`, `close_summary/1`, `pair/BTCUSDT`, `open_position/BTCUSDT`, `loss_summary/BTCUSDT`, and so on. Its label is its type (`Position`, `Approach`, `AnalyticalResult`, `DataSnapshot`, `CloseSummary`, `Pair`, `PairHub`, ...), so Cypher can match on it:

```
autotrade knowledge query "MATCH (c:CloseSummary)-[:have_summary]->(p:Position) RETURN p.n, p.pair, c.outcome, c.net"
```

goraphdb matches one relationship per pattern (two nodes); a longer path such as Approach → Analytical Result → Position is refused. Walk it in steps, or use `knowledge approach NAME`, which already does.

- **Approach:** an entry of the approach catalog (`data/approaches.json`, written with `approach` / `autotrade_approach_define`) or a coded setup. It carries its record: closed trades, wins, win rate, average R, net, open positions.
- **Position:** each successful `open` in the journal, numbered in order (#1 is the first). It holds side, entry, initial stop (1R), the stop after the last `protect` (`stop_now`), target, quantity and risk, status, and the net and R once closed.
- **Analytical Result:** the open's reason (thesis, trigger, invalidation, target), linked from the approaches named with `-approach`.
- **Data Snapshot:** the snapshot the analysis was made on. Every `snapshot` is saved per pair as `data/snapshots/last_<SYMBOL>.json` and `.md`, and `open` keeps it as `data/snapshots/<SYMBOL>_<time>.json`. A fresh one is taken if the last is over 10 minutes old. It stores the full markdown plus market conditions as fields to query by: price, interval, `trend_<interval>`, regime, RSI, ATR %, open-interest change, taker ratio, long/short ratio, funding.
- **Close Summary:**
  - **How it ended:** closed by the loop or by the exchange (its stop or target), when, after how long, and the net PnL after fees and funding as the exchange booked it, in USDT and R.
  - **Excursions:** the best and worst point before the close, in R, from the candles.
  - **Text:** the loop's close reason and the trader's note (`note` / `autotrade_note`: what happened and the lesson).
- **Pair** (`btc/usdt`, with `symbol` BTCUSDT): the pair's record. Its **Profit Summary / Loss Summary** total its winning and losing closes, and its **Open Position / Close Positions** hold its positions.
- **Portfolio:** totals over every pair, and the wallet when the account could be read.
- **Configuration:** `autotrade.yaml`'s pairs, intervals, coded setups, fee rate and trade URL. Its **Open Position Limit** holds the limits on an open: `max_position_usdt`, `max_risk_usdt`, one open position per pair, and a stop-loss on each. Its **Leverage** node holds the leverage and the margin a full-size position needs.
- **Pair That Traded:** the pairs of `autotrade.yaml`, linked to each one's pair node (`MATCH (t:PairThatTraded)-[:have]->(p:Pair) RETURN p.symbol`).
- **Risk Summary:** the limits, the leverage and the portfolio together.
  - **Open positions:** their worth, their margin, and what they lose if every stop fills now. The stop is the one after the last `protect`, so a stop moved past the entry has nothing at stake.
  - **Closed positions:** the worst trade, the max drawdown, how far the net is below its peak, and the losses in a row.
  - **Wallet and warnings:** each figure as a % of the wallet. A warning appears when an open position is over today's limits, for example after a limit was lowered.
  - **Where to read it:** `knowledge portfolio` / `autotrade_portfolio` end with it.

The graph is **derived**. `SyncKnowledge` rebuilds it from the journal, the approach catalog and the snapshot files, so `data/knowledge/` can be deleted at any time. The exchange is asked once for each closed position's figures; after that they're kept as final. The CLI and the MCP tools sync on their own after `snapshot`, `open`, `close`, `protect`, `note` and `approach`. The database file is opened per operation, so the CLI and a running MCP server can share it.

## Knowledge view

`autotrade knowledge view` syncs the graph, serves a preview on <http://127.0.0.1:7475/>, and opens the browser (`-no-open` to skip, `-addr` for another port). Press Ctrl+C to stop it.

- **Graph:** nodes are coloured and shaped by type. Positions show side and net (open ones have a dashed border), and approaches show their closed trades and net. The legend chips hide or show a type. **Re-layout** arranges the graph again; **Sync** rebuilds it from the files.
- **Collapse and expand,** Neo4j style: any node with links can do both, with a double-click or the buttons in its panel.
  - **Expand** shows the node's hidden neighbours, one hop out in either direction. A node with hidden neighbours has a double border and says how many (`+3 hidden`). A double-click expands when something around the node is hidden, and collapses otherwise.
  - **Collapse** hides what hangs on the node. That is everything it holds in the hierarchy of [docs/knowledge.md](docs/knowledge.md), plus its other links except the way back to its holder, plus whatever is then left linked only to those. For example, a pair hides its Open Position, Close Positions and summaries with their positions; a position hides its analysis, data snapshot and close summary; an approach hides its analyses; Pair That Traded hides the pairs. The hierarchy: Strategy Approach holds the approaches; Portfolio holds Pair (each pair, its Open Position, Close Positions and summaries, their positions) and the Risk Summary; a position holds its analysis, data snapshot and close summary; Configuration holds Pair That Traded, Open Position Limit and Leverage. Strategy Approach, Portfolio and Configuration are never hidden as leftovers.
  - **Collapse all** leaves Strategy Approach, Portfolio and Configuration to expand from; **Expand all** shows everything again.
  - The rest of the graph stays where it is. New nodes fan out around the node they hang on, and the view zooms out if they fall outside it. What is hidden is remembered in the browser, and a link to a hidden node (`#k=...`) shows it with its holders.
- **Panel:** clicking a node fades everything but its neighbours. The panel shows the node's readable view: a position with its analysis, approaches, data snapshot and close summary; an approach with its record and the notes of its trades. It also lists the node's properties, and its links, which can be clicked.
- **URL:** `#k=position/1` selects a node, and `?theme=light|dark` sets the theme.

The page is embedded in the binary: plain HTML, CSS and JavaScript plus a vendored cytoscape, with no build step. The database is opened per request, so trading, the CLI and the MCP server keep working while the view runs.

## MCP server

`autotrade mcp run` serves the trading actions and the knowledge graph over MCP (stdio). It is registered as `autotrade` in the repo's `.mcp.json` (`mcp run`; a bare `mcp` still works). After a rebuild, reconnect it with `/mcp` so the session picks up the new binary.

| tool | does |
|---|---|
| `autotrade_snapshot` | the snapshot as markdown (and saves it as the latest data) |
| `autotrade_open` / `_protect` / `_close` / `_hold` | the trading actions, with `approaches` on open |
| `autotrade_journal`, `autotrade_review` | last decisions; trades scored with the exchange's PnL |
| `autotrade_approaches`, `autotrade_approach` | the catalog with records; one approach with its positions and notes |
| `autotrade_approach_define`, `autotrade_note` | write the catalog; write a position's lesson |
| `autotrade_positions`, `autotrade_position`, `autotrade_pair`, `autotrade_portfolio` | read positions (with analysis, snapshot, close summary), one pair, and the totals per pair with the risk summary |
| `autotrade_query`, `autotrade_knowledge_sync` | read-only Cypher; rebuild the graph and list the closed positions without a lesson |

## Journal

Every decision is appended to `data/journal.jsonl` (git-ignored): `open` (with its `setup`), `close`, `protect`, `hold`, and refusals too, each with its reason. `snapshot` shows the last 5. The position plan reads the trade's `open` entry from the whole journal. The snapshot also notes when a position disappeared because its stop or take-profit was hit.

## Running the loop

By hand, shortly after a 5m close, type:

```
/autotrade
```

Or schedule it with `/loop 5m /autotrade`.

`/autotrade` is the project skill in `.claude/skills/autotrade/SKILL.md`. It reads `tick.md` and follows its three phases: analyze, act (one action per pair), update the knowledge. Text after it is a note for that tick. While it runs, the autotrade MCP tools are allowed without a prompt, the trading ones included, so a `/loop` doesn't stop to ask. The tool still enforces the limits. Only you can start it; Claude doesn't run it on its own. Sending `follow golang/apps/autotrade/tick.md` does the same, with the usual permission prompts.

The live config trades 5m candles with 15m, 1h and 4h as context, and has no coded setups: on 5m they lost 0.33R (trend_pullback) and 0.49R (trend_breakout) per trade after fees over 90 days, and on 15m 0.13R and 0.17R over 730 days, while breaking even before fees. On 5m the stop floor in tick.md matters most: the 15m ATR can be as small as the round-trip fee (0.06% with a limit entry, 0.08% at market on this account).

The loop's Bash calls need permission. Allow `golang/apps/autotrade/bin/autotrade.exe` once with "don't ask again", or add it to `.claude/settings.local.json`.
