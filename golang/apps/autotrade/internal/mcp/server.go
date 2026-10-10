// Package mcp serves autotrade to Claude over the Model Context Protocol
// (JSON-RPC 2.0, newline-delimited on stdio): the trading actions of the
// loop and the trade knowledge graph (docs/knowledge.md).
//
// Stdout carries only JSON-RPC; nothing else may print to it.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/bot"
	"github.com/wargasipil/autotrade/internal/knowledge"
)

type Server struct {
	Bot     *bot.Bot
	Version string
	// Log receives notes that must not go to stdout (a failed knowledge sync).
	Log io.Writer
}

var supportedVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const instructions = `autotrade: Binance USD-M futures on the test network, a MetaTrader 5 demo account (MIFX) for the pairs in autotrade.yaml's mt5.pairs, and the knowledge graph of their trades.
Trading: each pair in autotrade.yaml's pairs and mt5.pairs is traded on its own, on its venue. A MetaTrader pair's snapshot says so: amounts in the account currency (USD) where the tools say usdt, sizes in units of the symbol (not lots), its own limits, no positioning data. autotrade_snapshot gives, per pair, the market, indicators, levels, positioning, portfolio and the pair's last decisions; then for each pair exactly one of autotrade_open, autotrade_protect, autotrade_close or autotrade_hold, naming the pair, always with a reason. autotrade_open with limit places a post-only maker entry that waits in the book with its stop and target; autotrade_close cancels it while unfilled. The tool enforces the limits (test network only, configured pairs only, a stop-loss on every trade, max size and max loss, stops only tighten). golang/apps/autotrade/tick.md is the full procedure of a tick.
Knowledge graph: Strategy Approach -have-> Approach -with_approach-> Analytical Result -have-> Position; Data Snapshot -data_snapshot-> Position; Close Summary -have_summary-> Position; Portfolio -have_pair-> Pair -have-> each pair (btc/usdt, symbol BTCUSDT) -have-> its Open Position / Close Positions (-have-> Position) and its Profit Summary / Loss Summary (-have-> Close Summary); Configuration -have-> Pair That Traded -have-> each configured pair; Configuration -have-> Open Position Limit and Leverage, which with the Portfolio -have-> Risk Summary (what the open positions have at stake, drawdown, against the limits). Labels are the node types (Position, Approach, AnalyticalResult, DataSnapshot, CloseSummary, Pair, RiskSummary, ...); positions carry symbol and pair.
Before an open, check what is known: autotrade_approach for each approach you are using (its record and the notes of its past positions). An open names the approaches its analysis used; an approach not in the catalog must first be defined with autotrade_approach_define. A tick ends by updating the knowledge: autotrade_knowledge_sync lists the closed positions without a lesson; write each with autotrade_note. A record of a few trades is mostly luck: weigh it by the trade count.`

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve answers requests from in until it closes.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<20), 32<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			enc.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcError{-32700, "parse error"}})
			continue
		}
		result, rerr := s.dispatch(ctx, req)
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue // a notification
		}
		msg := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		if rerr != nil {
			msg["error"] = rerr
		} else {
			msg["result"] = result
		}
		if err := enc.Encode(msg); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) dispatch(ctx context.Context, req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		version := supportedVersions[0]
		for _, v := range supportedVersions {
			if v == p.ProtocolVersion {
				version = v
			}
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "autotrade", "title": "Autotrade", "version": s.Version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": toolDefs(s.Bot.Cfg.Pairs)}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		text, err := s.Call(ctx, p.Name, p.Arguments)
		if err != nil {
			if text != "" {
				text += "\n\n"
			}
			return map[string]any{"content": []any{textContent(text + "Error: " + err.Error())}, "isError": true}, nil
		}
		return map[string]any{"content": []any{textContent(text)}}, nil
	}
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil, nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

func textContent(t string) map[string]any { return map[string]any{"type": "text", "text": t} }

// ---- tools ---------------------------------------------------------------------------

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any  { return map[string]any{"type": "number", "description": desc} }
func intg(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
func flag(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}
func enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "enum": vals, "description": desc}
}

func tool(name, title, desc string, input map[string]any, readOnly, destructive bool) map[string]any {
	return map[string]any{
		"name": name, "title": title, "description": desc, "inputSchema": input,
		"annotations": map[string]any{"readOnlyHint": readOnly, "destructiveHint": destructive, "openWorldHint": true},
	}
}

var reasonProp = str("Why: for an open `thesis: ...; trigger: ...; invalid if: ...; target: ...`; for a hold `watching: ...`; against a coded signal start with `veto: ...`")

