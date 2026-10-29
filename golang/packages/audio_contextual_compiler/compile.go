package audio_contextual_compiler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Compiler runs the two passes of contextual compiling.
type Compiler struct {
	Claude Claude
	// SonicPi adds the code checks that need an installed Sonic Pi (its
	// synth, FX and sample names, and Ruby's syntax check); nil skips them.
	SonicPi *SonicPi
	// Retries is how many more times a pass asks Claude when its answer
	// fails the checks. The failed answer and its problems go with it.
	Retries int
	Log     func(msg string) // progress, optional
}

// CheckError is returned when a pass's answer still fails the checks after
// the retries.
type CheckError struct {
	What     string // "the graph" or "the code"
	Tries    int    // 0 when no answer was asked for
	Problems []string
}

func (e *CheckError) Error() string {
	head := fmt.Sprintf("%s fails %d check(s)", e.What, len(e.Problems))
	if e.Tries > 0 {
		head = fmt.Sprintf("%s still fails %d check(s) after %d tries", e.What, len(e.Problems), e.Tries)
	}
	return head + ":\n- " + strings.Join(e.Problems, "\n- ")
}

func (c *Compiler) log(format string, args ...any) {
	if c.Log != nil {
		c.Log(fmt.Sprintf(format, args...))
	}
}

const graphSystem = `You are pass 1 of an LLM contextual compiler for music: you turn a prompt spec context into a jsongraph context.

A prompt spec context is a markdown file that describes one piece of music as OOP-like pseudo code: classes for the song, its sections and its instruments, with properties, lists such as flow = [Intro(), Climax()], arguments such as Gong(role = "..."), extends, and interactions such as "on Kendang.signal -> all.speedUp". Prose around the code is part of the spec too. The classes describe contexts (what is true in that part of the music), not code to translate.

The graph:
- Nodes have a type: song (exactly one, id "song"), section, instrument (anything that makes sound: instruments, drums, voices) or effect (reverb, echo, filters...). name is the class name as written. props are the node's own properties and arguments as key/value text, with the values as written ("slow", "5min", "soft fingerpicked melody").
- Ids: a section is its name in snake_case ("intro"); a part is its section's id, a dot, and its own name in snake_case ("intro.guitar"); a part of the whole song is "song.<name>". The same class used in two sections is two nodes, one in each.
- Edges: contains goes from the song to each section, with order 1, 2, 3... in flow order, and from the song or a section to each instrument or effect in it. follows goes from an instrument to the instrument whose timing or notes it follows. signals goes from an instrument or section to what reacts to it, with the event and the reaction in detail ("signal: speed up"); when every part of a section reacts, it points at the section. applies goes from an effect to what it is applied to. detail says in the spec's words what a relation means.
- Resolve extends between the spec's own classes: copy the inherited properties into the node; the node's own values win. A base such as Section or Instrument only tells the type.
- Prose about the whole piece goes into the song's props as "notes"; prose about one class goes into that node's "notes".
- Keep the spec's words. Do not invent properties, parts or sections the spec does not have: the next pass makes the musical choices.

If previous_graph is given, it is the graph of an earlier version of this spec: keep its ids for everything that is still there. If errors are given, rejected_graph is your last answer and it failed these checks: fix them.
Answer only through the structured output.`

const graphSchema = `{
  "type": "object",
  "properties": {
    "nodes": {"type": "array", "items": {
      "type": "object",
      "properties": {
        "id":   {"type": "string"},
        "type": {"type": "string", "enum": ["song", "section", "instrument", "effect"]},
        "name": {"type": "string"},
        "props": {"type": "array", "items": {
          "type": "object",
          "properties": {"key": {"type": "string"}, "value": {"type": "string"}},
          "required": ["key", "value"]
        }}
      },
      "required": ["id", "type", "name", "props"]
    }},
    "edges": {"type": "array", "items": {
      "type": "object",
      "properties": {
        "from":   {"type": "string"},
        "rel":    {"type": "string", "enum": ["contains", "follows", "signals", "applies"]},
        "to":     {"type": "string"},
        "order":  {"type": "integer"},
        "detail": {"type": "string"}
      },
      "required": ["from", "rel", "to"]
    }}
  },
  "required": ["nodes", "edges"]
}`

type graphInput struct {
	Spec          string          `json:"spec"`
	PreviousGraph *Graph          `json:"previous_graph,omitempty"`
	RejectedGraph json.RawMessage `json:"rejected_graph,omitempty"`
	Errors        []string        `json:"errors,omitempty"`
}

// graphAnswer is pass 1's structured output. Props are a list there, since
// the answer's schema has no free-form objects.
type graphAnswer struct {
	Nodes []struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Name  string `json:"name"`
		Props []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"props"`
	} `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func decodeGraph(raw json.RawMessage) (*Graph, error) {
	var a graphAnswer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("the answer is not a graph: %w", err)
	}
	g := &Graph{Nodes: []Node{}, Edges: a.Edges}
	if g.Edges == nil {
		g.Edges = []Edge{}
	}
	for _, n := range a.Nodes {
		node := Node{ID: n.ID, Type: n.Type, Name: n.Name}
		for _, p := range n.Props {
			if node.Props == nil {
				node.Props = map[string]string{}
			}
			node.Props[p.Key] = p.Value
		}
		g.Nodes = append(g.Nodes, node)
	}
	return g, nil
}

