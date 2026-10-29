package instagram

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/wargasipil/san_remote/internal/page"
)

// Deleting (the user, 2026-10-29: "dont forget instagram delete node"): a
// published post from its page on instagram.com, a scheduled one from
// Business Suite's Dijadwalkan tab. Instagram keeps a deleted post in Recently
// deleted for 30 days; a deleted scheduled post is gone.

var (
	// ErrNoMatch means no scheduled post matches.
	ErrNoMatch = errors.New("instagram: no scheduled post matches")
	// ErrAmbiguous means several scheduled posts match; name one more
	// exactly.
	ErrAmbiguous = errors.New("instagram: several scheduled posts match")
	// ErrNotOwn means the post is not the signed-in account's: its menu has
	// no Delete.
	ErrNotOwn = errors.New("instagram: the post is not the signed-in account's")
	// ErrBadPost means a post's link or code cannot be read.
	ErrBadPost = errors.New("instagram: not a post's link or code")
)

// postLink is a post's link: instagram.com/p/<code>/, /reel/<code>/, or
// /<user>/p/<code>/.
var (
	postLink = regexp.MustCompile(`^(?:https?://)?(?:www\.)?instagram\.com/(?:[\w.]+/)?(?:p|reel|reels)/([\w-]+)/?(?:[?#].*)?$`)
	postCode = regexp.MustCompile(`^[\w-]{5,}$`)
)

// PostCode reads a post's code from its link or the code itself.
func PostCode(s string) (string, error) {
	s = strings.TrimSpace(s)
	if m := postLink.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	if postCode.MatchString(s) {
		return s, nil
	}
	return "", fmt.Errorf("%w: %q (give https://www.instagram.com/p/<code>/ or the code)", ErrBadPost, s)
}

// Delete deletes a published post of the signed-in account, named by its
// link or code: its page's More Options, Delete, then Delete when Instagram
// asks "Delete post?". Instagram keeps it in Recently deleted for 30 days.
// It returns the post as its page showed it, and brings Chrome to the
// front.
func (s *Service) Delete(ctx context.Context, browser, post string) (*Listed, error) {
	code, err := PostCode(post)
	if err != nil {
		return nil, err
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	t, err := s.open(ctx, browser, true)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	l, err := t.deletePost(ctx, code)
	return l, t.Fail(err)
}

func (t *session) deletePost(ctx context.Context, code string) (*Listed, error) {
	if err := t.home(ctx, 2*time.Second); err != nil {
		return nil, err
	}
	handle := t.account(ctx)
	if handle == "" {
		return nil, fmt.Errorf("%w: the side menu has no profile link", ErrPage)
	}
	l, err := t.read(ctx, "/p/"+code+"/")
	if err != nil {
		return nil, err
	}
	t.Log(fmt.Sprintf("deleting %s (%s) %q", l.Link, l.At.Format("2006-01-02 15:04"), clip(l.Caption, 40)))
	if err := t.Press(ctx, "More Options", "pressMoreOptions"); err != nil {
		return nil, err
	}
	var items []string
	if err := t.Until(ctx, "the post's menu", 10*time.Second, func() (bool, error) {
		err := t.Call(ctx, &items, "menu")
		return len(items) > 0, err
	}); err != nil {
		return nil, err
	}
	if !slices.Contains(items, "Delete") {
		return nil, fmt.Errorf("%w (@%s): its menu has %q", ErrNotOwn, handle, items)
	}
	if err := t.Press(ctx, "Delete", "pressDelete"); err != nil {
		return nil, err
	}
	t.Log("answering Delete to \"Delete post?\"")
	if err := t.Until(ctx, "\"Delete post?\"", 10*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "confirmDelete")
		return ok, err
	}); err != nil {
		return nil, err
	}
	// Instagram goes to the profile once the post is gone; the grid then
	// must not have it.
	if _, err := t.waitState(ctx, "Instagram to leave the post", 30*time.Second, func(st state) bool { return !strings.Contains(st.URL, code) }); err != nil {
		return nil, err
	}
	grid, err := t.grid(ctx, handle)
	if err != nil {
		return nil, err
	}
	for _, g := range grid {
		if strings.Contains(g.Href, "/"+code+"/") {
			return nil, fmt.Errorf("%w: the profile still shows %s after Delete", ErrPage, l.Link)
		}
	}
	t.Log("deleted " + l.Link)
	return l, nil
}

