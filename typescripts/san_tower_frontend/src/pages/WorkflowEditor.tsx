import { timestampDate } from '@bufbuild/protobuf/wkt';
import {
  Badge,
  Box,
  Button,
  CloseButton,
  HStack,
  Heading,
  IconButton,
  Menu,
  NativeSelect,
  Portal,
  Stack,
  Switch,
  Tabs,
  Text,
  Textarea,
} from '@chakra-ui/react';
import {
  Background,
  BackgroundVariant,
  Controls,
  MiniMap,
  Panel,
  ReactFlow,
  ReactFlowProvider,
  SelectionMode,
  useEdgesState,
  useNodesState,
  useReactFlow,
  type Connection,
  type XYPosition,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  ArrowLeftIcon,
  DownloadIcon,
  HistoryIcon,
  PencilIcon,
  PlayIcon,
  PlusIcon,
  Redo2Icon,
  ScrollTextIcon,
  Undo2Icon,
  WorkflowIcon,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent, type ReactNode } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router';
import { errorText, workflows } from '../api';
import {
  RunStatus,
  type Run,
  type Workflow as StoredWorkflow,
  type TriggerState,
} from '../gen/san/tower/v1/workflow_pb';
import { FormDialog, FormField, toast } from '../ui';
import {
  canLink,
  fromFlow,
  isTrigger,
  link,
  loadStepKinds,
  newStep,
  stepKind,
  toFlow,
  type Graph,
  type Step,
} from '../workflow/catalog';
import { useHistory } from '../workflow/history';
import { runStatus, StepRunContext } from '../workflow/runs';
import { RunLog } from '../workflow/RunLog';
import { RunsPanel } from '../workflow/RunsPanel';
import { AddPalette } from '../workflow/AddPalette';
import { type Details, DetailsDialog } from '../workflow/DetailsDialog';
import { exportWorkflows } from '../workflow/transfer';
import { StepNode } from '../workflow/StepNode';

const nodeTypes = { step: StepNode };

// saves runs the editors' saves one at a time, in order. It outlives an
// editor, so the list can wait for the save made when one closes.
let saves = Promise.resolve();
export const savesDone = () => saves;

// WorkflowEditor edits one workflow (WorkflowService, in the tower's
// database), whose id is in the path (/workflow/3), and runs it: by hand
// (Run), or by its triggers once it is switched on (Active). Edits save by
// themselves. It has three tabs (the user, 2026-10-25): Workflow, the canvas,
// Runs (?tab=runs), its runs, and Logs (?tab=logs), its runs as they come
// with the lines they logged.
export function WorkflowEditor() {
  const id = Number(useParams().id) || 0;
  const navigate = useNavigate();
  const [workflow, setWorkflow] = useState<StoredWorkflow>();
  const [error, setError] = useState('');
  useEffect(() => {
    setWorkflow(undefined);
    setError('');
    // The steps are drawn from the kinds the tower has.
    Promise.all([workflows.getWorkflow({ id }), loadStepKinds()]).then(
      ([r]) => setWorkflow(r.workflow),
      (err: unknown) => setError(errorText(err)),
    );
  }, [id]);

  // Back goes to the folder the workflow is in.
  const back = workflow?.folderId ? `/workflow?folder=${workflow.folderId}` : '/workflow';
  return (
    <Stack gap="3" h="calc(100vh - 3rem)">
      <Button asChild size="sm" variant="ghost" color="fg.muted" alignSelf="flex-start" ms="-3">
        <Link to={back}>
          <ArrowLeftIcon />
          Workflows
        </Link>
      </Button>
      {error ? (
        <Text color="fg.error">{error}</Text>
      ) : !workflow ? (
        <Text color="fg.muted">Loading...</Text>
      ) : (
        <ReactFlowProvider key={id}>
          <Canvas workflow={workflow} onDeleted={() => navigate(back)} />
        </ReactFlowProvider>
      )}
    </Stack>
  );
}

type SaveState = { state: 'saved' | 'unsaved' | 'saving' } | { state: 'error'; message: string };

