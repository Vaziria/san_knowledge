package trends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/page"
)

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		q       Query
		bad     bool
		geo, tm string
		prop    string
	}{
		{q: Query{Keyword: "  aplikasi   kasir "}, tm: DefaultTime},
		{q: Query{Keyword: "kasir", Geo: "id-jk", Time: "now 7-d"}, geo: "ID-JK", tm: "now 7-d"},
		{q: Query{Keyword: "kasir", Geo: "Worldwide", Property: "web"}, tm: DefaultTime},
		{q: Query{Keyword: "kasir", Time: "2025-01-01 2025-12-31", Property: "Froogle"}, tm: "2025-01-01 2025-12-31", prop: "froogle"},
		{q: Query{Keyword: "kasir", Geo: "US-CA-803"}, geo: "US-CA-803", tm: DefaultTime},
		{q: Query{Keyword: ""}, bad: true},
		{q: Query{Keyword: strings.Repeat("k", MaxKeyword+1)}, bad: true},
		{q: Query{Keyword: "kasir", Geo: "Indonesia"}, bad: true},
		{q: Query{Keyword: "kasir", Time: "today 2-m"}, bad: true},
		{q: Query{Keyword: "kasir", Time: "2003-01-01 2004-01-01"}, bad: true},
		{q: Query{Keyword: "kasir", Time: "2025-02-01 2025-01-01"}, bad: true},
		{q: Query{Keyword: "kasir", Time: "2025-02-30 2025-03-01"}, bad: true},
		{q: Query{Keyword: "kasir", Category: -1}, bad: true},
		{q: Query{Keyword: "kasir", Property: "shopping"}, bad: true},
	} {
		q := c.q
		err := q.Check()
		if c.bad {
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("%+v: %v, want ErrInvalid", c.q, err)
			}
			continue
		}
		if err != nil || q.Geo != c.geo || q.Time != c.tm || q.Property != c.prop || strings.Contains(q.Keyword, "  ") {
			t.Errorf("%+v: %v, got geo %q time %q prop %q keyword %q", c.q, err, q.Geo, q.Time, q.Property, q.Keyword)
		}
		// A checked query passes again unchanged.
		again := q
		if err := again.Check(); err != nil || again != q {
			t.Errorf("%+v: checked again: %v %+v", q, err, again)
		}
	}
}

// The new Explore's own address: its defaults left out, no hl (which sends
// the page to the classic Explore).
func TestExploreLink(t *testing.T) {
	for _, c := range []struct {
		q    Query
		want string
	}{
		{Query{Keyword: "aplikasi kasir", Geo: "ID", Time: DefaultTime}, "https://trends.google.com/explore?q=aplikasi%20kasir&geo=ID"},
		{Query{Keyword: "point of sale", Time: "now 7-d", Category: 12, Property: "youtube"}, "https://trends.google.com/explore?q=point%20of%20sale&date=now%207-d&cat=12&gprop=youtube"},
		{Query{Keyword: "kasir & pos", Geo: "ID-JK", Time: "2025-01-01 2025-12-31"}, "https://trends.google.com/explore?q=kasir%20%26%20pos&geo=ID-JK&date=2025-01-01%202025-12-31"},
	} {
		if got := c.q.URL(); got != c.want {
			t.Errorf("%+v: %s", c.q, got)
		}
		if strings.Contains(c.q.URL(), "hl=") {
			t.Errorf("%s has hl", c.q.URL())
		}
	}
	if !sameSearch("https://trends.google.com/explore?q=a&geo=ID&date=today%2012-m", "https://trends.google.com/explore?q=a&geo=ID") ||
		sameSearch("https://trends.google.com/explore", "https://trends.google.com/explore?q=a&geo=ID") ||
		sameSearch("https://trends.google.com/trends/explore?q=a&geo=ID", "https://trends.google.com/explore?q=a&geo=ID") {
		t.Error("sameSearch")
	}
}

