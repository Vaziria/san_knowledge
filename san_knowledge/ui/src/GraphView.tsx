import { Box, HStack, Text } from '@chakra-ui/react';
import cytoscape from 'cytoscape';
import type { Collection, CoseLayoutOptions, Core, EdgeSingular, ElementDefinition, Layouts, NodeSingular, Position, StylesheetJson } from 'cytoscape';
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { Explanation, KNode } from './api';
import { communityLabel, communityOf, hiddenByCollapse, locOf, typeInfo, type Graph } from './model';
import type { Palette } from './theme';
import { Swatch } from './ui';

// Asks the graph to move the viewport; a new object each time, so asking twice works.
export type ViewRequest = { center: string } | { fit: Set<string> } | { fitAll: true } | { relayout: true };

interface Props {
  graph: Graph;
  pal: Palette;
  colorOf: (n: KNode) => string;
  visible: (n: KNode) => boolean; // includes the sections hidden by a collapsed parent
  collapsed: Set<string>;
  onToggle: (key: string) => void; // double-click: collapse or expand a node's sections
  explain: Explanation | null;
  selected: string | null;
  onSelect: (key: string | null) => void;
  request: ViewRequest | null;
  active: boolean;
}

// Positions persist per browser (convenience only).
const POS_KEY = 'knowledge-graph-positions-v2'; // v2: community layout

function loadPositions(): Record<string, Position> {
  try { return JSON.parse(localStorage.getItem(POS_KEY) || '{}'); } catch { return {}; }
}

