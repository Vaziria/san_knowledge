package san_backtest

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/wargasipil/san_backtest/plugin/pb"
)

// remote is an approach served by a plugin, as an ordinary Strategy: Run,
// NewLive, walk-forward and the reports use it unchanged. It sends the
// plugin only what changed since its last step and adds the plugin's
// notes to the step's own. A plugin failure (exit, panic, timeout) fails
// the run like a strategy panic.
type remote struct {
	proc *pluginProc
	info PluginApproach
	id   uint64
	// owns: Close also stops the process (LoadApproach).
	owns bool
	// sent is how many candles of each timeframe the plugin has, and base
	// the first of the host's candles they were copied from: a run on the
	// same candles (walk-forward trials, which share the loaded data)
	// keeps them instead of sending the whole history again.
	sent map[string]int
	base map[string]*Candle
	// err is the first failure; every later step fails with it.
	err error
	// pendingStart is OnStart's request, sent with the first step.
	pendingStart *pb.StartRequest
}

// remoteTunable is a remote whose approach has params (Tunable).
type remoteTunable struct{ *remote }

func (s remoteTunable) Params() map[string]float64 {
	out := map[string]float64{}
	for k, v := range s.info.Params {
		out[k] = v
	}
	return out
}

func newRemote(p *pluginProc, info PluginApproach, owns bool) (Strategy, error) {
	s := &remote{proc: p, info: info, owns: owns}
	s.id, s.err = p.newInstance(info.Name)
	if info.Params != nil {
		return remoteTunable{s}, s.err
	}
	return s, s.err
}

func (s *remote) Name() string {
	if s.info.StrategyName != "" {
		return s.info.StrategyName
	}
	return s.info.Name
}
func (s *remote) Timeframes() []string { return append([]string{}, s.info.Timeframes...) }
func (s *remote) Warmup() int          { return s.info.Warmup }

// MaxOpen is the approach's own limit, 0 when it has none (the same as
// not being a MultiPosition).
func (s *remote) MaxOpen() int { return s.info.MaxOpen }

// PluginFile is the plugin file serving the approach.
func (s *remote) PluginFile() string { return s.info.File }

// Close drops the instance in the plugin, and stops the plugin process
// when the strategy owns it (LoadApproach).
func (s *remote) Close() error {
	if s.owns {
		s.proc.stop()
		return nil
	}
	if s.err == nil {
		s.proc.call(&pb.Request{Call: &pb.Request_Close{Close: &pb.CloseRequest{Id: s.id}}})
	}
	s.err = fmt.Errorf("san_backtest: approach %s from plugin %s is closed", s.info.Name, filepath.Base(s.info.File))
	return nil
}

// Grid asks the approach's own walk-forward grid; nil when it has none.
// A failure is kept for the next step, so the trial fails.
func (s *remote) Grid(name string, centre float64) []float64 {
	if !s.info.HasGrid || s.err != nil {
		return nil
	}
	r, err := s.proc.call(&pb.Request{Call: &pb.Request_Grid{Grid: &pb.GridRequest{Id: s.id, Name: name, Centre: centre}}})
	if err != nil {
		s.err = err
		return nil
	}
	if r.GetGrid().GetNone() {
		return nil
	}
	return append([]float64{}, r.GetGrid().GetValues()...)
}

// OnStart begins a run in the plugin. The engine calls it outside its
// panic guard, so a failure waits for the first step.
func (s *remote) OnStart(info StartInfo) {
	if s.err != nil {
		return
	}
	in := info.Instrument
	req := &pb.StartRequest{Id: s.id, Instrument: &pb.Instrument{Name: in.Name, Base: in.Base, Quote: in.Quote, Digits: int32(in.Digits), Point: in.Point,
		ContractSize: in.ContractSize, LotMin: in.LotMin, LotMax: in.LotMax, LotStep: in.LotStep, Spread: in.Spread},
		Timeframes: info.Timeframes, FromUnixNano: unixNano(info.From), ToUnixNano: unixNano(info.To), Balance: info.Balance, Currency: info.Currency,
		Params: info.Params, AnchorsUnixNano: map[string]int64{}}
	s.pendingStart = req
}

