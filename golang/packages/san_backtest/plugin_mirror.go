package san_backtest

import "time"

// Mirror is the Context an approach plugin keeps on its side of the pipe
// (package plugin): the host sends only what changed each step (the new
// closed candles, the positions, orders and account now), and the plugin
// computes indicators, Forming and the rest with this package's own code,
// so the approach decides exactly as it would compiled in. Hosts never
// need it.
type Mirror struct {
	c *Context
}

// NewMirror starts an empty Context for one run: info is what OnStart got,
// anchors each timeframe's bucket anchor (the first loaded candle's open,
// for Forming), record whether notes are kept (false in walk-forward
// trials).
func NewMirror(info StartInfo, anchors map[string]time.Time, record bool) *Mirror {
	frames := map[string]*series{}
	an := map[string]time.Time{}
	for _, tf := range info.Timeframes {
		frames[tf] = newSeries(tf, nil)
		an[tf] = time.Date(1970, 1, 4, 0, 0, 0, 0, time.UTC)
		if t, ok := anchors[tf]; ok {
			an[tf] = t
		}
	}
	step := ""
	if len(info.Timeframes) > 0 {
		step = info.Timeframes[0]
	}
	params := info.Params
	if params == nil {
		params = map[string]float64{}
	}
	return &Mirror{c: &Context{step: step, tfs: append([]string{}, info.Timeframes...), frames: frames, anchors: an, params: params,
		record: record, inst: info.Instrument}}
}

// Restart is NewMirror for the next run of the same instance, keeping the
// candles of the timeframes in keep (the host checked they are the same
// candles). Indicators start over, as in a compiled run.
func (m *Mirror) Restart(info StartInfo, anchors map[string]time.Time, record bool, keep []string) *Mirror {
	n := NewMirror(info, anchors, record)
	for _, tf := range keep {
		if old, ok := m.c.frames[tf]; ok {
			if f, ok := n.c.frames[tf]; ok {
				f.all = old.all
				f.n = len(f.all)
			}
		}
	}
	return n
}

// Add appends tf's newly closed candles, oldest first. A timeframe the run
// does not have is ignored.
func (m *Mirror) Add(tf string, cs []Candle) {
	f, ok := m.c.frames[tf]
	if !ok {
		return
	}
	f.all = append(f.all, cs...)
	f.n = len(f.all)
}

// Closed sets how many of tf's candles the Context sees, when the mirror
// holds more (kept from an earlier, longer run).
func (m *Mirror) Closed(tf string, n int) {
	if f, ok := m.c.frames[tf]; ok && n >= 0 && n <= len(f.all) {
		f.n = n
	}
}

// MirrorStep is the state of one step as the host sends it.
type MirrorStep struct {
	Time      time.Time
	Gap       time.Duration
	Spread    float64
	Positions []Position
	Orders    []Order
	MaxOpen   int
	Account   Account
}

// Context is the Context for the step st, the same object every step of
// the run (a chain keeps its state on it), with this step's notes empty.
func (m *Mirror) Context(st MirrorStep) *Context {
	x := m.c
	x.time, x.gap = st.Time, st.Gap
	x.inst.Spread = st.Spread
	x.positions, x.cur, x.orders, x.maxOpen = st.Positions, nil, st.Orders, st.MaxOpen
	x.account = st.Account
	x.notes = nil
	return x
}

// Notes are the Note and Explain texts made since Context, and clears
// them.
func (m *Mirror) Notes() []string {
	n := m.c.notes
	m.c.notes = nil
	return n
}
