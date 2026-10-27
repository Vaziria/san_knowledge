import type { GraphResponse, KNode, Link } from './api';
import type { Palette } from './theme';

export type Shape = 'round-hexagon' | 'round-rectangle' | 'ellipse';

// Fixed encoding per node_type: the first three palette slots plus a distinct
// shape, so type is never shown by colour alone.
const TYPES: Record<string, { slot: number; shape: Shape; label: string; base: number }> = {
  domain: { slot: 1, shape: 'round-hexagon', label: 'domain', base: 40 },
  doc: { slot: 2, shape: 'round-rectangle', label: 'doc', base: 28 },
  doc_section: { slot: 3, shape: 'ellipse', label: 'section', base: 16 },
};
export const TYPE_ORDER = ['domain', 'doc', 'doc_section'];
export const typeInfo = (t: string) => TYPES[t] ?? { slot: 0, shape: 'ellipse' as Shape, label: t, base: 16 };

export type ColorBy = 'community' | 'type';

// The eight palette slots go to the largest communities; the rest share "other".
const PALETTE_SLOTS = 8;
const slot = (pal: Palette, i: number) => pal[`s${i}` as keyof Palette];
export const typeColor = (pal: Palette, t: string) => (typeInfo(t).slot ? slot(pal, typeInfo(t).slot) : pal.other);
export const communityColor = (pal: Palette, c: number) => (c >= 0 && c < PALETTE_SLOTS ? slot(pal, c + 1) : pal.other);
// A type swatch is neutral when colour means community.
export const typeSwatchColor = (pal: Palette, colorBy: ColorBy, t: string) => (colorBy === 'community' ? pal.ink2 : typeColor(pal, t));

export const communityLabel = (c: number) => (c >= 0 ? `community ${c + 1}` : '');
export const locOf = (n: Partial<KNode>) => (n.loc ? (n.line_loc ? `${n.loc}:${n.line_loc}` : n.loc) : '');
export const splitKeywords = (s: string) => s.split(',').map((k) => k.trim()).filter(Boolean);

export interface Graph {
  nodes: KNode[];
  links: Link[];
  byKey: Map<string, KNode>;
  degree: Map<string, number>;
  domainsOf: Map<string, string[]>; // own domain_of edges
  // What a node hangs from in the graph: a section from its parent doc or
  // section, a doc from its domains. Nodes without one are the top level.
  upOf: Map<string, string[]>;
  childrenOf: Map<string, string[]>; // the reverse of upOf
  communities: Map<string, number>; // Leiden, 0 = largest
  domains: KNode[]; // sorted by title
  keywords: string[]; // sorted
}

export function buildGraph(res: GraphResponse): Graph {
  const nodes = res.nodes ?? [];
  const links = res.links ?? [];
  const byKey = new Map(nodes.map((n) => [n.key, n]));
  const degree = new Map<string, number>();
  const domainsOf = new Map<string, string[]>();
  const parentOf = new Map<string, string>();
  for (const l of links) {
    degree.set(l.from, (degree.get(l.from) ?? 0) + 1);
    degree.set(l.to, (degree.get(l.to) ?? 0) + 1);
    if (l.rel === 'domain_of') domainsOf.set(l.from, [...(domainsOf.get(l.from) ?? []), l.to]);
    if (l.rel === 'section_of') parentOf.set(l.from, l.to);
  }
  const upOf = new Map<string, string[]>();
  const childrenOf = new Map<string, string[]>();
  for (const n of nodes) {
    const parent = parentOf.get(n.key);
    const ups = (parent ? [parent] : domainsOf.get(n.key) ?? []).filter((k) => byKey.has(k));
    if (!ups.length) continue;
    upOf.set(n.key, ups);
    for (const u of ups) childrenOf.set(u, [...(childrenOf.get(u) ?? []), n.key]);
  }
  return {
    nodes, links, byKey, degree, domainsOf, upOf, childrenOf,
    communities: new Map(Object.entries(res.communities ?? {})),
    domains: nodes.filter((n) => n.node_type === 'domain').sort((a, b) => a.title.localeCompare(b.title)),
    keywords: [...new Set(nodes.flatMap((n) => n.keyword ?? []))].sort(),
  };
}

export const EMPTY_GRAPH = buildGraph({ nodes: [], links: [], communities: {} });

