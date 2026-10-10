// Trade Knowledge: the graph of docs/knowledge.md in the browser.
// All content goes in through textContent (h()), never innerHTML.
(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);

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

  // Colour and shape per node type.
  const STYLE = {
    strategy_approach: { color: '--s7', shape: 'round-rectangle', size: 46 },
    approach: { color: '--s1', shape: 'ellipse', size: 40 },
    analytical_result: { color: '--s3', shape: 'round-rectangle', size: 30 },
    position: { color: '--s2', shape: 'ellipse', size: 48 },
    data_snapshot: { color: '--s4', shape: 'rectangle', size: 28 },
    open_position: { color: '--other', shape: 'round-rectangle', size: 42 },
    close_positions: { color: '--other', shape: 'round-rectangle', size: 42 },
    close_summary: { color: '--s5', shape: 'round-rectangle', size: 32 },
    profit_summary: { color: '--s6', shape: 'round-rectangle', size: 42 },
    loss_summary: { color: '--s8', shape: 'round-rectangle', size: 42 },
    portfolio: { color: '--s7', shape: 'round-rectangle', size: 46 },
    pair_hub: { color: '--s7', shape: 'diamond', size: 40 },
    pair: { color: '--s1', shape: 'hexagon', size: 46 },
    configuration: { color: '--other', shape: 'round-rectangle', size: 46 },
    pair_that_traded: { color: '--s1', shape: 'diamond', size: 40 },
    open_position_limit: { color: '--s4', shape: 'round-rectangle', size: 40 },
    leverage: { color: '--s4', shape: 'triangle', size: 40 },
    risk_summary: { color: '--s8', shape: 'octagon', size: 46 },
  };
  const NAMES = {
    strategy_approach: 'Strategy Approach', approach: 'Approach', analytical_result: 'Analytical Result',
    position: 'Position', data_snapshot: 'Data Snapshot', open_position: 'Open Position',
    close_positions: 'Close Positions', close_summary: 'Close Summary', profit_summary: 'Profit Summary',
    loss_summary: 'Loss Summary', portfolio: 'Portfolio', pair_hub: 'Pair', pair: 'Trading Pair',
    configuration: 'Configuration', pair_that_traded: 'Pair That Traded', open_position_limit: 'Open Position Limit', leverage: 'Leverage', risk_summary: 'Risk Summary',
  };

  const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const colorOf = (type) => css((STYLE[type] || { color: '--other' }).color) || '#888';

  let cy = null;
  let data = { nodes: [], links: [], types: [] };
  const hidden = new Set();
  let selected = null;

  // ---- collapse and expand ----------------------------------------------------------
  // Neo4j style: any node with links can be expanded, which shows its hidden
  // neighbours one hop out, or collapsed, which hides what hangs on it. The
  // hierarchy of docs/knowledge.md says what a node holds and its way back:
  // the node types a type is held by.
  const PARENT_TYPES = {
    approach: ['strategy_approach'],
    pair_hub: ['portfolio'], risk_summary: ['portfolio'],
    pair: ['pair_hub'],
    open_position: ['pair'], close_positions: ['pair'], profit_summary: ['pair'], loss_summary: ['pair'],
    position: ['open_position', 'close_positions'],
    analytical_result: ['position'], data_snapshot: ['position'], close_summary: ['position'],
    pair_that_traded: ['configuration'], open_position_limit: ['configuration'], leverage: ['configuration'],
  };
  const HIDDEN_KEY = 'autotrade-hidden';
  let gone = new Set(); // keys hidden by collapsing
  try { gone = new Set(JSON.parse(localStorage.getItem(HIDDEN_KEY) || '[]')); } catch (e) { /* storage blocked */ }
  let children = new Map(); // key -> keys it holds
  let parentOf = new Map(); // key -> the key holding it
  let neighbours = new Map(); // key -> keys linked to it, either way
  const placed = new Set(); // nodes with a position of their own

  function saveGone() {
    try { localStorage.setItem(HIDDEN_KEY, JSON.stringify([...gone])); } catch (e) { /* storage blocked */ }
  }

  // buildTree finds the neighbours, and who holds whom from the links,
  // whichever way they point.
  function buildTree() {
    children = new Map();
    parentOf = new Map();
    neighbours = new Map(data.nodes.map((n) => [n.key, new Set()]));
    const typeOf = new Map(data.nodes.map((n) => [n.key, n.node_type]));
    const adopt = (p, c) => {
      if (parentOf.has(c) || !(PARENT_TYPES[typeOf.get(c)] || []).includes(typeOf.get(p))) return;
      parentOf.set(c, p);
      if (!children.has(p)) children.set(p, []);
      children.get(p).push(c);
    };
    for (const l of data.links) {
      if (!neighbours.has(l.from) || !neighbours.has(l.to)) continue;
      neighbours.get(l.from).add(l.to);
      neighbours.get(l.to).add(l.from);
      adopt(l.from, l.to);
      adopt(l.to, l.from);
    }
    gone = new Set([...gone].filter((k) => neighbours.has(k)));
  }

  function descendants(key, out = new Set()) {
    for (const c of children.get(key) || []) {
      if (!out.has(c)) { out.add(c); descendants(c, out); }
    }
    return out;
  }

  // present: in the graph (its type not hidden by the legend); shown: and
  // not collapsed away.
  const present = (k) => !!cy && cy.getElementById(k).length > 0;
  const shown = (k) => present(k) && !gone.has(k);

  // hiddenNeighbours are what expanding key shows.
  function hiddenNeighbours(key) {
    return [...(neighbours.get(key) || [])].filter((k) => present(k) && gone.has(k));
  }

  // collapseSet is what collapsing key hides: everything it holds, its other
  // links except the way back to its holder, and then whatever is left
  // hanging only on those. The top nodes (Strategy Approach, Portfolio,
  // Configuration) are never left out that way.
  function collapseSet(key) {
    const back = parentOf.get(key);
    const out = new Set();
    for (const k of descendants(key)) if (shown(k)) out.add(k);
    for (const k of neighbours.get(key) || []) if (shown(k) && k !== back) out.add(k);
    for (let grew = true; grew;) {
      grew = false;
      for (const [k, ns] of neighbours) {
        if (out.has(k) || k === key || k === back || !parentOf.has(k) || !shown(k)) continue;
        const linked = [...ns].filter(shown);
        if (linked.length && linked.every((m) => out.has(m) || m === key)) { out.add(k); grew = true; }
      }
    }
    return out;
  }

  // fold hides the collapsed nodes and badges each node that has hidden
  // neighbours.
  function fold() {
    cy.batch(() => {
      cy.nodes().forEach((n) => {
        const k = n.id(), off = gone.has(k) ? 0 : hiddenNeighbours(k).length;
        n.toggleClass('gone', gone.has(k));
        n.toggleClass('collapsed', off > 0);
        n.data('label', n.data('base') + (off ? '\n+' + off + ' hidden' : ''));
      });
      cy.edges().forEach((e) => e.toggleClass('gone', gone.has(e.source().id()) || gone.has(e.target().id())));
    });
    renderCounts();
  }

  // placeNew puts nodes shown for the first time around a placed neighbour
  // (their holder first, else the node that was expanded), fanned out away
  // from where that neighbour hangs, so the rest stays put.
  function placeNew(anchor) {
    const around = (k) => {
      const p = parentOf.get(k);
      if (p && placed.has(p) && shown(p)) return p;
      if (anchor && placed.has(anchor) && neighbours.get(k).has(anchor)) return anchor;
      return [...neighbours.get(k)].find((m) => placed.has(m) && shown(m));
    };
    let fresh = cy.nodes().not('.gone').map((n) => n.id()).filter((k) => !placed.has(k));
    while (fresh.length) {
      const groups = new Map();
      for (const k of fresh) {
        const a = around(k);
        if (!a) continue;
        if (!groups.has(a)) groups.set(a, []);
        groups.get(a).push(k);
      }
      if (!groups.size) break;
      for (const [a, kids] of groups) fan(a, kids);
      fresh = fresh.filter((k) => !placed.has(k));
    }
    fresh.forEach((k) => placed.add(k)); // nothing to place them by: they stay where they are
  }

  function fan(a, kids) {
    const c = cy.getElementById(a).position();
    const from = [parentOf.get(a), ...neighbours.get(a)].find((m) => m && placed.has(m) && shown(m) && !kids.includes(m));
    const f = from && cy.getElementById(from).position();
    const base = f ? Math.atan2(c.y - f.y, c.x - f.x) : -Math.PI / 2;
    const spread = f ? Math.min(Math.PI * 1.2, 0.7 * kids.length) : 2 * Math.PI * (1 - 1 / kids.length);
    const step = kids.length > 1 ? spread / (kids.length - 1) : 1;
    const r = Math.max(110, 85 / step); // neighbours at least ~85px apart
    // A spot already taken (by a node fanned out from somewhere else) moves
    // the new node a little sideways, then further out.
    const taken = cy.nodes().not('.gone').filter((n) => placed.has(n.id())).map((n) => ({ ...n.position() }));
    const clear = (p) => taken.every((q) => Math.hypot(p.x - q.x, p.y - q.y) >= 80);
    kids.forEach((k, i) => {
      const t = kids.length === 1 ? base : base - spread / 2 + step * i;
      const at = (ring, dt) => ({ x: c.x + (r + 60 * ring) * Math.cos(t + dt), y: c.y + (r + 60 * ring) * Math.sin(t + dt) });
      let p = null;
      for (let ring = 0; ring < 6 && !p; ring++) {
        p = [0, 0.3, -0.3, 0.6, -0.6].map((dt) => at(ring, dt)).find(clear) || null;
      }
      p = p || at(0, 0);
      cy.getElementById(k).position(p);
      taken.push(p);
      placed.add(k);
    });
  }

  function fitShown() {
    const all = cy.elements().not('.gone');
    cy.fit(all, 40);
    if (cy.zoom() > 1.2) { cy.zoom(1.2); cy.center(all); }
  }

  // setGone shows and hides nodes. A selected node that gets hidden is let
  // go; otherwise its focus and panel buttons follow.
  function setGone(next, anchor) {
    gone = next;
    saveGone();
    if (!cy) return;
    fold();
    const before = new Set(placed);
    placeNew(anchor);
    // Keep what was just shown in view.
    const e = cy.extent();
    const off = cy.nodes().not('.gone').some((n) => {
      const p = n.position();
      return !before.has(n.id()) && (p.x < e.x1 || p.x > e.x2 || p.y < e.y1 || p.y > e.y2);
    });
    if (off) fitShown();
    if (!selected) return;
    if (gone.has(selected)) {
      clearFocus();
      return;
    }
    focus(selected);
    const f = $('fold');
    if (f) f.replaceWith(foldButtons(selected));
  }

  function expand(key) {
    const next = new Set(gone);
    for (const k of hiddenNeighbours(key)) next.delete(k);
    setGone(next, key);
  }

  function collapse(key) {
    setGone(new Set([...gone, ...collapseSet(key)]), key);
  }

  // toggle is the double-click: expand what is hidden, else collapse.
  function toggle(key) {
    if (gone.has(key)) return;
    if (hiddenNeighbours(key).length) expand(key);
    else if (collapseSet(key).size) collapse(key);
  }

  // reveal shows a hidden key with the holders on its way back.
  function reveal(key) {
    if (!gone.has(key)) return;
    const next = new Set(gone);
    for (let k = key; k; k = parentOf.get(k)) next.delete(k);
    setGone(next, null);
  }

  // foldButtons expand and collapse key from the panel.
  function foldButtons(key) {
    const off = cy ? hiddenNeighbours(key).length : 0, hide = cy ? collapseSet(key).size : 0;
    return h('div', { class: 'fold', id: 'fold' },
      off ? h('button', { class: 'btn ghost small', type: 'button', onclick: () => expand(key),
        title: 'Show its ' + off + ' hidden neighbours (or double-click the node)' }, 'Expand (' + off + ')') : null,
      hide ? h('button', { class: 'btn ghost small', type: 'button', onclick: () => collapse(key),
        title: 'Hide the ' + hide + ' nodes that hang on it' + (off ? '' : ' (or double-click the node)') }, 'Collapse (' + hide + ')') : null);
  }

  // ---- theme ------------------------------------------------------------------------
  const params = new URLSearchParams(location.search);
  function setTheme(t) {
    if (t) document.documentElement.setAttribute('data-theme', t);
    else document.documentElement.removeAttribute('data-theme');
    try { if (t) localStorage.setItem('autotrade-theme', t); } catch (e) { /* storage blocked */ }
    if (cy) cy.style(graphStyle());
    renderLegend();
  }
  let theme = params.get('theme');
  if (!theme) { try { theme = localStorage.getItem('autotrade-theme'); } catch (e) { theme = null; } }
  if (theme === 'light' || theme === 'dark') document.documentElement.setAttribute('data-theme', theme);
  $('btn-theme').addEventListener('click', () => {
    const dark = document.documentElement.getAttribute('data-theme') === 'dark' ||
      (!document.documentElement.getAttribute('data-theme') && matchMedia('(prefers-color-scheme: dark)').matches);
    setTheme(dark ? 'light' : 'dark');
  });

  // ---- data ---------------------------------------------------------------------------
  async function api(path, opts) {
    const r = await fetch(path, opts);
    const body = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(body.error || r.statusText);
    return body;
  }

  function toast(text, isError) {
    const t = $('toast');
    t.textContent = text;
    t.className = 'toast' + (isError ? ' error' : '');
    t.hidden = false;
    clearTimeout(toast.timer);
    toast.timer = setTimeout(() => { t.hidden = true; }, isError ? 8000 : 3500);
  }

  function money(v) { return (v >= 0 ? '+' : '') + Number(v || 0).toFixed(2); }

  function label(n) {
    const p = n.props || {};
    switch (n.node_type) {
      case 'position': {
        const head = n.title + ' ' + (p.side || '');
        if (p.status === 'closed') return head + '\n' + money(p.net) + ' USDT · ' + Number(p.r || 0).toFixed(2) + 'R';
        return head + '\nopen · ' + money(p.unrealized);
      }
      case 'approach':
        return n.title.replace(/^Approach /, '') + '\n' + (p.trades || 0) + ' closed · ' + money(p.net);
      case 'close_summary':
        return n.title + '\n' + (p.outcome || '');
      case 'profit_summary': case 'loss_summary': case 'portfolio': case 'pair':
        return n.title + '\n' + (p.trades || 0) + ' trades · ' + money(p.net);
      case 'configuration':
        return n.title + '\n' + (p.interval || '') + ' · ' + (p.pairs || []).length + ' pairs';
      case 'pair_that_traded':
        return n.title + '\n' + (p.pairs || []).join(', ');
      case 'open_position_limit':
        return n.title + '\nmax ' + (p.max_position_usdt || 0) + ' · loss ' + (p.max_risk_usdt || 0);
      case 'leverage':
        return n.title + '\n' + (p.leverage || 0) + 'x';
      case 'risk_summary':
        return n.title + '\nat stake ' + Number(p.open_risk || 0).toFixed(2) + ' · drawdown ' + Number(p.max_drawdown || 0).toFixed(2);
      default:
        return n.title;
    }
  }

  function graphStyle() {
    const ink = css('--ink'), ink2 = css('--ink-2'), page = css('--page'), edge = css('--axis') || css('--muted');
    const s = [
      { selector: 'node', style: {
        'label': 'data(label)', 'font-size': 10, 'color': ink, 'text-wrap': 'wrap', 'text-max-width': 120,
        'text-valign': 'bottom', 'text-margin-y': 4, 'text-outline-color': page, 'text-outline-width': 2,
        'background-color': 'data(color)', 'shape': 'data(shape)', 'width': 'data(size)', 'height': 'data(size)',
        'border-width': 0,
      } },
      { selector: 'node[open = 1]', style: { 'border-width': 3, 'border-style': 'dashed', 'border-color': ink2 } },
      { selector: 'edge', style: {
        'width': 1.4, 'line-color': edge, 'target-arrow-color': edge, 'target-arrow-shape': 'triangle',
        'arrow-scale': 0.8, 'curve-style': 'bezier', 'label': 'data(rel)', 'font-size': 8, 'color': css('--muted'),
        'text-rotation': 'autorotate', 'text-outline-color': page, 'text-outline-width': 2,
      } },
      { selector: 'node.collapsed', style: { 'border-width': 3, 'border-style': 'double', 'border-color': ink2, 'font-weight': 'bold' } },
      { selector: '.gone', style: { 'display': 'none' } },
      { selector: '.faded', style: { 'opacity': 0.15 } },
      { selector: 'node:selected', style: { 'border-width': 3, 'border-color': css('--accent'), 'border-style': 'solid' } },
    ];
    return s;
  }

  function elements() {
    const els = [];
    const shown = new Set();
    for (const n of data.nodes) {
      if (hidden.has(n.node_type)) continue;
      shown.add(n.key);
      const st = STYLE[n.node_type] || { shape: 'ellipse', size: 30 };
      els.push({ group: 'nodes', data: {
        id: n.key, label: label(n), base: label(n), type: n.node_type, color: colorOf(n.node_type), shape: st.shape, size: st.size,
        open: n.node_type === 'position' && n.props && n.props.status === 'open' ? 1 : 0,
      } });
    }
    data.links.forEach((l, i) => {
      if (shown.has(l.from) && shown.has(l.to)) {
        els.push({ group: 'edges', data: { id: 'e' + i, source: l.from, target: l.to, rel: l.rel.replace(/_/g, ' ') } });
      }
    });
    return els;
  }

  // layout arranges the shown nodes; folded ones are placed when they unfold.
  function layout() {
    if (!cy) return;
    const shown = cy.elements().not('.gone');
    shown.layout({ name: 'cose', animate: false, nodeRepulsion: () => 9000, idealEdgeLength: () => 90, padding: 30, randomize: true }).run();
    placed.clear();
    shown.nodes().forEach((n) => placed.add(n.id()));
    fitShown();
  }

  function draw() {
    const els = elements();
    $('empty').hidden = data.nodes.length > 0;
    if (!cy) {
      cy = cytoscape({ container: $('graph'), elements: els, style: graphStyle(), minZoom: 0.15, maxZoom: 3, boxSelectionEnabled: false });
      cy.on('tap', 'node', (e) => select(e.target.id()));
      cy.on('dbltap', 'node', (e) => toggle(e.target.id()));
      cy.on('tap', (e) => { if (e.target === cy) clearFocus(); });
    } else {
      cy.elements().remove();
      cy.add(els);
      cy.style(graphStyle());
    }
    fold();
    layout();
  }

  function renderLegend() {
    const counts = {};
    for (const n of data.nodes) counts[n.node_type] = (counts[n.node_type] || 0) + 1;
    const legend = $('legend');
    legend.replaceChildren(...(data.types || []).filter((t) => counts[t]).map((t) => h('button', {
      class: 'chip' + (hidden.has(t) ? ' off' : ''), type: 'button', title: 'Show or hide ' + NAMES[t],
      onclick: () => { hidden.has(t) ? hidden.delete(t) : hidden.add(t); renderLegend(); draw(); },
    }, h('span', { class: 'dot', style: 'background:' + colorOf(t) }), NAMES[t] || t, h('span', { class: 'n', text: String(counts[t]) }))));
  }

  function renderCounts() {
    const by = (t) => data.nodes.filter((n) => n.node_type === t);
    const pos = by('position');
    const open = pos.filter((n) => n.props.status === 'open').length;
    const pf = by('portfolio')[0];
    let text = by('pair').length + ' pairs · ' + pos.length + ' positions (' + open + ' open) · ' + by('approach').length + ' approaches';
    if (pf) text += ' · net ' + money(pf.props.net) + ' USDT';
    if (gone.size) text += ' · ' + gone.size + ' hidden';
    $('counts').textContent = text;
    $('btn-expand').hidden = !gone.size;
  }

  async function load() {
    try {
      data = await api('/api/graph');
    } catch (e) {
      $('counts').textContent = 'Could not load the graph';
      toast(e.message, true);
      return;
    }
    $('btn-sync').hidden = !data.sync;
    buildTree();
    renderCounts();
    renderLegend();
    draw();
    const m = location.hash.match(/^#k=(.+)$/);
    if (m) select(decodeURIComponent(m[1]));
  }

  // ---- detail panel ----------------------------------------------------------------------

  // inline renders `code` and **bold**.
  function inline(text) {
    const out = [];
    const re = /(`[^`]+`|\*\*[^*]+\*\*)/g;
    let last = 0, m;
    while ((m = re.exec(text))) {
      if (m.index > last) out.push(text.slice(last, m.index));
      const t = m[0];
      out.push(t[0] === '`' ? h('code', { text: t.slice(1, -1) }) : h('strong', { text: t.slice(2, -2) }));
      last = m.index + t.length;
    }
    if (last < text.length) out.push(text.slice(last));
    return out;
  }

  // markdown turns the server's markdown into elements: headings, lists,
  // tables and paragraphs.
  function markdown(md) {
    const root = h('div', { class: 'md' });
    const lines = (md || '').split('\n');
    let list = null, table = null;
    const flush = () => { list = null; table = null; };
    for (const raw of lines) {
      const line = raw.replace(/\s+$/, '');
      if (!line.trim()) { flush(); continue; }
      if (line.startsWith('|')) {
        const cells = line.split('|').slice(1, -1).map((c) => c.trim());
        if (cells.every((c) => /^-+$/.test(c))) continue;
        if (!table) {
          table = h('table');
          root.append(h('div', { class: 'tbl' }, table));
          table.append(h('tr', {}, cells.map((c) => h('th', {}, inline(c)))));
        } else {
          table.append(h('tr', {}, cells.map((c) => h('td', {}, inline(c)))));
        }
        continue;
      }
      table = null;
      let m;
      if ((m = line.match(/^(#{1,3}) (.*)$/))) {
        flush();
        root.append(h(m[1].length === 1 ? 'h3' : 'h4', {}, inline(m[2])));
      } else if ((m = line.match(/^(\s*)- (.*)$/))) {
        if (!list) { list = h('ul'); root.append(list); }
        list.append(h('li', { class: m[1].length ? 'sub' : null }, inline(m[2])));
      } else {
        list = null;
        root.append(h('p', {}, inline(line)));
      }
    }
    return root;
  }

  function propValue(v) {
    if (Array.isArray(v)) return v.join(', ');
    if (v && typeof v === 'object') return JSON.stringify(v);
    return String(v);
  }

  function clearFocus() {
    selected = null;
    if (cy) cy.elements().removeClass('faded');
    $('detail').hidden = true;
    $('panel-empty').hidden = false;
    history.replaceState(null, '', location.pathname + location.search);
  }

  // focus fades everything but key and its neighbours.
  function focus(key) {
    if (!cy) return;
    const n = cy.getElementById(key);
    cy.elements().removeClass('faded');
    if (n && n.length) {
      cy.elements().not(n.closedNeighborhood()).addClass('faded');
      cy.$(':selected').unselect();
      n.select();
    }
  }

  async function select(key) {
    selected = key;
    history.replaceState(null, '', '#k=' + encodeURIComponent(key));
    reveal(key);
    focus(key);
    let d;
    try {
      d = await api('/api/node?key=' + encodeURIComponent(key));
    } catch (e) {
      toast(e.message, true);
      return;
    }
    if (selected !== key) return; // a newer click won
    const n = d.node;
    const link = (k) => h('button', { class: 'linkbtn', type: 'button', onclick: () => select(k) }, k);
    const props = Object.entries(n.props || {}).filter(([k, v]) => v !== '' && v != null).sort(([a], [b]) => a.localeCompare(b));
    const detail = $('detail');
    // Sections are optional (null) and some are arrays: flatten them first,
    // or replaceChildren would print them as text.
    const parts = (...xs) => xs.flat().filter((x) => x != null && x !== false);
    detail.replaceChildren(...parts(
      h('div', { class: 'detail-head' },
        h('h2', { text: n.title }),
        h('span', { class: 'type-chip', style: 'background:' + colorOf(n.node_type), text: NAMES[n.node_type] || n.node_type })),
      foldButtons(n.key),
      h('p', { class: 'key', text: n.key }),
      d.markdown ? [h('div', { class: 'section-title', text: 'View' }), markdown(d.markdown)] : null,
      props.length ? [h('div', { class: 'section-title', text: 'Properties' }),
        h('table', { class: 'props' }, props.map(([k, v]) => h('tr', {}, h('td', { text: k }), h('td', { text: propValue(v) }))))] : null,
      (d.in || []).length ? [h('div', { class: 'section-title', text: 'Incoming' }),
        h('ul', { class: 'links' }, d.in.map((l) => h('li', {}, link(l.from), ' ', h('span', { class: 'rel', text: l.rel }))))] : null,
      (d.out || []).length ? [h('div', { class: 'section-title', text: 'Outgoing' }),
        h('ul', { class: 'links' }, d.out.map((l) => h('li', {}, h('span', { class: 'rel', text: l.rel }), ' ', link(l.to))))] : null,
    ));
    detail.hidden = false;
    $('panel-empty').hidden = true;
  }

  // ---- actions ----------------------------------------------------------------------------
  $('btn-fit').addEventListener('click', () => cy && cy.fit(undefined, 30));
  $('btn-layout').addEventListener('click', layout);
  // Folding or unfolding everything arranges the graph again; single nodes don't.
  $('btn-collapse').addEventListener('click', () => { setGone(new Set(data.nodes.map((n) => n.key).filter((k) => parentOf.has(k)))); layout(); });
  $('btn-expand').addEventListener('click', () => { setGone(new Set()); layout(); });
  $('btn-sync').addEventListener('click', async () => {
    const b = $('btn-sync');
    b.disabled = true;
    b.textContent = 'Syncing…';
    try {
      const r = await api('/api/sync', { method: 'POST' });
      toast('Synced: ' + r.positions + ' positions (' + r.open + ' open), ' + r.approaches + ' approaches');
      await load();
    } catch (e) {
      toast(e.message, true);
    } finally {
      b.disabled = false;
      b.textContent = 'Sync';
    }
  });

  load();
})();
