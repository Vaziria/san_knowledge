package backtestanalysis

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Decision log limits (GetDecisions' limit).
const (
	DefaultDecisionLimit = 200
	MaxDecisionLimit     = 5000
)

// DecisionQuery picks lines of a decision log.
type DecisionQuery struct {
	Around   time.Time // set: the Limit lines nearest it
	From, To time.Time // else the lines in [From, To] (zero: unbounded)
	Limit    int
	Kinds    []string // empty: all
	Leg      *uint32
}

type decisionLine struct {
	Time    string             `json:"time"`
	Leg     uint32             `json:"leg"`
	Kind    string             `json:"kind"`
	Side    string             `json:"side"`
	Details map[string]float64 `json:"details"`
	Reason  string             `json:"reason"`
}

// Decisions reads a report's decision log. A report without one comes back
// with Available false; a line that is not JSON is skipped.
func Decisions(dir string, q DecisionQuery) (*v1.BacktestGetDecisionsResponse, error) {
	res := &v1.BacktestGetDecisionsResponse{}
	f, err := os.Open(filepath.Join(dir, DecisionsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	res.Available = true
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultDecisionLimit
	}
	limit = min(limit, MaxDecisionLimit)
	kinds := map[string]bool{}
	for _, k := range q.Kinds {
		kinds[k] = true
	}
	var all []*v1.BacktestDecision
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	n := 0
	for sc.Scan() {
		n++
		var l decisionLine
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, l.Time)
		if err != nil {
			continue
		}
		if len(kinds) > 0 && !kinds[l.Kind] || q.Leg != nil && l.Leg != *q.Leg {
			continue
		}
		if q.Around.IsZero() && (!q.From.IsZero() && t.Before(q.From) || !q.To.IsZero() && t.After(q.To)) {
			continue
		}
		all = append(all, &v1.BacktestDecision{Time: tp(t), Leg: l.Leg, Kind: l.Kind, Side: l.Side, Details: l.Details, Reason: l.Reason, Line: uint32(n)})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	res.Total = uint32(len(all))
	if !q.Around.IsZero() && len(all) > limit {
		// the first line at or after Around, then half the limit each side
		at := sort.Search(len(all), func(i int) bool { return !all[i].Time.AsTime().Before(q.Around) })
		lo := max(0, at-limit/2)
		hi := min(len(all), lo+limit)
		lo = max(0, hi-limit)
		all = all[lo:hi]
	} else if len(all) > limit {
		all = all[:limit]
	}
	res.Decisions = all
	return res, nil
}
