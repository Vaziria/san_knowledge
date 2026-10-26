import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  Button,
  HStack,
  Heading,
  IconButton,
  Image,
  Input,
  Menu,
  NativeSelect,
  Portal,
  Stack,
  Table,
  Text,
} from '@chakra-ui/react';
import { EllipsisVerticalIcon, PencilIcon, PlugZapIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { connections, errorText, storages } from '../api';
import type { Connection, ConnectionSettings, TestConnectionResponse } from '../gen/san/tower/v1/connection_pb';
import type { Storage } from '../gen/san/tower/v1/storage_pb';
import { FormDialog, FormField, toast } from '../ui';
import { forgetConnections } from '../workflow/catalog';

// typeLabels names each connection type, by ConnectionSettings' field.
const typeLabels: Record<string, string> = {
  sanRemote: 'San Remote',
  telegramBot: 'Telegram Bot',
  whatsapp: 'WhatsApp',
  binance: 'Binance',
};
type ConnectionType = 'sanRemote' | 'telegramBot' | 'whatsapp' | 'binance';

// binanceNetworks are a Binance connection's networks; Live trades real money.
const binanceNetworks: Record<string, string> = {
  testnet: 'Testnet (testnet.binancefuture.com)',
  demo: 'Demo Trading (demo-fapi.binance.com)',
  live: 'Live (fapi.binance.com): real money',
};

// address is what a connection points at, for the list: a bot by its id,
// the part of its token before the colon, which is not secret; a WhatsApp
// account by its number; a Binance account by its network and the start of
// its API key.
const address = (s?: ConnectionSettings) => {
  switch (s?.type.case) {
    case 'sanRemote':
      return s.type.value.url;
    case 'telegramBot':
      return `bot ${s.type.value.token.split(':')[0]}`;
    case 'whatsapp':
      return `+${s.type.value.phone}`;
    case 'binance':
      return `${s.type.value.network || 'testnet'} · key ${s.type.value.apiKey.slice(0, 6)}…`;
  }
  return '';
};

// pngURL is a PNG's bytes as a data URL, for an <img>.
const pngURL = (png: Uint8Array) => {
  let s = '';
  for (const b of png) s += String.fromCharCode(b);
  return `data:image/png;base64,${btoa(s)}`;
};

// Connections lists the tower's connections to other programs
// (ConnectionService), which workflow steps use, and creates, edits, tests
// and deletes them. The types are San Remote, `san_remote api`, which posts
// through the user's own browser, Telegram Bot, a bot from @BotFather,
// WhatsApp, and Binance, a futures account through an API key.
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
        reads and sends Telegram messages as. A WhatsApp connection is your WhatsApp account, linked by scanning a QR
        code as WhatsApp Web is. A Binance connection is a Binance futures account, through an API key, which the
        Finance steps read and trade on.
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
// typed in before it is saved. A new WhatsApp connection is made by linking
// (PairWhatsApp): the dialog shows the QR codes to scan, and the connection
// exists once the phone is linked and connected; closing it stops that.
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
  const [apiKey, setApiKey] = useState(old?.case === 'binance' ? old.value.apiKey : '');
  const [apiSecret, setApiSecret] = useState(old?.case === 'binance' ? old.value.apiSecret : '');
  const [network, setNetwork] = useState(old?.case === 'binance' ? old.value.network || 'testnet' : 'testnet');
  const [result, setResult] = useState<TestConnectionResponse>();
  const [testing, setTesting] = useState(false);
  const [qr, setQr] = useState<string>(); // the QR code to scan, as a data URL
  const [asking, setAsking] = useState(false); // linking, before the first code
  const abort = useRef<AbortController>(undefined);
  const linking = type === 'whatsapp' && !connection;
  // The storages, for a WhatsApp connection's database: required.
  const [storageList, setStorageList] = useState<Storage[]>();
  const [storageId, setStorageId] = useState(0);
  useEffect(() => {
    if (type !== 'whatsapp' || storageList) return;
    storages.listStorages({}).then(
      (r) => setStorageList(r.storages),
      () => setStorageList([]),
    );
  }, [type, storageList]);
  const settings =
    type === 'sanRemote'
      ? { type: { case: 'sanRemote' as const, value: { url: url.trim() } } }
      : type === 'telegramBot'
        ? { type: { case: 'telegramBot' as const, value: { token: token.trim() } } }
        : type === 'binance'
          ? { type: { case: 'binance' as const, value: { apiKey: apiKey.trim(), apiSecret: apiSecret.trim(), network } } }
          : { type: { case: 'whatsapp' as const, value: old?.case === 'whatsapp' ? old.value : { jid: '' } } };
  const close = () => {
    abort.current?.abort();
    onClose();
  };

  // link links a WhatsApp account: it shows each QR code until the phone
  // has scanned one and the tower made the connection.
  const link = async () => {
    if (!storageId) throw new Error("Pick a storage: its folder keeps the device's database");
    const ac = new AbortController();
    abort.current = ac;
    setAsking(true);
    try {
      for await (const r of connections.pairWhatsApp({ name: name.trim(), storageId }, { signal: ac.signal })) {
        if (r.connection) {
          const wa = r.connection.settings?.type;
          toast(`Linked ${wa?.case === 'whatsapp' ? `+${wa.value.phone}` : 'WhatsApp'} as ${r.connection.name}`);
          onSaved();
          return;
        }
        if (r.qrPng.length) {
          setQr(pngURL(r.qrPng));
          setAsking(false);
        }
      }
      throw new Error('WhatsApp ended the linking before a phone scanned the code');
    } finally {
      setQr(undefined);
      setAsking(false);
    }
  };

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
      submitText={linking ? 'Link with QR code' : connection ? 'Save' : 'Create'}
      onClose={close}
      onSubmit={async () => {
        if (linking) return link();
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
          placeholder={({ sanRemote: 'My Chrome', telegramBot: 'My bot', whatsapp: 'My WhatsApp', binance: 'Binance testnet' } as Record<ConnectionType, string>)[type]}
        />
      </FormField>
      {type === 'whatsapp' ? (
        old?.case === 'whatsapp' ? (
          <>
            <FormField label="Account" help="To link another account, make a new connection.">
              <Text fontFamily="mono">
                +{old.value.phone} {old.value.name && `(${old.value.name})`}
              </Text>
            </FormField>
            <FormField
              label="Database"
              help="The device's keys, in this storage's folder; named by the hash of the connection's name as it was made, so a rename keeps it."
            >
              <Text>
                {storageList?.find((s) => s.id === old.value.storageId)?.name ?? `storage ${old.value.storageId}`}
              </Text>
              <Text fontFamily="mono" textStyle="xs" color="fg.muted" wordBreak="break-all">
                {old.value.database}
              </Text>
            </FormField>
          </>
        ) : qr ? (
          <Stack align="center" gap="2">
            <Image src={qr} alt="QR code to scan with WhatsApp" w="64" h="64" rounded="md" />
            <Text textStyle="sm" color="fg.muted" textAlign="center">
              On your phone: WhatsApp › Settings › Linked devices › Link a device, then scan this code. A new code
              comes every 20 seconds or so.
            </Text>
          </Stack>
        ) : asking ? (
          <Text textStyle="sm" color="fg.muted">
            Asking WhatsApp for a code...
          </Text>
        ) : (
          <>
            <FormField
              label="Storage"
              help="Required: its folder keeps the device's database, a file named by the hash of the connection's name. Storages are registered on the Storage page."
            >
              {storageList?.length === 0 ? (
                <Text textStyle="sm" color="fg.error">
                  No storage yet: register one on the Storage page first.
                </Text>
              ) : (
                <NativeSelect.Root>
                  <NativeSelect.Field value={storageId} onChange={(e) => setStorageId(Number(e.currentTarget.value))}>
                    <option value={0}>Pick a storage...</option>
                    {storageList?.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.absolutePath})
                      </option>
                    ))}
                  </NativeSelect.Field>
                  <NativeSelect.Indicator />
                </NativeSelect.Root>
              )}
            </FormField>
            <Text textStyle="sm" color="fg.muted">
              Links this tower to your WhatsApp account as a linked device, as WhatsApp Web is: Link shows a QR code to
              scan with your phone, and the connection is made once the phone is linked. WhatsApp does not allow
              unofficial programs and may ban an account that uses one: use a number you can risk.
            </Text>
          </>
        )
      ) : type === 'binance' ? (
        <>
          <FormField
            label="Network"
            help="Testnet and Demo Trading are play money: make the API key there. Live trades real money with a key from your Binance account."
          >
            <NativeSelect.Root>
              <NativeSelect.Field value={network} onChange={(e) => setNetwork(e.currentTarget.value)}>
                {Object.entries(binanceNetworks).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </NativeSelect.Field>
              <NativeSelect.Indicator />
            </NativeSelect.Root>
          </FormField>
          <FormField label="API key" help="USD-M futures. Test reads the wallet with it; nothing is traded.">
            <Input fontFamily="mono" value={apiKey} onChange={(e) => setApiKey(e.currentTarget.value)} />
          </FormField>
          <FormField label="API secret" help="It stays in the tower: an export leaves connection settings behind.">
            <Input
              type="password"
              fontFamily="mono"
              value={apiSecret}
              onChange={(e) => setApiSecret(e.currentTarget.value)}
            />
          </FormField>
        </>
      ) : type === 'sanRemote' ? (
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
      {!linking && (
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
      )}
    </FormDialog>
  );
}