// The page's related-queries reply as saved from "aplikasi kasir",
// Indonesia, past year (2026-11-11): the same 48 queries sorted by search
// interest (Top) and by change (Rising, which also lists the ones that
// fell).
func TestParseRelated(t *testing.T) {
	link := func(q string) string { return ExploreLink(q, "ID", DefaultTime, 0, "") }
	top, rising, err := ParseRelated(read(t, "related_aplikasi_kasir.txt"), link)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 48 || len(rising) != 15 {
		t.Fatalf("%d top, %d rising", len(top), len(rising))
	}
	if it := top[0]; it.Query != "aplikasi untuk kasir" || it.SearchInterest != 100 || it.Change != 0 || it.Breakout ||
		it.Link != "https://trends.google.com/explore?q=aplikasi%20untuk%20kasir&geo=ID" || it.ChangeText() != "No change" {
		t.Errorf("top 1 = %+v", it)
	}
	if it := top[1]; it.Query != "aplikasi kasir gratis" || it.SearchInterest != 68 || it.ChangeText() != "-20%" {
		t.Errorf("top 2 = %+v", it)
	}
	if it := rising[0]; !strings.HasPrefix(it.Query, "semangat kebangsaan") || it.Change != 5000 || !it.Breakout || it.SearchInterest != 3 || it.ChangeText() != "Breakout" {
		t.Errorf("rising 1 = %+v", it)
	}
	if it := rising[1]; !strings.HasPrefix(it.Query, "seorang kasir") || it.Change != 550 || it.Breakout || it.ChangeText() != "+550%" {
		t.Errorf("rising 2 = %+v", it)
	}
	for _, it := range rising {
		if it.Change <= 0 {
			t.Errorf("rising holds %+v, which fell", it)
		}
	}
	for i := 1; i < len(top); i++ {
		if top[i].SearchInterest > top[i-1].SearchInterest {
			t.Errorf("top is not by search interest at %d", i)
		}
	}

	// Too little data is no queries, not an error.
	top, rising, err = ParseRelated(read(t, "related_empty.txt"), nil)
	if err != nil || top == nil || rising == nil || len(top)+len(rising) != 0 {
		t.Errorf("empty: %v %v %v", top, rising, err)
	}
	// Trends' error reply, or none, is the page's error.
	for _, body := range []string{
		")]}'\n\n50\n[[\"er\",\"fXqlme\",null,null,null,429,\"generic\"]]\n",
		")]}'\n\n50\n[[\"wrb.fr\",\"g4kJzf\",\"[]\",null,null,null,\"generic\"]]\n",
		"<html>Error 429</html>",
	} {
		if _, _, err := ParseRelated(body, nil); !errors.Is(err, ErrPage) {
			t.Errorf("%q: %v, want ErrPage", body, err)
		}
	}
}

