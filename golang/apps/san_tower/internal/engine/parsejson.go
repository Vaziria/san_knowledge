package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// parseJSON is Parse JSON (the user, 2026-11-08: "have we node to string ->
// compose object", then "2", the Parse JSON proposed): it reads its Text as
// JSON into Object and goes on by Done. Text that is not JSON goes on by Not
// JSON with Error when Not JSON is linked; else the step fails with it.
func parseJSON(c *call) error {
	v, err := readJSONText(c.text("Text"), c.setting("Read") == "JSON in Text", c.setting("Expect"))
	if err != nil {
		if c.linked == nil || !c.linked("Not JSON") {
			return err
		}
		c.outputs["Error"] = stringValue(err.Error())
		c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "%v", err)
		c.fire("Not JSON")
		return nil
	}
	c.outputs["Object"] = v
	c.fire("Done")
	return nil
}

// readJSONText reads text as one JSON value. A text that is a single fenced
// code block (```json ... ```) counts as its content. With find, the JSON
// may sit in other text, as in an AI's answer: its first code block holding
// JSON, else the text from its first { to its last } (or [ to ]), repaired
// as looseJSON repairs a chat site's copy. expect is "Object", "List" or
// any ("" or "Any").
func readJSONText(text string, find bool, expect string) (*structpb.Value, error) {
	text = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "\ufeff"))
	if text == "" {
		return nil, errors.New("Text is empty")
	}
	raw := text
	if strings.HasPrefix(text, "```") && strings.HasSuffix(text, "```") {
		if blocks := codeBlocks(text); len(blocks) == 1 {
			raw = strings.TrimSpace(blocks[0])
		}
	}
	if find && !json.Valid([]byte(raw)) {
		found := ""
		for _, b := range codeBlocks(text) {
			if j := looseJSON(b); j != nil {
				found = string(j)
				break
			}
		}
		if found == "" {
			if j := looseJSON(text); j != nil {
				found = string(j)
			}
		}
		if found == "" {
			return nil, errors.New("no JSON found in Text")
		}
		raw = found
	}
	var x any
	if err := json.Unmarshal([]byte(raw), &x); err != nil {
		var se *json.SyntaxError
		if errors.As(err, &se) {
			return nil, fmt.Errorf("Text is not JSON: %s (at character %d)", strings.TrimPrefix(se.Error(), "json: "), se.Offset)
		}
		return nil, fmt.Errorf("Text is not JSON: %v", err)
	}
	switch got := jsonTypeName(x); {
	case expect == "Object" && got != "an object", expect == "List" && got != "a list":
		want := map[string]string{"Object": "an object", "List": "a list"}[expect]
		return nil, fmt.Errorf("Text is JSON but %s, not %s", got, want)
	}
	v, err := structpb.NewValue(x)
	if err != nil {
		return nil, fmt.Errorf("Text: %v", err)
	}
	return v, nil
}

// jsonTypeName names a decoded JSON value's type for a message.
func jsonTypeName(x any) string {
	switch x.(type) {
	case map[string]any:
		return "an object"
	case []any:
		return "a list"
	case string:
		return "a text"
	case float64:
		return "a number"
	case bool:
		return "true or false"
	default:
		return "null"
	}
}
