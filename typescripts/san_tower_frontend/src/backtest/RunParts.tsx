import { Badge, Box, Button, Grid, HStack, NativeSelect, Stack, Table, Text } from '@chakra-ui/react';
import { ChevronDownIcon, ChevronUpIcon } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { BacktestDecision, BacktestRun, BacktestStats, BacktestTrade } from '../gen/san/tower/v1/backtest_analysis_pb';
import { money, pct, pf, price, sec, signed, utc, wib } from './format';

// The run page's panels: stats, trades, missed signals, decision log.

function Cell({ label, value, tone }: { label: string; value: string; tone?: number }) {
  return (
    <Box>
      <Text textStyle="xs" color="fg.muted">
        {label}
      </Text>
      <Text fontWeight="medium" color={tone ? (tone > 0 ? 'fg.success' : 'fg.error') : undefined}>
        {value}
      </Text>
    </Box>
  );
}

function StatsGrid({ s }: { s: BacktestStats }) {
  return (
    <Grid templateColumns="repeat(auto-fill, minmax(120px, 1fr))" gap="3">
      <Cell label="Trades" value={`${s.trades} (${s.wins} won, ${s.losses} lost)`} />
      <Cell label="Win rate" value={pct(s.winRate)} />
      <Cell label="Profit factor" value={pf(s.profitFactor)} />
      <Cell label="Average R" value={s.avgR.toFixed(2)} />
      <Cell label="Net profit" value={`${signed(s.netProfit)} (${pct(s.returnPct, 2)})`} tone={s.netProfit} />
      <Cell label="Balance" value={`${money(s.startBalance)} -> ${money(s.endBalance)}`} />
      <Cell label="Max drawdown" value={`${money(s.maxDrawdown)} (${pct(s.maxDrawdownPct, 2)})`} />
      <Cell label="Losing streak" value={String(s.longestLosingStreak)} />
      <Cell label="Commission" value={money(s.commission)} />
      <Cell label="Swap" value={money(s.swap)} />
    </Grid>
  );
}

