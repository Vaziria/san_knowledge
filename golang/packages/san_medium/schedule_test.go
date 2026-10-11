package san_medium

import (
	"testing"
	"time"
)

func TestParseShown(t *testing.T) {
	jakarta := time.FixedZone("", 7*3600)
	for in, want := range map[string]time.Time{
		"10/18/2026, 9:00 AM GMT+7":   time.Date(2026, 10, 18, 9, 0, 0, 0, jakarta),
		"10/11/2026, 2:59 PM GMT+7":   time.Date(2026, 10, 11, 14, 59, 0, 0, jakarta),
		"1/2/2027, 12:05 AM GMT+7":    time.Date(2027, 1, 2, 0, 5, 0, 0, jakarta),
		"1/2/2027, 12:05 PM GMT+5:30": time.Date(2027, 1, 2, 12, 5, 0, 0, time.FixedZone("", 5*3600+1800)),
		"3/4/2027, 7:15 PM GMT-3":     time.Date(2027, 3, 4, 19, 15, 0, 0, time.FixedZone("", -3*3600)),
		"3/4/2027, 7:15 PM GMT":       time.Date(2027, 3, 4, 19, 15, 0, 0, time.UTC),
	} {
		got, err := parseShown(in)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseShown(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "tomorrow", "10/18/2026 9:00 AM"} {
		if _, err := parseShown(in); err == nil {
			t.Errorf("parseShown(%q) took it", in)
		}
	}
}
