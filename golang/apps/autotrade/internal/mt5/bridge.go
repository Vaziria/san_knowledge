// Package mt5 is autotrade's second venue: a MetaTrader 5 account (MIFX,
// demo only), reached through the AutotradeBridge Expert Advisor
// (AutotradeBridge.mq5) running in the terminal.
//
// MetaTrader has no API to call from outside, and an EA can't listen on a
// port, so the two sides talk through files in the terminals' common Files
// folder: autotrade writes a request as <id>.req, the EA claims it, answers
// in <id>.res, and autotrade reads and deletes the answer. Any number of
// autotrade processes (the CLI, the MCP server) can ask at once.
package mt5

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Source is the Expert Advisor, for `autotrade mt5 install`.
//
//go:embed AutotradeBridge.mq5
var Source []byte

// DefaultDir is the bridge folder in MetaTrader's common Files folder, where
// the EA looks (FILE_COMMON + "autotrade"). Empty off Windows: there the
// folder is under Wine's prefix and must be set in autotrade.yaml.
func DefaultDir() string {
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "MetaQuotes", "Terminal", "Common", "Files", "autotrade")
	}
	return ""
}

// ErrNoBridge means nothing took the request: the terminal is closed, or the
// EA is not on a chart.
var ErrNoBridge = errors.New("MetaTrader did not answer: open the MT5 terminal, log in to the demo account, and attach the AutotradeBridge EA to a chart with Algo Trading on (see USAGE.md, MetaTrader 5)")

// Error is a refusal from the EA or the trade server. Code is the trade
// server's return code (10015 = invalid price, ...) or the EA's own word
// (symbol, no_position, refused, ...).
type Error struct {
	Op, Code, Msg string
}

func (e *Error) Error() string { return "MetaTrader " + e.Op + ": " + e.Msg }

// Return codes of the trade server that get their own explanation.
const (
	codeInvalidPrice = "10015"
	codeInvalidStops = "10016"
	codeMarketClosed = "10018"
	codeNoMoney      = "10019"
)

type bridge struct {
	dir string
	// wait is how long a request may wait to be claimed; once claimed, the
	// EA has busy longer to answer (an order, or history being loaded).
	wait, busy time.Duration
}

var seq atomic.Int64

// call sends op with its arguments (key, value pairs) and returns the
// answer's records.
func (b bridge) call(ctx context.Context, op string, kv ...string) ([]record, error) {
	if b.dir == "" {
		return nil, errors.New("mt5.bridge is not set in autotrade.yaml: set it to MetaTrader's Common/Files/autotrade folder")
	}
	if err := os.MkdirAll(b.dir, 0o755); err != nil {
		return nil, err
	}
	var req strings.Builder
	req.WriteString("op=" + op + "\r\n")
	for i := 0; i+1 < len(kv); i += 2 {
		req.WriteString(kv[i] + "=" + kv[i+1] + "\r\n")
	}
	id := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), seq.Add(1))
	base := filepath.Join(b.dir, id)
	if err := os.WriteFile(base+".part", []byte(req.String()), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(base+".part", base+".req"); err != nil {
		os.Remove(base + ".part")
		return nil, err
	}

	deadline := time.Now().Add(b.wait)
	claimed := false
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		if data, err := os.ReadFile(base + ".res"); err == nil {
			os.Remove(base + ".res")
			return parse(op, data)
		}
		if time.Now().After(deadline) {
			if claimed {
				return nil, fmt.Errorf("MetaTrader took the %s request but gave no answer within %s: see the terminal's Experts tab; an order may have gone through", op, b.wait+b.busy)
			}
			// Unclaimed, it is ours to delete; if the EA claimed it first, the
			// delete fails and the answer is on its way.
			if err := os.Remove(base + ".req"); err == nil {
				return nil, ErrNoBridge
			}
			claimed, deadline = true, time.Now().Add(b.busy)
		}
		select {
		case <-ctx.Done():
			os.Remove(base + ".req")
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}

// record is one line of an answer: key=value fields.
type record map[string]string

func (r record) str(k string) string { return r[k] }

func (r record) num(k string) float64 {
	v, _ := strconv.ParseFloat(r[k], 64)
	return v
}

func (r record) int(k string) int64 {
	v, _ := strconv.ParseInt(r[k], 10, 64)
	return v
}

// time reads UTC Unix seconds; 0 is the zero time.
func (r record) time(k string) time.Time {
	if s := r.int(k); s > 0 {
		return time.Unix(s, 0).UTC()
	}
	return time.Time{}
}

func parse(op string, data []byte) ([]record, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	switch head := lines[0]; {
	case strings.HasPrefix(head, "error\t"):
		parts := strings.SplitN(head, "\t", 3)
		e := &Error{Op: op, Code: parts[1]}
		if len(parts) == 3 {
			e.Msg = parts[2]
		}
		return nil, e
	case head != "ok":
		return nil, fmt.Errorf("MetaTrader %s: unreadable answer %q", op, head)
	}
	var out []record
	for _, l := range lines[1:] {
		if l == "" {
			continue
		}
		r := record{}
		for _, f := range strings.Split(l, "\t") {
			k, v, _ := strings.Cut(f, "=")
			r[k] = v
		}
		out = append(out, r)
	}
	return out, nil
}