// Each helper of trends.js is named once, and every one the Go steps call is
// there. Line ends are read either way: a Windows checkout has CRLF.
func TestScript(t *testing.T) {
	js := strings.ReplaceAll(trendsJS, "\r\n", "\n")
	_, helpers, ok := strings.Cut(js, "\n  return {\n")
	if !ok {
		t.Fatal("trends.js returns no object of helpers")
	}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^    (?:async )?([A-Za-z]\w*)\(`).FindAllStringSubmatch(helpers, -1) {
		if seen[m[1]] {
			t.Errorf("trends.js defines %s twice", m[1])
		}
		seen[m[1]] = true
	}
	b, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	called := regexp.MustCompile(`(?:Call|CallFor|Do)\(ctx, (?:\d+\s*\*\s*time\.Second, )?(?:&?[\w.]+, |"[^"]*", )?"(\w+)"`).FindAllStringSubmatch(string(b), -1)
	if len(called) < 8 {
		t.Fatalf("service.go calls %d helpers, want at least 8", len(called))
	}
	for _, m := range called {
		if !seen[m[1]] {
			t.Errorf("service.go calls %s, which trends.js does not define", m[1])
		}
	}
	// The hook keeps the related queries' replies only, and readJS puts the
	// page's fetch back.
	if !strings.Contains(hookJS, "rpcids=[^&]*fXqlme") || !strings.Contains(hookJS, "undo()") || !strings.Contains(readJS, "h.undo()") {
		t.Error("hook.js / readJS")
	}
}

// trends.js finds the page's parts by these attributes, and the pages had
// each of them when they were saved (testdata, 2026-11-11): explore.html is
// parts of the new Explore, classic.html the classic Explore's banner with
// its switch to the new one.
func TestPageHasControls(t *testing.T) {
	explore, classic := read(t, "explore.html"), read(t, "classic.html")
	for _, c := range []struct {
		page, name, attr, script string
	}{
		{explore, "explore.html", `>location_on</i>`, `filterButton('location_on')`},
		{explore, "explore.html", `aria-haspopup="menu"`, `button[aria-haspopup]`},
		{explore, "explore.html", `data-country-code="ID"`, `[data-country-code]`},
		{explore, "explore.html", `data-submenu-id="ucc-544"`, `data-submenu-id`},
		{explore, "explore.html", `data-geo-code="Worldwide" data-event-label="select-country"`, `[data-event-label="select-country"]`},
		{explore, "explore.html", `my-location=""`, `my-location`},
		{explore, "explore.html", `data-geo-code="ID-JK" data-event-label="select-region"`, `[data-event-label="select-region"]`},
		{explore, "explore.html", `data-default-submenu-item="true"`, `data-default-submenu-item`},
		{explore, "explore.html", `data-value="10" data-year="2025"`, `menuWith('[data-year]')`},
		{explore, "explore.html", `data-value="froogle"`, `menuWith('[data-value="froogle"]')`},
		{explore, "explore.html", `aria-checked="true" data-value="0"`, `aria-checked`},
		{explore, "explore.html", `jsname="K4r5Ff"`, `[jsname="K4r5Ff"]`},
		{explore, "explore.html", `AF_initDataCallback({key: 'ds:0'`, `key: 'ds:0'`},
		{explore, "explore.html", `, sideChannel`, `', sideChannel'`},
		{explore, "explore.html", `<h2 class="jR7J6b">Commonly searched queries</h2>`, `Commonly searched`},
		{classic, "classic.html", `Classic Explore is expiring soon.`, `/trends/explore`},
		{classic, "classic.html", `href="/explore"`, `a[href], button`},
		{classic, "classic.html", `Go to new Explore`, `/new Explore/i`},
	} {
		if !strings.Contains(c.page, c.attr) {
			t.Errorf("%s has no %s", c.name, c.attr)
		}
		if !strings.Contains(trendsJS, c.script) {
			t.Errorf("trends.js does not look for %s", c.script)
		}
	}
	// state() of the classic page, as it read it there: classic, with its
	// switch, and not the new Explore.
	var st state
	if err := json.Unmarshal([]byte(read(t, "classic_state.json")), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Classic || !st.Switch || st.Explore || st.Filters || !strings.Contains(st.URL, "/trends/explore") || !ready(st) {
		t.Errorf("classic state = %+v", st)
	}
}

// fakeTrends is the browser with an Explore tab (id 7) behind the user's tab
// (id 3), both in window 1.
type fakeTrends struct {
	page.Browser
	mu sync.Mutex
	// classic is how many loads land on the classic Explore (the address
	// asked, under /trends/explore), and switchWorks whether its banner's
	// switch brings the bare new Explore.
	classic     int
	switchWorks bool
	// reply is the related queries' reply, status its HTTP status; blocked
	// sends the tab to Google's robot check once it is shown.
	reply   string
	status  int
	blocked bool
	// The page takes no category from its address.
	noCategory bool

	at, opened                   string
	navs, helpers                []string
	focus, closed                []int32
	visible, hooked, revealed    bool
	shownBeforeHook, hookedTwice bool
}

func (f *fakeTrends) load(u string) {
	if f.classic > 0 {
		f.classic--
		u = strings.Replace(u, "/explore", "/trends/explore", 1)
	}
	f.at, f.hooked, f.revealed = u, false, false
}

func (f *fakeTrends) OpenTab(_ context.Context, r *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = r.GetUrl()
	if !r.GetBackground() || r.GetFocus() {
		return nil, errors.New("fake: the tab must open in the background")
	}
	f.load(r.GetUrl())
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 7, WindowId: 1}}, nil
}

func (f *fakeTrends) Navigate(_ context.Context, r *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.navs = append(f.navs, r.GetUrl())
	f.load(r.GetUrl())
	return &v1.NavigateResponse{}, nil
}

func (f *fakeTrends) FocusTab(_ context.Context, r *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.focus = append(f.focus, r.GetTabId())
	f.visible = r.GetTabId() == 7
	return &v1.FocusTabResponse{}, nil
}

func (f *fakeTrends) CloseTab(_ context.Context, r *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = append(f.closed, r.GetTabId())
	return &v1.CloseTabResponse{}, nil
}

func (f *fakeTrends) ListTabs(context.Context, *v1.ListTabsRequest) (*v1.ListTabsResponse, error) {
	return &v1.ListTabsResponse{Tabs: []*v1.Tab{{Id: 3, WindowId: 1, Active: true}, {Id: 7, WindowId: 1}, {Id: 9, WindowId: 2, Active: true}}}, nil
}

func (f *fakeTrends) Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return nil, errors.New("no screen in a test")
}

var helperCall = regexp.MustCompile(`globalThis\.__sanTrends\.(\w+)\(([^\n]*)\);$`)

