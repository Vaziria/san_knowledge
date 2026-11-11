import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Code, ConnectError } from '@connectrpc/connect';
import {
  Badge,
  Box,
  Button,
  CloseButton,
  HStack,
  Heading,
  IconButton,
  Input,
  Link,
  Menu,
  NativeSelect,
  Portal,
  Stack,
  Tabs,
  Text,
  Wrap,
} from '@chakra-ui/react';
import {
  ArrowLeftIcon,
  BanIcon,
  CopyIcon,
  DownloadIcon,
  EllipsisVerticalIcon,
  ExternalLinkIcon,
  HandIcon,
  LayoutGridIcon,
  PlayIcon,
  RefreshCwIcon,
  RotateCcwIcon,
  ScanIcon,
  SearchIcon,
  SquareDashedMousePointerIcon,
  SquareIcon,
  WaypointsIcon,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Link as RouterLink, useParams, useSearchParams } from 'react-router';
import { connections, errorText, keywordResearch } from '../api';
import type { Connection } from '../gen/san/tower/v1/connection_pb';
import type {
  GetTrendsOptionsResponse,
  KeywordCommunity,
  KeywordEdge,
  KeywordNode,
  KeywordResearch,
  TrendsCategory,
  TrendsGeo,
} from '../gen/san/tower/v1/keyword_research_pb';
import { useDark } from '../knowledge/GraphCanvas';
import { downloadCsv, keywordsCsv } from '../keywordresearch/csv';
import { BlacklistPanel, copyText, plural } from '../keywordresearch/Blacklist';
import { communityColor, KeywordGraph, keywordColors, type DragMode } from '../keywordresearch/KeywordGraph';
import { applyChange, applyRemoval, emptyGraph, type Graph } from '../keywordresearch/live';
import {
  active,
  changeText,
  CUSTOM,
  customRange,
  defaultTime,
  duration,
  estimate,
  exploreLink,
  progressText,
  properties,
  statusOf,
  times,
} from '../keywordresearch/trends';
import { usePoll } from '../poll';
import { StreamIndicator, useStream } from '../stream';
import { FormDialog, toast } from '../ui';

// Form is the research's settings as the top bar edits them.
type Form = {
  seed: string;
  deepLimit: string;
  geo: string;
  time: string;
  from: string;
  to: string;
  category: number;
  property: string;
  connectionId: number;
};

function formOf(k: KeywordResearch): Form {
  const t = k.filters?.time || defaultTime;
  const m = customRange.exec(t);
  return {
    seed: k.seed,
    deepLimit: String(k.deepLimit || 2),
    geo: k.filters?.geo ?? '',
    time: m ? CUSTOM : t,
    from: m?.[1] ?? '',
    to: m?.[2] ?? '',
    category: k.filters?.category ?? 0,
    property: k.filters?.property ?? '',
    connectionId: k.connectionId,
  };
}

// timeOf is the form's time range as Trends writes it; "" while a custom
// range is not complete.
const timeOf = (f: Form) => (f.time !== CUSTOM ? f.time : f.from && f.to && f.from <= f.to ? `${f.from} ${f.to}` : '');

// countryOf is a location's country: ID of ID-JK.
const countryOf = (geo: string) => geo.split('-')[0];

// flatCategories lists the category tree in order, each with its depth.
function flatCategories(list: TrendsCategory[], depth = 0, out: { c: TrendsCategory; depth: number }[] = []) {
  for (const c of list) {
    out.push({ c, depth });
    flatCategories(c.children, depth + 1, out);
  }
  return out;
}

// MAX_LISTED is how many keywords a node's panel lists under "found here".
const MAX_LISTED = 60;

// FEW is how many keywords blacklisting one keyword of the graph may remove
// without asking first.
const FEW = 3;

// What a drag on the graph's empty space does, Pan or Select (the user,
// 2026-11-12: "Add select area"), as the workflow editor's toggle; the
// browser remembers it.
const dragModeKey = 'san-tower-keywords-drag';

function loadDragMode(): DragMode {
  try {
    return localStorage.getItem(dragModeKey) === 'select' ? 'select' : 'pan';
  } catch {
    return 'pan';
  }
}

function saveDragMode(mode: DragMode) {
  try {
    localStorage.setItem(dragModeKey, mode);
  } catch {
    // Storage blocked: the graph just starts in Pan next time.
  }
}

