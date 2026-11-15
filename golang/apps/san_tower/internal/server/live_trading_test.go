package server

import (
	"context"
	"log/slog"
	"math"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/wargasipil/san_message"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/backtestanalysis"
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/livetrading/livefake"
	"github.com/wargasipil/san_tower/internal/store"
)

// liveMarket gives the fake days of flat-trending M15/H1/H4 candles up to
// now (weekdays only).
func liveMarket(f *livefake.Fake, days int) {
	now := time.Now().UTC()
	var m15 []livefake.Candle
	p := 2400.0
	for t, i := now.AddDate(0, 0, -days).Truncate(15*time.Minute), 0; t.Before(now); t, i = t.Add(15*time.Minute), i+1 {
		if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		o := p
		p += 3*math.Sin(float64(i)/7) + 0.8*math.Sin(float64(i)/60)
		m15 = append(m15, livefake.Candle{Time: t, Open: o, High: math.Max(o, p) + 1, Low: math.Min(o, p) - 1, Close: p, Spread: 0.2, Volume: 50})
	}
	f.SetCandles("XAUUSD", "M15", m15)
	for _, d := range []time.Duration{time.Hour, 4 * time.Hour} {
		var cs []livefake.Candle
		for _, c := range m15 {
			b := c.Time.Truncate(d)
			if k := len(cs); k > 0 && cs[k-1].Time.Equal(b) {
				x := &cs[k-1]
				x.High, x.Low, x.Close = math.Max(x.High, c.High), math.Min(x.Low, c.Low), c.Close
				continue
			}
			c.Time = b
			cs = append(cs, c)
		}
		tf := "H1"
		if d > time.Hour {
			tf = "H4"
		}
		f.SetCandles("XAUUSD", tf, cs)
	}
}

