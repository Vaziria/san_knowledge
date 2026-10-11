package san_profile_manager

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	cu "github.com/Davincible/chromedp-undetected"

	"github.com/chromedp/cdproto"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// portFile is where Chrome writes the port it listens on for debugging, in
// the profile folder, when started with --remote-debugging-port.
const portFile = "DevToolsActivePort"

// Login opens url, a site's sign-in page, in a plain Chrome on the profile and
// returns when that window is closed. Sign in there, then close it.
//
// Chrome is started without any automation and without a debugging port,
// because Google's sign-in, which most sites offer, turns away a browser that
// is being driven.
func Login(ctx context.Context, c Config, url string) error {
	if c.Remote != "" {
		return errors.New("san_profile_manager: Login starts its own Chrome; sign in to the remote one by hand")
	}
	exe, err := c.exec()
	if err != nil {
		return err
	}
	dir, err := Dir(c.Profile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("san_profile_manager: %w", err)
	}
	if Locked(dir) {
		return ErrProfileInUse
	}
	args := append(flags(dir), url)
	if c.Plain {
		args = append(plainFlags(dir), url)
	} else if err := noBrowserSignin(dir); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	start := time.Now()
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("san_profile_manager: chrome: %w", err)
	}
	// A Chrome already running on this profile takes the URL over and the
	// one just started exits at once, so returning here would be too early.
	if time.Since(start) < 3*time.Second {
		return ErrProfileInUse
	}
	return ctx.Err()
}

// Browser is a Chrome on a profile, and one tab in it that a tool works in.
type Browser struct {
	tab    context.Context
	cancel context.CancelFunc
	url    string    // the debugging URL
	id     target.ID // the tab's
	proc   *process  // the Chrome this Browser started, if it did
	remote bool
	// undetected: chromedp started this Chrome, through
	// chromedp-undetected (startUndetected); there is no url or proc.
	undetected bool
	once       sync.Once

	mu   sync.Mutex
	tabs map[target.ID]func() // NewTab's, still open
}

// Start opens a tab of its own in Chrome on the profile: a Chrome a tool
// already has running there, or a new one, headful unless Headless is set.
// With Remote it opens the tab in that Chrome instead.
//
// The tool works in a tab opened over the debugging port, whoever started
// Chrome. A Chrome that chromedp launches itself carries automation flags
// that sites can see: Medium's Cloudflare refused every such browser, and
// never one started this way.
//
// In the tab, a page's "leave this page?" question is answered "stay", so
// that nothing can throw away unsaved work; other dialogs are accepted.
func Start(ctx context.Context, c Config) (*Browser, error) {
	b := &Browser{url: c.Remote, remote: c.Remote != ""}
	var initial []target.ID
	if !b.remote {
		dir, err := Dir(c.Profile)
		if err != nil {
			return nil, err
		}
		if c.Plain && !c.Headless && !Locked(dir) {
			return startUndetected(ctx, c, dir)
		}
		if b.url, b.proc, initial, err = open(ctx, c, dir); err != nil {
			return nil, err
		}
	}

	alloc, acancel := chromedp.NewRemoteAllocator(ctx, b.url)
	tab, tcancel := chromedp.NewContext(alloc)
	b.tab = tab
	b.cancel = func() { tcancel(); acancel() }
	if err := chromedp.Run(tab); err != nil {
		b.cancel()
		stop(b.proc)
		return nil, fmt.Errorf("san_profile_manager: attach to chrome: %w", err)
	}
	b.id = chromedp.FromContext(tab).Target.TargetID
	answerDialogs(tab)

	// The blank tab a new Chrome starts with; the tool's own tab stays.
	for _, id := range initial {
		_ = closeTab(b.url, id)
	}
	return b, nil
}

// startUndetected starts Chrome as cmd/example does, through
// chromedp-undetected, and works in the tab Chrome starts with. On
// 2026-10-11 Medium's Cloudflare let this start in, and blocked Chrome
// started by launch.
func startUndetected(ctx context.Context, c Config, dir string) (*Browser, error) {
	opts := []cu.Option{
		cu.WithContext(ctx),
		cu.WithUserDataDir(dir),
		cu.WithChromeFlags(
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.Flag("profile-directory", "Default"),
		),
	}
	if c.ExecPath != "" {
		opts = append(opts, cu.WithChromeBinary(c.ExecPath))
	}
	tab, cancel, err := cu.New(cu.NewConfig(opts...))
	if err != nil {
		return nil, fmt.Errorf("san_profile_manager: start chrome: %w", err)
	}
	if err := chromedp.Run(tab); err != nil {
		cancel()
		return nil, fmt.Errorf("san_profile_manager: start chrome: %w", err)
	}
	b := &Browser{tab: tab, cancel: cancel, undetected: true}
	b.id = chromedp.FromContext(tab).Target.TargetID
	answerDialogs(tab)
	return b, nil
}

// Tab is the context of the Browser's tab, for chromedp.Run.
func (b *Browser) Tab() context.Context { return b.tab }

