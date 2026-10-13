package san_radio

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// panels keeps the rest of the screen current: the clock in the tab bar,
// the neofetch card's Tracks, Now and Uptime, and the music tree, which
// marks the track playing:
//
//	.
//	├── ♪ arabic.wav
//	├── ♪ auntum.wav  ◀
//	└── ♪ ruin.wav
type panels struct {
	l       layout
	started time.Time
	clock   *textFile
	info    *textFile
	tree    *textFile // the tree and the names
	playing *textFile // the playing track's line
	log     *log.Logger

	mu     sync.Mutex
	files  []string
	now    string
	render sync.Mutex
}

// The tree's colours.
const (
	treeMuted = iota
	treePlaying
)

func newPanels(dir string, l layout, logger *log.Logger) *panels {
	return &panels{l: l, started: time.Now(), log: logger,
		clock: newTextFile(dir, "clock"), info: newTextFile(dir, "info"),
		tree: newTextFile(dir, "tree"), playing: newTextFile(dir, "playing")}
}

func (p *panels) texts() []text {
	return []text{
		{p.clock, colorWhite, p.l.clock},
		{p.info, colorWhite, p.l.info.point},
		{p.tree, colorMuted, p.l.tree.point},
		{p.playing, colorPink, p.l.tree.point},
	}
}

// track is told each track as it starts, with the folder's files.
func (p *panels) track(name string, files []string) {
	p.mu.Lock()
	p.now, p.files = name, files
	p.mu.Unlock()
	p.update(time.Now())
}

// run keeps the clock and the uptime current until ctx is done.
func (p *panels) run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			p.update(now)
		}
	}
}

// update writes the files whose text changed.
func (p *panels) update(now time.Time) {
	p.render.Lock()
	defer p.render.Unlock()
	p.mu.Lock()
	files, playing := p.files, p.now
	p.mu.Unlock()

	write := func(f *textFile, s string) {
		if err := f.write(s); err != nil {
			p.log.Printf("can't write %s: %v", f.path, err)
		}
	}
	write(p.clock, now.Format("Mon, Jan 2  15:04"))
	write(p.info, strings.Join([]string{
		fmt.Sprint(len(files)),
		fit(playing, p.l.info.cols),
		uptime(now.Sub(p.started)),
	}, "\n"))
	lines := treeLines(files, playing, p.l.tree.cols, p.l.tree.rows)
	write(p.tree, colorText(lines, treeMuted))
	write(p.playing, colorText(lines, treePlaying))
}

// treeLines is the music tree, the playing track in its own colour. When
// the files don't fit, it shows the part around the playing one.
func treeLines(files []string, playing string, cols, rows int) [][]cell {
	lines := [][]cell{cells(".", treeMuted)}
	shown := files
	if n := rows - 1; len(files) > n && n > 0 {
		at := 0
		for i, f := range files {
			if f == playing {
				at = i
			}
		}
		start := min(max(0, at-n/2), len(files)-n)
		shown = files[start : start+n]
	}
	for i, f := range shown {
		branch := "├── "
		if f == files[len(files)-1] && i == len(shown)-1 {
			branch = "└── "
		}
		line := cells(branch, treeMuted)
		if f == playing {
			line = append(line, cells(fit("♪ "+f, cols-7)+"  ◀", treePlaying)...)
		} else {
			line = append(line, cells(fit("♪ "+f, cols-4), treeMuted)...)
		}
		lines = append(lines, line)
	}
	return lines
}

// uptime is how long the stream has run: "12m", "3h 12m", "2d 4h".
func uptime(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m < 60:
		return fmt.Sprintf("%dm", m)
	case m < 24*60:
		return fmt.Sprintf("%dh %dm", m/60, m%60)
	}
	return fmt.Sprintf("%dd %dh", m/(24*60), m%(24*60)/60)
}

// init writes every file once, so they all exist before ffmpeg starts:
// drawtext stops the stream when a file it reads is missing.
func (p *panels) init() error {
	p.update(time.Now())
	for _, f := range []*textFile{p.clock, p.info, p.tree, p.playing} {
		if !f.written {
			return fmt.Errorf("can't write %s", f.path)
		}
	}
	return nil
}
