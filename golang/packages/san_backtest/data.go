package san_backtest

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// Query asks a DataSource for the closed candles of one instrument and
// timeframe whose open time is in [From, To), oldest first.
type Query struct {
	Instrument string
	Timeframe  string
	From, To   time.Time
}

// DataSource gives candles and instrument conditions. Exness (through
// san_exness) and CSV are built in; a Binance or other source plugs in from
// code. Candles must be closed (never the one still forming), UTC, oldest
// first, with gaps left as gaps.
type DataSource interface {
	Candles(ctx context.Context, q Query) ([]Candle, error)
	Instrument(ctx context.Context, name string) (Instrument, error)
}

// Cacheable is a DataSource worth caching on disk (a remote one). Run wraps
// it in the Config.Cache folder.
type Cacheable interface {
	DataSource
	// CacheKey names the source in the cache path, e.g. "exness".
	CacheKey() string
	// Now is the source's clock: candles closing after it may still change.
	Now() time.Time
}

// ExnessAPI is the part of *san_exness.Client the Exness source uses.
type ExnessAPI interface {
	Candles(ctx context.Context, q exness.CandleQuery) ([]exness.Candle, error)
	Instrument(ctx context.Context, name string) (exness.InstrumentCondition, error)
	Now() time.Time
}

// Exness reads candles (bid and ask, price_type both) and instrument
// conditions through san_exness, 1000 candles per request.
type Exness struct {
	API ExnessAPI
	// Wait sleeps between rate-limit retries (tests make it instant).
	Wait func(ctx context.Context, d time.Duration) error
}

// NewExness makes the Exness source for a san_exness client.
func NewExness(api ExnessAPI) *Exness { return &Exness{API: api} }

func (e *Exness) CacheKey() string { return "exness" }
func (e *Exness) Now() time.Time   { return e.API.Now().UTC() }

func (e *Exness) Instrument(ctx context.Context, name string) (Instrument, error) {
	var c exness.InstrumentCondition
	err := e.retry(ctx, func() error {
		var err error
		c, err = e.API.Instrument(ctx, name)
		return err
	})
	if err != nil {
		return Instrument{}, err
	}
	return FromExness(c)
}

