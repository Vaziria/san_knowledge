// What san_remote knows of instagram.com's pages, as found on 2026-10-29
// with the interface in English (lang en-gb, the account @archetipe__): the
// side menu, the "Create new post" dialog (Crop, Edit, then the caption
// screen), the profile's grid and a post's page. Like tiktok.js, it runs in
// the extension's isolated world, where the page cannot see it.
//
// What reaches Instagram from here (2026-10-29):
//   - the files: the dialog's own file input (multiple, accept image/avif,
//     jpeg, png, heic, heif, video/mp4, quicktime), given them through a
//     DataTransfer. It takes up to 20; a 21st is dropped with a note "You can
//     only choose 10 or fewer files" all the same.
//   - the crop: "Select Crop", then Original, 1:1, 4:5 or 16:9, plain clicks.
//     1:1 is chosen at first; the chosen one is drawn in another colour.
//   - the caption (Lexical): a paste event, which keeps line breaks, emoji,
//     #tags and @mentions as text. Instagram takes the caption for the post
//     only when the box loses focus, as it does when a person clicks Share:
//     its counter counts a pasted caption at once, but Share pressed while
//     the box still had focus sent caption= empty, and a post went out with
//     none (2026-10-29, seen in /api/v1/media/configure/). So the box is
//     blurred after the paste, and Share is pressed with the caption checked
//     in that request (shareGuard in service.go).
//   - alt text: Accessibility opens one input per photo, in order; a value
//     set through the input's own setter and an input event is kept.
//   - the switches (Add AI label, Share to Threads, Share to Facebook):
//     role=switch inputs with aria-checked, off at first.
// Clicks are .click() on the element that has the handler. React does not
// render a hidden page, so all but reading needs the tab in front.
globalThis.__sanInstagram = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const own = (e) => (e ? (e.innerText || '').trim() : '');
  const press = (e) => {
    if (!e) return false;
    e.click();
    return true;
  };
  const dialogs = () => [...document.querySelectorAll('[role=dialog]')].filter(vis);
  // create is the "Create new post" dialog: the one with the file input or a
  // step of the post in it (the last dialog shown).
  const create = () => dialogs().filter((d) => d.querySelector('input[type=file]') || d.querySelector('h1, [role=heading]')).pop() ?? null;
  const heading = (d) => own(d?.querySelector('h1, [role=heading]'));
  const buttons = (root) => [...(root ?? document).querySelectorAll('[role=button], button')].filter(vis);
  const button = (root, label) => buttons(root).find((b) => own(b) === label) ?? null;
  // svgButton is the control holding the icon named label ("Select Crop").
  const svgButton = (root, label) =>
    [...(root ?? document).querySelectorAll(`svg[aria-label="${label}"]`)].filter(vis).map((s) => s.closest('[role=button], button, a')).find(Boolean) ?? null;
  const editor = () => create()?.querySelector('[contenteditable=true][aria-label^="Add a caption"]') ?? create()?.querySelector('[contenteditable=true]') ?? null;
  const altInputs = () => [...(create()?.querySelectorAll('input') ?? [])].filter((i) => /alt text/i.test(i.placeholder));
  // switchFor is the switch whose row reads like re ("Add AI label",
  // "Threads", "Facebook").
  const switchFor = (re) =>
    [...(create()?.querySelectorAll('input[role=switch], [role=switch]') ?? [])].find((s) => {
      let c = s;
      for (let k = 0; k < 8 && c && !/[A-Za-z]/.test(c.innerText || ''); k++) c = c.parentElement;
      return re.test(c?.innerText || '');
    }) ?? null;
  const switches = { ai: /Add AI label/, threads: /Threads/, facebook: /Facebook/ };
  const checked = (s) => !!s && (s.checked === true || s.getAttribute('aria-checked') === 'true');
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  const profileLink = () =>
    [...document.querySelectorAll('a[href]')].find((a) => /'s profile picture$/.test(a.querySelector('img')?.alt ?? '') && /^\/[^/]+\/$/.test(a.getAttribute('href')));

  return {
    // state is what the page shows, for the waits.
    state() {
      const d = create();
      return {
        url: location.href,
        lang: document.documentElement.lang,
        home: !!svgButton(document, 'New post') && !!profileLink(),
        signin: /^\/accounts\/login/.test(location.pathname) || !!document.querySelector('input[name=username]'),
        picker: !!d?.querySelector('input[type=file]') && /Select [Ff]rom [Cc]omputer/.test(own(d)),
        step: heading(d),
        crop: !!svgButton(d, 'Select Crop'),
        caption: !!editor(),
        // A dialog about the files: "File couldn't be uploaded" and why, or
        // "Video posts are now shared as reels".
        notice: dialogs().map(own).find((t) => /couldn.t be uploaded|shared as reels/i.test(t)) ?? '',
        shared: dialogs().some((x) => /has been shared/i.test(own(x))),
        failed: dialogs().map(own).find((t) => /could not be shared|couldn.t be shared|Something went wrong/i.test(t)) ?? '',
        sharing: /^Sharing/.test(heading(d)),
        dialog: !!d,
      };
    },
    // account is the signed-in account's @name, from the side menu's
    // profile link.
    account() {
      const h = profileLink()?.getAttribute('href') ?? '';
      return decodeURIComponent(h.replace(/\//g, ''));
    },
    // profileName is the name a profile's page shows, from its og:title
    // ("Resan (@archetipe__) • Instagram photos and videos").
    profileName(handle) {
      const og = document.querySelector('meta[property="og:title"]')?.content ?? '';
      const i = og.indexOf(` (@${handle})`);
      return i > 0 ? og.slice(0, i) : '';
    },
    pressNewPost: () => press(svgButton(document, 'New post')),
    // pressPostItem presses Post in the menu New post opens on some accounts
    // (Post, Live video, Ad) instead of the dialog.
    pressPostItem: () => press([...document.querySelectorAll('a, [role=link], [role=button], [role=menuitem]')].filter(vis).find((e) => own(e) === 'Post' && e.querySelector('svg'))),
    // A file to upload comes in base64 pieces; each becomes a Blob at once.
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // choose gives the dialog's file input the staged files, in order, as if
    // they were chosen in its file dialog. It returns "" or what went wrong.
    choose(files) {
      const input = create()?.querySelector('input[type=file]') ?? document.querySelector('[role=dialog] input[type=file]');
      if (!input) return 'no file input in the dialog';
      const list = new DataTransfer();
      for (const f of files) {
        const parts = staged.get(f.id);
        if (!parts) return 'nothing staged for ' + f.name;
        staged.delete(f.id);
        list.items.add(new File(parts, f.name, { type: f.type, lastModified: Date.now() }));
      }
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // okNotice presses OK in "Video posts are now shared as reels".
    okNotice: () => press(button(dialogs().find((d) => /shared as reels/i.test(own(d))), 'OK')),
    // gallery is how many files the post holds: the thumbnails in Open Media
    // Gallery, which it opens; -1 when there is no gallery.
    gallery() {
      const d = create();
      const g = svgButton(d, 'Open Media Gallery');
      if (!g) return -1;
      const thumbs = () => [...d.querySelectorAll('[style*="background-image"]')].filter((e) => vis(e) && e.getBoundingClientRect().width < 150);
      if (!thumbs().length) g.click();
      return thumbs().length;
    },
    closeGallery() {
      const d = create();
      const g = svgButton(d, 'Open Media Gallery');
      return !!g && (g.click(), true);
    },
    pressSelectCrop: () => press(svgButton(create(), 'Select Crop')),
    // crop is the crop options shown and the chosen one: the one drawn in a
    // colour of its own.
    crop() {
      const opts = buttons(create()).filter((b) => /^(Original|1:1|4:5|16:9|9:16)$/.test(own(b)));
      const colour = (b) => getComputedStyle(b.querySelector('span') ?? b).color;
      const count = {};
      opts.forEach((b) => { count[colour(b)] = (count[colour(b)] ?? 0) + 1; });
      const lone = opts.filter((b) => count[colour(b)] === 1);
      return { options: opts.map(own), chosen: opts.length > 2 && lone.length === 1 ? own(lone[0]) : '' };
    },
    pressCrop: (label) => press(button(create(), label)),
    pressNext: () => press(button(create(), 'Next')),
    // editor: the caption as it reads, one line per line, and the counter
    // ("56/2,200").
    editor() {
      const e = editor();
      if (!e) return null;
      return {
        focused: document.activeElement === e,
        text: e.innerText.replace(/\n$/, ''),
        counter: buttons(create()).map(own).find((t) => /^\d[\d,.]*\/2[,.]?200$/.test(t)) ?? '',
      };
    },
    // paste pastes text at the end of the caption.
    paste(t) {
      const e = editor();
      if (!e) return false;
      e.focus();
      const r = document.createRange();
      r.selectNodeContents(e);
      r.collapse(false);
      getSelection().removeAllRanges();
      getSelection().addRange(r);
      const dt = new DataTransfer();
      dt.setData('text/plain', t);
      const ev = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true });
      e.dispatchEvent(ev);
      return ev.defaultPrevented;
    },
    // blurCaption takes the focus off the caption box, which is when
    // Instagram takes the caption: on the box's blur event, which blur()
    // fires only while the window has the focus. A shown window the user is
    // not typing in has not, and posts went out with no caption
    // (2026-10-29), so the events are sent too.
    blurCaption() {
      const e = editor();
      if (!e) return false;
      e.blur();
      e.dispatchEvent(new FocusEvent('blur'));
      e.dispatchEvent(new FocusEvent('focusout', { bubbles: true }));
      return true;
    },
    // switches: Add AI label, Share to Threads and Share to Facebook, on or
    // off; a switch that is not there is null.
    switches() {
      const out = {};
      for (const [k, re] of Object.entries(switches)) {
        const s = switchFor(re);
        out[k] = s ? checked(s) : null;
      }
      return out;
    },
    pressSwitch: (k) => press(switchFor(switches[k])),
    // alts is each photo's alt text input's value, once Accessibility is
    // open; null while it is closed.
    alts() {
      const all = altInputs();
      return all.length && all.every(vis) ? all.map((i) => i.value) : null;
    },
    pressAccessibility: () => press(button(create(), 'Accessibility')),
    // setAlt sets the alt text of photo i (from 0) as typing would.
    setAlt(i, text) {
      const input = altInputs()[i];
      if (!input) return false;
      input.focus();
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set.call(input, text);
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.blur();
      return true;
    },
    // shareButton is Share on the caption screen, and whether it can be
    // pressed.
    shareButton() {
      const b = button(create(), 'Share');
      return b ? { enabled: b.getAttribute('aria-disabled') !== 'true' } : null;
    },
    pressShare: () => press(button(create(), 'Share')),
    // serverTime is Instagram's clock, from the Date of a request to it, in
    // ms; 0 when it cannot be read. This computer's clock may be minutes off
    // (it was 4.5 minutes fast, 2026-10-29), and posts carry Instagram's.
    async serverTime() {
      try {
        const r = await fetch('/', { method: 'HEAD', credentials: 'include', cache: 'no-store' });
        return Date.parse(r.headers.get('date') ?? '') || 0;
      } catch {
        return 0;
      }
    },
    // close closes the dialog (its Close icon), and Discard answers "Discard
    // post?".
    pressClose: () => press(svgButton(document, 'Close')),
    pressDiscard: () => press(button(dialogs().find((d) => /Discard post\?/.test(own(d))), 'Discard')),

    // ---- deleting a post (2026-10-29: its page's More Options, Delete, then
    // "Delete post?" Delete; Instagram then goes to the profile) ----
    pressMoreOptions: () => press(svgButton(document, 'More Options') ?? svgButton(document, 'More options')),
    // menu is the More Options menu's items: Delete, Edit, … for the
    // account's own post.
    menu: () => dialogs().flatMap((d) => buttons(d).map(own)).filter(Boolean),
    pressDelete: () => press(dialogs().map((d) => button(d, 'Delete')).find(Boolean)),
    // confirmDelete answers Delete to "Delete post?".
    confirmDelete: () => press(button(dialogs().find((d) => /^Delete post\?/.test(own(d))), 'Delete')),

    // ---- the profile and a post ----
    // grid is the profile's posts as its grid lists them, newest first,
    // pinned ones on top: their links ("/archetipe__/p/<code>/") and whether
    // they are pinned.
    grid: () =>
      [...document.querySelectorAll('main a[href]')]
        .filter((a) => /\/(p|reel)\/[^/]+\/$/.test(a.getAttribute('href')))
        .map((a) => ({ href: a.getAttribute('href'), pinned: !!a.querySelector('svg[aria-label*="Pinned" i]') })),
    // post is a post's page: its time (the time that links to the post
    // itself; another is when its caption was edited), its caption, from
    // og:title (`Name on Instagram: "caption"`), and its kind.
    post() {
      const path = location.pathname;
      const code = path.match(/\/(p|reel)\/([^/]+)\//);
      if (!code) return null;
      const t = [...document.querySelectorAll('time[datetime]')].find((t) => {
        const h = t.closest('a')?.getAttribute('href') ?? '';
        return h === path || h.endsWith(`/${code[1]}/${code[2]}/`);
      });
      const og = document.querySelector('meta[property="og:title"]')?.content ?? '';
      const m = og.match(/^.*? on Instagram: "([\s\S]*)"$/);
      return { kind: code[1], code: code[2], at: t?.getAttribute('datetime') ?? '', caption: m ? m[1] : '', carousel: !!document.querySelector('main button[aria-label="Next"]'), video: !!document.querySelector('main video') };
    },
  };
})();
