// Package gmaps reads Google Maps (www.google.com/maps) in the user's
// browser, as a person does there: a search for a keyword around a point,
// then each place it lists. No Places API. It only reads.
package gmaps

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrInvalid is a search Maps cannot run: no keyword, a point off the
	// map, a zoom out of range.
	ErrInvalid = errors.New("gmaps")
	// ErrPage is a page that did not do what a step expected.
	ErrPage = errors.New("gmaps: the page")
	// ErrBlocked is Google asking whether the browser is a robot (its
	// /sorry/ page): someone has to answer it in the browser.
	ErrBlocked = errors.New("gmaps: Google asks whether this is a robot (google.com/sorry); answer it in the browser, then try again later")
	// ErrConsent is Google's cookie consent page, which has to be answered
	// once in the browser.
	ErrConsent = errors.New("gmaps: Google asks for cookie consent; open google.com/maps in the browser and answer it once")
)

const (
	// DefaultZoom is the zoom of a search that gives none: a few streets.
	DefaultZoom = 15
	MinZoom     = 3
	MaxZoom     = 21
	// MaxLimit bounds a search: Maps lists about 120 places at most.
	MaxLimit = 500
)

// Search is what to search Maps for.
type Search struct {
	Browser string
	// The point: Lat and Lng, or PlusCode, a full plus code.
	Lat, Lng float64
	PlusCode string
	// Zoom is the map's zoom level, MinZoom to MaxZoom; 0 is DefaultZoom.
	Zoom float64
	// Keyword is what is typed in Maps' search box.
	Keyword string
	// Limit is at most how many places; 0 is all the list holds.
	Limit int
	// VisitWebsites fetches each place's website once and collects the
	// social media links on it.
	VisitWebsites bool
}

// Check reads the point (a plus code is turned into its center), and fills
// in the zoom. A search it has checked passes again unchanged: the API
// checks before Search does.
func (q *Search) Check() error {
	q.Keyword = strings.Join(strings.Fields(q.Keyword), " ")
	if q.Keyword == "" {
		return fmt.Errorf("%w: give a keyword, what to search for, such as \"coffee\"", ErrInvalid)
	}
	if q.PlusCode = strings.TrimSpace(q.PlusCode); q.PlusCode != "" {
		lat, lng, err := DecodePlusCode(q.PlusCode)
		if err != nil {
			return err
		}
		if (q.Lat != 0 || q.Lng != 0) && (q.Lat != lat || q.Lng != lng) {
			return fmt.Errorf("%w: give latitude and longitude, or a plus code, not both", ErrInvalid)
		}
		q.Lat, q.Lng = lat, lng
	} else if q.Lat == 0 && q.Lng == 0 {
		return fmt.Errorf("%w: give the point to search around: latitude and longitude, or a plus code", ErrInvalid)
	}
	if math.IsNaN(q.Lat) || q.Lat < -90 || q.Lat > 90 {
		return fmt.Errorf("%w: latitude %v: give -90 to 90", ErrInvalid, q.Lat)
	}
	if math.IsNaN(q.Lng) || q.Lng < -180 || q.Lng > 180 {
		return fmt.Errorf("%w: longitude %v: give -180 to 180", ErrInvalid, q.Lng)
	}
	if q.Zoom == 0 {
		q.Zoom = DefaultZoom
	}
	if math.IsNaN(q.Zoom) || q.Zoom < MinZoom || q.Zoom > MaxZoom {
		return fmt.Errorf("%w: zoom %v: give %d to %d (15 is a few streets, 12 a city)", ErrInvalid, q.Zoom, MinZoom, MaxZoom)
	}
	if q.Limit < 0 || q.Limit > MaxLimit {
		return fmt.Errorf("%w: limit %d: give 0 (all) to %d", ErrInvalid, q.Limit, MaxLimit)
	}
	return nil
}

// URL is the search's page, in English.
func (q *Search) URL() string {
	return fmt.Sprintf("%s/maps/search/%s/@%s,%s,%sz?hl=en", origin,
		url.PathEscape(q.Keyword), num(q.Lat, 7), num(q.Lng, 7), num(q.Zoom, 2))
}

// num writes f with at most prec decimals, and none it does not need.
func num(f float64, prec int) string { return strconv.FormatFloat(round(f, prec), 'f', -1, 64) }

