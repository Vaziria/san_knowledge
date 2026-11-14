package san_backtest

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// Instrument is what the simulator needs to know about a symbol: how a lot
// is sized, what it costs to hold, and when it trades. FromExness reads it
// from san_exness's InstrumentCondition.
type Instrument struct {
	Name          string
	Base          string // base currency, e.g. "XAU", "EUR"
	Quote         string // quote currency: profit is made in it, e.g. "USD"
	Digits        int    // price decimals; a point is 10^-Digits
	ContractSize  float64
	MarginPercent float64 // 100 = plain notional / leverage
	LotMin        float64
	LotMax        float64
	LotStep       float64

	// Swap per lot per night, in points (Exness's condition values; see
	// FromExness), long and short. Negative is a charge.
	SwapLong  float64
	SwapShort float64
	// SwapDays: weekday -> multiplier of a rollover on that day (3 on
	// Wednesday for most symbols). A weekday that is not listed counts 1.
	SwapDays map[time.Weekday]int

	// Commission per lot per side (charged on the open and on the close)
	// in CommissionCurrency.
	Commission         float64
	CommissionCurrency string

	// Sessions when the instrument trades, in UTC weekly offsets. Empty
	// means always.
	Sessions []WeekRange

	// Notes on how the conditions were read, shown in the report.
	Notes []string
}

// WeekRange is a span of the week in UTC, as offsets from Sunday 00:00.
type WeekRange struct{ Start, End time.Duration }

const week = 7 * 24 * time.Hour

// weekOffset is t's offset from the Sunday 00:00 UTC before it.
func weekOffset(t time.Time) time.Duration {
	t = t.UTC()
	return time.Duration(t.Weekday())*24*time.Hour + time.Duration(t.Hour())*time.Hour +
		time.Duration(t.Minute())*time.Minute + time.Duration(t.Second())*time.Second
}

// Open says whether the instrument trades at t.
func (in Instrument) Open(t time.Time) bool {
	if len(in.Sessions) == 0 {
		return true
	}
	o := weekOffset(t)
	for _, s := range in.Sessions {
		if s.Start <= s.End {
			if o >= s.Start && o < s.End {
				return true
			}
		} else if o >= s.Start || o < s.End { // wraps over Sunday 00:00
			return true
		}
	}
	return false
}

// Point is one price point (10^-Digits).
func (in Instrument) Point() float64 { return math.Pow(10, -float64(in.Digits)) }

func (in Instrument) validate() error {
	switch {
	case in.Name == "":
		return fmt.Errorf("instrument has no name")
	case in.ContractSize <= 0:
		return fmt.Errorf("instrument %s: contract size %v", in.Name, in.ContractSize)
	case in.LotStep <= 0 || in.LotMin <= 0 || in.LotMax < in.LotMin:
		return fmt.Errorf("instrument %s: lots min %v max %v step %v", in.Name, in.LotMin, in.LotMax, in.LotStep)
	case in.Quote == "":
		return fmt.Errorf("instrument %s: no quote currency", in.Name)
	}
	return nil
}

func num(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f, err == nil
}

