// What san_remote knows of TikTok's pages, as found on 2026-10-25 with the
// interface in English (lang en, the account @getresolved.id): tiktok.com's
// side menu, TikTok Studio's upload page (/tiktokstudio/upload) and its Posts
// list (/tiktokstudio/content). Like x.js, it runs in the extension's isolated
// world, where the page cannot see it.
//
// What reaches Studio from here (2026-10-25):
//   - the video: the upload page's own file input (accept video/*), given the
//     file through a DataTransfer
//   - the caption (Draft.js): a paste event, which keeps line breaks; and a
//     Backspace keydown on a script-made selection. execCommand's insertText
//     doubled the text and broke the editor ("Something went wrong").
//   - a hashtag suggestion: a whole mouse press (mousedown, mouseup, click);
//     a plain click did nothing
//   - the schedule: Picker comboboxes "Select date" and "Select time"; the
//     date picker's days are buttons named "Tuesday, November 3rd, 2026", the
//     time picker's options "Hours 19" and "Minutes 00" (5-minute steps).
//     Escape closes the time picker.
//   - a row's "…" menu item: a whole mouse press on it
globalThis.__sanTikTok = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return false;
    for (let x = e; x && x !== document.body; x = x.parentElement) {
      const s = getComputedStyle(x);
      if (s.opacity === '0' || s.visibility === 'hidden') return false;
    }
    return true;
  };
  const first = (sel) => [...document.querySelectorAll(sel)].find(vis) ?? null;
  const all = (sel) => [...document.querySelectorAll(sel)].filter(vis);
  const text = (e) => (e ? (e.innerText || '').replace(/\s+/g, ' ').trim() : '');
  const own = (e) => (e ? (e.innerText || '').trim() : '');
  const press = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.click();
    return true;
  };
  // mouse presses e as a mouse does, for controls that act on mousedown.
  const mouse = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    const r = e.getBoundingClientRect();
    const o = { bubbles: true, cancelable: true, clientX: r.x + r.width / 2, clientY: r.y + r.height / 2, button: 0, view: window };
    e.dispatchEvent(new PointerEvent('pointerdown', { ...o, pointerType: 'mouse', isPrimary: true }));
    e.dispatchEvent(new MouseEvent('mousedown', o));
    e.dispatchEvent(new PointerEvent('pointerup', { ...o, pointerType: 'mouse', isPrimary: true }));
    e.dispatchEvent(new MouseEvent('mouseup', o));
    e.dispatchEvent(new MouseEvent('click', o));
    return true;
  };
  const keydown = (el, key, code) => el.dispatchEvent(new KeyboardEvent('keydown', { key, code: key, keyCode: code, which: code, bubbles: true, cancelable: true }));
  const editor = () => document.querySelector('.caption-editor .public-DraftEditor-content');
  // caret puts the caret at the end of the caption, where Draft.js reads it
  // from the selection.
  const caret = (ed) => {
    ed.focus();
    const r = document.createRange();
    r.selectNodeContents(ed);
    r.collapse(false);
    const s = getSelection();
    s.removeAllRanges();
    s.addRange(r);
  };
  const combo = (label) => document.querySelector(`[role=combobox][aria-label="${label}"]`);
  const radio = (value) => document.querySelector(`[data-e2e=schedule_container] input[name=postSchedule][value=${value}]`) ??
    [...document.querySelectorAll('[data-e2e=schedule_container] input[type=radio]')].find((i) => i.value === value);
  const datePanel = () => first('.DatePicker__panel');
  const timePanel = () => first('.TimePicker__panel');
  const rows = () => all('[data-tt=components_RowLayout_FlexRow]');
  const squash = (s) => s.replace(/\s+/g, '');
  const matching = (caption, when) => rows().filter((r) => squash(text(r)).includes(squash(caption)) && squash(text(r)).includes(squash(when)));
  // rowBox is the largest ancestor of a row that holds no other row: the
  // whole line of the list, actions included.
  const rowBox = (r) => {
    let box = r;
    for (let p = r.parentElement; p && p !== document.body; p = p.parentElement) {
      if (p.querySelectorAll('[data-tt=components_RowLayout_FlexRow]').length > 1) break;
      box = p;
    }
    return box;
  };
  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };

  return {
    // state is what the page shows, for the waits.
    state() {
      return {
        url: location.href,
        lang: document.documentElement.lang,
        home: !!first('[data-e2e=nav-upload]') && !!document.querySelector('[data-e2e=nav-profile]'),
        signin: /^\/login/.test(location.pathname) || !!first('[data-e2e=top-login-button], #header-login-button'),
        uploadPage: !!first('[data-e2e=select_video_button]'),
        form: !!editor(),
        crashed: /Something went wrong/.test(document.body?.innerText.slice(0, 2000) ?? '') && !editor() && !first('[data-e2e=select_video_button]'),
        content: /\/tiktokstudio\/content/.test(location.pathname),
      };
    },
    // account is the signed-in account, from the side menu's profile link.
    account() {
      const a = document.querySelector('[data-e2e=nav-profile]');
      const href = a?.getAttribute('href') ?? a?.closest('a')?.getAttribute('href') ?? a?.querySelector('a')?.getAttribute('href') ?? '';
      const m = href.match(/\/@([^/?#]+)/);
      return m ? decodeURIComponent(m[1]) : '';
    },
    pressUpload: () => press(first('[data-e2e=nav-upload] button') ?? first('[data-e2e=nav-upload]')),
    pressPostsLink: () => press(all('button[data-tt=Sidebar_Sidebar_Clickable], a').find((b) => /^Posts$/i.test(own(b)))),
    // A file to upload comes in base64 pieces; each becomes a Blob at once.
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // choose gives the upload page's file input the staged video, as if it
    // were chosen in Select video's file dialog. It returns "" or what went
    // wrong.
    choose(id, name, type) {
      const input = document.querySelector('input[type=file][accept*=video]') ?? document.querySelector('[data-e2e=select_video_container] input[type=file]');
      const parts = staged.get(id);
      if (!input) return 'no file input on the upload page';
      if (!parts) return 'nothing staged';
      staged.delete(id);
      const list = new DataTransfer();
      list.items.add(new File(parts, name, { type, lastModified: Date.now() }));
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // upload is the file's line above the form: its name and the progress,
    // or "Uploaded（62.88KB）"; done when Studio says it is uploaded.
    upload() {
      const t = text(document.querySelector('[data-e2e=upload_status_container]'));
      return { text: t, done: /\bUploaded\b/i.test(t) };
    },
    // gotIt closes an onboarding popover ("… Got it") over the form.
    gotIt: () => press(all('button, [role=button]').find((b) => /^Got it$/i.test(own(b)))),
    // editor: the caption's text (one line per block), its hashtags, and the
    // suggestion list.
    editor() {
      const e = editor();
      if (!e) return null;
      return {
        focused: document.activeElement === e,
        text: [...e.querySelectorAll('[data-block=true]')].map((b) => b.textContent).join('\n'),
        tags: [...e.querySelectorAll('[data-testid=mentionText]')].map((m) => m.textContent),
        list: all('.mention-list-popover').length > 0,
        items: all('.mention-list-popover .hashtag-suggestion-item').map((i) => own(i).split('\n')[0].trim()),
      };
    },
    // selectAll selects the whole caption, for a Backspace to empty it
    // (Studio puts the file's name there). Draft.js takes the selection on
    // its own a moment later: a Backspace right after it deleted one
    // character (2026-10-25).
    selectAll() {
      const e = editor();
      if (!e) return false;
      e.focus();
      const r = document.createRange();
      r.selectNodeContents(e);
      getSelection().removeAllRanges();
      getSelection().addRange(r);
      return true;
    },
    // paste pastes text at the end of the caption.
    paste(t) {
      const e = editor();
      if (!e) return false;
      if (document.activeElement !== e) caret(e);
      const dt = new DataTransfer();
      dt.setData('text/plain', t);
      const ev = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true });
      e.dispatchEvent(ev);
      return ev.defaultPrevented;
    },
    backspace() {
      const e = editor();
      return !!e && (keydown(e, 'Backspace', 8), true);
    },
    escape() {
      const e = editor();
      return !!e && (keydown(e, 'Escape', 27), true);
    },
    // pressSuggestion picks the hashtag suggestion that is exactly tag.
    pressSuggestion: (tag) => mouse(all('.mention-list-popover .hashtag-suggestion-item').find((i) => own(i).split('\n')[0].trim().toLowerCase() === tag.toLowerCase())),
    // schedule: when to post ("post_now" or "schedule"), the date ("Nov 3,
    // 2026") and time ("19:00") shown, the time zone box ("(GMT+7) Local
    // Time"), which picker is open, and Studio's note about the choice
    // ("Private videos can’t be scheduled.").
    schedule() {
      const sc = document.querySelector('[data-e2e=schedule_container]');
      const checked = [...(sc?.querySelectorAll('input[type=radio]') ?? [])].find((i) => i.checked || i.getAttribute('aria-checked') === 'true');
      const value = (l) => own(combo(l)?.querySelector('.Picker__value') ?? combo(l));
      return {
        mode: checked?.value ?? '',
        date: vis(combo('Select date')) ? value('Select date') : '',
        time: vis(combo('Select time')) ? value('Select time') : '',
        zone: sc?.querySelector('.timezone-select input')?.value ?? '',
        dateOpen: !!datePanel(),
        timeOpen: !!timePanel(),
        note: [...(sc?.querySelectorAll('*') ?? [])].filter((e) => e.children.length === 0 && /can.t be scheduled/i.test(own(e))).map(own)[0] ?? '',
      };
    },
    pressRadio: (value) => press(radio(value)?.closest('label')),
    pressDate: () => press(combo('Select date')),
    // month is the date picker's month and year, "November 2026".
    month: () => text(datePanel()?.querySelector('.DatePicker__title')),
    pressNextMonth: () => press([...(datePanel()?.querySelectorAll('.DatePicker__nav') ?? [])].find((b) => b.getAttribute('aria-label') === 'Next month')),
    // day is the date picker's button named label, and whether it can be
    // chosen; null when the month shown has no such day.
    day(label) {
      const b = datePanel()?.querySelector(`button.DatePicker__day[aria-label="${label}"]`);
      return b ? { enabled: !b.disabled && b.getAttribute('aria-disabled') !== 'true' } : null;
    },
    pressDay: (label) => press(datePanel()?.querySelector(`button.DatePicker__day[aria-label="${label}"]:not([disabled])`)),
    pressTime: () => press(combo('Select time')),
    // pressTimeOption presses the time picker's option: column "Hours" or
    // "Minutes", value "19" or "05".
    pressTimeOption: (col, v) => press(timePanel()?.querySelector(`[role=listbox][aria-label="${col}"] [role=option][aria-label="${col} ${v}"]`)),
    closeTime() {
      const p = timePanel();
      return !p || (keydown(p, 'Escape', 27), true);
    },
    // tzOffset is the browser's offset from UTC, in minutes east, at the
    // instant ms: the time zone Studio's Local Time is.
    tzOffset: (ms) => -new Date(ms).getTimezoneOffset(),
    // visibility: "Who can see this post", the label shown, and when its
    // list is open, the label of each value ("0" Everyone, "2" Friends, "1"
    // Only you).
    visibility() {
      const opts = all('[role=option][data-value]');
      const labels = {};
      opts.forEach((o) => { labels[JSON.parse(o.getAttribute('data-value'))] = own(o).split('\n')[0].trim(); });
      return { label: text(first('[data-e2e=video_visibility_container] .Select__trigger')), open: opts.length > 0, labels };
    },
    pressVisibility: () => press(first('[data-e2e=video_visibility_container] .Select__trigger')),
    pressVisibilityOption: (value) => press(first(`[role=option][data-value='"${value}"']`)),
    // postButton is the post button: "Post" or "Schedule", and whether it
    // can be pressed.
    postButton() {
      const b = first('[data-e2e=post_video_button]');
      return b ? { text: text(b), enabled: b.getAttribute('aria-disabled') !== 'true' && b.dataset.disabled !== 'true' && b.dataset.loading !== 'true' } : null;
    },
    pressPost: () => press(first('[data-e2e=post_video_button]')),
    // checks: Studio's checks (music copyright, content check lite) still
    // running. Post pressed meanwhile makes Studio ask "Continue to post?".
    checks: () => ({ pending: /Checking in progress/i.test(document.body.innerText) }),
    // modal is a modal Studio shows: its text and buttons.
    modal() {
      const m = first('.TUXModal');
      return m ? { text: text(m).slice(0, 300), buttons: all('.TUXModal button').map(text) } : null;
    },

    // ---- the Posts list ----
    posts: () => (/\/tiktokstudio\/content/.test(location.pathname) ? rows().map(text) : null),
    // hoverRow points the mouse at the one row holding caption and when, so
    // its actions show; it returns how many rows hold them.
    hoverRow(caption, when) {
      const m = matching(caption, when);
      if (m.length === 1) {
        const box = rowBox(m[0]);
        const r = box.getBoundingClientRect();
        for (const t of ['pointerover', 'pointerenter', 'mouseover', 'mouseenter', 'mousemove']) {
          box.dispatchEvent(new MouseEvent(t, { bubbles: true, clientX: r.x + r.width / 2, clientY: r.y + r.height / 2, view: window }));
        }
      }
      return m.length;
    },
    // pressRowMore opens the "…" menu of the one row holding caption and
    // when (Download, Delete for a scheduled post).
    pressRowMore(caption, when) {
      const m = matching(caption, when);
      return m.length === 1 && press(rowBox(m[0]).querySelector('button[data-tt=components_ActionCell_Clickable]'));
    },
    menu: () => all('[data-tt=components_ActionCell_FlexRow]').map(own),
    pressMenuItem: (label) => mouse(all('[data-tt=components_ActionCell_FlexRow]').find((e) => own(e) === label)),
    // confirm presses button in the dialog whose text starts with question
    // ("Delete post?").
    confirm(question, button) {
      const d = all('[role=dialog]').find((d) => own(d).startsWith(question) && [...d.querySelectorAll('button')].some((b) => own(b) === button));
      return press([...(d?.querySelectorAll('button') ?? [])].find((b) => vis(b) && own(b) === button));
    },
  };
})();
