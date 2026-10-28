package chatgpt

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

var (
	//go:embed chatgpt.js
	chatgptJS string
	// copyJS runs in the page's own world (page.Session.CallMain).
	//go:embed copy.js
	copyJS string
)

var site = page.Site{Name: "chatgpt", Global: "__sanChatGPT", JS: chatgptJS, ErrPage: ErrPage}

const homeURL = "https://chatgpt.com/"

// Service asks ChatGPT in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI, the MCP server and
// the API each run one, in their own process, through page.Remote.
//
// A question works in a background tab: ChatGPT writes its answer there too,
// and reading it needs no shown page.
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

func (s *Service) open(ctx context.Context, browser, url string) (*session, error) {
	p, err := page.Open(ctx, s.B, site, browser, url, false, s.Progress)
	if err != nil {
		return nil, err
	}
	return &session{p}, nil
}

// session is the one tab a call works in, with what is known of ChatGPT.
type session struct{ *page.Session }

// state is what chatgpt.js state() reports.
type state struct {
	URL             string `json:"url"`
	Lang            string `json:"lang"`
	Composer        bool   `json:"composer"`
	SignedOut       bool   `json:"signedOut"`
	Challenge       bool   `json:"challenge"`
	Temporary       bool   `json:"temporary"`
	TemporaryButton bool   `json:"temporaryButton"`
	Turns           int    `json:"turns"`
	LastTurn        string `json:"lastTurn"`
	TurnState       string `json:"turnState"`
	Answered        bool   `json:"answered"`
	Chars           int    `json:"chars"`
	Busy            bool   `json:"busy"`
	Send            bool   `json:"send"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// newTurn says whether the page shows a turn that before did not: one more
// turn, or another last one. A long chat shows only its last 6 turns, so
// in it the count stays the same (2026-10-28: an ask in a long chat waited
// for its answer until Wait, with the answer there).
func (st state) newTurn(before state) bool {
	return st.Turns > before.Turns || (st.LastTurn != "" && st.LastTurn != before.LastTurn)
}

// waitState waits until the page's state passes ok. A call that lands while
// ChatGPT swaps the page can be refused by the browser; that is only a reason
// to ask again.
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

// ready waits for the composer, or for the signs of a signed-out page, and
// checks the interface is in English. A verification check is said once, for
// a person to clear in the browser.
func (t *session) ready(ctx context.Context) (state, error) {
	t.Log("opening ChatGPT")
	said := false
	st, err := t.waitState(ctx, "ChatGPT's composer", 2*time.Minute, func(st state) bool {
		if st.Challenge && !said {
			t.Log("ChatGPT shows a verification check; clear it in the browser")
			said = true
		}
		return st.Composer || st.SignedOut
	})
	if err != nil {
		return st, err
	}
	if st.SignedOut {
		return st, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return st, fmt.Errorf("%w: it is %q; switch it in ChatGPT's Settings, General, Language", ErrLanguage, st.Lang)
	}
	return st, nil
}

// ---- status ------------------------------------------------------------------------

// Status says whether the browser is signed in to ChatGPT, as whom, and on
// which plan.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, homeURL)
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
	if err := t.Until(ctx, "the profile button", 20*time.Second, func() (bool, error) {
		err := t.Call(ctx, &me, "account")
		return me != nil && me.Name != "", err
	}); err != nil {
		return nil, t.Fail(err)
	}
	return &Status{SignedIn: true, Name: me.Name, Plan: me.Plan}, nil
}

// ---- ask ---------------------------------------------------------------------------

// Ask sends a prompt and returns ChatGPT's answer. A new chat is a Temporary
// chat unless a.Keep says otherwise; a.Chat continues a kept chat. It works in
// a background tab, which it closes afterwards unless a.Stay, or with a.Idle
// keeps open on the chat for its next ask.
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
	return s.open(ctx, a.Browser, homeURL)
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
		if err := t.Navigate(ctx, homeURL+"c/"+id); err != nil {
			return nil, err
		}
		if st, err = t.ready(ctx); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the chat's turns", 30*time.Second, func(st state) bool { return st.Turns > 0 }); err != nil {
			return nil, fmt.Errorf("%w: %s, deleted or not this account's (%w)", ErrChatGone, a.Chat, err)
		}
	case !a.Keep && !st.Temporary:
		t.Log("starting a Temporary chat")
		if err := t.Press(ctx, "Temporary chat", "temporary"); err != nil {
			return nil, err
		}
		if _, err := t.waitState(ctx, "the Temporary chat", 20*time.Second, func(st state) bool { return st.Temporary && st.Composer }); err != nil {
			return nil, err
		}
	}
	st, err = t.state(ctx)
	if err != nil {
		return nil, err
	}
	if err := t.send(ctx, a.Prompt, st); err != nil {
		return nil, err
	}
	wait := a.Wait
	if wait == 0 {
		wait = DefaultWait
	}
	t.Log("waiting for the answer")
	if err := t.settled(ctx, st, wait); err != nil {
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
	ans := &Answer{Text: text, Temporary: st.Temporary}
	if !st.Temporary {
		if id, err := ChatID(st.URL); err == nil {
			ans.Chat = homeURL + "c/" + id
		}
	}
	return ans, nil
}

// send writes the prompt and presses Send, then waits for its turn to show:
// before is the page as it was before (newTurn). ChatGPT now and then empties the
// composer and starts nothing when Send comes right after the text
// (2026-10-25); then the prompt is written and sent once more.
func (t *session) send(ctx context.Context, prompt string, before state) error {
	for try := 1; ; try++ {
		t.Log("writing the prompt")
		if err := t.Front(ctx); err != nil {
			return err
		}
		if err := t.Do(ctx, "the composer", "write", prompt); err != nil {
			return err
		}
		var got *string
		if err := t.Until(ctx, "the prompt in the composer", composerWait(prompt), func() (bool, error) {
			err := t.Call(ctx, &got, "prompt")
			return got != nil && holds(*got, prompt), err
		}); err != nil {
			held := 0
			if got != nil {
				held = len(squash(*got))
			}
			return fmt.Errorf("%w (the composer holds %d characters, the prompt has %d)", err, held, len(squash(prompt)))
		}
		if _, err := t.waitState(ctx, "Send", 10*time.Second, func(st state) bool { return st.Send }); err != nil {
			return err
		}
		t.Log("sending the prompt")
		if err := t.Press(ctx, "Send", "send"); err != nil {
			return err
		}
		st, err := t.waitState(ctx, "the prompt's turn", 15*time.Second, func(st state) bool { return st.newTurn(before) || st.Busy })
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

// settled waits until the prompt's turn (new since before) holds
// an answer that is done: the turn complete, no Stop button, and its text
// unchanged for a moment. A turn that stays done without any answer for 10
// seconds is let through too, for answer to report what ChatGPT said instead.
func (t *session) settled(ctx context.Context, before state, wait time.Duration) error {
	var last int
	var since time.Time
	_, err := t.waitState(ctx, "the answer", wait, func(st state) bool {
		done := st.newTurn(before) && st.TurnState == "complete" && !st.Busy
		if !done || (st.Answered && st.Chars == 0) || st.Chars != last {
			last, since = st.Chars, time.Time{}
			return false
		}
		if since.IsZero() {
			since = time.Now()
		}
		if !st.Answered {
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

// answer takes the last answer with its Copy button, which gives ChatGPT's
// markdown; when that gives nothing, the answer's text as the page shows it.
// A turn that ends without an answer is ChatGPT saying no, in its own words.
func (t *session) answer(ctx context.Context) (string, error) {
	st, err := t.state(ctx)
	if err != nil {
		return "", err
	}
	if !st.Answered {
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
