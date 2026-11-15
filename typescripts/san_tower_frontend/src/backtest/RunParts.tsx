import { Badge, Box, Button, Grid, HStack, NativeSelect, Stack, Table, Text } from '@chakra-ui/react';
import { useEffect, useMemo, useState } from 'react';
import type { BacktestDecision, BacktestRun, BacktestStats, BacktestTrade, BacktestWalkWindow } from '../gen/san/tower/v1/backtest_analysis_pb';
import { money, pct, pf, price, sec, signed, utc, wib } from './format';
import { useSort, type SortValue } from './sort';

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

type WalkKey = 'from' | 'params' | 'trainNet' | 'testTrades' | 'testNet' | 'testWinRate';

const walkValue = (w: BacktestWalkWindow, k: WalkKey): SortValue => (k === 'from' ? sec(w.testFrom) : w[k]);

// StatsPanel is the summary, the first half against the second, skipped
// signals, the walk-forward windows, costs, params and account.
export function StatsPanel({ run }: { run: BacktestRun }) {
  const s = run.stats;
  const halves = [
    ['First half', s?.firstHalf],
    ['Second half', s?.secondHalf],
  ] as const;
  const skipped = Object.entries(s?.skipped ?? {});
  const { sorted: walk, header: walkHead } = useSort(run.walk, { key: 'from', desc: false }, walkValue, (k) => k !== 'from' && k !== 'params');
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
                  {walkHead('from', 'Test from')}
                  {walkHead('params', 'Params')}
                  {walkHead('trainNet', 'Train net', true)}
                  {walkHead('testTrades', 'Test trades', true)}
                  {walkHead('testNet', 'Test net', true)}
                  {walkHead('testWinRate', 'Test win %', true)}
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {walk.map((w, i) => (
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
            {run.legs.some((l) => l.maxOpen > 1) && `, up to ${Math.max(...run.legs.map((l) => l.maxOpen))} positions at once per leg`}
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

type TradeKey =
  | 'index'
  | 'open'
  | 'close'
  | 'side'
  | 'entry'
  | 'exit'
  | 'lots'
  | 'net'
  | 'r'
  | 'swap'
  | 'reason'
  | 'session'
  | 'positionId'
  | 'orderType';

// orderKey sorts by order type, then by order number.
export const orderKey = (type: string, id: number) => `${type} ${String(id).padStart(9, '0')}`;

const tradeValue = (t: BacktestTrade, k: TradeKey): SortValue =>
  k === 'open' ? sec(t.openTime) : k === 'close' ? sec(t.closeTime) : k === 'orderType' ? orderKey(t.orderType, t.orderId) : (t[k] as number | string);

// money and size columns start with the largest first.
const tradeDescFirst = (k: TradeKey) => ['lots', 'net', 'r', 'swap'].includes(k);

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

export type TradeFilter = { side: string; result: string; session: string; month: string; reason: string; orderType: string };

export const noFilter: TradeFilter = { side: '', result: '', session: '', month: '', reason: '', orderType: '' };

// hasOrders: the report knows positions and order types (San Backtest's
// several positions and pending orders; older reports have neither).
export const hasOrders = (ts: BacktestTrade[]) => ts.some((t) => t.positionId > 0 || t.orderType !== '');

const orderColor: Record<string, string> = { market: 'gray', limit: 'cyan', stop: 'purple' };

// OrderBadge is how a position was opened: market, or the pending order's type, id and tag.
export function OrderBadge({ type, id, tag }: { type: string; id: number; tag: string }) {
  if (!type) return <>-</>;
  return (
    <HStack gap="1">
      <Badge size="sm" variant="subtle" colorPalette={orderColor[type] ?? 'gray'}>
        {type}
      </Badge>
      {id > 0 && (
        <Text as="span" color="fg.muted">
          #{id}
        </Text>
      )}
      {tag && <Text as="span">{tag}</Text>}
    </HStack>
  );
}

export function filterTrades(ts: BacktestTrade[], f: TradeFilter) {
  return ts.filter(
    (t) =>
      (!f.side || t.side === f.side) &&
      (!f.result || (f.result === 'win' ? t.net > 0 : t.net <= 0)) &&
      (!f.session || t.session === f.session) &&
      (!f.month || t.month === f.month) &&
      (!f.reason || t.reason === f.reason) &&
      (!f.orderType || t.orderType === f.orderType),
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
  const [page, setPage] = useState(0);
  const uniq = (f: (t: BacktestTrade) => string) => [...new Set(all.map(f))].sort();
  const filtered = useMemo(() => filterTrades(all, filter), [all, filter]);
  const { sorted: list, header, sort } = useSort(filtered, { key: 'index', desc: false }, tradeValue, tradeDescFirst);
  useEffect(() => setPage(0), [sort]); // a new order starts at its first page
  const pages = Math.max(1, Math.ceil(list.length / PAGE));
  const p = Math.min(page, pages - 1);
  const net = list.reduce((a, t) => a + t.net, 0);
  const wins = list.filter((t) => t.net > 0).length;
  const head = (key: TradeKey, label: string, end = false) => header(key, label, end);
  const orders = hasOrders(all);
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
        {orders && <Select label="Order" value={filter.orderType} options={uniq((t) => t.orderType)} onChange={set('orderType')} />}
        <Text textStyle="xs" color="fg.muted">
          {list.length} trades, {list.length ? pct((100 * wins) / list.length) : '-'} won, net {money(net)}
        </Text>
      </HStack>
      <Table.ScrollArea borderWidth="1px" rounded="md">
        <Table.Root size="sm" interactive>
          <Table.Header>
            <Table.Row>
              {head('index', '#', true)}
              {orders && head('positionId', 'Pos', true)}
              {head('open', 'Open (WIB)')}
              {head('side', 'Side')}
              {head('close', 'Close (WIB)')}
              {head('entry', 'Entry', true)}
              {head('exit', 'Exit', true)}
              {head('lots', 'Lots', true)}
              {head('net', 'Net', true)}
              {head('r', 'R', true)}
              {head('swap', 'Swap', true)}
              {head('reason', 'Reason')}
              {head('session', 'Session')}
              {orders && head('orderType', 'Order')}
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
                {orders && <Table.Cell textAlign="end">{t.positionId || '-'}</Table.Cell>}
                <Table.Cell title={utc(sec(t.openTime))}>{wib(sec(t.openTime))}</Table.Cell>
                <Table.Cell>
                  <Badge size="sm" colorPalette={t.side === 'buy' ? 'green' : 'red'}>
                    {t.side}
                  </Badge>
                </Table.Cell>
                <Table.Cell title={utc(sec(t.closeTime))}>{wib(sec(t.closeTime))}</Table.Cell>
                <Table.Cell textAlign="end">{price(t.entry)}</Table.Cell>
                <Table.Cell textAlign="end">{price(t.exit)}</Table.Cell>
                <Table.Cell textAlign="end">{t.lots.toFixed(2)}</Table.Cell>
                <Table.Cell textAlign="end" color={t.net > 0 ? 'fg.success' : 'fg.error'}>
                  {money(t.net)}
                </Table.Cell>
                <Table.Cell textAlign="end">{t.r.toFixed(2)}</Table.Cell>
                <Table.Cell textAlign="end">{t.swap ? money(t.swap) : '-'}</Table.Cell>
                <Table.Cell>{t.reason}</Table.Cell>
                <Table.Cell>{t.session}</Table.Cell>
                {orders && (
                  <Table.Cell>
                    <OrderBadge type={t.orderType} id={t.orderId} tag={t.tag} />
                  </Table.Cell>
                )}
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

const kindColor: Record<string, string> = {
  signal: 'blue',
  note: 'gray',
  entry: 'green',
  exit: 'orange',
  stop_move: 'purple',
  missed: 'red',
  // pending orders (San Backtest's limit and stop orders)
  order_placed: 'cyan',
  order_filled: 'teal',
  order_cancelled: 'yellow',
  order_expired: 'pink',
};

// detail keys that are numbers of positions and orders, not prices
const idKeys: Record<string, string> = { position_id: 'position', order_id: 'order', id: 'for' };

const detailText = (d: Record<string, number>) =>
  Object.entries(d)
    .filter(([k]) => k !== 'action')
    .map(([k, v]) =>
      idKeys[k]
        ? `${idKeys[k]} #${v}`
        : k === 'type'
          ? v === 2
            ? 'stop order'
            : 'limit order'
          : `${k} ${k === 'lots' || k === 'r' || k === 'bars' ? v : price(v)}`,
    )
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
                {d.kind.replaceAll('_', ' ')}
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
