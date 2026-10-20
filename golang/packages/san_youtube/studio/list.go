package studio

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Listed is a video as Studio's content list shows it.
type Listed struct {
	Title      string `json:"title"`
	Visibility string `json:"visibility"` // "Scheduled", "Private", "Public", …
	Date       string `json:"date"`       // such as "Oct 16, 2026 Scheduled"
	Short      bool   `json:"short"`      // from the Shorts tab
}

// selDialogClose is the upload dialog's close (X) button, once studio.js's
// markClose has found and marked it.
const selDialogClose = `[data-san-close]`

// Videos reads Studio's content list, the Videos tab and then the Shorts tab,
// newest first: what is already up or scheduled, to check before scheduling
// another. It gets there the way a person does: Create and Upload video, as
// Upload does, then closing the empty upload dialog, which leaves Studio on
// the content list. Nothing is uploaded or changed.
func (c *Client) Videos(ctx context.Context, opt ...UploadOption) ([]Listed, error) {
	o := uploadOptions{progress: func(string) {}}
	for _, f := range opt {
		f(&o)
	}
	var out []Listed
	err := c.session(ctx, o.progress, func(t *tab) error {
		if err := t.openUpload(); err != nil {
			return err
		}
		var channel string
		_ = t.call(&channel, "channel")
		if channel != "" {
			t.progress("channel: " + channel)
		}
		t.progress("closing the upload dialog")
		var found bool
		if err := t.until("the upload dialog's close button", 20*time.Second, func() (bool, error) {
			err := t.call(&found, "markClose")
			return err == nil && found, err
		}); err != nil {
			var buttons []string
			_ = t.call(&buttons, "dialogButtons")
			return fmt.Errorf("%w; the dialog's buttons: %q", err, buttons)
		}
		if err := t.click(selDialogClose); err != nil {
			return err
		}
		for _, tab := range []struct {
			sel, name string
			short     bool
		}{{selVideosTab, "Videos", false}, {selShortsTab, "Shorts", true}} {
			t.progress("reading " + tab.name)
			if err := t.click(tab.sel); err != nil {
				return err
			}
			// The rows fill in after the tab shows: read until two reads agree.
			var rows []Listed
			n := -1
			err := t.until("the "+tab.name+" list", 20*time.Second, func() (bool, error) {
				if err := t.call(&rows, "rows"); err != nil {
					return false, err
				}
				done := len(rows) > 0 && len(rows) == n
				n = len(rows)
				return done, nil
			})
			if errors.Is(err, ErrStudio) {
				continue // an empty tab
			}
			if err != nil {
				return err
			}
			for i := range rows {
				rows[i].Short = tab.short
			}
			out = append(out, rows...)
		}
		return nil
	})
	return out, err
}
