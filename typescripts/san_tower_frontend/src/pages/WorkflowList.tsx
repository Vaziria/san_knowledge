import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  Button,
  ButtonGroup,
  HStack,
  Heading,
  IconButton,
  Input,
  InputGroup,
  Link,
  Menu,
  Pagination,
  Portal,
  Stack,
  Switch,
  Table,
  Tabs,
  Text,
} from '@chakra-ui/react';
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  DownloadIcon,
  EllipsisVerticalIcon,
  FolderIcon,
  FolderInputIcon,
  FolderPlusIcon,
  PencilIcon,
  ScrollTextIcon,
  SearchIcon,
  Trash2Icon,
  UploadIcon,
  WorkflowIcon,
} from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { errorText, workflows } from '../api';
import type { Folder, Workflow, WorkflowSummary } from '../gen/san/tower/v1/workflow_pb';
import { FormDialog, FormField, toast } from '../ui';
import { dragType, dropTarget, FolderCrumbs, FolderDialog, folderPath, MoveDialog, pathText } from '../workflow/folders';
import { DescriptionField, DetailsDialog } from '../workflow/DetailsDialog';
import { RunLog } from '../workflow/RunLog';
import { exportWorkflows, ImportResult, importFiles, type Imported } from '../workflow/transfer';
import { savesDone } from './WorkflowEditor';

const pageSize = 20;

