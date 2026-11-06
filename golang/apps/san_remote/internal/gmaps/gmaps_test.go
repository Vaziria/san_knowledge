package gmaps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/internal/page"
)

// The codes are the reference library's (openlocationcode 1.0.1 for Python).
func TestPlusCode(t *testing.T) {
	for _, c := range []struct {
		lat, lng float64
		n        int
		want     string
	}{
		{-6.1931299, 106.8152684, 10, "6P58RR48+P4"}, // Toko Kopi Style, Jakarta; Maps shows RR48+P4
		{47.365562, 8.524813, 10, "8FVC9G8F+6W"},
		{-41.2730625, 174.7859375, 10, "4VCPPQGP+Q9"},
		{90, 1, 10, "CFX3X2X2+X2"},
		{-90, -180, 10, "22222222+22"},
		{0, 180, 10, "62G22222+22"},
		{1.2833, 103.85, 11, "6PH57VM2+82C"},
		{-6.2, 106.816, 10, "6P58RR28+2C"},
		{40.7484, -73.9857, 12, "87G8P2X7+9P74"},
	} {
		if got := PlusCode(c.lat, c.lng, c.n); got != c.want {
			t.Errorf("PlusCode(%v, %v, %d) = %s, want %s", c.lat, c.lng, c.n, got, c.want)
		}
	}
}

func TestDecodePlusCode(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-7 }
	for _, c := range []struct {
		code     string
		lat, lng float64
	}{
		{"6P58RR48+P4", -6.1931875, 106.8153125},
		{" 6p58rr48+p4 ", -6.1931875, 106.8153125},
		{"8FVC9G8F+6X", 47.3655625, 8.5249375},
		{"6P580000+", -6.5, 106.5},
		{"7FG49QCJ+2VX", 20.3701125, 2.78223437},
	} {
		lat, lng, err := DecodePlusCode(c.code)
		if err != nil || !near(lat, c.lat) || !near(lng, c.lng) {
			t.Errorf("DecodePlusCode(%q) = %v, %v, %v; want %v, %v", c.code, lat, lng, err, c.lat, c.lng)
		}
	}
	for _, code := range []string{"", "RR48+P4", "RR48+P4 Kebon Kacang, Jakarta", "6P58RR48P4", "6P58RR48+P", "6P58RR4A+P4", "6P58RR48++P4", "6P5800+", "6P580000+P4", "XP58RR48+P4", "6X58RR48+P4", "6P58RR48X+P4"} {
		if _, _, err := DecodePlusCode(code); !errors.Is(err, ErrInvalid) {
			t.Errorf("DecodePlusCode(%q) = %v, want ErrInvalid", code, err)
		}
	}
	// Round trip: a code's center has the same code.
	for _, code := range []string{"6P58RR48+P4", "4VCPPQGP+Q9", "87G8P2X7+9P74"} {
		lat, lng, _ := DecodePlusCode(code)
		if got := PlusCode(lat, lng, len(code)-1); got != code {
			t.Errorf("PlusCode(DecodePlusCode(%s)) = %s", code, got)
		}
	}
}

func TestSearchCheck(t *testing.T) {
	q := Search{Lat: -6.2, Lng: 106.816, Keyword: "  coffee   shop "}
	if err := q.Check(); err != nil {
		t.Fatal(err)
	}
	if q.Zoom != DefaultZoom || q.Keyword != "coffee shop" {
		t.Errorf("checked = %+v", q)
	}
	if got, want := q.URL(), "https://www.google.com/maps/search/coffee%20shop/@-6.2,106.816,15z?hl=en"; got != want {
		t.Errorf("URL = %s, want %s", got, want)
	}
	q = Search{PlusCode: "6P58RR48+P4", Keyword: "kopi", Zoom: 13.5}
	if err := q.Check(); err != nil {
		t.Fatal(err)
	}
	if again := q; again.Check() != nil || again != q {
		t.Errorf("a checked search does not pass again unchanged: %+v", again)
	}
	if math.Abs(q.Lat - -6.1931875) > 1e-7 || q.URL() != "https://www.google.com/maps/search/kopi/@-6.1931875,106.8153125,13.5z?hl=en" {
		t.Errorf("plus code search = %+v, %s", q, q.URL())
	}
	for _, bad := range []Search{
		{Lat: -6.2, Lng: 106.8}, // no keyword
		{Keyword: "kopi"},       // no point
		{Lat: -6.2, Lng: 106.8, PlusCode: "6P58RR48+P4", Keyword: "kopi"}, // both
		{Lat: 91, Lng: 106.8, Keyword: "kopi"},
		{Lat: -6.2, Lng: 181, Keyword: "kopi"},
		{Lat: -6.2, Lng: 106.8, Keyword: "kopi", Zoom: 2},
		{Lat: -6.2, Lng: 106.8, Keyword: "kopi", Zoom: 22},
		{Lat: -6.2, Lng: 106.8, Keyword: "kopi", Limit: -1},
		{Lat: -6.2, Lng: 106.8, Keyword: "kopi", Limit: MaxLimit + 1},
		{PlusCode: "RR48+P4 Jakarta", Keyword: "kopi"},
	} {
		if err := bad.Check(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Check(%+v) = %v, want ErrInvalid", bad, err)
		}
	}
}

