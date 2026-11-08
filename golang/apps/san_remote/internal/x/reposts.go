package x

import (
	"context"
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/page"
)

// repostsJS reads a profile's timeline from X's own app, in the page's
// world.
//
//go:embed reposts.js
var repostsJS string

// The user, 2026-11-08: "Can we have node to get reposted post, and cancel
// reposted post", for a workflow that every 5 minutes sends each repost's
// link to a topic and then undoes the repost.

// Repost is a post the account reposted, as its profile's Reposts tab shows
// it: the reposted post's link, id and author, its text as written (not X's
// translation), when it was made, its media types (photo, video,
// animated_gif), and when the account reposted it. X's page does not show
// that time: it is read from X's own app data, the repost's time, and is
// zero when that is not there.
type Repost struct {
	Link, ID string
	Author   string // the name shown
	Handle   string // without @
	Text     string
	Created  time.Time
	Reposted time.Time
	HasMedia bool
	Media    []string
}

// Unreposted is what Unrepost did: Undone, or nothing, as the account had
// not reposted the post at Link.
type Unreposted struct {
	Link   string
	Undone bool
}

// DefaultReposts is how many reposts Reposts lists when not told.
const DefaultReposts = 20

// repostRow is what reposts.js returns of one article.
type repostRow struct {
	Context    string   `json:"context"`
	By         string   `json:"by"`
	Promoted   bool     `json:"promoted"`
	App        bool     `json:"app"`
	Reposter   string   `json:"reposter"`
	RepostedAt string   `json:"repostedAt"`
	ID         string   `json:"id"`
	Handle     string   `json:"handle"`
	Name       string   `json:"name"`
	Text       string   `json:"text"`
	Created    string   `json:"created"`
	Media      []string `json:"media"`
	Href       string   `json:"href"`
	At         string   `json:"at"`
	User       []string `json:"user"`
	Shown      string   `json:"shown"`
	DrawnMedia bool     `json:"drawnMedia"`
}

// statusPath is a post's path on x.com: /<handle>/status/<id>.
var statusPath = regexp.MustCompile(`^/([A-Za-z0-9_]+)/status/(\d+)`)

// reposted picks handle's reposts from rows, in their order: the rows X's
// app holds as a repost by handle, or whose social context says handle
// reposted them ("<name> reposted" linking to /<handle>, or "You reposted"
// when handle is the signed-in account, self), so a change in either still
// finds them. The account's own
// posts, its pinned post and ads are left out. X's app data comes first;
// what the page draws fills in for it.
func reposted(rows []repostRow, handle string, self bool) []Repost {
	var out []Repost
	for _, r := range rows {
		if r.Promoted || !isRepost(r, handle, self) {
			continue
		}
		p := Repost{ID: r.ID, Handle: r.Handle, Author: r.Name, Text: r.Text, Media: r.Media}
		if m := statusPath.FindStringSubmatch(r.Href); m != nil {
			p.ID = cmpOr(p.ID, m[2])
			p.Handle = cmpOr(p.Handle, m[1])
		}
		if p.ID == "" || p.Handle == "" {
			continue
		}
		if !r.App {
			p.Text = r.Shown
			if len(r.User) > 0 && !strings.HasPrefix(r.User[0], "@") {
				p.Author = r.User[0]
			}
		}
		p.Text = strings.TrimSpace(strings.ReplaceAll(p.Text, "\r", ""))
		p.Link = postLink(p.Handle, p.ID)
		p.Created = xTime(cmpOr(r.Created, r.At))
		p.Reposted = xTime(r.RepostedAt)
		p.HasMedia = len(p.Media) > 0 || (!r.App && r.DrawnMedia)
		out = append(out, p)
	}
	return out
}

