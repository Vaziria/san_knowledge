import { Box, Button, Flex, Grid, HStack, Input, NativeSelect, Stack, Text } from '@chakra-ui/react';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import {
  brainClaims, brainRecall, brainRuns, brains as fetchBrains, errorText,
  type BrainInfo, type ClaimRow, type RecallFact, type RecallNode, type RunRow, type SourceRow,
} from './api';
import { Mono, SectionTitle, Segmented, useApp } from './ui';

// The Brain view (docs/brain.md; kb/brainview.go): a brain's claims with what
// they rest on and how they worked out, its latest runs, and what it recalls
// for a task. Read-only: a brain changes by learning, runs and sleep.

const PAGE = 50;

type Status = '' | 'proposed' | 'verified' | 'confirmed' | 'retired';
type Sort = 'weight' | 'uses' | 'new';

const pct = (x: number) => `${Math.round(x * 100)}%`;

function Chip({ children, tone = 'muted', title }: { children: ReactNode; tone?: 'muted' | 'accent' | 'ok' | 'bad'; title?: string }) {
  const color = { muted: 'fg.muted', accent: 'accent.fg', ok: 'green.fg', bad: 'red.fg' }[tone];
  return (
    <Text as="span" display="inline-flex" whiteSpace="nowrap" fontSize="11px" fontWeight="600" px="7px" py="1px" rounded="full"
      borderWidth="1px" borderColor="border" color={color} title={title}>
      {children}
    </Text>
  );
}

// A source of the knowledge graph: its file:line, which selects its node.
function SourceLink({ s, against }: { s: SourceRow; against?: boolean }) {
  const { select } = useApp();
  return (
    <Box fontSize="12px" mt="4px">
      <Box as="button" textAlign="start" color={against ? 'red.fg' : 'accent.fg'} _hover={{ textDecoration: 'underline' }}
        title={`Show ${s.fact} in the knowledge graph`} onClick={() => select(s.fact, true)}>
        <Mono as="span" color="inherit">{against ? 'against · ' : ''}{s.loc || s.fact}</Mono>
      </Box>
      {s.title && <Text as="span" color="fg.muted"> {s.title}</Text>}
      {s.quote && <Text color="fg.subtle" fontStyle="italic" lineClamp={2}>“{s.quote}”</Text>}
    </Box>
  );
}

function Claim({ c }: { c: ClaimRow }) {
  const [open, setOpen] = useState(false);
  return (
    <Box bg="bg.panel" borderWidth="1px" borderColor={open ? 'accent.solid' : 'border'} rounded="md" p="10px">
      <Box cursor="pointer" role="button" tabIndex={0} aria-expanded={open} onClick={() => setOpen(!open)} onKeyDown={(ev) => { if (ev.key === 'Enter') setOpen(!open); }}>
        <Text fontSize="13px" lineHeight="1.4">{c.text}</Text>
        <HStack gap="4px" wrap="wrap" mt="6px">
          <Chip tone={c.status === 'proposed' ? 'muted' : 'accent'}>{c.retired ? 'retired' : c.status}{c.stale ? ', stale' : ''}</Chip>
          <Chip title="Confidence">{pct(c.confidence)}</Chip>
          <Chip tone={c.uses ? (c.wins * 2 >= c.uses ? 'ok' : 'bad') : 'muted'} title="Won in runs / used in runs">{c.wins}/{c.uses} won</Chip>
          <Chip title="Weight: (0.5 + confidence) × record × trust; sleep retires it under 0.1">w {c.weight.toFixed(2)}</Chip>
          {c.about.map((a) => <Chip key={a}>{a}</Chip>)}
          {c.applies_when.map((a) => <Chip key={a} tone="accent" title="Holds only in this context">when {a}</Chip>)}
          {c.against.length > 0 && <Chip tone="bad">{c.against.length} against</Chip>}
        </HStack>
      </Box>
      {open && (
        <Box mt="8px" borderTopWidth="1px" borderColor="border" pt="6px">
          {c.sources.length === 0 && c.against.length === 0 && <Text fontSize="12px" color="fg.subtle">No evidence left.</Text>}
          {c.sources.map((s) => <SourceLink key={s.fact} s={s} />)}
          {c.against.map((s) => <SourceLink key={s.fact} s={s} against />)}
          {c.retired && <Text fontSize="12px" color="fg.subtle" mt="4px">Retired: {c.retired}</Text>}
          <Mono mt="6px" fontSize="11px">{c.key}</Mono>
        </Box>
      )}
    </Box>
  );
}

