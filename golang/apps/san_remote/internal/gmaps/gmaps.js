// What san_remote knows of Google Maps (www.google.com/maps), as found on
// 2026-11-06. Every page is opened with hl=en, so the few words read here are
// English. Like the other sites' scripts, it runs in the extension's isolated
// world, where the page cannot see it. It only reads.
//
// - A search, /maps/search/<keyword>/@<lat>,<lng>,<zoom>z, lists its places
//   in div[role="feed"], one a.hfpxzc a place (aria-label its name, href its
//   page). The card around it has the rating, span[role="img"] "4.9 stars
//   1,234 Reviews". Scrolling the feed to its bottom loads the next places;
//   the last ones are followed by "You've reached the end of the list.".
//   A search Maps finds one place for goes straight to that place's page;
//   one it finds none for says "Google Maps can't find <keyword>".
// - A place's page, /maps/place/<name>/data=...!3d<lat>!4d<lng>..., has its
//   name in h1, its category in button[jsaction*="category"], and its
//   details in elements named by data-item-id: address, phone:tel:<number>,
//   authority (the website, a link), oloc (the plus code). Their text starts
//   with an icon, a private-use character, which is dropped.
// - A browser Maps does not trust gets "You're seeing a limited view of
//   Google Maps", with a few places only.
globalThis.__sanGmaps = (() => {
  const pua = /[\ue000-\uf8ff]/g;
  const clean = (s) => (s || '').replace(pua, ' ').replace(/\s+/g, ' ').trim();
  const text = (e) => (e ? clean(e.innerText || e.textContent) : '');
  const feed = () => document.querySelector('div[role="feed"]');
  const ended = (f) => /reached the end of the list/i.test(f.innerText || '');
  const limited = () => /limited view of Google Maps/i.test(document.body ? document.body.innerText : '');
  const item = (sel) => document.querySelector(`[data-item-id${sel}]`);

  return {
    // state is where the tab is.
    state() {
      const f = feed();
      return {
        url: location.href,
        consent: location.hostname.startsWith('consent.'),
        blocked: location.pathname.startsWith('/sorry/') || !!document.querySelector('form#captcha-form'),
        feed: !!f,
        results: f ? f.querySelectorAll('a.hfpxzc').length : 0,
        end: f ? ended(f) : false,
        none: !f && /Google Maps can.t find/i.test(document.body ? document.body.innerText : ''),
        limited: limited(),
        title: text(document.querySelector('h1')),
        items: document.querySelectorAll('[data-item-id]').length,
      };
    },

    // results lists the places in the result list, in order.
    results() {
      const f = feed();
      if (!f) return [];
      return [...f.querySelectorAll('a.hfpxzc')].map((a) => {
        const card = a.parentElement;
        const star = card ? card.querySelector('span[role="img"][aria-label]') : null;
        return {
          name: clean(a.getAttribute('aria-label')),
          href: a.href,
          stars: star ? clean(star.getAttribute('aria-label')) : '',
        };
      });
    },

    // more scrolls the list to its bottom, which has Maps load the next
    // places.
    more() {
      const f = feed();
      if (!f) return false;
      f.scrollTop = f.scrollHeight;
      f.dispatchEvent(new Event('scroll'));
      return true;
    },

    // place reads the place whose page the tab shows.
    place() {
      const phone = item('^="phone:tel:"');
      const site = item('="authority"');
      const rating = document.querySelector('div.F7nice');
      return {
        url: location.href,
        name: text(document.querySelector('h1')),
        category: text(document.querySelector('button[jsaction*="category"]')),
        address: text(item('="address"')),
        phone: text(phone),
        tel: phone ? phone.getAttribute('data-item-id').slice('phone:tel:'.length) : '',
        website: site && site.href ? site.href : '',
        plusCode: text(item('="oloc"')),
        rating: text(rating),
      };
    },
  };
})();