func (f *fakeTrends) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out any
	code := r.GetCode()
	switch {
	case r.GetWorld() == v1.World_WORLD_MAIN && code == hookJS:
		if f.revealed {
			f.shownBeforeHook = true
		}
		if f.hooked {
			f.hookedTwice = true
		}
		f.hooked, out = true, true
	case r.GetWorld() == v1.World_WORLD_MAIN && code == readJS:
		switch {
		case !f.hooked || (f.blocked && f.visible):
			out = map[string]any{"hooked": false}
		case f.revealed && f.visible:
			out = map[string]any{"hooked": true, "reply": map[string]any{"status": f.status, "text": f.reply}}
			f.hooked = false
		default:
			out = map[string]any{"hooked": true, "reply": nil}
		}
	case strings.Contains(code, "document.visibilityState === 'visible'"):
		out = f.visible
	default:
		m := helperCall.FindStringSubmatch(code)
		if m == nil {
			return nil, fmt.Errorf("fake: not a helper call: %.100s", code)
		}
		f.helpers = append(f.helpers, m[1])
		u, _ := url.Parse(f.at)
		switch m[1] {
		case "state":
			classic := strings.HasPrefix(u.Path, "/trends/explore")
			st := state{URL: f.at, Classic: classic, Switch: classic, Explore: u.Path == "/explore", Filters: u.Path == "/explore", Section: u.Path == "/explore"}
			if f.blocked && f.visible {
				st = state{URL: "https://www.google.com/sorry/index", Blocked: true}
			}
			out = st
		case "switchToNew":
			if f.switchWorks {
				f.at = ExploreURL
			}
			out = true
		case "filters":
			q := u.Query()
			geo, tm, cat, prop := q.Get("geo"), "7", q.Get("cat"), q.Get("gprop")
			if geo == "" {
				geo = "Worldwide"
			}
			for i, o := range Times {
				if d := q.Get("date"); d == o.Value {
					tm = strconv.Itoa(i + 1)
				} else if dateRange.MatchString(d) {
					tm = "10"
				}
			}
			if cat == "" || f.noCategory {
				cat = "0"
			}
			if prop == "" {
				prop = "web"
			}
			out = map[string]any{"geo": geo, "time": tm, "category": cat, "property": prop}
		case "reveal":
			f.revealed, out = true, true
		default:
			return nil, fmt.Errorf("fake: no helper %s", m[1])
		}
	}
	b, err := json.Marshal(out)
	return &v1.ExecResponse{ResultJson: string(b)}, err
}

func fast(t *testing.T) {
	old := []time.Duration{LoadWait, ReplyWait, SwitchWait, page.FrontRetry}
	LoadWait, ReplyWait, SwitchWait, page.FrontRetry = 2*time.Second, 2*time.Second, time.Second, 200*time.Millisecond
	t.Cleanup(func() { LoadWait, ReplyWait, SwitchWait, page.FrontRetry = old[0], old[1], old[2], old[3] })
}