// OnCandle sends the step and returns the plugin's signal.
func (s *remote) OnCandle(c *Context) Signal {
	if s.err != nil {
		panic(s.err)
	}
	if st := s.pendingStart; st != nil {
		// The Context exists only from the first step: its anchors and
		// whether it records come with the start.
		for tf, t := range c.anchors {
			st.AnchorsUnixNano[tf] = unixNano(t)
		}
		st.Record = c.record
		s.pendingStart = nil
		// Keep what the plugin has of a timeframe only when it was copied
		// from these same candles (the same array, still at least as
		// long); otherwise it starts empty.
		sent, base := map[string]int{}, map[string]*Candle{}
		for _, tf := range c.tfs {
			f := c.frames[tf]
			if len(f.all) == 0 {
				continue
			}
			base[tf] = &f.all[0]
			if n := s.sent[tf]; n > 0 && s.base[tf] == &f.all[0] && n <= len(f.all) {
				sent[tf] = n
				st.Keep = append(st.Keep, tf)
			}
		}
		s.sent, s.base = sent, base
		if _, err := s.proc.call(&pb.Request{Call: &pb.Request_Start{Start: st}}); err != nil {
			s.err = err
			panic(err)
		}
	}
	req := &pb.StepRequest{Id: s.id, Candles: map[string]*pb.Candles{}, Time: unixNano(c.time), GapNanos: int64(c.gap), Spread: c.inst.Spread,
		MaxOpen: int64(c.maxOpen), Account: &pb.Account{Currency: c.account.Currency, Balance: c.account.Balance, Equity: c.account.Equity,
			Margin: c.account.Margin, Open: int64(c.account.Open)}}
	for _, tf := range c.tfs {
		f := c.frames[tf]
		if from := s.sent[tf]; f.n < from {
			// the plugin holds candles of an earlier run past this step
			if req.Closed == nil {
				req.Closed = map[string]int64{}
			}
			req.Closed[tf] = int64(f.n)
		} else if f.n > from {
			cs := &pb.Candles{Candles: make([]*pb.Candle, 0, f.n-from)}
			for _, k := range f.all[from:f.n] {
				cs.Candles = append(cs.Candles, candleToPB(k))
			}
			req.Candles[tf] = cs
			s.sent[tf] = f.n
		}
	}
	for _, p := range c.positions {
		req.Positions = append(req.Positions, &pb.Position{Side: string(p.Side), Entry: p.Entry, Stop: p.Stop, Target: p.Target, Lots: p.Lots,
			OpenTime: unixNano(p.OpenTime), Risk: p.Risk, Profit: p.Profit, Id: int64(p.ID), OrderId: int64(p.OrderID), Type: p.Type, Tag: p.Tag})
	}
	for _, o := range c.orders {
		req.Orders = append(req.Orders, &pb.Order{Id: int64(o.ID), Side: string(o.Side), Type: o.Type, Price: o.Price, Stop: o.Stop, Target: o.Target,
			Placed: unixNano(o.Placed), BarsLeft: int64(o.BarsLeft), Expires: unixNano(o.Expires), Tag: o.Tag})
	}
	r, err := s.proc.call(&pb.Request{Call: &pb.Request_Step{Step: req}})
	if err != nil {
		s.err = err
		panic(err)
	}
	if c.record {
		c.notes = append(c.notes, r.GetStep().GetNotes()...)
	}
	return signalFromPB(r.GetStep().GetSignal())
}

// unixNano is t in Unix nanoseconds, 0 for the zero time; fromUnixNano
// the reverse, in UTC.
func unixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

func fromUnixNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}

func candleToPB(k Candle) *pb.Candle {
	return &pb.Candle{OpenTime: unixNano(k.OpenTime), CloseTime: unixNano(k.CloseTime), Open: k.Open, High: k.High, Low: k.Low, Close: k.Close,
		AskOpen: k.AskOpen, AskHigh: k.AskHigh, AskLow: k.AskLow, AskClose: k.AskClose, Volume: k.Volume}
}

func signalFromPB(p *pb.Signal) Signal {
	if p == nil {
		return Wait
	}
	s := Signal{Action: Action(p.GetAction()), Stop: p.GetStop(), Target: p.GetTarget(), Reason: p.GetReason(), Price: p.GetPrice(), ID: int(p.GetId()),
		Bars: int(p.GetBars()), Expires: fromUnixNano(p.GetExpires()), Label: p.GetLabel()}
	for _, b := range p.GetBatch() {
		s.Batch = append(s.Batch, signalFromPB(b))
	}
	return s
}
