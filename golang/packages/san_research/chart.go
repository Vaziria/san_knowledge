package san_research

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Charts are drawn with the standard library's image packages, as
// san_backtest's equity chart is. The one dependency, golang.org/x/image
// (already used elsewhere in the repo), gives a built-in bitmap font
// (basicfont 7x13) so titles and axis labels can carry letters; it needs no
// font files and no cgo.

var (
	white = color.RGBA{0xff, 0xff, 0xff, 0xff}
	ink   = color.RGBA{0x30, 0x30, 0x30, 0xff}
	grid  = color.RGBA{0xe4, 0xe4, 0xe4, 0xff}
	blue  = color.RGBA{0x1f, 0x6f, 0xd0, 0xff}
	red   = color.RGBA{0xd0, 0x3b, 0x2f, 0xff}
	green = color.RGBA{0x2e, 0x9b, 0x4f, 0xff}
	grey  = color.RGBA{0x80, 0x80, 0x80, 0xff}
)

type canvas struct{ *image.RGBA }

func newCanvas(w, h int) canvas {
	c := canvas{image.NewRGBA(image.Rect(0, 0, w, h))}
	draw.Draw(c.RGBA, c.Rect, &image.Uniform{white}, image.Point{}, draw.Src)
	return c
}

func (c canvas) rect(x0, y0, x1, y1 int, col color.RGBA) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	draw.Draw(c.RGBA, image.Rect(x0, y0, x1+1, y1+1), &image.Uniform{col}, image.Point{}, draw.Src)
}

func (c canvas) hline(x0, x1, y int, col color.RGBA) { c.rect(x0, y, x1, y, col) }
func (c canvas) vline(x, y0, y1 int, col color.RGBA) { c.rect(x, y0, x, y1, col) }

// text draws s with its top-left at (x, y).
func (c canvas) text(x, y int, s string, col color.RGBA) {
	d := font.Drawer{Dst: c.RGBA, Src: &image.Uniform{col}, Face: basicfont.Face7x13,
		Dot: fixed.P(x, y+11)}
	d.DrawString(s)
}

func textW(s string) int { return len(s) * 7 }

// textC centres s on x.
func (c canvas) textC(x, y int, s string, col color.RGBA) { c.text(x-textW(s)/2, y, s, col) }

func save(path string, c canvas) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, c.RGBA); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// niceTicks gives about n round tick values covering lo..hi.
func niceTicks(lo, hi float64, n int) []float64 {
	if hi <= lo {
		hi = lo + 1
	}
	raw := (hi - lo) / float64(n)
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := mag
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if m*mag >= raw {
			step = m * mag
			break
		}
	}
	var out []float64
	for v := math.Ceil(lo/step) * step; v <= hi+step*1e-9; v += step {
		out = append(out, v)
	}
	return out
}

func tickLabel(v float64) string {
	a := math.Abs(v)
	switch {
	case a == 0:
		return "0"
	case a >= 100:
		return fmt.Sprintf("%.0f", v)
	case a >= 1:
		return fmt.Sprintf("%.1f", v)
	case a >= 0.01:
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%.3f", v)
}

// BarChart is one bar per label, with optional error bars (Lo/Hi, same
// length as Values) and a second series drawn as dots (Line).
type BarChart struct {
	Title, YLabel string
	Labels        []string
	Values        []float64
	Lo, Hi        []float64 // error bars, optional
	Line          []float64 // optional second series (grey dots joined)
	LineLabel     string
	Colors        []color.RGBA // per bar, optional (default blue, red when negative)
	Note          string       // small text under the title
}

// Save draws the chart to path (PNG).
func (b BarChart) Save(path string) error {
	const w, h, l, r, t, bot = 1000, 460, 80, 20, 50, 50
	c := newCanvas(w, h)
	c.text(l, 8, b.Title, ink)
	if b.Note != "" {
		c.text(l, 24, b.Note, grey)
	}
	lo, hi := 0.0, 0.0
	all := append(append(append(append([]float64{}, b.Values...), b.Lo...), b.Hi...), b.Line...)
	for _, v := range all {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
	}
	if hi == lo {
		hi = lo + 1
	}
	pad := (hi - lo) * 0.05
	if lo < 0 {
		lo -= pad
	}
	hi += pad
	x0, x1, y0, y1 := l, w-r, h-bot, t
	py := func(v float64) int { return y0 - int(float64(y0-y1)*(v-lo)/(hi-lo)) }
	for _, v := range niceTicks(lo, hi, 6) {
		y := py(v)
		c.hline(x0, x1, y, grid)
		s := tickLabel(v)
		c.text(x0-6-textW(s), y-7, s, ink)
	}
	c.hline(x0, x1, py(0), ink)
	c.vline(x0, y1, y0, ink)
	n := len(b.Values)
	slot := float64(x1-x0) / float64(max(n, 1))
	bw := max(int(slot*0.7), 1)
	cx := func(i int) int { return x0 + int(slot*(float64(i)+0.5)) }
	for i, v := range b.Values {
		if math.IsNaN(v) {
			continue
		}
		col := blue
		if v < 0 {
			col = red
		}
		if i < len(b.Colors) {
			col = b.Colors[i]
		}
		c.rect(cx(i)-bw/2, py(0), cx(i)+bw/2, py(v), col)
		if i < len(b.Lo) && i < len(b.Hi) && !math.IsNaN(b.Lo[i]) {
			c.vline(cx(i), py(b.Hi[i]), py(b.Lo[i]), ink)
			c.hline(cx(i)-3, cx(i)+3, py(b.Hi[i]), ink)
			c.hline(cx(i)-3, cx(i)+3, py(b.Lo[i]), ink)
		}
	}
	for i := range b.Line {
		if math.IsNaN(b.Line[i]) {
			continue
		}
		c.rect(cx(i)-2, py(b.Line[i])-2, cx(i)+2, py(b.Line[i])+2, ink)
		if i > 0 && !math.IsNaN(b.Line[i-1]) {
			line(c, cx(i-1), py(b.Line[i-1]), cx(i), py(b.Line[i]), grey)
		}
	}
	every := max(1, (n*textW("00")+(x1-x0)-1)/(x1-x0))
	for i, s := range b.Labels {
		if i%every == 0 || textW(s) < int(slot) {
			c.textC(cx(i), y0+6, s, ink)
		}
	}
	if b.YLabel != "" {
		c.text(4, t-18, b.YLabel, grey)
	}
	if b.LineLabel != "" {
		c.text(x1-textW(b.LineLabel)-4, t-18, "o "+b.LineLabel, ink)
	}
	return save(path, c)
}

func line(c canvas, x0, y0, x1, y1 int, col color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sgn(x1-x0), sgn(y1-y0)
	e := dx + dy
	for {
		c.Set(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			return
		}
		if e2 := 2 * e; e2 >= dy {
			e += dy
			x0 += sx
		} else {
			e += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sgn(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}
