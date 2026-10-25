package store

import (
	"errors"
	"time"

	"gorm.io/gorm"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Run is one run of a workflow. What its steps did is one protojson column;
// the columns the list shows stay columns.
type Run struct {
	ID         uint32       `gorm:"primaryKey"`
	WorkflowID uint32       `gorm:"not null;index"`
	Status     v1.RunStatus `gorm:"not null"`
	Trigger    string       `gorm:"not null"`
	StartedAt  time.Time    `gorm:"not null"`
	FinishedAt *time.Time
	Error      string       `gorm:"not null"`
	Steps      *v1.RunSteps `gorm:"serializer:protojson"`
}

// MaxRunsKept is how many runs of a workflow are kept: finishing one deletes
// the oldest past this.
const MaxRunsKept = 100

// StartRun records that a run of a workflow started.
func (s *Store) StartRun(workflowID uint32, trigger string) (Run, error) {
	r := Run{WorkflowID: workflowID, Status: v1.RunStatus_RUN_STATUS_RUNNING, Trigger: trigger, StartedAt: time.Now(), Steps: &v1.RunSteps{}}
	return r, s.db.Create(&r).Error
}

// FinishRun records how a run ended, then keeps only the newest MaxRunsKept
// runs of its workflow.
func (s *Store) FinishRun(r Run, status v1.RunStatus, errText string, steps []*v1.StepRun) (Run, error) {
	now := time.Now()
	r.Status, r.Error, r.FinishedAt, r.Steps = status, errText, &now, &v1.RunSteps{Steps: steps}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Run{ID: r.ID}).Select("Status", "Error", "FinishedAt", "Steps").Updates(&r).Error; err != nil {
			return err
		}
		keep := tx.Model(&Run{}).Select("id").Where("workflow_id = ?", r.WorkflowID).Order("id DESC").Limit(MaxRunsKept)
		return tx.Where("workflow_id = ? AND id NOT IN (?)", r.WorkflowID, keep).Delete(&Run{}).Error
	})
	return r, err
}

// ListRuns lists a workflow's newest runs first, without their steps.
func (s *Store) ListRuns(workflowID uint32, limit int) ([]Run, error) {
	var list []Run
	err := s.db.Omit("Steps").Where("workflow_id = ?", workflowID).Order("id DESC").Limit(limit).Find(&list).Error
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

// ListAllRuns lists the newest runs of every workflow, or only of those
// switched on (activeOnly), newest first, with their steps. With statuses,
// only the runs with one of them.
func (s *Store) ListAllRuns(activeOnly bool, statuses []v1.RunStatus, limit int) ([]Run, error) {
	q := s.db.Order("id DESC").Limit(limit)
	if activeOnly {
		q = q.Where("workflow_id IN (?)", s.db.Model(&Workflow{}).Select("id").Where("active = ?", true))
	}
	if len(statuses) > 0 {
		q = q.Where("status IN ?", statuses)
	}
	var list []Run
	err := q.Find(&list).Error
	return list, err
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
