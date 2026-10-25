// What san_remote knows of X's pages, as found on 2026-10-24 with the
// interface in English: the home page, the compose dialog (/compose/post) and
// its schedule dialog (/compose/post/schedule), and a profile's posts. Like
// threads.js, it runs in the extension's isolated world, where the page cannot
// see it.
//
// X marks its controls with data-testid attributes, which do not change with
// the language; those are used wherever X has them. Clicks are .click() on
// the element that has the handler; the waits and the pace are in Go. The
// compose dialog only renders in a shown page.
globalThis.__sanX = (() => {
  const vis = (e) => {
    if (!e) return false;
    const r = e.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const tid = (root, id) => [...(root ?? document).querySelectorAll(`[data-testid="${id}"]`)].find(vis);
  // press clicks e as a mouse does: the focus leaves the editor first. With
  // .click() alone the editor keeps it, and "Add post" then empties the post
  // instead of adding one (2026-10-24).
  const press = (e) => {
    if (!e) return false;
    document.activeElement?.blur?.();
    e.focus?.();
    e.click();
    return true;
  };
  const dialog = () => [...document.querySelectorAll('[role=dialog]')].filter(vis).pop();
  // The compose dialog: the top dialog with the first post's editor. The home
  // page has an editor of its own behind it, so editors are looked up in here.
  const composer = () => {
    const d = dialog();
    return d && d.querySelector('[data-testid=tweetTextarea_0]') ? d : null;
  };
  const editors = () => [...(composer()?.querySelectorAll('[data-testid^=tweetTextarea_]') ?? [])].filter((e) => /^tweetTextarea_\d+$/.test(e.dataset.testid));
  const editor = (i) => composer()?.querySelector(`[data-testid=tweetTextarea_${i}]`);
  // A post's attachments are the ones after its editor and before the next.
  const owner = (el) => {
    let o = -1;
    editors().forEach((e, i) => {
      if (e.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING) o = i;
    });
    return o;
  };
  // The schedule dialog's dropdowns. Their ids change each time it opens, so
  // each is known by its options.
  const selects = () => {
    const all = [...(dialog()?.querySelectorAll('select') ?? [])].filter(vis);
    const values = (s) => [...s.options].map((o) => o.value);
    const month = all.find((s) => [...s.options].some((o) => o.text === 'January'));
    return {
      month,
      // Every month has a 28th; not every month a 31st.
      day: all.find((s) => s !== month && values(s).includes('28') && !values(s).includes('59')),
      year: all.find((s) => values(s).some((v) => /^\d{4}$/.test(v))),
      hour: all.find((s) => s !== month && values(s).includes('12') && !values(s).includes('13')),
      minute: all.find((s) => values(s).includes('59')),
      ampm: all.find((s) => values(s).includes('am')),
    };
  };
  const setSelect = (s, v) => {
    Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value').set.call(s, String(v));
    s.dispatchEvent(new Event('change', { bubbles: true }));
  };
  // line is "Will send on Fri, Nov 20, 2026 at 9:00 AM", in the composer once
  // a time is confirmed, and in the schedule dialog while it is chosen.
  const line = () => (dialog()?.innerText ?? '').split('\n').map((l) => l.trim()).find((l) => /^Will send on /.test(l)) ?? '';
  const unsent = () => [...(dialog()?.querySelectorAll('[data-testid=unsentTweet]') ?? [])].filter(vis);
  const textButton = (label) => [...(dialog()?.querySelectorAll('[role=button], button') ?? [])].filter(vis).find((b) => (b.innerText ?? '').trim() === label);
  const toasts = () => [...document.querySelectorAll('[data-testid=toast], [role=alert]')].filter(vis).map((t) => t.innerText.trim()).filter(Boolean);
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
      const c = composer();
      const s = selects();
      const submit = tid(c, 'tweetButton');
      return {
        url: location.href,
        lang: document.documentElement.lang,
        visible: document.visibilityState === 'visible',
        signedIn: !!tid(document, 'SideNav_AccountSwitcher_Button'),
        signedOut: !!document.querySelector('[data-testid=loginButton], a[href="/login"], a[href="/i/flow/login"]'),
        composer: !!c,
        posts: editors().length,
        schedule: !!(s.month && s.day && s.year && s.hour && s.minute && s.ampm),
        line: line(),
        submit: submit ? submit.innerText.trim() : '',
        enabled: !!submit && submit.getAttribute('aria-disabled') !== 'true',
        sheet: !!tid(document, 'confirmationSheetDialog'),
        toasts: toasts(),
        // notices are the composer's own messages, such as the red "Some of
        // your media failed to upload." it shows when X refuses a post.
        notices: [...(c?.querySelectorAll('[role=status]') ?? [])].filter(vis).map((n) => n.innerText.trim()).filter(Boolean),
      };
    },
    // account is the signed-in account, from the side bar's account button:
    // its name, then @handle.
    account() {
      const b = tid(document, 'SideNav_AccountSwitcher_Button');
      if (!b) return null;
      const lines = b.innerText.split('\n').map((l) => l.trim()).filter(Boolean);
      const handle = lines.find((l) => l.startsWith('@')) ?? '';
      return { name: lines.find((l) => !l.startsWith('@')) ?? '', handle: handle.slice(1) };
    },
    openComposer: () => press(tid(document, 'SideNav_NewTweet_Button')),
    // paste pastes text into post i's editor, as a person pastes it.
    paste(i, text) {
      const ed = editor(i);
      if (!ed) return false;
      ed.focus();
      const data = new DataTransfer();
      data.setData('text/plain', text);
      ed.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
      return true;
    },
    text: (i) => editor(i)?.innerText ?? null,
    addPost: () => press(tid(composer(), 'addButton')),
    focus(i) {
      const ed = editor(i);
      if (!ed) return false;
      ed.focus();
      return true;
    },
    stage(id, piece) {
      staged.set(id, (staged.get(id) ?? '') + piece);
      return staged.get(id).length;
    },
    // attach gives post i the staged files through the composer's file input,
    // as if they were chosen in its file dialog. The input adds to the post
    // that has the cursor.
    attach(i, files) {
      const ed = editor(i);
      const input = composer()?.querySelector('input[data-testid=fileInput]');
      if (!ed || !input) return 'no editor or file input';
      ed.focus();
      const list = new DataTransfer();
      for (const f of files) {
        list.items.add(new File([bytes(staged.get(f.id) ?? '')], f.name, { type: f.type }));
        staged.delete(f.id);
      }
      input.files = list.files;
      input.dispatchEvent(new Event('change', { bubbles: true }));
      return '';
    },
    // media is how many images and videos post i shows.
    media: (i) =>
      [...(composer()?.querySelectorAll('[data-testid=attachments]') ?? [])]
        .filter((a) => owner(a) === i)
        .reduce((n, a) => n + a.querySelectorAll('img, video').length, 0),
    openSchedule: () => press(tid(composer(), 'scheduleOption')),
    // setWhen sets the schedule dialog's dropdowns, and returns what they then
    // hold and the line the dialog shows.
    setWhen(year, month, day, hour, minute, ampm) {
      const s = selects();
      if (!s.month || !s.day || !s.year || !s.hour || !s.minute || !s.ampm) return null;
      // The year and month first, so the day exists in them.
      for (const [sel, v] of [[s.year, year], [s.month, month], [s.day, day], [s.hour, hour], [s.minute, minute], [s.ampm, ampm]]) setSelect(sel, v);
      return this.when();
    },
    when() {
      const s = selects();
      if (!s.month) return null;
      return { year: s.year.value, month: s.month.value, day: s.day.value, hour: s.hour.value, minute: s.minute.value, ampm: s.ampm.value, line: line() };
    },
    confirmSchedule: () => press(tid(document, 'scheduledConfirmationPrimaryAction')),
    submit: () => press(tid(composer(), 'tweetButton')),
    // close closes the composer; X then asks whether to save the draft, and
    // discard answers Discard.
    close: () => press(tid(composer(), 'app-bar-close')),
    discard: () => press(tid(document, 'confirmationSheetCancel')),

    // ---- Drafts → Scheduled (/compose/post/unsent/scheduled) ----
    openDrafts: () => press(tid(composer(), 'unsentButton')),
    // openScheduledTab presses the Drafts dialog's Scheduled tab, known by
    // where it leads.
    openScheduledTab: () =>
      press([...(dialog()?.querySelectorAll('[role=tab]') ?? [])].find((t) => /\/unsent\/scheduled$/.test(t.getAttribute('href') ?? t.closest('a')?.getAttribute('href') ?? ''))),
    // listLoaded says the Scheduled tab shows its posts, or that it has none
    // ("Hold that thought").
    listLoaded: () =>
      /\/compose\/post\/unsent\/scheduled$/.test(location.pathname) &&
      (!!tid(dialog(), 'unsentTweet') || /Hold that thought/.test(dialog()?.innerText ?? '')),
    // scheduled lists the Scheduled tab's threads: the "Will send on" line,
    // the first post's text, and "1 more post" when there are more.
    scheduled: () =>
      [...(dialog()?.querySelectorAll('[data-testid=unsentTweet]') ?? [])].filter(vis).map((el) => {
        const lines = el.innerText.split('\n').map((l) => l.trim()).filter(Boolean);
        return {
          line: lines.find((l) => /^Will send on /.test(l)) ?? '',
          text: el.querySelector('[data-testid=tweetText]')?.innerText ?? lines[1] ?? '',
          more: lines.find((l) => /^\d+ more posts?$/.test(l)) ?? '',
        };
      }),
    // openScheduled opens scheduled thread i in the composer, its posts and
    // time loaded. In Edit mode the same click ticks its checkbox instead.
    openScheduled: (i) => press(unsent()[i]),
    // Edit mode: "Edit" turns the list into one to choose from, with "Select
    // all" and "Delete" under it. These buttons have no test id.
    openEdit: () => press(textButton('Edit')),
    editing: () => !!textButton('Delete') && (!!textButton('Select all') || !!textButton('Deselect all')),
    // selected says which threads are ticked: each one sits in a label with
    // a checkbox.
    selected: () => unsent().map((u) => !!u.closest('label')?.querySelector('input[type=checkbox]')?.checked),
    deleteSelected: () => press(textButton('Delete')),
    // confirm answers the question X asks before a delete ("Discard unsent
    // posts", "Delete post?") with its first button.
    confirm: () => press(tid(document, 'confirmationSheetConfirm')),
    sheetText: () => tid(document, 'confirmationSheetDialog')?.innerText ?? '',

    // ---- a profile ----
    openProfile: () => press(tid(document, 'AppTabBar_Profile_Link')),
    // posts lists the posts the page shows by handle, without reposts:
    // text, time and link.
    posts(handle) {
      const out = [];
      for (const a of document.querySelectorAll('article[data-testid=tweet]')) {
        const t = a.querySelector('time');
        const href = t?.closest('a')?.getAttribute('href') ?? '';
        if (!href.toLowerCase().startsWith(`/${handle.toLowerCase()}/status/`)) continue;
        if (/reposted/i.test(a.querySelector('[data-testid=socialContext]')?.innerText ?? '')) continue;
        out.push({ text: a.querySelector('[data-testid=tweetText]')?.innerText ?? '', at: t.getAttribute('datetime'), link: href });
      }
      return out;
    },
    // postMenu opens the "More" menu (caret) of the post whose link is link,
    // such as /handle/status/123.
    postMenu(link) {
      const a = [...document.querySelectorAll('article[data-testid=tweet]')].find((x) => x.querySelector('time')?.closest('a')?.getAttribute('href') === link);
      return press(tid(a, 'caret'));
    },
    menu: () => [...document.querySelectorAll('[role=menu] [role=menuitem]')].filter(vis).map((m) => m.innerText.trim()),
    menuItem: (label) => press([...document.querySelectorAll('[role=menu] [role=menuitem]')].filter(vis).find((m) => m.innerText.trim() === label)),
    hasPost: (link) => [...document.querySelectorAll('article[data-testid=tweet] time')].some((t) => t.closest('a')?.getAttribute('href') === link),
    top: () => window.scrollTo(0, 0),
    // more scrolls the timeline down a screen, for the posts below.
    more() {
      window.scrollBy(0, window.innerHeight);
      return document.querySelectorAll('article[data-testid=tweet]').length;
    },
    // profileLoaded says the profile's header is there: its post count line
    // or its empty state.
    profileLoaded: () => !!tid(document, 'UserName') || !!tid(document, 'emptyState'),
  };
})();