const tokoKopi = "https://www.google.com/maps/place/Toko+Kopi+Style/data=!4m7!3m6!1s0x2e69f7000685c423:0x21c4cf00756a2979!8m2!3d-6.1931299!4d106.8152684!16s%2Fg%2F11ntq065rh!19sChIJI8SFBgD3aS4ReSlqdQDPxCE?authuser=0&hl=en&g_ep=EgoyMDI2MTEwMy4wIJJjKgBIAVAD&rclk=1"

func TestLinks(t *testing.T) {
	lat, lng, ok := pointOf(tokoKopi)
	if !ok || lat != -6.1931299 || lng != 106.8152684 {
		t.Errorf("pointOf = %v, %v, %v", lat, lng, ok)
	}
	if id := placeIDOf(tokoKopi); id != "ChIJI8SFBgD3aS4ReSlqdQDPxCE" {
		t.Errorf("placeIDOf = %q", id)
	}
	if u := pageURL(tokoKopi); strings.Contains(u, "?") || !strings.HasSuffix(u, "!19sChIJI8SFBgD3aS4ReSlqdQDPxCE") {
		t.Errorf("pageURL = %s", u)
	}
	if u := withEnglish(strings.Replace(tokoKopi, "hl=en", "hl=id", 1)); !strings.Contains(u, "hl=en") || strings.Contains(u, "rclk") {
		t.Errorf("withEnglish = %s", u)
	}
	for _, c := range []struct {
		in      string
		rating  float64
		reviews int
	}{{"4.9 stars 1,234 Reviews", 4.9, 1234}, {"5.0 stars", 5, 0}, {"4.7 stars 87 Reviews", 4.7, 87}, {"", 0, 0}} {
		if r, n := starsOf(c.in); r != c.rating || n != c.reviews {
			t.Errorf("starsOf(%q) = %v, %d", c.in, r, n)
		}
	}
	if r, n := ratingOf("4.6(2,045)"); r != 4.6 || n != 2045 {
		t.Errorf("ratingOf = %v, %d", r, n)
	}
	if u := unwrap("https://www.google.com/url?q=https://kopi.id/&opi=1"); u != "https://kopi.id/" {
		t.Errorf("unwrap = %s", u)
	}
	if u := unwrap("https://kopi.id/"); u != "https://kopi.id/" {
		t.Errorf("unwrap of a plain link = %s", u)
	}
}

