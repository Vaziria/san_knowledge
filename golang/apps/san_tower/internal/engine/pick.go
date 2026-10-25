package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// pickGreater gives the greater of A and B as it came: two numbers are
// compared as numbers, two times (2026-10-25T09:00:00+07:00, 2026-10-25,
// 09:00) as the moments they name. An empty one gives the other, so a time
// that is not known (Last Scheduled Post's None) leaves the other one; a tie
// gives A.
func pickGreater(c *call) error {
	a, b := strings.TrimSpace(c.text("A")), strings.TrimSpace(c.text("B"))
	v := a
	switch {
	case a == "":
		v = b
	case b == "":
	default:
		greater, err := greaterOf(a, b)
		if err != nil {
			return err
		}
		v = greater
	}
	c.outputs["Value"] = stringValue(v)
	return nil
}

func greaterOf(a, b string) (string, error) {
	if x, err := strconv.ParseFloat(a, 64); err == nil {
		if y, err := strconv.ParseFloat(b, 64); err == nil {
			return choose(a, b, y > x), nil
		}
	}
	if x, err := parseMoment(a); err == nil {
		if y, err := parseMoment(b); err == nil {
			return choose(a, b, y.After(x)), nil
		}
	}
	return "", fmt.Errorf("A %q and B %q are not two numbers or two times", a, b)
}

func choose(a, b string, takeB bool) string {
	if takeB {
		return b
	}
	return a
}

// parseMoment reads a datetime as parseDatetime does, or a time alone
// (today at it).
func parseMoment(s string) (time.Time, error) {
	if t, err := parseDatetime(s); err == nil {
		return t, nil
	}
	return dateAndTime("", s)
}
