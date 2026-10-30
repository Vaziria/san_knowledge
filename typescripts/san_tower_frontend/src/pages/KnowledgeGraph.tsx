import {
  Badge,
  Box,
  Button,
  CloseButton,
  Dialog,
  HStack,
  Heading,
  IconButton,
  Input,
  InputGroup,
  Link,
  Menu,
  Portal,
  Stack,
  Text,
  Wrap,
} from '@chakra-ui/react';
import {
  ArrowLeftIcon,
  EyeIcon,
  FoldVerticalIcon,
  RefreshCwIcon,
  ScanIcon,
  SearchIcon,
  ShrinkIcon,
  UnfoldVerticalIcon,
  WorkflowIcon,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { Link as RouterLink, useNavigate, useParams, useSearchParams } from 'react-router';
import { errorText, knowledge, workflows } from '../api';
import type {
  GetKnowledgeNodeResponse,
  Knowledge,
  KnowledgeLink,
  KnowledgeNode,
  SyncKnowledgeResponse,
} from '../gen/san/tower/v1/knowledge_pb';
import { BATCH, CORE, buildNet, pruned, shownNodes } from '../knowledge/fold';
import { GraphCanvas, graphColors, nodeTypes, typeOf, useDark } from '../knowledge/GraphCanvas';
import { createSyncWorkflow } from '../knowledge/syncSample';
import { toast } from '../ui';

type Graph = { nodes: KnowledgeNode[]; links: KnowledgeLink[]; byKey: Map<string, KnowledgeNode> };

// MAX_MATCHES_SHOWN is how many of a search's matches are shown with the
// path to them: a common word matches hundreds.
const MAX_MATCHES_SHOWN = 30;

// KnowledgeGraph is a knowledge's page (/knowledge/<id>; the user,
// 2026-10-28): the graph of its folder's docs, as the knowledge tool's view
// shows one, the whole width of the page. Opening it brings the graph in
// line with the files (a sync, no AI), as the view does when it starts; Sync
// does it again. The summaries that are missing are a workflow's to write
// (Knowledge Summarize; the user, 2026-10-29, dropped the page's Summarize
// button, a weaker copy of the step): the page names the workflows that do
// it for this knowledge. A click on a node marks it and its links (?node=<key>); a
// right-click opens its menu, whose View shows it in a dialog (the user,
// 2026-10-28, in place of a panel beside the graph): its summary, keywords,
// links and text. The search box marks the nodes it matches.
//
// The graph starts with its core, the most-connected nodes
// (knowledge/fold.ts; the user, 2026-10-29: a knowledge of 2,435 nodes left
// the page blank and lagging, drawing and laying out every node at once,
// then "load with biggest connection, and when right click, we can expand
// or collapse"). A node's menu Expands it, its neighbours, the
// most-connected first, a batch at a time (Expand more), and Collapses it
// again; Collapse all goes back to the core. The selected node, and the
// first matches of a search, are shown with the path to them.
export function KnowledgeGraph() {
  const id = Number(useParams().id) || 0;
  const [params, setParams] = useSearchParams();
  const selected = params.get('node') ?? undefined;
  const [k, setK] = useState<Knowledge>();
  const [graph, setGraph] = useState<Graph>();
  const [version, setVersion] = useState(0);
  const [error, setError] = useState('');
  const [syncing, setSyncing] = useState(false);
  // The workflows whose Knowledge Summarize writes this knowledge's
  // summaries (Knowledge Base picks it, linked to the step's Knowledge).
  const [summarizers, setSummarizers] = useState<{ id: number; name: string }[]>();
  const [creating, setCreating] = useState(false);
  const navigate = useNavigate();
  // Sync workflow makes the sample sync workflow for this knowledge and
  // opens it in the editor.
  const createSync = async () => {
    if (!k) return;
    setCreating(true);
    try {
      const w = await createSyncWorkflow(id, k.name);
      toast(`Created workflow ${w.name}: press Run to sync`);
      navigate(`/workflow/${w.id}`);
    } catch (err) {
      toast(`Not created: ${errorText(err)}`, true);
      setCreating(false);
    }
  };
  const [query, setQuery] = useState('');
  const [fit, setFit] = useState(0);
  const [menu, setMenu] = useState<{ key: string; at: { x: number; y: number } }>();
  const [viewing, setViewing] = useState<string>();
  const [opened, setOpened] = useState<Map<string, number>>(new Map());

  const select = (key?: string) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p);
        if (key) next.set('node', key);
        else next.delete('node');
        return next;
      },
      { replace: true },
    );
  const view = (key: string) => {
    select(key);
    setViewing(key);
  };

  const load = async () => {
    const g = await knowledge.getKnowledgeGraph({ id });
    setGraph({ nodes: g.nodes, links: g.links, byKey: new Map(g.nodes.map((n) => [n.key, n])) });
    setVersion((v) => v + 1);
  };

  useEffect(() => {
    let stale = false;
    setGraph(undefined);
    setOpened(new Map());
    (async () => {
      try {
        const { knowledge: got } = await knowledge.getKnowledge({ id });
        if (stale) return;
        setK(got);
        await knowledge.syncKnowledge({ id });
        if (!stale) await load();
        if (!stale) setError('');
      } catch (err) {
        if (!stale) setError(errorText(err));
      }
    })();
    return () => {
      stale = true;
    };
  }, [id]);

  useEffect(() => {
    let stale = false;
    (async () => {
      try {
        const { workflows: list } = await workflows.listWorkflows({ allFolders: true, limit: 500 });
        const found = [];
        for (const w of list) {
          const { workflow } = await workflows.getWorkflow({ id: w.id });
          const steps = new Map((workflow?.steps ?? []).map((st) => [st.id, st]));
          const writes = (workflow?.links ?? []).some(
            (l) =>
              l.targetSocket === 'Knowledge' &&
              steps.get(l.target)?.kind === 'knowledge-summarize' &&
              steps.get(l.source)?.kind === 'knowledge-base' &&
              steps.get(l.source)?.values.Knowledge === String(id),
          );
          if (writes) found.push({ id: w.id, name: w.name });
        }
        if (!stale) setSummarizers(found);
      } catch {
        if (!stale) setSummarizers([]);
      }
    })();
    return () => {
      stale = true;
    };
  }, [id]);

  const sync = async () => {
    setSyncing(true);
    try {
      toast(syncText(await knowledge.syncKnowledge({ id })));
      await load();
    } catch (err) {
      toast(`Not synced: ${errorText(err)}`, true);
    }
    setSyncing(false);
  };

  const stats = useMemo(() => {
    const count = (t: string) => graph?.nodes.filter((n) => n.type === t).length ?? 0;
    const pending = graph?.nodes.filter((n) => n.needsSummary) ?? [];
    return { domains: count('domain'), docs: count('doc'), sections: count('doc_section'), pending: pending.length };
  }, [graph]);

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q || !graph) return undefined;
    return new Set(
      graph.nodes
        .filter((n) => [n.title, n.summary, n.key, ...n.keywords].some((s) => s.toLowerCase().includes(q)))
        .map((n) => n.key),
    );
  }, [query, graph]);

  // What is shown: the core, what is opened, and the selected node and the
  // first matches with the paths to them.
  const net = useMemo(() => graph && buildNet(graph.nodes, graph.links), [graph]);
  const shown = useMemo(() => {
    if (!graph || !net) return undefined;
    const pinned = [...(selected ? [selected] : []), ...[...(matches ?? [])].slice(0, MAX_MATCHES_SHOWN)];
    const s = shownNodes(net, opened, pinned.filter((k) => graph.byKey.has(k)));
    return {
      ...s,
      nodes: graph.nodes.filter((n) => s.keys.has(n.key)),
      links: graph.links.filter((l) => s.keys.has(l.from) && s.keys.has(l.to)),
    };
  }, [graph, net, opened, selected, matches]);
  // Expand shows a batch more of a node's neighbours; Collapse hides what it
  // opened, unless something else shows it too.
  const expand = (key: string) => {
    if (!net || !shown) return;
    // Far enough down its neighbours for BATCH that are not shown yet:
    // some are shown already, by the core or another node.
    const list = net.neighbours.get(key) ?? [];
    let n = opened.get(key) ?? 0;
    for (let added = 0; n < list.length && added < BATCH; n++) if (!shown.keys.has(list[n])) added++;
    setOpened((o) => new Map(o).set(key, n));
  };
  const collapse = (key: string) => {
    if (!net || !graph) return;
    const next = new Map(opened);
    next.delete(key);
    const pinned = selected && graph.byKey.has(selected) ? [selected] : [];
    setOpened(pruned(next, shownNodes(net, next, pinned).keys));
  };

  const back = k?.folderId ? `/knowledge?folder=${k.folderId}` : '/knowledge';
  return (
    <Stack gap="3" h="calc(100vh - 3rem)">
      <Button asChild size="sm" variant="ghost" color="fg.muted" alignSelf="flex-start" ms="-3">
        <RouterLink to={back}>
          <ArrowLeftIcon />
          Knowledge
        </RouterLink>
      </Button>
      {error ? (
        <Text color="fg.error">{error}</Text>
      ) : !k || !graph ? (
        <Text color="fg.muted">{k ? 'Reading its files...' : 'Loading...'}</Text>
      ) : (
        <>
          <HStack justify="space-between" align="flex-start" wrap="wrap" gap="2">
            <Stack gap="0" minW="0">
              <Heading size="lg">{k.name}</Heading>
              <Text color="fg.muted" textStyle="xs" fontFamily="mono" wordBreak="break-all">
                {k.storage?.absolutePath}
              </Text>
            </Stack>
            <HStack gap="2">
              <Button size="sm" variant="outline" loading={syncing} onClick={() => void sync()} title="Read the files again">
                <RefreshCwIcon />
                Sync
              </Button>
              <Button
                size="sm"
                variant="outline"
                loading={creating}
                onClick={() => void createSync()}
                title="Create a sample workflow that syncs this knowledge and writes its summaries, and open it"
              >
                <WorkflowIcon />
                Sync workflow
              </Button>
            </HStack>
          </HStack>
          {stats.pending > 0 && summarizers && (
            <Text textStyle="sm" color="fg.muted">
              {stats.pending} nodes wait for a summary.{' '}
              {summarizers.length > 0 ? (
                <>
                  A workflow writes them:{' '}
                  {summarizers.map((w, i) => (
                    <span key={w.id}>
                      {i > 0 && ', '}
                      <Link asChild color="fg">
                        <RouterLink to={`/workflow/${w.id}`}>{w.name}</RouterLink>
                      </Link>
                    </span>
                  ))}
                  .
                </>
              ) : (
                <>
                  A workflow with a Knowledge Summarize step writes them: Sync workflow makes one.
                </>
              )}
            </Text>
          )}
          <HStack gap="3" wrap="wrap">
            <InputGroup maxW="xs" startElement={<SearchIcon size="16" />}>
              <Input
                size="sm"
                value={query}
                onChange={(e) => setQuery(e.currentTarget.value)}
                onKeyDown={(e) => {
                  const first = matches?.values().next().value;
                  if (e.key === 'Enter' && first) select(first);
                }}
                placeholder="Find a node by title, keyword or summary"
              />
            </InputGroup>
            <Text textStyle="sm" color="fg.muted">
              {matches
                ? `${matches.size} found` + (matches.size > MAX_MATCHES_SHOWN ? `, the first ${MAX_MATCHES_SHOWN} shown` : '')
                : `${stats.docs} docs, ${stats.sections} sections, ${stats.domains} domains` +
                  (shown && shown.nodes.length < graph.nodes.length ? ` · ${shown.nodes.length} shown` : '')}
            </Text>
          </HStack>
          <Box flex="1" minH="80" position="relative" borderWidth="1px" rounded="md" bg="bg.panel" overflow="hidden">
            {graph.nodes.length === 0 ? (
              <Text color="fg.muted" p="4">
                No markdown files in its folder yet: add .md files there, then Sync.
              </Text>
            ) : (
              <>
                <GraphCanvas
                  nodes={shown?.nodes ?? []}
                  links={shown?.links ?? []}
                  selected={selected}
                  matches={matches}
                  fit={fit}
                  folded={shown?.hidden}
                  anchors={shown?.anchors}
                  onSelect={select}
                  onMenu={(key, at) => setMenu({ key, at })}
                />
                <HStack position="absolute" top="2" right="2" gap="1">
                  <IconButton
                    aria-label="Collapse all"
                    title={`Collapse all: back to the ${CORE} most-connected nodes`}
                    size="xs"
                    variant="outline"
                    bg="bg.panel"
                    onClick={() => {
                      setOpened(new Map());
                      select(undefined);
                      setQuery('');
                    }}
                  >
                    <ShrinkIcon />
                  </IconButton>
                  <IconButton
                    aria-label="Fit the graph in view"
                    title="Fit"
                    size="xs"
                    variant="outline"
                    bg="bg.panel"
                    onClick={() => setFit((n) => n + 1)}
                  >
                    <ScanIcon />
                  </IconButton>
                </HStack>
                <Legend />
              </>
            )}
          </Box>
          <Text fontSize="xs" color="fg.muted">
            The most-connected nodes first · click a node to mark its links · right-click it to expand (+N) or collapse it · drag to pan · wheel to zoom
          </Text>
        </>
      )}
      {/* The pointer menu's trigger is an empty box at the pointer, as the
          workflow editor's: a menu opened from code ignores its anchorPoint
          prop. A new key per opening places it afresh. */}
      {menu && graph && (
        <Menu.Root
          key={`${menu.at.x},${menu.at.y}`}
          open
          positioning={{ placement: 'bottom-start', gutter: 2 }}
          onOpenChange={(e) => !e.open && setMenu(undefined)}
          onSelect={(e) => {
            if (e.value === 'view') view(menu.key);
            if (e.value === 'expand') expand(menu.key);
            if (e.value === 'collapse') collapse(menu.key);
          }}
        >
          <Menu.Trigger asChild>
            <Box position="fixed" left={`${menu.at.x}px`} top={`${menu.at.y}px`} w="0" h="0" pointerEvents="none" />
          </Menu.Trigger>
          <Portal>
            <Menu.Positioner>
              <Menu.Content minW="44">
                <Menu.ItemGroup>
                  <Menu.ItemGroupLabel maxW="64" truncate>
                    {graph.byKey.get(menu.key)?.title || menu.key}
                  </Menu.ItemGroupLabel>
                  <Menu.Item value="view">
                    <EyeIcon />
                    View
                  </Menu.Item>
                  {(shown?.hidden.get(menu.key) ?? 0) > 0 && (
                    <Menu.Item value="expand">
                      <UnfoldVerticalIcon />
                      {`${opened.has(menu.key) ? 'Expand more' : 'Expand'} (${Math.min(BATCH, shown?.hidden.get(menu.key) ?? 0)} of ${shown?.hidden.get(menu.key)})`}
                    </Menu.Item>
                  )}
                  {opened.has(menu.key) && (
                    <Menu.Item value="collapse">
                      <FoldVerticalIcon />
                      Collapse
                    </Menu.Item>
                  )}
                </Menu.ItemGroup>
              </Menu.Content>
            </Menu.Positioner>
          </Portal>
        </Menu.Root>
      )}
      {viewing && graph?.byKey.has(viewing) && (
        <NodeDialog id={id} nodeKey={viewing} graph={graph} version={version} onView={view} onClose={() => setViewing(undefined)} />
      )}
    </Stack>
  );
}

