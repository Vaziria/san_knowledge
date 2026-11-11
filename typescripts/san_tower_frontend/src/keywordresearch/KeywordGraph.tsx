import { Box, Text } from '@chakra-ui/react';
import cytoscape from 'cytoscape';
import type { Core, CoseLayoutOptions, EdgeSingular, NodeCollection, NodeSingular, StylesheetJson } from 'cytoscape';
import { useEffect, useRef, useState } from 'react';
import type { KeywordCommunity, KeywordEdge, KeywordNode } from '../gen/san/tower/v1/keyword_research_pb';
import { useDark } from '../knowledge/GraphCanvas';
import { edgeKey } from './live';
import { changeText } from './trends';

// A research's graph (the user, 2026-11-11), drawn as the tower's Knowledge
// graph is (zoom, pan, fit, dark mode), grouped by Leiden community as
// san_knowledge's own view groups one: a keyword's fill is its community's
// colour, its size its search interest; the seed is the biggest, with a
// thick border; TOP links are solid and RISING ones dashed, and a keyword
// found only as Rising has a dashed border.

// COLORS are the graph's colours, light and dark, and the community
// palette of san_knowledge's view (theme.ts s1-s8; "other" past 8).
const COLORS = {
  light: {
    surface: '#ffffff',
    ink: '#18181b',
    ink2: '#52525b',
    muted: '#a1a1aa',
    edge: '#c4c4cc',
    accent: '#2563eb',
    other: '#898781',
    slots: ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#4a3aa7', '#e34948'],
  },
  dark: {
    surface: '#111111',
    ink: '#fafafa',
    ink2: '#d4d4d8',
    muted: '#71717a',
    edge: '#4b4b55',
    accent: '#3b82f6',
    other: '#898781',
    slots: ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#008300', '#9085e9', '#e66767'],
  },
};
type Colors = (typeof COLORS)['light'];
export const keywordColors = (dark: boolean): Colors => (dark ? COLORS.dark : COLORS.light);
export const communityColor = (p: Colors, c: number) => (c < p.slots.length ? p.slots[c] : p.other);

// The force layout of san_knowledge's GraphView: links inside a community
// short and stiff, links between communities long and loose, so each
// community gathers in its own place.
const sameCommunity = (e: EdgeSingular) => e.source().data('community') === e.target().data('community');
const COSE: CoseLayoutOptions = {
  name: 'cose',
  padding: 40,
  nodeRepulsion: () => 60000,
  idealEdgeLength: (e: EdgeSingular) => (sameCommunity(e) ? 55 : 190),
  edgeElasticity: (e: EdgeSingular) => (sameCommunity(e) ? 80 : 600),
  nodeOverlap: 20,
  gravity: 1,
  numIter: 2500,
  componentSpacing: 80,
  nodeDimensionsIncludeLabels: true,
};

// PLAIN_FROM is how many keywords and links make a graph big enough for
// plain links (cytoscape draws curves with arrows slowest).
const PLAIN_FROM = 600;

// size is a keyword's diameter: the seed biggest, the others by search
// interest.
const size = (n: KeywordNode) => (n.seed ? 64 : Math.round(16 + Math.min(100, Math.max(0, n.searchInterest)) * 0.34));

// nodeData is a keyword's data as cytoscape draws it.
const nodeData = (n: KeywordNode, p: Colors) => ({
  id: n.name,
  label: n.name,
  size: size(n),
  community: n.community,
  color: communityColor(p, n.community),
});

// setClasses gives a drawn keyword the classes of its kind, keeping the
// view's own (selected, faded).
function setClasses(el: NodeSingular, n: KeywordNode) {
  el.toggleClass('seed', n.seed);
  el.toggleClass('risingOnly', n.rising && !n.top && !n.seed);
  el.toggleClass('unexplored', !n.explored);
}

// pulse fades new keywords in with a ring of the accent colour that shrinks
// back to the usual border (the user, 2026-11-11: realtime, so the eye
// finds what a lookup just found).
function pulse(els: NodeCollection, p: Colors) {
  els.forEach((n) => {
    n.style({ opacity: 0, 'border-color': p.accent, 'border-width': 7 });
    n.animate(
      { style: { opacity: 1 } },
      {
        duration: 450,
        complete: () =>
          n.animate(
            { style: { 'border-width': 2 } },
            { duration: 1200, complete: () => n.removeStyle('opacity border-color border-width') },
          ),
      },
    );
  });
}