// NewTab opens another tab in the same Chrome, for a tool that works in
// several at once, such as one per conversation. It answers dialogs as the
// Browser's own tab does. close closes the tab and returns once Chrome no
// longer lists it, so that Close counts only tabs still open; it is safe to
// call more than once.
func (b *Browser) NewTab() (ctx context.Context, close func(), err error) {
	ctx, cancel := chromedp.NewContext(b.tab)
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("san_profile_manager: open a tab: %w", err)
	}
	id := chromedp.FromContext(ctx).Target.TargetID
	answerDialogs(ctx)
	var once sync.Once
	close = func() {
		once.Do(func() {
			_ = closeTab(b.url, id)
			cancel()
			b.mu.Lock()
			delete(b.tabs, id)
			b.mu.Unlock()
		})
	}
	b.mu.Lock()
	if b.tabs == nil {
		b.tabs = map[target.ID]func(){}
	}
	b.tabs[id] = close
	b.mu.Unlock()
	return ctx, close, nil
}

// answerDialogs answers a page's "leave this page?" question with "stay", so
// that nothing can throw away unsaved work, and accepts other dialogs.
func answerDialogs(tab context.Context) {
	chromedp.ListenTarget(tab, func(ev any) {
		if ev, ok := ev.(*page.EventJavascriptDialogOpening); ok {
			accept := ev.Type != page.DialogTypeBeforeunload
			go chromedp.Run(tab, page.HandleJavaScriptDialog(accept))
		}
	})
}

// Cookie is one cookie of the profile.
type Cookie struct{ Name, Domain, Value string }

// Cookies reads every cookie the profile holds, for any site. It loads no
// page.
func (b *Browser) Cookies(ctx context.Context) ([]Cookie, error) {
	tctx, cancel := context.WithCancel(b.tab)
	defer cancel()
	defer context.AfterFunc(ctx, cancel)()
	var out []Cookie
	err := chromedp.Run(tctx, chromedp.ActionFunc(func(ctx context.Context) error {
		cs, err := storage.GetCookies().Do(ctx)
		if err != nil {
			return err
		}
		for _, k := range cs {
			out = append(out, Cookie{Name: k.Name, Domain: k.Domain, Value: k.Value})
		}
		return nil
	}))
	if err != nil {
		return nil, fmt.Errorf("san_profile_manager: read cookies: %w", err)
	}
	return out, nil
}

// Close closes the tab, and those NewTab opened. Chrome on a profile is
// closed too when no tab is left in it: another tool's, or one a person
// opened, keeps it running. A Remote Chrome is never closed. Close works
// after the context given to Start is cancelled.
func (b *Browser) Close() error {
	b.once.Do(func() {
		b.mu.Lock()
		var opened []func()
		for _, f := range b.tabs {
			opened = append(opened, f)
		}
		b.mu.Unlock()
		for _, f := range opened {
			f()
		}

		// A Chrome from startUndetected is chromedp's own: closing its first
		// tab's context closes that Chrome.
		if b.undetected {
			_ = chromedp.Cancel(b.tab)
			b.cancel()
			return
		}

		// The tab is closed and gone before Close returns. Cancelling its
		// context alone would close it in the background, too late for a
		// program that exits next, and another tool would count it as still
		// in use.
		_ = closeTab(b.url, b.id)
		b.cancel()
		if b.remote {
			return
		}
		// The last one out closes Chrome. Counting after closing also catches
		// two tools that close at the same moment.
		n, err := otherPages(b.url, "")
		if err == nil && n == 0 {
			_ = closeBrowser(b.url)
		}
		if b.proc != nil && (err != nil || n == 0) {
			select {
			case <-b.proc.exited:
			case <-time.After(5 * time.Second):
				stop(b.proc)
			}
		}
	})
	return nil
}

// open returns the debugging URL of Chrome on the profile: one a tool has
// running there already, or a new one this call starts. initial are the tabs
// a new Chrome starts with.
func open(ctx context.Context, c Config, dir string) (url string, proc *process, initial []target.ID, err error) {
	if Locked(dir) {
		url, err := attachable(ctx, dir, c.Headless)
		if !errors.Is(err, errClosed) {
			return url, nil, nil, err
		}
	}
	return launch(ctx, c, dir)
}

// errClosed means the Chrome that had the profile open has closed meanwhile,
// so a new one can be started.
var errClosed = errors.New("san_profile_manager: the Chrome on the profile closed")

