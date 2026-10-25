import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Badge, Button, Clipboard, HStack, Heading, Input, Stack, Switch, Table, Text } from '@chakra-ui/react';
import { useEffect, useState } from 'react';
import { browserRemote, errorText } from '../api';
import type { BrowserRemote } from '../gen/san/tower/v1/browser_remote_pb';
import { FormField, toast } from '../ui';

// every is how often the page asks how the server is doing, so a browser
// that attaches shows up.
const every = 3000;

// BrowserRemoteServer is the Browser Remote Server page: the San Remote
// server (golang/apps/san_remote) the tower can run, for when nothing else
// runs it on this machine (BrowserRemoteService). The switch starts or stops
// it; while on, it starts with the tower. It shows the URI to paste in the
// San Remote extension's Server field, the one a San Remote connection
// calls, and the browsers attached, asked for every 3 s.
export function BrowserRemoteServer() {
  const [remote, setRemote] = useState<BrowserRemote>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let stale = false;
    const load = () =>
      browserRemote.getBrowserRemote({}).then(
        (r) => {
          if (stale) return;
          setRemote(r.remote);
          setError('');
        },
        (err: unknown) => !stale && setError(errorText(err)),
      );
    void load();
    const t = setInterval(load, every);
    return () => {
      stale = true;
      clearInterval(t);
    };
  }, []);

  const setActive = async (active: boolean) => {
    setBusy(true);
    try {
      const r = await browserRemote.setBrowserRemoteActive({ active });
      setRemote(r.remote);
      toast(active ? 'The browser remote server is running' : 'The browser remote server is stopped');
    } catch (err) {
      toast(errorText(err), true);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Stack gap="5" maxW="3xl">
      <HStack justify="space-between" wrap="wrap" gap="3">
        <Heading size="lg">Browser Remote Server</Heading>
        <Switch.Root
          colorPalette="green"
          checked={remote?.active ?? false}
          disabled={!remote || busy}
          onCheckedChange={(e) => void setActive(e.checked)}
        >
          <Switch.HiddenInput />
          <Switch.Control>
            <Switch.Thumb />
          </Switch.Control>
          <Switch.Label>Active</Switch.Label>
        </Switch.Root>
      </HStack>
      <Text color="fg.muted">
        Runs San Remote inside the tower: the server the San Remote browser extension attaches to, and the Threads, X,
        Facebook and Vidara services that workflow steps post through. Switch it on when san_remote does not already run
        on this machine; while it is on, it starts with the tower.
      </Text>
      {error && <Text color="fg.error">The tower does not answer: {error}</Text>}
      {remote && (
        <>
          <Status remote={remote} />
          <UriField
            label="Server URI, for the extension"
            uri={remote.serverUri}
            help="In the San Remote extension's popup, paste it in Server, then press Save and reconnect."
          />
          <UriField
            label="API URI, for a San Remote connection"
            uri={remote.apiUri}
            help="The URL of a San Remote connection (Connections page), which Threads, X, Facebook and Vidara steps use."
          />
          <Browsers remote={remote} />
        </>
      )}
    </Stack>
  );
}

// Status says whether it runs in the tower or, when it does not, whether a
// san_remote run outside the tower (serve, api) answers at its addresses.
function Status({ remote }: { remote: BrowserRemote }) {
  if (remote.running) {
    return (
      <HStack gap="2">
        <Badge colorPalette="green">Running</Badge>
        <Text textStyle="sm" color="fg.muted">
          in the tower, since {remote.startedAt && timestampDate(remote.startedAt).toLocaleString()}
        </Text>
      </HStack>
    );
  }
  const outside = remote.serverElsewhere || remote.apiElsewhere;
  return (
    <Stack gap="1">
      <HStack>
        {outside ? (
          <Badge colorPalette="blue">Running outside the tower</Badge>
        ) : remote.active ? (
          <Badge colorPalette="red">Not running</Badge>
        ) : (
          <Badge>Off</Badge>
        )}
        {remote.active && (
          <Text textStyle="sm" color="fg.muted">
            {outside ? 'Stop it there, then switch this off and on.' : 'Switch it off and on to try again.'}
          </Text>
        )}
      </HStack>
      {outside && (
        <Text textStyle="sm">
          On this machine, outside the tower: san_remote serve {remote.serverElsewhere ? 'runs' : 'does not run'}
          , san_remote api {remote.apiElsewhere ? 'runs' : 'does not run'}.
          {!remote.apiElsewhere && ' Without the api, the Threads, X, Facebook and Vidara steps fail.'}
          {!remote.serverElsewhere && ' Without serve, no browser can attach.'}
        </Text>
      )}
      {remote.active && remote.error && (
        <Text textStyle="sm" color="fg.error">
          {remote.error}
        </Text>
      )}
    </Stack>
  );
}

// UriField is a URI to copy, with a button that copies it.
function UriField({ label, uri, help }: { label: string; uri: string; help: string }) {
  return (
    <FormField label={label} help={help}>
      <Clipboard.Root value={uri} w="full">
        <HStack>
          <Clipboard.Input asChild>
            <Input readOnly fontFamily="mono" size="sm" onFocus={(e) => e.currentTarget.select()} />
          </Clipboard.Input>
          <Clipboard.Trigger asChild>
            <Button size="sm" variant="outline">
              <Clipboard.Indicator />
              Copy
            </Button>
          </Clipboard.Trigger>
        </HStack>
      </Clipboard.Root>
    </FormField>
  );
}

// Browsers are the browsers attached through the extension: to the server
// in the tower, or to the san_remote serve outside it.
function Browsers({ remote }: { remote: BrowserRemote }) {
  return (
    <Stack gap="2">
      <Heading size="sm">
        Attached browsers
        {!remote.running && remote.serverElsewhere && (
          <Text as="span" fontWeight="normal" color="fg.muted">
            {' '}
            (to the san_remote serve outside the tower)
          </Text>
        )}
      </Heading>
      {!remote.running && !remote.serverElsewhere ? (
        <Text textStyle="sm" color="fg.muted">
          None: the server does not run.
        </Text>
      ) : remote.browsers.length === 0 ? (
        <Text textStyle="sm" color="fg.muted">
          None yet. Once the extension has the server URI, its browser shows here.
        </Text>
      ) : (
        <Table.Root size="sm" variant="outline">
          <Table.Header>
            <Table.Row>
              <Table.ColumnHeader>Name</Table.ColumnHeader>
              <Table.ColumnHeader>Attached</Table.ColumnHeader>
              <Table.ColumnHeader>Extension</Table.ColumnHeader>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {remote.browsers.map((b) => (
              <Table.Row key={b.id}>
                <Table.Cell>{b.name || b.id}</Table.Cell>
                <Table.Cell>{b.attachedAt && timestampDate(b.attachedAt).toLocaleString()}</Table.Cell>
                <Table.Cell>{b.extensionVersion}</Table.Cell>
              </Table.Row>
            ))}
          </Table.Body>
        </Table.Root>
      )}
    </Stack>
  );
}
