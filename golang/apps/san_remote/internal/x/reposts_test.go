package x

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

// fixtureRows is testdata/reposts.json: what reposts.js returns, modelled
// on @BangsaPersatuan's Reposts tab on 2026-11-08, with the rows to leave
// out (a pinned post, the account's own, an ad, others' reposts) and one
// without X's app data.
func fixtureRows(t *testing.T) []repostRow {
	t.Helper()
	b, err := os.ReadFile("testdata/reposts.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []repostRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestReposted(t *testing.T) {
	got := reposted(fixtureRows(t), "bangsapersatuan", true)
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	want := []Repost{
		{
			Link: "https://x.com/dayamatest0607/status/2118787984070005168", ID: "2118787984070005168",
			Author: "lokal a joss", Handle: "dayamatest0607", Text: "Pergaulan anak jaman now",
			Created: at("2026-11-06T19:29:55Z"), Reposted: at("2026-11-08T06:19:04Z"), HasMedia: true, Media: []string{"video"},
		},
		{
			Link: "https://x.com/Il6sekarwangi/status/2119220613994995911", ID: "2119220613994995911",
			Author: "Sekar Wangi", Handle: "Il6sekarwangi", Text: "Teks asli, bukan terjemahan https://example.com/a",
			Created: at("2026-11-08T00:09:02Z"), Reposted: at("2026-11-08T06:18:32Z"), Media: []string{},
		},
		// Without X's app data: what the page draws, and no repost time.
		{
			Link: "https://x.com/nadamelinda26/status/2118925351212798285", ID: "2118925351212798285",
			Author: "nada melinda", Handle: "nadamelinda26", Text: "while walking around hehehe",
			Created: at("2026-11-07T04:35:46Z"), HasMedia: true, Media: []string{},
		},
	}
	if len(got) != len(want) {
		t.Fatalf("%d reposts, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if !g.Created.Equal(w.Created) || !g.Reposted.Equal(w.Reposted) {
			t.Errorf("%d: times %v %v, want %v %v", i, g.Created, g.Reposted, w.Created, w.Reposted)
		}
		g.Created, g.Reposted, w.Created, w.Reposted = time.Time{}, time.Time{}, time.Time{}, time.Time{}
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%d:\n%+v\nwant\n%+v", i, g, w)
		}
	}
	// "You reposted" is the signed-in account's only.
	if l := reposted(fixtureRows(t), "someone_else", false); len(l) != 2 || l[0].ID != "1111" || l[1].ID != "2222" {
		t.Errorf("someone_else's: %+v", l)
	}
}

func TestIsRepost(t *testing.T) {
	for _, tt := range []struct {
		r    repostRow
		want bool
	}{
		{repostRow{Reposter: "BangsaPersatuan"}, true},
		{repostRow{Context: "You reposted"}, true},
		{repostRow{Context: "You reposted", By: "/x"}, true},
		{repostRow{Context: "Persatuan Bangsa reposted", By: "/bangsapersatuan/"}, true},
		{repostRow{Context: "Persatuan Bangsa reposted", By: "/other"}, false},
		{repostRow{Context: "Pinned", By: "/BangsaPersatuan"}, false},
		{repostRow{Reposter: "other"}, false},
		{repostRow{}, false},
	} {
		if got := isRepost(tt.r, "BangsaPersatuan", true); got != tt.want {
			t.Errorf("%+v: %v", tt.r, got)
		}
	}
	if isRepost(repostRow{Context: "You reposted"}, "other", false) {
		t.Error("You reposted, on another account's tab")
	}
}

func TestXTime(t *testing.T) {
	for s, want := range map[string]string{
		"2026-11-08T06:19:04.000Z":       "2026-11-08T06:19:04Z",
		"Sun Nov 08 06:18:32 +0000 2026": "2026-11-08T06:18:32Z",
		"":                               "0001-01-01T00:00:00Z",
		"7h":                             "0001-01-01T00:00:00Z",
	} {
		if got := xTime(s).UTC().Format(time.RFC3339); got != want {
			t.Errorf("%q: %s, want %s", s, got, want)
		}
	}
}

// Unrepost takes a post's link in any of X's forms.
func TestUnrepostLinks(t *testing.T) {
	for _, link := range []string{
		"https://x.com/dayamatest0607/status/2118787984070005168",
		"https://twitter.com/dayamatest0607/status/2118787984070005168?s=20",
		"https://mobile.twitter.com/dayamatest0607/status/2118787984070005168/photo/1",
		"https://mobile.x.com/dayamatest0607/status/2118787984070005168/video/1?s=46&t=abc",
		"www.x.com/dayamatest0607/status/2118787984070005168/",
		"2118787984070005168",
	} {
		if id, err := PostID(link); err != nil || id != "2118787984070005168" {
			t.Errorf("%q: %q, %v", link, id, err)
		}
	}
}

