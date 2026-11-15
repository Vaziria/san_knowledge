package main

import (
	"testing"
	"time"

	exness "github.com/wargasipil/san_exness"
)

func TestWaitFor(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		name    string
		retry   time.Duration
		attempt int
		want    time.Duration
	}{
		{"seconds", 7 * time.Second, 0, 7 * time.Second},
		{"none: backoff", 0, 2, 15 * time.Second},
		{"unix time read as seconds", time.Duration(1_800_000_030) * time.Second, 0, 31 * time.Second},
		{"capped", time.Hour, 0, 15 * time.Minute},
	}
	for _, c := range cases {
		if got := waitFor(&exness.APIError{Status: 429, RetryAfter: c.retry}, now, c.attempt); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}
