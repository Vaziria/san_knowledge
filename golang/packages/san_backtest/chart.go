package san_backtest

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"time"
)

// The equity chart uses only the standard library (image, image/png): a
// small built-in digit font labels the axes, so no chart or font module is
// needed.

const (
	chartW, chartH       = 1200, 600
	padL, padR           = 90, 20
	padT, padB           = 20, 40
	glyphScale           = 2
	glyphW, glyphH       = 3, 5
	glyphGap             = 1
	gridLines            = 5
	labelColor, gridGray = 0x30, 0xe0
)

// glyphs: 3x5 pixels per character, one string per row.
var glyphs = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"}, '1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"}, '3': {"###", "..#", "###", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"}, '5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"}, '7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"}, '9': {"###", "#.#", "###", "..#", "###"},
	'-': {"...", "...", "###", "...", "..."}, '.': {"...", "...", "...", "...", ".#."},
	':': {"...", ".#.", "...", ".#.", "..."}, ' ': {"...", "...", "...", "...", "..."},
}

type canvas struct{ *image.RGBA }

func (c canvas) dot(x, y int, col color.RGBA) {
	if image.Pt(x, y).In(c.Rect) {
		c.SetRGBA(x, y, col)
	}
}

func (c canvas) line(x0, y0, x1, y1 int, col color.RGBA) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for {
		c.dot(x0, y0, col)
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

func (c canvas) text(x, y int, s string, col color.RGBA) {
	for _, r := range s {
		g, ok := glyphs[r]
		if !ok {
			g = glyphs[' ']
		}
		for row := 0; row < glyphH; row++ {
			for colx := 0; colx < glyphW; colx++ {
				if g[row][colx] == '#' {
					for a := 0; a < glyphScale; a++ {
						for b := 0; b < glyphScale; b++ {
							c.dot(x+colx*glyphScale+a, y+row*glyphScale+b, col)
						}
					}
				}
			}
		}
		x += (glyphW + glyphGap) * glyphScale
	}
}

func textWidth(s string) int { return len(s) * (glyphW + glyphGap) * glyphScale }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// WritePNG draws the equity curve (blue) and the balance (grey) over time,
// with the money on the left and dates (UTC) along the bottom.
func (r *Result) WritePNG(out io.Writer) error {
	img := canvas{image.NewRGBA(image.Rect(0, 0, chartW, chartH))}
	for i := range img.Pix {
		img.Pix[i] = 0xff // white, opaque
	}
	ink := color.RGBA{labelColor, labelColor, labelColor, 0xff}
	grid := color.RGBA{gridGray, gridGray, gridGray, 0xff}
	blue := color.RGBA{0x1f, 0x6f, 0xd0, 0xff}
	grey := color.RGBA{0x99, 0x99, 0x99, 0xff}
	x0, x1, y0, y1 := padL, chartW-padR, chartH-padB, padT
	eq := r.Equity
	if len(eq) == 0 {
		eq = []EquityPoint{{Time: r.Range.From, Balance: r.Stats.StartBalance, Equity: r.Stats.StartBalance},
			{Time: r.Range.To, Balance: r.Stats.StartBalance, Equity: r.Stats.StartBalance}}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, e := range eq {
		lo, hi = math.Min(lo, math.Min(e.Equity, e.Balance)), math.Max(hi, math.Max(e.Equity, e.Balance))
	}
	if hi-lo < 1e-9 {
		lo, hi = lo-1, hi+1
	}
	pad := (hi - lo) * 0.05
	lo, hi = lo-pad, hi+pad
	t0, t1 := eq[0].Time, eq[len(eq)-1].Time
	if !t1.After(t0) {
		t1 = t0.Add(time.Hour)
	}
	px := func(t time.Time) int { return x0 + int(float64(x1-x0)*float64(t.Sub(t0))/float64(t1.Sub(t0))) }
	py := func(v float64) int { return y0 - int(float64(y0-y1)*(v-lo)/(hi-lo)) }
	for i := 0; i <= gridLines; i++ {
		v := lo + (hi-lo)*float64(i)/gridLines
		y := py(v)
		img.line(x0, y, x1, y, grid)
		label := fmt.Sprintf("%.2f", v)
		img.text(x0-8-textWidth(label), y-glyphH*glyphScale/2, label, ink)
		t := t0.Add(time.Duration(float64(t1.Sub(t0)) * float64(i) / gridLines))
		x := px(t)
		img.line(x, y0, x, y1, grid)
		d := t.UTC().Format("2006-01-02")
		lx := min(max(x-textWidth(d)/2, 0), chartW-textWidth(d))
		img.text(lx, y0+12, d, ink)
	}
	img.line(x0, y0, x1, y0, ink)
	img.line(x0, y0, x0, y1, ink)
	draw := func(val func(EquityPoint) float64, col color.RGBA) {
		px0, py0 := px(eq[0].Time), py(val(eq[0]))
		for _, e := range eq[1:] {
			a, b := px(e.Time), py(val(e))
			img.line(px0, py0, a, b, col)
			px0, py0 = a, b
		}
	}
	draw(func(e EquityPoint) float64 { return e.Balance }, grey)
	draw(func(e EquityPoint) float64 { return e.Equity }, blue)
	return png.Encode(out, img)
}
