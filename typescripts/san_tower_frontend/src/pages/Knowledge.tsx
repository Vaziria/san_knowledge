import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Button, Code, HStack, Heading, Input, Stack, Table, Text } from '@chakra-ui/react';
import { BrainIcon, FolderPlusIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { errorText, knowledge } from '../api';
import {
  draggable,
  FolderCrumbs,
  folderCounts,
  FolderDialog,
  folderPath,
  FolderRow,
  MoveDialog,
  RowMenu,
  type FolderApi,
} from '../folders';
import type { Knowledge as TowerKnowledge, KnowledgeFolder } from '../gen/san/tower/v1/knowledge_pb';
import { FormDialog, FormField, toast } from '../ui';
import { forgetKnowledge, forgetStorages } from '../workflow/catalog';

// knowledgeFolders are the folders of knowledge, for folders.tsx.
const knowledgeFolders: FolderApi = {
  top: 'All knowledge',
  create: async (name, parentId) => (await knowledge.createKnowledgeFolder({ name, parentId })).folder!,
  update: async (id, name, parentId) => (await knowledge.updateKnowledgeFolder({ id, name, parentId })).folder!,
};

// Knowledge is the Knowledge page (after Chat; the user, 2026-10-28): the
// tower's knowledge (KnowledgeService). A knowledge is a graph of the
// markdown files in a folder, a storage made with it, of its name, at the
// path given; a click opens its graph (KnowledgeGraph). The page shows one
// folder (?folder=<id>; none is the top), its folders first, with a
// breadcrumb back up, as the Workflow page does (../folders.tsx). New
// knowledge and New folder make them there; Move to... (a row's menu) or
// dragging a knowledge onto a folder, or onto the breadcrumb, moves it;
// Delete deletes a knowledge and its storage, and leaves its files.
export function Knowledge() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const folder = Math.max(0, Number(params.get('folder')) || 0);
  const [list, setList] = useState<TowerKnowledge[]>();
  const [folders, setFolders] = useState<KnowledgeFolder[]>();
  const [dataDir, setDataDir] = useState('');
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const [creating, setCreating] = useState(false);
  const [moving, setMoving] = useState<TowerKnowledge>();
  const [deleting, setDeleting] = useState<TowerKnowledge>();
  const [folderEdit, setFolderEdit] = useState<KnowledgeFolder | 'new'>();
  const [folderMove, setFolderMove] = useState<KnowledgeFolder>();
  const [folderDelete, setFolderDelete] = useState<KnowledgeFolder>();
  const changed = () => setReload((n) => n + 1);
  const openFolder = (id: number) => setParams(id ? { folder: String(id) } : {});

  useEffect(() => {
    let stale = false;
    Promise.all([knowledge.listKnowledge({ folderId: folder }), knowledge.listKnowledgeFolders({})]).then(
      ([r, f]) => {
        if (stale) return;
        setList(r.knowledge);
        setDataDir(r.dataDir);
        setFolders(f.folders);
        setError('');
      },
      (err: unknown) => !stale && setError(errorText(err)),
    );
    return () => {
      stale = true;
    };
  }, [folder, reload]);

  // A folder that is gone shows the top.
  useEffect(() => {
    if (folders && folder && !folders.some((f) => f.id === folder)) openFolder(0);
  });

  const inside = (folders ?? []).filter((f) => f.parentId === folder);
  const folderName = (id: number) =>
    id ? (folderPath(folders ?? [], id).at(-1)?.name ?? 'a folder') : knowledgeFolders.top;
  const nameOf = (id: number) => list?.find((k) => k.id === id)?.name;

  const move = async (id: number, to: number, name?: string) => {
    try {
      await knowledge.moveKnowledge({ ids: [id], folderId: to });
      toast(`Moved ${name ?? 'the knowledge'} to ${folderName(to)}`);
      changed();
    } catch (err) {
      toast(`Not moved: ${errorText(err)}`, true);
    }
  };

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Knowledge</Heading>
        <HStack gap="2">
          <Button size="sm" variant="outline" onClick={() => setFolderEdit('new')}>
            <FolderPlusIcon />
            New folder
          </Button>
          <Button size="sm" colorPalette="blue" onClick={() => setCreating(true)}>
            New knowledge
          </Button>
        </HStack>
      </HStack>
      <Text color="fg.muted" textStyle="sm">
        A knowledge is a graph of the markdown files in a folder on this computer: every file a doc, every heading a
        section, with the summaries Claude writes. Its folder is a storage of its name, made with it.
      </Text>
      {folders && (
        <FolderCrumbs
          api={knowledgeFolders}
          folders={folders}
          folder={folder}
          onOpen={openFolder}
          onDrop={(id, to) => void move(id, to, nameOf(id))}
        />
      )}
      {error ? (
        <Text color="fg.error">The tower does not answer: {error}</Text>
      ) : !list || !folders ? (
        <Text color="fg.muted">Loading...</Text>
      ) : list.length === 0 && inside.length === 0 ? (
        <Text color="fg.muted">
          {folder
            ? 'This folder is empty: New knowledge makes one here, and a knowledge dragged onto a folder moves there.'
            : 'No knowledge yet.'}
        </Text>
      ) : (
        <Table.ScrollArea borderWidth="1px" rounded="md">
          {/* Fixed layout, so a long path wraps instead of pushing the other columns out. */}
          <Table.Root size="sm" interactive tableLayout="fixed">
            <Table.Header>
              <Table.Row>
                <Table.ColumnHeader w="12" />
                <Table.ColumnHeader w="56">Name</Table.ColumnHeader>
                <Table.ColumnHeader>Path</Table.ColumnHeader>
                <Table.ColumnHeader w="48">Last change</Table.ColumnHeader>
                <Table.ColumnHeader w="12" />
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {inside.map((f) => (
                <FolderRow
                  key={`f${f.id}`}
                  folder={f}
                  counts={folderCounts(f.knowledgeCount, 'knowledge', 'knowledge', f.folderCount)}
                  align="start"
                  onOpen={() => openFolder(f.id)}
                  onDrop={(id) => void move(id, f.id, nameOf(id))}
                  onRename={() => setFolderEdit(f)}
                  onMove={() => setFolderMove(f)}
                  onDelete={() => setFolderDelete(f)}
                />
              ))}
              {list.map((k) => (
                <Table.Row
                  key={k.id}
                  cursor="pointer"
                  onClick={() => navigate(`/knowledge/${k.id}`)}
                  {...draggable(k.id)}
                >
                  <Table.Cell color="fg.muted">
                    <BrainIcon size="18" />
                  </Table.Cell>
                  <Table.Cell fontWeight="semibold">{k.name}</Table.Cell>
                  {/* A relative path says where it is under it; long paths wrap. */}
                  <Table.Cell fontFamily="mono" whiteSpace="normal" wordBreak="break-all">
                    {k.storage ? (
                      <>
                        {k.storage.path}
                        {k.storage.absolutePath !== k.storage.path && (
                          <Text textStyle="xs" color="fg.muted">
                            {k.storage.absolutePath}
                          </Text>
                        )}
                      </>
                    ) : (
                      <Text fontFamily="body" color="fg.error">
                        Its storage is gone.
                      </Text>
                    )}
                  </Table.Cell>
                  <Table.Cell color="fg.muted" whiteSpace="nowrap">
                    {k.updatedAt && timestampDate(k.updatedAt).toLocaleString()}
                  </Table.Cell>
                  {/* The menu is the row's own: a click in it does not open the graph. */}
                  <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
                    <RowMenu name={k.name} items={{ move: () => setMoving(k), delete: () => setDeleting(k) }} />
                  </Table.Cell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table.Root>
        </Table.ScrollArea>
      )}
      {creating && (
        <CreateKnowledge
          folder={folder}
          where={folderName(folder)}
          dataDir={dataDir}
          onClose={() => setCreating(false)}
          onCreated={changed}
        />
      )}
      {moving && folders && (
        <MoveDialog
          api={knowledgeFolders}
          name={moving.name}
          folders={folders}
          from={moving.folderId}
          onClose={() => setMoving(undefined)}
          onMove={(to) => move(moving.id, to, moving.name)}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete knowledge ${deleting.name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await knowledge.deleteKnowledge({ id: deleting.id });
            forgetStorages();
            forgetKnowledge();
            toast(`Deleted knowledge ${deleting.name}`);
            changed();
          }}
        >
          {deleting.storage ? (
            <Text>
              Its storage {deleting.storage.name} goes with it; the folder <Code>{deleting.storage.absolutePath}</Code>{' '}
              and its files stay as they are. Workflow steps that use the storage fail from then on.
            </Text>
          ) : (
            <Text>Its storage is gone already.</Text>
          )}
        </FormDialog>
      )}
      {folderEdit && (
        <FolderDialog
          api={knowledgeFolders}
          folder={folderEdit === 'new' ? undefined : folderEdit}
          parent={folder}
          onClose={() => setFolderEdit(undefined)}
          onSaved={(f) => {
            toast(folderEdit === 'new' ? `Created folder ${f.name}` : `Renamed the folder to ${f.name}`);
            changed();
          }}
        />
      )}
      {folderMove && folders && (
        <MoveDialog
          api={knowledgeFolders}
          name={`folder ${folderMove.name}`}
          folders={folders}
          from={folderMove.parentId}
          folder={folderMove}
          onClose={() => setFolderMove(undefined)}
          onMove={async (to) => {
            await knowledgeFolders.update(folderMove.id, folderMove.name, to);
            toast(`Moved folder ${folderMove.name} to ${folderName(to)}`);
            changed();
          }}
        />
      )}
      {folderDelete && (
        <FormDialog
          title={`Delete folder ${folderDelete.name}?`}
          submitText="Delete folder"
          danger
          onClose={() => setFolderDelete(undefined)}
          onSubmit={async () => {
            await knowledge.deleteKnowledgeFolder({ id: folderDelete.id });
            toast(`Deleted folder ${folderDelete.name}`);
            changed();
          }}
        >
          <Text>
            Its {folderDelete.knowledgeCount} knowledge and {folderDelete.folderCount} folders move to{' '}
            {folderName(folderDelete.parentId)}. No knowledge is deleted.
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}

// CreateKnowledge makes a knowledge in folder, and its storage at the path
// typed.
function CreateKnowledge({
  folder,
  where,
  dataDir,
  onClose,
  onCreated,
}: {
  folder: number;
  where: string;
  dataDir: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState('');
  const [path, setPath] = useState('');
  return (
    <FormDialog
      title={folder ? `New knowledge in ${where}` : 'New knowledge'}
      submitText="Create"
      onClose={onClose}
      onSubmit={async () => {
        const { knowledge: k } = await knowledge.createKnowledge({ name: name.trim(), path: path.trim(), folderId: folder });
        forgetStorages();
        forgetKnowledge();
        toast(`Created knowledge ${k!.name}: ${k!.storage?.absolutePath}`);
        onCreated();
      }}
    >
      <FormField label="Name" help="Unique, at most 100 characters. Its storage takes this name too.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Marketing" />
      </FormField>
      <FormField
        label="Path"
        help={`The folder of its markdown files: absolute, such as D:/notes, or relative to the tower's data folder${dataDir ? ` (${dataDir})` : ''}, such as knowledge/marketing. The folder is made if it is not there; \\ is turned into /. Its graph is kept in it, in knowledge_data.`}
      >
        <Input
          fontFamily="mono"
          value={path}
          onChange={(e) => setPath(e.currentTarget.value)}
          placeholder="D:/notes or knowledge/marketing"
        />
      </FormField>
    </FormDialog>
  );
}
