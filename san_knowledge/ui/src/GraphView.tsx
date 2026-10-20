import { Box, HStack, Text } from '@chakra-ui/react';
import cytoscape from 'cytoscape';
import type { Collection, CoseLayoutOptions, Core, EdgeSingular, ElementDefinition, Layouts, NodeSingular, Position, StylesheetJson } from 'cytoscape';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { Explanation, KNode } from './api';
import { communityLabel, communityOf, locOf, typeInfo, type Graph } from './model';
import type { Palette } from './theme';
import { Swatch } from './ui';

// Asks the graph to move the viewport; a new object each time, so asking twice works.
export type ViewRequest = { center: string } | { fit: Set<string> } | { fitAll: true } | { relayout: true };

interface Props {
  graph: Graph;
  pal: Palette;
  colorOf: (n: KNode) => string;
  shown: Set<string>; // passes the filters and is not folded away
  expanded: Set<string>;
  folded: Map<string, number>; // shown nodes with more under them, and how many
  onOpen: (key: string) => void; // click: select the node and show what is under it
  explain: Explanation | null;
  selected: string | null;
  onSelect: (key: string | null) => void;
  request: ViewRequest | null;
  active: boolean;
}

// Positions persist per browser (convenience only). Only shown nodes are laid
// out, so what a reload can restore is the top level.
const POS_KEY = 'knowledge-graph-positions-v3'; // v3: only shown nodes are laid out

function loadPositions(): Record<string, Position> {
  try { return JSON.parse(localStorage.getItem(POS_KEY) || '{}'); } catch { return {}; }
}

function savePositions(cy: Core) {
  const pos = loadPositions();
  cy.nodes().not('.hidden').forEach((n) => { pos[n.id()] = n.position(); });
  try { localStorage.setItem(POS_KEY, JSON.stringify(pos)); } catch { /* storage unavailable */ }
}

// Force layout grouped by Leiden community: edges inside a community are
// short and stiff, edges between communities long and loose.
const sameCommunity = (edge: EdgeSingular) => edge.source().data('community') === edge.target().data('community');
const COSE: CoseLayoutOptions = {
  name: 'cose', padding: 40, nodeRepulsion: () => 60000,
  idealEdgeLength: (edge: EdgeSingular) => (sameCommunity(edge) ? 55 : 190),
  edgeElasticity: (edge: EdgeSingular) => (sameCommunity(edge) ? 80 : 600),
  nodeOverlap: 20, gravity: 1, numIter: 2500, componentSpacing: 80, nodeDimensionsIncludeLabels: true,
};

// The shown nodes and the edges between them; layouts leave hidden nodes alone.
function shownElements(cy: Core): Collection {
  return cy.nodes().not('.hidden').union(cy.edges().filter((e) => !e.source().hasClass('hidden') && !e.target().hasClass('hidden')));
}

function runFullLayout(cy: Core) {
  const eles = shownElements(cy);
  const scale = Math.max(1, Math.sqrt(eles.nodes().length / 20));
  const w = Math.max(cy.width(), 600) * scale, h = Math.max(cy.height(), 400) * scale;
  eles.layout({ ...COSE, animate: false, randomize: true, boundingBox: { x1: 0, y1: 0, w, h } }).run();
  savePositions(cy);
}

