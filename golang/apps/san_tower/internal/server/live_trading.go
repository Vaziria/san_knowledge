package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"
	bt "github.com/wargasipil/san_backtest"
	exness "github.com/wargasipil/san_exness"
	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/backtestanalysis"
	"github.com/wargasipil/san_tower/internal/livetrading"
	"github.com/wargasipil/san_tower/internal/store"
)

// LiveTradingService: the Live Trading page (the user, 2026-11-15:
// "realtime trading menu in tower", named "Live Trading"), stage one:
// "Monitor + Signals (paper trading)". It reads the Exness connection
// (the user, 2026-11-15: "1a": monitor the tower's existing Exness
// connection) through livetrading's read-only interface and never trades;
// safety_test.go in livetrading checks this file too. The work is in
// internal/livetrading; here are the RPCs and the proto mapping.

// liveService answers LiveTradingService (a type of its own, as
// analysisService, so its method names never clash with other services').
type liveService struct{ *Server }

var _ towerv1connect.LiveTradingServiceHandler = liveService{}

var errLiveNotLocal = connect.NewError(connect.CodePermissionDenied, errors.New("Live Trading answers on the tower's own computer only"))

// exnessClient is the read-only client of Exness connection id (0: the
// first one by name).
func (s *Server) exnessClient(_ context.Context, id uint32) (livetrading.Exness, uint32, string, error) {
	var conn store.Connection
	if id != 0 {
		c, err := s.store.GetConnection(id)
		if err != nil {
			return nil, 0, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("Exness connection %d: %w", id, err))
		}
		conn = c
	} else {
		list, err := s.store.ListConnections(store.Exness)
		if err != nil {
			return nil, 0, "", err
		}
		if len(list) == 0 {
			return nil, 0, "", connect.NewError(connect.CodeFailedPrecondition, errors.New("the tower has no Exness connection: add one under Connections"))
		}
		conn = list[0]
	}
	if conn.Type != store.Exness {
		return nil, 0, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("connection %q is not an Exness connection", conn.Name))
	}
	if s.engine == nil {
		return nil, 0, "", errors.New("the tower has no engine")
	}
	c, err := s.engine.Exness().Client(conn.Settings.GetExness())
	if err != nil {
		return nil, 0, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("Exness connection %q: %w", conn.Name, err))
	}
	return livetrading.ReadOnly(c), conn.ID, conn.Name, nil
}

func liveError(err error) error {
	var ce *connect.Error
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ce):
		return err
	case errors.Is(err, store.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, store.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	case errors.Is(err, store.ErrExists):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewError(connect.CodeUnavailable, err)
}

func tsOf(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return tsOf(*t)
}

func (s liveService) connections() []*v1.LiveConnection {
	list, _ := s.store.ListConnections(store.Exness)
	var out []*v1.LiveConnection
	for _, c := range list {
		out = append(out, &v1.LiveConnection{Id: c.ID, Name: c.Name})
	}
	return out
}

func monitorProto(st livetrading.MonitorState) *v1.LiveMonitorState {
	a := st.Account
	out := &v1.LiveMonitorState{Connection: &v1.LiveConnection{Id: st.ConnectionID, Name: st.Connection}, Status: st.Status, Message: st.Message,
		Account: &v1.LiveAccount{AccountId: a.ID, Name: a.Name, Currency: a.Currency, Leverage: a.Leverage, Balance: a.Balance, Equity: a.Equity,
			Margin: a.Margin, FreeMargin: a.FreeMargin}, Updated: tsOf(st.Updated), Source: st.Source, ReadOnly: true}
	for _, p := range st.Positions {
		out.Positions = append(out.Positions, &v1.LivePosition{Id: p.ID, Instrument: p.Instrument, Side: p.Side, Volume: p.Volume, OpenPrice: p.OpenPrice,
			OpenTime: tsOf(p.OpenTime), StopLoss: p.StopLoss, TakeProfit: p.TakeProfit, Price: p.Price, Profit: p.Profit, Currency: p.Currency,
			Swap: p.Swap, Commission: p.Commission})
	}
	for _, o := range st.Orders {
		out.Orders = append(out.Orders, &v1.LiveOrder{Id: o.ID, Instrument: o.Instrument, Side: o.Side, Type: o.Type, Volume: o.Volume, Price: o.Price,
			StopLoss: o.StopLoss, TakeProfit: o.TakeProfit, Created: tsOf(o.Created)})
	}
	return out
}