// toolDefs lists the tools. pairs are the configured pairs: the trading
// actions take one of them, and must name it when there are several.
func toolDefs(pairs []string) []any {
	pairProp := enum("The pair to act on: one of the pairs in autotrade.yaml", pairs...)
	// action is the input of a trading action: props plus the pair.
	action := func(props map[string]any, required ...string) map[string]any {
		props["pair"] = pairProp
		if len(pairs) > 1 {
			required = append([]string{"pair"}, required...)
		}
		return schema(props, required...)
	}
	return []any{
		tool("autotrade_snapshot", "Market snapshot",
			"Per pair: market, indicators on every timeframe, key levels, positioning, coded signals, portfolio with the pair's open position and its plan, and the pair's last decisions, as markdown. Saved as the data the pair's next decision is made on.",
			schema(map[string]any{
				"recent": intg("Candles of the trading interval to list (default 12)"),
				"pair":   enum("Only this pair (default: every pair in autotrade.yaml)", pairs...),
			}), true, false),
		tool("autotrade_open", "Open a position",
			"Open a long or short on a pair with a stop-loss and a take-profit, sized in USDT: at market, or with `limit` as a post-only limit entry that rests in the book and pays the maker fee (refused if it would fill at once; a long's limit goes at or under the best bid). A limit entry's stop and target wait with it as reduce-only orders; it is cancelled after expires_min if unfilled, or when the price reaches the target first, and recorded as opened when it fills. Refused when the pair has a position or a limit entry waiting, over the limits, or with a level on the wrong side of the price. Keeps the snapshot the analysis was made on.",
			action(map[string]any{
				"side":        enum("long or short", "long", "short"),
				"approaches":  strs("Approaches the analysis used (catalog names or coded setups); the first one labels the trade"),
				"usdt":        num("Position size in USDT (notional); on a MetaTrader pair, in the account currency (USD)"),
				"sl":          num("Stop-loss trigger price"),
				"tp":          num("Take-profit trigger price"),
				"limit":       num("Limit price for a post-only maker entry; omit to enter at market"),
				"expires_min": intg("Minutes an unfilled limit entry waits before it is cancelled (default 15)"),
				"reason":      reasonProp,
			}, "side", "approaches", "usdt", "sl", "reason"), false, true),
		tool("autotrade_protect", "Move stop / target",
			"Replace the pair's open position's stop-loss (only closer, never further away) and/or take-profit.",
			action(map[string]any{"sl": num("New stop-loss"), "tp": num("New take-profit"), "reason": reasonProp}, "reason"), false, true),
		tool("autotrade_close", "Close the position",
			"Close the pair's whole position at market and cancel its stop-loss and take-profit. With no position but a limit entry waiting, cancel the entry.",
			action(map[string]any{"reason": reasonProp}, "reason"), false, true),
		tool("autotrade_hold", "Hold",
			"Record a decision to do nothing on the pair this tick, with what you are watching.",
			action(map[string]any{"reason": reasonProp}, "reason"), false, false),
		tool("autotrade_journal", "Journal",
			"The last decisions, one per line.",
			schema(map[string]any{"n": intg("Entries (default 20)")}), true, false),
		tool("autotrade_review", "Review trades",
			"Every trade scored with the exchange's PnL after fees and funding, by label, and own ideas against coded signals.",
			schema(map[string]any{}), true, false),
		tool("autotrade_note", "Write a position note",
			"Write the summary and lesson of a position, normally after it closed. It becomes the note of its Close Summary and shows up under each of its approaches.",
			schema(map[string]any{"position": intg("Position number (#1 = the first open)"), "text": str("What happened, why, and the lesson")}, "position", "text"), false, false),
		tool("autotrade_approach_define", "Define an approach",
			"Add an approach to the catalog, or update its title and description. An open can only name approaches that are defined (or coded setups).",
			schema(map[string]any{
				"name":        str("Short lowercase label, e.g. big_trend"),
				"title":       str("Readable name, e.g. Big Trend"),
				"description": str("What it reads and when it says to act"),
			}, "name", "description"), false, false),
		tool("autotrade_approaches", "Approach catalog",
			"Every approach with its description and record: closed trades, win rate, average R, net USDT, open positions.",
			schema(map[string]any{}), true, false),
		tool("autotrade_approach", "Approach knowledge",
			"One approach: its description, its record, and every position that used it with outcome and note. Read this before using an approach.",
			schema(map[string]any{"name": str("Approach name")}, "name"), true, false),
		tool("autotrade_pair", "Pair knowledge",
			"One pair (BTCUSDT): its record, its profit and loss summaries, and its positions.",
			schema(map[string]any{"pair": str("Symbol, e.g. BTCUSDT")}, "pair"), true, false),
		tool("autotrade_positions", "List positions",
			"Positions from the knowledge graph, optionally filtered.",
			schema(map[string]any{
				"pair":     str("Only this pair's positions, by symbol (BTCUSDT)"),
				"status":   enum("open or closed", "open", "closed"),
				"approach": str("Only positions that used this approach"),
				"outcome":  enum("profit or loss (closed positions)", "profit", "loss"),
			}), true, false),
		tool("autotrade_position", "Position knowledge",
			"One position with its analytical result, approaches, data snapshot (market conditions; the full snapshot on request) and close summary.",
			schema(map[string]any{"n": intg("Position number"), "include_snapshot": flag("Include the full data snapshot")}, "n"), true, false),
		tool("autotrade_portfolio", "Portfolio",
			"The portfolio's record and, for each pair, its record with its profit and loss summaries; then the risk summary: the limits, leverage, wallet, what the open positions have at stake, and the drawdown.",
			schema(map[string]any{}), true, false),
		tool("autotrade_query", "Cypher query",
			"Run a read-only Cypher query on the knowledge graph, e.g. MATCH (c:CloseSummary)-[:have_summary]->(p:Position) RETURN p.n, p.pair, c.outcome, c.net. One relationship per pattern (two nodes): goraphdb does not match longer paths. Results as JSON.",
			schema(map[string]any{"cypher": str("Cypher query (MATCH ... RETURN ...)")}, "cypher"), true, false),
		tool("autotrade_knowledge_sync", "Sync the knowledge graph",
			"Rebuild the graph from the journal, the approach catalog and the saved snapshots, with the exchange's final figures for closed trades, and list the closed positions that have no lesson yet. The last step of a tick; the trading tools also sync on their own.",
			schema(map[string]any{}), false, false),
	}
}

