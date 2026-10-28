package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// The datetime steps give a time three ways, in the tower's local time:
// Datetime in RFC 3339 (2026-10-25T09:00:00+07:00, which the post steps'
// At and CEL's timestamp() take), Date (2026-10-25) and Time (09:00).
// Their datetime, date and time inputs are pickers in the editor; they are
// text sockets all the same, so they link to and from any text.

// now is the clock, for tests.
var now = time.Now

func picker(name, kind string) *v1.Socket {
	return &v1.Socket{Name: name, Type: Text, Field: true, Picker: kind}
}

func timeOutputs() []*v1.Socket {
	return []*v1.Socket{sock("Datetime", Text), sock("Date", Text), sock("Time", Text)}
}

var datetimeKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "now", Title: "Now", Category: "Datetime",
			Description: "The time the run asks for it: Datetime (2026-10-25T09:00:00+07:00), Date and Time, in the tower's local time.",
			Outputs:     timeOutputs(),
		},
		run: func(c *call) error {
			c.putTime(now())
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "datetime", Title: "Datetime", Category: "Datetime",
			Description: "A time from a Date and a Time, picked or linked (no date: today; no time: midnight): Datetime, Date and Time.",
			Outputs:     timeOutputs(),
			Inputs:      []*v1.Socket{picker("Date", "date"), picker("Time", "time")},
		},
		run: func(c *call) error {
			t, err := dateAndTime(strings.TrimSpace(c.text("Date")), strings.TrimSpace(c.text("Time")))
			if err != nil {
				return err
			}
			c.putTime(t)
			return nil
		},
	},
	{
		StepKind: &v1.StepKind{
			Kind: "datetime-operation", Title: "Datetime Operation", Category: "Datetime",
			Description: "Adds an Amount of minutes, hours, days, weeks, months or years to a Datetime, or subtracts it: Datetime, Date and Time.",
			Outputs:     timeOutputs(),
			Settings: []*v1.Setting{
				{Name: "Operation", Options: []string{"Add", "Subtract"}},
				{Name: "Unit", Options: []string{"Days", "Hours", "Minutes", "Weeks", "Months", "Years"}},
			},
			Inputs: []*v1.Socket{picker("Datetime", "datetime"), field("Amount")},
		},
		run: datetimeOperation,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "string-to-datetime", Title: "String to Datetime", Category: "Datetime",
			Description: "Reads a time from Text (ISO, RFC 3339, Unix seconds, or your Format such as DD/MM/YYYY HH:mm) and gives it To a Datetime, Date or Time.",
			Outputs:     []*v1.Socket{sock("Value", Text)},
			Settings: []*v1.Setting{
				{Name: "To", Options: []string{"Datetime", "Date", "Time"}},
				{Name: "Format", Placeholder: "auto, or DD/MM/YYYY HH:mm"},
			},
			Inputs: []*v1.Socket{field("Text")},
		},
		run: stringToDatetime,
	},
	{
		// The user, 2026-10-27: "add node time wait", to delete a demo
		// upload a minute after it.
		StepKind: &v1.StepKind{
			Kind: "wait", Title: "Wait", Category: "Datetime",
			Description: "Waits For a while (\"30s\", \"1m\", \"2h\", \"1d\") before the run goes on by Done; a run stopped while it waits ends there.",
			Outputs:     []*v1.Socket{sock("Done", Event)},
			Inputs:      []*v1.Socket{sock("Event", Event), field("For")},
		},
		run: wait,
	},
}