export const communityOf = (g: Graph, key: string) => g.communities.get(key) ?? -1;
export const titleOf = (g: Graph, key: string) => g.byKey.get(key)?.title || key;

// A doc_section inherits its doc's domains for filtering.
export function domainsFor(g: Graph, n: KNode): string[] {
  const own = g.domainsOf.get(n.key) ?? [];
  if (own.length || n.node_type !== 'doc_section') return own;
  return g.domainsOf.get(n.loc ?? '') ?? [];
}

export interface Filters {
  hiddenTypes: Set<string>;
  hiddenCommunities: Set<number>;
  keyword: string;
  domain: string;
}

export function isVisible(g: Graph, f: Filters, n: KNode): boolean {
  if (f.hiddenTypes.has(n.node_type)) return false;
  if (f.hiddenCommunities.has(communityOf(g, n.key))) return false;
  if (f.keyword && !(n.keyword ?? []).includes(f.keyword)) return false;
  if (f.domain) {
    if (n.node_type === 'domain') return n.key === f.domain;
    if (!domainsFor(g, n).includes(f.domain)) return false;
  }
  return true;
}

// The nodes the graph shows: those that pass the filters and are not folded
// away. Everything starts folded, so only the top level shows (domains, and
// docs without a domain); a node shows once something it hangs from is
// expanded and shown. A node the filters hide folds nothing: what hangs from
// it takes its place, so a keyword filter still shows its matching sections.
export function shownNodes(g: Graph, pass: (n: KNode) => boolean, expanded: Set<string>): Set<string> {
  const memo = new Map<string, boolean>();
  const placed = (key: string): boolean => {
    const known = memo.get(key);
    if (known !== undefined) return known;
    memo.set(key, false); // guards against a section_of cycle
    const ups = g.upOf.get(key) ?? [];
    const p = !ups.length || ups.some((u) => (!pass(g.byKey.get(u)!) || expanded.has(u)) && placed(u));
    memo.set(key, p);
    return p;
  };
  return new Set(g.nodes.filter((n) => pass(n) && placed(n.key)).map((n) => n.key));
}

export const expandable = (g: Graph, key: string) => (g.childrenOf.get(key) ?? []).length > 0;

// The nodes to expand so that `key` is no longer folded away (it is selected).
export function pathTo(g: Graph, expanded: Set<string>, key: string): string[] {
  const out: string[] = [], seen = new Set<string>();
  for (let k: string | undefined = key; k && !seen.has(k);) {
    seen.add(k);
    const ups: string[] = g.upOf.get(k) ?? [];
    const up: string | undefined = ups.find((u) => expanded.has(u)) ?? ups[0];
    if (up && !expanded.has(up)) out.push(up);
    k = up;
  }
  return out;
}

// For each shown node that is folded, how many nodes expanding it would show.
export function foldedCounts(g: Graph, pass: (n: KNode) => boolean, expanded: Set<string>, shown: Set<string>): Map<string, number> {
  const reveals = (key: string, seen: Set<string>): number => (g.childrenOf.get(key) ?? []).reduce((sum, c) => {
    if (seen.has(c)) return sum;
    seen.add(c);
    return sum + (pass(g.byKey.get(c)!) ? (shown.has(c) ? 0 : 1) : reveals(c, seen));
  }, 0);
  const out = new Map<string, number>();
  for (const key of shown) {
    const n = expanded.has(key) ? 0 : reveals(key, new Set());
    if (n) out.set(key, n);
  }
  return out;
}

// A community is named after its best-connected node, preferring domains, then docs.
export function communityNames(g: Graph): Map<number, string> {
  const best = new Map<number, KNode>();
  const score = (x: KNode) => [-TYPE_ORDER.indexOf(x.node_type), g.degree.get(x.key) ?? 0];
  for (const n of g.nodes) {
    const c = communityOf(g, n.key), cur = best.get(c);
    const [a, b] = score(n), [ca, cb] = cur ? score(cur) : [-Infinity, -Infinity];
    if (!cur || a > ca || (a === ca && b > cb)) best.set(c, n);
  }
  return new Map([...best].map(([c, n]) => [c, n.title || n.key]));
}

export const toggled = <T,>(set: Set<T>, v: T) => {
  const next = new Set(set);
  if (next.has(v)) next.delete(v); else next.add(v);
  return next;
};