// isRepost says whether row r is a repost by handle, which is the
// signed-in account when self.
func isRepost(r repostRow, handle string, self bool) bool {
	switch {
	case strings.EqualFold(r.Reposter, handle), self && strings.EqualFold(r.Context, "You reposted"):
		return true
	}
	return strings.HasSuffix(strings.ToLower(r.Context), " reposted") && strings.EqualFold(strings.TrimSuffix(r.By, "/"), "/"+handle)
}

// firstLine is X's words up to the first line break: a gone page's "Hmm...
// this page doesn't exist. Try searching for something else." is followed
// by its Search button.
func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(s)
}

// postLink is a post's link as X writes it.
func postLink(handle, id string) string {
	return "https://x.com/" + handle + "/status/" + id
}

// xTime reads a time X's app or page gives: RFC 3339, or X's own
// "Wed Oct 10 20:19:24 +0000 2018"; zero when it is neither.
func xTime(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RubyDate, s); err == nil {
		return t
	}
	return time.Time{}
}

// ---- reposts -----------------------------------------------------------------------

// Reposts lists the reposts of handle (empty: the signed-in account), the
// latest repost first, up to limit (0: DefaultReposts), from its profile's
// Reposts tab, scrolling down for more. It reads in a background tab, and
// returns whose reposts they are.
func (s *Service) Reposts(ctx context.Context, browser, handle string, limit int) (string, []Repost, error) {
	if limit <= 0 {
		limit = DefaultReposts
	}
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle != "" && !handlePattern.MatchString(handle) {
		return "", nil, fmt.Errorf("%w: %q is not an X handle", ErrInvalidPost, handle)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return "", nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, false)
	if err != nil {
		return "", nil, err
	}
	defer t.Close()
	who, l, err := t.reposts(ctx, handle, limit)
	return who, l, t.Fail(err)
}

var handlePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)

type tabState struct {
	State string `json:"state"`
	Text  string `json:"text"`
}

func (t *session) reposts(ctx context.Context, handle string, limit int) (string, []Repost, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return "", nil, err
	}
	me, err := t.account(ctx)
	if err != nil {
		return "", nil, err
	}
	handle = cmpOr(handle, me.Handle)
	self := strings.EqualFold(handle, me.Handle)
	t.Log("opening @" + handle + "'s Reposts tab")
	if err := t.Navigate(ctx, "https://x.com/"+handle+"/reposts"); err != nil {
		return "", nil, err
	}
	var tab tabState
	if err := t.Until(ctx, "the Reposts tab", 30*time.Second, func() (bool, error) {
		err := t.Call(ctx, &tab, "repostsTab", handle)
		return tab.State != "", err
	}); err != nil {
		return "", nil, err
	}
	switch tab.State {
	case "gone":
		return "", nil, fmt.Errorf("%w: @%s: X says %q", ErrNotFound, handle, firstLine(tab.Text))
	case "none":
		// A profile without a Reposts tab: its Posts tab shows the reposts
		// among the account's posts, each with "You reposted" over it.
		t.Log("the profile has no Reposts tab: reading its Posts tab")
		if err := t.Navigate(ctx, "https://x.com/"+handle); err != nil {
			return "", nil, err
		}
		if err := t.Until(ctx, "the profile's posts", 30*time.Second, func() (bool, error) {
			var ok bool
			err := t.Call(ctx, &ok, "timelineLoaded")
			return ok, err
		}); err != nil {
			return "", nil, err
		}
	}
	if err := t.Look(ctx, 2*time.Second); err != nil {
		return "", nil, err
	}
	if tab.State == "empty" {
		// "You haven't reposted yet", still after a look.
		if err := t.Call(ctx, &tab, "repostsTab", handle); err != nil || tab.State == "empty" {
			return handle, nil, err
		}
	}
	seen := map[string]bool{}
	var out []Repost
	for k, still := 0, 0; len(out) < limit && still < 3 && k < 200; k++ {
		var rows []repostRow
		if err := t.CallMain(ctx, 30*time.Second, &rows, repostsJS); err != nil {
			return "", nil, err
		}
		before := len(out)
		for _, p := range reposted(rows, handle, self) {
			if !seen[p.Link] && len(out) < limit {
				seen[p.Link] = true
				out = append(out, p)
			}
		}
		if len(out) == before {
			still++
		} else {
			still = 0
		}
		if len(out) < limit {
			if err := t.Call(ctx, nil, "more"); err != nil {
				return "", nil, err
			}
			if err := t.Look(ctx, time.Second); err != nil {
				return "", nil, err
			}
		}
	}
	return handle, out, nil
}

