package san_radio

import (
	"io"
	"log"
	"os"
	"testing"
	"time"
)

func TestTreeLines(t *testing.T) {
	files := []string{"a.wav", "b.wav", "c.wav"}
	lines := treeLines(files, "b.wav", 40, 10)
	if got, want := colorText(lines, treeMuted), ".\n├── ♪ a.wav\n├──\n└── ♪ c.wav"; got != want {
		t.Errorf("tree = %q, want %q", got, want)
	}
	if got, want := colorText(lines, treePlaying), "\n\n    ♪ b.wav  ◀\n"; got != want {
		t.Errorf("playing = %q, want %q", got, want)
	}

	// Too many files: the part around the playing one, still ending in └──
	// only at the real last file.
	many := []string{"1.wav", "2.wav", "3.wav", "4.wav", "5.wav", "6.wav"}
	lines = treeLines(many, "6.wav", 40, 4)
	if got, want := colorText(lines, treeMuted), ".\n├── ♪ 4.wav\n├── ♪ 5.wav\n└──"; got != want {
		t.Errorf("scrolled tree = %q, want %q", got, want)
	}
	lines = treeLines(many, "1.wav", 40, 4)
	if got, want := colorText(lines, treeMuted), ".\n├──\n├── ♪ 2.wav\n├── ♪ 3.wav"; got != want {
		t.Errorf("tree at the top = %q, want %q", got, want)
	}
}

func TestUptime(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second:             "0m",
		12 * time.Minute:             "12m",
		3*time.Hour + 12*time.Minute: "3h 12m",
		52 * time.Hour:               "2d 4h",
	}
	for d, want := range cases {
		if got := uptime(d); got != want {
			t.Errorf("uptime(%s) = %q, want %q", d, got, want)
		}
	}
}

func TestFit(t *testing.T) {
	if got := fit("mistjapanese.wav", 10); got != "mistjapan…" {
		t.Errorf("fit = %q", got)
	}
	if got := fit("ruin.wav", 10); got != "ruin.wav" {
		t.Errorf("fit = %q", got)
	}
}

func TestPanelsWrite(t *testing.T) {
	p := newPanels(t.TempDir(), newLayout(1280, 720), log.New(io.Discard, "", 0))
	if err := p.init(); err != nil {
		t.Fatal(err)
	}
	p.track("auntum.wav", []string{"arabic.wav", "auntum.wav"})
	info, _ := os.ReadFile(p.info.path)
	if string(info) != "2\nauntum.wav\n0m" {
		t.Errorf("info = %q", info)
	}
	clock, _ := os.ReadFile(p.clock.path)
	if _, err := time.Parse("Mon, Jan 2  15:04", string(clock)); err != nil {
		t.Errorf("clock = %q: %v", clock, err)
	}
}
