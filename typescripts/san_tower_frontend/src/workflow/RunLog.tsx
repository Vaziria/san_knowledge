import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Badge, Box, Button, ButtonGroup, HStack, Link, Stack, Switch, Text } from '@chakra-ui/react';
import { ChevronDownIcon, ChevronRightIcon, PauseIcon, PlayIcon } from 'lucide-react';
import { memo, useCallback, useEffect, useRef, useState } from 'react';
import { Link as RouterLink } from 'react-router';
import { errorText, workflows } from '../api';
import { RunStatus, type Run } from '../gen/san/tower/v1/workflow_pb';
import { usePoll } from '../poll';
import { loadStepKinds } from './catalog';
import { LogLine, runDuration, runStatus } from './runs';
import { RunDetail } from './RunsPanel';

// every is how often the log asks for new runs; shown is how many it shows.
const every = 2000;
const shown = 200;
// fresh is how long a run that just came in stays marked.
const fresh = 3000;

// The status filter's choices. Failed takes in the runs the tower stopped
// during (interrupted), as a run's row does.
const statusFilters = {
  all: { label: 'All', statuses: [] },
  succeeded: { label: 'Succeeded', statuses: [RunStatus.SUCCEEDED] },
  failed: { label: 'Failed', statuses: [RunStatus.FAILED, RunStatus.INTERRUPTED] },
} satisfies Record<string, { label: string; statuses: RunStatus[] }>;
type StatusFilter = keyof typeof statusFilters;

// RunLog is the Workflow page's Logs: the runs of every workflow as they
// come, newest first, asked for every two seconds (ListRuns with no
// workflow). By default only those of the workflows switched on, which run by
// themselves, and of any status, or only those that succeeded or failed (the
// tower filters, so the list still holds the newest of those). A run shows
// its time, workflow, status, trigger and duration,
// and under it the lines its steps logged and why it failed; a run that just
// came in is marked for a moment. Clicking a run shows what each of its steps
// did. Pause stops asking, so the list stands still to read. With a
// workflowId it is the editor's Logs: that workflow's runs (ListRuns with
// logs), switched on or not, without the workflow's name on each.
//
// However much the runs log, it stays light (2026-10-27): a run comes with
// its last lines only (the tower's summary; moreLogs says how many came
// before, which opening the run shows), nothing is asked while the browser
// tab is hidden, and a run that did not change keeps its row as drawn.
export function RunLog({ workflowId = 0 }: { workflowId?: number }) {
  const [activeSwitch, setActiveOnly] = useState(true);
  const activeOnly = !workflowId && activeSwitch;
  const [status, setStatus] = useState<StatusFilter>('all');
  const [paused, setPaused] = useState(false);
  const [runs, setRuns] = useState<Run[]>();
  const [error, setError] = useState('');
  const [checked, setChecked] = useState<Date>();
  const [open, setOpen] = useState<Run>();
  // A run's steps are shown by their kinds' titles, which the tower gives.
  const [, setKindsLoaded] = useState(false);
  useEffect(() => {
    loadStepKinds().then(
      () => setKindsLoaded(true),
      () => {}, // the steps show their kinds' names instead
    );
  }, []);
  // The runs seen so far, and when the new ones came in.
  const seen = useRef<Set<number> | null>(null);
  const [arrived, setArrived] = useState<Map<number, number>>(new Map());
  // The runs as last drawn, by id, with what tells that one changed.
  const drawn = useRef(new Map<number, { sig: string; run: Run }>());

  useEffect(() => {
    seen.current = null; // a new filter marks nothing as new
  }, [workflowId, activeOnly, status]);
  usePoll(
    (stale) =>
      workflows.listRuns({ workflowId, logs: true, activeOnly, statuses: statusFilters[status].statuses, limit: shown }).then(
        (r) => {
          if (stale()) return;
          const now = Date.now();
          if (seen.current) {
            const known = seen.current;
            const added = r.runs.filter((x) => !known.has(x.id));
            if (added.length) setArrived((m) => new Map([...m, ...added.map((x) => [x.id, now] as const)]));
          }
          seen.current = new Set(r.runs.map((x) => x.id));
          setArrived((m) => (m.size ? new Map([...m].filter(([, at]) => now - at < fresh)) : m));
          // A run that did not change stays the same object, so its row is
          // not drawn again; the list is set only when one did.
          const before = drawn.current;
          const ids = [...before.keys()];
          const next = new Map<number, { sig: string; run: Run }>();
          let changed = before.size !== r.runs.length;
          const list = r.runs.map((x, i) => {
            const sig = runSig(x);
            const old = before.get(x.id);
            const run = old?.sig === sig ? old.run : x;
            if (run !== old?.run || ids[i] !== x.id) changed = true;
            next.set(x.id, { sig, run });
            return run;
          });
          drawn.current = next;
          if (changed) setRuns(list);
          setChecked(new Date());
          setError('');
        },
        (err: unknown) => !stale() && setError(errorText(err)),
      ),
    every,
    [workflowId, activeOnly, status],
    !paused,
  );
  const toggle = useCallback((r: Run) => setOpen((o) => (o?.id === r.id ? undefined : r)), []);

  // The open run's steps, again when its status changes (it finished).
  const openStatus = runs?.find((r) => r.id === open?.id)?.status;
  useEffect(() => {
    if (!open) return;
    workflows.getRun({ id: open.id }).then(
      (r) => setOpen(r.run),
      (err: unknown) => setError(errorText(err)),
    );
  }, [open?.id, openStatus]);

  return (
    <Stack gap="3">
      <HStack justify="space-between" wrap="wrap" gap="3">
        <HStack gap="4">
          {!workflowId && (
            <Switch.Root
              size="sm"
              colorPalette="green"
              checked={activeSwitch}
              onCheckedChange={(e) => setActiveOnly(e.checked)}
            >
              <Switch.HiddenInput />
              <Switch.Control>
                <Switch.Thumb />
              </Switch.Control>
              <Switch.Label>Active workflows only</Switch.Label>
            </Switch.Root>
          )}
          <ButtonGroup size="xs" variant="outline" attached>
            {Object.entries(statusFilters).map(([key, f]) => (
              <Button
                key={key}
                variant={status === key ? 'solid' : 'outline'}
                onClick={() => setStatus(key as StatusFilter)}
              >
                {f.label}
              </Button>
            ))}
          </ButtonGroup>
          <Text textStyle="sm" color="fg.muted">
            {runs ? `${runs.length}${runs.length === shown ? '+' : ''} runs` : ''}
          </Text>
        </HStack>
        <HStack gap="3">
          <HStack gap="1.5" textStyle="xs" color="fg.muted">
            <Box w="2" h="2" rounded="full" bg={paused || error ? 'fg.subtle' : 'green.solid'} />
            {paused ? 'Paused' : error ? 'Not answering' : `Live · ${checked?.toLocaleTimeString() ?? ''}`}
          </HStack>
          <Button size="xs" variant="outline" onClick={() => setPaused((p) => !p)}>
            {paused ? <PlayIcon /> : <PauseIcon />}
            {paused ? 'Resume' : 'Pause'}
          </Button>
        </HStack>
      </HStack>
      {error && <Text color="fg.error">The tower does not answer: {error}</Text>}
      {runs?.length === 0 && (
        <Text color="fg.muted">
          {status !== 'all'
            ? `No ${statusFilters[status].label.toLowerCase()} runs${activeOnly ? ' of active workflows' : ''}.`
            : activeOnly
              ? 'No runs of active workflows yet. Switch a workflow on (Workflows tab), and its runs show here as they happen.'
              : workflowId
                ? 'No runs yet: run it by hand, or switch it on, and its runs show here as they happen.'
                : 'No runs yet.'}
        </Text>
      )}
      {runs && runs.length > 0 && (
        <Stack gap="0" borderWidth="1px" rounded="md" overflow="hidden">
          {runs.map((r) => (
            <RunRow
              key={r.id}
              run={r}
              named={!workflowId}
              isNew={arrived.has(r.id)}
              open={open?.id === r.id ? open : undefined}
              onToggle={toggle}
            />
          ))}
        </Stack>
      )}
    </Stack>
  );
}

