import type {
  KeywordCommunity,
  KeywordEdge,
  KeywordGraphChange,
  KeywordGraphRemoval,
  KeywordNode,
} from '../gen/san/tower/v1/keyword_research_pb';

// Graph is a research's graph as its page holds it: from WatchResearch's
// snapshot, then grown by each of its changes (the user, 2026-11-11: "make
// it realtime").
export type Graph = { nodes: KeywordNode[]; edges: KeywordEdge[]; communities: KeywordCommunity[] };

export const emptyGraph: Graph = { nodes: [], edges: [], communities: [] };

// edgeKey is a relationship's id: its keywords and TOP or RISING.
export const edgeKey = (e: KeywordEdge) => `${e.from}|${e.rel}|${e.to}`;

// applyChange is g with what one lookup wrote: its keywords and
// relationships put in (new ones at the end) or replaced, the other keywords
// whose community changed moved, and the communities replaced. Keywords and
// relationships it did not touch stay the same objects, so the graph view
// redraws only what changed. Applying the same change twice gives the same
// graph.
export function applyChange(g: Graph, ch: KeywordGraphChange): Graph {
  const at = new Map(g.nodes.map((n, i) => [n.name, i]));
  const nodes = g.nodes.slice();
  for (const [name, community] of Object.entries(ch.moved)) {
    const i = at.get(name);
    if (i !== undefined && nodes[i].community !== community) nodes[i] = { ...nodes[i], community };
  }
  for (const n of ch.nodes) {
    const i = at.get(n.name);
    if (i === undefined) {
      at.set(n.name, nodes.length);
      nodes.push(n);
    } else {
      nodes[i] = n;
    }
  }
  const eat = new Map(g.edges.map((e, i) => [edgeKey(e), i]));
  const edges = g.edges.slice();
  for (const e of ch.edges) {
    const i = eat.get(edgeKey(e));
    if (i === undefined) {
      eat.set(edgeKey(e), edges.length);
      edges.push(e);
    } else {
      edges[i] = e;
    }
  }
  return { nodes, edges, communities: ch.communities };
}

// applyRemoval is g without what blacklist entries took out (the user,
// 2026-11-12: "remove the node and child"): the keywords removed and their
// relationships gone, the kept keywords it changed replaced, the other
// keywords whose community changed moved, and the communities replaced.
// Keywords and relationships it did not touch stay the same objects, so the
// graph view takes out only what went. Applying it twice gives the same
// graph.
export function applyRemoval(g: Graph, r: KeywordGraphRemoval): Graph {
  const gone = new Set(r.removed);
  const changed = new Map(r.nodes.map((n) => [n.name, n]));
  const nodes = g.nodes
    .filter((n) => !gone.has(n.name))
    .map((n) => {
      const c = changed.get(n.name);
      if (c) return c;
      const community = r.moved[n.name];
      return community !== undefined && community !== n.community ? { ...n, community } : n;
    });
  const dropped = new Set(r.edges.map(edgeKey));
  const edges = g.edges.filter((e) => !dropped.has(edgeKey(e)) && !gone.has(e.from) && !gone.has(e.to));
  return { nodes, edges, communities: r.communities };
}
