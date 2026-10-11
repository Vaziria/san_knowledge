// Helpers installed in Medium's pages (the editor, and the stories page that
// lists drafts) as window.__sanMedium. Every selector that tracks Medium's
// markup is in this file or in editor.go.
window.__sanMedium = (() => {
  const EDITOR = '.postArticle-content';
  const grafs = () => [...document.querySelectorAll(EDITOR + ' .section-inner > .graf')];
  const figures = () => [...document.querySelectorAll(EDITOR + ' figure.graf--figure')];
  const byName = (name) => document.querySelector(EDITOR + ' [name="' + name + '"]');

  // The stories page has generated class names, so it is read by structure:
  // a table row per story, its link to /p/<id>/edit, its title heading, and
  // the "Toggle actions menu" button. The link's source parameter says which
  // tab the row is in ("...writer_outbox_drafts...").
  const editLink = (tr) => [...tr.querySelectorAll('a[href*="/p/"]')].find(a => /\/p\/[0-9a-f]+\/edit/.test(a.href));
  const rowOf = (id) => [...document.querySelectorAll('tbody tr')].find(tr => {
    const a = editLink(tr);
    return a && a.href.includes('/p/' + id + '/edit');
  });
  // Elements the Go side should click next are marked, then clicked by this
  // attribute; only one is marked at a time.
  const mark = (el) => {
    document.querySelectorAll('[data-san-medium]').forEach(e => e.removeAttribute('data-san-medium'));
    if (!el) return false;
    el.scrollIntoView({block: 'center'});
    el.setAttribute('data-san-medium', 'target');
    return true;
  };

  return {
    // The Drafts tab of the stories page: its count, and its rows as far as
    // they are loaded. null until the tab is drawn.
    drafts() {
      const tab = [...document.querySelectorAll('a[href*="/me/stories"]')].find(a => /^Drafts/.test(a.innerText.trim()));
      if (!tab) return null;
      const n = tab.innerText.match(/(\d+)/);
      const rows = [...document.querySelectorAll('tbody tr')].map(tr => {
        const a = editLink(tr);
        if (!a) return null;
        const h = tr.querySelector('h2, h3');
        const ps = [...tr.querySelectorAll('p')].map(p => p.innerText.trim());
        return {
          id: a.href.match(/\/p\/([0-9a-f]+)\/edit/)[1],
          title: h ? h.innerText.trim() : '',
          draft: /outbox_drafts/.test(a.href),
          read: ps.find(t => /read/.test(t)) || '',
          updated: ps.find(t => /^Updated/.test(t)) || '',
        };
      }).filter(Boolean);
      return {count: n ? +n[1] : 0, rows};
    },
    draftsReady() {
      return this.drafts() !== null;
    },

    // Any tab of the stories page, by its label ("Scheduled", "Published"):
    // its count, and its rows, with every line of text each row shows.
    stories(label) {
      const tab = [...document.querySelectorAll('a[href*="/me/stories"]')].find(a => a.innerText.trim().startsWith(label));
      if (!tab) return null;
      const n = tab.innerText.match(/(\d+)/);
      const rows = [...document.querySelectorAll('tbody tr')].map(tr => {
        const a = [...tr.querySelectorAll('a[href*="/p/"], a[href*="medium.com/"]')].find(a => /\/p\/[0-9a-f]+|-[0-9a-f]{8,16}(\?|$)/.test(a.href));
        if (!a) return null;
        const m = a.href.match(/\/p\/([0-9a-f]+)/) || a.href.match(/-([0-9a-f]{8,16})(\?|$)/);
        const h = tr.querySelector('h2, h3');
        return {
          id: m[1],
          title: h ? h.innerText.trim() : '',
          href: a.href,
          lines: [...tr.querySelectorAll('p')].map(p => p.innerText.trim()).filter(Boolean),
        };
      }).filter(Boolean);
      return {count: n ? +n[1] : 0, rows};
    },
    storiesReady(label) {
      return this.stories(label) !== null;
    },

    // The publish page, /p/<id>/submission?submitType=publishing-post.
    button(text) {
      return [...document.querySelectorAll('button')].find(b => b.innerText.trim() === text && b.getBoundingClientRect().width > 0) || null;
    },
    markButton(text) {
      return mark(this.button(text));
    },
    publishReady() {
      return !!this.button('Publish') || !!this.button('Schedule to publish');
    },
    // The publish page itself, not the editor, which has a Publish button too.
    onPublishPage() {
      return location.pathname.includes('/submission') && this.publishReady();
    },
    // Topics already on the story: each is a chip with a "Remove <topic>"
    // button next to the topic box.
    topics() {
      const i = document.querySelector('input[aria-controls="tagMultiSelectMenu"]');
      if (!i) return null;
      return [...i.parentElement.querySelectorAll('button[aria-label^="Remove "]')].map(b => b.getAttribute('aria-label').slice(7));
    },
    topicFocused() {
      const a = document.activeElement;
      return !!a && a.matches('input[aria-controls="tagMultiSelectMenu"]');
    },
    // Where the topic box's suggestion that is exactly topic is in its list,
    // from 0, read without its count ("Programming (14M)"). Each suggestion
    // is a button. Null while there is none.
    topicSuggestion(topic) {
      const menu = document.getElementById('tagMultiSelectMenu');
      if (!menu) return null;
      const want = topic.trim().toLowerCase();
      const name = (b) => b.innerText.replace(/\s*\([\d.,]+[KMB]?\)\s*$/i, '').trim().toLowerCase();
      const i = [...menu.querySelectorAll('button')].findIndex(b => name(b) === want);
      return i >= 0 ? i : null;
    },
    // The schedule form: the button that opens the date picker shows the
    // chosen time, e.g. "10/18/2026, 9:00 AM GMT+7".
    scheduleShown() {
      const b = document.querySelector('button[aria-controls="date-picker"]');
      return b ? b.innerText.replace(/\s+/g, ' ').trim() : '';
    },
    pickerOpen() {
      const p = document.getElementById('date-picker');
      return !!p && p.getBoundingClientRect().height > 0;
    },
    pickerMonth() {
      const h = document.querySelector('#date-picker h2');
      return h ? h.innerText.trim() : '';
    },
    markNextMonth() {
      const h = document.querySelector('#date-picker h2');
      const bs = h ? [...h.parentElement.querySelectorAll('button')] : [];
      return mark(bs[bs.length - 1]);
    },
    // Mark a button of the picker: kind "day" (of the month shown), "hour"
    // (0-23) or "minute" (0-59). The picker's buttons are, in order: the two
    // month arrows, the days (with the previous month's last days first),
    // then 24 hours and 60 minutes. False when it is missing or disabled.
    markPick(kind, n) {
      const p = document.getElementById('date-picker');
      if (!p) return false;
      const h = p.querySelector('h2');
      const head = h ? [...h.parentElement.querySelectorAll('button')] : [];
      const rest = [...p.querySelectorAll('button')].filter(b => !head.includes(b));
      if (rest.length < 84 + 28) return false;
      let b = null;
      if (kind === 'day') {
        const days = rest.slice(0, rest.length - 84);
        const start = days.findIndex(d => d.innerText.trim() === '1');
        b = start < 0 ? null : days[start + n - 1];
        if (b && b.innerText.trim() !== String(n)) b = null;
      } else {
        const list = kind === 'hour' ? rest.slice(-84, -60) : rest.slice(-60);
        b = list[n] || null;
        if (b && +b.innerText.trim() !== n) b = null;
      }
      if (!b || b.disabled) return false;
      return mark(b);
    },
    scrollEnd() {
      window.scrollTo(0, document.body.scrollHeight);
      return true;
    },
    hasRow(id) {
      return !!rowOf(id);
    },
    // Mark the shown actions button of draft id's row. Each row has two, one
    // of them hidden for narrow screens.
    markRowMenu(id) {
      const tr = rowOf(id);
      if (!tr) return false;
      return mark([...tr.querySelectorAll('button[aria-label="Toggle actions menu"]')].find(b => b.getBoundingClientRect().width > 0));
    },
    // Mark the button labelled label in an open dialog whose text matches re.
    markDialogButton(label, re) {
      const d = [...document.querySelectorAll('[role=dialog]')].find(d => new RegExp(re).test(d.innerText));
      if (!d) return false;
      return mark([...d.querySelectorAll('button')].find(b => b.innerText.trim() === label));
    },

    // The Write link in the header of Medium's pages, which opens a new story.
    // Signed out, Write goes through the sign-in page instead, so a shown
    // Write link means signed in.
    writeLink() {
      return [...document.querySelectorAll('a[href*="/new-story"]')]
        .find(a => !a.href.includes('/m/signin') && a.getBoundingClientRect().width > 0) || null;
    },
    markWrite() {
      return mark(this.writeLink());
    },
    // The home page is drawn, with its Write link.
    homeReady() {
      return !!this.writeLink();
    },

    // The editor is drawn.
    editorReady() {
      return !!document.querySelector(EDITOR + ' h3.graf--title');
    },
    // What the page is showing: the editor, a sign-in page, or a challenge.
    // Outside the editor, a page with no Write link but a shown link to the
    // sign-in page is a signed-out one.
    state() {
      const path = location.pathname;
      const outside = !document.querySelector(EDITOR);
      const signinLink = [...document.querySelectorAll('a[href*="/m/signin"]')].some(a => a.getBoundingClientRect().width > 0);
      return {
        editor: !!document.querySelector(EDITOR + ' h3.graf--title'),
        signin: /\/m\/signin|\/signin|\/login/.test(path) ||
          outside && (!!document.querySelector('[data-testid="signInButton"]') || signinLink && !this.writeLink()),
        blocked: /Attention Required!.*Cloudflare/.test(document.title) ||
          /Sorry, you have been blocked/.test(document.body ? document.body.innerText.slice(0, 300) : ''),
        challenge: document.title.includes('Just a moment') ||
          !!document.querySelector('#challenge-form, .cf-turnstile, iframe[src*="challenges.cloudflare.com"]'),
        url: location.href,
      };
    },

    // Hand HTML to the editor's own paste handler, as a paste from the
    // clipboard would. Medium turns it into its paragraphs and saves them.
    paste(html) {
      // Never over the title: a paste there replaces it.
      const n = getSelection().anchorNode;
      const g = n && (n.nodeType === 1 ? n : n.parentElement).closest('.graf');
      if (!g || g.classList.contains('graf--title')) return false;
      const dt = new DataTransfer();
      dt.setData('text/html', html);
      dt.setData('text/plain', html.replace(/<[^>]+>/g, ' '));
      const ev = new ClipboardEvent('paste', {clipboardData: dt, bubbles: true, cancelable: true});
      (document.activeElement || document.querySelector(EDITOR)).dispatchEvent(ev);
      return ev.defaultPrevented;
    },

    // The name of the paragraph whose whole text is token, scrolled into
    // view, or "".
    find(token) {
      const g = grafs().find(e => e.textContent.trim() === token);
      if (!g) return '';
      g.scrollIntoView({block: 'center'});
      return g.getAttribute('name');
    },

    // Select all text of a paragraph, for the Backspace that empties it. True
    // only when the selection is that text and nothing else.
    select(name, token) {
      const g = byName(name);
      if (!g) return false;
      const r = document.createRange();
      r.selectNodeContents(g);
      const s = getSelection();
      s.removeAllRanges();
      s.addRange(r);
      return s.toString().trim() === token;
    },

    // Paste plain text into the field that has the caret, when that field is
    // the one meant: "alt" for the alt text box, "caption" for the caption of
    // figure name. One paste, unlike typing, cannot spill into another field
    // if the page moves the caret half-way.
    pasteText(kind, text, name) {
      const a = document.activeElement;
      let ok = false;
      if (kind === 'alt') ok = !!a && a.classList.contains('js-textAreaEditor');
      if (kind === 'caption') {
        const f = byName(name);
        ok = !!a && !!f && a.matches('figcaption') && f.contains(a);
      }
      if (!ok) return false;
      const dt = new DataTransfer();
      dt.setData('text/plain', text);
      const ev = new ClipboardEvent('paste', {clipboardData: dt, bubbles: true, cancelable: true});
      a.dispatchEvent(ev);
      return ev.defaultPrevented;
    },

    altText() {
      const a = document.activeElement;
      return a && a.classList.contains('js-textAreaEditor') ? a.textContent : '';
    },

    empty(name) {
      const g = byName(name);
      return !!g && g.classList.contains('graf--empty');
    },

    // Where the first match of sel is drawn, or null while it is not shown.
    rect(sel) {
      const e = document.querySelector(sel);
      if (!e) return null;
      const r = e.getBoundingClientRect();
      if (r.width < 4 || r.height < 4) return null;
      return [Math.round(r.left), Math.round(r.top), Math.round(r.width), Math.round(r.height)].join(',');
    },

    // The + button, once it has moved next to the empty paragraph name.
    plusAt(name) {
      const t = document.querySelector('.inlineTooltip');
      const g = byName(name);
      if (!t || !g || !t.classList.contains('is-active')) return false;
      const b = t.querySelector('button[data-action="inline-menu"]');
      const dy = b.getBoundingClientRect().top - g.getBoundingClientRect().top;
      return Math.abs(dy) < 40 && b.getBoundingClientRect().width > 20;
    },

    // The + menu: open when its buttons are laid out and full size.
    menuOpen() {
      const t = document.querySelector('.inlineTooltip');
      return !!t && t.classList.contains('is-scaled');
    },
    menuReady() {
      const b = document.querySelector('button[data-action="inline-menu-image"]');
      return this.menuOpen() && !!b && b.getBoundingClientRect().width > 20;
    },

    // The i-th image of the story, in document order.
    figure(i) {
      const f = figures()[i];
      if (!f) return null;
      const img = f.querySelector('img.graf-image');
      const cap = f.querySelector('figcaption');
      return {
        name: f.getAttribute('name'),
        src: img ? img.src : '',
        uploaded: !!img && !!img.getAttribute('data-image-id') && /^https:/.test(img.src),
        alt: img ? img.alt : '',
        caption: cap && !f.classList.contains('is-defaultValue') ? cap.textContent.trim() : '',
        captionVisible: !!cap && cap.getBoundingClientRect().height > 0,
      };
    },
    figureCount() {
      return figures().length;
    },

    altEditorFocused() {
      const a = document.activeElement;
      return !!a && a.classList.contains('js-textAreaEditor');
    },

    // Whether Medium has saved everything: no unsaved changes, and the status
    // next to the logo reads Saved. error is the banner Medium shows when a
    // save fails.
    saveState() {
      let dirty = false;
      try {
        dirty = window.onbeforeunload ? window.onbeforeunload(new Event('beforeunload', {cancelable: true})) !== undefined : false;
      } catch (e) {
        dirty = true;
      }
      const msg = document.querySelector('.js-metabarMessage');
      const banner = [...document.querySelectorAll('body *')].find(e => e.children.length <= 1 && /cannot save your story/i.test(e.textContent));
      return {
        dirty,
        saved: !!msg && /Saved/.test(msg.textContent),
        error: banner ? banner.textContent.trim() : '',
      };
    },

    // What the saved story holds, to check it against the article.
    titleText() {
      const t = document.querySelector(EDITOR + ' h3.graf--title');
      return t ? t.textContent : '';
    },

    snapshot() {
      const t = document.querySelector(EDITOR + ' h3.graf--title');
      const s = document.querySelector(EDITOR + ' h4.graf--subtitle');
      const tokens = grafs().filter(e => /^san-medium-image-\d+$/.test(e.textContent.trim())).length;
      const msg = document.querySelector('.js-metabarMessage');
      return {
        status: msg ? msg.firstChild && msg.firstChild.textContent.trim() : '',
        title: t ? t.textContent.trim() : '',
        subtitle: s ? s.textContent.trim() : '',
        grafs: grafs().length,
        tokens,
        figures: figures().map((_, i) => this.figure(i)),
        text: document.querySelector(EDITOR) ? document.querySelector(EDITOR).innerText : '',
      };
    },
  };
})();
true;
