import type { KeywordCommunity, KeywordEdge, KeywordNode } from '../gen/san/tower/v1/keyword_research_pb';

// keywordsCsv is a research's keywords as CSV (the user, 2026-11-12: "Add
// download keyword as csv"): one row per keyword, sorted by Leiden community
// (0, the largest, first), then by search interest and change, highest
// first, each with where it was found. Download puts a byte order mark
// before it, so Excel reads it as UTF-8.
export function keywordsCsv(nodes: KeywordNode[], edges: KeywordEdge[], communities: KeywordCommunity[]): string {
  const names = new Map(communities.map((c) => [c.id, c.name]));
  const from = new Map<string, string[]>();
  for (const e of edges) {
    const how = e.rel === 'RISING' ? `Rising ${changeOf(e)}` : `Top ${e.searchInterest}`;
    from.set(e.to, [...(from.get(e.to) ?? []), `${e.from} (${how})`]);
  }
  const rows = [...nodes].sort(
    (a, b) =>
      a.community - b.community ||
      Number(b.seed) - Number(a.seed) ||
      b.searchInterest - a.searchInterest ||
      b.change - a.change ||
      a.name.localeCompare(b.name),
  );
  const lines = [
    [
      'Community',
      'Community name',
      'Keyword',
      'Type',
      'Search interest',
      'Change',
      'Breakout',
      'Depth',
      'Looked up',
      'Found from',
    ],
    ...rows.map((n) => [
      String(n.community + 1),
      names.get(n.community) ?? '',
      n.name,
      n.seed ? 'seed' : n.top && n.rising ? 'top, rising' : n.top ? 'top' : 'rising',
      n.top ? String(n.searchInterest) : '',
      n.rising ? changeOf(n) : '',
      n.breakout ? 'yes' : '',
      String(n.depth),
      n.explored ? 'yes' : '',
      (from.get(n.name) ?? []).join('; '),
    ]),
  ];
  return lines.map((l) => l.map(cell).join(',')).join('\r\n') + '\r\n';
}

// changeOf is a Rising value as Trends shows it: "+350%" or "Breakout".
const changeOf = (v: { change: number; changeText: string; breakout: boolean }) =>
  v.changeText || (v.breakout ? 'Breakout' : `+${v.change}%`);

// cell quotes a value when it holds a comma, a quote or a line break, and
// keeps a spreadsheet from reading one that starts with = or @ as a formula.
function cell(v: string) {
  const s = /^[=@]/.test(v) ? `'${v}` : v;
  return /[",\r\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

// downloadCsv saves text as a .csv file named after name.
export function downloadCsv(name: string, text: string) {
  const file = `${name.trim().replace(/[\\/:*?"<>|]+/g, '-') || 'keywords'}-keywords.csv`;
  const url = URL.createObjectURL(new Blob(['\ufeff' + text], { type: 'text/csv;charset=utf-8' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = file;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
