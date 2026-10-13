package san_radio

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	san_youtube "github.com/wargasipil/san_youtube"
)

// textFile is one file a drawtext reads on every frame. It is written only
// when its text changes.
type textFile struct {
	path    string
	mu      sync.Mutex
	last    string
	written bool
}

func newTextFile(dir, name string) *textFile {
	return &textFile{path: filepath.Join(dir, "san_radio-"+name+".txt")}
}

func (f *textFile) write(s string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.written && s == f.last {
		return nil
	}
	if err := writeFile(f.path, s); err != nil {
		return err
	}
	f.last, f.written = s, true
	return nil
}

// writeFile replaces path in one step, so drawtext never reads half of it.
func writeFile(path, s string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(s), 0o644); err != nil {
		return err
	}
	var err error
	for range 10 {
		if err = os.Rename(tmp, path); err == nil {
			return nil
		}
		// On Windows the rename fails while ffmpeg has the file open.
		time.Sleep(20 * time.Millisecond)
	}
	return err
}

func notify(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

var linkPattern = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)

// commentText is the part of a chat message that can be drawn: its text
// without links and emoji. The font has no colour emoji, and channel emoji
// are pictures.
func commentText(runs []san_youtube.Run, text string) string {
	if len(runs) > 0 {
		var b strings.Builder
		for _, r := range runs {
			if r.Emoji != nil || r.URL != "" {
				b.WriteString(" ")
				continue
			}
			b.WriteString(r.Text)
		}
		text = b.String()
	}
	return clean(linkPattern.ReplaceAllString(text, " "))
}

// handleName is the author as "@name", cut to 40 characters. Chat names are
// handles already; one without the "@" gets it.
func handleName(name string) string {
	name = clean(name)
	if name == "" {
		return ""
	}
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	if r := []rune(name); len(r) > 40 {
		name = string(r[:39]) + "…"
	}
	return name
}

// clean drops emoji and control characters and squeezes the spaces.
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r), unicode.IsControl(r):
			b.WriteRune(' ')
		case drawable(r):
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func drawable(r rune) bool {
	switch {
	case r == utf8.RuneError,
		r >= 0x1F000,               // emoji and pictographs
		r >= 0x2600 && r <= 0x27BF, // symbols and dingbats
		r >= 0xFE00 && r <= 0xFE0F, // variation selectors
		r == 0x200D,                // zero-width joiner
		r >= 0xE000 && r <= 0xF8FF: // private use
		return false
	}
	return unicode.IsPrint(r)
}
