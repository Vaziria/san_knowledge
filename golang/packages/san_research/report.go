package san_research

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Labels for a finding.
const (
	Strong = "STRONG" // significant after correction and stable in most years
	Weak   = "WEAK"   // nominally significant, or stable but not surviving correction
	Noise  = "NOISE"  // indistinguishable from chance
)

// Finding is one labelled fact.
type Finding struct {
	ID      string // short key, e.g. "S1-peak-hour"
	Study   int
	Label   string
	Text    string // the fact with its numbers
	Meaning string // one plain sentence for trading
	Rank    int    // >0: goes into the summary, lower first
}

// Report collects the markdown and the findings while the studies run.
type Report struct {
	Dir      string // the report folder; charts go to Dir/charts
	md       strings.Builder
	Findings []Finding
	charts   []string

	hourTests  []bucketTest // study 2's hour tests, for the approach ideas
	strong3    []string     // study 3's STRONG rows
	asian      []AsianDay   // study 3's Asian ranges
	strong4    []string
	verdict    string // one line on the backtests, for the summary
	levelStats map[string]float64
}

// NewReport starts a report in dir.
func NewReport(dir string) *Report { return &Report{Dir: dir} }

func (r *Report) p(format string, a ...any) { fmt.Fprintf(&r.md, format+"\n", a...) }

// chart saves a chart through save and embeds it.
func (r *Report) chart(name, alt string, save func(path string) error) {
	rel := "charts/" + name + ".png"
	if err := save(filepath.Join(r.Dir, filepath.FromSlash(rel))); err != nil {
		r.p("_chart %s failed: %v_\n", name, err)
		return
	}
	r.charts = append(r.charts, rel)
	r.p("![%s](%s)\n", alt, rel)
}

// table writes a markdown table.
func (r *Report) table(head []string, rows [][]string) {
	r.p("| %s |", strings.Join(head, " | "))
	seps := make([]string, len(head))
	for i := range seps {
		seps[i] = "---"
		if i > 0 {
			seps[i] = "---:"
		}
	}
	r.p("| %s |", strings.Join(seps, " | "))
	for _, row := range rows {
		r.p("| %s |", strings.Join(row, " | "))
	}
	r.p("")
}

// find records a finding and writes it in place.
func (r *Report) find(f Finding) {
	r.Findings = append(r.Findings, f)
	r.p("> **%s** - %s  \n> _For trading:_ %s\n", f.Label, f.Text, f.Meaning)
}

// Label applies the report's rule: STRONG when the test survives the
// multiple-testing correction and the sign holds in at least most of the
// years; WEAK when only one of those holds or the raw p < 0.05; NOISE
// otherwise.
func Label(corrected bool, p float64, sameSignYears, years int) string {
	stable := years > 0 && sameSignYears*5 >= years*4 // at least 80% of years (4 of 5)
	switch {
	case corrected && stable:
		return Strong
	case corrected || (p < 0.05 && stable) || p < 0.01:
		return Weak
	}
	return Noise
}

// Years in the data, for year-by-year splits.
func years(bars []Bar) []int {
	var out []int
	for _, b := range bars {
		if y := b.Time.Year(); len(out) == 0 || out[len(out)-1] != y {
			out = append(out, y)
		}
	}
	return out
}

func f1(v float64) string { return num(v, 1) }
func f2(v float64) string { return num(v, 2) }
func f3(v float64) string { return num(v, 3) }

func num(v float64, places int) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "-"
	}
	return fmt.Sprintf("%.*f", places, v)
}

func pct(v float64) string { return num(100*v, 1) + "%" }

func pv(p float64) string {
	switch {
	case p < 0.0001:
		return "<0.0001"
	case p < 0.001:
		return fmt.Sprintf("%.4f", p)
	}
	return fmt.Sprintf("%.3f", p)
}

// wibHour is the WIB hour of t.
func wibHour(t time.Time) int { return t.In(WIB).Hour() }

// wibDay is the WIB weekday of t (Monday 0 .. Saturday 5, Sunday 6).
func wibDay(t time.Time) int { return (int(t.In(WIB).Weekday()) + 6) % 7 }

var dayNames = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// bp is a price change in basis points of the reference price.
func bp(change, ref float64) float64 { return 1e4 * change / ref }

// Write saves report.md: the header and summary first, then the studies
// written so far, then what follows (approaches, limits).
func (r *Report) Write(header, tail string) (string, error) {
	path := filepath.Join(r.Dir, "report.md")
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return "", err
	}
	body := header + r.md.String() + tail
	return path, os.WriteFile(path, []byte(body), 0o644)
}

// Summary is the ranked findings as a list.
func (r *Report) Summary() string {
	var b strings.Builder
	var ranked []Finding
	for _, f := range r.Findings {
		if f.Rank > 0 {
			ranked = append(ranked, f)
		}
	}
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].Rank < ranked[i].Rank {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	for i, f := range ranked {
		fmt.Fprintf(&b, "%d. **%s** %s _%s_\n", i+1, f.Label, f.Text, f.Meaning)
	}
	return b.String()
}

// Counts is how many findings carry each label.
func (r *Report) Counts() map[string]int {
	m := map[string]int{}
	for _, f := range r.Findings {
		m[f.Label]++
	}
	return m
}
