import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Badge, Button, Code, HStack, Heading, IconButton, Input, Menu, Portal, Stack, Table, Text } from '@chakra-ui/react';
import { BrainIcon, EllipsisVerticalIcon, PencilIcon, Trash2Icon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { errorText, storages } from '../api';
import type { Storage as TowerStorage } from '../gen/san/tower/v1/storage_pb';
import { FormDialog, FormField, toast } from '../ui';
import { forgetStorages } from '../workflow/catalog';

// Storage lists the tower's storages (StorageService): named base folders on
// this computer, which workflow steps use (the Storage step gives its Path
// and Absolute Path). A path is absolute, or relative to the tower's data
// folder. It creates, edits and deletes them; the folders themselves are
// left as they are.
export function Storage() {
  const [list, setList] = useState<TowerStorage[]>();
  const [dataDir, setDataDir] = useState('');
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const [editing, setEditing] = useState<TowerStorage | 'new'>();
  const [deleting, setDeleting] = useState<TowerStorage>();

  useEffect(() => {
    storages.listStorages({}).then(
      (r) => {
        setList(r.storages);
        setDataDir(r.dataDir);
        setError('');
      },
      (err: unknown) => setError(errorText(err)),
    );
  }, [reload]);
  const changed = () => {
    forgetStorages();
    setReload((n) => n + 1);
  };

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Storage</Heading>
        <Button size="sm" colorPalette="blue" onClick={() => setEditing('new')}>
          New storage
        </Button>
      </HStack>
      <Text color="fg.muted" textStyle="sm">
        Base folders the workflow steps use: a Storage step gives a storage's path as set here, and its absolute path. A
        path is absolute, or relative to the tower's data folder{dataDir && <> ({dataDir})</>}.
      </Text>
      {error ? (
        <Text color="fg.error">The tower does not answer: {error}</Text>
      ) : !list ? (
        <Text color="fg.muted">Loading...</Text>
      ) : list.length === 0 ? (
        <Text color="fg.muted">No storages yet.</Text>
      ) : (
        <Table.ScrollArea borderWidth="1px" rounded="md">
          {/* Fixed layout, so a long path wraps instead of pushing the other columns out. */}
          <Table.Root size="sm" interactive tableLayout="fixed">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader w="48">Name</Table.ColumnHeader>
                <Table.ColumnHeader>Base path</Table.ColumnHeader>
                <Table.ColumnHeader w="48">Last change</Table.ColumnHeader>
                <Table.ColumnHeader w="12" />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {list.map((s) => (
                <Table.Row key={s.id} cursor="pointer" onClick={() => setEditing(s)}>
                  {/* A knowledge's storage is tagged (the user, 2026-10-28): it goes only with the knowledge. */}
                  <Table.Cell>
                    <Text fontWeight="semibold">{s.name}</Text>
                    {s.knowledge && (
                      <Badge
                        colorPalette="purple"
                        size="sm"
                        mt="1"
                        maxW="full"
                        title={`Holds the files of knowledge ${s.knowledge}, and is deleted with it on the Knowledge page`}
                      >
                        <BrainIcon size="12" />
                        <Text as="span" truncate>
                          {s.knowledge === s.name ? 'Knowledge' : `Knowledge: ${s.knowledge}`}
                        </Text>
                      </Badge>
                    )}
                  </Table.Cell>
                  {/* A relative path says where it is under it; long paths wrap. */}
                  <Table.Cell fontFamily="mono" whiteSpace="normal" wordBreak="break-all">
                    {s.path}
                    {s.absolutePath !== s.path && (
                      <Text textStyle="xs" color="fg.muted">
                        {s.absolutePath}
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell color="fg.muted" whiteSpace="nowrap">
                    {s.updatedAt && timestampDate(s.updatedAt).toLocaleString()}
                  </Table.Cell>
                  {/* The menu is the row's own: a click in it does not open the editor. */}
                  <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
                    <RowMenu name={s.name} onEdit={() => setEditing(s)} onDelete={() => setDeleting(s)} />
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </Table.ScrollArea>
      )}
      {editing && (
        <StorageDialog
          storage={editing === 'new' ? undefined : editing}
          dataDir={dataDir}
          onClose={() => setEditing(undefined)}
          onSaved={changed}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete storage ${deleting.name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await storages.deleteStorage({ id: deleting.id });
            toast(`Deleted storage ${deleting.name}`);
            changed();
          }}
        >
          <Text>
            The folder <Code>{deleting.absolutePath}</Code> and what is in it stay as they are. Workflow steps that use
            this storage fail from then on, until you pick another storage in them.
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}

function RowMenu({ name, onEdit, onDelete }: { name: string; onEdit: () => void; onDelete: () => void }) {
  return (
    <Menu.Root
      positioning={{ placement: 'bottom-end' }}
      onSelect={(e) => (({ edit: onEdit, delete: onDelete }) as Record<string, () => void>)[e.value]?.()}
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

// StorageDialog creates a storage, or edits one.
function StorageDialog({
  storage,
  dataDir,
  onClose,
  onSaved,
}: {
  storage?: TowerStorage;
  dataDir: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(storage?.name ?? '');
  const [path, setPath] = useState(storage?.path ?? '');
  return (
    <FormDialog
      title={storage ? `Edit storage ${storage.name}` : 'New storage'}
      submitText={storage ? 'Save' : 'Create'}
      onClose={onClose}
      onSubmit={async () => {
        const s = { id: storage?.id ?? 0, name: name.trim(), path: path.trim() };
        const r = storage ? await storages.updateStorage({ storage: s }) : await storages.createStorage({ storage: s });
        toast(`${storage ? 'Saved' : 'Created'} storage ${s.name}: ${r.storage?.absolutePath}`);
        onSaved();
      }}
    >
      <FormField label="Name" help="Unique; steps show it when you pick a storage.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Videos" />
      </FormField>
      <FormField
        label="Base path"
        help={`Absolute, such as D:/videos, or relative to the tower's data folder${dataDir ? ` (${dataDir})` : ''}, such as videos. It is saved with / between folders; \\ is turned into /.`}
      >
        <Input
          fontFamily="mono"
          value={path}
          onChange={(e) => setPath(e.currentTarget.value)}
          placeholder="D:/videos or videos"
        />
      </FormField>
    </FormDialog>
  );
}
