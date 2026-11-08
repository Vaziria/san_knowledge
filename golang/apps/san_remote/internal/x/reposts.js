// Runs in the page's own world, as the body of an async function, on a
// profile's Reposts tab (or its Posts tab). It returns each post the
// timeline shows, as X's own app holds it (the React props of its article,
// as post.js reads them) and as the page draws it, for Go (reposted,
// reposts.go) to pick the profile's reposts from. Found on 2026-11-08: a
// repost's article holds the repost, by the reposter, whose
// retweeted_status is the post reposted; its social context reads
// "You reposted" (or "<name> reposted" to others) and links to the
// reposter's profile. It only reads.
const props = (el, ok) => {
  const k = el && Object.keys(el).find((k) => k.startsWith('__reactFiber$'));
  for (let f = k ? el[k] : null, i = 0; f && i < 80; f = f.return, i++) {
    if (f.memoizedProps && ok(f.memoizedProps)) return f.memoizedProps;
  }
  return null;
};
const unescape = (s) => s.replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
// text is a post's text as its author wrote it: a long post's whole text
// (note_tweet), else full_text past a reply's leading @names, its t.co
// links written out, the links to its own media dropped.
const text = (t) => {
  const note = t.note_tweet?.text ? t.note_tweet : null;
  const ents = note?.entity_set ?? t.entities ?? {};
  let s = note ? note.text : Array.from(t.full_text ?? '').slice(t.display_text_range?.[0] ?? 0).join('');
  for (const u of ents.urls ?? []) s = s.split(u.url).join(u.expanded_url ?? u.url);
  for (const m of t.entities?.media ?? []) s = s.split(m.url).join('');
  return unescape(s);
};
const lines = (el) => (el?.innerText ?? '').split('\n').map((l) => l.trim()).filter(Boolean);
return [...document.querySelectorAll('article[data-testid="tweet"]')].map((a) => {
  const context = a.querySelector('[data-testid="socialContext"]');
  const p = props(a, (p) => p.tweet?.id_str);
  const tw = p?.tweet ?? null;
  const rt = tw?.retweeted_status?.id_str ? tw.retweeted_status : null;
  const post = rt ?? tw;
  const time = a.querySelector('time');
  return {
    context: context?.innerText.trim() ?? '',
    by: context?.closest('a')?.getAttribute('href') ?? '',
    promoted: !!p?.promotedContent,
    app: !!post,
    reposter: rt ? (tw.user?.screen_name ?? '') : '',
    repostedAt: rt ? (tw.created_at ?? '') : '',
    id: post?.id_str ?? '',
    handle: post?.user?.screen_name ?? '',
    name: post?.user?.name ?? '',
    text: post ? text(post) : '',
    created: post?.created_at ?? '',
    media: (post?.extended_entities?.media ?? []).map((m) => m.type),
    // As the page draws it, for when X's app data is not there.
    href: time?.closest('a')?.getAttribute('href') ?? '',
    at: time?.getAttribute('datetime') ?? '',
    user: lines(a.querySelector('[data-testid="User-Name"]')),
    shown: a.querySelector('[data-testid="tweetText"]')?.innerText ?? '',
    drawnMedia: !!a.querySelector('[data-testid="tweetPhoto"], [data-testid="videoPlayer"], [data-testid="videoComponent"]'),
  };
});
