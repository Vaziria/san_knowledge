package store

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func textOf(t *testing.T, a *anypb.Any) string {
	t.Helper()
	var s wrapperspb.StringValue
	if err := a.UnmarshalTo(&s); err != nil {
		t.Fatal(err)
	}
	return s.GetValue()
}

// A step that logs without end keeps its last lines, each cut short, and
// big outputs keep a text's start or a note; past MaxRunBytes the steps
// after keep only a note.
func TestBoundSteps(t *testing.T) {
	var lines []string
	var levels []v1.LogLevel
	for i := range 2000 {
		lines = append(lines, fmt.Sprintf("line %d", i))
		levels = append(levels, v1.LogLevel_LOG_LEVEL_WARNING)
	}
	lines[1999] = strings.Repeat("x", 5000)
	page, _ := anypb.New(wrapperspb.String(strings.Repeat("p", 200<<10)))
	big, _ := structpb.NewValue(map[string]any{"body": strings.Repeat("b", 100<<10)})
	obj, _ := anypb.New(big)
	small, _ := anypb.New(wrapperspb.String("ok"))
	st := &v1.StepRun{Kind: "log", Logs: lines, LogLevels: levels, Outputs: map[string]*anypb.Any{"Page": page, "Object": obj, "Small": small}}
	boundSteps([]*v1.StepRun{st})

	if len(st.Logs) != MaxStepLines || st.Logs[0] != "(1501 earlier lines left out)" || st.Logs[1] != "line 1501" {
		t.Errorf("lines: %d, %q, %q", len(st.Logs), st.Logs[0], st.Logs[1])
	}
	if len(st.LogLevels) != MaxStepLines || st.LogLevels[0] != v1.LogLevel_LOG_LEVEL_UNSPECIFIED || st.LogLevels[1] != v1.LogLevel_LOG_LEVEL_WARNING {
		t.Errorf("levels: %d, %v", len(st.LogLevels), st.LogLevels[:2])
	}
	if last := st.Logs[len(st.Logs)-1]; len([]rune(last)) != MaxLineLen+1 || !strings.HasSuffix(last, "…") {
		t.Errorf("a long line keeps %d characters", len([]rune(last)))
	}
	if p := textOf(t, st.Outputs["Page"]); !strings.HasPrefix(p, "ppp") || !strings.Contains(p, "(200 KB in all") || len(p) > MaxOutputBytes {
		t.Errorf("page output: %d bytes, ends %q", len(p), p[len(p)-40:])
	}
	if o := textOf(t, st.Outputs["Object"]); !strings.Contains(o, "an object of 100 KB") {
		t.Errorf("object output: %q", o)
	}
	if textOf(t, st.Outputs["Small"]) != "ok" {
		t.Error("a small output changed")
	}

	// Many big steps: the run stops keeping their lines and outputs.
	var steps []*v1.StepRun
	for range 60 {
		out, _ := anypb.New(wrapperspb.String(strings.Repeat("o", 60<<10)))
		steps = append(steps, &v1.StepRun{Kind: "read-file", Logs: []string{"read"}, Outputs: map[string]*anypb.Any{"Text": out}, Fired: []string{"Done"}})
	}
	boundSteps(steps)
	size := 0
	for _, s := range steps {
		size += proto.Size(s)
	}
	if size > MaxRunBytes+(MaxOutputBytes*2) {
		t.Errorf("the run keeps %d KB", size>>10)
	}
	last := steps[len(steps)-1]
	if !strings.Contains(last.Logs[0], "left out: the run kept 2 MB") || len(last.Fired) != 1 || len(last.Outputs) != 1 {
		t.Errorf("last step: %v", last)
	}
}

// A run's summary is its last lines, cut shorter, with their types, and
// how many came before.
func TestSummarize(t *testing.T) {
	steps := []*v1.StepRun{
		{Logs: []string{"a", "b"}, LogLevels: []v1.LogLevel{v1.LogLevel_LOG_LEVEL_INFO, v1.LogLevel_LOG_LEVEL_FAILED}},
		{Logs: []string{"old line, no type"}},
	}
	sum := summarize(steps)
	if sum.GetMore() != 0 || strings.Join(sum.GetLogs(), "|") != "a|b|old line, no type" ||
		sum.GetLogLevels()[1] != v1.LogLevel_LOG_LEVEL_FAILED || sum.GetLogLevels()[2] != v1.LogLevel_LOG_LEVEL_UNSPECIFIED {
		t.Errorf("summary = %v", sum)
	}
	var many []string
	for i := range 50 {
		many = append(many, fmt.Sprintf("%d %s", i, strings.Repeat("z", 400)))
	}
	sum = summarize([]*v1.StepRun{{Logs: many}})
	if sum.GetMore() != 30 || len(sum.GetLogs()) != SummaryLines || !strings.HasPrefix(sum.GetLogs()[0], "30 ") || len([]rune(sum.GetLogs()[0])) != SummaryLineLen+1 {
		t.Errorf("summary of 50 lines: more %d, %d lines, first %q", sum.GetMore(), len(sum.GetLogs()), sum.GetLogs()[0][:10])
	}
}

// A run stored before runs had a summary gets one from its steps the first
// time a list reads it, and keeps it.
func TestSummarizeOld(t *testing.T) {
	s := open(t)
	w, err := s.CreateWorkflow("old")
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartRun(w.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	steps := &v1.RunSteps{Steps: []*v1.StepRun{{Logs: []string{"from before"}}}}
	if err := s.db.Model(&Run{ID: r.ID}).Updates(map[string]any{"status": v1.RunStatus_RUN_STATUS_SUCCEEDED}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Model(&Run{ID: r.ID}).Select("Steps").Updates(&Run{Steps: steps}).Error; err != nil {
		t.Fatal(err)
	}
	// The column AutoMigrate adds is NULL in the rows already there.
	if err := s.db.Exec("UPDATE runs SET summary = NULL WHERE id = ?", r.ID).Error; err != nil {
		t.Fatal(err)
	}
	list, err := s.ListAllRuns(w.ID, false, nil, 10)
	if err != nil || len(list) != 1 || strings.Join(list[0].Summary.GetLogs(), "|") != "from before" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	var stored Run
	if err := s.db.Select("id", "summary").First(&stored, r.ID).Error; err != nil || stored.Summary == nil {
		t.Errorf("the summary was not stored: %+v, %v", stored, err)
	}
}

// A run whose steps logged bytes that are not UTF-8 (a Windows program's
// output in its console code page) is recorded, those bytes U+FFFD; before,
// protojson refused it and the run stayed running (2026-10-28).
func TestFinishRunNotUTF8(t *testing.T) {
	s := open(t)
	w, err := s.CreateWorkflow("cp1252")
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.StartRun(w.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	steps := []*v1.StepRun{{StepId: "s", Logs: []string{"caf\xe9 ok"}, Error: "bad \xff"}}
	if _, err := s.FinishRun(r, v1.RunStatus_RUN_STATUS_FAILED, "failed: \xfe", steps); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRun(r.ID)
	if err != nil || got.Status != v1.RunStatus_RUN_STATUS_FAILED || got.Steps.GetSteps()[0].Logs[0] != "caf� ok" || got.Error != "failed: �" {
		t.Errorf("%+v, %v", got, err)
	}
}
