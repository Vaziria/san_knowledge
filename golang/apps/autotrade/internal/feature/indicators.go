// Package feature turns candles into the indicators the decision step reads.
//
// Every series function returns a slice as long as its input, with NaN where
// there is not enough history yet.
package feature

import "math"

func nans(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	return out
}

// EMA is the exponential moving average over n periods, seeded with the
// simple average of the first n values. Leading NaNs in v are skipped, so it
// can smooth another indicator.
func EMA(v []float64, n int) []float64 {
	out := nans(len(v))
	start := 0
	for start < len(v) && math.IsNaN(v[start]) {
		start++
	}
	if n <= 0 || len(v)-start < n {
		return out
	}
	sum := 0.0
	for _, x := range v[start : start+n] {
		sum += x
	}
	i := start + n - 1
	out[i] = sum / float64(n)
	k := 2 / float64(n+1)
	for i++; i < len(v); i++ {
		out[i] = v[i]*k + out[i-1]*(1-k)
	}
	return out
}

// SMA is the simple moving average over n periods.
func SMA(v []float64, n int) []float64 {
	out := nans(len(v))
	if n <= 0 || len(v) < n {
		return out
	}
	sum := 0.0
	for i, x := range v {
		sum += x
		if i >= n {
			sum -= v[i-n]
		}
		if i >= n-1 {
			out[i] = sum / float64(n)
		}
	}
	return out
}

// RSI is Wilder's relative strength index over n periods.
func RSI(closes []float64, n int) []float64 {
	out := nans(len(closes))
	if n <= 0 || len(closes) <= n {
		return out
	}
	var gain, loss float64
	for i := 1; i <= n; i++ {
		g, l := move(closes[i] - closes[i-1])
		gain += g
		loss += l
	}
	gain /= float64(n)
	loss /= float64(n)
	out[n] = rsi(gain, loss)
	for i := n + 1; i < len(closes); i++ {
		g, l := move(closes[i] - closes[i-1])
		gain = (gain*float64(n-1) + g) / float64(n)
		loss = (loss*float64(n-1) + l) / float64(n)
		out[i] = rsi(gain, loss)
	}
	return out
}

func move(d float64) (gain, loss float64) {
	if d > 0 {
		return d, 0
	}
	return 0, -d
}

func rsi(gain, loss float64) float64 {
	if loss == 0 {
		if gain == 0 {
			return 50
		}
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

// ATR is Wilder's average true range over n periods.
func ATR(high, low, close []float64, n int) []float64 {
	out := nans(len(close))
	if n <= 0 || len(close) <= n {
		return out
	}
	tr := func(i int) float64 {
		pc := close[i-1]
		return math.Max(high[i]-low[i], math.Max(math.Abs(high[i]-pc), math.Abs(low[i]-pc)))
	}
	sum := 0.0
	for i := 1; i <= n; i++ {
		sum += tr(i)
	}
	out[n] = sum / float64(n)
	for i := n + 1; i < len(close); i++ {
		out[i] = (out[i-1]*float64(n-1) + tr(i)) / float64(n)
	}
	return out
}

// MACD returns the MACD line (fast EMA minus slow EMA), its signal line and
// the histogram (line minus signal).
func MACD(closes []float64, fast, slow, signal int) (line, sig, hist []float64) {
	f, s := EMA(closes, fast), EMA(closes, slow)
	line = make([]float64, len(closes))
	for i := range closes {
		line[i] = f[i] - s[i] // NaN until the slow EMA starts
	}
	sig = EMA(line, signal)
	hist = make([]float64, len(closes))
	for i := range closes {
		hist[i] = line[i] - sig[i]
	}
	return line, sig, hist
}

// Bollinger returns the n-period SMA and the bands k population standard
// deviations above and below it.
func Bollinger(closes []float64, n int, k float64) (mid, upper, lower []float64) {
	mid = SMA(closes, n)
	upper, lower = nans(len(closes)), nans(len(closes))
	for i := n - 1; i < len(closes) && n > 0; i++ {
		var ss float64
		for _, x := range closes[i-n+1 : i+1] {
			ss += (x - mid[i]) * (x - mid[i])
		}
		sd := math.Sqrt(ss / float64(n))
		upper[i], lower[i] = mid[i]+k*sd, mid[i]-k*sd
	}
	return mid, upper, lower
}
