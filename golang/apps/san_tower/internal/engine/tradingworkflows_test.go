package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The trading workflows (workflows/trading, the user, 2026-10-30: autotrade
// as workflows) are a file to import, made by hand: every step, setting,
// link and reference in it must be one this engine has, the Filters and
// Templates must work on what the AI sends, and the scripts must keep the
// journal as rules.md says.
func TestTradingWorkflowsFile(t *testing.T) {
	b, err := os.ReadFile("../../workflows/trading/trading.workflows.json")
	if err != nil {
		t.Fatal(err)
	}
	var f v1.WorkflowFile
	if err := protojson.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	names := map[string]*v1.ExportedWorkflow{}
	for _, w := range f.GetWorkflows() {
		names[w.GetName()] = w
	}
	for _, w := range f.GetWorkflows() {
		steps := map[string]*kind{}
		for _, st := range w.GetSteps() {
			k := byKind[st.GetKind()]
			if k == nil {
				t.Errorf("%s: step %s is a %q, which is no kind", w.GetName(), st.GetId(), st.GetKind())
				continue
			}
			steps[st.GetId()] = k
			for name, value := range st.GetValues() {
				i := slices.IndexFunc(k.Settings, func(s *v1.Setting) bool { return s.GetName() == name })
				switch {
				case i >= 0:
					s := k.Settings[i]
					if opts := s.GetOptions(); len(opts) > 0 && !slices.Contains(opts, value) {
						t.Errorf("%s: %s's %s is %q, none of %v", w.GetName(), st.GetId(), name, value, opts)
					}
					var refs map[string]*v1.ExportedRef
					switch {
					case s.GetConnectionType() != "":
						refs = f.GetConnections()
					case s.GetStorage():
						refs = f.GetStorages()
					case s.GetKnowledge():
						refs = f.GetKnowledge()
					}
					if refs != nil && refs[value] == nil {
						t.Errorf("%s: %s's %s is %q, not in the file", w.GetName(), st.GetId(), name, value)
					}
					if s.GetConnectionType() != "" && refs[value].GetType() != s.GetConnectionType() {
						t.Errorf("%s: %s's %s is a %s connection", w.GetName(), st.GetId(), name, refs[value].GetType())
					}
				case !slices.ContainsFunc(k.Inputs, func(s *v1.Socket) bool { return s.GetName() == name && s.GetField() }):
					t.Errorf("%s: %s has a value %q, which is no setting or field of %s", w.GetName(), st.GetId(), name, k.Kind)
				}
			}
			if k.Kind == "workflow-tool" && names[st.GetValues()["Workflow"]] == nil {
				t.Errorf("%s: %s runs %q, which is not in the file", w.GetName(), st.GetId(), st.GetValues()["Workflow"])
			}
		}
		for _, l := range w.GetLinks() {
			from, to := steps[l.GetSource()], steps[l.GetTarget()]
			if from == nil || to == nil {
				t.Errorf("%s: a link %s → %s between steps that are not there", w.GetName(), l.GetSource(), l.GetTarget())
				continue
			}
			out := slices.IndexFunc(from.Outputs, func(s *v1.Socket) bool { return s.GetName() == l.GetSourceSocket() })
			in := slices.IndexFunc(to.Inputs, func(s *v1.Socket) bool { return s.GetName() == l.GetTargetSocket() })
			switch {
			case out < 0:
				t.Errorf("%s: %s (%s) has no output %q", w.GetName(), l.GetSource(), from.Kind, l.GetSourceSocket())
			case in < 0:
				t.Errorf("%s: %s (%s) has no input %q", w.GetName(), l.GetTarget(), to.Kind, l.GetTargetSocket())
			case from.Outputs[out].GetType() != to.Inputs[in].GetType():
				t.Errorf("%s: %s.%s is %s, %s.%s %s", w.GetName(), l.GetSource(), l.GetSourceSocket(), from.Outputs[out].GetType(),
					l.GetTarget(), l.GetTargetSocket(), to.Inputs[in].GetType())
			}
		}
	}

	// What the AI sends each workflow, as rules.md shows it: the Filter lets
	// it through, an empty one not, and the Templates fill in.
	samples := map[string]string{
		"trade open": `{"symbol": "BTCUSDT", "side": "buy", "type": "limit", "quantity": 0.012, "price": 80000, "stop_loss": 79500,
			"take_profit": 81200, "expires_min": 15, "approaches": "big_trend,positioning", "reason": "thesis: up; trigger: retest; invalid if: 79600; target: 81200"}`,
		"trade close":    `{"symbol": "ETHUSDT", "reason": "the thesis broke"}`,
		"trade protect":  `{"symbol": "BNBUSDT", "stop_loss": 760.5, "reason": "+1R"}`,
		"trade lesson":   `{"id": "BTCUSDT-20261030-130512", "symbol": "BTCUSDT", "approaches": "big_trend,new_one", "result": "-1.04R, -3.26 USDT", "lesson": "What happened: it fell. Lesson: wait."}`,
		"trade approach": `{"name": "big_trend", "title": "Big Trend", "description": "Trade with the 1h and 4h."}`,
	}
	rendered := map[string]map[string]string{}
	for name, sample := range samples {
		w := names[name]
		if w == nil {
			t.Fatalf("no workflow %q", name)
		}
		p := payload([]byte(sample))
		rendered[name] = map[string]string{}
		for _, st := range w.GetSteps() {
			switch st.GetKind() {
			case "filter":
				if ok, err := evalBool(st.GetValues()["If"], p); err != nil || !ok {
					t.Errorf("%s: the sample does not pass: %v %v", name, ok, err)
				}
				if ok, _ := evalBool(st.GetValues()["If"], payload([]byte(`{}`))); ok {
					t.Errorf("%s: an empty payload passes", name)
				}
			case "template":
				text, err := interpolate(st.GetValues()["Text"], p)
				if err != nil {
					t.Errorf("%s: %s: %v", name, st.GetId(), err)
				}
				rendered[name][st.GetId()] = text
			}
		}
	}
	if got := rendered["trade open"]["say"]; got != "trade BTCUSDT open buy limit 0.012 at 80000, stop loss 79500, take profit 81200; approaches big_trend,positioning\nthesis: up; trigger: retest; invalid if: 79600; target: 81200" {
		t.Errorf("open's message: %q", got)
	}
	if got := rendered["trade protect"]["say"]; got != "trade BNBUSDT protect: stop loss 760.5, take profit kept; +1R" {
		t.Errorf("protect's message: %q", got)
	}

	// The scripts, on a journal of their own.
	dir := t.TempDir()
	script := func(workflow, step string) string { return stepOf(names[workflow], step).GetValues()["Script"] }
	sh := func(s, input string) scriptResult {
		t.Helper()
		r, err := runScript(context.Background(), s, input, dir, time.Minute, nil)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	read := func(path string) string {
		b, _ := os.ReadFile(filepath.Join(dir, path))
		return string(b)
	}
	month := time.Now().UTC().Format("2006-01")
	if r := sh(script("trade open", "keep"), rendered["trade open"]["entry"]); r.code != 0 {
		t.Fatalf("open's journal: %d %s", r.code, r.stderr)
	}
	if got := read("trades/" + month + ".md"); !strings.HasPrefix(got, "# Trades "+month+"\n\n## ") || !strings.Contains(got, " BTCUSDT\n\nopen buy limit 0.012 at 80000") {
		t.Errorf("trades:\n%s", got)
	}
	if got := read("approaches/positioning.md"); !strings.Contains(got, "## Trades and lessons\n\n- ") || !strings.Contains(got, "BTCUSDT: open buy limit 0.012") {
		t.Errorf("a new approach's file:\n%s", got)
	}
	if r := sh(script("trade close", "keep"), "rm -rf /\n\nclose: x"); r.code == 0 {
		t.Error("a symbol that is a command was taken")
	}
	if r := sh(script("trade lesson", "keep"), rendered["trade lesson"]["entry"]); r.code != 0 {
		t.Fatalf("lesson: %d %s", r.code, r.stderr)
	}
	if got := read("lessons/" + month + ".md"); !strings.Contains(got, "## BTCUSDT-20261030-130512\n\nBTCUSDT -1.04R, -3.26 USDT. What happened: it fell. Lesson: wait.\n") {
		t.Errorf("lessons:\n%s", got)
	}
	if got := read("lessons/done.txt"); got != "BTCUSDT-20261030-130512\n" {
		t.Errorf("done.txt: %q", got)
	}
	if got := read("approaches/new_one.md"); !strings.Contains(got, "- BTCUSDT-20261030-130512: -1.04R, -3.26 USDT. Its lesson:") {
		t.Errorf("new_one:\n%s", got)
	}
	if r := sh(script("trade lesson", "keep"), rendered["trade lesson"]["entry"]); r.code == 0 || !strings.Contains(r.stderr, "already has a lesson") {
		t.Errorf("a second lesson: %d %s", r.code, r.stderr)
	}
	if r := sh(script("trade approach", "keep"), rendered["trade approach"]["entry"]); r.code != 0 {
		t.Fatalf("approach: %d %s", r.code, r.stderr)
	}
	if got := read("approaches/big_trend.md"); !strings.HasPrefix(got, "# Big Trend (`big_trend`)\n\nTrade with the 1h and 4h.\n\n## Trades and lessons\n") || !strings.Contains(got, "BTCUSDT-20261030-130512") {
		t.Errorf("big_trend rewritten:\n%s", got)
	}

	// The tick's notes before any decision, and after two.
	tick := names["trade tick"]
	if r := sh(stepOf(tick, "notes").GetValues()["Script"], "no trades closed"); !strings.Contains(r.stdout, "### Closed trades without a lesson (the last 72h)\n\nno trades closed\n") || !strings.Contains(r.stdout, "the first tick") {
		t.Errorf("first notes:\n%s", r.stdout)
	}
	sh(stepOf(tick, "keep").GetValues()["Script"], "BTCUSDT: hold, watching: one")
	sh(stepOf(tick, "keep").GetValues()["Script"], "BTCUSDT: hold, watching: two\nKnowledge: up to date")
	r := sh(stepOf(tick, "notes").GetValues()["Script"], "none")
	if !strings.Contains(r.stdout, "### Your last decisions\n\n### ") || !strings.Contains(r.stdout, "watching: two\nKnowledge: up to date") || strings.Contains(r.stdout, "watching: one") {
		t.Errorf("notes:\n%s", r.stdout)
	}
}

func stepOf(w *v1.ExportedWorkflow, id string) *v1.Step {
	for _, s := range w.GetSteps() {
		if s.GetId() == id {
			return s
		}
	}
	return &v1.Step{}
}