function style(p: Colors): StylesheetJson {
  return [
    {
      selector: 'node',
      style: {
        'background-color': 'data(color)',
        width: 'data(size)',
        height: 'data(size)',
        'border-width': 2,
        'border-color': p.surface,
        label: 'data(label)',
        color: p.ink,
        'font-size': 11,
        'text-valign': 'bottom',
        'text-margin-y': 4,
        'text-wrap': 'ellipsis',
        'text-max-width': '160px',
        'text-outline-color': p.surface,
        'text-outline-width': 2.5,
        'min-zoomed-font-size': 5,
        'overlay-opacity': 0,
      },
    },
    { selector: 'node.risingOnly', style: { 'border-style': 'dashed', 'border-color': p.ink2, 'border-width': 2 } },
    { selector: 'node.unexplored', style: { opacity: 0.85 } },
    {
      selector: 'node.seed',
      style: { 'border-width': 6, 'border-color': p.ink, 'font-size': 15, 'font-weight': 700 },
    },
    {
      selector: 'edge',
      style: {
        width: 1.3,
        'line-color': p.edge,
        'curve-style': 'bezier',
        'target-arrow-shape': 'triangle',
        'target-arrow-color': p.edge,
        'arrow-scale': 0.7,
        'overlay-opacity': 0,
      },
    },
    { selector: 'edge[rel = "RISING"]', style: { 'line-style': 'dashed' } },
    { selector: 'edge.plain', style: { 'curve-style': 'haystack', 'haystack-radius': 0, 'target-arrow-shape': 'none' } },
    {
      selector: 'edge.linked',
      style: { 'line-color': p.ink2, 'target-arrow-color': p.ink2, width: 2.2, 'z-index': 9 },
    },
    {
      selector: 'node.selected',
      style: { 'border-color': p.accent, 'border-width': 5, 'border-style': 'solid', 'font-weight': 700 },
    },
    { selector: '.faded', style: { opacity: 0.15 } },
  ];
}

type Tip = { x: number; y: number; node: KeywordNode };

