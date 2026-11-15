package livetrading

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ClockEnv names a virtual clock for replay checks of the Live Trading page
// against a fake Exness (never with a real account): the sessions then
// live in that clock's time, faster than real time, so a fake that replays
// past candles shows trades within minutes. Its value is
// "<virtual start, RFC 3339>*<speed>@<real anchor, Unix seconds>": the
// virtual time is start + (now - anchor) x speed. Unset: real time.
const ClockEnv = "SAN_TOWER_LIVE_CLOCK"

// Clock is a virtual clock (nil: real time).
type Clock struct {
	start  time.Time
	anchor time.Time
	speed  float64
}

// ParseClock reads a ClockEnv value.
func ParseClock(s string) (*Clock, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	rest, anchorText, hasAnchor := strings.Cut(s, "@")
	startText, speedText, ok := strings.Cut(rest, "*")
	if !ok {
		return nil, fmt.Errorf("%s %q: want <start>*<speed>@<anchor>", ClockEnv, s)
	}
	start, err := time.Parse(time.RFC3339, startText)
	if err != nil {
		return nil, fmt.Errorf("%s: start: %w", ClockEnv, err)
	}
	speed, err := strconv.ParseFloat(speedText, 64)
	if err != nil || speed <= 0 {
		return nil, fmt.Errorf("%s: speed %q", ClockEnv, speedText)
	}
	anchor := time.Now()
	if hasAnchor {
		sec, err := strconv.ParseInt(anchorText, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: anchor %q", ClockEnv, anchorText)
		}
		anchor = time.Unix(sec, 0)
	}
	return &Clock{start: start.UTC(), anchor: anchor, speed: speed}, nil
}

// Now is the clock's time.
func (c *Clock) Now() time.Time {
	if c == nil {
		return time.Now().UTC()
	}
	return c.start.Add(time.Duration(float64(time.Since(c.anchor)) * c.speed)).UTC()
}

// Real is how long d of the clock's time takes.
func (c *Clock) Real(d time.Duration) time.Duration {
	if c == nil || d <= 0 {
		return d
	}
	return time.Duration(float64(d) / c.speed)
}

// clocked is an Exness client on a virtual clock.
type clocked struct {
	Exness
	c *Clock
}

func (x clocked) Now() time.Time { return x.c.Now() }

// sleep waits d of the manager's clock.
func (m *Manager) sleep(ctx context.Context, d time.Duration) { pause(ctx, m.cfg.Clock.Real(d)) }
