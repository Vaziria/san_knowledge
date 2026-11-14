package san_backtest

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	exness "github.com/wargasipil/san_exness"
	"github.com/wargasipil/san_exness/exnesstest"
)

// fakeExness is an Exness Access Point serving hand-made candles and one
// instrument's conditions. It does not check signatures (exnesstest does
// that for san_exness itself); it counts requests.
type fakeExness struct {
	mu      sync.Mutex
	srv     *httptest.Server
	cond    exness.InstrumentCondition
	candles map[string][]Candle // tf
	calls   []string
}

func newFakeExness(t *testing.T) *fakeExness {
	f := &fakeExness{candles: map[string][]Candle{}, cond: exness.InstrumentCondition{Instrument: "XAUUSD", BaseCurrency: "XAU", QuoteCurrency: "USD",
		PointDigits: 2, ContractSize: "100", VolumeMin: "0.01", VolumeMax: "200", VolumeStep: "0.01", SwapLong: "-10", SwapShort: "5",
		CommissionSettings: exness.CommissionSettings{CommissionType: "per_lot_per_side", CommissionValue: exness.Money{Amount: "3.50", Currency: "USD"}}}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeExness) client(t *testing.T) *exness.Client {
	c, err := exness.New(exness.Config{APIKey: exnesstest.APIKey, PrivateKey: exnesstest.PrivateKey, AccountID: exnesstest.Account,
		AccessPoint: f.srv.URL, HTTPClient: f.srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (f *fakeExness) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.Contains(c, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeExness) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.URL.Path+"?"+r.URL.RawQuery)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/instruments/"+f.cond.Instrument+"/conditions"):
		json.NewEncoder(w).Encode(f.cond)
	case strings.HasSuffix(r.URL.Path, "/candles"):
		q := r.URL.Query()
		from, _ := time.Parse(time.RFC3339, q.Get("from"))
		n, _ := strconv.Atoi(q.Get("count"))
		if q.Get("price_type") != "both" || n <= 0 || n > 1000 {
			w.WriteHeader(400)
			w.Write([]byte(`{"code":3002,"error_message":"REQUEST_INVALID_PARAMETERS"}`))
			return
		}
		var out []map[string]any
		for _, c := range f.candles[q.Get("timeframe")] {
			if c.OpenTime.Before(from) || len(out) == n {
				continue
			}
			out = append(out, map[string]any{"time": c.OpenTime.Format(time.RFC3339), "volume": c.Volume,
				"bid": map[string]float64{"open": c.Open, "high": c.High, "low": c.Low, "close": c.Close},
				"ask": map[string]float64{"open": c.AskOpen, "high": c.AskHigh, "low": c.AskLow, "close": c.AskClose}})
		}
		if out == nil {
			out = []map[string]any{}
		}
		json.NewEncoder(w).Encode(map[string]any{"candles": out})
	default:
		w.WriteHeader(404)
		w.Write([]byte(`{"code":3001,"error_message":"REQUEST_INVALID"}`))
	}
}

func TestExnessCacheSecondRunNoRequests(t *testing.T) {
	f := newFakeExness(t)
	// 2500 M1 candles: three pages of at most 1000
	var bars []bar
	for i := 0; i < 2500; i++ {
		p := 100 + math.Sin(float64(i)/50)
		bars = append(bars, bar{p, p + 0.3, p - 0.3, p + 0.1})
	}
	f.candles["M1"] = candles("M1", at("2026-10-13T00:00:00Z"), 0.2, bars...)
	cache := t.TempDir()
	src := NewExness(f.client(t))
	cfg := func() Config {
		return Config{Strategy: &script{name: "c", tfs: []string{"M1"}, warm: 10, on: func(*Context) Signal { return Wait }}, Instrument: "XAUUSD",
			Data: src, From: at("2026-10-13T01:00:00Z"), To: at("2026-10-14T12:00:00Z"), Cache: cache}
	}
	res := mustRun(t, cfg())
	first := f.count("/candles")
	if first < 3 || f.count("/conditions") != 1 {
		t.Fatalf("first run: %d candle requests, %d conditions", first, f.count("/conditions"))
	}
	if !res.Range.From.Equal(at("2026-10-13T01:00:00Z")) || !res.Range.To.Equal(at("2026-10-14T12:00:00Z")) {
		t.Errorf("range %s to %s", res.Range.From, res.Range.To)
	}
	if res.Costs[0].Commission != "exness: 3.5 USD per lot per side" {
		t.Errorf("commission report %q", res.Costs[0].Commission)
	}
	before := len(f.calls)
	res2 := mustRun(t, cfg())
	if len(f.calls) != before {
		t.Fatalf("second run made %d requests: %v", len(f.calls)-before, f.calls[before:])
	}
	if len(res2.Equity) != len(res.Equity) {
		t.Errorf("cached run differs: %d vs %d equity points", len(res2.Equity), len(res.Equity))
	}
	if _, err := os.Stat(filepath.Join(cache, "exness", "XAUUSD", "M1", "2026-10.csv")); err != nil {
		t.Errorf("cache file: %v", err)
	}
	// Cache "off" keeps nothing and asks again
	c := cfg()
	c.Cache = "off"
	mustRun(t, c)
	if len(f.calls) == before {
		t.Error("Cache off made no requests")
	}
}

func TestExnessPagingAndForming(t *testing.T) {
	f := newFakeExness(t)
	f.candles["M1"] = candles("M1", at("2026-10-13T00:00:00Z"), 0.2, flats(2100, 100)...)
	src := NewExness(f.client(t))
	cs, err := src.Candles(context.Background(), Query{Instrument: "XAUUSD", Timeframe: "M1", From: at("2026-10-13T00:00:00Z"), To: at("2026-10-13T20:00:00Z")})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1200 || !cs[1199].OpenTime.Equal(at("2026-10-13T19:59:00Z")) || cs[0].AskOpen != 100.2 {
		t.Fatalf("%d candles, last %v", len(cs), cs[len(cs)-1].OpenTime)
	}
	if n := f.count("/candles"); n != 2 {
		t.Errorf("%d requests, want 2 pages", n)
	}
}

type stubAPI struct {
	now   time.Time
	cands []exness.Candle
	fails int
	calls int
}

func (s *stubAPI) Candles(_ context.Context, q exness.CandleQuery) ([]exness.Candle, error) {
	s.calls++
	if s.fails > 0 {
		s.fails--
		return nil, &exness.APIError{Status: 429, Code: exness.CodeRequestRateLimit, RetryAfter: time.Second}
	}
	return s.cands, nil
}
func (s *stubAPI) Instrument(context.Context, string) (exness.InstrumentCondition, error) {
	return exness.InstrumentCondition{}, errors.New("no")
}
func (s *stubAPI) Now() time.Time { return s.now }

func TestExnessDropsFormingAndRetries(t *testing.T) {
	api := &stubAPI{now: at("2026-10-13T10:30:30Z"), fails: 2, cands: []exness.Candle{
		{Time: at("2026-10-13T10:00:00Z"), Open: 1, High: 2, Low: 0.5, Close: 1.5},
		{Time: at("2026-10-13T10:15:00Z"), Bid: &exness.CandleSide{Open: 1, High: 1, Low: 1, Close: 1}, Ask: &exness.CandleSide{Open: 2, High: 2, Low: 2, Close: 2}},
		{Time: at("2026-10-13T10:30:00Z"), Open: 1, High: 1, Low: 1, Close: 1}, // still forming at 10:30:30
	}}
	var waited []time.Duration
	src := &Exness{API: api, Wait: func(_ context.Context, d time.Duration) error { waited = append(waited, d); return nil }}
	cs, err := src.Candles(context.Background(), Query{Instrument: "X", Timeframe: "M15", From: at("2026-10-13T10:00:00Z"), To: at("2026-10-13T11:00:00Z")})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].AskClose != 1.5 || cs[1].AskClose != 2 || !cs[1].CloseTime.Equal(at("2026-10-13T10:30:00Z")) {
		t.Fatalf("%+v", cs)
	}
	if len(waited) != 2 || waited[0] != time.Second {
		t.Errorf("waited %v", waited)
	}
}

