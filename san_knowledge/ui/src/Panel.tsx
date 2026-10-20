import { Box, Button, chakra, CloseButton, Code, Flex, Grid, Heading, HStack, Link, Stack, Table, Text } from '@chakra-ui/react';
import { useEffect, useState, type ReactNode } from 'react';
import { api, errorText, q, type Explanation, type KNode, type Link as KLink, type NodeResponse } from './api';
import { expandable, communityLabel, communityOf, locOf, titleOf, typeInfo, typeSwatchColor } from './model';
import { Footnote, Mono, PendingBadge, SectionTitle, Swatch, toast, useApp } from './ui';

function ConnList({ children }: { children: ReactNode }) {
  return <chakra.ul listStyleType="none" m="0" p="0">{children}</chakra.ul>;
}

function NodeRow({ nodeKey, meta, onRemove }: { nodeKey: string; meta?: string; onRemove?: () => void }) {
  const { graph, colorOf, select } = useApp();
  const n: KNode = graph.byKey.get(nodeKey) ?? { key: nodeKey, title: nodeKey, node_type: '' };
  return (
    <Grid as="li" templateColumns="14px 1fr auto" gap="8px" alignItems="start" px="4px" py="6px" rounded="md"
      _hover={{ bg: 'bg.subtle' }} css={{ '&:hover .rm': { opacity: 1 } }}>
      <Box pt="4px"><Swatch type={n.node_type} color={colorOf(n)} /></Box>
      <Box minW="0">
        <chakra.button type="button" fontWeight="500" textAlign="start" cursor="pointer" overflowWrap="anywhere"
          _hover={{ textDecoration: 'underline' }} onClick={() => select(nodeKey, true)}>
          {n.title || nodeKey}
        </chakra.button>
        {meta && <Text fontSize="12px" color="fg.subtle" overflowWrap="anywhere">{meta}</Text>}
      </Box>
      {onRemove
        ? <CloseButton className="rm" size="2xs" opacity={0} _focusVisible={{ opacity: 1 }} title="Remove" aria-label={`Remove ${n.title || nodeKey}`} onClick={onRemove} />
        : <span />}
    </Grid>
  );
}

export function Overview() {
  const { graph, ai } = useApp();
  const pending = graph.nodes.filter((n) => n.needs_summary);
  const docs = graph.nodes.filter((n) => n.node_type === 'doc').sort((a, b) => a.key.localeCompare(b.key));
  const noDomain = docs.filter((d) => !(graph.domainsOf.get(d.key) ?? []).length);
  return (
    <>
      <Heading as="h2" fontSize="17px" mt="6px" mb="2px">Explore the graph</Heading>
      <Text color="fg.muted">The graph starts with domains and the docs that have none. Click a node for details and to show what is under it. Explain searches docs, sections and domains and highlights the context around a question.</Text>
      {pending.length > 0 && (
        <Text color="fg.muted" mt="8px">
          <PendingBadge>{pending.length} need a summary</PendingBadge>{' '}
          {ai ? 'Use “Summarize with AI”, or ' : ''}run <Code size="sm">knowledge sync</Code>.
        </Text>
      )}
      <SectionTitle>Domains ({graph.domains.length})</SectionTitle>
      {graph.domains.length
        ? <ConnList>{graph.domains.map((d) => <NodeRow key={d.key} nodeKey={d.key} meta={d.summary} />)}</ConnList>
        : <Footnote mt="0">No domains yet. Add one, or let the AI assign them.</Footnote>}
      <SectionTitle>Docs ({docs.length})</SectionTitle>
      <ConnList>
        {docs.map((d) => (
          <NodeRow key={d.key} nodeKey={d.key}
            meta={[d.loc, (graph.domainsOf.get(d.key) ?? []).map((k) => titleOf(graph, k)).join(', ')].filter(Boolean).join(' · ')} />
        ))}
      </ConnList>
      {noDomain.length > 0 && <Footnote>{noDomain.length} doc(s) without a domain.</Footnote>}
    </>
  );
}

