package backtestanalysis

import (
	"strconv"
	"strings"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Paper sessions: the Live Trading page's paper sessions open here as runs
// (the "Paper sessions" source next to Open folder), so a session shows
// with the same chart, tables and decision log as a backtest and compares
// with them. A session's path is "paper:<id>"; the tower writes the
// session from its database as a San Backtest report folder
// (livetrading's View) and this package reads that folder as any other.

// PaperPrefix starts a paper session's path.
const PaperPrefix = "paper:"

// PaperPath is session id's path.
func PaperPath(id uint32) string { return PaperPrefix + strconv.FormatUint(uint64(id), 10) }

// PaperID reads a "paper:<id>" path.
func PaperID(path string) (uint32, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(path), PaperPrefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(rest, 10, 32)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint32(n), true
}

// PaperSummary makes a report's summary a paper session's: its path, its
// name as the relative path, the paper mark and the session's status.
func PaperSummary(sum *v1.BacktestRunSummary, id uint32, name, status string) *v1.BacktestRunSummary {
	sum.Path, sum.RelPath, sum.Paper, sum.Status = PaperPath(id), name, true, status
	return sum
}
