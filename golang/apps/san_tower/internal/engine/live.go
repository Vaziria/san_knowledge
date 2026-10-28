package engine

import (
	"fmt"
	"slices"
	"strings"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/store"
)

// Live logs (the user, 2026-10-27: "When remote shell node execute command,
// can it log streamed to frontend log"). A run's steps go into its record
// only when the run ends, so a long one showed nothing until then. A step
// that logs with call.log, as Remote Shell does a script's output line by
// line, passes each line on as it comes (Env.Live): the engine keeps a
// running run's last lines here, and the Logs pages, which ask every 2 s,
// show them as the run's lines until its record has them.

// liveLog is the last lines a running run logged, and how many came
// before them.
type liveLog struct {
	lines  []string
	levels []v1.LogLevel
	more   uint32
}

// addLive keeps line as run's last, of store.SummaryLines, cut as a
// summary's lines are.
func (e *Engine) addLive(run uint32, line string, level v1.LogLevel) {
	e.liveMu.Lock()
	defer e.liveMu.Unlock()
	if e.live == nil {
		e.live = make(map[uint32]*liveLog)
	}
	l := e.live[run]
	if l == nil {
		l = &liveLog{}
		e.live[run] = l
	}
	l.lines = append(l.lines, clip(line, store.SummaryLineLen))
	l.levels = append(l.levels, level)
	if over := len(l.lines) - store.SummaryLines; over > 0 {
		l.lines, l.levels = l.lines[over:], l.levels[over:]
		l.more += uint32(over)
	}
}

// dropLive forgets a run's live lines: it ended.
func (e *Engine) dropLive(run uint32) {
	e.liveMu.Lock()
	defer e.liveMu.Unlock()
	delete(e.live, run)
}

// LiveLogs is what a running run's steps logged as they went: its last
// lines, with their types, and how many came before them; ok is false when
// it logged none that way.
func (e *Engine) LiveLogs(run uint32) (lines []string, levels []v1.LogLevel, more uint32, ok bool) {
	e.liveMu.Lock()
	defer e.liveMu.Unlock()
	l := e.live[run]
	if l == nil {
		return nil, nil, 0, false
	}
	return slices.Clone(l.lines), slices.Clone(l.levels), l.more, true
}

// maxCallLines is the most lines a step keeps as it runs. The store keeps a
// step's last store.MaxStepLines; a step that logs without end, such as a
// script printing a render's progress, lets its earliest go past this, and
// its record counts them in a first line.
const maxCallLines = 2 * store.MaxStepLines

// log logs line, of type level, as the step's, and passes it on at once
// (Env.Live). Several goroutines may call it, such as a script's stdout and
// stderr.
func (c *call) log(level v1.LogLevel, line string) {
	c.logMu.Lock()
	defer c.logMu.Unlock()
	for len(c.levels) < len(c.logs) { // lines appended without a type
		c.levels = append(c.levels, v1.LogLevel_LOG_LEVEL_UNSPECIFIED)
	}
	line = clip(strings.ToValidUTF8(line, "�"), store.MaxLineLen) // UTF-8, and all the store keeps of a line
	c.logs = append(c.logs, line)
	c.levels = append(c.levels, level)
	if len(c.logs) > maxCallLines {
		n := len(c.logs) - store.MaxStepLines
		c.logs, c.levels = slices.Clone(c.logs[n:]), slices.Clone(c.levels[n:])
		c.dropped += n
	}
	if c.env.Live != nil {
		c.env.Live(c.step, line, level)
	}
}

// logf is log with a format.
func (c *call) logf(level v1.LogLevel, format string, args ...any) {
	c.log(level, fmt.Sprintf(format, args...))
}
