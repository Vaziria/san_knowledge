import { Badge, Box, Button, Checkbox, CloseButton, HStack, Heading, Input, InputGroup, Stack, Table, Text, Wrap } from '@chakra-ui/react';
import { ChevronDownIcon, ChevronUpIcon, FolderOpenIcon, SearchIcon } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { backtestAnalysis, errorText } from '../api';
import type { BacktestRunSummary, BacktestWatchFolderResponse } from '../gen/san/tower/v1/backtest_analysis_pb';
import { Compare } from './Compare';
import { FolderPicker } from './FolderPicker';
import { pct, pf, rangeText, runLink, sec } from './format';
import { StreamIndicator, useStream } from '../stream';

type SortKey = 'approach' | 'instrument' | 'from' | 'trades' | 'winRate' | 'profitFactor' | 'returnPct' | 'maxDrawdownPct' | 'relPath';

const columns: { key: SortKey; label: string; end?: boolean; w?: string }[] = [
  { key: 'approach', label: 'Approach' },
  { key: 'instrument', label: 'Instrument', w: '28' },
  { key: 'from', label: 'Range (WIB)', w: '52' },
  { key: 'trades', label: 'Trades', end: true, w: '20' },
  { key: 'winRate', label: 'Win %', end: true, w: '20' },
  { key: 'profitFactor', label: 'PF', end: true, w: '16' },
  { key: 'returnPct', label: 'Net %', end: true, w: '20' },
  { key: 'maxDrawdownPct', label: 'Max DD %', end: true, w: '24' },
  { key: 'relPath', label: 'Folder', w: '48' },
];

// idle is a stream that sends nothing until it is closed: no folder open.
async function* idle(signal: AbortSignal): AsyncIterable<BacktestWatchFolderResponse> {
  await new Promise<void>((resolve) => signal.addEventListener('abort', () => resolve()));
}

const value = (r: BacktestRunSummary, k: SortKey) => (k === 'from' ? sec(r.from) : r[k]);

