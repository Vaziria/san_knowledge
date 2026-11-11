package trends

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	neturl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed trends.js
var trendsJS string

// hookJS keeps the replies of the page's related-queries call; it runs in
// the page's main world, since the isolated world cannot see the page's
// fetch.
//
//go:embed hook.js
var hookJS string

// readJS takes the hook's first reply and puts the page's fetch back.
const readJS = `const h = window.__sanTrendsHook;
if (!h) return { hooked: false };
const c = h.cap.find((x) => x.text || x.status || x.error);
if (c) h.undo();
return { hooked: true, reply: c || null };`

var site = page.Site{Name: "trends", Global: "__sanTrends", JS: trendsJS, ErrPage: ErrPage}

var (
	// LoadWait is how long the Explore page may take to come up.
	LoadWait = 45 * time.Second
	// ReplyWait is how long the related queries may take once the page
	// shows them.
	ReplyWait = 30 * time.Second
	// DefaultWait bounds a lookup that gives no Wait.
	DefaultWait = 2 * time.Minute
	// SwitchWait is how long the classic Explore's switch may take to bring
	// the new one.
	SwitchWait = 15 * time.Second
)

// sameSearch says whether two Explore addresses search the same: the same
// term and filters, a missing date being Past year.
func sameSearch(a, b string) bool {
	ua, err1 := neturl.Parse(a)
	ub, err2 := neturl.Parse(b)
	if err1 != nil || err2 != nil || ua.Path != ub.Path {
		return false
	}
	qa, qb := ua.Query(), ub.Query()
	for _, k := range []string{"q", "geo", "date", "cat", "gprop"} {
		va, vb := qa.Get(k), qb.Get(k)
		if k == "date" {
			va, vb = strings.TrimSuffix(va, DefaultTime), strings.TrimSuffix(vb, DefaultTime)
		}
		if va != vb {
			return false
		}
	}
	return true
}

// Service reads Trends in the browser. Its calls run one at a time, since
// each one works in a tab of the user's browser. The CLI, the MCP server
// and the API each run one, in their own process, through page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "reading the related
	// queries of aplikasi kasir".
	Progress func(step string)

	once sync.Once
	turn chan struct{}
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

// tabLister is a browser that lists its tabs (page.Remote): a call that
// shows its tab gives the user's tab back afterwards.
type tabLister interface {
	ListTabs(context.Context, *v1.ListTabsRequest) (*v1.ListTabsResponse, error)
}

// session is the one tab a call works in. user is the tab the user was on
// in its window when it opened; once the session's tab has been in front
// (shown, or a screenshot of a failure, which brings it there), closing it
// shows the user's tab again.
type session struct {
	*page.Session
	b       page.Browser
	user    int32
	inFront bool
}

// close closes the tab, and shows the user's tab again when this one was
// in front.
func (t *session) close() {
	t.Close()
	if t.inFront && t.user != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		t.b.FocusTab(ctx, &v1.FocusTabRequest{Browser: t.Browser, TabId: t.user})
	}
}

// fail is page.Session.Fail, whose screenshot brings the tab to the front.
func (t *session) fail(err error) error {
	if errors.Is(err, ErrPage) {
		t.inFront = true
	}
	return t.Fail(err)
}

// show brings the tab to the front, which the page needs before it builds
// some of its parts: its related queries, its location menu.
func (t *session) show(ctx context.Context) error {
	t.inFront, t.Shown = true, true
	return t.Front(ctx)
}