function Claims({ brain, version }: { brain: string; version: number }) {
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState(''); // the search, once typing pauses
  const [status, setStatus] = useState<Status>('');
  const [sort, setSort] = useState<Sort>('weight');
  const [page, setPage] = useState<{ total: number; claims: ClaimRow[] } | null>(null);
  const [error, setError] = useState('');
  const end = useRef<HTMLButtonElement>(null);

  useEffect(() => { const t = setTimeout(() => setQuery(search.trim()), 250); return () => clearTimeout(t); }, [search]);
  useEffect(() => {
    let stale = false;
    brainClaims({ brain, q: query, status, sort, limit: PAGE })
      .then((p) => { if (!stale) { setPage(p); setError(''); } })
      .catch((err) => { if (!stale) setError(errorText(err)); });
    return () => { stale = true; };
  }, [brain, query, status, sort, version]);

  const more = !!page && page.claims.length < page.total;
  const loadMore = useCallback(() => {
    if (!page) return;
    brainClaims({ brain, q: query, status, sort, offset: page.claims.length, limit: PAGE })
      .then((p) => setPage((old) => old && { total: p.total, claims: [...old.claims, ...p.claims] }))
      .catch((err) => setError(errorText(err)));
  }, [brain, query, status, sort, page]);
  // The Load more button presses itself when it scrolls into view.
  useEffect(() => {
    if (!more || !end.current) return;
    const io = new IntersectionObserver((entries) => { if (entries.some((en) => en.isIntersecting)) loadMore(); }, { rootMargin: '0px 0px 300px 0px' });
    io.observe(end.current);
    return () => io.disconnect();
  }, [more, loadMore]);

  return (
    <Stack gap="8px" minW="0">
      <Flex gap="8px" wrap="wrap" align="center">
        <Input size="sm" flex="1" minW="200px" bg="bg.panel" type="search" placeholder="Filter claims: words in the text, a concept or a source"
          aria-label="Filter claims" value={search} onChange={(e) => setSearch(e.target.value)} />
        <Segmented size="xs" label="Status" value={status} onChange={setStatus} options={[
          { value: '', text: 'Active' }, { value: 'proposed', text: 'Proposed' }, { value: 'verified', text: 'Verified' },
          { value: 'confirmed', text: 'Confirmed', title: 'Confirmed by you (knowledge brain confirm)' }, { value: 'retired', text: 'Retired' },
        ]} />
        <Segmented size="xs" label="Sort" value={sort} onChange={setSort} options={[
          { value: 'weight', text: 'Weight' }, { value: 'uses', text: 'Uses' }, { value: 'new', text: 'Newest' },
        ]} />
      </Flex>
      {error && <Text color="red.fg" fontSize="13px">{error}</Text>}
      {page && <Text fontSize="12px" color="fg.subtle">{page.total} claims{query ? ` match "${query}"` : ''}</Text>}
      {page?.claims.map((c) => <Claim key={c.key} c={c} />)}
      {more && page && (
        <Button ref={end} size="xs" variant="ghost" color="fg.muted" onClick={loadMore}>Load more ({page.total - page.claims.length} left)</Button>
      )}
    </Stack>
  );
}

function Recall({ brain }: { brain: string }) {
  const { select } = useApp();
  const [task, setTask] = useState('');
  const [result, setResult] = useState<{ nodes: RecallNode[]; facts: RecallFact[] } | null>(null);
  const [error, setError] = useState('');
  const run = () => {
    if (!task.trim()) return;
    brainRecall(brain, task.trim()).then((r) => { setResult(r); setError(''); }).catch((err) => setError(errorText(err)));
  };
  useEffect(() => { setResult(null); }, [brain]);
  const claims = result?.nodes.filter((n) => n.node.node_type === 'claim').slice(0, 8) ?? [];
  return (
    <Box>
      <SectionTitle>Recall</SectionTitle>
      <Flex as="form" gap="6px" onSubmit={(e) => { e.preventDefault(); run(); }}>
        <Input size="sm" flex="1" minW="0" bg="bg.panel" type="search" placeholder="A task or question" aria-label="Task to recall for"
          value={task} onChange={(e) => setTask(e.target.value)} />
        <Button type="submit" size="sm" variant="outline">Recall</Button>
      </Flex>
      {error && <Text color="red.fg" fontSize="13px" mt="6px">{error}</Text>}
      {result && (
        <Stack gap="4px" mt="8px" fontSize="12px">
          {claims.length === 0 && result.facts.length === 0 && <Text color="fg.subtle">The brain knows nothing about it yet.</Text>}
          {claims.map((n) => (
            <Text key={n.node.key} lineHeight="1.4">
              <Text as="span" color="fg.subtle" fontVariantNumeric="tabular-nums">{n.score.toFixed(2)} </Text>{n.node.title}
            </Text>
          ))}
          {result.facts.length > 0 && <Text color="fg.subtle" mt="6px">Docs and sections to read and cite:</Text>}
          {result.facts.slice(0, 10).map((f) => (
            <Box key={f.key} as="button" textAlign="start" color="accent.fg" _hover={{ textDecoration: 'underline' }} onClick={() => select(f.key, true)}>
              <Mono as="span" color="inherit">{f.loc || f.key}</Mono> <Text as="span" color="fg.muted">{f.title}</Text>
            </Box>
          ))}
        </Stack>
      )}
    </Box>
  );
}

