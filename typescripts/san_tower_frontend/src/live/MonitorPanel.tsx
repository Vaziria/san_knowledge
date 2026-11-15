import { Badge, Box, Grid, HStack, Input, NativeSelect, Stack, Table, Text } from '@chakra-ui/react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { liveTrading, errorText } from '../api';
import { money, price, sec, utc, wib } from '../backtest/format';
import { useSort, type SortValue } from '../backtest/sort';
import type { BacktestCandle } from '../gen/san/tower/v1/backtest_analysis_pb';
import type { LiveMonitorState, LiveOrder, LivePosition, LiveWatchMonitorResponse } from '../gen/san/tower/v1/live_trading_pb';
import { StreamIndicator, useStream, type StreamState } from '../stream';
import { LiveChart, tfSeconds, type Level, type Tick } from './LiveChart';

export type Ticks = Record<string, { bid: number; ask: number; time: number }>;

// useMonitor watches an Exness connection's monitor (0: the first): its
// state (the account every ~2 s, positions and orders) and the ticks of
// instruments and of the positions.
export function useMonitor(connection: number, instruments: string[]) {
  const [state, setState] = useState<LiveMonitorState>();
  const [ticks, setTicks] = useState<Ticks>({});
  const key = instruments.join(',');
  const stream = useStream<LiveWatchMonitorResponse>(
    (signal) => liveTrading.watchMonitor({ connectionId: connection, instruments }, { signal }),
    (m) => {
      const ev = m.event;
      if (ev.case === 'state') setState(ev.value);
      else if (ev.case === 'tick') {
        const t = ev.value;
        setTicks((old) => ({ ...old, [t.instrument]: { bid: t.bid, ask: t.ask, time: sec(t.time) } }));
      }
    },
    [connection, key],
  );
  return { state, ticks, stream };
}

const statusColor: Record<string, string> = { connected: 'green', reconnecting: 'orange', connecting: 'orange', no_connection: 'red' };

export function MonitorStatus({ state, stream }: { state?: LiveMonitorState; stream: StreamState }) {
  return (
    <HStack gap="2" wrap="wrap">
      <Badge colorPalette="blue" variant="solid" data-testid="read-only-badge" title="The monitor only reads: it never places, changes or closes anything">
        read-only
      </Badge>
      {state && (
        <Badge colorPalette={statusColor[state.status] ?? 'gray'} data-testid="monitor-status" title={state.message}>
          Exness {state.status.replace('_', ' ')}
          {state.source === 'rest' && state.status !== 'connected' ? ' (REST snapshot)' : ''}
        </Badge>
      )}
      <StreamIndicator state={stream} />
    </HStack>
  );
}

function Stat({ label, value, color }: { label: string; value: string; color?: string }) {
  return (
    <Box borderWidth="1px" rounded="md" px="3" py="2" minW="32">
      <Text textStyle="xs" color="fg.muted">
        {label}
      </Text>
      <Text fontWeight="semibold" color={color} data-testid={`account-${label.toLowerCase().replace(' ', '-')}`}>
        {value}
      </Text>
    </Box>
  );
}

type PosKey = 'instrument' | 'side' | 'volume' | 'openPrice' | 'price' | 'stopLoss' | 'takeProfit' | 'profit' | 'openTime';
const posValue = (p: LivePosition, k: PosKey): SortValue => (k === 'openTime' ? sec(p.openTime) : p[k]);
const moneyFirst = (k: string) => ['volume', 'profit'].includes(k);