// state is what trends.js state() reports.
type state struct {
	URL      string `json:"url"`
	Classic  bool   `json:"classic"`
	Explore  bool   `json:"explore"`
	Switch   bool   `json:"switch"`
	Blocked  bool   `json:"blocked"`
	TooMany  bool   `json:"tooMany"`
	Consent  bool   `json:"consent"`
	Filters  bool   `json:"filters"`
	Section  bool   `json:"section"`
	SignedIn bool   `json:"signedIn"`
	Rows     int    `json:"rows"`
}

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again. Google's robot check, its 429 page and its cookie consent end the
// wait with their errors.
func (t *session) waitState(ctx context.Context, what string, timeout time.Duration, ok func(state) bool) (state, error) {
	var st state
	err := t.Until(ctx, what, timeout, func() (bool, error) {
		var err error
		st, err = t.state(ctx)
		if err != nil && ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		switch {
		case st.Blocked || st.TooMany:
			return false, ErrRateLimited
		case st.Consent:
			return false, ErrConsent
		}
		return ok(st), nil
	})
	return st, err
}

// ready is the new Explore with its filters up, or the classic one.
func ready(st state) bool { return st.Classic || (st.Explore && st.Filters) }

// open opens url, a new Explore page, in a background tab and waits for it.
// When Trends opens the classic Explore instead, it switches to the new one:
// the page's own "Go to new Explore" first, then url again; switched says
// so. The classic page is never read.
func (s *Service) open(ctx context.Context, browser, url string) (t *session, switched bool, err error) {
	return s.openAt(ctx, browser, url, url)
}

// openAt is open starting at start, which a test sets to a classic page.
func (s *Service) openAt(ctx context.Context, browser, start, url string) (t *session, switched bool, err error) {
	p, err := page.Open(ctx, s.B, site, browser, start, false, s.Progress)
	if err != nil {
		return nil, false, err
	}
	t = &session{Session: p, b: s.B}
	// The user's tab, to show again: the active one of this tab's window,
	// which this background tab is not.
	if l, ok := s.B.(tabLister); ok {
		if r, err := l.ListTabs(ctx, &v1.ListTabsRequest{Browser: browser}); err == nil {
			var win int32
			for _, tab := range r.GetTabs() {
				if tab.GetId() == t.Tab {
					win = tab.GetWindowId()
				}
			}
			for _, tab := range r.GetTabs() {
				if tab.GetActive() && tab.GetWindowId() == win && tab.GetId() != t.Tab {
					t.user = tab.GetId()
				}
			}
		}
	}
	st, err := t.waitState(ctx, "Trends' Explore page", LoadWait, ready)
	if err == nil && st.Classic {
		t.Log("Trends opened the classic Explore: switched to the new one")
		switched = true
		if st.Switch {
			if err = t.Do(ctx, "the classic Explore's switch to the new Explore", "switchToNew"); err == nil {
				st, err = t.waitState(ctx, "the new Explore", SwitchWait, func(st state) bool { return st.Explore && st.Filters })
				if errors.Is(err, ErrPage) {
					err, st.Classic = nil, true // the switch did not take: go by the address
				}
			}
		}
		// The switch may drop the page's filters, or not be there: the new
		// Explore's own address has them all.
		if err == nil && (st.Classic || !sameSearch(st.URL, url)) {
			if err = t.Navigate(ctx, url); err == nil {
				st, err = t.waitState(ctx, "the new Explore", LoadWait, ready)
			}
		}
		if err == nil && st.Classic {
			err = ErrClassic
		}
	}
	if err != nil {
		err = t.fail(err)
		t.close()
		return nil, false, err
	}
	return t, switched, nil
}

// filters is what trends.js filters() reports: the menu items in use.
type filters struct {
	Geo      *string `json:"geo"`
	Time     *string `json:"time"`
	Year     string  `json:"year"`
	Category *string `json:"category"`
	Property *string `json:"property"`
}

// checkFilters makes sure the page took the query's filters from its
// address: the item in use of each of its menus. A menu the page has not
// built is not checked.
func (t *session) checkFilters(ctx context.Context, q Query) error {
	var f filters
	if err := t.Call(ctx, &f, "filters"); err != nil {
		return err
	}
	geo := q.Geo
	if geo == "" {
		geo = "Worldwide"
	}
	tm := "10"
	for i, o := range Times {
		if o.Value == q.Time {
			tm = strconv.Itoa(i + 1)
		}
	}
	prop := q.Property
	if prop == "" {
		prop = "web"
	}
	var wrong []string
	check := func(what string, got *string, want string) {
		if got != nil && *got != want {
			wrong = append(wrong, fmt.Sprintf("%s %q (the page has %q)", what, want, *got))
		}
	}
	check("location", f.Geo, geo)
	check("time range", f.Time, tm)
	check("category", f.Category, strconv.Itoa(q.Category))
	check("search type", f.Property, prop)
	if len(wrong) > 0 {
		return fmt.Errorf("%w: Trends did not take the %s", ErrPage, strings.Join(wrong, ", "))
	}
	return nil
}