// Web sources: the page address (http/https only) and when it was fetched.
function SourceLine({ n }: { n: KNode }) {
  const safe = /^https?:\/\//i.test(n.uri ?? '');
  const when = n.last_fetched ? new Date(n.last_fetched) : null;
  return (
    <Stack gap="2px" mt="4px" fontSize="12px">
      {safe ? <Link href={n.uri} target="_blank" rel="noopener noreferrer" color="accent.fg" wordBreak="break-all">{n.uri}</Link> : <Text>{n.uri}</Text>}
      {when && !isNaN(when.getTime()) && <Text color="fg.subtle">fetched {when.toLocaleString()}</Text>}
    </Stack>
  );
}

export function NodeDetail({ nodeKey }: { nodeKey: string }) {
  const app = useApp();
  const { graph, colorOf, select, openDialog, reload, version, expanded, toggleExpand } = app;
  const [data, setData] = useState<NodeResponse | null>(null);

  useEffect(() => {
    let live = true;
    api<NodeResponse>('GET', `/api/node?key=${q(nodeKey)}`).then(
      (d) => { if (live) setData(d); },
      (err) => { if (live) { toast(errorText(err), true); select(null); } },
    );
    return () => { live = false; };
  }, [nodeKey, version, select]);

  if (!data || data.node.key !== nodeKey) return null;
  const n = data.node, out = data.out ?? [], inc = data.in ?? [];
  const isDomain = n.node_type === 'domain';
  const domainLinks = out.filter((l) => l.rel === 'domain_of');
  const parent = out.find((l) => l.rel === 'section_of');
  const lineOf = (k: string) => graph.byKey.get(k)?.line_loc ?? 0;
  const children = inc.filter((l) => l.rel === 'section_of').sort((a, b) => lineOf(a.from) - lineOf(b.from));
  const members = inc.filter((l) => l.rel === 'domain_of');
  const refsOut = out.filter((l) => l.rel === 'reference');
  const refsIn = inc.filter((l) => l.rel === 'reference');
  const refMeta = (k: string) => { const x = graph.byKey.get(k); return x ? [locOf(x), x.summary].filter(Boolean).join(' · ') : ''; };
  const props = Object.entries(n.props ?? {}).sort(([a], [b]) => a.localeCompare(b));
  const keywords = n.keyword ?? [];
  const inherited = n.node_type === 'doc_section' ? (graph.domainsOf.get(n.loc ?? '') ?? []) : [];

  const unassign = (nodeKey: string, domainKey: string) => async () => {
    try {
      await api('DELETE', `/api/assign?key=${q(nodeKey)}&domain=${q(domainKey)}`);
      toast('Domain removed');
      await reload();
    } catch (err) { toast(errorText(err), true); }
  };
  const list = (links: KLink[], end: 'from' | 'to', meta: (k: string) => string, remove?: (l: KLink) => () => void) => (
    <ConnList>{links.map((l) => <NodeRow key={l[end]} nodeKey={l[end]} meta={meta(l[end])} onRemove={remove?.(l)} />)}</ConnList>
  );

  return (
    <>
      <HStack gap="6px" wrap="wrap">
        <HStack gap="6px" color="fg.muted" fontSize="12px"><Swatch type={n.node_type} color={colorOf(n)} />{typeInfo(n.node_type).label}</HStack>
        {communityOf(graph, n.key) >= 0 && <Text fontSize="12px" px="8px" py="2px" rounded="full" bg="bg.subtle">{communityLabel(communityOf(graph, n.key))}</Text>}
        {n.needs_summary && <PendingBadge>needs summary</PendingBadge>}
      </HStack>
      <Heading as="h2" fontSize="17px" mt="6px" mb="2px" lineHeight="1.3" overflowWrap="anywhere">{n.title || n.key}</Heading>
      <Mono>{n.loc ? locOf(n) : n.key}</Mono>
      {n.uri && <SourceLine n={n} />}
      {n.summary
        ? <Text mt="10px">{n.summary}</Text>
        : <Text mt="10px" color="fg.subtle" fontStyle="italic">{isDomain ? 'No summary yet.' : 'No summary yet. Run “Summarize with AI” or write one.'}</Text>}
      <HStack gap="6px" mt="12px" wrap="wrap">
        <Button size="xs" variant="outline" onClick={() => openDialog({ kind: 'edit', node: n })}>Edit</Button>
        {!isDomain && <Button size="xs" variant="outline" onClick={() => openDialog({ kind: 'assign', key: n.key })}>Assign domain</Button>}
        {expandable(graph, n.key) && (
          <Button size="xs" variant="outline" title="Hide or show what is under it in the graph (clicking the node shows it)" onClick={() => toggleExpand(n.key)}>
            {expanded.has(n.key) ? 'Collapse' : 'Expand'} {isDomain ? 'docs' : 'sections'}
          </Button>
        )}
        {isDomain && <Button size="xs" variant="ghost" colorPalette="red" onClick={() => openDialog({ kind: 'delete', node: n, members: members.length })}>Delete</Button>}
      </HStack>

      {keywords.length > 0 && (
        <>
          <SectionTitle>Keywords</SectionTitle>
          <Flex wrap="wrap" gap="4px">
            {keywords.map((k) => <KeywordTag key={k} keyword={k} />)}
          </Flex>
        </>
      )}

      {!isDomain && (
        <>
          <SectionTitle>Domain</SectionTitle>
          {domainLinks.length ? (
            <Flex wrap="wrap" gap="6px">
              {domainLinks.map((l) => (
                <HStack key={l.to} gap="6px" fontSize="12px" pl="8px" pr="2px" py="2px" rounded="full" bg="bg.subtle">
                  <Swatch type="domain" color={typeSwatchColor(app.pal, app.colorBy, 'domain')} size={10} />
                  <chakra.button type="button" cursor="pointer" _hover={{ textDecoration: 'underline' }} onClick={() => select(l.to, true)}>{titleOf(graph, l.to)}</chakra.button>
                  <CloseButton size="2xs" rounded="full" title="Remove domain" aria-label={`Remove domain ${titleOf(graph, l.to)}`} onClick={unassign(n.key, l.to)} />
                </HStack>
              ))}
            </Flex>
          ) : (
            <Footnote mt="0">{inherited.length ? `Inherits from its doc: ${inherited.map((k) => titleOf(graph, k)).join(', ')}` : 'No domain.'}</Footnote>
          )}
        </>
      )}

      {parent && <><SectionTitle>Part of</SectionTitle>{list([parent], 'to', (k) => typeInfo(graph.byKey.get(k)?.node_type ?? '').label)}</>}
      {children.length > 0 && (
        <>
          <SectionTitle>Sections ({children.length})</SectionTitle>
          {list(children, 'from', (k) => {
            const c = graph.byKey.get(k);
            return [c?.line_loc ? `line ${c.line_loc}` : '', c?.needs_summary ? 'needs summary' : c?.summary].filter(Boolean).join(' · ');
          })}
        </>
      )}
      {refsOut.length > 0 && <><SectionTitle>References ({refsOut.length})</SectionTitle>{list(refsOut, 'to', refMeta)}</>}
      {refsIn.length > 0 && <><SectionTitle>Referenced by ({refsIn.length})</SectionTitle>{list(refsIn, 'from', refMeta)}</>}
      {members.length > 0 && (
        <>
          <SectionTitle>Docs and sections ({members.length})</SectionTitle>
          {list(members, 'from', (k) => locOf(graph.byKey.get(k) ?? {}), (l) => unassign(l.from, n.key))}
        </>
      )}

      {props.length > 0 && (
        <>
          <SectionTitle>Properties</SectionTitle>
          <Table.Root size="sm" variant="line" fontSize="12px">
            <Table.Body>
              {props.map(([k, v]) => (
                <Table.Row key={k} bg="transparent">
                  <Table.Cell px="0" py="3px" color="fg.subtle" width="40%" verticalAlign="top">{k.replace(/_/g, ' ')}</Table.Cell>
                  <Table.Cell px="0" py="3px" fontVariantNumeric="tabular-nums" wordBreak="break-word">{typeof v === 'object' ? JSON.stringify(v) : String(v)}</Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </>
      )}

      {data.text && (
        <chakra.details open={!n.summary}>
          <chakra.summary cursor="pointer" color="fg.subtle" fontSize="11px" textTransform="uppercase" letterSpacing="0.06em" fontWeight="600" mt="18px" mb="6px">
            {n.node_type === 'doc' ? 'Document text' : 'Section text'}
          </chakra.summary>
          <chakra.pre whiteSpace="pre-wrap" fontFamily="mono" fontSize="12px" lineHeight="1.5" bg="bg.subtle" color="fg.muted" rounded="lg" px="12px" py="10px" maxH="320px" overflow="auto" m="0">
            {data.text}
          </chakra.pre>
        </chakra.details>
      )}
      <Footnote wordBreak="break-all">key {n.key} · by {n.author || 'unknown'} · updated {(n.updated ?? '').replace('T', ' ').slice(0, 16)}</Footnote>
    </>
  );
}

function KeywordTag({ keyword }: { keyword: string }) {
  const { filterKeyword } = useApp();
  return (
    <chakra.button type="button" fontSize="12px" px="8px" py="1px" rounded="full" bg="bg.subtle" color="fg.muted" cursor="pointer"
      _hover={{ color: 'fg' }} title={`Show only “${keyword}”`} onClick={() => filterKeyword(keyword)}>
      {keyword}
    </chakra.button>
  );
}

export function ExplainPanel({ explanation, markdown, onClear }: { explanation: Explanation; markdown: string; onClear: () => void }) {
  const { colorOf, select } = useApp();
  const { matches } = explanation;
  const copy = async () => {
    try { await navigator.clipboard.writeText(markdown); toast('Copied context'); } catch { toast('Clipboard not available', true); }
  };
  return (
    <>
      <SectionTitle>Explain</SectionTitle>
      <Heading as="h2" fontSize="17px" mt="6px" mb="2px">{explanation.query}</Heading>
      <HStack gap="6px" mt="12px">
        <Button size="xs" variant="outline" onClick={copy}>Copy as markdown</Button>
        <Button size="xs" variant="ghost" onClick={onClear}>Clear</Button>
      </HStack>
      {matches.length === 0 && <Footnote>No matching knowledge. Try other words.</Footnote>}
      <SectionTitle>{matches.length} match{matches.length === 1 ? '' : 'es'}</SectionTitle>
      <Stack gap="0">
        {matches.map((m) => (
          <Box key={m.node.key} p="8px" rounded="lg" cursor="pointer" tabIndex={0} _hover={{ bg: 'bg.subtle' }} _focusVisible={{ bg: 'bg.subtle', outline: 'none' }}
            onClick={() => select(m.node.key, true)} onKeyDown={(ev) => { if (ev.key === 'Enter') select(m.node.key, true); }}>
            <HStack gap="6px" fontWeight="600"><Swatch type={m.node.node_type} color={colorOf(m.node)} />{m.node.title || m.node.key}</HStack>
            {m.node.loc && <Mono>{locOf(m.node)}</Mono>}
            {m.node.summary
              ? <Text color="fg.muted" fontSize="13px" mt="2px">{m.node.summary}</Text>
              : m.excerpt ? <Text color="fg.muted" fontSize="13px" mt="2px"><em>No summary yet: </em>{m.excerpt.slice(0, 220)}</Text> : null}
            {m.domains.length > 0 && <Text color="fg.subtle" fontSize="12px" mt="4px">Domain: {m.domains.join(', ')}</Text>}
          </Box>
        ))}
      </Stack>
    </>
  );
}