func TestCacheTopsUpCurrentMonth(t *testing.T) {
	src := &clockSource{memSource: newMem().add(gold(), "H1", candles("H1", at("2026-10-01T00:00:00Z"), 0.1, flats(24*20, 100)...)),
		now: at("2026-10-10T00:00:00Z")}
	c := NewCache(src, t.TempDir())
	q := Query{Instrument: "XAUUSD", Timeframe: "H1", From: at("2026-10-01T00:00:00Z"), To: at("2026-10-20T00:00:00Z")}
	cs, err := c.Candles(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 24*9 { // closed by Oct 10 00:00 only
		t.Fatalf("%d candles", len(cs))
	}
	src.now = at("2026-10-15T00:00:00Z")
	cs, _ = c.Candles(context.Background(), q)
	if len(cs) != 24*14 {
		t.Fatalf("after top-up %d candles", len(cs))
	}
	last := src.queries[len(src.queries)-1]
	if !last.From.Equal(at("2026-10-09T23:00:00Z")) {
		t.Errorf("top-up asked from %s, want the covered_to", last.From)
	}
	n := len(src.queries)
	c.Candles(context.Background(), Query{Instrument: "XAUUSD", Timeframe: "H1", From: at("2026-10-02T00:00:00Z"), To: at("2026-10-12T00:00:00Z")})
	if len(src.queries) != n {
		t.Error("a covered range was fetched again")
	}
	in1, _ := c.Instrument(context.Background(), "XAUUSD")
	in2, _ := c.Instrument(context.Background(), "XAUUSD")
	if in1.Name != "XAUUSD" || in2.ContractSize != 100 || src.instCalls != 1 {
		t.Errorf("instrument cached: %d calls", src.instCalls)
	}
}

type clockSource struct {
	*memSource
	now       time.Time
	instCalls int
}

func (c *clockSource) CacheKey() string { return "mem" }
func (c *clockSource) Now() time.Time   { return c.now }
func (c *clockSource) Candles(ctx context.Context, q Query) ([]Candle, error) {
	cs, err := c.memSource.Candles(ctx, q)
	var out []Candle
	for _, x := range cs {
		if !x.CloseTime.After(c.now) {
			out = append(out, x)
		}
	}
	return out, err
}
func (c *clockSource) Instrument(ctx context.Context, n string) (Instrument, error) {
	c.instCalls++
	return c.memSource.Instrument(ctx, n)
}

func TestCSVSource(t *testing.T) {
	dir := t.TempDir()
	body := "time,open,high,low,close,volume\n2026-10-13T10:00:00Z,1,2,0.5,1.5,10\n1791886500,1.5,1.6,1.4,1.55,\n"
	os.WriteFile(filepath.Join(dir, "XAUUSD_M15.csv"), []byte(body), 0o644)
	b, _ := json.Marshal(gold())
	os.WriteFile(filepath.Join(dir, "XAUUSD.json"), b, 0o644)
	src := &CSV{Dir: dir}
	cs, err := src.Candles(context.Background(), Query{Instrument: "XAUUSD", Timeframe: "M15", From: at("2026-10-01T00:00:00Z"), To: at("2026-11-01T00:00:00Z")})
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].AskClose != 1.5 || cs[0].Volume != 10 || !cs[1].OpenTime.Equal(at("2026-10-13T10:15:00Z")) {
		t.Fatalf("%+v", cs)
	}
	in, err := src.Instrument(context.Background(), "XAUUSD")
	if err != nil || in.ContractSize != 100 {
		t.Fatalf("%+v %v", in, err)
	}
	os.WriteFile(filepath.Join(dir, "BAD_M15.csv"), []byte("time,open,high,low,close\n2026-10-13T10:00:00Z,1,1,1,x\n"), 0o644)
	if _, err := src.Candles(context.Background(), Query{Instrument: "BAD", Timeframe: "M15", From: at("2026-10-01T00:00:00Z"), To: at("2026-11-01T00:00:00Z")}); err == nil || !strings.Contains(err.Error(), "close") {
		t.Errorf("bad csv: %v", err)
	}
	if _, err := src.Instrument(context.Background(), "NONE"); err == nil {
		t.Error("missing instrument json")
	}
}

