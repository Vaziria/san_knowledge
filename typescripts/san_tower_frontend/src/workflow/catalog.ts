import type { Connection, Edge, Node, XYPosition } from '@xyflow/react';
import { connections, knowledge, storages, workflows } from '../api';
import type { Connection as TowerConnection } from '../gen/san/tower/v1/connection_pb';
import type { Knowledge as TowerKnowledge } from '../gen/san/tower/v1/knowledge_pb';
import type { Storage as TowerStorage } from '../gen/san/tower/v1/storage_pb';
import type { Link, Socket, StepKind, Step as StoredStep } from '../gen/san/tower/v1/workflow_pb';

// A workflow is a graph of steps, drawn the way Blender's node editors draw
// one: each step has typed sockets, outputs at the top and inputs at the
// bottom, and a link goes from an output to an input of the same type. The
// tower defines the kinds of step (ListStepKinds), stores the workflows, and
// runs them.

export type { Setting, Socket, StepKind } from '../gen/san/tower/v1/workflow_pb';

// socketColor follows Blender's socket colours: the flow (event) is green
// like geometry, structured data (object) purple like a vector, text blue
// like a string, a knowledge (Knowledge Base's) orange like an object, a
// tool (a tool step's, for an AI step's Tools) pink like a texture. A type
// the UI does not know yet is grey.
const socketColors: Record<string, string> = {
  event: '#00d6a3',
  object: '#6363c7',
  text: '#70b2ff',
  knowledge: '#ed9e5c',
  tool: '#e66ae6',
};
export const socketColor = (type: string) => socketColors[type] ?? '#a1a1a1';

// categories are the Add menu's groups, in order; a kind in another category
// goes last.
export const categories = [
  'Trigger',
  'Logic',
  'Data',
  'Storage',
  'Datetime',
  'Action',
  'Messengers',
  'Social',
  'Finance',
  'Knowledge',
  'AI',
];
const palettes: Record<string, string> = {
  Trigger: 'red',
  Logic: 'blue',
  Data: 'purple',
  Storage: 'orange',
  Datetime: 'teal',
  Action: 'green',
  Messengers: 'cyan',
  Social: 'pink',
  Finance: 'gray',
  Knowledge: 'purple',
  AI: 'yellow',
};
export const categoryPalette = (category: string) => palettes[category] ?? 'gray';

// The kinds of step, from the tower. loadStepKinds asks once; the editor
// waits for it before it draws a workflow.
let kinds: StepKind[] = [];
let byKind = new Map<string, StepKind>();
let loading: Promise<void> | undefined;

export function loadStepKinds(): Promise<void> {
  loading ??= workflows.listStepKinds({}).then(
    (r) => {
      kinds = r.kinds;
      byKind = new Map(kinds.map((k) => [k.kind, k]));
    },
    (err: unknown) => {
      loading = undefined; // try again next time
      throw err;
    },
  );
  return loading;
}

export const stepKinds = () => kinds;

// stepKind is undefined for a kind the tower does not have (any more).
export const stepKind = (kind: string) => byKind.get(kind);

// A trigger starts runs: the tower says so (StepKind.trigger), since a
// messenger's trigger is in Messengers, not Trigger (2026-10-27).
export const isTrigger = (kind: string) => !!stepKind(kind)?.trigger;

// The connections a step can pick (a setting with a connection type), from
// the tower: asked once per type, and again after the Connections page
// changes one (forgetConnections).
const connectionLists = new Map<string, Promise<TowerConnection[]>>();

export function loadConnections(type: string): Promise<TowerConnection[]> {
  let p = connectionLists.get(type);
  if (!p) {
    p = connections.listConnections({ type }).then((r) => r.connections);
    p.catch(() => connectionLists.delete(type)); // try again next time
    connectionLists.set(type, p);
  }
  return p;
}

export const forgetConnections = () => connectionLists.clear();

// The storages a step can pick (a setting with storage), from the tower:
// asked once, and again after the Storage page changes one
// (forgetStorages).
let storageList: Promise<TowerStorage[]> | undefined;

export function loadStorages(): Promise<TowerStorage[]> {
  if (!storageList) {
    const p = storages.listStorages({}).then((r) => r.storages);
    p.catch(() => {
      if (storageList === p) storageList = undefined; // try again next time
    });
    storageList = p;
  }
  return storageList;
}

