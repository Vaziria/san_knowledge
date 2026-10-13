package san_radio

import (
	"fmt"
	"strings"
)

// The live text's colours, from the background's palette.
const (
	colorPink   = "0xff7ac6"
	colorBlue   = "0x5ccfe6"
	colorGreen  = "0x9ece6a"
	colorWhite  = "0xe6e6f0"
	colorYellow = "0xe0af68"
	colorMuted  = "0x8b8fb5"
)

// DejaVu Sans Mono's advance is 0.602 of the font size, and drawtext
// advances a line by its height, 1.164. No line_spacing: drawtext (ffmpeg
// 7.1) leaves it out of the text's height and cuts off the bottom lines.
const (
	monoAdvance = 0.602
	lineAdvance = 1.164
)

type point struct{ x, y int }

// area is a block of text: its top left corner, and how many characters
// fit.
type area struct {
	point
	cols, rows int
}

// layout is where the live text goes on the background,
// background/terminal.html, which leaves room for it.
type layout struct {
	size  int // font size
	clock point
	info  area // the neofetch card's Tracks, Now and Uptime values
	tree  area // the music tree
	chat  area // the chat's history
	input area // the chat's input line, after its prompt bar
}

// newLayout is the background's layout at 1280x720, in DejaVu Sans Mono
// 16px (19px a line), scaled to w x h. The places are the LIVE ones in
// terminal.html: move one, move both.
func newLayout(w, h int) layout {
	sx := func(x int) int { return x * w / 1280 }
	sy := func(y int) int { return y * h / 720 }
	size := max(8, (16*h+360)/720)
	adv, line := float64(size)*monoAdvance, float64(size)*lineAdvance
	a := func(x, y, width, height int) area {
		return area{point{sx(x), sy(y)}, max(1, int(float64(sx(width))/adv)), max(1, int(float64(sy(height))/line))}
	}
	return layout{
		size:  size,
		clock: point{sx(896), sy(29)},
		info:  a(470, 227, 170, 57),
		tree:  a(672, 118, 578, 228),
		chat:  a(38, 416, 1204, 228),
		input: a(226, 666, 1016, 19),
	}
}

// text is one drawtext: a file's text in one colour, from a corner.
type text struct {
	file  *textFile
	color string
	at    point
}

// videoFilter scales the background to the stream's size and frame rate and
// draws the live text over it. y_align=font puts the lines of every text at
// the same heights, whatever characters each holds, so texts of different
// colours at one place line up.
func videoFilter(c Config, size int, texts []text) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[0:v]scale=%d:%d,setsar=1,fps=%d,format=yuv420p", c.Width, c.Height, c.FPS)
	font := filterValue(c.Font)
	for _, t := range texts {
		fmt.Fprintf(&b, ",drawtext=fontfile=%s:textfile=%s:reload=1:expansion=none:text_shaping=0:fontsize=%d:"+
			"y_align=font:fontcolor=%s:x=%d:y=%d",
			font, filterValue(t.file.path), size, t.color, t.at.x, t.at.y)
	}
	b.WriteString("[v]")
	return b.String()
}

// cell is one character on a screen, in one colour.
type cell struct {
	r     rune
	color int
}

func cells(s string, color int) []cell {
	var cs []cell
	for _, r := range s {
		cs = append(cs, cell{r, color})
	}
	return cs
}

// wrap breaks a line every cols characters, as a terminal does.
func wrap(cs []cell, cols int) [][]cell {
	var out [][]cell
	for len(cs) > cols {
		out = append(out, cs[:cols])
		cs = cs[cols:]
	}
	return append(out, cs)
}

// colorText is the lines as one colour's file: its characters in place, the
// others as spaces, so in a monospace font the colours line up.
func colorText(lines [][]cell, color int) string {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		var l strings.Builder
		for _, c := range line {
			if c.color == color {
				l.WriteRune(c.r)
			} else {
				l.WriteByte(' ')
			}
		}
		b.WriteString(strings.TrimRight(l.String(), " "))
	}
	return b.String()
}

// fit cuts s to n characters, ending with "…" when it was longer.
func fit(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return string(r[:n-1]) + "…"
}
