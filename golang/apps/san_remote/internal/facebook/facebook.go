// Package facebook posts to Facebook through the user's own browser, the
// way a person does it: on the profile, from its composer ("Buat postingan")
// with an audience, now or scheduled; on a Page the user manages, from Meta
// Business Suite's composer, which posts as the Page without switching the
// user's profile; and in a group, from the group's composer. Posts on the
// profile are listed and deleted in its Content Library (Kelola postingan).
//
// Facebook's interface must be in Indonesian: controls are found by the
// names Facebook gives them there ("Jadwalkan", "Hanya saya").
//
// The page knowledge is in facebook.js; the steps, their waits and their
// pace are in Go (service.go), on internal/page like Threads and Vidara.
package facebook

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidPost is a post Facebook would refuse, or one the tool cannot
	// make: no text and no media, a media file it does not take, a schedule
	// for a group, a target that is not known.
	ErrInvalidPost = errors.New("facebook: the post is not valid")
	// ErrTooSoon is a publish time less than MinLead from now.
	ErrTooSoon = errors.New("facebook: the publish time is too soon")
	// ErrTooLate is a publish time more than MaxDays days from now.
	ErrTooLate = errors.New("facebook: the publish time is too far ahead")
	// ErrNotSignedIn means the browser has no Facebook session.
	ErrNotSignedIn = errors.New("facebook: the browser is not signed in to Facebook")
	// ErrLanguage means Facebook's interface is not in Indonesian, which the
	// tool's labels are.
	ErrLanguage = errors.New("facebook: Facebook's interface is not in Indonesian")
	// ErrPage means Facebook did not do what a step expected: an element never
	// appeared, or a change did not take. The page may have been redesigned.
	ErrPage = errors.New("facebook: Facebook did not respond as expected")
	// ErrNoMatch means no post, Page or group matches what was asked for.
	ErrNoMatch = errors.New("facebook: nothing matches")
	// ErrAmbiguous means several posts, Pages or groups match.
	ErrAmbiguous = errors.New("facebook: several match")
)

// Scheduling limits, as the composers offer them (2026-10-24): the time list
// starts about 20 minutes from now, in 5-minute steps, and the date picker
// ends 29 days ahead.
const (
	MinLead = 20 * time.Minute
	MaxDays = 29
	Step    = 5 * time.Minute
)

// Where a post goes.
type Target string

const (
	Profile Target = "profile"
	PageT   Target = "page"
	Group   Target = "group"
)

// Who sees a profile post; a Page's posts and a group's follow their own
// settings.
type Audience string

const (
	Public  Audience = "public"
	Friends Audience = "friends"
	OnlyMe  Audience = "only_me"
)

// audienceLabel is each audience's name in the audience dialog.
var audienceLabel = map[Audience]string{Public: "Publik", Friends: "Teman", OnlyMe: "Hanya saya"}

// mediaTypes are the files the composers' file inputs accept, by extension.
var mediaTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".gif": "image/gif", ".webp": "image/webp",
	".heic": "image/heic", ".heif": "image/heif",
	".mp4": "video/mp4", ".m4v": "video/x-m4v", ".mov": "video/quicktime", ".mkv": "video/x-matroska", ".webm": "video/webm",
}

// Post is one post to make.
type Post struct {
	Browser string
	Target  Target
	// Page is the Page's name, for Target page; Group is the group's name,
	// its id or its link, for Target group.
	Page, Group string
	Text        string
	// Media are image or video files, in this order.
	Media []string
	// Audience is for the profile; empty keeps the composer's default.
	Audience Audience
	// At schedules it; zero means now, which Now must then say.
	At  time.Time
	Now bool
}

// Result is a post made: the time Facebook showed for a scheduled one.
type Result struct {
	Shown string
	At    time.Time
}

// Status is the browser's Facebook session.
type Status struct {
	SignedIn bool
	Name     string
	Profile  string
	Pages    []Place
	Groups   []Place
}

// Place is a Page or a group the user can post to.
type Place struct{ Name, URL string }

// Listed is a profile post as the Content Library lists it.
type Listed struct {
	Text, Status, When string
}