func TestFromExness(t *testing.T) {
	in, err := FromExness(exness.InstrumentCondition{Instrument: "XAUUSDm", BaseCurrency: "XAU", QuoteCurrency: "USD", PointDigits: 3, ContractSize: "100",
		VolumeMin: "0.01", VolumeMax: "200", VolumeStep: "0.01", MarginPercent: "50", SwapLong: "-25.5", SwapShort: "4",
		SwapMultipliers:    []exness.SwapMultiplier{{Day: "Friday", Multiplier: 3}, {Day: "nope", Multiplier: 2}},
		TradeSessions:      []exness.InstrumentSession{{TradeMode: "enabled", WeekStartTime: "1 00:05:00", WeekEndTime: "5 21:55:00"}},
		CommissionSettings: exness.CommissionSettings{CommissionType: "per_lot_round_turn", CommissionValue: exness.Money{Amount: "7", Currency: "USD"}}})
	if err != nil {
		t.Fatal(err)
	}
	if in.MarginPercent != 50 || in.SwapLong != -25.5 || in.SwapDays[time.Friday] != 3 || in.SwapDays[time.Wednesday] != 0 || in.Commission != 3.5 || len(in.Notes) != 2 {
		t.Fatalf("%+v", in)
	}
	near(t, "point", in.Point(), 0.001)
	if !in.Open(at("2026-10-13T10:00:00Z")) || in.Open(at("2026-10-17T10:00:00Z")) || in.Open(at("2026-10-16T22:00:00Z")) || in.Open(at("2026-10-12T00:01:00Z")) {
		t.Error("sessions")
	}
	in, _ = FromExness(exness.InstrumentCondition{Instrument: "X", QuoteCurrency: "USD", ContractSize: "1", VolumeMin: "1", VolumeMax: "1", VolumeStep: "1",
		TradeSessions: []exness.InstrumentSession{{WeekStartTime: "whenever", WeekEndTime: "1 00:00"}}})
	if len(in.Sessions) != 0 || in.SwapDays[time.Wednesday] != 3 || !strings.Contains(strings.Join(in.Notes, ";"), "not understood") {
		t.Fatalf("%+v", in)
	}
	if _, err := FromExness(exness.InstrumentCondition{Instrument: "X", ContractSize: "abc"}); err == nil {
		t.Error("bad contract size accepted")
	}
	// a week range wrapping over Sunday 00:00
	w := Instrument{Sessions: []WeekRange{{Start: 6 * 24 * time.Hour, End: 24 * time.Hour}}}
	if !w.Open(at("2026-10-18T05:00:00Z")) || w.Open(at("2026-10-14T05:00:00Z")) {
		t.Error("wrapping session")
	}
}