// WorkflowList is the Workflow page. Its Workflows tab lists the tower's
// workflows (WorkflowService, in its database) a page at a time, searches
// them by name, creates them, switches their triggers on and off, renames and
// deletes them, and exports and imports them as files (transfer.tsx); the
// search and the page are in the URL (?q=order&page=2). A
// workflow opens in its editor, /workflow/<id>. Its Logs tab (?tab=logs,
// RunLog) watches the runs of the active workflows as they come.
//
// Workflows are in folders (folders.tsx; the user, 2026-10-27): the tab
// shows one folder (?folder=<id>; none is the top), its folders first, with
// a breadcrumb back up. New workflow and New folder make them there; Move
// to... (a row's menu) or dragging a workflow onto a folder, or onto the
// breadcrumb, moves it. A search looks in every folder and says where each
// workflow is.
export function WorkflowList() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'logs' ? 'logs' : 'workflows';
  const q = params.get('q') ?? '';
  const page = Math.max(1, Number(params.get('page')) || 1);
  const folder = Math.max(0, Number(params.get('folder')) || 0);
  const [text, setText] = useState(q);
  const [result, setResult] = useState<{ list: WorkflowSummary[]; total: number }>();
  const [folders, setFolders] = useState<Folder[]>();
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<WorkflowSummary>();
  const [renaming, setRenaming] = useState<WorkflowSummary>();
  const [moving, setMoving] = useState<WorkflowSummary>();
  const [folderEdit, setFolderEdit] = useState<Folder | 'new'>();
  const [folderMove, setFolderMove] = useState<Folder>();
  const [folderDelete, setFolderDelete] = useState<Folder>();
  const [importing, setImporting] = useState(false);
  const [imported, setImported] = useState<Imported>();
  const fileInput = useRef<HTMLInputElement>(null);
  const changed = () => setReload((n) => n + 1);

  // param sets one search parameter, keeping the others, a new page at 1.
  const param = (key: string, value: string, replace = false) =>
    setParams(
      (p) => {
        const next = new URLSearchParams(p);
        if (value) next.set(key, value);
        else next.delete(key);
        if (key !== 'page') next.delete('page');
        return next;
      },
      { replace },
    );
  const setPage = (n: number) => param('page', n > 1 ? String(n) : '');
  const openFolder = (id: number) => {
    setText('');
    setParams(id ? { folder: String(id) } : {});
  };

  // The search box follows the URL (back and forward), and sets it once the
  // typing pauses; a new search starts at the first page.
  useEffect(() => setText(q), [q]);
  useEffect(() => {
    const query = text.trim();
    if (query === q) return;
    const t = setTimeout(() => param('q', query, true), 300);
    return () => clearTimeout(t);
  }, [text, q]);

  useEffect(() => {
    let stale = false;
    // An editor just closed saves its last change first.
    savesDone()
      .then(() =>
        Promise.all([
          workflows.listWorkflows({
            query: q,
            offset: (page - 1) * pageSize,
            limit: pageSize,
            folderId: folder,
            allFolders: q !== '',
          }),
          workflows.listFolders({}),
        ]),
      )
      .then(
        ([r, f]) => {
          if (stale) return;
          setResult({ list: r.workflows, total: r.total });
          setFolders(f.folders);
          setError('');
        },
        (err: unknown) => !stale && setError(errorText(err)),
      );
    return () => {
      stale = true;
    };
  }, [q, page, folder, reload]);

  // A page past the end (after a delete, or from an old link) shows the
  // last; a folder that is gone shows the top.
  const last = result ? Math.max(1, Math.ceil(result.total / pageSize)) : 1;
  useEffect(() => {
    if (result && page > last) setPage(last);
    if (folders && folder && !folders.some((f) => f.id === folder)) openFolder(0);
  });

  const open = (id: number) => navigate(`/workflow/${id}`);
  const inside = q || page > 1 ? [] : (folders ?? []).filter((f) => f.parentId === folder);
  const folderName = (id: number) => (id ? (folderPath(folders ?? [], id).at(-1)?.name ?? 'a folder') : 'All workflows');

  const switchActive = async (w: WorkflowSummary, on: boolean) => {
    try {
      await workflows.setWorkflowActive({ id: w.id, active: on });
      changed();
    } catch (err) {
      toast(`${w.name} not switched ${on ? 'on' : 'off'}: ${errorText(err)}`, true);
    }
  };

  const moveWorkflow = async (id: number, to: number, name?: string) => {
    try {
      await workflows.moveWorkflows({ ids: [id], folderId: to });
      toast(`Moved ${name ?? 'the workflow'} to ${folderName(to)}`);
      changed();
    } catch (err) {
      toast(`Not moved: ${errorText(err)}`, true);
    }
  };
  const nameOf = (id: number) => result?.list.find((w) => w.id === id)?.name;

  // Export saves what an editor just closed has not saved yet first: the
  // file is the workflows as stored.
  const exportSome = async (ids: number[]) => {
    try {
      await savesDone();
      toast(`Exported to ${await exportWorkflows(ids)}`);
    } catch (err) {
      toast(`Not exported: ${errorText(err)}`, true);
    }
  };

  const importPicked = async (files: File[]) => {
    if (files.length === 0) return;
    setImporting(true);
    const result = await importFiles(files);
    setImporting(false);
    setImported(result);
    changed();
  };

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Workflow</Heading>
        <HStack gap="2">
          <input
            ref={fileInput}
            type="file"
            accept=".json,application/json"
            multiple
            hidden
            onChange={(e) => {
              const files = [...(e.currentTarget.files ?? [])];
              e.currentTarget.value = ''; // the same file can be picked again
              void importPicked(files);
            }}
          />
          <Button
            size="sm"
            variant="outline"
            loading={importing}
            title="Make workflows from files another tower, or this one, exported"
            onClick={() => fileInput.current?.click()}
          >
            <UploadIcon />
            Import
          </Button>
          <Button
            size="sm"
            variant="outline"
            disabled={!result?.total && !folders?.length}
            title="Download every workflow as one file"
            onClick={() => void exportSome([])}
          >
            <DownloadIcon />
            Export all
          </Button>
          <Button size="sm" colorPalette="blue" onClick={() => setCreating(true)}>
            New workflow
          </Button>
        </HStack>
      </HStack>
      <Tabs.Root
        value={tab}
        onValueChange={(e) => setParams(e.value === 'logs' ? { tab: 'logs' } : {})}
        lazyMount
        unmountOnExit
      >
        <Tabs.List>
          <Tabs.Trigger value="workflows">
            <WorkflowIcon size="16" />
            Workflows
          </Tabs.Trigger>
          <Tabs.Trigger value="logs">
            <ScrollTextIcon size="16" />
            Logs
          </Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="logs">
          <RunLog />
        </Tabs.Content>
        <Tabs.Content value="workflows">
          <Stack gap="4">
            <HStack gap="2" wrap="wrap">
              <InputGroup maxW="sm" startElement={<SearchIcon size="16" />}>
                <Input
                  size="sm"
                  value={text}
                  onChange={(e) => setText(e.currentTarget.value)}
                  placeholder="Search by name or description, in every folder"
                />
              </InputGroup>
              <Button size="sm" variant="outline" onClick={() => setFolderEdit('new')}>
                <FolderPlusIcon />
                New folder
              </Button>
            </HStack>
            {!q && folders && (
              <FolderCrumbs
                folders={folders}
                folder={folder}
                onOpen={openFolder}
                onDropWorkflow={(id, to) => void moveWorkflow(id, to, nameOf(id))}
              />
            )}
            {error ? (
              <Text color="fg.error">The tower does not answer: {error}</Text>
            ) : !result || !folders ? (
              <Text color="fg.muted">Loading...</Text>
            ) : result.total === 0 && inside.length === 0 ? (
              <Text color="fg.muted">
                {q
                  ? `No workflow's name contains "${q}".`
                  : folder
                    ? 'This folder is empty: New workflow makes one here, and a workflow dragged onto a folder moves there.'
                    : 'No workflows yet.'}
              </Text>
            ) : (
              <>
                <Table.ScrollArea borderWidth="1px" rounded="md">
                  <Table.Root size="sm" interactive>
                    <Table.Header>
                      <Table.Row>
                        <Table.ColumnHeader w="20">Active</Table.ColumnHeader>
                        <Table.ColumnHeader>Name</Table.ColumnHeader>
                        <Table.ColumnHeader textAlign="end">Steps</Table.ColumnHeader>
                        <Table.ColumnHeader>Last change</Table.ColumnHeader>
                        <Table.ColumnHeader w="12" />
                      </Table.Row>
                    </Table.Header>
                    <Table.Body>
                      {inside.map((f) => (
                        <FolderRow
                          key={`f${f.id}`}
                          folder={f}
                          onOpen={() => openFolder(f.id)}
                          onDropWorkflow={(id) => void moveWorkflow(id, f.id, nameOf(id))}
                          onRename={() => setFolderEdit(f)}
                          onMove={() => setFolderMove(f)}
                          onDelete={() => setFolderDelete(f)}
                        />
                      ))}
                      {result.list.map((w) => (
                        <Table.Row
                          key={w.id}
                          cursor="pointer"
                          onClick={() => open(w.id)}
                          draggable
                          onDragStart={(e) => {
                            e.dataTransfer.setData(dragType, String(w.id));
                            e.dataTransfer.effectAllowed = 'move';
                          }}
                        >
                          {/* A control's cell: a click in it does not open the workflow. */}
                          <Table.Cell onClick={(e) => e.stopPropagation()}>
                            <Switch.Root
                              size="sm"
                              colorPalette="green"
                              checked={w.active}
                              onCheckedChange={(e) => void switchActive(w, e.checked)}
                              aria-label={`${w.name} active`}
                            >
                              <Switch.HiddenInput />
                              <Switch.Control>
                                <Switch.Thumb />
                              </Switch.Control>
                            </Switch.Root>
                          </Table.Cell>
                          <Table.Cell maxW="xl">
                            <Text fontWeight="semibold">{w.name}</Text>
                            {w.description ? (
                              // A click on it edits it; the rest of the row
                              // opens the workflow.
                              <Text
                                color="fg.muted"
                                textStyle="xs"
                                truncate
                                cursor="text"
                                _hover={{ color: 'fg', textDecoration: 'underline dotted' }}
                                title={`${w.description}\n(click to edit)`}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setRenaming(w);
                                }}
                              >
                                {w.description}
                              </Text>
                            ) : (
                              // Without one, a way to write it: the row itself
                              // opens the workflow.
                              <Link
                                as="button"
                                color="fg.subtle"
                                textStyle="xs"
                                _hover={{ color: 'fg.muted' }}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  setRenaming(w);
                                }}
                              >
                                Add a description
                              </Link>
                            )}
                            {q && w.folderId > 0 && (
                              <HStack gap="1" color="fg.muted" textStyle="xs">
                                <FolderIcon size="12" />
                                {pathText(folders, w.folderId)}
                              </HStack>
                            )}
                          </Table.Cell>
                          <Table.Cell textAlign="end">{w.stepCount}</Table.Cell>
                          <Table.Cell color="fg.muted">
                            {w.updatedAt && timestampDate(w.updatedAt).toLocaleString()}
                          </Table.Cell>
                          {/* The menu is the row's own: a click in it (its items are
                        in a portal, but React events still bubble here) does
                        not open the workflow. */}
                          <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
                            <RowMenu
                              name={w.name}
                              items={{
                                rename: () => setRenaming(w),
                                move: () => setMoving(w),
                                export: () => void exportSome([w.id]),
                                delete: () => setDeleting(w),
                              }}
                            />
                          </Table.Cell>
                        </Table.Row>
                      ))}
                    </Table.Body>
                  </Table.Root>
                </Table.ScrollArea>
                {result.total > pageSize && (
                  <Pagination.Root
                    count={result.total}
                    pageSize={pageSize}
                    page={Math.min(page, last)}
                    onPageChange={(e) => setPage(e.page)}
                  >
                    <HStack justify="space-between">
                      <Pagination.PageText format="long" color="fg.muted" textStyle="sm" />
                      <ButtonGroup variant="ghost" size="sm">
                        <Pagination.PrevTrigger asChild>
                          <IconButton aria-label="Previous page">
                            <ChevronLeftIcon />
                          </IconButton>
                        </Pagination.PrevTrigger>
                        <Pagination.Items
                          render={(p) => (
                            <IconButton variant={{ base: 'ghost', _selected: 'outline' }}>{p.value}</IconButton>
                          )}
                        />
                        <Pagination.NextTrigger asChild>
                          <IconButton aria-label="Next page">
                            <ChevronRightIcon />
                          </IconButton>
                        </Pagination.NextTrigger>
                      </ButtonGroup>
                    </HStack>
                  </Pagination.Root>
                )}
              </>
            )}
          </Stack>
        </Tabs.Content>
      </Tabs.Root>
      {creating && (
        <CreateWorkflow
          folder={folder}
          where={folderName(folder)}
          onClose={() => setCreating(false)}
          onCreated={(w) => open(w.id)}
        />
      )}
      {imported && <ImportResult result={imported} onOpen={open} onClose={() => setImported(undefined)} />}
      {renaming && (
        <DetailsDialog
          details={renaming}
          onClose={() => setRenaming(undefined)}
          onSave={async (next) => {
            // UpdateWorkflow replaces the settings and the graph: send the
            // graph as last saved, after any save still on its way.
            await savesDone();
            const { workflow } = await workflows.getWorkflow({ id: renaming.id });
            await workflows.updateWorkflow({ workflow: { ...workflow!, ...next } });
            toast(`Saved ${next.name}`);
            changed();
          }}
        />
      )}
      {moving && folders && (
        <MoveDialog
          name={moving.name}
          folders={folders}
          from={moving.folderId}
          onClose={() => setMoving(undefined)}
          onMove={(to) => moveWorkflow(moving.id, to, moving.name)}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete workflow ${deleting.name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(undefined)}
          onSubmit={async () => {
            await workflows.deleteWorkflow({ id: deleting.id });
            toast(`Deleted workflow ${deleting.name}`);
            changed();
          }}
        >
          <Text>Its steps, links, runs and triggers go with it. This cannot be undone.</Text>
        </FormDialog>
      )}
      {folderEdit && (
        <FolderDialog
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
          name={`folder ${folderMove.name}`}
          folders={folders}
          from={folderMove.parentId}
          folder={folderMove}
          onClose={() => setFolderMove(undefined)}
          onMove={async (to) => {
            await workflows.updateFolder({ id: folderMove.id, name: folderMove.name, parentId: to });
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
            await workflows.deleteFolder({ id: folderDelete.id });
            toast(`Deleted folder ${folderDelete.name}`);
            changed();
          }}
        >
          <Text>
            Its {folderDelete.workflowCount} workflows and {folderDelete.folderCount} folders move to{' '}
            {folderName(folderDelete.parentId)}. No workflow is deleted.
          </Text>
        </FormDialog>
      )}
    </Stack>
  );
}

