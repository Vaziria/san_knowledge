package engine

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// A running run keeps its last lines, counting those before them, until it
// ends.
func TestLiveLogs(t *testing.T) {
	e := &Engine{}
	if _, _, _, ok := e.LiveLogs(1); ok {
		t.Fatal("lines of a run that logged none")
	}
	for i := range store.SummaryLines + 5 {
		e.addLive(1, fmt.Sprint("line ", i), v1.LogLevel_LOG_LEVEL_DEBUG)
	}
	e.addLive(1, strings.Repeat("x", 1000), v1.LogLevel_LOG_LEVEL_INFO)
	lines, levels, more, ok := e.LiveLogs(1)
	if !ok || len(lines) != store.SummaryLines || len(levels) != len(lines) || more != 6 || lines[0] != "line 6" {
		t.Errorf("%d lines from %q, %d more, %v", len(lines), lines[0], more, ok)
	}
	if last := lines[len(lines)-1]; len([]rune(last)) != store.SummaryLineLen+1 || levels[len(levels)-1] != v1.LogLevel_LOG_LEVEL_INFO {
		t.Errorf("last line %d characters", len([]rune(last)))
	}
	e.dropLive(1)
	if _, _, _, ok := e.LiveLogs(1); ok {
		t.Error("lines of a run that ended")
	}
}

// A step that logs without end keeps its last lines, and its record counts
// the ones that went.
func TestCallLogBounded(t *testing.T) {
	var live int
	c := &call{step: step("s", "log"), env: Env{Live: func(*v1.Step, string, v1.LogLevel) { live++ }}}
	c.logs = []string{"appended without a type"}
	total := maxCallLines + 10
	for i := 1; i < total; i++ {
		c.log(v1.LogLevel_LOG_LEVEL_DEBUG, fmt.Sprint("line ", i))
	}
	if live != total-1 || len(c.logs) > maxCallLines || len(c.logs) != len(c.levels) || c.logs[len(c.logs)-1] != fmt.Sprint("line ", total-1) {
		t.Fatalf("%d live, %d lines, %d levels, last %q", live, len(c.logs), len(c.levels), c.logs[len(c.logs)-1])
	}
	x := &execution{}
	r := x.record(c.step, c, time.Now(), nil)
	if want := fmt.Sprintf("(%d earlier lines left out)", c.dropped); r.Logs[0] != want || len(r.Logs) != len(r.LogLevels) {
		t.Errorf("first line %q, want %q", r.Logs[0], want)
	}
}

// A script's output is passed on line by line, a \r ending a line too, and
// a line without a line break at its end.
func TestLineWriter(t *testing.T) {
	var got []string
	w := &lineWriter{line: func(s string) { got = append(got, s) }}
	for _, p := range []string{"one\ntw", "o\r\n", "10%\r20%\r", "\n\nlast"} {
		w.Write([]byte(p))
	}
	w.flush()
	if want := []string{"one", "two", "10%", "20%", "last"}; !slices.Equal(got, want) {
		t.Errorf("%q, want %q", got, want)
	}
	got = nil
	w.Write([]byte(strings.Repeat("y", lineMax+1)))
	w.flush()
	if len(got) != 1 || len(got[0]) != lineMax+1 {
		t.Errorf("a long line came as %d pieces", len(got))
	}
}
