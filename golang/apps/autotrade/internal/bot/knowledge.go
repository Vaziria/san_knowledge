package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wargasipil/autotrade/internal/knowledge"
	"github.com/wargasipil/autotrade/internal/strategy"
)

// The knowledge files live next to the journal:
//
//	data/approaches.json   the approach catalog (names, titles, descriptions)
//	data/snapshots/        last_<SYMBOL>.json/.md (each pair's latest snapshot) and one pair per open
//	data/knowledge/        the graph database, rebuilt from the files by SyncKnowledge

func (b *Bot) dataDir() string      { return filepath.Dir(b.Journal.Path) }
func (b *Bot) snapshotDir() string  { return filepath.Join(b.dataDir(), "snapshots") }
func (b *Bot) approachFile() string { return filepath.Join(b.dataDir(), "approaches.json") }

// KnowledgeDir is where the graph database is.
func (b *Bot) KnowledgeDir() string { return filepath.Join(b.dataDir(), "knowledge") }

// ---- approaches -----------------------------------------------------------------

// Approaches returns the approach catalog, sorted by name.
func (b *Bot) Approaches() ([]knowledge.Approach, error) {
	data, err := os.ReadFile(b.approachFile())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []knowledge.Approach
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", b.approachFile(), err)
	}
	return out, nil
}

// DefineApproach adds an approach to the catalog or updates its title and
// description.
func (b *Bot) DefineApproach(name, title, description string) (knowledge.Approach, error) {
	if !setupLabel.MatchString(name) {
		return knowledge.Approach{}, fmt.Errorf("approach name %q must be a short lowercase label such as big_trend", name)
	}
	if strings.TrimSpace(description) == "" {
		return knowledge.Approach{}, errors.New("an approach needs a description: what it reads and when it says to act")
	}
	list, err := b.Approaches()
	if err != nil {
		return knowledge.Approach{}, err
	}
	now := time.Now().UTC().Truncate(time.Second)
	var a *knowledge.Approach
	for i := range list {
		if list[i].Name == name {
			a = &list[i]
		}
	}
	if a == nil {
		list = append(list, knowledge.Approach{Name: name, Created: now})
		a = &list[len(list)-1]
	}
	if title == "" {
		title = a.Title
	}
	a.Title, a.Description, a.Updated, a.Coded = title, description, now, strategy.Known(name)
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	data, _ := json.MarshalIndent(list, "", "  ")
	if err := os.MkdirAll(b.dataDir(), 0o755); err != nil {
		return knowledge.Approach{}, err
	}
	if err := os.WriteFile(b.approachFile(), append(data, '\n'), 0o644); err != nil {
		return knowledge.Approach{}, err
	}
	for _, x := range list {
		if x.Name == name {
			return x, nil
		}
	}
	return *a, nil
}

// checkApproaches refuses names that are neither in the catalog nor coded
// setups, so every analysis links to approaches that say what they mean.
func (b *Bot) checkApproaches(names []string) error {
	list, err := b.Approaches()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, a := range list {
		known[a.Name] = true
	}
	var missing []string
	for _, n := range names {
		if !known[n] && !strategy.Known(n) {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unknown approach %s: define it first (approach -name NAME -description \"...\")", strings.Join(missing, ", "))
	}
	return nil
}

// ---- snapshots ---------------------------------------------------------------------

// lastName is the file name, without extension, of the pair's latest
// snapshot: last_BTCUSDT.
func (b *Bot) lastName() string { return "last_" + b.Cfg.Symbol }

// saveLast keeps the snapshot as data/snapshots/last_<SYMBOL>.json and .md,
// the data the pair's next decision is made on.
func (b *Bot) saveLast(s *Snapshot) error {
	if err := os.MkdirAll(b.snapshotDir(), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var md bytes.Buffer
	s.Render(&md)
	if err := os.WriteFile(filepath.Join(b.snapshotDir(), b.lastName()+".json"), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.snapshotDir(), b.lastName()+".md"), md.Bytes(), 0o644)
}

// keepSnapshot copies the pair's latest snapshot to a file of its own for an
// open, and returns its name (BTCUSDT_20261009T150405Z). A latest snapshot
// older than 10 minutes is not what the decision was made on, so a fresh one
// is taken instead.
func (b *Bot) keepSnapshot(ctx context.Context, at time.Time) (string, error) {
	last := filepath.Join(b.snapshotDir(), b.lastName()+".json")
	var s Snapshot
	data, err := os.ReadFile(last)
	if err == nil {
		err = json.Unmarshal(data, &s)
	}
	if err != nil || s.Symbol != b.Cfg.Symbol || at.Sub(s.Now) > 10*time.Minute || s.Now.Sub(at) > time.Minute {
		fresh, err := b.Snapshot(ctx, 12) // saves the last snapshot
		if err != nil {
			return "", err
		}
		s = *fresh
	}
	name := b.Cfg.Symbol + "_" + s.Now.UTC().Format("20060102T150405Z")
	for _, ext := range []string{".json", ".md"} {
		data, err := os.ReadFile(filepath.Join(b.snapshotDir(), b.lastName()+ext))
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(b.snapshotDir(), name+ext), data, 0o644); err != nil {
			return "", err
		}
	}
	return name, nil
}

