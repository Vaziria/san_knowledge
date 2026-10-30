package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Run is one run of a workflow. What its steps did is one protojson column,
// cut to what a run keeps (boundSteps), and its summary, the last lines
// they logged, another, for the Logs pages; the columns the list shows stay
// columns.
type Run struct {
	ID         uint32       `gorm:"primaryKey"`
	WorkflowID uint32       `gorm:"not null;index"`
	Status     v1.RunStatus `gorm:"not null"`
	Trigger    string       `gorm:"not null"`
	StartedAt  time.Time    `gorm:"not null"`
	FinishedAt *time.Time
	Error      string         `gorm:"not null"`
	Steps      *v1.RunSteps   `gorm:"serializer:protojson"`
	Summary    *v1.RunSummary `gorm:"serializer:protojson"`
}

// MaxRunsKept is how many runs of a workflow are kept: finishing one deletes
// the oldest past this.
const MaxRunsKept = 100

// StartRun records that a run of a workflow started.
func (s *Store) StartRun(workflowID uint32, trigger string) (Run, error) {
	r := Run{WorkflowID: workflowID, Status: v1.RunStatus_RUN_STATUS_RUNNING, Trigger: trigger, StartedAt: time.Now(), Steps: &v1.RunSteps{}}
	return r, s.db.Create(&r).Error
}

// FinishRun records how a run ended, with what its steps did cut to what a
// run keeps (in place) and its summary, then keeps only the newest
// MaxRunsKept runs of its workflow.
func (s *Store) FinishRun(r Run, status v1.RunStatus, errText string, steps []*v1.StepRun) (Run, error) {
	now := time.Now()
	boundSteps(steps)
	r.Status, r.Error, r.FinishedAt = status, clip(errText, MaxLineLen), &now
	r.Steps, r.Summary = &v1.RunSteps{Steps: steps}, summarize(steps)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Run{ID: r.ID}).Select("Status", "Error", "FinishedAt", "Steps", "Summary").Updates(&r).Error; err != nil {
			return err
		}
		// A run still waiting in its workflow's queue is kept, however old.
		keep := tx.Model(&Run{}).Select("id").Where("workflow_id = ?", r.WorkflowID).Order("id DESC").Limit(MaxRunsKept)
		return tx.Where("workflow_id = ? AND status <> ? AND id NOT IN (?)", r.WorkflowID, v1.RunStatus_RUN_STATUS_RUNNING, keep).Delete(&Run{}).Error
	})
	return r, err
}

// ListRuns lists a workflow's newest runs first, without their steps or
// summaries.
func (s *Store) ListRuns(workflowID uint32, limit int) ([]Run, error) {
	var list []Run
	err := s.db.Omit("Steps", "Summary").Where("workflow_id = ?", workflowID).Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

func (s *Store) GetRun(id uint32) (Run, error) {
	var r Run
	err := s.db.First(&r, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r, fail(ErrNotFound, "no run %d", id)
	}
	return r, err
}

// RunHeads are the runs of ids that are still kept, without their steps or
// summaries: how a chat's runs are doing.
func (s *Store) RunHeads(ids []uint32) ([]Run, error) {
	var list []Run
	if len(ids) == 0 {
		return list, nil
	}
	return list, s.db.Omit("Steps", "Summary").Where("id IN ?", ids).Order("id").Find(&list).Error
}

// RunBatch counts the batch run is in (the Queue Progress step): the runs
// of its workflow each started, as a one-at-a-time workflow queues them,
// before the one before it ended. done is how many of the batch before run
// ended, waiting how many after it are still going or waiting. A run the
// tower stopped (interrupted) ends a batch: its event runs again as a new
// run.
func (s *Store) RunBatch(workflowID, runID uint32) (done, waiting int, err error) {
	var cur Run
	if err := s.db.Omit("Steps", "Summary").First(&cur, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, fail(ErrNotFound, "no run %d", runID)
		}
		return 0, 0, err
	}
	var n int64
	if err := s.db.Model(&Run{}).Where("workflow_id = ? AND id > ? AND status = ?", workflowID, runID, v1.RunStatus_RUN_STATUS_RUNNING).Count(&n).Error; err != nil {
		return 0, 0, err
	}
	var before []Run
	if err := s.db.Omit("Steps", "Summary").Where("workflow_id = ? AND id < ?", workflowID, runID).Order("id DESC").Limit(MaxRunsKept).Find(&before).Error; err != nil {
		return 0, 0, err
	}
	next := cur
	for _, r := range before {
		if r.Status == v1.RunStatus_RUN_STATUS_INTERRUPTED || (r.FinishedAt != nil && !r.FinishedAt.After(next.StartedAt)) {
			break
		}
		done++
		next = r
	}
	return done, int(n), nil
}