// The service end to end on a fake Exness: the monitor, a paper session
// created, started (a catch-up from its first candle), opened in Backtest
// Analysis as "paper:<id>", exported, stopped and deleted. No trading
// request reaches the fake.
func TestLiveTradingService(t *testing.T) {
	f, err := livefake.New("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	liveMarket(f, 30)
	f.AddPosition("XAUUSD", "buy", 0.1, 2390, 2380, 2450)
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	st, err := store.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	b, err := broker.Open(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(st, b, log)
	eng.Exness().UseHTTPClient(f.Client())
	srv := New("test", b, st, eng, offRemote(t, dir))
	ts := httptest.NewUnstartedServer(srv.Handler())
	san_message.ConfigureServer(ts.Config)
	ts.Start()
	defer func() {
		srv.Close()
		ts.Close()
		eng.Close()
		b.Close()
		st.Close()
	}()
	ctx := context.Background()
	conns := towerv1connect.NewConnectionServiceClient(ts.Client(), ts.URL)
	live := towerv1connect.NewLiveTradingServiceClient(ts.Client(), ts.URL)
	ba := towerv1connect.NewBacktestAnalysisServiceClient(ts.Client(), ts.URL)

	// no connection yet: the monitor says so
	gm, err := live.GetMonitor(ctx, connect.NewRequest(&v1.LiveGetMonitorRequest{}))
	if err != nil || gm.Msg.State.Status != "no_connection" || !gm.Msg.State.ReadOnly {
		t.Fatalf("monitor without a connection: %v %v", gm, err)
	}
	if _, err := conns.CreateConnection(ctx, connect.NewRequest(&v1.CreateConnectionRequest{Connection: &v1.Connection{Name: "Fake Exness",
		Settings: exnessConn(livefake.APIKey, livefake.PrivateKey, f.URL, false)}})); err != nil {
		t.Fatal(err)
	}
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	ws, err := live.WatchMonitor(wctx, connect.NewRequest(&v1.LiveWatchMonitorRequest{Instruments: []string{"XAUUSD"}}))
	if err != nil {
		t.Fatal(err)
	}
	var priced, tick bool
	for ws.Receive() && !(priced && tick) {
		if s := ws.Msg().GetState(); s != nil && s.Status == "connected" && len(s.Positions) == 1 && s.Positions[0].Price > 0 {
			priced = true
		}
		tick = tick || ws.Msg().GetTick() != nil
	}
	cancel()
	if !priced || !tick {
		t.Fatalf("monitor stream: priced position %v, tick %v (%v)", priced, tick, ws.Err())
	}

	ap, err := live.ListApproaches(ctx, connect.NewRequest(&v1.LiveListApproachesRequest{}))
	if err != nil || len(ap.Msg.Approaches) < 10 {
		t.Fatalf("approaches: %v %v", ap, err)
	}
	if _, err := live.CreatePaperSession(ctx, connect.NewRequest(&v1.LiveCreatePaperSessionRequest{Settings: &v1.LivePaperSettings{
		Approach: "moving_average", Instrument: "XAUUSD", Params: map[string]float64{"nope": 1}}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("an unknown param: %v", err)
	}
	cr, err := live.CreatePaperSession(ctx, connect.NewRequest(&v1.LiveCreatePaperSessionRequest{Settings: &v1.LivePaperSettings{
		Approach: "moving_average", Instrument: "XAUUSD", Balance: 10000, Risk: 1, Leverage: 2000, Params: map[string]float64{"fast": 20}}}))
	if err != nil {
		t.Fatal(err)
	}
	id := cr.Msg.Session.Id
	if s := cr.Msg.Session; s.Status != "stopped" || !s.Paper || s.ReportPath != "paper:"+itoa(id) || len(s.Settings.Params) != 0 {
		t.Fatalf("created %+v", s)
	}
	// make it one that ran before, from 10 days ago
	p, _ := st.GetPaperSession(id)
	from := time.Now().UTC().AddDate(0, 0, -10).Truncate(time.Hour)
	p.From = &from
	if err := st.SavePaperSession(p); err != nil {
		t.Fatal(err)
	}
	if _, err := live.StartPaperSession(ctx, connect.NewRequest(&v1.LivePaperSessionRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	var s *v1.LivePaperSession
	deadline := time.Now().Add(60 * time.Second)
	for {
		g, err := live.GetPaperSession(ctx, connect.NewRequest(&v1.LivePaperSessionRequest{Id: id}))
		if err != nil {
			t.Fatal(err)
		}
		s = g.Msg.Session
		if s.Status == "waiting" || s.Status == "market_closed" || s.Status == "error" || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if s.Status == "error" || s.Candles == 0 || s.Trades == 0 {
		t.Fatalf("after the catch-up: %s %q, %d candles, %d trades", s.Status, s.Message, s.Candles, s.Trades)
	}
	// Backtest Analysis: the Paper sessions source
	runs, err := ba.ListPaperRuns(ctx, connect.NewRequest(&v1.BacktestListPaperRunsRequest{}))
	if err != nil || len(runs.Msg.Runs) != 1 || runs.Msg.Runs[0].Path != s.ReportPath || !runs.Msg.Runs[0].Paper || runs.Msg.Runs[0].Trades != s.Trades {
		t.Fatalf("paper runs: %v %v", runs, err)
	}
	run, err := ba.GetRun(ctx, connect.NewRequest(&v1.BacktestGetRunRequest{Path: s.ReportPath}))
	if err != nil || len(run.Msg.Run.Trades) != int(s.Trades) || !run.Msg.Run.Summary.Paper || len(run.Msg.Run.ChartTimeframes) == 0 {
		t.Fatalf("paper run: %v", err)
	}
	cs, err := ba.GetCandles(ctx, connect.NewRequest(&v1.BacktestGetCandlesRequest{Path: s.ReportPath}))
	if err != nil || len(cs.Msg.Candles) != int(s.Candles) {
		t.Fatalf("paper candles: %v (%d vs %d)", err, len(cs.Msg.GetCandles()), s.Candles)
	}
	dec, err := ba.GetDecisions(ctx, connect.NewRequest(&v1.BacktestGetDecisionsRequest{Path: s.ReportPath, Limit: 10}))
	if err != nil || !dec.Msg.Available || len(dec.Msg.Decisions) == 0 {
		t.Fatalf("paper decisions: %v", err)
	}
	// the detail stream's snapshot has the journal
	dctx, dcancel := context.WithTimeout(ctx, 5*time.Second)
	ds, err := live.WatchPaperSession(dctx, connect.NewRequest(&v1.LiveWatchPaperSessionRequest{Id: id}))
	if err != nil || !ds.Receive() {
		t.Fatalf("watch session: %v", err)
	}
	snap := ds.Msg().GetSnapshot()
	dcancel()
	if snap == nil || len(snap.Journal.Trades) != int(s.Trades) || len(snap.Journal.Candles) != int(s.Candles) || len(snap.Journal.Equity) == 0 {
		t.Fatalf("snapshot: %v", snap)
	}
	ex, err := live.ExportPaperSession(ctx, connect.NewRequest(&v1.LiveExportPaperSessionRequest{Id: id}))
	if err != nil || !strings.HasPrefix(ex.Msg.Path, dir) || len(ex.Msg.Files) < 5 {
		t.Fatalf("export: %v %v", ex, err)
	}
	if !backtestanalysis.IsReport(ex.Msg.Path) {
		t.Fatal("the export is not a report folder")
	}
	if _, err := live.StopPaperSession(ctx, connect.NewRequest(&v1.LivePaperSessionRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	if _, err := live.DeletePaperSession(ctx, connect.NewRequest(&v1.LivePaperSessionRequest{Id: id})); err != nil {
		t.Fatal(err)
	}
	if _, err := live.GetPaperSession(ctx, connect.NewRequest(&v1.LivePaperSessionRequest{Id: id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("deleted: %v", err)
	}
	if r := f.Refused(); len(r) > 0 {
		t.Fatalf("trading requests reached the fake: %v", r)
	}
}

func itoa(n uint32) string { return strconv.FormatUint(uint64(n), 10) }
