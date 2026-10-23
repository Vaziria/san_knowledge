// What san_remote knows of Vidara's pages, as found on 2026-10-24 with the
// interface in English: the dashboard (/index), Upload (/upload), the Export
// page an upload ends on, My videos (/files), Settings, a video's page
// (/v/<code>), and the player that page embeds from another host. Like
// threads.js, it runs in the extension's isolated world, where the page
// cannot see it, and stays there until the page is left: the Go side sends it
// with each call and it is set up only once per page.
//
// Vidara's pages are rendered on the server (Tailwind, Dropzone), so controls
// are found by id, by href and by title, and every link loads a new page. The
// waits and the pace between steps are in Go.
globalThis.__sanVidara = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const press = (e) => {
    if (!e) return false;
    e.click();
    return true;
  };
  const text = (e) => (e?.innerText ?? '').trim().replace(/\s+/g, ' ');
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  // The account's own pages have a Logout link in the header.
  const signedIn = () => !!document.querySelector('a[href="/logout"]');
  // A modal is shown when it has lost its "hidden" class.
  const shown = (id) => {
    const m = document.getElementById(id);
    return !!m && !m.classList.contains('hidden');
  };
  // A video's row in My videos: its checkbox carries the filecode.
  const rows = () => [...document.querySelectorAll('tr')].filter((tr) => tr.querySelector('.video-checkbox[data-filecode]'));
  const row = (code) => rows().find((tr) => tr.querySelector('.video-checkbox').dataset.filecode === code);
  const searchBox = () => [...document.querySelectorAll('input[name=search]')].find(vis);
  // Base64 both ways; Uint8Array has it built in from Chrome 140.
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };
  const base64 = (b) => {
    if (b.toBase64) return b.toBase64();
    let bin = '';
    for (let k = 0; k < b.length; k += 0x8000) bin += String.fromCharCode(...b.subarray(k, k + 0x8000));
    return btoa(bin);
  };
  // A file to upload is sent in base64 pieces. Each piece becomes a Blob at
  // once, so the bytes wait in the browser's blob store rather than in this
  // page's memory.
  const staged = new Map();
  // Downloaded segments wait here while Go reads them in pieces.
  const loaded = new Map();

  return {
    // state is what the page shows, for the waits.
    state() {
      return {
        url: location.href,
        path: location.pathname,
        signedIn: signedIn(),
        signedOut: !signedIn() && (location.pathname.startsWith('/login') || [...document.querySelectorAll('input[type=password]')].some(vis)),
        uploader: !!document.querySelector('input.dz-hidden-input') && !!document.getElementById('file-previews'),
        videos: [...document.querySelectorAll('h2')].some((h) => /^Videos \(\d+\)$/.test(text(h))),
        query: new URL(location.href).searchParams.get('search') ?? '',
        deleteAsk: shown('modal-delete-confirm'),
      };
    },
    // nav presses a link of the sidebar by its address: /upload, /files,
    // /settings.
    nav: (href) => press(document.querySelector(`#sidebar a[href="${href}"]`) ?? document.querySelector(`a[href="${href}"]`)),
    // username is the name Settings greets: "Hello, getresolved!".
    username() {
      const p = [...document.querySelectorAll('p')].find((e) => /^Hello, .+!$/.test(text(e)));
      return p ? text(p).slice('Hello, '.length, -1) : null;
    },
    // storage is the sidebar's "0.01 GB used", once it has loaded.
    storage() {
      const s = text(document.getElementById('storage-used'));
      return s && !s.startsWith('Loading') ? s : '';
    },

    // ---- Upload ----
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // choose gives the uploader the staged file through Dropzone's own file
    // input, as if it were chosen in Browse files. The page then sends it in
    // chunks, and names the video after the file.
    choose(f) {
      const input = document.querySelector('input.dz-hidden-input');
      if (!input) return 'no file input';
      const parts = staged.get(f.id);
      if (!parts) return 'nothing staged';
      const file = new File(parts, f.name, { type: f.type, lastModified: f.modified });
      staged.delete(f.id);
      if (file.size !== f.size) return `the page holds ${file.size} bytes of the ${f.size}`;
      const list = new DataTransfer();
      list.items.add(file);
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // progress reads the file's preview while it uploads: the bar, the speed
    // line ("Upload complete!" at the end), and Vidara's error, if any. The
    // uploader goes to the Export page when it is done.
    progress() {
      const p = document.querySelector('#file-previews [data-dz-preview]');
      return {
        path: location.pathname,
        preview: !!p,
        percent: parseFloat(p?.querySelector('[data-dz-uploadprogress]')?.style.width) || 0,
        speed: text(p?.querySelector('[data-dz-speed]')),
        left: text(p?.querySelector('[data-dz-time]')),
        error: text(p?.querySelector('[data-dz-errormessage]')) || text(document.getElementById('upload-message')),
      };
    },
    // exported reads the Export page: the video's link, and its title from
    // the HTML code.
    exported() {
      if (location.pathname !== '/export') return null;
      const url = (document.getElementById('textarea_download')?.value ?? '').trim().split('\n')[0];
      const html = document.getElementById('textarea_html')?.value ?? '';
      const a = new DOMParser().parseFromString(html, 'text/html').querySelector('a');
      return url ? { url, title: a?.textContent.trim() ?? '' } : null;
    },

    // ---- My videos ----
    // typeSearch types into the search box key by key, as a person does;
    // paced, with a person's gaps between the keys. A tab that is not shown
    // runs timers once a second, so it types without them: the box has no
    // key handlers, and the form sends only its final text.
    async typeSearch(q, paced) {
      const box = searchBox();
      if (!box) return null;
      box.focus();
      box.value = '';
      for (const ch of q) {
        box.value += ch;
        box.dispatchEvent(new InputEvent('input', { bubbles: true, data: ch, inputType: 'insertText' }));
        if (paced) await sleep(70 + Math.random() * 120);
      }
      return box.value;
    },
    // search presses the search box's button, which loads My videos again
    // with ?search=. It finds titles and filecodes.
    search: () => press(searchBox()?.form?.querySelector('button[type=submit]')),
    // total is the list's "(161 total)"; the page shows only its first
    // entries (100 here).
    total() {
      const e = [...document.querySelectorAll('*')].find((e) => e.children.length === 0 && /^\(\d+ total\)$/.test(e.textContent.trim()));
      return e ? parseInt(e.textContent.trim().slice(1), 10) : rows().length;
    },
    // rows reads the video rows, by the table's own column names.
    rows() {
      const head = [...(rows()[0]?.closest('table')?.querySelectorAll('thead th') ?? [])].map(text);
      const col = (tr, name) => text(tr.cells[head.indexOf(name)]);
      return rows().map((tr) => {
        const a = tr.querySelector('a[href*="/v/"]');
        return {
          code: tr.querySelector('.video-checkbox').dataset.filecode,
          title: text(a),
          url: a ? new URL(a.getAttribute('href'), location.origin).href : '',
          size: col(tr, 'Size'),
          views: col(tr, 'Views'),
          uploaded: col(tr, 'Uploaded'),
          status: col(tr, 'Status'),
        };
      });
    },
    // rowDelete presses the Delete button of the video's row, which asks
    // first.
    rowDelete: (code) => press(row(code)?.querySelector('button[title=Delete]')),
    // deleteList is what the confirmation says it will delete: `1 video: "…"`.
    deleteList: () => (shown('modal-delete-confirm') ? text(document.getElementById('delete-items-list')) : null),
    confirmDelete: () => press(document.getElementById('confirm-delete-btn')),

    // ---- a video's page and its player ----
    // player is the video's page: its title, and the address of the player
    // it embeds (another host, /e/<code>). The page's "Download file" button
    // is an ad and is never pressed.
    player(code) {
      const f = [...document.querySelectorAll('iframe')].find((f) => new URL(f.src, location.href).pathname === `/e/${code}`);
      return f ? { src: f.src, title: text(document.querySelector('main h1')) } : null;
    },
    // gone says whether the video's page says the video is not there.
    gone: () => /file (was )?(not found|deleted|removed)|video (not found|was deleted)/i.test(document.body?.innerText ?? ''),
    // stream is the HLS playlist the player loaded, from the page's resource
    // timing, or "".
    stream: () => performance.getEntriesByType('resource').map((e) => e.name).find((n) => /\.m3u8(\?|$)/.test(n)) ?? '',
    // playlist fetches a playlist's text, as the player does.
    async playlist(url) {
      const r = await fetch(url);
      if (!r.ok) throw new Error(`${url}: HTTP ${r.status}`);
      return await r.text();
    },
    // load fetches a segment and keeps it for piece; it says how many bytes
    // it has.
    async load(id, url) {
      const r = await fetch(url);
      if (!r.ok) throw new Error(`${url}: HTTP ${r.status}`);
      const b = new Uint8Array(await r.arrayBuffer());
      loaded.set(id, b);
      return b.length;
    },
    piece: (id, from, to) => base64(loaded.get(id).subarray(from, to)),
    drop: (id) => loaded.delete(id),
  };
})();
