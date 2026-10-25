package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/store"
)

// WorkflowService's export and import (the user's ask, 2026-10-26): a file
// of workflows that this tower, or another, reads back.

const (
	workflowFormat  = "san_tower.workflows"
	workflowVersion = 1
)

// ref is a setting of a kind of step that picks a connection of a type, or
// a storage.
type ref struct {
	setting, connType string
	storage           bool
}

// stepRefs are each kind of step's settings that pick a connection or a
// storage, as the kinds declare them.
func stepRefs() map[string][]ref {
	m := make(map[string][]ref)
	for _, k := range engine.Kinds() {
		for _, st := range k.GetSettings() {
			switch {
			case st.GetConnectionType() != "":
				m[k.GetKind()] = append(m[k.GetKind()], ref{setting: st.GetName(), connType: st.GetConnectionType()})
			case st.GetStorage():
				m[k.GetKind()] = append(m[k.GetKind()], ref{setting: st.GetName(), storage: true})
			}
		}
	}
	return m
}

func (s *Server) ExportWorkflows(_ context.Context, req *connect.Request[v1.ExportWorkflowsRequest]) (*connect.Response[v1.ExportWorkflowsResponse], error) {
	var list []store.Workflow
	if ids := req.Msg.GetIds(); len(ids) > 0 {
		for _, id := range ids {
			w, err := s.store.GetWorkflow(id)
			if err != nil {
				return nil, storeError(err)
			}
			list = append(list, w)
		}
	} else {
		all, err := s.store.Workflows()
		if err != nil {
			return nil, storeError(err)
		}
		list = all
	}
	if len(list) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("there is no workflow to export"))
	}
	f := &v1.WorkflowFile{
		Format: workflowFormat, Version: workflowVersion, ExportedAt: timestamppb.Now(), Tower: s.version,
		Connections: make(map[string]*v1.ExportedRef), Storages: make(map[string]*v1.ExportedRef),
	}
	refs := stepRefs()
	for _, w := range list {
		f.Workflows = append(f.Workflows, &v1.ExportedWorkflow{Name: w.Name, Steps: w.Graph.GetSteps(), Links: w.Graph.GetLinks()})
		for _, st := range w.Graph.GetSteps() {
			for _, r := range refs[st.GetKind()] {
				id := strings.TrimSpace(st.GetValues()[r.setting])
				n, err := strconv.ParseUint(id, 10, 32)
				if err != nil {
					continue
				}
				// Only the name and the type: a connection's settings hold
				// secrets, such as a bot's token.
				switch {
				case r.storage && f.Storages[id] == nil:
					if sto, err := s.store.GetStorage(uint32(n)); err == nil {
						f.Storages[id] = &v1.ExportedRef{Name: sto.Name}
					}
				case !r.storage && f.Connections[id] == nil:
					if c, err := s.store.GetConnection(uint32(n)); err == nil {
						f.Connections[id] = &v1.ExportedRef{Name: c.Name, Type: c.Type}
					}
				}
			}
		}
	}
	// protojson varies its spacing on purpose; the file gets steady
	// indentation, as a person reads it or a diff compares it.
	raw, err := protojson.Marshal(f)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	b.WriteByte('\n')
	name := "san_tower-workflows-" + time.Now().Format("2006-01-02") + ".json"
	if len(list) == 1 {
		name = fileName(list[0].Name) + ".workflow.json"
	}
	return connect.NewResponse(&v1.ExportWorkflowsResponse{Json: b.String(), FileName: name}), nil
}

