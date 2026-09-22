/* Knowledge Graph UI. Vanilla JS + cytoscape. All document/AI content is
   rendered with textContent (never innerHTML). */
(() => {
  'use strict';

  // Fixed encoding per node_type: the first three palette slots (validated
  // all-pairs) plus a distinct shape, so type is never shown by colour alone.
  const TYPES = {
    domain: { slot: 1, shape: 'round-hexagon', label: 'domain', base: 40 },
    doc: { slot: 2, shape: 'round-rectangle', label: 'doc', base: 28 },
    doc_section: { slot: 3, shape: 'ellipse', label: 'section', base: 16 },
  };
  const TYPE_ORDER = ['domain', 'doc', 'doc_section'];

  const state = {
    nodes: [], links: [], byKey: new Map(), degree: new Map(), domainsOf: new Map(),
    hiddenTypes: new Set(), keyword: '', domain: '',
    // Leiden communities from /api/graph (0 = largest). Nodes are coloured by
    // community by default; shape still shows the type.
    communities: new Map(), hiddenCommunities: new Set(), colorBy: 'community',
    selected: null, explain: null, explainMarkdown: '', mode: 'graph', ai: false,
    sort: { col: 'title', dir: 1 },
  };

  const $ = (id) => document.getElementById(id);
  const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

  function h(tag, attrs, ...children) {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      if (v == null || v === false) continue;
      if (k === 'class') el.className = v;
      else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
      else if (k === 'text') el.textContent = v;
      else el.setAttribute(k, v === true ? '' : v);
    }
    for (const c of children.flat()) {
      if (c == null || c === false) continue;
      el.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return el;
  }

  const typeInfo = (t) => TYPES[t] || { slot: 0, shape: 'ellipse', label: t, base: 16 };
  const typeColor = (t) => (typeInfo(t).slot ? css('--s' + typeInfo(t).slot) : css('--other'));
  // The eight palette slots go to the largest communities; the rest share --other.
  const PALETTE = 8;
  const communityOf = (key) => state.communities.get(key) ?? -1;
  const communityColor = (c) => (c >= 0 && c < PALETTE ? css('--s' + (c + 1)) : css('--other'));
  const nodeColor = (n) => (state.colorBy === 'community' ? communityColor(communityOf(n.key)) : typeColor(n.node_type));
  const titleOf = (key) => (state.byKey.get(key) || {}).title || key;
  const locOf = (n) => (n.loc ? (n.line_loc ? `${n.loc}:${n.line_loc}` : n.loc) : '');

  // A type swatch is neutral when colour means community; pass color to override.
  function swatch(type, size = 12, color) {
    const ns = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(ns, 'svg');
    svg.setAttribute('width', size); svg.setAttribute('height', size);
    svg.setAttribute('viewBox', '0 0 12 12'); svg.setAttribute('aria-hidden', 'true');
    const shape = typeInfo(type).shape;
    let el;
    if (shape === 'round-hexagon') {
      el = document.createElementNS(ns, 'path'); el.setAttribute('d', 'M3 1h6l3 5-3 5H3L0 6z');
    } else if (shape === 'round-rectangle') {
      el = document.createElementNS(ns, 'path'); el.setAttribute('d', 'M2 1h8a2 2 0 012 2v6a2 2 0 01-2 2H2a2 2 0 01-2-2V3a2 2 0 012-2z');
    } else {
      el = document.createElementNS(ns, 'circle'); el.setAttribute('cx', 6); el.setAttribute('cy', 6); el.setAttribute('r', 5.5);
    }
    el.setAttribute('fill', color || (state.colorBy === 'community' ? css('--ink-2') : typeColor(type)));
    svg.append(el);
    return svg;
  }

  // ---------- API ----------
  async function api(method, path, body) {
    const res = await fetch(path, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(data.error || res.statusText);
    return data;
  }
  const q = (key) => encodeURIComponent(key);

  function toast(msg, isError) {
    const t = $('toast');
    t.textContent = msg;
    t.className = 'toast' + (isError ? ' error' : '');
    t.hidden = false;
    clearTimeout(toast.timer);
    toast.timer = setTimeout(() => { t.hidden = true; }, isError ? 6000 : 2600);
  }

  async function busy(text, fn) {
    $('busy-text').textContent = text;
    $('busy').hidden = false;
    for (const id of ['btn-sync', 'btn-ai', 'btn-fetch']) $(id).disabled = true;
    try { return await fn(); } finally {
      $('busy').hidden = true;
      for (const id of ['btn-sync', 'btn-ai', 'btn-fetch']) $(id).disabled = false;
    }
  }

  // ---------- positions persist per browser (convenience only) ----------
  const POS_KEY = 'knowledge-graph-positions-v2'; // v2: community layout
  function loadPositions() {
    try { return JSON.parse(localStorage.getItem(POS_KEY) || '{}'); } catch { return {}; }
  }
  function savePositions() {
    if (!cy) return;
    const pos = {};
    cy.nodes().forEach((n) => { pos[n.id()] = n.position(); });
    try { localStorage.setItem(POS_KEY, JSON.stringify(pos)); } catch { /* storage unavailable */ }
  }

  // ---------- graph ----------
  let cy = null;

  // Force layout grouped by Leiden community: edges inside a community are
  // short and stiff, edges between communities long and loose.
  const sameCommunity = (edge) => edge.source().data('community') === edge.target().data('community');
  const COSE = {
    name: 'cose', padding: 40, nodeRepulsion: () => 60000,
    idealEdgeLength: (edge) => (sameCommunity(edge) ? 55 : 190),
    edgeElasticity: (edge) => (sameCommunity(edge) ? 80 : 600),
    nodeOverlap: 20, gravity: 1, numIter: 2500, componentSpacing: 80, nodeDimensionsIncludeLabels: true,
  };

  function runFullLayout() {
    const scale = Math.max(1, Math.sqrt(cy.nodes().length / 20));
    const w = Math.max(cy.width(), 600) * scale, hgt = Math.max(cy.height(), 400) * scale;
    cy.layout({ ...COSE, animate: false, randomize: true, boundingBox: { x1: 0, y1: 0, w, h: hgt } }).run();
    savePositions();
  }

  function graphStyle() {
    const surface = css('--surface'), ink = css('--ink'), ink2 = css('--ink-2');
    const axis = css('--axis'), accent = css('--accent'), muted = css('--muted');
    return [
      { selector: 'node', style: {
        'background-color': 'data(color)', 'shape': 'data(shape)',
        'width': 'data(size)', 'height': 'data(size)',
        'border-width': 2, 'border-color': surface,
        'label': 'data(label)', 'color': ink, 'font-size': 12, 'font-weight': 500,
        'text-valign': 'bottom', 'text-margin-y': 5, 'text-wrap': 'ellipsis', 'text-max-width': 150,
        'text-outline-color': surface, 'text-outline-width': 2.5,
        'min-zoomed-font-size': 4, 'overlay-opacity': 0,
      } },
      { selector: 'node[type = "doc_section"]', style: { 'font-size': 10, 'font-weight': 400, 'color': ink2 } },
      { selector: 'node[type = "domain"]', style: { 'font-size': 13, 'font-weight': 700 } },
      { selector: 'node.pending', style: { 'border-style': 'dashed', 'border-color': muted, 'border-width': 2 } },
      { selector: 'edge', style: {
        'width': 1.5, 'line-color': axis, 'curve-style': 'bezier',
        'target-arrow-shape': 'triangle', 'target-arrow-color': axis, 'arrow-scale': 0.8,
        'font-size': 10, 'color': muted, 'text-rotation': 'autorotate',
        'text-background-color': surface, 'text-background-opacity': 1, 'text-background-padding': 2,
        'overlay-opacity': 0,
      } },
      { selector: 'edge[rel = "domain_of"]', style: { 'line-style': 'dashed', 'width': 1.5 } },
      { selector: 'edge[rel = "reference"]', style: { 'line-style': 'dotted', 'width': 2, 'curve-style': 'unbundled-bezier', 'control-point-distances': 30 } },
      { selector: 'edge.show-label', style: { 'label': 'data(rel)', 'line-color': ink2, 'target-arrow-color': ink2, 'width': 2 } },
      { selector: 'node.selected', style: { 'border-color': accent, 'border-width': 4, 'border-style': 'solid', 'font-weight': 700 } },
      { selector: 'node.match', style: { 'border-color': ink, 'border-width': 3, 'border-style': 'solid', 'font-weight': 700 } },
      { selector: '.faded', style: { 'opacity': 0.18 } },
      { selector: '.hidden', style: { 'display': 'none' } },
    ];
  }

  function nodeData(n) {
    const deg = state.degree.get(n.key) || 0, info = typeInfo(n.node_type);
    return {
      id: n.key, label: n.title || n.key, type: n.node_type, community: communityOf(n.key),
      color: nodeColor(n), shape: info.shape,
      size: Math.round(info.base + Math.min(deg, 12) * 2),
    };
  }

  function renderGraph() {
    const saved = cy ? Object.fromEntries(cy.nodes().map((n) => [n.id(), n.position()])) : loadPositions();
    const elements = [
      ...state.nodes.map((n) => ({
        group: 'nodes', data: nodeData(n), classes: n.needs_summary ? 'pending' : '',
        position: saved[n.key] ? { ...saved[n.key] } : undefined,
      })),
      ...state.links.filter((l) => state.byKey.has(l.from) && state.byKey.has(l.to))
        .map((l) => ({ group: 'edges', data: { id: `${l.from}|${l.rel}|${l.to}`, source: l.from, target: l.to, rel: l.rel } })),
    ];
    const fresh = state.nodes.filter((n) => !saved[n.key]);

    if (!cy) {
      cy = cytoscape({
        container: $('graph'), elements, style: graphStyle(),
        minZoom: 0.1, maxZoom: 3, boxSelectionEnabled: false,
        // Redraw the whole graph (outlined labels, dashed/dotted beziers) only when
        // the viewport settles; during pan/zoom a cached bitmap is scaled instead.
        textureOnViewport: true,
      });
      bindGraphEvents();
    } else {
      cy.elements().remove();
      cy.add(elements);
    }

    if (fresh.length && fresh.length === state.nodes.length) {
      runFullLayout();
    } else if (fresh.length) {
      for (const n of fresh) {
        const el = cy.getElementById(n.key);
        const nb = el.neighborhood('node').filter((m) => saved[m.id()]);
        const p = nb.length ? nb[0].position() : { x: cy.width() / 2, y: cy.height() / 2 };
        el.position({ x: p.x + 40 + Math.random() * 40, y: p.y + 40 + Math.random() * 40 });
      }
      cy.layout({ ...COSE, animate: true, animationDuration: 400, randomize: false, fit: false }).run();
      setTimeout(savePositions, 500);
    }
    applyFilters();
    applyHighlight();
    if (fresh.length === state.nodes.length) fitVisible(false);
  }

  function fitVisible(animate, eles) {
    if (!cy) return;
    const target = eles || cy.nodes(':visible');
    if (target.empty()) return;
    cy.stop();
    const clamp = () => { if (cy.zoom() > 1.1) { cy.zoom(1.1); cy.center(target); } };
    if (animate) cy.animate({ fit: { eles: target, padding: 40 }, duration: 300, complete: clamp });
    else { cy.fit(target, 40); clamp(); }
  }

  function bindGraphEvents() {
    const tip = $('tooltip');
    cy.on('mouseover', 'node', (ev) => {
      const n = state.byKey.get(ev.target.id());
      if (!n) return;
      tip.replaceChildren(...[
        h('div', { class: 'tt-title', text: n.title || n.key }),
        h('div', { class: 'tt-kind' }, swatch(n.node_type, 10, nodeColor(n)), [typeInfo(n.node_type).label, communityLabel(communityOf(n.key)), locOf(n), n.needs_summary ? 'needs summary' : ''].filter(Boolean).join(' · ')),
        n.summary ? h('div', { class: 'tt-sum', text: n.summary }) : null,
      ].filter(Boolean));
      const p = ev.target.renderedPosition(), box = $('graph').getBoundingClientRect();
      tip.hidden = false;
      const left = Math.min(p.x + 16, box.width - tip.offsetWidth - 8);
      const top = p.y + 16 + tip.offsetHeight > box.height ? p.y - tip.offsetHeight - 16 : p.y + 16;
      tip.style.left = Math.max(8, left) + 'px';
      tip.style.top = Math.max(8, top) + 'px';
      ev.target.connectedEdges().addClass('show-label');
      $('graph').style.cursor = 'pointer';
    });
    cy.on('mouseout', 'node', (ev) => {
      tip.hidden = true;
      $('graph').style.cursor = '';
      if (ev.target.id() !== state.selected) ev.target.connectedEdges().removeClass('show-label');
      if (state.selected) cy.getElementById(state.selected).connectedEdges().addClass('show-label');
    });
    cy.on('tap', 'node', (ev) => select(ev.target.id()));
    cy.on('tap', (ev) => { if (ev.target === cy) select(null); });
    cy.on('dragfree', 'node', savePositions);
  }

  // A doc_section inherits its doc's domains for filtering.
  function domainsFor(n) {
    const own = state.domainsOf.get(n.key) || [];
    if (own.length || n.node_type !== 'doc_section') return own;
    return state.domainsOf.get(n.loc) || [];
  }

  function visible(n) {
    if (state.hiddenTypes.has(n.node_type)) return false;
    if (state.hiddenCommunities.has(communityOf(n.key))) return false;
    if (state.keyword && !(n.keyword || []).includes(state.keyword)) return false;
    if (state.domain) {
      if (n.node_type === 'domain') return n.key === state.domain;
      if (!domainsFor(n).includes(state.domain)) return false;
    }
    return true;
  }

  function applyFilters() {
    if (cy) {
      cy.batch(() => {
        cy.nodes().forEach((el) => {
          const n = state.byKey.get(el.id());
          el.toggleClass('hidden', !n || !visible(n));
        });
      });
    }
    if (state.mode === 'table') renderTable();
  }

  function applyHighlight() {
    if (!cy) return;
    cy.batch(() => {
      cy.elements().removeClass('faded match selected show-label');
      const ex = state.explain;
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
      if (state.selected) {
        const el = cy.getElementById(state.selected);
        if (el.nonempty()) { el.addClass('selected'); el.connectedEdges().addClass('show-label'); }
      }
    });
    $('btn-clear').hidden = !state.explain;
  }

  // ---------- table ----------
  function renderTable() {
    const { col, dir } = state.sort;
    const val = (n) => ({
      title: (n.title || n.key).toLowerCase(), type: TYPE_ORDER.indexOf(n.node_type), loc: `${n.loc || ''}:${String(n.line_loc || 0).padStart(6, '0')}`,
      domain: domainsFor(n).map(titleOf).join(','), keyword: (n.keyword || []).join(','),
      status: n.needs_summary ? 0 : n.summary ? 2 : 1,
    })[col];
    const rows = state.nodes.filter(visible).sort((a, b) => (val(a) > val(b) ? dir : val(a) < val(b) ? -dir : 0));
    $('table-body').replaceChildren(...rows.map((n) => h('tr', {
      class: n.key === state.selected ? 'selected' : '', tabindex: 0,
      onclick: () => select(n.key),
      onkeydown: (ev) => { if (ev.key === 'Enter') select(n.key); },
    },
      h('td', {}, h('div', { text: n.title || n.key }), n.node_type === 'domain' ? h('div', { class: 'key', text: n.key }) : null),
      h('td', {}, h('span', { class: 'cell-kind' }, swatch(n.node_type), typeInfo(n.node_type).label)),
      h('td', { class: 'loc', text: locOf(n) }),
      h('td', { class: 'muted', text: domainsFor(n).map(titleOf).join(', ') }),
      h('td', { class: 'muted', text: (n.keyword || []).join(', ') }),
      h('td', {}, n.needs_summary ? h('span', { class: 'badge', text: 'needs summary' }) : h('span', { class: 'muted', text: n.summary ? 'done' : '—' })),
    )));
    document.querySelectorAll('.data-table th').forEach((th) => {
      const active = th.dataset.sort === col;
      th.setAttribute('aria-sort', active ? (dir > 0 ? 'ascending' : 'descending') : 'none');
      th.textContent = th.textContent.replace(/ [▲▼]$/, '') + (active ? (dir > 0 ? ' ▲' : ' ▼') : '');
    });
  }

  function setMode(mode) {
    state.mode = mode;
    for (const [id, m] of [['mode-graph', 'graph'], ['mode-table', 'table']]) {
      $(id).classList.toggle('active', mode === m);
      $(id).setAttribute('aria-pressed', mode === m);
    }
    $('graph').style.visibility = mode === 'graph' ? 'visible' : 'hidden';
    $('table-wrap').hidden = mode !== 'table';
    $('btn-fit').hidden = mode !== 'graph';
    if (mode === 'table') renderTable();
    else if (cy) cy.resize();
  }

  // ---------- legend & filters ----------
  function renderFilters() {
    const counts = {};
    for (const n of state.nodes) counts[n.node_type] = (counts[n.node_type] || 0) + 1;
    $('legend').replaceChildren(...TYPE_ORDER.map((t) => h('button', {
      class: 'chip', type: 'button', 'aria-pressed': String(!state.hiddenTypes.has(t)),
      title: state.hiddenTypes.has(t) ? `Show ${typeInfo(t).label}s` : `Hide ${typeInfo(t).label}s`,
      onclick: () => {
        if (state.hiddenTypes.has(t)) state.hiddenTypes.delete(t); else state.hiddenTypes.add(t);
        renderFilters(); applyFilters();
      },
    }, swatch(t), typeInfo(t).label, h('span', { class: 'n', text: counts[t] || 0 }))));

    for (const [id, c] of [['color-community', 'community'], ['color-type', 'type']]) {
      $(id).classList.toggle('active', state.colorBy === c);
      $(id).setAttribute('aria-pressed', String(state.colorBy === c));
    }
    renderCommunityLegend();

    const domains = state.nodes.filter((n) => n.node_type === 'domain').sort((a, b) => a.title.localeCompare(b.title));
    const dsel = $('domain-select');
    dsel.replaceChildren(h('option', { value: '', text: 'All domains' }), ...domains.map((d) => h('option', { value: d.key, text: d.title })));
    dsel.value = domains.some((d) => d.key === state.domain) ? state.domain : '';
    state.domain = dsel.value;

    const keywords = [...new Set(state.nodes.flatMap((n) => n.keyword || []))].sort();
    const ksel = $('keyword-select');
    ksel.replaceChildren(h('option', { value: '', text: 'All keywords' }), ...keywords.map((k) => h('option', { value: k, text: k })));
    ksel.value = keywords.includes(state.keyword) ? state.keyword : '';
    state.keyword = ksel.value;
  }

  // A community is named after its best-connected node, preferring domains, then docs.
  function communityNames() {
    const best = new Map();
    for (const n of state.nodes) {
      const c = communityOf(n.key), cur = best.get(c);
      const score = (x) => [-TYPE_ORDER.indexOf(x.node_type), state.degree.get(x.key) || 0];
      const [a, b] = score(n), [ca, cb] = cur ? score(cur) : [-Infinity, -Infinity];
      if (!cur || a > ca || (a === ca && b > cb)) best.set(c, n);
    }
    return new Map([...best].map(([c, n]) => [c, n.title || n.key]));
  }
  const communityLabel = (c) => (c >= 0 ? `community ${c + 1}` : '');

  function renderCommunityLegend() {
    const box = $('communities');
    box.hidden = state.colorBy !== 'community' || state.communities.size === 0;
    if (box.hidden) return;
    const counts = new Map();
    for (const n of state.nodes) counts.set(communityOf(n.key), (counts.get(communityOf(n.key)) || 0) + 1);
    const names = communityNames();
    const ids = [...counts.keys()].sort((a, b) => a - b);
    box.replaceChildren(...ids.map((c) => {
      const hidden = state.hiddenCommunities.has(c);
      return h('button', {
        class: 'chip', type: 'button', 'aria-pressed': String(!hidden),
        title: `${hidden ? 'Show' : 'Hide'} community ${c + 1}: ${names.get(c)}`,
        onclick: () => {
          if (hidden) state.hiddenCommunities.delete(c); else state.hiddenCommunities.add(c);
          renderFilters(); applyFilters();
        },
      }, swatch('doc_section', 12, communityColor(c)), h('span', { class: 't', text: names.get(c) }), h('span', { class: 'n', text: counts.get(c) }));
    }));
  }

  function setColorBy(mode) {
    state.colorBy = mode;
    try { localStorage.setItem('knowledge-graph-color', mode); } catch { /* ignore */ }
    if (cy) cy.nodes().forEach((el) => { const n = state.byKey.get(el.id()); if (n) el.data('color', nodeColor(n)); });
    renderFilters();
    renderPanel();
    if (state.mode === 'table') renderTable();
  }

  // ---------- panel ----------
  // replaceChildren would print null/false as text, so drop empty slots.
  function panel(...children) { $('panel').replaceChildren(...children.flat().filter((c) => c != null && c !== false)); }

  function nodeRow(key, meta, onRemove) {
    const n = state.byKey.get(key) || { key, title: key, node_type: '' };
    return h('li', { class: 'conn' },
      h('span', { style: 'padding-top:4px' }, swatch(n.node_type, 12, nodeColor(n))),
      h('div', {},
        h('button', { class: 'link', type: 'button', text: n.title || key, onclick: () => select(key, true) }),
        meta ? h('span', { class: 'meta', text: meta }) : null,
      ),
      onRemove ? h('button', { class: 'btn ghost small rm', type: 'button', title: 'Remove', 'aria-label': `Remove ${n.title || key}`, text: '✕', onclick: onRemove }) : h('span'),
    );
  }

  function renderOverview() {
    const pending = state.nodes.filter((n) => n.needs_summary);
    const noDomain = state.nodes.filter((n) => n.node_type === 'doc' && !(state.domainsOf.get(n.key) || []).length);
    const domains = state.nodes.filter((n) => n.node_type === 'domain').sort((a, b) => a.title.localeCompare(b.title));
    const docs = state.nodes.filter((n) => n.node_type === 'doc').sort((a, b) => a.key.localeCompare(b.key));
    panel(
      h('div', { class: 'hint' },
        h('h2', { text: 'Explore the graph' }),
        h('p', { text: 'Click a node for details. Explain searches docs, sections and domains and highlights the context around a question.' }),
      ),
      pending.length ? h('p', { class: 'hint' }, h('span', { class: 'badge', text: `${pending.length} need a summary` }), ' ',
        state.ai ? 'Use “Summarize with AI”, or ' : '', 'run ', h('code', { text: 'knowledge sync' }), '.') : null,
      h('h3', { text: `Domains (${domains.length})` }),
      domains.length ? h('ul', { class: 'conn-list' }, domains.map((d) => nodeRow(d.key, d.summary))) :
        h('p', { class: 'footnote', text: 'No domains yet. Add one, or let the AI assign them.' }),
      h('h3', { text: `Docs (${docs.length})` }),
      h('ul', { class: 'conn-list' }, docs.map((d) => nodeRow(d.key, [d.loc, (state.domainsOf.get(d.key) || []).map(titleOf).join(', ')].filter(Boolean).join(' · ')))),
      noDomain.length ? h('p', { class: 'footnote', text: `${noDomain.length} doc(s) without a domain.` }) : null,
    );
  }

  async function renderDetail(key) {
    let data;
    try {
      data = await api('GET', `/api/node?key=${q(key)}`);
    } catch (err) {
      toast(err.message, true);
      select(null);
      return;
    }
    if (state.selected !== key) return;
    const n = data.node;
    const isDomain = n.node_type === 'domain';
    const domainLinks = data.out.filter((l) => l.rel === 'domain_of');
    const parent = data.out.find((l) => l.rel === 'section_of');
    const children = data.in.filter((l) => l.rel === 'section_of')
      .sort((a, b) => ((state.byKey.get(a.from) || {}).line_loc || 0) - ((state.byKey.get(b.from) || {}).line_loc || 0));
    const members = data.in.filter((l) => l.rel === 'domain_of');
    const refsOut = data.out.filter((l) => l.rel === 'reference');
    const refsIn = data.in.filter((l) => l.rel === 'reference');
    const refMeta = (k) => { const x = state.byKey.get(k) || {}; return [locOf(x), x.summary].filter(Boolean).join(' · '); };
    const props = Object.entries(n.props || {}).sort(([a], [b]) => a.localeCompare(b));

    const unassign = (nodeKey, domainKey) => async () => {
      try {
        await api('DELETE', `/api/assign?key=${q(nodeKey)}&domain=${q(domainKey)}`);
        toast('Domain removed');
        await reload();
      } catch (err) { toast(err.message, true); }
    };

    panel(
      h('div', { class: 'chips-row' },
        h('span', { class: 'kind-badge' }, swatch(n.node_type, 12, nodeColor(n)), typeInfo(n.node_type).label),
        communityOf(n.key) >= 0 ? h('span', { class: 'chip-static', style: 'padding-right:8px', text: communityLabel(communityOf(n.key)) }) : null,
        n.needs_summary ? h('span', { class: 'badge', text: 'needs summary' }) : null,
      ),
      h('h2', { text: n.title || n.key }),
      n.loc ? h('div', { class: 'loc', text: locOf(n) }) : h('div', { class: 'key', text: n.key }),
      n.uri ? sourceLine(n) : null,
      n.summary ? h('p', { class: 'summary', text: n.summary }) :
        h('p', { class: 'muted-note', text: isDomain ? 'No summary yet.' : 'No summary yet. Run “Summarize with AI” or write one.' }),
      h('div', { class: 'panel-actions' },
        h('button', { class: 'btn', type: 'button', text: 'Edit', onclick: () => openEditDialog(n) }),
        !isDomain ? h('button', { class: 'btn', type: 'button', text: 'Assign domain', onclick: () => openAssignDialog(n.key) }) : null,
        isDomain ? h('button', { class: 'btn ghost danger', type: 'button', text: 'Delete', onclick: () => confirmDelete(n, members.length) }) : null,
      ),
      (n.keyword || []).length ? h('h3', { text: 'Keywords' }) : null,
      (n.keyword || []).length ? h('div', { class: 'tags' }, n.keyword.map((k) => h('button', {
        class: 'tag', type: 'button', text: k, title: `Show only “${k}”`,
        onclick: () => { state.keyword = k; $('keyword-select').value = k; applyFilters(); },
      }))) : null,
      !isDomain ? h('h3', { text: 'Domain' }) : null,
      !isDomain ? (domainLinks.length
        ? h('div', { class: 'chips-row' }, domainLinks.map((l) => h('span', { class: 'chip-static' },
          swatch('domain', 10),
          h('button', { class: 'chip-link', type: 'button', text: titleOf(l.to), onclick: () => select(l.to, true) }),
          h('button', { type: 'button', title: 'Remove domain', 'aria-label': `Remove domain ${titleOf(l.to)}`, text: '✕', onclick: unassign(n.key, l.to) }))))
        : h('p', { class: 'footnote', text: n.node_type === 'doc_section' && (state.domainsOf.get(n.loc) || []).length
          ? `Inherits from its doc: ${(state.domainsOf.get(n.loc) || []).map(titleOf).join(', ')}` : 'No domain.' })) : null,
      parent ? h('h3', { text: 'Part of' }) : null,
      parent ? h('ul', { class: 'conn-list' }, nodeRow(parent.to, typeInfo((state.byKey.get(parent.to) || {}).node_type).label)) : null,
      children.length ? h('h3', { text: `Sections (${children.length})` }) : null,
      children.length ? h('ul', { class: 'conn-list' }, children.map((l) => {
        const c = state.byKey.get(l.from) || {};
        return nodeRow(l.from, [c.line_loc ? `line ${c.line_loc}` : '', c.needs_summary ? 'needs summary' : c.summary].filter(Boolean).join(' · '));
      })) : null,
      refsOut.length ? h('h3', { text: `References (${refsOut.length})` }) : null,
      refsOut.length ? h('ul', { class: 'conn-list' }, refsOut.map((l) => nodeRow(l.to, refMeta(l.to)))) : null,
      refsIn.length ? h('h3', { text: `Referenced by (${refsIn.length})` }) : null,
      refsIn.length ? h('ul', { class: 'conn-list' }, refsIn.map((l) => nodeRow(l.from, refMeta(l.from)))) : null,
      members.length ? h('h3', { text: `Docs and sections (${members.length})` }) : null,
      members.length ? h('ul', { class: 'conn-list' }, members.map((l) => nodeRow(l.from, locOf(state.byKey.get(l.from) || {}), unassign(l.from, n.key)))) : null,
      props.length ? h('h3', { text: 'Properties' }) : null,
      props.length ? h('table', { class: 'props' }, h('tbody', {}, props.map(([k, v]) => h('tr', {}, h('td', { text: k.replace(/_/g, ' ') }), h('td', { text: typeof v === 'object' ? JSON.stringify(v) : String(v) }))))) : null,
      data.text ? h('details', { class: 'text-block', open: !n.summary }, h('summary', { text: n.node_type === 'doc' ? 'Document text' : 'Section text' }), h('pre', { class: 'doc-text', text: data.text })) : null,
      h('p', { class: 'footnote', text: `key ${n.key} · by ${n.author || 'unknown'} · updated ${(n.updated || '').replace('T', ' ').slice(0, 16)}` }),
    );
  }

  function renderExplain() {
    const ex = state.explain;
    if (!ex) return;
    panel(
      h('h3', { text: 'Explain' }),
      h('h2', { text: ex.query }),
      h('div', { class: 'panel-actions' },
        h('button', { class: 'btn small', type: 'button', text: 'Copy as markdown', onclick: async () => {
          try { await navigator.clipboard.writeText(state.explainMarkdown); toast('Copied context'); }
          catch { toast('Clipboard not available', true); }
        } }),
        h('button', { class: 'btn ghost small', type: 'button', text: 'Clear', onclick: clearExplain }),
      ),
      ex.matches.length === 0 ? h('p', { class: 'footnote', text: 'No matching knowledge. Try other words.' }) : null,
      h('h3', { text: `${ex.matches.length} match${ex.matches.length === 1 ? '' : 'es'}` }),
      ...ex.matches.map((m) => h('div', {
        class: 'result', tabindex: 0,
        onclick: () => select(m.node.key, true),
        onkeydown: (ev) => { if (ev.key === 'Enter') select(m.node.key, true); },
      },
        h('div', { class: 'r-head' }, swatch(m.node.node_type, 12, nodeColor(m.node)), m.node.title || m.node.key),
        m.node.loc ? h('div', { class: 'loc', text: locOf(m.node) }) : null,
        m.node.summary ? h('div', { class: 'r-sum', text: m.node.summary }) :
          m.excerpt ? h('div', { class: 'r-sum' }, h('em', { text: 'No summary yet: ' }), m.excerpt.slice(0, 220)) : null,
        m.domains.length ? h('div', { class: 'r-rel', text: 'Domain: ' + m.domains.join(', ') }) : null,
      )),
    );
  }

  function renderPanel() {
    if (state.selected) renderDetail(state.selected);
    else if (state.explain) renderExplain();
    else renderOverview();
  }

  // ---------- selection ----------
  function select(key, center) {
    state.selected = key && state.byKey.has(key) ? key : null;
    const target = state.selected ? '#k=' + q(state.selected) : ' ';
    if (location.hash !== target.trim()) history.replaceState(null, '', target);
    applyHighlight();
    if (state.mode === 'table') renderTable();
    if (center && state.selected && cy && state.mode === 'graph') {
      const el = cy.getElementById(state.selected);
      if (el.nonempty() && !el.hasClass('hidden')) cy.animate({ center: { eles: el }, duration: 300 });
    }
    renderPanel();
  }

  function clearExplain() {
    state.explain = null;
    $('search').value = '';
    applyHighlight();
    renderPanel();
  }

  async function runExplain(query) {
    if (!query.trim()) { clearExplain(); return; }
    try {
      const data = await api('GET', `/api/explain?q=${q(query)}&limit=8&depth=1`);
      state.explain = data.explanation;
      state.explainMarkdown = data.markdown;
      state.selected = null;
      applyHighlight();
      renderPanel();
      if (cy && state.mode === 'graph' && state.explain.matches.length) {
        const keys = new Set(state.explain.matches.flatMap((m) => [m.node.key, ...m.related.map((r) => r.key)]));
        const eles = cy.nodes().filter((el) => keys.has(el.id()) && !el.hasClass('hidden'));
        if (eles.nonempty()) fitVisible(true, eles);
      }
    } catch (err) { toast(err.message, true); }
  }

  // Web sources: the page address (http/https only) and when it was fetched.
  function sourceLine(n) {
    const safe = /^https?:\/\//i.test(n.uri);
    const when = n.last_fetched ? new Date(n.last_fetched) : null;
    return h('div', { class: 'source' },
      safe ? h('a', { href: n.uri, target: '_blank', rel: 'noopener noreferrer', text: n.uri }) : h('span', { text: n.uri }),
      when && !isNaN(when) ? h('span', { class: 'footnote', text: `fetched ${when.toLocaleString()}` }) : null,
    );
  }

  // ---------- dialogs ----------
  const dialog = $('dialog');
  const field = (label, input, help) => h('label', { class: 'field' }, h('span', { text: label }), input, help ? h('small', { text: help }) : null);
  const datalist = (id, values) => h('datalist', { id }, [...new Set(values)].sort().map((v) => h('option', { value: v })));
  const showErr = (el, msg) => { el.textContent = msg; el.hidden = false; };
  const splitKeywords = (s) => s.split(',').map((k) => k.trim()).filter(Boolean);

  function formDialog(titleText, fields, submitText, onSubmit, focusEl) {
    const err = h('p', { class: 'form-error', hidden: true });
    const form = h('form', { method: 'dialog' },
      h('h2', { text: titleText }), err, ...fields,
      h('div', { class: 'dialog-actions' },
        h('button', { class: 'btn ghost', type: 'button', text: 'Cancel', onclick: () => dialog.close() }),
        h('button', { class: 'btn primary', type: 'submit', text: submitText }),
      ),
    );
    form.addEventListener('submit', async (ev) => {
      ev.preventDefault();
      try { await onSubmit(); dialog.close(); } catch (x) { showErr(err, x.message); }
    });
    dialog.replaceChildren(form);
    dialog.showModal();
    if (focusEl) focusEl.focus();
  }

  function openEditDialog(n) {
    const isDomain = n.node_type === 'domain';
    const title = h('input', { value: n.title || '', required: true, autocomplete: 'off' });
    const summary = h('textarea', { rows: 4 }); summary.value = n.summary || '';
    const keyword = h('input', { value: (n.keyword || []).join(', '), autocomplete: 'off', placeholder: 'comma separated' });
    formDialog(`Edit ${typeInfo(n.node_type).label}`, [
      isDomain ? field('Title', title) : h('p', { class: 'hint' }, h('strong', { text: n.title }), h('br'), h('span', { class: 'loc', text: locOf(n) })),
      field('Summary', summary, '1–3 sentences. Saving marks it as written by you, so the AI will not overwrite it.'),
      field('Keywords', keyword),
    ], 'Save', async () => {
      const body = { summary: summary.value.trim(), keyword: splitKeywords(keyword.value) };
      if (isDomain && title.value.trim() !== n.title) body.title = title.value.trim();
      await api('PUT', `/api/node?key=${q(n.key)}`, body);
      toast('Saved');
      await reload();
      select(n.key); // a domain keeps its key when retitled
    }, summary);
  }

  function openDomainDialog() {
    const title = h('input', { required: true, autocomplete: 'off', placeholder: 'e.g. Internet Marketing' });
    const summary = h('textarea', { rows: 3 });
    const keyword = h('input', { autocomplete: 'off', placeholder: 'comma separated' });
    formDialog('New domain', [field('Title', title), field('Summary', summary), field('Keywords', keyword)], 'Create', async () => {
      const res = await api('POST', '/api/domains', { title: title.value.trim(), summary: summary.value.trim(), keyword: splitKeywords(keyword.value) });
      toast(res.created ? 'Domain created' : 'Domain already existed; updated');
      await reload();
      select(res.node.key, true);
    }, title);
  }

  function openAssignDialog(key) {
    const domain = h('input', { required: true, list: 'domain-list', autocomplete: 'off', placeholder: 'existing or new domain' });
    formDialog(`Assign domain to “${titleOf(key)}”`, [
      field('Domain', domain, 'Pick an existing domain or type a new title to create it.'),
      datalist('domain-list', state.nodes.filter((n) => n.node_type === 'domain').map((n) => n.title)),
    ], 'Assign', async () => {
      const res = await api('POST', '/api/assign', { key, domain: domain.value.trim() });
      toast(res.domain_created ? `Created and assigned “${res.domain.title}”` : `Assigned “${res.domain.title}”`);
      await reload();
      select(key);
    }, domain);
  }

  function confirmDelete(n, memberCount) {
    formDialog(`Delete domain “${n.title}”?`, [
      h('p', { class: 'hint', text: `This removes the domain and its ${memberCount} domain_of edge${memberCount === 1 ? '' : 's'}. Docs and sections are kept.` }),
    ], 'Delete', async () => {
      await api('DELETE', `/api/node?key=${q(n.key)}`);
      toast('Domain deleted');
      state.selected = null;
      await reload();
    });
  }

  function openFetchDialog() {
    const url = h('input', { type: 'url', required: true, autocomplete: 'off', placeholder: 'https://…' });
    formDialog('Fetch web page', [
      field('URL', url, 'The page’s main content is saved as markdown in docs/external_sources/web and synced. Fetching a saved URL again refreshes it.'),
    ], 'Fetch', async () => {
      const u = url.value.trim();
      queueMicrotask(() => runFetch(u)); // after the dialog closes, so the busy overlay is visible
    }, url);
  }

  async function runFetch(url) {
    await busy('Fetching page…', async () => {
      try {
        const r = await api('POST', '/api/fetch', { url });
        const s = r.saved;
        const pending = state.ai && r.sync.needs_summary ? ' · run “Summarize with AI” for summaries' : '';
        toast(`${s.created ? 'Saved' : s.changed ? 'Updated' : 'Unchanged'}: ${s.loc}${pending}`);
        await reload();
        select(s.loc, true);
      } catch (err) { toast(err.message, true); }
    });
  }

  // ---------- sync & AI ----------
  async function runSync() {
    await busy('Syncing ./docs…', async () => {
      try {
        const r = await api('POST', '/api/sync');
        toast(`Docs +${r.docs_added.length} ~${r.docs_updated.length} −${r.docs_removed.length} · sections +${r.sections_added} ~${r.sections_updated} −${r.sections_removed}`);
        await reload();
      } catch (err) { toast(err.message, true); }
    });
  }

  async function runAI() {
    await busy('Summarizing with Claude… this can take a minute', async () => {
      try {
        const r = await api('POST', '/api/ai');
        const failed = Object.keys(r.ai.failed || {}).length;
        const created = (r.ai.domains_created || []).length;
        toast(r.ai.docs.length === 0 ? 'Nothing to summarize'
          : `Summarized ${r.ai.nodes_summarized} node(s)${created ? ` · ${created} new domain(s)` : ''}${failed ? ` · ${failed} failed` : ''}`, failed > 0);
        await reload();
      } catch (err) { toast(err.message, true); }
    });
  }

  // ---------- data ----------
  async function reload() {
    const data = await api('GET', '/api/graph');
    state.nodes = data.nodes || [];
    state.links = data.links || [];
    state.byKey = new Map(state.nodes.map((n) => [n.key, n]));
    state.communities = new Map(Object.entries(data.communities || {}));
    state.hiddenCommunities.clear(); // community numbers can change after a sync
    state.degree = new Map();
    state.domainsOf = new Map();
    for (const l of state.links) {
      state.degree.set(l.from, (state.degree.get(l.from) || 0) + 1);
      state.degree.set(l.to, (state.degree.get(l.to) || 0) + 1);
      if (l.rel === 'domain_of') state.domainsOf.set(l.from, [...(state.domainsOf.get(l.from) || []), l.to]);
    }
    if (state.selected && !state.byKey.has(state.selected)) state.selected = null;

    const count = (t) => state.nodes.filter((n) => n.node_type === t).length;
    const pending = state.nodes.filter((n) => n.needs_summary).length;
    $('counts').textContent = `${count('domain')} domains · ${count('doc')} docs · ${count('doc_section')} sections` + (pending ? ` · ${pending} need summary` : '');
    $('empty').hidden = state.nodes.length > 0;
    $('filters').hidden = state.nodes.length === 0;

    renderFilters();
    renderGraph();
    renderPanel();
  }

  function applyTheme(theme) {
    if (theme) document.documentElement.setAttribute('data-theme', theme);
    else document.documentElement.removeAttribute('data-theme');
    if (cy) {
      cy.style(graphStyle());
      cy.nodes().forEach((el) => { const n = state.byKey.get(el.id()); if (n) el.data('color', nodeColor(n)); });
    }
    renderFilters();
    renderPanel();
    if (state.mode === 'table') renderTable();
  }

  // ---------- wire up ----------
  $('search-form').addEventListener('submit', (ev) => { ev.preventDefault(); runExplain($('search').value); });
  $('search').addEventListener('search', () => { if (!$('search').value) clearExplain(); });
  $('btn-domain').addEventListener('click', openDomainDialog);
  $('btn-sync').addEventListener('click', runSync);
  $('btn-fetch').addEventListener('click', openFetchDialog);
  $('btn-ai').addEventListener('click', runAI);
  document.querySelector('[data-action="sync"]').addEventListener('click', runSync);
  $('btn-fit').addEventListener('click', () => fitVisible(true));
  $('btn-layout').addEventListener('click', () => { if (cy) { runFullLayout(); fitVisible(false); } });
  $('color-community').addEventListener('click', () => setColorBy('community'));
  $('color-type').addEventListener('click', () => setColorBy('type'));
  $('btn-clear').addEventListener('click', clearExplain);
  $('mode-graph').addEventListener('click', () => setMode('graph'));
  $('mode-table').addEventListener('click', () => setMode('table'));
  $('keyword-select').addEventListener('change', (ev) => { state.keyword = ev.target.value; applyFilters(); });
  $('domain-select').addEventListener('change', (ev) => { state.domain = ev.target.value; applyFilters(); });
  document.querySelectorAll('.data-table th').forEach((th) => th.addEventListener('click', () => {
    const col = th.dataset.sort;
    state.sort = { col, dir: state.sort.col === col ? -state.sort.dir : 1 };
    renderTable();
  }));
  $('btn-theme').addEventListener('click', () => {
    const attr = document.documentElement.getAttribute('data-theme');
    const dark = attr === 'dark' || (!attr && matchMedia('(prefers-color-scheme: dark)').matches);
    const next = dark ? 'light' : 'dark';
    try { localStorage.setItem('knowledge-graph-theme', next); } catch { /* ignore */ }
    applyTheme(next);
  });
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (!document.documentElement.getAttribute('data-theme')) applyTheme(null);
  });
  document.addEventListener('keydown', (ev) => {
    const tag = document.activeElement.tagName;
    if (ev.key === '/' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
      ev.preventDefault(); $('search').focus();
    } else if (ev.key === 'Escape' && !dialog.open) {
      if (state.selected) select(null); else if (state.explain) clearExplain();
    }
  });
  window.addEventListener('hashchange', () => {
    const m = location.hash.match(/^#k=(.+)$/);
    select(m ? decodeURIComponent(m[1]) : null, true);
  });

  (async () => {
    const params = new URLSearchParams(location.search);
    let theme = params.get('theme');
    if (!theme) { try { theme = localStorage.getItem('knowledge-graph-theme'); } catch { /* ignore */ } }
    if (theme === 'light' || theme === 'dark') document.documentElement.setAttribute('data-theme', theme);
    let color = params.get('color');
    if (!color) { try { color = localStorage.getItem('knowledge-graph-color'); } catch { /* ignore */ } }
    if (color === 'community' || color === 'type') state.colorBy = color;
    try {
      const schema = await api('GET', '/api/schema');
      state.ai = !!schema.ai;
      $('btn-ai').hidden = !state.ai;
      await reload();
      const m = location.hash.match(/^#k=(.+)$/);
      if (params.get('q')) { $('search').value = params.get('q'); await runExplain(params.get('q')); }
      else if (m) select(decodeURIComponent(m[1]), true);
    } catch (err) {
      $('counts').textContent = 'Could not load';
      toast(err.message, true);
    }
  })();
})();
