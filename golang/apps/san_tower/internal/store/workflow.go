package store

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Workflow is a workflow as stored. Its steps and links are one column, the
// proto Graph as protojson, since they are only ever read and written whole.
type Workflow struct {
	ID        uint32    `gorm:"primaryKey"`
	Name      string    `gorm:"not null;uniqueIndex"`
	Graph     *v1.Graph `gorm:"serializer:protojson;not null"`
	Active    bool      `gorm:"not null;default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Summary is a workflow in the list.
type Summary struct {
	ID        uint32
	Name      string
	StepCount int
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MaxNameLen is the longest workflow name, in characters.
const MaxNameLen = 100

// ListWorkflows lists the workflows whose name contains query, ignoring case
// (all when it is empty), by name, ignoring case: limit of them (all when 0)
// from offset, and how many match.
func (s *Store) ListWorkflows(query string, offset, limit int) ([]Summary, int, error) {
	match := s.db.Model(&Workflow{})
	if query = strings.TrimSpace(query); query != "" {
		match = match.Where("instr(lower(name), lower(?)) > 0", query)
	}
	var total int64
	if err := match.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = -1 // no limit
	}
	var list []Summary
	err := match.
		Select("id, name, coalesce(json_array_length(graph, '$.steps'), 0) AS step_count, active, created_at, updated_at").
		Order("name COLLATE NOCASE, id").
		Offset(offset).Limit(limit).
		Scan(&list).Error
	return list, int(total), err
}

// CreateWorkflow creates an empty, inactive workflow.
func (s *Store) CreateWorkflow(name string) (Workflow, error) {
	w := Workflow{Name: strings.TrimSpace(name), Graph: &v1.Graph{}}
	if err := checkName(w.Name); err != nil {
		return w, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := nameFree(tx, w.Name, 0); err != nil {
			return err
		}
		return tx.Create(&w).Error
	})
	return w, err
}

func (s *Store) GetWorkflow(id uint32) (Workflow, error) {
	var w Workflow
	err := s.db.First(&w, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return w, fail(ErrNotFound, "no workflow %d", id)
	}
	return w, err
}

// UpdateWorkflow replaces a workflow's name and graph, and returns it as
// stored. It leaves Active as it is.
func (s *Store) UpdateWorkflow(w Workflow) (Workflow, error) {
	w.Name = strings.TrimSpace(w.Name)
	if err := checkName(w.Name); err != nil {
		return w, err
	}
	if w.Graph == nil {
		w.Graph = &v1.Graph{}
	}
	if err := checkGraph(w.Graph); err != nil {
		return w, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&Workflow{}).Where("id = ?", w.ID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return fail(ErrNotFound, "no workflow %d", w.ID)
		}
		if err := nameFree(tx, w.Name, w.ID); err != nil {
			return err
		}
		return tx.Model(&Workflow{ID: w.ID}).Select("Name", "Graph").Updates(&Workflow{Name: w.Name, Graph: w.Graph}).Error
	})
	if err != nil {
		return w, err
	}
	return s.GetWorkflow(w.ID)
}

// DeleteWorkflow deletes a workflow and its runs.
func (s *Store) DeleteWorkflow(id uint32) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		r := tx.Delete(&Workflow{}, id)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return fail(ErrNotFound, "no workflow %d", id)
		}
		return tx.Where("workflow_id = ?", id).Delete(&Run{}).Error
	})
}

// SetWorkflowActive switches a workflow's triggers on or off, and returns it
// as stored.
func (s *Store) SetWorkflowActive(id uint32, active bool) (Workflow, error) {
	r := s.db.Model(&Workflow{ID: id}).Update("active", active)
	if r.Error != nil {
		return Workflow{}, r.Error
	}
	if r.RowsAffected == 0 {
		return Workflow{}, fail(ErrNotFound, "no workflow %d", id)
	}
	return s.GetWorkflow(id)
}

// Workflows lists every workflow with its graph, by id.
func (s *Store) Workflows() ([]Workflow, error) {
	var list []Workflow
	return list, s.db.Order("id").Find(&list).Error
}

func checkName(name string) error {
	if name == "" {
		return fail(ErrInvalid, "a workflow needs a name")
	}
	if utf8.RuneCountInString(name) > MaxNameLen {
		return fail(ErrInvalid, "a workflow name has at most %d characters", MaxNameLen)
	}
	return nil
}

// nameFree says whether no workflow but the one with id is called name,
// ignoring case.
func nameFree(tx *gorm.DB, name string, id uint32) error {
	var n int64
	if err := tx.Model(&Workflow{}).Where("name = ? COLLATE NOCASE AND id <> ?", name, id).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fail(ErrExists, "a workflow named %q already exists", name)
	}
	return nil
}

// checkGraph checks that steps have a kind and an id of their own, and that
// links join steps that are there.
func checkGraph(g *v1.Graph) error {
	ids := make(map[string]bool, len(g.GetSteps()))
	for _, st := range g.GetSteps() {
		switch {
		case st.GetId() == "":
			return fail(ErrInvalid, "a step has no id")
		case st.GetKind() == "":
			return fail(ErrInvalid, "step %s has no kind", st.GetId())
		case ids[st.GetId()]:
			return fail(ErrInvalid, "two steps have the id %s", st.GetId())
		}
		ids[st.GetId()] = true
	}
	for _, l := range g.GetLinks() {
		if !ids[l.GetSource()] || !ids[l.GetTarget()] {
			return fail(ErrInvalid, "a link joins a step that is not in the workflow (%s to %s)", l.GetSource(), l.GetTarget())
		}
	}
	return nil
}
