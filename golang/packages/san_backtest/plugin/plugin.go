// Package plugin serves approaches from a program of their own, for
// san_backtest's Registry.Load, LoadApproach, the CLI's --plugin and San
// Tower's plugins folder:
//
//	func main() {
//		plugin.Serve(map[string]func() bt.Strategy{"my_scalp": MyScalp})
//	}
//
// Build it with `go build -o plugins/my_scalp.exe .`. The approach is
// written exactly as one compiled in (plain Go or a chain with steps). The
// host sends each step's new closed candles plus the positions, orders and
// account; this side keeps a mirror of the Context (bt.Mirror) and
// computes indicators, Forming and the rest with san_backtest's own code,
// so the approach decides exactly as it would compiled in.
//
// The protocol is in pb/plugin.proto: length-prefixed protobuf messages on
// stdin and stdout, one answer per request. stdout belongs to the
// protocol, so Serve points os.Stdout at stderr: a stray fmt.Println goes
// to the host's log instead of breaking the pipe.
package plugin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"time"

	bt "github.com/wargasipil/san_backtest"
	"github.com/wargasipil/san_backtest/internal/pipe"
	"github.com/wargasipil/san_backtest/plugin/pb"
	"google.golang.org/protobuf/encoding/protodelim"
)

// The versions this plugin answers Hello with; tests change them.
var (
	protocolVersion int32 = bt.PluginProtocol
	apiVersion      int32 = bt.PluginAPI
)

// Serve answers the host until it shuts the plugin down or closes stdin,
// then exits the program. makers are the approaches by name; each instance
// the host asks for is a fresh maker() call.
func Serve(makers map[string]func() bt.Strategy) {
	out := os.Stdout
	os.Stdout = os.Stderr
	err := ServeIO(makers, os.Stdin, out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plugin:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// Makers are a registry's approaches as Serve's makers, e.g.
// plugin.Serve(plugin.Makers(r)) after examples.Register(r).
func Makers(r *bt.Registry) map[string]func() bt.Strategy {
	out := map[string]func() bt.Strategy{}
	for _, n := range r.Names() {
		out[n] = func() bt.Strategy {
			s, err := r.New(n)
			if err != nil {
				panic(err)
			}
			return s
		}
	}
	return out
}

// ServeIO is Serve on any pipe pair, returning when in ends or the host
// sends Shutdown.
func ServeIO(makers map[string]func() bt.Strategy, in io.Reader, out io.Writer) error {
	s := &server{makers: makers, inst: map[uint64]*instance{}}
	r := bufio.NewReaderSize(pipe.Reader(in), 1<<16)
	w := bufio.NewWriterSize(pipe.Writer(out), 1<<16)
	for {
		req := &pb.Request{}
		if err := (protodelim.UnmarshalOptions{MaxSize: -1}).UnmarshalFrom(r, req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading a request: %w", err)
		}
		res := s.handle(req)
		if _, err := protodelim.MarshalTo(w, res); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
		if req.GetShutdown() != nil {
			return nil
		}
	}
}

type server struct {
	makers map[string]func() bt.Strategy
	inst   map[uint64]*instance
	next   uint64
}

type instance struct {
	s      bt.Strategy
	mirror *bt.Mirror
}

// handle answers one request; a panic in the approach becomes the
// answer's error, its stack goes to stderr.
func (s *server) handle(req *pb.Request) (res *pb.Response) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "plugin: panic: %v\n%s", r, debug.Stack())
			res = &pb.Response{Error: fmt.Sprintf("panic: %v", r)}
		}
	}()
	ok := &pb.Response{Reply: &pb.Response_Ok{Ok: &pb.Empty{}}}
	switch c := req.GetCall().(type) {
	case *pb.Request_Hello:
		return &pb.Response{Reply: &pb.Response_Hello{Hello: &pb.HelloReply{Protocol: protocolVersion, Api: apiVersion,
			Program: filepath.Base(os.Args[0])}}}
	case *pb.Request_List:
		return &pb.Response{Reply: &pb.Response_List{List: s.list()}}
	case *pb.Request_NewInstance:
		mk, found := s.makers[c.NewInstance.GetName()]
		if !found {
			return &pb.Response{Error: fmt.Sprintf("no approach %q", c.NewInstance.GetName())}
		}
		s.next++
		s.inst[s.next] = &instance{s: mk()}
		return &pb.Response{Reply: &pb.Response_NewInstance{NewInstance: &pb.NewInstanceReply{Id: s.next}}}
	case *pb.Request_Start:
		in, err := s.get(c.Start.GetId())
		if err != nil {
			return &pb.Response{Error: err.Error()}
		}
		in.start(c.Start)
		return ok
	case *pb.Request_Step:
		in, err := s.get(c.Step.GetId())
		if err != nil {
			return &pb.Response{Error: err.Error()}
		}
		if in.mirror == nil {
			return &pb.Response{Error: "step before start"}
		}
		return &pb.Response{Reply: &pb.Response_Step{Step: in.step(c.Step)}}
	case *pb.Request_Grid:
		in, err := s.get(c.Grid.GetId())
		if err != nil {
			return &pb.Response{Error: err.Error()}
		}
		g, has := in.s.(bt.GridTuner)
		var vs []float64
		if has {
			vs = g.Grid(c.Grid.GetName(), c.Grid.GetCentre())
		}
		return &pb.Response{Reply: &pb.Response_Grid{Grid: &pb.GridReply{Values: vs, None: vs == nil}}}
	case *pb.Request_Close:
		delete(s.inst, c.Close.GetId())
		return ok
	case *pb.Request_Shutdown:
		return ok
	}
	return &pb.Response{Error: "unknown request"}
}

