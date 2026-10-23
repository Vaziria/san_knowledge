import { Badge, Box, Button, HStack, Heading, Stack, Text } from '@chakra-ui/react';
import { useEffect, useState } from 'react';
import { type LogEntry, loadLog } from '../log';
import { type Status, loadStatus } from '../settings';
import { StatusBadge } from '../ui';

type OnChanged = Parameters<typeof chrome.storage.onChanged.addListener>[0];

// The debug panel: every command the server sent, newest first, with its
// reply, live as they arrive.
export function App() {
  const [status, setStatus] = useState<Status>();
  const [log, setLog] = useState<LogEntry[]>([]);

  useEffect(() => {
    void loadStatus().then(setStatus);
    void loadLog().then(setLog);
    const onChanged: OnChanged = (changes, area) => {
      if (area !== 'session') return;
      if (changes.status) setStatus(changes.status.newValue as Status);
      if (changes.log) setLog((changes.log.newValue as LogEntry[] | undefined) ?? []);
    };
    chrome.storage.onChanged.addListener(onChanged);
    return () => chrome.storage.onChanged.removeListener(onChanged);
  }, []);

  return (
    <Stack gap="3" p="3">
      <HStack justify="space-between">
        <Heading size="sm">San Remote commands</Heading>
        <StatusBadge status={status} />
      </HStack>
      <HStack justify="space-between" gap="2">
        <Text fontSize="xs" color="fg.muted">
          {statusLine(status)}
        </Text>
        <Button size="xs" variant="outline" disabled={log.length === 0} onClick={() => void chrome.runtime.sendMessage('clear-log')}>
          Clear
        </Button>
      </HStack>
      {log.length === 0 && (
        <Text fontSize="sm" color="fg.muted">
          No commands yet. They appear here as the server sends them.
        </Text>
      )}
      {[...log].reverse().map((e) => (
        <Entry key={e.id} entry={e} />
      ))}
    </Stack>
  );
}

function statusLine(s?: Status): string {
  if (!s) return '';
  if (s.state === 'connected') return `${s.server} · last ping ${s.lastPing ? time(s.lastPing) : 'none yet'}`;
  return s.error ? `${s.server} · ${s.error}` : s.server;
}

function Entry({ entry: e }: { entry: LogEntry }) {
  const color = e.state === 'error' ? 'red' : e.state === 'ok' ? 'green' : 'yellow';
  return (
    <Box borderWidth="1px" borderRadius="md" p="2">
      <Stack gap="1.5">
        <HStack gap="2" fontSize="xs">
          <Badge colorPalette="blue">{e.kind}</Badge>
          <Badge colorPalette={color}>
            {e.state}
            {e.ms !== undefined && ` · ${e.ms} ms`}
          </Badge>
          <Text color="fg.muted" ml="auto">
            {time(e.at)}
          </Text>
        </HStack>
        {e.summary && (
          <Text fontSize="xs" truncate title={e.summary}>
            {e.summary}
          </Text>
        )}
        {e.request && <Block label={e.kind === 'exec' ? 'code' : 'request'} text={e.request} />}
        {e.reply !== undefined && (
          <Block label={e.state === 'error' ? 'error' : e.kind === 'exec' ? 'result' : 'reply'} text={e.reply} error={e.state === 'error'} />
        )}
      </Stack>
    </Box>
  );
}

function Block({ label, text, error }: { label: string; text: string; error?: boolean }) {
  return (
    <Box>
      <Text fontSize="2xs" color="fg.muted" textTransform="uppercase">
        {label}
      </Text>
      <Box
        as="pre"
        fontSize="xs"
        fontFamily="mono"
        whiteSpace="pre-wrap"
        wordBreak="break-word"
        maxH="200px"
        overflow="auto"
        bg="bg.muted"
        color={error ? 'fg.error' : undefined}
        p="2"
        borderRadius="sm"
      >
        {text}
      </Box>
    </Box>
  );
}

function time(ms: number): string {
  return new Date(ms).toLocaleTimeString();
}
