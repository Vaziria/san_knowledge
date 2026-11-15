package store

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// PaperSession is a paper trading session of the tower's Live Trading page
// (the user, 2026-11-15: "realtime trading menu in tower", first stage
// "Monitor + Signals (paper trading)"): a San Backtest approach run live on
// an Exness connection's candles, placing no order anywhere. Several run at
// once (the user, 2026-11-15: "several paper sessions at once"). Its
// journal (trades, decisions, equity, missed signals and the candles it
// ran) is in PaperRecord, here in the tower's own database (the user,
// 2026-11-15: "4b (using storage)").
type PaperSession struct {
	ID           uint32 `gorm:"primaryKey"`
	Name         string `gorm:"not null;default:''"`
	ConnectionID uint32 `gorm:"not null;default:0"` // 0: the first Exness connection
	Approach     string `gorm:"not null"`
	Instrument   string `gorm:"not null"`
	// Timeframes are the approach's, the step first, comma separated.
	Timeframes  string  `gorm:"not null;default:''"`
	Balance     float64 `gorm:"not null;default:10000"`
	Risk        float64 `gorm:"not null;default:1"`
	Leverage    int32   `gorm:"not null;default:2000"`
	MaxOpen     int32   `gorm:"not null;default:0"` // 0: the approach's own, else 1
	MaxOpenRisk float64 `gorm:"not null;default:0"`
	// Params is the approach's params set on the session, a JSON object
	// (only the ones that differ from the approach's defaults).
	Params string `gorm:"not null;default:'{}'"`
	// Costs is a JSON object of san_backtest Costs overrides; "{}" keeps
	// Exness's real costs (the default).
	Costs string `gorm:"not null;default:'{}'"`
	// From is the open of the session's first step candle, set by the
	// first Start; a restart replays from it.
	From *time.Time
	// Status is stopped, starting, running, waiting, market_closed or
	// error; Message says more (why it stopped, what it waits for).
	Status  string `gorm:"not null;default:'stopped'"`
	Message string `gorm:"not null;default:''"`
	// The session's numbers as of its last candle, for the list.
	NowBalance    float64 `gorm:"not null;default:0"`
	Equity        float64 `gorm:"not null;default:0"`
	NetProfit     float64 `gorm:"not null;default:0"`
	Trades        uint32  `gorm:"not null;default:0"`
	OpenPositions uint32  `gorm:"not null;default:0"`
	PendingOrders uint32  `gorm:"not null;default:0"`
	Candles       uint32  `gorm:"not null;default:0"`
	LastClose     *time.Time
	NextClose     *time.Time
	LastSignal    string `gorm:"not null;default:''"`
	LastSignalAt  *time.Time
	// State is the open positions and pending orders as JSON, Info the
	// run's range, costs and settings as ran (JSON), so a stopped session
	// still shows and exports whole.
	State     string `gorm:"not null;default:'{}'"`
	Info      string `gorm:"not null;default:'{}'"`
	StartedAt *time.Time
	StoppedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// The statuses of a paper session.
const (
	PaperStopped  = "stopped"
	PaperStarting = "starting" // loading the warm-up, catching up
	PaperRunning  = "running"  // stepping a candle
	PaperWaiting  = "waiting"  // waiting for the next candle
	PaperClosed   = "market_closed"
	PaperError    = "error"
)

// PaperRecord is one line of a paper session's journal: Kind trade,
// decision, missed, equity or candle, Data its san_backtest value as JSON,
// Time its time (a trade's close, a candle's open).
type PaperRecord struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	SessionID uint32    `gorm:"not null;index:idx_paper_record,priority:1"`
	Kind      string    `gorm:"not null;index:idx_paper_record,priority:2"`
	Time      time.Time `gorm:"not null;index:idx_paper_record,priority:3"`
	Data      string    `gorm:"not null"`
}

// The kinds of journal lines.
const (
	PaperTrade    = "trade"
	PaperDecision = "decision"
	PaperMissed   = "missed"
	PaperEquity   = "equity"
	PaperCandle   = "candle"
)

func checkPaper(p *PaperSession) error {
	p.Approach, p.Instrument, p.Name = strings.TrimSpace(p.Approach), strings.TrimSpace(p.Instrument), strings.TrimSpace(p.Name)
	switch {
	case p.Approach == "":
		return fail(ErrInvalid, "a paper session needs an approach")
	case p.Instrument == "":
		return fail(ErrInvalid, "a paper session needs an instrument, such as XAUUSD")
	case p.Balance <= 0:
		return fail(ErrInvalid, "the balance must be more than 0")
	case p.Risk <= 0 || p.Risk > 100:
		return fail(ErrInvalid, "the risk must be a percent from above 0 to 100")
	case p.Leverage <= 0:
		return fail(ErrInvalid, "the leverage must be more than 0")
	case p.MaxOpen < 0:
		return fail(ErrInvalid, "max open must be 0 (the approach's own) or more")
	case p.MaxOpenRisk < 0 || p.MaxOpenRisk > 100:
		return fail(ErrInvalid, "max open risk must be a percent from 0 (no limit) to 100")
	}
	if p.Name == "" {
		p.Name = p.Approach + " on " + p.Instrument
	}
	if p.Params == "" {
		p.Params = "{}"
	}
	if p.Costs == "" {
		p.Costs = "{}"
	}
	if p.State == "" {
		p.State = "{}"
	}
	if p.Info == "" {
		p.Info = "{}"
	}
	return nil
}