// Graph is pass 1: Claude turns the spec into a jsongraph context. prev, the
// graph of an earlier version of the spec, keeps the ids stable; it may be nil.
func (c *Compiler) Graph(ctx context.Context, spec string, prev *Graph) (*Graph, error) {
	in := graphInput{Spec: spec}
	if prev != nil {
		p := *prev
		p.SpecSHA256 = ""
		in.PreviousGraph = &p
	}
	for try := 1; ; try++ {
		raw, err := c.Claude.Ask(ctx, Request{
			System: graphSystem,
			Prompt: "Build the jsongraph context of the prompt spec context in the JSON on stdin.",
			Input:  in,
			Schema: graphSchema,
		})
		if err != nil {
			return nil, err
		}
		g, err := decodeGraph(raw)
		var problems []string
		if err != nil {
			problems = []string{err.Error()}
		} else {
			problems = g.Check()
		}
		if len(problems) == 0 {
			g.SpecSHA256 = SpecHash(spec)
			return g, nil
		}
		if try > c.Retries {
			return nil, &CheckError{What: "the graph", Tries: try, Problems: problems}
		}
		c.log("the graph fails %d check(s), asking Claude again", len(problems))
		in.RejectedGraph, in.Errors = raw, problems
	}
}

const codeSystem = `You are pass 2 of an LLM contextual compiler for music: you write the Sonic Pi code of the song in the input.

The input's song is a set of nested contexts: the song, its sections in flow order, and the parts (instruments, effects) inside each, with their relations. props say what is true in a context; a section's props override the song's, and a part's override its section's. Turn the descriptions into concrete musical choices yourself (tempo, scale, notes, rhythm, synth, envelope, filter, volume) so that the music fits every context it is in.

Rules for the code:
- Sonic Pi only: synths, FX and samples that ship with Sonic Pi, never an invented name. When sonic_pi lists the names of the installed version, use only those. Approximate instruments Sonic Pi has no sound for (a gamelan gong, bonang, kendang, ...) with built-in synths or samples, and say so in a comment.
- The piece ends: no live_loop. Each section is a define that plays for the section's length and then returns: its parts play together with in_thread and counted loops, and the section's own thread sleeps for the section's length. The code ends by calling the sections in flow order. The total length matches the song's duration.
- Start with use_bpm and use_random_seed, so it plays the same every time.
- Put a comment line "# node: <id>" right above the code of every section and every part, with the ids from the input.
- Relations: follows locks a part's timing or notes to the other part's; signals is a cue in the signalling part and a sync in what reacts; applies wraps the target in with_fx.
- Scales Sonic Pi does not have (pelog, slendro, ...): compute the notes; play takes fractional MIDI note numbers.
- Write nested calls with parentheses (ring(60, 62), scale(:e3, :minor)), and do not use Sonic Pi function names such as scale, chord, ring or note as variable names.

If errors are given, rejected_code is your last answer and it failed these checks: fix them and answer with the whole code again.
Answer only through the structured output.`

const codeSchema = `{
  "type": "object",
  "properties": {"code": {"type": "string"}},
  "required": ["code"]
}`

type codeInput struct {
	Song         Context  `json:"song"`
	SonicPi      *SonicPi `json:"sonic_pi,omitempty"`
	RejectedCode string   `json:"rejected_code,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}

// Code is pass 2: Claude writes Sonic Pi code from the graph's contexts.
func (c *Compiler) Code(ctx context.Context, g *Graph) (string, error) {
	if problems := g.Check(); len(problems) > 0 {
		return "", &CheckError{What: "the graph", Problems: problems}
	}
	in := codeInput{Song: Contexts(g), SonicPi: c.SonicPi}
	for try := 1; ; try++ {
		raw, err := c.Claude.Ask(ctx, Request{
			System: codeSystem,
			Prompt: "Write the Sonic Pi code of the song in the JSON on stdin.",
			Input:  in,
			Schema: codeSchema,
		})
		if err != nil {
			return "", err
		}
		var out struct {
			Code string `json:"code"`
		}
		var problems []string
		if err := json.Unmarshal(raw, &out); err != nil {
			problems = []string{fmt.Sprintf("the answer is not {\"code\": ...}: %v", err)}
		} else {
			problems = CheckCode(ctx, out.Code, g, c.SonicPi)
		}
		if len(problems) == 0 {
			return out.Code, nil
		}
		if try > c.Retries {
			return "", &CheckError{What: "the code", Tries: try, Problems: problems}
		}
		c.log("the code fails %d check(s), asking Claude again", len(problems))
		in.RejectedCode, in.Errors = out.Code, problems
	}
}