// BacktestAnalysisList is the Backtest Analysis page (the user, 2026-11-15:
// "We should have menu in tower named backtest" ... "Sorry 'backtest
// analysis'"): it opens a folder of San Backtest reports on this computer
// (a path, the folder picker, or a recent folder) and lists the runs in it
// and its subfolders with their key stats, sortable and searchable. It only
// reads reports; runs are started with san_backtest itself (the user,
// 2026-11-15: "1a"). The list follows the folder (WatchFolder): a report
// written meanwhile shows by itself. Ticking two or more runs compares them.
export function BacktestAnalysisList() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const folder = params.get('folder') ?? '';
  const [typed, setTyped] = useState(folder);
  const [picking, setPicking] = useState(false);
  const [recent, setRecent] = useState<string[]>([]);
  const [runs, setRuns] = useState<BacktestRunSummary[]>();
  const [truncated, setTruncated] = useState(false);
  const [error, setError] = useState('');
  const [q, setQ] = useState('');
  const [sort, setSort] = useState<{ key: SortKey; desc: boolean }>({ key: 'relPath', desc: false });
  const [ticked, setTicked] = useState<string[]>([]);

  const loadRecent = () =>
    backtestAnalysis
      .listRecentFolders({})
      .then((r) => setRecent(r.folders))
      .catch(() => {});
  useEffect(() => {
    void loadRecent();
  }, []);

  const open = async (path: string) => {
    setError('');
    try {
      const r = await backtestAnalysis.openFolder({ path });
      setParams({ folder: r.path });
      setTyped(r.path);
      setTicked([]);
      void loadRecent();
    } catch (err) {
      setError(errorText(err));
    }
  };

  const live = useStream<BacktestWatchFolderResponse>(
    (signal) => (folder ? backtestAnalysis.watchFolder({ path: folder }, { signal }) : idle(signal)),
    (m) => {
      const ev = m.event;
      if (ev.case === 'snapshot') {
        setRuns(ev.value.runs);
        setTruncated(ev.value.truncated);
      } else if (ev.case === 'changed') {
        const r = ev.value;
        setRuns((old) => [...(old ?? []).filter((x) => x.path !== r.path), r]);
      } else if (ev.case === 'removed') {
        const p = ev.value;
        setRuns((old) => (old ?? []).filter((x) => x.path !== p));
      }
    },
    [folder],
    { onEnd: (err) => setError(errorText(err)) },
  );

  useEffect(() => {
    setRuns(undefined);
    setError('');
  }, [folder]);

  const shown = useMemo(() => {
    const words = q.toLowerCase().split(/\s+/).filter(Boolean);
    const list = (runs ?? []).filter((r) => {
      const text = [r.approach, r.instrument, r.relPath, r.kind, r.timeframes.join(' ')].join(' ').toLowerCase();
      return words.every((w) => text.includes(w));
    });
    list.sort((a, b) => {
      const x = value(a, sort.key);
      const y = value(b, sort.key);
      const c = typeof x === 'string' ? x.localeCompare(String(y)) : (x as number) - (y as number);
      return sort.desc ? -c : c;
    });
    return list;
  }, [runs, q, sort]);

  const tick = (p: string, on: boolean) => setTicked((t) => (on ? [...t.filter((x) => x !== p), p] : t.filter((x) => x !== p)));
  const comparing = (runs ?? []).filter((r) => ticked.includes(r.path));

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Backtest Analysis</Heading>
        {folder && <StreamIndicator state={live} />}
      </HStack>
      <Text color="fg.muted" textStyle="sm">
        Opens a folder of San Backtest reports on this computer (san_backtest's --out) and its subfolders. It only reads
        them: nothing in a report folder is changed. New reports written into the open folder show by themselves.
      </Text>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void open(typed);
        }}
      >
        <HStack maxW="4xl">
          <Input size="sm" value={typed} onChange={(e) => setTyped(e.currentTarget.value)} placeholder="C:\path\to\reports" aria-label="Report folder" />
          <Button size="sm" variant="outline" onClick={() => setPicking(true)}>
            <FolderOpenIcon /> Browse
          </Button>
          <Button size="sm" colorPalette="blue" type="submit" disabled={!typed.trim()}>
            Open
          </Button>
        </HStack>
      </form>
      {recent.length > 0 && (
        <Wrap gap="2" align="center" data-testid="recent-folders">
          <Text textStyle="sm" color="fg.muted">
            Recent:
          </Text>
          {recent.map((p) => (
            <HStack key={p} gap="0" borderWidth="1px" rounded="md" ps="2" bg={p === folder ? 'bg.muted' : undefined}>
              <Button size="xs" variant="plain" px="0" onClick={() => void open(p)} title={p} maxW="md">
                <Text truncate>{p}</Text>
              </Button>
              <CloseButton
                size="2xs"
                aria-label={`Forget ${p}`}
                onClick={() => backtestAnalysis.forgetRecentFolder({ path: p }).then((r) => setRecent(r.folders))}
              />
            </HStack>
          ))}
        </Wrap>
      )}
      {error && <Text color="fg.error">{error}</Text>}
      {folder && (
        <>
          <HStack justify="space-between" wrap="wrap" gap="3">
            <InputGroup maxW="sm" startElement={<SearchIcon size="16" />}>
              <Input size="sm" value={q} onChange={(e) => setQ(e.currentTarget.value)} placeholder="Search approach, instrument, folder" />
            </InputGroup>
            <HStack>
              {runs && (
                <Text textStyle="sm" color="fg.muted">
                  {shown.length} of {runs.length} runs{truncated && ' (the scan stopped at its folder limit)'}
                </Text>
              )}
              {ticked.length > 0 && (
                <Button size="xs" variant="ghost" onClick={() => setTicked([])}>
                  Clear ticks ({ticked.length})
                </Button>
              )}
            </HStack>
          </HStack>
          {!runs ? (
            <Text color="fg.muted">Reading the folder...</Text>
          ) : runs.length === 0 ? (
            <Text color="fg.muted">
              No reports in this folder or its subfolders. A report is a folder san_backtest wrote with --out (run.json, or
              summary.txt and trades.csv for older ones).
            </Text>
          ) : (
            <Table.ScrollArea borderWidth="1px" rounded="md">
              <Table.Root size="sm" interactive data-testid="run-list">
                <Table.Header>
                  <Table.Row>
                    <Table.ColumnHeader w="10" />
                    {columns.map((c) => (
                      <Table.ColumnHeader
                        key={c.key}
                        textAlign={c.end ? 'end' : undefined}
                        cursor="pointer"
                        userSelect="none"
                        onClick={() => setSort((s) => ({ key: c.key, desc: s.key === c.key ? !s.desc : c.end === true }))}
                      >
                        <HStack gap="1" justify={c.end ? 'flex-end' : undefined}>
                          {c.label}
                          {sort.key === c.key && (sort.desc ? <ChevronDownIcon size={14} /> : <ChevronUpIcon size={14} />)}
                        </HStack>
                      </Table.ColumnHeader>
                    ))}
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {shown.map((r) => (
                    <Table.Row key={r.path} data-testid="run-row">
                      <Table.Cell>
                        <Checkbox.Root
                          size="sm"
                          checked={ticked.includes(r.path)}
                          onCheckedChange={(e) => tick(r.path, e.checked === true)}
                          aria-label={`Compare ${r.relPath}`}
                        >
                          <Checkbox.HiddenInput />
                          <Checkbox.Control />
                        </Checkbox.Root>
                      </Table.Cell>
                      <Table.Cell>
                        <HStack gap="2">
                          <Link to={runLink(r.path, folder)}>
                            <Text fontWeight="medium" _hover={{ textDecoration: 'underline' }}>
                              {r.approach || '(no name)'}
                            </Text>
                          </Link>
                          {r.kind === 'portfolio' && <Badge size="sm">portfolio</Badge>}
                          {r.format === 'legacy' && (
                            <Badge size="sm" colorPalette="gray" title="Written before run.json: no candles or decision log">
                              old
                            </Badge>
                          )}
                          {r.hasWalk && <Badge size="sm" colorPalette="purple">walk</Badge>}
                          {r.problem && (
                            <Badge size="sm" colorPalette="red" title={r.problem}>
                              problem
                            </Badge>
                          )}
                        </HStack>
                      </Table.Cell>
                      <Table.Cell>
                        {r.instrument}
                        {r.timeframes.length > 0 && (
                          <Text as="span" color="fg.muted">
                            {' '}
                            {r.timeframes[0]}
                          </Text>
                        )}
                      </Table.Cell>
                      <Table.Cell>{rangeText(r.from, r.to)}</Table.Cell>
                      <Table.Cell textAlign="end">{r.trades}</Table.Cell>
                      <Table.Cell textAlign="end">{pct(r.winRate)}</Table.Cell>
                      <Table.Cell textAlign="end">{pf(r.profitFactor)}</Table.Cell>
                      <Table.Cell textAlign="end" color={r.returnPct > 0 ? 'fg.success' : r.returnPct < 0 ? 'fg.error' : undefined}>
                        {pct(r.returnPct, 2)}
                      </Table.Cell>
                      <Table.Cell textAlign="end">{pct(r.maxDrawdownPct, 2)}</Table.Cell>
                      <Table.Cell>
                        <Text truncate title={r.path} color="fg.muted">
                          {r.relPath}
                        </Text>
                      </Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Root>
            </Table.ScrollArea>
          )}
          {comparing.length === 1 && (
            <Text textStyle="sm" color="fg.muted">
              Tick one more run to compare.
            </Text>
          )}
          {comparing.length >= 2 && (
            <Box>
              <Compare runs={comparing} onOpen={(p) => navigate(runLink(p, folder))} />
            </Box>
          )}
        </>
      )}
      {!folder && (
        <Text color="fg.muted" textStyle="sm">
          Open a folder to see its runs: type its path, Browse for it, or pick a recent one.
        </Text>
      )}
      {picking && (
        <FolderPicker
          start={typed || recent[0] || ''}
          onClose={() => setPicking(false)}
          onPick={(p: string) => {
            setPicking(false);
            void open(p);
          }}
        />
      )}
    </Stack>
  );
}