// MenuTarget is what the pointer menu is for: some steps, or a link
// (right-click). Adding a step has its own menu, AddPalette.
type MenuTarget = { type: 'steps'; ids: string[] } | { type: 'link'; id: string };

// Canvas is the editor, made to look and handle like Blender's node editors:
// Shift+A adds a step at the pointer, links go from an output socket to an
// input of its type, an input takes one link (a new one replaces it),
// right-clicking a step or a link opens its menu, dragging on empty space
// pans (Shift+drag box-selects) and the wheel zooms. Ctrl+Z undoes a change,
// Ctrl+Shift+Z or Ctrl+Y redoes it. A change is saved once the editing
// pauses, and when the editor closes. Run, and
// switching it on, first save what is not saved: the tower runs the saved
// steps. The canvas stays mounted while the other tabs show, so nothing typed
// or undoable is lost, and the run picked in Runs marks its steps on it.
function Canvas({ workflow, onDeleted }: { workflow: StoredWorkflow; onDeleted: () => void }) {
  const [start] = useState(() => toFlow(workflow));
  const [nodes, setNodes, onNodesChange] = useNodesState<Step>(start.nodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(start.edges);
  const { screenToFlowPosition, deleteElements, getNodes, getEdges } = useReactFlow<Step>();
  const canvas = useRef<HTMLDivElement>(null);
  const pointer = useRef<XYPosition | null>(null);
  const [menu, setMenu] = useState<{ at: XYPosition; target: MenuTarget } | null>(null);
  // palette is the open Add menu: where it shows, and where the step goes
  // (null: the middle of the canvas).
  const [palette, setPalette] = useState<{ at: XYPosition; place: XYPosition | null } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [active, setActive] = useState(workflow.active);
  const [triggers, setTriggers] = useState(workflow.triggers);
  const [running, setRunning] = useState(false);
  const [params, setParams] = useSearchParams();
  const tab = (['runs', 'logs'] as const).find((t) => t === params.get('tab')) ?? 'workflow';
  const [picked, setPicked] = useState<Run>();
  const [runsReload, setRunsReload] = useState(0);
  const pickedSteps = useMemo(() => new Map(picked?.steps.map((s) => [s.stepId, s])), [picked]);
  // While steps are selected, their links stand out and the others fade
  // (the user, 2026-10-25). The class is set on what is drawn, not saved,
  // and only when the selection or the links change, not on every drag.
  const selectedSteps = nodes
    .filter((n) => n.selected)
    .map((n) => n.id)
    .join(' ');
  const shownEdges = useMemo(() => {
    const ids = new Set(selectedSteps.split(' ').filter(Boolean));
    if (ids.size === 0) return edges.map((e) => ({ ...e, className: undefined }));
    return edges.map((e) => ({ ...e, className: ids.has(e.source) || ids.has(e.target) ? 'linked' : 'faded' }));
  }, [edges, selectedSteps]);

  // Saving: the graph as JSON is compared with the last one saved. Saves run
  // one at a time, in order (saves), and each sends the latest graph.
  const graph = useMemo(() => JSON.stringify(fromFlow(nodes, edges)), [nodes, edges]);
  const latest = useRef(graph);
  latest.current = graph;
  const saved = useRef(graph);
  // Undo and redo put back a recorded graph. Steps and links that stay keep
  // their size and selection; the change saves like any other.
  const history = useHistory(
    graph,
    nodes.some((n) => n.dragging),
    (g) => {
      const next = toFlow(JSON.parse(g) as Graph);
      const oldNodes = new Map(getNodes().map((n) => [n.id, n]));
      const oldEdges = new Map(getEdges().map((e) => [e.id, e]));
      setNodes(
        next.nodes.map((n) => {
          const old = oldNodes.get(n.id);
          return old ? { ...old, position: n.position, data: n.data } : n;
        }),
      );
      setEdges(next.edges.map((e) => oldEdges.get(e.id) ?? e));
    },
  );
  const { undo, redo } = history;
  const [save, setSave] = useState<SaveState>({ state: 'saved' });
  // The name goes with every save; renaming changes it (rename). The
  // description and one at a time are not sent with the graph's saves,
  // which keep them; the settings dialog sends them (rename).
  const [name, setName] = useState(workflow.name);
  const [description, setDescription] = useState(workflow.description ?? '');
  const [oneAtATime, setOneAtATime] = useState(workflow.oneAtATime ?? false);
  const nameRef = useRef(name);
  const flush = useCallback(() => {
    saves = saves.then(async () => {
      const body = latest.current;
      if (body === saved.current) return;
      setSave({ state: 'saving' });
      try {
        const g = JSON.parse(body) as Graph;
        const r = await workflows.updateWorkflow({ workflow: { id: workflow.id, name: nameRef.current, ...g } });
        setTriggers(r.workflow!.triggers);
        saved.current = body;
        setSave({ state: latest.current === body ? 'saved' : 'unsaved' });
      } catch (err) {
        setSave({ state: 'error', message: errorText(err) });
      }
    });
  }, [workflow.id]);
  // rename saves the graph as it is now with new settings, in line with the
  // other saves. Its error goes to the caller, and later saves still run.
  const [renaming, setRenaming] = useState(false);
  const rename = (next: Details) => {
    const done = saves.then(async () => {
      const body = latest.current;
      const g = JSON.parse(body) as Graph;
      const r = await workflows.updateWorkflow({ workflow: { id: workflow.id, ...next, ...g } });
      nameRef.current = next.name;
      setName(next.name);
      setDescription(r.workflow!.description ?? next.description);
      setOneAtATime(r.workflow!.oneAtATime ?? next.oneAtATime);
      setTriggers(r.workflow!.triggers);
      saved.current = body;
      setSave({ state: latest.current === body ? 'saved' : 'unsaved' });
    });
    saves = done.catch(() => {});
    return done;
  };
  useEffect(() => {
    if (graph === saved.current) return;
    setSave({ state: 'unsaved' });
    const t = setTimeout(flush, 700);
    return () => clearTimeout(t);
  }, [graph, flush]);
  const flushRef = useRef(flush);
  flushRef.current = flush;
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => {
      if (latest.current !== saved.current) e.preventDefault();
    };
    window.addEventListener('beforeunload', warn);
    return () => {
      window.removeEventListener('beforeunload', warn);
      flushRef.current();
    };
  }, []);

  // saveNow saves what is not saved yet, and says whether all is saved.
  const saveNow = async () => {
    flush();
    await savesDone();
    return latest.current === saved.current;
  };

  // exportThis downloads the workflow as a file, saved first: the tower
  // writes it from what is stored.
  const exportThis = async () => {
    try {
      if (!(await saveNow())) throw new Error('the last change is not saved');
      toast(`Exported to ${await exportWorkflows([workflow.id])}`);
    } catch (err) {
      toast(`Not exported: ${errorText(err)}`, true);
    }
  };

  const switchActive = async (on: boolean) => {
    try {
      if (!(await saveNow())) throw new Error('the last change is not saved');
      const { workflow: w } = await workflows.setWorkflowActive({ id: workflow.id, active: on });
      setActive(w!.active);
      setTriggers(w!.triggers);
    } catch (err) {
      toast(`Not switched ${on ? 'on' : 'off'}: ${errorText(err)}`, true);
    }
  };

  // Once a schedule's next time has come, ask for the one after it.
  useEffect(() => {
    const times = triggers.flatMap((t) => (t.next ? [timestampDate(t.next).getTime()] : []));
    if (!active || times.length === 0) return;
    const wait = Math.max(Math.min(...times) - Date.now(), 0) + 1000;
    const t = setTimeout(
      async () => {
        try {
          const { workflow: w } = await workflows.getWorkflow({ id: workflow.id });
          setTriggers(w!.triggers);
        } catch {
          // The next save or switch says what is wrong.
        }
      },
      Math.min(wait, 2 ** 31 - 1),
    );
    return () => clearTimeout(t);
  }, [active, triggers, workflow.id]);

  const run = async (stepId: string, data: string) => {
    if (!(await saveNow())) throw new Error('The last change is not saved, so it would not run.');
    const { run } = await workflows.runWorkflow({
      workflowId: workflow.id,
      stepId,
      data: new TextEncoder().encode(data),
    });
    setPicked(run);
    setRunsReload((n) => n + 1);
    if (run!.status === RunStatus.SUCCEEDED) toast('The run succeeded');
    else toast(`The run failed: ${run!.error}`, true);
  };
  const triggerSteps = nodes.filter((n) => isTrigger(n.data.kind));

  const onConnect = useCallback(
    (c: Connection) =>
      setEdges((es) => [...es.filter((e) => !(e.target === c.target && e.targetHandle === c.targetHandle)), link(c)]),
    [setEdges],
  );

  // add puts a new step with its header under a point on the screen, selected
  // on its own.
  const add = (kind: string, at: XYPosition) => {
    const p = screenToFlowPosition(at);
    const step = { ...newStep(kind, { x: p.x - 112, y: p.y - 12 }), selected: true };
    setNodes((ns) => [...ns.map((n) => ({ ...n, selected: false })), step]);
  };
  const addAtCenter = (kind: string) => {
    const r = canvas.current!.getBoundingClientRect();
    add(kind, { x: r.left + r.width / 2, y: r.top + r.height / 3 });
  };

  // duplicate copies steps, and the links between them, a little down and to
  // the right, and selects the copies.
  const duplicate = (ids: string[]) => {
    const copies = new Map(ids.map((id) => [id, crypto.randomUUID()]));
    setNodes((ns) => [
      ...ns.map((n) => ({ ...n, selected: false })),
      ...ns
        .filter((n) => copies.has(n.id))
        .map((n) => ({
          ...n,
          id: copies.get(n.id)!,
          position: { x: n.position.x + 40, y: n.position.y + 40 },
          data: { ...n.data, values: { ...n.data.values } },
          selected: true,
        })),
    ]);
    setEdges((es) => [
      ...es,
      ...es
        .filter((e) => copies.has(e.source) && copies.has(e.target))
        .map((e) =>
          link({
            source: copies.get(e.source)!,
            sourceHandle: e.sourceHandle ?? null,
            target: copies.get(e.target)!,
            targetHandle: e.targetHandle ?? null,
          }),
        ),
    ]);
  };

  const onMenuSelect = (value: string) => {
    if (!menu) return;
    const { target } = menu;
    if (target.type === 'steps') {
      const ids = new Set(target.ids);
      if (value === 'duplicate') duplicate(target.ids);
      if (value === 'disconnect') setEdges((es) => es.filter((e) => !ids.has(e.source) && !ids.has(e.target)));
      if (value === 'delete') void deleteElements({ nodes: target.ids.map((id) => ({ id })) });
    }
    if (target.type === 'link' && value === 'delete') void deleteElements({ edges: [{ id: target.id }] });
  };

  // A right-click on a selected step is for the whole selection; on another
  // step, for that step alone, which it selects.
  const onStepMenu = (e: MouseEvent, step: Step) => {
    e.preventDefault();
    let ids = [step.id];
    if (step.selected) ids = nodes.filter((n) => n.selected).map((n) => n.id);
    else {
      setNodes((ns) => ns.map((n) => ({ ...n, selected: n.id === step.id })));
      setEdges((es) => es.map((x) => ({ ...x, selected: false })));
    }
    setMenu({ at: { x: e.clientX, y: e.clientY }, target: { type: 'steps', ids } });
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // A field being typed in keeps its own keys, and its own undo; the
      // other tabs have none of the canvas's.
      if (isTyping(e.target) || tab !== 'workflow') return;
      const key = e.key.toLowerCase();
      if (e.ctrlKey || e.metaKey) {
        if (key === 'z' || key === 'y') {
          e.preventDefault();
          if (key === 'y' || e.shiftKey) redo();
          else undo();
        }
        return;
      }
      if (!e.shiftKey || key !== 'a' || !pointer.current) return;
      e.preventDefault();
      setPalette({ at: pointer.current, place: pointer.current });
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [undo, redo, tab]);

  const dark = document.documentElement.classList.contains('dark');

  return (
    <>
      <HStack justify="space-between">
        <HStack gap="3" minW="0">
          <HStack gap="1" minW="0">
            <Stack gap="0" minW="0">
              <Heading size="lg" truncate>
                {name}
              </Heading>
              {description && (
                <Text color="fg.muted" textStyle="xs" truncate title={description}>
                  {description}
                </Text>
              )}
            </Stack>
            <IconButton
              aria-label="The workflow's settings"
              title="Settings: name, description, one run at a time"
              size="xs"
              variant="ghost"
              color="fg.muted"
              onClick={() => setRenaming(true)}
            >
              <PencilIcon />
            </IconButton>
          </HStack>
          <SaveStatus save={save} />
        </HStack>
        <HStack gap="2">
          <HStack gap="0" me="2" display={tab === 'workflow' ? undefined : 'none'}>
            <IconButton
              aria-label="Undo"
              title="Undo (Ctrl+Z)"
              size="sm"
              variant="ghost"
              disabled={!history.canUndo}
              onClick={undo}
            >
              <Undo2Icon />
            </IconButton>
            <IconButton
              aria-label="Redo"
              title="Redo (Ctrl+Shift+Z)"
              size="sm"
              variant="ghost"
              disabled={!history.canRedo}
              onClick={redo}
            >
              <Redo2Icon />
            </IconButton>
          </HStack>
          <Switch.Root
            size="sm"
            colorPalette="green"
            checked={active}
            onCheckedChange={(e) => void switchActive(e.checked)}
            me="2"
          >
            <Switch.HiddenInput />
            <Switch.Control>
              <Switch.Thumb />
            </Switch.Control>
            <Switch.Label>Active</Switch.Label>
          </Switch.Root>
          <Button
            size="sm"
            variant="outline"
            disabled={triggerSteps.length === 0}
            title={triggerSteps.length === 0 ? 'Add a trigger step first' : 'Run it once, with data you type'}
            onClick={() => setRunning(true)}
          >
            <PlayIcon />
            Run
          </Button>
          <Button
            size="sm"
            variant="outline"
            display={tab === 'workflow' ? undefined : 'none'}
            title="Add a step (Shift+A over the canvas adds it at the pointer)"
            onClick={(e) => {
              const r = e.currentTarget.getBoundingClientRect();
              setPalette({ at: { x: r.left, y: r.bottom + 4 }, place: null });
            }}
          >
            <PlusIcon />
            Add
          </Button>
          <Button
            size="sm"
            variant="ghost"
            title="Download it as a file this tower, or another, can import"
            onClick={() => void exportThis()}
          >
            <DownloadIcon />
            Export
          </Button>
          <Button size="sm" variant="ghost" colorPalette="red" onClick={() => setDeleting(true)}>
            Delete
          </Button>
        </HStack>
      </HStack>
      {active && <TriggerStatus triggers={triggers} kindOf={(id) => nodes.find((n) => n.id === id)?.data.kind} />}
      <Tabs.Root value={tab} onValueChange={(e) => setParams(e.value === 'workflow' ? {} : { tab: e.value })} size="sm">
        <Tabs.List>
          <Tabs.Trigger value="workflow">
            <WorkflowIcon size="16" />
            Workflow
          </Tabs.Trigger>
          <Tabs.Trigger value="runs">
            <HistoryIcon size="16" />
            Runs
          </Tabs.Trigger>
          <Tabs.Trigger value="logs">
            <ScrollTextIcon size="16" />
            Logs
          </Tabs.Trigger>
        </Tabs.List>
      </Tabs.Root>
      <Box
        ref={canvas}
        flex="1"
        minH="0"
        display={tab === 'workflow' ? undefined : 'none'}
        borderWidth="1px"
        rounded="md"
        overflow="hidden"
        onMouseMove={(e) => (pointer.current = { x: e.clientX, y: e.clientY })}
        onMouseLeave={() => (pointer.current = null)}
        css={{
          // Steps stand out from the canvas: lighter than it, as in Blender.
          '--xy-background-color': 'var(--chakra-colors-bg-muted)',
          _dark: { '--xy-background-color': 'var(--chakra-colors-bg)' },
          '& .react-flow__edge-path': { strokeWidth: 2 },
          '& .react-flow__edge:hover .react-flow__edge-path': { strokeWidth: 3.5 },
          // The links of the selected steps are bolder and glow a little; the
          // others fade.
          '& .react-flow__edge.linked .react-flow__edge-path': {
            strokeWidth: 3.5,
            filter: 'drop-shadow(0 0 1.5px currentColor)',
          },
          '& .react-flow__edge.faded:not(.selected)': { opacity: 0.3 },
          // A selected link is bolder than a hovered one and glows in its own colour.
          '& .react-flow__edge.selected .react-flow__edge-path': {
            strokeWidth: 6,
            filter: 'drop-shadow(0 0 2px currentColor)',
          },
        }}
      >
        <StepRunContext.Provider value={pickedSteps}>
          <ReactFlow
            nodes={nodes}
            edges={shownEdges}
            nodeTypes={nodeTypes}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            isValidConnection={canLink}
            onNodeContextMenu={onStepMenu}
            onSelectionContextMenu={(e, steps) => {
              e.preventDefault();
              setMenu({ at: { x: e.clientX, y: e.clientY }, target: { type: 'steps', ids: steps.map((s) => s.id) } });
            }}
            onEdgeContextMenu={(e, edge) => {
              e.preventDefault();
              setNodes((ns) => ns.map((n) => ({ ...n, selected: false })));
              setEdges((es) => es.map((x) => ({ ...x, selected: x.id === edge.id })));
              setMenu({ at: { x: e.clientX, y: e.clientY }, target: { type: 'link', id: edge.id } });
            }}
            colorMode={dark ? 'dark' : 'light'}
            deleteKeyCode={['Backspace', 'Delete']}
            // Dragging empty space pans, with any button; Shift+drag
            // (selectionKeyCode) box-selects.
            selectionMode={SelectionMode.Partial}
            panOnDrag={[0, 1, 2]}
            onPaneContextMenu={(e) => e.preventDefault()}
            minZoom={0.2}
            maxZoom={1.5}
            // Only a workflow with steps is fitted: on an empty one React Flow
            // would wait and fit the first step added, moving it from the pointer.
            fitView={start.nodes.length > 0}
            fitViewOptions={{ padding: 0.2, maxZoom: 1 }}
          >
            <Background variant={BackgroundVariant.Dots} gap={20} />
            <Controls />
            <MiniMap pannable zoomable style={{ width: 150, height: 100 }} />
            {picked && (
              <Panel position="top-left">
                <HStack bg="bg.panel" borderWidth="1px" rounded="md" shadow="sm" ps="2" pe="1" py="1" gap="2">
                  <Badge size="sm" colorPalette={runStatus[picked.status].palette}>
                    {runStatus[picked.status].label}
                  </Badge>
                  <Text textStyle="xs">Run {picked.id}: each step shows what it did</Text>
                  <CloseButton size="2xs" aria-label="Stop showing the run" onClick={() => setPicked(undefined)} />
                </HStack>
              </Panel>
            )}
            {nodes.length === 0 && (
              <Panel position="top-center">
                <Text color="fg.muted" textStyle="sm" mt="6">
                  Empty workflow: press Shift+A over the canvas, or Add, to add a step.
                </Text>
              </Panel>
            )}
          </ReactFlow>
        </StepRunContext.Provider>
      </Box>
      {tab === 'runs' && <RunsPanel workflowId={workflow.id} picked={picked} onPick={setPicked} reload={runsReload} />}
      {tab === 'logs' && (
        <Box flex="1" minH="0" overflowY="auto">
          <RunLog workflowId={workflow.id} />
        </Box>
      )}
      <Text fontSize="xs" color="fg.muted" display={tab === 'workflow' ? undefined : 'none'}>
        Shift+A add a step · drag from a socket to link · right-click a step or link for its menu · Delete removes ·
        Ctrl+Z undo · Ctrl+Shift+Z redo · drag on empty space to pan · Shift+drag to select · wheel to zoom
      </Text>
      {/* The pointer menu's trigger is an empty box at the pointer: a menu
          opened from code ignores its anchorPoint prop. A new key per opening
          places it afresh. */}
      {menu && (
        <Menu.Root
          key={`${menu.at.x},${menu.at.y}`}
          open
          positioning={{ placement: 'bottom-start', gutter: 2 }}
          onOpenChange={(e) => !e.open && setMenu(null)}
          onSelect={(e) => onMenuSelect(e.value)}
        >
          <Menu.Trigger asChild>
            <Box position="fixed" left={`${menu.at.x}px`} top={`${menu.at.y}px`} w="0" h="0" pointerEvents="none" />
          </Menu.Trigger>
          <MenuPanel>
            {menu.target.type === 'steps' && (
              <>
                <Menu.ItemGroup>
                  <Menu.ItemGroupLabel>
                    {menu.target.ids.length === 1 ? 'Step' : `${menu.target.ids.length} steps`}
                  </Menu.ItemGroupLabel>
                  <Menu.Item value="duplicate">Duplicate</Menu.Item>
                  <Menu.Item value="disconnect">Disconnect links</Menu.Item>
                </Menu.ItemGroup>
                <Menu.Separator />
                <Menu.Item value="delete" color="fg.error" _hover={{ bg: 'bg.error', color: 'fg.error' }}>
                  Delete
                  <Menu.ItemCommand>Del</Menu.ItemCommand>
                </Menu.Item>
              </>
            )}
            {menu.target.type === 'link' && (
              <Menu.Item value="delete" color="fg.error" _hover={{ bg: 'bg.error', color: 'fg.error' }}>
                Delete link
                <Menu.ItemCommand>Del</Menu.ItemCommand>
              </Menu.Item>
            )}
          </MenuPanel>
        </Menu.Root>
      )}
      {palette && (
        <AddPalette
          at={palette.at}
          onPick={(kind) => (palette.place ? add(kind, palette.place) : addAtCenter(kind))}
          onClose={() => setPalette(null)}
        />
      )}
      {running && <RunDialog triggers={triggerSteps} onRun={run} onClose={() => setRunning(false)} />}
      {renaming && (
        <DetailsDialog
          details={{ name, description, oneAtATime }}
          onClose={() => setRenaming(false)}
          onSave={async (next) => {
            await rename(next);
            toast(`Saved ${next.name}`);
          }}
        />
      )}
      {deleting && (
        <FormDialog
          title={`Delete workflow ${name}?`}
          submitText="Delete"
          danger
          onClose={() => setDeleting(false)}
          onSubmit={async () => {
            saved.current = latest.current; // nothing left to save
            await workflows.deleteWorkflow({ id: workflow.id });
            toast(`Deleted workflow ${name}`);
            onDeleted();
          }}
        >
          <Text>Its steps, links, runs and triggers go with it. This cannot be undone.</Text>
        </FormDialog>
      )}
    </>
  );
}

