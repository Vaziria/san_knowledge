import { Breadcrumb, IconButton, Input, Menu, NativeSelect, Portal, Table, Text } from '@chakra-ui/react';
import {
  DownloadIcon,
  EllipsisVerticalIcon,
  FolderIcon,
  FolderInputIcon,
  HomeIcon,
  PencilIcon,
  Trash2Icon,
} from 'lucide-react';
import { useState, type DragEvent } from 'react';
import { FormDialog, FormField } from './ui';

// Folders (the user, 2026-10-27: "In workflow, add folder like. So I can
// manage workflow easily"; the Knowledge page's too, 2026-10-28): a page
// shows one folder at a time, its folders first, with a breadcrumb back up;
// folders nest, and 0 is the top. The workflows' and the knowledge's are two
// trees: each page passes its own (FolderApi).

// AnyFolder is a folder of either tree.
export type AnyFolder = { id: number; name: string; parentId: number };

// FolderApi is a page's tree of folders: what its top is called, and how a
// folder is made, and renamed or moved.
export type FolderApi = {
  top: string;
  create: (name: string, parentId: number) => Promise<AnyFolder>;
  update: (id: number, name: string, parentId: number) => Promise<AnyFolder>;
};

// dragType marks a row (a workflow, a knowledge) being dragged onto a folder.
export const dragType = 'application/x-san-tower-item';

// path is a folder and the folders it is in, top first.
export function folderPath<F extends AnyFolder>(folders: F[], id: number): F[] {
  const byId = new Map(folders.map((f) => [f.id, f]));
  const out: F[] = [];
  for (let f = byId.get(id); f && out.length < 100; f = byId.get(f.parentId)) out.unshift(f);
  return out;
}

// pathText is a folder's place, "Trading / Binance"; the top is "".
export const pathText = (folders: AnyFolder[], id: number) =>
  folderPath(folders, id)
    .map((f) => f.name)
    .join(' / ');

// folderCounts says what is right in a folder: "3 workflows, 1 folder".
export const folderCounts = (items: number, one: string, many: string, folders: number) =>
  [`${items} ${items === 1 ? one : many}`, folders > 0 && `${folders} ${folders === 1 ? 'folder' : 'folders'}`]
    .filter(Boolean)
    .join(', ');

// tree lists the folders in order, each under its parent, with its depth;
// except and the folders inside it are left out (where a folder cannot go).
function tree(folders: AnyFolder[], except = 0): { folder: AnyFolder; depth: number }[] {
  const out: { folder: AnyFolder; depth: number }[] = [];
  const walk = (parent: number, depth: number) => {
    for (const f of folders) {
      if (f.parentId !== parent || f.id === except) continue;
      out.push({ folder: f, depth });
      walk(f.id, depth + 1);
    }
  };
  walk(0, 0);
  return out;
}

