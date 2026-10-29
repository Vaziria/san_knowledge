package audio_contextual_compiler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fakeClaude answers pass 1 with graphs and pass 2 with code, in order.
type fakeClaude struct {
	graphs []string
	codes  []string
	reqs   []Request
}

func (f *fakeClaude) Ask(_ context.Context, req Request) (json.RawMessage, error) {
	f.reqs = append(f.reqs, req)
	answers := &f.codes
	if req.Schema == graphSchema {
		answers = &f.graphs
	}
	if len(*answers) == 0 {
		return nil, errors.New("fake: no answer left")
	}
	a := (*answers)[0]
	*answers = (*answers)[1:]
	return json.RawMessage(a), nil
}

func codeAnswer(code string) string {
	b, _ := json.Marshal(map[string]string{"code": code})
	return string(b)
}

func TestGraphPass(t *testing.T) {
	bad := strings.Replace(folkAnswer, `"order": 2`, `"order": 1`, 1)
	f := &fakeClaude{graphs: []string{bad, folkAnswer}}
	c := &Compiler{Claude: f, Retries: 1}
	g, err := c.Graph(context.Background(), "class FolkSong {}", nil)
	if err != nil {
		t.Fatal(err)
	}
	if g.SpecSHA256 != SpecHash("class FolkSong {}") || len(g.Nodes) != 7 {
		t.Errorf("graph: %+v", g)
	}
	if len(f.reqs) != 2 {
		t.Fatalf("%d requests, want 2", len(f.reqs))
	}
	first := f.reqs[0].Input.(graphInput)
	if first.Spec != "class FolkSong {}" || first.Errors != nil || first.PreviousGraph != nil {
		t.Errorf("first input: %+v", first)
	}
	retry := f.reqs[1].Input.(graphInput)
	if string(retry.RejectedGraph) != bad || len(retry.Errors) != 1 || !strings.Contains(retry.Errors[0], "both have order 1") {
		t.Errorf("retry input: errors %q", retry.Errors)
	}
}

func TestGraphPassGivesUp(t *testing.T) {
	f := &fakeClaude{graphs: []string{`{"nodes": [], "edges": []}`}}
	_, err := (&Compiler{Claude: f}).Graph(context.Background(), "spec", nil)
	var ce *CheckError
	if !errors.As(err, &ce) || ce.Tries != 1 || !strings.Contains(err.Error(), "exactly one song") {
		t.Fatalf("want a CheckError after 1 try, got %v", err)
	}
	f = &fakeClaude{graphs: []string{`not json`}}
	if _, err := (&Compiler{Claude: f}).Graph(context.Background(), "spec", nil); err == nil || !strings.Contains(err.Error(), "not a graph") {
		t.Errorf("want 'not a graph', got %v", err)
	}
}

func TestGraphPassKeepsIDs(t *testing.T) {
	prev := folkGraph(t)
	prev.SpecSHA256 = "old"
	f := &fakeClaude{graphs: []string{folkAnswer}}
	if _, err := (&Compiler{Claude: f}).Graph(context.Background(), "spec v2", prev); err != nil {
		t.Fatal(err)
	}
	in := f.reqs[0].Input.(graphInput)
	if in.PreviousGraph == nil || in.PreviousGraph.SpecSHA256 != "" || len(in.PreviousGraph.Nodes) != 7 {
		t.Errorf("previous graph: %+v", in.PreviousGraph)
	}
	if prev.SpecSHA256 != "old" {
		t.Error("the caller's graph was changed")
	}
}

func TestCodePass(t *testing.T) {
	missing := strings.Replace(folkCode, "# node: climax.bonang", "", 1)
	f := &fakeClaude{codes: []string{codeAnswer(missing), codeAnswer(folkCode)}}
	sp := &SonicPi{Version: "5.0.0", Synths: []string{"pluck", "dull_bell"}, FX: []string{"reverb"}, Samples: []string{"bd_haus"}}
	c := &Compiler{Claude: f, SonicPi: sp, Retries: 2}
	var logs []string
	c.Log = func(msg string) { logs = append(logs, msg) }
	code, err := c.Code(context.Background(), folkGraph(t))
	if err != nil {
		t.Fatal(err)
	}
	if code != folkCode {
		t.Errorf("code: %q", code)
	}
	first := f.reqs[0].Input.(codeInput)
	if first.Song.ID != "song" || len(first.Song.Sections) != 2 || first.SonicPi != sp {
		t.Errorf("first input: %+v", first)
	}
	b, _ := json.Marshal(first)
	if !strings.Contains(string(b), `"sonic_pi":{"version":"5.0.0","synths":["pluck","dull_bell"]`) {
		t.Errorf("Claude does not get the names: %s", b)
	}
	retry := f.reqs[1].Input.(codeInput)
	if retry.RejectedCode != missing || !reflect.DeepEqual(retry.Errors, []string{`there is no "# node: <id>" comment for climax.bonang`}) {
		t.Errorf("retry: errors %q", retry.Errors)
	}
	if fmt.Sprint(logs) != "[the code fails 1 check(s), asking Claude again]" {
		t.Errorf("logs: %q", logs)
	}
}

func TestCodePassChecksTheGraphFirst(t *testing.T) {
	g := folkGraph(t)
	g.Edges = g.Edges[1:]
	f := &fakeClaude{}
	_, err := (&Compiler{Claude: f}).Code(context.Background(), g)
	if err == nil || !strings.HasPrefix(err.Error(), "the graph fails") || len(f.reqs) > 0 {
		t.Errorf("want the graph's problems before any request, got %v", err)
	}
}
