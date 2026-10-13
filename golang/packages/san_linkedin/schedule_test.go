package san_linkedin

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParseStated(t *testing.T) {
	for in, want := range map[string][3]string{
		"Fri, Nov 20, 9:00 AM Western Indonesia Time, based on your location":           {"Fri, Nov 20", "9:00 AM", "Western Indonesia Time"},
		"Wed, Oct 14, 12:00 AM Western Indonesia Time, based on your location":          {"Wed, Oct 14", "12:00 AM", "Western Indonesia Time"},
		" Sat, Jan 2, 11:45 PM Central European Standard Time, based on your location ": {"Sat, Jan 2", "11:45 PM", "Central European Standard Time"},
	} {
		day, clock, zone, err := parseStated(in)
		if err != nil || [3]string{day, clock, zone} != want {
			t.Errorf("parseStated(%q) = %q, %q, %q, %v; want %q", in, day, clock, zone, err, want)
		}
	}
	for _, in := range []string{"", "Fri, Nov 20, 9:00 AM", "tomorrow, based on your location"} {
		if _, _, _, err := parseStated(in); err == nil {
			t.Errorf("parseStated(%q) took it", in)
		}
	}
}

func TestWhenOf(t *testing.T) {
	if got := whenOf(" Posting Fri, Nov 20 at 9:00 AM "); got != "Fri, Nov 20 at 9:00 AM" {
		t.Errorf("whenOf = %q", got)
	}
}

// The checks before any browser opens.
func TestScheduleChecks(t *testing.T) {
	c := &Client{}
	ctx := context.Background()
	soon := time.Now().Add(time.Hour).Truncate(time.Hour)
	if err := c.Schedule(ctx, "7515808633696194561", soon.Add(10*time.Minute)); !errors.Is(err, ErrNotQuarterHour) {
		t.Errorf("10 past: %v", err)
	}
	if err := c.Schedule(ctx, "7515808633696194561", time.Now().Truncate(15*time.Minute)); !errors.Is(err, ErrTooSoon) {
		t.Errorf("now: %v", err)
	}
	if err := c.Schedule(ctx, "not-an-id", soon.Add(24*time.Hour)); err == nil {
		t.Error("a bad id was taken")
	}
}
