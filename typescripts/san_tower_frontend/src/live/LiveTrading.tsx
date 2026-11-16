import { Badge, Button, Dialog, Field, Grid, HStack, Heading, Input, Portal, Stack, Table, Text } from '@chakra-ui/react';
import { PlayIcon, PlusIcon, SquareIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router';
import { errorText, liveTrading } from '../api';
import { money, pct, runLink, sec, signed, utc, wib } from '../backtest/format';
import { useSort, type SortValue } from '../backtest/sort';
import type { LiveApproach, LivePaperSession, LiveWatchPaperSessionsResponse } from '../gen/san/tower/v1/live_trading_pb';
import { StreamIndicator, useStream } from '../stream';
import { MonitorPanel } from './MonitorPanel';
import { ApproachPicker } from './ApproachPicker';
import { PluginHint, PluginsPanel } from './PluginsPanel';

export const PAPER_BADGE = 'PAPER - no orders are sent';

// statusLabel is a session's status in words.
const statusLabel: Record<string, [string, string]> = {
  running: ['running', 'green'],
  waiting: ['waiting for candle', 'blue'],
  starting: ['starting', 'orange'],
  market_closed: ['market closed', 'gray'],
  stopped: ['stopped', 'gray'],
  error: ['error', 'red'],
};

export function SessionStatus({ s }: { s: LivePaperSession }) {
  const [label, color] = statusLabel[s.status] ?? [s.status, 'gray'];
  return (
    <Badge colorPalette={color} title={s.message} data-testid="session-status">
      {label}
    </Badge>
  );
}

export function PaperBadge() {
  return (
    <Badge colorPalette="purple" variant="solid" data-testid="paper-badge" title="Paper trading: the session simulates its trades; it never sends an order to Exness">
      {PAPER_BADGE}
    </Badge>
  );
}

// useSessions follows the paper sessions (WatchPaperSessions).
export function useSessions() {
  const [list, setList] = useState<LivePaperSession[]>();
  const stream = useStream<LiveWatchPaperSessionsResponse>(
    (signal) => liveTrading.watchPaperSessions({}, { signal }),
    (m) => {
      const ev = m.event;
      if (ev.case === 'snapshot') setList(ev.value.sessions);
      else if (ev.case === 'changed') {
        const s = ev.value;
        setList((old) => {
          const rest = (old ?? []).filter((x) => x.id !== s.id);
          return [s, ...rest].sort((a, b) => b.id - a.id);
        });
      } else if (ev.case === 'removed') {
        const id = ev.value;
        setList((old) => (old ?? []).filter((x) => x.id !== id));
      }
    },
    [],
  );
  return { list, stream };
}

// sessionActions are Start, Stop, Delete (asking first), Export and Open in
// Backtest Analysis.
export function SessionActions({ s, onMessage }: { s: LivePaperSession; onMessage: (m: string) => void }) {
  const [asking, setAsking] = useState(false);
  const going = s.status !== 'stopped' && s.status !== 'error';
  const run = (f: () => Promise<unknown>) => f().catch((err) => onMessage(errorText(err)));
  return (
    <HStack gap="1" wrap="wrap">
      {going ? (
        <Button size="xs" variant="outline" onClick={() => run(() => liveTrading.stopPaperSession({ id: s.id }))} data-testid="stop">
          <SquareIcon /> Stop
        </Button>
      ) : (
        <Button size="xs" variant="outline" colorPalette="green" onClick={() => run(() => liveTrading.startPaperSession({ id: s.id }))} data-testid="start">
          <PlayIcon /> Start
        </Button>
      )}
      <Button
        size="xs"
        variant="outline"
        disabled={s.candles === 0}
        onClick={() =>
          run(async () => {
            const r = await liveTrading.exportPaperSession({ id: s.id });
            onMessage(`Exported to ${r.path} (${r.files.length} files)`);
          })
        }
        data-testid="export"
      >
        Export
      </Button>
      {s.candles > 0 && (
        <Link to={runLink(s.reportPath, 'paper:')} data-testid="open-analysis">
          <Button size="xs" variant="outline" as="span">
            Open in Backtest Analysis
          </Button>
        </Link>
      )}
      <Button size="xs" variant="ghost" colorPalette="red" onClick={() => setAsking(true)} aria-label={`Delete session ${s.id}`} data-testid="delete">
        <Trash2Icon />
      </Button>
      {asking && (
        <Dialog.Root open placement="center" onOpenChange={(e) => !e.open && setAsking(false)}>
          <Portal>
            <Dialog.Backdrop />
            <Dialog.Positioner>
              <Dialog.Content data-testid="delete-dialog">
                <Dialog.Header>
                  <Dialog.Title>Delete paper session {s.settings?.name}?</Dialog.Title>
                </Dialog.Header>
                <Dialog.Body>
                  <Text>
                    It stops, and its journal ({s.trades} trades, {s.candles} candles) is deleted from the tower. Exports already
                    written stay.
                  </Text>
                </Dialog.Body>
                <Dialog.Footer>
                  <Button size="sm" variant="outline" onClick={() => setAsking(false)}>
                    Cancel
                  </Button>
                  <Button
                    size="sm"
                    colorPalette="red"
                    onClick={() => {
                      setAsking(false);
                      void run(() => liveTrading.deletePaperSession({ id: s.id }));
                    }}
                    data-testid="confirm-delete"
                  >
                    Delete
                  </Button>
                </Dialog.Footer>
              </Dialog.Content>
            </Dialog.Positioner>
          </Portal>
        </Dialog.Root>
      )}
    </HStack>
  );
}

type SessionKey = 'id' | 'status' | 'approach' | 'instrument' | 'balance' | 'equity' | 'netProfit' | 'trades' | 'lastSignalAt';
const sessionValue = (s: LivePaperSession, k: SessionKey): SortValue =>
  k === 'approach' ? (s.settings?.approach ?? '') : k === 'instrument' ? (s.settings?.instrument ?? '') : k === 'lastSignalAt' ? sec(s.lastSignalAt) : s[k];
const sessionDescFirst = (k: string) => ['balance', 'equity', 'netProfit', 'trades', 'id'].includes(k);

function SessionsTable({ list, onMessage }: { list: LivePaperSession[]; onMessage: (m: string) => void }) {
  const { sorted, header } = useSort(list, { key: 'id', desc: true }, sessionValue, sessionDescFirst);
  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm" data-testid="sessions-table">
        <Table.Header>
          <Table.Row>
            {header('id', 'Session')}
            {header('status', 'Status')}
            {header('approach', 'Approach')}
            {header('instrument', 'Instrument')}
            {header('balance', 'Balance', true)}
            {header('equity', 'Equity', true)}
            {header('netProfit', 'P/L', true)}
            {header('trades', 'Trades', true)}
            {header('lastSignalAt', 'Last signal')}
            <Table.ColumnHeader />
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {sorted.map((s) => (
            <Table.Row key={s.id} data-testid="session-row" data-session={s.id}>
              <Table.Cell>
                <Stack gap="0.5">
                  <Link to={`/live-trading/session/${s.id}`}>
                    <Text fontWeight="medium" _hover={{ textDecoration: 'underline' }}>
                      {s.settings?.name}
                    </Text>
                  </Link>
                  <Badge size="xs" colorPalette="purple" w="fit-content">
                    PAPER
                  </Badge>
                </Stack>
              </Table.Cell>
              <Table.Cell>
                <Stack gap="0.5">
                  <SessionStatus s={s} />
                  {s.message && (
                    <Text textStyle="2xs" color="fg.muted" maxW="56" lineClamp={2} title={s.message}>
                      {s.message}
                    </Text>
                  )}
                </Stack>
              </Table.Cell>
              <Table.Cell>
                {s.settings?.approach}
                <Text as="span" color="fg.muted">
                  {' '}
                  {s.timeframes[0]}
                </Text>
                <PluginHint s={s} />
              </Table.Cell>
              <Table.Cell>{s.settings?.instrument}</Table.Cell>
              <Table.Cell textAlign="end">{money(s.balance)}</Table.Cell>
              <Table.Cell textAlign="end">{money(s.equity)}</Table.Cell>
              <Table.Cell textAlign="end" color={s.netProfit > 0 ? 'fg.success' : s.netProfit < 0 ? 'fg.error' : undefined}>
                {signed(s.netProfit)}
                {s.settings && s.settings.balance > 0 && (
                  <Text as="span" color="fg.muted" textStyle="xs">
                    {' '}
                    ({pct((100 * s.netProfit) / s.settings.balance, 2)})
                  </Text>
                )}
              </Table.Cell>
              <Table.Cell textAlign="end" data-testid="session-trades">
                {s.trades}
                {s.openPositions > 0 && (
                  <Text as="span" color="fg.muted" textStyle="xs">
                    {' '}
                    +{s.openPositions} open
                  </Text>
                )}
              </Table.Cell>
              <Table.Cell title={s.lastSignalAt ? utc(sec(s.lastSignalAt)) : ''}>
                {s.lastSignalAt ? (
                  <Stack gap="0">
                    <Text textStyle="xs">{wib(sec(s.lastSignalAt))}</Text>
                    <Text textStyle="2xs" color="fg.muted" maxW="64" lineClamp={2} title={s.lastSignal}>
                      {s.lastSignal}
                    </Text>
                  </Stack>
                ) : (
                  '-'
                )}
              </Table.Cell>
              <Table.Cell>
                <SessionActions s={s} onMessage={onMessage} />
              </Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  );
}

// NewSession is the New session dialog: approach, instrument, balance,
// risk, leverage, max open and the approach's params with their defaults.
// Costs are Exness's real ones.
function NewSession({ onClose, onMessage }: { onClose: () => void; onMessage: (m: string) => void }) {
  const [approaches, setApproaches] = useState<LiveApproach[]>([]);
  const [name, setName] = useState('');
  const [approach, setApproach] = useState('');
  const [instrument, setInstrument] = useState('XAUUSD');
  const [balance, setBalance] = useState('10000');
  const [risk, setRisk] = useState('1');
  const [leverage, setLeverage] = useState('2000');
  const [maxOpen, setMaxOpen] = useState('0');
  const [params, setParams] = useState<Record<string, string>>({});
  const [error, setError] = useState('');
  useEffect(() => {
    liveTrading
      .listApproaches({})
      .then((r) => {
        setApproaches(r.approaches);
        const first = r.approaches.find((a) => a.name === 'pullback_trend') ?? r.approaches[0];
        if (first) pick(first);
      })
      .catch((err) => setError(errorText(err)));
  }, []);
  const pick = (a: LiveApproach) => {
    setApproach(a.name);
    setParams(Object.fromEntries(Object.entries(a.params).map(([k, v]) => [k, String(v)])));
  };
  const cur = approaches.find((a) => a.name === approach);
  const create = async () => {
    setError('');
    try {
      const p: Record<string, number> = {};
      for (const [k, v] of Object.entries(params)) {
        const n = Number(v);
        if (!Number.isFinite(n)) throw new Error(`param ${k}: ${v} is not a number`);
        p[k] = n;
      }
      await liveTrading.createPaperSession({
        settings: { name, approach, instrument, balance: Number(balance), risk: Number(risk), leverage: Number(leverage), maxOpen: Number(maxOpen), params: p },
      });
      onMessage(`Created ${name || `${approach} on ${instrument}`}: start it from the list.`);
      onClose();
    } catch (err) {
      setError(errorText(err));
    }
  };
  const num = (label: string, value: string, set: (v: string) => void, hint?: string) => (
    <Field.Root>
      <Field.Label textStyle="xs">{label}</Field.Label>
      <Input size="sm" value={value} onChange={(e) => set(e.currentTarget.value)} inputMode="decimal" aria-label={label} />
      {hint && <Field.HelperText textStyle="2xs">{hint}</Field.HelperText>}
    </Field.Root>
  );
  return (
    <Dialog.Root open placement="center" size="lg" onOpenChange={(e) => !e.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content data-testid="new-session">
            <Dialog.Header>
              <Dialog.Title>New paper session</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap="3">
                <PaperBadge />
                <Grid templateColumns="1fr 1fr" gap="3">
                  <Field.Root>
                    <Field.Label textStyle="xs">Approach</Field.Label>
                    <ApproachPicker approaches={approaches} value={approach} onPick={pick} />
                    {cur && (
                      <Field.HelperText textStyle="2xs">
                        steps on {cur.timeframes[0]} closes; {cur.warmup} warm-up candles; {cur.maxOpen > 1 ? `${cur.maxOpen} positions at once` : 'one position at a time'}
                        {cur.plugin && `; from the plugin ${cur.plugin}`}
                      </Field.HelperText>
                    )}
                  </Field.Root>
                  <Field.Root>
                    <Field.Label textStyle="xs">Instrument</Field.Label>
                    <Input size="sm" value={instrument} onChange={(e) => setInstrument(e.currentTarget.value.toUpperCase())} aria-label="Instrument" />
                  </Field.Root>
                  {num('Balance', balance, setBalance, 'USD, simulated')}
                  {num('Risk %', risk, setRisk, 'of the balance per trade')}
                  {num('Leverage', leverage, setLeverage)}
                  {num('Max open', maxOpen, setMaxOpen, "0: the approach's own")}
                  <Field.Root gridColumn="span 2">
                    <Field.Label textStyle="xs">Name</Field.Label>
                    <Input size="sm" value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder={`${approach} on ${instrument}`} aria-label="Name" />
                  </Field.Root>
                </Grid>
                {Object.keys(params).length > 0 && (
                  <Stack gap="1">
                    <Text textStyle="xs" fontWeight="medium">
                      Params (defaults from the approach)
                    </Text>
                    <Grid templateColumns="repeat(3, 1fr)" gap="2">
                      {Object.keys(params)
                        .sort()
                        .map((k) => (
                          <Field.Root key={k}>
                            <Field.Label textStyle="2xs">{k}</Field.Label>
                            <Input size="xs" value={params[k]} onChange={(e) => setParams({ ...params, [k]: e.currentTarget.value })} aria-label={`param ${k}`} />
                          </Field.Root>
                        ))}
                    </Grid>
                  </Stack>
                )}
                <Text textStyle="xs" color="fg.muted">
                  Costs are Exness's real ones (each candle's spread, the instrument's commission and swap). The session starts with
                  the candle forming when you press Start, and steps on each closed candle.
                </Text>
                {error && <Text color="fg.error">{error}</Text>}
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Button size="sm" variant="outline" onClick={onClose}>
                Cancel
              </Button>
              <Button size="sm" colorPalette="blue" onClick={() => void create()} disabled={!approach || !instrument} data-testid="create-session">
                Create
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

// LiveTrading is the Live Trading page (the user, 2026-11-15: "realtime
// trading menu in tower", named "Live Trading"; first stage "Monitor +
// Signals (paper trading)"): the read-only monitor of the Exness
// connection, and the paper sessions, which never send an order.
export function LiveTrading() {
  const { list, stream } = useSessions();
  const [adding, setAdding] = useState(false);
  const [message, setMessage] = useState('');
  const running = useMemo(() => (list ?? []).filter((s) => s.status !== 'stopped' && s.status !== 'error').length, [list]);
  return (
    <Stack gap="6">
      <HStack justify="space-between" wrap="wrap">
        <Heading size="lg">Live Trading</Heading>
        <Text textStyle="sm" color="fg.muted">
          Stage one: monitor and paper trading. Nothing here places, changes or closes an order.
        </Text>
      </HStack>
      <MonitorPanel />
      <Stack gap="3" data-testid="paper-panel">
        <HStack justify="space-between" wrap="wrap" gap="3">
          <HStack gap="3" wrap="wrap">
            <Text fontWeight="semibold">Paper sessions</Text>
            <PaperBadge />
            {list && (
              <Text textStyle="sm" color="fg.muted">
                {list.length} sessions, {running} going
              </Text>
            )}
          </HStack>
          <HStack gap="3">
            <StreamIndicator state={stream} />
            <Button size="sm" colorPalette="blue" onClick={() => setAdding(true)} data-testid="new-session-button">
              <PlusIcon /> New session
            </Button>
          </HStack>
        </HStack>
        <Text textStyle="xs" color="fg.muted">
          A session runs a San Backtest approach on the closed candles of the Exness connection, with the backtest's fills and
          costs. After a tower restart sessions stay stopped until you start them; a start replays from the session's first
          candle, so the time it was stopped is caught up.
        </Text>
        {message && (
          <Text textStyle="sm" data-testid="session-message">
            {message}
          </Text>
        )}
        {!list ? (
          <Text color="fg.muted">Reading the sessions...</Text>
        ) : list.length === 0 ? (
          <Text color="fg.muted">No paper sessions yet: make one with New session.</Text>
        ) : (
          <SessionsTable list={list} onMessage={setMessage} />
        )}
      </Stack>
      <PluginsPanel />
      {adding && <NewSession onClose={() => setAdding(false)} onMessage={setMessage} />}
    </Stack>
  );
}