// KeywordGraph draws a research's keywords and links. A click on a keyword
// selects it (onSelect), on the background nothing. It draws changes, not
// the whole graph again: nodes and edges are compared with what it drew by
// object (the page replaces only what a change touched, live.ts), so a
// changed keyword is updated in place (size, colour, community, kind), a
// new one is put next to the keyword it was found from, fades in with a
// pulse, and a short layout of only the new ones and the keywords they came
// from (held still) spreads them; one that is gone is taken out. A new
// layout number lays the whole graph out again by community; a new fit
// number fits it in view. While a crawl grows it, the whole graph is kept
// in view after each change until the user zooms or pans by hand (then the
// view is theirs until Fit or a layout). focus, when set, fades every
// community but that one.
export function KeywordGraph({
  nodes,
  edges,
  communities,
  selected,
  focus,
  layout,
  fit,
  onSelect,
}: {
  nodes: KeywordNode[];
  edges: KeywordEdge[];
  communities: KeywordCommunity[];
  selected?: string;
  focus?: number;
  layout: number;
  fit: number;
  onSelect: (name?: string) => void;
}) {
  const box = useRef<HTMLDivElement>(null);
  const cy = useRef<Core>(null);
  const select = useRef(onSelect);
  select.current = onSelect;
  const byName = useRef(new Map<string, KeywordNode>());
  // What it drew: each keyword and relationship as last given.
  const drawnNodes = useRef(new Map<string, KeywordNode>());
  const drawnEdges = useRef(new Map<string, KeywordEdge>());
  const plain = useRef(false);
  // moved: the user zoomed or panned by hand since the last fit.
  const moved = useRef(false);
  const [tip, setTip] = useState<Tip>();
  const dark = useDark();

  useEffect(() => {
    const c = cytoscape({ container: box.current, minZoom: 0.05, maxZoom: 3, hideEdgesOnViewport: true });
    c.on('tap', 'node', (e) => select.current(e.target.id()));
    c.on('tap', (e) => {
      if (e.target === c) select.current(undefined);
    });
    c.on('mouseover', 'node', (e) => {
      const n = byName.current.get(e.target.id());
      const p = e.target.renderedPosition();
      if (n) setTip({ x: p.x, y: p.y + e.target.renderedHeight() / 2, node: n });
    });
    c.on('mouseout', 'node', () => setTip(undefined));
    c.on('viewport', () => setTip(undefined));
    c.on('tapstart', (e) => {
      if (e.target === c) moved.current = true; // a pan starts on the background
    });
    const wheel = () => (moved.current = true);
    const el = box.current!;
    el.addEventListener('wheel', wheel, { passive: true });
    const resized = new ResizeObserver(() => c.resize());
    resized.observe(el);
    cy.current = c;
    // A new instance has drawn nothing (React's StrictMode makes it twice in
    // dev, the refs staying).
    drawnNodes.current = new Map();
    drawnEdges.current = new Map();
    plain.current = false;
    return () => {
      el.removeEventListener('wheel', wheel);
      resized.disconnect();
      c.destroy();
      cy.current = null;
    };
  }, []);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    c.style(style(keywordColors(dark)));
    const p = keywordColors(dark);
    c.nodes().forEach((n) => {
      n.data('color', communityColor(p, n.data('community') as number));
    });
  }, [dark]);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    byName.current = new Map(nodes.map((n) => [n.name, n]));
    const p = keywordColors(dark);
    const big = nodes.length + edges.length > PLAIN_FROM;
    const first = drawnNodes.current.size === 0;
    const placed = new Map(c.nodes().map((n) => [n.id(), { ...n.position() }]));
    const fresh: string[] = [];
    // A new keyword starts in a ring around the keyword it was first found
    // from, when that one has a place (drawn before, or placed just now).
    const parentOf = new Map<string, string>();
    for (const e of edges) if (!parentOf.has(e.to)) parentOf.set(e.to, e.from);
    const ring = new Map<string, string[]>();
    c.batch(() => {
      // Gone (the graph was cleared or read again): taken out.
      for (const name of [...drawnNodes.current.keys()]) {
        if (!byName.current.has(name)) {
          c.getElementById(name).remove();
          drawnNodes.current.delete(name);
          placed.delete(name);
        }
      }
      const keys = new Set(edges.map(edgeKey));
      for (const k of [...drawnEdges.current.keys()]) {
        if (!keys.has(k)) {
          c.getElementById(k).remove();
          drawnEdges.current.delete(k);
        }
      }
      // New keywords added, changed ones updated in place.
      for (const n of nodes) {
        const was = drawnNodes.current.get(n.name);
        const drawn = c.getElementById(n.name);
        if (was === n && drawn.nonempty()) continue;
        drawnNodes.current.set(n.name, n);
        if (drawn.nonempty()) {
          drawn.data(nodeData(n, p));
          setClasses(drawn, n);
          continue;
        }
        const el = c.add({ group: 'nodes', data: nodeData(n, p) });
        setClasses(el, n);
        fresh.push(n.name);
        const from = parentOf.get(n.name);
        if (from && placed.has(from)) ring.set(from, [...(ring.get(from) ?? []), n.name]);
      }
      for (const [from, names] of ring) {
        const at = placed.get(from)!;
        const r = 60 + names.length * 3;
        names.forEach((k, i) => {
          const angle = (2 * Math.PI * i) / names.length;
          const pos = { x: at.x + r * Math.cos(angle), y: at.y + r * Math.sin(angle) };
          c.getElementById(k).position(pos);
          placed.set(k, pos);
        });
      }
      for (const e of edges) {
        const k = edgeKey(e);
        const drawn = c.getElementById(k);
        if (drawnEdges.current.get(k) === e && drawn.nonempty()) continue;
        if (c.getElementById(e.from).empty() || c.getElementById(e.to).empty()) continue;
        if (drawn.empty()) {
          const data = { id: k, source: e.from, target: e.to, rel: e.rel };
          c.add({ group: 'edges', data, classes: big ? 'plain' : '' });
        }
        drawnEdges.current.set(k, e);
      }
      if (big !== plain.current) c.edges().toggleClass('plain', big);
      plain.current = big;
    });
    if (first) {
      if (c.nodes().nonempty()) runLayout(c, true);
      moved.current = false;
      return;
    }
    if (fresh.length === 0) return;
    const isFresh = new Set(fresh);
    const added = c.nodes().filter((n) => isFresh.has(n.id()));
    // Only the new keywords move; the ones they came from and those
    // keywords' other neighbours are held still (so the new ones are pushed
    // off them), and the rest of the graph is not touched. It runs at once,
    // before they fade in, so they show in their places.
    const anchors = c.nodes().filter((n) => ring.has(n.id()));
    const held = anchors.union(anchors.neighborhood('node').difference(added));
    const part = added.union(held);
    held.lock();
    const l = part.union(part.edgesWith(part)).layout({
      ...COSE,
      numIter: 300,
      randomize: false,
      animate: false,
      fit: false,
    });
    l.one('layoutstop', () => held.unlock());
    l.run();
    pulse(added, p);
    if (!moved.current) fitView(c, 400);
  }, [nodes, edges]);

  useEffect(() => {
    const c = cy.current;
    if (!layout || !c || c.nodes().empty()) return;
    runLayout(c, false);
    moved.current = false;
  }, [layout]);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    c.batch(() => {
      c.elements().removeClass('selected linked');
      if (selected) c.getElementById(selected).addClass('selected').connectedEdges().addClass('linked');
    });
    const n = selected ? c.getElementById(selected) : undefined;
    if (n?.nonempty()) {
      const at = n.renderedPosition();
      if (at.x < 0 || at.y < 0 || at.x > c.width() || at.y > c.height()) c.animate({ center: { eles: n }, duration: 300 });
    }
  }, [selected, nodes]);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    c.batch(() => {
      c.elements().removeClass('faded');
      if (focus === undefined) return;
      c.nodes().forEach((n) => {
        if (n.data('community') !== focus) n.addClass('faded');
      });
      c.edges().forEach((e) => {
        if (e.source().data('community') !== focus || e.target().data('community') !== focus) e.addClass('faded');
      });
    });
  }, [focus, nodes]);

  useEffect(() => {
    const c = cy.current;
    if (!fit || !c || c.nodes().empty()) return;
    fitView(c, 300);
    moved.current = false;
  }, [fit]);

  const communityName = (id: number) => communities.find((x) => x.id === id)?.name;
  return (
    <>
      <Box ref={box} position="absolute" inset="0" />
      {tip && (
        <Box
          position="absolute"
          left={`${tip.x}px`}
          top={`${tip.y + 18}px`}
          transform="translateX(-50%)"
          bg="bg.panel"
          borderWidth="1px"
          rounded="md"
          shadow="md"
          px="2"
          py="1"
          pointerEvents="none"
          zIndex="1"
          maxW="xs"
        >
          <Text textStyle="xs" fontWeight="semibold">
            {tip.node.name}
          </Text>
          <Text textStyle="xs" color="fg.muted">
            {[
              `search interest ${tip.node.searchInterest}`,
              changeText(tip.node) && `change ${changeText(tip.node)}`,
              `community ${tip.node.community + 1}${communityName(tip.node.community) ? ` (${communityName(tip.node.community)})` : ''}`,
            ]
              .filter(Boolean)
              .join(' · ')}
          </Text>
        </Box>
      )}
    </>
  );
}