// Call runs one tool and returns its text.
func (s *Server) Call(ctx context.Context, name string, raw json.RawMessage) (string, error) {
	var a struct {
		Recent          int      `json:"recent"`
		Side            string   `json:"side"`
		Approaches      []string `json:"approaches"`
		USDT            float64  `json:"usdt"`
		SL              float64  `json:"sl"`
		TP              float64  `json:"tp"`
		Limit           float64  `json:"limit"`
		ExpiresMin      int      `json:"expires_min"`
		Reason          string   `json:"reason"`
		N               int      `json:"n"`
		Position        int      `json:"position"`
		Text            string   `json:"text"`
		Name            string   `json:"name"`
		Title           string   `json:"title"`
		Description     string   `json:"description"`
		Status          string   `json:"status"`
		Approach        string   `json:"approach"`
		Pair            string   `json:"pair"`
		Outcome         string   `json:"outcome"`
		IncludeSnapshot bool     `json:"include_snapshot"`
		Cypher          string   `json:"cypher"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("arguments: %w", err)
		}
	}
	b := s.Bot
	entry := func(e bot.Entry, err error) (string, error) {
		s.sync(ctx)
		if e.Time.IsZero() {
			return "", err
		}
		return e.Line(-1), err
	}
	graph := func(fn func(st *knowledge.Store) (string, error)) (string, error) {
		var out string
		err := knowledge.With(b.KnowledgeDir(), func(st *knowledge.Store) error {
			var err error
			out, err = fn(st)
			return err
		})
		return out, err
	}

	if pairActions[name] {
		pb, err := b.ForPair(a.Pair)
		if err != nil {
			return "", err
		}
		b = pb
	}

	switch name {
	case "autotrade_snapshot":
		if a.Recent <= 0 {
			a.Recent = 12
		}
		pairs := b.Cfg.Pairs
		if a.Pair != "" {
			pairs = []string{a.Pair}
		}
		// One pair's failure leaves the others' snapshots usable.
		var buf bytes.Buffer
		var failed []string
		for _, p := range pairs {
			pb, err := b.ForPair(p)
			if err == nil {
				var snap *bot.Snapshot
				if snap, err = pb.Snapshot(ctx, a.Recent); err == nil {
					if buf.Len() > 0 {
						buf.WriteString("\n")
					}
					snap.Render(&buf)
					continue
				}
			}
			failed = append(failed, fmt.Sprintf("%s: %v", p, err))
		}
		s.sync(ctx)
		if len(failed) > 0 {
			return buf.String(), fmt.Errorf("no snapshot of %s", strings.Join(failed, "; "))
		}
		return buf.String(), nil
	case "autotrade_open":
		if a.Limit > 0 {
			return entry(b.OpenLimit(ctx, a.Side, a.Approaches, a.USDT, a.Limit, a.SL, a.TP, time.Duration(a.ExpiresMin)*time.Minute, a.Reason))
		}
		return entry(b.Open(ctx, a.Side, a.Approaches, a.USDT, a.SL, a.TP, a.Reason))
	case "autotrade_protect":
		return entry(b.Protect(ctx, a.SL, a.TP, a.Reason))
	case "autotrade_close":
		return entry(b.Close(ctx, a.Reason))
	case "autotrade_hold":
		e, err := b.Hold(ctx, a.Reason)
		if e.Time.IsZero() {
			return "", err
		}
		return e.Line(-1), err
	case "autotrade_note":
		return entry(b.Note(ctx, a.Position, a.Text))
	case "autotrade_journal":
		if a.N <= 0 {
			a.N = 20
		}
		es, err := b.Journal.Last(a.N)
		if err != nil {
			return "", err
		}
		var lines []string
		for _, e := range es {
			lines = append(lines, e.Line(-1))
		}
		if len(lines) == 0 {
			return "No decisions yet.", nil
		}
		return strings.Join(lines, "\n"), nil
	case "autotrade_review":
		r, err := b.Review(ctx)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		r.Render(&buf)
		return buf.String(), nil
	case "autotrade_approach_define":
		ap, err := b.DefineApproach(a.Name, a.Title, a.Description)
		if err != nil {
			return "", err
		}
		s.sync(ctx)
		return fmt.Sprintf("Defined approach %s (%s): %s", ap.Name, ap.Title, ap.Description), nil
	case "autotrade_approaches":
		return graph(knowledge.ApproachesMarkdown)
	case "autotrade_approach":
		return graph(func(st *knowledge.Store) (string, error) { return knowledge.ApproachMarkdown(st, a.Name) })
	case "autotrade_positions":
		return graph(func(st *knowledge.Store) (string, error) {
			ns, err := knowledge.Positions(st, knowledge.Filter{Pair: a.Pair, Status: a.Status, Approach: a.Approach, Outcome: a.Outcome})
			return knowledge.PositionsMarkdown(ns), err
		})
	case "autotrade_pair":
		return graph(func(st *knowledge.Store) (string, error) { return knowledge.PairMarkdown(st, a.Pair) })
	case "autotrade_position":
		return graph(func(st *knowledge.Store) (string, error) {
			d, err := knowledge.GetPosition(st, a.N)
			if err != nil {
				return "", err
			}
			return d.Markdown(a.IncludeSnapshot), nil
		})
	case "autotrade_portfolio":
		return graph(knowledge.PortfolioMarkdown)
	case "autotrade_query":
		if !readOnly(a.Cypher) {
			return "", errors.New("only read queries (MATCH ... RETURN) are allowed: the graph is rebuilt from the journal")
		}
		return graph(func(st *knowledge.Store) (string, error) {
			res, err := st.Query(ctx, a.Cypher)
			if err != nil {
				return "", err
			}
			out, err := json.MarshalIndent(map[string]any{"columns": res.Columns, "rows": res.Rows}, "", "  ")
			return string(out), err
		})
	case "autotrade_knowledge_sync":
		res, err := b.SyncKnowledge(ctx)
		if err != nil {
			return "", err
		}
		out := fmt.Sprintf("Synced: %d approaches, %d pairs, %d positions (%d open, %d closed).", res.Approaches, res.Pairs, res.Positions, res.Open, res.Closed)
		if len(res.NoLesson) > 0 {
			nums := make([]string, len(res.NoLesson))
			for i, n := range res.NoLesson {
				nums[i] = fmt.Sprintf("#%d", n)
			}
			out += fmt.Sprintf("\nClosed without a lesson: %s. Write each with autotrade_note (autotrade_position N shows the trade).", strings.Join(nums, ", "))
		}
		return out, nil
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// pairActions act on one pair, named by their pair argument.
var pairActions = map[string]bool{"autotrade_open": true, "autotrade_protect": true, "autotrade_close": true, "autotrade_hold": true}

// sync brings the graph up to date after an action. A failure is logged,
// never returned: the trade itself went through.
func (s *Server) sync(ctx context.Context) {
	if _, err := s.Bot.SyncKnowledge(ctx); err != nil && s.Log != nil {
		fmt.Fprintf(s.Log, "autotrade: knowledge sync: %v\n", err)
	}
}

// readOnly accepts queries without write clauses.
func readOnly(q string) bool {
	u := " " + strings.ToUpper(strings.Join(strings.Fields(q), " ")) + " "
	for _, w := range []string{" CREATE ", " MERGE ", " DELETE ", " DETACH ", " SET ", " REMOVE ", " DROP "} {
		if strings.Contains(u, w) {
			return false
		}
	}
	return strings.Contains(u, " MATCH ") || strings.HasPrefix(strings.TrimSpace(u), "MATCH")
}
