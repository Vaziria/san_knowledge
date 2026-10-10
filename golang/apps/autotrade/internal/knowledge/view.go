package knowledge

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// PositionDetail is a position with everything linked to it.
type PositionDetail struct {
	Position   *Node   `json:"position"`
	Analysis   *Node   `json:"analysis"`
	Approaches []*Node `json:"approaches"`
	Snapshot   *Node   `json:"snapshot,omitempty"`
	Summary    *Node   `json:"close_summary,omitempty"`
}

// GetPosition walks the graph around position n.
func GetPosition(s *Store, n int) (*PositionDetail, error) {
	pos, err := s.Get(PositionKey(n))
	if err != nil {
		return nil, err
	}
	d := &PositionDetail{Position: pos}
	if as, err := s.From(pos.Key, RelHave); err == nil {
		for _, a := range as {
			if a.Type == TypeAnalysis {
				d.Analysis = a
			}
		}
	}
	if d.Analysis != nil {
		d.Approaches, _ = s.From(d.Analysis.Key, RelWithApproach)
	}
	if sn, err := s.From(pos.Key, RelDataSnapshot); err == nil && len(sn) > 0 {
		d.Snapshot = sn[0]
	}
	if cs, err := s.From(pos.Key, RelHaveSummary); err == nil && len(cs) > 0 {
		d.Summary = cs[0]
	}
	return d, nil
}

// Filter selects positions. Empty fields match everything.
type Filter struct {
	Pair     string // symbol, e.g. BTCUSDT
	Status   string // open, closed
	Approach string
	Outcome  string // profit, loss
}

// Positions returns the positions that match, oldest first.
func Positions(s *Store, f Filter) ([]*Node, error) {
	nodes, err := s.ByType(TypePosition)
	if err != nil {
		return nil, err
	}
	var out []*Node
	for _, n := range nodes {
		if f.Pair != "" && !strings.EqualFold(n.Str("symbol"), f.Pair) {
			continue
		}
		if f.Status != "" && n.Str("status") != f.Status {
			continue
		}
		if f.Approach != "" && !slices.Contains(n.Strs("approaches"), f.Approach) {
			continue
		}
		if f.Outcome != "" {
			if n.Str("status") != "closed" || (f.Outcome == "profit") != (n.Num("net") > 0) {
				continue
			}
		}
		out = append(out, n)
	}
	return out, nil
}

// ---- markdown ---------------------------------------------------------------------

func money(v float64) string { return fmt.Sprintf("%+.2f", v) }

func positionLine(n *Node) string {
	stop := fmt.Sprintf("%g", n.Num("stop"))
	if now := n.Num("stop_now"); now != 0 && now != n.Num("stop") {
		stop += fmt.Sprintf(" (now %g)", now)
	}
	line := fmt.Sprintf("#%d %s %s %s @ %g, stop %s, target %g, approaches %s, opened %s",
		int(n.Num("n")), n.Str("pair"), n.Str("status"), n.Str("side"), n.Num("entry"), stop, n.Num("target"),
		strings.Join(n.Strs("approaches"), ", "), n.Str("opened"))
	if n.Str("status") == "closed" {
		line += fmt.Sprintf(": %s USDT (%+.2fR), closed %s", money(n.Num("net")), n.Num("r"), n.Str("closed"))
	} else {
		line += fmt.Sprintf(": unrealized %s USDT", money(n.Num("unrealized")))
	}
	return line
}

// PositionsMarkdown lists positions one per line.
func PositionsMarkdown(nodes []*Node) string {
	if len(nodes) == 0 {
		return "No positions match.\n"
	}
	var b strings.Builder
	for _, n := range nodes {
		fmt.Fprintf(&b, "- %s\n", positionLine(n))
	}
	return b.String()
}