func round(f float64, prec int) float64 {
	p := math.Pow(10, float64(prec))
	return math.Round(f*p) / p
}

// Place is one place Maps lists. Its JSON is what the CLI's --json and the
// MCP tool give.
type Place struct {
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Address  string `json:"address,omitempty"`
	// Phone is as Maps shows it, "0813-2345-6387"; Tel is its tel: link's
	// number, "081323456387".
	Phone   string   `json:"phone,omitempty"`
	Tel     string   `json:"tel,omitempty"`
	Website string   `json:"website,omitempty"`
	Social  []Social `json:"social,omitempty"`
	// PlusCode is the full plus code of the place's point; PlusCodeLocal is
	// the plus code as Maps shows it, short with a place name.
	PlusCode      string  `json:"plus_code,omitempty"`
	PlusCodeLocal string  `json:"plus_code_local,omitempty"`
	Lat           float64 `json:"latitude"`
	Lng           float64 `json:"longitude"`
	Rating        float64 `json:"rating,omitempty"`
	Reviews       int     `json:"reviews,omitempty"`
	// PlaceID is Google's place id, "ChIJ...", and URL its page on Maps.
	PlaceID string `json:"place_id,omitempty"`
	URL     string `json:"url"`
	// WebsiteError says why the website gave no social links, when it was
	// visited and could not be read.
	WebsiteError string `json:"website_error,omitempty"`
}

// Result is a search's places.
type Result struct {
	Places []Place `json:"places"`
	Lat    float64 `json:"latitude"`
	Lng    float64 `json:"longitude"`
	Zoom   float64 `json:"zoom"`
	// End is whether the list was read to its end.
	End bool `json:"end_of_list"`
	// Limited is whether Maps showed its limited view.
	Limited bool `json:"limited_view,omitempty"`
}

var (
	pointIn   = regexp.MustCompile(`!3d(-?\d+(?:\.\d+)?)!4d(-?\d+(?:\.\d+)?)`)
	placeIDIn = regexp.MustCompile(`!19s(ChIJ[\w-]+)`)
	starsIn   = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)\s*stars?(?:\s+([\d.,]+)\s*Reviews?)?`)
	ratingIn  = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)(?:\s*\(([\d.,]+)\))?`)
)

// pointOf reads a place's point from its link: !3d<lat>!4d<lng>.
func pointOf(link string) (lat, lng float64, ok bool) {
	m := pointIn.FindStringSubmatch(link)
	if m == nil {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(m[1], 64)
	lng, err2 := strconv.ParseFloat(m[2], 64)
	return lat, lng, err1 == nil && err2 == nil
}

// placeIDOf reads a place's id from its link: !19sChIJ...
func placeIDOf(link string) string {
	if u, err := url.PathUnescape(link); err == nil {
		link = u
	}
	if m := placeIDIn.FindStringSubmatch(link); m != nil {
		return m[1]
	}
	return ""
}

// pageURL is a place's link without what Maps adds for itself (authuser,
// g_ep, rclk), and in no language of its own.
func pageURL(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String()
}

// withEnglish adds hl=en to a Maps link, so its page is in English.
func withEnglish(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	q := u.Query()
	q.Set("hl", "en")
	q.Del("rclk")
	u.RawQuery = q.Encode()
	return u.String()
}

// starsOf reads a card's "4.9 stars 1,234 Reviews".
func starsOf(s string) (rating float64, reviews int) {
	m := starsIn.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0
	}
	return number(m[1]), count(m[2])
}

// ratingOf reads a place page's "4.9(1,234)".
func ratingOf(s string) (rating float64, reviews int) {
	m := ratingIn.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0
	}
	return number(m[1]), count(m[2])
}

func number(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return f
}

func count(s string) int {
	n, _ := strconv.Atoi(strings.NewReplacer(",", "", ".", "").Replace(s))
	return n
}

// unwrap gives the link a Google redirect (www.google.com/url?q=...) leads
// to.
func unwrap(link string) string {
	u, err := url.Parse(link)
	if err != nil || !strings.HasSuffix(u.Hostname(), "google.com") || u.Path != "/url" {
		return link
	}
	for _, k := range []string{"q", "url"} {
		if v := u.Query().Get(k); strings.HasPrefix(v, "http") {
			return v
		}
	}
	return link
}