// runLayout lays the whole graph out sorted by Leiden community (the user,
// 2026-11-12: "Is Leiden sorted?"): each community is laid out on its own,
// with the force layout inside it, then the communities are packed in rows,
// the largest first (Leiden's 0), so each stands apart in its own place and
// the links between them show how they touch. first: from random places.
// Then it fits it in view.
function runLayout(c: Core, first: boolean) {
  const groups = new Map<number, NodeCollection>();
  c.nodes().forEach((n) => {
    const id = n.data('community') as number;
    groups.set(id, (groups.get(id) ?? c.collection()).union(n));
  });
  const order = [...groups.entries()].sort((a, b) => a[0] - b[0] || b[1].length - a[1].length);
  const boxes = order.map(([, nodes]) => {
    if (nodes.length > 1) {
      nodes.union(nodes.edgesWith(nodes)).layout({
        ...COSE,
        idealEdgeLength: () => 55,
        edgeElasticity: () => 80,
        componentSpacing: 40,
        numIter: nodes.length > 300 ? 1200 : 2500,
        randomize: first,
        animate: false,
        fit: false,
      }).run();
    }
    return { nodes, bb: nodes.boundingBox({ includeLabels: true }) };
  });
  // Rows about as wide as the whole is tall, so it fits a screen.
  const gap = 120;
  const area = boxes.reduce((sum, b) => sum + (b.bb.w + gap) * (b.bb.h + gap), 0);
  const width = Math.max(Math.sqrt(area) * 1.4, ...boxes.map((b) => b.bb.w));
  let x = 0;
  let y = 0;
  let row = 0;
  c.batch(() => {
    for (const b of boxes) {
      if (x > 0 && x + b.bb.w > width) {
        x = 0;
        y += row + gap;
        row = 0;
      }
      const dx = x - b.bb.x1;
      const dy = y - b.bb.y1;
      b.nodes.positions((n) => ({ x: n.position('x') + dx, y: n.position('y') + dy }));
      x += b.bb.w + gap;
      row = Math.max(row, b.bb.h);
    }
  });
  fitView(c, 0);
}

// FIT_ZOOM is the most fitting the graph in view zooms in: a seed alone, or
// a handful of keywords, would otherwise fill the screen.
const FIT_ZOOM = 1.5;

// fitView fits the whole graph in view, animated over duration ms (0: at
// once).
function fitView(c: Core, duration: number) {
  const max = c.maxZoom();
  c.maxZoom(FIT_ZOOM); // the fit's zoom is worked out now, within it
  if (duration) c.animate({ fit: { eles: c.elements(), padding: 40 }, duration });
  else c.fit(c.elements(), 40);
  c.maxZoom(max);
}