// conditions are the snapshot's market conditions as flat fields, so the
// graph can be queried by them.
func conditions(s *Snapshot) map[string]any {
	m := map[string]any{"price": s.Price}
	if len(s.Frames) == 0 {
		return m
	}
	base := s.Frames[0]
	m["interval"] = base.Interval
	var trends []string
	for _, f := range s.Frames {
		m["trend_"+f.Interval] = f.Trend
		trends = append(trends, f.Interval+" "+f.Trend)
	}
	m["trends"] = strings.Join(trends, ", ")
	if base.RSI14.Valid() {
		m["rsi"] = math.Round(float64(base.RSI14)*10) / 10
	}
	if base.ATR14.Valid() && base.Close > 0 {
		m["atr_pct"] = math.Round(float64(base.ATR14)/base.Close*10000) / 100
	}
	if s.Signals != nil {
		m["regime"] = s.Signals.Regime
	}
	if p := s.Positioning; p != nil {
		m["oi_change_24h"] = math.Round(p.OIChange24h*100) / 100
		m["taker_buy_sell_1h"] = p.TakerBuySell1h
		m["accounts_long_short"] = p.AccountsLS
		if n := len(p.Funding); n > 0 {
			m["funding"] = p.Funding[n-1].Rate
		}
	}
	return m
}

func (b *Bot) loadSnapshot(name string) (*knowledge.Snapshot, error) {
	data, err := os.ReadFile(filepath.Join(b.snapshotDir(), name+".json"))
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	md, _ := os.ReadFile(filepath.Join(b.snapshotDir(), name+".md"))
	return &knowledge.Snapshot{File: name, Time: s.Now, Markdown: string(md), Fields: conditions(&s)}, nil
}

// ---- notes ----------------------------------------------------------------------------

// Note records the trader's summary of a position, normally the lesson after
// it closed. The latest note of a position is its close summary's note.
func (b *Bot) Note(ctx context.Context, position int, text string) (Entry, error) {
	if position < 1 || strings.TrimSpace(text) == "" {
		return Entry{}, errors.New("a note needs a position number (1 = the first open) and a text")
	}
	all, err := b.Journal.Last(math.MaxInt)
	if err != nil {
		return Entry{}, err
	}
	opens, sym := 0, b.Cfg.Symbol
	for _, e := range all {
		if e.Error == "" && strings.HasPrefix(e.Action, "open_") {
			if opens++; opens == position && e.Symbol != "" {
				sym = e.Symbol // the note belongs to the position's pair
			}
		}
	}
	if position > opens {
		return Entry{}, fmt.Errorf("there is no position #%d: the journal has %d", position, opens)
	}
	e := Entry{Time: b.now(ctx), Symbol: sym, Action: "note", Position: position, Reason: text}
	return b.record(e, nil)
}

// ---- sync -------------------------------------------------------------------------------

