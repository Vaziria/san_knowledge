import { Badge, Button, Code, HStack, Heading, Input, NativeSelect, Stack, Table, Tabs, Text } from '@chakra-ui/react';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router';
import { broker, brokerSettings, errorText } from '../api';
import { SubscriberType, type Subscriber, type Topic } from '../gen/san/message/v1/message_pb';
import type { BrokerState } from '../gen/san/tower/v1/broker_pb';
import { FormDialog, FormField, toast } from '../ui';

// MessageBroker manages the San Message broker the tower runs
// (golang/packages/san_message): its topics and their subscribers, and its
// settings. The tab is in the URL (?tab=subscribers), and so is the
// subscriber list's topic (?topic=orders), so a view can be linked to.
export function MessageBroker() {
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') ?? 'topics';
  const topic = params.get('topic') ?? '';
  const [topics, setTopics] = useState<Topic[]>();
  const [subscribers, setSubscribers] = useState<Subscriber[]>();
  const [error, setError] = useState('');

  const load = useCallback(async () => {
    try {
      const [t, s] = await Promise.all([broker.listTopics({}), broker.listSubscribers({})]);
      setTopics(t.topics);
      setSubscribers(s.subscribers);
      setError('');
    } catch (err) {
      setError(errorText(err));
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);

  const showSubscribers = (topic: string) => setParams(topic ? { tab: 'subscribers', topic } : { tab: 'subscribers' });

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Message Broker</Heading>
        <Button size="sm" variant="outline" onClick={() => void load()}>
          Refresh
        </Button>
      </HStack>
      {error && <Text color="fg.error">The broker does not answer: {error}</Text>}
      <Tabs.Root value={tab} onValueChange={(e) => setParams(e.value === 'topics' ? {} : { tab: e.value })}>
        <Tabs.List>
          <Tabs.Trigger value="topics">
            Topics <Badge size="sm">{topics?.length ?? 0}</Badge>
          </Tabs.Trigger>
          <Tabs.Trigger value="subscribers">
            Subscribers <Badge size="sm">{subscribers?.length ?? 0}</Badge>
          </Tabs.Trigger>
          <Tabs.Trigger value="settings">Settings</Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="topics">
          <Topics topics={topics} reload={load} onShowSubscribers={showSubscribers} />
        </Tabs.Content>
        <Tabs.Content value="subscribers">
          <Subscribers topics={topics} subscribers={subscribers} topic={topic} onTopic={showSubscribers} reload={load} />
        </Tabs.Content>
        <Tabs.Content value="settings">
          <Settings reload={load} />
        </Tabs.Content>
      </Tabs.Root>
    </Stack>
  );
}

// position is a place in a topic's log: segment file number and byte offset.
const position = (segment: number, offset: number) => `${segment}:${offset}`;

function DataTable({ head, children }: { head: ReactNode; children: ReactNode }) {
  return (
    <Table.ScrollArea borderWidth="1px" rounded="md">
      <Table.Root size="sm">
        <Table.Header>
          <Table.Row>{head}</Table.Row>
        </Table.Header>
        <Table.Body>{children}</Table.Body>
      </Table.Root>
    </Table.ScrollArea>
  );
}

// ------------------------------------------------------------- topics ---

function Topics({
  topics,
  reload,
  onShowSubscribers,
}: {
  topics?: Topic[];
  reload: () => Promise<void>;
  onShowSubscribers: (topic: string) => void;
}) {
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Topic>();
  return (
    <Stack gap="3">
      <HStack>
        <Button size="sm" colorPalette="blue" onClick={() => setCreating(true)}>
          New topic
        </Button>
      </HStack>
      {topics?.length === 0 ? (
        <Text color="fg.muted">No topics yet.</Text>
      ) : (
        <DataTable
          head={
            <>
              <Table.ColumnHeader>Topic</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Last event</Table.ColumnHeader>
              <Table.ColumnHeader title="segment:byte offset">Log end</Table.ColumnHeader>
              <Table.ColumnHeader textAlign="end">Subscribers</Table.ColumnHeader>
              <Table.ColumnHeader />
            </>
          }
        >
          {topics?.map((t) => (
            <Table.Row key={t.key}>
              <Table.Cell fontFamily="mono">{t.key}</Table.Cell>
              <Table.Cell textAlign="end">{String(t.lastEventId)}</Table.Cell>
              <Table.Cell fontFamily="mono" color="fg.muted">
                {position(t.segment, t.offset)}
              </Table.Cell>
              <Table.Cell textAlign="end">
                <Button size="xs" variant="ghost" onClick={() => onShowSubscribers(t.key)}>
                  {t.subscribers}
                </Button>
              </Table.Cell>
              <Table.Cell textAlign="end">
                <Button
                  size="xs"
                  variant="ghost"
                  colorPalette="red"
                  disabled={t.subscribers > 0}
                  title={t.subscribers > 0 ? 'Delete its subscribers first' : undefined}
                  onClick={() => setDeleting(t)}
                >
                  Delete
                </Button>
              </Table.Cell>
            </Table.Row>
          ))}
        </DataTable>
      )}
      {creating && <CreateTopic reload={reload} onClose={() => setCreating(false)} />}
      {deleting && (
        <FormDialog
          title={`Delete topic ${deleting.key}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await broker.deleteTopic({ topic: deleting.key });
            toast(`Deleted topic ${deleting.key}`);
            await reload();
          }}
        >
          <Text>Its event log files are deleted too. This cannot be undone.</Text>
        </FormDialog>
      )}
    </Stack>
  );
}

function CreateTopic({ reload, onClose }: { reload: () => Promise<void>; onClose: () => void }) {
  const [key, setKey] = useState('');
  return (
    <FormDialog
      title="New topic"
      submitText="Create"
      onClose={onClose}
      onSubmit={async () => {
        await broker.createTopic({ topic: key.trim() });
        toast(`Created topic ${key.trim()}`);
        await reload();
      }}
    >
      <FormField label="Topic" help="Letters, digits, - and _, at most 255 bytes: it is also a folder name.">
        <Input autoFocus value={key} onChange={(e) => setKey(e.currentTarget.value)} placeholder="orders" />
      </FormField>
    </FormDialog>
  );
}

// -------------------------------------------------------- subscribers ---

function Subscribers({
  topics,
  subscribers,
  topic,
  onTopic,
  reload,
}: {
  topics?: Topic[];
  subscribers?: Subscriber[];
  topic: string;
  onTopic: (topic: string) => void;
  reload: () => Promise<void>;
}) {
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Subscriber>();
  const shown = subscribers?.filter((s) => !topic || s.topic === topic);
  return (
    <Stack gap="3">
      <HStack gap="3">
        <NativeSelect.Root size="sm" width="56">
          <NativeSelect.Field value={topic} onChange={(e) => onTopic(e.currentTarget.value)}>
            <option value="">Every topic</option>
            {topics?.map((t) => (
              <option key={t.key} value={t.key}>
                {t.key}
              </option>
            ))}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
        <Button size="sm" colorPalette="blue" disabled={!topics?.length} onClick={() => setCreating(true)}>
          New subscriber
        </Button>
      </HStack>
      {topics?.length === 0 ? (
        <Text color="fg.muted">A subscriber reads one topic: create a topic first.</Text>
      ) : shown?.length === 0 ? (
        <Text color="fg.muted">No subscribers{topic && ` on ${topic}`} yet.</Text>
      ) : (
        <DataTable
          head={
            <>
              <Table.ColumnHeader>Subscriber</Table.ColumnHeader>
              <Table.ColumnHeader>Topic</Table.ColumnHeader>
              <Table.ColumnHeader>Type</Table.ColumnHeader>
              <Table.ColumnHeader>Delivery</Table.ColumnHeader>
              <Table.ColumnHeader title="segment:byte offset of the next event it gets">At</Table.ColumnHeader>
              <Table.ColumnHeader>Retry</Table.ColumnHeader>
              <Table.ColumnHeader />
            </>
          }
        >
          {shown?.map((s) => (
            <Table.Row key={s.key}>
              <Table.Cell fontFamily="mono">{s.key}</Table.Cell>
              <Table.Cell fontFamily="mono">{s.topic}</Table.Cell>
              <Table.Cell>
                <Badge colorPalette={s.type === SubscriberType.WEBHOOK ? 'purple' : 'blue'}>{typeName(s.type)}</Badge>
              </Table.Cell>
              <Table.Cell maxW="xs" truncate title={s.webhookUri || undefined}>
                {s.type === SubscriberType.WEBHOOK ? (
                  s.webhookUri
                ) : s.connected ? (
                  <Badge colorPalette="green">Connected</Badge>
                ) : (
                  <Badge>Not connected</Badge>
                )}
              </Table.Cell>
              <Table.Cell fontFamily="mono" color="fg.muted">
                {position(s.segment, s.offset)}
              </Table.Cell>
              <Table.Cell>
                {s.retryCount > 0 ? (
                  <HStack gap="2">
                    <Badge colorPalette="orange">{s.retryCount} failed</Badge>
                    {s.retryAtMs > 0n && (
                      <Text fontSize="xs" color="fg.muted">
                        next {new Date(Number(s.retryAtMs)).toLocaleTimeString()}
                      </Text>
                    )}
                  </HStack>
                ) : (
                  <Text color="fg.subtle">-</Text>
                )}
              </Table.Cell>
              <Table.Cell textAlign="end">
                <Button size="xs" variant="ghost" colorPalette="red" onClick={() => setDeleting(s)}>
                  Delete
                </Button>
              </Table.Cell>
            </Table.Row>
          ))}
        </DataTable>
      )}
      {creating && topics && (
        <CreateSubscriber topics={topics} topic={topic} reload={reload} onClose={() => setCreating(false)} />
      )}
      {deleting && (
        <FormDialog
          title={`Delete subscriber ${deleting.key}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await broker.deleteSubscriber({ subscriber: deleting.key });
            toast(`Deleted subscriber ${deleting.key}`);
            await reload();
          }}
        >
          <Text>
            Its place in <b>{deleting.topic}</b> is lost: made again, it starts at the end of the log.
            {deleting.connected && ' Its open stream is closed.'}
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}

const typeName = (t: SubscriberType) => (t === SubscriberType.WEBHOOK ? 'Webhook' : t === SubscriberType.PULL ? 'Pull' : 'Unknown');

function CreateSubscriber({
  topics,
  topic,
  reload,
  onClose,
}: {
  topics: Topic[];
  topic: string;
  reload: () => Promise<void>;
  onClose: () => void;
}) {
  const [key, setKey] = useState('');
  const [topicKey, setTopicKey] = useState(topic || topics[0]?.key || '');
  const [type, setType] = useState(SubscriberType.PULL);
  const [uri, setUri] = useState('');
  const webhook = type === SubscriberType.WEBHOOK;
  return (
    <FormDialog
      title="New subscriber"
      submitText="Create"
      onClose={onClose}
      onSubmit={async () => {
        await broker.createSubscriber({
          subscriber: key.trim(),
          topic: topicKey,
          type,
          webhookUri: webhook ? uri.trim() : '',
        });
        toast(`Created subscriber ${key.trim()}`);
        await reload();
      }}
    >
      <FormField label="Subscriber" help="Unique across every topic, at most 255 bytes.">
        <Input autoFocus value={key} onChange={(e) => setKey(e.currentTarget.value)} placeholder="billing" />
      </FormField>
      <FormField label="Topic">
        <NativeSelect.Root>
          <NativeSelect.Field value={topicKey} onChange={(e) => setTopicKey(e.currentTarget.value)}>
            {topics.map((t) => (
              <option key={t.key} value={t.key}>
                {t.key}
              </option>
            ))}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      </FormField>
      <FormField
        label="Type"
        help={webhook ? 'The broker POSTs each event to a URI.' : 'The app connects to the broker and acks each event.'}
      >
        <NativeSelect.Root>
          <NativeSelect.Field value={String(type)} onChange={(e) => setType(Number(e.currentTarget.value))}>
            <option value={SubscriberType.PULL}>Pull</option>
            <option value={SubscriberType.WEBHOOK}>Webhook</option>
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      </FormField>
      {webhook && (
        <FormField label="Webhook URI" help="http or https, at most 255 bytes.">
          <Input value={uri} onChange={(e) => setUri(e.currentTarget.value)} placeholder="https://example.com/hook" />
        </FormField>
      )}
      <Text fontSize="sm" color="fg.muted">
        It starts at the end of the topic's log, so it gets only events published from now on.
      </Text>
    </FormDialog>
  );
}

// ----------------------------------------------------------- settings ---

// Settings sets the broker's data folder. Saving a new one switches the
// broker to it (BrokerService.UpdateBrokerSettings); nothing is moved.
function Settings({ reload }: { reload: () => Promise<void> }) {
  const [state, setState] = useState<BrokerState>();
  const [dataDir, setDataDir] = useState('');
  const [error, setError] = useState('');
  const [confirming, setConfirming] = useState(false);

  const show = (s?: BrokerState) => {
    setState(s);
    setDataDir(s?.settings?.dataDir ?? '');
  };
  useEffect(() => {
    brokerSettings.getBrokerSettings({}).then(
      (r) => show(r.state),
      (err: unknown) => setError(errorText(err)),
    );
  }, []);

  if (error) return <Text color="fg.error">{error}</Text>;
  if (!state) return <Text color="fg.muted">Loading...</Text>;
  const changed = dataDir.trim() !== (state.settings?.dataDir ?? '');
  return (
    <Stack gap="4" maxW="2xl">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (changed) setConfirming(true);
        }}
      >
        <Stack gap="4">
          <FormField
            label="Data directory"
            help={`Where the broker keeps its topics, subscribers and event logs. Empty for the default, ${state.defaultDataPath}. A relative path is inside the tower's data folder, and the folder above it must exist.`}
          >
            <Input
              fontFamily="mono"
              value={dataDir}
              onChange={(e) => setDataDir(e.currentTarget.value)}
              placeholder={state.defaultDataPath}
            />
          </FormField>
          <Text fontSize="sm" color="fg.muted">
            In use: <Code>{state.dataPath}</Code>
          </Text>
          <HStack>
            <Button type="submit" size="sm" colorPalette="blue" disabled={!changed}>
              Save
            </Button>
            <Button size="sm" variant="outline" disabled={dataDir === ''} onClick={() => setDataDir('')}>
              Use the default
            </Button>
          </HStack>
        </Stack>
      </form>
      {confirming && (
        <FormDialog
          title="Switch the broker's data directory?"
          submitText="Switch"
          onClose={() => setConfirming(false)}
          onSubmit={async () => {
            const r = await brokerSettings.updateBrokerSettings({ settings: { dataDir: dataDir.trim() } });
            show(r.state);
            toast(`The broker now uses ${r.state?.dataPath}`);
            await reload();
          }}
        >
          <Text>
            The broker in <Code>{state.dataPath}</Code> closes, and its pull subscribers are disconnected. It opens in{' '}
            <Code>{dataDir.trim() || state.defaultDataPath}</Code>.
          </Text>
          <Text>
            Nothing is moved or deleted: the topics and subscribers become the ones in that folder, none if it is new.
            Do not point two towers at one folder.
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}
