import { KeywordResearchStatus, type KeywordEdge, type KeywordFilters, type KeywordNode, type KeywordResearch } from '../gen/san/tower/v1/keyword_research_pb';

// Google Trends' filters as the Keyword Research page offers them (the user,
// 2026-11-11), each with the value Trends' own addresses use.

// times are the date ranges, in Trends' words; Custom range is two dates.
export const times = [
  { value: 'now 1-H', label: 'Past hour' },
  { value: 'now 4-H', label: 'Past 4 hours' },
  { value: 'now 1-d', label: 'Past day' },
  { value: 'now 7-d', label: 'Past 7 days' },
  { value: 'today 1-m', label: 'Past 30 days' },
  { value: 'today 3-m', label: 'Past 90 days' },
  { value: 'today 12-m', label: 'Past 12 months' },
  { value: 'today 5-y', label: 'Past 5 years' },
  { value: 'all', label: '2004 - present' },
];
export const CUSTOM = 'custom';
export const defaultTime = 'today 12-m';
export const customRange = /^(\d{4}-\d{2}-\d{2}) (\d{4}-\d{2}-\d{2})$/;

// properties are the search types.
export const properties = [
  { value: '', label: 'Web Search' },
  { value: 'images', label: 'Image search' },
  { value: 'news', label: 'News search' },
  { value: 'froogle', label: 'Google Shopping' },
  { value: 'youtube', label: 'YouTube Search' },
];

export const timeLabel = (t: string) => {
  const m = customRange.exec(t);
  return m ? `${m[1]} to ${m[2]}` : (times.find((o) => o.value === t)?.label ?? t);
};

// exploreLink is the new Explore's address for a keyword with a research's
// filters, its defaults left out, as San Remote opens it.
export function exploreLink(keyword: string, f?: KeywordFilters) {
  const enc = (s: string) => encodeURIComponent(s);
  const v = [`q=${enc(keyword)}`];
  if (f?.geo) v.push(`geo=${enc(f.geo)}`);
  if (f?.time && f.time !== defaultTime) v.push(`date=${enc(f.time)}`);
  if (f?.category) v.push(`cat=${f.category}`);
  if (f?.property) v.push(`gprop=${enc(f.property)}`);
  return `https://trends.google.com/explore?${v.join('&')}`;
}

// MAX_TOP is about the most Top queries the new Explore lists for a keyword
// (48 for "aplikasi kasir", 2026-11-11): an empty graph's estimate takes it.
export const MAX_TOP = 50;

// explorable says whether Run looks a keyword up under a deep limit: the
// seed and Top keywords not looked up, above the limit (as the crawler).
export const explorable = (n: KeywordNode, limit: number) => !n.explored && (n.seed || n.top) && n.depth < limit;

// estimate is how many lookups Run makes at most: each explorable keyword,
// and below each, up to perKeyword Top keywords a level, to the deep limit.
// perKeyword is the graph's own average of Top keywords per lookup, or
// MAX_TOP while it has none. It is worked out from the page's graph, so a
// blacklist's removal lowers it at once, and the average counts only the
// Top keywords kept, as a crawl skips blacklisted ones.
export function estimate(nodes: KeywordNode[], edges: KeywordEdge[], limit: number) {
  const explored = nodes.filter((n) => n.explored).length;
  const tops = edges.filter((e) => e.rel === 'TOP').length;
  const per = explored > 0 ? Math.max(1, Math.round(tops / explored)) : MAX_TOP;
  const list = nodes.length > 0 ? nodes : [{ seed: true, explored: false, top: false, depth: 0 } as KeywordNode];
  let total = 0;
  for (const n of list) {
    if (!explorable(n, limit)) continue;
    let level = 1;
    for (let d = n.depth; d < limit && total < 1e7; d++) {
      total += level;
      level *= per;
    }
  }
  return { lookups: total, perKeyword: per };
}

// SECONDS_PER_LOOKUP is how long a lookup and the pause after it take: the
// page needs ~4-7 s, the pause 3-7 s.
const SECONDS_PER_LOOKUP = [8, 14];

// duration is roughly how long n lookups take: "about 3-5 min", "about 1-2 h".
export function duration(n: number) {
  const [lo, hi] = SECONDS_PER_LOOKUP.map((s) => s * n);
  const fmt = (s: number, unit: 'min' | 'h') => Math.max(1, Math.round(unit === 'h' ? s / 3600 : s / 60));
  if (hi < 60) return 'under a minute';
  if (hi < 2 * 3600) return `about ${fmt(lo, 'min')}-${fmt(hi, 'min')} min`;
  if (hi < 72 * 3600) return `about ${fmt(lo, 'h')}-${fmt(hi, 'h')} h`;
  return `about ${Math.round(lo / 86400)}-${Math.round(hi / 86400)} days`;
}

// statusText is a research's status as a word, and its colour.
export function statusOf(k: KeywordResearch): { label: string; color: string } {
  switch (k.status) {
    case KeywordResearchStatus.WAITING:
      return { label: 'Waiting', color: 'orange' };
    case KeywordResearchStatus.RUNNING:
      return { label: 'Running', color: 'blue' };
    case KeywordResearchStatus.STOPPED:
      return { label: 'Stopped', color: 'gray' };
    case KeywordResearchStatus.DONE:
      return { label: 'Done', color: 'green' };
    case KeywordResearchStatus.FAILED:
      return { label: 'Failed', color: 'red' };
  }
  return { label: 'Idle', color: 'gray' };
}

export const active = (k?: KeywordResearch) =>
  k?.status === KeywordResearchStatus.RUNNING || k?.status === KeywordResearchStatus.WAITING;

// progressText is a running research's line: "Running · 48 keywords · depth
// 2 · now: aplikasi kasir android" (the user, 2026-11-11), or a waiting
// one's place in the queue.
export function progressText(k: KeywordResearch) {
  const p = k.progress;
  const n = `${k.nodeCount} ${k.nodeCount === 1 ? 'keyword' : 'keywords'}`;
  if (k.status === KeywordResearchStatus.WAITING) {
    return `Waiting · ${n} · another research's crawl runs; this one is ${ordinal(p?.queuePosition ?? 2)} in the queue`;
  }
  if (k.status !== KeywordResearchStatus.RUNNING) return '';
  const parts = ['Running', n];
  if (p?.keyword) parts.push(`depth ${p.depth}`, `now: ${p.keyword}`);
  if (p?.lookups) parts.push(`${p.lookups} looked up`);
  if (p?.todo) parts.push(`${p.todo} to go at this depth or above`);
  if (p?.note) parts.push(p.note);
  return parts.join(' · ');
}

const ordinal = (n: number) => `${n}${n % 10 === 1 && n % 100 !== 11 ? 'st' : n % 10 === 2 && n % 100 !== 12 ? 'nd' : n % 10 === 3 && n % 100 !== 13 ? 'rd' : 'th'}`;

// changeText is a change as Trends writes it, or "" when there is none.
export const changeText = (n: { change: number; changeText: string; breakout: boolean }) =>
  n.changeText || (n.breakout ? 'Breakout' : n.change ? `${n.change > 0 ? '+' : ''}${n.change}%` : '');
