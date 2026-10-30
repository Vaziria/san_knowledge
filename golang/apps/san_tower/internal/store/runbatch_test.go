package store

import (
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// A batch is the runs each started before the one before it ended: a run
// that ended before the next started, or one the tower stopped, ends it;
// those after the run still going are waiting.
func TestRunBatch(t *testing.T) {
	s := open(t)
	w, err := s.CreateWorkflow("underground")
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 29, 19, 0, 0, 0, time.Local)
	at := func(m int) *time.Time { v := t0.Add(time.Duration(m) * time.Minute); return &v }
	run := func(start int, end *time.Time, status v1.RunStatus) uint32 {
		t.Helper()
		r, err := s.StartRun(w.ID, "telegram")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.db.Model(&Run{ID: r.ID}).Updates(map[string]any{"started_at": *at(start), "finished_at": end, "status": status}).Error; err != nil {
			t.Fatal(err)
		}
		return r.ID
	}
	run(0, at(5), v1.RunStatus_RUN_STATUS_SUCCEEDED)     // ended before the next came
	run(6, at(20), v1.RunStatus_RUN_STATUS_INTERRUPTED)  // the tower stopped
	run(21, at(31), v1.RunStatus_RUN_STATUS_SUCCEEDED)   // the batch: 1
	run(22, at(40), v1.RunStatus_RUN_STATUS_FAILED)      // 2, failed but ended
	cur := run(30, nil, v1.RunStatus_RUN_STATUS_RUNNING) // this run
	run(35, nil, v1.RunStatus_RUN_STATUS_RUNNING)        // waiting
	done, waiting, err := s.RunBatch(w.ID, cur)
	if err != nil || done != 2 || waiting != 1 {
		t.Errorf("done %d, waiting %d, %v", done, waiting, err)
	}

	// A run alone: nothing before it in its batch, nothing waiting.
	alone := run(60, nil, v1.RunStatus_RUN_STATUS_RUNNING)
	if err := s.db.Model(&Run{}).Where("id < ? AND status = ?", alone, v1.RunStatus_RUN_STATUS_RUNNING).Updates(map[string]any{"status": v1.RunStatus_RUN_STATUS_SUCCEEDED, "finished_at": *at(50)}).Error; err != nil {
		t.Fatal(err)
	}
	if done, waiting, err := s.RunBatch(w.ID, alone); err != nil || done != 0 || waiting != 0 {
		t.Errorf("alone: done %d, waiting %d, %v", done, waiting, err)
	}
	if _, _, err := s.RunBatch(w.ID, 9999); err == nil {
		t.Error("no run: no error")
	}
}
