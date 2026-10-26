// What san_remote knows of YouTube's pages: youtube.com's masthead (Create,
// Upload video, the account menu) and YouTube Studio's upload dialog and
// content list. The selectors are golang/packages/san_youtube's (studio.js
// and upload.go, found 2026-10-13 with chromedp), with the reading kept and
// the doing moved here, since the extension clicks and types from the page
// (2026-10-26). Like tiktok.js, it runs in the extension's isolated world,
// where the page cannot see it. Ids and names are used rather than labels,
// so the language matters only for the schedule's date and time.
//
// What reaches Studio from here:
//   - the video: the upload dialog's own file input, given the file through
//     a DataTransfer (the dialog's Select files opens a file chooser a page
//     cannot answer)
//   - the title and description (contenteditable #textbox): selected, then
//     execCommand insertText
//   - tags: inserted in the chip bar's input with commas, then Enter
//   - the schedule's date and time inputs: selected, insertText, Enter
//   - buttons and radio buttons: a click
globalThis.__sanYouTube = (() => {
  // Shown: laid out, and not faded out. After Save, Studio fades the upload
  // dialog to opacity 0 and leaves it laid out (2026-10-13).
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
  const text = (e) => (e ? (e.innerText || '').replace(/\s+/g, ' ').trim() : '');
  const press = (e) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.click();
    return true;
  };
  const keydown = (el, key, code) => {
    for (const type of ['keydown', 'keypress', 'keyup']) {
      el.dispatchEvent(new KeyboardEvent(type, { key, code: key, keyCode: code, which: code, bubbles: true, cancelable: true, composed: true }));
    }
  };
  const dialog = () => (first('ytcp-uploads-dialog tp-yt-paper-dialog') ? document.querySelector('ytcp-uploads-dialog') : null);

  // The elements the steps use, as san_youtube's upload.go names them.
  const sel = {
    create: 'ytd-masthead #end #buttons ytd-button-renderer button',
    uploadItem: 'ytd-compact-link-renderer a#endpoint[href$="/upload"]',
    avatar: 'ytd-masthead #avatar-btn',
    selectFiles: 'ytcp-uploads-dialog #select-files-button',
    title: 'ytcp-uploads-dialog #title-textarea #textbox',
    description: 'ytcp-uploads-dialog #description-textarea #textbox',
    notForKids: 'ytcp-uploads-dialog tp-yt-paper-radio-button[name=VIDEO_MADE_FOR_KIDS_NOT_MFK]',
    showMore: 'ytcp-uploads-dialog #toggle-button',
    tags: 'ytcp-uploads-dialog #chip-bar input#text-input',
    next: 'ytcp-uploads-dialog #next-button',
    private: 'ytcp-uploads-dialog tp-yt-paper-radio-button[name=PRIVATE]',
    schedule: 'ytcp-uploads-dialog #second-container-expand-button',
    date: 'ytcp-uploads-dialog ytcp-visibility-scheduler #datepicker-trigger',
    dateInput: 'ytcp-date-picker tp-yt-paper-input#textbox input',
    time: 'ytcp-uploads-dialog ytcp-visibility-scheduler #time-of-day-container input',
    done: 'ytcp-uploads-dialog #done-button',
    shortsTab: 'tp-yt-paper-tab#video-list-shorts-tab',
    videosTab: 'tp-yt-paper-tab#video-list-uploads-tab',
  };

  // replace selects what the field holds and types t over it as one
  // insertion, as an input method or a paste would.
  const replace = (e, t) => {
    if (!e) return false;
    e.scrollIntoView({ block: 'center' });
    e.focus();
    if (e.select) {
      e.select();
    } else {
      const r = document.createRange();
      r.selectNodeContents(e);
      getSelection().removeAllRanges();
      getSelection().addRange(r);
    }
    if (!document.execCommand('insertText', false, t)) {
      // An input that refuses the command takes the value and an input event.
      if ('value' in e) {
        e.value = t;
        e.dispatchEvent(new Event('input', { bubbles: true, composed: true }));
      } else {
        e.textContent = t;
        e.dispatchEvent(new InputEvent('input', { bubbles: true, composed: true, inputType: 'insertText', data: t }));
      }
    }
    return true;
  };

  // rowOf is the content list's row of video id.
  const rowOf = (id) => {
    const rows = [...document.querySelectorAll('ytcp-video-row')];
    return rows.find((r) => r.querySelector(`a[href*="/video/${id}/"]`)) || rows.find((r) => r.outerHTML.includes(id)) || null;
  };
  // confirmDialog is the shown dialog with a button named label.
  const confirmButton = (d, label) => d && [...d.querySelectorAll('ytcp-button, button, [role=button]')].filter(vis).find((b) => text(b) === label);
  const confirmDialog = (label) => [...document.querySelectorAll('tp-yt-paper-dialog, ytcp-dialog, [role=dialog]')].filter(vis).find((d) => confirmButton(d, label)) ?? null;
  // visPopup is a row's shown visibility choice,
  // ytcp-video-visibility-edit-popup (2026-10-27): "Save or publish"
  // (#first-container, its radio buttons PRIVATE, UNLISTED and PUBLIC drawn
  // only once it is expanded) and "Schedule" (#second-container), then
  // Cancel and #save-button ("Save", or "Schedule").
  const visPopup = () => [...document.querySelectorAll('ytcp-video-visibility-edit-popup')].find((p) => vis(p.querySelector('tp-yt-paper-dialog'))) ??
    [...document.querySelectorAll('tp-yt-paper-dialog, ytcp-dialog, [role=dialog]')].filter(vis)
      .find((d) => !d.closest('ytcp-uploads-dialog') && d.querySelector('tp-yt-paper-radio-button[name=PRIVATE]')) ?? null;
  const saveButton = (p) => p && ([...p.querySelectorAll('#save-button')].find(vis) ?? [...p.querySelectorAll('ytcp-button, button')].filter(vis).find((b) => text(b) === 'Save'));

  const staged = new Map();
  const bytes = (b64) => {
    if (Uint8Array.fromBase64) return Uint8Array.fromBase64(b64);
    const bin = atob(b64);
    const out = new Uint8Array(bin.length);
    for (let k = 0; k < bin.length; k++) out[k] = bin.charCodeAt(k);
    return out;
  };

  return {
    // state is where the tab is, for the waits.
    state() {
      return {
        url: location.href,
        lang: document.documentElement.lang,
        signin: location.host === 'accounts.google.com' ||
          !!first('ytd-masthead a[href*="accounts.google.com/ServiceLogin"]'),
        // youtube.com's home page signed in: the account button and Create.
        home: location.host === 'www.youtube.com' && !!first(sel.avatar) && !!first(sel.create),
        studio: location.host === 'studio.youtube.com',
        createMenu: !!first(sel.uploadItem),
        dialog: !!first(sel.selectFiles) || !!first(sel.title),
        selectFiles: !!first(sel.selectFiles),
        list: !!first(sel.videosTab),
      };
    },

    pressCreate: () => press(first(sel.create)),
    pressUploadItem: () => press(first(sel.uploadItem)),
    pressAvatar: () => press(first(sel.avatar)),
    // account is the account menu's name and @handle, once it is open.
    account() {
      const h = first('ytd-active-account-header-renderer');
      if (!h) return null;
      return { name: text(h.querySelector('#account-name')), handle: text(h.querySelector('#channel-handle')) };
    },
    closeMenu() {
      keydown(document.activeElement || document.body, 'Escape', 27);
      return true;
    },
    // channel is Studio's channel name, from its left menu.
    channel: () => text(document.querySelector('#entity-name')),

    // A file to upload comes in base64 pieces; each becomes a Blob at once.
    stage(id, piece) {
      const parts = staged.get(id) ?? [];
      parts.push(new Blob([bytes(piece)]));
      staged.set(id, parts);
      return parts.length;
    },
    // choose gives the upload dialog's file input the staged video, as if it
    // were chosen with Select files. It returns "" or what went wrong.
    choose(id, name, type) {
      const d = document.querySelector('ytcp-uploads-dialog');
      const input = d?.querySelector('input[type=file]') ?? document.querySelector('ytcp-uploads-file-picker input[type=file]');
      const parts = staged.get(id);
      if (!input) return 'no file input in the upload dialog';
      if (!parts) return 'nothing staged';
      staged.delete(id);
      const list = new DataTransfer();
      list.items.add(new File(parts, name, { type, lastModified: Date.now() }));
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true, composed: true }));
      return '';
    },

    // link is the video's link, which the dialog shows as soon as Studio has
    // made the video: https://youtube.com/shorts/<id> or https://youtu.be/<id>.
    link() {
      const d = dialog();
      if (!d) return '';
      const a = [...d.querySelectorAll('a[href]')].find((a) => /^https:\/\/(youtube\.com\/shorts\/|youtu\.be\/)/.test(a.href));
      return a ? a.href : '';
    },

    // details is what the details step holds.
    details() {
      const d = dialog();
      if (!d) return null;
      const box = (s) => {
        const e = d.querySelector(s);
        return e ? e.innerText.replace(/\n$/, '') : null;
      };
      const kids = d.querySelector('tp-yt-paper-radio-button[name=VIDEO_MADE_FOR_KIDS_NOT_MFK]');
      return {
        title: box('#title-textarea #textbox'),
        description: box('#description-textarea #textbox'),
        notForKids: !!kids && kids.classList.contains('iron-selected'),
        tags: [...d.querySelectorAll('#chip-bar ytcp-chip')].map((c) => c.innerText.trim()),
        tagsShown: !!first(sel.tags),
      };
    },
    typeTitle: (t) => replace(first(sel.title), t),
    typeDescription: (t) => replace(first(sel.description), t),
    pressNotForKids: () => press(first(sel.notForKids)),
    pressShowMore: () => press(first(sel.showMore)),
    // typeTags puts the tags in the chip bar's box with commas between them;
    // Enter turns them into chips.
    typeTags(tags) {
      const e = first(sel.tags);
      if (!e) return false;
      replace(e, tags.join(','));
      keydown(e, 'Enter', 13);
      return true;
    },
    pressNext: () => press(first(sel.next)),

    // visibility is the visibility step: which choice is selected, and the
    // schedule; null before that step.
    visibility() {
      const d = dialog();
      if (!d || !first(sel.done)) return null;
      const radio = [...d.querySelectorAll('tp-yt-paper-radio-button')].find((r) => vis(r) && r.classList.contains('iron-selected'));
      const s = first('ytcp-uploads-dialog ytcp-visibility-scheduler');
      return {
        selected: radio ? radio.getAttribute('name') : '',
        privateShown: !!first(sel.private),
        scheduleShown: !!s,
        date: s ? text(s.querySelector('#datepicker-trigger')) : '',
        time: s ? (s.querySelector('#time-of-day-container input') || {}).value || '' : '',
        timeInvalid: !!s && !!s.querySelector('#time-of-day-container tp-yt-paper-input[invalid]'),
        datePickerOpen: !!first('ytcp-scrollable-calendar'),
        timeListOpen: [...document.querySelectorAll('tp-yt-paper-item')].filter(vis).some((e) => /^\d{1,2}:\d{2}\s?[AP]M$/.test(e.innerText.replace(/[  ]/g, ' ').trim())),
        done: text(d.querySelector('#done-button')),
        doneDisabled: !!d.querySelector('#done-button[aria-disabled=true]'),
      };
    },
    pressPrivate: () => press(first(sel.private)),
    pressSchedule: () => press(first(sel.schedule)),
    pressDate: () => press(first(sel.date)),
    // The date picker is a scrolling calendar, ytcp-scrollable-calendar:
    // months div.calendar-month[aria-label="Nov 2026"], days
    // span.calendar-day (.disabled before today); a click on a day sets the
    // date and closes it. A date typed in its box with an Enter from the page
    // did not take (2026-10-27).
    //
    // pickDay picks day of month ("Nov 2026"): "picked", "scrolled" when
    // the month was not drawn yet (the list draws a few at a time), or
    // "disabled".
    pickDay(month, day) {
      const cal = first('ytcp-scrollable-calendar');
      if (!cal) return 'closed';
      const m = [...cal.querySelectorAll('.calendar-month')].find((x) => x.getAttribute('aria-label') === month);
      if (!m) {
        const list = cal.querySelector('#calendar-main');
        let sc = list;
        for (let x = list; x && x !== cal.parentElement; x = x.parentElement) {
          if (x.scrollHeight > x.clientHeight + 4) { sc = x; break; }
        }
        sc.scrollTop += 180;
        return 'scrolled';
      }
      const d = [...m.querySelectorAll('span.calendar-day')].find((s) => s.innerText.trim() === String(day));
      if (!d) return 'none';
      if (d.classList.contains('disabled')) return 'disabled';
      return press(d) ? 'picked' : 'none';
    },
    // The time is a list of 96 times, a quarter hour apart (tp-yt-paper-item
    // "7:00 PM"), opened by a click on the time box; a click on one sets it
    // (2026-10-27).
    pressTime: () => press(first(sel.time)),
    pickTime(label) {
      const norm = (s) => s.replace(/[  ]/g, ' ').trim();
      return press([...document.querySelectorAll('tp-yt-paper-item')].filter(vis).find((e) => norm(e.innerText) === label));
    },
    pressDone: () => press(first(sel.done)),

    // tzOffset is the browser's offset from UTC, in minutes east, at the
    // instant ms: the time zone Studio's "Local Time" schedules in.
    tzOffset: (ms) => -new Date(ms).getTimezoneOffset(),

    // progress is Studio's progress line at the dialog's foot: "Uploading
    // 45% … 2 minutes left", then "Upload complete … Processing".
    progress() {
      const d = dialog();
      return d ? text(d.querySelector('ytcp-video-upload-progress')) : '';
    },
    dialogOpen: () => !!dialog(),
    // formGone: Save has taken effect.
    formGone: () => !first(sel.done),
    // afterDialog is a dialog Studio shows after saving, such as "Video
    // processing" or "Video scheduled": its text; pressAfterDialog closes it.
    afterDialog() {
      const b = first('tp-yt-paper-dialog #close-button');
      return b ? text(b.closest('tp-yt-paper-dialog')).slice(0, 120) : null;
    },
    pressAfterDialog: () => press(first('tp-yt-paper-dialog #close-button')),
    // dialogs are the shown dialogs, for an error message.
    dialogs() {
      return [...document.querySelectorAll('tp-yt-paper-dialog')].filter(vis).map((d) => {
        const host = d.getRootNode().host || d.parentElement;
        return `${host ? host.tagName.toLowerCase() : ''} > ${text(d).slice(0, 80)}`;
      });
    },
    // pressClose presses the upload dialog's close (X) button.
    pressClose() {
      const d = document.querySelector('ytcp-uploads-dialog');
      if (!d) return false;
      const c = [...d.querySelectorAll('[id*=close i], [aria-label]')].filter(vis)
        .find((e) => /close/i.test(e.id) || /^close$/i.test(e.getAttribute('aria-label') || ''));
      return press(c);
    },

    // ---- the content list ----
    pressShortsTab: () => press(first(sel.shortsTab)),
    pressVideosTab: () => press(first(sel.videosTab)),
    // row is the open tab's row of video id: its title, visibility and date
    // as Studio shows them; null when it is not listed.
    row(id) {
      const rows = [...document.querySelectorAll('ytcp-video-row')];
      const r = rows.find((r) => r.querySelector(`a[href*="/video/${id}/"]`)) || rows.find((r) => r.outerHTML.includes(id));
      if (!r) return null;
      return {
        title: text(r.querySelector('#video-title')),
        visibility: text(r.querySelector('.tablecell-visibility')),
        date: text(r.querySelector('.tablecell-date')),
      };
    },
    // ---- a row's actions: delete, make private ----
    // The row's Options (⋮), its visibility cell and the dialogs they open
    // were not seen by san_youtube; they are found by their roles and
    // English labels (2026-10-26), and checked at each step.
    //
    // hoverRow points the mouse at video id's row, so its actions show; it
    // says whether the row is there.
    // The row renders its actions (Details, Analytics, Comments, View on
    // YouTube, Options) only once a mouseover reaches its inner elements;
    // one on the row itself renders nothing (2026-10-27).
    hoverRow(id) {
      const r = rowOf(id);
      if (!r) return false;
      r.scrollIntoView({ block: 'center' });
      const b = r.getBoundingClientRect();
      const o = { bubbles: true, composed: true, clientX: b.x + b.width / 2, clientY: b.y + b.height / 2, view: window };
      for (const e of [r, ...r.querySelectorAll('*')]) {
        e.dispatchEvent(new MouseEvent('mouseover', o));
      }
      r.dispatchEvent(new MouseEvent('mousemove', o));
      return true;
    },
    // pressRowOptions opens the row's Options menu (⋮): an
    // ytcp-icon-button named Options, there once the row is hovered.
    pressRowOptions: (id) => press(rowOf(id)?.querySelector('ytcp-icon-button[aria-label="Options"]')),
    // menu is the open menu's items, as they read: "Edit title and
    // description", …, "Delete forever" (test-id delete).
    menu: () => [...document.querySelectorAll('tp-yt-paper-item, [role=menuitem]')].filter(vis).map((e) => text(e)),
    pressMenuItem: (label) => press([...document.querySelectorAll('tp-yt-paper-item, [role=menuitem]')].filter(vis).find((e) => text(e) === label)),
    // confirm is the shown dialog that asks about label ("Delete forever"):
    // its text, whether its checkbox ("I understand…") is ticked, and
    // whether its button can be pressed; null when none is shown.
    confirm(label) {
      const d = confirmDialog(label);
      if (!d) return null;
      const box = d.querySelector('ytcp-checkbox-lit, tp-yt-paper-checkbox, [role=checkbox]');
      const btn = confirmButton(d, label);
      return {
        text: text(d).slice(0, 200),
        hasCheckbox: !!box,
        checked: !!box && (box.getAttribute('aria-checked') === 'true' || box.hasAttribute('checked') || !!box.querySelector('[aria-checked=true]')),
        enabled: !!btn && btn.getAttribute('aria-disabled') !== 'true' && !btn.hasAttribute('disabled'),
      };
    },
    pressConfirmCheckbox(label) {
      const d = confirmDialog(label);
      const box = d?.querySelector('ytcp-checkbox-lit, tp-yt-paper-checkbox, [role=checkbox]');
      return press(box?.querySelector('[role=checkbox]') ?? box);
    },
    pressConfirmButton: (label) => press(confirmButton(confirmDialog(label), label)),
    // pressVisibilityCell opens the row's visibility choice (Public ▾).
    pressVisibilityCell(id) {
      const c = rowOf(id)?.querySelector('.tablecell-visibility');
      return press(c && ([...c.querySelectorAll('[role=button], ytcp-dropdown-trigger, #trigger, button')].find(vis) ?? c));
    },
    // visibilityPopup is the open visibility choice of a row: which radio
    // button is selected, and its Save; null when it is not open.
    visibilityPopup() {
      const p = visPopup();
      if (!p) return null;
      const radio = [...p.querySelectorAll('tp-yt-paper-radio-button')].find((r) => vis(r) && r.classList.contains('iron-selected'));
      const save = saveButton(p);
      return {
        selected: radio ? radio.getAttribute('name') : '',
        privateShown: vis(p.querySelector('tp-yt-paper-radio-button[name=PRIVATE]')),
        // "Save or publish" can be expanded (a scheduled video's popup
        // opens on Schedule).
        expandable: vis(p.querySelector('#first-container-expand-button')),
        saveLabel: save ? text(save) : '',
        saveEnabled: !!save && save.getAttribute('aria-disabled') !== 'true' && !save.hasAttribute('disabled'),
      };
    },
    pressPopupExpand: () => press(visPopup()?.querySelector('#first-container-expand-button')),
    pressPopupPrivate: () => press(visPopup()?.querySelector('tp-yt-paper-radio-button[name=PRIVATE]')),
    pressPopupSave: () => press(saveButton(visPopup())),

    // rows are the open tab's rows, as row reads one, with each video's id.
    rows() {
      return [...document.querySelectorAll('ytcp-video-row')].map((r) => {
        const a = r.querySelector('a[href*="/video/"]');
        const m = a ? a.getAttribute('href').match(/\/video\/([A-Za-z0-9_-]{11})\//) : null;
        return {
          id: m ? m[1] : '',
          title: text(r.querySelector('#video-title')),
          visibility: text(r.querySelector('.tablecell-visibility')),
          date: text(r.querySelector('.tablecell-date')),
        };
      });
    },
  };
})();
