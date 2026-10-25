// What san_remote knows of Medium's pages, as found on 2026-10-25 with the
// interface in English (the account Getresolvedid): the home page, the
// classic editor (.postArticle-content, graf--* paragraphs), the publish page
// (/p/<id>/submission) and the stories page (/me/stories). Most selectors are
// golang/packages/san_medium's (editor.js), found with real input on
// 2026-10-11; what differs is how input reaches the page from the extension's
// isolated world, where the page cannot see it:
//   - the title: one keydown and keypress, then execCommand's insertText.
//     insertText alone leaves Medium's "Title" placeholder in place and makes
//     no story; Medium drops it on the key, then makes the story.
//   - Backspace in an image's placeholder: a keydown Medium handles itself.
//   - an image: the + menu's image button adds a file input and clicks it;
//     the click opens no chooser (no user gesture), and the input is given
//     the file instead.
//   - a topic: a click on the suggestion adds it. With a real mouse the box
//     lost the focus first and only closed (2026-10-11).
//   - the date picker closes on Escape.
globalThis.__sanMedium = (() => {
  const EDITOR = '.postArticle-content';
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const grafs = () => [...document.querySelectorAll(EDITOR + ' .section-inner > .graf')];
  const figures = () => [...document.querySelectorAll(EDITOR + ' figure.graf--figure')];
  const byName = (name) => document.querySelector(EDITOR + ' [name="' + name + '"]');
  const press = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.click();
    return true;
  };
  const button = (text) => [...document.querySelectorAll('button')].find((b) => b.innerText.trim() === text && vis(b)) ?? null;
  const select = (node, collapse) => {
    const r = document.createRange();
    r.selectNodeContents(node);
    if (collapse !== undefined) r.collapse(collapse);
    const s = getSelection();
    s.removeAllRanges();
    s.addRange(r);
    return s;
  };
  const key = (type, k, code) =>
    (document.activeElement ?? document.body).dispatchEvent(
      new KeyboardEvent(type, { key: k, keyCode: code, which: code, charCode: type === 'keypress' ? k.charCodeAt(0) : 0, bubbles: true, cancelable: true }),
    );
  const pasteInto = (el, data) => {
    const dt = new DataTransfer();
    for (const [type, v] of Object.entries(data)) dt.setData(type, v);
    const ev = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true });
    el.dispatchEvent(ev);
    return ev.defaultPrevented;
  };
  // The Write link in the header. Signed out, Write goes through the sign-in
  // page instead, so a shown Write link means signed in.
  const writeLink = () => [...document.querySelectorAll('a[href*="/new-story"]')].find((a) => !a.href.includes('/m/signin') && vis(a)) ?? null;
  // The stories page is read by structure: a table row per story, its link to
  // /p/<id>/edit, its title heading, and the "Toggle actions menu" button.
  // The link's source parameter says the tab ("...writer_outbox_drafts...").
  const editLink = (tr) => [...tr.querySelectorAll('a[href*="/p/"]')].find((a) => /\/p\/[0-9a-f]+\/edit/.test(a.href));
  const rowOf = (id) => [...document.querySelectorAll('tbody tr')].find((tr) => editLink(tr)?.href.includes('/p/' + id + '/edit'));
  const tabLink = (label) => [...document.querySelectorAll('a[href*="/me/stories"]')].find((a) => a.innerText.trim().startsWith(label));
  const picker = () => document.getElementById('date-picker');
  const pickerHead = () => {
    const h = picker()?.querySelector('h2');
    return h ? [...h.parentElement.querySelectorAll('button')] : [];
  };
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

  return {
    // state is what the page shows: the editor, a sign-in page, a block, or a
    // Cloudflare check. Outside the editor, a page with no Write link but a
    // shown link to the sign-in page is a signed-out one.
    state() {
      const outside = !document.querySelector(EDITOR);
      const signinLink = [...document.querySelectorAll('a[href*="/m/signin"]')].some(vis);
      return {
        url: location.href,
        lang: document.documentElement.lang,
        home: !!writeLink(),
        editor: !!document.querySelector(EDITOR + ' h3.graf--title'),
        signin: /\/m\/signin|\/signin|\/login/.test(location.pathname) ||
          (outside && (!!document.querySelector('[data-testid="signInButton"]') || (signinLink && !writeLink()))),
        blocked: /Attention Required!.*Cloudflare/.test(document.title) ||
          /Sorry, you have been blocked/.test(document.body ? document.body.innerText.slice(0, 300) : ''),
        challenge: document.title.includes('Just a moment') ||
          !!document.querySelector('#challenge-form, .cf-turnstile, iframe[src*="challenges.cloudflare.com"]'),
        publish: location.pathname.includes('/submission') && (!!button('Publish') || !!button('Schedule to publish')),
      };
    },
    // account is the avatar's name ("user options menu") and the sidebar's
    // Profile link's @handle.
    account() {
      const img = document.querySelector('button[aria-label="user options menu"] img[alt]');
      const prof = [...document.querySelectorAll('a[href*="medium.com/@"], a[href^="/@"]')].find((a) => a.innerText.trim() === 'Profile');
      const m = prof?.getAttribute('href')?.match(/\/@([^/?#]+)/);
      return { name: img?.alt ?? '', handle: m ? decodeURIComponent(m[1]) : '' };
    },
    pressWrite: () => press(writeLink()),
    // pressStories opens the stories page from the sidebar's Stories link.
    pressStories: () => press([...document.querySelectorAll('a[href*="/me/stories"]')].find((a) => a.innerText.trim() === 'Stories' && vis(a))),

    // ---- the editor ----
    // typeTitle puts text in the title, replacing what it holds.
    typeTitle(text) {
      const root = document.querySelector(EDITOR);
      const t = root?.querySelector('h3.graf--title');
      if (!t) return false;
      root.focus();
      select(t);
      const c = text.charCodeAt(0);
      const code = /[a-z]/i.test(text[0]) ? text[0].toUpperCase().charCodeAt(0) : /\d/.test(text[0]) ? c : 0;
      const down = key('keydown', text[0], code) && key('keypress', text[0], code);
      if (down) document.execCommand('insertText', false, text);
      key('keyup', text[0], code);
      return down;
    },
    titleText: () => document.querySelector(EDITOR + ' h3.graf--title')?.textContent ?? '',
    // caretInBody puts the caret at the end of the paragraph under the title.
    caretInBody() {
      const root = document.querySelector(EDITOR);
      const p = root?.querySelector('.section-inner > p.graf--p');
      if (!p) return false;
      root.focus();
      select(p, false);
      return true;
    },
    // paste hands HTML to the editor's own paste handler, as a paste from the
    // clipboard would; never over the title, where a paste replaces it.
    paste(html) {
      const n = getSelection().anchorNode;
      const g = n && (n.nodeType === 1 ? n : n.parentElement).closest('.graf');
      if (!g || g.classList.contains('graf--title')) return false;
      return pasteInto(document.activeElement ?? document.querySelector(EDITOR), { 'text/html': html, 'text/plain': html.replace(/<[^>]+>/g, ' ') });
    },
    // find is the name of the paragraph whose whole text is token, or "".
    find(token) {
      const g = grafs().find((e) => e.textContent.trim() === token);
      if (!g) return '';
      g.scrollIntoView({ block: 'center' });
      return g.getAttribute('name');
    },
    // emptyGraf selects the paragraph holding token and presses Backspace,
    // which Medium handles itself: the paragraph stays, empty, with the +
    // button beside it.
    emptyGraf(name, token) {
      const g = byName(name);
      if (!g || g.textContent.trim() !== token) return false;
      document.querySelector(EDITOR).focus();
      if (select(g).toString().trim() !== token) return false;
      if (key('keydown', 'Backspace', 8)) document.execCommand('delete');
      key('keyup', 'Backspace', 8);
      return true;
    },
    empty: (name) => !!byName(name)?.classList.contains('graf--empty'),
    // plusAt: the + button has moved beside the empty paragraph name.
    plusAt(name) {
      const t = document.querySelector('.inlineTooltip');
      const g = byName(name);
      if (!t || !g || !t.classList.contains('is-active')) return false;
      const b = t.querySelector('button[data-action="inline-menu"]');
      return !!b && Math.abs(b.getBoundingClientRect().top - g.getBoundingClientRect().top) < 40 && b.getBoundingClientRect().width > 20;
    },
    pressPlus: () => press(document.querySelector('button[data-action="inline-menu"]')),
    menuReady() {
      const t = document.querySelector('.inlineTooltip');
      const b = document.querySelector('button[data-action="inline-menu-image"]');
      return !!t && t.classList.contains('is-scaled') && !!b && b.getBoundingClientRect().width > 20;
    },
    stage(id, piece) {
      staged.set(id, (staged.get(id) ?? '') + piece);
      return staged.get(id).length;
    },
    // chooseImage presses the + menu's image button, which adds a file input
    // and clicks it, and gives that input the staged file. It returns "" or
    // what went wrong.
    async chooseImage(id, name, type) {
      const before = new Set(document.querySelectorAll('input[type=file]'));
      const b = document.querySelector('button[data-action="inline-menu-image"]');
      if (!b) return 'no image button';
      b.click();
      let input = null;
      for (let i = 0; i < 30 && !input; i++) {
        input = [...document.querySelectorAll('input[type=file]')].find((x) => !before.has(x)) ?? null;
        if (!input) await sleep(100);
      }
      if (!input) return 'the image button added no file input';
      const list = new DataTransfer();
      list.items.add(new File([bytes(staged.get(id) ?? '')], name, { type, lastModified: Date.now() }));
      staged.delete(id);
      input.files = list.files;
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // figure is the i-th image of the story, in document order.
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
    pressImage: (name) => press(byName(name)?.querySelector('img.graf-image')),
    pressAlt: () => press(document.querySelector('.highlightMenu--active button[data-action="alt"]')),
    altEditorFocused: () => !!document.activeElement?.classList.contains('js-textAreaEditor'),
    altText: () => (document.activeElement?.classList.contains('js-textAreaEditor') ? document.activeElement.textContent : ''),
    pressAltSave: () => press(document.querySelector('button[data-action="overlay-submit"]')),
    // pasteText pastes plain text into the field meant: "alt" for the alt
    // text box once it has the caret, "caption" for the caption of figure
    // name, which gets the caret first.
    pasteText(kind, text, name) {
      let el = null;
      if (kind === 'alt' && document.activeElement?.classList.contains('js-textAreaEditor')) el = document.activeElement;
      if (kind === 'caption') {
        const cap = byName(name)?.querySelector('figcaption');
        if (cap && vis(cap)) {
          cap.focus();
          select(cap, true);
          if (document.activeElement === cap) el = cap;
        }
      }
      return !!el && pasteInto(el, { 'text/plain': text });
    },
    // saveState: whether Medium has saved everything, by its beforeunload
    // handler and the status beside the logo; error is the banner Medium
    // shows when a save fails.
    saveState() {
      let dirty = false;
      try {
        dirty = window.onbeforeunload ? window.onbeforeunload(new Event('beforeunload', { cancelable: true })) !== undefined : false;
      } catch (e) {
        dirty = true;
      }
      const msg = document.querySelector('.js-metabarMessage');
      const banner = [...document.querySelectorAll('body *')].find((e) => e.children.length <= 1 && /cannot save your story/i.test(e.textContent));
      return { dirty, saved: !!msg && /Saved/.test(msg.textContent), error: banner ? banner.textContent.trim() : '' };
    },
    // snapshot is what the saved story holds, to check it against the
    // article.
    snapshot() {
      const t = document.querySelector(EDITOR + ' h3.graf--title');
      const s = document.querySelector(EDITOR + ' h4.graf--subtitle');
      const msg = document.querySelector('.js-metabarMessage');
      return {
        status: msg?.firstChild?.textContent.trim() ?? '',
        title: t ? t.textContent.trim() : '',
        subtitle: s ? s.textContent.trim() : '',
        tokens: grafs().filter((e) => /^san-medium-image-\d+$/.test(e.textContent.trim())).length,
        figures: figures().map((_, i) => this.figure(i)),
        text: document.querySelector(EDITOR)?.innerText ?? '',
      };
    },
    // ---- the publish page ----
    pressButton: (text) => press(button(text)),
    hasButton: (text) => !!button(text),
    // topics on the story: each a chip with a "Remove <topic>" button by the
    // topic box. With five topics Medium takes the box away, so then the
    // chips are looked for on the whole page.
    topics() {
      const i = document.querySelector('input[aria-controls="tagMultiSelectMenu"]');
      return [...(i ? i.parentElement : document).querySelectorAll('button[aria-label^="Remove "]')].map((b) => b.getAttribute('aria-label').slice(7));
    },
    pressRemoveTopic: (t) => press([...document.querySelectorAll('button[aria-label^="Remove "]')].find((b) => b.getAttribute('aria-label').slice(7).toLowerCase() === t.toLowerCase())),
    // typeTopic types into the topic box as React reads it: the native value
    // setter and an input event per key.
    async typeTopic(text) {
      const i = document.querySelector('input[aria-controls="tagMultiSelectMenu"]');
      if (!i) return false;
      i.focus();
      const set = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
      let v = '';
      for (const ch of text) {
        v += ch;
        i.dispatchEvent(new KeyboardEvent('keydown', { key: ch, bubbles: true }));
        set.call(i, v);
        i.dispatchEvent(new InputEvent('input', { bubbles: true, data: ch, inputType: 'insertText' }));
        i.dispatchEvent(new KeyboardEvent('keyup', { key: ch, bubbles: true }));
        await sleep(60 + Math.random() * 90);
      }
      return i.value === text;
    },
    // suggestions are the topic box's list, without their counts
    // ("Programming (14M)" is "Programming"); null while it is closed.
    suggestions() {
      const menu = document.getElementById('tagMultiSelectMenu');
      return menu ? [...menu.querySelectorAll('button')].map((b) => b.innerText.replace(/\s*\([\d.,]+[KMB]?\)\s*$/i, '').trim()) : null;
    },
    pressSuggestion: (topic) =>
      press([...(document.getElementById('tagMultiSelectMenu')?.querySelectorAll('button') ?? [])].find((b) => b.innerText.replace(/\s*\([\d.,]+[KMB]?\)\s*$/i, '').trim().toLowerCase() === topic.toLowerCase())),
    // The schedule form: the button that opens the date picker shows the
    // chosen time, "11/24/2026, 9:00 AM GMT+7".
    scheduleShown: () => document.querySelector('button[aria-controls="date-picker"]')?.innerText.replace(/\s+/g, ' ').trim() ?? '',
    pressDate: () => press(document.querySelector('button[aria-controls="date-picker"]')),
    pickerOpen: () => vis(picker()),
    pickerMonth: () => picker()?.querySelector('h2')?.innerText.trim() ?? '',
    pressNextMonth: () => press(pickerHead().at(-1)),
    // pressPick presses a button of the picker: kind "day" (of the month
    // shown), "hour" (0-23) or "minute" (0-59). After the two month arrows,
    // the picker's buttons are the days (the last days of the month before
    // first), then 24 hours and 60 minutes.
    pressPick(kind, n) {
      const p = picker();
      if (!p) return false;
      const head = pickerHead();
      const rest = [...p.querySelectorAll('button')].filter((b) => !head.includes(b));
      if (rest.length < 84 + 28) return false;
      let b = null;
      if (kind === 'day') {
        const days = rest.slice(0, rest.length - 84);
        const start = days.findIndex((d) => d.innerText.trim() === '1');
        b = start < 0 ? null : days[start + n - 1];
        if (b && b.innerText.trim() !== String(n)) b = null;
      } else {
        b = (kind === 'hour' ? rest.slice(-84, -60) : rest.slice(-60))[n] ?? null;
        if (b && +b.innerText.trim() !== n) b = null;
      }
      return !!b && !b.disabled && press(b);
    },
    // closePicker sends Escape to the focused element, the picker and the
    // page, as long as the picker stays open. It closed on the picker's once
    // (2026-10-25), and not at all in the next run; the date box shows the
    // time chosen either way.
    async closePicker() {
      for (const el of [document.activeElement, picker(), document]) {
        if (!vis(picker())) return true;
        el?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', keyCode: 27, which: 27, bubbles: true, cancelable: true }));
        await sleep(300);
      }
      return !vis(picker());
    },

    // ---- the stories page ----
    // stories reads a tab of the stories page by its label ("Drafts",
    // "Scheduled"): its count, and its rows with every line each shows. null
    // until the tab is drawn.
    stories(label) {
      const tab = tabLink(label);
      if (!tab) return null;
      const n = tab.innerText.match(/(\d+)/);
      const rows = [...document.querySelectorAll('tbody tr')].map((tr) => {
        const a = editLink(tr);
        if (!a) return null;
        return {
          id: a.href.match(/\/p\/([0-9a-f]+)\/edit/)[1],
          title: tr.querySelector('h2, h3')?.innerText.trim() ?? '',
          draft: /outbox_drafts/.test(a.href),
          scheduled: /outbox_scheduled/.test(a.href),
          lines: [...tr.querySelectorAll('p')].map((p) => p.innerText.trim()).filter(Boolean),
        };
      }).filter(Boolean);
      return { count: n ? +n[1] : -1, rows };
    },
    storiesReady: () => /\/me\/stories/.test(location.pathname) && !!tabLink('Drafts'),
    pressTab: (label) => press(tabLink(label)),
    scrollEnd() {
      window.scrollTo(0, document.body.scrollHeight);
      return true;
    },
    hasRow: (id) => !!rowOf(id),
    // pressRowMenu opens the shown actions menu of story id's row; each row
    // has two, one hidden for narrow screens.
    pressRowMenu: (id) => press([...(rowOf(id)?.querySelectorAll('button[aria-label="Toggle actions menu"]') ?? [])].find(vis)),
    // menuItems are what the open row menu offers: "Edit story", "Publish
    // story", "Review scheduled story", "Delete story", links or buttons.
    menuItems: () => [...document.querySelectorAll('[role=dialog] a, [role=dialog] button, [role=tooltip] a, [role=tooltip] button')].filter(vis).map((e) => e.innerText.trim()).filter(Boolean),
    pressMenuItem: (text) => press([...document.querySelectorAll('[role=dialog] a, [role=dialog] button, [role=tooltip] a, [role=tooltip] button')].find((e) => vis(e) && e.innerText.trim() === text)),
    // pressDialogButton presses the button labelled label in an open dialog
    // whose text matches re.
    pressDialogButton(label, re) {
      const d = [...document.querySelectorAll('[role=dialog]')].find((d) => new RegExp(re).test(d.innerText));
      return press([...(d?.querySelectorAll('button') ?? [])].find((b) => b.innerText.trim() === label));
    },
  };
})();