// attachable waits for the Chrome that has the profile open to answer on its
// debugging port. A tool may have only just started it, so a missing port is
// given 5 seconds; after that, the Chrome is Login's window or a person's. A
// headless Chrome, which only reads cookies for a moment, is waited out for
// up to 15 seconds when a window is wanted.
func attachable(ctx context.Context, dir string, headless bool) (string, error) {
	start := time.Now()
	for {
		r, ok := running(dir)
		switch {
		case ok && (headless || !r.headless):
			return r.url, nil
		case !Locked(dir):
			return "", errClosed
		case !ok && time.Since(start) > 5*time.Second:
			return "", ErrProfileInUse
		case time.Since(start) > 15*time.Second:
			return "", fmt.Errorf("%w (a headless Chrome is reading its cookies)", ErrProfileInUse)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

type chrome struct {
	url      string
	headless bool
}

// running is the Chrome answering on the debugging port the profile's
// DevToolsActivePort names, if any.
func running(dir string) (chrome, bool) {
	port, ok := readPort(filepath.Join(dir, portFile))
	if !ok {
		return chrome{}, false
	}
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	var v struct {
		UserAgent string `json:"User-Agent"`
	}
	if err := getJSON(url+"/json/version", &v); err != nil {
		return chrome{}, false
	}
	return chrome{url: url, headless: strings.Contains(v.UserAgent, "HeadlessChrome")}, true
}

// readPort reads the first line of DevToolsActivePort: the port. The second
// line is the browser's websocket path.
func readPort(path string) (int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	if !s.Scan() {
		return 0, false
	}
	port, err := strconv.Atoi(strings.TrimSpace(s.Text()))
	return port, err == nil && port > 0
}

// launch starts Chrome on the profile as a plain process with a debugging
// port, the way a person would start it from a shortcut, and returns the
// port's URL once Chrome answers on it. Chrome picks the port itself and
// writes it into the profile, where other tools find it.
func launch(ctx context.Context, c Config, dir string) (string, *process, []target.ID, error) {
	exe, err := c.exec()
	if err != nil {
		return "", nil, nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, nil, fmt.Errorf("san_profile_manager: %w", err)
	}
	if !c.Plain {
		if err := noBrowserSignin(dir); err != nil {
			return "", nil, nil, err
		}
	}
	_ = os.Remove(filepath.Join(dir, portFile)) // left by a Chrome that did not exit cleanly

	cmd := exec.Command(exe, launchArgs(c, dir)...)
	if err := cmd.Start(); err != nil {
		return "", nil, nil, fmt.Errorf("san_profile_manager: start chrome: %w", err)
	}
	p := &process{cmd: cmd, exited: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(p.exited) }()

	deadline := time.Now().Add(30 * time.Second)
	for {
		// Chrome opens its port a moment before its first tab, the
		// about:blank it was given, which is closed once the tool's own tab
		// is open.
		if r, ok := running(dir); ok {
			var tabs []struct {
				ID   target.ID `json:"id"`
				Type string    `json:"type"`
			}
			_ = getJSON(r.url+"/json/list", &tabs)
			var initial []target.ID
			for _, t := range tabs {
				if t.Type == "page" {
					initial = append(initial, t.ID)
				}
			}
			if len(initial) > 0 || time.Now().After(deadline) {
				return r.url, p, initial, nil
			}
		}
		select {
		case <-p.exited:
			// A Chrome already running on the profile takes the window over,
			// and the one just started exits: another tool may have started
			// it at the same moment.
			if Locked(dir) {
				url, err := attachable(ctx, dir, c.Headless)
				if !errors.Is(err, errClosed) {
					return url, nil, nil, err
				}
			}
			return "", nil, nil, errors.New("san_profile_manager: chrome exited at start")
		case <-ctx.Done():
			stop(p)
			return "", nil, nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			stop(p)
			return "", nil, nil, errors.New("san_profile_manager: chrome did not open its debugging port")
		}
	}
}

// closeTab closes tab id of the Chrome at url, and returns once Chrome no
// longer lists it.
func closeTab(url string, id target.ID) error {
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(url + "/json/close/" + string(id))
	if err != nil {
		return err
	}
	resp.Body.Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var tabs []struct {
			ID target.ID `json:"id"`
		}
		if err := getJSON(url+"/json/list", &tabs); err != nil {
			return err
		}
		gone := true
		for _, t := range tabs {
			gone = gone && t.ID != id
		}
		if gone {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("san_profile_manager: tab %s did not close", id)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// otherPages counts the tabs in the Chrome at url besides tab id.
func otherPages(url string, id target.ID) (int, error) {
	var tabs []struct {
		ID   target.ID `json:"id"`
		Type string    `json:"type"`
	}
	if err := getJSON(url+"/json/list", &tabs); err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tabs {
		if t.Type == "page" && t.ID != id {
			n++
		}
	}
	return n, nil
}

// closeBrowser asks the Chrome at url to quit, over a connection of its own,
// since the tab's may already be gone. chromedp only sends Browser.close for
// a Chrome it launched itself, so the message is written by hand.
func closeBrowser(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var v struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := getJSON(url+"/json/version", &v); err != nil {
		return err
	}
	conn, err := chromedp.DialContext(ctx, v.WS)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Write(ctx, &cdproto.Message{ID: 1, Method: browser.CommandClose}); err != nil {
		return err
	}
	var reply cdproto.Message
	return conn.Read(ctx, &reply) // the answer, or the connection closing
}

func getJSON(url string, v any) error {
	c := http.Client{Timeout: 3 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// process is a Chrome that Start launched; exited is closed when it ends.
type process struct {
	cmd    *exec.Cmd
	exited chan struct{}
}

func stop(p *process) {
	if p != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
}