// dropTarget makes an element a place to drop a dragged row on.
export function dropTarget(onDrop: (id: number) => void, setOver?: (over: boolean) => void) {
  return {
    onDragOver: (e: DragEvent) => {
      if (!e.dataTransfer.types.includes(dragType)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      setOver?.(true);
    },
    onDragLeave: () => setOver?.(false),
    onDrop: (e: DragEvent) => {
      setOver?.(false);
      const id = Number(e.dataTransfer.getData(dragType));
      if (!id) return;
      e.preventDefault();
      onDrop(id);
    },
  };
}

// draggable makes a row one to drag onto a folder.
export const draggable = (id: number) => ({
  draggable: true,
  onDragStart: (e: DragEvent) => {
    e.dataTransfer.setData(dragType, String(id));
    e.dataTransfer.effectAllowed = 'move';
  },
});

// FolderCrumbs is the way from the top to the folder shown; a row can be
// dropped on any folder of it.
export function FolderCrumbs({
  api,
  folders,
  folder,
  onOpen,
  onDrop,
}: {
  api: FolderApi;
  folders: AnyFolder[];
  folder: number;
  onOpen: (id: number) => void;
  onDrop: (id: number, folder: number) => void;
}) {
  const path = folderPath(folders, folder);
  const crumb = (id: number, name: string, current: boolean) => (
    <Breadcrumb.Item key={id} {...dropTarget((w) => onDrop(w, id))}>
      {current ? (
        <Breadcrumb.CurrentLink>{name}</Breadcrumb.CurrentLink>
      ) : (
        <Breadcrumb.Link
          href="#"
          onClick={(e) => {
            e.preventDefault();
            onOpen(id);
          }}
        >
          {id === 0 ? <HomeIcon size="14" /> : <FolderIcon size="14" />}
          {name}
        </Breadcrumb.Link>
      )}
    </Breadcrumb.Item>
  );
  return (
    <Breadcrumb.Root size="sm">
      <Breadcrumb.List>
        {crumb(0, api.top, path.length === 0)}
        {path.map((f, i) => [
          <Breadcrumb.Separator key={`s${f.id}`} />,
          crumb(f.id, f.name, i === path.length - 1),
        ])}
      </Breadcrumb.List>
    </Breadcrumb.Root>
  );
}

// FolderRow is a folder in a page's table, in its five columns: a click opens
// it, and a row dragged onto it moves there. counts says what is in it, in
// the third column.
export function FolderRow({
  folder,
  counts,
  align = 'end',
  onOpen,
  onDrop,
  onRename,
  onMove,
  onDelete,
}: {
  folder: AnyFolder;
  counts: string;
  align?: 'start' | 'end';
  onOpen: () => void;
  onDrop: (id: number) => void;
  onRename: () => void;
  onMove: () => void;
  onDelete: () => void;
}) {
  const [over, setOver] = useState(false);
  return (
    <Table.Row cursor="pointer" onClick={onOpen} bg={over ? 'bg.emphasized' : undefined} {...dropTarget(onDrop, setOver)}>
      <Table.Cell color="fg.muted">
        <FolderIcon size="18" />
      </Table.Cell>
      <Table.Cell fontWeight="semibold">{folder.name}</Table.Cell>
      <Table.Cell textAlign={align} color="fg.muted" whiteSpace="nowrap">
        {counts}
      </Table.Cell>
      <Table.Cell />
      <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
        <RowMenu name={folder.name} items={{ rename: onRename, move: onMove, delete: onDelete }} />
      </Table.Cell>
    </Table.Row>
  );
}

// RowMenu is a row's actions, behind the three dots at the end of it: those
// given of Rename (Settings for a workflow, which has Export), Move to...,
// Export and Delete.
export function RowMenu({
  name,
  items,
}: {
  name: string;
  items: Partial<Record<'rename' | 'move' | 'export' | 'delete', () => void>>;
}) {
  return (
    <Menu.Root positioning={{ placement: 'bottom-end' }} onSelect={(e) => items[e.value as keyof typeof items]?.()}>
      <Menu.Trigger asChild>
        <IconButton aria-label={`Actions for ${name}`} title="Actions" size="xs" variant="ghost" color="fg.muted">
          <EllipsisVerticalIcon />
        </IconButton>
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content minW="36">
            {items.rename && (
              <Menu.Item value="rename">
                <PencilIcon />
                {items.export ? 'Settings' : 'Rename'}
              </Menu.Item>
            )}
            {items.move && (
              <Menu.Item value="move">
                <FolderInputIcon />
                Move to...
              </Menu.Item>
            )}
            {items.export && (
              <Menu.Item value="export">
                <DownloadIcon />
                Export
              </Menu.Item>
            )}
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

// FolderDialog makes a folder in parent, or renames one.
export function FolderDialog({
  api,
  folder,
  parent,
  onClose,
  onSaved,
}: {
  api: FolderApi;
  folder?: AnyFolder;
  parent: number;
  onClose: () => void;
  onSaved: (f: AnyFolder) => void;
}) {
  const [name, setName] = useState(folder?.name ?? '');
  return (
    <FormDialog
      title={folder ? `Rename folder ${folder.name}` : 'New folder'}
      submitText={folder ? 'Rename' : 'Create'}
      onClose={onClose}
      onSubmit={async () => {
        onSaved(
          folder ? await api.update(folder.id, name.trim(), folder.parentId) : await api.create(name.trim(), parent),
        );
      }}
    >
      <FormField label="Name" help="Unique in the folder it is in; at most 100 characters.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Trading" />
      </FormField>
    </FormDialog>
  );
}

// MoveDialog moves a row, or a folder, to another folder; a folder cannot go
// into itself or a folder inside it, so those are not offered.
export function MoveDialog({
  api,
  name,
  folders,
  from,
  folder,
  onClose,
  onMove,
}: {
  api: FolderApi;
  name: string;
  folders: AnyFolder[];
  from: number;
  folder?: AnyFolder;
  onClose: () => void;
  onMove: (to: number) => Promise<void>;
}) {
  const [to, setTo] = useState(from);
  const options = tree(folders, folder?.id);
  return (
    <FormDialog title={`Move ${name}`} submitText="Move" onClose={onClose} onSubmit={() => onMove(to)}>
      <FormField label="To">
        <NativeSelect.Root>
          <NativeSelect.Field value={to} onChange={(e) => setTo(Number(e.currentTarget.value))}>
            <option value={0}>{api.top} (the top)</option>
            {options.map(({ folder: f, depth }) => (
              <option key={f.id} value={f.id}>
                {`${'   '.repeat(depth + 1)}${f.name}`}
              </option>
            ))}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      </FormField>
      {to === from && (
        <Text textStyle="sm" color="fg.muted">
          It is there already.
        </Text>
      )}
    </FormDialog>
  );
}
