package san_radio

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	san_youtube "github.com/wargasipil/san_youtube"
)

// terminal is the chat pane, in the style of a two-line shell prompt. Each
// new track is typed as "play <file>", each chat comment as
// `greet @name "comment"`, which prints "Hello, @name!". A command is typed
// on the input line after the pane's prompt bar (part of the background),
// then moves up into the history:
//
//	radio@getresolved ~
//	❯ greet @budi "lagunya enak"
//	Hello, @budi!
//
// Between commands the cursor blinks on the input line.
type terminal struct {
	host        string // "radio@getresolved"
	chat, input area
	history     [historyColors]*textFile
	line        *textFile
	blocked     []string // lowercase
	log         *log.Logger

	mu     sync.Mutex
	done   []entry // typed, oldest first
	queue  []entry
	typing *entry // being typed
	typed  int    // runes of typing.command shown
	cursor bool
	wake   chan struct{}

	// renderMu keeps renders in order, so an older screen never lands after
	// a newer one.
	renderMu sync.Mutex
}

// The history's colours, one file and one drawtext each.
const (
	histHost   = iota // radio@getresolved
	histPath          // ~
	histArrow         // ❯
	histCmd           // the command
	histOutput        // Hello, @name!
	historyColors
)

var historyNames = [historyColors]string{"host", "path", "arrow", "command", "output"}
var historyColorValues = [historyColors]string{colorPink, colorBlue, colorGreen, colorWhite, colorYellow}

type entry struct {
	id, channel string // the chat message, for deletions; empty for a track
	command     string
	output      []string
}

const (
	// maxQueue is how many commands wait at most. When the chat is busier,
	// the oldest waiting one is dropped.
	maxQueue = 10
	// keepDone is how many typed commands are kept; the pane shows fewer.
	keepDone   = 100
	cursorRune = '█'
)

func newTerminal(dir, host string, chat, input area, blocked []string, logger *log.Logger) *terminal {
	t := &terminal{host: host, chat: chat, input: input, blocked: blocked, log: logger,
		cursor: true, wake: make(chan struct{}, 1), line: newTextFile(dir, "input")}
	for i, name := range historyNames {
		t.history[i] = newTextFile(dir, name)
	}
	return t
}

// hostFor is the shell's user@host for a channel: "radio@getresolved" for
// @GetResolved, "radio@radio" without one.
func hostFor(channel string) string {
	host := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		}
		return -1
	}, strings.ToLower(channel))
	if host == "" {
		host = "radio"
	}
	return "radio@" + host
}

// texts are the terminal's drawtexts.
func (t *terminal) texts() []text {
	var ts []text
	for i, f := range t.history {
		ts = append(ts, text{f, historyColorValues[i], t.chat.point})
	}
	return append(ts, text{t.line, colorWhite, t.input.point})
}

// play queues "play <name>" for a track that starts.
func (t *terminal) play(name string) {
	t.enqueue(entry{command: "play " + name})
}

// handle takes one chat message. Comments and Super Chats are greeted; a
// deleted message, or every message of a banned author, leaves the pane and
// the queue.
func (t *terminal) handle(msg san_youtube.Message) {
	switch m := msg.(type) {
	case *san_youtube.TextMessage:
		t.greet(m.Base, m.Runs, m.Text)
	case *san_youtube.SuperChat:
		t.greet(m.Base, m.Runs, m.Text)
	case *san_youtube.Replacement:
		// A message released from review.
		if m.Message != nil {
			t.handle(m.Message)
		}
	case *san_youtube.Deletion:
		t.remove(func(e entry) bool { return e.id == m.TargetID })
	case *san_youtube.AuthorDeletion:
		t.remove(func(e entry) bool { return e.channel == m.Author.ChannelID })
	}
}

func (t *terminal) greet(b san_youtube.Base, runs []san_youtube.Run, text string) {
	name := handleName(b.Author.Name)
	if name == "" {
		return
	}
	comment := commentText(runs, text)
	if w := t.blockedWord(name + " " + comment); w != "" {
		t.log.Printf("not greeting %s: the comment has the blocked word %q", name, w)
		return
	}
	cmd := "greet " + name
	if comment != "" {
		cmd += ` "` + strings.ReplaceAll(fit(comment, 200), `"`, `'`) + `"`
	}
	t.enqueue(entry{id: b.ID, channel: b.Author.ChannelID, command: cmd, output: []string{"Hello, " + name + "!"}})
}

func (t *terminal) blockedWord(s string) string {
	s = strings.ToLower(s)
	for _, w := range t.blocked {
		if strings.Contains(s, w) {
			return w
		}
	}
	return ""
}

