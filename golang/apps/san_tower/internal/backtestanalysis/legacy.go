package backtestanalysis

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// legacy is what an older report's summary.txt says (reports written
// before run.json; San Backtest's Summary() text).
type legacy struct {
	name     string
	from, to time.Time
	params   map[string]float64
	costs    []string // the instrument named on each costs line
	notes    []string
	stats    statsJSON
	oos      *statsJSON // walk-forward out-of-sample, when the summary has it
}

var (
	reRange  = regexp.MustCompile(`^range\s+(\d{4}-\d\d-\d\d \d\d:\d\d) UTC .* to (\d{4}-\d\d-\d\d \d\d:\d\d) UTC`)
	reTrades = regexp.MustCompile(`^trades\s+(\d+) \((\d+) won, (\d+) lost\)`)
	reNet    = regexp.MustCompile(`^net profit\s+(-?[\d.]+) \((-?[\d.]+)%\), commission (-?[\d.]+), swap (-?[\d.]+)`)
	reBal    = regexp.MustCompile(`^balance\s+(-?[\d.]+) -> (-?[\d.]+)`)
	reDD     = regexp.MustCompile(`^max drawdown\s+(-?[\d.]+) \((-?[\d.]+)%\)`)
	reHalf   = regexp.MustCompile(`^(first|second) half\s+(\d+) trades, (-?[\d.]+)% won, net (-?[\d.]+), avg R (-?[\d.]+)`)
	reCosts  = regexp.MustCompile(`^costs ([^:]+):`)
	reOOS    = regexp.MustCompile(`^out of sample\s+(\d+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s*$`)
	reOOS2   = regexp.MustCompile(`^out of sample: profit factor (\S+), average R (-?[\d.]+), max drawdown (-?[\d.]+)%`)
	reSkip   = regexp.MustCompile(`([a-z ]+?) (\d+)(?:, |$)`)
)

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func u32(s string) uint32 {
	n, _ := strconv.ParseUint(s, 10, 32)
	return uint32(n)
}

func pfText(s string) pfJSON {
	if s == "inf" {
		return pfJSON(math.Inf(1))
	}
	return pfJSON(num(s))
}

// field is the value after a summary line's label ("win rate   28.9%").
func field(line, label string) (string, bool) {
	if !strings.HasPrefix(line, label) {
		return "", false
	}
	rest := line[len(label):]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	return strings.TrimSpace(rest), true
}

func readLegacy(dir string) (*legacy, error) {
	f, err := os.Open(filepath.Join(dir, SummaryFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	l := &legacy{params: map[string]float64{}, stats: statsJSON{Skipped: map[string]uint32{}}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	first := true
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r ")
		if first {
			l.name, first = strings.TrimSpace(line), false
			continue
		}
		st := &l.stats
		if m := reRange.FindStringSubmatch(line); m != nil {
			l.from, _ = time.Parse("2006-01-02 15:04", m[1])
			l.to, _ = time.Parse("2006-01-02 15:04", m[2])
			continue
		}
		if m := reCosts.FindStringSubmatch(line); m != nil {
			l.costs = append(l.costs, strings.TrimSpace(m[1]))
			continue
		}
		if v, ok := field(line, "note"); ok {
			l.notes = append(l.notes, v)
			continue
		}
		if v, ok := field(line, "params"); ok {
			for _, kv := range strings.Fields(v) {
				if k, x, ok := strings.Cut(kv, "="); ok {
					l.params[k] = num(x)
				}
			}
			continue
		}
		if m := reTrades.FindStringSubmatch(line); m != nil {
			st.Trades, st.Wins, st.Losses = u32(m[1]), u32(m[2]), u32(m[3])
			continue
		}
		if v, ok := field(line, "win rate"); ok {
			st.WinRate = num(strings.TrimSuffix(v, "%"))
			continue
		}
		if v, ok := field(line, "profit factor"); ok {
			st.ProfitFactor = pfText(v)
			continue
		}
		if v, ok := field(line, "average R"); ok {
			st.AvgR = num(v)
			continue
		}
		if m := reNet.FindStringSubmatch(line); m != nil {
			st.NetProfit, st.ReturnPct, st.Commission, st.Swap = num(m[1]), num(m[2]), num(m[3]), num(m[4])
			continue
		}
		if m := reBal.FindStringSubmatch(line); m != nil {
			st.StartBalance, st.EndBalance = num(m[1]), num(m[2])
			continue
		}
		if m := reDD.FindStringSubmatch(line); m != nil {
			st.MaxDrawdown, st.MaxDrawdownPct = num(m[1]), num(m[2])
			continue
		}
		if v, ok := field(line, "losing streak"); ok {
			st.LongestLosingStreak = u32(v)
			continue
		}
		if m := reHalf.FindStringSubmatch(line); m != nil {
			h := halfJSON{Trades: u32(m[2]), WinRate: num(m[3]), NetProfit: num(m[4]), AvgR: num(m[5])}
			if m[1] == "first" {
				st.FirstHalf = h
			} else {
				st.SecondHalf = h
			}
			continue
		}
		if v, ok := field(line, "skipped"); ok && v != "none" {
			for _, m := range reSkip.FindAllStringSubmatch(v, -1) {
				st.Skipped[strings.TrimSpace(m[1])] = u32(m[2])
			}
			continue
		}
		if m := reOOS.FindStringSubmatch(line); m != nil {
			if l.oos == nil {
				l.oos = &statsJSON{}
			}
			l.oos.Trades, l.oos.NetProfit, l.oos.WinRate = u32(m[1]), num(m[2]), num(m[3])
			continue
		}
		if m := reOOS2.FindStringSubmatch(line); m != nil {
			if l.oos == nil {
				l.oos = &statsJSON{}
			}
			l.oos.ProfitFactor, l.oos.AvgR, l.oos.MaxDrawdownPct = pfText(m[1]), num(m[2]), num(m[3])
		}
	}
	return l, sc.Err()
}

// legacySummary fills s from an older report's summary.txt and trades.csv.
func legacySummary(dir string, s *v1.BacktestRunSummary) error {
	s.Kind = "run"
	l, err := readLegacy(dir)
	if err != nil {
		return err
	}
	s.Approach = l.name
	if !l.from.IsZero() {
		s.From, s.To = timestamppb.New(l.from), timestamppb.New(l.to)
	}
	st := l.stats
	s.Trades, s.WinRate, s.ProfitFactor, s.NetProfit, s.ReturnPct, s.MaxDrawdownPct, s.StartBalance = st.Trades, st.WinRate, float64(st.ProfitFactor),
		st.NetProfit, st.ReturnPct, st.MaxDrawdownPct, st.StartBalance
	s.Legs = uint32(max(1, len(l.costs)))
	if strings.Contains(l.name, " + ") {
		s.Kind = "portfolio"
	}
	var names []string
	for _, c := range l.costs {
		if !contains(names, c) {
			names = append(names, c)
		}
	}
	s.Instrument = strings.Join(names, ", ")
	if s.Instrument == "" {
		s.Instrument = firstTradeInstrument(dir)
	}
	return nil
}
