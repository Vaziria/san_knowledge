package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// setup writes a config whose trade and market URLs point at url, and clears
// the key from the environment.
func setup(t *testing.T, url string) string {
	t.Helper()
	t.Setenv("BINANCE_TESTNET_API_KEY", "")
	t.Setenv("BINANCE_TESTNET_API_SECRET", "")
	dir := t.TempDir()
	cfg := fmt.Sprintf("trade_url: %s\nmarket_url: %s\ncandles: 60\ncontext_intervals: [1h, 4h]\n", url, url)
	path := filepath.Join(dir, configFile)
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(context.Background(), args, &out, &errOut)
	return out.String(), err
}

func TestOpenParsesSideAnywhere(t *testing.T) {
	cfg := setup(t, "http://127.0.0.1:1")
	for _, args := range [][]string{
		{"open", "long", "-approach", "trend_pullback", "-usdt", "500", "-sl", "79500", "-reason", "x"},
		{"open", "--setup", "trend_pullback", "--usdt", "500", "--sl", "79500", "--reason", "x", "short"},
	} {
		// Without a key the open is refused after the arguments are checked.
		_, err := runCmd(t, append([]string{"-config", cfg}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "no API key") {
			t.Errorf("%v: err = %v, want the no-key refusal", args, err)
		}
	}
	if _, err := runCmd(t, "-config", cfg, "open", "long", "-usdt", "500", "-sl", "79500"); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Errorf("open without -reason: %v", err)
	}
	out, err := runCmd(t, "-config", cfg, "journal")
	if err != nil || strings.Count(out, "no API key") != 2 {
		t.Errorf("journal should list both refusals:\n%s %v", out, err)
	}
}

func TestUnknownCommand(t *testing.T) {
	cfg := setup(t, "http://127.0.0.1:1")
	if _, err := runCmd(t, "-config", cfg, "buy"); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("err = %v", err)
	}
}

func TestMainnetRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFile)
	os.WriteFile(path, []byte("trade_url: https://fapi.binance.com\n"), 0o644)
	if _, err := runCmd(t, "-config", path, "snapshot"); err == nil || !strings.Contains(err.Error(), "test network") {
		t.Errorf("err = %v", err)
	}
}

func TestSnapshotWithoutKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		switch r.URL.Path {
		case "/fapi/v1/time":
			fmt.Fprintf(w, `{"serverTime":%d}`, now.UnixMilli())
		case "/fapi/v1/klines":
			step := map[string]time.Duration{"15m": 15 * time.Minute, "1h": time.Hour, "4h": 4 * time.Hour}[r.URL.Query().Get("interval")]
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			end := now.Truncate(step).Add(step)
			var rows []string
			for i := 0; i < limit; i++ {
				open := end.Add(-time.Duration(limit-i) * step)
				c := 80000 + float64(i)*10
				rows = append(rows, fmt.Sprintf(`[%d,"%.1f","%.1f","%.1f","%.1f","1.5",%d,"120000.0",10,"1","1","0"]`,
					open.UnixMilli(), c, c+20, c-20, c+5, open.Add(step).UnixMilli()-1))
			}
			fmt.Fprint(w, "["+strings.Join(rows, ",")+"]")
		case "/fapi/v1/exchangeInfo":
			fmt.Fprint(w, `{"symbols":[{"symbol":"BTCUSDT","filters":[{"filterType":"PRICE_FILTER","tickSize":"0.10"},{"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001"},{"filterType":"MIN_NOTIONAL","notional":"100"}]}]}`)
		case "/fapi/v1/premiumIndex":
			fmt.Fprint(w, `{"markPrice":"80600.5","lastFundingRate":"0.0001","nextFundingTime":1791561600000}`)
		default:
			http.Error(w, `{"code":-5000,"msg":"unexpected"}`, 400)
		}
	}))
	defer srv.Close()
	cfg := setup(t, srv.URL)

	out, err := runCmd(t, "-config", cfg, "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# BTCUSDT 15m snapshot", "Trend: up", "mark price 80,600.5", "- 1h: trend", "Unavailable: no API key", "None yet."} {
		if !strings.Contains(out, want) {
			t.Errorf("snapshot is missing %q:\n%s", want, out)
		}
	}

	// 60 candles is too few for EMA200: JSON must still encode (as null).
	out, err = runCmd(t, "-config", cfg, "-json", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Frames []struct {
			EMA200 *float64 `json:"ema200"`
			RSI14  *float64 `json:"rsi14"`
		} `json:"frames"`
	}
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(s.Frames) != 3 || s.Frames[0].EMA200 != nil || s.Frames[0].RSI14 == nil {
		t.Errorf("frames = %+v", s.Frames)
	}
}

