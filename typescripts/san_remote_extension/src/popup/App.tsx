import { Alert, Box, Button, Field, HStack, Heading, Input, Stack, Text } from '@chakra-ui/react';
import { useEffect, useState } from 'react';
import {
  type Settings,
  type Status,
  extensionPage,
  loadSettings,
  loadStatus,
  saveSettings,
  userScriptsAllowed,
} from '../settings';
import { StatusBadge } from '../ui';

type OnChanged = Parameters<typeof chrome.storage.onChanged.addListener>[0];

export function App() {
  const [status, setStatus] = useState<Status>();
  const [saved, setSaved] = useState<Settings>();
  const [form, setForm] = useState<Settings>({ server: '', name: '' });
  const [windowId, setWindowId] = useState<number>();
  const scriptsAllowed = userScriptsAllowed();

  useEffect(() => {
    // Known before the click: sidePanel.open must be called straight from the
    // click, without waiting on anything first.
    void chrome.windows.getCurrent().then((w) => setWindowId(w.id));
    // Wakes the service worker, which attaches if it is not attached yet.
    chrome.runtime.sendMessage('attach').catch(() => {});
    void loadSettings().then((s) => {
      setSaved(s);
      setForm(s);
    });
    void loadStatus().then(setStatus);
    const onChanged: OnChanged = (changes, area) => {
      if (area === 'session' && changes.status) setStatus(changes.status.newValue as Status);
    };
    chrome.storage.onChanged.addListener(onChanged);
    return () => chrome.storage.onChanged.removeListener(onChanged);
  }, []);

  const dirty = saved !== undefined && (form.server !== saved.server || form.name !== saved.name);
  const save = async () => {
    const s = { server: form.server.trim().replace(/\/+$/, ''), name: form.name.trim() };
    await saveSettings(s);
    setSaved(s);
    setForm(s);
  };

  return (
    <Box w="360px" p="4">
      <Stack gap="4">
        <HStack justify="space-between">
          <Heading size="md">San Remote</Heading>
          <StatusBadge status={status} />
        </HStack>

        <StatusText status={status} />

        <Button
          size="sm"
          variant="outline"
          disabled={windowId === undefined}
          onClick={() => {
            void chrome.sidePanel.open({ windowId: windowId! }).then(() => window.close());
          }}
        >
          Open debug panel
        </Button>

        {!scriptsAllowed && (
          <Alert.Root status="warning">
            <Alert.Indicator />
            <Alert.Content>
              <Alert.Title>Allow User Scripts is off</Alert.Title>
              <Alert.Description>
                <Stack gap="2" align="start">
                  <Text>San Remote cannot run JavaScript until it is on, on the extension's details page.</Text>
                  <Button size="xs" onClick={() => void chrome.tabs.create({ url: extensionPage() })}>
                    Open the details page
                  </Button>
                </Stack>
              </Alert.Description>
            </Alert.Content>
          </Alert.Root>
        )}

        <Field.Root>
          <Field.Label>Server</Field.Label>
          <Input size="sm" value={form.server} onChange={(e) => setForm({ ...form, server: e.target.value })} />
        </Field.Root>
        <Field.Root>
          <Field.Label>Browser name</Field.Label>
          <Input size="sm" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          <Field.HelperText>Picks this browser when several are attached.</Field.HelperText>
        </Field.Root>
        <Button size="sm" disabled={!dirty} onClick={() => void save()}>
          Save and reconnect
        </Button>
      </Stack>
    </Box>
  );
}

function StatusText({ status }: { status?: Status }) {
  if (!status) return <Text color="fg.muted">Starting…</Text>;
  const since = new Date(status.since).toLocaleTimeString();
  if (status.state === 'connected') {
    return (
      <Text color="fg.muted" fontSize="sm">
        Attached to {status.server} since {since}.
      </Text>
    );
  }
  if (status.state === 'connecting') {
    return (
      <Text color="fg.muted" fontSize="sm">
        Connecting to {status.server}…
      </Text>
    );
  }
  return (
    <Stack gap="1">
      <Text fontSize="sm">
        Cannot reach {status.server}. Is <code>san_remote serve</code> running? Trying again every 30s.
      </Text>
      {status.error && (
        <Text color="fg.muted" fontSize="xs">
          {status.error}
        </Text>
      )}
    </Stack>
  );
}
