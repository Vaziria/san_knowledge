// What san_remote knows of Threads' pages, as found on 2026-10-23 with the
// interface in English. It runs in the extension's isolated world, where the
// page cannot see it, and stays there until the page is left: the Go side
// sends it with each call and it is set up only once per page.
//
// Threads' class names are generated, so everything is found by role and by
// the names Threads gives its controls: an aria-label, an SVG <title>, or the
// visible text. Clicks are .click() on the element that has the handler; the
// waits and the pace between steps are in Go.
globalThis.__sanThreads = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  // name is a control's accessible name: its aria-label, its text, or the
  // <title> of its icon.
  const name = (e) =>
    (e.getAttribute('aria-label') || e.innerText || e.querySelector('svg title')?.textContent || '').trim().replace(/\s+/g, ' ');
  const buttons = (root) => [...(root ?? document).querySelectorAll('[role=button]')].filter(vis);
  const button = (root, label) => root && buttons(root).find((b) => name(b) === label);
  const press = (e) => {
    if (!e) return false;
    (e.closest('[role=button],[role=menuitem],[role=option],[role=gridcell],a,button') ?? e).click();
    return true;
  };
  const dialogs = () => [...document.querySelectorAll('[role=dialog]')].filter(vis);
  // The composer: the dialog with the post editors (Lexical) shown. Drafts
  // opens in the same dialog, with the editors kept but hidden.
  const composer = () => dialogs().find((d) => [...d.querySelectorAll('[data-lexical-editor]')].some(vis));
  const editors = () => [...(composer()?.querySelectorAll('[data-lexical-editor]') ?? [])];
  // drafts is the Drafts list, which also holds the scheduled threads.
  const drafts = () => dialogs().find((d) => d.innerText.trim().startsWith('Drafts'));
  const picker = () => {
    const grid = document.querySelector('[role=grid][aria-label="Choose date"]');
    let box = grid;
    while (box && !box.querySelector('input[placeholder=hh]')) box = box.parentElement;
    return box && { grid, box };
  };
  const menuItem = (label) => [...document.querySelectorAll('[role=menuitem]')].find((m) => vis(m) && m.innerText.trim() === label);
  const profileLink = () => [...document.querySelectorAll('a[href^="/@"]')].find((a) => vis(a) && name(a) === 'Profile');
  // postBox is the part of the composer that holds post i: the largest box
  // around its editor that holds no other editor.
  const postBox = (i) => {
    const all = editors();
    let e = all[i];
    while (e?.parentElement && e.parentElement !== composer() && e.parentElement.querySelectorAll('[data-lexical-editor]').length === 1) {
      e = e.parentElement;
    }
    return e;
  };
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  // Media are sent in base64 pieces and kept here until attached.
  const staged = new Map();

  // A scheduled row of the Drafts list: "Posting on Sun 22 Nov at 09:00
  // GMT+7", the handle, the topic when it has one, the age ("1m",
  // "20/04/2025"), the first post's text, and "1 reply" when it is a thread.
  const age = /^(\d+[smhdwy]|\d{1,2}\/\d{1,2}\/\d{2,4})$/;
  const rows = () =>
    buttons(drafts())
      .filter((b) => b.innerText.trim().startsWith('Posting '))
      .map((b) => {
        const lines = b.innerText.split('\n').map((l) => l.trim()).filter(Boolean);
        const at = lines.findIndex((l, k) => k > 0 && age.test(l));
        let text = lines.slice(at + 1);
        if (text.length && /^\d+ repl(y|ies)$/.test(text[text.length - 1])) text = text.slice(0, -1);
        return { el: b, shown: lines[0], text: text.join('\n') };
      });

  return {
    // state is what the page shows, for the waits.
    state() {
      const profile = profileLink();
      return {
        url: location.href,
        visible: document.visibilityState === 'visible',
        signedIn: !!profile,
        username: profile ? decodeURIComponent(profile.getAttribute('href').slice(2).split('/')[0]) : '',
        signedOut:
          !profile &&
          (location.pathname.startsWith('/login') ||
            [...document.querySelectorAll('a, [role=button]')].some((e) => vis(e) && /^log in/i.test(name(e)))),
        newThread: !!button(document, 'New thread'),
        composer: !!composer(),
        posts: editors().length,
        picker: !!picker(),
        drafts: !!drafts(),
        menu: [...document.querySelectorAll('[role=menuitem]')].filter(vis).map((m) => m.innerText.trim()),
        saveAsk: !!button(document, "Don't save"),
      };
    },

    // ---- the composer ----
    newThread: () => press(button(document, 'New thread')),
    // composerButton presses a button of the composer by its name: Add to
    // thread, Post, Schedule, Cancel, Close (the schedule chip's).
    composerButton: (label) => press(button(composer(), label)),
    // enabled says whether that button can be pressed now.
    enabled(label) {
      const b = button(composer(), label);
      return !!b && b.getAttribute('aria-disabled') !== 'true';
    },
    dontSave: () => press(button(document, "Don't save")),
    texts: () => editors().map((e) => e.innerText),
    // paste puts the text in post i with one paste event, the way Lexical
    // takes text with its line breaks.
    paste(i, text) {
      const ed = editors()[i];
      if (!ed) return false;
      ed.focus();
      const data = new DataTransfer();
      data.setData('text/plain', text);
      ed.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
      return true;
    },
    // focusPost makes post i the one being edited, which shows its topic box.
    focusPost(i) {
      const ed = editors()[i];
      if (!ed) return false;
      ed.click();
      ed.focus();
      return true;
    },
    topicBox() {
      const box = [...(composer()?.querySelectorAll('input[placeholder="Community or topic"]') ?? [])].find(vis);
      return box ? box.value : null;
    },
    // typeTopic types the topic key by key, as a person does, so Threads
    // looks up its suggestions.
    async typeTopic(text) {
      const box = [...(composer()?.querySelectorAll('input[placeholder="Community or topic"]') ?? [])].find(vis);
      if (!box) return null;
      box.focus();
      const set = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
      set.call(box, '');
      box.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'deleteContentBackward' }));
      for (const ch of text) {
        set.call(box, box.value + ch);
        box.dispatchEvent(new InputEvent('input', { bubbles: true, data: ch, inputType: 'insertText' }));
        await sleep(90 + Math.random() * 130);
      }
      return box.value;
    },
    topicOptions: () => [...document.querySelectorAll('[role=option]')].filter(vis).map((o) => o.innerText.trim().split('\n')[0]),
    // marked is the suggestion the arrow keys are on (aria-selected).
    marked: () =>
      [...document.querySelectorAll('[role=option]')].find((o) => vis(o) && o.getAttribute('aria-selected') === 'true')?.innerText.trim().split('\n')[0] ?? '',
    // topicKey presses a key in the topic box. A click on a suggestion does
    // not choose it (2026-10-24); the down arrow and Enter do, as with
    // Medium's topics.
    topicKey(key) {
      const box = [...(composer()?.querySelectorAll('input[placeholder="Community or topic"]') ?? [])].find(vis);
      if (!box) return false;
      box.focus();
      box.dispatchEvent(new KeyboardEvent('keydown', { key, code: key, bubbles: true, cancelable: true }));
      return true;
    },
    stage(id, piece) {
      staged.set(id, (staged.get(id) ?? '') + piece);
      return staged.get(id).length;
    },
    // attach gives post i the staged files, through the composer's own file
    // input, as if they were chosen in its file dialog.
    attach(i, files) {
      let e = editors()[i];
      while (e && !e.querySelector('input[type=file]')) e = e.parentElement;
      const input = e?.querySelector('input[type=file]');
      if (!input) return 'no file input';
      const list = new DataTransfer();
      for (const f of files) {
        const bin = atob(staged.get(f.id) ?? '');
        const bytes = new Uint8Array(bin.length);
        for (let k = 0; k < bin.length; k++) bytes[k] = bin.charCodeAt(k);
        list.items.add(new File([bytes], f.name, { type: f.type }));
        staged.delete(f.id);
      }
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // media counts the previews in post i.
    media: (i) => postBox(i)?.querySelectorAll('img[src^="blob:"], video').length ?? 0,

    // ---- More and the schedule picker ----
    openMore: () => press(composer()?.querySelector('[aria-label=More]')),
    menu: (label) => press(menuItem(label)),
    month() {
      const p = picker();
      return p ? [...p.box.querySelectorAll('*')].find((e) => e.children.length === 0 && /^[A-Z][a-z]+ \d{4}$/.test(e.textContent.trim()))?.textContent.trim() ?? '' : null;
    },
    nextMonth: () => press(picker()?.box.querySelector('[aria-label="Next Month"]')),
    prevMonth: () => press(picker()?.box.querySelector('[aria-label="Previous Month"]')),
    // pickDay clicks the day whose label starts with day, such as "Sunday 22
    // November 2026", and says what is selected then.
    pickDay(day) {
      const p = picker();
      const cell = p && [...p.grid.querySelectorAll('[role=gridcell]')].find((c) => c.innerText.trim().startsWith(day));
      if (!cell) return 'no such day';
      if (cell.getAttribute('aria-disabled') === 'true') return 'that day cannot be chosen';
      press(cell);
      return '';
    },
    selectedDay() {
      const p = picker();
      return p ? ([...p.grid.querySelectorAll('[role=gridcell]')].map((c) => c.innerText.trim()).find((t) => /, selected/.test(t))?.split(',')[0] ?? '') : null;
    },
    // setTime types the hour (00–23) and the minute into the picker.
    setTime(hh, mm) {
      const p = picker();
      if (!p) return null;
      const out = [];
      for (const [ph, v] of [['hh', hh], ['mm', mm]]) {
        const box = p.box.querySelector(`input[placeholder=${ph}]`);
        box.focus();
        box.select();
        document.execCommand('insertText', false, v);
        out.push(box.value);
      }
      return out;
    },
    // pickerButton presses Done (a new thread) or Update (Edit schedule).
    pickerButton: (label) => press(button(picker()?.box, label)),
    // chip is the composer's schedule line: "Posting on Sun 22 Nov at 09:00
    // GMT+7", or "" when the thread is not scheduled.
    chip: () => buttons(composer()).map(name).find((n) => n.startsWith('Posting ')) ?? '',
    toasts: () => [...document.querySelectorAll('[role=alert], [role=status]')].filter(vis).map((e) => e.innerText.trim()).filter(Boolean),

    // ---- Drafts: the scheduled threads ----
    openDrafts: () => press(composer()?.querySelector('[aria-label=Drafts]')),
    rows: () => rows().map(({ shown, text }) => ({ shown, text })),
    // rowMenu opens the More menu of scheduled row i.
    rowMenu(i) {
      const row = rows()[i];
      return !!row && press(buttons(row.el).find((b) => name(b) === 'More'));
    },
    back: () => press(button(drafts(), 'Back')),

    // ---- the profile ----
    openProfile: () => press(profileLink()),
    // posts reads the profile's posts, newest first: the link of each, its
    // time and its text.
    posts(limit) {
      const user = location.pathname.split('/')[1];
      if (!user?.startsWith('@')) return null;
      const links = [...document.querySelectorAll(`a[href^="/${user}/post/"]`)].filter((a) => vis(a) && a.querySelector('time'));
      return links.slice(0, limit).map((a) => {
        // The post's box: the largest one around its time link that holds no
        // other post's.
        let box = a;
        while (box.parentElement && box.parentElement.querySelectorAll(`a[href^="/${user}/post/"] time`).length === 1) box = box.parentElement;
        const shown = a.innerText.trim();
        const skip = new Set([user.slice(1), shown, 'Translate', 'Author', '·']);
        const text = box.innerText
          .split('\n')
          .map((l) => l.trim())
          .filter((l) => l && !skip.has(l) && !/^[\d.,]+[KM]?$/.test(l))
          .join('\n');
        return { url: new URL(a.getAttribute('href'), location.origin).href, at: a.querySelector('time').getAttribute('datetime'), text };
      });
    },
  };
})();