// reposts.js and x.js rely on these names of X's app and page.
func TestRepostsJS(t *testing.T) {
	for _, s := range []string{"__reactFiber$", "retweeted_status", "promotedContent", "screen_name", "created_at", "extended_entities", "note_tweet", "socialContext", "User-Name"} {
		if !strings.Contains(repostsJS, s) {
			t.Errorf("reposts.js lacks %q", s)
		}
	}
	for _, s := range []string{"'unretweet'", "'retweet'", "'unretweetConfirm'", "/reposts`", "error-detail"} {
		if !strings.Contains(xJS, s) {
			t.Errorf("x.js lacks %q", s)
		}
	}
	// Every field reposts.js returns is read.
	b, _ := json.Marshal(repostRow{})
	var fields map[string]any
	_ = json.Unmarshal(b, &fields)
	for f := range fields {
		if !regexp.MustCompile(`\n\s+` + f + `:`).MatchString(repostsJS) {
			t.Errorf("reposts.js returns no %s", f)
		}
	}
}

// xTab is a fake X tab: each helper of x.js answers from a Go function, the
// main-world reposts.js from rows, and every helper call is kept, to see
// what was pressed.
type xTab struct {
	page.Browser
	mu      sync.Mutex
	opened  []string
	visited []string
	answers map[string]func(args []json.RawMessage) any
	calls   []string
	// pages are what reposts.js finds, one more each time the timeline is
	// scrolled; the last stays.
	pages  [][]repostRow
	scroll int
}

var xHelper = regexp.MustCompile(`globalThis\.__sanX\.(\w+)\(([^\n]*)\);$`)

func newXTab() *xTab {
	f := &xTab{}
	f.answers = map[string]func([]json.RawMessage) any{
		"state":   func([]json.RawMessage) any { return state{URL: homeURL, Lang: "en", SignedIn: true} },
		"account": func([]json.RawMessage) any { return account{Name: "Persatuan Bangsa", Handle: "BangsaPersatuan"} },
		"more": func([]json.RawMessage) any {
			f.scroll++
			return 0
		},
		"repostsTab": func([]json.RawMessage) any { return tabState{State: "posts"} },
	}
	return f
}

func (f *xTab) OpenTab(_ context.Context, r *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, fmt.Sprintf("%s shown=%v", r.GetUrl(), r.GetFocus()))
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 1}}, nil
}

func (f *xTab) CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	return &v1.CloseTabResponse{}, nil
}

func (f *xTab) Navigate(_ context.Context, r *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.visited = append(f.visited, r.GetUrl())
	return &v1.NavigateResponse{}, nil
}

func (f *xTab) FocusTab(context.Context, *v1.FocusTabRequest) (*v1.FocusTabResponse, error) {
	return &v1.FocusTabResponse{}, nil
}

func (f *xTab) Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return nil, errors.New("no screen in a test")
}

func (f *xTab) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out any
	switch m := xHelper.FindStringSubmatch(r.GetCode()); {
	case r.GetCode() == "return document.visibilityState === 'visible';":
		out = true
	case r.GetWorld() == v1.World_WORLD_MAIN && r.GetCode() == repostsJS:
		if len(f.pages) > 0 {
			out = f.pages[min(f.scroll, len(f.pages)-1)]
		}
	case m != nil:
		var args []json.RawMessage
		if err := json.Unmarshal([]byte("["+m[2]+"]"), &args); err != nil {
			return nil, err
		}
		f.calls = append(f.calls, m[1]+"("+m[2]+")")
		if a := f.answers[m[1]]; a != nil {
			out = a(args)
		}
	default:
		return nil, fmt.Errorf("fake: not a helper call: %.100s", r.GetCode())
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &v1.ExecResponse{ResultJson: string(b)}, nil
}

// pressed lists the helpers called that press something on X.
func (f *xTab) pressed() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "openUnrepost(") || strings.HasPrefix(c, "confirmUnrepost(") {
			out = append(out, c)
		}
	}
	return out
}

