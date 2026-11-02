import {
  Badge,
  Box,
  Button,
  Grid,
  HStack,
  Input,
  InputGroup,
  Link,
  NativeSelect,
  Stack,
  Text,
  Wrap,
} from '@chakra-ui/react';
import { BrainIcon, SearchIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { errorText, knowledge } from '../api';
import type { Brain, BrainClaim, BrainRun, BrainSource, RecallBrainResponse } from '../gen/san/tower/v1/knowledge_pb';

// BrainPanel is a knowledge's brains, on its page's Brain view (the user,
// 2026-11-01: "make tower can show it"; docs/brain.md in the research repo):
// a brain's claims with what they rest on and how they worked out, its
// latest runs, and what it recalls for a task. Read-only: a brain changes by
// learning, runs and sleep (the Brain steps). A source opens its doc or
// section in the page's node dialog (onView).

const PAGE = 50;

type Status = '' | 'proposed' | 'verified' | 'confirmed' | 'retired';
type Sort = 'weight' | 'uses' | 'new';

const pct = (x: number) => `${Math.round(x * 100)}%`;

export function BrainPanel({ id, onView }: { id: number; onView: (key: string) => void }) {
  const [brains, setBrains] = useState<Brain[]>();
  const [brain, setBrain] = useState('');
  const [error, setError] = useState('');
  const [version, setVersion] = useState(0);

  useEffect(() => {
    let stale = false;
    knowledge
      .listBrains({ id })
      .then((r) => {
        if (stale) return;
        setBrains(r.brains);
        setBrain((b) =>
          r.brains.some((x) => x.name === b)
            ? b
            : ((r.brains.find((x) => x.name === 'root') ?? r.brains[0])?.name ?? ''),
        );
        setError('');
      })
      .catch((err) => !stale && setError(errorText(err)));
    return () => {
      stale = true;
    };
  }, [id, version]);

  if (error) return <Text color="fg.error">{error}</Text>;
  if (!brains) return <Text color="fg.muted">Loading brains...</Text>;
  if (brains.length === 0) {
    return (
      <Stack align="center" textAlign="center" gap="2" py="10" color="fg.muted">
        <BrainIcon size="28" />
        <Text fontWeight="semibold" color="fg">
          No brain yet
        </Text>
        <Text textStyle="sm" maxW="lg">
          A brain learns from this knowledge's docs: a workflow's Brain Learn step makes it, then Brain Sleep keeps it
          tidy. Its claims, runs and recall show here once it has learned.
        </Text>
      </Stack>
    );
  }
  const b = brains.find((x) => x.name === brain) ?? brains[0];
  return (
    <Stack gap="3" minH="0">
      <HStack gap="3" wrap="wrap">
        {brains.length > 1 ? (
          <NativeSelect.Root size="sm" w="56">
            <NativeSelect.Field aria-label="Brain" value={b.name} onChange={(e) => setBrain(e.currentTarget.value)}>
              {brains.map((x) => (
                <option key={x.name} value={x.name}>
                  {x.name}
                  {x.forkedFrom ? ` (from ${x.forkedFrom})` : ''}
                </option>
              ))}
            </NativeSelect.Field>
            <NativeSelect.Indicator />
          </NativeSelect.Root>
        ) : (
          <HStack gap="1.5">
            <BrainIcon size="16" />
            <Text fontWeight="semibold">{b.name}</Text>
          </HStack>
        )}
        <Text textStyle="sm" color="fg.muted">
          {b.claims} claims · {b.concepts} concepts · {b.sources} sources · {b.runs} runs ({b.passed} passed,{' '}
          {b.waiting} waiting) · grammar {b.grammar} ·{' '}
          {b.lastSleep ? `slept ${b.lastSleep.slice(0, 16).replace('T', ' ')}` : 'never slept'}
        </Text>
        <Button size="xs" variant="ghost" onClick={() => setVersion((v) => v + 1)}>
          Refresh
        </Button>
      </HStack>
      <Grid templateColumns={{ base: '1fr', xl: 'minmax(0, 1fr) 380px' }} gap="5" alignItems="start" minH="0">
        <Claims id={id} brain={b.name} version={version} onView={onView} />
        <Stack gap="5" minW="0">
          <Recall id={id} brain={b.name} onView={onView} />
          <Runs id={id} brain={b.name} version={version} />
        </Stack>
      </Grid>
    </Stack>
  );
}

function Title({ children }: { children: string }) {
  return (
    <Text textStyle="xs" textTransform="uppercase" letterSpacing="wider" color="fg.muted" fontWeight="semibold">
      {children}
    </Text>
  );
}

// Source is a doc or section a claim rests on: its file:line opens it.
function Source({ s, against, onView }: { s: BrainSource; against?: boolean; onView: (key: string) => void }) {
  return (
    <Box textStyle="xs">
      <Link
        color={against ? 'fg.error' : 'colorPalette.fg'}
        colorPalette="blue"
        fontFamily="mono"
        onClick={() => onView(s.fact)}
        title={`Open ${s.fact}`}
      >
        {against ? 'against · ' : ''}
        {s.loc || s.fact}
      </Link>{' '}
      <Text as="span" color="fg.muted">
        {s.title}
      </Text>
      {s.quote && (
        <Text color="fg.subtle" fontStyle="italic" lineClamp={2}>
          “{s.quote}”
        </Text>
      )}
    </Box>
  );
}

function Claim({ c, onView }: { c: BrainClaim; onView: (key: string) => void }) {
  const [open, setOpen] = useState(false);
  const record = c.uses === 0 ? 'gray' : c.wins * 2 >= c.uses ? 'green' : 'red';
  return (
    <Box
      borderWidth="1px"
      rounded="md"
      p="3"
      bg="bg.panel"
      borderColor={open ? 'colorPalette.solid' : 'border'}
      colorPalette="blue"
    >
      <Box cursor="pointer" onClick={() => setOpen(!open)}>
        <Text textStyle="sm">{c.text}</Text>
        <Wrap gap="1" mt="1.5">
          <Badge
            variant={c.status === 'proposed' ? 'outline' : 'subtle'}
            colorPalette={c.status === 'confirmed' ? 'green' : 'blue'}
          >
            {c.retired ? 'retired' : c.status}
            {c.stale ? ', stale' : ''}
          </Badge>
          <Badge variant="outline" title="Confidence">
            {pct(c.confidence)}
          </Badge>
          <Badge variant="outline" colorPalette={record} title="Won in runs / used in runs">
            {c.wins}/{c.uses} won
          </Badge>
          <Badge variant="outline" title="Weight: (0.5 + confidence) × record × trust; sleep retires it under 0.1">
            w {c.weight.toFixed(2)}
          </Badge>
          {c.about.map((a) => (
            <Badge key={a} variant="surface">
              {a}
            </Badge>
          ))}
          {c.appliesWhen.map((a) => (
            <Badge key={a} variant="surface" colorPalette="purple" title="Holds only in this context">
              when {a}
            </Badge>
          ))}
          {c.against.length > 0 && <Badge colorPalette="red">{c.against.length} against</Badge>}
        </Wrap>
      </Box>
      {open && (
        <Stack gap="1.5" mt="2" pt="2" borderTopWidth="1px">
          {c.sources.length === 0 && c.against.length === 0 && (
            <Text textStyle="xs" color="fg.muted">
              No evidence left.
            </Text>
          )}
          {c.sources.map((s) => (
            <Source key={s.fact} s={s} onView={onView} />
          ))}
          {c.against.map((s) => (
            <Source key={s.fact} s={s} against onView={onView} />
          ))}
          {c.retired && (
            <Text textStyle="xs" color="fg.muted">
              Retired: {c.retired}
            </Text>
          )}
          <Text textStyle="2xs" color="fg.subtle" fontFamily="mono">
            {c.key}
          </Text>
        </Stack>
      )}
    </Box>
  );
}

function Claims({
  id,
  brain,
  version,
  onView,
}: {
  id: number;
  brain: string;
  version: number;
  onView: (key: string) => void;
}) {
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState(''); // the search, once typing pauses
  const [status, setStatus] = useState<Status>('');
  const [sort, setSort] = useState<Sort>('weight');
  const [page, setPage] = useState<{ total: number; claims: BrainClaim[] }>();
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setQuery(search.trim()), 250);
    return () => clearTimeout(t);
  }, [search]);
  useEffect(() => {
    let stale = false;
    knowledge
      .listBrainClaims({ id, brain, query, status, sort, limit: PAGE })
      .then((r) => {
        if (!stale) {
          setPage({ total: r.total, claims: r.claims });
          setError('');
        }
      })
      .catch((err) => !stale && setError(errorText(err)));
    return () => {
      stale = true;
    };
  }, [id, brain, query, status, sort, version]);

  const more = async () => {
    if (!page) return;
    setLoading(true);
    try {
      const r = await knowledge.listBrainClaims({
        id,
        brain,
        query,
        status,
        sort,
        offset: page.claims.length,
        limit: PAGE,
      });
      setPage({ total: r.total, claims: [...page.claims, ...r.claims] });
    } catch (err) {
      setError(errorText(err));
    }
    setLoading(false);
  };

  return (
    <Stack gap="2" minW="0">
      <HStack gap="2" wrap="wrap">
        <InputGroup flex="1" minW="56" startElement={<SearchIcon size="16" />}>
          <Input
            size="sm"
            value={search}
            onChange={(e) => setSearch(e.currentTarget.value)}
            placeholder="Filter claims: words in the text, a concept or a source"
          />
        </InputGroup>
        <NativeSelect.Root size="sm" w="36">
          <NativeSelect.Field
            aria-label="Status"
            value={status}
            onChange={(e) => setStatus(e.currentTarget.value as Status)}
          >
            <option value="">In use</option>
            <option value="proposed">Proposed</option>
            <option value="verified">Verified</option>
            <option value="confirmed">Confirmed</option>
            <option value="retired">Retired</option>
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
        <NativeSelect.Root size="sm" w="36">
          <NativeSelect.Field aria-label="Sort" value={sort} onChange={(e) => setSort(e.currentTarget.value as Sort)}>
            <option value="weight">By weight</option>
            <option value="uses">By uses</option>
            <option value="new">Newest</option>
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      </HStack>
      {error && <Text color="fg.error">{error}</Text>}
      {page && (
        <Text textStyle="xs" color="fg.muted">
          {page.total} claims{query ? ` match "${query}"` : ''}
        </Text>
      )}
      {page?.claims.map((c) => (
        <Claim key={c.key} c={c} onView={onView} />
      ))}
      {page && page.claims.length < page.total && (
        <Button size="xs" variant="ghost" color="fg.muted" loading={loading} onClick={() => void more()}>
          Load more ({page.total - page.claims.length} left)
        </Button>
      )}
    </Stack>
  );
}

