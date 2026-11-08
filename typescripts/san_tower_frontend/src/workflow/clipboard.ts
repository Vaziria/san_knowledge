import type { XYPosition } from '@xyflow/react';
import { stepKind, toFlow, type Graph } from './catalog';

// Copy and paste of steps (the user, 2026-11-08: "can workflow have select
// area and copy paste ?"). Copied steps go to the system clipboard as JSON
// marked as the tower's, so they paste into this workflow or another one, in
// another tab too:
//
//   {"sanTower":"steps","version":1,"steps":[...],"links":[...]}
//
// steps and links are as WorkflowService takes them (Graph): each step with
// its id, kind, x, y and values, and the links between the copied steps, by
// socket name. Values (a Connection's or a Storage's id) are kept as they
// are: they mean the same on the same tower.
const marker = 'steps';
const version = 1;

export const clipText = (g: Graph) => JSON.stringify({ sanTower: marker, version, ...g });

// readClip reads copied steps from the clipboard's text. Text that is not
// ours is undefined; ours from a newer tower, whose format this one may not
// read, is 'newer'.
export function readClip(text: string): Graph | 'newer' | undefined {
  if (!text.trimStart().startsWith('{')) return undefined;
  let v: unknown;
  try {
    v = JSON.parse(text);
  } catch {
    return undefined;
  }
  if (!isObject(v) || v.sanTower !== marker) return undefined;
  if (v.version !== version) return typeof v.version === 'number' && v.version > version ? 'newer' : undefined;
  if (!Array.isArray(v.steps) || !Array.isArray(v.links)) return undefined;
  const steps: Graph['steps'] = [];
  const seen = new Set<string>();
  for (const s of v.steps) {
    if (!isObject(s) || typeof s.id !== 'string' || typeof s.kind !== 'string' || seen.has(s.id)) continue;
    seen.add(s.id);
    const values = isObject(s.values) ? s.values : {};
    steps.push({
      id: s.id,
      kind: s.kind,
      x: Number(s.x) || 0,
      y: Number(s.y) || 0,
      values: Object.fromEntries(Object.entries(values).filter((e): e is [string, string] => typeof e[1] === 'string')),
    });
  }
  const links: Graph['links'] = v.links.filter(
    (l): l is Graph['links'][number] =>
      isObject(l) && ['source', 'sourceSocket', 'target', 'targetSocket'].every((k) => typeof l[k] === 'string'),
  );
  return { steps, links };
}

const isObject = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);

// pasteSteps makes the copied steps anew, with new ids (as Duplicate gives
// them) and the links between them, moved so the middle of their places is
// at `at` and kept where they were to each other. A step of a kind this
// tower does not have is left out, with its links: skipped are their kinds,
// one per step.
export function pasteSteps(g: Graph, at: XYPosition) {
  const known = g.steps.filter((s) => stepKind(s.kind));
  const skipped = g.steps.filter((s) => !stepKind(s.kind)).map((s) => s.kind);
  if (known.length === 0) return { ...toFlow({ steps: [], links: [] }), skipped };
  const ids = new Map(known.map((s) => [s.id, crypto.randomUUID()]));
  const xs = known.map((s) => s.x);
  const ys = known.map((s) => s.y);
  const dx = at.x - (Math.min(...xs) + Math.max(...xs)) / 2;
  const dy = at.y - (Math.min(...ys) + Math.max(...ys)) / 2;
  const flow = toFlow({
    steps: known.map((s) => ({ ...s, id: ids.get(s.id)!, x: Math.round(s.x + dx), y: Math.round(s.y + dy) })),
    links: g.links
      .filter((l) => ids.has(l.source) && ids.has(l.target))
      .map((l) => ({ ...l, source: ids.get(l.source)!, target: ids.get(l.target)! })),
  });
  return { nodes: flow.nodes.map((n) => ({ ...n, selected: true })), edges: flow.edges, skipped };
}

// keptKey keeps, per browser, the steps copied last: the paste falls back
// on it when the browser gives no clipboard (a paste event without
// clipboardData).
const keptKey = 'san-tower-copied-steps';

export function keepClip(text: string) {
  try {
    localStorage.setItem(keptKey, text);
  } catch {
    // Storage blocked: the clipboard still has them.
  }
}

export function keptClip() {
  try {
    return localStorage.getItem(keptKey) ?? '';
  } catch {
    return '';
  }
}

// writeClip puts copied steps on the clipboard from a click (the step menu's
// Copy and Cut), and says whether it could. navigator.clipboard is missing
// outside a secure page (the tower opened by its LAN address over http), and
// the browser may refuse it: then a copy event made with execCommand carries
// them, as the Ctrl+C keys' does. The steps are kept in the browser too.
export async function writeClip(text: string) {
  keepClip(text);
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    return copyByEvent(text);
  }
}

function copyByEvent(text: string) {
  const put = (e: ClipboardEvent) => {
    e.clipboardData?.setData('text/plain', text);
    e.preventDefault();
    e.stopImmediatePropagation(); // not the editor's own copy of the selection
  };
  document.addEventListener('copy', put, { capture: true });
  try {
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    document.removeEventListener('copy', put, { capture: true });
  }
}
