package san_linkedin

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ErrDraftNotFound means no draft with that id is in the Drafts tab of the
// Articles dialog: it does not exist, it was deleted, or it is not a draft
// (a published or scheduled article is never touched by DeleteDraft).
var ErrDraftNotFound = errors.New("san_linkedin: no such draft")

// DraftContent is what a draft holds, read from the editor.
type DraftContent struct {
	ID     string
	URL    string
	Status string // what the editor shows in its corner, "Draft - saved" for a draft
	Title  string
	Cover  string // the cover image's URL on LinkedIn's media host, or ""
	Text   string // the whole body as text, the subtitle paragraph included
	Images []DraftImage
}

type DraftImage struct {
	URL     string // on LinkedIn's media host
	Alt     string
	Caption string
}

var (
	idOnly  = regexp.MustCompile(`^\d{10,25}$`)
	idInURL = regexp.MustCompile(`(?:/article/edit/|fsd_firstPartyArticle:|linkedInArticle:)(\d{10,25})`)
)

// DraftID takes a draft's id, or its edit URL
// (https://www.linkedin.com/article/edit/<id>/), and returns the id.
func DraftID(s string) (string, error) {
	s = strings.TrimSpace(s)
	if idOnly.MatchString(s) {
		return s, nil
	}
	if m := idInURL.FindStringSubmatch(s); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("san_linkedin: %q is not a LinkedIn article id or edit URL", s)
}

// Drafts lists the drafts in the Drafts tab of the editor's Articles dialog
// (Manage, then Drafts), as LinkedIn orders them.
func (c *Client) Drafts(ctx context.Context) ([]Draft, error) {
	var out []Draft
	err := c.session(ctx, nil, func(e *editor) error {
		rows, err := e.articles("Drafts")
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.ID == "" {
				continue
			}
			out = append(out, Draft{ID: r.ID, URL: c.editURL(r.ID), Title: r.Title, Modified: r.Date})
		}
		return nil
	})
	return out, err
}

// GetDraft opens a draft from the Drafts tab, as a person clicks its title,
// and reads it. id may also be its edit URL. Nothing in the draft is
// changed.
func (c *Client) GetDraft(ctx context.Context, id string) (*DraftContent, error) {
	id, err := DraftID(id)
	if err != nil {
		return nil, err
	}
	var out *DraftContent
	err = c.session(ctx, nil, func(e *editor) error {
		if err := e.openDraft(id); err != nil {
			return err
		}
		var s snapshot
		// Images come in a moment after the text.
		if err := e.until("the draft to settle", 10*time.Second, func() (bool, error) {
			var a snapshot
			if err := e.call(&a, "snapshot"); err != nil {
				return false, err
			}
			still := len(a.Figures) == len(s.Figures) && a.Text == s.Text && a.Text != ""
			s = a
			return still, nil
		}); err != nil && !errors.Is(err, ErrEditor) {
			return err
		}
		out = &DraftContent{ID: id, URL: c.editURL(id), Status: s.Status, Title: s.Title, Cover: s.Cover, Text: s.Text}
		for _, f := range s.Figures {
			if f != nil {
				out.Images = append(out.Images, DraftImage{URL: f.Src, Alt: f.Alt, Caption: f.Caption})
			}
		}
		return nil
	})
	return out, err
}

// openDraft opens draft id in the editor by clicking its title in the Drafts
// tab, and fails with ErrDraftNotFound when it is not listed there.
func (e *editor) openDraft(id string) error {
	if d := e.partial(); d != nil && d.ID == id {
		var ready bool
		if err := e.call(&ready, "editorReady"); err == nil && ready {
			return nil // already open, as right after NewDraft
		}
	}
	rows, err := e.articles("Drafts")
	if err != nil {
		return err
	}
	if !listed(rows, id) {
		return fmt.Errorf("%w: %s", ErrDraftNotFound, id)
	}
	if err := e.clickOn("the draft's title", "markRowLink", id); err != nil {
		return err
	}
	// The click loads the draft; until the page has a script context, a
	// failed question only means "not yet".
	return e.until("the draft in the editor", time.Minute, func() (bool, error) {
		if d := e.partial(); d == nil || d.ID != id {
			return false, nil
		}
		var (
			ready bool
			list  *articleList
		)
		if e.call(&ready, "editorReady") != nil || e.call(&list, "articles") != nil {
			return false, nil
		}
		return ready && list == nil, nil
	})
}

