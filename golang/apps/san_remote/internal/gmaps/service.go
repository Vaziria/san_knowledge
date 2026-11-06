package gmaps

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/wargasipil/san_remote/internal/page"
)

//go:embed gmaps.js
var gmapsJS string

var site = page.Site{Name: "gmaps", Global: "__sanGmaps", JS: gmapsJS, ErrPage: ErrPage}

const origin = "https://www.google.com"

var (
	// ListWait is how long the list may go without growing, after a scroll,
	// before it is taken as all Maps gives.
	ListWait = 8 * time.Second
	// maxScrolls bounds the scrolls of one list.
	maxScrolls = 80
	// websites is how many websites are read at once.
	websites = 4
)

// Service reads Google Maps in the browser. Its calls run one at a time,
// since each one works in a tab of the user's browser. The CLI, the MCP
// server and the API each run one, in their own process, through
// page.Remote.
type Service struct {
	B page.Browser
	// Progress gets each step as it starts, such as "reading place 3 of 20".
	Progress func(step string)
	// HTTP fetches the places' websites; nil is http.DefaultClient.
	HTTP *http.Client
	// Shown opens the tab in front rather than behind the user's. Maps
	// loads the next places of its list as it is scrolled, which a tab
	// behind others may do slowly.
	Shown bool

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

func (s *Service) progress(step string) {
	if s.Progress != nil {
		s.Progress(step)
	}
}

// Search searches Maps for q.Keyword around q's point, reads the list (to
// its end, or q.Limit places), then opens each place in turn and reads it.
// With q.VisitWebsites, each place's website is then fetched, from here
// rather than the browser, for the social media links on it.
func (s *Service) Search(ctx context.Context, q Search) (*Result, error) {
	if err := q.Check(); err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	res := &Result{Lat: q.Lat, Lng: q.Lng, Zoom: q.Zoom}
	if err := s.read(ctx, q, res); err != nil {
		return nil, err
	}
	if q.VisitWebsites {
		s.visit(ctx, res.Places)
	}
	return res, ctx.Err()
}

// read opens the search in a tab and reads its places, then closes the tab.
func (s *Service) read(ctx context.Context, q Search, res *Result) error {
	p, err := page.Open(ctx, s.B, site, q.Browser, q.URL(), s.Shown, s.Progress)
	if err != nil {
		return err
	}
	t := &session{Session: p}
	defer t.Close()
	t.Log(fmt.Sprintf("searching Google Maps for %q around %s,%s at zoom %s", q.Keyword, num(q.Lat, 7), num(q.Lng, 7), num(q.Zoom, 2)))
	list, err := t.list(ctx, q.Limit, res)
	if err != nil {
		return t.Fail(err)
	}
	for i, l := range list {
		if i > 0 {
			if err := t.Pause(ctx); err != nil {
				return err
			}
		}
		t.Log(fmt.Sprintf("reading place %d of %d: %s", i+1, len(list), l.Name))
		pl, err := t.place(ctx, l)
		if err != nil {
			return t.Fail(fmt.Errorf("%w (place %d of %d, %s)", err, i+1, len(list), l.Name))
		}
		res.Places = append(res.Places, *pl)
	}
	return nil
}

// visit fetches each place's website, a few at a time, and adds the social
// profiles it links to. A website that cannot be read says why in
// WebsiteError; the search goes on.
func (s *Service) visit(ctx context.Context, places []Place) {
	hc := s.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	var todo []int
	for i, p := range places {
		if _, social := socialOf(p.Website); p.Website != "" && !social {
			todo = append(todo, i)
		}
	}
	if len(todo) == 0 {
		return
	}
	s.progress(fmt.Sprintf("reading the websites of %d places for their social media links", len(todo)))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(websites, len(todo)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				p := &places[i]
				links, err := fetchSocial(ctx, hc, p.Website)
				if err != nil {
					p.WebsiteError = err.Error()
					continue
				}
				p.Social = addSocial(p.Social, links...)
			}
		}()
	}
	for _, i := range todo {
		if ctx.Err() != nil {
			break
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
}

// session is the one tab a search works in.
type session struct {
	*page.Session
}

// state is what gmaps.js state() reports.
type state struct {
	URL     string `json:"url"`
	Consent bool   `json:"consent"`
	Blocked bool   `json:"blocked"`
	Feed    bool   `json:"feed"`
	Results int    `json:"results"`
	End     bool   `json:"end"`
	None    bool   `json:"none"`
	Limited bool   `json:"limited"`
	Title   string `json:"title"`
	Items   int    `json:"items"`
}

// onPlace is whether the tab shows a place's page.
func (st state) onPlace() bool { return !st.Feed && st.Title != "" && st.Title != "Results" }

func (t *session) state(ctx context.Context) (state, error) {
	var st state
	err := t.Call(ctx, &st, "state")
	return st, err
}

// waitState waits until the page's state passes ok. A call that lands while
// a page loads can be refused by the browser; that is only a reason to ask
// again. Google's robot check and its cookie consent end the wait with
// their errors.
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
		case st.Blocked:
			return false, ErrBlocked
		case st.Consent:
			return false, ErrConsent
		}
		return ok(st), nil
	})
	return st, err
}

