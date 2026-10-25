import { useCallback, useEffect, useRef, useState } from 'react';

// pause is how long the editing stops before a change is recorded, so a burst
// of typing in a step's field is one step back.
const pause = 500;
const limit = 200;

// useHistory is the editor's undo and redo. It keeps the graph (JSON, as
// saved) as it was after each change; undo and redo hand one back to apply.
// A change is recorded once the editing pauses, and not while busy (a step
// is being dragged), so a whole drag is one step back. Undo and redo first
// record a change not yet recorded.
export function useHistory(graph: string, busy: boolean, apply: (graph: string) => void) {
  const past = useRef<string[]>([]);
  const future = useRef<string[]>([]);
  // base is the graph as last recorded or applied.
  const base = useRef(graph);
  const current = useRef(graph);
  current.current = graph;
  const applyRef = useRef(apply);
  applyRef.current = apply;
  const [sizes, setSizes] = useState({ past: 0, future: 0 });
  const changed = () => setSizes({ past: past.current.length, future: future.current.length });

  const record = useCallback(() => {
    if (current.current === base.current) return;
    past.current.push(base.current);
    if (past.current.length > limit) past.current.shift();
    future.current = [];
    base.current = current.current;
    changed();
  }, []);

  useEffect(() => {
    if (busy || graph === base.current) return;
    const t = setTimeout(record, pause);
    return () => clearTimeout(t);
  }, [graph, busy, record]);

  const step = useCallback(
    (from: { current: string[] }, to: { current: string[] }) => {
      record();
      const next = from.current.pop();
      if (next === undefined) return;
      to.current.push(base.current);
      base.current = next;
      applyRef.current(next);
      changed();
    },
    [record],
  );
  const undo = useCallback(() => step(past, future), [step]);
  const redo = useCallback(() => step(future, past), [step]);

  const pending = graph !== base.current;
  return { undo, redo, canUndo: sizes.past > 0 || pending, canRedo: sizes.future > 0 && !pending };
}
