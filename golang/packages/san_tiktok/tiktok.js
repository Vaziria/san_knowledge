// Helpers the tool runs in TikTok's pages, as window.__sanTikTok. They read
// the page and mark elements for the tool to click (mark); they never change
// what the page holds: every change is a real click or real typing from the Go
// side. The selectors are TikTok's and TikTok Studio's markup as of
// 2026-10-14; a redesign is a change here.
window.__sanTikTok = (() => {
  const visible = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return false;
    for (let x = e; x && x !== document.body; x = x.parentElement) {
      const s = getComputedStyle(x);
      if (s.opacity === '0' || s.visibility === 'hidden') return false;
    }
    return true;
  };
  const first = (sel) => [...document.querySelectorAll(sel)].find(visible) || null;
  const all = (sel) => [...document.querySelectorAll(sel)].filter(visible);
  const text = (e) => (e ? (e.innerText || '').replace(/\s+/g, ' ').trim() : '');
  const own = (e) => (e ? (e.innerText || '').trim() : '');

  const editorSel = '.caption-editor .public-DraftEditor-content';
  const pickerInputs = () => [...document.querySelectorAll('[data-e2e=schedule_container] .scheduled-picker input')];
  const timePicker = () => document.querySelector('.tiktok-timepicker-time-picker-container');
  const timeOpen = () => { const p = timePicker(); return !!p && visible(p) && !p.className.includes('invisible'); };
  const calendar = () => first('.calendar-wrapper');

  // The Posts list rows holding both caption (its start will do) and when, as
  // the list shows it ("Oct 16, 7:00 PM"); spaces ignored.
  const squash = (s) => s.replace(/\s+/g, '');
  const postRows = (caption, when) => all('[data-tt=components_RowLayout_FlexRow]')
    .filter((r) => squash(text(r)).includes(squash(caption)) && squash(text(r)).includes(squash(when)));
  // The largest ancestor of a row that holds no other row: the whole line of
  // the list, actions column included.
  const rowBox = (r) => {
    let box = r;
    for (let p = r.parentElement; p && p !== document.body; p = p.parentElement) {
      if (p.querySelectorAll('[data-tt=components_RowLayout_FlexRow]').length > 1) break;
      box = p;
    }
    return box;
  };
  const describe = (e) => [e.tagName.toLowerCase(), e.getAttribute('data-tt') || '', e.getAttribute('data-e2e') || '',
    e.getAttribute('aria-label') || '', own(e).slice(0, 40), visible(e) ? 'shown' : 'hidden'].join('|');

  // Short texts shown on the page, outermost element first: the "…" menu is
  // plain elements (Download, Delete as of 2026-10-15), so a menu item is the
  // outermost shown element whose whole text is the item's.
  const shortTexts = () => all('div, span, li, button, a, p, h1, h2, h3, h4, [role]')
    .filter((e) => { const t = own(e); return t && t.length < 40 && !t.includes('\n'); });

  // The dialog around the shown title whose whole text matches question: the
  // nearest ancestor of the title that also holds buttons.
  const confirmBox = (question) => {
    const title = shortTexts().find((e) => new RegExp(question, 'i').test(own(e)));
    for (let p = title && title.parentElement; p && p !== document.body; p = p.parentElement) {
      if ([...p.querySelectorAll('button, [role=button]')].some(visible)) return p;
    }
    return null;
  };

  // The pickable things, for mark: each returns an element or null.
  const finders = {
    upload: () => first('[data-e2e=nav-upload] button') || first('[data-e2e=nav-upload]'),
    selectVideo: () => first('[data-e2e=select_video_button]'),
    editor: () => first(editorSel),
    // A hashtag suggestion that is exactly tag, the # included.
    suggestion: (tag) => all('.mention-list-popover .hashtag-suggestion-item')
      .find((i) => own(i).split('\n')[0].trim().toLowerCase() === tag.toLowerCase()) || null,
    radio: (value) => {
      const i = document.querySelector(`[data-e2e=schedule_container] input[name=postSchedule][value=${value}]`);
      return i ? i.closest('label') : null;
    },
    timeInput: () => pickerInputs()[0] || null,
    dateInput: () => pickerInputs()[1] || null,
    // A day of the date picker that can be chosen. Within the 10 days Studio
    // offers, a day number is never there twice.
    day: (n) => calendar() && [...calendar().querySelectorAll('.day.valid')].find((d) => own(d) === String(n)) || null,
    nextMonth: () => calendar() && calendar().querySelectorAll('.month-header-wrapper .arrow')[1] || null,
    hour: (hh) => timeOpen() && [...timePicker().querySelectorAll('.tiktok-timepicker-option-text.tiktok-timepicker-left')].find((o) => own(o) === hh) || null,
    minute: (mm) => timeOpen() && [...timePicker().querySelectorAll('.tiktok-timepicker-option-text.tiktok-timepicker-right')].find((o) => own(o) === mm) || null,
    // A neutral spot to click, which closes an open picker.
    scheduleTitle: () => first('[data-e2e=schedule_container] .scheduled-title-container'),
    visibility: () => first('[data-e2e=video_visibility_container] .Select__trigger'),
    // The options of "Who can see this post", by value: "0" Everyone,
    // "2" Friends, "1" Only you.
    visibilityOption: (value) => first(`[role=option][data-value='"${value}"']`),
    post: () => first('[data-e2e=post_video_button]'),
    discard: () => first('[data-e2e=discard_post_button]'),
    // The confirm modal's main button, such as Discard after Discard.
    modalPrimary: () => first('.TUXModal .common-modal-footer .TUXButton--primary'),
    // An onboarding popover's "Got it", which can sit over the form.
    gotIt: () => all('button, [role=button]').find((b) => /^Got it$/i.test(own(b))) || null,
    postsLink: () => all('button[data-tt=Sidebar_Sidebar_Clickable]').find((b) => /^Posts$/i.test(own(b))) || null,
    // The "…" button in the actions of the row markRow marked; for a scheduled
    // post the other actions are greyed out.
    rowMore: () => first('[data-san-row] button[data-tt=components_ActionCell_Clickable]'),
    // An item of the menu that "…" opens, by its whole text (a regexp).
    menuItem: (re) => shortTexts().find((e) => new RegExp(re, 'i').test(own(e)) && !e.closest('[data-san-row] [data-tt^=components_PostInfoCell]')) || null,
    // The button (by its whole text, re) of the dialog that asks question
    // (the whole text of its title, a regexp), such as "Delete post?" with
    // Delete and Cancel (2026-10-15; not a TUXModal).
    confirmButton: (question, re) => {
      const box = confirmBox(question);
      return box && [...box.querySelectorAll('button, [role=button]')].filter(visible).find((b) => new RegExp(re, 'i').test(own(b))) || null;
    },
    // A button of the modal Studio shows, by its whole text (a regexp).
    modalButton: (re) => all('.TUXModal button').find((b) => new RegExp(re, 'i').test(own(b))) || null,
  };

  return {
    state() {
      return {
        url: location.href,
        signin: /^\/login/.test(location.pathname) || !!first('[data-e2e=top-login-button]'),
      };
    },

    // tiktok.com signed in: Upload and the profile in the side menu.
    homeReady() { return !!finders.upload() && !!document.querySelector('[data-e2e=nav-profile]'); },

    // The signed-in account, from the side menu's profile link: "@name".
    account() {
      const a = document.querySelector('[data-e2e=nav-profile]');
      const m = a && (a.getAttribute('href') || '').match(/\/(@[^/?#]+)/);
      return m ? decodeURIComponent(m[1]) : '';
    },

    // The rect of the first shown match of sel, as a string, so that two
    // reads can be compared; null while there is none. An element outside the
    // window is scrolled into the middle of it first.
    rect(sel) {
      const e = first(sel);
      if (!e) return null;
      const r = e.getBoundingClientRect();
      if (r.top < 0 || r.bottom > innerHeight) {
        e.scrollIntoView({ block: 'center' });
        return null;
      }
      return [r.x, r.y, r.width, r.height].map(Math.round).join(',');
    },

    shown(sel) { return !!first(sel); },

    // Marks what finders[what](...args) finds with data-san-pick, for a
    // click, and returns that selector; null when there is none.
    mark(what, ...args) {
      document.querySelectorAll('[data-san-pick]').forEach((e) => e.removeAttribute('data-san-pick'));
      const e = finders[what](...args);
      if (!e) return null;
      e.setAttribute('data-san-pick', '');
      return '[data-san-pick]';
    },

    has(what, ...args) { return !!finders[what](...args); },

    // The upload page's two stages: the drop zone, then the form.
    uploadPage() { return !!finders.selectVideo(); },
    formShown() { return !!first(editorSel); },

    // The file's line above the form: its name, and the progress or
    // "Uploaded (44.38MB)". done: Studio shows the file as uploaded.
    upload() {
      const c = document.querySelector('[data-e2e=upload_status_container]');
      const t = text(c);
      return { text: t, done: /\bUploaded\b/i.test(t) };
    },

    // The caption editor: whether it has the caret, its text (one line per
    // block), the hashtags it holds, and whether a suggestion list is open.
    editor() {
      const e = document.querySelector(editorSel);
      if (!e) return null;
      const blocks = [...e.querySelectorAll('[data-block=true]')].map((b) => b.textContent);
      return {
        focused: document.activeElement === e,
        text: blocks.join('\n'),
        tags: [...e.querySelectorAll('[data-testid=mentionText]')].map((m) => m.textContent),
        list: all('.mention-list-popover').length > 0,
        items: all('.mention-list-popover .hashtag-suggestion-item').map((i) => own(i).split('\n')[0].trim()),
      };
    },

    // When to post: the radio chosen ("post_now" or "schedule"), the time
    // ("19:00") and date ("2026-10-17") boxes, and which picker is open.
    schedule() {
      const r = document.querySelector('[data-e2e=schedule_container] input[name=postSchedule]:checked') ||
        [...document.querySelectorAll('[data-e2e=schedule_container] input[name=postSchedule]')].find((i) => i.getAttribute('aria-checked') === 'true');
      const [time, date] = pickerInputs();
      return {
        mode: r ? r.value : '',
        time: time && visible(time) ? time.value : '',
        date: date && visible(date) ? date.value : '',
        timeOpen: timeOpen(),
        dateOpen: !!calendar(),
      };
    },

    // Who can see this post: the label shown, and when the list is open, the
    // value chosen and the label of each value.
    visibility() {
      const t = finders.visibility();
      const opts = all('[role=option][data-value]');
      const labels = {};
      opts.forEach((o) => { labels[JSON.parse(o.getAttribute('data-value'))] = text(o); });
      const sel = opts.find((o) => o.getAttribute('aria-selected') === 'true');
      return {
        label: text(t),
        open: opts.length > 0,
        selected: sel ? JSON.parse(sel.getAttribute('data-value')) : '',
        labels,
      };
    },

    // The post button: its text ("Post" or "Schedule") and whether it can be
    // pressed.
    postButton() {
      const b = finders.post();
      if (!b) return null;
      return {
        text: text(b),
        enabled: b.getAttribute('aria-disabled') !== 'true' && b.dataset.disabled !== 'true' && b.dataset.loading !== 'true',
      };
    },

    // Studio's checks under "Checks" (music copyright, content check lite):
    // whether one is still running. Post pressed while one runs makes Studio
    // ask "Continue to post?" before the check is complete.
    checks() {
      return { pending: /Checking in progress/i.test(document.body.innerText) };
    },

    // A modal shown over the page: its text and buttons; null when there is
    // none.
    modal() {
      const m = first('.TUXModal');
      if (!m) return null;
      return { text: text(m).slice(0, 300), buttons: all('.TUXModal button').map(text) };
    },

    // The Posts list rows that hold caption and when: each row's text and its
    // controls, as tag|data-tt|data-e2e|aria-label|text|shown.
    postRows(caption, when) {
      return postRows(caption, when).map((r) => {
        const box = rowBox(r);
        return {
          text: text(r),
          controls: [...box.querySelectorAll('button, [role=button], [data-tt*=Action], [data-tt*=Icon], [data-tt*=More]')]
            .filter((e) => !/VideoCover|PostInfoCell|PrivacyCell|Popover|Select/.test(e.getAttribute('data-tt') || ''))
            .map(describe).slice(0, 40),
        };
      });
    },

    // The text of the dialog that asks question (confirmBox), or null.
    confirmText(question) { const b = confirmBox(question); return b ? text(b) : null; },

    // The short texts on the page (shortTexts): before and after opening a
    // menu, their difference is what the menu offers.
    menuTexts() { return shortTexts().map(own); },

    // Marks the one row that holds caption and when with data-san-row (for a
    // hover), and returns how many rows hold them.
    markRow(caption, when) {
      document.querySelectorAll('[data-san-row]').forEach((e) => e.removeAttribute('data-san-row'));
      const rows = postRows(caption, when);
      if (rows.length === 1) rowBox(rows[0]).setAttribute('data-san-row', '');
      return rows.length;
    },

    // The browser's offset from UTC, in minutes east, at the instant ms: the
    // time zone Studio schedules in.
    tzOffset(ms) { return -new Date(ms).getTimezoneOffset(); },

    // Studio's Posts list (tiktokstudio/content): each row's text, from the
    // video's length and caption to the counts, such as "00:30 #gamedev …
    // Oct 17, 7:00 PM Everyone 0 0 0".
    posts() {
      if (!/\/tiktokstudio\/content/.test(location.pathname)) return null;
      return all('[data-tt=components_RowLayout_FlexRow]').map(text);
    },
  };
})();
