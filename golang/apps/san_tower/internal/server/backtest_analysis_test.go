package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/settings"
)

// analysisReport writes a small San Backtest report (run.json format).
func analysisReport(t *testing.T, dir, approach string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"run.json": `{"schema": 1, "kind": "run", "approach": "` + approach + `", "instrument": "XAUUSD", "timeframes": ["M15"],
			"range": {"from": "2026-10-13T00:00:00Z", "to": "2026-10-13T01:00:00Z"},
			"stats": {"trades": 1, "wins": 1, "win_rate": 100, "profit_factor": "inf", "net_profit": 10, "start_balance": 1000}}`,
		"trades.csv": "leg,strategy,instrument,side,signal_utc,open_utc,open_wib,close_utc,close_wib,entry,exit,stop,final_stop,target,lots,spread,risk,gross,commission,swap,net,r,reason\n" +
			"0," + approach + ",XAUUSD,buy,2026-10-13T00:15:00Z,2026-10-13T00:15:00Z,x,2026-10-13T00:45:00Z,x,100,101,99,99,101,0.1,0.1,10,10,0,0,10,1,target\n",
		"candles.csv": "instrument,timeframe,time_utc,open,high,low,close,volume,spread\n" +
			"XAUUSD,M15,2026-10-13T00:00:00Z,100,101,99,100.5,1,0.1\nXAUUSD,M15,2026-10-13T00:15:00Z,100.5,102,100,101,1,0.1\n",
		"decisions.jsonl": `{"time":"2026-10-13T00:15:00Z","leg":0,"kind":"entry","side":"buy","details":{"price":100}}` + "\n",
		"summary.txt":     approach + "\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func connectCode(err error) connect.Code {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return 0
}

