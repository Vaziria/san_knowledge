package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func TestReadJSONText(t *testing.T) {
	for _, c := range []struct {
		text   string
		find   bool
		expect string
		want   string // stringify of the value, or "error: <part of the error>"
	}{
		{`{"title": "Night Train", "year": 2024, "tags": ["Korea", "Zombie"]}`, false, "", `{"tags":["Korea","Zombie"],"title":"Night Train","year":2024}`},
		{"  \ufeff[1, 2.5, true, null]\n", false, "", `[1,2.5,true,null]`},
		{`"hi"`, false, "", `"hi"`},
		{"```json\n{\"a\": 1}\n```", false, "", `{"a":1}`},
		{"```\n[1]\n```", false, "List", `[1]`},
		{"", false, "", "error: Text is empty"},
		{"   ", true, "", "error: Text is empty"},
		{"hello", false, "", "error: Text is not JSON: invalid character 'h' looking for beginning of value (at character 1)"},
		{`{"a": 1,}`, false, "", "error: Text is not JSON: invalid character '}'"},
		{`{"a": 1} extra`, false, "", "error: after top-level value"},
		{"Here it is:\n```json\n{\"a\": 1}\n```\nDone.", false, "", "error: Text is not JSON"},
		{"Here it is:\n```json\n{\"a\": 1}\n```\nDone.", true, "", `{"a":1}`},
		{"```text\nnot json\n```\n```json\n{\"b\": 2}\n```", true, "", `{"b":2}`},
		{`Sure! {"title": "Night Train"} is the payload.`, true, "Object", `{"title":"Night Train"}`},
		{"no json here", true, "", "error: no JSON found in Text"},
		{`[1, 2]`, false, "Object", "error: Text is JSON but a list, not an object"},
		{`{"a": 1}`, false, "List", "error: Text is JSON but an object, not a list"},
		{`42`, false, "Object", "error: Text is JSON but a number, not an object"},
		{`{"a": 1}`, false, "Any", `{"a":1}`},
	} {
		v, err := readJSONText(c.text, c.find, c.expect)
		if want, ok := strings.CutPrefix(c.want, "error: "); ok {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%q find=%v expect=%q: %v, %v; want error %q", c.text, c.find, c.expect, v, err, want)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q find=%v expect=%q: %v", c.text, c.find, c.expect, err)
			continue
		}
		if got, _ := stringify(v, false); got != c.want {
			t.Errorf("%q find=%v expect=%q = %s; want %s", c.text, c.find, c.expect, got, c.want)
		}
	}
}

// parseGraph: a Telegram message's text goes through Parse JSON, its Object
// through Template to a Log, and Not JSON (when withElse) to another Log.
func parseGraph(withElse bool, values ...string) *v1.Graph {
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("msg", TelegramMessage),
			step("p", "parse-json", values...),
			step("t", "template", "Text", "{{ payload.title }} ({{ payload.year }})"),
			step("ok", "log"),
		},
		Links: []*v1.Link{
			link("msg", "Event", "p", "Event"),
			link("msg", "Text", "p", "Text"),
			link("p", "Done", "ok", "Event"),
			link("p", "Object", "t", "Payload"),
			link("t", "Text", "ok", "Message"),
		},
	}
	if withElse {
		g.Steps = append(g.Steps, step("bad", "log"))
		g.Links = append(g.Links, link("p", "Not JSON", "bad", "Event"), link("p", "Error", "bad", "Message"))
	}
	return g
}

// tgText is a Telegram message with text, as the trigger gets it.
func tgText(text string) []byte {
	b, _ := json.Marshal(map[string]any{"text": text, "chat": map[string]any{"id": 1}})
	return b
}

func TestParseJSONStep(t *testing.T) {
	ctx := context.Background()
	steps, err := Execute(ctx, parseGraph(true), "msg", tgText(`{"title": "Night Train", "year": 2024}`), Env{})
	if got := ran(steps); err != nil || !strings.Contains(got, "p:Done:") || !strings.HasSuffix(got, "ok::Night Train (2024)") {
		t.Errorf("object: %s, %v", got, err)
	}
	steps, err = Execute(ctx, parseGraph(true), "msg", tgText("hello"), Env{})
	if got := ran(steps); err != nil || !strings.Contains(got, "p:Not JSON:") || !strings.Contains(got, "bad::Text is not JSON") || strings.Contains(got, "ok::") {
		t.Errorf("not JSON, linked: %s, %v", got, err)
	}
	// Not JSON not linked: the step fails, and the run with it.
	if _, err := Execute(ctx, parseGraph(false), "msg", tgText("hello"), Env{}); err == nil || !strings.Contains(err.Error(), "Text is not JSON") {
		t.Errorf("not JSON, unlinked: %v", err)
	}
	// An AI's answer, with Read set to JSON in Text and Expect Object.
	answer := "Berikut datanya:\n```json\n{\"title\": \"Exhuma\", \"year\": 2024}\n```"
	steps, err = Execute(ctx, parseGraph(true, "Read", "JSON in Text", "Expect", "Object"), "msg", tgText(answer), Env{})
	if got := ran(steps); err != nil || !strings.HasSuffix(got, "ok::Exhuma (2024)") {
		t.Errorf("JSON in text: %s, %v", got, err)
	}
	steps, err = Execute(ctx, parseGraph(true, "Expect", "Object"), "msg", tgText(`["a"]`), Env{})
	if got := ran(steps); err != nil || !strings.Contains(got, "bad::Text is JSON but a list, not an object") {
		t.Errorf("expect object: %s, %v", got, err)
	}
}