// wait holds the run for its For, unless the run is stopped first.
func wait(c *call) error {
	s := strings.TrimSpace(c.text("For"))
	if s == "" {
		return fmt.Errorf("For %w", errEmpty)
	}
	d, err := days(s)
	if err != nil || d <= 0 {
		return fmt.Errorf(`For %q: give a time to wait, such as "30s", "1m", "2h" or "1d"`, s)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
	c.fire("Done")
	return nil
}

// stringToDatetime reads its Text as a time and gives it as To says: an
// RFC 3339 Datetime, a Date or a Time, in local time.
func stringToDatetime(c *call) error {
	s := strings.TrimSpace(c.text("Text"))
	if s == "" {
		return fmt.Errorf("Text %w", errEmpty)
	}
	var t time.Time
	var err error
	if f := c.setting("Format"); f != "" {
		t, err = time.ParseInLocation(goLayout(f), s, time.Local)
		if err != nil {
			return fmt.Errorf("Text %q is not in the Format %s", s, f)
		}
	} else if t, err = parseAnyTime(s); err != nil {
		return fmt.Errorf("Text: %w", err)
	}
	t = t.Local()
	switch to := c.setting("To"); to {
	case "", "Datetime":
		c.outputs["Value"] = stringValue(t.Format(time.RFC3339))
	case "Date":
		c.outputs["Value"] = stringValue(t.Format(time.DateOnly))
	case "Time":
		c.outputs["Value"] = stringValue(t.Format("15:04"))
	default:
		return fmt.Errorf("To %q: Datetime, Date or Time", to)
	}
	return nil
}

// parseAnyTime reads a datetime as parseDatetime does, a time alone (today
// at it), or Unix seconds or milliseconds (all digits, as Telegram gives a
// message's date).
func parseAnyTime(s string) (time.Time, error) {
	if t, err := parseDatetime(s); err == nil {
		return t, nil
	}
	if t, err := dateAndTime("", s); err == nil {
		return t, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if len(s) >= 13 {
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	}
	return time.Time{}, fmt.Errorf("%q is not a time this step knows: give a Format such as DD/MM/YYYY HH:mm", s)
}

// layoutTokens turn a Format's tokens into Go's layout, longest first:
// YYYY 2026, YY 26, MMMM January, MMM Jan, MM 01, M 1, DD 02, D 2, HH 15
// (24-hour), hh 03, h 3 (12-hour, with A for AM/PM), mm 04, ss 05.
var layoutTokens = strings.NewReplacer(
	"YYYY", "2006", "YY", "06",
	"MMMM", "January", "MMM", "Jan", "MM", "01", "M", "1",
	"DD", "02", "D", "2",
	"HH", "15", "hh", "03", "h", "3",
	"mm", "04", "ss", "05", "A", "PM",
)

func goLayout(format string) string { return layoutTokens.Replace(format) }

// putTime gives t as the step's Datetime, Date and Time outputs.
func (c *call) putTime(t time.Time) {
	t = t.Local()
	c.outputs["Datetime"] = stringValue(t.Format(time.RFC3339))
	c.outputs["Date"] = stringValue(t.Format(time.DateOnly))
	c.outputs["Time"] = stringValue(t.Format("15:04"))
}

// noTime gives empty Datetime, Date and Time outputs, for a time that is
// not known.
func (c *call) noTime() {
	for _, name := range []string{"Datetime", "Date", "Time"} {
		c.outputs[name] = stringValue("")
	}
}

// datetimeLayouts are the datetimes the steps read, local time unless they
// say otherwise: RFC 3339, the picker's 2026-10-25T09:00, with a space or
// seconds too, and a date alone (midnight).
var datetimeLayouts = []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02 15:04:05", time.DateOnly}

// parseDatetime reads a datetime input.
func parseDatetime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, l := range datetimeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not a datetime: give one such as 2026-10-25T09:00 or 2026-10-25T09:00:00+07:00", s)
}

// dateAndTime puts a date (2026-10-25; empty: today) and a time (09:00 or
// 09:00:00; empty: midnight) together, in local time.
func dateAndTime(date, clock string) (time.Time, error) {
	day := now().Local()
	if date != "" {
		d, err := time.ParseInLocation(time.DateOnly, date, time.Local)
		if err != nil {
			return time.Time{}, fmt.Errorf("Date %q: give one such as 2026-10-25", date)
		}
		day = d
	}
	h, m, sec := 0, 0, 0
	if clock != "" {
		t, err := time.Parse("15:04", clock)
		if err != nil {
			if t, err = time.Parse(time.TimeOnly, clock); err != nil {
				return time.Time{}, fmt.Errorf("Time %q: give one such as 09:00", clock)
			}
		}
		h, m, sec = t.Clock()
	}
	y, mo, d := day.Date()
	return time.Date(y, mo, d, h, m, sec, 0, time.Local), nil
}

func datetimeOperation(c *call) error {
	s := strings.TrimSpace(c.text("Datetime"))
	if s == "" {
		return fmt.Errorf("Datetime %w: link Now or Datetime, or pick one", errEmpty)
	}
	t, err := parseDatetime(s)
	if err != nil {
		return fmt.Errorf("Datetime: %w", err)
	}
	a := strings.TrimSpace(c.text("Amount"))
	n, err := strconv.Atoi(a)
	if err != nil {
		return fmt.Errorf("Amount %q: give a whole number, such as 1 or 30", a)
	}
	if c.setting("Operation") == "Subtract" {
		n = -n
	}
	switch unit := c.setting("Unit"); unit {
	case "Minutes":
		t = t.Add(time.Duration(n) * time.Minute)
	case "Hours":
		t = t.Add(time.Duration(n) * time.Hour)
	case "", "Days":
		t = t.AddDate(0, 0, n)
	case "Weeks":
		t = t.AddDate(0, 0, 7*n)
	case "Months":
		t = t.AddDate(0, n, 0)
	case "Years":
		t = t.AddDate(n, 0, 0)
	default:
		return fmt.Errorf("Unit %q: Minutes, Hours, Days, Weeks, Months or Years", unit)
	}
	c.putTime(t)
	return nil
}