func TestBacktestAnalysisService(t *testing.T) {
	ts, data := towerDir(t)
	c := towerv1connect.NewBacktestAnalysisServiceClient(ts.Client(), ts.URL)
	ctx := context.Background()
	root := t.TempDir()
	analysisReport(t, filepath.Join(root, "a"), "alpha")
	analysisReport(t, filepath.Join(root, "sub", "b"), "beta")

	pick, err := c.ListFolders(ctx, connect.NewRequest(&v1.BacktestListFoldersRequest{Path: root}))
	if err != nil {
		t.Fatal(err)
	}
	if len(pick.Msg.Folders) != 2 || !pick.Msg.Folders[0].Report || pick.Msg.Folders[1].Report {
		t.Errorf("picker %+v", pick.Msg.Folders)
	}
	open, err := c.OpenFolder(ctx, connect.NewRequest(&v1.BacktestOpenFolderRequest{Path: root}))
	if err != nil {
		t.Fatal(err)
	}
	if len(open.Msg.Runs) != 2 || open.Msg.Runs[0].Approach != "alpha" || open.Msg.Runs[1].RelPath != "sub/b" {
		t.Fatalf("open %+v", open.Msg.Runs)
	}
	// recent folders live in the tower's settings, newest first
	other := t.TempDir()
	if _, err := c.OpenFolder(ctx, connect.NewRequest(&v1.BacktestOpenFolderRequest{Path: other})); err != nil {
		t.Fatal(err)
	}
	if _, err := c.OpenFolder(ctx, connect.NewRequest(&v1.BacktestOpenFolderRequest{Path: root})); err != nil {
		t.Fatal(err)
	}
	recent, _ := c.ListRecentFolders(ctx, connect.NewRequest(&v1.BacktestListRecentFoldersRequest{}))
	if len(recent.Msg.Folders) != 2 || recent.Msg.Folders[0] != root || recent.Msg.Folders[1] != other {
		t.Errorf("recent %v", recent.Msg.Folders)
	}
	st, _ := settings.Load(data)
	if st.BacktestAnalysis == nil || len(st.BacktestAnalysis.Recent) != 2 {
		t.Errorf("tower.json %+v", st.BacktestAnalysis)
	}
	if _, err := os.Stat(filepath.Join(root, "tower.json")); err == nil {
		t.Error("wrote into the report folder")
	}
	f, _ := c.ForgetRecentFolder(ctx, connect.NewRequest(&v1.BacktestForgetRecentFolderRequest{Path: other}))
	if len(f.Msg.Folders) != 1 {
		t.Errorf("forget %v", f.Msg.Folders)
	}

	run, err := c.GetRun(ctx, connect.NewRequest(&v1.BacktestGetRunRequest{Path: filepath.Join(root, "a")}))
	if err != nil {
		t.Fatal(err)
	}
	if run.Msg.Run.Summary.Approach != "alpha" || len(run.Msg.Run.Trades) != 1 || run.Msg.Run.Trades[0].Session == "" {
		t.Errorf("run %+v", run.Msg.Run)
	}
	cs, err := c.GetCandles(ctx, connect.NewRequest(&v1.BacktestGetCandlesRequest{Path: filepath.Join(root, "a"), Timeframe: "h1"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.Msg.Candles) != 1 || cs.Msg.Candles[0].High != 102 || cs.Msg.StepTimeframe != "M15" {
		t.Errorf("candles %+v", cs.Msg)
	}
	ds, err := c.GetDecisions(ctx, connect.NewRequest(&v1.BacktestGetDecisionsRequest{Path: filepath.Join(root, "a")}))
	if err != nil || len(ds.Msg.Decisions) != 1 || !ds.Msg.Available {
		t.Errorf("decisions %v %+v", err, ds)
	}

	for _, tc := range []struct {
		path string
		code connect.Code
	}{{"", connect.CodeInvalidArgument}, {filepath.Join(root, "nope"), connect.CodeNotFound}, {filepath.Join(root, "sub"), connect.CodeInvalidArgument}} {
		_, err := c.GetRun(ctx, connect.NewRequest(&v1.BacktestGetRunRequest{Path: tc.path}))
		if connectCode(err) != tc.code {
			t.Errorf("GetRun(%q): %v, want %v", tc.path, err, tc.code)
		}
	}
	if _, err := c.OpenFolder(ctx, connect.NewRequest(&v1.BacktestOpenFolderRequest{Path: filepath.Join(root, "nope")})); connectCode(err) != connect.CodeNotFound {
		t.Errorf("open missing: %v", err)
	}
}

// A caller that is not on this computer is refused (a tower with --lan).
func TestBacktestAnalysisRefusesRemote(t *testing.T) {
	a := analysisService{&Server{}}
	_, err := a.OpenFolder(context.Background(), connect.NewRequest(&v1.BacktestOpenFolderRequest{Path: t.TempDir()}))
	if connectCode(err) != connect.CodePermissionDenied {
		t.Errorf("open: %v", err)
	}
	_, err = a.ListFolders(context.Background(), connect.NewRequest(&v1.BacktestListFoldersRequest{}))
	if connectCode(err) != connect.CodePermissionDenied {
		t.Errorf("list: %v", err)
	}
	for addr, local := range map[string]bool{"127.0.0.1:5000": true, "[::1]:80": true, "192.168.1.5:5000": false, "": false} {
		if fromThisComputer(connect.Peer{Addr: addr}) != local {
			t.Errorf("%q local = %v", addr, !local)
		}
	}
}

func TestWatchFolder(t *testing.T) {
	old := analysisPoll
	analysisPoll = 50 * time.Millisecond
	t.Cleanup(func() { analysisPoll = old })
	ts, _ := towerDir(t)
	c := towerv1connect.NewBacktestAnalysisServiceClient(ts.Client(), ts.URL)
	root := t.TempDir()
	analysisReport(t, filepath.Join(root, "a"), "alpha")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := c.WatchFolder(ctx, connect.NewRequest(&v1.BacktestWatchFolderRequest{Path: root}))
	if err != nil {
		t.Fatal(err)
	}
	next := func() *v1.BacktestWatchFolderResponse {
		t.Helper()
		for stream.Receive() {
			if !stream.Msg().GetHeartbeat() {
				return stream.Msg()
			}
		}
		t.Fatalf("stream ended: %v", stream.Err())
		return nil
	}
	if s := next().GetSnapshot(); s == nil || len(s.Runs) != 1 {
		t.Fatalf("snapshot %+v", s)
	}
	analysisReport(t, filepath.Join(root, "b"), "beta")
	if ch := next().GetChanged(); ch == nil || ch.Approach != "beta" {
		t.Fatalf("changed %+v", ch)
	}
	os.RemoveAll(filepath.Join(root, "a"))
	if rm := next().GetRemoved(); rm != filepath.Join(root, "a") {
		t.Fatalf("removed %q", rm)
	}
	stream.Close()
}
