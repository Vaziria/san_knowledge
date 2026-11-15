package livetrading

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bt "github.com/wargasipil/san_backtest"

	"github.com/wargasipil/san_tower/internal/store"
)

// Result puts a session's run together again from the store: its journal
// (trades, decisions, missed signals, equity, candles) and its info, with
// the stats recomputed by San Backtest. nil when the session has not run a
// candle yet.
func (m *Manager) Result(id uint32) (*bt.Result, store.PaperSession, error) {
	p, err := m.cfg.Store.GetPaperSession(id)
	if err != nil {
		return nil, p, err
	}
	var info infoJSON
	if err := json.Unmarshal([]byte(p.Info), &info); err != nil {
		return nil, p, fmt.Errorf("paper session %d: info: %w", id, err)
	}
	if info.Run.Balance == 0 {
		return nil, p, nil
	}
	res := &bt.Result{Name: info.Name, Range: info.Range, Costs: info.Costs, Run: info.Run, Params: info.Params}
	read := func(kind string, each func(data []byte) error) error {
		recs, err := m.cfg.Store.PaperRecords(id, kind, time.Time{}, time.Time{}, 0)
		if err != nil {
			return err
		}
		for _, r := range recs {
			if err := each([]byte(r.Data)); err != nil {
				return fmt.Errorf("paper session %d: %s: %w", id, kind, err)
			}
		}
		return nil
	}
	var cs []bt.Candle
	err = errors.Join(
		read(store.PaperTrade, func(b []byte) error {
			var t bt.Trade
			err := json.Unmarshal(b, &t)
			res.Trades = append(res.Trades, t)
			return err
		}),
		read(store.PaperDecision, func(b []byte) error {
			var d bt.Decision
			err := json.Unmarshal(b, &d)
			res.Decisions = append(res.Decisions, d)
			return err
		}),
		read(store.PaperMissed, func(b []byte) error {
			var x bt.Missed
			err := json.Unmarshal(b, &x)
			res.Missed = append(res.Missed, x)
			return err
		}),
		read(store.PaperEquity, func(b []byte) error {
			var e bt.EquityPoint
			err := json.Unmarshal(b, &e)
			res.Equity = append(res.Equity, e)
			return err
		}),
		read(store.PaperCandle, func(b []byte) error { var c bt.Candle; err := json.Unmarshal(b, &c); cs = append(cs, c); return err }),
	)
	if err != nil {
		return nil, p, err
	}
	if len(cs) > 0 && len(info.Run.Legs) > 0 {
		l := info.Run.Legs[0]
		res.Candles = []bt.CandleSet{{Leg: l.Leg, Instrument: l.Instrument, Timeframe: l.Timeframes[0], Candles: cs}}
	}
	res.Range.Notes = append(res.Range.Notes, paperNote(p))
	res.Restat()
	return res, p, nil
}

// paperNote says in a report what it is.
func paperNote(p store.PaperSession) string {
	n := fmt.Sprintf("PAPER session %d (%s), San Tower's Live Trading: no orders were sent", p.ID, p.Name)
	if p.OpenPositions > 0 || p.PendingOrders > 0 {
		n += fmt.Sprintf("; %d positions and %d pending orders were still open (not in the trades)", p.OpenPositions, p.PendingOrders)
	}
	return n
}

// View is session id's report folder for Backtest Analysis (San
// Backtest's Save of its Result, in <data>/live_trading/view/<id>), made
// again when the session changed since. The store is the source; the
// folder is only how the reader opens it, like a backtest's report.
func (m *Manager) View(id uint32) (string, store.PaperSession, error) {
	m.views.Lock()
	defer m.views.Unlock()
	p, err := m.cfg.Store.GetPaperSession(id)
	if err != nil {
		return "", p, err
	}
	dir := m.viewDir(id)
	stamp := filepath.Join(dir, ".stamp")
	want := p.UpdatedAt.UTC().Format(time.RFC3339Nano) + " " + p.Status
	if b, err := os.ReadFile(stamp); err == nil && string(b) == want {
		return dir, p, nil
	}
	res, p, err := m.Result(id)
	if err != nil {
		return "", p, err
	}
	if res == nil {
		return "", p, fmt.Errorf("%w: paper session %d has not run a candle yet", store.ErrNotFound, id)
	}
	if err := removeAll(dir); err != nil {
		return "", p, err
	}
	if _, err := res.Save(dir); err != nil {
		return "", p, err
	}
	return dir, p, os.WriteFile(stamp, []byte(want), 0o644)
}

// Export writes session id as a normal San Backtest report folder (run.json,
// trades.csv, ...: Result.Save) at path, absolute; "" is
// <data>/live_trading/exports/<id>-<approach>-<instrument>-<time>. The
// folder must be new or empty. It returns the folder and its files.
func (m *Manager) Export(id uint32, path string) (string, []string, error) {
	res, p, err := m.Result(id)
	if err != nil {
		return "", nil, err
	}
	if res == nil {
		return "", nil, fmt.Errorf("%w: paper session %d has not run a candle yet: nothing to export", store.ErrInvalid, id)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = m.exportDir(p)
	}
	if !filepath.IsAbs(path) {
		return "", nil, fmt.Errorf("%w: the export folder must be an absolute path, got %q", store.ErrInvalid, path)
	}
	path = filepath.Clean(path)
	if ents, err := os.ReadDir(path); err == nil && len(ents) > 0 {
		return "", nil, fmt.Errorf("%w: %s is not empty", store.ErrExists, path)
	}
	files, err := res.Save(path)
	return path, files, err
}

// removeAll removes a view folder, only below a "view" folder (a guard
// against a wrong path).
func removeAll(dir string) error {
	if filepath.Base(filepath.Dir(dir)) != "view" {
		return fmt.Errorf("refusing to remove %s", dir)
	}
	return os.RemoveAll(dir)
}