// Reposts reads the signed-in account's Reposts tab, scrolling until it has
// the limit or nothing new comes, and keeps each repost once, in the tab's
// order. The cases run side by side: each looks at the page for seconds,
// as a person does.
func TestRepostsList(t *testing.T) {
	rows := fixtureRows(t)
	ctx := context.Background()
	t.Run("scrolled", func(t *testing.T) {
		t.Parallel()
		f := newXTab()
		// The first screen, then one more below (the first again, as X
		// keeps it, and the DOM-only one), then nothing new.
		f.pages = [][]repostRow{rows[:4], {rows[2], rows[5]}, {rows[5]}}
		who, list, err := (&Service{B: f}).Reposts(ctx, "", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, p := range list {
			ids = append(ids, p.ID)
		}
		if who != "BangsaPersatuan" || strings.Join(ids, " ") != "2118787984070005168 2119220613994995911 2118925351212798285" {
			t.Errorf("%s: %v", who, ids)
		}
		if !reflect.DeepEqual(f.visited, []string{"https://x.com/BangsaPersatuan/reposts"}) || !strings.HasSuffix(f.opened[0], "shown=false") {
			t.Errorf("visited %v, opened %v", f.visited, f.opened)
		}
		if len(f.pressed()) > 0 {
			t.Errorf("pressed %v", f.pressed())
		}
	})
	t.Run("limit", func(t *testing.T) {
		t.Parallel()
		f := newXTab()
		f.pages = [][]repostRow{rows}
		if _, list, err := (&Service{B: f}).Reposts(ctx, "", "", 1); err != nil || len(list) != 1 || f.scroll != 0 {
			t.Errorf("limit 1: %v %v, scrolled %d", list, err, f.scroll)
		}
	})
	t.Run("another handle", func(t *testing.T) {
		t.Parallel()
		f := newXTab()
		f.pages = [][]repostRow{rows}
		who, list, err := (&Service{B: f}).Reposts(ctx, "", "@Some_One", 1)
		if err != nil || who != "Some_One" || len(list) != 0 || f.visited[0] != "https://x.com/Some_One/reposts" {
			t.Errorf("another handle: %s %v %v %v", who, list, err, f.visited)
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		// "You haven't reposted yet."
		f := newXTab()
		f.answers["repostsTab"] = func([]json.RawMessage) any { return tabState{State: "empty"} }
		if who, list, err := (&Service{B: f}).Reposts(ctx, "", "", 0); err != nil || who != "BangsaPersatuan" || len(list) != 0 {
			t.Errorf("none: %s %v %v", who, list, err)
		}
	})
	t.Run("bad handle", func(t *testing.T) {
		t.Parallel()
		// Refused before a tab opens.
		f := newXTab()
		if _, _, err := (&Service{B: f}).Reposts(ctx, "", "not a handle", 0); !errors.Is(err, ErrInvalidPost) || len(f.opened) > 0 {
			t.Errorf("bad handle: %v, opened %v", err, f.opened)
		}
	})
}

// postPage is a fake post page whose Repost button is green while reposted,
// and turns back once Undo repost is pressed.
func postPage(reposted, gone bool) *xTab {
	f := newXTab()
	menu := false
	f.answers["postState"] = func([]json.RawMessage) any {
		if gone {
			return postState{Gone: "Hmm...this page doesn’t exist. Try searching for something else.\nSearch"}
		}
		return postState{Found: true, Link: "/dayamatest0607/status/2118787984070005168", Reposted: reposted, Repost: !reposted}
	}
	f.answers["openUnrepost"] = func([]json.RawMessage) any {
		menu = reposted
		return reposted
	}
	f.answers["undoOffered"] = func([]json.RawMessage) any { return menu }
	f.answers["confirmUnrepost"] = func([]json.RawMessage) any {
		if !menu {
			return false
		}
		menu, reposted = false, false
		return true
	}
	return f
}

func TestUnrepost(t *testing.T) {
	ctx := context.Background()
	const link = "https://twitter.com/dayamatest0607/status/2118787984070005168/photo/1?s=20"
	const want = "https://x.com/dayamatest0607/status/2118787984070005168"

	f := postPage(true, false)
	u, err := (&Service{B: f}).Unrepost(ctx, "", link)
	if err != nil || !u.Undone || u.Link != want {
		t.Fatalf("reposted: %+v, %v", u, err)
	}
	if got := f.pressed(); !slices.Equal(got, []string{`openUnrepost("2118787984070005168")`, "confirmUnrepost()"}) {
		t.Errorf("pressed %v", got)
	}
	if f.opened[0] != "https://x.com/i/status/2118787984070005168 shown=true" {
		t.Errorf("opened %v", f.opened)
	}

	// Not reposted: said so, and nothing pressed.
	f = postPage(false, false)
	if u, err = (&Service{B: f}).Unrepost(ctx, "", link); err != nil || u.Undone || u.Link != want || len(f.pressed()) > 0 {
		t.Errorf("not reposted: %+v, %v, pressed %v", u, err, f.pressed())
	}

	// A post that is not there.
	f = postPage(true, true)
	if _, err = (&Service{B: f}).Unrepost(ctx, "", link); !errors.Is(err, ErrNotFound) || !strings.HasSuffix(err.Error(), "something else.\"") || len(f.pressed()) > 0 {
		t.Errorf("gone: %v, pressed %v", err, f.pressed())
	}

	// Not a post's link: refused before a tab opens.
	f = postPage(true, false)
	if _, err = (&Service{B: f}).Unrepost(ctx, "", "https://x.com/dayamatest0607"); !errors.Is(err, ErrInvalidPost) || len(f.opened) > 0 {
		t.Errorf("bad link: %v, opened %v", err, f.opened)
	}
}