// listed is what gmaps.js results() reports of a place.
type listed struct {
	Name  string `json:"name"`
	Href  string `json:"href"`
	Stars string `json:"stars"`
}

// list waits for the search's page and reads its places: it scrolls the
// list until Maps says it has reached the end, limit places are listed
// (0: no limit), or the list stops growing. A search Maps finds one place
// for shows that place's page, which is the only place.
func (t *session) list(ctx context.Context, limit int, res *Result) ([]listed, error) {
	st, err := t.waitState(ctx, "Maps' list of places", 45*time.Second, func(st state) bool {
		return st.Feed || st.None || (st.onPlace() && st.Items > 0)
	})
	if err != nil {
		return nil, err
	}
	res.Limited = st.Limited
	switch {
	case st.None:
		t.Log("Maps found no place")
		res.End = true
		return nil, nil
	case !st.Feed:
		res.End = true
		return []listed{{Name: st.Title, Href: st.URL}}, nil
	}
	for n := 0; !st.End && (limit == 0 || st.Results < limit) && n < maxScrolls; n++ {
		t.Log(fmt.Sprintf("%d places listed; scrolling for more", st.Results))
		if err := t.Do(ctx, "Maps' list of places", "more"); err != nil {
			return nil, err
		}
		before := st.Results
		st, err = t.waitState(ctx, "more places", ListWait, func(s state) bool { return s.Results > before || s.End || !s.Feed })
		if errors.Is(err, ErrPage) && st.Feed {
			break // the list stopped growing: that is all Maps gives
		}
		if err != nil {
			return nil, err
		}
	}
	res.End, res.Limited = st.End, st.Limited
	var list []listed
	if err := t.Call(ctx, &list, "results"); err != nil {
		return nil, err
	}
	if limit > 0 && len(list) > limit {
		list = list[:limit]
		res.End = false
	}
	t.Log(fmt.Sprintf("%d places listed (end of the list: %v)", len(list), res.End))
	return list, nil
}

// read is what gmaps.js place() reports.
type read struct {
	URL      string `json:"url"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Address  string `json:"address"`
	Phone    string `json:"phone"`
	Tel      string `json:"tel"`
	Website  string `json:"website"`
	PlusCode string `json:"plusCode"`
	Rating   string `json:"rating"`
}

// place opens a listed place's page and reads it.
func (t *session) place(ctx context.Context, l listed) (*Place, error) {
	if err := t.Navigate(ctx, withEnglish(l.Href)); err != nil {
		return nil, err
	}
	st, err := t.waitState(ctx, "the place's page", 30*time.Second, func(st state) bool { return st.onPlace() })
	if err != nil {
		return nil, err
	}
	if st.Items == 0 {
		// The details come in a moment after the name; a place may have
		// none at all.
		t.waitState(ctx, "the place's details", 5*time.Second, func(st state) bool { return st.Items > 0 })
	}
	var r read
	if err := t.Call(ctx, &r, "place"); err != nil {
		return nil, err
	}
	p := &Place{
		Name: r.Name, Category: r.Category, Address: r.Address, Phone: r.Phone, Tel: r.Tel,
		Website: unwrap(r.Website), PlusCodeLocal: r.PlusCode,
		PlaceID: placeIDOf(l.Href), URL: pageURL(l.Href),
	}
	if p.Name == "" {
		p.Name = l.Name
	}
	lat, lng, ok := pointOf(l.Href)
	if !ok {
		lat, lng, ok = pointOf(r.URL)
	}
	if ok {
		p.Lat, p.Lng = lat, lng
		p.PlusCode = PlusCode(lat, lng, 10)
	}
	if p.Rating, p.Reviews = starsOf(l.Stars); p.Rating == 0 {
		p.Rating, p.Reviews = ratingOf(r.Rating)
	}
	if s, ok := socialOf(p.Website); ok {
		p.Social = []Social{s}
	}
	return p, nil
}
