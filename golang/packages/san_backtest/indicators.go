package san_backtest

import (
	"fmt"
	"math"
	"time"
)

// series is one timeframe's candles and the indicators on them. Only the
// first n candles are visible (closed at the current step); indicators
// catch up on the newly visible candles when read, so each candle is fed
// once (incremental), and nothing past n is ever touched.
type series struct {
	tf   string
	all  []Candle
	n    int
	inds map[string]indicator
}

type indicator interface {
	feed(c Candle)
}

func newSeries(tf string, all []Candle) *series {
	return &series{tf: tf, all: all, inds: map[string]indicator{}}
}

func (s *series) closed() []Candle { return s.all[:s.n:s.n] }

// fedIndicator remembers how far it has been fed.
type fedIndicator struct{ fed int }

// catchUp returns the indicator under key (made by mk the first time),
// fed with every candle that closed since it was last read.
func catchUp[T interface {
	indicator
	pos() *int
}](s *series, key string, mk func() T) T {
	ind, ok := s.inds[key]
	if !ok {
		ind = mk()
		s.inds[key] = ind
	}
	t := ind.(T)
	p := t.pos()
	for ; *p < s.n; *p++ {
		t.feed(s.all[*p])
	}
	return t
}

func (f *fedIndicator) pos() *int { return &f.fed }

// ema: seeded with the SMA of the first period values.
type ema struct {
	fedIndicator
	period int
	k      float64
	sum    float64
	count  int
	value  float64
}

func newEMA(period int) *ema {
	return &ema{period: period, k: 2 / float64(period+1), value: math.NaN()}
}

func (e *ema) add(v float64) {
	e.count++
	if e.count <= e.period {
		e.sum += v
		if e.count == e.period {
			e.value = e.sum / float64(e.period)
		}
		return
	}
	e.value = v*e.k + e.value*(1-e.k)
}

func (e *ema) feed(c Candle) { e.add(c.Close) }

type sma struct {
	fedIndicator
	period int
	ring   []float64
	sum    float64
	count  int
}

func newSMA(period int) *sma { return &sma{period: period, ring: make([]float64, period)} }

func (m *sma) add(v float64) {
	i := m.count % m.period
	m.sum += v - m.ring[i]
	m.ring[i] = v
	m.count++
}
func (m *sma) feed(c Candle) { m.add(c.Close) }
func (m *sma) value() float64 {
	if m.count < m.period {
		return math.NaN()
	}
	return m.sum / float64(m.period)
}

// stddev of the same window (population), for Bollinger.
func (m *sma) stddev() float64 {
	if m.count < m.period {
		return math.NaN()
	}
	mean := m.sum / float64(m.period)
	var ss float64
	for _, v := range m.ring {
		ss += (v - mean) * (v - mean)
	}
	return math.Sqrt(ss / float64(m.period))
}

// rsi: Wilder's smoothing.
type rsi struct {
	fedIndicator
	period         int
	prev           float64
	count          int
	gain, loss     float64
	avgG, avgL, rs float64
	value          float64
}

func newRSI(period int) *rsi { return &rsi{period: period, value: math.NaN()} }

func (r *rsi) feed(c Candle) {
	r.count++
	if r.count == 1 {
		r.prev = c.Close
		return
	}
	ch := c.Close - r.prev
	r.prev = c.Close
	g, l := math.Max(ch, 0), math.Max(-ch, 0)
	n := r.count - 1
	switch {
	case n < r.period:
		r.gain += g
		r.loss += l
		return
	case n == r.period:
		r.avgG, r.avgL = (r.gain+g)/float64(r.period), (r.loss+l)/float64(r.period)
	default:
		p := float64(r.period)
		r.avgG, r.avgL = (r.avgG*(p-1)+g)/p, (r.avgL*(p-1)+l)/p
	}
	if r.avgL == 0 {
		r.value = 100
		return
	}
	r.value = 100 - 100/(1+r.avgG/r.avgL)
}

// atr: Wilder's average of the true range.
type atr struct {
	fedIndicator
	period    int
	prevClose float64
	count     int
	sum       float64
	value     float64
}

func newATR(period int) *atr { return &atr{period: period, value: math.NaN()} }

func (a *atr) feed(c Candle) {
	tr := c.High - c.Low
	if a.count > 0 {
		tr = math.Max(tr, math.Max(math.Abs(c.High-a.prevClose), math.Abs(c.Low-a.prevClose)))
	}
	a.prevClose = c.Close
	a.count++
	switch {
	case a.count < a.period:
		a.sum += tr
	case a.count == a.period:
		a.value = (a.sum + tr) / float64(a.period)
	default:
		p := float64(a.period)
		a.value = (a.value*(p-1) + tr) / p
	}
}

type macd struct {
	fedIndicator
	fast, slow, signal *ema
}

func newMACD(fast, slow, signal int) *macd {
	return &macd{fast: newEMA(fast), slow: newEMA(slow), signal: newEMA(signal)}
}

func (m *macd) feed(c Candle) {
	m.fast.add(c.Close)
	m.slow.add(c.Close)
	if !math.IsNaN(m.slow.value) && !math.IsNaN(m.fast.value) {
		m.signal.add(m.fast.value - m.slow.value)
	}
}

func (m *macd) values() (line, signal, hist float64) {
	line = m.fast.value - m.slow.value
	if math.IsNaN(m.slow.value) {
		line = math.NaN()
	}
	signal = m.signal.value
	return line, signal, line - signal
}

// Swing is a confirmed swing high or low: a candle whose high (low) is
// above (below) the Strength candles on each side. It is known only once
// the Strength candles after it have closed, so there is no look-ahead.
type Swing struct {
	Time      time.Time // the swing candle's open time
	Price     float64
	Confirmed time.Time // close time of the candle that confirmed it
}

type swings struct {
	fedIndicator
	strength   int
	buf        []Candle
	highs, lws []Swing
}

func (w *swings) feed(c Candle) {
	w.buf = append(w.buf, c)
	k := w.strength
	if len(w.buf) < 2*k+1 {
		return
	}
	if len(w.buf) > 2*k+1 {
		w.buf = w.buf[1:]
	}
	mid := w.buf[k]
	hi, lo := true, true
	for i, x := range w.buf {
		if i == k {
			continue
		}
		if x.High >= mid.High {
			hi = false
		}
		if x.Low <= mid.Low {
			lo = false
		}
	}
	if hi {
		w.highs = append(w.highs, Swing{Time: mid.OpenTime, Price: mid.High, Confirmed: c.CloseTime})
	}
	if lo {
		w.lws = append(w.lws, Swing{Time: mid.OpenTime, Price: mid.Low, Confirmed: c.CloseTime})
	}
}

func key(kind string, a ...any) string { return fmt.Sprint(kind, a) }