// StatsPanel is the summary, the first half against the second, skipped
// signals, the walk-forward windows, costs, params and account.
export function StatsPanel({ run }: { run: BacktestRun }) {
  const s = run.stats;
  const halves = [
    ['First half', s?.firstHalf],
    ['Second half', s?.secondHalf],
  ] as const;
  const skipped = Object.entries(s?.skipped ?? {});
  return (
    <Stack gap="4" data-testid="stats-panel">
      {s && <StatsGrid s={s} />}
      {s && (
        <Table.Root size="sm" variant="outline">
          <Table.Header>
            <Table.Row>
              <Table.ColumnHeader />
              <Table.ColumnHeader textAlign="end">Trades</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Win %</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Net</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Avg R</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {halves.map(([label, h]) => (
              <Table.Row key={label}>
                <Table.Cell>{label}</Table.Cell>
                <Table.Cell textAlign="end">{h?.trades ?? 0}</Table.Cell>
                <Table.Cell textAlign="end">{pct(h?.winRate ?? 0)}</Table.Cell>
                <Table.Cell textAlign="end" color={(h?.netProfit ?? 0) >= 0 ? 'fg.success' : 'fg.error'}>
                  {money(h?.netProfit ?? 0)}
                </Table.Cell>
                <Table.Cell textAlign="end">{(h?.avgR ?? 0).toFixed(2)}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Root>
      )}
      {skipped.length > 0 && (
        <Text textStyle="sm">
          <Text as="span" color="fg.muted">
            Skipped:{' '}
          </Text>
          {skipped.map(([k, v]) => `${k} ${v}`).join(', ')}
        </Text>
      )}
      {run.walk.length > 0 && (
        <Stack gap="2" data-testid="walk-windows">
          <Text fontWeight="medium">
            Walk-forward {run.walkTrain && `(train ${run.walkTrain}, test ${run.walkTest})`}
          </Text>
          <Table.ScrollArea borderWidth="1px" rounded="md">
            <Table.Root size="sm">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeader>Test from</Table.ColumnHeader>
                  <Table.ColumnHeader>Params</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Train net</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Test trades</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Test net</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Test win %</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {run.walk.map((w, i) => (
                  <Table.Row key={i}>
                    <Table.Cell>{wib(sec(w.testFrom), false)}</Table.Cell>
                    <Table.Cell>{w.params}</Table.Cell>
                    <Table.Cell textAlign="end">{money(w.trainNet)}</Table.Cell>
                    <Table.Cell textAlign="end">{w.testTrades}</Table.Cell>
                    <Table.Cell textAlign="end" color={w.testNet >= 0 ? 'fg.success' : 'fg.error'}>
                      {money(w.testNet)}
                    </Table.Cell>
                    <Table.Cell textAlign="end">{pct(w.testWinRate)}</Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Root>
          </Table.ScrollArea>
          {run.outOfSample && (
            <Text textStyle="sm">
              Out of sample: {run.outOfSample.trades} trades, net {money(run.outOfSample.netProfit)}, win {pct(run.outOfSample.winRate)}, profit factor{' '}
              {pf(run.outOfSample.profitFactor)}, max drawdown {pct(run.outOfSample.maxDrawdownPct, 2)}
            </Text>
          )}
        </Stack>
      )}
      <Stack gap="1" textStyle="sm">
        {Object.keys(run.params).length > 0 && (
          <Text>
            <Text as="span" color="fg.muted">
              Params:{' '}
            </Text>
            {Object.entries(run.params)
              .sort()
              .map(([k, v]) => `${k}=${v}`)
              .join(' ')}
          </Text>
        )}
        {run.account && run.account.currency && (
          <Text>
            <Text as="span" color="fg.muted">
              Account:{' '}
            </Text>
            {money(run.account.balance)} {run.account.currency}, leverage 1:{run.account.leverage}
            {run.account.maxOpen > 0 && `, max open ${run.account.maxOpen}`}
            {run.account.maxDailyLoss > 0 && `, max daily loss ${run.account.maxDailyLoss}%`}
          </Text>
        )}
        {run.costs
          .filter((c) => c.spread)
          .map((c) => (
            <Text key={c.leg}>
              <Text as="span" color="fg.muted">
                Costs {c.instrument}:{' '}
              </Text>
              spread {c.spread}; commission {c.commission}; swap {c.swap}; slippage {c.slippage}
              {c.overridden.length > 0 && <Badge ms="2">overridden: {c.overridden.join(', ')}</Badge>}
            </Text>
          ))}
        {run.notes.map((n, i) => (
          <Text key={i} color="fg.warning">
            {n}
          </Text>
        ))}
      </Stack>
    </Stack>
  );
}

type TradeKey = 'index' | 'open' | 'side' | 'net' | 'r' | 'reason' | 'session';

function Select({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (v: string) => void }) {
  return (
    <NativeSelect.Root size="xs" w="auto">
      <NativeSelect.Field aria-label={label} value={value} onChange={(e) => onChange(e.currentTarget.value)}>
        <option value="">{label}: all</option>
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </NativeSelect.Field>
      <NativeSelect.Indicator />
    </NativeSelect.Root>
  );
}

export type TradeFilter = { side: string; result: string; session: string; month: string; reason: string };

export const noFilter: TradeFilter = { side: '', result: '', session: '', month: '', reason: '' };

export function filterTrades(ts: BacktestTrade[], f: TradeFilter) {
  return ts.filter(
    (t) =>
      (!f.side || t.side === f.side) &&
      (!f.result || (f.result === 'win' ? t.net > 0 : t.net <= 0)) &&
      (!f.session || t.session === f.session) &&
      (!f.month || t.month === f.month) &&
      (!f.reason || t.reason === f.reason),
  );
}

const PAGE = 50;

// TradesTable lists the trades, filtered and sorted; a click selects one
// (the chart jumps to it).
export function TradesTable({
  all,
  filter,
  onFilter,
  selected,
  onSelect,
}: {
  all: BacktestTrade[];
  filter: TradeFilter;
  onFilter: (f: TradeFilter) => void;
  selected?: BacktestTrade;
  onSelect: (t: BacktestTrade) => void;
}) {
  const [sort, setSort] = useState<{ key: TradeKey; desc: boolean }>({ key: 'index', desc: false });
  const [page, setPage] = useState(0);
  const uniq = (f: (t: BacktestTrade) => string) => [...new Set(all.map(f))].sort();
  const list = useMemo(() => {
    const v = (t: BacktestTrade): number | string =>
      sort.key === 'open' ? sec(t.openTime) : sort.key === 'index' ? t.index : (t[sort.key] as number | string);
    const out = filterTrades(all, filter);
    out.sort((a, b) => {
      const x = v(a);
      const y = v(b);
      const c = typeof x === 'string' ? x.localeCompare(String(y)) : x - (y as number);
      return sort.desc ? -c : c;
    });
    return out;
  }, [all, filter, sort]);
  const pages = Math.max(1, Math.ceil(list.length / PAGE));
  const p = Math.min(page, pages - 1);
  const net = list.reduce((a, t) => a + t.net, 0);
  const wins = list.filter((t) => t.net > 0).length;
  const head = (key: TradeKey, label: string, end = false) => (
    <Table.ColumnHeader
      textAlign={end ? 'end' : undefined}
      cursor="pointer"
      userSelect="none"
      onClick={() => setSort((s) => ({ key, desc: s.key === key ? !s.desc : false }))}
    >
      <HStack gap="1" justify={end ? 'flex-end' : undefined}>
        {label}
        {sort.key === key && (sort.desc ? <ChevronDownIcon size={12} /> : <ChevronUpIcon size={12} />)}
      </HStack>
    </Table.ColumnHeader>
  );
  const set = (k: keyof TradeFilter) => (v: string) => {
    setPage(0);
    onFilter({ ...filter, [k]: v });
  };
  return (
    <Stack gap="2" data-testid="trades-table">
      <HStack wrap="wrap" gap="2">
        <Select label="Side" value={filter.side} options={['buy', 'sell']} onChange={set('side')} />
        <Select label="Result" value={filter.result} options={['win', 'loss']} onChange={set('result')} />
        <Select label="Session" value={filter.session} options={uniq((t) => t.session)} onChange={set('session')} />
        <Select label="Month" value={filter.month} options={uniq((t) => t.month)} onChange={set('month')} />
        <Select label="Reason" value={filter.reason} options={uniq((t) => t.reason)} onChange={set('reason')} />
        <Text textStyle="xs" color="fg.muted">
          {list.length} trades, {list.length ? pct((100 * wins) / list.length) : '-'} won, net {money(net)}
        </Text>
      </HStack>
      <Table.ScrollArea borderWidth="1px" rounded="md">
        <Table.Root size="sm" interactive>
          <Table.Header>
            <Table.Row>
              {head('index', '#', true)}
              {head('open', 'Open (WIB)')}
              {head('side', 'Side')}
              <Table.ColumnHeader textAlign="end">Entry</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Exit</Table.ColumnHeader>
              {head('net', 'Net', true)}
              {head('r', 'R', true)}
              {head('reason', 'Reason')}
              {head('session', 'Session')}
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {list.slice(p * PAGE, (p + 1) * PAGE).map((t) => (
              <Table.Row
                key={t.index}
                cursor="pointer"
                bg={selected?.index === t.index ? 'bg.emphasized' : undefined}
                onClick={() => onSelect(t)}
                data-testid="trade-row"
              >
                <Table.Cell textAlign="end">{t.index + 1}</Table.Cell>
                <Table.Cell title={utc(sec(t.openTime))}>{wib(sec(t.openTime))}</Table.Cell>
                <Table.Cell>
                  <Badge size="sm" colorPalette={t.side === 'buy' ? 'green' : 'red'}>
                    {t.side}
                  </Badge>
                </Table.Cell>
                <Table.Cell textAlign="end">{price(t.entry)}</Table.Cell>
                <Table.Cell textAlign="end">{price(t.exit)}</Table.Cell>
                <Table.Cell textAlign="end" color={t.net > 0 ? 'fg.success' : 'fg.error'}>
                  {money(t.net)}
                </Table.Cell>
                <Table.Cell textAlign="end">{t.r.toFixed(2)}</Table.Cell>
                <Table.Cell>{t.reason}</Table.Cell>
                <Table.Cell>{t.session}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Root>
      </Table.ScrollArea>
      {pages > 1 && (
        <HStack justify="flex-end" gap="2">
          <Button size="xs" variant="outline" disabled={p === 0} onClick={() => setPage(p - 1)}>
            Previous
          </Button>
          <Text textStyle="xs">
            Page {p + 1} of {pages}
          </Text>
          <Button size="xs" variant="outline" disabled={p >= pages - 1} onClick={() => setPage(p + 1)}>
            Next
          </Button>
        </HStack>
      )}
    </Stack>
  );
}

const kindColor: Record<string, string> = { signal: 'blue', note: 'gray', entry: 'green', exit: 'orange', stop_move: 'purple', missed: 'red' };

const detailText = (d: Record<string, number>) =>
  Object.entries(d)
    .filter(([k]) => k !== 'action')
    .map(([k, v]) => `${k} ${k === 'lots' || k === 'r' ? v : price(v)}`)
    .join(', ');

// DecisionLog shows decision log lines (decisions.jsonl): what the
// approach saw and did around the selected trade or candle.
export function DecisionLog({ lines, title, available }: { lines?: BacktestDecision[]; title: string; available: boolean }) {
  return (
    <Stack gap="2" data-testid="decision-log">
      <Text fontWeight="medium">{title}</Text>
      {!available ? (
        <Text color="fg.muted" textStyle="sm">
          This report has no decision log: it was written before San Backtest kept one (decisions.jsonl). Run the backtest again
          to get it.
        </Text>
      ) : !lines ? (
        <Text color="fg.muted" textStyle="sm">
          Click a trade or a candle to see the decisions around it.
        </Text>
      ) : lines.length === 0 ? (
        <Text color="fg.muted" textStyle="sm">
          Nothing happened there.
        </Text>
      ) : (
        <Stack gap="1" maxH="md" overflowY="auto" borderWidth="1px" rounded="md" p="2">
          {lines.map((d) => (
            <HStack key={d.line} align="start" gap="2" textStyle="xs" data-testid="decision-line">
              <Text whiteSpace="nowrap" title={utc(sec(d.time))} color="fg.muted">
                {wib(sec(d.time))}
              </Text>
              <Badge size="xs" colorPalette={kindColor[d.kind] ?? 'gray'}>
                {d.kind.replace('_', ' ')}
              </Badge>
              {d.side && <Text>{d.side}</Text>}
              <Text flex="1">
                {d.reason && <Text as="span">{d.reason}</Text>}
                {Object.keys(d.details).length > 0 && (
                  <Text as="span" color="fg.muted">
                    {d.reason ? ' · ' : ''}
                    {detailText(d.details)}
                  </Text>
                )}
              </Text>
            </HStack>
          ))}
        </Stack>
      )}
    </Stack>
  );
}