// DeleteScheduled deletes the one post scheduled in Business Suite whose
// caption starts with caption (spaces aside) and, when at is not zero, that
// goes out at at: its row in Dijadwalkan, Tindakan, Hapus postingan, then
// Hapus when Business Suite asks "Hapus Postingan?". It cannot be undone. It
// returns the post as the list showed it, and brings Chrome to the front.
func (s *Service) DeleteScheduled(ctx context.Context, browser, caption string, at time.Time) (*Listed, error) {
	if strings.TrimSpace(caption) == "" && at.IsZero() {
		return nil, fmt.Errorf("%w: give the post's caption (its start will do), its time, or both", ErrInvalidPost)
	}
	done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	sp, err := page.Open(ctx, s.B, bsSite, browser, bsHomeURL, true, s.Progress)
	if err != nil {
		return nil, err
	}
	t := &bsSession{Session: sp}
	defer t.Close()
	l, err := t.deleteScheduled(ctx, caption, at)
	return l, t.Fail(err)
}

// matches says which of list hold caption at their start (spaces aside) and,
// when at is not zero, go out at at.
func matches(list []Listed, caption string, at time.Time) []int {
	squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
	var hits []int
	for i, l := range list {
		if strings.HasPrefix(squash(l.Caption), squash(caption)) && (at.IsZero() || l.At.Equal(at.Local())) {
			hits = append(hits, i)
		}
	}
	return hits
}

func (t *bsSession) deleteScheduled(ctx context.Context, caption string, at time.Time) (*Listed, error) {
	list, err := t.scheduled(ctx)
	if err != nil {
		return nil, err
	}
	what := fmt.Sprintf("%q", caption)
	if !at.IsZero() {
		what += " at " + at.Local().Format("2006-01-02 15:04")
	}
	hits := matches(list, caption, at)
	switch len(hits) {
	case 0:
		return nil, fmt.Errorf("%w %s (%d scheduled)", ErrNoMatch, what, len(list))
	case 1:
	default:
		return nil, fmt.Errorf("%w %s (%d of them); not deleting", ErrAmbiguous, what, len(hits))
	}
	l := list[hits[0]]
	t.Log(fmt.Sprintf("deleting the post scheduled %s: %q", l.When, clip(l.Caption, 40)))
	if err := t.Press(ctx, "the post's checkbox", "tickRow", hits[0]); err != nil {
		return nil, err
	}
	// The panel must be open, and that post's row the only one ticked,
	// before anything is pressed in it: its preview loads late (2026-10-29,
	// "Memuat cuplikan" for more than 15 seconds), so the row tells which
	// post it is.
	var ticked []string
	if err := t.Until(ctx, "the post's Detail Postingan", 30*time.Second, func() (bool, error) {
		var p string
		if err := t.Call(ctx, &p, "panel"); err != nil {
			return false, err
		}
		err := t.Call(ctx, &ticked, "ticked")
		return p != "" && len(ticked) == 1 && ticked[0] == l.Caption, err
	}); err != nil {
		return nil, fmt.Errorf("%w (rows ticked: %q)", err, ticked)
	}
	if err := t.Press(ctx, "Tindakan", "pressActions"); err != nil {
		return nil, err
	}
	if err := t.Until(ctx, "Tindakan's Hapus postingan", 10*time.Second, func() (bool, error) {
		var items []string
		err := t.Call(ctx, &items, "menuItems")
		return slices.Contains(items, "Hapus postingan"), err
	}); err != nil {
		return nil, err
	}
	if err := t.Press(ctx, "Hapus postingan", "menuItem", "Hapus postingan"); err != nil {
		return nil, err
	}
	t.Log("answering Hapus to \"Hapus Postingan?\"")
	if err := t.Until(ctx, "\"Hapus Postingan?\"", 10*time.Second, func() (bool, error) {
		var ok bool
		err := t.Call(ctx, &ok, "confirm", "Hapus Postingan?", "Hapus")
		return ok, err
	}); err != nil {
		return nil, err
	}
	// The list redraws late: read it again from the page until the post
	// has left it.
	if err := t.Until(ctx, "the post to leave Dijadwalkan", time.Minute, func() (bool, error) {
		if err := page.Sleep(ctx, 3*time.Second); err != nil {
			return false, err
		}
		again, err := t.scheduled(ctx)
		if err != nil {
			return false, err
		}
		return len(matches(again, l.Caption, l.At)) < len(matches(list, l.Caption, l.At)), nil
	}); err != nil {
		return nil, err
	}
	t.Log("deleted the post scheduled " + l.When)
	return &l, nil
}
