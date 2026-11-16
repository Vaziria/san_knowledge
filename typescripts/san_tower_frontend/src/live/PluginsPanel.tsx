import { Badge, Button, HStack, Stack, Table, Text, Wrap } from '@chakra-ui/react';
import { PuzzleIcon, RefreshCwIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { errorText, liveTrading } from '../api';
import { sec, wib } from '../backtest/format';
import type { LiveListPluginsResponse, LivePaperSession } from '../gen/san/tower/v1/live_trading_pb';

// PluginsPanel lists the approach plugins in the tower's plugins folder
// (programs built with san_backtest's plugin package; the user,
// 2026-11-16: "Yes I'm thinking loading like dll plugin"): each file, the
// approaches it serves and why it cannot be used. Reload plugins reads the
// folder again; running sessions keep their own plugin process until they
// are started again.
export function PluginsPanel() {
  const [res, setRes] = useState<LiveListPluginsResponse>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => {
    liveTrading
      .listPlugins({})
      .then(setRes)
      .catch((err) => setError(errorText(err)));
  }, []);
  const reload = async () => {
    setBusy(true);
    setError('');
    try {
      setRes(await liveTrading.reloadPlugins({}));
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Stack gap="3" data-testid="plugins-panel">
      <HStack justify="space-between" wrap="wrap" gap="3">
        <HStack gap="2">
          <PuzzleIcon size={16} />
          <Text fontWeight="semibold">Plugins</Text>
          {res && (
            <Text textStyle="sm" color="fg.muted">
              {res.files.length} files
            </Text>
          )}
        </HStack>
        <Button size="sm" variant="outline" onClick={() => void reload()} loading={busy} data-testid="reload-plugins">
          <RefreshCwIcon /> Reload plugins
        </Button>
      </HStack>
      <Text textStyle="xs" color="fg.muted">
        Approaches built as programs of their own (san_backtest&apos;s plugin package, <code>go build -o</code> into this folder):{' '}
        <Text as="span" fontFamily="mono" data-testid="plugins-dir">
          {res?.dir ?? '...'}
        </Text>
        . A plugin only gets candles, positions and the account; it never sees the Exness connection. Reload after adding or
        rebuilding a file; a running session keeps the build it started with until you start it again.
      </Text>
      {error && <Text color="fg.error">{error}</Text>}
      {res && res.files.length === 0 && <Text color="fg.muted">No plugin files (*.exe) in the folder.</Text>}
      {res && res.files.length > 0 && (
        <Table.ScrollArea borderWidth="1px" rounded="md">
          <Table.Root size="sm" data-testid="plugins-table">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>File</Table.ColumnHeader>
                <Table.ColumnHeader>Modified</Table.ColumnHeader>
                <Table.ColumnHeader>Approaches</Table.ColumnHeader>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {res.files.map((f) => (
                <Table.Row key={f.path} data-testid="plugin-row" data-file={f.name}>
                  <Table.Cell fontFamily="mono" title={f.path}>
                    {f.name}
                  </Table.Cell>
                  <Table.Cell textStyle="xs">{f.modTime ? wib(sec(f.modTime)) : '-'}</Table.Cell>
                  <Table.Cell>
                    <Stack gap="1">
                      {f.approaches.length > 0 && (
                        <Wrap gap="1">
                          {f.approaches.map((a) => (
                            <Badge key={a} size="sm" colorPalette="purple" variant="subtle">
                              {a}
                            </Badge>
                          ))}
                        </Wrap>
                      )}
                      {f.error && (
                        <Text textStyle="xs" color="fg.error" data-testid="plugin-error" lineClamp={3} title={f.error}>
                          {f.error}
                        </Text>
                      )}
                    </Stack>
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </Table.ScrollArea>
      )}
    </Stack>
  );
}

// PluginHint is a running session's plugin: its file and the build it
// started with, and a hint when the file was rebuilt since.
export function PluginHint({ s }: { s: LivePaperSession }) {
  const p = s.plugin;
  if (!p) return null;
  const name = p.file.split(/[\\/]/).pop();
  return (
    <Stack gap="0.5" data-testid="plugin-run">
      <Badge size="xs" colorPalette="purple" variant="subtle" w="fit-content" title={p.file}>
        plugin · {name}
        {p.modTime ? ` · built ${wib(sec(p.modTime))}` : ''}
      </Badge>
      {p.changed && (
        <Text textStyle="2xs" color="fg.warning" data-testid="plugin-changed">
          plugin changed: restart the session to use the new build
        </Text>
      )}
    </Stack>
  );
}
