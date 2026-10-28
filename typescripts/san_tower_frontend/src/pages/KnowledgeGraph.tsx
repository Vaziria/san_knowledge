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
import { ArrowLeftIcon, EyeIcon, RefreshCwIcon, ScanIcon, SearchIcon, SparklesIcon } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { Link as RouterLink, useParams, useSearchParams } from 'react-router';
import { errorText, knowledge } from '../api';
import type {
  GetKnowledgeNodeResponse,
  Knowledge,
  KnowledgeLink,
  KnowledgeNode,
  SyncKnowledgeResponse,
} from '../gen/san/tower/v1/knowledge_pb';
import { GraphCanvas, graphColors, nodeTypes, typeOf, useDark } from '../knowledge/GraphCanvas';
import { FormDialog, toast } from '../ui';

type Graph = { nodes: KnowledgeNode[]; links: KnowledgeLink[]; byKey: Map<string, KnowledgeNode> };

// KnowledgeGraph is a knowledge's page (/knowledge/<id>; the user,
// 2026-10-28): the graph of its folder's docs, as the knowledge tool's view
// shows one, the whole width of the page. Opening it brings the graph in
// line with the files (a sync, no AI), as the view does when it starts; Sync
// does it again. Summarize has Claude write the summaries that are missing,
// after asking. A click on a node marks it and its links (?node=<key>); a
// right-click opens its menu, whose View shows it in a dialog (the user,
// 2026-10-28, in place of a panel beside the graph): its summary, keywords,
// links and text. The search box marks the nodes it matches.
export function KnowledgeGraph() {
  const id = Number(useParams().id) || 0;
  const [params, setParams] = useSearchParams();
  const selected = params.get('node') ?? undefined;
  const [k, setK] = useState<Knowledge>();
  const [graph, setGraph] = useState<Graph>();
  const [version, setVersion] = useState(0);
  const [error, setError] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [query, setQuery] = useState('');
  const [fit, setFit] = useState(0);
  const [menu, setMenu] = useState<{ key: string; at: { x: number; y: number } }>();
  const [viewing, setViewing] = useState<string>();

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
    // Docs the AI step takes: those with a node waiting, or without a domain.
    const withDomain = new Set(graph?.links.filter((l) => l.rel === 'domain_of').map((l) => l.from));
    const docs = new Set(pending.map((n) => n.loc));
    for (const n of graph?.nodes ?? []) if (n.type === 'doc' && !withDomain.has(n.key)) docs.add(n.loc);
    return { domains: count('domain'), docs: count('doc'), sections: count('doc_section'), pending: pending.length, aiDocs: docs.size };
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
                colorPalette="blue"
                disabled={stats.aiDocs === 0}
                title={stats.aiDocs ? 'Have Claude write the missing summaries' : 'Every node has its summary'}
                onClick={() => setConfirming(true)}
              >
                <SparklesIcon />
                Summarize
              </Button>
            </HStack>
          </HStack>
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
                ? `${matches.size} found`
                : `${stats.docs} docs, ${stats.sections} sections, ${stats.domains} domains` +
                  (stats.pending ? `; ${stats.pending} waiting for a summary` : '')}
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
                  nodes={graph.nodes}
                  links={graph.links}
                  selected={selected}
                  matches={matches}
                  fit={fit}
                  onSelect={select}
                  onMenu={(key, at) => setMenu({ key, at })}
                />
                <IconButton
                  aria-label="Fit the graph in view"
                  title="Fit"
                  size="xs"
                  variant="outline"
                  bg="bg.panel"
                  position="absolute"
                  top="2"
                  right="2"
                  onClick={() => setFit((n) => n + 1)}
                >
                  <ScanIcon />
                </IconButton>
                <Legend />
              </>
            )}
          </Box>
          <Text fontSize="xs" color="fg.muted">
            Click a node to mark its links · right-click it for its menu · drag to pan · wheel to zoom
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
          onSelect={(e) => e.value === 'view' && view(menu.key)}
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
                </Menu.ItemGroup>
              </Menu.Content>
            </Menu.Positioner>
          </Portal>
        </Menu.Root>
      )}
      {viewing && graph?.byKey.has(viewing) && (
        <NodeDialog id={id} nodeKey={viewing} graph={graph} version={version} onView={view} onClose={() => setViewing(undefined)} />
      )}
      {confirming && (
        <FormDialog
          title="Write the summaries with Claude?"
          submitText="Summarize"
          onClose={() => setConfirming(false)}
          onSubmit={async () => {
            const r = await knowledge.summarizeKnowledge({ id });
            const failed = Object.keys(r.failed).length;
            toast(
              `Summarized ${r.summarized} nodes in ${r.docs.length - failed} docs` +
                (failed ? `; ${failed} failed (${Object.values(r.failed)[0]})` : ''),
              failed > 0,
            );
            await load();
          }}
        >
          <Text>
            {stats.pending} nodes in {stats.aiDocs} docs wait for a summary, keywords or a domain. Claude (claude -p, on
            this computer's Claude Code login) writes them, one call per doc, which can take a few minutes. A summary a
            person wrote, or a file's frontmatter gave, stays as it is.
          </Text>
        </FormDialog>
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
                <Text color="fg.muted">No summary yet: Summarize has Claude write one.</Text>
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
