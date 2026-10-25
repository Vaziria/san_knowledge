import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  Button,
  HStack,
  Heading,
  IconButton,
  Input,
  Menu,
  NativeSelect,
  Portal,
  Stack,
  Table,
  Text,
} from '@chakra-ui/react';
import { EllipsisVerticalIcon, PencilIcon, PlugZapIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { connections, errorText } from '../api';
import type { Connection, ConnectionSettings, TestConnectionResponse } from '../gen/san/tower/v1/connection_pb';
import { FormDialog, FormField, toast } from '../ui';
import { forgetConnections } from '../workflow/catalog';

// typeLabels names each connection type, by ConnectionSettings' field.
const typeLabels: Record<string, string> = { sanRemote: 'San Remote', telegramBot: 'Telegram Bot' };
type ConnectionType = 'sanRemote' | 'telegramBot';

// address is what a connection points at, for the list: a bot by its id,
// the part of its token before the colon, which is not secret.
const address = (s?: ConnectionSettings) => {
  switch (s?.type.case) {
    case 'sanRemote':
      return s.type.value.url;
    case 'telegramBot':
      return `bot ${s.type.value.token.split(':')[0]}`;
  }
  return '';
};

// Connections lists the tower's connections to other programs
// (ConnectionService), which workflow steps use, and creates, edits, tests
// and deletes them. The types are San Remote, `san_remote api`, which posts
// through the user's own browser, and Telegram Bot, a bot from @BotFather.
export function Connections() {
  const [list, setList] = useState<Connection[]>();
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const [editing, setEditing] = useState<Connection | 'new'>();
  const [deleting, setDeleting] = useState<Connection>();

  useEffect(() => {
    connections.listConnections({}).then(
      (r) => {
        setList(r.connections);
        setError('');
      },
      (err: unknown) => setError(errorText(err)),
    );
  }, [reload]);
  const changed = () => {
    forgetConnections();
    setReload((n) => n + 1);
  };

  const test = async (c: Connection) => {
    try {
      const r = await connections.testConnection({ settings: c.settings });
      toast(`${c.name}: ${r.message}`, !r.ok);
    } catch (err) {
      toast(`${c.name}: ${errorText(err)}`, true);
    }
  };

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Connections</Heading>
        <Button size="sm" colorPalette="blue" onClick={() => setEditing('new')}>
          New connection
        </Button>
      </HStack>
      <Text color="fg.muted" textStyle="sm">
        Programs the workflow steps use. A San Remote connection is `san_remote api`, which posts to Threads, X,
        Facebook and Vidara through your browser. A Telegram Bot connection is a bot from @BotFather, which the tower
        reads and sends Telegram messages as.
      </Text>
      {error ? (
        <Text color="fg.error">The tower does not answer: {error}</Text>
      ) : !list ? (
        <Text color="fg.muted">Loading...</Text>
      ) : list.length === 0 ? (
        <Text color="fg.muted">No connections yet.</Text>
      ) : (
        <Table.ScrollArea borderWidth="1px" rounded="md">
          <Table.Root size="sm" interactive>
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader>Name</Table.ColumnHeader>
                <Table.ColumnHeader>Type</Table.ColumnHeader>
                <Table.ColumnHeader>Address</Table.ColumnHeader>
                <Table.ColumnHeader>Last change</Table.ColumnHeader>
                <Table.ColumnHeader w="12" />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {list.map((c) => (
                <Table.Row key={c.id} cursor="pointer" onClick={() => setEditing(c)}>
                  <Table.Cell fontWeight="semibold">{c.name}</Table.Cell>
                  <Table.Cell>{typeLabels[c.settings?.type.case ?? ''] ?? 'Unknown'}</Table.Cell>
                  <Table.Cell fontFamily="mono">{address(c.settings)}</Table.Cell>
                  <Table.Cell color="fg.muted">{c.updatedAt && timestampDate(c.updatedAt).toLocaleString()}</Table.Cell>
                  {/* The menu is the row's own: a click in it does not open the editor. */}
                  <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
                    <RowMenu
                      name={c.name}
                      onEdit={() => setEditing(c)}
                      onTest={() => void test(c)}
                      onDelete={() => setDeleting(c)}
                    />
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </Table.ScrollArea>
      )}
      {editing && (
        <ConnectionDialog
          connection={editing === 'new' ? undefined : editing}
          onClose={() => setEditing(undefined)}
          onSaved={changed}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete connection ${deleting.name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await connections.deleteConnection({ id: deleting.id });
            toast(`Deleted connection ${deleting.name}`);
            changed();
          }}
        >
          <Text>Workflow steps that use it fail from then on, until you pick another connection in them.</Text>
        </FormDialog>
      )}
    </Stack>
  );
}

function RowMenu({
  name,
  onEdit,
  onTest,
  onDelete,
}: {
  name: string;
  onEdit: () => void;
  onTest: () => void;
  onDelete: () => void;
}) {
  return (
    <Menu.Root
      positioning={{ placement: 'bottom-end' }}
      onSelect={(e) => (({ edit: onEdit, test: onTest, delete: onDelete }) as Record<string, () => void>)[e.value]?.()}
    >
      <Menu.Trigger asChild>
        <IconButton aria-label={`Actions for ${name}`} title="Actions" size="xs" variant="ghost" color="fg.muted">
          <EllipsisVerticalIcon />
        </IconButton>
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content minW="36">
            <Menu.Item value="edit">
              <PencilIcon />
              Edit
            </Menu.Item>
            <Menu.Item value="test">
              <PlugZapIcon />
              Test
            </Menu.Item>
            <Menu.Separator />
            <Menu.Item value="delete" color="fg.error" _hover={{ bg: 'bg.error', color: 'fg.error' }}>
              <Trash2Icon />
              Delete
            </Menu.Item>
          </Menu.Content>
        </Menu.Positioner>
      </Portal>
    </Menu.Root>
  );
}

// ConnectionDialog creates a connection, or edits one, and tests what is
// typed in before it is saved.
function ConnectionDialog({
  connection,
  onClose,
  onSaved,
}: {
  connection?: Connection;
  onClose: () => void;
  onSaved: () => void;
}) {
  const old = connection?.settings?.type;
  const [type, setType] = useState<ConnectionType>(old?.case ?? 'sanRemote');
  const [name, setName] = useState(connection?.name ?? '');
  const [url, setUrl] = useState(old?.case === 'sanRemote' ? old.value.url : 'http://127.0.0.1:7478');
  const [token, setToken] = useState(old?.case === 'telegramBot' ? old.value.token : '');
  const [result, setResult] = useState<TestConnectionResponse>();
  const [testing, setTesting] = useState(false);
  const settings =
    type === 'sanRemote'
      ? { type: { case: 'sanRemote' as const, value: { url: url.trim() } } }
      : { type: { case: 'telegramBot' as const, value: { token: token.trim() } } };

  const test = async () => {
    setTesting(true);
    try {
      setResult(await connections.testConnection({ settings }));
    } catch (err) {
      setResult({ ok: false, message: errorText(err) } as TestConnectionResponse);
    }
    setTesting(false);
  };

  return (
    <FormDialog
      title={connection ? `Edit connection ${connection.name}` : 'New connection'}
      submitText={connection ? 'Save' : 'Create'}
      onClose={onClose}
      onSubmit={async () => {
        const c = { id: connection?.id ?? 0, name: name.trim(), settings };
        if (connection) await connections.updateConnection({ connection: c });
        else await connections.createConnection({ connection: c });
        toast(`${connection ? 'Saved' : 'Created'} connection ${c.name}`);
        onSaved();
      }}
    >
      {/* A connection keeps its type: the steps that use it are of that type. */}
      <FormField label="Type">
        <NativeSelect.Root disabled={!!connection}>
          <NativeSelect.Field
            value={type}
            onChange={(e) => {
              setType(e.currentTarget.value as ConnectionType);
              setResult(undefined);
            }}
          >
            {Object.entries(typeLabels).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      </FormField>
      <FormField label="Name" help="Unique; steps show it when you pick a connection.">
        <Input
          autoFocus
          value={name}
          onChange={(e) => setName(e.currentTarget.value)}
          placeholder={type === 'sanRemote' ? 'My Chrome' : 'My bot'}
        />
      </FormField>
      {type === 'sanRemote' ? (
        <FormField
          label="URL"
          help="Where `san_remote api` listens. Start it with `san_remote api` (it needs `san_remote serve`)."
        >
          <Input fontFamily="mono" value={url} onChange={(e) => setUrl(e.currentTarget.value)} />
        </FormField>
      ) : (
        <FormField
          label="Token"
          help="From @BotFather in Telegram: /newbot makes a bot and gives its token. In a group the bot sees every message only with its privacy mode off (/setprivacy)."
        >
          <Input
            type="password"
            fontFamily="mono"
            value={token}
            onChange={(e) => setToken(e.currentTarget.value)}
            placeholder="123456:ABC-DEF1234"
          />
        </FormField>
      )}
      <HStack gap="3" align="flex-start">
        <Button type="button" size="sm" variant="outline" loading={testing} onClick={() => void test()}>
          <PlugZapIcon />
          Test
        </Button>
        {result && (
          <Text textStyle="sm" color={result.ok ? 'fg.success' : 'fg.error'} pt="1.5">
            {result.message}
          </Text>
        )}
      </HStack>
    </FormDialog>
  );
}