func TestCredentialsFromEnvFile(t *testing.T) {
	t.Setenv("BINANCE_TESTNET_API_KEY", "")
	t.Setenv("BINANCE_TESTNET_API_SECRET", "")
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("# testnet\nBINANCE_TESTNET_API_KEY=abc\nexport BINANCE_TESTNET_API_SECRET=\"def\"\n"), 0o600)
	if k, s := credentials(dir); k != "abc" || s != "def" {
		t.Errorf("credentials = %q %q", k, s)
	}
	t.Setenv("BINANCE_TESTNET_API_KEY", "envkey")
	t.Setenv("BINANCE_TESTNET_API_SECRET", "envsecret")
	if k, s := credentials(dir); k != "envkey" || s != "envsecret" {
		t.Errorf("environment should win: %q %q", k, s)
	}
}

func TestKnowledgeCommands(t *testing.T) {
	cfg := setup(t, "http://127.0.0.1:1")
	if _, err := runCmd(t, "-config", cfg, "open", "long", "-approach", "big_trend", "-usdt", "500", "-sl", "79500", "-reason", "x"); err == nil || !strings.Contains(err.Error(), "unknown approach big_trend") {
		t.Errorf("open with an undefined approach: %v", err)
	}
	if _, err := runCmd(t, "-config", cfg, "approach", "-name", "big_trend", "-title", "Big Trend", "-description", "trade with the 1h/4h trend"); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, "-config", cfg, "knowledge", "approaches")
	if err != nil || !strings.Contains(out, "`big_trend`") || !strings.Contains(out, "`trend_pullback` (coded)") {
		t.Errorf("approaches %v:\n%s", err, out)
	}
	out, err = runCmd(t, "-config", cfg, "knowledge", "query", "MATCH (a:Approach) RETURN a.name")
	if err != nil || !strings.Contains(out, `"big_trend"`) {
		t.Errorf("query %v:\n%s", err, out)
	}
	if _, err := runCmd(t, "-config", cfg, "knowledge", "query", "MATCH (a:Approach) DETACH DELETE a"); err == nil {
		t.Error("a write query was accepted")
	}
	if _, err := runCmd(t, "-config", cfg, "note", "-position", "1", "-text", "x"); err == nil || !strings.Contains(err.Error(), "no position #1") {
		t.Errorf("note without positions: %v", err)
	}
	out, err = runCmd(t, "-config", cfg, "knowledge", "portfolio")
	if err != nil || !strings.Contains(out, "Portfolio: 0 closed trades") {
		t.Errorf("portfolio %v:\n%s", err, out)
	}
}

func TestParseInterleaved(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	snap := fs.Bool("snapshot", false, "")
	status := fs.String("status", "", "")
	words, err := parseInterleaved(fs, []string{"position", "3", "-snapshot", "-status", "open"})
	if err != nil || strings.Join(words, " ") != "position 3" || !*snap || *status != "open" {
		t.Errorf("words %v snapshot %v status %q err %v", words, *snap, *status, err)
	}
}

func TestMCPRunArgs(t *testing.T) {
	cfg := setup(t, "http://127.0.0.1:1")
	if _, err := runCmd(t, "-config", cfg, "mcp", "start"); err == nil || !strings.Contains(err.Error(), "mcp run") {
		t.Errorf("mcp start: %v", err)
	}
}
