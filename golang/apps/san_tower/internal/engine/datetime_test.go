package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// runKind runs a step of kind with values (settings and typed inputs) and
// gives its Datetime, Date and Time.
func runKind(t *testing.T, kind string, values ...string) ([3]string, error) {
	t.Helper()
	st := step("s", kind, values...)
	c := &call{ctx: context.Background(), step: st, inputs: map[string]proto.Message{}, outputs: map[string]proto.Message{}}
	for k, v := range st.Values {
		c.inputs[k] = wrapperspb.String(v)
	}
	if err := byKind[kind].run(c); err != nil {
		return [3]string{}, err
	}
	var out [3]string
	for i, name := range []string{"Datetime", "Date", "Time"} {
		out[i] = c.outputs[name].(*wrapperspb.StringValue).Value
	}
	return out, nil
}

func TestDatetimeSteps(t *testing.T) {
	defer func(f func() time.Time) { now = f }(now)
	clock := time.Date(2026, 10, 25, 9, 30, 15, 0, time.Local)
	now = func() time.Time { return clock }
	local := func(y int, mo time.Month, d, h, m int) [3]string {
		tm := time.Date(y, mo, d, h, m, 0, 0, time.Local)
		return [3]string{tm.Format(time.RFC3339), tm.Format(time.DateOnly), tm.Format("15:04")}
	}

	if got, err := runKind(t, "now"); err != nil || got != [3]string{clock.Format(time.RFC3339), "2026-10-25", "09:30"} {
		t.Errorf("now: %v, %v", got, err)
	}

	for _, c := range []struct {
		date, clock string
		want        [3]string
	}{
		{"2026-11-20", "09:00", local(2026, 11, 20, 9, 0)},
		{"2026-11-20", "", local(2026, 11, 20, 0, 0)},
		{"", "18:45", local(2026, 10, 25, 18, 45)},
	} {
		if got, err := runKind(t, "datetime", "Date", c.date, "Time", c.clock); err != nil || got != c.want {
			t.Errorf("datetime %q %q: %v, %v; want %v", c.date, c.clock, got, err, c.want)
		}
	}
	if _, err := runKind(t, "datetime", "Date", "20/11/2026"); err == nil || !strings.Contains(err.Error(), "Date") {
		t.Errorf("bad date: %v", err)
	}

	utc := time.Date(2026, 10, 25, 2, 0, 0, 0, time.UTC).Add(24 * time.Hour).Local()
	for _, c := range []struct {
		datetime, op, unit, amount string
		want                       [3]string
	}{
		{"2026-10-25T09:00", "Add", "Days", "1", local(2026, 10, 26, 9, 0)},
		{"2026-10-25T09:00", "Subtract", "Hours", "2", local(2026, 10, 25, 7, 0)},
		{"2026-10-25 09:00", "Add", "Minutes", "90", local(2026, 10, 25, 10, 30)},
		{"2026-10-25", "Add", "Weeks", "1", local(2026, 11, 1, 0, 0)},
		{"2026-10-25T09:00", "Add", "Months", "1", local(2026, 11, 25, 9, 0)},
		{"2026-10-25T09:00", "Subtract", "Years", "1", local(2025, 10, 25, 9, 0)},
		{"2026-10-25T02:00:00Z", "Add", "Days", "1", [3]string{utc.Format(time.RFC3339), utc.Format(time.DateOnly), utc.Format("15:04")}},
	} {
		got, err := runKind(t, "datetime-operation", "Datetime", c.datetime, "Operation", c.op, "Unit", c.unit, "Amount", c.amount)
		if err != nil || got != c.want {
			t.Errorf("%s %s %s %s: %v, %v; want %v", c.datetime, c.op, c.amount, c.unit, got, err, c.want)
		}
	}
	for values, says := range map[[2]string]string{
		{"", "1"}:                 "Datetime is empty",
		{"soon", "1"}:             "not a datetime",
		{"2026-10-25T09:00", "x"}: "whole number",
	} {
		if _, err := runKind(t, "datetime-operation", "Datetime", values[0], "Amount", values[1]); err == nil || !strings.Contains(err.Error(), says) {
			t.Errorf("%v: %v; want %q", values, err, says)
		}
	}
}