function savePositions(cy: Core) {
  const pos: Record<string, Position> = {};
  cy.nodes().forEach((n) => { pos[n.id()] = n.position(); });
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

function runFullLayout(cy: Core) {
  const scale = Math.max(1, Math.sqrt(cy.nodes().length / 20));
  const w = Math.max(cy.width(), 600) * scale, h = Math.max(cy.height(), 400) * scale;
  cy.layout({ ...COSE, animate: false, randomize: true, boundingBox: { x1: 0, y1: 0, w, h } }).run();
  savePositions(cy);
}

function fitVisible(cy: Core, animate: boolean, eles?: Collection) {
  const target = eles ?? cy.nodes(':visible');
  if (target.empty()) return;
  cy.stop();
  const clamp = () => { if (cy.zoom() > 1.1) { cy.zoom(1.1); cy.center(target); } };
  if (animate) cy.animate({ fit: { eles: target, padding: 40 }, duration: 300, complete: clamp });
  else { cy.fit(target, 40); clamp(); }
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
    { selector: 'node.collapsed', style: { 'border-style': 'double', 'border-color': p.ink2, 'border-width': 5 } },
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

function applyFilters(cy: Core, g: Graph, visible: (n: KNode) => boolean) {
  cy.batch(() => {
    cy.nodes().forEach((el) => {
      const n = g.byKey.get(el.id());
      el.toggleClass('hidden', !n || !visible(n));
    });
  });
}

// A collapsed node is marked and its label counts what it hides; the hiding
// itself is done by the filters.
function applyCollapsed(cy: Core, g: Graph, collapsed: Set<string>) {
  cy.batch(() => {
    cy.nodes().forEach((el) => {
      const n = g.byKey.get(el.id());
      if (!n) return;
      const hidden = collapsed.has(n.key) ? hiddenByCollapse(g, new Set([n.key])).size : 0;
      el.toggleClass('collapsed', hidden > 0);
      el.data('label', (n.title || n.key) + (hidden ? ` (+${hidden})` : ''));
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

export function GraphView({ graph, pal, colorOf, visible, collapsed, onToggle, explain, selected, onSelect, request, active }: Props) {
  const container = useRef<HTMLDivElement>(null);
  const tipRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const [hover, setHover] = useState<{ key: string; x: number; y: number } | null>(null);

  // The latest props, for the cytoscape handlers that are bound once.
  const latest = useRef({ graph, colorOf, visible, collapsed, onToggle, explain, selected, onSelect });
  useLayoutEffect(() => { latest.current = { graph, colorOf, visible, collapsed, onToggle, explain, selected, onSelect }; });

  useEffect(() => {
    const box = container.current!;
    const cy = cytoscape({
      container: box, style: graphStyle(pal), minZoom: 0.1, maxZoom: 3, boxSelectionEnabled: false,
      // Redraw the whole graph (outlined labels, dashed/dotted beziers) only when
      // the viewport settles; during pan/zoom a cached bitmap is scaled instead.
      textureOnViewport: true,
      // Cytoscape's default (250 ms) misses ordinary double-clicks; 500 ms is the Windows default.
      multiClickDebounceTime: 500,
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
    cy.on('tap', 'node', (ev) => latest.current.onSelect((ev.target as NodeSingular).id()));
    cy.on('dbltap', 'node', (ev) => latest.current.onToggle((ev.target as NodeSingular).id()));
    cy.on('tap', (ev) => { if (ev.target === cy) latest.current.onSelect(null); });
    cy.on('dragfree', 'node', () => savePositions(cy));
    // Cytoscape only notices window resizes; the bars above can wrap and change the height.
    const sizes = new ResizeObserver(() => cy.resize());
    sizes.observe(box);
    return () => { sizes.disconnect(); cy.destroy(); cyRef.current = null; };
  }, []);

  // New data: keep known positions, place new nodes next to a known neighbour,
  // and lay out from scratch only when nothing is known. The first load shows
  // the whole graph; later reloads keep the viewport.
  useEffect(() => {
    const cy = cyRef.current!;
    const { colorOf, visible, collapsed, explain, selected } = latest.current;
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
    const fresh = graph.nodes.filter((n) => !saved[n.key]);
    cy.resize(); // the page around the graph may have changed size since mount
    cy.batch(() => { cy.elements().remove(); cy.add(elements); });

    let settle: Layouts | undefined;
    if (fresh.length && fresh.length === graph.nodes.length) {
      runFullLayout(cy);
    } else if (fresh.length) {
      for (const n of fresh) {
        const el = cy.getElementById(n.key);
        const nb = el.neighborhood('node').filter((m) => !!saved[m.id()]);
        const p = nb.length ? nb[0].position() : { x: cy.width() / 2, y: cy.height() / 2 };
        el.position({ x: p.x + 40 + Math.random() * 40, y: p.y + 40 + Math.random() * 40 });
      }
      // This moves every node a little, so fit (on first load) only once it has stopped.
      settle = cy.layout({ ...COSE, animate: true, animationDuration: 400, randomize: false, fit: false });
      settle.one('layoutstop', () => { savePositions(cy); if (first) fitVisible(cy, false); });
      settle.run();
    }
    applyCollapsed(cy, graph, collapsed);
    applyFilters(cy, graph, visible);
    applyHighlight(cy, explain, selected);
    if ((first && !settle) || fresh.length === graph.nodes.length) fitVisible(cy, false);
    return () => { settle?.stop(); };
  }, [graph]);

  useEffect(() => { cyRef.current?.style(graphStyle(pal)); }, [pal]);

  useEffect(() => {
    const cy = cyRef.current!;
    cy.batch(() => cy.nodes().forEach((el) => {
      const n = graph.byKey.get(el.id());
      if (n) el.data('color', colorOf(n));
    }));
  }, [colorOf]);

  useEffect(() => { applyCollapsed(cyRef.current!, latest.current.graph, collapsed); }, [collapsed]);

  useEffect(() => {
    applyFilters(cyRef.current!, latest.current.graph, visible);
    setHover(null); // a node hidden under the pointer never gets its mouseout
  }, [visible]);

  useEffect(() => { applyHighlight(cyRef.current!, explain, selected); }, [explain, selected]);

  useEffect(() => {
    const cy = cyRef.current;
    if (!cy || !request || !active) return;
    if ('center' in request) {
      const el = cy.getElementById(request.center);
      if (el.nonempty() && !el.hasClass('hidden')) cy.animate({ center: { eles: el }, duration: 300 });
    } else if ('fit' in request) {
      const eles = cy.nodes().filter((el) => request.fit.has(el.id()) && !el.hasClass('hidden'));
      if (eles.nonempty()) fitVisible(cy, true, eles);
    } else if ('fitAll' in request) {
      fitVisible(cy, true);
    } else {
      runFullLayout(cy);
      fitVisible(cy, false);
    }
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
              collapsed.has(tipNode.key) ? 'collapsed, double-click to expand' : '']
              .filter(Boolean).join(' · ')}
          </HStack>
          {tipNode.summary && <Text color="fg.muted">{tipNode.summary}</Text>}
        </Box>
      )}
    </>
  );
}