func (s *server) get(id uint64) (*instance, error) {
	in, found := s.inst[id]
	if !found {
		return nil, fmt.Errorf("no instance %d", id)
	}
	return in, nil
}

// list describes every approach from one maker() call each.
func (s *server) list() *pb.ListReply {
	var names []string
	for n := range s.makers {
		names = append(names, n)
	}
	sort.Strings(names)
	out := &pb.ListReply{}
	for _, n := range names {
		st := s.makers[n]()
		a := &pb.Approach{Name: n, StrategyName: st.Name(), Timeframes: st.Timeframes(), Warmup: int32(st.Warmup())}
		if t, ok := st.(bt.Tunable); ok {
			a.HasParams, a.Params = true, t.Params()
		}
		if mp, ok := st.(bt.MultiPosition); ok {
			a.HasMaxOpen, a.MaxOpen = true, int32(mp.MaxOpen())
		}
		_, a.HasOnStart = st.(bt.Starter)
		_, a.HasGrid = st.(bt.GridTuner)
		out.Approaches = append(out.Approaches, a)
	}
	return out
}

func (in *instance) start(r *pb.StartRequest) {
	i := r.GetInstrument()
	info := bt.StartInfo{Instrument: bt.InstrumentInfo{Name: i.GetName(), Base: i.GetBase(), Quote: i.GetQuote(), Digits: int(i.GetDigits()),
		Point: i.GetPoint(), ContractSize: i.GetContractSize(), LotMin: i.GetLotMin(), LotMax: i.GetLotMax(), LotStep: i.GetLotStep(), Spread: i.GetSpread()},
		Timeframes: r.GetTimeframes(), From: fromUnixNano(r.GetFromUnixNano()), To: fromUnixNano(r.GetToUnixNano()), Balance: r.GetBalance(),
		Currency: r.GetCurrency(), Params: map[string]float64{}}
	for k, v := range r.GetParams() {
		info.Params[k] = v
	}
	anchors := map[string]time.Time{}
	for tf, t := range r.GetAnchorsUnixNano() {
		anchors[tf] = fromUnixNano(t)
	}
	if in.mirror != nil && len(r.GetKeep()) > 0 {
		in.mirror = in.mirror.Restart(info, anchors, r.GetRecord(), r.GetKeep())
	} else {
		in.mirror = bt.NewMirror(info, anchors, r.GetRecord())
	}
	if st, ok := in.s.(bt.Starter); ok {
		st.OnStart(info)
	}
}

func (in *instance) step(r *pb.StepRequest) *pb.StepReply {
	for tf, cs := range r.GetCandles() {
		add := make([]bt.Candle, 0, len(cs.GetCandles()))
		for _, k := range cs.GetCandles() {
			add = append(add, bt.Candle{OpenTime: fromUnixNano(k.GetOpenTime()), CloseTime: fromUnixNano(k.GetCloseTime()), Open: k.GetOpen(),
				High: k.GetHigh(), Low: k.GetLow(), Close: k.GetClose(), AskOpen: k.GetAskOpen(), AskHigh: k.GetAskHigh(), AskLow: k.GetAskLow(),
				AskClose: k.GetAskClose(), Volume: k.GetVolume()})
		}
		in.mirror.Add(tf, add)
	}
	for tf, n := range r.GetClosed() {
		in.mirror.Closed(tf, int(n))
	}
	st := bt.MirrorStep{Time: fromUnixNano(r.GetTime()), Gap: time.Duration(r.GetGapNanos()), Spread: r.GetSpread(), MaxOpen: int(r.GetMaxOpen())}
	for _, p := range r.GetPositions() {
		st.Positions = append(st.Positions, bt.Position{Side: bt.Side(p.GetSide()), Entry: p.GetEntry(), Stop: p.GetStop(), Target: p.GetTarget(),
			Lots: p.GetLots(), OpenTime: fromUnixNano(p.GetOpenTime()), Risk: p.GetRisk(), Profit: p.GetProfit(), ID: int(p.GetId()),
			OrderID: int(p.GetOrderId()), Type: p.GetType(), Tag: p.GetTag()})
	}
	for _, o := range r.GetOrders() {
		st.Orders = append(st.Orders, bt.Order{ID: int(o.GetId()), Side: bt.Side(o.GetSide()), Type: o.GetType(), Price: o.GetPrice(), Stop: o.GetStop(),
			Target: o.GetTarget(), Placed: fromUnixNano(o.GetPlaced()), BarsLeft: int(o.GetBarsLeft()), Expires: fromUnixNano(o.GetExpires()), Tag: o.GetTag()})
	}
	if a := r.GetAccount(); a != nil {
		st.Account = bt.Account{Currency: a.GetCurrency(), Balance: a.GetBalance(), Equity: a.GetEquity(), Margin: a.GetMargin(), Open: int(a.GetOpen())}
	}
	sig := in.s.OnCandle(in.mirror.Context(st))
	return &pb.StepReply{Signal: signalToPB(sig), Notes: in.mirror.Notes()}
}

func signalToPB(s bt.Signal) *pb.Signal {
	p := &pb.Signal{Action: int32(s.Action), Stop: s.Stop, Target: s.Target, Reason: s.Reason, Price: s.Price, Id: int64(s.ID), Bars: int64(s.Bars),
		Expires: unixNano(s.Expires), Label: s.Label}
	for _, b := range s.Batch {
		p.Batch = append(p.Batch, signalToPB(b))
	}
	return p
}

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
