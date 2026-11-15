package san_research

import (
	"math"
	"math/rand/v2"
	"sort"
)

// Mean is the average (NaN when empty).
func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// StdDev is the sample standard deviation (n-1).
func StdDev(xs []float64) float64 {
	if len(xs) < 2 {
		return math.NaN()
	}
	m := Mean(xs)
	s := 0.0
	for _, x := range xs {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

// Quantile is the q-th quantile (0..1) with linear interpolation between
// the sorted values (type 7, as R and numpy do by default).
func Quantile(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	return quantileSorted(s, q)
}

func quantileSorted(s []float64, q float64) float64 {
	h := q * float64(len(s)-1)
	i := int(math.Floor(h))
	if i >= len(s)-1 {
		return s[len(s)-1]
	}
	return s[i] + (h-float64(i))*(s[i+1]-s[i])
}

// Median is the 0.5 quantile.
func Median(xs []float64) float64 { return Quantile(xs, 0.5) }

// TStat is the one-sample t statistic of the mean against 0.
func TStat(xs []float64) float64 {
	sd := StdDev(xs)
	if math.IsNaN(sd) || sd == 0 {
		return math.NaN()
	}
	return Mean(xs) / (sd / math.Sqrt(float64(len(xs))))
}

// PValue is the two-sided p-value of a z or t statistic, from the normal
// distribution (fine for the hundreds-to-thousands of samples here).
func PValue(t float64) float64 {
	if math.IsNaN(t) {
		return 1
	}
	return math.Erfc(math.Abs(t) / math.Sqrt2)
}

// Bootstrap is the percentile confidence interval of the mean (level
// e.g. 0.95) from n resamples, seeded so reports are repeatable.
func Bootstrap(xs []float64, n int, level float64, seed uint64) (lo, hi float64) {
	if len(xs) == 0 {
		return math.NaN(), math.NaN()
	}
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	means := make([]float64, n)
	for i := range means {
		s := 0.0
		for range xs {
			s += xs[r.IntN(len(xs))]
		}
		means[i] = s / float64(len(xs))
	}
	sort.Float64s(means)
	a := (1 - level) / 2
	return quantileSorted(means, a), quantileSorted(means, 1-a)
}

// Bonferroni says which p-values stay significant at alpha after
// dividing alpha by the number of tests.
func Bonferroni(ps []float64, alpha float64) []bool {
	out := make([]bool, len(ps))
	for i, p := range ps {
		out[i] = p <= alpha/float64(len(ps))
	}
	return out
}

// BenjaminiHochberg says which p-values are discoveries at false
// discovery rate q: sort them, find the largest k with p(k) <= k*q/m, and
// keep the k smallest.
func BenjaminiHochberg(ps []float64, q float64) []bool {
	m := len(ps)
	idx := make([]int, m)
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return ps[idx[a]] < ps[idx[b]] })
	k := -1
	for r, i := range idx {
		if ps[i] <= float64(r+1)*q/float64(m) {
			k = r
		}
	}
	out := make([]bool, m)
	for r := 0; r <= k; r++ {
		out[idx[r]] = true
	}
	return out
}

// Autocorr is the lag-k autocorrelation of xs.
func Autocorr(xs []float64, k int) float64 {
	if k <= 0 || k >= len(xs) {
		return math.NaN()
	}
	m := Mean(xs)
	num, den := 0.0, 0.0
	for i, x := range xs {
		den += (x - m) * (x - m)
		if i >= k {
			num += (x - m) * (xs[i-k] - m)
		}
	}
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

// PropZ is the z statistic of k successes in n against the rate p0.
func PropZ(k, n int, p0 float64) float64 {
	if n == 0 || p0 <= 0 || p0 >= 1 {
		return math.NaN()
	}
	return (float64(k)/float64(n) - p0) / math.Sqrt(p0*(1-p0)/float64(n))
}

// TwoPropZ is the z statistic of rate k1/n1 against k2/n2 (pooled).
func TwoPropZ(k1, n1, k2, n2 int) float64 {
	if n1 == 0 || n2 == 0 {
		return math.NaN()
	}
	p := float64(k1+k2) / float64(n1+n2)
	se := math.Sqrt(p * (1 - p) * (1/float64(n1) + 1/float64(n2)))
	if se == 0 {
		return math.NaN()
	}
	return (float64(k1)/float64(n1) - float64(k2)/float64(n2)) / se
}

// Summary describes one bucket of values.
type Summary struct {
	N            int
	Mean, Median float64
	SD, T, P     float64
	P10, P90     float64
}

// Summarize fills a Summary.
func Summarize(xs []float64) Summary {
	s := Summary{N: len(xs), Mean: Mean(xs), SD: StdDev(xs), T: TStat(xs)}
	s.P = PValue(s.T)
	if len(xs) > 0 {
		c := append([]float64(nil), xs...)
		sort.Float64s(c)
		s.Median, s.P10, s.P90 = quantileSorted(c, 0.5), quantileSorted(c, 0.1), quantileSorted(c, 0.9)
	}
	return s
}

// GroupBy splits values into n buckets by key (keys outside 0..n-1 are
// dropped).
func GroupBy[T any](items []T, n int, key func(T) int, val func(T) (float64, bool)) [][]float64 {
	out := make([][]float64, n)
	for _, it := range items {
		k := key(it)
		if k < 0 || k >= n {
			continue
		}
		if v, ok := val(it); ok {
			out[k] = append(out[k], v)
		}
	}
	return out
}

// ATR is Wilder's average true range of period p at every bar (NaN until
// p bars), using only that bar and earlier ones.
func ATR(bars []Bar, p int) []float64 {
	out := make([]float64, len(bars))
	v := math.NaN()
	sum := 0.0
	for i, b := range bars {
		tr := b.Range()
		if i > 0 {
			pc := bars[i-1].Close
			tr = math.Max(tr, math.Max(math.Abs(b.High-pc), math.Abs(b.Low-pc)))
		}
		switch {
		case i < p:
			sum += tr
			if i == p-1 {
				v = sum / float64(p)
			}
		default:
			v = (v*float64(p-1) + tr) / float64(p)
		}
		out[i] = v
		if i < p-1 {
			out[i] = math.NaN()
		}
	}
	return out
}