func (t *terminal) enqueue(e entry) {
	t.mu.Lock()
	if len(t.queue) >= maxQueue {
		t.log.Printf("the chat is busy: dropping %q", t.queue[0].command)
		t.queue = t.queue[1:]
	}
	t.queue = append(t.queue, e)
	t.mu.Unlock()
	notify(t.wake)
}

func (t *terminal) remove(match func(entry) bool) {
	t.mu.Lock()
	keep := func(list []entry) []entry {
		kept := list[:0]
		for _, e := range list {
			if !match(e) {
				kept = append(kept, e)
			}
		}
		return kept
	}
	t.queue = keep(t.queue)
	t.done = keep(t.done)
	if t.typing != nil && match(*t.typing) {
		t.typing = nil
	}
	t.mu.Unlock()
	t.render()
}

// run types the queued commands until ctx is done.
func (t *terminal) run(ctx context.Context) {
	blink := time.NewTicker(500 * time.Millisecond)
	defer blink.Stop()
	for {
		t.mu.Lock()
		var next *entry
		if len(t.queue) > 0 {
			e := t.queue[0]
			t.queue = t.queue[1:]
			next = &e
		}
		t.typing, t.typed = next, 0
		waiting := len(t.queue)
		t.mu.Unlock()

		if next == nil {
			select {
			case <-ctx.Done():
				return
			case <-t.wake:
				t.setCursor(true)
			case <-blink.C:
				t.mu.Lock()
				t.cursor = !t.cursor
				t.mu.Unlock()
				t.render()
			}
			continue
		}
		t.typeOut(ctx, next, waiting)
		if ctx.Err() != nil {
			return
		}
	}
}

// typeOut types one command, about 25 characters a second, faster when
// others wait, and never longer than 3 seconds; then moves it into the
// history with its output, and leaves that to be read.
func (t *terminal) typeOut(ctx context.Context, e *entry, waiting int) {
	n := utf8.RuneCountInString(e.command)
	cps := 25.0
	if waiting > 0 {
		cps = 50
	}
	cps = max(cps, float64(n)/3)
	step := time.Duration(float64(time.Second) / cps)
	if e.id != "" {
		t.log.Printf("%s", e.command)
	}
	for i := 1; i <= n; i++ {
		t.mu.Lock()
		gone := t.typing == nil // deleted while it was typed
		t.typed, t.cursor = i, true
		t.mu.Unlock()
		if gone {
			return
		}
		t.render()
		sleep(ctx, step)
		if ctx.Err() != nil {
			return
		}
	}
	sleep(ctx, 250*time.Millisecond) // Enter

	t.mu.Lock()
	if t.typing != nil {
		t.done = append(t.done, *t.typing)
		if len(t.done) > keepDone {
			t.done = t.done[len(t.done)-keepDone:]
		}
	}
	t.typing = nil
	t.cursor = true
	t.mu.Unlock()
	t.render()

	pause := 2 * time.Second
	if waiting > 0 {
		pause = 700 * time.Millisecond
	}
	sleep(ctx, pause)
}

func (t *terminal) setCursor(on bool) {
	t.mu.Lock()
	t.cursor = on
	t.mu.Unlock()
	t.render()
}

// screen is the history's last rows lines, and the input line.
func (t *terminal) screen() (lines [][]cell, input string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Each command takes two lines at least, so these fill the pane.
	for _, e := range t.done[max(0, len(t.done)-t.chat.rows):] {
		prompt := append(cells(t.host, histHost), cell{' ', histHost})
		lines = append(lines, append(prompt, cells("~", histPath)...))
		cmd := append(cells("❯ ", histArrow), cells(e.command, histCmd)...)
		lines = append(lines, wrap(cmd, t.chat.cols)...)
		for _, o := range e.output {
			lines = append(lines, wrap(cells(o, histOutput), t.chat.cols)...)
		}
	}
	lines = lines[max(0, len(lines)-t.chat.rows):]

	if t.typing != nil {
		input = string([]rune(t.typing.command)[:t.typed]) + string(cursorRune)
	} else if t.cursor {
		input = string(cursorRune)
	}
	// A long command scrolls: the line shows its end.
	if r := []rune(input); len(r) > t.input.cols {
		input = string(r[len(r)-t.input.cols:])
	}
	return lines, input
}

// render writes the files whose text changed.
func (t *terminal) render() {
	t.renderMu.Lock()
	defer t.renderMu.Unlock()
	lines, input := t.screen()
	for i, f := range t.history {
		if err := f.write(colorText(lines, i)); err != nil {
			t.log.Printf("can't write the chat's %s text: %v", historyNames[i], err)
		}
	}
	if err := t.line.write(input); err != nil {
		t.log.Printf("can't write the chat's input line: %v", err)
	}
}