func TestSocialOf(t *testing.T) {
	for _, c := range []struct{ in, site, url string }{
		{"https://www.instagram.com/tokokopistyle/?hl=id", "instagram", "https://www.instagram.com/tokokopistyle"},
		{"http://instagram.com/tokokopistyle", "instagram", "https://www.instagram.com/tokokopistyle"},
		{"https://m.facebook.com/tokokopi/posts/1", "facebook", "https://www.facebook.com/tokokopi"},
		{"https://web.facebook.com/profile.php?id=1000123&sk=about", "facebook", "https://www.facebook.com/profile.php?id=1000123"},
		{"https://www.facebook.com/people/Kopi-Nako/100063720000000/?ref=x", "facebook", "https://www.facebook.com/people/Kopi-Nako/100063720000000"},
		{"https://www.facebook.com/pages/Kopi/123", "facebook", "https://www.facebook.com/pages/Kopi/123"},
		{"https://www.tiktok.com/@tokokopi?lang=id", "tiktok", "https://www.tiktok.com/@tokokopi"},
		{"https://twitter.com/tokokopi", "x", "https://x.com/tokokopi"},
		{"https://www.youtube.com/@tokokopi/videos", "youtube", "https://www.youtube.com/@tokokopi"},
		{"https://youtube.com/channel/UC123/about", "youtube", "https://www.youtube.com/channel/UC123"},
		{"https://id.linkedin.com/company/toko-kopi/", "linkedin", "https://www.linkedin.com/company/toko-kopi"},
		{"https://wa.me/6281323456387?text=halo", "whatsapp", "https://wa.me/6281323456387"},
		{"https://api.whatsapp.com/send?phone=+6281323456387", "whatsapp", "https://wa.me/6281323456387"},
		{"https://t.me/tokokopi", "telegram", "https://t.me/tokokopi"},
		{"https://www.threads.net/@tokokopi", "threads", "https://www.threads.net/@tokokopi"},
	} {
		s, ok := socialOf(c.in)
		if !ok || s.Site != c.site || s.URL != c.url {
			t.Errorf("socialOf(%q) = %+v, %v; want %s %s", c.in, s, ok, c.site, c.url)
		}
	}
	for _, in := range []string{
		"https://kopi.id/", "https://www.instagram.com/", "https://www.instagram.com/p/Cx1/", "https://www.facebook.com/sharer/sharer.php?u=x",
		"https://twitter.com/intent/tweet?text=x", "https://www.youtube.com/watch?v=1", "https://www.youtube.com/embed/1",
		"https://www.tiktok.com/embed/1", "https://www.linkedin.com/shareArticle?url=x", "https://api.whatsapp.com/send?text=x",
		"https://www.facebook.com/tr?id=1", "https://www.facebook.com/people", "https://www.facebook.com/people/", "mailto:a@kopi.id", "https://notinstagram.com/x",
	} {
		if s, ok := socialOf(in); ok {
			t.Errorf("socialOf(%q) = %+v, want none", in, s)
		}
	}
}

func TestSocialIn(t *testing.T) {
	// A website, and a link page such as Linktree, whose links are in its
	// script's JSON with escaped slashes.
	html := `<a href="https://www.instagram.com/tokokopi/">IG</a>
<a href="https://www.facebook.com/sharer/sharer.php?u=https://kopi.id">share</a>
<a href="https://wa.me/6281323456387?text=Halo&amp;x=1">WA</a>
<script id="__NEXT_DATA__">{"links":[{"url":"https:\/\/www.tiktok.com\/@tokokopi"},{"url":"https:\/\/instagram.com\/tokokopi"}]}</script>`
	got := socialIn(html)
	var s []string
	for _, l := range got {
		s = append(s, l.Site+" "+l.URL)
	}
	want := "instagram https://www.instagram.com/tokokopi|whatsapp https://wa.me/6281323456387|tiktok https://www.tiktok.com/@tokokopi"
	if strings.Join(s, "|") != want {
		t.Errorf("socialIn = %s\nwant %s", strings.Join(s, "|"), want)
	}
}

// fakeMaps is a Maps tab: a search lists places two at a time as it is
// scrolled, and each place has a page.
type fakeMaps struct {
	page.Browser
	mu       sync.Mutex
	places   []fakePlace
	shown    int
	at       string // the tab's page: "search" or a place's link
	opened   string
	navs     []string
	limited  bool
	blockAt  int // the place page that shows Google's robot check (1-based; 0: none)
	stall    bool
	hasShown bool
}

type fakePlace struct {
	listed listed
	read   read
}

func (f *fakeMaps) OpenTab(_ context.Context, r *v1.OpenTabRequest) (*v1.OpenTabResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened, f.at, f.shown, f.hasShown = r.GetUrl(), "search", 2, r.GetFocus()
	return &v1.OpenTabResponse{Tab: &v1.Tab{Id: 1}}, nil
}

func (f *fakeMaps) CloseTab(context.Context, *v1.CloseTabRequest) (*v1.CloseTabResponse, error) {
	return &v1.CloseTabResponse{}, nil
}

func (f *fakeMaps) Navigate(_ context.Context, r *v1.NavigateRequest) (*v1.NavigateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at = r.GetUrl()
	f.navs = append(f.navs, r.GetUrl())
	return &v1.NavigateResponse{}, nil
}

