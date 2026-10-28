import { Box } from '@chakra-ui/react';
import cytoscape from 'cytoscape';
import type { Core, CoseLayoutOptions, EdgeSingular, ElementDefinition, StylesheetJson } from 'cytoscape';
import { useEffect, useRef, useState } from 'react';
import type { KnowledgeLink, KnowledgeNode } from '../gen/san/tower/v1/knowledge_pb';

// A knowledge's graph, drawn as the knowledge tool's view draws one
// (golang/packages/san_knowledge/ui): a node type is a colour and a shape,
// domains hexagons, docs rectangles and sections circles, sized by how many
// links they have, with a dashed border while their summary is missing.

// graphColors are the graph's colours, light and dark: cytoscape cannot read
// CSS variables. The node types take the knowledge view's first three
// palette slots. surface is the box's background (Chakra's bg.panel).
const COLORS = {
  light: {
    surface: '#ffffff',
    ink: '#18181b',
    ink2: '#52525b',
    muted: '#a1a1aa',
    edge: '#d4d4d8',
    accent: '#2563eb',
    domain: '#2a78d6',
    doc: '#eb6834',
    doc_section: '#1baf7a',
  },
  dark: {
    surface: '#111111',
    ink: '#fafafa',
    ink2: '#d4d4d8',
    muted: '#71717a',
    edge: '#3f3f46',
    accent: '#3b82f6',
    domain: '#3987e5',
    doc: '#d95926',
    doc_section: '#199e70',
  },
};
type Colors = (typeof COLORS)['light'];
export const graphColors = (dark: boolean): Colors => (dark ? COLORS.dark : COLORS.light);

// nodeTypes are the node types in the order a legend lists them.
export const nodeTypes = [
  { type: 'domain', label: 'Domain', shape: 'round-hexagon', base: 40 },
  { type: 'doc', label: 'Doc', shape: 'round-rectangle', base: 28 },
  { type: 'doc_section', label: 'Section', shape: 'ellipse', base: 16 },
] as const;
export type NodeType = (typeof nodeTypes)[number]['type'];
export const typeOf = (t: string) => nodeTypes.find((n) => n.type === t) ?? nodeTypes[2];

// useDark follows the page's dark mode (the .dark class on <html>).
export function useDark() {
  const [dark, setDark] = useState(() => document.documentElement.classList.contains('dark'));
  useEffect(() => {
    const o = new MutationObserver(() => setDark(document.documentElement.classList.contains('dark')));
    o.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    return () => o.disconnect();
  }, []);
  return dark;
}

// A force layout: a section close to its doc, docs further from their
// domain, references loose.
const idealLength = (e: EdgeSingular) => ({ section_of: 50, domain_of: 150 })[e.data('rel') as string] ?? 120;
const COSE: CoseLayoutOptions = {
  name: 'cose',
  padding: 40,
  nodeRepulsion: () => 60000,
  idealEdgeLength: idealLength,
  edgeElasticity: (e: EdgeSingular) => (e.data('rel') === 'section_of' ? 80 : 300),
  nodeOverlap: 20,
  gravity: 1,
  numIter: 2500,
  componentSpacing: 80,
  nodeDimensionsIncludeLabels: true,
};

function graphStyle(p: Colors): StylesheetJson {
  return [
    {
      selector: 'node',
      style: {
        'background-color': p.doc_section,
        // A data mapper; the types only list literal shapes.
        shape: 'data(shape)' as cytoscape.Css.NodeShape,
        width: 'data(size)',
        height: 'data(size)',
        'border-width': 2,
        'border-color': p.surface,
        label: 'data(label)',
        color: p.ink,
        'font-size': 12,
        'font-weight': 500,
        'text-valign': 'bottom',
        'text-margin-y': 5,
        'text-wrap': 'ellipsis',
        'text-max-width': '150px',
        'text-outline-color': p.surface,
        'text-outline-width': 2.5,
        'min-zoomed-font-size': 4,
        'overlay-opacity': 0,
      },
    },
    { selector: 'node[type = "domain"]', style: { 'background-color': p.domain, 'font-size': 13, 'font-weight': 700 } },
    { selector: 'node[type = "doc"]', style: { 'background-color': p.doc } },
    { selector: 'node[type = "doc_section"]', style: { 'font-size': 10, 'font-weight': 400, color: p.ink2 } },
    { selector: 'node.pending', style: { 'border-style': 'dashed', 'border-color': p.muted } },
    {
      selector: 'edge',
      style: {
        width: 1.5,
        'line-color': p.edge,
        'curve-style': 'bezier',
        'target-arrow-shape': 'triangle',
        'target-arrow-color': p.edge,
        'arrow-scale': 0.8,
        'font-size': 10,
        color: p.muted,
        'text-rotation': 'autorotate',
        'text-background-color': p.surface,
        'text-background-opacity': 1,
        'text-background-padding': '2px',
        'overlay-opacity': 0,
      },
    },
    { selector: 'edge[rel = "domain_of"]', style: { 'line-style': 'dashed' } },
    { selector: 'edge[rel = "reference"]', style: { 'line-style': 'dotted', width: 2 } },
    {
      selector: 'edge.linked',
      style: { label: 'data(rel)', 'line-color': p.ink2, 'target-arrow-color': p.ink2, width: 2 },
    },
    {
      selector: 'node.selected',
      style: { 'border-color': p.accent, 'border-width': 4, 'border-style': 'solid', 'font-weight': 700 },
    },
    {
      selector: 'node.match',
      style: { 'border-color': p.ink, 'border-width': 3, 'border-style': 'solid', 'font-weight': 700 },
    },
    { selector: '.faded', style: { opacity: 0.18 } },
  ];
}