export const forgetStorages = () => (storageList = undefined);

// The knowledge a step can pick (a setting with knowledge), of every folder,
// from the tower: asked once, and again after the Knowledge page changes one
// (forgetKnowledge).
let knowledgeList: Promise<TowerKnowledge[]> | undefined;

export function loadKnowledge(): Promise<TowerKnowledge[]> {
  if (!knowledgeList) {
    const p = knowledge.listKnowledge({ allFolders: true }).then((r) => r.knowledge);
    p.catch(() => {
      if (knowledgeList === p) knowledgeList = undefined; // try again next time
    });
    knowledgeList = p;
  }
  return knowledgeList;
}

export const forgetKnowledge = () => (knowledgeList = undefined);

// StepData is what a step holds: its kind, and the values of its settings
// and of its unlinked fields, by name.
export type StepData = { kind: string; values: Record<string, string> };
export type Step = Node<StepData, 'step'>;

// A handle's id is "<socket type>:<socket name>", so a link can be checked
// from the ids alone.
export const handleId = (s: Socket) => `${s.type}:${s.name}`;
const handleType = (id?: string | null) => id?.split(':')[0];

// manyLinks says whether an input keeps every link drawn into it: a tool
// input takes any number of tools (the user, 2026-10-29), any other input
// one link, which a new link replaces.
export const manyLinks = (handle?: string | null) => handleType(handle) === 'tool';
const socketName = (id?: string | null) => id?.slice(id.indexOf(':') + 1) ?? '';

export const canLink = (c: Connection | Edge) =>
  c.source !== c.target && !!handleType(c.sourceHandle) && handleType(c.sourceHandle) === handleType(c.targetHandle);

// link is an edge for a connection, coloured by its socket type. color is set
// too, so the selected link's glow can use it as currentColor.
export function link(c: Connection): Edge {
  const color = socketColor(handleType(c.sourceHandle) ?? '');
  return {
    ...c,
    id: `${c.source}.${c.sourceHandle}>${c.target}.${c.targetHandle}`,
    style: { stroke: color, color },
  };
}

export function newStep(kind: string, position: XYPosition): Step {
  const defaults = Object.fromEntries(
    (stepKind(kind)?.settings ?? []).filter((s) => s.options.length > 0).map((s) => [s.name, s.options[0]]),
  );
  return { id: crypto.randomUUID(), type: 'step', position, data: { kind, values: defaults } };
}

// toFlow turns a stored workflow (or a Graph, undo's) into the editor's nodes
// and edges. A link whose socket is no longer in the catalogue is left out.
export function toFlow(w: Graph): { nodes: Step[]; edges: Edge[] } {
  const kinds = new Map(w.steps.map((s) => [s.id, stepKind(s.kind)]));
  const nodes = w.steps.map((s): Step => ({
    id: s.id,
    type: 'step',
    position: { x: s.x, y: s.y },
    data: { kind: s.kind, values: { ...s.values } },
  }));
  const edges = w.links.flatMap((l) => {
    const out = kinds.get(l.source)?.outputs.find((o) => o.name === l.sourceSocket);
    const input = kinds.get(l.target)?.inputs.find((i) => i.name === l.targetSocket);
    if (!out || !input) return [];
    return [link({ source: l.source, sourceHandle: handleId(out), target: l.target, targetHandle: handleId(input) })];
  });
  return { nodes, edges };
}

// Graph is a workflow's steps and links, as WorkflowService takes them.
export type Graph = {
  steps: Pick<StoredStep, 'id' | 'kind' | 'x' | 'y' | 'values'>[];
  links: Pick<Link, 'source' | 'sourceSocket' | 'target' | 'targetSocket'>[];
};

export function fromFlow(nodes: Step[], edges: Edge[]): Graph {
  return {
    steps: nodes.map((n) => ({
      id: n.id,
      kind: n.data.kind,
      x: Math.round(n.position.x),
      y: Math.round(n.position.y),
      values: n.data.values,
    })),
    links: edges.map((e) => ({
      source: e.source,
      sourceSocket: socketName(e.sourceHandle),
      target: e.target,
      targetSocket: socketName(e.targetHandle),
    })),
  };
}
