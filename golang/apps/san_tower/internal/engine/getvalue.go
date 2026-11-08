package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

// getValue is Get Value (the user, 2026-11-08: "Can we have Json extract
// value from path ?"): the value at Path in Object, as Value (its own type)
// and Text, with Found Yes or No. A path not there gives Default (typed or
// linked; nothing gives null), or fails the step when Missing is Fail.
func getValue(c *call) error {
	path := strings.TrimSpace(c.text("Path"))
	v, missing, err := valueAt(c.object("Object"), path)
	if err != nil {
		return err
	}
	found := "Yes"
	if missing != "" {
		if c.setting("Missing") == "Fail" {
			return fmt.Errorf("Path %s: %s", path, missing)
		}
		found = "No"
		v = structpb.NewNullValue()
		if d := c.text("Default"); d != "" {
			v = structpb.NewStringValue(d)
		}
	}
	c.outputs["Value"] = v
	c.outputs["Text"] = stringValue(jsonText(v))
	c.outputs["Found"] = stringValue(found)
	return nil
}

// pathSeg is one step of a path: a key, an index (a bare number is either,
// by what it meets), or * for every item.
type pathSeg struct {
	key   string
	index int
	isNum bool // key is a number: an index into a list, a key in an object
	quote bool // ["key"]: a key even when it reads as a number
	all   bool
}

func (s pathSeg) String() string {
	switch {
	case s.all:
		return "*"
	case s.quote && strings.ContainsAny(s.key, ".[]* "):
		return strconv.Quote(s.key)
	}
	return s.key
}