// reply is what readJS reports.
type reply struct {
	Hooked bool `json:"hooked"`
	Reply  *struct {
		Status int    `json:"status"`
		Text   string `json:"text"`
		Error  string `json:"error"`
	} `json:"reply"`
}

// Related reads q's related queries: it opens the Explore page in a
// background tab, checks the page took q's filters, hooks the page's fetch,
// then shows the tab for the moment the page needs to fetch its related
// queries (it fetches them only when they are on screen), and reads its
// reply. The user's tab is shown again afterwards.
func (s *Service) Related(ctx context.Context, q Query) (*Result, error) {
	if err := q.Check(); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	wait := q.Wait
	if wait == 0 {
		wait = DefaultWait
	}
	ctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	t, switched, err := s.open(ctx, q.Browser, q.URL())
	if err != nil {
		return nil, err
	}
	defer t.close()
	t.Log(fmt.Sprintf("reading the related queries of %q (%s)", q.Keyword, q.URL()))
	if err := t.checkFilters(ctx, q); err != nil {
		return nil, t.fail(err)
	}
	if err := t.show(ctx); err != nil {
		return nil, t.fail(err)
	}
	body, err := t.related(ctx, q)
	if err != nil {
		return nil, t.fail(err)
	}
	link := func(query string) string { return ExploreLink(query, q.Geo, q.Time, q.Category, q.Property) }
	top, rising, err := ParseRelated(body, link)
	if err != nil {
		return nil, t.fail(err)
	}
	t.Log(fmt.Sprintf("%d top and %d rising queries", len(top), len(rising)))
	return &Result{Keyword: q.Keyword, Top: top, Rising: rising, URL: q.URL(), Switched: switched}, nil
}

// related hooks the page's fetch, scrolls the related queries into view and
// waits for the page's reply. A page that already has them (it was shown
// before the hook) is loaded again once.
func (t *session) related(ctx context.Context, q Query) (string, error) {
	for try := 0; ; try++ {
		if err := t.CallMain(ctx, 10*time.Second, nil, hookJS); err != nil {
			return "", err
		}
		if err := t.Do(ctx, "the related queries' heading", "reveal"); err != nil {
			return "", err
		}
		var r reply
		err := t.Until(ctx, "Trends' related queries", ReplyWait, func() (bool, error) {
			if err := t.CallMain(ctx, 10*time.Second, &r, readJS); err != nil {
				if ctx.Err() == nil && connect.CodeOf(err) == connect.CodeAborted {
					return false, nil
				}
				return false, err
			}
			if !r.Hooked {
				// The page went elsewhere: Google's checks say why.
				st, err := t.state(ctx)
				switch {
				case err != nil:
					return false, nil
				case st.Blocked || st.TooMany:
					return false, ErrRateLimited
				}
				return false, fmt.Errorf("%w: the page loaded again (%s)", ErrPage, st.URL)
			}
			return r.Reply != nil, nil
		})
		if errors.Is(err, ErrPage) && try == 0 {
			if st, serr := t.state(ctx); serr == nil && st.Rows > 0 {
				t.Log("the page had its related queries before the hook: loading it again")
				if err := t.Navigate(ctx, q.URL()); err != nil {
					return "", err
				}
				if _, err := t.waitState(ctx, "Trends' Explore page", LoadWait, ready); err != nil {
					return "", err
				}
				continue
			}
		}
		if err != nil {
			return "", err
		}
		switch {
		case r.Reply.Status == 429:
			return "", ErrRateLimited
		case r.Reply.Status != 200:
			return "", fmt.Errorf("%w: Trends answered its related queries with HTTP %d %s", ErrPage, r.Reply.Status, r.Reply.Error)
		}
		return r.Reply.Text, nil
	}
}