var weekdays = map[string]time.Weekday{"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday}

// parseWeekday reads "wednesday", "Wed", "3" (0 or 7 = Sunday).
func parseWeekday(s string) (time.Weekday, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 7 {
		return time.Weekday(n % 7), true
	}
	if len(s) >= 3 {
		if d, ok := weekdays[s[:3]]; ok {
			return d, true
		}
	}
	return 0, false
}

// parseWeekTime reads a session bound such as "1 00:05:00" (the shape
// san_exness's tests use: weekday number, Monday = 1, then UTC time),
// "Monday 00:05" or "mon 00:05:00".
func parseWeekTime(s string) (time.Duration, bool) {
	f := strings.Fields(strings.ReplaceAll(s, ",", " "))
	if len(f) != 2 {
		return 0, false
	}
	d, ok := parseWeekday(f[0])
	if !ok {
		return 0, false
	}
	var h, m, sec int
	parts := strings.Split(f[1], ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	var err error
	if h, err = strconv.Atoi(parts[0]); err != nil || h < 0 || h > 24 {
		return 0, false
	}
	if m, err = strconv.Atoi(parts[1]); err != nil || m < 0 || m > 59 {
		return 0, false
	}
	if len(parts) == 3 {
		if sec, err = strconv.Atoi(parts[2]); err != nil || sec < 0 || sec > 59 {
			return 0, false
		}
	}
	return time.Duration(d)*24*time.Hour + time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, true
}

// FromExness reads an Exness instrument's conditions. How the fields are
// read (the API reference gives names, not units, so this is a reading):
//
//   - contract_size, volume_min/max/step: as is. margin_percent: percent
//     of the plain margin (empty = 100).
//   - swap_long/swap_short: points per lot per night (MT5's swap-in-points
//     mode, which Exness's contract specifications show), so the money is
//     swap x point x contract size x lots, in the quote currency.
//   - swap_multipliers: day (a weekday name or number) -> multiplier. With
//     none listed, Wednesday counts 3, the forex convention.
//   - commission_settings: commission_type "per_lot_per_side" (the shape
//     san_exness's tests use) is per lot, charged on the open and again on
//     the close; a type naming a round turn ("round", "per_trade") is split
//     in half per side; anything else is read as per lot per side and the
//     report notes it.
//   - trade_sessions: week_start_time/week_end_time as "1 00:05:00"
//     (weekday number, 0 or 7 = Sunday, then UTC time) or "Monday 00:05";
//     sessions whose trade_mode is not empty, "enabled" or "full" are not
//     open for new trades. Bounds that cannot be read drop the session
//     limit, with a note.
func FromExness(c exness.InstrumentCondition) (Instrument, error) {
	in := Instrument{Name: c.Instrument, Base: c.BaseCurrency, Quote: c.QuoteCurrency, Digits: c.PointDigits, MarginPercent: 100,
		CommissionCurrency: c.CommissionSettings.CommissionValue.Currency}
	var ok bool
	if in.ContractSize, ok = num(c.ContractSize); !ok {
		return in, fmt.Errorf("instrument %s: contract_size %q", c.Instrument, c.ContractSize)
	}
	if in.LotMin, ok = num(c.VolumeMin); !ok {
		return in, fmt.Errorf("instrument %s: volume_min %q", c.Instrument, c.VolumeMin)
	}
	if in.LotMax, ok = num(c.VolumeMax); !ok {
		return in, fmt.Errorf("instrument %s: volume_max %q", c.Instrument, c.VolumeMax)
	}
	if in.LotStep, ok = num(c.VolumeStep); !ok {
		in.LotStep = in.LotMin
	}
	if c.MarginPercent != "" {
		if v, ok := num(c.MarginPercent); ok && v > 0 {
			in.MarginPercent = v
		}
	}
	in.SwapLong, _ = num(c.SwapLong)
	in.SwapShort, _ = num(c.SwapShort)
	in.SwapDays = map[time.Weekday]int{}
	for _, m := range c.SwapMultipliers {
		d, ok := parseWeekday(m.Day)
		if !ok {
			in.Notes = append(in.Notes, fmt.Sprintf("swap multiplier day %q not understood; ignored", m.Day))
			continue
		}
		in.SwapDays[d] = m.Multiplier
	}
	if len(c.SwapMultipliers) == 0 {
		in.SwapDays[time.Wednesday] = 3
	}
	if v, ok := num(c.CommissionSettings.CommissionValue.Amount); ok {
		in.Commission = v
		t := strings.ToLower(c.CommissionSettings.CommissionType)
		switch {
		case t == "per_lot_per_side" || t == "":
		case strings.Contains(t, "round") || strings.Contains(t, "per_trade"):
			in.Commission = v / 2
			in.Notes = append(in.Notes, fmt.Sprintf("commission_type %q read as a round turn: half per side", c.CommissionSettings.CommissionType))
		default:
			in.Notes = append(in.Notes, fmt.Sprintf("commission_type %q read as per lot per side", c.CommissionSettings.CommissionType))
		}
	}
	for _, s := range c.TradeSessions {
		mode := strings.ToLower(s.TradeMode)
		if mode != "" && mode != "enabled" && mode != "full" {
			continue
		}
		a, ok1 := parseWeekTime(s.WeekStartTime)
		b, ok2 := parseWeekTime(s.WeekEndTime)
		if !ok1 || !ok2 {
			in.Notes = append(in.Notes, fmt.Sprintf("trade session %q-%q not understood; trading at any time", s.WeekStartTime, s.WeekEndTime))
			in.Sessions = nil
			break
		}
		in.Sessions = append(in.Sessions, WeekRange{a, b})
	}
	return in, in.validate()
}
