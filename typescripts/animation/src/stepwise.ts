// Building a piece at a time: a generator that yields wherever it may stop
// for a frame to be drawn, and returns what it built. The forest lake
// meeting builds the next season this way while it plays (Stage.ts,
// Preview.changes), so the stream doesn't stop while it is built.
export type Stepwise<T> = Generator<void, T>;

// Runs one to its end at once.
export function finish<T>(steps: Stepwise<T>): T {
  for (;;) {
    const step = steps.next();
    if (step.done) return step.value;
  }
}
