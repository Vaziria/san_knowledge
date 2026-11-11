// Package trends reads Google Trends' new Explore page
// (trends.google.com/explore) in the user's browser, as a person does there:
// a keyword's related queries ("Commonly searched queries": Top and Rising,
// each with its search interest and change) and the page's own filter lists.
// Trends has no open API, and the page's own calls carry a reCAPTCHA token
// that only the page makes, so the page makes them and san_remote reads what
// they bring (a hook on the page's fetch, in its main world). It only reads.
//
// Only the new Explore is read (the user, 2026-11-11: "Don't use classic
// explore"). A page that opens in the classic Explore (/trends/explore, as
// Google does for an hl=en-US link or an account set to it) is switched to
// the new one first (the user, 2026-11-11: "If it classic explore, switch
// automatically").
package trends

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrInvalid is a lookup Trends cannot run: no keyword, a time range or
	// search type it does not have.
	ErrInvalid = errors.New("trends")
	// ErrPage is a page that did not do what a step expected.
	ErrPage = errors.New("trends: the page")
	// ErrRateLimited is Google limiting the browser's lookups: an HTTP 429
	// or its "unusual traffic" page. Wait a while before the next one.
	ErrRateLimited = errors.New("trends: Google is limiting this browser's Trends lookups (too many requests, or its \"unusual traffic\" check); wait a while before the next one, and answer the check in the browser if it shows one")
	// ErrConsent is Google's cookie consent page, which has to be answered
	// once in the browser.
	ErrConsent = errors.New("trends: Google asks for cookie consent; open trends.google.com in the browser and answer it once")
	// ErrClassic is a page that stays on the classic Explore after the
	// switch to the new one.
	ErrClassic = errors.New("trends: Trends keeps opening the classic Explore; open trends.google.com/explore in the browser, switch to the new Explore once, then try again")
)

// Origin is Trends' address, and ExploreURL the new Explore page.
const (
	Origin     = "https://trends.google.com"
	ExploreURL = Origin + "/explore"
)

// MaxKeyword is the longest keyword Trends takes, in characters.
const MaxKeyword = 100

// DefaultTime is the page's default time range, Past year.
const DefaultTime = "today 12-m"

// Times are the time ranges of the page's time menu with a value of their
// own, by its data-value (1 to 9), as the page writes them in its date
// parameter (2026-11-11: picking Past week gave date=now 7-d, Past 3 months
// today 3-m, the year 2025 2025-01-01 2025-12-31). A year (data-value 10) or
// Custom is a range, "YYYY-MM-DD YYYY-MM-DD".
var Times = []Option{
	{"now 1-H", "Past hour"},
	{"now 4-H", "Past 4 hours"},
	{"now 1-d", "Past 24 hours"},
	{"now 7-d", "Past week"},
	{"today 1-m", "Past month"},
	{"today 3-m", "Past 3 months"},
	{"today 12-m", "Past year"},
	{"today 5-y", "Past 5 years"},
	{"all", "2004 - present"},
}

// Properties are the page's search types, by their data-value; Web Search
// is "" in a request, "web" on the page.
var Properties = []Option{
	{"", "Web Search"},
	{"images", "Image Search"},
	{"news", "News Search"},
	{"froogle", "Google Shopping"},
	{"youtube", "YouTube Search"},
}

// Option is one choice of a menu: its value and the page's label.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var dateRange = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}) (\d{4}-\d{2}-\d{2})$`)

// geoCode is a location code: a country ("ID") or a region ("ID-JK",
// "US-CA-803").
var geoCode = regexp.MustCompile(`^[A-Z]{2}(-[A-Z0-9]{1,3}){0,2}$`)

// Query is a lookup of a keyword's related queries.
type Query struct {
	Browser string
	// Keyword is the search term.
	Keyword string
	// Geo is the location: "ID", "ID-JK", "" for Worldwide.
	Geo string
	// Time is the page's date value (Times, or a range); "" is DefaultTime.
	Time string
	// Category is the category's id; 0 is All Categories.
	Category int
	// Property is the search type: "", images, news, froogle or youtube.
	Property string
	// Wait bounds the lookup; 0 is 2 minutes.
	Wait time.Duration
}