function PositionsTable({ positions }: { positions: LivePosition[] }) {
  const { sorted, header } = useSort(positions, { key: 'openTime', desc: false }, posValue, moneyFirst);
  if (positions.length === 0) return <Text color="fg.muted">No open positions.</Text>;
  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" data-testid="positions-table">
        <Table.Header>
          <Table.Row>
            {header('instrument', 'Instrument')}
            {header('side', 'Side')}
            {header('volume', 'Lots', true)}
            {header('openPrice', 'Open', true)}
            {header('price', 'Price', true)}
            {header('stopLoss', 'SL', true)}
            {header('takeProfit', 'TP', true)}
            {header('profit', 'P/L', true)}
            {header('openTime', 'Opened (WIB)')}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {sorted.map((p) => (
            <Table.Row key={p.id} data-testid="position-row">
              <Table.Cell>{p.instrument}</Table.Cell>
              <Table.Cell>{p.side}</Table.Cell>
              <Table.Cell textAlign="end">{p.volume}</Table.Cell>
              <Table.Cell textAlign="end">{price(p.openPrice)}</Table.Cell>
              <Table.Cell textAlign="end">{price(p.price)}</Table.Cell>
              <Table.Cell textAlign="end">{price(p.stopLoss)}</Table.Cell>
              <Table.Cell textAlign="end">{price(p.takeProfit)}</Table.Cell>
              <Table.Cell textAlign="end" color={p.profit > 0 ? 'fg.success' : p.profit < 0 ? 'fg.error' : undefined} data-testid="position-profit">
                {money(p.profit)} {p.currency}
              </Table.Cell>
              <Table.Cell title={utc(sec(p.openTime))}>{wib(sec(p.openTime))}</Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  );
}

type OrderKey = 'instrument' | 'side' | 'type' | 'volume' | 'price' | 'stopLoss' | 'takeProfit' | 'created';
const orderValue = (o: LiveOrder, k: OrderKey): SortValue => (k === 'created' ? sec(o.created) : o[k]);

function OrdersTable({ orders }: { orders: LiveOrder[] }) {
  const { sorted, header } = useSort(orders, { key: 'created', desc: false }, orderValue, moneyFirst);
  if (orders.length === 0) return <Text color="fg.muted">No pending orders.</Text>;
  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" data-testid="orders-table">
        <Table.Header>
          <Table.Row>
            {header('instrument', 'Instrument')}
            {header('side', 'Side')}
            {header('type', 'Type')}
            {header('volume', 'Lots', true)}
            {header('price', 'Price', true)}
            {header('stopLoss', 'SL', true)}
            {header('takeProfit', 'TP', true)}
            {header('created', 'Placed (WIB)')}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {sorted.map((o) => (
            <Table.Row key={o.id}>
              <Table.Cell>{o.instrument}</Table.Cell>
              <Table.Cell>{o.side}</Table.Cell>
              <Table.Cell>{o.type}</Table.Cell>
              <Table.Cell textAlign="end">{o.volume}</Table.Cell>
              <Table.Cell textAlign="end">{price(o.price)}</Table.Cell>
              <Table.Cell textAlign="end">{price(o.stopLoss)}</Table.Cell>
              <Table.Cell textAlign="end">{price(o.takeProfit)}</Table.Cell>
              <Table.Cell title={utc(sec(o.created))}>{wib(sec(o.created))}</Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  );
}

// useLiveCandles reads an instrument's latest candles, again when a new
// candle opens (the ticks draw the one forming in between).
export function useLiveCandles(connection: number, instrument: string, timeframe: string, tickTime?: number) {
  const [candles, setCandles] = useState<BacktestCandle[]>([]);
  const [error, setError] = useState('');
  const step = tfSeconds[timeframe] ?? 900;
  const bucket = tickTime ? Math.floor(tickTime / step) : 0;
  const last = useRef('');
  useEffect(() => {
    const key = `${connection}|${instrument}|${timeframe}|${bucket}`;
    if (!instrument || last.current === key) return;
    last.current = key;
    let live = true;
    liveTrading
      .getLiveCandles({ connectionId: connection, instrument, timeframe, count: 300 })
      .then((r) => {
        if (!live) return;
        setError('');
        // the forming one comes from the ticks
        setCandles(r.forming ? r.candles.slice(0, -1) : r.candles);
      })
      .catch((err) => live && setError(errorText(err)));
    return () => {
      live = false;
    };
  }, [connection, instrument, timeframe, bucket]);
  return { candles, error };
}

// MonitorPanel is the read-only monitor of the tower's Exness connection
// (the user, 2026-11-15: "1a": monitor the tower's existing Exness
// connection): the account, open positions and pending orders with live
// P/L, and a live chart.
export function MonitorPanel() {
  const [connection, setConnection] = useState(0);
  const [connections, setConnections] = useState<{ id: number; name: string }[]>([]);
  const [instrument, setInstrument] = useState('XAUUSD');
  const [typed, setTyped] = useState('XAUUSD');
  const [tf, setTf] = useState('M15');
  const instruments = useMemo(() => [instrument], [instrument]);
  const { state, ticks, stream } = useMonitor(connection, instruments);
  const tick = ticks[instrument];
  const { candles, error } = useLiveCandles(connection, instrument, tf, tick?.time);

  useEffect(() => {
    liveTrading
      .getMonitor({})
      .then((r) => setConnections(r.connections.map((c) => ({ id: c.id, name: c.name }))))
      .catch(() => {});
  }, []);

  const a = state?.account;
  const levels = useMemo<Level[]>(
    () =>
      (state?.positions ?? [])
        .filter((p) => p.instrument === instrument)
        .flatMap((p) => [
          { price: p.openPrice, color: 'entry' as const, title: `${p.side} ${p.volume}` },
          { price: p.stopLoss, color: 'stop' as const, title: 'SL' },
          { price: p.takeProfit, color: 'target' as const, title: 'TP' },
        ]),
    [state, instrument],
  );
  const liveTick: Tick | undefined = tick ? { bid: tick.bid, time: tick.time } : undefined;
  const ccy = a?.currency ?? '';
  return (
    <Stack gap="4" data-testid="monitor-panel">
      <HStack justify="space-between" wrap="wrap" gap="3">
        <HStack gap="3" wrap="wrap">
          <Text fontWeight="semibold">Monitor</Text>
          {connections.length > 1 && (
            <NativeSelect.Root size="xs" w="auto">
              <NativeSelect.Field aria-label="Exness connection" value={connection} onChange={(e) => setConnection(Number(e.currentTarget.value))}>
                <option value={0}>First connection</option>
                {connections.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
          )}
          {state?.connection?.name && (
            <Text textStyle="sm" color="fg.muted">
              {state.connection.name}
              {a?.accountId && ` · account ${a.accountId}`}
              {a?.leverage && ` · 1:${a.leverage}`}
            </Text>
          )}
        </HStack>
        <MonitorStatus state={state} stream={stream} />
      </HStack>
      {state?.status === 'no_connection' && <Text color="fg.warning">{state.message}</Text>}
      {state?.message && state.status !== 'no_connection' && (
        <Text textStyle="sm" color="fg.warning">
          {state.message}
        </Text>
      )}
      <HStack gap="3" wrap="wrap" data-testid="account">
        <Stat label="Balance" value={a ? `${money(a.balance)} ${ccy}` : '-'} />
        <Stat label="Equity" value={a ? `${money(a.equity)} ${ccy}` : '-'} color={a && a.equity < a.balance ? 'fg.error' : undefined} />
        <Stat label="Margin" value={a ? money(a.margin) : '-'} />
        <Stat label="Free margin" value={a ? money(a.freeMargin) : '-'} />
      </HStack>
      <Grid templateColumns={{ base: '1fr', xl: 'minmax(0, 1.3fr) minmax(0, 1fr)' }} gap="4" alignItems="start">
        <Stack gap="2" minW="0">
          <HStack gap="2" wrap="wrap">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                setInstrument(typed.trim().toUpperCase());
              }}
            >
              <Input size="xs" w="28" value={typed} onChange={(e) => setTyped(e.currentTarget.value)} aria-label="Chart instrument" />
            </form>
            {Object.keys(tfSeconds).map((x) => (
              <Badge key={x} cursor="pointer" variant={x === tf ? 'solid' : 'outline'} onClick={() => setTf(x)} data-testid={`tf-${x}`}>
                {x}
              </Badge>
            ))}
            {tick && (
              <Text textStyle="sm" data-testid="live-price">
                bid {price(tick.bid)} · ask {price(tick.ask)}
              </Text>
            )}
          </HStack>
          {error && <Text color="fg.error">{error}</Text>}
          <LiveChart candles={candles} timeframe={tf} tick={liveTick} levels={levels} height={380} />
        </Stack>
        <Stack gap="3" minW="0">
          <Text fontWeight="medium">Positions ({state?.positions.length ?? 0})</Text>
          <PositionsTable positions={state?.positions ?? []} />
          <Text fontWeight="medium">Pending orders ({state?.orders.length ?? 0})</Text>
          <OrdersTable orders={state?.orders ?? []} />
        </Stack>
      </Grid>
    </Stack>
  );
}