function Recall({ id, brain, onView }: { id: number; brain: string; onView: (key: string) => void }) {
  const [task, setTask] = useState('');
  const [result, setResult] = useState<RecallBrainResponse>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  useEffect(() => setResult(undefined), [brain]);
  const run = async () => {
    if (!task.trim()) return;
    setBusy(true);
    try {
      setResult(await knowledge.recallBrain({ id, brain, query: task.trim() }));
      setError('');
    } catch (err) {
      setError(errorText(err));
    }
    setBusy(false);
  };
  return (
    <Stack gap="2">
      <Title>Recall</Title>
      <HStack
        as="form"
        gap="2"
        onSubmit={(e) => {
          e.preventDefault();
          void run();
        }}
      >
        <Input
          size="sm"
          value={task}
          onChange={(e) => setTask(e.currentTarget.value)}
          placeholder="A task or question"
        />
        <Button type="submit" size="sm" variant="outline" loading={busy}>
          Recall
        </Button>
      </HStack>
      {error && (
        <Text color="fg.error" textStyle="sm">
          {error}
        </Text>
      )}
      {result && (
        <Stack gap="1" textStyle="xs">
          {result.claims.length === 0 && result.facts.length === 0 && (
            <Text color="fg.muted">The brain knows nothing about it yet.</Text>
          )}
          {result.claims.map((c) => (
            <Text key={c.key}>
              <Text as="span" color="fg.subtle" fontFamily="mono">
                {c.score.toFixed(2)}{' '}
              </Text>
              {c.text}
            </Text>
          ))}
          {result.facts.length > 0 && (
            <Text color="fg.muted" mt="1">
              Docs and sections to read and cite:
            </Text>
          )}
          {result.facts.map((f) => (
            <Box key={f.key}>
              <Link colorPalette="blue" color="colorPalette.fg" fontFamily="mono" onClick={() => onView(f.key)}>
                {f.loc || f.key}
              </Link>{' '}
              <Text as="span" color="fg.muted">
                {f.title}
              </Text>
            </Box>
          ))}
        </Stack>
      )}
    </Stack>
  );
}

