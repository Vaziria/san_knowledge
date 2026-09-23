import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

// The behaviours of the figure being shown, for the panel's Behaviours
// section: one button for each behaviour its spec names (Flap, Speech,
// PlayChord…), with an input for each argument. Each preview lists its own
// (Preview.behaviours in previews.ts), with the arguments' defaults, and the
// stage hands them to this store when it shows the preview.
//
// Behaviours are actions, not settings, so they are not kept in the URL. They
// are the one place the panel reaches the scene: a click runs the behaviour
// on the figure directly. Running one stops the preview's demo, so the figure
// does only what it is told; `restart` builds the preview again with its demo.

export interface Param {
  name: string; // as in the spec: `Speech(text string)` has a param named text
  value: string; // the default
  options?: string[]; // a menu of these; without them, a text field
}

export interface Behaviour {
  name: string; // exactly as the spec writes it
  params?: Param[];
  run(...values: string[]): void; // may throw on a value the figure can't read
}

interface State {
  behaviours: Behaviour[];
  restart: () => void;
}

export const behaviours = createStore<State>()(() => ({ behaviours: [], restart: () => {} }));

export function useBehaviours<T>(select: (state: State) => T): T {
  return useStore(behaviours, select);
}

// Wraps a preview's behaviour runners so that running any of them first stops
// its demo: `const act = demoStopper(() => (demo = false))`, then
// `run: act(() => penguin.Flap())`.
export function demoStopper(stop: () => void) {
  return (run: (...values: string[]) => void) =>
    (...values: string[]) => {
      stop();
      run(...values);
    };
}