// ParseFile reads a post from a markdown file:
//
//	---
//	to: profile          # or "page: Uni_Q", or "group: FREELANCER INDONESIA"
//	audience: only_me    # public, friends or only_me (profile only)
//	at: 2026-11-16 09:00
//	---
//
//	The post's text, line breaks kept.
//
//	![](images/one.png)
//
// A markdown image puts that file in the post instead of in its text; files
// are relative to the markdown file.
func ParseFile(path string) (*Post, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("facebook: %w", err)
	}
	p, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	for i, m := range p.Media {
		if !filepath.IsAbs(m) {
			p.Media[i] = filepath.Join(filepath.Dir(path), filepath.FromSlash(m))
		}
	}
	return p, nil
}

var image = regexp.MustCompile(`!\[[^\]]*\]\(\s*<?([^)<>]+?)>?\s*\)`)
var blankLines = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)

// Parse reads a post from markdown, as ParseFile does, leaving media paths
// as the markdown gives them.
func Parse(md []byte) (*Post, error) {
	text := strings.ReplaceAll(string(md), "\r\n", "\n")
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	var front struct {
		To       string `yaml:"to"`
		Audience string `yaml:"audience"`
		At       string `yaml:"at"`
	}
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		head, body, found := strings.Cut("\n"+rest, "\n---\n")
		if !found {
			head, found = strings.CutSuffix("\n"+rest, "\n---")
			body = ""
		}
		if !found {
			return nil, fmt.Errorf("%w: the frontmatter has no closing ---", ErrInvalidPost)
		}
		if err := yaml.NewDecoder(strings.NewReader(head)).Decode(&front); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%w: frontmatter: %v", ErrInvalidPost, err)
		}
		text = body
	}
	p := &Post{Target: Profile, Audience: Audience(front.Audience)}
	if front.To != "" {
		if err := p.SetTarget(front.To); err != nil {
			return nil, err
		}
	}
	for _, m := range image.FindAllStringSubmatch(text, -1) {
		p.Media = append(p.Media, strings.TrimSpace(m[1]))
	}
	text = image.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	p.Text = strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
	if front.At != "" {
		at, err := ParseTime(front.At)
		if err != nil {
			return nil, err
		}
		p.At = at
	}
	return p, nil
}

// SetTarget reads where a post goes: "profile", "page: <name>" or
// "group: <name, id or link>".
func (p *Post) SetTarget(to string) error {
	kind, name, _ := strings.Cut(to, ":")
	name = strings.TrimSpace(name)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "profile", "":
		p.Target = Profile
	case "page":
		p.Target, p.Page = PageT, name
	case "group":
		p.Target, p.Group = Group, name
	default:
		return fmt.Errorf(`%w: to %q: give "profile", "page: <name>" or "group: <name or link>"`, ErrInvalidPost, to)
	}
	if p.Target != Profile && name == "" {
		return fmt.Errorf("%w: to %q needs the %s's name", ErrInvalidPost, to, p.Target)
	}
	return nil
}

// ParseTime reads a publish time: "2006-01-02 15:04" in local time, or RFC
// 3339 with its own offset.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf(`%w: time %q: give "2006-01-02 15:04" (local time) or RFC 3339`, ErrInvalidPost, s)
}

// Check reports what the tool or Facebook would refuse, before any browser
// is opened. now is the current time.
func (p *Post) Check(now time.Time) error {
	if p.Text == "" && len(p.Media) == 0 {
		return fmt.Errorf("%w: the post has no text and no media", ErrInvalidPost)
	}
	for _, m := range p.Media {
		if mediaType(m) == "" {
			return fmt.Errorf("%w: %s is not an image or video Facebook takes (jpg, png, gif, webp, heic, mp4, mov, mkv, webm)", ErrInvalidPost, m)
		}
		if fi, err := os.Stat(m); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidPost, err)
		} else if !fi.Mode().IsRegular() {
			return fmt.Errorf("%w: %s is not a file", ErrInvalidPost, m)
		}
	}
	switch p.Target {
	case Profile:
		if p.Audience != "" && audienceLabel[p.Audience] == "" {
			return fmt.Errorf("%w: audience %q: give public, friends or only_me", ErrInvalidPost, p.Audience)
		}
	case PageT, Group:
		if p.Audience != "" {
			return fmt.Errorf("%w: an audience is for the profile; a %s's posts follow its own settings", ErrInvalidPost, p.Target)
		}
	default:
		return fmt.Errorf("%w: target %q: give profile, page or group", ErrInvalidPost, p.Target)
	}
	switch {
	case p.Now && !p.At.IsZero():
		return fmt.Errorf("%w: give a time or now, not both", ErrInvalidPost)
	case !p.Now && p.At.IsZero():
		return fmt.Errorf("%w: give a time to schedule it, or now to post it at once", ErrInvalidPost)
	case p.At.IsZero():
		return nil
	case p.Target == Group:
		return fmt.Errorf("%w: a member cannot schedule in a group; post it now", ErrInvalidPost)
	}
	return checkTime(p.At, now)
}

