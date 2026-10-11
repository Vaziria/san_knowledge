package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/chromedp/chromedp"
)

// The draft WriteDraft makes.
const (
	draftTitle = "Uji draft (hapus saja)"
	draftText  = "Satu baris."
)

// Medium's classic editor, as san_medium clicks it, and the element markWrite
// marks for a click.
const (
	selEditorTitle = `.postArticle-content h3.graf--title`
	selEditorBody  = `.postArticle-content .section-inner > p.graf--p`
	selMarked      = `[data-san-target]`
)

// markWrite marks the shown Write link of Medium's header, false when the
// page has none.
const markWrite = `(() => {
	document.querySelectorAll('[data-san-target]').forEach(e => e.removeAttribute('data-san-target'));
	const a = [...document.querySelectorAll('a[href*="/new-story"]')]
		.find(a => !a.href.includes('/m/signin') && a.getBoundingClientRect().width > 0);
	if (!a) return false;
	a.scrollIntoView({block: 'center'});
	a.setAttribute('data-san-target', '');
	return true;
})()`

// draftSaved is true when the editor has nothing unsaved and its status reads
// Saved.
const draftSaved = `(() => {
	let dirty = false;
	try {
		dirty = window.onbeforeunload ? window.onbeforeunload(new Event('beforeunload', {cancelable: true})) !== undefined : false;
	} catch (e) {
		dirty = true;
	}
	const msg = document.querySelector('.js-metabarMessage');
	return !dirty && !!msg && /Saved/.test(msg.textContent);
})()`

// WriteDraft makes one Medium draft from the page in the tab, which shows
// Medium signed in (WaitMediumLogined). It clicks Write, types the title and
// a line of text as a person does, and returns once Medium has saved them.
func WriteDraft(ctx context.Context) error {
	slog.Info("opening a new story with write.")
	var marked bool
	if !evaluate(ctx, markWrite, &marked) || !marked {
		return errors.New("medium: the page has no Write link")
	}
	if err := clickStill(ctx, selMarked); err != nil {
		return err
	}
	editor := fmt.Sprintf("!!document.querySelector(%q)", selEditorTitle)
	if err := until(ctx, "the editor", 2*time.Minute, func() bool {
		var in bool
		return evaluate(ctx, editor, &in) && in
	}); err != nil {
		return err
	}

	slog.Info("typing the title.")
	if err := clickStill(ctx, selEditorTitle); err != nil {
		return err
	}
	if err := typeText(ctx, draftTitle); err != nil {
		return err
	}

	slog.Info("typing the text.")
	if err := clickStill(ctx, selEditorBody); err != nil {
		return err
	}
	if err := typeText(ctx, draftText); err != nil {
		return err
	}

	// Saved has to hold for a few seconds: Medium saves in steps.
	slog.Info("waiting for medium to save.")
	var since time.Time
	if err := until(ctx, "Medium to save", 2*time.Minute, func() bool {
		var saved bool
		if !evaluate(ctx, draftSaved, &saved) || !saved {
			since = time.Time{}
			return false
		}
		if since.IsZero() {
			since = time.Now()
		}
		return time.Since(since) >= 3*time.Second
	}); err != nil {
		return err
	}

	var url string
	if err := chromedp.Run(ctx, chromedp.Location(&url)); err != nil {
		return err
	}
	slog.Info("draft saved.", "url", url)
	return nil
}

// clickStill clicks the first match of sel with the mouse, after a person's
// pause, once it is shown and has stopped moving: Medium's editor and menus
// slide into place, and a click aimed where a button was misses it.
func clickStill(ctx context.Context, sel string) error {
	if err := pause(ctx, 600*time.Millisecond, 1500*time.Millisecond); err != nil {
		return err
	}
	rect := fmt.Sprintf(`(() => {
		const e = document.querySelector(%q);
		if (!e) return '';
		const r = e.getBoundingClientRect();
		return r.width < 4 || r.height < 4 ? '' : [r.left, r.top, r.width, r.height].map(Math.round).join(',');
	})()`, sel)
	var last string
	if err := until(ctx, "a still "+sel, 15*time.Second, func() bool {
		var r string
		if !evaluate(ctx, rect, &r) || r == "" {
			last = ""
			return false
		}
		still := r == last
		last = r
		return still
	}); err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := chromedp.Run(cctx, chromedp.Click(sel, chromedp.ByQuery, chromedp.NodeVisible)); err != nil {
		return fmt.Errorf("medium: click %s: %w", sel, err)
	}
	return nil
}

// typeText types s key by key, at a person's pace.
func typeText(ctx context.Context, s string) error {
	for _, r := range s {
		if err := chromedp.Run(ctx, chromedp.KeyEvent(string(r))); err != nil {
			return err
		}
		if err := pause(ctx, 60*time.Millisecond, 180*time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}

// evaluate runs js in the page into out. False when it fails: a page still
// loading has no script context yet, and a frozen one never answers.
func evaluate(ctx context.Context, js string, out any) bool {
	ectx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return chromedp.Run(ectx, chromedp.Evaluate(js, out)) == nil
}

// until polls ok until it is true, for at most timeout.
func until(ctx context.Context, what string, timeout time.Duration, ok func() bool) error {
	deadline := time.Now().Add(timeout)
	for !ok() {
		if time.Now().After(deadline) {
			return fmt.Errorf("medium: waited %s for %s", timeout, what)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return nil
}

// pause waits a random time between lo and hi.
func pause(ctx context.Context, lo, hi time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(lo + rand.N(hi-lo)):
		return nil
	}
}
