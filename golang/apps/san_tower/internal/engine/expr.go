package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/types"
	"cel.dev/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/structpb"
)

// Expressions are CEL (cel.dev/cel-go): `payload.total > 1000000`,
// `payload.items.size()`. The step's object input is the variable payload,
// read as JSON, so a missing key is an error and numbers are doubles, which
// compare with integers. Format Text's text input is the variable text.

var celEnv = sync.OnceValues(func() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("payload", cel.DynType),
		cel.Variable("text", cel.StringType),
		cel.CrossTypeNumericComparisons(true),
	)
})

// eval evaluates expr with the payload p.
func eval(expr string, p *structpb.Value) (ref.Val, error) {
	return evalVars(expr, map[string]any{"payload": p})
}

// evalVars evaluates expr with vars; a variable not given is an error where
// expr reads it.
func evalVars(expr string, vars map[string]any) (ref.Val, error) {
	env, err := celEnv()
	if err != nil {
		return nil, err
	}
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		return nil, fmt.Errorf("%s: %w", expr, iss.Err())
	}
	prg, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", expr, err)
	}
	out, _, err := prg.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", expr, err)
	}
	return out, nil
}

// evalBool evaluates a condition.
func evalBool(expr string, p *structpb.Value) (bool, error) {
	if expr == "" {
		return false, fmt.Errorf("If %w", errEmpty)
	}
	v, err := eval(expr, p)
	if err != nil {
		return false, err
	}
	b, ok := v.(types.Bool)
	if !ok {
		return false, fmt.Errorf("%s gives %s %s, not true or false", expr, v.Type().TypeName(), format(v))
	}
	return bool(b), nil
}

// interpolate replaces each {{ expression }} in text with its value, with
// the payload p.
func interpolate(text string, p *structpb.Value) (string, error) {
	return interpolateVars(text, map[string]any{"payload": p})
}

// interpolateVars replaces each {{ expression }} in text with its value,
// with vars.
func interpolateVars(text string, vars map[string]any) (string, error) {
	var b strings.Builder
	for {
		start := strings.Index(text, "{{")
		if start < 0 {
			b.WriteString(text)
			return b.String(), nil
		}
		end := strings.Index(text[start:], "}}")
		if end < 0 {
			return "", fmt.Errorf("a {{ has no }}: %s", text[start:])
		}
		b.WriteString(text[:start])
		v, err := evalVars(strings.TrimSpace(text[start+2:start+end]), vars)
		if err != nil {
			return "", err
		}
		b.WriteString(format(v))
		text = text[start+end+2:]
	}
}

// format writes a value as text: strings as they are, whole numbers without
// a decimal point, null as nothing, lists and maps as JSON.
func format(v ref.Val) string {
	switch x := v.(type) {
	case types.String:
		return string(x)
	case types.Double:
		f := float64(x)
		if f == math.Trunc(f) && math.Abs(f) < 1e15 {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case types.Null:
		return ""
	case types.Int, types.Uint, types.Bool:
		return fmt.Sprint(x.Value())
	}
	if j, err := v.ConvertToNative(reflect.TypeFor[*structpb.Value]()); err == nil {
		if b, err := json.Marshal(j.(*structpb.Value).AsInterface()); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v.Value())
}