// TriggerStatus says what an active workflow's triggers are doing: a topic
// trigger whether it listens, a schedule when it runs next.
function TriggerStatus({
  triggers,
  kindOf,
}: {
  triggers: TriggerState[];
  kindOf: (stepId: string) => string | undefined;
}) {
  if (triggers.length === 0) {
    return (
      <Text textStyle="sm" color="fg.warning">
        Active, but it has no trigger step to start it.
      </Text>
    );
  }
  const text = (t: TriggerState) => {
    if (kindOf(t.stepId) !== 'schedule') {
      return t.listening ? `Listening on ${t.source}` : `Not listening (${t.source}): ${t.error}`;
    }
    if (!t.listening) return `Not scheduled: ${t.error}`;
    const next = t.next && timestampDate(t.next).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
    return `Scheduled ${t.source}${next ? `, next ${next}` : ''}`;
  };
  return (
    <HStack gap="4" wrap="wrap" textStyle="sm">
      {triggers.map((t) => (
        <HStack key={t.stepId} gap="1.5">
          <Box w="2" h="2" rounded="full" bg={t.listening ? 'green.solid' : 'red.solid'} />
          <Text color={t.listening ? 'fg.muted' : 'fg.error'}>{text(t)}</Text>
        </HStack>
      ))}
    </HStack>
  );
}

