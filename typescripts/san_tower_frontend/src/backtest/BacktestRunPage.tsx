import { Badge, Box, Button, Grid, HStack, Heading, Stack, Table, Tabs, Text } from '@chakra-ui/react';
import { ArrowLeftIcon } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { backtestAnalysis, errorText } from '../api';
import type { BacktestDecision, BacktestMissed, BacktestRun, BacktestTrade } from '../gen/san/tower/v1/backtest_analysis_pb';
import { EquityChart, type TimeRange } from './EquityChart';
import { chartColors, rangeText, sec, utc, wib } from './format';
import { PriceChart } from './PriceChart';
import { useSort, type SortValue } from './sort';
import { DecisionLog, filterTrades, noFilter, OrderBadge, orderKey, StatsPanel, TradesTable, type TradeFilter } from './RunParts';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';

// defaultTimeframe is the step timeframe, or H1 when a long run would make
// the step chart heavy (more than about 30 000 candles).
function defaultTimeframe(run: BacktestRun) {
  const tfs = run.chartTimeframes;
  if (tfs.length === 0) return '';
  const days = (sec(run.summary?.to) - sec(run.summary?.from)) / 86400;
  const minutes: Record<string, number> = { M1: 1, M5: 5, M15: 15, M30: 30, H1: 60 };
  const step = minutes[tfs[0]];
  if (step && (days * 24 * 60 * 5) / 7 / step > 30000 && tfs.includes('H1')) return 'H1';
  return tfs[0];
}