// Puts nodes that appear in a ring around the shown node they hang from (or
// next to a shown neighbour), parents first so their children can ring them.
function placeAround(cy: Core, g: Graph, fresh: string[], shown: Set<string>) {
  const todo = new Set(fresh);
  const depth = (key: string) => {
    let d = 0;
    for (let u = g.upOf.get(key)?.[0]; u && d < 64; u = g.upOf.get(u)?.[0]) d++;
    return d;
  };
  const anchorOf = (key: string): string | undefined => {
    for (let u: string | undefined = key, i = 0; u && i < 64; i++) {
      const ups: string[] = g.upOf.get(u) ?? [];
      const up = ups.find((x) => shown.has(x));
      if (up) return up;
      u = ups[0]; // filtered out: what it hangs from
    }
    return cy.getElementById(key).neighborhood('node').filter((m) => shown.has(m.id()) && !todo.has(m.id())).first().id() || undefined;
  };
  const levels = new Map<number, string[]>();
  for (const k of fresh) {
    const d = depth(k);
    levels.set(d, [...(levels.get(d) ?? []), k]);
  }
  for (const d of [...levels.keys()].sort((a, b) => a - b)) {
    const rings = new Map<string, string[]>();
    for (const k of levels.get(d)!) {
      const a = anchorOf(k) ?? '';
      rings.set(a, [...(rings.get(a) ?? []), k]);
    }
    for (const [a, keys] of rings) {
      const ext = cy.extent();
      const c = a ? { ...cy.getElementById(a).position() } : { x: (ext.x1 + ext.x2) / 2, y: (ext.y1 + ext.y2) / 2 };
      const r = Math.max(80, (keys.length * 36) / (2 * Math.PI));
      keys.forEach((k, i) => {
        const t = (2 * Math.PI * i) / keys.length;
        cy.getElementById(k).position({ x: c.x + r * Math.cos(t), y: c.y + r * Math.sin(t) });
        todo.delete(k);
      });
    }
  }
}

function fitVisible(cy: Core, animate: boolean, eles?: Collection) {
  const target = eles ?? cy.nodes(':visible');
  if (target.empty()) return;
  cy.stop();
  const clamp = () => { if (cy.zoom() > 1.1) { cy.zoom(1.1); cy.center(target); } };
  if (animate) cy.animate({ fit: { eles: target, padding: 40 }, duration: 300, complete: clamp });
  else { cy.fit(target, 40); clamp(); }
}

// Moves the view to what an expansion showed when it is not in view, zooming
// out only if it does not fit at the current zoom.
function bringIntoView(cy: Core, eles: Collection) {
  if (eles.empty()) return;
  const bb = eles.boundingBox(), ext = cy.extent();
  if (bb.x1 >= ext.x1 && bb.x2 <= ext.x2 && bb.y1 >= ext.y1 && bb.y2 <= ext.y2) return;
  cy.stop();
  if (bb.w <= ext.w * 0.9 && bb.h <= ext.h * 0.9) cy.animate({ center: { eles }, duration: 300 });
  else cy.animate({ fit: { eles, padding: 40 }, duration: 300 });
}

function graphStyle(p: Palette): StylesheetJson {
  return [
    { selector: 'node', style: {
      'background-color': 'data(color)', 'shape': 'data(shape)' as cytoscape.Css.NodeShape, // a data mapper; the types only list literal shapes
      'width': 'data(size)', 'height': 'data(size)',
      'border-width': 2, 'border-color': p.surface,
      'label': 'data(label)', 'color': p.ink, 'font-size': 12, 'font-weight': 500,
      'text-valign': 'bottom', 'text-margin-y': 5, 'text-wrap': 'ellipsis', 'text-max-width': '150px',
      'text-outline-color': p.surface, 'text-outline-width': 2.5,
      'min-zoomed-font-size': 4, 'overlay-opacity': 0,
    } },
    { selector: 'node[type = "doc_section"]', style: { 'font-size': 10, 'font-weight': 400, 'color': p.ink2 } },
    { selector: 'node[type = "domain"]', style: { 'font-size': 13, 'font-weight': 700 } },
    { selector: 'node.pending', style: { 'border-style': 'dashed', 'border-color': p.muted, 'border-width': 2 } },
    { selector: 'node.folded', style: { 'border-style': 'double', 'border-color': p.ink2, 'border-width': 5 } },
    { selector: 'edge', style: {
      'width': 1.5, 'line-color': p.axis, 'curve-style': 'bezier',
      'target-arrow-shape': 'triangle', 'target-arrow-color': p.axis, 'arrow-scale': 0.8,
      'font-size': 10, 'color': p.muted, 'text-rotation': 'autorotate',
      'text-background-color': p.surface, 'text-background-opacity': 1, 'text-background-padding': '2px',
      'overlay-opacity': 0,
    } },
    { selector: 'edge[rel = "domain_of"]', style: { 'line-style': 'dashed', 'width': 1.5 } },
    { selector: 'edge[rel = "reference"]', style: { 'line-style': 'dotted', 'width': 2, 'curve-style': 'unbundled-bezier', 'control-point-distances': 30 } },
    { selector: 'edge.show-label', style: { 'label': 'data(rel)', 'line-color': p.ink2, 'target-arrow-color': p.ink2, 'width': 2 } },
    { selector: 'node.selected', style: { 'border-color': p.accent, 'border-width': 4, 'border-style': 'solid', 'font-weight': 700 } },
    { selector: 'node.match', style: { 'border-color': p.ink, 'border-width': 3, 'border-style': 'solid', 'font-weight': 700 } },
    { selector: '.faded', style: { 'opacity': 0.18 } },
    { selector: '.hidden', style: { 'display': 'none' } },
  ];
}

