package store

import (
	"fmt"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// What a run keeps of what its steps did (the user, 2026-10-27: "check
// heavy frontend load, like log, what if log contain many log"). A step can
// log without end (a loop, a long Claude session) and output a whole file or
// page; the run itself had it all, but what is stored, and so what the UI
// gets, is cut to these sizes, with a note saying what was left out.
const (
	// MaxLineLen is the most characters a log line keeps.
	MaxLineLen = 2000
	// MaxStepLines is the most lines a step keeps: its last ones.
	MaxStepLines = 500
	// MaxOutputBytes is the most an output keeps: a text its start, any
	// other value a note.
	MaxOutputBytes = 64 << 10
	// MaxRunBytes is the most a run's steps keep in all: past it, the steps
	// after keep only their errors and what they fired.
	MaxRunBytes = 2 << 20
	// SummaryLines and SummaryLineLen are what a run's summary (the Logs
	// pages, asked for every 2 s) keeps: its last lines, cut shorter.
	SummaryLines   = 20
	SummaryLineLen = 300
)

// boundSteps cuts what steps did to what a run keeps, in place.
func boundSteps(steps []*v1.StepRun) {
	total := 0
	for _, st := range steps {
		if total > MaxRunBytes {
			st.Logs = []string{fmt.Sprintf("(%d lines left out: the run kept %d MB)", len(st.Logs), MaxRunBytes>>20)}
			st.LogLevels = nil
			if len(st.Outputs) > 0 {
				st.Outputs = map[string]*anypb.Any{"(left out)": note(fmt.Sprintf("%d outputs left out: the run kept %d MB", len(st.Outputs), MaxRunBytes>>20))}
			}
			continue
		}
		boundLines(st)
		for name, a := range st.Outputs {
			if n := proto.Size(a); n > MaxOutputBytes {
				st.Outputs[name] = boundOutput(a, n)
			}
		}
		total += proto.Size(st)
	}
}

// boundLines keeps a step's last MaxStepLines lines, each cut to
// MaxLineLen, after a line saying how many came before.
func boundLines(st *v1.StepRun) {
	if n := len(st.Logs); n > MaxStepLines {
		cut := n - MaxStepLines + 1
		levels := st.LogLevels
		st.Logs = append([]string{fmt.Sprintf("(%d earlier lines left out)", cut)}, st.Logs[cut:]...)
		if len(levels) > cut {
			st.LogLevels = append([]v1.LogLevel{v1.LogLevel_LOG_LEVEL_UNSPECIFIED}, levels[cut:]...)
		} else {
			st.LogLevels = nil
		}
	}
	for i, l := range st.Logs {
		st.Logs[i] = clip(l, MaxLineLen)
	}
}

// boundOutput keeps the start of a text output of n bytes, or a note for
// any other value.
func boundOutput(a *anypb.Any, n int) *anypb.Any {
	var s wrapperspb.StringValue
	if a.MessageIs(&s) && a.UnmarshalTo(&s) == nil {
		return text(clip(s.GetValue(), MaxOutputBytes/2) + fmt.Sprintf("\n(%d KB in all; the run kept the start)", n>>10))
	}
	var v structpb.Value
	if a.MessageIs(&v) {
		return note(fmt.Sprintf("an object of %d KB, more than a run keeps (%d KB)", n>>10, MaxOutputBytes>>10))
	}
	return note(fmt.Sprintf("%d KB, more than a run keeps", n>>10))
}

func text(s string) *anypb.Any {
	a, _ := anypb.New(wrapperspb.String(s))
	return a
}

func note(s string) *anypb.Any { return text("(" + s + ")") }

// clip cuts s to max characters, marking the cut.
func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

// summarize is a run's summary: the last SummaryLines lines its steps
// logged, in order, each cut to SummaryLineLen, with their types.
func summarize(steps []*v1.StepRun) *v1.RunSummary {
	sum := &v1.RunSummary{}
	total := 0
	for _, st := range steps {
		total += len(st.GetLogs())
	}
	skip := max(total-SummaryLines, 0)
	sum.More = uint32(skip)
	for _, st := range steps {
		for i, l := range st.GetLogs() {
			if skip > 0 {
				skip--
				continue
			}
			level := v1.LogLevel_LOG_LEVEL_UNSPECIFIED
			if i < len(st.GetLogLevels()) {
				level = st.GetLogLevels()[i]
			}
			sum.Logs = append(sum.Logs, clip(l, SummaryLineLen))
			sum.LogLevels = append(sum.LogLevels, level)
		}
	}
	return sum
}
