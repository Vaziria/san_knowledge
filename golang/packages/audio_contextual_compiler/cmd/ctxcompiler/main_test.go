package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acc "github.com/wargasipil/audio_contextual_compiler"
)

const graphAnswer = `{
  "nodes": [
    {"id": "song", "type": "song", "name": "Lullaby", "props": [{"key": "duration", "value": "8 beats"}]},
    {"id": "verse", "type": "section", "name": "Verse", "props": []},
    {"id": "verse.piano", "type": "instrument", "name": "Piano", "props": [{"key": "role", "value": "soft chords"}]}
  ],
  "edges": [
    {"from": "song", "rel": "contains", "to": "verse", "order": 1},
    {"from": "verse", "rel": "contains", "to": "verse.piano"}
  ]
}`

const code = `use_bpm 60
# node: verse
define :verse do
  # node: verse.piano
  in_thread do
    play_chord chord(:c4, :major)
  end
  sleep 8
end
verse
`

// fake answers pass 1 and pass 2 and records what it was asked.
type fake struct {
	graphs, codes []string
	inputs        []string
}

func (f *fake) Ask(_ context.Context, req acc.Request) (json.RawMessage, error) {
	in, _ := json.Marshal(req.Input)
	f.inputs = append(f.inputs, string(in))
	answers := &f.codes
	if strings.Contains(req.Prompt, "jsongraph") {
		answers = &f.graphs
	}
	if len(*answers) == 0 {
		return nil, errors.New("fake: not expected: " + req.Prompt)
	}
	a := (*answers)[0]
	*answers = (*answers)[1:]
	return json.RawMessage(a), nil
}

func codeAnswer() string {
	b, _ := json.Marshal(map[string]string{"code": code})
	return string(b)
}

func runCLI(t *testing.T, f *fake, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(context.Background(), args, &out, &errOut, f)
	t.Logf("ctxcompiler %s\n%s%s", strings.Join(args, " "), errOut.String(), out.String())
	return out.String(), err
}

func TestCompile(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "lullaby.md")
	if err := os.WriteFile(spec, []byte("class Lullaby { flow = [Verse()] }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &fake{graphs: []string{graphAnswer}, codes: []string{codeAnswer()}}
	if _, err := runCLI(t, f, "compile", spec, "--no-sonic-pi"); err != nil {
		t.Fatal(err)
	}
	g, err := acc.LoadGraph(filepath.Join(dir, "lullaby.graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	if g.SpecSHA256 == "" || len(g.Nodes) != 3 {
		t.Errorf("graph: %+v", g)
	}
	rb, err := os.ReadFile(filepath.Join(dir, "lullaby.rb"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Compiled by ctxcompiler from lullaby.md (graph: lullaby.graph.json).\n\n" + code; string(rb) != want {
		t.Errorf("code file:\n%s", rb)
	}

	// The same spec again: the graph is reused, only pass 2 runs.
	f = &fake{codes: []string{codeAnswer()}}
	out := filepath.Join(dir, "other.rb")
	if _, err := runCLI(t, f, "compile", spec, "-o", out, "--no-sonic-pi"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Error(err)
	}

	// --fresh builds the graph again, without the old ids.
	f = &fake{graphs: []string{graphAnswer}, codes: []string{codeAnswer()}}
	if _, err := runCLI(t, f, "compile", "--fresh", spec, "--no-sonic-pi"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.inputs[0], "previous_graph") {
		t.Error("--fresh sent the old graph")
	}

	// A changed spec builds the graph again, keeping the old ids.
	if err := os.WriteFile(spec, []byte("class Lullaby { flow = [Verse(), Verse()] }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f = &fake{graphs: []string{graphAnswer}, codes: []string{codeAnswer()}}
	if _, err := runCLI(t, f, "compile", spec, "--no-sonic-pi"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.inputs[0], `"previous_graph":{"nodes"`) {
		t.Errorf("the changed spec was sent without the old graph: %s", f.inputs[0])
	}

	// From a graph: pass 2 only, next to the graph.
	if err := os.Remove(filepath.Join(dir, "lullaby.rb")); err != nil {
		t.Fatal(err)
	}
	f = &fake{codes: []string{codeAnswer()}}
	if _, err := runCLI(t, f, "compile", filepath.Join(dir, "lullaby.graph.json"), "--no-sonic-pi"); err != nil {
		t.Fatal(err)
	}
	rb, err = os.ReadFile(filepath.Join(dir, "lullaby.rb"))
	if err != nil || !strings.HasPrefix(string(rb), "# Compiled by ctxcompiler from lullaby.graph.json.\n") {
		t.Errorf("code from a graph: %v\n%s", err, rb)
	}
}

func TestCompileFails(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "x.md")
	os.WriteFile(spec, []byte("spec"), 0o644)
	f := &fake{graphs: []string{graphAnswer}, codes: []string{`{"code": "verse"}`, `{"code": "verse"}`}}
	_, err := runCLI(t, f, "compile", spec, "--retries", "1", "--no-sonic-pi")
	if err == nil || !strings.Contains(err.Error(), "the code still fails 1 check(s) after 2 tries") {
		t.Errorf("want the code's problems, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "x.rb")); err == nil {
		t.Error("failed code was written")
	}
	if _, err := runCLI(t, &fake{}, "compile", "a.md", "b.md"); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Errorf("two files: %v", err)
	}
}

func TestGraphAndCheck(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "x.md")
	os.WriteFile(spec, []byte("spec"), 0o644)
	graph := filepath.Join(dir, "g.json")
	f := &fake{graphs: []string{graphAnswer}}
	if _, err := runCLI(t, f, "graph", spec, "-o", graph, "--no-sonic-pi"); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, nil, "check", graph)
	if err != nil || out != "ok: 3 nodes, 2 edges\n" {
		t.Errorf("check graph: %v %q", err, out)
	}
	rb := filepath.Join(dir, "x.rb")
	os.WriteFile(rb, []byte(code), 0o644)
	out, err = runCLI(t, nil, "check", graph, rb, "--no-sonic-pi")
	if err != nil || out != "ok: 3 nodes, 2 edges; the code passes the checks\n" {
		t.Errorf("check code: %v %q", err, out)
	}
	os.WriteFile(rb, []byte(strings.Replace(code, "define :verse do", "live_loop :verse do", 1)), 0o644)
	out, err = runCLI(t, nil, "check", graph, rb, "--no-sonic-pi")
	if err == nil || !strings.Contains(out, "- live_loop never ends") {
		t.Errorf("check bad code: %v %q", err, out)
	}
}
