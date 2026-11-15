package san_research

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Options for one research run.
type Options struct {
	Cache      string // san_backtest's cache folder
	Instrument string // e.g. XAUUSD
	Out        string // report folder
	From, To   time.Time
	Log        io.Writer
	// Backtests, when set, also runs the approaches (see backtest.go) and
	// puts their results into the report.
	Backtests bool
}

// Research loads the data, runs every study and writes report.md.
func Research(ctx context.Context, o Options) (*Report, error) {
	logf := func(format string, a ...any) {
		if o.Log != nil {
			fmt.Fprintf(o.Log, format+"\n", a...)
		}
	}
	d, err := Load(ctx, o.Cache, o.Instrument, o.From, o.To)
	if err != nil {
		return nil, err
	}
	logf("loaded %s: M15 %d bars, H1 %d bars, %d trading days, %s to %s", d.Name, len(d.M15), len(d.H1), len(d.D1),
		d.M15[0].Time.Format(time.RFC3339), d.M15[len(d.M15)-1].Time.Format(time.RFC3339))
	r := NewReport(o.Out)
	steps := []struct {
		name string
		run  func(*Data)
	}{{"study 1", r.Study1}, {"study 2", r.Study2}, {"study 3", r.Study3}, {"study 4", r.Study4}, {"study 5", r.Study5}}
	for _, s := range steps {
		start := time.Now()
		s.run(d)
		logf("%s done in %s", s.name, time.Since(start).Round(time.Millisecond))
	}
	var tail strings.Builder
	if o.Backtests {
		if err := r.Backtests(ctx, d, o, &tail, logf); err != nil {
			return r, err
		}
	} else {
		tail.WriteString(r.savedBacktests())
	}
	tail.WriteString(r.limits(d))
	path, err := r.Write(r.header(d), tail.String())
	if err != nil {
		return r, err
	}
	logf("wrote %s (%d charts); findings %v", path, len(r.charts), r.Counts())
	return r, nil
}
