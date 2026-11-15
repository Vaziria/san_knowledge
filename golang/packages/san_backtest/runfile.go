package san_backtest

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"time"
)

// RunSchema is the version of run.json's layout. A reader that finds a
// higher one may meet fields it does not know. 2 (several positions and
// pending orders): legs[].max_open; trades.csv gained position_id,
// order_id, order_type and tag, missed.csv order_id, order_type and tag,
// both at the end; decisions.jsonl the order_* kinds. Nothing else moved.
const RunSchema = 2

// Run kinds (RunInfo.Kind).
const (
	KindRun       = "run"       // Run: one strategy on one instrument
	KindPortfolio = "portfolio" // RunPortfolio: several legs on one account
)

// RunInfo is what ran, for run.json: the account settings and each leg's
// strategy, instrument, timeframes and params in force. Run and
// RunPortfolio fill it.
type RunInfo struct {
	Kind         string    // KindRun or KindPortfolio
	RanAt        time.Time // when the backtest ran, UTC
	Balance      float64   // starting balance
	Currency     string
	Leverage     int
	MaxOpen      int
	MaxOpenRisk  float64
	MaxDailyLoss float64
	DayTZ        string
	Walk         *Walk // the walk-forward windows' sizes, with Config.Walk
	Legs         []LegInfo
}

// LegInfo is one leg of a run as it ran.
type LegInfo struct {
	Leg        int
	Strategy   string
	Instrument string
	Timeframes []string // step first
	Risk       float64  // percent per trade
	Params     map[string]float64
	Overridden []string // costs not taken from Exness: "spread", "commission", "swap", "slippage"
	MaxOpen    int      // open positions at once on this leg (1 = one at a time)
}

// CandleSet is one leg's step candles over the range that ran.
type CandleSet struct {
	Leg        int
	Instrument string
	Timeframe  string
	Candles    []Candle
}

