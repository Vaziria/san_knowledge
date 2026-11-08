package engine

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

func TestValueAt(t *testing.T) {
	obj, err := readJSONText(`{
		"status": 200,
		"body": {
			"total": 2.5,
			"ok": true,
			"none": null,
			"items": [
				{"id": 1, "title": "Night Train", "tags": ["Korea", "Zombie"]},
				{"id": 2, "title": "Exhuma"},
				{"id": 3}
			],
			"a.b": "dotted",
			"my key": "spaced",
			"7": "seven",
			"args": "{\"city\": \"Blitar\", \"days\": [1, 2]}"
		}
	}`, false, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ path, want string }{
		// want: the Text output, or "missing: <part of why>", or "error: <part>".
		{"status", "200"},
		{"body.total", "2.5"},
		{"body.ok", "true"},
		{"body.none", ""},
		{"body.items.0.title", "Night Train"},
		{"$.body.items[1].title", "Exhuma"},
		{"body.items[-1].id", "3"},
		{"body.items.-3.id", "1"},
		{"body.items.0.tags", `["Korea","Zombie"]`},
		{"body.items.0", `{"id":1,"tags":["Korea","Zombie"],"title":"Night Train"}`},
		{"body.items.*.title", `["Night Train","Exhuma"]`},
		{"body.items[*].tags.1", `["Zombie"]`},
		{`body["a.b"]`, "dotted"},
		{`body['my key']`, "spaced"},
		{`body.7`, "seven"},
		{`body["7"]`, "seven"},
		{"body.args.city", "Blitar"},
		{"body.args.days.1", "2"},
		{"", `{"body":{"7":"seven","a.b":"dotted","args":"{\"city\": \"Blitar\", \"days\": [1, 2]}","items":[{"id":1,"tags":["Korea","Zombie"],"title":"Night Train"},{"id":2,"title":"Exhuma"},{"id":3}],"my key":"spaced","none":null,"ok":true,"total":2.5},"status":200}`},
		{"body.titel", `missing: no "titel" in body (it has: 7, a.b, args, items, my key, none, ok, total)`},
		{"body.items.5.title", "missing: body.items has 3 items, no 5"},
		{"body.items.first", "missing: body.items is a list: use a number, as body.items.0"},
		{"body.items.2.title", `missing: no "title" in body.items.2 (it has: id)`},
		{"status.code", "missing: status is a number, not an object or list"},
		{"body.none.x", "missing: body.none is null"},
		{"body.items.*.year", `missing: in no item: no "year" in body.items.* (it has: id, tags, title)`},
		{"body..x", "error: an empty part"},
		{"body.items[0", "error: a [ without ]"},
		{`body["a.b`, "error: not closed"},
		{"body.items[]", "error: an empty []"},
	} {
		v, missing, err := valueAt(obj, c.path)
		switch {
		case strings.HasPrefix(c.want, "error: "):
			if err == nil || !strings.Contains(err.Error(), strings.TrimPrefix(c.want, "error: ")) {
				t.Errorf("%q: %v, %q, %v; want %s", c.path, v, missing, err, c.want)
			}
		case strings.HasPrefix(c.want, "missing: "):
			if err != nil || missing != strings.TrimPrefix(c.want, "missing: ") {
				t.Errorf("%q: missing %q, %v; want %s", c.path, missing, err, c.want)
			}
		default:
			if err != nil || missing != "" || jsonText(v) != c.want {
				t.Errorf("%q = %s, %q, %v; want %s", c.path, jsonText(v), missing, err, c.want)
			}
		}
	}
	// Value keeps the type: a list stays a list, a number a number.
	if v, _, _ := valueAt(obj, "body.items.*.id"); len(v.GetListValue().GetValues()) != 3 || v.GetListValue().GetValues()[2].GetNumberValue() != 3 {
		t.Errorf("ids: %v", v)
	}
	if v, _, _ := valueAt(structpb.NewNullValue(), "a"); v != nil {
		t.Errorf("null: %v", v)
	}
}

// getGraph: a Telegram message's text through Parse JSON to Get Value at
// path, whose Text and Found a Log writes.
func getGraph(path string, values ...string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("msg", TelegramMessage),
			step("p", "parse-json"),
			step("g", "get-value", append([]string{"Path", path}, values...)...),
			step("ok", "log"),
		},
		Links: []*v1.Link{
			link("msg", "Event", "p", "Event"),
			link("msg", "Text", "p", "Text"),
			link("p", "Object", "g", "Object"),
			link("p", "Done", "ok", "Event"),
			link("g", "Text", "ok", "Message"),
		},
	}
}

func TestGetValueStep(t *testing.T) {
	ctx := context.Background()
	msg := tgText(`{"movie": {"title": "Night Train", "year": 2024, "tags": ["Korea"]}}`)
	for _, c := range []struct {
		path   string
		values []string
		want   string // the Log's line, or "error: <part>"
	}{
		{"movie.title", nil, "Night Train"},
		{"movie.year", nil, "2024"},
		{"movie.tags", nil, `["Korea"]`},
		{"movie.director", []string{"Default", "tidak diketahui"}, "tidak diketahui"},
		{"movie.director", nil, ""},
		{"movie.director", []string{"Missing", "Fail"}, `error: Path movie.director: no "director" in movie (it has: tags, title, year)`},
	} {
		steps, err := Execute(ctx, getGraph(c.path, c.values...), "msg", msg, Env{})
		if want, ok := strings.CutPrefix(c.want, "error: "); ok {
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Errorf("%s %v: %s, %v; want error %q", c.path, c.values, ran(steps), err, want)
			}
			continue
		}
		if got := ran(steps); err != nil || !strings.HasSuffix(got, "ok::"+c.want) {
			t.Errorf("%s %v: %s, %v; want ok::%s", c.path, c.values, got, err, c.want)
		}
	}
	// Found: Yes or No, and Value an object input for the next step.
	g := &v1.Graph{
		Steps: []*v1.Step{
			step("msg", TelegramMessage),
			step("p", "parse-json"),
			step("g", "get-value", "Path", "movie"),
			step("f", "get-value", "Path", "nope"),
			step("t", "template", "Text", "{{ payload.title }}"),
			step("ok", "log"),
			step("found", "log"),
		},
		Links: []*v1.Link{
			link("msg", "Event", "p", "Event"),
			link("msg", "Text", "p", "Text"),
			link("p", "Object", "g", "Object"),
			link("p", "Object", "f", "Object"),
			link("g", "Value", "t", "Payload"),
			link("p", "Done", "ok", "Event"),
			link("t", "Text", "ok", "Message"),
			link("p", "Done", "found", "Event"),
			link("f", "Found", "found", "Message"),
		},
	}
	steps, err := Execute(ctx, g, "msg", msg, Env{})
	if got := ran(steps); err != nil || !strings.Contains(got, "ok::Night Train") || !strings.HasSuffix(got, "found::No") {
		t.Errorf("value and found: %s, %v", got, err)
	}
}
