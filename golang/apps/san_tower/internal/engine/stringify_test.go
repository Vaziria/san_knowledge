package engine

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestStringify(t *testing.T) {
	obj, err := structpb.NewValue(map[string]any{"status": 200, "body": map[string]any{"b": "<x> & y", "a": []any{1, 2.5}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		v      *structpb.Value
		pretty bool
		want   string
	}{
		{obj, false, `{"body":{"a":[1,2.5],"b":"<x> & y"},"status":200}`},
		{obj, true, "{\n  \"body\": {\n    \"a\": [\n      1,\n      2.5\n    ],\n    \"b\": \"<x> & y\"\n  },\n  \"status\": 200\n}"},
		{structpb.NewStringValue("hi"), false, `"hi"`},
		{structpb.NewNullValue(), false, `null`},
	} {
		if got, err := stringify(c.v, c.pretty); err != nil || got != c.want {
			t.Errorf("stringify(%v, %v) = %q, %v; want %q", c.v, c.pretty, got, err, c.want)
		}
	}
}
