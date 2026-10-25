package engine

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// commands: a Telegram message's text goes through a filter of kind kind,
// set by setting, to one Log or the other.
func commands(kind, setting, value string) *v1.Graph {
	return &v1.Graph{
		Steps: []*v1.Step{
			step("msg", TelegramMessage),
			step("f", kind, setting, value),
			step("yes", "log", "Message", "match"),
			step("no", "log", "Message", "else"),
		},
		Links: []*v1.Link{
			link("msg", "Event", "f", "Event"),
			link("msg", "Text", "f", "Text"),
			link("f", "Match", "yes", "Event"),
			link("f", "Else", "no", "Event"),
		},
	}
}

func TestPrefixFilter(t *testing.T) {
	for text, want := range map[string]string{
		"/start":        "match",
		"/start please": "match",
		"/Start":        "else",
		"hello /start":  "else",
		"":              "else",
	} {
		steps, err := Execute(context.Background(), commands("prefix-filter", "Prefix", "/start"), "msg", []byte(text), Env{})
		if err != nil || len(steps) != 3 || steps[2].Logs[0] != want {
			t.Errorf("%q: %s, %v; want %s", text, ran(steps), err, want)
		}
	}
	// A space typed after the prefix counts.
	if steps, _ := Execute(context.Background(), commands("prefix-filter", "Prefix", "/say "), "msg", []byte("/sayhi"), Env{}); steps[2].Logs[0] != "else" {
		t.Errorf("space: %s", ran(steps))
	}
	if _, err := Execute(context.Background(), commands("prefix-filter", "Prefix", " "), "msg", []byte("x"), Env{}); err == nil || !strings.Contains(err.Error(), "Prefix is empty") {
		t.Errorf("empty: %v", err)
	}
}

func TestRegexFilter(t *testing.T) {
	for text, want := range map[string]string{
		"/start":       "match",
		"/help me":     "match",
		"/starting":    "else",
		"order A-1234": "else",
	} {
		steps, err := Execute(context.Background(), commands("regex-filter", "Pattern", `^/(start|help)\b`), "msg", []byte(text), Env{})
		if err != nil || len(steps) != 3 || steps[2].Logs[0] != want {
			t.Errorf("%q: %s, %v; want %s", text, ran(steps), err, want)
		}
	}
	if steps, _ := Execute(context.Background(), commands("regex-filter", "Pattern", `(?i)^ORDER [a-z]-\d+$`), "msg", []byte("order A-1234"), Env{}); steps[2].Logs[0] != "match" {
		t.Errorf("ignoring case: %s", ran(steps))
	}
	if _, err := Execute(context.Background(), commands("regex-filter", "Pattern", `(unclosed`), "msg", []byte("x"), Env{}); err == nil ||
		!strings.Contains(err.Error(), `Pattern "(unclosed": missing closing )`) {
		t.Errorf("bad pattern: %v", err)
	}
}

func TestPickGreater(t *testing.T) {
	pick := func(a, b string) (string, error) {
		c := &call{ctx: context.Background(), step: step("p", "pick-greater"),
			inputs: map[string]proto.Message{"A": wrapperspb.String(a), "B": wrapperspb.String(b)}, outputs: map[string]proto.Message{}}
		if err := byKind["pick-greater"].run(c); err != nil {
			return "", err
		}
		return c.outputs["Value"].(*wrapperspb.StringValue).Value, nil
	}
	for _, tc := range []struct{ a, b, want string }{
		{"3", "10", "10"}, // as numbers, not as text
		{"2.5", "-1", "2.5"},
		{"2026-10-26T09:00:00+07:00", "2026-10-25T09:00", "2026-10-26T09:00:00+07:00"},
		{"2026-10-25T09:00:00+07:00", "2026-10-25T03:00:00Z", "2026-10-25T03:00:00Z"}, // 10:00 in +07:00
		{"2026-10-25", "2026-10-24T23:59", "2026-10-25"},
		{"09:00", "17:30", "17:30"},
		{"", "2026-10-25", "2026-10-25"},
		{" 7 ", "", "7"},
		{"", "", ""},
		{"5", "5.0", "5"}, // a tie gives A
	} {
		if got, err := pick(tc.a, tc.b); err != nil || got != tc.want {
			t.Errorf("%q, %q: %q, %v; want %q", tc.a, tc.b, got, err, tc.want)
		}
	}
	for _, tc := range [][2]string{{"abc", "1"}, {"1", "2026-10-25"}} {
		if _, err := pick(tc[0], tc[1]); err == nil || !strings.Contains(err.Error(), "not two numbers or two times") {
			t.Errorf("%q, %q: %v", tc[0], tc[1], err)
		}
	}
	// In a run: a pure step, worked out when the Log needs its Value; B is a
	// message's text.
	g := &v1.Graph{
		Steps: []*v1.Step{step("msg", TelegramMessage), step("p", "pick-greater", "A", "2026-10-25T09:00"), step("log", "log")},
		Links: []*v1.Link{
			link("msg", "Event", "log", "Event"),
			link("msg", "Text", "p", "B"),
			link("p", "Value", "log", "Message"),
		},
	}
	for text, want := range map[string]string{"2026-10-26T09:00": "2026-10-26T09:00", "2026-10-24T09:00": "2026-10-25T09:00"} {
		if steps, err := Execute(context.Background(), g, "msg", []byte(text), Env{}); err != nil || ran(steps) != "msg:Event: p:: log::"+want {
			t.Errorf("%q: %s, %v; want %s", text, ran(steps), err, want)
		}
	}
}
