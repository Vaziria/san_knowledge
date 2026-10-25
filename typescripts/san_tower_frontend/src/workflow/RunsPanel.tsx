import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Badge, Box, Button, Code, Flex, HStack, Stack, Text } from '@chakra-ui/react';
import { useEffect, useState } from 'react';
import { errorText, workflows } from '../api';
import type { Run } from '../gen/san/tower/v1/workflow_pb';
import { stepKind } from './catalog';
import { LogLine, outputText, runDuration, runStatus, stepDuration } from './runs';

// RunsPanel is the editor's Runs tab: a workflow's runs, newest first,
// checking for new ones every few seconds (an active workflow runs on its
// own), and beside them the picked one. Picking one shows what each step did,
// here and on the canvas (onPick).
export function RunsPanel({
  workflowId,
  picked,
  onPick,
  reload,
}: {
  workflowId: number;
  picked?: Run;
  onPick: (run: Run) => void;
  reload: number;
}) {
  const [runs, setRuns] = useState<Run[]>();
  const [error, setError] = useState('');

  useEffect(() => {
    let stale = false;
    const load = () =>
      workflows.listRuns({ workflowId, limit: 50 }).then(
        (r) => {
          if (stale) return;
          setRuns(r.runs);
          setError('');
        },
        (err: unknown) => !stale && setError(errorText(err)),
      );
    void load();
    const t = setInterval(load, 3000);
    return () => {
      stale = true;
      clearInterval(t);
    };
  }, [workflowId, reload]);

  const pick = async (id: number) => {
    try {
      const { run } = await workflows.getRun({ id });
      onPick(run!);
    } catch (err) {
      setError(errorText(err));
    }
  };

  return (
    <Flex flex="1" minH="0" gap="3">
      <Stack w="80" flexShrink={0} borderWidth="1px" rounded="md" p="3" gap="1" overflowY="auto">
        {error && <Text color="fg.error">{error}</Text>}
        {runs === undefined && !error && <Text color="fg.muted">Loading...</Text>}
        {runs?.length === 0 && <Text color="fg.muted">No runs yet: run it by hand, or switch it on.</Text>}
        {runs?.map((r) => (
          <Button
            key={r.id}
            variant="ghost"
            size="sm"
            h="auto"
            py="1.5"
            justifyContent="flex-start"
            textAlign="start"
            bg={r.id === picked?.id ? 'bg.emphasized' : undefined}
            onClick={() => void pick(r.id)}
          >
            <Stack gap="0" minW="0" w="full">
              <HStack justify="space-between">
                <Badge size="sm" colorPalette={runStatus[r.status].palette}>
                  {runStatus[r.status].label}
                </Badge>
                <Text textStyle="xs" color="fg.muted" fontWeight="normal">
                  {runDuration(r)}
                </Text>
              </HStack>
              <Text textStyle="xs" fontWeight="normal" truncate>
                {r.trigger} · {r.startedAt && timestampDate(r.startedAt).toLocaleTimeString()}
              </Text>
            </Stack>
          </Button>
        ))}
      </Stack>
      <Box flex="1" minW="0" borderWidth="1px" rounded="md" p="3" overflowY="auto">
        {picked ? (
          <RunDetail run={picked} divided={false} />
        ) : (
          <Text color="fg.muted">Pick a run to see what each step did; the Workflow tab marks its steps too.</Text>
        )}
      </Box>
    </Flex>
  );
}

// RunDetail is what each step of a run did: its error, the event outputs it
// fired, the lines it logged and its data outputs. divided puts a line above
// it, for when it follows a list.
export function RunDetail({ run, divided = true }: { run: Run; divided?: boolean }) {
  return (
    <Stack gap="2" borderTopWidth={divided ? '1px' : undefined} pt={divided ? '3' : undefined}>
      <HStack>
        <Badge colorPalette={runStatus[run.status].palette}>{runStatus[run.status].label}</Badge>
        <Text textStyle="sm" truncate>
          Run {run.id} · {run.trigger}
        </Text>
      </HStack>
      {run.error && (
        <Text textStyle="sm" color="fg.error">
          {run.error}
        </Text>
      )}
      {run.steps.map((s, i) => (
        <Box key={i} borderWidth="1px" rounded="md" p="2" fontSize="xs">
          <HStack justify="space-between">
            <Text fontWeight="semibold">{stepKind(s.kind)?.title ?? s.kind}</Text>
            <Text color="fg.muted">{stepDuration(s)}</Text>
          </HStack>
          {s.error && <Text color="fg.error">{s.error}</Text>}
          {s.fired.length > 0 && <Text color="fg.muted">Fired {s.fired.join(', ')}</Text>}
          {s.logs.map((l, j) => (
            <LogLine key={j} text={l} level={s.logLevels[j]} display="block" mt="1" />
          ))}
          {Object.entries(s.outputs).map(([name, value]) => (
            <Box key={name} mt="1">
              <Text color="fg.muted">{name}</Text>
              <Code display="block" whiteSpace="pre-wrap" maxH="40" overflow="auto">
                {outputText(value)}
              </Code>
            </Box>
          ))}
        </Box>
      ))}
    </Stack>
  );
}