// ---- unrepost ----------------------------------------------------------------------

// Unrepost undoes the account's repost of the post link names (its link in
// any of X's forms, or its id): on the post's page it presses the green
// Repost button, then Undo repost, and checks the button turned back. When
// the account has not reposted the post, nothing is pressed and Undone is
// false. A post that is not there is ErrNotFound. It brings Chrome to the
// front: X's menus only open in a shown page.
func (s *Service) Unrepost(ctx context.Context, browser, link string) (*Unreposted, error) {
	id, err := PostID(link)
	if err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	s.log("opening the post")
	p, err := page.Open(ctx, s.B, site, browser, "https://x.com/i/status/"+id, true, s.Progress)
	if err != nil {
		return nil, err
	}
	t := &session{p}
	defer t.Close()
	u, err := t.unrepost(ctx, id)
	return u, t.Fail(err)
}

// postState is what x.js postState reports.
type postState struct {
	Found    bool   `json:"found"`
	Gone     string `json:"gone"`
	Link     string `json:"link"`
	Reposted bool   `json:"reposted"`
	Repost   bool   `json:"repost"`
}

func (t *session) unrepost(ctx context.Context, id string) (*Unreposted, error) {
	st, err := t.waitState(ctx, "X's page", time.Minute, func(st state) bool { return st.SignedIn || st.SignedOut })
	if err != nil {
		return nil, err
	}
	if st.SignedOut && !st.SignedIn {
		return nil, fmt.Errorf("%w (the tab is at %s)", ErrNotSignedIn, st.URL)
	}
	if st.Lang != "" && !strings.HasPrefix(st.Lang, "en") {
		return nil, fmt.Errorf("%w: it is %q; switch it in X's settings (Accessibility, display and languages)", ErrLanguage, st.Lang)
	}
	var ps postState
	read := func() (bool, error) {
		ps = postState{}
		err := t.Call(ctx, &ps, "postState", id)
		return (ps.Found && (ps.Reposted || ps.Repost)) || (!ps.Found && ps.Gone != ""), err
	}
	if err := t.Until(ctx, "the post and its Repost button", 30*time.Second, read); err != nil {
		return nil, err
	}
	if !ps.Found {
		return nil, fmt.Errorf("%w: X says %q", ErrNotFound, firstLine(ps.Gone))
	}
	u := &Unreposted{Link: postLink("i", id)}
	if m := statusPath.FindStringSubmatch(ps.Link); m != nil {
		u.Link = postLink(m[1], id)
	}
	if !ps.Reposted {
		t.Log("the account has not reposted it: nothing to undo")
		return u, nil
	}
	if err := t.Look(ctx, time.Second); err != nil {
		return nil, err
	}
	t.Log("pressing the green Repost button")
	if err := t.Press(ctx, "the post's green Repost button", "openUnrepost", id); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "X's Undo repost", 10*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "undoOffered")
		return ok, err
	}); err != nil {
		return nil, err
	}
	t.Log("pressing Undo repost")
	if err := t.Press(ctx, "Undo repost", "confirmUnrepost"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "the Repost button to turn back", 15*time.Second, func() (bool, error) {
		ok, err := read()
		return ok && ps.Found && !ps.Reposted && ps.Repost, err
	}); err != nil {
		return nil, err
	}
	u.Undone = true
	return u, nil
}
