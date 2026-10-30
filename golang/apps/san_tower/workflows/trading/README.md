# Trading workflows

autotrade ([golang/apps/autotrade](../../../autotrade)) as tower workflows
(the user, 2026-10-30: "can we have autotrade, to workflow version?", with
Gemini through Remote Shell, the same four pairs, replacing autotrade, and as
close to it as the tower's steps allow). The procedure is autotrade's
`tick.md`, cut down to these tools, in [journal/rules.md](journal/rules.md).

| autotrade | here |
|---|---|
| `/loop 5m /autotrade`, Claude Code | **trade tick**: Schedule `1-59/5 * * * *`, Gemini in Remote Shell (Shell Off), one chat per hour (Browser Chat Session, New Chat Hourly) |
| `autotrade_snapshot` | Binance Snapshot: 5m, 15m, 1h, 4h from the real market's candles, the positioning, the book, funding and fees, the position or entry with its stops, the account |
| reconcile before each action | Binance Snapshot's Tidy (Binance Tidy): an entry expired, run to its target first or without its stop is cancelled, stops left over swept |
| `autotrade_open` (market or post-only limit, `expires_min`) | **trade open**: Binance Order with Require Stop, Max Notional 1000, Max Loss 100, Testnet Only, One Per Symbol, Max Leverage 5, Limit Expiry 15 |
| `autotrade_close` | **trade close**: Binance Close Position (also cancels an entry waiting) |
| `autotrade_protect` | **trade protect**: Binance Move Stop (a stop only moves closer) |
| `autotrade_hold` | the tick's reply, kept in `decisions/<date>.log` and read by the next tick |
| knowledge graph, `autotrade_note`, `knowledge_sync` | the **Trading Journal** knowledge: `trades/`, `lessons/`, `approaches/`; Binance Closed Trades finds the trades without a lesson; **trade lesson** writes one |
| `autotrade_approach_define` | **trade approach** |
| `autotrade review` (daily) | rules.md: at the first tick after 00:00 UTC, read the lessons and the approaches' trades |
| MetaTrader 5 pairs | not here |

The journal starts with autotrade's approach catalog and its four closed
trades with their lessons ([journal/lessons/autotrade.md](journal/lessons/autotrade.md)).

## Each tick

Schedule (or Run by hand) → Binance Closed Trades of the last 72h, leaving
out the ids in `lessons/done.txt` → Bash adds the last tick's reply →
Binance Snapshot (tidies first; those two as its Notes) → Format Text (the
tick's instructions) → Remote Shell, Gemini, with tools: `journal`
(Knowledge Tool, synced each tick), `files` (File Tool on the journal),
`binance` (Binance Tool, read only), and `open`, `close`, `protect`,
`lesson`, `approach` (Workflow Tools) → Bash keeps the reply in
`decisions/` → Telegram (Developer Bot). A failed chat goes to Telegram too,
and the run fails.

Gemini never gets a shell: the workflows it runs check what it sends (a
Filter, else Fail Run saying what is missing), the order goes through
Binance Order's own limits, and the Bash steps get what it wrote on their
standard input, never in the script.

## Set up

With the tower running, its connections **Binance Tesnet** (the testnet
key), **Developer Bot** (Telegram) and **Tower Local** (San Remote, with
Gemini signed in), all of which this tower has:

```powershell
pwsh golang/apps/san_tower/workflows/trading/setup.ps1
```

It copies `journal/` to `<tower data>/trading-journal` (unless that folder
is there: the journal is never overwritten), makes the Trading Journal
knowledge on it, and imports the six workflows into the folder Trading,
switched off. Or by hand: copy the folder, make the knowledge on the
Knowledge page, and import `trading.workflows.json` in the editor.

## Switching over from autotrade

Both trade BTCUSDT, ETHUSDT, BNBUSDT and BCHUSDT on the same testnet
account, in one-way mode: one position per pair, shared. Stop autotrade's
`/loop 5m /autotrade` first, then switch on **trade tick**. A position or
entry autotrade left is the tick's to manage (the snapshot shows it), but
its lesson belongs in autotrade's knowledge.

## Files

- `trading.workflows.json`: the six workflows, as the editor's Export
  writes them. Edit them in the editor and export again to keep a change.
  `internal/engine/tradingworkflows_test.go` checks every step, setting,
  link and script in it; `internal/server/trading_test.go` imports it.
- `journal/`: the Trading Journal's first files. Once set up, the tower's
  copy is the journal.
- `setup.ps1`: the setup above.