// runSig is what tells that a run of the list changed: its status, its end,
// and its lines.
function runSig(r: Run): string {
  return `${r.status}|${r.finishedAt?.seconds ?? ''}|${r.moreLogs}|${r.logs.length}|${r.logs.at(-1) ?? ''}|${r.error}`;
}

// RunRow is one run of the log; named shows its workflow's name. It is drawn
// again only when its props change.
const RunRow = memo(function RunRow({
  run,
  named,
  isNew,
  open,
  onToggle,
}: {
  run: Run;
  named: boolean;
  isNew: boolean;
  open?: Run;
  onToggle: (run: Run) => void;
}) {
  const status = runStatus[run.status];
  const failed = run.status === RunStatus.FAILED || run.status === RunStatus.INTERRUPTED;
  const Chevron = open ? ChevronDownIcon : ChevronRightIcon;
  return (
    <Box
      borderBottomWidth="1px"
      _last={{ borderBottomWidth: 0 }}
      bg={isNew ? 'bg.muted' : undefined}
      transition="background 0.6s"
    >
      <HStack px="3" py="2" gap="3" cursor="pointer" _hover={{ bg: 'bg.subtle' }} onClick={() => onToggle(run)}>
        <Chevron size="14" color="var(--chakra-colors-fg-subtle)" />
        <Text fontFamily="mono" textStyle="xs" color="fg.muted" flexShrink={0}>
          {run.startedAt && timestampDate(run.startedAt).toLocaleTimeString()}
        </Text>
        <Badge size="sm" colorPalette={status.palette} flexShrink={0} w="20" justifyContent="center">
          {status.label}
        </Badge>
        {named && (
          <Link asChild fontWeight="semibold" textStyle="sm" flexShrink={0} onClick={(e) => e.stopPropagation()}>
            <RouterLink to={`/workflow/${run.workflowId}`}>{run.workflowName || `Workflow ${run.workflowId}`}</RouterLink>
          </Link>
        )}
        <Text textStyle="sm" color="fg.muted" truncate flex="1">
          {run.trigger}
        </Text>
        <Text textStyle="xs" color="fg.muted" flexShrink={0}>
          {runDuration(run)}
        </Text>
      </HStack>
      {(run.logs.length > 0 || (failed && run.error)) && (
        <Stack gap="0.5" ms="12" me="3" mb="2" ps="3" borderStartWidth="2px" borderColor={`${status.palette}.muted`}>
          {run.moreLogs > 0 && (
            <Text textStyle="xs" color="fg.muted">
              {run.moreLogs} earlier {run.moreLogs === 1 ? 'line' : 'lines'}: open the run to see them
            </Text>
          )}
          {run.logs.map((l, i) => (
            <LogLine key={i} text={l} level={run.logLevels[i]} variant="plain" textStyle="xs" px="0" />
          ))}
          {failed && run.error && (
            <Text textStyle="xs" color="fg.error">
              {run.error}
            </Text>
          )}
        </Stack>
      )}
      {open && (
        <Box px="3" pb="3" ms="9">
          <RunDetail run={open} />
        </Box>
      )}
    </Box>
  );
});