// RunDialog runs the workflow once from one of its trigger steps, with the
// data typed in, as if that trigger got it; a schedule takes no data, as if
// its time came now. A Manual trigger, there only for this, is the one
// picked first, with no data.
function RunDialog({
  triggers,
  onRun,
  onClose,
}: {
  triggers: Step[];
  onRun: (stepId: string, data: string) => Promise<void>;
  onClose: () => void;
}) {
  const manual = triggers.find((s) => s.data.kind === 'manual');
  const [stepId, setStepId] = useState((manual ?? triggers[0])?.id ?? '');
  const [data, setData] = useState(manual ? '' : '{\n  "id": "A1",\n  "total": 1500000\n}');
  const label = (s: Step) => {
    const { Topic: topic, Every: every } = s.data.values;
    const what = topic ? `topic ${topic}` : every;
    return `${stepKind(s.data.kind)?.title ?? s.data.kind}${what ? ` (${what})` : ''}`;
  };
  const scheduled = triggers.find((s) => s.id === stepId)?.data.kind === 'schedule';
  return (
    <FormDialog
      title="Run the workflow"
      submitText="Run"
      onClose={onClose}
      onSubmit={() => onRun(stepId, scheduled ? '' : data)}
    >
      {triggers.length > 1 && (
        <FormField label="Start from">
          <NativeSelect.Root>
            <NativeSelect.Field value={stepId} onChange={(e) => setStepId(e.currentTarget.value)}>
              {triggers.map((s) => (
                <option key={s.id} value={s.id}>
                  {label(s)}
                </option>
              ))}
            </NativeSelect.Field>
            <NativeSelect.Indicator />
          </NativeSelect.Root>
        </FormField>
      )}
      {scheduled ? (
        <Text textStyle="sm" color="fg.muted">
          It runs as if its time came now: its Payload's time is now.
        </Text>
      ) : (
        <FormField
          label={triggers.find((s) => s.id === stepId)?.data.kind === 'manual' ? 'Payload (optional)' : 'Event data'}
          help={
            triggers.find((s) => s.id === stepId)?.data.kind === 'manual'
              ? "The Manual step's Payload: JSON, any text, or nothing."
              : 'What the trigger gets, as if an event came on its topic: JSON, or any text.'
          }
        >
          <Textarea
            autoFocus
            rows={8}
            fontFamily="mono"
            textStyle="sm"
            value={data}
            onChange={(e) => setData(e.currentTarget.value)}
          />
        </FormField>
      )}
    </FormDialog>
  );
}

function SaveStatus({ save }: { save: SaveState }) {
  if (save.state === 'error') {
    return (
      <Text textStyle="sm" color="fg.error" truncate title={save.message}>
        Not saved: {save.message}
      </Text>
    );
  }
  const text = { saved: 'Saved', unsaved: 'Unsaved changes', saving: 'Saving...' }[save.state];
  return (
    <Text textStyle="sm" color="fg.muted" flexShrink={0}>
      {text}
    </Text>
  );
}

function MenuPanel({ children }: { children: ReactNode }) {
  return (
    <Portal>
      <Menu.Positioner>
        <Menu.Content minW="44">{children}</Menu.Content>
      </Menu.Positioner>
    </Portal>
  );
}

const isTyping = (t: EventTarget | null) =>
  t instanceof HTMLElement && (t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(t.tagName));