// retry runs call again (up to 4 times) when Exness says to slow down.
func (e *Exness) retry(ctx context.Context, call func() error) error {
	for i := 0; ; i++ {
		err := call()
		var ae *exness.APIError
		if err == nil || i == 3 || !errors.As(err, &ae) || (ae.Status != 429 && ae.RetryAfter <= 0) {
			return err
		}
		d := ae.RetryAfter
		if d <= 0 {
			d = time.Duration(i+1) * time.Second
		}
		wait := e.Wait
		if wait == nil {
			wait = sleep
		}
		if err := wait(ctx, d); err != nil {
			return err
		}
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (e *Exness) Candles(ctx context.Context, q Query) ([]Candle, error) {
	now := e.Now()
	var out []Candle
	from := q.From
	for from.Before(q.To) && from.Before(now) {
		var page []exness.Candle
		err := e.retry(ctx, func() error {
			var err error
			page, err = e.API.Candles(ctx, exness.CandleQuery{Instrument: q.Instrument, Timeframe: exness.Timeframe(q.Timeframe),
				PriceType: exness.PriceBoth, From: from, Count: 1000})
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("candles %s %s from %s: %w", q.Instrument, q.Timeframe, from.Format(time.RFC3339), err)
		}
		last := from
		for _, x := range page {
			c := fromExnessCandle(q.Timeframe, x)
			if c.OpenTime.Before(from) || !c.OpenTime.Before(q.To) || c.CloseTime.After(now) {
				continue
			}
			if n := len(out); n > 0 && !c.OpenTime.After(out[n-1].OpenTime) {
				continue
			}
			out = append(out, c)
			last = c.OpenTime
		}
		if len(page) < 1000 || !last.After(from) {
			break
		}
		from = closeTime(q.Timeframe, last)
	}
	return out, nil
}

func fromExnessCandle(tf string, x exness.Candle) Candle {
	c := Candle{OpenTime: x.Time.UTC(), Volume: x.Volume}
	c.CloseTime = closeTime(tf, c.OpenTime)
	if x.Bid != nil {
		c.Open, c.High, c.Low, c.Close = x.Bid.Open, x.Bid.High, x.Bid.Low, x.Bid.Close
	} else {
		c.Open, c.High, c.Low, c.Close = x.Open, x.High, x.Low, x.Close
	}
	if x.Ask != nil {
		c.AskOpen, c.AskHigh, c.AskLow, c.AskClose = x.Ask.Open, x.Ask.High, x.Ask.Low, x.Ask.Close
	} else {
		c.AskOpen, c.AskHigh, c.AskLow, c.AskClose = c.Open, c.High, c.Low, c.Close
	}
	return c
}

// CSV reads candles from files named <Dir>/<instrument>_<TF>.csv with the
// header line
//
//	time,open,high,low,close,ask_open,ask_high,ask_low,ask_close,volume
//
// (time is the open time, RFC 3339 or Unix seconds; the ask columns and
// volume may be left out, and then ask = bid). Instruments come from
// Instruments, or from <Dir>/<instrument>.json (an Instrument as JSON).
type CSV struct {
	Dir         string
	Instruments map[string]Instrument
}

func (s *CSV) Instrument(_ context.Context, name string) (Instrument, error) {
	if in, ok := s.Instruments[name]; ok {
		return in, in.validate()
	}
	b, err := os.ReadFile(filepath.Join(s.Dir, name+".json"))
	if err != nil {
		return Instrument{}, fmt.Errorf("csv: instrument %s: %w", name, err)
	}
	var in Instrument
	if err := json.Unmarshal(b, &in); err != nil {
		return Instrument{}, fmt.Errorf("csv: instrument %s: %w", name, err)
	}
	return in, in.validate()
}

func (s *CSV) Candles(_ context.Context, q Query) ([]Candle, error) {
	path := filepath.Join(s.Dir, q.Instrument+"_"+q.Timeframe+".csv")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	all, err := readCandles(f, q.Timeframe)
	if err != nil {
		return nil, fmt.Errorf("csv %s: %w", path, err)
	}
	return between(all, q.From, q.To), nil
}

// between keeps candles opening in [from, to).
func between(all []Candle, from, to time.Time) []Candle {
	i := sort.Search(len(all), func(i int) bool { return !all[i].OpenTime.Before(from) })
	j := sort.Search(len(all), func(i int) bool { return !all[i].OpenTime.Before(to) })
	return all[i:j]
}

var csvHeader = []string{"time", "open", "high", "low", "close", "ask_open", "ask_high", "ask_low", "ask_close", "volume"}

// readCandles reads the CSV shape above; lines starting with # are skipped.
func readCandles(r io.Reader, tf string) ([]Candle, error) {
	cr := csv.NewReader(bufio.NewReader(r))
	cr.Comment = '#'
	cr.FieldsPerRecord = -1
	head, err := cr.Read()
	if err == io.EOF {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	col := map[string]int{}
	for i, h := range head {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, need := range csvHeader[:5] {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("no %q column", need)
		}
	}
	var out []Candle
	for line := 2; ; line++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		get := func(name string) (float64, bool, error) {
			i, ok := col[name]
			if !ok || i >= len(rec) || rec[i] == "" {
				return 0, false, nil
			}
			v, err := strconv.ParseFloat(rec[i], 64)
			if err != nil {
				return 0, false, fmt.Errorf("line %d: %s %q", line, name, rec[i])
			}
			return v, true, nil
		}
		t, err := parseTime(rec[col["time"]])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		c := Candle{OpenTime: t, CloseTime: closeTime(tf, t)}
		vals := []*float64{&c.Open, &c.High, &c.Low, &c.Close}
		for i, name := range csvHeader[1:5] {
			v, _, err := get(name)
			if err != nil {
				return nil, err
			}
			*vals[i] = v
		}
		asks := []*float64{&c.AskOpen, &c.AskHigh, &c.AskLow, &c.AskClose}
		for i, name := range csvHeader[5:9] {
			v, ok, err := get(name)
			if err != nil {
				return nil, err
			}
			if !ok {
				v = *vals[i]
			}
			*asks[i] = v
		}
		if c.Volume, _, err = get("volume"); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && !c.OpenTime.After(out[n-1].OpenTime) {
			return nil, fmt.Errorf("line %d: %s is not after the line before", line, t.Format(time.RFC3339))
		}
		out = append(out, c)
	}
	return out, nil
}

func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0).UTC(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("time %q", s)
}

func f64(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// writeCandles writes the CSV shape readCandles reads.
func writeCandles(w io.Writer, cs []Candle) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, c := range cs {
		if err := cw.Write([]string{c.OpenTime.Format(time.RFC3339), f64(c.Open), f64(c.High), f64(c.Low), f64(c.Close),
			f64(c.AskOpen), f64(c.AskHigh), f64(c.AskLow), f64(c.AskClose), f64(c.Volume)}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
