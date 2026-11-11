package db

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
)

// TextArray is a Postgres text[] for GORM, which would expand a []string
// argument into a list. It is written as an array literal, which the SQL
// casts (?::text::text[]), and read from one (SELECT x::text).
type TextArray []string

var quoter = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func (a TextArray) Value() (driver.Value, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, s := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(quoter.Replace(s))
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

func (a *TextArray) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		*a = nil
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("text array: cannot read %T", src)
	}
	list, err := parseTextArray(s)
	if err != nil {
		return fmt.Errorf("text array %q: %w", s, err)
	}
	*a = list
	return nil
}

// parseTextArray reads a one-dimensional array literal: {a,"b c","d\"e"}.
func parseTextArray(s string) ([]string, error) {
	if len(s) < 2 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil, errors.New("not an array")
	}
	body := s[1 : len(s)-1]
	list := []string{}
	if body == "" {
		return list, nil
	}
	for i := 0; ; {
		var el strings.Builder
		if i < len(body) && body[i] == '"' {
			i++
			for ; i < len(body) && body[i] != '"'; i++ {
				if body[i] == '\\' {
					i++
					if i == len(body) {
						return nil, errors.New("unfinished escape")
					}
				}
				el.WriteByte(body[i])
			}
			if i == len(body) {
				return nil, errors.New("unfinished quote")
			}
			i++
		} else {
			start := i
			for i < len(body) && body[i] != ',' {
				i++
			}
			raw := body[start:i]
			if strings.EqualFold(raw, "NULL") || raw == "" || strings.ContainsAny(raw, "{}") {
				return nil, errors.New("unsupported element " + raw)
			}
			el.WriteString(raw)
		}
		list = append(list, el.String())
		if i == len(body) {
			return list, nil
		}
		if body[i] != ',' {
			return nil, errors.New("expected a comma")
		}
		i++
	}
}