// BacktestRunPage is one report (?path=<report folder>): the candle chart
// with every trade drawn (zoom and pan with the mouse, a timeframe
// switch), the equity and drawdown on the same time axis, the trades with
// filters (a click jumps the chart to the trade), the missed signals, the
// decision log around the selected trade or candle, and the stats.
export function BacktestRunPage() {
  const [params] = useSearchParams();
  const path = params.get('path') ?? '';
  const folder = params.get('folder') ?? '';
  const [run, setRun] = useState<BacktestRun>();
  const [error, setError] = useState('');
  const [tf, setTf] = useState('');
  const [filter, setFilter] = useState<TradeFilter>(noFilter);
  const [selected, setSelected] = useState<BacktestTrade>();
  const [range, setRange] = useState<TimeRange>();
  const [log, setLog] = useState<{ title: string; lines?: BacktestDecision[] }>({ title: 'Decision log' });

  useEffect(() => {
    let live = true;
    setRun(undefined);
    setError('');
    backtestAnalysis
      .getRun({ path, equityPoints: 20000 })
      .then((r) => {
        if (!live || !r.run) return;
        setRun(r.run);
        setTf(defaultTimeframe(r.run));
      })
      .catch((err) => live && setError(errorText(err)));
    return () => {
      live = false;
    };
  }, [path]);

  const hasLog = run?.summary?.hasDecisions ?? false;
  const loadLog = useCallback(
    async (title: string, q: { around?: number; from?: number; to?: number; limit: number }) => {
      if (!hasLog) return;
      const ts = (s?: number) => (s ? timestampFromDate(new Date(s * 1000)) : undefined);
      try {
        const r = await backtestAnalysis.getDecisions({ path, around: ts(q.around), from: ts(q.from), to: ts(q.to), limit: q.limit });
        setLog({ title, lines: r.decisions });
      } catch (err) {
        setLog({ title: `${title}: ${errorText(err)}`, lines: [] });
      }
    },
    [path, hasLog],
  );

  const select = useCallback(
    (t: BacktestTrade) => {
      setSelected(t);
      void loadLog(`Decisions of trade #${t.index + 1} (${wib(sec(t.signalTime))} to ${wib(sec(t.closeTime))} WIB)`, {
        from: sec(t.signalTime) - 1,
        to: sec(t.closeTime),
        limit: 500,
      });
    },
    [loadLog],
  );
  const candle = useCallback(
    (time: number) => {
      void loadLog(`Decisions around ${wib(time)} WIB`, { around: time, limit: 30 });
    },
    [loadLog],
  );

  const trades = useMemo(() => filterTrades(run?.trades ?? [], filter), [run, filter]);
  const equity = useMemo(
    () => [{ name: 'equity', color: chartColors().equity, points: (run?.equity ?? []).map((p) => ({ time: sec(p.time), value: p.equity })) }],
    [run],
  );
  const drawdown = useMemo(() => (run?.equity ?? []).map((p) => ({ time: sec(p.time), value: p.drawdownPct })), [run]);

  if (error)
    return (
      <Stack gap="3">
        <BackLink folder={folder} />
        <Text color="fg.error">{error}</Text>
      </Stack>
    );
  if (!run) return <Text color="fg.muted">Reading the report...</Text>;
  const s = run.summary!;
  return (
    <Stack gap="4">
      <BackLink folder={folder} />
      <Stack gap="1">
        <HStack gap="2" wrap="wrap">
          <Heading size="lg">{s.approach || '(no name)'}</Heading>
          <Text color="fg.muted">
            {s.instrument} {s.timeframes.join(' / ')}
          </Text>
          {s.kind === 'portfolio' && <Badge>portfolio</Badge>}
          {s.format === 'legacy' && <Badge title="Written before run.json">old report</Badge>}
          {s.paper && (
            <Badge colorPalette="purple" data-testid="paper-badge" title="A Live Trading paper session: no orders are sent">
              PAPER - no orders are sent{s.status && ` · ${s.status.replace('_', ' ')}`}
            </Badge>
          )}
        </HStack>
        <Text textStyle="sm" color="fg.muted">
          {rangeText(s.from, s.to)} WIB{s.ranAt && `, ran ${wib(sec(s.ranAt))} WIB`} · {s.path}
        </Text>
        {s.problem && <Text color="fg.warning">{s.problem}</Text>}
      </Stack>
      <Grid templateColumns={{ base: '1fr', xl: 'minmax(0, 3fr) minmax(360px, 1.2fr)' }} gap="5" alignItems="start">
        <Stack gap="3" minW="0">
          {run.chartTimeframes.length > 0 ? (
            <>
              <HStack gap="1" wrap="wrap" data-testid="timeframes">
                {run.chartTimeframes.map((x) => (
                  <Button key={x} size="xs" variant={x === tf ? 'solid' : 'outline'} onClick={() => setTf(x)}>
                    {x}
                  </Button>
                ))}
                {s.candlesFrom && (
                  <Badge size="sm" variant="subtle" colorPalette="blue" data-testid="candles-from"
                    title="This report saved no candles of its own: the chart uses a sibling report's, clipped to this report's range">
                    candles from {s.candlesFrom}
                  </Badge>
                )}
                <Text textStyle="xs" color="fg.muted" ms="2">
                  Wheel zooms, drag pans. Arrows are entries, dots exits; green won, red lost; dashed lines are stops and
                  targets, one set per open position.
                </Text>
              </HStack>
              <PriceChart
                path={path}
                timeframe={tf === run.chartTimeframes[0] ? '' : tf}
                trades={trades}
                selected={selected}
                onSelect={select}
                onCandle={candle}
                onRange={setRange}
              />
            </>
          ) : (
            <Text color="fg.muted" borderWidth="1px" rounded="md" p="4" data-testid="no-chart">
              No price chart: this report saved no candles and no sibling report has them; run the backtest again to
              chart its trades.
            </Text>
          )}
          <Text textStyle="sm" fontWeight="medium">
            Equity and drawdown{' '}
            {run.equityTotal > run.equity.length && (
              <Text as="span" color="fg.muted" fontWeight="normal">
                ({run.equity.length} of {run.equityTotal} points, keeping each stretch's low and high)
              </Text>
            )}
          </Text>
          <EquityChart curves={equity} drawdown={drawdown} range={range} height={280} />
        </Stack>
        <Box minW="0">
          <StatsPanel run={run} />
        </Box>
      </Grid>
      <Grid templateColumns={{ base: '1fr', xl: 'minmax(0, 3fr) minmax(360px, 1.2fr)' }} gap="5" alignItems="start">
        <Tabs.Root defaultValue="trades" size="sm" minW="0">
          <Tabs.List>
            <Tabs.Trigger value="trades">Trades ({run.trades.length})</Tabs.Trigger>
            <Tabs.Trigger value="missed">Missed ({run.missed.length})</Tabs.Trigger>
            <Tabs.Trigger value="summary">summary.txt</Tabs.Trigger>
          </Tabs.List>
          <Tabs.Content value="trades">
            <TradesTable all={run.trades} filter={filter} onFilter={setFilter} selected={selected} onSelect={select} />
          </Tabs.Content>
          <Tabs.Content value="missed">
            <MissedTable run={run} onTime={candle} />
          </Tabs.Content>
          <Tabs.Content value="summary">
            <Box as="pre" textStyle="xs" fontFamily="mono" whiteSpace="pre" overflowX="auto" borderWidth="1px" rounded="md" p="3">
              {run.summaryText || 'No summary.txt'}
            </Box>
          </Tabs.Content>
        </Tabs.Root>
        <Box minW="0" pt="10">
          <DecisionLog lines={log.lines} title={log.title} available={hasLog} />
        </Box>
      </Grid>
    </Stack>
  );
}

function BackLink({ folder }: { folder: string }) {
  return (
    <Link to={folder ? `/backtest-analysis?folder=${encodeURIComponent(folder)}` : '/backtest-analysis'}>
      <HStack gap="1" color="fg.muted" textStyle="sm" _hover={{ color: 'fg' }}>
        <ArrowLeftIcon size={14} /> Backtest Analysis
      </HStack>
    </Link>
  );
}

type MissedKey = 'time' | 'side' | 'reason' | 'strategy' | 'orderType';

// reasonColor: a pending order that ended unfilled is not a refused trade
const reasonColor: Record<string, string> = { expired: 'gray', cancelled: 'yellow' };

const missedValue = (m: BacktestMissed, k: MissedKey): SortValue =>
  k === 'time' ? sec(m.time) : k === 'orderType' ? orderKey(m.orderType, m.orderId) : m[k];

function MissedTable({ run, onTime }: { run: BacktestRun; onTime: (t: number) => void }) {
  const { sorted, header } = useSort(run.missed, { key: 'time', desc: false }, missedValue);
  if (run.missed.length === 0) return <Text color="fg.muted">No missed signals.</Text>;
  const orders = run.missed.some((m) => m.orderType !== '');
  return (
    <Table.ScrollArea borderWidth="1px" rounded="md" maxH="lg" data-testid="missed-table">
      <Table.Root size="sm" interactive stickyHeader>
        <Table.Header>
          <Table.Row>
            {header('time', 'Time (WIB)')}
            {header('side', 'Side')}
            {header('reason', 'Reason')}
            {header('strategy', 'Strategy')}
            {orders && header('orderType', 'Order')}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {sorted.map((m, i) => (
            <Table.Row key={i} cursor="pointer" onClick={() => onTime(sec(m.time))}>
              <Table.Cell title={utc(sec(m.time))}>{wib(sec(m.time))}</Table.Cell>
              <Table.Cell>{m.side}</Table.Cell>
              <Table.Cell>
                <Badge size="sm" variant="subtle" colorPalette={reasonColor[m.reason] ?? 'red'}>
                  {m.reason}
                </Badge>
              </Table.Cell>
              <Table.Cell>
                {m.strategy} {run.legs.length > 1 && `(leg ${m.leg})`}
              </Table.Cell>
              {orders && (
                <Table.Cell>
                  <OrderBadge type={m.orderType} id={m.orderId} tag={m.tag} />
                </Table.Cell>
              )}
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  );
}
