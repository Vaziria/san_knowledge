// What san_remote knows of Meta Business Suite's pages for an Instagram
// account, as found on 2026-10-29 with the interface in Indonesian (lang id,
// signed in with Instagram as @archetipe__, a creator account): the home
// (/latest/home, "Buat postingan"), the post composer (/latest/composer/)
// and the Content page's tabs (/latest/posts/scheduled_posts). Like
// instagram.js, it runs in the extension's isolated world.
//
// What reaches Business Suite from here (2026-10-29):
//   - the files: "Tambahkan foto/video" makes a file input in code (accept
//     .jpg,.png,.jpeg,.bmp,.tif,.tiff,video/*, multiple) and calls its
//     click(), which opens no dialog for a script. A patch made in the page's
//     own world just before (catchFiles in service.go) marks that input and
//     puts it in the document, and attach() gives it the files at once: given
//     them later, after the window had lost and won the focus, it took none.
//     Each file it takes gets a row with "Edit foto", "Edit" and "Hapus foto"
//     (a video "Hapus video").
//   - the caption: a paste event into "Tulis ke dalam kotak dialog…",
//     blurred after, as on instagram.com.
//   - the schedule: the switch "Atur tanggal dan waktu", then a date box
//     (dd/mm/yyyy, shown as "30 Oktober 2026") and spinbuttons jam and menit,
//     typed with insertText as facebook.js does for a Page.
// With them, Jadwalkan sent BusinessComposerStoryCreationMutation with the
// caption, the photo, channels INSTAGRAM_POST, share_to_threads false and
// scheduled_publish_time (seen with the request blocked).
globalThis.__sanInstagramBS = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const name = (e) => (e.getAttribute('aria-label') || e.innerText || '').replace(/​/g, '').trim().replace(/\s+/g, ' ');
  const buttons = (root) => [...(root ?? document).querySelectorAll('[role=button]')].filter(vis);
  const button = (root, label) => buttons(root).filter((b) => name(b) === label).pop() ?? null;
  const press = (e) => {
    if (!e) return false;
    e.click();
    return true;
  };
  const loginPage = () => /\/business\/loginpage/.test(location.pathname) || /\/login/.test(location.pathname);
  const editor = () => [...document.querySelectorAll('[contenteditable=true]')].find((e) => vis(e) && /^Tulis ke dalam kotak dialog/.test(e.getAttribute('aria-label') || ''));
  const mediaCard = () => {
    let c = button(document, 'Tambahkan foto/video');
    for (let k = 0; k < 10 && c && !/^Media/.test((c.innerText || '').trim()); k++) c = c.parentElement;
    return c && /^Media/.test((c.innerText || '').trim()) ? c : null;
  };
  const rows = () => buttons(mediaCard()).filter((b) => /^Hapus (foto|video)$/.test(name(b)));
  const scheduleSwitch = () => [...document.querySelectorAll('[role=switch]')].find((s) => vis(s) && name(s) === 'Atur tanggal dan waktu');
  // on reads a switch by its checked property too: its aria-checked once
  // read false while it showed on (2026-10-29).
  const on = (s) => !!s && (s.checked === true || s.getAttribute('aria-checked') === 'true');
  const dateBox = () => [...document.querySelectorAll('input[placeholder="dd/mm/yyyy"]')].find(vis);
  const spin = (label) => [...document.querySelectorAll('input[role=spinbutton]')].find((i) => vis(i) && i.getAttribute('aria-label') === label);
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
    // state is what the page shows, for the waits.
    state: () => ({
      url: location.href,
      lang: document.documentElement.lang,
      signedOut: loginPage(),
      signedIn: !loginPage() && !!button(document, 'Buat postingan'),
      composer: !!editor(),
      fields: !!dateBox() && !!spin('jam') && !!spin('menit'),
      scheduledTab: /\/latest\/posts\/scheduled_posts/.test(location.pathname),
    }),
    // account is the Instagram account the home shows: "@archetipe__"
    // gives "archetipe__".
    account() {
      const t = [...document.querySelectorAll('span, div')].find((e) => vis(e) && e.children.length === 0 && /^@[\w.]+$/.test((e.textContent || '').trim()));
      return t ? t.textContent.trim().slice(1) : '';
    },
    // assetId is the account's asset id, from the address Business Suite
    // goes to.
    assetId: () => new URL(location.href).searchParams.get('asset_id') ?? '',
    pressCreate: () => press(button(document, 'Buat postingan')),
    // composer: who it posts to ("Posting ke"), and where the media card says
    // the post goes ("Bagikan foto dan video di Instagram. …").
    composer() {
      const to = [...document.querySelectorAll('[role=combobox]')].filter((c) => vis(c) && c.tagName !== 'INPUT' && !c.isContentEditable).map(name).find(Boolean) ?? '';
      const card = mediaCard();
      return { to, media: card ? (card.innerText.split('\n')[1] ?? '').trim() : '' };
    },
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // attach presses "Tambahkan foto/video" and gives the file input it makes
    // (marked by the page-world patch) the staged files at once. It returns
    // "" or what went wrong.
    async attach(files) {
      document.querySelectorAll('input[type=file][data-san-file]').forEach((i) => i.removeAttribute('data-san-file'));
      const list = new DataTransfer();
      for (const f of files) {
        const parts = staged.get(f.id);
        if (!parts) return 'nothing staged for ' + f.name;
        list.items.add(new File(parts, f.name, { type: f.type, lastModified: Date.now() }));
      }
      if (!press(button(document, 'Tambahkan foto/video'))) return 'no Tambahkan foto/video';
      let input = null;
      for (let k = 0; k < 20 && !input; k++) {
        await sleep(50);
        input = document.querySelector('input[type=file][data-san-file]');
      }
      if (!input) return 'Tambahkan foto/video made no file input';
      for (const f of files) staged.delete(f.id);
      input.files = list.files;
      // Business Suite reads the files on input: change alone took none.
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // media is how many files the post holds: the media card's rows.
    media: () => rows().length,
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
    blurCaption() {
      const e = editor();
      return !!e && (e.blur(), true);
    },
    caption() {
      const e = editor();
      return e ? { text: e.innerText.replace(/\n$/, ''), focused: document.activeElement === e } : null;
    },
    // schedule turns "Atur tanggal dan waktu" on, and says whether it is on;
    // null when there is no such switch.
    schedule() {
      const s = scheduleSwitch();
      if (!s) return null;
      if (!on(s)) (s.closest('label') ?? s).click();
      return on(s);
    },
    scheduled: () => on(scheduleSwitch()),
    // when types the date (dd/mm/yyyy) and the time into the schedule
    // fields.
    when(ddmmyyyy, hh, mm) {
      const d = dateBox();
      const h = spin('jam');
      const m = spin('menit');
      if (!d || !h || !m) return false;
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
      return true;
    },
    // shown is what the schedule fields show: the date as "30 Oktober 2026",
    // the hour and the minute.
    shown() {
      const d = dateBox();
      const h = spin('jam');
      const m = spin('menit');
      return d && h && m ? { date: d.value, hour: h.getAttribute('aria-valuenow'), minute: m.getAttribute('aria-valuenow') } : null;
    },
    // submit is the composer's main button (Terbitkan, or Jadwalkan once the
    // switch is on), and whether it can be pressed.
    submit() {
      const b = buttons(document).filter((x) => /^(Terbitkan|Jadwalkan)$/.test(name(x))).pop();
      return b ? { text: name(b), enabled: b.getAttribute('aria-disabled') !== 'true' } : null;
    },
    pressSubmit: (label) => press(button(document, label)),
    // notices are what Business Suite says in alerts and dialogs.
    notices: () => [...document.querySelectorAll('[role=alert], [role=alertdialog], [role=status]')].filter(vis).map((e) => e.innerText.trim()).filter(Boolean),

    // ---- the Content page's Dijadwalkan tab (2026-10-29) ----
    // scheduledList is the tab's rows, or null off the tab: each row's caption,
    // whole, from its checkbox ('Centang item dengan keterangan "…"'), its
    // type ("Foto ·" under the caption) and when it goes out, as the date
    // cell shows it ("26 November 09:00"). empty is the tab saying "Tidak Ada
    // Postingan Terjadwal".
    scheduledList() {
      if (!/\/latest\/posts\/scheduled_posts/.test(location.pathname)) return null;
      const rows = [...document.querySelectorAll('[role=row]')].flatMap((r) => {
        const box = r.querySelector('input[type=checkbox][aria-label^="Centang item"]');
        if (!box) return [];
        const m = (box.getAttribute('aria-label') || '').match(/^Centang item dengan keterangan "([\s\S]*)"$/);
        const cells = [...r.querySelectorAll('[role=cell], [role=gridcell]')].map((c) => (c.innerText || '').replace(/​/g, '').trim());
        const when = cells.find((c) => /^\d{1,2} \S+( \d{4})? \d{1,2}[.:]\d{2}$/.test(c)) ?? '';
        const type = ((r.innerText || '').match(/\n(\S+)\s*·/) ?? [])[1] ?? '';
        return [{ caption: m ? m[1] : '', type, when }];
      });
      return { rows, empty: /Tidak Ada Postingan Terjadwal/.test(document.body.innerText) };
    },
    // Deleting a scheduled post (2026-10-29): its row's checkbox opens the
    // Detail Postingan panel, whose Tindakan menu has Hapus postingan, then
    // "Hapus Postingan?" Hapus. The row's own menu (Buka Menu Pilihan) has
    // only Kelola postingan and Salin ID postingan; the panel also has
    // Terbitkan sekarang, which publishes at once and is never pressed.
    // tickRow ticks the checkbox of row i (as scheduledList lists them).
    tickRow(i) {
      const boxes = [...document.querySelectorAll('[role=row] input[type=checkbox][aria-label^="Centang item"]')];
      const b = boxes[i];
      if (!b) return false;
      if (!b.checked) b.click();
      return true;
    },
    // ticked is the captions of the rows ticked, from their checkboxes.
    ticked: () =>
      [...document.querySelectorAll('[role=row] input[type=checkbox][aria-label^="Centang item"]')]
        .filter((b) => b.checked)
        .map((b) => ((b.getAttribute('aria-label') || '').match(/^Centang item dengan keterangan "([\s\S]*)"$/) ?? [])[1] ?? ''),
    // panel is the Detail Postingan panel's text, or "" when it is not open.
    // Its preview loads late ("Memuat cuplikan"), so the post is told by the
    // row ticked, not by the panel's text.
    panel() {
      const t = [...document.querySelectorAll('div')].filter((d) => vis(d) && /^Detail Postingan\n/.test(d.innerText || '')).pop();
      return t ? t.innerText.replace(/​/g, '') : '';
    },
    pressActions: () => press(button(document, 'Tindakan')),
    // menuItem presses the open menu's item named label.
    menuItem(label) {
      const item = [...document.querySelectorAll('[role=menu] *')].filter(vis).find((e) => name(e) === label && (e.getAttribute('role') === 'menuitem' || e.hasAttribute('tabindex')));
      return press(item);
    },
    menuItems: () => [...document.querySelectorAll('[role=menu]')].filter(vis).flatMap((m) => (m.innerText || '').replace(/​/g, '').split('\n')).map((s) => s.trim()).filter(Boolean),
    // confirm presses label in the dialog that asks question ("Hapus
    // Postingan?").
    confirm(question, label) {
      const d = [...document.querySelectorAll('[role=dialog], [role=alertdialog]')].filter(vis).find((x) => (x.innerText || '').trim().startsWith(question));
      return press(d && button(d, label));
    },
  };
})();
