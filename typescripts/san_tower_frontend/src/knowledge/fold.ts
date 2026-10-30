import type { KnowledgeLink, KnowledgeNode } from '../gen/san/tower/v1/knowledge_pb';

// What a knowledge's page shows (the user, 2026-10-29: Research Marketing,
// 2,435 nodes, left the page blank and lagging; then "load with biggest
// connection, and when right click, we can expand or collapse"). It starts
// with the core, the most-connected nodes and the links between them, and a
// node's menu opens its neighbours, the most-connected first, a batch at a
// time. Only what is shown is drawn and laid out.

// CORE is how many of the most-connected nodes the page starts with, and
// BATCH how many neighbours each Expand adds.
export const CORE = 30;
export const BATCH = 30;

export type Net = {
  degree: Map<string, number>;
  neighbours: Map<string, string[]>; // linked either way, the most-connected first
  core: string[];
  up: Map<string, string>; // what a node hangs from: a section its parent, a doc its first domain
};

export function buildNet(nodes: KnowledgeNode[], links: KnowledgeLink[]): Net {
  const type = new Map(nodes.map((n) => [n.key, n.type]));
  const title = new Map(nodes.map((n) => [n.key, n.title || n.key]));
  const degree = new Map<string, number>();
  const linked = new Map<string, Set<string>>();
  const up = new Map<string, string>();
  for (const l of links) {
    if (!type.has(l.from) || !type.has(l.to) || l.from === l.to) continue;
    degree.set(l.from, (degree.get(l.from) ?? 0) + 1);
    degree.set(l.to, (degree.get(l.to) ?? 0) + 1);
    for (const [a, b] of [
      [l.from, l.to],
      [l.to, l.from],
    ]) {
      if (!linked.has(a)) linked.set(a, new Set());
      linked.get(a)!.add(b);
    }
    const t = type.get(l.from);
    if (!up.has(l.from) && ((l.rel === 'section_of' && t === 'doc_section') || (l.rel === 'domain_of' && t === 'doc'))) {
      up.set(l.from, l.to);
    }
  }
  const byDegree = (a: string, b: string) =>
    (degree.get(b) ?? 0) - (degree.get(a) ?? 0) || title.get(a)!.localeCompare(title.get(b)!);
  const neighbours = new Map([...linked].map(([k, set]) => [k, [...set].sort(byDegree)]));
  const core = nodes
    .map((n) => n.key)
    .sort(byDegree)
    .slice(0, CORE);
  return { degree, neighbours, core, up };
}

// pathTo is what leads to key from the top: what it hangs from, and what
// that hangs from (a doc by its first domain).
export function pathTo(net: Net, key: string): string[] {
  const path: string[] = [];
  for (let k = net.up.get(key); k && !path.includes(k) && k !== key; k = net.up.get(k)) path.push(k);
  return path;
}

export type Shown = {
  keys: Set<string>;
  anchors: Map<string, string[]>; // a node a node was opened from, to place it near
  hidden: Map<string, number>; // a shown node's neighbours that are not shown
};

// shownNodes is the core and pinned (with the paths to them), then the
// neighbours each shown, opened node opens: opened says how many of them,
// the most-connected first.
export function shownNodes(net: Net, opened: Map<string, number>, pinned: string[]): Shown {
  const keys = new Set<string>();
  const anchors = new Map<string, string[]>();
  const queue: string[] = [];
  const show = (k: string, from?: string) => {
    if (keys.has(k)) return;
    keys.add(k);
    if (from) anchors.set(k, [from]);
    queue.push(k);
  };
  net.core.forEach((k) => show(k));
  for (const key of pinned) {
    const path = pathTo(net, key);
    // From the top down, each placed near the one above it.
    [...path].reverse().forEach((k, i, top) => show(k, top[i - 1]));
    show(key, path[0]);
  }
  while (queue.length) {
    const k = queue.shift()!;
    const n = opened.get(k);
    if (n) for (const c of (net.neighbours.get(k) ?? []).slice(0, n)) show(c, k);
  }
  const hidden = new Map<string, number>();
  for (const k of keys) {
    const n = (net.neighbours.get(k) ?? []).filter((c) => !keys.has(c)).length;
    if (n) hidden.set(k, n);
  }
  return { keys, anchors, hidden };
}

// pruned is opened without the nodes that are no longer shown, so that a
// node collapsed away does not come back open.
export function pruned(opened: Map<string, number>, keys: Set<string>): Map<string, number> {
  return new Map([...opened].filter(([k]) => keys.has(k)));
}