func TestIndicators(t *testing.T) {
	closes := []float64{44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28, 46.00}
	var bars []bar
	for _, c := range closes {
		bars = append(bars, bar{c, c + 0.5, c - 0.5, c})
	}
	cs := candles("H1", at("2026-10-13T00:00:00Z"), 0, bars...)
	s := newSeries("H1", cs)
	c := &Context{step: "H1", frames: map[string]*series{"H1": s}}
	s.n = 2
	if !math.IsNaN(c.SMA("H1", 3)) || !math.IsNaN(c.EMA("H1", 3)) {
		t.Error("indicators before enough candles are not NaN")
	}
	s.n = 3
	near(t, "SMA3", c.SMA("H1", 3), (44.34+44.09+44.15)/3)
	near(t, "EMA3 seed", c.EMA("H1", 3), (44.34+44.09+44.15)/3)
	s.n = 4
	near(t, "EMA3", c.EMA("H1", 3), 43.61*0.5+(44.34+44.09+44.15)/3*0.5)
	// RSI(14) after 15 closes, Wilder's first average
	s.n = 15
	var g, l float64
	for i := 1; i < 15; i++ {
		d := closes[i] - closes[i-1]
		if d > 0 {
			g += d
		} else {
			l -= d
		}
	}
	near(t, "RSI14", c.RSI("H1", 14), 100-100/(1+g/l))
	// ATR(3): Wilder's average of the true ranges, by hand
	tr := func(i int) float64 {
		h, l := closes[i]+0.5, closes[i]-0.5
		if i == 0 {
			return h - l
		}
		return math.Max(h-l, math.Max(math.Abs(h-closes[i-1]), math.Abs(l-closes[i-1])))
	}
	want := (tr(0) + tr(1) + tr(2)) / 3
	for i := 3; i < 15; i++ {
		want = (want*2 + tr(i)) / 3
	}
	near(t, "ATR3", c.ATR("H1", 3), want)
	mid, up, lo := c.Bollinger("H1", 2, 2)
	sd := math.Abs(closes[14]-closes[13]) / 2
	near(t, "BB mid", mid, (closes[13]+closes[14])/2)
	near(t, "BB up", up, mid+2*sd)
	near(t, "BB lo", lo, mid-2*sd)
	line, sig, hist := c.MACD("H1", 3, 6, 3)
	if math.IsNaN(line) || math.IsNaN(sig) || math.Abs(hist-(line-sig)) > 1e-12 {
		t.Errorf("MACD %v %v %v", line, sig, hist)
	}
	// swing high at index 9 (46.08, high 46.58) needs 2 candles after it
	s.n = 11
	if len(c.SwingHighs("H1", 2)) != 0 {
		t.Error("swing high seen before it was confirmed")
	}
	s.n = 12
	hs := c.SwingHighs("H1", 2)
	if len(hs) != 1 || hs[0].Price != 46.58 || !hs[0].Confirmed.Equal(cs[11].CloseTime) {
		t.Fatalf("swing highs %+v", hs)
	}
	if lows := c.SwingLows("H1", 2); len(lows) != 1 || lows[0].Price != 43.11 {
		t.Fatalf("swing lows %+v", lows)
	}
}