// Markdown writes the position and what is linked to it. The full data
// snapshot is long; it is included only when asked.
func (d *PositionDetail) Markdown(withSnapshot bool) string {
	var b strings.Builder
	p := d.Position
	fmt.Fprintf(&b, "# %s\n\n- %s\n- Risk at the stop: %.2f USDT, quantity %g\n", p.Title, positionLine(p), p.Num("risk"), p.Num("qty"))
	if a := d.Analysis; a != nil {
		fmt.Fprintf(&b, "\n## %s (%s)\n\n%s\n", a.Title, a.Str("time"), a.Str("text"))
	}
	if len(d.Approaches) > 0 {
		fmt.Fprintf(&b, "\n## Approaches\n\n")
		for _, a := range d.Approaches {
			fmt.Fprintf(&b, "- %s (`%s`): %s\n", a.Title, a.Str("name"), orDash(a.Str("description")))
		}
	}
	if sn := d.Snapshot; sn != nil {
		fmt.Fprintf(&b, "\n## %s (%s)\n\n", sn.Title, sn.Str("time"))
		shown := map[string]bool{"n": true, "file": true, "time": true, "markdown": true}
		for _, k := range snapshotFieldOrder {
			if v, ok := sn.Props[k]; ok {
				fmt.Fprintf(&b, "- %s: %v\n", k, v)
				shown[k] = true
			}
		}
		var rest []string
		for k := range sn.Props {
			if !shown[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			fmt.Fprintf(&b, "- %s: %v\n", k, sn.Props[k])
		}
		if withSnapshot {
			fmt.Fprintf(&b, "\n%s\n", sn.Str("markdown"))
		}
	} else {
		fmt.Fprintf(&b, "\n## Data Snapshot\n\nNot recorded for this position.\n")
	}
	if cs := d.Summary; cs != nil {
		fmt.Fprintf(&b, "\n## %s: %s\n\n", cs.Title, strings.ToUpper(cs.Str("outcome")))
		fmt.Fprintf(&b, "- Closed by %s at %s after %d minutes: %s USDT (%+.2fR) after fees and funding\n",
			cs.Str("closed_by"), cs.Str("closed"), int(cs.Num("hold_minutes")), money(cs.Num("net")), cs.Num("r"))
		if cs.Num("mfe_r") != 0 || cs.Num("mae_r") != 0 {
			fmt.Fprintf(&b, "- Best point %+.2fR, worst point %+.2fR before the close\n", cs.Num("mfe_r"), cs.Num("mae_r"))
		}
		if r := cs.Str("reason"); r != "" {
			fmt.Fprintf(&b, "- Close reason: %s\n", r)
		}
		fmt.Fprintf(&b, "- Note: %s\n", orDash(cs.Str("note")))
	}
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "(none yet)"
	}
	return s
}

// ApproachesMarkdown writes the approach catalog with each approach's record.
func ApproachesMarkdown(s *Store) (string, error) {
	nodes, err := s.ByType(TypeApproach)
	if err != nil {
		return "", err
	}
	if len(nodes) == 0 {
		return "No approaches defined yet.\n", nil
	}
	var b strings.Builder
	b.WriteString("| approach | closed trades | win | avg R | net USDT | open | description |\n|---|---|---|---|---|---|---|\n")
	for _, n := range nodes {
		coded := ""
		if n.Bool("coded") {
			coded = " (coded)"
		}
		fmt.Fprintf(&b, "| `%s`%s | %d | %.0f%% | %+.2f | %s | %d | %s |\n", n.Str("name"), coded, int(n.Num("trades")),
			n.Num("win_rate"), n.Num("avg_r"), money(n.Num("net")), int(n.Num("open")), orDash(n.Str("description")))
	}
	b.WriteString("\nA record of a few trades is mostly luck: weigh it by the trade count.\n")
	return b.String(), nil
}

// ApproachMarkdown writes one approach: its description, record, and the
// positions that used it with their outcome and notes.
func ApproachMarkdown(s *Store, name string) (string, error) {
	a, err := s.Get(ApproachKey(name))
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("no approach %q", name)
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s (`%s`)\n\n%s\n\nRecord: %d closed trades, %.0f%% won, %+.2fR average, %s USDT net; %d open.\n",
		a.Title, name, orDash(a.Str("description")), int(a.Num("trades")), a.Num("win_rate"), a.Num("avg_r"), money(a.Num("net")), int(a.Num("open")))
	analyses, err := s.To(a.Key, RelWithApproach)
	if err != nil {
		return "", err
	}
	if len(analyses) > 0 {
		b.WriteString("\n## Positions\n\n")
	}
	for _, an := range analyses {
		d, err := GetPosition(s, int(an.Num("n")))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", positionLine(d.Position))
		if cs := d.Summary; cs != nil && cs.Str("note") != "" {
			fmt.Fprintf(&b, "  - Note: %s\n", cs.Str("note"))
		}
	}
	return b.String(), nil
}

func recordLine(n *Node) string {
	return fmt.Sprintf("%d closed trades, %s USDT, %+.2fR average, %.0f%% won, %d open",
		int(n.Num("trades")), money(n.Num("net")), n.Num("avg_r"), n.Num("win_rate"), int(n.Num("open")))
}

