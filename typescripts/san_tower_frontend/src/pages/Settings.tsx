import { Button, Code, HStack, Heading, Input, RadioGroup, Stack, Text } from '@chakra-ui/react';
import { useEffect, useState } from 'react';
import { brokerSettings, errorText } from '../api';
import type { BrokerState } from '../gen/san/tower/v1/broker_pb';
import { FormDialog, FormField, toast } from '../ui';

// Settings is the tower's settings page (tower.json in its data folder), one
// section per part of the tower.
export function Settings() {
  return (
    <Stack gap="6" maxW="2xl">
      <Heading size="lg">Settings</Heading>
      <Stack as="section" gap="4">
        <Heading size="md">Message Broker</Heading>
        <BrokerSettings />
      </Stack>
    </Stack>
  );
}

type Form = { remote: boolean; dataDir: string; uri: string };

// where says which broker settings name, for people.
const where = (f: Form, defaultPath: string) =>
  f.remote ? `the san_message server at ${f.uri}` : `the broker inside the tower, in ${f.dataDir || defaultPath}`;

// BrokerSettings chooses the broker the tower uses: its own, inside it, in a
// data folder, or a separate san_message server at a URI, on this machine or
// another. Saving switches to it at once (BrokerService.UpdateBrokerSettings)
// once it opens or answers; nothing is moved. Both the folder and the URI
// are kept, whichever is in use.
function BrokerSettings() {
  const [state, setState] = useState<BrokerState>();
  const [form, setForm] = useState<Form>({ remote: false, dataDir: '', uri: '' });
  const [error, setError] = useState('');
  const [confirming, setConfirming] = useState(false);

  const show = (s?: BrokerState) => {
    setState(s);
    setForm({ remote: s?.settings?.remote ?? false, dataDir: s?.settings?.dataDir ?? '', uri: s?.settings?.uri ?? '' });
  };
  useEffect(() => {
    brokerSettings.getBrokerSettings({}).then(
      (r) => show(r.state),
      (err: unknown) => setError(errorText(err)),
    );
  }, []);

  if (error) return <Text color="fg.error">{error}</Text>;
  if (!state) return <Text color="fg.muted">Loading...</Text>;
  const saved: Form = {
    remote: state.settings?.remote ?? false,
    dataDir: state.settings?.dataDir ?? '',
    uri: state.settings?.uri ?? '',
  };
  const next: Form = { remote: form.remote, dataDir: form.dataDir.trim(), uri: form.uri.trim().replace(/\/+$/, '') };
  const changed = next.remote !== saved.remote || next.dataDir !== saved.dataDir || next.uri !== saved.uri;
  const set = (f: Partial<Form>) => setForm((x) => ({ ...x, ...f }));

  return (
    <Stack gap="4">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (changed) setConfirming(true);
        }}
      >
        <Stack gap="4">
          <RadioGroup.Root
            value={form.remote ? 'remote' : 'inside'}
            onValueChange={(e) => set({ remote: e.value === 'remote' })}
          >
            <Stack gap="3">
              <RadioGroup.Item value="inside" alignItems="flex-start">
                <RadioGroup.ItemHiddenInput />
                <RadioGroup.ItemIndicator />
                <Stack gap="0">
                  <RadioGroup.ItemText fontWeight="medium">Inside the tower</RadioGroup.ItemText>
                  <Text textStyle="sm" color="fg.muted">
                    The tower runs its own broker, in a data folder.
                  </Text>
                </Stack>
              </RadioGroup.Item>
              <RadioGroup.Item value="remote" alignItems="flex-start">
                <RadioGroup.ItemHiddenInput />
                <RadioGroup.ItemIndicator />
                <Stack gap="0">
                  <RadioGroup.ItemText fontWeight="medium">Separate server</RadioGroup.ItemText>
                  <Text textStyle="sm" color="fg.muted">
                    The tower uses a san_message server at a URI, on this machine or another.
                  </Text>
                </Stack>
              </RadioGroup.Item>
            </Stack>
          </RadioGroup.Root>
          {form.remote ? (
            <FormField
              label="Server URI"
              help="A san_message server (san_message server --listen <address>), such as http://10.8.0.5:7070. On another machine it must listen on an address this machine reaches, such as its VPN one. The tower asks it for its topics before it switches; the server needs no way back to the tower."
            >
              <Input
                fontFamily="mono"
                value={form.uri}
                onChange={(e) => set({ uri: e.currentTarget.value })}
                placeholder="http://10.8.0.5:7070"
              />
            </FormField>
          ) : (
            <FormField
              label="Data directory"
              help={`Where the broker keeps its topics, subscribers and event logs. Empty for the default, ${state.defaultDataPath}. A relative path is inside the tower's data folder, and the folder above it must exist.`}
            >
              <Input
                fontFamily="mono"
                value={form.dataDir}
                onChange={(e) => set({ dataDir: e.currentTarget.value })}
                placeholder={state.defaultDataPath}
              />
            </FormField>
          )}
          <Text fontSize="sm" color="fg.muted">
            In use:{' '}
            {saved.remote ? (
              <>
                the server at <Code>{saved.uri}</Code>
              </>
            ) : (
              <Code>{state.dataPath}</Code>
            )}
          </Text>
          <HStack>
            <Button type="submit" size="sm" colorPalette="blue" disabled={!changed || (next.remote && !next.uri)}>
              Save
            </Button>
            {!form.remote && (
              <Button size="sm" variant="outline" disabled={form.dataDir === ''} onClick={() => set({ dataDir: '' })}>
                Use the default
              </Button>
            )}
            {changed && (
              <Button size="sm" variant="ghost" onClick={() => setForm(saved)}>
                Undo changes
              </Button>
            )}
          </HStack>
        </Stack>
      </form>
      {confirming && (
        <FormDialog
          title="Switch the message broker?"
          submitText="Switch"
          onClose={() => setConfirming(false)}
          onSubmit={async () => {
            const r = await brokerSettings.updateBrokerSettings({ settings: next });
            show(r.state);
            toast(`The tower now uses ${where(next, state.defaultDataPath)}`);
          }}
        >
          <Text>
            The tower switches from {where(saved, state.defaultDataPath)} to {where(next, state.defaultDataPath)}
            {next.remote && next.uri !== saved.uri ? ', once it answers' : ''}.
          </Text>
          {!saved.remote && (next.remote || next.dataDir !== saved.dataDir) && (
            <Text>The broker inside the tower closes, and its pull subscribers are disconnected.</Text>
          )}
          <Text>
            The active workflows' triggers move with it: their subscribers are made in the new broker, at the end of its
            topics. Nothing is moved or deleted: the topics and subscribers become the ones there, none if it is new.
          </Text>
          {!next.remote && <Text>Do not point two towers at one folder.</Text>}
        </FormDialog>
      )}
    </Stack>
  );
}
