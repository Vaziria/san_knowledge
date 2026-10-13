// Helpers installed in LinkedIn's pages (the feed, and the article editor
// with its dialogs) as window.__sanLinkedin. Every selector that tracks
// LinkedIn's markup is in this file or in editor.go.
window.__sanLinkedin = (() => {
  // The article editor is LinkedIn's Ember app. Loaded by its own URL it is
  // the page itself; reached by a click from the feed, it runs in a
  // same-origin iframe (/preload/?_bprMode=vanilla) that fills the page. The
  // feed loads that iframe ahead of time, full size but transparent and
  // behind the page (opacity 0, z-index -1), so it counts only once it is
  // shown. The helpers work on whichever shows the page, and report points
  // in the page's coordinates, for the mouse.
  const app = () => {
    const f = [...document.querySelectorAll('iframe')].find(f => /\/preload\//.test(f.src));
    if (f) {
      const r = f.getBoundingClientRect();
      const cs = getComputedStyle(f);
      let d = null;
      try { d = f.contentDocument; } catch (e) { d = null; }
      if (d && d.body && r.width > 0 && r.height > 0 && +cs.opacity > 0 && cs.visibility === 'visible' && !(+cs.zIndex < 0)) {
        return {d, w: f.contentWindow, x: r.left, y: r.top};
      }
    }
    return {d: document, w: window, x: 0, y: 0};
  };
  const $ = (sel) => app().d.querySelector(sel);
  const $$ = (sel, root) => [...(root || app().d).querySelectorAll(sel)];
  const shown = (e) => !!e && e.getBoundingClientRect().width > 0 && e.getBoundingClientRect().height > 0;
  const text = (e) => (e && (e.innerText || e.textContent) || '').replace(/\s+/g, ' ').trim();

  const TITLE = '#article-editor-headline__textarea';
  const BODY = '.ProseMirror[data-test-article-editor-content-textbox], .ProseMirror';
  const MEDIA = '.article-editor__media-editor-modal';
  const MANAGE = '.article-editor-article-management-modal';
  const SHARE = '.share-box-v2__modal';
  const body = () => $(BODY);
  // The article's top-level blocks: paragraphs, headings, lists, figures.
  const blocks = () => body() ? [...body().children] : [];
  const figures = () => body() ? $$('figure.article-editor-figure-image', body()) : [];

  // Elements the Go side clicks next are marked, and found again by this
  // attribute; only one is marked at a time. ProseMirror undoes attributes
  // set on its own nodes, so text in the article is never marked: its
  // points are asked for by what it holds instead.
  const MARK = 'data-san-linkedin';
  const mark = (el) => {
    $$('[' + MARK + ']').forEach(e => e.removeAttribute(MARK));
    if (!el) return false;
    el.setAttribute(MARK, 'target');
    return true;
  };

  // The point to click on e, in the page's coordinates, once it is in view:
  // its middle, or "end" just inside its right edge. key changes while e
  // moves.
  const pointOf = (e, where) => {
    if (!shown(e)) return null;
    const a = app();
    let r = e.getBoundingClientRect();
    const vh = a.w.innerHeight;
    if (r.top < 70 || r.bottom > vh - 10) {
      e.scrollIntoView({block: 'center'});
      r = e.getBoundingClientRect();
    }
    const x = where === 'end' ? r.right - 3 : r.left + r.width / 2;
    return {x: a.x + x, y: a.y + r.top + Math.min(r.height / 2, 14), key: [r.left, r.top, r.width, r.height].map(Math.round).join(',')};
  };

  const modal = (sel) => $$(sel || '.artdeco-modal').find(shown) || null;
  // A button that can be pressed: shown, and not disabled, as the media
  // dialog's buttons are while an image is still being read.
  const live = (b) => shown(b) && !b.disabled && b.getAttribute('aria-disabled') !== 'true';
  const button = (root, label) => $$('button, [role=button]', root).find(b => live(b) && (text(b) === label || b.getAttribute('aria-label') === label)) || null;
  const iconButton = (root, icon) => $$('button', root).find(b => live(b) && b.querySelector('[data-test-icon="' + icon + '"]')) || null;

  return {
    point(sel, where) {
      return pointOf($$(sel).find(shown), where);
    },
    // The point of the top-level block i of the article (negative counts
    // from the end).
    blockPoint(i, where) {
      const bs = blocks();
      return pointOf(bs[i < 0 ? bs.length + i : i], where);
    },

    // --- where the tab is

    // The feed's "Write article" link, shown to a signed-in member only.
    writeLink() {
      return $$('a[href*="/article/new"]').find(shown) || null;
    },
    markWrite() {
      return mark(this.writeLink());
    },
    feedReady() {
      return !!this.writeLink();
    },
    editorReady() {
      return !!$(TITLE) && !!body();
    },
    state() {
      const path = location.pathname;
      return {
        editor: this.editorReady(),
        signin: /^\/(login|uas\/login|authwall|signup|checkpoint\/lg\/login)/.test(path) ||
          !!document.querySelector('form.login__form, #session_password'),
        challenge: /^\/checkpoint\//.test(path) && !/\/lg\/login/.test(path),
        url: location.href,
      };
    },

    // --- the editor

    titleText() {
      const t = $(TITLE);
      return t ? t.value : '';
    },
    // Hand HTML to ProseMirror's paste handler, as a paste from the
    // clipboard would. LinkedIn turns it into its blocks and saves them.
    paste(html) {
      const a = app();
      const pm = body();
      const s = a.d.getSelection();
      if (!pm || !s.anchorNode || !pm.contains(s.anchorNode)) return false;
      const dt = new a.w.DataTransfer();
      dt.setData('text/html', html);
      dt.setData('text/plain', html.replace(/<[^>]+>/g, ' '));
      const ev = new a.w.ClipboardEvent('paste', {clipboardData: dt, bubbles: true, cancelable: true});
      pm.dispatchEvent(ev);
      return ev.defaultPrevented;
    },
    // Where the paragraph whose whole text is token is among the blocks, or
    // -1.
    find(token) {
      return blocks().findIndex(b => b.tagName === 'P' && b.textContent.trim() === token);
    },
    // Select all text of the paragraph token, for the Backspace that empties
    // it. True only when the selection is that text and nothing else.
    select(token) {
      const a = app();
      const p = blocks().find(b => b.tagName === 'P' && b.textContent.trim() === token);
      if (!p) return false;
      const r = a.d.createRange();
      r.selectNodeContents(p);
      const s = a.d.getSelection();
      s.removeAllRanges();
      s.addRange(r);
      return s.toString().trim() === token;
    },
    // The index of the block that has the caret, when it is an empty
    // paragraph; else -1.
    caretInEmpty() {
      const a = app();
      const s = a.d.getSelection();
      if (!s.anchorNode) return -1;
      const bs = blocks();
      const i = bs.findIndex(b => b.contains(s.anchorNode));
      return i >= 0 && bs[i].tagName === 'P' && bs[i].textContent === '' ? i : -1;
    },
    markIcon(icon) {
      return mark(iconButton(app().d, icon));
    },

    // The cover: "Upload from computer" while there is none, then the image.
    markCoverUpload() {
      const c = $('.article-editor-cover-media');
      return mark(c && button(c, 'Upload from computer'));
    },
    cover() {
      const i = $('.article-editor-cover-media img.article-editor-cover-image-v2__image');
      return i ? i.src : '';
    },

    // The media dialog that opens for an image: what it shows, its buttons.
    media() {
      const m = modal(MEDIA);
      if (!m) return null;
      return {
        title: text(m.querySelector('h1, h2')),
        alt: !!iconButton(m, 'alt-text-medium'),
        altBox: shown(m.querySelector('.media-editor-tools-alt-text__input')),
        buttons: $$('button', m).filter(shown).map(b => text(b) || b.getAttribute('aria-label')),
      };
    },
    markMediaButton(label) {
      return mark(button(modal(MEDIA), label));
    },
    markMediaIcon(icon) {
      return mark(iconButton(modal(MEDIA), icon));
    },
    altFocused() {
      const a = app().d.activeElement;
      return !!a && a.matches('.media-editor-tools-alt-text__input');
    },
    altText() {
      const t = $(MEDIA + ' .media-editor-tools-alt-text__input');
      return t ? t.value : '';
    },

    // The i-th image of the article, in document order.
    figure(i) {
      const f = figures()[i];
      if (!f) return null;
      const img = f.querySelector('img.article-editor-inline-image__image');
      const cap = f.querySelector('textarea.article-editor-figure-caption');
      const bs = blocks();
      const at = bs.indexOf(f);
      const next = bs[at + 1];
      return {
        block: at,
        src: img ? img.src : '',
        uploaded: !!img && /^https:/.test(img.src),
        alt: img ? img.alt : '',
        caption: cap ? cap.value : '',
        captionFocused: !!cap && app().d.activeElement === cap,
        // An empty paragraph the editor leaves after an inserted image,
        // when more of the article follows it.
        emptyAfter: !!next && next.tagName === 'P' && next.textContent === '' && at + 2 < bs.length,
      };
    },
    captionPoint(i) {
      const f = figures()[i];
      return pointOf(f && f.querySelector('textarea.article-editor-figure-caption'));
    },
    // Select what the caption box of image i holds, so that the text put in
    // next replaces it. False unless that box has the caret.
    selectCaption(i) {
      const f = figures()[i];
      const cap = f && f.querySelector('textarea.article-editor-figure-caption');
      if (!cap || app().d.activeElement !== cap) return false;
      cap.select();
      return true;
    },

    // Whether LinkedIn has saved the article: the status in the corner,
    // "Draft - saved" once it has.
    saveState() {
      const m = $('.article-editor-article-status__message');
      const s = text(m);
      return {
        status: s,
        saved: /saved/i.test(s) && !/saving|not saved|unsaved|unable|fail|error/i.test(s),
        error: /not saved|unable|fail|error/i.test(s) ? s : '',
      };
    },

    // --- the share box that publishes or schedules the article (the
    // editor's Next). Its main button reads "Publish" until a time is set,
    // then "Schedule"; only a button that reads exactly "Schedule" is ever
    // marked, and never "Publish" or the Scheduled tab's "Post now".

    markNext() {
      return mark($$('button.article-editor-nav__publish').find(live));
    },
    // The toast LinkedIn shows for a Next pressed while the article is
    // being saved: "Sorry, your article is still saving."
    stillSaving() {
      const all = [document].concat(app().d !== document ? [app().d] : []);
      return all.some(d => [...d.querySelectorAll('.artdeco-toast-item, [role=alert], [data-test-artdeco-toast-item-type]')]
        .some(t => shown(t) && /still saving/i.test(text(t))));
    },
    share() {
      const m = modal(SHARE);
      if (!m) return null;
      const box = m.querySelector('.ql-editor');
      // "Posting Fri, Nov 20 at 9:00 AM", next to its Edit button, read
      // from the box's whole text.
      const posting = text(m).match(/Posting (\w{3}, \w{3} \d{1,2} at \d{1,2}:\d{2}\s?[AP]M)/);
      return {
        text: box ? box.innerText.replace(/\n+$/, '') : null,
        textFocused: !!box && (app().d.activeElement === box || box.contains(app().d.activeElement)),
        posting: posting ? 'Posting ' + posting[1] : '',
        all: text(m).slice(0, 200),
        primary: text(m.querySelector('.share-actions__primary-action')),
        clock: !!iconButton(m, 'clock-medium'),
      };
    },
    textPoint() {
      const m = modal(SHARE);
      return pointOf(m && m.querySelector('.ql-editor'), 'end');
    },
    markClock() {
      const m = modal(SHARE);
      return mark(m && $$('button[aria-label="Schedule post"]', m).find(live));
    },
    markSchedule() {
      const m = modal(SHARE);
      const b = m && m.querySelector('.share-actions__primary-action');
      return mark(b && live(b) && text(b) === 'Schedule' ? b : null);
    },
    // The share box's schedule view: the time it states ("Fri, Nov 20, 9:00
    // AM Western Indonesia Time, based on your location"), and its date and
    // time boxes.
    when() {
      const m = modal(SHARE);
      const d = m && m.querySelector('input[aria-label="Date"]');
      if (!d) return null;
      const t = m.querySelector('input[aria-label="Time"]');
      const head = $$('*', m).find(e => e.children.length === 0 && /based on your location/i.test(text(e)));
      return {stated: head ? text(head) : '', date: d.value, time: t ? t.value : ''};
    },
    markDateBox() {
      const m = modal(SHARE);
      return mark(m && m.querySelector('input[aria-label="Date"]'));
    },
    markTimeBox() {
      const m = modal(SHARE);
      return mark(m && m.querySelector('input[aria-label="Time"]'));
    },
    // The date picker: the month it shows ("November 2026"), null while
    // closed. Its day buttons are labelled with their whole date ("Friday,
    // November 20, 2026."), its arrows "Previous month" and "Next month".
    calendarMonth() {
      const c = $$('.artdeco-calendar').find(shown);
      return c ? text(c.querySelector('.artdeco-calendar__month')) : null;
    },
    markCalendar(label) {
      const c = $$('.artdeco-calendar').find(shown);
      return mark(c && $$('button', c).find(b => live(b) && b.getAttribute('aria-label') === label));
    },
    // An option of the time list, at 15-minute steps ("9:00 AM"), scrolled
    // into the list's view first.
    timeOptions() {
      return $$('.timepicker__typeahead-result-list [role=option]').filter(shown).length;
    },
    markTimeOption(label) {
      const o = $$('.timepicker__typeahead-result-list [role=option]').find(o => text(o) === label);
      if (o) o.scrollIntoView({block: 'nearest'});
      return mark(o);
    },
    markShareButton(label) {
      const m = modal(SHARE);
      return mark(m && $$('button', m).find(b => live(b) && text(b) === label && label !== 'Publish'));
    },
    // The browser's own name and offset for its zone at a local time, to
    // compare with the zone the schedule view states.
    zone(y, mo, d, h, mi) {
      const t = new Date(y, mo - 1, d, h, mi);
      const p = Intl.DateTimeFormat('en-US', {timeZoneName: 'long'}).formatToParts(t).find(p => p.type === 'timeZoneName');
      return {name: p ? p.value : '', offset: -t.getTimezoneOffset() * 60};
    },
    // LinkedIn's word that the article is scheduled.
    scheduledNotice() {
      return !!$$('.artdeco-modal').find(m => shown(m) && /has been scheduled/i.test(text(m)));
    },

    // The body as text, a block per paragraph; an image stands as its
    // caption, without the buttons drawn over it.
    bodyText() {
      return blocks().map(b => {
        if (b.matches('figure')) {
          const cap = b.querySelector('textarea.article-editor-figure-caption');
          return cap ? cap.value : '';
        }
        if (b.matches('.article-editor-horizontal-rule__container')) return '---';
        return b.innerText.trim();
      }).filter(Boolean).join('\n\n');
    },

    snapshot() {
      const tokens = blocks().filter(b => /^san-linkedin-image-\d+$/.test(b.textContent.trim())).length;
      return {
        status: this.saveState().status,
        title: this.titleText(),
        cover: this.cover(),
        tokens,
        figures: figures().map((_, i) => this.figure(i)),
        text: this.bodyText(),
        url: location.href,
      };
    },

    // --- the Manage menu and its "Articles" dialog (Drafts, Scheduled,
    // Published)

    markManage() {
      return mark($$('button[aria-label="Manage menu"]').find(shown));
    },
    markDropdownItem(label) {
      return mark($$('.artdeco-dropdown__item').find(e => shown(e) && text(e) === label));
    },
    // The dialog's open tab ("Drafts"), and its rows as far as they are
    // loaded; null while the dialog is not shown.
    articles() {
      const m = modal(MANAGE);
      if (!m) return null;
      const tab = m.querySelector('[role=tab][aria-selected="true"]');
      const loading = $$('.artdeco-loader', m).some(shown);
      // The list shows 10 rows, then a "Show more results" button
      // (2026-10-14).
      const more = !!this.moreButton();
      const rows = $$('ul.article-editor-article-management-modal__list > li', m).map(li => {
        const a = li.querySelector('a[href*="/article/edit/"], a[href*="/pulse/"], h2 a');
        const m2 = a && a.getAttribute('href').match(/\/article\/edit\/(\d+)/);
        return {
          id: m2 ? m2[1] : '',
          title: text(li.querySelector('h2')),
          href: a ? a.href : '',
          date: text(li.querySelector('.article-editor-article-management-list-item__date')),
        };
      });
      const empty = !rows.length && /No .* yet/.test(text(m));
      return {tab: text(tab), rows, loading, more, empty};
    },
    moreButton() {
      const m = modal(MANAGE);
      return m ? $$('button', m).find(b => live(b) && text(b) === 'Show more results') || null : null;
    },
    markMore() {
      return mark(this.moreButton());
    },
    markTab(label) {
      const m = modal(MANAGE);
      return mark(m && $$('[role=tab]', m).find(t => shown(t) && text(t) === label));
    },
    // Scroll the dialog's list to its end, for more rows.
    scrollArticles() {
      const m = modal(MANAGE);
      const c = m && m.querySelector('.artdeco-modal__content');
      if (c) c.scrollTop = c.scrollHeight;
      return !!c;
    },
    rowOf(id) {
      const m = modal(MANAGE);
      return m ? $$('ul.article-editor-article-management-modal__list > li', m).find(li => !!li.querySelector('a[href*="/article/edit/' + id + '/"]')) || null : null;
    },
    markRowLink(id) {
      const li = this.rowOf(id);
      return mark(li && $$('h2 a', li).find(shown));
    },
    markRowMenu(id) {
      const li = this.rowOf(id);
      return mark(li && $$('.article-editor-article-management-list-item__actions-trigger', li).find(shown));
    },
    markRowMenuItem(id, label) {
      const li = this.rowOf(id);
      return mark(li && $$('.artdeco-dropdown__item', li).find(e => shown(e) && text(e) === label));
    },
    // Where draft id's row is in the list, from 0, or -1.
    rowIndex(id) {
      const m = modal(MANAGE);
      return m ? $$('ul.article-editor-article-management-modal__list > li', m).indexOf(this.rowOf(id)) : -1;
    },
    // LinkedIn asks in the row itself, over its content: "Delete draft? You
    // can't undo this action." with "Not now" and "Delete", and, for a
    // scheduled article, "To edit this scheduled article, move it to
    // drafts." with "Not now" and "Move to drafts" (2026-10-13). Marks the
    // button labelled label of row i, when the row's text matches re.
    markRowAsk(i, re, label) {
      const m = modal(MANAGE);
      const li = m && $$('ul.article-editor-article-management-modal__list > li', m)[i];
      if (!li || !new RegExp(re, 'i').test(text(li))) return false;
      return mark(button(li, label));
    },
    // The title of row i, a link for a draft and a button for a scheduled
    // article, when it reads title.
    markRowTitle(i, title) {
      const m = modal(MANAGE);
      const li = m && $$('ul.article-editor-article-management-modal__list > li', m)[i];
      const t = li && $$('h2 a, h2 button', li).find(live);
      return mark(t && text(t) === title.replace(/\s+/g, ' ').trim() ? t : null);
    },
    markButton(label) {
      return mark($$('button').find(b => shown(b) && text(b) === label));
    },
    markedPoint(where) {
      return pointOf($('[' + MARK + '="target"]'), where);
    },
    // Mark with the helper fn, then give the marked element's point: found
    // afresh each time, since Ember may draw a button again after it was
    // marked.
    pointBy(fn, ...args) {
      return this[fn](...args) ? this.markedPoint() : null;
    },
    // The article's blocks and images in a line, for SAN_LINKEDIN_DEBUG.
    debug() {
      const a = app();
      const s = a.d.getSelection();
      const act = a.d.activeElement;
      const bs = blocks().map(b => b.tagName + (b.textContent === '' ? '()' : '') + (s.anchorNode && b.contains(s.anchorNode) ? '*' : '') + (act && b.contains(act) ? '!' : '')).join(' ');
      const fs = figures().map(f => '[' + (f.getAttribute('data-id') || '').slice(0, 8) + ' alt=' + JSON.stringify(((f.querySelector('img') || {}).alt || '').slice(0, 12)) + ' cap=' + JSON.stringify((f.querySelector('textarea') || {}).value || '') + ']').join(' ');
      return (a.d === document ? 'page' : 'frame') + ' | ' + bs + ' | ' + fs;
    },
    // The dialogs shown in the page and in the editor's frame, for an error
    // message.
    modals() {
      const list = (d, where) => [...d.querySelectorAll('.artdeco-modal, [role=dialog]')].filter(shown)
        .map(m => where + ': ' + String(m.className).trim().split(/\s+/).slice(0, 3).join('.') + ' "' + text(m).slice(0, 80) + '"');
      const a = app();
      return list(document, 'page').concat(a.d !== document ? list(a.d, 'frame') : []).join('; ') || 'none';
    },
    // What is drawn at the page point x, y, for an error message.
    hit(x, y) {
      const a = app();
      const e = a.d.elementFromPoint(x - a.x, y - a.y);
      if (!e) return 'nothing';
      return e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + (e.className && typeof e.className === 'string' ? '.' + e.className.trim().split(/\s+/).slice(0, 3).join('.') : '');
    },
  };
})();
true;
