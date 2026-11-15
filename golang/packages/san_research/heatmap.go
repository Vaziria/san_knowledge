package san_research

import (
	"fmt"
	"image/color"
	"math"
)

// Heatmap is a grid of values: Rows x Cols labels, Values[row][col] (NaN =
// no data, drawn grey). Diverging colours red-white-blue around 0 when
// Diverge, else white to blue.
type Heatmap struct {
	Title, Note string
	Rows, Cols  []string
	Values      [][]float64
	Diverge     bool
	Format      string // cell text, e.g. "%.2f"; empty = no text
}

// Save draws the heatmap to path.
func (m Heatmap) Save(path string) error {
	const l, t, r, bot = 50, 50, 20, 40
	cw, ch := 40, 34
	w := l + r + cw*len(m.Cols)
	h := t + bot + ch*len(m.Rows)
	c := newCanvas(max(w, 600), h)
	c.text(l, 8, m.Title, ink)
	if m.Note != "" {
		c.text(l, 24, m.Note, grey)
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, row := range m.Values {
		for _, v := range row {
			if !math.IsNaN(v) {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
		}
	}
	if m.Diverge {
		a := math.Max(math.Abs(lo), math.Abs(hi))
		lo, hi = -a, a
	}
	if !(hi > lo) {
		hi = lo + 1
	}
	for i, row := range m.Values {
		y := t + i*ch
		c.text(4, y+ch/2-7, m.Rows[i], ink)
		for j, v := range row {
			x := l + j*cw
			col := color.RGBA{0xd8, 0xd8, 0xd8, 0xff}
			if !math.IsNaN(v) {
				col = shade((v-lo)/(hi-lo), m.Diverge)
			}
			c.rect(x, y, x+cw-2, y+ch-2, col)
			if m.Format != "" && !math.IsNaN(v) {
				s := fmt.Sprintf(m.Format, v)
				tc := ink
				if lum(col) < 110 {
					tc = white
				}
				c.textC(x+cw/2-1, y+ch/2-7, s, tc)
			}
		}
	}
	for j, s := range m.Cols {
		c.textC(l+j*cw+cw/2-1, t+len(m.Rows)*ch+6, s, ink)
	}
	return save(path, c)
}

func lum(c color.RGBA) float64 { return 0.3*float64(c.R) + 0.59*float64(c.G) + 0.11*float64(c.B) }

// shade maps f in 0..1 to a colour.
func shade(f float64, diverge bool) color.RGBA {
	f = math.Max(0, math.Min(1, f))
	mix := func(a, b color.RGBA, k float64) color.RGBA {
		return color.RGBA{uint8(float64(a.R) + (float64(b.R)-float64(a.R))*k), uint8(float64(a.G) + (float64(b.G)-float64(a.G))*k),
			uint8(float64(a.B) + (float64(b.B)-float64(a.B))*k), 0xff}
	}
	deep := color.RGBA{0x10, 0x4a, 0x9c, 0xff}
	if !diverge {
		return mix(white, deep, f)
	}
	if f < 0.5 {
		return mix(color.RGBA{0xb2, 0x18, 0x2b, 0xff}, white, f*2)
	}
	return mix(white, deep, (f-0.5)*2)
}

// Histogram counts values into Bins between Lo and Hi (outside values go
// to the end bins) and draws them as bars.
type Histogram struct {
	Title, Note, XLabel string
	Values              []float64
	Lo, Hi              float64
	Bins                int
	Marks               map[string]float64 // vertical marks, e.g. "median"
}

// Save draws the histogram to path.
func (hg Histogram) Save(path string) error {
	counts := make([]float64, hg.Bins)
	width := (hg.Hi - hg.Lo) / float64(hg.Bins)
	for _, v := range hg.Values {
		i := int(math.Floor((v - hg.Lo) / width))
		counts[min(max(i, 0), hg.Bins-1)]++
	}
	labels := make([]string, hg.Bins)
	step := max(1, hg.Bins/10)
	for i := range labels {
		if i%step == 0 {
			labels[i] = tickLabel(hg.Lo + width*float64(i))
		}
	}
	note := hg.Note
	for k, v := range hg.Marks {
		note += fmt.Sprintf("  %s %s", k, tickLabel(v))
	}
	cols := make([]color.RGBA, hg.Bins)
	for i := range cols {
		cols[i] = blue
	}
	return BarChart{Title: hg.Title, Note: note, YLabel: "count  (x: " + hg.XLabel + ")", Labels: labels, Values: counts, Colors: cols}.Save(path)
}

// hourLabels are "00".."23".
func hourLabels() []string {
	out := make([]string, 24)
	for i := range out {
		out[i] = fmt.Sprintf("%02d", i)
	}
	return out
}

// colorsBy picks green for flagged bars and grey for the rest.
func colorsBy(flag []bool) []color.RGBA {
	out := make([]color.RGBA, len(flag))
	for i, f := range flag {
		out[i] = color.RGBA{0xa0, 0xb8, 0xd8, 0xff}
		if f {
			out[i] = green
		}
	}
	return out
}