func TestStringToDatetime(t *testing.T) {
	defer func(f func() time.Time) { now = f }(now)
	now = func() time.Time { return time.Date(2026, 10, 25, 9, 30, 0, 0, time.Local) }
	value := func(text, to, format string) (string, error) {
		t.Helper()
		st := step("s", "string-to-datetime", "To", to, "Format", format)
		c := &call{ctx: context.Background(), step: st, inputs: map[string]proto.Message{"Text": wrapperspb.String(text)}, outputs: map[string]proto.Message{}}
		if err := stringToDatetime(c); err != nil {
			return "", err
		}
		return c.outputs["Value"].(*wrapperspb.StringValue).Value, nil
	}
	nov20 := time.Date(2026, 11, 20, 9, 0, 0, 0, time.Local)
	unix := time.Unix(1795000000, 0).Local()
	for _, c := range []struct{ text, to, format, want string }{
		{"2026-11-20 09:00", "Datetime", "", nov20.Format(time.RFC3339)},
		{"2026-11-20T09:00", "Date", "", "2026-11-20"},
		{"2026-11-20T09:00:00+07:00", "Time", "", time.Date(2026, 11, 20, 2, 0, 0, 0, time.UTC).Local().Format("15:04")},
		{"2026-11-20", "Datetime", "", time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local).Format(time.RFC3339)},
		{"18:45", "Datetime", "", time.Date(2026, 10, 25, 18, 45, 0, 0, time.Local).Format(time.RFC3339)},
		{"1795000000", "Datetime", "", unix.Format(time.RFC3339)},
		{"1795000000000", "Date", "", unix.Format(time.DateOnly)},
		{"20/11/2026 09:00", "Datetime", "DD/MM/YYYY HH:mm", nov20.Format(time.RFC3339)},
		{"20 November 2026, 9:00 AM", "Time", "D MMMM YYYY, h:mm A", "09:00"},
		{"2026-11-20 09:00", "", "", nov20.Format(time.RFC3339)},
	} {
		if got, err := value(c.text, c.to, c.format); err != nil || got != c.want {
			t.Errorf("%q to %q (format %q): %q, %v; want %q", c.text, c.to, c.format, got, err, c.want)
		}
	}
	for _, c := range []struct{ text, format, says string }{
		{"", "", "Text is empty"},
		{"next friday", "", "give a Format"},
		{"2026-11-20", "DD/MM/YYYY", "not in the Format DD/MM/YYYY"},
	} {
		if _, err := value(c.text, "Datetime", c.format); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%q (format %q): %v; want %q", c.text, c.format, err, c.says)
		}
	}
}

func TestWait(t *testing.T) {
	waitFor := func(ctx context.Context, d string) (*call, time.Duration, error) {
		st := step("s", "wait", "For", d)
		c := &call{ctx: ctx, step: st, inputs: map[string]proto.Message{"For": wrapperspb.String(d)}, outputs: map[string]proto.Message{}}
		start := time.Now()
		err := byKind["wait"].run(c)
		return c, time.Since(start), err
	}

	c, took, err := waitFor(context.Background(), "50ms")
	if err != nil || took < 50*time.Millisecond || len(c.fired) != 1 || c.fired[0] != "Done" {
		t.Errorf("50ms: took %s, fired %v, %v", took, c.fired, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	c, took, err = waitFor(ctx, "1h")
	if err == nil || took > time.Second || len(c.fired) != 0 {
		t.Errorf("stopped while waiting: took %s, fired %v, %v", took, c.fired, err)
	}

	for _, d := range []string{"", "soon", "0s", "-1m"} {
		if _, _, err := waitFor(context.Background(), d); err == nil || !strings.Contains(err.Error(), "For") {
			t.Errorf("For %q: %v", d, err)
		}
	}
}