func TestWalkForwardWindows(t *testing.T) {
	var bars []bar
	for i := 0; i < 24*31; i++ {
		p := 100 + 3*math.Sin(float64(i)/9)
		bars = append(bars, bar{p, p + 0.4, p - 0.4, p + 0.1})
	}
	src := newMem().add(gold(), "H1", candles("H1", at("2026-09-30T00:00:00Z"), 0.05, bars...))
	st := &tunable{script{name: "every", tfs: []string{"H1"}, warm: 2, params: map[string]float64{"n": 4}}}
	st.on = func(c *Context) Signal {
		n := int(c.Param("n"))
		if c.Position() == nil && len(c.Candles("H1"))%n == 0 {
			return Buy(c.Close("H1")-1, c.Close("H1")+1)
		}
		return Wait
	}
	res := mustRun(t, Config{Strategy: st, Instrument: "XAUUSD", Data: src, From: at("2026-10-01T00:00:00Z"), To: at("2026-10-31T00:00:00Z"),
		Walk: &Walk{Train: 10 * 24 * time.Hour, Test: 5 * 24 * time.Hour}})
	w := res.Walk
	if w == nil || len(w.Windows) != 4 {
		t.Fatalf("walk %+v", w)
	}
	for i, win := range w.Windows {
		s := at("2026-10-01T00:00:00Z").Add(time.Duration(i) * 5 * 24 * time.Hour)
		if !win.TrainFrom.Equal(s) || !win.TestFrom.Equal(s.Add(10*24*time.Hour)) || !win.TestTo.Equal(s.Add(15*24*time.Hour)) || win.Trials != 5 {
			t.Errorf("window %d %+v", i, win)
		}
		if n := win.Params["n"]; n != 2 && n != 3 && n != 4 && n != 5 && n != 6 {
			t.Errorf("window %d picked n=%v", i, n)
		}
		for _, tr := range w.Trades {
			if tr.OpenTime.Before(w.Windows[0].TestFrom) {
				t.Errorf("out-of-sample trade opened in the first train window: %s", tr.OpenTime)
			}
		}
	}
	var trades int
	for _, win := range w.Windows {
		trades += win.Test.Trades
	}
	if w.OutOfSample.Trades != trades || len(w.Trades) != trades {
		t.Errorf("out of sample %d trades, windows %d", w.OutOfSample.Trades, trades)
	}
	if st.starts < 1+4*6 {
		t.Errorf("OnStart ran %d times", st.starts)
	}
	if got := grid(14); len(got) != 5 || got[0] != 7 || got[4] != 21 {
		t.Errorf("grid(14) %v", got)
	}
	if got := grid(1); len(got) != 2 {
		t.Errorf("grid(1) %v", got)
	}
	if got := grid(0.5); len(got) != 5 || got[0] != 0.25 {
		t.Errorf("grid(0.5) %v", got)
	}
}

func TestSpanZoneSession(t *testing.T) {
	for in, want := range map[string]time.Duration{"90d": 90 * 24 * time.Hour, "3m": 90 * 24 * time.Hour, "2w": 14 * 24 * time.Hour, "1y": 365 * 24 * time.Hour, "12h": 12 * time.Hour} {
		if got, err := ParseSpan(in); err != nil || got != want {
			t.Errorf("ParseSpan(%q) = %v %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "d", "0d", "5x", "-1d"} {
		if _, err := ParseSpan(bad); err == nil {
			t.Errorf("ParseSpan(%q) accepted", bad)
		}
	}
	if z, _ := ParseZone("wib"); z != WIB {
		t.Error("WIB")
	}
	if _, err := ParseZone("Mars/Base"); err == nil {
		t.Error("bad zone accepted")
	}
	// 13:30 UTC on a summer weekday: London and New York overlap
	s := SessionAt(at("2026-07-14T13:30:00Z"))
	if !s.London || !s.NewYork || s.Asia || !s.Overlap() || s.String() != "London+New York" {
		t.Errorf("%+v", s)
	}
	if s := SessionAt(at("2026-07-14T02:00:00Z")); !s.Asia || s.London || s.String() != "Asia" {
		t.Errorf("%+v", s)
	}
	from, to := Last(48 * time.Hour)
	if to.Sub(from) != 48*time.Hour || to.Location() != time.UTC {
		t.Errorf("Last %s %s", from, to)
	}
	if b := bucket("MN", at("2026-01-01T00:00:00Z"), at("2026-10-13T10:00:00Z")); !b.Equal(at("2026-10-01T00:00:00Z")) {
		t.Errorf("MN bucket %s", b)
	}
	if b := bucket("W1", at("2026-01-04T00:00:00Z"), at("2026-10-14T10:00:00Z")); !b.Equal(at("2026-10-11T00:00:00Z")) {
		t.Errorf("W1 bucket %s", b)
	}
}
