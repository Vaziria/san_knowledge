import { Badge, Box, Grid, HStack, Heading, Stack, Table, Tabs, Text } from '@chakra-ui/react';
import { ArrowLeftIcon } from 'lucide-react';
import { useMemo, useState } from 'react';
import { Link, useParams } from 'react-router';
import { EquityChart } from '../backtest/EquityChart';
import { chartColors, money, price, sec, signed, utc, wib } from '../backtest/format';
import { DecisionLog, noFilter, TradesTable, type TradeFilter } from '../backtest/RunParts';
import type { BacktestCandle, BacktestDecision, BacktestEquityPoint, BacktestTrade } from '../gen/san/tower/v1/backtest_analysis_pb';
import type { LivePaperSession, LiveWatchPaperSessionResponse } from '../gen/san/tower/v1/live_trading_pb';
import { liveTrading } from '../api';
import { StreamIndicator, useStream } from '../stream';
import { LiveChart, type Level } from './LiveChart';
import { PaperBadge, SessionActions, SessionStatus } from './LiveTrading';
import { useMonitor } from './MonitorPanel';
import { PluginHint } from './PluginsPanel';

type Journal = { trades: BacktestTrade[]; decisions: BacktestDecision[]; equity: BacktestEquityPoint[]; candles: BacktestCandle[] };

const empty: Journal = { trades: [], decisions: [], equity: [], candles: [] };