function syncText(r: SyncKnowledgeResponse) {
  const parts = [
    [r.docsAdded, 'docs added'],
    [r.docsUpdated, 'docs changed'],
    [r.docsRemoved, 'docs removed'],
    [r.sectionsAdded, 'sections added'],
    [r.sectionsUpdated, 'sections changed'],
    [r.sectionsRemoved, 'sections removed'],
  ]
    .filter(([n]) => n)
    .map(([n, what]) => `${n} ${what}`);
  return parts.length ? `Synced: ${parts.join(', ')}` : 'Already in line with the files';
}

// Swatch is a node type's colour and shape, small.
function Swatch({ type, pending }: { type: string; pending?: boolean }) {
  const dark = useDark();
  const shape = typeOf(type).shape;
  return (
    <Box
      w="3"
      h="3"
      flexShrink="0"
      bg={pending ? 'transparent' : graphColors(dark)[type as keyof ReturnType<typeof graphColors>]}
      borderWidth={pending ? '2px' : '0'}
      borderStyle="dashed"
      borderColor="fg.muted"
      rounded={shape === 'ellipse' ? 'full' : shape === 'round-rectangle' ? 'xs' : '0'}
      clipPath={shape === 'round-hexagon' ? 'polygon(25% 0, 75% 0, 100% 50%, 75% 100%, 25% 100%, 0 50%)' : undefined}
    />
  );
}