function Runs({ id, brain, version }: { id: number; brain: string; version: number }) {
  const [runs, setRuns] = useState<BrainRun[]>();
  const [error, setError] = useState('');
  useEffect(() => {
    let stale = false;
    knowledge
      .listBrainRuns({ id, brain, limit: 30 })
      .then((r) => !stale && setRuns(r.runs))
      .catch((err) => !stale && setError(errorText(err)));
    return () => {
      stale = true;
    };
  }, [id, brain, version]);
  return (
    <Stack gap="2">
      <Title>Latest runs</Title>
      {error && (
        <Text color="fg.error" textStyle="sm">
          {error}
        </Text>
      )}
      {runs?.length === 0 && (
        <Text textStyle="xs" color="fg.muted">
          No runs yet: a run is a task done with the brain, and its check.
        </Text>
      )}
      {runs?.map((r) => (
        <Box key={r.key} borderWidth="1px" rounded="md" p="2.5" bg="bg.panel" textStyle="xs">
          <Text lineClamp={3}>{r.task}</Text>
          <Wrap gap="1" mt="1">
            {r.hasOutcome ? (
              <Badge colorPalette={r.passed ? 'green' : 'red'}>
                {r.passed ? 'passed' : 'failed'}
                {r.score !== undefined ? ` ${pct(r.score)}` : ''}
              </Badge>
            ) : (
              <Badge variant="outline">waiting for its outcome</Badge>
            )}
            <Badge variant="outline">{r.used} used</Badge>
            {r.author && <Badge variant="outline">{r.author}</Badge>}
          </Wrap>
          {r.detail && (
            <Text color="fg.muted" mt="1">
              {r.detail}
            </Text>
          )}
          <Text color="fg.subtle" mt="0.5" lineClamp={1} title={r.check}>
            {r.check}
          </Text>
        </Box>
      ))}
    </Stack>
  );
}