func TestRelated(t *testing.T) {
	fast(t)
	body := read(t, "related_aplikasi_kasir.txt")
	q := Query{Keyword: "aplikasi kasir", Geo: "ID"}

	f := &fakeTrends{reply: body, status: 200}
	res, err := (&Service{B: f}).Related(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Top) != 48 || len(res.Rising) != 15 || res.Switched || res.URL != "https://trends.google.com/explore?q=aplikasi%20kasir&geo=ID" {
		t.Errorf("result: %d top, %d rising, switched %v, %s", len(res.Top), len(res.Rising), res.Switched, res.URL)
	}
	// Opened behind, hooked before it was shown, shown for the reply, then
	// closed, and the user's tab shown again.
	if f.opened != res.URL || f.shownBeforeHook || len(f.navs) != 0 {
		t.Errorf("opened %s, navigated %v, shown before the hook %v", f.opened, f.navs, f.shownBeforeHook)
	}
	if fmt.Sprint(f.focus) != "[7 3]" || fmt.Sprint(f.closed) != "[7]" {
		t.Errorf("focused %v, closed %v", f.focus, f.closed)
	}
	if !strings.Contains(strings.Join(f.helpers, " "), "filters") {
		t.Errorf("helpers %v: the filters were not checked", f.helpers)
	}

	// The classic Explore is switched to the new one: its own switch (the
	// bare Explore), then the asked address; never read.
	f = &fakeTrends{reply: body, status: 200, classic: 1, switchWorks: true}
	var steps []string
	res, err = (&Service{B: f, Progress: func(s string) { steps = append(steps, s) }}).Related(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Switched || fmt.Sprint(f.navs) != "["+res.URL+"]" || !strings.Contains(strings.Join(f.helpers, " "), "switchToNew") {
		t.Errorf("classic: switched %v, navigated %v, helpers %v", res.Switched, f.navs, f.helpers)
	}
	if !strings.Contains(strings.Join(steps, "\n"), "Trends opened the classic Explore: switched to the new one") {
		t.Errorf("steps %q", steps)
	}
	// Its switch doing nothing: the address alone.
	f = &fakeTrends{reply: body, status: 200, classic: 1}
	if res, err = (&Service{B: f}).Related(context.Background(), q); err != nil || !res.Switched || len(f.navs) != 1 {
		t.Errorf("classic, a dead switch: %v, navigated %v", err, f.navs)
	}
	// A page that stays classic is refused, unread.
	f = &fakeTrends{reply: body, status: 200, classic: 5, switchWorks: false}
	if _, err := (&Service{B: f}).Related(context.Background(), q); !errors.Is(err, ErrClassic) || f.hooked || fmt.Sprint(f.closed) != "[7]" {
		t.Errorf("stays classic: %v, hooked %v, closed %v", err, f.hooked, f.closed)
	}

	// Too little data is an empty result.
	f = &fakeTrends{reply: read(t, "related_empty.txt"), status: 200}
	if res, err = (&Service{B: f}).Related(context.Background(), q); err != nil || len(res.Top)+len(res.Rising) != 0 {
		t.Errorf("empty: %v %+v", err, res)
	}
	// Google limiting the browser: a 429 reply, or its robot check.
	f = &fakeTrends{reply: "", status: 429}
	if _, err := (&Service{B: f}).Related(context.Background(), q); !errors.Is(err, ErrRateLimited) {
		t.Errorf("429: %v, want ErrRateLimited", err)
	}
	f = &fakeTrends{reply: body, status: 200, blocked: true}
	if _, err := (&Service{B: f}).Related(context.Background(), q); !errors.Is(err, ErrRateLimited) {
		t.Errorf("robot check: %v, want ErrRateLimited", err)
	}
	// A page that did not take a filter from its address is refused.
	f = &fakeTrends{reply: body, status: 200, noCategory: true}
	if _, err := (&Service{B: f}).Related(context.Background(), Query{Keyword: "kasir", Category: 12}); !errors.Is(err, ErrPage) || f.hooked || !strings.Contains(err.Error(), `category "12"`) {
		t.Errorf("no category: %v", err)
	}
	// A bad lookup opens no tab.
	f = &fakeTrends{}
	if _, err := (&Service{B: f}).Related(context.Background(), Query{Keyword: "kasir", Time: "yesterday"}); !errors.Is(err, ErrInvalid) || f.opened != "" {
		t.Errorf("bad time: %v, opened %q", err, f.opened)
	}
}

// TestLiveClassic opens the classic Explore in the browser attached to a
// running `san_remote serve` and sees it switched to the new one (a real
// Trends page: SAN_REMOTE_E2E=1 and SAN_REMOTE_E2E_TRENDS=1). The lookups
// themselves are e2e/trends_test.go's.
func TestLiveClassic(t *testing.T) {
	if os.Getenv("SAN_REMOTE_E2E") != "1" || os.Getenv("SAN_REMOTE_E2E_TRENDS") != "1" {
		t.Skip("opens Google Trends in the real browser: set SAN_REMOTE_E2E=1 and SAN_REMOTE_E2E_TRENDS=1")
	}
	server := os.Getenv("SAN_REMOTE_SERVER")
	if server == "" {
		server = hub.DefaultServer
	}
	s := &Service{B: page.Remote{C: hub.NewClient(server)}, Progress: func(step string) { t.Log("== " + step) }}
	ctx := context.Background()
	// The classic Explore (an hl=en-US link stays there), switched.
	q := Query{Keyword: "aplikasi kasir", Geo: "ID"}
	if err := q.Check(); err != nil {
		t.Fatal(err)
	}
	tab, switched, err := s.openAt(ctx, os.Getenv("SAN_REMOTE_BROWSER"), "https://trends.google.com/trends/explore?q=aplikasi%20kasir&geo=ID&hl=en-US", q.URL())
	if err != nil {
		t.Fatal(err)
	}
	st, _ := tab.state(ctx)
	tab.close()
	if !switched || !st.Explore || st.Classic {
		t.Errorf("classic: switched %v, state %+v", switched, st)
	}
}