function Legend() {
  return (
    <HStack
      position="absolute"
      left="2"
      bottom="2"
      gap="3"
      px="2"
      py="1"
      rounded="md"
      bg="bg.panel"
      borderWidth="1px"
      textStyle="xs"
      color="fg.muted"
      wrap="wrap"
    >
      {nodeTypes.map((t) => (
        <HStack key={t.type} gap="1">
          <Swatch type={t.type} />
          {t.label}
        </HStack>
      ))}
      <HStack gap="1">
        <Swatch type="doc_section" pending />
        Waiting for a summary
      </HStack>
    </HStack>
  );
}

// NodeLink is a link to a node, which views it.
function NodeLink({ graph, nodeKey, onView }: { graph: Graph; nodeKey: string; onView: (key: string) => void }) {
  const n = graph.byKey.get(nodeKey);
  return (
    <HStack gap="1.5" align="center">
      <Swatch type={n?.type ?? 'doc_section'} />
      <Link as="button" textAlign="start" colorPalette="blue" onClick={() => onView(nodeKey)}>
        {n?.title || nodeKey}
      </Link>
    </HStack>
  );
}

// The dialog's groups of links, by relation and direction.
const LINK_GROUPS: { title: string; dir: 'out' | 'in'; rel: string }[] = [
  { title: 'Domain', dir: 'out', rel: 'domain_of' },
  { title: 'Part of', dir: 'out', rel: 'section_of' },
  { title: 'Sections', dir: 'in', rel: 'section_of' },
  { title: 'Links to', dir: 'out', rel: 'reference' },
  { title: 'Linked from', dir: 'in', rel: 'reference' },
  { title: 'In this domain', dir: 'in', rel: 'domain_of' },
];

