package san_medium

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrDraftNotFound means no draft with that id is in the Drafts tab of the
// stories page: it does not exist, it was deleted, or it is not a draft (a
// published story is never touched by DeleteDraft).
var ErrDraftNotFound = errors.New("san_medium: no such draft")

// DraftContent is what a draft holds, read from the editor.
type DraftContent struct {
	ID       string
	URL      string
	Status   string // what the editor shows next to the logo, "Draft" for a draft
	Title    string
	Subtitle string
	Text     string // the whole story as text, title and captions included
	Images   []DraftImage
}

type DraftImage struct {
	URL     string // on Medium's image host
	Alt     string
	Caption string
}

var (
	idOnly  = regexp.MustCompile(`^[0-9a-f]{8,16}$`)
	idInURL = regexp.MustCompile(`/p/([0-9a-f]{8,16})(?:/|$|\?)`)
	idAtEnd = regexp.MustCompile(`-([0-9a-f]{8,16})(?:$|\?)`)
	words   = regexp.MustCompile(`\(([\d.,]+) words?\)`)
)

// DraftID takes a draft's id, or any URL of it (https://medium.com/p/<id>/edit,
// or a story URL ending in -<id>), and returns the id.
func DraftID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if idOnly.MatchString(s) {
		return s, nil
	}
	if m := idInURL.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	if m := idAtEnd.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("san_medium: %q is not a Medium post id or URL", s)
}

// Drafts lists the drafts on the stories page, newest first, as Medium
// orders them.
func (c *Client) Drafts(ctx context.Context) ([]Draft, error) {
	var out []Draft
	err := c.session(ctx, nil, func(e *editor) error {
		rows, err := e.drafts()
		if err != nil {
			return err
		}
		for _, r := range rows {
			if !r.Draft {
				continue
			}
			d := Draft{ID: r.ID, URL: c.editURL(r.ID), Title: r.Title, Updated: r.Updated}
			if m := words.FindStringSubmatch(r.Read); m != nil {
				d.Words, _ = strconv.Atoi(strings.NewReplacer(",", "", ".", "").Replace(m[1]))
			}
			out = append(out, d)
		}
		return nil
	})
	return out, err
}

// GetDraft opens a draft in the editor and reads it. id may also be a URL.
// Nothing in the draft is changed.
func (c *Client) GetDraft(ctx context.Context, id string) (*DraftContent, error) {
	id, err := DraftID(id)
	if err != nil {
		return nil, err
	}
	var out *DraftContent
	err = c.session(ctx, nil, func(e *editor) error {
		if err := e.open(c.editURL(id)); err != nil {
			return err
		}
		var s struct {
			Status   string         `json:"status"`
			Title    string         `json:"title"`
			Subtitle string         `json:"subtitle"`
			Figures  []*figureState `json:"figures"`
			Text     string         `json:"text"`
		}
		if err := e.call(&s, "snapshot"); err != nil {
			return err
		}
		out = &DraftContent{ID: id, URL: c.editURL(id), Status: s.Status, Title: s.Title, Subtitle: s.Subtitle, Text: s.Text}
		for _, f := range s.Figures {
			if f != nil {
				out.Images = append(out.Images, DraftImage{URL: f.Src, Alt: f.Alt, Caption: f.Caption})
			}
		}
		return nil
	})
	return out, err
}

// DeleteDraft deletes a draft, as a person would on the stories page: the
// row's menu, "Delete story", then "Delete" in Medium's "this action cannot
// be undone" question. It cannot be undone. Only rows of the Drafts tab are
// looked at, so a published story gives ErrDraftNotFound and is never
// touched. It returns once a reload of the page no longer lists the draft.
func (c *Client) DeleteDraft(ctx context.Context, id string) error {
	id, err := DraftID(id)
	if err != nil {
		return err
	}
	return c.session(ctx, nil, func(e *editor) error {
		rows, err := e.drafts()
		if err != nil {
			return err
		}
		found := false
		for _, r := range rows {
			found = found || (r.ID == id && r.Draft)
		}
		if !found {
			return fmt.Errorf("%w: %s", ErrDraftNotFound, id)
		}

		var ok bool
		if err := e.call(&ok, "markRowMenu", id); err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: no menu button on the row of %s", ErrEditor, id)
		}
		if err := e.click(marked); err != nil {
			return err
		}
		// The row's menu is the dialog with "Edit story" in it; Medium's
		// question is the one that says the deletion cannot be undone.
		if err := e.wait(`"Delete story" in the menu`, 5*time.Second, "markDialogButton", "Delete story", "Edit story"); err != nil {
			return err
		}
		if err := e.click(marked); err != nil {
			return err
		}
		if err := e.wait(`Medium's "Delete" question`, 5*time.Second, "markDialogButton", "Delete", "Are you sure you want to delete"); err != nil {
			return err
		}
		if err := e.click(marked); err != nil {
			return err
		}
		if err := e.until("the row to go", 15*time.Second, func() (bool, error) {
			var has bool
			err := e.call(&has, "hasRow", id)
			return !has, err
		}); err != nil {
			return err
		}

		// The proof: a fresh load of the page.
		rows, err = e.drafts()
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.ID == id {
				return fmt.Errorf("%w: %s is still listed after the delete", ErrEditor, id)
			}
		}
		return nil
	})
}

// marked is the element a page helper picked for the next click.
const marked = `[data-san-medium="target"]`

type storyRow struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Draft   bool   `json:"draft"`
	Read    string `json:"read"`
	Updated string `json:"updated"`
}

// drafts opens the stories page on its Drafts tab and returns every row,
// scrolling until all of the tab's count are loaded or no more come.
func (e *editor) drafts() ([]storyRow, error) {
	if err := e.load(e.client.opts.baseURL+"/me/stories", "the stories page", "draftsReady"); err != nil {
		return nil, err
	}
	var (
		list struct {
			Count int        `json:"count"`
			Rows  []storyRow `json:"rows"`
		}
		last  = -1
		since time.Time
	)
	err := e.until("the drafts list", time.Minute, func() (bool, error) {
		if err := e.call(&list, "drafts"); err != nil {
			return false, err
		}
		if len(list.Rows) >= list.Count {
			return true, nil
		}
		// Rows still loading, or more behind a scroll.
		if len(list.Rows) != last {
			last, since = len(list.Rows), time.Now()
			return false, e.call(nil, "scrollEnd")
		}
		return time.Since(since) > 5*time.Second, nil
	})
	return list.Rows, err
}

func (c *Client) editURL(id string) string {
	return strings.TrimRight(c.opts.baseURL, "/") + "/p/" + id + "/edit"
}