// KnowledgeInput gathers what the graph is built from: the approach catalog,
// and every position of the journal with its analysis, snapshot and outcome.
func (b *Bot) KnowledgeInput(ctx context.Context, known map[string]knowledge.Close) (knowledge.Input, error) {
	all, err := b.Journal.Last(math.MaxInt)
	if err != nil {
		return knowledge.Input{}, err
	}
	trades, err := b.trades(ctx, all, known)
	if err != nil {
		return knowledge.Input{}, err
	}
	approaches, err := b.Approaches()
	if err != nil {
		return knowledge.Input{}, err
	}
	have := map[string]bool{}
	for _, a := range approaches {
		have[a.Name] = true
	}
	for _, name := range strategy.AllSetups {
		if !have[name] {
			approaches = append(approaches, knowledge.Approach{Name: name, Title: titleCase(name), Description: strategy.Descriptions[name], Coded: true})
		}
	}

	c := b.Cfg
	in := knowledge.Input{Approaches: approaches, Config: knowledge.Config{
		Pairs: c.Pairs, Interval: c.Interval, ContextIntervals: c.ContextIntervals, TradeURL: c.TradeURL, Setups: c.Setups,
		FeeRate: c.FeeRate, Leverage: c.Leverage, MaxPositionUSDT: c.MaxPositionUSDT, MaxRiskUSDT: c.MaxRiskUSDT,
		MaxOpenPerPair: maxOpenPerPair,
	}}
	if b.Ex.HasKey() {
		if a, err := b.Ex.Account(ctx); err == nil {
			in.Account = &knowledge.Account{Wallet: a.WalletBalance, Available: a.AvailableBalance}
		}
	}
	for i, t := range trades {
		p := knowledge.Position{N: i + 1, Symbol: t.Symbol, Side: t.Side, Status: "closed", Opened: t.Opened, Entry: t.Entry, Stop: t.Stop,
			StopNow: stopNow(all, t, c.Symbol), Target: t.Target, Qty: t.Qty, Approaches: t.Approaches, Analysis: t.Reason}
		if t.ClosedBy == "open" {
			p.Status, p.Unrealized = "open", t.Net
		} else {
			p.Close = &knowledge.Close{ClosedBy: t.ClosedBy, Closed: t.Ended, Net: t.Net, R: t.R, Final: t.Final,
				MFE: t.MFE, MAE: t.MAE, Reason: t.CloseNote, Note: t.Note}
		}
		if t.Snapshot != "" {
			if sn, err := b.loadSnapshot(t.Snapshot); err == nil {
				p.Snapshot = sn
			}
		}
		in.Positions = append(in.Positions, p)
	}
	return in, nil
}

// maxOpenPerPair is fixed: an open is refused while the pair has a position.
const maxOpenPerPair = 1

// stopNow is the stop of the trade's last protect, 0 if it was never moved.
func stopNow(all []Entry, t ReviewTrade, defaultSymbol string) float64 {
	stop := 0.0
	for _, e := range all {
		sym := e.Symbol
		if sym == "" {
			sym = defaultSymbol
		}
		if e.Action != "protect" || e.Error != "" || e.StopLoss <= 0 || sym != t.Symbol || !e.Time.After(t.Opened) {
			continue
		}
		if t.ClosedBy != "open" && e.Time.After(t.Ended) {
			break
		}
		stop = e.StopLoss
	}
	return stop
}

// SyncKnowledge rebuilds the knowledge graph from the files. The database is
// open only to read the final figures and to write the result, never while
// the exchange is asked.
func (b *Bot) SyncKnowledge(ctx context.Context) (knowledge.Result, error) {
	var known map[string]knowledge.Close
	if err := knowledge.With(b.KnowledgeDir(), func(s *knowledge.Store) error {
		var err error
		known, err = knowledge.Finals(s)
		return err
	}); err != nil {
		return knowledge.Result{}, err
	}
	in, err := b.KnowledgeInput(ctx, known)
	if err != nil {
		return knowledge.Result{}, err
	}
	var res knowledge.Result
	err = knowledge.With(b.KnowledgeDir(), func(s *knowledge.Store) error {
		var err error
		res, err = knowledge.Sync(s, in)
		return err
	})
	return res, err
}

func titleCase(name string) string {
	words := strings.Fields(strings.ReplaceAll(name, "_", " "))
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
