// Runs in the page's own world, as the body of an async function, on a
// post's page, after the line `const ID = "<the post's id>";`. It returns
// the post as X's own app holds it (the React props of its article,
// 2026-10-25): null while it is not drawn yet, { gone } when X says it is
// not there, else its text, tags, media and, for an X Article, the article
// as markdown, its images as `san-media:<id>` for the caller to fill in.
const gone = [...document.querySelectorAll('[data-testid="error-detail"], [data-testid="emptyState"]')]
  .map((e) => e.innerText.trim()).find(Boolean);
if (gone) return { gone };

// props walks up from el's React fiber to the first props that pass ok.
const props = (el, ok) => {
  const k = el && Object.keys(el).find((k) => k.startsWith('__reactFiber$'));
  for (let f = k ? el[k] : null, i = 0; f && i < 80; f = f.return, i++) {
    if (f.memoizedProps && ok(f.memoizedProps)) return f.memoizedProps;
  }
  return null;
};
let tweet = null;
for (const a of document.querySelectorAll('article[data-testid="tweet"]')) {
  tweet = props(a, (p) => p.tweet?.id_str === ID)?.tweet ?? null;
  if (tweet) break;
}
if (!tweet) return null;
const t = tweet.retweeted_status?.id_str ? tweet.retweeted_status : tweet;

const unescape = (s) => s.replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
// best is a video's mp4 with the highest bitrate.
const best = (variants) => (variants ?? []).filter((v) => v.content_type === 'video/mp4')
  .sort((a, b) => (b.bitrate ?? 0) - (a.bitrate ?? 0))[0]?.url ?? '';
// orig is a photo at its full size: .../media/ABC.jpg -> ...?format=jpg&name=orig
const orig = (u) => {
  const m = /^(.*)\.(\w+)$/.exec(u ?? '');
  return m ? `${m[1]}?format=${m[2]}&name=orig` : (u ?? '');
};

// The caption: a long post's whole text (note_tweet), else full_text past
// the reply's leading @names, its t.co links written out, the links to its
// own media and article dropped.
const note = t.note_tweet?.text ? t.note_tweet : null;
const ents = note?.entity_set ?? t.entities ?? {};
let caption = note ? note.text : Array.from(t.full_text ?? '').slice(t.display_text_range?.[0] ?? 0).join('');
for (const u of ents.urls ?? []) {
  caption = caption.split(u.url).join(/\/i\/article\//.test(u.expanded_url ?? '') ? '' : (u.expanded_url ?? u.url));
}
for (const m of t.entities?.media ?? []) caption = caption.split(m.url).join('');
caption = unescape(caption).trim();

const media = (t.extended_entities?.media ?? []).map((m) => ({
  type: m.type,
  url: m.type === 'photo' ? orig(m.media_url_https) : best(m.video_info?.variants),
  alt: m.ext_alt_text ?? '',
}));

// An X Article, drawn in full on its post's page.
let article = null;
const view = document.querySelector('[data-testid="twitterArticleReadView"]');
const a = props(view, (p) => p.twitterArticle?.content_state)?.twitterArticle;
if (a) {
  const urlOf = (info) => (info?.variants ? best(info.variants) : orig(info?.original_img_url));
  const files = (a.media_entities ?? []).map((m) => ({ id: m.media_id, url: urlOf(m.media_info) }));
  const em = a.content_state.entityMap ?? {};
  const entity = (k) => (Array.isArray(em) ? em.find((e) => String(e.key) === String(k))?.value : em[k]);
  // inline writes a block's links, bold and italic in markdown.
  const inline = (b) => {
    const at = Array.from({ length: b.text.length + 1 }, () => ({ open: [], close: [] }));
    for (const r of b.entityRanges ?? []) {
      const e = entity(r.key);
      if (e?.type === 'LINK' && e.data?.url) {
        at[r.offset].open.push('[');
        at[r.offset + r.length].close.push(`](${e.data.url})`);
      }
    }
    for (const r of b.inlineStyleRanges ?? []) {
      const mark = { Bold: '**', BOLD: '**', Italic: '*', ITALIC: '*' }[r.style];
      if (mark && r.length > 0) {
        at[r.offset].open.push(mark);
        at[r.offset + r.length].close.unshift(mark);
      }
    }
    let s = '';
    for (let i = 0; i <= b.text.length; i++) {
      s += at[i].close.join('') + at[i].open.join('');
      if (i < b.text.length) s += b.text[i];
    }
    return s;
  };
  const md = [];
  for (const b of a.content_state.blocks ?? []) {
    const text = inline(b);
    const pad = '  '.repeat(b.depth ?? 0);
    switch (b.type) {
      case 'header-one': md.push('## ' + text); break;
      case 'header-two': md.push('### ' + text); break;
      case 'header-three': md.push('#### ' + text); break;
      case 'unordered-list-item': md.push(pad + '- ' + text); break;
      case 'ordered-list-item': md.push(pad + '1. ' + text); break;
      case 'blockquote': md.push('> ' + text); break;
      case 'code-block': md.push('```\n' + b.text + '\n```'); break;
      case 'atomic': {
        const e = entity(b.entityRanges?.[0]?.key);
        if (e?.type === 'MARKDOWN') md.push(e.data.markdown.trim());
        else if (e?.type === 'MEDIA') {
          for (const it of e.data?.mediaItems ?? []) md.push(`![](san-media:${it.mediaId ?? it.media_id})`);
        } else if (e?.type === 'DIVIDER') md.push('---');
        else if (e?.type === 'TWEET' && e.data?.tweetId) md.push(`https://x.com/i/status/${e.data.tweetId}`);
        break;
      }
      default:
        if (b.text.trim()) md.push(text);
    }
  }
  article = { title: a.title ?? '', cover: urlOf(a.cover_media?.media_info), markdown: md.join('\n\n'), media: files };
}

const user = t.user ?? tweet.user ?? {};
return {
  id: t.id_str,
  author: user.name ?? '',
  handle: user.screen_name ?? '',
  created: t.created_at ?? '',
  caption,
  tags: (ents.hashtags ?? []).map((h) => h.text),
  media,
  article,
};