func (f *fakeMaps) Screenshot(context.Context, *v1.ScreenshotRequest) (*v1.ScreenshotResponse, error) {
	return nil, errors.New("no screen in a test")
}

var helperCall = regexp.MustCompile(`globalThis\.__sanGmaps\.(\w+)\(([^\n]*)\);$`)

func (f *fakeMaps) Exec(_ context.Context, r *v1.ExecRequest) (*v1.ExecResponse, error) {
	m := helperCall.FindStringSubmatch(r.GetCode())
	if m == nil {
		return nil, fmt.Errorf("fake: not a helper call: %.100s", r.GetCode())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out any
	switch m[1] {
	case "state":
		st := state{URL: f.at, Limited: f.limited}
		if f.at == "search" {
			st.Feed, st.Results, st.End, st.Title = true, min(f.shown, len(f.places)), f.shown >= len(f.places), "Results"
		} else if p := f.placeAt(); p != nil {
			if f.blockAt > 0 && strings.HasPrefix(f.at, pageURL(f.places[f.blockAt-1].listed.Href)) {
				st.Blocked = true
			}
			st.Title, st.Items = p.read.Name, 3
		}
		out = st
	case "results":
		var list []listed
		for _, p := range f.places[:min(f.shown, len(f.places))] {
			list = append(list, p.listed)
		}
		out = list
	case "more":
		if !f.stall {
			f.shown += 2
		}
		out = true
	case "place":
		p := f.placeAt()
		if p == nil {
			return nil, errors.New("fake: no place page")
		}
		out = p.read
	default:
		return nil, fmt.Errorf("fake: no helper %s", m[1])
	}
	b, err := json.Marshal(out)
	return &v1.ExecResponse{ResultJson: string(b)}, err
}

// placeAt is the place whose page the tab shows (its link with hl=en).
func (f *fakeMaps) placeAt() *fakePlace {
	for i := range f.places {
		if strings.HasPrefix(f.at, pageURL(f.places[i].listed.Href)) {
			return &f.places[i]
		}
	}
	return nil
}

func fakePlaces(n int) []fakePlace {
	var out []fakePlace
	for i := range n {
		name := fmt.Sprintf("Kopi %d", i+1)
		href := fmt.Sprintf("https://www.google.com/maps/place/Kopi+%d/data=!4m7!3m6!1s0x1:0x%d!8m2!3d-6.19%d!4d106.81%d!16s%%2Fg%%2F1!19sChIJkopi%d?authuser=0&hl=en&rclk=1", i+1, i+1, i+1, i+1, i+1)
		out = append(out, fakePlace{
			listed: listed{Name: name, Href: href, Stars: "4.5 stars 10 Reviews"},
			read: read{
				URL: href, Name: name, Category: "Coffee shop", Address: fmt.Sprintf("Jl. Kopi No.%d, Jakarta", i+1),
				Phone: "0813-2345-638" + fmt.Sprint(i), Tel: "081323456387" + fmt.Sprint(i), PlusCode: "RR48+P4 Kebon Kacang, Central Jakarta City, Jakarta",
			},
		})
	}
	return out
}

func TestSearch(t *testing.T) {
	old := ListWait
	ListWait = 300 * time.Millisecond
	t.Cleanup(func() { ListWait = old })

	// The websites: one links to its Instagram, one does not answer, one is
	// an Instagram profile itself.
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/down" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprint(w, `<a href="https://www.instagram.com/kopi1/">IG</a><a href="https://wa.me/62813">WA</a>`)
	}))
	t.Cleanup(site.Close)

	f := &fakeMaps{places: fakePlaces(5)}
	f.places[0].read.Website = "https://www.google.com/url?q=" + site.URL + "/&opi=1"
	f.places[1].read.Website = site.URL + "/down"
	f.places[2].read.Website = "https://www.instagram.com/kopi3/"
	s := &Service{B: f}

	res, err := s.Search(context.Background(), Search{Lat: -6.2, Lng: 106.816, Keyword: "kopi", VisitWebsites: true})
	if err != nil {
		t.Fatal(err)
	}
	if f.opened != "https://www.google.com/maps/search/kopi/@-6.2,106.816,15z?hl=en" || f.hasShown {
		t.Errorf("opened %s (shown %v)", f.opened, f.hasShown)
	}
	if len(res.Places) != 5 || !res.End || res.Zoom != 15 {
		t.Fatalf("%d places, end %v, zoom %v", len(res.Places), res.End, res.Zoom)
	}
	for _, n := range f.navs {
		if !strings.Contains(n, "hl=en") || strings.Contains(n, "rclk") {
			t.Errorf("navigated to %s", n)
		}
	}
	p := res.Places[0]
	if p.Name != "Kopi 1" || p.Category != "Coffee shop" || p.Address != "Jl. Kopi No.1, Jakarta" || p.Tel != "0813234563870" ||
		p.Lat != -6.191 || p.Lng != 106.811 || p.PlusCode != PlusCode(-6.191, 106.811, 10) || p.PlusCodeLocal == "" ||
		p.Rating != 4.5 || p.Reviews != 10 || p.PlaceID != "ChIJkopi1" || strings.Contains(p.URL, "?") {
		t.Errorf("place 1 = %+v", p)
	}
	if p.Website != site.URL+"/" || len(p.Social) != 2 || p.Social[0].URL != "https://www.instagram.com/kopi1" || p.Social[1].Site != "whatsapp" {
		t.Errorf("place 1's website and social = %s %+v", p.Website, p.Social)
	}
	if p := res.Places[1]; p.WebsiteError != "HTTP 403" || len(p.Social) != 0 {
		t.Errorf("place 2 = %q %+v", p.WebsiteError, p.Social)
	}
	if p := res.Places[2]; len(p.Social) != 1 || p.Social[0].URL != "https://www.instagram.com/kopi3" || p.WebsiteError != "" {
		t.Errorf("place 3 = %+v", p.Social)
	}

	// A limit stops the list there, before its end.
	f = &fakeMaps{places: fakePlaces(5)}
	res, err = (&Service{B: f}).Search(context.Background(), Search{PlusCode: "6P58RR48+P4", Keyword: "kopi", Limit: 3})
	if err != nil || len(res.Places) != 3 || res.End || len(res.Places[0].Social) != 0 {
		t.Fatalf("limit 3: %v places, end %v, %v", len(res.Places), res.End, err)
	}

	// A list that stops growing is all Maps gives, with its limited view.
	f = &fakeMaps{places: fakePlaces(5), stall: true, limited: true}
	res, err = (&Service{B: f}).Search(context.Background(), Search{Lat: -6.2, Lng: 106.816, Keyword: "kopi"})
	if err != nil || len(res.Places) != 2 || res.End || !res.Limited {
		t.Fatalf("stalled: %v places, end %v, limited %v, %v", len(res.Places), res.End, res.Limited, err)
	}

	// Google's robot check ends the search.
	f = &fakeMaps{places: fakePlaces(3), blockAt: 2}
	if _, err := (&Service{B: f}).Search(context.Background(), Search{Lat: -6.2, Lng: 106.816, Keyword: "kopi"}); !errors.Is(err, ErrBlocked) {
		t.Fatalf("robot check: %v, want ErrBlocked", err)
	}

	// A bad search opens no tab.
	f = &fakeMaps{}
	if _, err := (&Service{B: f}).Search(context.Background(), Search{Keyword: "kopi"}); !errors.Is(err, ErrInvalid) || f.opened != "" {
		t.Fatalf("no point: %v, opened %q", err, f.opened)
	}
}

// Each helper of gmaps.js is named once, and every one the Go steps call is
// there. Line ends are read either way: a Windows checkout has CRLF.
func TestScript(t *testing.T) {
	js := strings.ReplaceAll(gmapsJS, "\r\n", "\n")
	_, helpers, ok := strings.Cut(js, "\n  return {\n")
	if !ok {
		t.Fatal("gmaps.js returns no object of helpers")
	}
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^    (?:async )?([A-Za-z]\w*)\(`).FindAllStringSubmatch(helpers, -1) {
		if seen[m[1]] {
			t.Errorf("gmaps.js defines %s twice", m[1])
		}
		seen[m[1]] = true
	}
	b, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	called := regexp.MustCompile(`(?:Call|CallFor|Do)\(ctx, (?:&?\w+, |"[^"]*", )?"(\w+)"`).FindAllStringSubmatch(string(b), -1)
	if len(called) < 4 {
		t.Fatalf("service.go calls %d helpers, want at least 4", len(called))
	}
	for _, m := range called {
		if !seen[m[1]] {
			t.Errorf("service.go calls %s, which gmaps.js does not define", m[1])
		}
	}
}