// FolderRow is a folder in the list: a click opens it, and a workflow
// dragged onto it moves there.
function FolderRow({
  folder,
  onOpen,
  onDropWorkflow,
  onRename,
  onMove,
  onDelete,
}: {
  folder: Folder;
  onOpen: () => void;
  onDropWorkflow: (workflowId: number) => void;
  onRename: () => void;
  onMove: () => void;
  onDelete: () => void;
}) {
  const [over, setOver] = useState(false);
  const counts = [
    `${folder.workflowCount} ${folder.workflowCount === 1 ? 'workflow' : 'workflows'}`,
    folder.folderCount > 0 && `${folder.folderCount} ${folder.folderCount === 1 ? 'folder' : 'folders'}`,
  ]
    .filter(Boolean)
    .join(', ');
  return (
    <Table.Row
      cursor="pointer"
      onClick={onOpen}
      bg={over ? 'bg.emphasized' : undefined}
      {...dropTarget(onDropWorkflow, setOver)}
    >
      <Table.Cell color="fg.muted">
        <FolderIcon size="18" />
      </Table.Cell>
      <Table.Cell fontWeight="semibold">{folder.name}</Table.Cell>
      <Table.Cell textAlign="end" color="fg.muted" whiteSpace="nowrap">
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
// given of Rename, Move to..., Export and Delete.
function RowMenu({ name, items }: { name: string; items: Partial<Record<'rename' | 'move' | 'export' | 'delete', () => void>> }) {
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

function CreateWorkflow({
  folder,
  where,
  onClose,
  onCreated,
}: {
  folder: number;
  where: string;
  onClose: () => void;
  onCreated: (w: Workflow) => void;
}) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  return (
    <FormDialog
      title={folder ? `New workflow in ${where}` : 'New workflow'}
      submitText="Create"
      onClose={onClose}
      onSubmit={async () => {
        const { workflow } = await workflows.createWorkflow({
          name: name.trim(),
          folderId: folder,
          description: description.trim(),
        });
        toast(`Created workflow ${workflow!.name}`);
        onCreated(workflow!);
      }}
    >
      <FormField label="Name" help="Unique, at most 100 characters.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Big order alert" />
      </FormField>
      <DescriptionField value={description} onChange={setDescription} />
    </FormDialog>
  );
}
