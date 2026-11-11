// What san_remote knows of Google Trends' new Explore page
// (trends.google.com/explore), as found on 2026-11-11. Like the other sites'
// scripts, it runs in the extension's isolated world, where the page cannot
// see it; the one hook on the page's fetch is service.go's, in the main
// world. It only reads.
//
// - The new Explore is /explore?q=<term>&geo=<code>&date=<range>&cat=<id>
//   &gprop=<type>. Its filters are four buttons with a menu each, told
//   apart by their icons (Material symbols, the same in every language):
//   location_on, calendar_month, category, language. Each menu item has the
//   value in a data attribute: data-geo-code / data-country-code (a country
//   with regions has a submenu, built only when it opens: focus the item and
//   press ArrowRight), data-value for the time range (1-9, 10 a year or
//   Custom), the category (its id) and the search type (web, images, news,
//   froogle, youtube); the item in use has aria-checked="true".
// - The categories are also the page's first data block,
//   AF_initDataCallback({key: 'ds:0', ... data:[[[3,"Arts & Entertainment",
//   [[184, ...]]], ...]]}): each [id, name, children].
// - The related queries are the last section ("Commonly searched queries"),
//   with a Top and a Rising table. The page fetches them (batchexecute rpc
//   fXqlme) only once the section is on screen, so a hidden tab never does.
// - The classic Explore is /trends/explore (with "legacy" in its address),
//   with a "Classic Explore is expiring soon" banner and its "Go to new
//   Explore" link. Google's robot check is google.com/sorry; its 429 page
//   says "Too Many Requests".
globalThis.__sanTrends = (() => {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const clean = (s) => (s || '').replace(/[\u200e\u200f\u202a-\u202e]/g, '').replace(/\s+/g, ' ').trim();
  const shown = (e) => !!e && e.offsetHeight > 0;
  const bodyText = () => (document.body ? document.body.innerText : '');
  const classic = () => location.pathname.startsWith('/trends/explore') || location.pathname.startsWith('/trends/embed');
  // filterButton is the filter whose icon is icon: location_on,
  // calendar_month, category or language.
  const filterButton = (icon) =>
    [...document.querySelectorAll('button[aria-haspopup]')].find(
      (b) => shown(b) && [...b.querySelectorAll('i')].some((i) => i.textContent.trim() === icon),
    );
  // checked is the data attribute of the menu item in use among items.
  const checked = (items, attr) => {
    const it = items.find((i) => i.getAttribute('aria-checked') === 'true');
    return it ? it.getAttribute(attr) : null;
  };
  // menuWith is the menu that holds an item matching sel: the time menu
  // has the years ([data-year]), the search type menu froogle.
  const menuWith = (sel) => {
    const it = document.querySelector(`[role="menu"] ${sel}`);
    return it ? it.closest('[role="menu"]') : null;
  };
  const menuItems = (menu) => (menu ? [...menu.querySelectorAll('[role^="menuitem"][data-value]')] : []);
  const itemName = (i) => clean((i.querySelector('[jsname="K4r5Ff"]') || i).innerText || i.textContent);
  // section is the related queries' heading: the last heading of the page
  // that is shown (the English one by its words, when there).
  const section = () => {
    const hs = [...document.querySelectorAll('h2')].filter(shown);
    return hs.find((h) => /Commonly searched|Related queries/i.test(h.innerText)) || hs[hs.length - 1] || null;
  };
  // switchControl is the classic Explore's own way to the new one: its
  // banner's "Go to new Explore".
  const switchControl = () =>
    [...document.querySelectorAll('a[href], button')].find(
      (e) => shown(e) && /new Explore/i.test(e.innerText + ' ' + (e.getAttribute('aria-label') || '')),
    );
  const key = (el, k, code) => {
    for (const t of ['keydown', 'keyup']) {
      el.dispatchEvent(new KeyboardEvent(t, { key: k, code: k, keyCode: code, which: code, bubbles: true, cancelable: true, composed: true }));
    }
  };

  return {
    // state is where the tab is.
    state() {
      const text = bodyText();
      const h = section();
      return {
        url: location.href,
        classic: classic(),
        explore: location.hostname === 'trends.google.com' && location.pathname === '/explore',
        switch: classic() && !!switchControl(),
        blocked:
          location.pathname.startsWith('/sorry/') ||
          !!document.querySelector('form#captcha-form') ||
          /unusual traffic from your computer/i.test(text),
        tooMany: /Too Many Requests|Error 429/i.test(document.title + ' ' + text.slice(0, 500)),
        consent: location.hostname.startsWith('consent.'),
        filters: !!filterButton('location_on'),
        section: !!h,
        signedIn: !!document.querySelector('a[aria-label^="Google Account"], a[href^="https://accounts.google.com/SignOutOptions"]'),
        rows: document.querySelectorAll('table[role="grid"] tbody tr').length,
      };
    },

    // switchToNew presses the classic Explore's own switch to the new one
    // ("Go to new Explore").
    switchToNew() {
      const c = switchControl();
      if (!c) return false;
      c.click();
      return true;
    },

    // filters are the filters the page uses: its checked location, time
    // range, category and search type.
    filters() {
      const time = menuItems(menuWith('[data-year]'));
      const prop = menuItems(menuWith('[data-value="froogle"]'));
      const cat = [...document.querySelectorAll('[role^="menuitem"][data-value]')].filter((i) => /^\d+$/.test(i.getAttribute('data-value')) && !time.includes(i));
      const geo = [...document.querySelectorAll('[role^="menuitem"][data-geo-code]')].filter((i) => !i.hasAttribute('my-location'));
      const tm = time.find((i) => i.getAttribute('aria-checked') === 'true');
      return {
        geo: checked(geo, 'data-geo-code'),
        time: tm ? tm.getAttribute('data-value') : null,
        year: tm ? tm.getAttribute('data-year') || '' : '',
        category: checked(cat, 'data-value'),
        property: checked(prop, 'data-value'),
      };
    },

    // reveal scrolls the related queries into view, which makes the page
    // fetch them while it is shown.
    reveal() {
      const h = section();
      if (!h) return false;
      h.scrollIntoView({ block: 'start' });
      return true;
    },

    // countries are Worldwide and the countries of the location menu, in its
    // order. The page builds the menu when it first opens, and only while
    // the page is shown: it is opened, read and closed again.
    async countries() {
      const sel = '[role^="menuitem"][data-country-code], [role^="menuitem"][data-geo-code][data-event-label="select-country"]';
      if (!document.querySelector(sel)) {
        const b = filterButton('location_on');
        if (!b) return [];
        b.click();
        for (let i = 0; i < 40 && !document.querySelector(sel); i++) await sleep(150);
        for (let i = 0; i < 3; i++) {
          key(document.activeElement || document.body, 'Escape', 27);
          await sleep(100);
        }
      }
      const out = [];
      const seen = new Set();
      for (const i of document.querySelectorAll(sel)) {
        if (i.hasAttribute('my-location') || i.hasAttribute('data-default-submenu-item')) continue;
        let code = i.getAttribute('data-country-code') || i.getAttribute('data-geo-code');
        if (code === 'Worldwide') code = '';
        if (seen.has(code)) continue;
        seen.add(code);
        out.push({ code, name: itemName(i), has_regions: i.hasAttribute('data-country-code') && i.hasAttribute('data-submenu-id') });
      }
      return out;
    },

    // categories are the page's category tree, from its first data block.
    categories() {
      const tree = (rows) =>
        (rows || [])
          .filter((r) => Array.isArray(r) && typeof r[0] === 'number' && typeof r[1] === 'string')
          .map((r) => ({ id: r[0], name: r[1], children: tree(Array.isArray(r[2]) ? r[2] : []) }));
      for (const s of document.scripts) {
        const t = s.textContent || '';
        if (!t.includes("key: 'ds:0'")) continue;
        const at = t.indexOf('data:');
        const end = t.lastIndexOf(', sideChannel');
        if (at < 0 || end < at) continue;
        try {
          const data = JSON.parse(t.slice(at + 5, end));
          const top = tree(Array.isArray(data) && Array.isArray(data[0]) ? data[0] : []);
          if (top.length) return top;
        } catch (e) {
          // the menu below
        }
      }
      return [];
    },

    // options are the time range and search type menus' items.
    options() {
      const read = (menu) =>
        menuItems(menu).map((i) => ({ value: i.getAttribute('data-value'), year: i.getAttribute('data-year') || '', label: itemName(i) }));
      return { times: read(menuWith('[data-year]')), properties: read(menuWith('[data-value="froogle"]')) };
    },

    // regions opens the location menu, then the country's submenu (the
    // page builds it only then, and only from the keyboard: focus the
    // country, ArrowRight), reads its regions and closes the menu again. The
    // tab must be shown.
    async regions(country) {
      const b = filterButton('location_on');
      if (!b) return { error: 'no location button' };
      const item = () => [...document.querySelectorAll(`[role^="menuitem"][data-country-code="${country}"]`)].find(shown);
      if (!item()) {
        b.click();
        for (let i = 0; i < 20 && !item(); i++) await sleep(150);
      }
      const it = item();
      if (!it) return { error: 'no country ' + country + ' in the location menu' };
      it.scrollIntoView({ block: 'center' });
      it.focus();
      await sleep(200);
      key(it, 'ArrowRight', 39);
      let list = [];
      for (let i = 0; i < 30 && !list.length; i++) {
        await sleep(150);
        const m = [...document.querySelectorAll('[role="menu"]')].find(
          (x) => shown(x) && x.querySelector('[role^="menuitem"][data-event-label="select-region"]'),
        );
        if (m) list = [...m.querySelectorAll('[role^="menuitem"][data-event-label="select-region"]')];
      }
      const regions = list.map((i) => ({ code: i.getAttribute('data-geo-code'), name: itemName(i) })).filter((r) => r.code);
      for (let i = 0; i < 3; i++) {
        key(document.activeElement || document.body, 'Escape', 27);
        await sleep(150);
      }
      return { regions };
    },
  };
})();