// DeleteDraft deletes a draft, as a person would in the Drafts tab: the
// row's menu, "Delete", then LinkedIn's question. It cannot be undone. Only
// rows of the Drafts tab are looked at, so a published or scheduled article
// gives ErrDraftNotFound and is never touched. It returns once a reload of
// the tab no longer lists the draft.
func (c *Client) DeleteDraft(ctx context.Context, id string) error {
	id, err := DraftID(id)
	if err != nil {
		return err
	}
	return c.session(ctx, nil, func(e *editor) error {
		rows, err := e.articles("Drafts")
		if err != nil {
			return err
		}
		if !listed(rows, id) {
			return fmt.Errorf("%w: %s", ErrDraftNotFound, id)
		}
		// The question takes the row's place, title and link included, so
		// the row is known by where it is while it asks.
		var at int
		if err := e.call(&at, "rowIndex", id); err != nil {
			return err
		}
		if at < 0 {
			return fmt.Errorf("%w: the row of %s is not shown", ErrEditor, id)
		}
		if err := e.clickOn("the draft's menu", "markRowMenu", id); err != nil {
			return err
		}
		if err := e.wait(`"Delete" in the draft's menu`, 5*time.Second, "markRowMenuItem", id, "Delete"); err != nil {
			return err
		}
		if err := e.clickOn(`"Delete"`, "markRowMenuItem", id, "Delete"); err != nil {
			return err
		}
		if err := e.wait(`LinkedIn's "Delete draft?" question`, 10*time.Second, "markRowAsk", at, `Delete draft\?`, "Delete"); err != nil {
			return fmt.Errorf("%w%s", err, e.screenshot("delete"))
		}
		if err := e.clickOn(`"Delete" in the question`, "markRowAsk", at, `Delete draft\?`, "Delete"); err != nil {
			return err
		}
		if err := e.until("the row to go", 15*time.Second, func() (bool, error) {
			var l *articleList
			if err := e.call(&l, "articles"); err != nil {
				return false, err
			}
			return l != nil && len(l.Rows) == len(rows)-1 && !listed(l.Rows, id), nil
		}); err != nil {
			return err
		}

		// The proof: the tab, reloaded.
		if err := e.reload("the Articles dialog", "articles"); err != nil {
			return err
		}
		rows, err = e.listRows("Drafts")
		if err != nil {
			return err
		}
		if listed(rows, id) {
			return fmt.Errorf("%w: %s is still listed after the delete", ErrEditor, id)
		}
		return nil
	})
}

func listed(rows []articleRow, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

type articleRow struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Href  string `json:"href"`
	Date  string `json:"date"`
}

type articleList struct {
	Tab     string       `json:"tab"`
	Rows    []articleRow `json:"rows"`
	Loading bool         `json:"loading"`
	More    bool         `json:"more"` // a "Show more results" button below the rows
	Empty   bool         `json:"empty"`
}

// articles opens a tab of the editor's Articles dialog ("Drafts",
// "Scheduled" or "Published") the way a person does, from the editor's
// Manage menu, and returns all its rows. Outside the editor, the tab goes
// there first with the feed's "Write article"; an article only comes to be
// once something is typed in it, so that makes nothing.
func (e *editor) articles(tab string) ([]articleRow, error) {
	var open *articleList
	if err := e.call(&open, "articles"); err != nil {
		open = nil
	}
	if open == nil {
		var ready bool
		if err := e.call(&ready, "editorReady"); err != nil || !ready {
			if err := e.newArticle(); err != nil {
				return nil, err
			}
		}
		if err := e.clickOn(`"Manage"`, "markManage"); err != nil {
			return nil, err
		}
		if err := e.wait(`"`+tab+`" in the Manage menu`, 5*time.Second, "markDropdownItem", tab); err != nil {
			return nil, err
		}
		if err := e.clickOn(`"`+tab+`"`, "markDropdownItem", tab); err != nil {
			return nil, err
		}
		if err := e.awaitPage("the Articles dialog", "articles"); err != nil {
			return nil, err
		}
	}
	return e.listRows(tab)
}

// listRows shows tab in the open Articles dialog and reads its rows,
// scrolling the list until no more come.
func (e *editor) listRows(tab string) ([]articleRow, error) {
	var l *articleList
	if err := e.call(&l, "articles"); err != nil {
		return nil, err
	}
	if l == nil {
		return nil, fmt.Errorf("%w: the Articles dialog is not open", ErrEditor)
	}
	if l.Tab != tab {
		if err := e.clickOn(`the "`+tab+`" tab`, "markTab", tab); err != nil {
			return nil, err
		}
	}
	var (
		last  = -1
		since time.Time
	)
	for page := 1; ; page++ {
		err := e.until("the "+tab+" list", time.Minute, func() (bool, error) {
			if err := e.call(&l, "articles"); err != nil {
				return false, err
			}
			if l == nil || l.Tab != tab || l.Loading {
				return false, nil
			}
			if l.Empty {
				return true, nil
			}
			// Rows still loading, or more behind a scroll.
			if len(l.Rows) != last {
				last, since = len(l.Rows), time.Now()
				return false, e.call(nil, "scrollArticles")
			}
			return len(l.Rows) > 0 && time.Since(since) > 2*time.Second, nil
		})
		if err != nil {
			return nil, err
		}
		if l.Empty || !l.More {
			return l.Rows, nil
		}
		if page == 50 {
			return nil, fmt.Errorf("%w: the %s list still shows more after 50 pages", ErrEditor, tab)
		}
		// Ten rows at a time, then "Show more results", clicked as a person
		// clicks it.
		n := len(l.Rows)
		if err := e.clickOn(`"Show more results"`, "markMore"); err != nil {
			return nil, err
		}
		if err := e.until("more rows", 30*time.Second, func() (bool, error) {
			var m *articleList
			err := e.call(&m, "articles")
			return m != nil && len(m.Rows) > n, err
		}); err != nil {
			return nil, err
		}
	}
}

func (c *Client) editURL(id string) string {
	return strings.TrimRight(c.opts.baseURL, "/") + "/article/edit/" + id + "/"
}
