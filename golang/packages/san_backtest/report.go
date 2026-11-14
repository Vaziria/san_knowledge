package san_backtest

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

func money(v float64) string { return fmt.Sprintf("%.2f", v) }

func pf(v float64) string {
	if math.IsInf(v, 1) {
		return "inf"
	}
	return fmt.Sprintf("%.2f", v)
}

func skippedText(m map[string]int) string {
	if len(m) == 0 {
		return "none"
	}
	var p []string
	for _, k := range sortedKeys(m) {
		p = append(p, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(p, ", ")
}

func stamp(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04") + " UTC (" + t.In(WIB).Format("2006-01-02 15:04") + " WIB)"
}

// Summary is the result as text: range, costs, stats, legs and
// walk-forward windows.
func (r *Result) Summary() string {
	var b strings.Builder
	s := r.Stats
	fmt.Fprintf(&b, "%s\n", r.Name)
	fmt.Fprintf(&b, "range       %s to %s\n", stamp(r.Range.From), stamp(r.Range.To))
	for _, n := range r.Range.Notes {
		fmt.Fprintf(&b, "note        %s\n", n)
	}
	for _, c := range r.Costs {
		fmt.Fprintf(&b, "costs %s: spread %s; commission %s; swap %s; slippage %s\n", c.Instrument, c.Spread, c.Commission, c.Swap, c.Slippage)
	}
	if len(r.Params) > 0 && len(r.Params[0]) > 0 {
		fmt.Fprintf(&b, "params      %s\n", paramsText(r.Params[0]))
	}
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "trades\t%d (%d won, %d lost)\n", s.Trades, s.Wins, s.Losses)
	fmt.Fprintf(w, "win rate\t%.1f%%\n", s.WinRate)
	fmt.Fprintf(w, "profit factor\t%s\n", pf(s.ProfitFactor))
	fmt.Fprintf(w, "average R\t%.2f\n", s.AvgR)
	fmt.Fprintf(w, "net profit\t%s (%.2f%%), commission %s, swap %s\n", money(s.NetProfit), s.ReturnPct, money(s.Commission), money(s.Swap))
	fmt.Fprintf(w, "balance\t%s -> %s\n", money(s.StartBalance), money(s.EndBalance))
	fmt.Fprintf(w, "max drawdown\t%s (%.2f%%)\n", money(s.MaxDrawdown), s.MaxDrawdownPct)
	fmt.Fprintf(w, "losing streak\t%d\n", s.LongestLosingStreak)
	fmt.Fprintf(w, "first half\t%d trades, %.1f%% won, net %s, avg R %.2f\n", s.FirstHalf.Trades, s.FirstHalf.WinRate, money(s.FirstHalf.NetProfit), s.FirstHalf.AvgR)
	fmt.Fprintf(w, "second half\t%d trades, %.1f%% won, net %s, avg R %.2f\n", s.SecondHalf.Trades, s.SecondHalf.WinRate, money(s.SecondHalf.NetProfit), s.SecondHalf.AvgR)
	fmt.Fprintf(w, "skipped\t%s\n", skippedText(s.Skipped))
	w.Flush()
	if len(r.Legs) > 1 {
		b.WriteString("\nlegs\n")
		w = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "leg\tstrategy\tinstrument\ttrades\twin %\tnet\tshare %\tskipped")
		for _, l := range r.Legs {
			fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%.1f\t%s\t%.1f\t%s\n", l.Leg, l.Strategy, l.Instrument, l.Stats.Trades, l.Stats.WinRate, money(l.Stats.NetProfit),
				l.Contribution, skippedText(l.Stats.Skipped))
		}
		w.Flush()
	}
	if r.Walk != nil {
		b.WriteString("\nwalk-forward (train tunes Params, test checks the pick)\n")
		w = tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "test from\ttest to\tparams\ttrain net\ttest trades\ttest net\ttest win %")
		for _, win := range r.Walk.Windows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%.1f\n", win.TestFrom.Format("2006-01-02"), win.TestTo.Format("2006-01-02"), paramsText(win.Params),
				money(win.Train.NetProfit), win.Test.Trades, money(win.Test.NetProfit), win.Test.WinRate)
		}
		o := r.Walk.OutOfSample
		fmt.Fprintf(w, "out of sample\t\t\t\t%d\t%s\t%.1f\n", o.Trades, money(o.NetProfit), o.WinRate)
		w.Flush()
		fmt.Fprintf(&b, "out of sample: profit factor %s, average R %.2f, max drawdown %.2f%%\n", pf(o.ProfitFactor), o.AvgR, o.MaxDrawdownPct)
	}
	return b.String()
}

func paramsText(m map[string]float64) string {
	var p []string
	for _, k := range sortedKeys(m) {
		p = append(p, fmt.Sprintf("%s=%g", k, m[k]))
	}
	return strings.Join(p, " ")
}

