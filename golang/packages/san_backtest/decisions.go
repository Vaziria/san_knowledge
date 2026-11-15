package san_backtest

import (
	"encoding/json"
	"io"
	"time"
)

// Decision is one line of the decision log (decisions.jsonl): a candle
// where something happened. Quiet candles are left out (the user,
// 2026-11-15: "2b": the log keeps only candles where something happened).
type Decision struct {
	Time time.Time // UTC: the signal candle's close, the fill's open, the exit time
	Leg  int
	Kind string // one of the Decision* kinds
	Side Side   // buy or sell, when the decision has one
	// Details are the numbers of the decision: price, stop, target, lots,
	// net, r, old_stop (by kind; see the README's report section).
	Details map[string]float64
	// Reason says why: the approach's notes for a signal or a note, the
	// exit reason ("stop", "target", "close: ..."), the Skip* reason of a
	// missed trade.
	Reason string
}

// Decision kinds.
const (
	DecisionSignal   = "signal"    // OnCandle asked for something; Details "action" is 1 buy, 2 sell, 3 close, 4 move stop
	DecisionEntry    = "entry"     // a position opened
	DecisionExit     = "exit"      // a position closed
	DecisionStopMove = "stop_move" // MoveStop moved the stop
	DecisionMissed   = "missed"    // a Buy or Sell did not open
	DecisionNote     = "note"      // the approach explained a step that did not signal
)

// decisionLine is a Decision as JSON.
type decisionLine struct {
	Time    string             `json:"time"`
	Leg     int                `json:"leg"`
	Kind    string             `json:"kind"`
	Side    Side               `json:"side,omitempty"`
	Details map[string]float64 `json:"details,omitempty"`
	Reason  string             `json:"reason,omitempty"`
}

// WriteDecisions writes the decision log as JSON lines, one Decision a
// line, in time order.
func (r *Result) WriteDecisions(out io.Writer) error {
	enc := json.NewEncoder(out)
	for _, d := range r.Decisions {
		det := map[string]float64{}
		for k, v := range d.Details {
			if finite(v) {
				det[k] = round(v, 6) // no float noise in the log
			}
		}
		if err := enc.Encode(decisionLine{Time: d.Time.UTC().Format(time.RFC3339), Leg: d.Leg, Kind: d.Kind, Side: d.Side, Details: det, Reason: d.Reason}); err != nil {
			return err
		}
	}
	return nil
}