func overridden(c Costs) []string {
	var out []string
	if c.Spread != nil {
		out = append(out, "spread")
	}
	if c.Commission != nil {
		out = append(out, "commission")
	}
	if c.Swap != nil {
		out = append(out, "swap")
	}
	if c.Slippage > 0 {
		out = append(out, "slippage")
	}
	return out
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// The run.json layout (snake_case, times RFC 3339 UTC).

type statsJSON struct {
	Trades              int            `json:"trades"`
	Wins                int            `json:"wins"`
	Losses              int            `json:"losses"`
	WinRate             float64        `json:"win_rate"`
	ProfitFactor        any            `json:"profit_factor"` // a number, or "inf" with no losing trade
	AvgR                float64        `json:"avg_r"`
	NetProfit           float64        `json:"net_profit"`
	GrossWin            float64        `json:"gross_win"`
	GrossLoss           float64        `json:"gross_loss"`
	Commission          float64        `json:"commission"`
	Swap                float64        `json:"swap"`
	StartBalance        float64        `json:"start_balance"`
	EndBalance          float64        `json:"end_balance"`
	ReturnPct           float64        `json:"return_pct"`
	MaxDrawdown         float64        `json:"max_drawdown"`
	MaxDrawdownPct      float64        `json:"max_drawdown_pct"`
	LongestLosingStreak int            `json:"longest_losing_streak"`
	FirstHalf           halfJSON       `json:"first_half"`
	SecondHalf          halfJSON       `json:"second_half"`
	Skipped             map[string]int `json:"skipped"`
}

type halfJSON struct {
	Trades    int     `json:"trades"`
	WinRate   float64 `json:"win_rate"`
	NetProfit float64 `json:"net_profit"`
	AvgR      float64 `json:"avg_r"`
}

func round(v float64, places int) float64 {
	if !finite(v) {
		return 0
	}
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

func statsOut(s Stats) statsJSON {
	h := func(x Half) halfJSON {
		return halfJSON{Trades: x.Trades, WinRate: round(x.WinRate, 2), NetProfit: round(x.NetProfit, 2), AvgR: round(x.AvgR, 3)}
	}
	var pf any = round(s.ProfitFactor, 3)
	if math.IsInf(s.ProfitFactor, 1) {
		pf = "inf"
	}
	sk := s.Skipped
	if sk == nil {
		sk = map[string]int{}
	}
	return statsJSON{Trades: s.Trades, Wins: s.Wins, Losses: s.Losses, WinRate: round(s.WinRate, 2), ProfitFactor: pf, AvgR: round(s.AvgR, 3),
		NetProfit: round(s.NetProfit, 2), GrossWin: round(s.GrossWin, 2), GrossLoss: round(s.GrossLoss, 2), Commission: round(s.Commission, 2),
		Swap: round(s.Swap, 2), StartBalance: round(s.StartBalance, 2), EndBalance: round(s.EndBalance, 2), ReturnPct: round(s.ReturnPct, 3),
		MaxDrawdown: round(s.MaxDrawdown, 2), MaxDrawdownPct: round(s.MaxDrawdownPct, 3), LongestLosingStreak: s.LongestLosingStreak,
		FirstHalf: h(s.FirstHalf), SecondHalf: h(s.SecondHalf), Skipped: sk}
}

type runJSON struct {
	Schema     int                `json:"schema"`
	Kind       string             `json:"kind"`
	Approach   string             `json:"approach"`   // Result.Name: the strategy, or the legs joined by " + "
	Instrument string             `json:"instrument"` // leg 0's (every leg's is in legs)
	Timeframes []string           `json:"timeframes"` // leg 0's, step first
	Params     map[string]float64 `json:"params"`     // leg 0's in force
	RanAt      string             `json:"ran_at"`
	Requested  rangeJSON          `json:"requested"`
	Range      rangeJSON          `json:"range"`
	Account    accountJSON        `json:"account"`
	Costs      []costJSON         `json:"costs"`
	Legs       []legJSON          `json:"legs"`
	Stats      statsJSON          `json:"stats"`
	Walk       *walkJSON          `json:"walk,omitempty"`
	Files      []string           `json:"files"`
}

type rangeJSON struct {
	From  string   `json:"from"`
	To    string   `json:"to"`
	Notes []string `json:"notes,omitempty"`
}

type accountJSON struct {
	Balance      float64 `json:"balance"`
	Currency     string  `json:"currency"`
	Leverage     int     `json:"leverage"`
	MaxOpen      int     `json:"max_open"`
	MaxOpenRisk  float64 `json:"max_open_risk"`
	MaxDailyLoss float64 `json:"max_daily_loss"`
	DayTZ        string  `json:"day_tz"`
}

type costJSON struct {
	Leg        int      `json:"leg"`
	Instrument string   `json:"instrument"`
	Spread     string   `json:"spread"`
	Commission string   `json:"commission"`
	Swap       string   `json:"swap"`
	Slippage   string   `json:"slippage"`
	Overridden []string `json:"overridden"` // which of the four are not Exness's real costs
	Notes      []string `json:"notes,omitempty"`
}

type legJSON struct {
	Leg          int                `json:"leg"`
	Strategy     string             `json:"strategy"`
	Instrument   string             `json:"instrument"`
	Timeframes   []string           `json:"timeframes"`
	Risk         float64            `json:"risk"`
	Params       map[string]float64 `json:"params"`
	MaxOpen      int                `json:"max_open,omitempty"` // the leg's open positions at once; missing = 1
	Stats        *statsJSON         `json:"stats,omitempty"`
	Contribution float64            `json:"contribution"` // percent of the run's net profit
}

type walkJSON struct {
	Train       string       `json:"train"` // Go durations, e.g. "2160h0m0s"
	Test        string       `json:"test"`
	Windows     []windowJSON `json:"windows"`
	OutOfSample statsJSON    `json:"out_of_sample"`
}

type windowJSON struct {
	TrainFrom string             `json:"train_from"`
	TrainTo   string             `json:"train_to"`
	TestFrom  string             `json:"test_from"`
	TestTo    string             `json:"test_to"`
	Params    map[string]float64 `json:"params"`
	Trials    int                `json:"trials"`
	Train     statsJSON          `json:"train"`
	Test      statsJSON          `json:"test"`
}

func utc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// runJSONOf is run.json's content; files are the report's file names.
func (r *Result) runJSONOf(files []string) runJSON {
	ri := r.Run
	kind := ri.Kind
	if kind == "" {
		kind = KindRun
		if len(r.Legs) > 1 || len(ri.Legs) > 1 {
			kind = KindPortfolio
		}
	}
	ranAt := ri.RanAt
	if ranAt.IsZero() {
		ranAt = time.Now()
	}
	out := runJSON{Schema: RunSchema, Kind: kind, Approach: r.Name, RanAt: utc(ranAt), Params: map[string]float64{}, Files: files,
		Requested: rangeJSON{From: utc(r.Range.RequestedFrom), To: utc(r.Range.RequestedTo)},
		Range:     rangeJSON{From: utc(r.Range.From), To: utc(r.Range.To), Notes: r.Range.Notes},
		Account: accountJSON{Balance: ri.Balance, Currency: ri.Currency, Leverage: ri.Leverage, MaxOpen: ri.MaxOpen, MaxOpenRisk: ri.MaxOpenRisk,
			MaxDailyLoss: ri.MaxDailyLoss, DayTZ: ri.DayTZ},
		Stats: statsOut(r.Stats), Costs: []costJSON{}, Legs: []legJSON{}}
	if out.Account.Balance == 0 {
		out.Account.Balance = r.Stats.StartBalance
	}
	for i, l := range ri.Legs {
		lj := legJSON{Leg: l.Leg, Strategy: l.Strategy, Instrument: l.Instrument, Timeframes: l.Timeframes, Risk: l.Risk, Params: l.Params, MaxOpen: l.MaxOpen}
		if lj.Params == nil {
			lj.Params = map[string]float64{}
		}
		for _, lr := range r.Legs {
			if lr.Leg == l.Leg && len(ri.Legs) > 1 {
				st := statsOut(lr.Stats)
				lj.Stats, lj.Contribution = &st, round(lr.Contribution, 2)
			}
		}
		if i == 0 {
			out.Instrument, out.Timeframes, out.Params = l.Instrument, l.Timeframes, lj.Params
		}
		out.Legs = append(out.Legs, lj)
	}
	if len(ri.Legs) == 0 && len(r.Params) > 0 && r.Params[0] != nil {
		out.Params = r.Params[0]
	}
	for _, c := range r.Costs {
		var ov []string
		for _, l := range ri.Legs {
			if l.Leg == c.Leg {
				ov = l.Overridden
			}
		}
		if ov == nil {
			ov = []string{}
		}
		if out.Instrument == "" {
			out.Instrument = c.Instrument
		}
		out.Costs = append(out.Costs, costJSON{Leg: c.Leg, Instrument: c.Instrument, Spread: c.Spread, Commission: c.Commission, Swap: c.Swap,
			Slippage: c.Slippage, Overridden: ov, Notes: c.Notes})
	}
	if w := r.Walk; w != nil {
		wj := &walkJSON{OutOfSample: statsOut(w.OutOfSample), Windows: []windowJSON{}}
		if ri.Walk != nil {
			wj.Train, wj.Test = ri.Walk.Train.String(), ri.Walk.Test.String()
		}
		for _, x := range w.Windows {
			wj.Windows = append(wj.Windows, windowJSON{TrainFrom: utc(x.TrainFrom), TrainTo: utc(x.TrainTo), TestFrom: utc(x.TestFrom), TestTo: utc(x.TestTo),
				Params: x.Params, Trials: x.Trials, Train: statsOut(x.Train), Test: statsOut(x.Test)})
		}
		out.Walk = wj
	}
	return out
}

// WriteRun writes run.json: what ran (approach, params, instrument,
// timeframes, range, costs, account) and its stats. files are the other
// report files to list in it.
func (r *Result) WriteRun(out io.Writer, files ...string) error {
	if files == nil {
		files = []string{}
	}
	b, err := json.MarshalIndent(r.runJSONOf(files), "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(b, '\n'))
	return err
}

// price formats a price with the instrument's digits when known.
func price(v float64, digits int) string {
	if digits > 0 && digits < 10 {
		return strconv.FormatFloat(v, 'f', digits, 64)
	}
	return strconv.FormatFloat(v, 'g', 10, 64)
}

// WriteCandles writes the step candles of the range that ran as CSV:
// instrument, timeframe, open time (UTC), bid OHLC, volume and the spread
// at the close (ask - bid). Legs on the same instrument and timeframe are
// written once.
func (r *Result) WriteCandles(out io.Writer) error {
	w := csv.NewWriter(out)
	w.Write([]string{"instrument", "timeframe", "time_utc", "open", "high", "low", "close", "volume", "spread"})
	seen := map[string]bool{}
	for _, set := range r.Candles {
		k := set.Instrument + "|" + set.Timeframe
		if seen[k] {
			continue
		}
		seen[k] = true
		digits := 0
		for _, c := range set.Candles {
			digits = max(digits, decimals(c.Open), decimals(c.Close))
			if digits >= 5 {
				break
			}
		}
		for _, c := range set.Candles {
			w.Write([]string{set.Instrument, set.Timeframe, c.OpenTime.UTC().Format(time.RFC3339), price(c.Open, digits), price(c.High, digits),
				price(c.Low, digits), price(c.Close, digits), strconv.FormatFloat(c.Volume, 'f', -1, 64), price(c.Spread(), digits+1)})
		}
	}
	w.Flush()
	return w.Error()
}

// decimals is how many decimal places v needs (at most 6), so candles keep
// the source's precision without float noise.
func decimals(v float64) int {
	for d := 0; d < 6; d++ {
		p := math.Pow(10, float64(d))
		if math.Abs(v*p-math.Round(v*p)) < 1e-6*p {
			return d
		}
	}
	return 6
}