// Compare is a table of several results side by side (strategies, or the
// legs of a run).
func Compare(rs ...*Result) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "name\ttrades\twin %\tprofit factor\tavg R\tnet\treturn %\tmax DD %\tmax DD\tlosing streak\tskipped\t")
	for _, r := range rs {
		s := r.Stats
		n := 0
		for _, v := range s.Skipped {
			n += v
		}
		fmt.Fprintf(w, "%s\t%d\t%.1f\t%s\t%.2f\t%s\t%.2f\t%.2f\t%s\t%d\t%d\t\n", r.Name, s.Trades, s.WinRate, pf(s.ProfitFactor), s.AvgR, money(s.NetProfit),
			s.ReturnPct, s.MaxDrawdownPct, money(s.MaxDrawdown), s.LongestLosingStreak, n)
	}
	w.Flush()
	return b.String()
}

func fnum(v float64) string { return fmt.Sprintf("%.6g", v) }

// WriteTrades writes the trade list as CSV, times in UTC and WIB.
func (r *Result) WriteTrades(out io.Writer) error {
	w := csv.NewWriter(out)
	w.Write([]string{"leg", "strategy", "instrument", "side", "signal_utc", "open_utc", "open_wib", "close_utc", "close_wib", "entry", "exit", "stop",
		"final_stop", "target", "lots", "spread", "risk", "gross", "commission", "swap", "net", "r", "reason"})
	t := func(x time.Time) string { return x.UTC().Format(time.RFC3339) }
	wib := func(x time.Time) string { return x.In(WIB).Format("2006-01-02T15:04:05") + "+07:00" }
	for _, x := range r.Trades {
		w.Write([]string{fmt.Sprint(x.Leg), x.Strategy, x.Instrument, string(x.Side), t(x.SignalTime), t(x.OpenTime), wib(x.OpenTime), t(x.CloseTime),
			wib(x.CloseTime), fnum(x.Entry), fnum(x.Exit), fnum(x.Stop), fnum(x.FinalStop), fnum(x.Target), fnum(x.Lots), fnum(x.Spread), money(x.Risk),
			money(x.Gross), money(x.Commission), money(x.Swap), money(x.Net), fmt.Sprintf("%.3f", x.R), x.Reason})
	}
	w.Flush()
	return w.Error()
}

// WriteMissed writes the skipped signals as CSV.
func (r *Result) WriteMissed(out io.Writer) error {
	w := csv.NewWriter(out)
	w.Write([]string{"time_utc", "time_wib", "leg", "strategy", "instrument", "side", "reason"})
	for _, m := range r.Missed {
		w.Write([]string{m.Time.UTC().Format(time.RFC3339), m.Time.In(WIB).Format("2006-01-02T15:04:05") + "+07:00", fmt.Sprint(m.Leg), m.Strategy,
			m.Instrument, string(m.Side), m.Reason})
	}
	w.Flush()
	return w.Error()
}

// WriteEquity writes the equity curve as CSV.
func (r *Result) WriteEquity(out io.Writer) error {
	w := csv.NewWriter(out)
	w.Write([]string{"time_utc", "balance", "equity"})
	for _, e := range r.Equity {
		w.Write([]string{e.Time.UTC().Format(time.RFC3339), money(e.Balance), money(e.Equity)})
	}
	w.Flush()
	return w.Error()
}

// WriteWalk writes the walk-forward windows as CSV.
func (r *Result) WriteWalk(out io.Writer) error {
	w := csv.NewWriter(out)
	w.Write([]string{"train_from", "train_to", "test_from", "test_to", "params", "trials", "train_trades", "train_net", "test_trades", "test_net", "test_win_rate"})
	if r.Walk != nil {
		for _, x := range r.Walk.Windows {
			w.Write([]string{x.TrainFrom.Format(time.RFC3339), x.TrainTo.Format(time.RFC3339), x.TestFrom.Format(time.RFC3339), x.TestTo.Format(time.RFC3339),
				paramsText(x.Params), fmt.Sprint(x.Trials), fmt.Sprint(x.Train.Trades), money(x.Train.NetProfit), fmt.Sprint(x.Test.Trades),
				money(x.Test.NetProfit), fmt.Sprintf("%.1f", x.Test.WinRate)})
		}
	}
	w.Flush()
	return w.Error()
}

// Save writes summary.txt, trades.csv, missed.csv, equity.csv and
// equity.png (and walk.csv with walk-forward) into dir. It returns the
// paths written.
func (r *Result) Save(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	files := []struct {
		name  string
		write func(io.Writer) error
	}{
		{"summary.txt", func(w io.Writer) error { _, err := io.WriteString(w, r.Summary()); return err }},
		{"trades.csv", r.WriteTrades},
		{"missed.csv", r.WriteMissed},
		{"equity.csv", r.WriteEquity},
		{"equity.png", r.WritePNG},
	}
	if r.Walk != nil {
		files = append(files, struct {
			name  string
			write func(io.Writer) error
		}{"walk.csv", r.WriteWalk})
	}
	var paths []string
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		fh, err := os.Create(p)
		if err != nil {
			return paths, err
		}
		err = f.write(fh)
		if cerr := fh.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return paths, fmt.Errorf("%s: %w", p, err)
		}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}
