import { Box, Button, Code, Grid, HStack, Spinner, Tabs, Text } from '@chakra-ui/react';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, errorText, explain as fetchExplain, q, type AIReport, type Explanation, type GraphResponse, type KNode, type SavedWeb, type SyncReport } from './api';
import { Dialogs } from './dialogs';
import { GraphView, type ViewRequest } from './GraphView';
import { buildGraph, collapsersOf, collapsible, communityColor, communityOf, EMPTY_GRAPH, hiddenByCollapse, isVisible, titleOf, toggled, typeColor, type ColorBy, type Filters, type Graph } from './model';
import { ExplainPanel, NodeDetail, Overview } from './Panel';
import { TableView, type Sort } from './TableView';
import { palette, useColorMode } from './theme';
import { Tools } from './Tools';
import { TopBar, type ViewMode } from './TopBar';
import { Ctx, toast, Toaster, type AppContext, type DialogRequest } from './ui';

type PanelTab = 'detail' | 'tools';

const COLOR_KEY = 'knowledge-graph-color';

function initialColorBy(): ColorBy {
  let c = new URLSearchParams(location.search).get('color');
  if (!c) { try { c = localStorage.getItem(COLOR_KEY); } catch { /* storage unavailable */ } }
  return c === 'type' ? 'type' : 'community';
}