func (s *Server) ImportWorkflows(_ context.Context, req *connect.Request[v1.ImportWorkflowsRequest]) (*connect.Response[v1.ImportWorkflowsResponse], error) {
	var f v1.WorkflowFile
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(req.Msg.GetJson()), &f); err != nil {
		return nil, invalidArg("not a San Tower workflow file: %v", err)
	}
	switch {
	case f.GetFormat() != workflowFormat:
		return nil, invalidArg("not a San Tower workflow file: it says it is %q, not %q", f.GetFormat(), workflowFormat)
	case f.GetVersion() > workflowVersion:
		return nil, invalidArg("the file is version %d, newer than this tower reads (%d): update the tower", f.GetVersion(), workflowVersion)
	case len(f.GetWorkflows()) == 0:
		return nil, invalidArg("the file has no workflow")
	}

	var notes []string
	conns, err := s.matchConnections(f.GetConnections(), &notes)
	if err != nil {
		return nil, storeError(err)
	}
	stores, err := s.matchStorages(f.GetStorages(), &notes)
	if err != nil {
		return nil, storeError(err)
	}
	refs, known := stepRefs(), make(map[string]bool)
	for _, k := range engine.Kinds() {
		known[k.GetKind()] = true
	}

	// All or nothing: a workflow that cannot be saved takes back those made
	// before it.
	var made []store.Workflow
	undo := func() {
		for _, w := range made {
			_ = s.store.DeleteWorkflow(w.ID)
		}
	}
	for _, ew := range f.GetWorkflows() {
		for _, st := range ew.GetSteps() {
			if !known[st.GetKind()] {
				notes = append(notes, fmt.Sprintf("%s: step %s is a %q, which this tower does not have", ew.GetName(), st.GetId(), st.GetKind()))
			}
			for _, r := range refs[st.GetKind()] {
				old := strings.TrimSpace(st.GetValues()[r.setting])
				if old == "" {
					continue
				}
				m := conns
				if r.storage {
					m = stores
				}
				if id := m[old]; id != "" {
					st.Values[r.setting] = id
					continue
				}
				delete(st.Values, r.setting)
				if _, listed := m[old]; !listed {
					notes = append(notes, fmt.Sprintf("%s: step %s's %s was not in the file: pick one", ew.GetName(), st.GetId(), r.setting))
				}
			}
		}
		w, err := s.createNamed(ew.GetName())
		if err != nil {
			undo()
			return nil, storeError(err)
		}
		made = append(made, w)
		if w.Name != strings.TrimSpace(ew.GetName()) {
			notes = append(notes, fmt.Sprintf("%s is called %s here: the name was taken", ew.GetName(), w.Name))
		}
		w.Graph = &v1.Graph{Steps: ew.GetSteps(), Links: ew.GetLinks()}
		if made[len(made)-1], err = s.store.UpdateWorkflow(w); err != nil {
			undo()
			return nil, storeError(fmt.Errorf("%s: %w", ew.GetName(), err))
		}
	}
	res := &v1.ImportWorkflowsResponse{Notes: notes}
	for _, w := range made {
		res.Workflows = append(res.Workflows, &v1.WorkflowSummary{
			Id: w.ID, Name: w.Name, StepCount: uint32(len(w.Graph.GetSteps())), Active: w.Active,
			CreatedAt: timestamppb.New(w.CreatedAt), UpdatedAt: timestamppb.New(w.UpdatedAt),
		})
	}
	return connect.NewResponse(res), nil
}

// matchConnections finds this tower's connection for each one of a file,
// by its id there: the one of the same name (case aside) and type, else the
// only one of that type, else none ("") for the user to pick.
func (s *Server) matchConnections(refs map[string]*v1.ExportedRef, notes *[]string) (map[string]string, error) {
	m := make(map[string]string, len(refs))
	for old, r := range refs {
		list, err := s.store.ListConnections(r.GetType())
		if err != nil {
			return nil, err
		}
		m[old] = ""
		for _, c := range list {
			if strings.EqualFold(c.Name, r.GetName()) {
				m[old] = strconv.FormatUint(uint64(c.ID), 10)
			}
		}
		switch {
		case m[old] != "":
		case len(list) == 1:
			m[old] = strconv.FormatUint(uint64(list[0].ID), 10)
			*notes = append(*notes, fmt.Sprintf("connection %s (%s) is not here: its steps use %s, the only %s connection", r.GetName(), r.GetType(), list[0].Name, r.GetType()))
		default:
			*notes = append(*notes, fmt.Sprintf("connection %s (%s) is not here: pick one in its steps (Connections page)", r.GetName(), r.GetType()))
		}
	}
	return m, nil
}

// matchStorages finds this tower's storage of the same name (case aside)
// for each one of a file; with none, the user picks one, since a storage is
// a folder where files go.
func (s *Server) matchStorages(refs map[string]*v1.ExportedRef, notes *[]string) (map[string]string, error) {
	m := make(map[string]string, len(refs))
	if len(refs) == 0 {
		return m, nil
	}
	list, err := s.store.ListStorages()
	if err != nil {
		return nil, err
	}
	for old, r := range refs {
		m[old] = ""
		for _, st := range list {
			if strings.EqualFold(st.Name, r.GetName()) {
				m[old] = strconv.FormatUint(uint64(st.ID), 10)
			}
		}
		if m[old] == "" {
			*notes = append(*notes, fmt.Sprintf("storage %s is not here: make it on the Storage page, or pick one in its steps", r.GetName()))
		}
	}
	return m, nil
}

// createNamed creates an empty workflow called name, or, while that is
// taken, "name (2)", "name (3)"...
func (s *Server) createNamed(name string) (store.Workflow, error) {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "Imported workflow"
	}
	for n := 1; ; n++ {
		try := base
		if n > 1 {
			suffix := fmt.Sprintf(" (%d)", n)
			if r := []rune(base); len(r) > store.MaxNameLen-len(suffix) {
				base = strings.TrimSpace(string(r[:store.MaxNameLen-len(suffix)]))
			}
			try = base + suffix
		}
		w, err := s.store.CreateWorkflow(try)
		if !errors.Is(err, store.ErrExists) || n >= 1000 {
			return w, err
		}
	}
}

// fileName makes a workflow's name safe as a file name on Windows.
func fileName(name string) string {
	f := strings.Trim(strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, name), " .")
	if f == "" {
		return "workflow"
	}
	return f
}

func invalidArg(format string, a ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, a...))
}
