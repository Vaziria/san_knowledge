// What san_remote knows of Facebook's pages, as found on 2026-10-24 with the
// interface in Indonesian: the home page and the profile composer ("Buat
// postingan"), its audience dialog and its schedule panel; the profile's
// Content Library (Kelola postingan); Meta Business Suite's composer for a
// Page; a group's composer; the Pages and groups lists. Like threads.js, it
// runs in the extension's isolated world, where the page cannot see it.
//
// Facebook's class names are generated, so controls are found by role and by
// the names Facebook gives them: an aria-label, or the visible text. Clicks
// are .click() on the element that has the handler; the waits and the pace
// are in Go. Meta's pickers only work in a shown page.
globalThis.__sanFacebook = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  // name is a control's accessible name, without the zero-width spaces
  // Business Suite puts in some.
  const name = (e) => (e.getAttribute('aria-label') || e.innerText || '').replace(/​/g, '').trim().replace(/\s+/g, ' ');
  const buttons = (root) => [...(root ?? document).querySelectorAll('[role=button]')].filter(vis);
  const button = (root, test) => root && buttons(root).find((b) => (typeof test === 'string' ? name(b) === test : test.test(name(b))));
  const press = (e) => {
    if (!e) return false;
    e.click();
    return true;
  };
  const dialogs = () => [...document.querySelectorAll('[role=dialog]')].filter(vis);
  // The composer: a dialog with a shown editor. On Business Suite the
  // composer is the page itself.
  const composer = () => dialogs().find((d) => [...d.querySelectorAll('[contenteditable=true]')].some(vis));
  const editor = () => [...((composer() ?? document).querySelectorAll('[contenteditable=true]'))].find(vis);
  // textOf is the text a radio input sits in: the first box above it with any.
  const textOf = (e) => {
    let c = e;
    for (let i = 0; i < 6 && c && !(c.innerText || '').trim(); i++) c = c.parentElement;
    return (c?.innerText || '').trim().split('\n')[0];
  };
  const audienceDialog = () => dialogs().find((d) => /Pilih audiens/.test(d.innerText));
  const picker = () => [...document.querySelectorAll('[role=grid][aria-label="Pilih Tanggal"]')].filter(vis).pop();
  const combos = () => [...document.querySelectorAll('input[role=combobox]')].filter(vis);
  const disabled = (c) => {
    const x = c.matches('[aria-disabled]') ? c : c.querySelector('[aria-disabled]');
    return x?.getAttribute('aria-disabled') === 'true';
  };
  const options = () => [...document.querySelectorAll('[role=option]')].filter(vis);
  const loginForm = () => !!document.querySelector('input[name=email]') && !!document.querySelector('input[name=pass]');
  // The profile: the left column's link with the user's picture and name.
  const reserved = /^\/(friends|groups|watch|marketplace|reel|gaming|saved|onthisday|events|pages|memories|bookmarks|feeds|business|ads|messages|notifications|me)\b/;
  const profileLink = () =>
    [...document.querySelectorAll('[role=navigation] a[href], [role=complementary] a[href]')].find((a) => {
      if (!vis(a) || !a.querySelector('img, image, svg') || !name(a)) return false;
      const u = new URL(a.href, location.origin);
      return u.host.endsWith('facebook.com') && !reserved.test(u.pathname) && (/^\/[A-Za-z0-9.]+\/?$/.test(u.pathname) || u.pathname === '/profile.php');
    });
  // A Content Library row: the post's text, then "Terjadwal •" and its time.
  const rows = () =>
    [...document.querySelectorAll('[role=row]')].filter(vis).flatMap((el) => {
      const lines = el.innerText.split('\n').map((l) => l.trim()).filter(Boolean);
      const at = lines.findIndex((l) => /^(Diterbitkan|Terjadwal|Draf)\b/.test(l));
      if (at < 0) return [];
      return [{ el, text: lines[0], status: lines[at].replace(/\s*•\s*$/, ''), when: lines[at + 1] ?? '' }];
    });
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  // Business Suite's controls.
  const bsEditor = () => [...document.querySelectorAll('[contenteditable=true]')].find((e) => vis(e) && /^Tulis ke dalam kotak dialog/.test(e.getAttribute('aria-label') || ''));
  const bsDate = () => [...document.querySelectorAll('input[placeholder="dd/mm/yyyy"]')].find(vis);
  const spin = (label) => [...document.querySelectorAll('input[role=spinbutton]')].find((i) => vis(i) && i.getAttribute('aria-label') === label);
  const bsSwitch = () => [...document.querySelectorAll('[role=switch]')].find((s) => vis(s) && name(s) === 'Atur tanggal dan waktu');

  return {
    // state is what the page shows, for the waits.
    state() {
      const tab = [...document.querySelectorAll('[role=tab]')].find((t) => vis(t) && t.getAttribute('aria-selected') === 'true');
      return {
        url: location.href,
        path: location.pathname,
        host: location.host,
        lang: document.documentElement.lang,
        visible: document.visibilityState === 'visible',
        signedOut: loginForm(),
        signedIn: !loginForm() && !!document.querySelector('[role=navigation], [role=main]'),
        composer: !!composer(),
        audience: !!audienceDialog(),
        panel: !!button(document, /^Buka Pemilih Tanggal/),
        picker: !!picker(),
        times: options().length,
        menu: [...document.querySelectorAll('[role=menuitem]')].filter(vis).map(name),
        confirm: dialogs().some((d) => !!button(d, 'Hapus')),
        rows: rows().length,
        tab: tab ? name(tab) : '',
        bsComposer: !!bsEditor(),
        bsDate: !!bsDate(),
      };
    },

    // ---- the profile and group composers ----
    // openComposer presses "Apa yang Anda pikirkan, …?" on the home page, or
    // "Tulis sesuatu..." in a group.
    openComposer: () => press(button(document, /^(Apa yang Anda pikirkan|Tulis sesuatu)/)),
    paste(text) {
      const ed = editor();
      if (!ed) return false;
      ed.focus();
      const data = new DataTransfer();
      data.setData('text/plain', text);
      ed.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
      return true;
    },
    text: () => editor()?.innerText ?? null,
    // audience is the composer's audience: "Audiens postingan Hanya saya"
    // gives "Hanya saya".
    audience() {
      const b = button(composer(), /^Audiens postingan/);
      return b ? name(b).replace(/^Audiens postingan\s*/, '') : null;
    },
    openAudience: () => press(button(composer(), /^Audiens postingan/)),
    // chooseAudience ticks the audience named label and says whether it took.
    chooseAudience(label) {
      const r = [...(audienceDialog()?.querySelectorAll('input[type=radio]') ?? [])].find((x) => textOf(x) === label);
      if (!r) return false;
      r.click();
      return r.checked;
    },
    audienceDone: () => press(button(audienceDialog(), 'Selesai')),
    stage(id, piece) {
      staged.set(id, (staged.get(id) ?? '') + piece);
      return staged.get(id).length;
    },
    // attach gives the composer the staged files through its file input, as
    // if they were chosen in its file dialog.
    attach(files) {
      const root = composer() ?? document;
      const input = root.querySelector('input[type=file]') ?? document.querySelector('input[type=file]');
      if (!input) return 'no file input';
      const list = new DataTransfer();
      for (const f of files) {
        list.items.add(new File([bytes(staged.get(f.id) ?? '')], f.name, { type: f.type }));
        staged.delete(f.id);
      }
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    media: () => (composer() ?? document).querySelectorAll('img[src^="blob:"], video').length,
    enabled(label) {
      const b = button(composer() ?? document, label);
      return !!b && b.getAttribute('aria-disabled') !== 'true';
    },
    submit: (label) => press(button(composer() ?? document, label)),

    // ---- the profile's schedule panel ----
    openSchedule: () => press(button(composer(), 'Jadwalkan')),
    openPicker: () => press(button(document, /^Buka Pemilih Tanggal/)),
    month() {
      let b = picker();
      for (let i = 0; i < 4 && b; i++) b = b.parentElement;
      return b ? b.innerText.split('\n')[0].trim() : null;
    },
    nextMonth: () => press(button(document, 'Bulan Berikutnya')),
    prevMonth: () => press(button(document, 'Bulan Sebelumnya')),
    // pickDay clicks the day labelled "Senin, 16 November 2026".
    pickDay(day) {
      const cell = [...(picker()?.querySelectorAll('[role=gridcell]') ?? [])].find((c) => c.innerText.trim().startsWith(day));
      if (!cell) return 'no such day';
      if (disabled(cell)) return 'that day cannot be chosen';
      cell.click();
      return '';
    },
    boxes: () => combos().map((b) => b.value),
    openTimes() {
      const t = combos()[1];
      if (!t) return false;
      t.click();
      t.focus();
      return true;
    },
    times: () => options().length,
    pickTime(hhmm) {
      const o = options().find((x) => x.innerText.trim() === hhmm);
      if (!o) return false;
      o.scrollIntoView({ block: 'center' });
      o.click();
      return true;
    },
    confirmSchedule: () => press(button(document, 'Jadwalkan untuk nanti')),
    // line is the composer's chosen time: "16 Nov pada 09.00".
    line: () => buttons(composer()).map(name).find((n) => /^\d{1,2} \S+ pada \d{1,2}\.\d{2}$/.test(n)) ?? '',
    toasts: () => [...document.querySelectorAll('[role=alert], [role=status]')].filter(vis).map((e) => e.innerText.trim()).filter(Boolean),

    // ---- the profile and its Content Library ----
    profile() {
      const a = profileLink();
      return a ? { name: name(a), url: new URL(a.getAttribute('href'), location.origin).href.split('?')[0] } : null;
    },
    openProfile: () => press(profileLink()),
    heading: () => [...document.querySelectorAll('h1')].filter(vis).map((h) => h.innerText.trim()).find(Boolean) ?? '',
    // link presses a link or button by its exact name, such as "Kelola postingan".
    link: (label) => press([...document.querySelectorAll('a, [role=link], [role=button]')].find((e) => vis(e) && name(e) === label)),
    openTab: (label) => press([...document.querySelectorAll('[role=tab]')].find((t) => vis(t) && name(t) === label)),
    rows: () => rows().map(({ text, status, when }) => ({ text, status, when })),
    rowMenu(i) {
      const r = rows()[i];
      return !!r && press(buttons(r.el).find((b) => name(b) === 'Tindakan untuk postingan ini'));
    },
    menu: (label) => press([...document.querySelectorAll('[role=menuitem]')].find((m) => vis(m) && name(m) === label)),
    confirm: (label) => press(dialogs().map((d) => button(d, label)).find(Boolean)),

    // ---- Pages and groups ----
    // pages lists the Pages the user manages (/pages/?category=your_pages).
    pages() {
      const main = document.querySelector('[role=main]') ?? document.body;
      const seen = new Map();
      for (const a of main.querySelectorAll('a[href]')) {
        const n = name(a);
        if (!vis(a) || !n || /^Foto profil untuk|^Pesan$|^Promosikan$/.test(n)) continue;
        const u = new URL(a.getAttribute('href'), location.origin);
        if (!/^\/[A-Za-z0-9.]+\/?$/.test(u.pathname) || reserved.test(u.pathname)) continue;
        seen.set(u.href.split('?')[0], n);
      }
      return [...seen].map(([url, n]) => ({ name: n, url }));
    },
    // pageId is the numeric id of the Page whose page this is: the first
    // delegate page id in it, which Business Suite calls the asset id.
    pageId: () => document.documentElement.outerHTML.match(/"delegate_page_id":"(\d+)"/)?.[1] ?? '',
    // groups lists the groups the user is in (/groups/joins/): each one with
    // a "Lihat Grup" link, which a pending request does not have.
    groups() {
      const main = document.querySelector('[role=main]') ?? document.body;
      const links = [...main.querySelectorAll('a[href*="/groups/"]')].filter(vis);
      const joined = new Set(links.filter((a) => name(a) === 'Lihat Grup').map((a) => a.href.split('?')[0]));
      const seen = new Map();
      for (const a of links) {
        const href = a.href.split('?')[0];
        const n = (a.innerText || '').trim().split('\n')[0];
        if (joined.has(href) && n && n !== 'Lihat Grup' && !seen.has(href)) seen.set(href, n);
      }
      return [...seen].map(([url, n]) => ({ name: n, url }));
    },

    // ---- Meta Business Suite ----
    // bsPage is the Page the composer posts as, from its Page box.
    bsPage: () => [...document.querySelectorAll('[role=combobox]')].filter((c) => vis(c) && c.tagName !== 'INPUT' && !c.isContentEditable).map(name).find(Boolean) ?? '',
    bsCreate: () => press(button(document, 'Buat postingan')),
    bsPaste(text) {
      const ed = bsEditor();
      if (!ed) return false;
      ed.focus();
      const data = new DataTransfer();
      data.setData('text/plain', text);
      ed.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
      return true;
    },
    bsText: () => bsEditor()?.innerText ?? null,
    // bsMedia opens "Tambahkan foto/video" so its file input exists (not run
    // yet: the first Page test was not allowed).
    bsMedia: () => press(button(document, 'Tambahkan foto/video')),
    bsSchedule() {
      const s = bsSwitch();
      if (!s) return null;
      if (s.getAttribute('aria-checked') !== 'true') (s.closest('label') ?? s).click();
      return s.getAttribute('aria-checked') === 'true';
    },
    // bsWhen types the date (dd/mm/yyyy) and the time into the schedule
    // fields, and returns what they then show: the date as "20 November
    // 2026", and the hour and minute.
    bsWhen(ddmmyyyy, hh, mm) {
      const d = bsDate();
      const h = spin('jam');
      const m = spin('menit');
      if (!d || !h || !m) return null;
      d.focus();
      d.select();
      document.execCommand('insertText', false, ddmmyyyy);
      d.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', bubbles: true }));
      d.blur();
      for (const [box, v] of [[h, hh], [m, mm]]) {
        box.focus();
        box.select();
        for (const ch of v) document.execCommand('insertText', false, ch);
        box.blur();
      }
      return { date: d.value, hour: h.getAttribute('aria-valuenow'), minute: m.getAttribute('aria-valuenow') };
    },
    bsShown: () => {
      const d = bsDate();
      const h = spin('jam');
      const m = spin('menit');
      return d && h && m ? { date: d.value, hour: h.getAttribute('aria-valuenow'), minute: m.getAttribute('aria-valuenow') } : null;
    },
  };
})();