// CountRunning is how many runs are still going.
func (s *Store) CountRunning() (int64, error) {
	var n int64
	err := s.db.Model(&Run{}).Where("status = ?", v1.RunStatus_RUN_STATUS_RUNNING).Count(&n).Error
	return n, err
}

// InterruptRuns marks the runs still running as interrupted: the tower
// stopped during them. It returns how many there were.
func (s *Store) InterruptRuns() (int64, error) {
	r := s.db.Model(&Run{}).Where("status = ?", v1.RunStatus_RUN_STATUS_RUNNING).Updates(map[string]any{
		"status":      v1.RunStatus_RUN_STATUS_INTERRUPTED,
		"error":       "the tower stopped during the run",
		"finished_at": time.Now(),
	})
	return r.RowsAffected, r.Error
}

// ListAllRuns lists the newest runs of one workflow (workflowID), or of
// every workflow (0) or only of those switched on (activeOnly), newest
// first, with their summaries and not their steps: the Logs pages ask every
// 2 s. With statuses, only the runs with one of them.
func (s *Store) ListAllRuns(workflowID uint32, activeOnly bool, statuses []v1.RunStatus, limit int) ([]Run, error) {
	q := s.db.Omit("Steps").Order("id DESC").Limit(limit)
	if workflowID != 0 {
		q = q.Where("workflow_id = ?", workflowID)
	} else if activeOnly {
		q = q.Where("workflow_id IN (?)", s.db.Model(&Workflow{}).Select("id").Where("active = ?", true))
	}
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var list []Run
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	return list, s.summarizeOld(list)
}

// summarizeOld gives the finished runs stored before runs had a summary
// theirs, from their steps, and stores it, so each is read once. Such a
// run's summary column is NULL (which reads as an empty summary).
func (s *Store) summarizeOld(list []Run) error {
	ids := make([]uint32, 0, len(list))
	at := map[uint32]int{}
	for i, r := range list {
		ids = append(ids, r.ID)
		at[r.ID] = i
	}
	if len(ids) == 0 {
		return nil
	}
	var old []Run
	err := s.db.Select("id", "steps").Where("id IN ? AND summary IS NULL AND status <> ?", ids, v1.RunStatus_RUN_STATUS_RUNNING).Find(&old).Error
	if err != nil {
		return err
	}
	for _, o := range old {
		sum := summarize(o.Steps.GetSteps())
		list[at[o.ID]].Summary = sum
		if err := s.db.Model(&Run{ID: o.ID}).Select("Summary").Updates(&Run{Summary: sum}).Error; err != nil {
			return err
		}
	}
	return nil
}

// WorkflowNames are the workflows' names by ID.
func (s *Store) WorkflowNames() (map[uint32]string, error) {
	var rows []struct {
		ID   uint32
		Name string
	}
	if err := s.db.Model(&Workflow{}).Select("id, name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	names := make(map[uint32]string, len(rows))
	for _, r := range rows {
		names[r.ID] = r.Name
	}
	return names, nil
}
