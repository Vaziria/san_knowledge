import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';
import { settings } from './settings';

// Walking the camera round the scene (the Camera tab's View "walk"): what is
// held down now. The Camera tab's pad holds a move while its button is
// pressed, and the keys below hold theirs while they are down; the stage
// walks the camera by what is held (Stage.setMoves, wired in main.tsx), and
// so does the dev server's hidden browser while it draws the stream
// (stream.ts). A move held two ways, a key and a button, is let go when both
// are.
//
// The keys work while the camera walks and no text field, menu, slider, tab
// or tree has the focus: W and S (or ↑ ↓) forward and back, A and D
// sideways, Q and E (or ← →) turn, R and F look up and down, Shift runs.

export const MOVES = ['forward', 'back', 'left', 'right', 'turnLeft', 'turnRight', 'lookUp', 'lookDown', 'run'] as const;
export type Move = (typeof MOVES)[number];

interface Walking {
  held: Move[]; // in the order of MOVES
}

export const walking = createStore<Walking>()(() => ({ held: [] }));

export function useWalking<T>(select: (walking: Walking) => T): T {
  return useStore(walking, select);
}

const holders = new Map<Move, Set<string>>(); // who holds each move: a key, a pointer, the stream

export function press(move: Move, by: string): void {
  const hands = holders.get(move) ?? new Set<string>();
  hands.add(by);
  holders.set(move, hands);
  publish();
}

export function release(move: Move, by: string): void {
  holders.get(move)?.delete(by);
  publish();
}

export function releaseAll(): void {
  holders.clear();
  publish();
}

// Holds exactly these, as the panel holds them (the hidden browser, from
// stream.ts); names it doesn't know are left out.
export function holdOnly(moves: readonly string[]): void {
  holders.clear();
  for (const move of MOVES.filter((m) => moves.includes(m))) holders.set(move, new Set(['panel']));
  publish();
}

function publish(): void {
  const held = MOVES.filter((move) => (holders.get(move)?.size ?? 0) > 0);
  const before = walking.getState().held;
  if (held.length !== before.length || held.some((move, i) => move !== before[i])) walking.setState({ held });
}

const KEYS: Record<string, Move> = {
  KeyW: 'forward',
  ArrowUp: 'forward',
  KeyS: 'back',
  ArrowDown: 'back',
  KeyA: 'left',
  KeyD: 'right',
  KeyQ: 'turnLeft',
  ArrowLeft: 'turnLeft',
  KeyE: 'turnRight',
  ArrowRight: 'turnRight',
  KeyR: 'lookUp',
  KeyF: 'lookDown',
  ShiftLeft: 'run',
  ShiftRight: 'run',
};

// What takes keys itself: typing, menus, sliders, the tabs, the figure tree.
const TAKES_KEYS = [
  'input',
  'textarea',
  'select',
  '[contenteditable=""]',
  '[contenteditable="true"]',
  ...['slider', 'tab', 'option', 'listbox', 'combobox', 'radio', 'tree', 'treeitem', 'menuitem', 'spinbutton'].map(
    (role) => `[role=${role}]`,
  ),
].join(', ');

// The keys walk the camera while it walks (the panel's page; main.tsx).
export function listenForWalkKeys(): void {
  window.addEventListener('keydown', (event) => {
    const move = KEYS[event.code];
    if (!move || settings.getState().camera !== 'walk' || event.ctrlKey || event.altKey || event.metaKey) return;
    if (event.target instanceof Element && event.target.closest(TAKES_KEYS)) return;
    event.preventDefault();
    if (!event.repeat) press(move, event.code);
  });
  window.addEventListener('keyup', (event) => {
    const move = KEYS[event.code];
    if (move) release(move, event.code);
  });
  // A key let go while the page is away never comes back up here.
  window.addEventListener('blur', releaseAll);
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) releaseAll();
  });
  settings.subscribe((s, before) => {
    if (before.camera === 'walk' && s.camera !== 'walk') releaseAll();
  });
}