const AUTHORS: Record<string, string> = { ai: 'Written by Claude', frontmatter: "From the file's frontmatter" };

// NodeDialog views a node: its type, title, file and line, summary (and who
// wrote it), keywords, its links grouped (each views that node in the
// dialog, and marks it on the graph) and its text in the file. It loads the
// node again when the graph reloads (version), since a sync or a summary may
// have changed it.
function NodeDialog({
  id,
  nodeKey,
  graph,
  version,
  onView,
  onClose,
}: {
  id: number;
  nodeKey: string;
  graph: Graph;
  version: number;
  onView: (key: string) => void;
  onClose: () => void;
}) {
  const [got, setGot] = useState<GetKnowledgeNodeResponse>();
  const [error, setError] = useState('');
  useEffect(() => {
    let stale = false;
    knowledge.getKnowledgeNode({ id, key: nodeKey }).then(
      (r) => {
        if (stale) return;
        setGot(r);
        setError('');
      },
      (err: unknown) => !stale && setError(errorText(err)),
    );
    return () => {
      stale = true;
    };
  }, [id, nodeKey, version]);

  const n = got?.node?.key === nodeKey ? got.node : graph.byKey.get(nodeKey)!;
  const links = got?.node?.key === nodeKey ? got : undefined;
  return (
    <Dialog.Root open size="lg" placement="center" scrollBehavior="inside" onOpenChange={(e) => !e.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content mx="4">
            <Dialog.Header>
              <Stack gap="1" minW="0" pe="8">
                <HStack gap="2" textStyle="xs" color="fg.muted">
                  <Swatch type={n.type} />
                  {typeOf(n.type).label}
                  {n.needsSummary && (
                    <Badge size="sm" colorPalette="orange">
                      Waiting for a summary
                    </Badge>
                  )}
                </HStack>
                <Dialog.Title>{n.title || n.key}</Dialog.Title>
                {n.loc && (
                  <Text textStyle="xs" color="fg.muted" fontFamily="mono" wordBreak="break-all">
                    {n.line ? `${n.loc}:${n.line}` : n.loc}
                  </Text>
                )}
              </Stack>
            </Dialog.Header>
            <Dialog.CloseTrigger asChild>
              <CloseButton size="sm" />
            </Dialog.CloseTrigger>
            <Dialog.Body display="flex" flexDirection="column" gap="4">
              {n.summary ? (
                <Stack gap="1">
                  <Text>{n.summary}</Text>
                  {AUTHORS[n.author] && (
                    <Text textStyle="xs" color="fg.muted">
                      {AUTHORS[n.author]}
                    </Text>
                  )}
                </Stack>
              ) : (
                <Text color="fg.muted">No summary yet: a workflow’s Knowledge Summarize writes one.</Text>
              )}
              {n.keywords.length > 0 && (
                <Wrap gap="1">
                  {n.keywords.map((w) => (
                    <Badge key={w} variant="subtle">
                      {w}
                    </Badge>
                  ))}
                </Wrap>
              )}
              {error && <Text color="fg.error">{error}</Text>}
              {links &&
                LINK_GROUPS.map((g) => {
                  const keys = (g.dir === 'out' ? links.out : links.in)
                    .filter((l) => l.rel === g.rel)
                    .map((l) => (g.dir === 'out' ? l.to : l.from));
                  if (keys.length === 0) return null;
                  return (
                    <Stack key={`${g.dir}${g.rel}`} gap="1">
                      <Text textStyle="xs" fontWeight="semibold" color="fg.muted" textTransform="uppercase">
                        {g.title}
                      </Text>
                      {keys.map((key) => (
                        <NodeLink key={key} graph={graph} nodeKey={key} onView={onView} />
                      ))}
                    </Stack>
                  );
                })}
              {links?.text && (
                <Stack gap="1">
                  <Text textStyle="xs" fontWeight="semibold" color="fg.muted" textTransform="uppercase">
                    Text
                  </Text>
                  <Box
                    as="pre"
                    whiteSpace="pre-wrap"
                    wordBreak="break-word"
                    textStyle="xs"
                    fontFamily="mono"
                    bg="bg.subtle"
                    p="3"
                    rounded="md"
                  >
                    {links.text}
                  </Box>
                </Stack>
              )}
            </Dialog.Body>
            <Dialog.Footer>
              <Button onClick={onClose}>Close</Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
