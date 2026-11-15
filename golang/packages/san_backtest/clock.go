package san_backtest

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // session hours follow London and New York daylight saving on any machine
)

// WIB is Western Indonesia Time, UTC+7, San's clock. Everything inside the
// framework is UTC; WIB is for display and for rules written in WIB.
var WIB = time.FixedZone("WIB", 7*60*60)

var (
	tokyo, _   = time.LoadLocation("Asia/Tokyo")
	london, _  = time.LoadLocation("Europe/London")
	newYork, _ = time.LoadLocation("America/New_York")
)

// Session says which trading sessions are open at a moment. Hours are
// local to each centre, so daylight saving moves them in UTC: Asia is
// Tokyo 09:00-18:00, London 08:00-17:00 London time, New York 08:00-17:00
// New York time. Overlaps have more than one set.
type Session struct {
	Asia, London, NewYork bool
}

// SessionAt is the session at t (weekends included: callers check Gap or
// the instrument's hours for closed markets).
func SessionAt(t time.Time) Session {
	in := func(loc *time.Location, from, to int) bool {
		h := t.In(loc).Hour()
		return h >= from && h < to
	}
	return Session{Asia: in(tokyo, 9, 18), London: in(london, 8, 17), NewYork: in(newYork, 8, 17)}
}

// Overlap is true when two sessions are open at once.
func (s Session) Overlap() bool {
	return (s.Asia && s.London) || (s.London && s.NewYork) || (s.Asia && s.NewYork)
}

func (s Session) String() string {
	var p []string
	if s.Asia {
		p = append(p, "Asia")
	}
	if s.London {
		p = append(p, "London")
	}
	if s.NewYork {
		p = append(p, "New York")
	}
	if len(p) == 0 {
		return "none"
	}
	return strings.Join(p, "+")
}

// rollovers lists the swap moments in (from, to]: 17:00 New York time
// (22:00 UTC in winter, 21:00 in summer, when Exness charges swaps),
// Monday to Friday.
func rollovers(from, to time.Time) []time.Time {
	var out []time.Time
	d := from.In(newYork)
	day := time.Date(d.Year(), d.Month(), d.Day(), 17, 0, 0, 0, newYork)
	for ; !day.After(to); day = day.AddDate(0, 0, 1) {
		if day.After(from) && day.Weekday() != time.Saturday && day.Weekday() != time.Sunday {
			out = append(out, day.UTC())
		}
	}
	return out
}

// Rollovers lists the daily swap moments in (from, to]: 17:00 New York
// time, Monday to Friday (04:00 or 05:00 WIB).
func Rollovers(from, to time.Time) []time.Time { return rollovers(from, to) }

// Last is the range of the last d up to now: (now - d, now), UTC, to the
// minute.
func Last(d time.Duration) (from, to time.Time) {
	to = time.Now().UTC().Truncate(time.Minute)
	return to.Add(-d), to
}

// ParseSpan reads a length such as "90d", "12h", "2w", "3m" (months of 30
// days, not minutes) or "1y" (365 days). Go durations ("90m" is not one of
// them: m is months here) are not accepted, so a slip cannot mean minutes.
func ParseSpan(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) < 2 {
		return 0, fmt.Errorf("length %q (like 90d, 12h, 2w, 3m, 1y)", s)
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("length %q (like 90d, 12h, 2w, 3m, 1y)", s)
	}
	unit := map[byte]time.Duration{'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour, 'm': 30 * 24 * time.Hour, 'y': 365 * 24 * time.Hour}[s[len(s)-1]]
	if unit == 0 {
		return 0, fmt.Errorf("length %q (like 90d, 12h, 2w, 3m, 1y)", s)
	}
	return time.Duration(n) * unit, nil
}

// ParseZone reads "UTC", "WIB" or an IANA name such as "Asia/Jakarta".
func ParseZone(s string) (*time.Location, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "UTC":
		return time.UTC, nil
	case "WIB":
		return WIB, nil
	}
	return time.LoadLocation(strings.TrimSpace(s))
}