// PaperSessionPage is one paper session live (/live-trading/session/:id):
// its chart (the candles it ran, and the one forming from the ticks), its
// open positions and orders, trades, decision log and equity, reusing
// Backtest Analysis's parts; each step adds to them as it happens.
export function PaperSessionPage() {
  const id = Number(useParams().id);
  const [session, setSession] = useState<LivePaperSession>();
  const [journal, setJournal] = useState<Journal>(empty);
  const [gone, setGone] = useState(false);
  const [filter, setFilter] = useState<TradeFilter>(noFilter);
  const [selected, setSelected] = useState<BacktestTrade>();
  const [message, setMessage] = useState('');
  const stream = useStream<LiveWatchPaperSessionResponse>(
    (signal) => liveTrading.watchPaperSession({ id, decisions: 300, candles: 1500 }, { signal }),
    (m) => {
      const ev = m.event;
      if (ev.case === 'snapshot') {
        setSession(ev.value.session);
        const j = ev.value.journal;
        setJournal({ trades: j?.trades ?? [], decisions: j?.decisions ?? [], equity: j?.equity ?? [], candles: j?.candles ?? [] });
      } else if (ev.case === 'update') {
        setSession(ev.value.session);
        const j = ev.value.journal;
        if (!j) return;
        setJournal((old) => {
          // candles by open time, each once, in order (the chart needs it)
          const byTime = new Map(old.candles.map((c) => [Number(c.time), c]));
          for (const c of j.candles) byTime.set(Number(c.time), c);
          const candles = [...byTime.values()].sort((a, b) => Number(a.time) - Number(b.time)).slice(-3000);
          return {
            trades: [...old.trades, ...j.trades],
            decisions: [...old.decisions, ...j.decisions].slice(-1000),
            equity: [...old.equity, ...j.equity],
            candles,
          };
        });
      } else if (ev.case === 'removed') setGone(true);
    },
    [id],
    { onEnd: () => setGone(true) },
  );
  const instrument = session?.settings?.instrument ?? '';
  const step = session?.timeframes[0] ?? 'M15';
  const instruments = useMemo(() => (instrument ? [instrument] : []), [instrument]);
  const { ticks } = useMonitor(session?.settings?.connectionId ?? 0, instruments);
  const tick = ticks[instrument];
  const going = session && session.status !== 'stopped' && session.status !== 'error';

  const decisions = useMemo(
    () => [...journal.decisions].reverse().map((d, i) => ({ ...d, line: i + 1 }) as BacktestDecision),
    [journal.decisions],
  );
  const trades = useMemo(() => journal.trades.map((t, i) => ({ ...t, index: i }) as BacktestTrade), [journal.trades]);
  const levels = useMemo<Level[]>(
    () =>
      (session?.positions ?? []).flatMap((p) => [
        { price: p.entry, color: 'entry' as const, title: `${p.side} #${p.id}` },
        { price: p.stop, color: 'stop' as const, title: 'stop' },
        { price: p.target, color: 'target' as const, title: 'target' },
      ]),
    [session],
  );
  const equity = useMemo(() => {
    let peak = -Infinity;
    const pts = journal.equity.map((p) => {
      peak = Math.max(peak, p.equity);
      return { time: sec(p.time), value: p.equity, dd: peak > 0 ? (100 * (peak - p.equity)) / peak : 0 };
    });
    return {
      curves: [{ name: 'equity', color: chartColors().equity, points: pts.map((p) => ({ time: p.time, value: p.value })) }],
      dd: pts.map((p) => ({ time: p.time, value: p.dd })),
    };
  }, [journal.equity]);

  if (gone && !session) return <Text color="fg.error">This paper session does not exist (deleted?).</Text>;
  if (!session) return <Text color="fg.muted">Reading the session...</Text>;
  const st = session.settings!;
  return (
    <Stack gap="4">
      <Link to="/live-trading">
        <HStack gap="1" color="fg.muted" textStyle="sm" _hover={{ color: 'fg' }}>
          <ArrowLeftIcon size={14} /> Live Trading
        </HStack>
      </Link>
      <HStack justify="space-between" wrap="wrap" gap="3">
        <HStack gap="2" wrap="wrap">
          <Heading size="lg">{st.name}</Heading>
          <PaperBadge />
          <SessionStatus s={session} />
          <PluginHint s={session} />
          <Text color="fg.muted" textStyle="sm">
            {st.approach} · {st.instrument} {session.timeframes.join(' / ')} · balance {money(st.balance)}, risk {st.risk}%, 1:{st.leverage}
          </Text>
        </HStack>
        <HStack gap="3">
          <StreamIndicator state={stream} />
          <SessionActions s={session} onMessage={setMessage} />
        </HStack>
      </HStack>
      {session.message && (
        <Text textStyle="sm" color={session.status === 'error' ? 'fg.error' : 'fg.muted'} data-testid="session-message-line">
          {session.message}
        </Text>
      )}
      {message && <Text textStyle="sm">{message}</Text>}
      {gone && <Text color="fg.error">The session was deleted.</Text>}
      <HStack gap="5" wrap="wrap" textStyle="sm">
        <Text>
          Equity <b>{money(session.equity)}</b>
        </Text>
        <Text>
          Balance <b>{money(session.balance)}</b>
        </Text>
        <Text color={session.netProfit >= 0 ? 'fg.success' : 'fg.error'}>
          Net <b>{signed(session.netProfit)}</b>
        </Text>
        <Text>
          Trades <b data-testid="detail-trades">{session.trades}</b>
        </Text>
        <Text>
          Candles <b>{session.candles}</b>
        </Text>
        {session.from && <Text title={utc(sec(session.from))}>From {wib(sec(session.from))} WIB</Text>}
        {session.lastClose && <Text>Last candle {wib(sec(session.lastClose))}</Text>}
        {going && session.nextClose && <Text>Next close {wib(sec(session.nextClose))}</Text>}
      </HStack>
      <Grid templateColumns={{ base: '1fr', xl: 'minmax(0, 3fr) minmax(360px, 1.3fr)' }} gap="5" alignItems="start">
        <Stack gap="3" minW="0">
          <Text textStyle="xs" color="fg.muted">
            {step} candles the session ran; the candle still forming is drawn from the ticks and is never given to the approach.
          </Text>
          <LiveChart candles={journal.candles} timeframe={step} tick={going && tick ? { bid: tick.bid, time: tick.time } : undefined} levels={levels} trades={trades} />
          <EquityChart curves={equity.curves} drawdown={equity.dd} height={220} />
        </Stack>
        <Stack gap="3" minW="0">
          <Text fontWeight="medium">Open positions ({session.positions.length})</Text>
          {session.positions.length === 0 ? (
            <Text color="fg.muted" textStyle="sm">
              None.
            </Text>
          ) : (
            <Table.Root size="sm" data-testid="paper-positions">
              <Table.Header>
                <Table.Row>
                  <Table.ColumnHeader>#</Table.ColumnHeader>
                  <Table.ColumnHeader>Side</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Lots</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Entry</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Stop</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">Target</Table.ColumnHeader>
                  <Table.ColumnHeader textAlign="end">P/L</Table.ColumnHeader>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {session.positions.map((p) => (
                  <Table.Row key={p.id}>
                    <Table.Cell>{p.id}</Table.Cell>
                    <Table.Cell>{p.side}</Table.Cell>
                    <Table.Cell textAlign="end">{p.lots}</Table.Cell>
                    <Table.Cell textAlign="end">{price(p.entry)}</Table.Cell>
                    <Table.Cell textAlign="end">{price(p.stop)}</Table.Cell>
                    <Table.Cell textAlign="end">{price(p.target)}</Table.Cell>
                    <Table.Cell textAlign="end">{money(p.profit)}</Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Root>
          )}
          {session.orders.length > 0 && (
            <Stack gap="1">
              <Text fontWeight="medium">Pending orders ({session.orders.length})</Text>
              {session.orders.map((o) => (
                <Text key={o.id} textStyle="xs">
                  <Badge size="xs">{o.type}</Badge> #{o.id} {o.side} at {price(o.price)}, stop {price(o.stop)}, target {price(o.target)}
                </Text>
              ))}
            </Stack>
          )}
          <DecisionLog lines={decisions} title={`Decision log (newest first, ${decisions.length})`} available />
        </Stack>
      </Grid>
      <Tabs.Root defaultValue="trades" size="sm">
        <Tabs.List>
          <Tabs.Trigger value="trades">Trades ({trades.length})</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="trades">
          <Box>
            {trades.length === 0 ? (
              <Text color="fg.muted">No closed trades yet.</Text>
            ) : (
              <TradesTable all={trades} filter={filter} onFilter={setFilter} selected={selected} onSelect={setSelected} />
            )}
          </Box>
        </Tabs.Content>
      </Tabs.Root>
    </Stack>
  );
}