function nodeData(g: Graph, n: KNode, color: string) {
  const info = typeInfo(n.node_type), deg = g.degree.get(n.key) ?? 0;
  return {
    id: n.key, label: n.title || n.key, type: n.node_type, community: communityOf(g, n.key),
    color, shape: info.shape, size: Math.round(info.base + Math.min(deg, 12) * 2),
  };
}

// A folded node is marked and its label counts what clicking it would show.
function applyFolded(cy: Core, g: Graph, folded: Map<string, number>) {
  cy.batch(() => {
    cy.nodes().forEach((el) => {
      const n = g.byKey.get(el.id());
      if (!n) return;
      const more = folded.get(n.key) ?? 0;
      el.toggleClass('folded', more > 0);
      el.data('label', (n.title || n.key) + (more ? ` (+${more})` : ''));
    });
  });
}

function applyHighlight(cy: Core, ex: Explanation | null, selected: string | null) {
  cy.batch(() => {
    cy.elements().removeClass('faded match selected show-label');
    if (ex && ex.matches.length) {
      const matched = new Set(ex.matches.map((m) => m.node.key));
      const related = new Set(ex.matches.flatMap((m) => m.related.map((r) => r.key)));
      cy.nodes().forEach((el) => {
        if (matched.has(el.id())) el.addClass('match');
        else if (!related.has(el.id())) el.addClass('faded');
      });
      cy.edges().forEach((ed) => {
        const keep = (matched.has(ed.source().id()) || matched.has(ed.target().id())) &&
          !ed.source().hasClass('faded') && !ed.target().hasClass('faded');
        ed.toggleClass('faded', !keep);
        if (keep) ed.addClass('show-label');
      });
    }
    if (selected) {
      const el = cy.getElementById(selected);
      if (el.nonempty()) { el.addClass('selected'); el.connectedEdges().addClass('show-label'); }
    }
  });
}