// CreatePaperSession saves a new session, stopped.
func (s *Store) CreatePaperSession(p PaperSession) (PaperSession, error) {
	if err := checkPaper(&p); err != nil {
		return p, err
	}
	p.ID, p.Status, p.From = 0, PaperStopped, nil
	return p, s.db.Create(&p).Error
}

// GetPaperSession reads one session.
func (s *Store) GetPaperSession(id uint32) (PaperSession, error) {
	var p PaperSession
	err := s.db.First(&p, id).Error
	if err == gorm.ErrRecordNotFound {
		return p, fail(ErrNotFound, "paper session %d not found", id)
	}
	return p, err
}

// ListPaperSessions lists every session, newest first.
func (s *Store) ListPaperSessions() ([]PaperSession, error) {
	var list []PaperSession
	return list, s.db.Order("id DESC").Find(&list).Error
}

// SavePaperSession writes a session's every field.
func (s *Store) SavePaperSession(p PaperSession) error {
	if err := checkPaper(&p); err != nil {
		return err
	}
	return s.db.Save(&p).Error
}

// SetPaperStatus changes a session's status and message only.
func (s *Store) SetPaperStatus(id uint32, status, message string) error {
	return s.db.Model(&PaperSession{ID: id}).Updates(map[string]any{"status": status, "message": message}).Error
}

// DeletePaperSession deletes a session and its journal.
func (s *Store) DeletePaperSession(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", id).Delete(&PaperRecord{}).Error; err != nil {
			return err
		}
		r := tx.Delete(&PaperSession{}, id)
		if r.Error == nil && r.RowsAffected == 0 {
			return fail(ErrNotFound, "paper session %d not found", id)
		}
		return r.Error
	})
}

// WritePaperJournal saves the session and adds recs to its journal, in
// one transaction; replace clears the journal first (a restart's replay
// writes the whole journal again).
func (s *Store) WritePaperJournal(p PaperSession, recs []PaperRecord, replace bool) error {
	if err := checkPaper(&p); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if replace {
			if err := tx.Where("session_id = ?", p.ID).Delete(&PaperRecord{}).Error; err != nil {
				return err
			}
		}
		for i := range recs {
			recs[i].ID, recs[i].SessionID = 0, p.ID
		}
		if len(recs) > 0 {
			if err := tx.CreateInBatches(recs, 500).Error; err != nil {
				return err
			}
		}
		return tx.Save(&p).Error
	})
}

// PaperRecords reads a session's journal lines of kind, in time order (and
// for one time, in the order written): those from from (inclusive) to to
// (exclusive) when set, the newest limit of them when limit > 0.
func (s *Store) PaperRecords(id uint32, kind string, from, to time.Time, limit int) ([]PaperRecord, error) {
	q := s.db.Where("session_id = ? AND kind = ?", id, kind)
	if !from.IsZero() {
		q = q.Where("time >= ?", from)
	}
	if !to.IsZero() {
		q = q.Where("time < ?", to)
	}
	var list []PaperRecord
	if limit > 0 {
		if err := q.Order("time DESC, id DESC").Limit(limit).Find(&list).Error; err != nil {
			return nil, err
		}
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
		return list, nil
	}
	return list, q.Order("time, id").Find(&list).Error
}

// CountPaperRecords counts a session's journal lines of kind.
func (s *Store) CountPaperRecords(id uint32, kind string) (int64, error) {
	var n int64
	return n, s.db.Model(&PaperRecord{}).Where("session_id = ? AND kind = ?", id, kind).Count(&n).Error
}

// StopPaperSessions marks every session that was going as stopped, with
// message: after a tower restart they stay stopped until started by hand
// (the user, 2026-11-15: "3b"). It returns how many it stopped.
func (s *Store) StopPaperSessions(message string) (int64, error) {
	now := time.Now()
	r := s.db.Model(&PaperSession{}).Where("status <> ? AND status <> ?", PaperStopped, PaperError).
		Updates(map[string]any{"status": PaperStopped, "message": message, "stopped_at": now})
	return r.RowsAffected, r.Error
}