// Pickers reads the page's filter lists from the Explore page without a
// search term (it shows Worldwide's top queries). Its location menu and a
// country's regions are built only when they open, and only while the page
// is shown, so the tab is shown for them.
func (s *Service) Pickers(ctx context.Context, browser, country string) (*Pickers, error) {
	country = strings.ToUpper(strings.TrimSpace(country))
	if country != "" && !geoCode.MatchString(country) {
		return nil, fmt.Errorf("%w: country %q: give a country's code, such as ID", ErrInvalid, country)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	ctx, cancel := context.WithTimeout(ctx, DefaultWait)
	defer cancel()
	t, _, err := s.open(ctx, browser, ExploreURL)
	if err != nil {
		return nil, err
	}
	defer t.close()
	t.Log("reading the Explore page's locations, categories, time ranges and search types")
	if err := t.show(ctx); err != nil {
		return nil, t.fail(err)
	}
	out := &Pickers{}
	if err := t.CallFor(ctx, 20*time.Second, &out.Geos, "countries"); err != nil {
		return nil, err
	}
	if err := t.Call(ctx, &out.Categories, "categories"); err != nil {
		return nil, err
	}
	var opts struct {
		Times []struct {
			Value, Year, Label string
		}
		Properties []Option
	}
	if err := t.Call(ctx, &opts, "options"); err != nil {
		return nil, err
	}
	for _, o := range opts.Times {
		if n, err := strconv.Atoi(o.Value); err == nil && n >= 1 && n <= len(Times) {
			out.Times = append(out.Times, Option{Value: Times[n-1].Value, Label: o.Label})
		}
	}
	for _, o := range opts.Properties {
		if o.Value == "web" {
			o.Value = ""
		}
		out.Properties = append(out.Properties, o)
	}
	if len(out.Geos) == 0 || len(out.Categories) == 0 || len(out.Times) == 0 || len(out.Properties) == 0 {
		return nil, t.fail(fmt.Errorf("%w: found %d locations, %d categories, %d time ranges, %d search types", ErrPage,
			len(out.Geos), len(out.Categories), len(out.Times), len(out.Properties)))
	}
	if country == "" {
		return out, nil
	}
	at := -1
	for i, g := range out.Geos {
		if g.Code == country {
			at = i
		}
	}
	switch {
	case at < 0:
		return nil, fmt.Errorf("%w: the location menu has no country %q", ErrInvalid, country)
	case !out.Geos[at].HasRegions:
		return out, nil
	}
	t.Log(fmt.Sprintf("reading the regions of %s", out.Geos[at].Name))
	var r struct {
		Error   string `json:"error"`
		Regions []Geo  `json:"regions"`
	}
	if err := t.CallFor(ctx, 20*time.Second, &r, "regions", country); err != nil {
		return nil, err
	}
	if r.Error != "" || len(r.Regions) == 0 {
		return nil, t.fail(fmt.Errorf("%w: no regions of %s: %s", ErrPage, country, r.Error))
	}
	out.Geos[at].Regions = r.Regions
	return out, nil
}

// Status opens the Explore page without a search term and says which
// Explore it shows and whether the browser is signed in to Google.
func (s *Service) Status(ctx context.Context, browser string) (*Status, error) {
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	ctx, cancel := context.WithTimeout(ctx, DefaultWait)
	defer cancel()
	t, switched, err := s.open(ctx, browser, ExploreURL)
	if err != nil {
		return nil, err
	}
	defer t.close()
	st, err := t.state(ctx)
	if err != nil {
		return nil, err
	}
	return &Status{NewExplore: st.Explore, Switched: switched, SignedIn: st.SignedIn, URL: st.URL}, nil
}
