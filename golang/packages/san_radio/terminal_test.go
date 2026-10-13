package san_radio

import (
	"context"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	san_youtube "github.com/wargasipil/san_youtube"
)

func testTerminal(t *testing.T, host string, cols, rows int, blocked ...string) *terminal {
	t.Helper()
	term := newTerminal(t.TempDir(), host, area{cols: cols, rows: rows}, area{cols: 20, rows: 1}, blocked, log.New(io.Discard, "", 0))
	term.render()
	return term
}

func comment(id, channel, name, text string) *san_youtube.TextMessage {
	return &san_youtube.TextMessage{
		Base: san_youtube.Base{ID: id, Author: san_youtube.Author{Name: name, ChannelID: channel}},
		Text: text,
	}
}

func TestHostFor(t *testing.T) {
	cases := map[string]string{
		"@GetResolved": "radio@getresolved",
		"":             "radio@radio",
		"@Lofi_Girl!":  "radio@lofigirl",
	}
	for in, want := range cases {
		if got := hostFor(in); got != want {
			t.Errorf("hostFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGreetCommand(t *testing.T) {
	term := testTerminal(t, "r@h", 80, 10)
	term.handle(comment("1", "UC1", "@budi", `lagu "enak" banget`))
	term.handle(&san_youtube.SuperChat{Base: san_youtube.Base{ID: "2", Author: san_youtube.Author{Name: "@sari"}}})
	term.handle(&san_youtube.Replacement{TargetID: "x", Message: comment("3", "UC3", "@dewi", "lolos review")})
	term.handle(&san_youtube.Membership{Base: san_youtube.Base{ID: "4", Author: san_youtube.Author{Name: "@eko"}}})
	want := []string{`greet @budi "lagu 'enak' banget"`, "greet @sari", `greet @dewi "lolos review"`}
	if len(term.queue) != len(want) {
		t.Fatalf("queue = %+v", term.queue)
	}
	for i, w := range want {
		if term.queue[i].command != w {
			t.Errorf("command %d = %q, want %q", i, term.queue[i].command, w)
		}
	}
	if o := term.queue[0].output; len(o) != 1 || o[0] != "Hello, @budi!" {
		t.Errorf("output = %q", o)
	}
}

// TestScreen checks the history's two-line prompts and that the colours'
// files line up: each holds its own characters, in the same rows and
// columns, and spaces elsewhere.
func TestScreen(t *testing.T) {
	term := testTerminal(t, "r@h", 12, 5)
	term.done = []entry{
		{command: "play a.wav"},
		{command: `greet @b "hi"`, output: []string{"Hello, @b!"}},
	}
	term.typing, term.typed = &entry{command: "play c.wav"}, 4
	lines, input := term.screen()
	// The first "r@h ~" scrolls away; "❯ play a.wav", "r@h ~",
	// `❯ greet @b "`, `hi"` and "Hello, @b!" stay.
	want := [historyColors]string{
		"\nr@h\n\n\n",
		"\n    ~\n\n\n",
		"❯\n\n❯\n\n",
		"  play a.wav\n\n  greet @b \"\nhi\"\n",
		"\n\n\n\nHello, @b!",
	}
	for i := range historyColors {
		if got := colorText(lines, i); got != want[i] {
			t.Errorf("%s = %q, want %q", historyNames[i], got, want[i])
		}
	}
	if input != "play█" {
		t.Errorf("input = %q, want the typed part and the cursor", input)
	}

	// A long command scrolls on the input line.
	term.typing, term.typed = &entry{command: strings.Repeat("x", 30)}, 30
	if _, input = term.screen(); input != strings.Repeat("x", 19)+"█" {
		t.Errorf("long input = %q", input)
	}
}

func TestDeletionLeavesThePane(t *testing.T) {
	term := testTerminal(t, "r@h", 80, 10)
	term.done = []entry{
		{id: "1", channel: "UC1", command: `greet @spam "kata kasar"`, output: []string{"Hello, @spam!"}},
		{id: "2", channel: "UC2", command: `greet @dewi "halo"`, output: []string{"Hello, @dewi!"}},
	}
	term.queue = []entry{{id: "3", channel: "UC1", command: `greet @spam "lagi"`}}
	term.handle(&san_youtube.Deletion{TargetID: "1"})
	lines, _ := term.screen()
	if s := colorText(lines, histCmd); strings.Contains(s, "kata kasar") || !strings.Contains(s, "dewi") {
		t.Errorf("after the deletion the pane reads %q", s)
	}
	term.handle(&san_youtube.AuthorDeletion{Base: san_youtube.Base{Author: san_youtube.Author{ChannelID: "UC1"}}})
	if len(term.queue) != 0 {
		t.Errorf("the banned author's comment still waits: %+v", term.queue)
	}
}

func TestBlockedWord(t *testing.T) {
	term := testTerminal(t, "r@h", 80, 10, "judi")
	term.handle(comment("1", "UC1", "@x", "main JUDI online"))
	term.handle(comment("2", "UC2", "@judi_slot", "halo"))
	if len(term.queue) != 0 {
		t.Fatalf("queue = %+v, want nothing", term.queue)
	}
}

func TestBusyChatDropsOldest(t *testing.T) {
	term := testTerminal(t, "r@h", 80, 10)
	for i := range maxQueue + 3 {
		term.handle(comment(string(rune('a'+i)), "UC", "@v"+string(rune('a'+i)), "hai"))
	}
	if len(term.queue) != maxQueue || !strings.HasPrefix(term.queue[0].command, "greet @vd") {
		t.Fatalf("queue has %d, first %q", len(term.queue), term.queue[0].command)
	}
}

// TestRunTypes runs the terminal: a track and a comment are typed into the
// files ffmpeg reads, and the comment's output printed.
func TestRunTypes(t *testing.T) {
	term := testTerminal(t, "r@h", 80, 10)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		term.run(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	term.play("arabic.wav")
	term.handle(comment("1", "UC1", "@budi", "halo"))
	waitFile(t, term.history[histOutput].path, "Hello, @budi!")
	cmd, _ := os.ReadFile(term.history[histCmd].path)
	if !strings.Contains(string(cmd), "play arabic.wav") || !strings.Contains(string(cmd), `greet @budi "halo"`) {
		t.Errorf("command text = %q", cmd)
	}
}

func waitFile(t *testing.T, file, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		b, _ := os.ReadFile(file)
		if strings.Contains(string(b), want) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s reads %q, want %q in it", file, b, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