export function GraphView({ graph, pal, colorOf, shown, expanded, folded, onOpen, explain, selected, onSelect, request, active }: Props) {
  const container = useRef<HTMLDivElement>(null);
  const tipRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const [hover, setHover] = useState<{ key: string; x: number; y: number } | null>(null);
  const restored = useRef(new Set<string>()); // placed from saved positions on the first load
  const shownBefore = useRef(new Set<string>());
  const expandedBefore = useRef(new Set<string>());
  const settling = useRef<Layouts | null>(null); // the layout that runs after nodes appear

  // The latest props, for the cytoscape handlers that are bound once.
  const latest = useRef({ graph, colorOf, expanded, folded, onOpen, explain, selected, onSelect });
  useLayoutEffect(() => { latest.current = { graph, colorOf, expanded, folded, onOpen, explain, selected, onSelect }; });

  // Runs fn once the nodes that are appearing have reached their places.
  const afterSettle = (fn: () => void) => {
    const layout = settling.current;
    if (layout) layout.one('layoutstop', () => afterSettle(fn)); else fn();
  };

  useEffect(() => {
    const box = container.current!;
    const cy = cytoscape({
      container: box, style: graphStyle(pal), minZoom: 0.1, maxZoom: 3, boxSelectionEnabled: false,
      // Redraw the whole graph (outlined labels, dashed/dotted beziers) only when
      // the viewport settles; during pan/zoom a cached bitmap is scaled instead.
      textureOnViewport: true,
    });
    cyRef.current = cy;
    cy.on('mouseover', 'node', (ev) => {
      const el = ev.target as NodeSingular, p = el.renderedPosition();
      setHover({ key: el.id(), x: p.x, y: p.y });
      el.connectedEdges().addClass('show-label');
      box.style.cursor = 'pointer';
    });
    cy.on('mouseout', 'node', (ev) => {
      const el = ev.target as NodeSingular, sel = latest.current.selected;
      setHover(null);
      box.style.cursor = '';
      if (el.id() !== sel) el.connectedEdges().removeClass('show-label');
      if (sel) cy.getElementById(sel).connectedEdges().addClass('show-label');
    });
    cy.on('tap', 'node', (ev) => latest.current.onOpen((ev.target as NodeSingular).id()));
    cy.on('tap', (ev) => { if (ev.target === cy) latest.current.onSelect(null); });
    cy.on('dragfree', 'node', () => savePositions(cy));
    // Cytoscape only notices window resizes; the bars above can wrap and change the height.
    const sizes = new ResizeObserver(() => cy.resize());
    sizes.observe(box);
    return () => { sizes.disconnect(); settling.current?.stop(); cy.destroy(); cyRef.current = null; };
  }, []);

  // New data: rebuild the elements, keeping the positions on screen (on the
  // first load, the saved ones). The effect below places nodes that appear.
  useEffect(() => {
    const cy = cyRef.current!;
    const { colorOf, folded, explain, selected } = latest.current;
    const first = cy.nodes().empty();
    const saved: Record<string, Position> = first
      ? loadPositions()
      : Object.fromEntries(cy.nodes().map((n) => [n.id(), { ...n.position() }]));
    const elements: ElementDefinition[] = [
      ...graph.nodes.map((n): ElementDefinition => ({
        group: 'nodes', data: nodeData(graph, n, colorOf(n)), classes: n.needs_summary ? 'pending' : '',
        position: saved[n.key] ? { ...saved[n.key] } : undefined,
      })),
      ...graph.links.filter((l) => graph.byKey.has(l.from) && graph.byKey.has(l.to))
        .map((l): ElementDefinition => ({ group: 'edges', data: { id: `${l.from}|${l.rel}|${l.to}`, source: l.from, target: l.to, rel: l.rel } })),
    ];
    cy.resize(); // the page around the graph may have changed size since mount
    cy.batch(() => { cy.elements().remove(); cy.add(elements); });
    if (first) restored.current = new Set(graph.nodes.filter((n) => saved[n.key]).map((n) => n.key));
    applyFolded(cy, graph, folded);
    applyHighlight(cy, explain, selected);
  }, [graph]);

  // What is shown changed (expanding, filters, new data): hide and show, put
  // nodes that appear around what they hang from, and let the graph settle.
  // Laid out from scratch only when nothing on screen has a place yet. The
  // first load fits the graph; an expansion brings what it showed into view.
  useEffect(() => {
    const cy = cyRef.current!;
    const { graph: g, expanded } = latest.current;
    cy.batch(() => cy.nodes().forEach((el) => { el.toggleClass('hidden', !shown.has(el.id())); }));
    setHover(null); // a node hidden under the pointer never gets its mouseout
    const before = shownBefore.current, keep = restored.current;
    const opened = [...expanded].filter((k) => !expandedBefore.current.has(k));
    shownBefore.current = shown;
    expandedBefore.current = expanded;
    restored.current = new Set();
    const first = before.size === 0;
    const fresh = [...shown].filter((k) => !before.has(k) && !keep.has(k));

    const prev = settling.current;
    let next: Layouts | null = null;
    if (!fresh.length) {
      if (first) fitVisible(cy, false);
    } else if (fresh.length === shown.size) {
      runFullLayout(cy);
      fitVisible(cy, false);
    } else {
      placeAround(cy, g, fresh, shown);
      const layout = shownElements(cy).layout({ ...COSE, animate: true, animationDuration: 400, randomize: false, fit: false });
      layout.one('layoutstop', () => {
        if (settling.current !== layout) return; // stopped for a newer one
        settling.current = null;
        savePositions(cy);
        if (first) fitVisible(cy, false);
        else if (opened.length) bringIntoView(cy, cy.nodes().filter((el) => fresh.includes(el.id()) || opened.includes(el.id())));
      });
      next = layout;
    }
    // Swap before stopping the old one: what waits for it then waits for the new one.
    settling.current = next;
    prev?.stop();
    next?.run();
  }, [shown]);

  useEffect(() => { cyRef.current?.style(graphStyle(pal)); }, [pal]);

  useEffect(() => {
    const cy = cyRef.current!;
    cy.batch(() => cy.nodes().forEach((el) => {
      const n = graph.byKey.get(el.id());
      if (n) el.data('color', colorOf(n));
    }));
  }, [colorOf]);

  useEffect(() => { applyFolded(cyRef.current!, latest.current.graph, folded); }, [folded]);

  useEffect(() => { applyHighlight(cyRef.current!, explain, selected); }, [explain, selected]);

  useEffect(() => {
    const cy = cyRef.current;
    if (!cy || !request || !active) return;
    afterSettle(() => {
      if ('center' in request) {
        const el = cy.getElementById(request.center);
        if (el.nonempty() && !el.hasClass('hidden')) { cy.stop(); cy.animate({ center: { eles: el }, duration: 300 }); }
      } else if ('fit' in request) {
        const eles = cy.nodes().filter((el) => request.fit.has(el.id()) && !el.hasClass('hidden'));
        if (eles.nonempty()) fitVisible(cy, true, eles);
      } else if ('fitAll' in request) {
        fitVisible(cy, true);
      } else {
        runFullLayout(cy);
        fitVisible(cy, false);
      }
    });
  }, [request]);

  // Keep the tooltip inside the graph area.
  useLayoutEffect(() => {
    const tip = tipRef.current, box = container.current;
    if (!tip || !box || !hover) return;
    const left = Math.min(hover.x + 16, box.clientWidth - tip.offsetWidth - 8);
    const top = hover.y + 16 + tip.offsetHeight > box.clientHeight ? hover.y - tip.offsetHeight - 16 : hover.y + 16;
    tip.style.left = Math.max(8, left) + 'px';
    tip.style.top = Math.max(8, top) + 'px';
  }, [hover]);

  const tipNode = hover ? graph.byKey.get(hover.key) : undefined;
  const more = tipNode ? folded.get(tipNode.key) ?? 0 : 0;
  return (
    <>
      <Box ref={container} position="absolute" inset="0" visibility={active ? 'visible' : 'hidden'} />
      {tipNode && (
        <Box ref={tipRef} role="tooltip" position="absolute" pointerEvents="none" zIndex={5} maxW="280px"
          bg="bg.panel" color="fg" borderWidth="1px" borderColor="border" rounded="lg" shadow="lg" px="10px" py="8px" fontSize="12px">
          <Text fontWeight="600" mb="2px">{tipNode.title || tipNode.key}</Text>
          <HStack gap="5px" mb="4px" color="fg.subtle">
            <Swatch type={tipNode.node_type} color={colorOf(tipNode)} size={10} />
            {[typeInfo(tipNode.node_type).label, communityLabel(communityOf(graph, tipNode.key)), locOf(tipNode), tipNode.needs_summary ? 'needs summary' : '',
              more ? `${more} more under it, click to show` : '']
              .filter(Boolean).join(' · ')}
          </HStack>
          {tipNode.summary && <Text color="fg.muted">{tipNode.summary}</Text>}
        </Box>
      )}
    </>
  );
}