// Check tidies the query and refuses one the page cannot run. A query it has
// checked passes again unchanged: the API checks before Related does.
func (q *Query) Check() error {
	q.Keyword = strings.Join(strings.Fields(q.Keyword), " ")
	switch {
	case q.Keyword == "":
		return fmt.Errorf("%w: give a keyword, what to look up, such as \"aplikasi kasir\"", ErrInvalid)
	case utf8.RuneCountInString(q.Keyword) > MaxKeyword:
		return fmt.Errorf("%w: a keyword has at most %d characters", ErrInvalid, MaxKeyword)
	}
	q.Geo = strings.ToUpper(strings.TrimSpace(q.Geo))
	if q.Geo == "WORLDWIDE" {
		q.Geo = ""
	}
	if q.Geo != "" && !geoCode.MatchString(q.Geo) {
		return fmt.Errorf("%w: location %q: give a country's code (ID), a region's (ID-JK), or none for Worldwide", ErrInvalid, q.Geo)
	}
	q.Time = strings.Join(strings.Fields(q.Time), " ")
	if q.Time == "" {
		q.Time = DefaultTime
	}
	if err := CheckTime(q.Time); err != nil {
		return err
	}
	if q.Category < 0 {
		return fmt.Errorf("%w: category %d: give a category's id, 0 for All Categories", ErrInvalid, q.Category)
	}
	q.Property = strings.ToLower(strings.TrimSpace(q.Property))
	if q.Property == "web" {
		q.Property = ""
	}
	if !slicesHas(Properties, q.Property) {
		return fmt.Errorf("%w: search type %q: give images, news, froogle (Google Shopping), youtube, or none for Web Search", ErrInvalid, q.Property)
	}
	if q.Wait < 0 {
		return fmt.Errorf("%w: wait %s", ErrInvalid, q.Wait)
	}
	return nil
}

// CheckTime refuses a time range the page does not have: one of Times, or
// a range of two dates from 2004-01-01, the first not after the second.
func CheckTime(t string) error {
	if slicesHas(Times, t) {
		return nil
	}
	m := dateRange.FindStringSubmatch(t)
	if m == nil {
		return fmt.Errorf("%w: time %q: give one of %s, or a range \"YYYY-MM-DD YYYY-MM-DD\"", ErrInvalid, t, timeValues())
	}
	from, err1 := time.Parse(time.DateOnly, m[1])
	to, err2 := time.Parse(time.DateOnly, m[2])
	switch {
	case err1 != nil || err2 != nil:
		return fmt.Errorf("%w: time %q: not a date", ErrInvalid, t)
	case from.Year() < 2004:
		return fmt.Errorf("%w: time %q: Trends starts at 2004-01-01", ErrInvalid, t)
	case to.Before(from):
		return fmt.Errorf("%w: time %q: the range ends before it starts", ErrInvalid, t)
	}
	return nil
}

func timeValues() string {
	v := make([]string, len(Times))
	for i, t := range Times {
		v[i] = strconv.Quote(t.Value)
	}
	return strings.Join(v, ", ")
}

func slicesHas(opts []Option, v string) bool {
	for _, o := range opts {
		if o.Value == v {
			return true
		}
	}
	return false
}

// URL is the new Explore's page of keyword with the query's filters, the
// way the page writes its own address (q, geo, date, cat, gprop). No hl: the
// new Explore sends an hl=en-US link to the classic one (2026-11-11).
func (q *Query) URL() string { return ExploreLink(q.Keyword, q.Geo, q.Time, q.Category, q.Property) }

// ExploreLink is the new Explore's page of keyword with these filters; the
// defaults (Past year, All Categories, Web Search) are left out, as the page
// leaves them out.
func ExploreLink(keyword, geo, tm string, category int, property string) string {
	v := []string{"q=" + escape(keyword)}
	if geo != "" {
		v = append(v, "geo="+escape(geo))
	}
	if tm != "" && tm != DefaultTime {
		v = append(v, "date="+escape(tm))
	}
	if category > 0 {
		v = append(v, "cat="+strconv.Itoa(category))
	}
	if property != "" {
		v = append(v, "gprop="+escape(property))
	}
	return ExploreURL + "?" + strings.Join(v, "&")
}

// escape is a query value as the page writes one: a space is %20.
func escape(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }

// Item is one query of a Top or Rising list. The new Explore gives both
// numbers for every query: its search interest (0-100) and its change
// against the period before, in percent.
type Item struct {
	Query          string `json:"query"`
	SearchInterest int    `json:"search_interest"`
	Change         int    `json:"change"`
	// Breakout: the change is past what Trends measures; the page shows
	// "Breakout" for it.
	Breakout bool   `json:"breakout,omitempty"`
	Link     string `json:"link,omitempty"`
}

// BreakoutChange is the change the page shows as Breakout (2026-11-11: a
// query of 5000 in its data showed BREAKOUT, one of 550 "+550%").
const BreakoutChange = 5000

// ChangeText is the change as the page shows it: "+550%", "-20%", "No
// change", "Breakout".
func (it Item) ChangeText() string {
	switch {
	case it.Breakout:
		return "Breakout"
	case it.Change > 0:
		return fmt.Sprintf("+%d%%", it.Change)
	case it.Change < 0:
		return fmt.Sprintf("%d%%", it.Change)
	}
	return "No change"
}

// Result is a keyword's related queries: Top by search interest, Rising by
// change (only those that rose).
type Result struct {
	Keyword string `json:"keyword"`
	Top     []Item `json:"top"`
	Rising  []Item `json:"rising"`
	URL     string `json:"url"`
	// Switched: Trends opened the classic Explore first.
	Switched bool `json:"switched,omitempty"`
}