// PortfolioMarkdown writes the portfolio and, for each pair, its record with
// its profit and loss summaries, then the risk summary.
func PortfolioMarkdown(s *Store) (string, error) {
	pf, err := s.Get(KeyPortfolio)
	if errors.Is(err, ErrNotFound) {
		return "The knowledge graph is empty: run a sync.\n", nil
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "- Portfolio: %s\n", recordLine(pf))
	pairs, err := s.ByType(TypePair)
	if err != nil {
		return "", err
	}
	for _, p := range pairs {
		out, err := pairLines(s, p)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	risk, err := RiskMarkdown(s)
	if err != nil {
		return "", err
	}
	return b.String() + "\n" + risk, nil
}

// RiskMarkdown writes the configuration, its limits and the risk the
// portfolio carries against them.
func RiskMarkdown(s *Store) (string, error) {
	r, err := s.Get(KeyRiskSummary)
	if errors.Is(err, ErrNotFound) {
		return "The knowledge graph has no risk summary: run a sync.\n", nil
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("## Risk Summary\n\n")
	if c, err := s.Get(KeyConfiguration); err == nil {
		setups := strings.Join(c.Strs("setups"), ", ")
		if setups == "" {
			setups = "none"
		}
		fmt.Fprintf(&b, "- Configuration: %s on %s (context %s), coded setups %s, fee %g%% a fill, orders to %s\n",
			strings.Join(c.Strs("pairs"), ", "), c.Str("interval"), strings.Join(c.Strs("context_intervals"), ", "), setups, c.Num("fee_rate")*100, c.Str("trade_url"))
	}
	wallet := r.Bool("wallet_known")
	ofWallet := func(k string) string {
		if !wallet {
			return ""
		}
		return fmt.Sprintf(" (%.2f%% of the wallet)", r.Num(k))
	}
	maxRisk := "no cap"
	if v := r.Num("max_risk_usdt"); v > 0 {
		maxRisk = fmt.Sprintf("%.2f USDT%s", v, ofWallet("max_risk_pct"))
	}
	lev := max(int(r.Num("leverage")), 1)
	fmt.Fprintf(&b, "- Open position limit: %d open per pair, at most %.2f USDT a position, a stop-loss on each, loss at the stop %s\n",
		int(r.Num("max_open_per_pair")), r.Num("max_position_usdt"), maxRisk)
	fmt.Fprintf(&b, "- Leverage: %dx, so a full-size position needs %.2f USDT of margin\n", lev, r.Num("max_position_usdt")/float64(lev))
	mt5 := r.Strs("mt5_pairs")
	if len(mt5) > 0 {
		mt5Risk := "no cap"
		if v := r.Num("mt5_max_risk"); v > 0 {
			mt5Risk = fmt.Sprintf("%.2f USD", v)
		}
		fmt.Fprintf(&b, "- MetaTrader 5 demo (%s), in the account currency: at most %.2f USD a position, loss at the stop %s, leverage %dx\n",
			strings.Join(mt5, ", "), r.Num("mt5_max_position"), mt5Risk, int(r.Num("mt5_leverage")))
	}
	switch {
	case wallet && len(mt5) > 0:
		fmt.Fprintf(&b, "- Wallet: %.2f, Binance's USDT and MetaTrader's USD together\n", r.Num("wallet"))
	case wallet:
		fmt.Fprintf(&b, "- Wallet: %.2f USDT\n", r.Num("wallet"))
	default:
		b.WriteString("- Wallet: unknown (no API key, or the account could not be read)\n")
	}
	if n := int(r.Num("open")); n == 0 {
		b.WriteString("- Open: nothing at stake\n")
	} else {
		fmt.Fprintf(&b, "- Open: %d positions worth %.2f USDT, margin %.2f USDT%s; %.2f USDT lost if every stop fills now%s; unrealized %s USDT\n",
			n, r.Num("open_notional"), r.Num("open_margin"), ofWallet("margin_pct"), r.Num("open_risk"), ofWallet("open_risk_pct"), money(r.Num("unrealized")))
	}
	if n := int(r.Num("trades")); n > 0 {
		fmt.Fprintf(&b, "- Closed: %d trades, %s USDT net; worst %s USDT (%+.2fR); max drawdown %.2f USDT, now %.2f USDT below the peak; %d losses in a row\n",
			n, money(r.Num("net")), money(r.Num("worst_net")), r.Num("worst_r"), r.Num("max_drawdown"), r.Num("drawdown"), int(r.Num("loss_streak")))
	}
	for _, w := range r.Strs("warnings") {
		fmt.Fprintf(&b, "- Warning: %s\n", w)
	}
	return b.String(), nil
}

// pairLines writes a pair's record and its profit and loss summaries.
func pairLines(s *Store, p *Node) (string, error) {
	var b strings.Builder
	sym := p.Str("symbol")
	fmt.Fprintf(&b, "- %s: %s\n", p.Title, recordLine(p))
	for _, k := range []string{ProfitSummaryKey(sym), LossSummaryKey(sym)} {
		n, err := s.Get(k)
		if err != nil {
			return "", err
		}
		name, _, _ := strings.Cut(n.Title, " · ")
		fmt.Fprintf(&b, "  - %s: %d closed trades, %s USDT, %+.2fR average\n", name, int(n.Num("trades")), money(n.Num("net")), n.Num("avg_r"))
	}
	return b.String(), nil
}

// PairMarkdown writes one pair: its record, profit and loss, and positions.
func PairMarkdown(s *Store, symbol string) (string, error) {
	p, err := s.Get(PairKey(strings.ToUpper(symbol)))
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("no pair %q", symbol)
	}
	if err != nil {
		return "", err
	}
	lines, err := pairLines(s, p)
	if err != nil {
		return "", err
	}
	ps, err := Positions(s, Filter{Pair: symbol})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# %s\n\n%s\n## Positions\n\n%s", p.Title, lines, PositionsMarkdown(ps)), nil
}

// Graph is every node and edge.
type Graph struct {
	Nodes []*Node `json:"nodes"`
	Links []Link  `json:"links"`
}

// Types lists the node types in the order of docs/knowledge.md.
var Types = []string{TypeStrategyApproach, TypeApproach, TypeAnalysis, TypePosition, TypeSnapshot, TypeCloseSummary,
	TypeOpenPosition, TypeClosePositions, TypeProfitSummary, TypeLossSummary, TypePair, TypePairHub, TypePortfolio,
	TypeConfiguration, TypePairsTraded, TypeOpenLimit, TypeLeverage, TypeRiskSummary}

// All returns the whole graph.
func All(s *Store) (*Graph, error) {
	g := &Graph{}
	for _, t := range Types {
		ns, err := s.ByType(t)
		if err != nil {
			return nil, err
		}
		g.Nodes = append(g.Nodes, ns...)
	}
	for _, n := range g.Nodes {
		_, out, err := s.Links(n.Key)
		if err != nil {
			return nil, err
		}
		g.Links = append(g.Links, out...)
	}
	return g, nil
}

// NodeMarkdown is the readable view of any node: a position or its parts
// show the whole position, an approach its record and positions, a pair and
// its hubs the pair's positions, the portfolio its pairs.
func NodeMarkdown(s *Store, n *Node) (string, error) {
	num := int(n.Num("n"))
	switch n.Type {
	case TypePosition, TypeAnalysis, TypeCloseSummary:
		d, err := GetPosition(s, num)
		if err != nil {
			return "", err
		}
		return d.Markdown(false), nil
	case TypeSnapshot:
		return n.Str("markdown"), nil
	case TypeApproach:
		return ApproachMarkdown(s, n.Str("name"))
	case TypeStrategyApproach:
		return ApproachesMarkdown(s)
	case TypePortfolio, TypePairHub, TypePairsTraded:
		return PortfolioMarkdown(s)
	case TypePair:
		return PairMarkdown(s, n.Str("symbol"))
	case TypeConfiguration, TypeOpenLimit, TypeLeverage, TypeRiskSummary:
		return RiskMarkdown(s)
	case TypeOpenPosition, TypeClosePositions, TypeProfitSummary, TypeLossSummary:
		f := Filter{Pair: n.Str("symbol")}
		switch n.Type {
		case TypeOpenPosition:
			f.Status = "open"
		case TypeClosePositions:
			f.Status = "closed"
		case TypeProfitSummary:
			f.Outcome = "profit"
		case TypeLossSummary:
			f.Outcome = "loss"
		}
		ps, err := Positions(s, f)
		return PositionsMarkdown(ps), err
	}
	return "", nil
}

// snapshotFieldOrder is the order the market conditions are shown in.
var snapshotFieldOrder = []string{"price", "interval", "regime", "trends", "rsi", "atr_pct", "oi_change_24h", "taker_buy_sell_1h", "accounts_long_short", "funding"}
