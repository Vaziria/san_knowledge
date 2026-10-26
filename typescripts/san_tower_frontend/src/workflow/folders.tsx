import { Breadcrumb, Input, NativeSelect, Text } from '@chakra-ui/react';
import { FolderIcon, HomeIcon } from 'lucide-react';
import { useState, type DragEvent } from 'react';
import { workflows } from '../api';
import type { Folder } from '../gen/san/tower/v1/workflow_pb';
import { FormDialog, FormField } from '../ui';

// Folders of workflows (the user, 2026-10-27: "In workflow, add folder like.
// So I can manage workflow easily"): the Workflow page shows one folder at
// a time, its folders first, with a breadcrumb back up; folders nest, and 0
// is the top.

// dragType marks a workflow being dragged onto a folder.
export const dragType = 'application/x-san-tower-workflow';

// path is a folder and the folders it is in, top first.
export function folderPath(folders: Folder[], id: number): Folder[] {
  const byId = new Map(folders.map((f) => [f.id, f]));
  const out: Folder[] = [];
  for (let f = byId.get(id); f && out.length < 100; f = byId.get(f.parentId)) out.unshift(f);
  return out;
}

// pathText is a folder's place, "Trading / Binance"; the top is "".
export const pathText = (folders: Folder[], id: number) =>
  folderPath(folders, id)
    .map((f) => f.name)
    .join(' / ');

// tree lists the folders in order, each under its parent, with its depth;
// except and the folders inside it are left out (where a folder cannot go).
function tree(folders: Folder[], except = 0): { folder: Folder; depth: number }[] {
  const out: { folder: Folder; depth: number }[] = [];
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

// dropTarget makes an element a place to drop a dragged workflow on.
export function dropTarget(onDrop: (workflowId: number) => void, setOver?: (over: boolean) => void) {
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

// FolderCrumbs is the way from the top to the folder shown; a workflow can
// be dropped on any folder of it.
export function FolderCrumbs({
  folders,
  folder,
  onOpen,
  onDropWorkflow,
}: {
  folders: Folder[];
  folder: number;
  onOpen: (id: number) => void;
  onDropWorkflow: (workflowId: number, folder: number) => void;
}) {
  const path = folderPath(folders, folder);
  const crumb = (id: number, name: string, current: boolean) => (
    <Breadcrumb.Item key={id} {...dropTarget((w) => onDropWorkflow(w, id))}>
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
        {crumb(0, 'All workflows', path.length === 0)}
        {path.map((f, i) => [
          <Breadcrumb.Separator key={`s${f.id}`} />,
          crumb(f.id, f.name, i === path.length - 1),
        ])}
      </Breadcrumb.List>
    </Breadcrumb.Root>
  );
}

// FolderDialog makes a folder in parent, or renames one.
export function FolderDialog({
  folder,
  parent,
  onClose,
  onSaved,
}: {
  folder?: Folder;
  parent: number;
  onClose: () => void;
  onSaved: (f: Folder) => void;
}) {
  const [name, setName] = useState(folder?.name ?? '');
  return (
    <FormDialog
      title={folder ? `Rename folder ${folder.name}` : 'New folder'}
      submitText={folder ? 'Rename' : 'Create'}
      onClose={onClose}
      onSubmit={async () => {
        const r = folder
          ? await workflows.updateFolder({ id: folder.id, name: name.trim(), parentId: folder.parentId })
          : await workflows.createFolder({ name: name.trim(), parentId: parent });
        onSaved(r.folder!);
      }}
    >
      <FormField label="Name" help="Unique in the folder it is in; at most 100 characters.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Trading" />
      </FormField>
    </FormDialog>
  );
}

// MoveDialog moves a workflow, or a folder, to another folder; a folder
// cannot go into itself or a folder inside it, so those are not offered.
export function MoveDialog({
  name,
  folders,
  from,
  folder,
  onClose,
  onMove,
}: {
  name: string;
  folders: Folder[];
  from: number;
  folder?: Folder;
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
            <option value={0}>All workflows (the top)</option>
            {options.map(({ folder: f, depth }) => (
              <option key={f.id} value={f.id}>
                {`${'   '.repeat(depth + 1)}${f.name}`}
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