// parsePath reads a path: keys and list indexes split by dots, as
// body.items.0.title, with brackets too (items[0], items[-1] for the last,
// ["a.b"] or ['a b'] for a key holding dots or spaces), * for every item of
// a list or object, and a leading $ or $. ignored (as JSONPath writes it).
// An empty path is the whole value.
func parsePath(path string) ([]pathSeg, error) {
	p := strings.TrimSpace(path)
	p = strings.TrimPrefix(p, "$")
	p = strings.TrimPrefix(p, ".")
	var segs []pathSeg
	bare := func(t string) pathSeg {
		if t == "*" {
			return pathSeg{all: true}
		}
		if n, err := strconv.Atoi(t); err == nil {
			return pathSeg{key: t, index: n, isNum: true}
		}
		return pathSeg{key: t}
	}
	for i := 0; i < len(p); {
		switch p[i] {
		case '.':
			if i+1 >= len(p) || p[i+1] == '.' || len(segs) == 0 {
				return nil, fmt.Errorf("Path %q: an empty part, at character %d", path, i+1)
			}
			i++
		case '[':
			j := i + 1
			for j < len(p) && p[j] == ' ' {
				j++
			}
			if j < len(p) && (p[j] == '"' || p[j] == '\'') {
				q := p[j]
				var b strings.Builder
				k := j + 1
				for ; k < len(p) && p[k] != q; k++ {
					if p[k] == '\\' && k+1 < len(p) {
						k++
					}
					b.WriteByte(p[k])
				}
				end := strings.IndexByte(p[min(k+1, len(p)):], ']')
				if k >= len(p) || end < 0 || strings.TrimSpace(p[k+1:k+1+end]) != "" {
					return nil, fmt.Errorf("Path %q: a [\"key\"] not closed", path)
				}
				segs = append(segs, pathSeg{key: b.String(), quote: true})
				i = k + 1 + end + 1
				continue
			}
			end := strings.IndexByte(p[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("Path %q: a [ without ]", path)
			}
			t := strings.TrimSpace(p[i+1 : i+end])
			if t == "" {
				return nil, fmt.Errorf("Path %q: an empty [], at character %d", path, i+1)
			}
			segs = append(segs, bare(t))
			i += end + 1
		case ']':
			return nil, fmt.Errorf("Path %q: a ] without [, at character %d", path, i+1)
		default:
			j := i
			for j < len(p) && p[j] != '.' && p[j] != '[' && p[j] != ']' {
				j++
			}
			segs = append(segs, bare(strings.TrimSpace(p[i:j])))
			i = j
		}
	}
	return segs, nil
}

// valueAt is the value at path in v, or why it is not there (missing); err
// is a path that does not read.
func valueAt(v *structpb.Value, path string) (out *structpb.Value, missing string, err error) {
	segs, err := parsePath(path)
	if err != nil {
		return nil, "", err
	}
	out, missing = walkPath(v, segs, "")
	return out, missing, nil
}

// walkPath follows segs from v; at is the path so far, for the reason a
// value is missing. A text met before the path ends is read as JSON, when
// it is (JSON inside JSON, as a tool call's arguments). * gives the list of
// what the rest of the path finds in each item, skipping those without it.
func walkPath(v *structpb.Value, segs []pathSeg, at string) (*structpb.Value, string) {
	if len(segs) == 0 {
		return v, ""
	}
	where := at
	if where == "" {
		where = "the object"
	}
	if s, ok := v.GetKind().(*structpb.Value_StringValue); ok {
		var x any
		if json.Unmarshal([]byte(s.StringValue), &x) == nil {
			if j, err := structpb.NewValue(x); err == nil && (j.GetStructValue() != nil || j.GetListValue() != nil) {
				v = j
			}
		}
	}
	s, rest := segs[0], segs[1:]
	next := func(name string) string {
		if at == "" {
			return name
		}
		return at + "." + name
	}
	switch x := v.GetKind().(type) {
	case *structpb.Value_StructValue:
		f := x.StructValue.GetFields()
		if s.all {
			keys := make([]string, 0, len(f))
			for k := range f {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			return collect(len(keys), func(i int) *structpb.Value { return f[keys[i]] }, rest, next("*"))
		}
		if c, ok := f[s.key]; ok {
			return walkPath(c, rest, next(s.String()))
		}
		keys := make([]string, 0, len(f))
		for k := range f {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		has := "nothing"
		if len(keys) > 10 {
			has = strings.Join(keys[:10], ", ") + ", …"
		} else if len(keys) > 0 {
			has = strings.Join(keys, ", ")
		}
		return nil, fmt.Sprintf("no %q in %s (it has: %s)", s.key, where, has)
	case *structpb.Value_ListValue:
		items := x.ListValue.GetValues()
		if s.all {
			return collect(len(items), func(i int) *structpb.Value { return items[i] }, rest, next("*"))
		}
		if !s.isNum || s.quote {
			return nil, fmt.Sprintf("%s is a list: use a number, as %s", where, next("0"))
		}
		i := s.index
		if i < 0 {
			i += len(items)
		}
		if i < 0 || i >= len(items) {
			return nil, fmt.Sprintf("%s has %d items, no %d", where, len(items), s.index)
		}
		return walkPath(items[i], rest, next(s.key))
	case *structpb.Value_NullValue, nil:
		return nil, fmt.Sprintf("%s is null", where)
	}
	return nil, fmt.Sprintf("%s is %s, not an object or list", where, jsonTypeName(v.AsInterface()))
}

// collect walks rest from each of n items, giving the list of those found,
// or why none was.
func collect(n int, item func(int) *structpb.Value, rest []pathSeg, at string) (*structpb.Value, string) {
	var out []*structpb.Value
	why := ""
	for i := range n {
		if v, missing := walkPath(item(i), rest, at); missing == "" {
			out = append(out, v)
		} else if why == "" {
			why = missing
		}
	}
	if len(out) == 0 {
		if n == 0 {
			return nil, fmt.Sprintf("%s: no items", strings.TrimSuffix(at, ".*"))
		}
		return nil, "in no item: " + why
	}
	return structpb.NewListValue(&structpb.ListValue{Values: out}), ""
}

// jsonText is a value as text: a text as it is, a whole number without a
// decimal point, true or false, null as nothing, and an object or list as
// compact JSON.
func jsonText(v *structpb.Value) string {
	switch x := v.GetKind().(type) {
	case *structpb.Value_StringValue:
		return x.StringValue
	case *structpb.Value_NumberValue:
		f := x.NumberValue
		if f == math.Trunc(f) && math.Abs(f) < 1e15 {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	case *structpb.Value_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	case *structpb.Value_NullValue, nil:
		return ""
	}
	s, err := stringify(v, false)
	if err != nil {
		return ""
	}
	return s
}