func noConnection(err error) *v1.LiveMonitorState {
	return &v1.LiveMonitorState{Status: livetrading.MonitorNoConnection, Message: err.Error(), ReadOnly: true, Account: &v1.LiveAccount{}}
}

func (s liveService) GetMonitor(ctx context.Context, req *connect.Request[v1.LiveGetMonitorRequest]) (*connect.Response[v1.LiveGetMonitorResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	res := &v1.LiveGetMonitorResponse{Connections: s.connections()}
	mo, err := s.live.Monitor(ctx, req.Msg.GetConnectionId())
	if err != nil {
		res.State = noConnection(err)
		return connect.NewResponse(res), nil
	}
	res.State = monitorProto(mo.State())
	return connect.NewResponse(res), nil
}

func (s liveService) WatchMonitor(ctx context.Context, req *connect.Request[v1.LiveWatchMonitorRequest], stream *connect.ServerStream[v1.LiveWatchMonitorResponse]) error {
	if !fromThisComputer(req.Peer()) {
		return errLiveNotLocal
	}
	mo, err := s.live.Monitor(ctx, req.Msg.GetConnectionId())
	if err != nil {
		return stream.Send(&v1.LiveWatchMonitorResponse{Event: &v1.LiveWatchMonitorResponse_State{State: noConnection(err)}})
	}
	ch, leave := mo.Watch(req.Msg.GetInstruments())
	defer leave()
	beat := time.NewTicker(watchHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.analysisStop:
			return nil
		case <-beat.C:
			if err := stream.Send(&v1.LiveWatchMonitorResponse{Event: &v1.LiveWatchMonitorResponse_Heartbeat{Heartbeat: true}}); err != nil {
				return err
			}
		case ev := <-ch:
			var msg *v1.LiveWatchMonitorResponse
			switch {
			case ev.State != nil:
				msg = &v1.LiveWatchMonitorResponse{Event: &v1.LiveWatchMonitorResponse_State{State: monitorProto(*ev.State)}}
			case ev.Tick != nil:
				t := ev.Tick
				msg = &v1.LiveWatchMonitorResponse{Event: &v1.LiveWatchMonitorResponse_Tick{Tick: &v1.LiveTick{Instrument: t.Instrument, Bid: t.Bid, Ask: t.Ask,
					Time: tsOf(t.Timestamp)}}}
			default:
				continue
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

func (s liveService) GetLiveCandles(ctx context.Context, req *connect.Request[v1.LiveGetCandlesRequest]) (*connect.Response[v1.LiveGetCandlesResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	m := req.Msg
	api, _, _, err := s.exnessClient(ctx, m.GetConnectionId())
	if err != nil {
		return nil, err
	}
	tf, err := bt.ParseTimeframe(m.GetTimeframe())
	if m.GetTimeframe() == "" {
		tf, err = "M15", nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	inst := strings.TrimSpace(m.GetInstrument())
	if inst == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an instrument is needed, such as XAUUSD"))
	}
	n := int(m.GetCount())
	if n <= 0 {
		n = 300
	}
	n = min(n, 1000)
	now := api.Now().UTC()
	cs, err := api.Candles(ctx, exness.CandleQuery{Instrument: inst, Timeframe: exness.Timeframe(tf), PriceType: exness.PriceBid, From: now, Count: -n})
	if err != nil {
		return nil, liveError(err)
	}
	res := &v1.LiveGetCandlesResponse{Instrument: inst, Timeframe: tf}
	for _, c := range cs {
		res.Candles = append(res.Candles, &v1.BacktestCandle{Time: c.Time.Unix(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume})
	}
	if k := len(cs); k > 0 {
		res.Forming = cs[k-1].Time.Add(bt.TimeframeDuration(tf)).After(now)
	}
	return connect.NewResponse(res), nil
}

func (s liveService) ListApproaches(_ context.Context, req *connect.Request[v1.LiveListApproachesRequest]) (*connect.Response[v1.LiveListApproachesResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	res := &v1.LiveListApproachesResponse{}
	for _, a := range s.live.Approaches() {
		res.Approaches = append(res.Approaches, &v1.LiveApproach{Name: a.Name, Timeframes: a.Timeframes, Warmup: uint32(a.Warmup), Params: a.Params,
			MaxOpen: uint32(max(a.MaxOpen, 0))})
	}
	return connect.NewResponse(res), nil
}

// costs maps the settings' cost overrides to the stored JSON.
func costsJSON(c *v1.LivePaperCosts) string {
	out := map[string]any{}
	if c == nil {
		return "{}"
	}
	if c.Spread != nil {
		out["spread"] = c.GetSpread()
	}
	if c.Commission != nil {
		out["commission"] = c.GetCommission()
	}
	if c.SwapLong != nil || c.SwapShort != nil {
		out["swap"] = [2]float64{c.GetSwapLong(), c.GetSwapShort()}
	}
	if c.GetSlippage() > 0 {
		out["slippage"] = c.GetSlippage()
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func costsProto(s string) *v1.LivePaperCosts {
	var c struct {
		Spread     *float64    `json:"spread"`
		Commission *float64    `json:"commission"`
		Swap       *[2]float64 `json:"swap"`
		Slippage   float64     `json:"slippage"`
	}
	_ = json.Unmarshal([]byte(s), &c)
	out := &v1.LivePaperCosts{Spread: c.Spread, Commission: c.Commission, Slippage: c.Slippage}
	if c.Swap != nil {
		out.SwapLong, out.SwapShort = &c.Swap[0], &c.Swap[1]
	}
	return out
}

func sessionProto(p store.PaperSession) *v1.LivePaperSession {
	params := map[string]float64{}
	_ = json.Unmarshal([]byte(p.Params), &params)
	out := &v1.LivePaperSession{Id: p.ID, Status: p.Status, Message: p.Message, From: tsPtr(p.From), StartedAt: tsPtr(p.StartedAt),
		StoppedAt: tsPtr(p.StoppedAt), Balance: p.NowBalance, Equity: p.Equity, NetProfit: p.NetProfit, Trades: p.Trades, OpenPositions: p.OpenPositions,
		PendingOrders: p.PendingOrders, Candles: p.Candles, LastClose: tsPtr(p.LastClose), NextClose: tsPtr(p.NextClose), LastSignal: p.LastSignal,
		LastSignalAt: tsPtr(p.LastSignalAt), CreatedAt: tsOf(p.CreatedAt), UpdatedAt: tsOf(p.UpdatedAt), ReportPath: backtestanalysis.PaperPath(p.ID),
		Paper: true,
		Settings: &v1.LivePaperSettings{Name: p.Name, ConnectionId: p.ConnectionID, Approach: p.Approach, Instrument: p.Instrument, Balance: p.Balance,
			Risk: p.Risk, Leverage: uint32(max(p.Leverage, 0)), MaxOpen: uint32(max(p.MaxOpen, 0)), MaxOpenRisk: p.MaxOpenRisk, Params: params,
			Costs: costsProto(p.Costs)}}
	if p.Timeframes != "" {
		out.Timeframes = strings.Split(p.Timeframes, ",")
	}
	if p.NowBalance == 0 && p.Candles == 0 {
		out.Balance, out.Equity = p.Balance, p.Balance
	}
	var st struct {
		Positions []bt.Position `json:"positions"`
		Orders    []bt.Order    `json:"orders"`
	}
	_ = json.Unmarshal([]byte(p.State), &st)
	for _, x := range st.Positions {
		out.Positions = append(out.Positions, &v1.LivePaperPosition{Id: uint32(x.ID), Side: string(x.Side), Entry: x.Entry, Stop: x.Stop, Target: x.Target,
			Lots: x.Lots, OpenTime: tsOf(x.OpenTime), Profit: x.Profit, Type: x.Type, Tag: x.Tag})
	}
	for _, o := range st.Orders {
		out.Orders = append(out.Orders, &v1.LivePaperOrder{Id: uint32(o.ID), Side: string(o.Side), Type: o.Type, Price: o.Price, Stop: o.Stop, Target: o.Target,
			Placed: tsOf(o.Placed), BarsLeft: uint32(max(o.BarsLeft, 0)), Tag: o.Tag})
	}
	return out
}

func (s liveService) CreatePaperSession(_ context.Context, req *connect.Request[v1.LiveCreatePaperSessionRequest]) (*connect.Response[v1.LivePaperSessionResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	in := req.Msg.GetSettings()
	if in == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("settings are needed"))
	}
	p := store.PaperSession{Name: in.GetName(), ConnectionID: in.GetConnectionId(), Approach: in.GetApproach(), Instrument: in.GetInstrument(),
		Balance: in.GetBalance(), Risk: in.GetRisk(), Leverage: int32(min(in.GetLeverage(), math.MaxInt32)), MaxOpen: int32(min(in.GetMaxOpen(), 1000)),
		MaxOpenRisk: in.GetMaxOpenRisk(), Costs: costsJSON(in.GetCosts())}
	if p.Balance == 0 {
		p.Balance = 10000
	}
	if p.Risk == 0 {
		p.Risk = 1
	}
	if p.Leverage == 0 {
		p.Leverage = 2000
	}
	p, err := s.live.Create(p, in.GetParams())
	if err != nil {
		return nil, liveError(err)
	}
	return connect.NewResponse(&v1.LivePaperSessionResponse{Session: sessionProto(p)}), nil
}

func (s liveService) session(id uint32) (*connect.Response[v1.LivePaperSessionResponse], error) {
	p, err := s.live.Session(id)
	if err != nil {
		return nil, liveError(err)
	}
	return connect.NewResponse(&v1.LivePaperSessionResponse{Session: sessionProto(p)}), nil
}

func (s liveService) StartPaperSession(_ context.Context, req *connect.Request[v1.LivePaperSessionRequest]) (*connect.Response[v1.LivePaperSessionResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	if err := s.live.Start(req.Msg.GetId()); err != nil {
		return nil, liveError(err)
	}
	return s.session(req.Msg.GetId())
}

func (s liveService) StopPaperSession(_ context.Context, req *connect.Request[v1.LivePaperSessionRequest]) (*connect.Response[v1.LivePaperSessionResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	s.live.Stop(req.Msg.GetId())
	return s.session(req.Msg.GetId())
}

func (s liveService) DeletePaperSession(_ context.Context, req *connect.Request[v1.LivePaperSessionRequest]) (*connect.Response[v1.LiveDeletePaperSessionResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	if err := s.live.Delete(req.Msg.GetId()); err != nil {
		return nil, liveError(err)
	}
	return connect.NewResponse(&v1.LiveDeletePaperSessionResponse{}), nil
}

func (s liveService) GetPaperSession(_ context.Context, req *connect.Request[v1.LivePaperSessionRequest]) (*connect.Response[v1.LivePaperSessionResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	return s.session(req.Msg.GetId())
}

func (s liveService) list() (*v1.LiveListPaperSessionsResponse, error) {
	list, err := s.live.Sessions()
	if err != nil {
		return nil, liveError(err)
	}
	res := &v1.LiveListPaperSessionsResponse{}
	for _, p := range list {
		res.Sessions = append(res.Sessions, sessionProto(p))
	}
	return res, nil
}

func (s liveService) ListPaperSessions(_ context.Context, req *connect.Request[v1.LiveListPaperSessionsRequest]) (*connect.Response[v1.LiveListPaperSessionsResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	res, err := s.list()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

func (s liveService) WatchPaperSessions(ctx context.Context, req *connect.Request[v1.LiveWatchPaperSessionsRequest], stream *connect.ServerStream[v1.LiveWatchPaperSessionsResponse]) error {
	if !fromThisComputer(req.Peer()) {
		return errLiveNotLocal
	}
	ch, leave := s.live.Watch(0) // subscribe first, then read: an overlap is harmless (full rows)
	defer leave()
	snap, err := s.list()
	if err != nil {
		return err
	}
	if err := stream.Send(&v1.LiveWatchPaperSessionsResponse{Event: &v1.LiveWatchPaperSessionsResponse_Snapshot{Snapshot: snap}}); err != nil {
		return err
	}
	beat := time.NewTicker(watchHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.analysisStop:
			return nil
		case <-beat.C:
			if err := stream.Send(&v1.LiveWatchPaperSessionsResponse{Event: &v1.LiveWatchPaperSessionsResponse_Heartbeat{Heartbeat: true}}); err != nil {
				return err
			}
		case ev, ok := <-ch:
			if !ok {
				return nil // too slow, or the tower stops: the page reconnects and reads again
			}
			msg := &v1.LiveWatchPaperSessionsResponse{Event: &v1.LiveWatchPaperSessionsResponse_Removed{Removed: ev.Removed}}
			if ev.Session != nil {
				msg = &v1.LiveWatchPaperSessionsResponse{Event: &v1.LiveWatchPaperSessionsResponse_Changed{Changed: sessionProto(*ev.Session)}}
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		}
	}
}

func tradeProto(i int, t bt.Trade) *v1.BacktestTrade {
	return &v1.BacktestTrade{Index: uint32(i), Leg: uint32(t.Leg), Strategy: t.Strategy, Instrument: t.Instrument, Side: string(t.Side),
		SignalTime: tsOf(t.SignalTime), OpenTime: tsOf(t.OpenTime), CloseTime: tsOf(t.CloseTime), Entry: t.Entry, Exit: t.Exit, Stop: t.Stop,
		FinalStop: t.FinalStop, Target: t.Target, Lots: t.Lots, Spread: t.Spread, Risk: t.Risk, Gross: t.Gross, Commission: t.Commission, Swap: t.Swap,
		Net: t.Net, R: t.R, Reason: t.Reason, Session: backtestanalysis.Session(t.OpenTime), Month: t.OpenTime.UTC().Format("2006-01"),
		PositionId: uint32(t.PositionID), OrderId: uint32(t.OrderID), OrderType: t.OrderType, Tag: t.Tag}
}

func decisionProto(line int, d bt.Decision) *v1.BacktestDecision {
	det := map[string]float64{}
	for k, v := range d.Details {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			det[k] = v
		}
	}
	return &v1.BacktestDecision{Time: tsOf(d.Time), Leg: uint32(d.Leg), Kind: d.Kind, Side: string(d.Side), Details: det, Reason: d.Reason, Line: uint32(line)}
}

func candleProto(c bt.Candle) *v1.BacktestCandle {
	return &v1.BacktestCandle{Time: c.OpenTime.Unix(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume, Spread: c.AskClose - c.Close}
}

func missedProto(m bt.Missed) *v1.BacktestMissed {
	return &v1.BacktestMissed{Time: tsOf(m.Time), Leg: uint32(m.Leg), Strategy: m.Strategy, Instrument: m.Instrument, Side: string(m.Side), Reason: m.Reason,
		OrderId: uint32(m.OrderID), OrderType: m.OrderType, Tag: m.Tag}
}

// journal reads a session's journal from the store: every trade, missed
// signal and equity point, the newest decisions and candles.
func (s liveService) journal(id uint32, decisions, candles int) (*v1.LivePaperJournal, error) {
	j := &v1.LivePaperJournal{}
	read := func(kind string, limit int, each func(i int, b []byte) error) error {
		recs, err := s.store.PaperRecords(id, kind, time.Time{}, time.Time{}, limit)
		if err != nil {
			return err
		}
		for i, r := range recs {
			if err := each(i, []byte(r.Data)); err != nil {
				return err
			}
		}
		return nil
	}
	peak := math.Inf(-1)
	total, _ := s.store.CountPaperRecords(id, store.PaperDecision)
	first := int(total) - decisions
	err := errors.Join(
		read(store.PaperTrade, 0, func(i int, b []byte) error {
			var t bt.Trade
			err := json.Unmarshal(b, &t)
			j.Trades = append(j.Trades, tradeProto(i, t))
			return err
		}),
		read(store.PaperDecision, decisions, func(i int, b []byte) error {
			var d bt.Decision
			err := json.Unmarshal(b, &d)
			j.Decisions = append(j.Decisions, decisionProto(max(first, 0)+i+1, d))
			return err
		}),
		read(store.PaperEquity, 0, func(_ int, b []byte) error {
			var e bt.EquityPoint
			err := json.Unmarshal(b, &e)
			peak = math.Max(peak, e.Equity)
			dd := 0.0
			if peak > 0 {
				dd = 100 * (peak - e.Equity) / peak
			}
			j.Equity = append(j.Equity, &v1.BacktestEquityPoint{Time: tsOf(e.Time), Balance: e.Balance, Equity: e.Equity, DrawdownPct: dd})
			return err
		}),
		read(store.PaperCandle, candles, func(_ int, b []byte) error {
			var c bt.Candle
			err := json.Unmarshal(b, &c)
			j.Candles = append(j.Candles, candleProto(c))
			return err
		}),
		read(store.PaperMissed, 0, func(_ int, b []byte) error {
			var m bt.Missed
			err := json.Unmarshal(b, &m)
			j.Missed = append(j.Missed, missedProto(m))
			return err
		}),
	)
	sort.SliceStable(j.Trades, func(a, b int) bool { return j.Trades[a].CloseTime.AsTime().Before(j.Trades[b].CloseTime.AsTime()) })
	return j, err
}

func (s liveService) WatchPaperSession(ctx context.Context, req *connect.Request[v1.LiveWatchPaperSessionRequest], stream *connect.ServerStream[v1.LiveWatchPaperSessionResponse]) error {
	if !fromThisComputer(req.Peer()) {
		return errLiveNotLocal
	}
	id := req.Msg.GetId()
	dec, cand := int(req.Msg.GetDecisions()), int(req.Msg.GetCandles())
	if dec <= 0 {
		dec = 500
	}
	if cand <= 0 {
		cand = 2000
	}
	ch, leave := s.live.Watch(id)
	defer leave()
	p, err := s.live.Session(id)
	if err != nil {
		return liveError(err)
	}
	j, err := s.journal(id, dec, cand)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if err := stream.Send(&v1.LiveWatchPaperSessionResponse{Event: &v1.LiveWatchPaperSessionResponse_Snapshot{Snapshot: &v1.LivePaperSessionDetail{
		Session: sessionProto(p), Journal: j}}}); err != nil {
		return err
	}
	trades := len(j.Trades)
	beat := time.NewTicker(watchHeartbeat)
	defer beat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.analysisStop:
			return nil
		case <-beat.C:
			if err := stream.Send(&v1.LiveWatchPaperSessionResponse{Event: &v1.LiveWatchPaperSessionResponse_Heartbeat{Heartbeat: true}}); err != nil {
				return err
			}
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if ev.Removed != 0 {
				return stream.Send(&v1.LiveWatchPaperSessionResponse{Event: &v1.LiveWatchPaperSessionResponse_Removed{Removed: true}})
			}
			if ev.Reset {
				// a start's replay wrote the journal again: a new snapshot
				j, err := s.journal(id, dec, cand)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				trades = len(j.Trades)
				if err := stream.Send(&v1.LiveWatchPaperSessionResponse{Event: &v1.LiveWatchPaperSessionResponse_Snapshot{Snapshot: &v1.LivePaperSessionDetail{
					Session: sessionProto(*ev.Session), Journal: j}}}); err != nil {
					return err
				}
				continue
			}
			up := &v1.LivePaperJournal{}
			for _, t := range ev.Trades {
				up.Trades = append(up.Trades, tradeProto(trades, t))
				trades++
			}
			for _, d := range ev.Decisions {
				up.Decisions = append(up.Decisions, decisionProto(0, d))
			}
			for _, e := range ev.Equity {
				up.Equity = append(up.Equity, &v1.BacktestEquityPoint{Time: tsOf(e.Time), Balance: e.Balance, Equity: e.Equity})
			}
			for _, c := range ev.Candles {
				up.Candles = append(up.Candles, candleProto(c))
			}
			for _, m := range ev.Missed {
				up.Missed = append(up.Missed, missedProto(m))
			}
			if err := stream.Send(&v1.LiveWatchPaperSessionResponse{Event: &v1.LiveWatchPaperSessionResponse_Update{Update: &v1.LivePaperSessionDetail{
				Session: sessionProto(*ev.Session), Journal: up}}}); err != nil {
				return err
			}
		}
	}
}

func (s liveService) ExportPaperSession(_ context.Context, req *connect.Request[v1.LiveExportPaperSessionRequest]) (*connect.Response[v1.LiveExportPaperSessionResponse], error) {
	if !fromThisComputer(req.Peer()) {
		return nil, errLiveNotLocal
	}
	path, files, err := s.live.Export(req.Msg.GetId(), req.Msg.GetPath())
	if err != nil {
		return nil, liveError(err)
	}
	return connect.NewResponse(&v1.LiveExportPaperSessionResponse{Path: path, Files: files}), nil
}

// paperDir is the report folder of a "paper:<id>" path (made from the
// store when the session changed).
func (s *Server) paperDir(path string) (string, uint32, bool, error) {
	id, ok := backtestanalysis.PaperID(path)
	if !ok {
		return "", 0, false, nil
	}
	dir, _, err := s.live.View(id)
	if err != nil {
		return "", id, true, liveError(err)
	}
	return dir, id, true, nil
}

// paperRuns lists the paper sessions as Backtest Analysis runs.
func (s *Server) paperRuns() (*v1.BacktestOpenFolderResponse, error) {
	list, err := s.live.Sessions()
	if err != nil {
		return nil, err
	}
	res := &v1.BacktestOpenFolderResponse{Path: backtestanalysis.PaperPrefix}
	for _, p := range list {
		var sum *v1.BacktestRunSummary
		dir, _, verr := s.live.View(p.ID)
		if verr == nil {
			sum, verr = backtestanalysis.Summary(dir)
		}
		if verr != nil {
			sum = &v1.BacktestRunSummary{Format: backtestanalysis.FormatRun, Kind: "run", Approach: p.Approach, Instrument: p.Instrument,
				StartBalance: p.Balance, Problem: "the session has not run a candle yet"}
		}
		res.Runs = append(res.Runs, backtestanalysis.PaperSummary(sum, p.ID, p.Name, p.Status))
	}
	return res, nil
}