function Runs({ runs }: { runs: RunRow[] }) {
  return (
    <Box>
      <SectionTitle>Latest runs</SectionTitle>
      {runs.length === 0 && <Text fontSize="12px" color="fg.subtle">No runs yet: a run is a task done with the brain, with its check.</Text>}
      <Stack gap="6px">
        {runs.map((r) => (
          <Box key={r.key} borderWidth="1px" borderColor="border" rounded="md" p="8px" bg="bg.panel" fontSize="12px">
            <Text lineHeight="1.4" lineClamp={3}>{r.task}</Text>
            <HStack gap="4px" wrap="wrap" mt="4px">
              {r.passed === null ? <Chip>waiting for its outcome</Chip>
                : <Chip tone={r.passed ? 'ok' : 'bad'}>{r.passed ? 'passed' : 'failed'}{r.score !== undefined ? ` ${pct(r.score)}` : ''}</Chip>}
              <Chip>{r.used} used</Chip>
              {r.author && <Chip>{r.author}</Chip>}
            </HStack>
            {r.detail && <Text color="fg.muted" mt="4px">{r.detail}</Text>}
            <Text color="fg.subtle" mt="2px" lineClamp={1} title={r.check}>{r.check}</Text>
          </Box>
        ))}
      </Stack>
    </Box>
  );
}

export function BrainView() {
  const { version } = useApp();
  const [list, setList] = useState<BrainInfo[] | null>(null);
  const [brain, setBrain] = useState('');
  const [runs, setRuns] = useState<RunRow[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    fetchBrains().then((r) => {
      setList(r.brains);
      setBrain((b) => b || (r.brains.some((x) => x.name === r.default) ? r.default : r.brains[0]?.name ?? ''));
    }).catch((err) => setError(errorText(err)));
  }, [version]);
  useEffect(() => {
    if (!brain) return;
    brainRuns(brain).then(setRuns).catch((err) => setError(errorText(err)));
  }, [brain, version]);

  if (error && !list) return <Text p="16px" color="red.fg">{error}</Text>;
  if (!list) return <Text p="16px" color="fg.muted">Loading brains…</Text>;
  if (list.length === 0) {
    return (
      <Box p="24px" color="fg.muted" textAlign="center">
        <Text fontSize="16px" fontWeight="600" color="fg">No brain yet</Text>
        <Text mt="6px">A brain learns from these docs: <Mono as="span">knowledge brain learn</Mono> (docs/brain.md).</Text>
      </Box>
    );
  }
  const info = list.find((b) => b.name === brain);
  const st = info?.stats;
  const types = st?.node_types ?? {};
  return (
    <Box h="100%" overflowY="auto" p="16px">
      <Flex gap="12px" align="center" wrap="wrap" mb="12px">
        <NativeSelect.Root size="sm" w="180px">
          <NativeSelect.Field aria-label="Brain" value={brain} onChange={(e) => setBrain(e.target.value)} bg="bg.panel">
            {list.map((b) => <option key={b.name} value={b.name}>{b.name}{b.forked_from ? ` (from ${b.forked_from})` : ''}</option>)}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
        {st && (
          <Text fontSize="12px" color="fg.muted" fontVariantNumeric="tabular-nums">
            {types.claim ?? 0} claims · {types.concept ?? 0} concepts · {types.source ?? 0} sources · {st.runs} runs ({st.passed} passed, {st.waiting} waiting)
            · grammar {st.grammar}{info?.last_sleep ? ` · slept ${info.last_sleep.slice(0, 16).replace('T', ' ')}` : ' · never slept'}
          </Text>
        )}
      </Flex>
      {error && <Text color="red.fg" fontSize="13px" mb="8px">{error}</Text>}
      {brain && (
        <Grid templateColumns={{ base: '1fr', xl: 'minmax(0, 1fr) 360px' }} gap="20px" alignItems="start">
          <Claims brain={brain} version={version} />
          <Stack gap="8px" minW="0">
            <Recall brain={brain} />
            <Runs runs={runs} />
          </Stack>
        </Grid>
      )}
    </Box>
  );
}
