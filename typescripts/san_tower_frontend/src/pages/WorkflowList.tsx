import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  Button,
  ButtonGroup,
  HStack,
  Heading,
  IconButton,
  Input,
  InputGroup,
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
  EllipsisVerticalIcon,
  PencilIcon,
  ScrollTextIcon,
  SearchIcon,
  Trash2Icon,
  WorkflowIcon,
} from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router';
import { errorText, workflows } from '../api';
import type { Workflow, WorkflowSummary } from '../gen/san/tower/v1/workflow_pb';
import { FormDialog, FormField, toast } from '../ui';
import { RenameDialog } from '../workflow/RenameDialog';
import { RunLog } from '../workflow/RunLog';
import { savesDone } from './WorkflowEditor';

const pageSize = 20;

// WorkflowList is the Workflow page. Its Workflows tab lists the tower's
// workflows (WorkflowService, in its database) a page at a time, searches
// them by name, creates them, switches their triggers on and off, renames and
// deletes them; the search and the page are in the URL (?q=order&page=2). A
// workflow opens in its editor, /workflow/<id>. Its Logs tab (?tab=logs,
// RunLog) watches the runs of the active workflows as they come.
export function WorkflowList() {
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const tab = params.get('tab') === 'logs' ? 'logs' : 'workflows';
  const q = params.get('q') ?? '';
  const page = Math.max(1, Number(params.get('page')) || 1);
  const [text, setText] = useState(q);
  const [result, setResult] = useState<{ list: WorkflowSummary[]; total: number }>();
  const [error, setError] = useState('');
  const [reload, setReload] = useState(0);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<WorkflowSummary>();
  const [renaming, setRenaming] = useState<WorkflowSummary>();

  const setPage = (n: number) =>
    setParams((p) => {
      const next = new URLSearchParams(p);
      if (n > 1) next.set('page', String(n));
      else next.delete('page');
      return next;
    });

  // The search box follows the URL (back and forward), and sets it once the
  // typing pauses; a new search starts at the first page.
  useEffect(() => setText(q), [q]);
  useEffect(() => {
    const query = text.trim();
    if (query === q) return;
    const t = setTimeout(() => setParams(query ? { q: query } : {}, { replace: true }), 300);
    return () => clearTimeout(t);
  }, [text, q, setParams]);

  useEffect(() => {
    let stale = false;
    // An editor just closed saves its last change first.
    savesDone()
      .then(() => workflows.listWorkflows({ query: q, offset: (page - 1) * pageSize, limit: pageSize }))
      .then(
        (r) => {
          if (stale) return;
          setResult({ list: r.workflows, total: r.total });
          setError('');
        },
        (err: unknown) => !stale && setError(errorText(err)),
      );
    return () => {
      stale = true;
    };
  }, [q, page, reload]);

  // A page past the end (after a delete, or from an old link) shows the last.
  const last = result ? Math.max(1, Math.ceil(result.total / pageSize)) : 1;
  useEffect(() => {
    if (result && page > last) setPage(last);
  });

  const open = (id: number) => navigate(`/workflow/${id}`);

  const switchActive = async (w: WorkflowSummary, on: boolean) => {
    try {
      await workflows.setWorkflowActive({ id: w.id, active: on });
      setReload((n) => n + 1);
    } catch (err) {
      toast(`${w.name} not switched ${on ? 'on' : 'off'}: ${errorText(err)}`, true);
    }
  };

  return (
    <Stack gap="4">
      <HStack justify="space-between">
        <Heading size="lg">Workflow</Heading>
        <Button size="sm" colorPalette="blue" onClick={() => setCreating(true)}>
          New workflow
        </Button>
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
            <InputGroup maxW="sm" startElement={<SearchIcon size="16" />}>
              <Input
                size="sm"
                value={text}
                onChange={(e) => setText(e.currentTarget.value)}
                placeholder="Search by name"
              />
            </InputGroup>
            {error ? (
              <Text color="fg.error">The tower does not answer: {error}</Text>
            ) : !result ? (
              <Text color="fg.muted">Loading...</Text>
            ) : result.total === 0 ? (
              <Text color="fg.muted">{q ? `No workflow's name contains "${q}".` : 'No workflows yet.'}</Text>
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
                      {result.list.map((w) => (
                        <Table.Row key={w.id} cursor="pointer" onClick={() => open(w.id)}>
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
                          <Table.Cell fontWeight="semibold">{w.name}</Table.Cell>
                          <Table.Cell textAlign="end">{w.stepCount}</Table.Cell>
                          <Table.Cell color="fg.muted">
                            {w.updatedAt && timestampDate(w.updatedAt).toLocaleString()}
                          </Table.Cell>
                          {/* The menu is the row's own: a click in it (its items are
                        in a portal, but React events still bubble here) does
                        not open the workflow. */}
                          <Table.Cell textAlign="end" py="0" onClick={(e) => e.stopPropagation()}>
                            <RowMenu name={w.name} onRename={() => setRenaming(w)} onDelete={() => setDeleting(w)} />
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
      {creating && <CreateWorkflow onClose={() => setCreating(false)} onCreated={(w) => open(w.id)} />}
      {renaming && (
        <RenameDialog
          name={renaming.name}
          onClose={() => setRenaming(undefined)}
          onRename={async (name) => {
            // UpdateWorkflow replaces the name and the graph: send the graph
            // as last saved, after any save still on its way.
            await savesDone();
            const { workflow } = await workflows.getWorkflow({ id: renaming.id });
            await workflows.updateWorkflow({ workflow: { ...workflow!, name } });
            toast(`Renamed ${renaming.name} to ${name}`);
            setReload((n) => n + 1);
          }}
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
            setReload((n) => n + 1);
          }}
        >
          <Text>Its steps, links, runs and triggers go with it. This cannot be undone.</Text>
        </FormDialog>
      )}
    </Stack>
  );
}

// RowMenu is a workflow's actions, behind the three dots at the end of its
// row.
function RowMenu({ name, onRename, onDelete }: { name: string; onRename: () => void; onDelete: () => void }) {
  return (
    <Menu.Root
      positioning={{ placement: 'bottom-end' }}
      onSelect={(e) => (e.value === 'rename' ? onRename() : e.value === 'delete' && onDelete())}
    >
      <Menu.Trigger asChild>
        <IconButton aria-label={`Actions for ${name}`} title="Actions" size="xs" variant="ghost" color="fg.muted">
          <EllipsisVerticalIcon />
        </IconButton>
      </Menu.Trigger>
      <Portal>
        <Menu.Positioner>
          <Menu.Content minW="36">
            <Menu.Item value="rename">
              <PencilIcon />
              Rename
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

function CreateWorkflow({ onClose, onCreated }: { onClose: () => void; onCreated: (w: Workflow) => void }) {
  const [name, setName] = useState('');
  return (
    <FormDialog
      title="New workflow"
      submitText="Create"
      onClose={onClose}
      onSubmit={async () => {
        const { workflow } = await workflows.createWorkflow({ name: name.trim() });
        toast(`Created workflow ${workflow!.name}`);
        onCreated(workflow!);
      }}
    >
      <FormField label="Name" help="Unique, at most 100 characters.">
        <Input autoFocus value={name} onChange={(e) => setName(e.currentTarget.value)} placeholder="Big order alert" />
      </FormField>
    </FormDialog>
  );
}