// GraphCanvas draws nodes and links. A click on a node selects it, on the
// background nothing; a right-click selects it and asks for its menu at the
// pointer (onMenu). selected is marked and its links labelled; matches,
// when given, stand out and the rest fade. A new fit number fits the view.
// Nodes keep their places across reloads; new ones are laid out.
export function GraphCanvas({
  nodes,
  links,
  selected,
  matches,
  fit,
  onSelect,
  onMenu,
}: {
  nodes: KnowledgeNode[];
  links: KnowledgeLink[];
  selected?: string;
  matches?: Set<string>;
  fit: number;
  onSelect: (key?: string) => void;
  onMenu?: (key: string, at: { x: number; y: number }) => void;
}) {
  const box = useRef<HTMLDivElement>(null);
  const cy = useRef<Core>(null);
  const select = useRef(onSelect);
  select.current = onSelect;
  const menu = useRef(onMenu);
  menu.current = onMenu;
  const dark = useDark();

  useEffect(() => {
    const c = cytoscape({ container: box.current, minZoom: 0.1, maxZoom: 3 });
    c.on('tap', 'node', (e) => select.current(e.target.id()));
    c.on('tap', (e) => {
      if (e.target === c) select.current(undefined);
    });
    c.on('cxttap', 'node', (e) => {
      const at = e.originalEvent as MouseEvent;
      select.current(e.target.id());
      menu.current?.(e.target.id(), { x: at.clientX, y: at.clientY });
    });
    const resized = new ResizeObserver(() => c.resize());
    resized.observe(box.current!);
    cy.current = c;
    return () => {
      resized.disconnect();
      c.destroy();
      cy.current = null;
    };
  }, []);

  useEffect(() => {
    cy.current?.style(graphStyle(graphColors(dark)));
  }, [dark]);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    const degree = new Map<string, number>();
    for (const l of links) {
      degree.set(l.from, (degree.get(l.from) ?? 0) + 1);
      degree.set(l.to, (degree.get(l.to) ?? 0) + 1);
    }
    const keys = new Set(nodes.map((n) => n.key));
    const els: ElementDefinition[] = [
      ...nodes.map((n) => {
        const t = typeOf(n.type);
        return {
          group: 'nodes' as const,
          data: {
            id: n.key,
            label: n.title || n.key,
            type: n.type,
            shape: t.shape,
            size: Math.round(t.base + Math.min(degree.get(n.key) ?? 0, 12) * 2),
          },
          classes: n.needsSummary ? 'pending' : '',
        };
      }),
      ...links
        .filter((l) => keys.has(l.from) && keys.has(l.to))
        .map((l) => ({
          group: 'edges' as const,
          data: { id: `${l.from}|${l.rel}|${l.to}`, source: l.from, target: l.to, rel: l.rel },
        })),
    ];
    const had = new Map(c.nodes().map((n) => [n.id(), { ...n.position() }]));
    c.batch(() => {
      c.elements().remove();
      c.add(els);
      c.nodes().forEach((n) => {
        const p = had.get(n.id());
        if (p) n.position(p);
      });
    });
    const first = had.size === 0;
    if (first || c.nodes().filter((n) => !had.has(n.id())).nonempty()) {
      c.layout({ ...COSE, randomize: first, animate: !first, fit: first }).run();
    }
  }, [nodes, links]);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    c.batch(() => {
      c.elements().removeClass('selected linked');
      if (selected) c.getElementById(selected).addClass('selected').connectedEdges().addClass('linked');
    });
    const n = selected ? c.getElementById(selected) : undefined;
    if (n?.nonempty()) {
      const p = n.renderedPosition();
      if (p.x < 0 || p.y < 0 || p.x > c.width() || p.y > c.height()) c.animate({ center: { eles: n }, duration: 300 });
    }
  }, [selected, nodes]);

  useEffect(() => {
    const c = cy.current;
    if (!c) return;
    c.batch(() => {
      c.elements().removeClass('faded match');
      if (!matches) return;
      c.nodes().forEach((n) => {
        n.addClass(matches.has(n.id()) ? 'match' : 'faded');
      });
      c.edges().addClass('faded');
    });
  }, [matches, nodes]);

  useEffect(() => {
    const c = cy.current;
    if (fit && c && c.nodes().nonempty()) c.animate({ fit: { eles: c.elements(), padding: 40 }, duration: 300 });
  }, [fit]);

  // The browser's own menu would cover the node's.
  return <Box ref={box} position="absolute" inset="0" onContextMenu={(e) => e.preventDefault()} />;
}