// KeywordResearchPage is a research's page (/keyword-research/<id>; the
// user, 2026-11-11): its seed keyword, deep limit, Google Trends' filters
// (location, date range, category, search type, as Trends lists them) and
// San Remote connection across the top, saved as they change; Run and Stop;
// and its graph, grouped by Leiden community, growing while its crawl runs.
// The crawl runs in the tower, so it goes on when the page closes; the page
// follows it live (the user, 2026-11-11: "make it realtime") through
// WatchResearch: a snapshot, then each lookup's keywords and links as soon
// as they are written, drawn into the graph without laying it out again
// (KeywordGraph), the status, progress and legend with them. A dropped
// stream (a tower restart) is opened again with backoff and starts from a
// new snapshot; meanwhile the page asks every 15 s. A click on a keyword
// shows where it was found and what it found, with Open in Google Trends and
// Explore from here.
//
// Two tabs (the user, 2026-11-12: "Add tab blacklist keyword"): Graph, all of
// the above, and Blacklist (BlacklistPanel), with its count; the tab is in
// the address (?tab=blacklist). The graph stays drawn while the Blacklist tab
// shows, so it keeps up live and comes back as it was. A right-click on a
// keyword opens its menu (the user, 2026-11-12: "And in node when right
// click add context menu add to blacklist, and remove the node and child"):
// Add to blacklist (not on the seed), Open in Google Trends, Explore from
// here, Copy keyword; on a selection of several (Shift+drag, Select, or
// Ctrl+click: "Add select area and right click add to blacklist too"), Add N
// keywords to blacklist and Copy keywords. Blacklisting from the graph asks
// first with what it removes (a dry run) when that is more than a few
// keywords, and always for several; the keywords and those found only
// through them fade out live, in every tab.
export function KeywordResearchPage() {
  const id = Number(useParams().id);
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'blacklist' ? 'blacklist' : 'graph';
  // shown is the display of what belongs to the Graph tab: none while the
  // Blacklist tab shows (given always: a layout's own display is a prop).
  const shown = (display: 'flex' | 'block') => (tab === 'graph' ? display : 'none');
  const [menu, setMenu] = useState<{ name: string; names: string[]; at: { x: number; y: number } }>();
  const [confirm, setConfirm] = useState<{ entries: string[]; removed: string[]; seedStays: boolean }>();
  const [dragMode, setDragMode] = useState(loadDragMode);
  const dark = useDark();
  const [k, setK] = useState<KeywordResearch>();
  const [graph, setGraph] = useState<Graph>();
  const [form, setForm] = useState<Form>();
  const [error, setError] = useState('');
  const [opts, setOpts] = useState<GetTrendsOptionsResponse>();
  const [optsError, setOptsError] = useState('');
  const [optsLoading, setOptsLoading] = useState(false);
  const [regions, setRegions] = useState<Map<string, TrendsGeo[]>>(new Map());
  const [conns, setConns] = useState<Connection[]>();
  const [selected, setSelected] = useState<string>();
  const [focus, setFocus] = useState<number>();
  const [layout, setLayout] = useState(0);
  const [fit, setFit] = useState(0);
  const [asking, setAsking] = useState<'clear' | 'restart'>();
  const [busy, setBusy] = useState(false);
  const [find, setFind] = useState('');
  const [gone, setGone] = useState(false);
  const dirty = useRef(false);
  const seen = useRef('');
  const wasActive = useRef(false);

  // took shows a research as it is now. The graph is laid out once more by
  // community when a crawl ends (the user, 2026-11-11).
  const took = (research: KeywordResearch) => {
    setK(research);
    setForm((f) => f ?? formOf(research));
    setError('');
    const ended = wasActive.current && !active(research);
    wasActive.current = active(research);
    if (ended) setLayout((n) => n + 1);
  };
  const signature = (r: KeywordResearch) => [r.nodeCount, r.edgeCount, r.exploredCount, r.status].join('|');

  // The research and its graph as they change, pushed by the tower.
  const live = useStream(
    (signal) => keywordResearch.watchResearch({ id }, { signal }),
    (m) => {
      const e = m.event;
      switch (e.case) {
        case 'snapshot':
          setGraph({ nodes: e.value.nodes, edges: e.value.edges, communities: e.value.communities });
          if (e.value.research) {
            seen.current = signature(e.value.research);
            took(e.value.research);
          }
          break;
        case 'research':
          took(e.value);
          break;
        case 'change': {
          const ch = e.value;
          setGraph((g) => applyChange(g ?? emptyGraph, ch));
          break;
        }
        case 'removal': {
          const r = e.value;
          setGraph((g) => applyRemoval(g ?? emptyGraph, r));
          setSelected((s) => (s && r.removed.includes(s) ? undefined : s));
          setMenu((m) => (m && m.names.some((n) => r.removed.includes(n)) ? undefined : m));
          break;
        }
        case 'deleted':
          setGone(true);
          break;
      }
    },
    [id],
    { onEnd: (err) => setError(errorText(err)) },
  );
  // fromCall shows the research a call (Run, Stop, a save) answered with,
  // only while the stream is down: live, the stream has it, and an answer
  // read a moment before the crawl changed it would put the old status back
  // (Stop's answer said Running after the stream had said Stopped).
  const fromCall = (research?: KeywordResearch) => {
    if (research && live !== 'live') setK(research);
  };

  // Only while the stream is down: the research every 15 s, its graph again
  // when its counts or status changed.
  const running = active(k);
  usePoll(
    async (stale) => {
      try {
        const { research } = await keywordResearch.getResearch({ id });
        if (stale() || !research) return;
        const sig = signature(research);
        if (sig !== seen.current) {
          const g = await keywordResearch.getResearchGraph({ id });
          if (stale()) return;
          seen.current = sig;
          setGraph({ nodes: g.nodes, edges: g.edges, communities: g.communities });
        }
        took(research);
      } catch (err) {
        if (!stale()) setError(errorText(err));
      }
    },
    15000,
    [id],
    live === 'reconnecting',
  );

  useEffect(() => {
    connections.listConnections({ type: 'san_remote' }).then(
      (r) => setConns(r.connections),
      () => setConns([]),
    );
  }, []);

  // Trends' lists: from the tower's file, or read from Trends the first time.
  const loadOptions = async (refresh = false) => {
    setOptsLoading(true);
    try {
      setOpts(await keywordResearch.getTrendsOptions({ connectionId: form?.connectionId ?? 0, refresh }));
      setOptsError('');
      if (refresh) setRegions(new Map());
    } catch (err) {
      setOptsError(errorText(err));
    } finally {
      setOptsLoading(false);
    }
  };
  const formReady = !!form;
  useEffect(() => {
    if (formReady) void loadOptions();
  }, [formReady]);

  // A country's regions, once it is picked.
  const country = form ? countryOf(form.geo) : '';
  const countryGeo = opts?.geos.find((g) => g.code === country);
  useEffect(() => {
    if (!country || !countryGeo?.hasRegions || regions.has(country)) return;
    keywordResearch.getTrendsOptions({ connectionId: form?.connectionId ?? 0, country }).then(
      (r) => setRegions((m) => new Map(m).set(country, r.regions)),
      (err: unknown) => toast(`The regions of ${countryGeo.name} were not read: ${errorText(err)}`, true),
    );
  }, [country, countryGeo?.hasRegions]);

  const save = async (f = form) => {
    if (!k || !f) return;
    dirty.current = false;
    const limit = Number(f.deepLimit);
    const time = timeOf(f);
    if (!Number.isInteger(limit) || limit < 1 || limit > 6 || !time) return;
    try {
      const { research } = await keywordResearch.updateResearch({
        id: k.id,
        name: k.name,
        description: k.description,
        seed: f.seed,
        deepLimit: limit,
        filters: { geo: f.geo, time, category: f.category, property: f.property },
        connectionId: f.connectionId,
      });
      fromCall(research);
    } catch (err) {
      toast(`Not saved: ${errorText(err)}`, true);
    }
  };
  // Each change is saved once the typing pauses.
  useEffect(() => {
    if (!dirty.current) return;
    const t = setTimeout(() => void save(), 600);
    return () => clearTimeout(t);
  }, [form]);
  const change = (patch: Partial<Form>) => {
    dirty.current = true;
    setForm((f) => (f ? { ...f, ...patch } : f));
  };

  const run = async (clear = false) => {
    setBusy(true);
    try {
      if (dirty.current) await save();
      const { research } = await keywordResearch.runResearch({ id, clear });
      fromCall(research);
      toast(active(research) ? `${research!.name}: ${statusOf(research!).label.toLowerCase()}` : 'Nothing to look up');
    } catch (err) {
      if (err instanceof ConnectError && err.code === Code.FailedPrecondition && /other settings/.test(err.rawMessage)) {
        setAsking('restart');
      } else {
        toast(`Not run: ${errorText(err)}`, true);
      }
    } finally {
      setBusy(false);
    }
  };
  const stop = async () => {
    try {
      const { research } = await keywordResearch.stopResearch({ id });
      fromCall(research);
      toast('Stopping after the lookup it is doing');
    } catch (err) {
      toast(`Not stopped: ${errorText(err)}`, true);
    }
  };
  const explore = async (keyword: string) => {
    try {
      const { research } = await keywordResearch.exploreKeyword({ id, keyword });
      fromCall(research);
      toast(`Queued: ${keyword} is looked up once`);
    } catch (err) {
      toast(`Not queued: ${errorText(err)}`, true);
    }
  };

  // blacklist blacklists keywords of the graph, each one an entry, the seed
  // left out: at once when it removes a few keywords, else after asking,
  // and always after asking for several.
  const blacklist = async (names: string[]) => {
    const seed = graph?.nodes.find((n) => n.seed)?.name;
    const entries = names.filter((n) => n !== seed);
    if (!entries.length) return;
    try {
      const r = await keywordResearch.addToBlacklist({ id, entries, dryRun: true });
      if (names.length > 1 || r.removed.length > FEW) {
        setConfirm({ entries, removed: r.removed, seedStays: entries.length < names.length });
      } else {
        await addBlacklist(entries);
      }
    } catch (err) {
      toast(`Not blacklisted: ${errorText(err)}`, true);
    }
  };
  const addBlacklist = async (entries: string[]) => {
    const r = await keywordResearch.addToBlacklist({ id, entries });
    fromCall(r.research);
    toast(
      `Blacklisted ${entries.length === 1 ? `'${entries[0]}'` : plural(entries.length, 'keyword')}: removed ${plural(r.removed.length, 'keyword')}`,
    );
  };
  const copy = async (names: string[]) => {
    try {
      await copyText(names.join('\n'));
      toast(names.length === 1 ? `Copied '${names[0]}'` : `Copied ${plural(names.length, 'keyword')}`);
    } catch (err) {
      toast(`Not copied: ${errorText(err)}`, true);
    }
  };
  const switchDragMode = () => {
    const next = dragMode === 'pan' ? 'select' : 'pan';
    setDragMode(next);
    saveDragMode(next);
  };

  const est = useMemo(() => {
    if (!graph || !form) return undefined;
    const limit = Number(form.deepLimit) || 0;
    return k?.settingsChanged ? estimate([], [], limit) : estimate(graph.nodes, graph.edges, limit);
  }, [graph, form?.deepLimit, k?.settingsChanged]);
  const node = selected ? graph?.nodes.find((n) => n.name === selected) : undefined;
  const st = k ? statusOf(k) : undefined;
  const p = keywordColors(dark);
  const back = k?.folderId ? `/keyword-research?folder=${k.folderId}` : '/keyword-research';
  const locked = running; // the crawl uses the seed, filters and connection

  const categories = useMemo(() => flatCategories(opts?.categories ?? []), [opts]);
  const geos = opts?.geos ?? [];

  return (
    <Stack gap="3" h="calc(100vh - 3rem)">
      <Button asChild size="sm" variant="ghost" color="fg.muted" alignSelf="flex-start" ms="-3">
        <RouterLink to={back}>
          <ArrowLeftIcon />
          Keyword Research
        </RouterLink>
      </Button>
      {gone ? (
        <Text color="fg.muted">This research was deleted.</Text>
      ) : error && !k ? (
        <Text color="fg.error">{error}</Text>
      ) : !k || !form || !graph ? (
        <Text color="fg.muted">Loading...</Text>
      ) : (
        <>
          <HStack justify="space-between" align="flex-start" gap="2">
            <Stack gap="0" minW="0">
              <Heading size="lg">{k.name}</Heading>
              {k.description && (
                <Text color="fg.muted" textStyle="sm">
                  {k.description}
                </Text>
              )}
            </Stack>
            <Menu.Root
              positioning={{ placement: 'bottom-end' }}
              onSelect={(e) => {
                if (e.value === 'lists') void loadOptions(true);
              }}
            >
              <Menu.Trigger asChild>
                <IconButton aria-label="More" title="More" size="sm" variant="ghost">
                  <EllipsisVerticalIcon />
                </IconButton>
              </Menu.Trigger>
              <Portal>
                <Menu.Positioner>
                  <Menu.Content>
                    <Menu.Item value="lists">
                      <RefreshCwIcon />
                      Read Trends' lists again
                    </Menu.Item>
                  </Menu.Content>
                </Menu.Positioner>
              </Portal>
            </Menu.Root>
          </HStack>

          <Tabs.Root size="sm" value={tab} onValueChange={(e) => setParams(e.value === 'graph' ? {} : { tab: e.value })}>
            <Tabs.List>
              <Tabs.Trigger value="graph">
                <WaypointsIcon />
                Graph
              </Tabs.Trigger>
              <Tabs.Trigger value="blacklist">
                <BanIcon />
                Blacklist ({k.blacklist.length})
              </Tabs.Trigger>
            </Tabs.List>
          </Tabs.Root>

          <Wrap gap="3" align="flex-end" display={shown('flex')}>
            <Field label="Seed keyword" w="56">
              <Input
                size="sm"
                value={form.seed}
                disabled={locked}
                onChange={(e) => change({ seed: e.currentTarget.value })}
                placeholder="aplikasi kasir"
              />
            </Field>
            <Field label="Deep limit" w="20">
              <Input
                size="sm"
                type="number"
                min={1}
                max={6}
                value={form.deepLimit}
                onChange={(e) => change({ deepLimit: e.currentTarget.value })}
                title="How deep Top keywords are looked up: the seed is depth 0"
              />
            </Field>
            <Field label="Location" w="44">
              <Select
                value={country}
                disabled={locked}
                onChange={(v) => change({ geo: v })}
                options={[
                  ...(geos.length ? geos : [{ code: '', name: 'Worldwide' } as TrendsGeo]).map((g) => ({
                    value: g.code,
                    label: g.name,
                  })),
                  ...(country && !geos.some((g) => g.code === country) ? [{ value: country, label: country }] : []),
                ]}
              />
            </Field>
            {countryGeo?.hasRegions && (
              <Field label="Region" w="48">
                <Select
                  value={form.geo}
                  disabled={locked || !regions.has(country)}
                  onChange={(v) => change({ geo: v })}
                  options={[
                    { value: country, label: `All of ${countryGeo.name}` },
                    ...(regions.get(country) ?? []).map((r) => ({ value: r.code, label: r.name })),
                    ...(form.geo !== country && !regions.get(country)?.some((r) => r.code === form.geo)
                      ? [{ value: form.geo, label: form.geo }]
                      : []),
                  ]}
                />
              </Field>
            )}
            <Field label="Date range" w="40">
              <Select
                value={form.time}
                disabled={locked}
                onChange={(v) => change({ time: v })}
                options={[...times, { value: CUSTOM, label: 'Custom range' }]}
              />
            </Field>
            {form.time === CUSTOM && (
              <>
                <Field label="From" w="36">
                  <Input
                    size="sm"
                    type="date"
                    min="2004-01-01"
                    value={form.from}
                    disabled={locked}
                    onChange={(e) => change({ from: e.currentTarget.value })}
                  />
                </Field>
                <Field label="To" w="36">
                  <Input
                    size="sm"
                    type="date"
                    min="2004-01-01"
                    value={form.to}
                    disabled={locked}
                    onChange={(e) => change({ to: e.currentTarget.value })}
                  />
                </Field>
              </>
            )}
            <Field label="Category" w="56">
              <Select
                value={String(form.category)}
                disabled={locked}
                onChange={(v) => change({ category: Number(v) })}
                options={[
                  { value: '0', label: 'All categories' },
                  ...categories.map(({ c, depth }) => ({
                    value: String(c.id),
                    label: `${'\u00a0\u00a0\u00a0'.repeat(depth)}${c.name}`,
                  })),
                  ...(form.category && !categories.some(({ c }) => c.id === form.category)
                    ? [{ value: String(form.category), label: `Category ${form.category}` }]
                    : []),
                ]}
              />
            </Field>
            <Field label="Search type" w="40">
              <Select
                value={form.property}
                disabled={locked}
                onChange={(v) => change({ property: v })}
                options={properties}
              />
            </Field>
            <Field label="San Remote" w="44">
              <Select
                value={String(form.connectionId)}
                disabled={locked}
                onChange={(v) => change({ connectionId: Number(v) })}
                options={[
                  {
                    value: '0',
                    label: conns?.length ? `${conns[0].name} (the first)` : 'The first connection',
                  },
                  ...(conns ?? []).map((c) => ({ value: String(c.id), label: c.name })),
                ]}
              />
            </Field>
            <HStack gap="2" pb="0.5">
              {running ? (
                <Button size="sm" colorPalette="red" variant="outline" onClick={() => void stop()}>
                  <SquareIcon />
                  Stop
                </Button>
              ) : (
                <Button
                  size="sm"
                  colorPalette="blue"
                  loading={busy}
                  disabled={!form.seed.trim()}
                  onClick={() => void run()}
                >
                  <PlayIcon />
                  Run
                </Button>
              )}
              {/* Reset (the user, 2026-11-12): the graph emptied, to start
                  again from the seed; it asks first. */}
              <Button
                size="sm"
                variant="outline"
                disabled={locked || graph.nodes.length === 0}
                title="Delete the graph's keywords, to start again from the seed"
                onClick={() => setAsking('clear')}
              >
                <RotateCcwIcon />
                Reset
              </Button>
            </HStack>
          </Wrap>

          <HStack gap="2" wrap="wrap" textStyle="sm" minH="6" display={shown('flex')}>
            {st && <Badge colorPalette={st.color}>{st.label}</Badge>}
            {running ? (
              <Text color="fg.muted">{progressText(k)}</Text>
            ) : (
              <Text color={st?.label === 'Failed' ? 'fg.error' : 'fg.muted'}>
                {k.message ||
                  (k.lastRunAt ? `Last run ${timestampDate(k.lastRunAt).toLocaleString()}` : 'Not run yet.')}
              </Text>
            )}
            {!running && est && est.lookups > 0 && (
              <Text color="fg.muted">
                · Run: up to {est.lookups.toLocaleString()} {est.lookups === 1 ? 'lookup' : 'lookups'},{' '}
                {duration(est.lookups)}
                {graph.nodes.length > 0 && !k.settingsChanged ? ', going on from where it is' : ''}
                {k.settingsChanged ? ' (the graph is cleared first)' : ''}
              </Text>
            )}
            {!running && est && est.lookups === 0 && graph.nodes.length > 0 && (
              <Text color="fg.muted">· Every Top keyword within the deep limit is looked up: raise it to go deeper.</Text>
            )}
            <Box ms="auto">
              <StreamIndicator state={live} />
            </Box>
          </HStack>
          {(optsError || optsLoading) && (
            <Text textStyle="xs" color={optsError ? 'fg.warning' : 'fg.muted'} display={shown('block')}>
              {optsLoading
                ? "Reading Trends' locations and categories through San Remote (once; the tower keeps them)..."
                : `Trends' locations and categories are not read yet: ${optsError}`}{' '}
              {optsError && (
                <Link as="button" onClick={() => void loadOptions()}>
                  Try again
                </Link>
              )}
            </Text>
          )}

          <Box
            flex="1"
            minH="80"
            position="relative"
            borderWidth="1px"
            rounded="md"
            bg="bg.panel"
            overflow="hidden"
            display={shown('block')}
          >
            {graph.nodes.length === 0 ? (
              <Text color="fg.muted" p="4">
                No keywords yet: Run looks up the seed keyword's related queries first.
              </Text>
            ) : (
              <>
                <KeywordGraph
                  nodes={graph.nodes}
                  edges={graph.edges}
                  communities={graph.communities}
                  selected={selected}
                  focus={focus}
                  layout={layout}
                  fit={fit}
                  dragMode={dragMode}
                  onSelect={setSelected}
                  onMenu={(name, at, names) => setMenu({ name, at, names })}
                />
                <HStack position="absolute" top="2" left="2" gap="1">
                  {/* Pan or Select: the icon is the mode it is in; a click
                      switches. */}
                  <IconButton
                    size="xs"
                    variant="outline"
                    bg="bg.panel"
                    aria-label={dragMode === 'pan' ? 'Drag pans' : 'Drag selects'}
                    title={
                      dragMode === 'pan'
                        ? 'Drag on empty space pans (Shift+drag selects). Click to make a drag select.'
                        : 'Drag on empty space selects keywords (the wheel zooms). Click to make a drag pan.'
                    }
                    onClick={switchDragMode}
                  >
                    {dragMode === 'pan' ? <HandIcon /> : <SquareDashedMousePointerIcon />}
                  </IconButton>
                  {/* Relayout, Fit and Download CSV (the user, 2026-11-12). */}
                  <Button
                    size="xs"
                    variant="outline"
                    bg="bg.panel"
                    title="Lay the graph out again, sorted by Leiden community: the largest first"
                    onClick={() => setLayout((n) => n + 1)}
                  >
                    <LayoutGridIcon />
                    Relayout
                  </Button>
                  <Button
                    size="xs"
                    variant="outline"
                    bg="bg.panel"
                    title="Fit the whole graph in view"
                    onClick={() => setFit((n) => n + 1)}
                  >
                    <ScanIcon />
                    Fit
                  </Button>
                  <Button
                    size="xs"
                    variant="outline"
                    bg="bg.panel"
                    title="Every keyword as a CSV file: keyword, type, interest, change, breakout, depth, community count"
                    onClick={() => downloadCsv(k.name, keywordsCsv(graph.nodes, graph.communities))}
                  >
                    <DownloadIcon />
                    Download CSV
                  </Button>
                  {/* Find a keyword: picking one selects it, as a click does. */}
                  <Input
                    size="xs"
                    w="56"
                    bg="bg.panel"
                    list="keywords"
                    aria-label="Find a keyword"
                    placeholder="Find a keyword"
                    value={find}
                    onChange={(e) => {
                      const v = e.currentTarget.value;
                      setFind(v);
                      const name = v.trim().toLowerCase();
                      if (graph.nodes.some((n) => n.name === name)) setSelected(name);
                    }}
                    onKeyDown={(e) => {
                      if (e.key !== 'Enter') return;
                      const q = find.trim().toLowerCase();
                      const hit = graph.nodes.find((n) => n.name === q) ?? graph.nodes.find((n) => n.name.includes(q));
                      if (hit) setSelected(hit.name);
                    }}
                  />
                  <datalist id="keywords">
                    {graph.nodes.map((n) => (
                      <option key={n.name} value={n.name} />
                    ))}
                  </datalist>
                </HStack>
                <Legend
                  communities={graph.communities}
                  color={(c) => communityColor(p, c)}
                  focus={focus}
                  onFocus={setFocus}
                />
                {node && (
                  <NodePanel
                    node={node}
                    graph={graph}
                    research={k}
                    color={communityColor(p, node.community)}
                    onSelect={setSelected}
                    onExplore={(name) => void explore(name)}
                    onBlacklist={(name) => void blacklist([name])}
                    onClose={() => setSelected(undefined)}
                  />
                )}
              </>
            )}
          </Box>
          <Text fontSize="xs" color="fg.muted" display={shown('block')}>
            {graph.nodes.length} keywords, {graph.edges.length} links, {graph.communities.length} communities · fill:
            community · size: search interest · solid link: Top · dashed link: Rising · dashed border: found only as
            Rising · thick border: the seed · click a keyword for where it was found · right-click for its menu ·
            Shift+drag (or Select) picks several, Ctrl+click one more, Escape none · drag to pan · wheel to zoom
          </Text>
          {tab === 'blacklist' && <BlacklistPanel research={k} keywords={graph.nodes.length} />}
        </>
      )}
      {menu && graph && k && (
        <KeywordMenu
          menu={menu}
          graph={graph}
          research={k}
          onClose={() => setMenu(undefined)}
          onBlacklist={(names) => void blacklist(names)}
          onExplore={(name) => void explore(name)}
          onCopy={(names) => void copy(names)}
        />
      )}
      {confirm && (
        <FormDialog
          title={
            confirm.entries.length === 1
              ? `Blacklist '${confirm.entries[0]}' and remove ${plural(confirm.removed.length, 'keyword')}?`
              : `Blacklist ${confirm.entries.length} keywords and remove ${confirm.removed.length}?`
          }
          submitText="Blacklist"
          danger
          onClose={() => setConfirm(undefined)}
          onSubmit={() => addBlacklist(confirm.entries)}
        >
          <Text>
            {confirm.entries.length === 1 ? 'It becomes a blacklist entry' : 'Each becomes a blacklist entry'}: the
            keywords that have its words and those found only through them leave the graph, and a crawl never adds them
            again. Taking an entry out of the blacklist later does not bring them back.
          </Text>
          {confirm.seedStays && <Text color="fg.warning">The seed stays: it cannot be blacklisted.</Text>}
          <Text textStyle="sm" color="fg.muted" data-testid="blacklist-removes">
            Removes: {confirm.removed.slice(0, 12).join(', ')}
            {confirm.removed.length > 12 ? ` and ${confirm.removed.length - 12} more` : ''}
          </Text>
        </FormDialog>
      )}
      {asking === 'clear' && (
        <FormDialog
          title="Reset the graph?"
          submitText="Reset"
          danger
          onClose={() => setAsking(undefined)}
          onSubmit={async () => {
            const { research } = await keywordResearch.clearResearchGraph({ id });
            fromCall(research);
            setGraph(emptyGraph);
            setSelected(undefined);
            seen.current = '';
            toast('Reset the graph: Run starts again from the seed');
          }}
        >
          <Text>
            Its {graph?.nodes.length ?? 0} keywords and their links are deleted. The research and its settings stay; Run
            starts again from the seed.
          </Text>
        </FormDialog>
      )}
      {asking === 'restart' && (
        <FormDialog
          title="The graph was made with other settings: clear it and start over?"
          submitText="Clear and run"
          danger
          onClose={() => setAsking(undefined)}
          onSubmit={async () => {
            seen.current = '';
            await run(true);
          }}
        >
          <Text>
            The seed keyword or a filter changed since its keywords were looked up. Running with the new settings clears
            its {graph?.nodes.length ?? 0} keywords first, then starts from the seed.
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}

// KeywordMenu is a right-click's menu on the graph, at the pointer: for one
// keyword Add to blacklist (never the seed), Open in Google Trends, Explore
// from here and Copy keyword; for a selection of several, Add N keywords to
// blacklist (the seed left out) and Copy keywords, one per line. Its
// trigger is an empty box at the pointer, as the Knowledge graph's: a menu
// opened from code ignores its anchorPoint prop. A new key per opening
// places it afresh.
function KeywordMenu({
  menu,
  graph,
  research,
  onClose,
  onBlacklist,
  onExplore,
  onCopy,
}: {
  menu: { name: string; names: string[]; at: { x: number; y: number } };
  graph: Graph;
  research: KeywordResearch;
  onClose: () => void;
  onBlacklist: (names: string[]) => void;
  onExplore: (name: string) => void;
  onCopy: (names: string[]) => void;
}) {
  const node = graph.nodes.find((n) => n.name === menu.name);
  if (!node) return null;
  const several = menu.names.length > 1;
  const seed = graph.nodes.find((n) => n.seed)?.name;
  const blacklistable = menu.names.filter((n) => n !== seed).length;
  const queued = research.progress?.keyword === node.name;
  return (
    <Menu.Root
      key={`${menu.at.x},${menu.at.y}`}
      open
      positioning={{ placement: 'bottom-start', gutter: 2 }}
      onOpenChange={(e) => !e.open && onClose()}
      onSelect={(e) => {
        if (e.value === 'blacklist') onBlacklist(menu.names);
        if (e.value === 'explore') onExplore(node.name);
        if (e.value === 'copy') onCopy(menu.names);
      }}
    >
      <Menu.Trigger asChild>
        <Box position="fixed" left={`${menu.at.x}px`} top={`${menu.at.y}px`} w="0" h="0" pointerEvents="none" />
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content minW="52">
            <Menu.ItemGroup>
              <Menu.ItemGroupLabel maxW="72" truncate>
                {several ? `${menu.names.length} keywords selected` : node.name}
              </Menu.ItemGroupLabel>
              {blacklistable > 0 && (
                <Menu.Item value="blacklist" color="fg.error">
                  <BanIcon />
                  {several ? `Add ${plural(blacklistable, 'keyword')} to blacklist` : 'Add to blacklist'}
                </Menu.Item>
              )}
              {!several && (
                <>
                  <Menu.Item value="trends" asChild>
                    <a href={exploreLink(node.name, research.filters)} target="_blank" rel="noreferrer">
                      <ExternalLinkIcon />
                      Open in Google Trends
                    </a>
                  </Menu.Item>
                  <Menu.Item value="explore" disabled={node.explored || queued || research.settingsChanged}>
                    <SearchIcon />
                    Explore from here
                  </Menu.Item>
                </>
              )}
              <Menu.Item value="copy">
                <CopyIcon />
                {several ? 'Copy keywords' : 'Copy keyword'}
              </Menu.Item>
            </Menu.ItemGroup>
          </Menu.Content>
        </Menu.Positioner>
      </Portal>
    </Menu.Root>
  );
}

function Field({ label, w, children }: { label: string; w: string; children: ReactNode }) {
  return (
    <Stack gap="1" w={w}>
      <Text textStyle="xs" color="fg.muted" fontWeight="medium">
        {label}
      </Text>
      {children}
    </Stack>
  );
}

function Select({
  value,
  options,
  disabled,
  onChange,
}: {
  value: string;
  options: { value: string; label: string }[];
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <NativeSelect.Root size="sm" disabled={disabled}>
      <NativeSelect.Field value={value} onChange={(e) => onChange(e.currentTarget.value)}>
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </NativeSelect.Field>
      <NativeSelect.Indicator />
    </NativeSelect.Root>
  );
}

// Legend lists the communities, each named after its keyword of the highest
// search interest, with how many keywords it has; a click shows only it.
function Legend({
  communities,
  color,
  focus,
  onFocus,
}: {
  communities: KeywordCommunity[];
  color: (c: number) => string;
  focus?: number;
  onFocus: (c?: number) => void;
}) {
  const [all, setAll] = useState(false);
  const shown = all ? communities : communities.slice(0, 10);
  return (
    <Stack
      position="absolute"
      left="2"
      bottom="2"
      gap="0.5"
      bg="bg.panel"
      borderWidth="1px"
      rounded="md"
      px="2"
      py="1.5"
      maxH="60%"
      overflowY="auto"
      maxW="64"
      textStyle="xs"
    >
      <Text fontWeight="semibold" color="fg.muted">
        Communities
      </Text>
      {shown.map((c) => (
        <HStack
          key={c.id}
          gap="1.5"
          cursor="pointer"
          opacity={focus === undefined || focus === c.id ? 1 : 0.45}
          onClick={() => onFocus(focus === c.id ? undefined : c.id)}
          title={focus === c.id ? 'Show every community' : 'Show only this community'}
        >
          <Box w="2.5" h="2.5" rounded="full" bg={color(c.id)} flexShrink="0" />
          <Text truncate>{c.name}</Text>
          <Text color="fg.muted" ms="auto">
            {c.size}
          </Text>
        </HStack>
      ))}
      {communities.length > 10 && (
        <Link as="button" color="fg.muted" onClick={() => setAll((a) => !a)}>
          {all ? 'Fewer' : `${communities.length - 10} more`}
        </Link>
      )}
    </Stack>
  );
}

// NodePanel is a keyword's side panel: its values, the keywords it was found
// from with the value it had in each (the user, 2026-11-11: "from
// 'aplikasi kasir' (interest 80)"), the ones it found, Open in Google Trends,
// Explore from here and Add to blacklist (not on the seed).
function NodePanel({
  node,
  graph,
  research,
  color,
  onSelect,
  onExplore,
  onBlacklist,
  onClose,
}: {
  node: KeywordNode;
  graph: Graph;
  research: KeywordResearch;
  color: string;
  onSelect: (name: string) => void;
  onExplore: (name: string) => void;
  onBlacklist: (name: string) => void;
  onClose: () => void;
}) {
  const from = graph.edges.filter((e) => e.to === node.name);
  const found = graph.edges.filter((e) => e.from === node.name);
  const community = graph.communities.find((c) => c.id === node.community);
  const value = (e: KeywordEdge) =>
    e.rel === 'TOP' ? `interest ${e.searchInterest}` : `change ${changeText(e) || `${e.change}%`}`;
  const queued = research.progress && research.progress.keyword === node.name;
  return (
    <Stack
      position="absolute"
      top="2"
      right="2"
      bottom="2"
      w="80"
      maxW="calc(100% - 1rem)"
      bg="bg.panel"
      borderWidth="1px"
      rounded="md"
      shadow="md"
      p="3"
      gap="3"
      overflowY="auto"
      textStyle="sm"
    >
      <HStack justify="space-between" align="flex-start">
        <HStack gap="2" align="flex-start" minW="0">
          <Box w="3" h="3" mt="1.5" rounded="full" bg={color} flexShrink="0" />
          <Heading size="sm" wordBreak="break-word">
            {node.name}
          </Heading>
        </HStack>
        <CloseButton size="2xs" onClick={onClose} aria-label="Close" />
      </HStack>
      <Wrap gap="1">
        {node.seed && <Badge>Seed</Badge>}
        {node.top && <Badge colorPalette="blue">Top</Badge>}
        {node.rising && <Badge colorPalette="orange">Rising</Badge>}
        <Badge colorPalette={node.explored ? 'green' : 'gray'}>{node.explored ? 'Looked up' : 'Not looked up'}</Badge>
      </Wrap>
      <Stack gap="0.5">
        <Row label="Search interest" value={node.seed && !node.searchInterest ? '-' : String(node.searchInterest)} />
        <Row label="Change" value={changeText(node) || '-'} />
        <Row label="Depth" value={String(node.depth)} />
        <Row label="Community" value={`${node.community + 1}${community ? ` · ${community.name} (${community.size})` : ''}`} />
        {node.foundAt && <Row label="Found" value={timestampDate(node.foundAt).toLocaleString()} />}
      </Stack>
      <HStack gap="2" wrap="wrap">
        <Button asChild size="xs" variant="outline">
          <a href={exploreLink(node.name, research.filters)} target="_blank" rel="noreferrer">
            <ExternalLinkIcon />
            Open in Google Trends
          </a>
        </Button>
        <Button
          size="xs"
          variant="outline"
          disabled={node.explored || !!queued || research.settingsChanged}
          title={
            node.explored
              ? 'Looked up already'
              : research.settingsChanged
                ? 'The graph was made with other settings'
                : 'Look this keyword up once, past the deep limit or a Rising one too; it waits its turn as a Run does'
          }
          onClick={() => onExplore(node.name)}
        >
          <SearchIcon />
          Explore from here
        </Button>
        {!node.seed && (
          <Button
            size="xs"
            variant="outline"
            colorPalette="red"
            title="Blacklist this keyword: it and the keywords found only through it leave the graph, and a crawl never adds them again"
            onClick={() => onBlacklist(node.name)}
          >
            <BanIcon />
            Add to blacklist
          </Button>
        )}
      </HStack>
      {from.length > 0 && (
        <Stack gap="0.5">
          <Text fontWeight="semibold">Found from</Text>
          {from.map((e) => (
            <Text key={`${e.from}|${e.rel}`} color="fg.muted">
              from{' '}
              <Link as="button" color="fg" textAlign="start" onClick={() => onSelect(e.from)}>
                '{e.from}'
              </Link>{' '}
              ({e.rel === 'TOP' ? 'Top' : 'Rising'}, {value(e)})
            </Text>
          ))}
        </Stack>
      )}
      {found.length > 0 && (
        <Stack gap="0.5">
          <Text fontWeight="semibold">Found here ({found.length})</Text>
          {found.slice(0, MAX_LISTED).map((e) => (
            <Text key={`${e.to}|${e.rel}`} color="fg.muted">
              <Link as="button" color="fg" textAlign="start" onClick={() => onSelect(e.to)}>
                {e.to}
              </Link>{' '}
              ({e.rel === 'TOP' ? 'Top' : 'Rising'}, {value(e)})
            </Text>
          ))}
          {found.length > MAX_LISTED && <Text color="fg.muted">and {found.length - MAX_LISTED} more</Text>}
        </Stack>
      )}
    </Stack>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <HStack justify="space-between" gap="3">
      <Text color="fg.muted">{label}</Text>
      <Text textAlign="end">{value}</Text>
    </HStack>
  );
}
