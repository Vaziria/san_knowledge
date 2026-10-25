// What san_remote knows of LinkedIn's pages: the feed, the article editor
// (Ember, with ProseMirror for the body) with its media dialog, its share box
// (Next) and its Articles dialog (Manage). The selectors are
// golang/packages/san_linkedin's (editor.js), found with real mouse and key
// input on 2026-10-13 and 14; here they run in the extension's isolated
// world, where the page cannot see them, and input reaches the page as on
// Medium (medium.js):
//   - text boxes (the title, alt text, caption): execCommand's insertText on
//     the focused box, whose input event Ember reads
//   - the body: one paste event with the HTML, to ProseMirror's own handler
//   - Backspace: a keydown ProseMirror handles itself
//   - a file: the button's own file input, given the file after the press
//   - buttons: .click(); the schedule view's calendar and time list only
//     take a whole mouse press (pointer and mouse events)
//
// The editor reached from the feed runs in a same-origin iframe
// (/preload/?_bprMode=vanilla) that fills the page; the feed loads it ahead
// of time, transparent and behind the page, so it counts only once shown.
// Loaded by its own URL, the editor is the page itself. app() is whichever
// shows the page.
globalThis.__sanLinkedIn = (() => {
  const app = () => {
    const f = [...document.querySelectorAll('iframe')].find((f) => /\/preload\//.test(f.src));
    if (f) {
      const r = f.getBoundingClientRect();
      const cs = getComputedStyle(f);
      let d = null;
      try {
        d = f.contentDocument;
      } catch (e) {
        d = null;
      }
      if (d && d.body && r.width > 0 && r.height > 0 && +cs.opacity > 0 && cs.visibility === 'visible' && !(+cs.zIndex < 0)) {
        return { d, w: f.contentWindow ?? window };
      }
    }
    return { d: document, w: window };
  };
  const $ = (sel) => app().d.querySelector(sel);
  const $$ = (sel, root) => [...(root ?? app().d).querySelectorAll(sel)];
  const shown = (e) => !!e && e.getBoundingClientRect().width > 0 && e.getBoundingClientRect().height > 0;
  const text = (e) => ((e && (e.innerText || e.textContent)) || '').replace(/\s+/g, ' ').trim();
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  const TITLE = '#article-editor-headline__textarea';
  const BODY = '.ProseMirror[data-test-article-editor-content-textbox], .ProseMirror';
  const MEDIA = '.article-editor__media-editor-modal';
  const MANAGE = '.article-editor-article-management-modal';
  const SHARE = '.share-box-v2__modal';
  const body = () => $(BODY);
  // The article's top-level blocks: paragraphs, headings, lists, figures.
  const blocks = () => (body() ? [...body().children] : []);
  const figures = () => (body() ? $$('figure.article-editor-figure-image', body()) : []);
  const modal = (sel) => $$(sel ?? '.artdeco-modal').find(shown) ?? null;
  // A button that can be pressed: shown, and not disabled, as the media
  // dialog's buttons are while an image is still being read.
  const live = (b) => shown(b) && !b.disabled && b.getAttribute('aria-disabled') !== 'true';
  const button = (root, label) => $$('button, [role=button]', root).find((b) => live(b) && (text(b) === label || b.getAttribute('aria-label') === label)) ?? null;
  const iconButton = (root, icon) => $$('button', root).find((b) => live(b) && b.querySelector('[data-test-icon="' + icon + '"]')) ?? null;
  const press = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.focus?.();
    e.click();
    return true;
  };
  // mouse presses e as a mouse does, for the schedule view's calendar and
  // time list, which ignore a bare click (2026-10-25).
  const mouse = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    const r = e.getBoundingClientRect();
    const o = { bubbles: true, cancelable: true, clientX: r.x + r.width / 2, clientY: r.y + r.height / 2, button: 0, view: e.ownerDocument.defaultView ?? window };
    e.dispatchEvent(new PointerEvent('pointerdown', { ...o, pointerType: 'mouse', isPrimary: true }));
    e.dispatchEvent(new MouseEvent('mousedown', { ...o, buttons: 1 }));
    e.dispatchEvent(new PointerEvent('pointerup', { ...o, pointerType: 'mouse', isPrimary: true }));
    e.dispatchEvent(new MouseEvent('mouseup', o));
    e.dispatchEvent(new MouseEvent('click', o));
    return true;
  };
  const select = (node, collapse) => {
    const d = app().d;
    const r = d.createRange();
    r.selectNodeContents(node);
    if (collapse !== undefined) r.collapse(collapse);
    const s = d.getSelection();
    s.removeAllRanges();
    s.addRange(r);
    return s;
  };
  const keydown = (el, key, code) => el.dispatchEvent(new KeyboardEvent('keydown', { key, code: key, keyCode: code, which: code, bubbles: true, cancelable: true }));
  // typeInto puts text into a text box (a textarea or input), replacing what
  // it holds, as one insertion.
  const typeInto = (box, t) => {
    if (!box) return false;
    box.focus();
    box.select?.();
    if (!app().d.execCommand('insertText', false, t)) {
      const proto = box.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
      Object.getOwnPropertyDescriptor(proto, 'value').set.call(box, t);
      box.dispatchEvent(new Event('input', { bubbles: true }));
    }
    return true;
  };
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  const rows = () => {
    const m = modal(MANAGE);
    return m ? $$('ul.article-editor-article-management-modal__list > li', m) : [];
  };
  const rowOf = (id) => rows().find((li) => !!li.querySelector('a[href*="/article/edit/' + id + '/"]')) ?? null;

  return {
    // state is what the page shows, for the waits.
    state() {
      const path = location.pathname;
      return {
        url: location.href,
        lang: document.documentElement.lang,
        feed: !!$$('a[href*="/article/new"]').find(shown),
        editor: !!$(TITLE) && !!body(),
        signin: /^\/(login|uas\/login|authwall|signup|checkpoint\/lg\/login)/.test(path) ||
          !!document.querySelector('form.login__form, #session_password') ||
          (path === '/' && !!document.querySelector('a[href*="/login"], a[data-tracking-control-name*="sign-in"]') && !document.querySelector('.global-nav__me')),
        challenge: /^\/checkpoint\//.test(path) && !/\/lg\/login/.test(path),
        articles: !!modal(MANAGE),
        share: !!modal(SHARE),
      };
    },
    // account is the member's name: the first line of the feed's profile
    // card (the first profile link with text), or the old nav's "Me" photo.
    account() {
      const card = $$('a[href*="/in/"]').find((a) => a.innerText.trim());
      if (card) return card.innerText.trim().split('\n')[0].trim();
      const img = document.querySelector('img.global-nav__me-photo, .global-nav__me img[alt]');
      return img?.getAttribute('alt')?.trim() ?? '';
    },
    // pressWrite presses the feed's "Write article", shown to a member only.
    pressWrite: () => press($$('a[href*="/article/new"]').find(shown)),

    // ---- the editor ----
    typeTitle: (t) => typeInto($(TITLE), t),
    titleText: () => $(TITLE)?.value ?? '',
    // caretInBody puts the caret in the body's first block.
    caretInBody() {
      const b = blocks()[0];
      if (!b) return false;
      body().focus();
      select(b, true);
      return true;
    },
    // paste hands HTML to ProseMirror's paste handler, as a paste from the
    // clipboard would; LinkedIn turns it into its blocks and saves them.
    paste(html) {
      const { d, w } = app();
      const pm = body();
      const s = d.getSelection();
      if (!pm || !s.anchorNode || !pm.contains(s.anchorNode)) return false;
      const DT = w.DataTransfer ?? DataTransfer;
      const CE = w.ClipboardEvent ?? ClipboardEvent;
      const dt = new DT();
      dt.setData('text/html', html);
      dt.setData('text/plain', html.replace(/<[^>]+>/g, ' '));
      const ev = new CE('paste', { clipboardData: dt, bubbles: true, cancelable: true });
      pm.dispatchEvent(ev);
      return ev.defaultPrevented;
    },
    // find is where the paragraph whose whole text is token is among the
    // blocks, or -1.
    find: (token) => blocks().findIndex((b) => b.tagName === 'P' && b.textContent.trim() === token),
    // emptyBlock selects the paragraph token and presses Backspace, which
    // ProseMirror handles, leaving the caret in the empty paragraph.
    emptyBlock(token) {
      const p = blocks().find((b) => b.tagName === 'P' && b.textContent.trim() === token);
      if (!p) return false;
      p.scrollIntoView({ block: 'center' });
      body().focus();
      if (select(p).toString().trim() !== token) return false;
      if (keydown(body(), 'Backspace', 8)) app().d.execCommand('delete');
      return true;
    },
    // caretInEmpty is the index of the block with the caret, when it is an
    // empty paragraph; else -1.
    caretInEmpty() {
      const s = app().d.getSelection();
      if (!s.anchorNode) return -1;
      const bs = blocks();
      const i = bs.findIndex((b) => b.contains(s.anchorNode));
      return i >= 0 && bs[i].tagName === 'P' && bs[i].textContent === '' ? i : -1;
    },
    // caretIn puts the caret in block i, for the Backspace that removes the
    // empty paragraph after an image.
    caretIn(i) {
      const b = blocks()[i];
      if (!b) return false;
      body().focus();
      select(b, true);
      return true;
    },
    backspace: () => !!body() && (keydown(body(), 'Backspace', 8), true),
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // chooseFile presses the button what names ("cover": the cover's
    // "Upload from computer"; "image": the toolbar's "Add image or video"),
    // which clicks a file input, and gives that input the staged file. A
    // click a script makes opens no file dialog, so the input waits for its
    // change. It returns "" or what went wrong.
    async chooseFile(id, name, type, what) {
      const { d } = app();
      const parts = staged.get(id);
      if (!parts) return 'nothing staged';
      const before = new Set(d.querySelectorAll('input[type=file]'));
      let clicked = null;
      const grab = (e) => {
        const t = e.target;
        if (t && t.tagName === 'INPUT' && t.type === 'file') clicked = t;
      };
      d.addEventListener('click', grab, true);
      try {
        const c = $('.article-editor-cover-media');
        const b = what === 'cover' ? c && button(c, 'Upload from computer') : iconButton(d, 'image-medium');
        if (!b) return 'no ' + what + ' button';
        press(b);
        let input = null;
        for (let i = 0; i < 30 && !input; i++) {
          input = clicked ?? [...d.querySelectorAll('input[type=file]')].find((x) => !before.has(x)) ?? null;
          if (!input) await sleep(100);
        }
        if (!input) return 'the ' + what + ' button clicked no file input';
        staged.delete(id);
        const list = new DataTransfer();
        list.items.add(new File(parts, name, { type, lastModified: Date.now() }));
        input.files = list.files;
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
        return '';
      } finally {
        d.removeEventListener('click', grab, true);
      }
    },
    cover: () => $('.article-editor-cover-media img.article-editor-cover-image-v2__image')?.src ?? '',
    // media is the media dialog that opens for an image.
    media() {
      const m = modal(MEDIA);
      if (!m) return null;
      return {
        title: text(m.querySelector('h1, h2')),
        alt: !!iconButton(m, 'alt-text-medium'),
        altBox: shown(m.querySelector('.media-editor-tools-alt-text__input')),
        buttons: $$('button', m).filter(shown).map((b) => text(b) || b.getAttribute('aria-label')),
      };
    },
    pressMediaButton: (label) => press(button(modal(MEDIA), label)),
    pressMediaIcon: (icon) => press(iconButton(modal(MEDIA), icon)),
    typeAlt: (t) => typeInto($(MEDIA + ' .media-editor-tools-alt-text__input'), t),
    altText: () => $(MEDIA + ' .media-editor-tools-alt-text__input')?.value ?? '',
    // figure is the i-th image of the article, in document order.
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
        // An empty paragraph the editor leaves after an inserted image, when
        // more of the article follows it.
        emptyAfter: !!next && next.tagName === 'P' && next.textContent === '' && at + 2 < bs.length,
      };
    },
    typeCaption: (i, t) => typeInto(figures()[i]?.querySelector('textarea.article-editor-figure-caption'), t),
    // saveState: the status in the corner, "Draft - saved" once LinkedIn has
    // saved, and just "Draft" on a draft opened with nothing to save
    // (2026-10-25).
    saveState() {
      const s = text($('.article-editor-article-status__message'));
      return {
        status: s,
        saved: s === 'Draft' || (/saved/i.test(s) && !/saving|not saved|unsaved|unable|fail|error/i.test(s)),
        error: /not saved|unable|fail|error/i.test(s) ? s : '',
      };
    },
    // bodyText is the body as text, a block per paragraph; an image stands
    // as its caption.
    bodyText() {
      return blocks().map((b) => {
        if (b.matches('figure')) return b.querySelector('textarea.article-editor-figure-caption')?.value ?? '';
        if (b.matches('.article-editor-horizontal-rule__container')) return '---';
        return b.innerText.trim();
      }).filter(Boolean).join('\n\n');
    },
    snapshot() {
      return {
        status: this.saveState().status,
        title: this.titleText(),
        cover: this.cover(),
        tokens: blocks().filter((b) => /^san-linkedin-image-\d+$/.test(b.textContent.trim())).length,
        figures: figures().map((_, i) => this.figure(i)),
        text: this.bodyText(),
        url: location.href,
      };
    },

    // ---- the share box (the editor's Next). Its main button reads
    // "Publish" until a time is set, then "Schedule"; only a button that
    // reads exactly "Schedule" is ever pressed, never "Publish" or the
    // Scheduled tab's "Post now".
    pressNext: () => press($$('button.article-editor-nav__publish').find(live)),
    // stillSaving: the toast for a Next pressed while the article is being
    // saved, "Sorry, your article is still saving."
    stillSaving() {
      const docs = [document].concat(app().d !== document ? [app().d] : []);
      return docs.some((d) => [...d.querySelectorAll('.artdeco-toast-item, [role=alert], [data-test-artdeco-toast-item-type]')]
        .some((t) => shown(t) && /still saving/i.test(text(t))));
    },
    share() {
      const m = modal(SHARE);
      if (!m) return null;
      const box = m.querySelector('.ql-editor');
      const p = text(m).match(/Posting (\w{3}, \w{3} \d{1,2} at \d{1,2}:\d{2}\s?[AP]M)/);
      return {
        text: box ? box.innerText.replace(/\n+$/, '') : null,
        posting: p ? 'Posting ' + p[1] : '',
        all: text(m).slice(0, 200),
        primary: text(m.querySelector('.share-actions__primary-action')),
        clock: !!iconButton(m, 'clock-medium'),
      };
    },
    // typeShare writes the post's text in the share box (Quill), a line at a
    // time, each next line after an Enter that Quill handles.
    async typeShare(t) {
      const box = modal(SHARE)?.querySelector('.ql-editor');
      if (!box) return false;
      box.focus();
      select(box, false);
      const lines = t.replace(/\r\n/g, '\n').trim().split('\n');
      for (let i = 0; i < lines.length; i++) {
        if (i > 0) {
          if (keydown(box, 'Enter', 13)) app().d.execCommand('insertParagraph');
          await sleep(300);
        }
        if (lines[i]) app().d.execCommand('insertText', false, lines[i]);
        await sleep(200);
      }
      return true;
    },
    pressClock: () => press($$('button[aria-label="Schedule post"]', modal(SHARE) ?? app().d).find(live)),
    // when is the schedule view: the time it states ("Fri, Nov 20, 9:00 AM
    // Western Indonesia Time, based on your location"), and its date and
    // time boxes.
    when() {
      const m = modal(SHARE);
      const d = m?.querySelector('input[aria-label="Date"]');
      if (!d) return null;
      const t = m.querySelector('input[aria-label="Time"]');
      const head = $$('*', m).find((e) => e.children.length === 0 && /based on your location/i.test(text(e)));
      return { stated: head ? text(head) : '', date: d.value, time: t ? t.value : '' };
    },
    pressDateBox: () => press(modal(SHARE)?.querySelector('input[aria-label="Date"]')),
    pressTimeBox: () => press(modal(SHARE)?.querySelector('input[aria-label="Time"]')),
    // calendarMonth is the month the date picker shows ("November 2026"),
    // null while it is closed. Its days are named with their whole date
    // ("Friday, November 20, 2026."), its arrows "Previous month" and "Next
    // month".
    calendarMonth() {
      const c = $$('.artdeco-calendar').find(shown);
      return c ? text(c.querySelector('.artdeco-calendar__month')) : null;
    },
    hasCalendar: (label) => !!$$('.artdeco-calendar').find(shown)?.querySelector(`button[aria-label="${label}"]:not([disabled])`),
    pressCalendar: (label) => mouse($$('button', $$('.artdeco-calendar').find(shown) ?? app().d).find((b) => live(b) && b.getAttribute('aria-label') === label)),
    // The time list, at 15-minute steps ("9:00 AM").
    timeOptions: () => $$('.timepicker__typeahead-result-list [role=option]').filter(shown).length,
    pressTimeOption: (label) => mouse($$('.timepicker__typeahead-result-list [role=option]').find((o) => text(o) === label)),
    pressShareButton: (label) => label !== 'Publish' && press($$('button', modal(SHARE) ?? app().d).find((b) => live(b) && text(b) === label)),
    pressSchedule() {
      const b = modal(SHARE)?.querySelector('.share-actions__primary-action');
      return !!b && live(b) && text(b) === 'Schedule' && press(b);
    },
    // zone is the browser's own name and offset for its zone at a local
    // time, to compare with the zone the schedule view states.
    zone(y, mo, d, h, mi) {
      const t = new Date(y, mo - 1, d, h, mi);
      const p = Intl.DateTimeFormat('en-US', { timeZoneName: 'long' }).formatToParts(t).find((p) => p.type === 'timeZoneName');
      return { name: p ? p.value : '', offset: -t.getTimezoneOffset() * 60 };
    },
    scheduledNotice: () => !!$$('.artdeco-modal').find((m) => shown(m) && /has been scheduled/i.test(text(m))),
    pressButton: (label) => press($$('button').find((b) => shown(b) && text(b) === label)),

    // ---- the Manage menu and its Articles dialog (Drafts, Scheduled,
    // Published)
    pressManage: () => press($$('button[aria-label="Manage menu"]').find(shown)),
    hasDropdownItem: (label) => !!$$('.artdeco-dropdown__item').find((e) => shown(e) && text(e) === label),
    pressDropdownItem: (label) => press($$('.artdeco-dropdown__item').find((e) => shown(e) && text(e) === label)),
    // articles is the dialog's open tab, and its rows as far as they are
    // loaded; null while it is not shown. The list shows 10 rows, then "Show
    // more results".
    articles() {
      const m = modal(MANAGE);
      if (!m) return null;
      const list = rows().map((li) => {
        const a = li.querySelector('a[href*="/article/edit/"], a[href*="/pulse/"], h2 a');
        const id = a?.getAttribute('href')?.match(/\/article\/edit\/(\d+)/);
        return {
          id: id ? id[1] : '',
          title: text(li.querySelector('h2')),
          href: a ? a.href : '',
          date: text(li.querySelector('.article-editor-article-management-list-item__date')),
        };
      });
      return {
        tab: text(m.querySelector('[role=tab][aria-selected="true"]')),
        rows: list,
        loading: $$('.artdeco-loader', m).some(shown),
        more: !!$$('button', m).find((b) => live(b) && text(b) === 'Show more results'),
        empty: !list.length && /No .* yet/.test(text(m)),
      };
    },
    pressMore: () => press($$('button', modal(MANAGE) ?? app().d).find((b) => live(b) && text(b) === 'Show more results')),
    pressTab: (label) => press($$('[role=tab]', modal(MANAGE) ?? app().d).find((t) => shown(t) && text(t) === label)),
    scrollArticles() {
      const c = modal(MANAGE)?.querySelector('.artdeco-modal__content');
      if (c) c.scrollTop = c.scrollHeight;
      return !!c;
    },
    rowIndex: (id) => rows().indexOf(rowOf(id)),
    pressRowLink: (id) => press($$('h2 a', rowOf(id) ?? app().d.createElement('i')).find(shown)),
    pressRowMenu: (id) => press($$('.article-editor-article-management-list-item__actions-trigger', rowOf(id) ?? app().d.createElement('i')).find(shown)),
    hasRowMenuItem: (id, label) => !!$$('.artdeco-dropdown__item', rowOf(id) ?? app().d.createElement('i')).find((e) => shown(e) && text(e) === label),
    pressRowMenuItem: (id, label) => press($$('.artdeco-dropdown__item', rowOf(id) ?? app().d.createElement('i')).find((e) => shown(e) && text(e) === label)),
    // LinkedIn asks in the row itself: "Delete draft? You can't undo this
    // action." with "Not now" and "Delete", and for a scheduled article "To
    // edit this scheduled article, move it to drafts." with "Not now" and
    // "Move to drafts". rowAsks says whether row i asks (its text matches re
    // and it offers label); pressRowAsk presses label there.
    rowAsks(i, re, label) {
      const li = rows()[i];
      return !!li && new RegExp(re, 'i').test(text(li)) && !!button(li, label);
    },
    rowAnswered(i, re, label) {
      return !this.rowAsks(i, re, label);
    },
    pressRowAsk(i, re, label) {
      const li = rows()[i];
      return !!li && new RegExp(re, 'i').test(text(li)) && press(button(li, label));
    },
    // pressRowTitle presses the title of row i, a link for a draft and a
    // button for a scheduled article, when it reads title.
    pressRowTitle(i, title) {
      const t = $$('h2 a, h2 button', rows()[i] ?? app().d.createElement('i')).find(live);
      return !!t && text(t) === title.replace(/\s+/g, ' ').trim() && press(t);
    },
    // modals are the dialogs shown, for an error message.
    modals() {
      const list = (d, where) => [...d.querySelectorAll('.artdeco-modal, [role=dialog]')].filter(shown)
        .map((m) => where + ': "' + text(m).slice(0, 80) + '"');
      const a = app();
      return list(document, 'page').concat(a.d !== document ? list(a.d, 'frame') : []).join('; ') || 'none';
    },
  };
})();