// relatedRPC is the batchexecute call of the page's related queries
// ("Commonly searched queries"), and its reply.
const relatedRPC = "fXqlme"

// ParseRelated reads the page's related-queries reply (a batchexecute
// response: ")]}'", then chunks, each a length line and a JSON line). Its
// payload is [[[term, rising, top]]]: rising rows are [query, change,
// interest] by change, top rows [query, interest, change] by interest
// (2026-11-11, "aplikasi kasir": the same 48 queries in both, sorted two
// ways). A term with too little data gives an empty payload, "[]", or a
// term without its rising and top parts (2026-11-12, "gamis rompi lepas
// 2026": the page's tables empty): no queries, not an error (the user: "if
// it dont have child query just skip"). link makes each item's link.
func ParseRelated(body string, link func(query string) string) (top, rising []Item, err error) {
	payload, err := rpcPayload(body, relatedRPC)
	if err != nil {
		return nil, nil, err
	}
	var data []any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, nil, fmt.Errorf("%w: the related queries are not JSON: %v", ErrPage, err)
	}
	if len(data) == 0 {
		return []Item{}, []Item{}, nil
	}
	terms, ok := data[0].([]any)
	if !ok || len(terms) == 0 {
		return []Item{}, []Item{}, nil
	}
	term, ok := terms[0].([]any)
	if terms[0] != nil && !ok {
		return nil, nil, fmt.Errorf("%w: the related queries have an unknown shape", ErrPage)
	}
	// A part the term lacks, or holds as null, has no queries.
	for len(term) < 3 {
		term = append(term, nil)
	}
	rows := func(v any, interestAt, changeAt int) ([]Item, error) {
		list, _ := v.([]any)
		out := []Item{}
		for _, r := range list {
			row, ok := r.([]any)
			if !ok || len(row) < 3 {
				return nil, fmt.Errorf("%w: a related query has an unknown shape", ErrPage)
			}
			q, _ := row[0].(string)
			in, ok1 := row[interestAt].(float64)
			ch, ok2 := row[changeAt].(float64)
			if q == "" || !ok1 || !ok2 {
				return nil, fmt.Errorf("%w: a related query has an unknown shape: %v", ErrPage, row)
			}
			it := Item{Query: q, SearchInterest: int(in), Change: int(ch), Breakout: int(ch) >= BreakoutChange}
			if link != nil {
				it.Link = link(q)
			}
			out = append(out, it)
		}
		return out, nil
	}
	if rising, err = rows(term[1], 2, 1); err != nil {
		return nil, nil, err
	}
	if top, err = rows(term[2], 1, 2); err != nil {
		return nil, nil, err
	}
	// The page's Rising list goes on into the queries that fell; only
	// those that rose are rising.
	up := rising[:0]
	for _, it := range rising {
		if it.Change > 0 {
			up = append(up, it)
		}
	}
	return top, up, nil
}

// rpcPayload finds rpc's reply in a batchexecute response: its payload, a
// JSON text. An error reply ("er") or none at all is an ErrPage.
func rpcPayload(body, rpc string) (string, error) {
	body = strings.TrimPrefix(strings.TrimSpace(body), ")]}'")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		var chunk []json.RawMessage
		if json.Unmarshal([]byte(line), &chunk) != nil {
			continue
		}
		for _, raw := range chunk {
			var e []any
			if json.Unmarshal(raw, &e) != nil || len(e) < 3 {
				continue
			}
			kind, _ := e[0].(string)
			id, _ := e[1].(string)
			if id != rpc {
				continue
			}
			switch kind {
			case "wrb.fr":
				if s, ok := e[2].(string); ok {
					return s, nil
				}
				return "", fmt.Errorf("%w: Trends answered its %s call with no data", ErrPage, rpc)
			case "er":
				return "", fmt.Errorf("%w: Trends answered its %s call with an error: %s", ErrPage, rpc, raw)
			}
		}
	}
	return "", fmt.Errorf("%w: no %s reply in the page's call", ErrPage, rpc)
}

// Geo is a location of the page's location menu.
type Geo struct {
	// Code is the page's: "ID", "ID-JK"; "" for Worldwide.
	Code       string `json:"code"`
	Name       string `json:"name"`
	HasRegions bool   `json:"has_regions,omitempty"`
	Regions    []Geo  `json:"regions,omitempty"`
}

// Category is a category of the page's category menu.
type Category struct {
	ID       int        `json:"id"`
	Name     string     `json:"name"`
	Children []Category `json:"children,omitempty"`
}

// Pickers are the page's own filter lists.
type Pickers struct {
	Geos       []Geo      `json:"geos"`
	Categories []Category `json:"categories"`
	Times      []Option   `json:"times"`
	Properties []Option   `json:"properties"`
}

// Status is what the Explore page says of the browser.
type Status struct {
	NewExplore bool   `json:"new_explore"`
	Switched   bool   `json:"switched,omitempty"`
	SignedIn   bool   `json:"signed_in"`
	URL        string `json:"url"`
}
