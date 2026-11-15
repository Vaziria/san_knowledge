package backtestanalysis

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // session hours follow Tokyo, London and New York time on any machine

	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// DefaultEquityPoints is how many equity points GetRun sends at most.
const DefaultEquityPoints = 4000

// csvRows reads a CSV file as maps by header; a missing file is no rows.
func csvRows(dir, name string, each func(i int, row map[string]string)) error {
	f, err := os.Open(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.ReuseRecord = true
	head, err := r.Read()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	head = append([]string{}, head...)
	row := map[string]string{}
	for i := 0; ; i++ {
		rec, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		clear(row)
		for k, h := range head {
			if k < len(rec) {
				row[h] = rec[k]
			}
		}
		each(i, row)
	}
}

func rfc(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func tp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func firstTradeInstrument(dir string) string {
	var inst string
	csvRows(dir, TradesFile, func(i int, row map[string]string) {
		if inst == "" {
			inst = row["instrument"]
		}
	})
	return inst
}

var (
	tokyo, _   = time.LoadLocation("Asia/Tokyo")
	london, _  = time.LoadLocation("Europe/London")
	newYork, _ = time.LoadLocation("America/New_York")
)

// Session names the market sessions open at t, as San Backtest's
// Session(): Tokyo 09-18, London 08-17 and New York 08-17 local time, on
// weekdays; "Off" when none is.
func Session(t time.Time) string {
	in := func(loc *time.Location, from, to int) bool {
		l := t.In(loc)
		if l.Weekday() == time.Saturday || l.Weekday() == time.Sunday {
			return false
		}
		return l.Hour() >= from && l.Hour() < to
	}
	var out []string
	if in(tokyo, 9, 18) {
		out = append(out, "Asia")
	}
	if in(london, 8, 17) {
		out = append(out, "London")
	}
	if in(newYork, 8, 17) {
		out = append(out, "New York")
	}
	if len(out) == 0 {
		return "Off"
	}
	return strings.Join(out, " + ")
}

func readTrades(dir string) ([]*v1.BacktestTrade, error) {
	var out []*v1.BacktestTrade
	err := csvRows(dir, TradesFile, func(i int, r map[string]string) {
		open := rfc(r["open_utc"])
		out = append(out, &v1.BacktestTrade{Index: uint32(i), Leg: u32(r["leg"]), Strategy: r["strategy"], Instrument: r["instrument"], Side: r["side"],
			SignalTime: tp(rfc(r["signal_utc"])), OpenTime: tp(open), CloseTime: tp(rfc(r["close_utc"])), Entry: num(r["entry"]), Exit: num(r["exit"]),
			Stop: num(r["stop"]), FinalStop: num(r["final_stop"]), Target: num(r["target"]), Lots: num(r["lots"]), Spread: num(r["spread"]),
			Risk: num(r["risk"]), Gross: num(r["gross"]), Commission: num(r["commission"]), Swap: num(r["swap"]), Net: num(r["net"]), R: num(r["r"]),
			Reason: r["reason"], Session: Session(open), Month: open.UTC().Format("2006-01")})
	})
	return out, err
}

func readMissed(dir string) ([]*v1.BacktestMissed, error) {
	var out []*v1.BacktestMissed
	err := csvRows(dir, MissedFile, func(_ int, r map[string]string) {
		out = append(out, &v1.BacktestMissed{Time: tp(rfc(r["time_utc"])), Leg: u32(r["leg"]), Strategy: r["strategy"], Instrument: r["instrument"],
			Side: r["side"], Reason: r["reason"]})
	})
	return out, err
}

func readEquity(dir string) ([]*v1.BacktestEquityPoint, error) {
	var out []*v1.BacktestEquityPoint
	peak := math.Inf(-1)
	err := csvRows(dir, EquityFile, func(_ int, r map[string]string) {
		e := num(r["equity"])
		peak = math.Max(peak, e)
		dd := 0.0
		if peak > 0 {
			dd = 100 * (peak - e) / peak
		}
		out = append(out, &v1.BacktestEquityPoint{Time: tp(rfc(r["time_utc"])), Balance: num(r["balance"]), Equity: e, DrawdownPct: dd})
	})
	return out, err
}

// thin keeps at most n points of a curve: the first, the last, and per
// stretch its lowest and highest equity, in time order, so the deepest
// drawdowns survive.
func thin(pts []*v1.BacktestEquityPoint, n int) []*v1.BacktestEquityPoint {
	if n < 4 || len(pts) <= n {
		return pts
	}
	buckets := n / 2
	out := make([]*v1.BacktestEquityPoint, 0, n+2)
	out = append(out, pts[0])
	size := float64(len(pts)) / float64(buckets)
	for b := 0; b < buckets; b++ {
		lo, hi := int(float64(b)*size), int(float64(b+1)*size)
		if hi > len(pts) {
			hi = len(pts)
		}
		if lo >= hi {
			continue
		}
		mn, mx := lo, lo
		for i := lo; i < hi; i++ {
			if pts[i].Equity < pts[mn].Equity {
				mn = i
			}
			if pts[i].Equity > pts[mx].Equity {
				mx = i
			}
		}
		a, c := min(mn, mx), max(mn, mx)
		for _, i := range []int{a, c} {
			if i > 0 && out[len(out)-1] != pts[i] {
				out = append(out, pts[i])
			}
		}
	}
	if last := pts[len(pts)-1]; out[len(out)-1] != last {
		out = append(out, last)
	}
	return out
}

func readWalkCSV(dir string) ([]*v1.BacktestWalkWindow, error) {
	var out []*v1.BacktestWalkWindow
	err := csvRows(dir, WalkFile, func(_ int, r map[string]string) {
		out = append(out, &v1.BacktestWalkWindow{TrainFrom: tp(rfc(r["train_from"])), TrainTo: tp(rfc(r["train_to"])), TestFrom: tp(rfc(r["test_from"])),
			TestTo: tp(rfc(r["test_to"])), Params: r["params"], Trials: u32(r["trials"]), TrainTrades: u32(r["train_trades"]), TrainNet: num(r["train_net"]),
			TestTrades: u32(r["test_trades"]), TestNet: num(r["test_net"]), TestWinRate: num(r["test_win_rate"])})
	})
	return out, err
}

func paramsText(m map[string]float64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var p []string
	for _, k := range keys {
		p = append(p, k+"="+strconv.FormatFloat(m[k], 'g', -1, 64))
	}
	return strings.Join(p, " ")
}

// Run reads a whole report. equityPoints caps the equity curve (0:
// DefaultEquityPoints).
func Run(dir string, equityPoints int) (*v1.BacktestRun, error) {
	sum, err := Summary(dir)
	if err != nil {
		return nil, err
	}
	run := &v1.BacktestRun{Summary: sum, Params: map[string]float64{}}
	if b, err := os.ReadFile(filepath.Join(dir, SummaryFile)); err == nil {
		run.SummaryText = string(b)
	}
	if sum.Format == FormatRun {
		r, err := readRunJSON(dir)
		if err != nil {
			return nil, err
		}
		fillRun(run, r)
	} else if l, err := readLegacy(dir); err == nil {
		run.Params, run.Stats, run.Notes = l.params, l.stats.proto(), l.notes
		if l.oos != nil {
			run.OutOfSample = l.oos.proto()
		}
		for i, c := range l.costs {
			run.Costs = append(run.Costs, &v1.BacktestCost{Leg: uint32(i), Instrument: c})
		}
		run.Account = &v1.BacktestAccount{Balance: l.stats.StartBalance}
	}
	if run.Trades, err = readTrades(dir); err != nil {
		return nil, err
	}
	if run.Missed, err = readMissed(dir); err != nil {
		return nil, err
	}
	eq, err := readEquity(dir)
	if err != nil {
		return nil, err
	}
	if equityPoints <= 0 {
		equityPoints = DefaultEquityPoints
	}
	run.EquityTotal, run.Equity = uint32(len(eq)), thin(eq, equityPoints)
	if len(run.Walk) == 0 {
		if run.Walk, err = readWalkCSV(dir); err != nil {
			return nil, err
		}
	}
	if sum.HasCandles {
		if step := stepTimeframe(dir); step != "" {
			run.ChartTimeframes = chartTimeframes(step)
		}
	}
	return run, nil
}

func fillRun(run *v1.BacktestRun, r *runJSON) {
	run.RequestedFrom, run.RequestedTo, run.Notes = ts(r.Requested.From), ts(r.Requested.To), r.Range.Notes
	a := r.Account
	run.Account = &v1.BacktestAccount{Balance: a.Balance, Currency: a.Currency, Leverage: a.Leverage, MaxOpen: a.MaxOpen, MaxOpenRisk: a.MaxOpenRisk,
		MaxDailyLoss: a.MaxDailyLoss, DayTz: a.DayTZ}
	for _, c := range r.Costs {
		run.Costs = append(run.Costs, &v1.BacktestCost{Leg: c.Leg, Instrument: c.Instrument, Spread: c.Spread, Commission: c.Commission, Swap: c.Swap,
			Slippage: c.Slippage, Overridden: c.Overridden, Notes: c.Notes})
	}
	for _, l := range r.Legs {
		leg := &v1.BacktestLeg{Leg: l.Leg, Strategy: l.Strategy, Instrument: l.Instrument, Timeframes: l.Timeframes, Risk: l.Risk, Params: l.Params,
			Contribution: l.Contribution}
		if l.Stats != nil {
			leg.Stats = l.Stats.proto()
		}
		run.Legs = append(run.Legs, leg)
	}
	if r.Params != nil {
		run.Params = r.Params
	}
	run.Stats = r.Stats.proto()
	if w := r.Walk; w != nil {
		run.WalkTrain, run.WalkTest, run.OutOfSample = w.Train, w.Test, w.OutOfSample.proto()
		for _, x := range w.Windows {
			run.Walk = append(run.Walk, &v1.BacktestWalkWindow{TrainFrom: ts(x.TrainFrom), TrainTo: ts(x.TrainTo), TestFrom: ts(x.TestFrom), TestTo: ts(x.TestTo),
				Params: paramsText(x.Params), Trials: x.Trials, TrainTrades: x.Train.Trades, TrainNet: x.Train.NetProfit, TestTrades: x.Test.Trades,
				TestNet: x.Test.NetProfit, TestWinRate: x.Test.WinRate})
		}
	}
}