func checkTime(at, now time.Time) error {
	at, now = at.Local(), now.Local()
	last := time.Date(now.Year(), now.Month(), now.Day()+MaxDays, 23, 59, 0, 0, time.Local)
	switch {
	case at.Second() != 0 || at.Nanosecond() != 0 || at.Minute()%5 != 0:
		return fmt.Errorf("%w: %s is not on a 5-minute mark, which is all the time list offers", ErrInvalidPost, at.Format("15:04:05"))
	case at.Sub(now) < MinLead:
		return fmt.Errorf("%w: %s is less than %s from now", ErrTooSoon, at.Format("2006-01-02 15:04"), MinLead)
	case at.After(last):
		return fmt.Errorf("%w: %s is more than %d days from now", ErrTooLate, at.Format("2006-01-02 15:04"), MaxDays)
	}
	return nil
}

func mediaType(name string) string {
	return mediaTypes[strings.ToLower(filepath.Ext(name))]
}

// The Indonesian names the composers use for months and days.
var (
	months      = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	shortMonths = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	weekdays    = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
)

// monthLabel is the picker's header for at's month: "November 2026".
func monthLabel(at time.Time) string { return fmt.Sprintf("%s %d", months[at.Month()-1], at.Year()) }

// dayLabel is the picker's label of at's day: "Senin, 16 November 2026", the
// day always with two digits.
func dayLabel(at time.Time) string {
	return fmt.Sprintf("%s, %02d %s %d", weekdays[at.Weekday()], at.Day(), months[at.Month()-1], at.Year())
}

// dateBox is the profile panel's date box for at: "16 Nov 2026".
func dateBox(at time.Time) string {
	return fmt.Sprintf("%d %s %d", at.Day(), shortMonths[at.Month()-1], at.Year())
}

// bsDate is how Business Suite's date box shows at: "20 November 2026".
func bsDate(at time.Time) string {
	return fmt.Sprintf("%d %s %d", at.Day(), months[at.Month()-1], at.Year())
}

// line is how the profile composer shows a chosen time: "16 Nov pada 09.00".
var line = regexp.MustCompile(`^(\d{1,2}) (\S+) pada (\d{1,2})\.(\d{2})$`)

// lineTime reads that line. It has no year: the time is taken as the first
// one from a day before now on.
func lineTime(s string, now time.Time) (time.Time, error) {
	m := line.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("%w: cannot read the time %q", ErrPage, s)
	}
	mon := -1
	for i, n := range shortMonths {
		if strings.EqualFold(n, m[2]) || strings.EqualFold(months[i], m[2]) || (i == 7 && strings.EqualFold(m[2], "Ags")) {
			mon = i
		}
	}
	if mon < 0 {
		return time.Time{}, fmt.Errorf("%w: cannot read the month in %q", ErrPage, s)
	}
	// strconv, not fmt.Sscan, which reads "09" as octal.
	day, _ := strconv.Atoi(m[1])
	hour, _ := strconv.Atoi(m[3])
	minute, _ := strconv.Atoi(m[4])
	now = now.Local()
	for y := now.Year(); ; y++ {
		at := time.Date(y, time.Month(mon+1), day, hour, minute, 0, 0, time.Local)
		if at.After(now.Add(-24 * time.Hour)) {
			return at, nil
		}
	}
}