const hashKey = () => {
  const m = location.hash.match(/^#k=(.+)$/);
  return m ? decodeURIComponent(m[1]) : null;
};

export function App() {
  const [colorMode, toggleColorMode] = useColorMode();
  const pal = useMemo(() => palette(colorMode), [colorMode]);

  const [graph, setGraph] = useState<Graph>(EMPTY_GRAPH);
  const graphNow = useRef(graph); // for select(), which must see a reload at once
  const [status, setStatus] = useState<'loading' | 'ready' | 'failed'>('loading');
  const [version, setVersion] = useState(0);
  const [ai, setAI] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [explained, setExplained] = useState<{ explanation: Explanation; markdown: string } | null>(null);
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<ViewMode>('graph');
  const [colorBy, setColorByState] = useState<ColorBy>(initialColorBy);
  const [filters, setFilters] = useState<Filters>({ hiddenTypes: new Set(), hiddenCommunities: new Set(), keyword: '', domain: '' });
  const [sort, setSort] = useState<Sort>({ col: 'title', dir: 1 });
  const [busy, setBusy] = useState<string | null>(null);
  const [dialog, setDialog] = useState<DialogRequest | null>(null);
  const [request, setRequest] = useState<ViewRequest | null>(null);
  const [tab, setTab] = useState<PanelTab>('detail');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const searchRef = useRef<HTMLInputElement>(null);

  const reload = useCallback(async () => {
    const g = buildGraph(await api<GraphResponse>('GET', '/api/graph'));
    graphNow.current = g;
    setGraph(g);
    setFilters((f) => ({
      ...f,
      hiddenCommunities: new Set(), // community numbers can change after a sync
      domain: g.byKey.has(f.domain) ? f.domain : '',
      keyword: g.keywords.includes(f.keyword) ? f.keyword : '',
    }));
    setSelected((s) => (s && g.byKey.has(s) ? s : null));
    setCollapsed((c) => new Set([...c].filter((k) => collapsible(g, k))));
    setVersion((v) => v + 1);
    setStatus('ready');
  }, []);

  // Selecting shows the Node Detail tab, and expands collapsed parents so the node is in view.
  const select = useCallback((key: string | null, center = false) => {
    const k = key && graphNow.current.byKey.has(key) ? key : null;
    setSelected(k);
    const target = k ? '#k=' + q(k) : ' ';
    if (location.hash !== target.trim()) history.replaceState(null, '', target);
    if (!k) return;
    setTab('detail');
    setCollapsed((c) => {
      const hiding = collapsersOf(graphNow.current, c, k);
      return hiding.length ? new Set([...c].filter((a) => !hiding.includes(a))) : c;
    });
    if (center) setRequest({ center: k });
  }, []);

  // Says what happened: in a big graph a few vanishing dots are easy to miss.
  const toggleCollapse = useCallback((key: string) => {
    const g = graphNow.current, title = `“${titleOf(g, key)}”`;
    if (!collapsible(g, key)) {
      toast(`${title} has nothing under it to collapse.`);
    } else if (collapsed.has(key)) {
      setCollapsed(toggled(collapsed, key));
      toast(`Expanded ${title}.`);
    } else {
      setCollapsed(toggled(collapsed, key));
      toast(`Collapsed ${title}: ${hiddenByCollapse(g, new Set([key])).size} nodes hidden. Double-click it again to expand.`);
    }
  }, [collapsed]);

  const collapseAllDocs = () => {
    setCollapsed(new Set(graph.nodes.filter((n) => n.node_type === 'doc' && collapsible(graph, n.key)).map((n) => n.key)));
  };

  const clearExplain = useCallback(() => { setExplained(null); setQuery(''); }, []);

  const runExplain = useCallback(async (text: string) => {
    if (!text.trim()) { clearExplain(); return; }
    try {
      const res = await fetchExplain(text);
      setExplained(res);
      select(null);
      setTab('detail');
      const keys = new Set(res.explanation.matches.flatMap((m) => [m.node.key, ...m.related.map((r) => r.key)]));
      if (keys.size) setRequest({ fit: keys });
    } catch (err) { toast(errorText(err), true); }
  }, [clearExplain, select]);

  const withBusy = useCallback(async (text: string, fn: () => Promise<void>) => {
    setBusy(text);
    try { await fn(); } catch (err) { toast(errorText(err), true); } finally { setBusy(null); }
  }, []);

  const runSync = () => withBusy('Syncing docs…', async () => {
    const r = await api<SyncReport>('POST', '/api/sync');
    const n = (a: string[] | null) => (a ?? []).length;
    toast(`Docs +${n(r.docs_added)} ~${n(r.docs_updated)} −${n(r.docs_removed)} · sections +${r.sections_added} ~${r.sections_updated} −${r.sections_removed}`);
    await reload();
  });

  const runAI = () => withBusy('Summarizing with Claude… this can take a minute', async () => {
    const r = (await api<{ ai: AIReport }>('POST', '/api/ai')).ai;
    const failed = Object.keys(r.failed ?? {}).length, created = (r.domains_created ?? []).length;
    toast((r.docs ?? []).length === 0 ? 'Nothing to summarize'
      : `Summarized ${r.nodes_summarized} node(s)${created ? ` · ${created} new domain(s)` : ''}${failed ? ` · ${failed} failed` : ''}`, failed > 0);
    await reload();
  });

  const runFetch = useCallback((url: string) => withBusy('Fetching page…', async () => {
    const r = await api<{ saved: SavedWeb; sync: SyncReport }>('POST', '/api/fetch', { url });
    const s = r.saved;
    const pending = ai && r.sync.needs_summary ? ' · run “Summarize with AI” for summaries' : '';
    toast(`${s.created ? 'Saved' : s.changed ? 'Updated' : 'Unchanged'}: ${s.loc}${pending}`);
    await reload();
    select(s.loc, true);
  }), [ai, reload, select, withBusy]);

  const setColorBy = (c: ColorBy) => {
    try { localStorage.setItem(COLOR_KEY, c); } catch { /* storage unavailable */ }
    setColorByState(c);
  };

  const colorOf = useCallback(
    (n: KNode) => (colorBy === 'community' ? communityColor(pal, communityOf(graph, n.key)) : typeColor(pal, n.node_type)),
    [graph, pal, colorBy],
  );
  const visible = useCallback((n: KNode) => isVisible(graph, filters, n), [graph, filters]);
  // Collapsing only folds the graph; the table still lists every node.
  const hidden = useMemo(() => hiddenByCollapse(graph, collapsed), [graph, collapsed]);
  const graphVisible = useCallback((n: KNode) => !hidden.has(n.key) && visible(n), [hidden, visible]);

  // Start: schema (is the AI step available?), the graph, then ?q= or #k=.
  useEffect(() => {
    (async () => {
      try {
        setAI(!!(await api<{ ai: boolean }>('GET', '/api/schema')).ai);
        await reload();
        const qp = new URLSearchParams(location.search).get('q');
        const k = hashKey();
        if (qp) { setQuery(qp); await runExplain(qp); } else if (k) select(k, true);
      } catch (err) {
        setStatus('failed');
        toast(errorText(err), true);
      }
    })();
  }, []);

  useEffect(() => {
    const onHash = () => select(hashKey(), true);
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, [select]);

  useEffect(() => {
    const onKey = (ev: KeyboardEvent) => {
      const tag = document.activeElement?.tagName;
      // The dialog closes itself on Escape, possibly before this runs.
      const inDialog = ev.target instanceof Element && ev.target.closest('[role=dialog]');
      if (ev.key === '/' && tag !== 'INPUT' && tag !== 'TEXTAREA') {
        ev.preventDefault();
        searchRef.current?.focus();
      } else if (ev.key === 'Escape' && !dialog && !inDialog) {
        if (selected) select(null); else if (explained) clearExplain();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [dialog, selected, explained, select, clearExplain]);

  const ctx: AppContext = {
    graph, pal, colorBy, colorOf, ai, version, select, reload, runFetch,
    openDialog: setDialog,
    filterKeyword: (keyword) => setFilters((f) => ({ ...f, keyword })),
    collapsed, toggleCollapse,
  };

  const count = (t: string) => graph.nodes.filter((n) => n.node_type === t).length;
  const pending = graph.nodes.filter((n) => n.needs_summary).length;
  const counts = status === 'loading' ? 'Loading…' : status === 'failed' ? 'Could not load'
    : `${count('domain')} domains · ${count('doc')} docs · ${count('doc_section')} sections` + (pending ? ` · ${pending} need summary` : '');
  const empty = status === 'ready' && graph.nodes.length === 0;

  return (
    <Ctx.Provider value={ctx}>
      {/* Wide: fills the window, panel scrolls. Narrow: graph above panel, the page scrolls. */}
      <Grid h={{ lg: '100%' }} minH="100%" templateRows={{ base: 'auto auto', lg: 'auto minmax(0, 1fr)' }} overflowX="hidden">
        <TopBar counts={counts} mode={mode} setMode={setMode} colorMode={colorMode} toggleColorMode={toggleColorMode}
          query={query} setQuery={(s) => { setQuery(s); if (!s && explained) clearExplain(); }} searchRef={searchRef}
          onExplain={() => runExplain(query)} />
        <Grid templateColumns={{ base: '1fr', lg: '600px minmax(0, 1fr)' }} templateRows={{ base: 'minmax(360px, 55vh) auto', lg: 'minmax(0, 1fr)' }} minH="0">
          <Box as="section" aria-label="Knowledge graph" position="relative" minW="0" minH="0" bg="bg.panel">
            <GraphView graph={graph} pal={pal} colorOf={colorOf} visible={graphVisible} collapsed={collapsed} onToggle={toggleCollapse}
              explain={explained?.explanation ?? null} selected={selected} onSelect={select} request={request} active={mode === 'graph'} />
            {mode === 'table' && (
              <Box position="absolute" inset="0" overflow="auto">
                <TableView visible={visible} selected={selected} sort={sort} setSort={setSort} />
              </Box>
            )}
            {empty && (
              <Box position="absolute" inset="0" display="flex" flexDirection="column" alignItems="center" justifyContent="center" textAlign="center" color="fg.muted" p="16px" gap="6px">
                <Text fontSize="16px" fontWeight="600" color="fg">No knowledge yet</Text>
                <Text>Add markdown files to <Code size="sm">./docs</Code>, then sync:<br /><Code size="sm">knowledge sync</Code></Text>
                <Button size="sm" colorPalette="accent" mt="8px" onClick={runSync}>Sync docs</Button>
              </Box>
            )}
            {busy && (
              <HStack position="absolute" top="12px" left="50%" transform="translateX(-50%)" zIndex={6} gap="8px" bg="bg.panel" borderWidth="1px" borderColor="border"
                rounded="full" px="14px" py="6px" shadow="lg" fontSize="13px" color="fg.muted">
                <Spinner size="xs" color="accent.solid" />{busy}
              </HStack>
            )}
          </Box>
          {/* Left of the graph when wide (below it when narrow). Tabs down its outer
              edge; the content beside them scrolls on its own. */}
          <Box as="aside" order={{ lg: -1 }} display="flex" minH="0" overflow={{ lg: 'hidden' }} bg="bg"
            borderRightWidth={{ lg: '1px' }} borderTopWidth={{ base: '1px', lg: '0' }} borderColor="border">
            <Tabs.Root orientation="vertical" value={tab} onValueChange={(e) => setTab(e.value as PanelTab)} variant="line" size="sm" display="flex" flex="1" minW="0" minH="0">
              <Tabs.List flex="none" w="88px" pt="8px" borderColor="border">
                {([['detail', 'Node Detail'], ['tools', 'Tools']] as const).map(([value, text]) => (
                  <Tabs.Trigger key={value} value={value} h="auto" py="8px" px="12px" whiteSpace="normal" textAlign="start" lineHeight="1.3" justifyContent="flex-start">
                    {text}
                  </Tabs.Trigger>
                ))}
              </Tabs.List>
              <Tabs.Content value="detail" aria-live="polite" flex="1" minW="0" overflowY="auto" px="16px" pt="12px" pb="16px">
                {selected ? <NodeDetail nodeKey={selected} />
                  : explained ? <ExplainPanel explanation={explained.explanation} markdown={explained.markdown} onClear={clearExplain} />
                  : <Overview />}
              </Tabs.Content>
              <Tabs.Content value="tools" flex="1" minW="0" overflowY="auto" px="16px" pt="0" pb="16px">
                <Tools busy={!!busy} onSync={runSync} onAI={runAI} onFetch={() => setDialog({ kind: 'fetch' })} onDomain={() => setDialog({ kind: 'domain' })}
                  filters={filters} setFilters={setFilters} setColorBy={setColorBy} mode={mode} hasExplain={!!explained} onClearExplain={clearExplain}
                  onRelayout={() => setRequest({ relayout: true })} onFit={() => setRequest({ fitAll: true })}
                  onCollapseAll={collapseAllDocs} onExpandAll={collapsed.size ? () => setCollapsed(new Set()) : undefined} />
              </Tabs.Content>
            </Tabs.Root>
          </Box>
        </Grid>
      </Grid>
      <Dialogs request={dialog} onClose={() => setDialog(null)} />
      <Toaster />
    </Ctx.Provider>
  );
}
