package claude

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

var (
	//go:embed claude.js
	claudeJS string
	// copyJS runs in the page's own world (page.Session.CallMain).
	//go:embed copy.js
	copyJS string
)

var site = page.Site{Name: "claude", Global: "__sanClaude", JS: claudeJS, ErrPage: ErrPage}

const homeURL = "https://claude.ai/new"

// chatLink is a chat's address: https://claude.ai/chat/<id>, with
// ?incognito for an incognito chat.
var chatLink = regexp.MustCompile(`^https://claude\.ai/chat/([0-9a-f-]{20,})(?:[?#].*)?$`)

// Service asks Claude in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI, the MCP server and
// the API each run one, in their own process, through page.Remote.
//
// An ask works in a background tab: claude.ai sends a prompt and writes the
// answer in a hidden page too (found 2026-10-25).
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "sending the prompt".
	Progress func(step string)

	once sync.Once
	turn chan struct{}
	// kept are the tabs left open on their chat for its next ask (a.Idle).
	kept page.Kept
}

func (s *Service) begin(ctx context.Context) (done func(), err error) {
	s.once.Do(func() { s.turn = make(chan struct{}, 1) })
	select {
	case s.turn <- struct{}{}:
		return func() { <-s.turn }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) open(ctx context.Context, browser string) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, homeURL, false, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in, with what is known of claude.ai.
type session struct{ *page.Session }

// state is what claude.js state() reports.
type state struct {
	URL             string `json:"url"`
	Lang            string `json:"lang"`
	Composer        bool   `json:"composer"`
	SignedOut       bool   `json:"signedOut"`
	Incognito       bool   `json:"incognito"`
	IncognitoButton bool   `json:"incognitoButton"`
	Turns           int    `json:"turns"`
	Answers         int    `json:"answers"`
	Streaming       bool   `json:"streaming"`
	Answered        bool   `json:"answered"`
	Chars           int    `json:"chars"`
	Copy            bool   `json:"copy"`
	Busy            bool   `json:"busy"`
	Send            bool   `json:"send"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// claude.ai swaps the page can be refused by the browser; that is only a
// reason to ask again.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	var refused error
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = t.state(ctx)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			refused = err
			return false, nil
		}
		return err == nil && ok(st), err
	})
	if err != nil && refused != nil && errors.Is(err, ErrPage) {
		err = fmt.Errorf("%w (the last call was refused: %v)", err, refused)
	}
	return st, err
}

// ready waits for the prompt box, or for the signs of a signed-out page (the
// sign-in page claude.ai sends a lapsed session to), and checks the
// interface is in English.
func (t *session) ready(ctx context.Context) (state, error) {
	t.Log("opening claude.ai")
	st, err := t.waitState(ctx, "claude.ai's prompt box", 2*time.Minute, func(st state) bool { return st.Composer || st.SignedOut })
	if err != nil {
		return st, err
	}
	if st.SignedOut {
		return st, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return st, fmt.Errorf("%w: it is %q; switch it in claude.ai's Settings, Language", ErrLanguage, st.Lang)
	}
	return st, nil
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to claude.ai, as whom, and on
// which plan.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if _, err := t.ready(ctx); err != nil {
		if errors.Is(err, ErrNotSignedIn) {
			return &Status{}, nil
		}
		return nil, t.Fail(err)
	}
	var me *struct {
		Name string `json:"name"`
		Plan string `json:"plan"`
	}
	if err := t.Until(ctx, "the account button", 20*time.Second, func() (bool, error) {
		err := t.Call(ctx, &me, "account")
		return me != nil && me.Name != "", err
	}); err != nil {
		return nil, t.Fail(err)
	}
	return &Status{SignedIn: true, Name: me.Name, Plan: me.Plan}, nil
}

// ---- ask ---------------------------------------------------------------------------

// Ask sends a prompt and returns Claude's answer. The chat is an incognito
// chat unless a.Keep says otherwise. It works in a background tab, which it
// closes afterwards unless a.Stay, or with a.Idle keeps open on the chat for
// its next ask.
func (s *Service) Ask(ctx context.Context, a Ask) (*Answer, error) {
	if err := a.Check(); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.tab(ctx, a)
	if err != nil {
		return nil, err
	}
	ans, err := t.ask(ctx, a)
	err = t.Fail(err)
	switch {
	case a.Stay:
	case a.Idle > 0 && err == nil && ans.Chat != "":
		id, _ := ChatID(ans.Chat)
		s.kept.Keep(a.Browser+" "+id, t.Session, a.Idle)
	default:
		t.Close()
	}
	return ans, err
}

// tab is the tab an ask works in: the one kept open on its chat when there
// is one and it is still there, else a new one.
func (s *Service) tab(ctx context.Context, a Ask) (*session, error) {
	if id, _ := ChatID(a.Chat); id != "" {
		if p := s.kept.Take(a.Browser + " " + id); p != nil {
			t := &session{p}
			if _, err := t.state(ctx); err == nil {
				t.Log("going on in the chat's open tab")
				return t, nil
			}
			t.Close() // closed by the user, or no longer the site's page
		}
	}
	return s.open(ctx, a.Browser)
}

func (t *session) ask(ctx context.Context, a Ask) (*Answer, error) {
	st, err := t.ready(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	switch {
	case a.Chat != "":
		// A chat is opened by its link, as a person opens one they kept.
		id, _ := ChatID(a.Chat)
		if on, _ := ChatID(st.URL); on == id {
			break // the tab kept open on the chat
		}
		t.Log("opening the chat " + id)
		if err := t.Navigate(ctx, "https://claude.ai/chat/"+id); err != nil {
			return nil, err
		}
		if _, err := t.ready(ctx); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the chat's turns", 30*time.Second, func(st state) bool { return st.Answers > 0 && st.Composer }); err != nil {
			return nil, fmt.Errorf("%w: %s, deleted or not this account's (%w)", ErrChatGone, a.Chat, err)
		}
	case !a.Keep && !st.Incognito:
		t.Log("starting an incognito chat")
		if err := t.Press(ctx, "Use incognito", "incognito"); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the incognito chat", 20*time.Second, func(st state) bool { return st.Incognito && st.Composer }); err != nil {
			return nil, err
		}
	}
	st, err = t.state(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.send(ctx, a.Prompt, st.Turns); err != nil {
		return nil, err
	}
	wait := a.Wait
	if wait == 0 {
		wait = DefaultWait
	}
	t.Log("waiting for the answer")
	if err := t.settled(ctx, st.Answers, wait); err != nil {
		return nil, err
	}
	text, err := t.answer(ctx)
	if err != nil {
		return nil, err
	}
	st, err = t.state(ctx)
	if err != nil {
		return nil, err
	}
	ans := &Answer{Text: text, Incognito: st.Incognito}
	if m := chatLink.FindStringSubmatch(st.URL); m != nil && !st.Incognito {
		ans.Chat = "https://claude.ai/chat/" + m[1]
	}
	return ans, nil
}

// send writes the prompt and presses Send message, then waits for its turn
// to show: turns is how many prompts the chat had before. A prompt that went
// nowhere is written and sent once more.
func (t *session) send(ctx context.Context, prompt string, turns int) error {
	for try := 1; ; try++ {
		t.Log("writing the prompt")
		if err := t.Do(ctx, "the prompt box", "write", prompt); err != nil {
			return err
		}
		var got *string
		if err := t.Until(ctx, "the prompt in the prompt box", 10*time.Second, func() (bool, error) {
			err := t.Call(ctx, &got, "prompt")
			return got != nil && words(*got) == words(prompt), err
		}); err != nil {
			return err
		}
		if _, err := t.waitState(ctx, "Send message", 10*time.Second, func(st state) bool { return st.Send }); err != nil {
			return err
		}
		t.Log("sending the prompt")
		if err := t.Press(ctx, "Send message", "send"); err != nil {
			return err
		}
		st, err := t.waitState(ctx, "the prompt's turn", 20*time.Second, func(st state) bool { return st.Turns > turns || st.Streaming || st.Busy })
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrPage) || try == 2 {
			return err
		}
		var left *string
		if err := t.Call(ctx, &left, "prompt"); err != nil {
			return err
		}
		if st.Busy || left == nil || words(*left) != "" {
			return err
		}
		t.Log("the prompt went nowhere; writing it again")
	}
}

// settled waits until a new answer (the chat had answers before it) is done:
// no longer streaming, no Stop button, and its text unchanged for a moment.
// An answer that stays without text for 10 seconds is let through too, for
// answer to report what claude.ai said.
func (t *session) settled(ctx context.Context, answers int, wait time.Duration) error {
	var last int
	var since time.Time
	_, err := t.waitState(ctx, "the answer", wait, func(st state) bool {
		done := st.Answers > answers && !st.Streaming && !st.Busy
		if !done || st.Chars != last {
			last, since = st.Chars, time.Time{}
			return false
		}
		if since.IsZero() {
			since = time.Now()
		}
		if st.Chars == 0 {
			return time.Since(since) >= 10*time.Second
		}
		return time.Since(since) >= 1500*time.Millisecond
	})
	return err
}

// copied is what copy.js reads from the Copy button.
type copied struct {
	Text string `json:"text"`
	HTML string `json:"html"`
}

// answer takes the last answer with its Copy button, which gives Claude's
// markdown; when that gives nothing, the answer's text as the page shows it.
// An answer with no text is claude.ai saying no, in its own words.
func (t *session) answer(ctx context.Context) (string, error) {
	st, err := t.state(ctx)
	if err != nil {
		return "", err
	}
	if !st.Answered || st.Chars == 0 {
		var said string
		_ = t.Call(ctx, &said, "turnText")
		return "", fmt.Errorf("%w: %s", ErrRefused, strings.TrimSpace(said))
	}
	t.Log("copying the answer")
	var c *copied
	if err := t.CallMain(ctx, 30*time.Second, &c, copyJS); err == nil && c != nil && strings.TrimSpace(c.Text) != "" {
		return strings.TrimSpace(c.Text), nil
	}
	var text *string
	if err := t.Call(ctx, &text, "answer"); err != nil {
		return "", err
	}
	if text == nil || strings.TrimSpace(*text) == "" {
		return "", fmt.Errorf("%w: the answer is empty", ErrRefused)
	}
	return *text, nil
}
