import type { Fish } from '../../figures/Fish/Fish';

// The meeting's fish leaping out of the water now and then of its own accord
// (Meeting.ts), as the user asked ("sometimes fish jumpout from water"):
// every EVERY s, at random, a leap of HEIGHT, lower than any supporter's (a
// Super Chat's smallest is 0.3 m; meeting.ts), so a supporter's stays the big
// one. Like a bot's doings, it takes no turn and doesn't move the camera; it
// is seen when the fish is in view, its loop by the landing. A supporter's
// leap comes first: none of its own while one waits or is under way, and the
// next at least AFTER_SUPPORTER s after it.

const EVERY = [20, 40] as const; // s between its own leaps, and up to some 20 s more until it is where it can leap: 20–60 s in all
const HEIGHT = [0.15, 0.28] as const; // m its lowest point clears the water by at the top
const AFTER_SUPPORTER = 20; // s after a supporter's leap before one of its own

export class FishLeaps {
  private readonly fish: Fish;
  private readonly random: () => number;
  private wait: number; // s until its next leap

  constructor(fish: Fish, random: () => number = Math.random) {
    this.fish = fish;
    this.random = random;
    this.wait = between(this.random, EVERY);
  }

  // `supporter`: a supporter's leap is waiting, or its turn is under way;
  // `can`: the fish is where a leap keeps it on its loop (Meeting.ts). Once
  // its time has come it leaps at the next place it can.
  update(delta: number, supporter: boolean, can: boolean): void {
    if (supporter) {
      this.wait = Math.max(this.wait, AFTER_SUPPORTER);
      return;
    }
    if (this.fish.jumping) return;
    this.wait -= delta;
    if (this.wait > 0 || !can) return;
    // The fish ignores a leap until the water has slowed its last plunge:
    // then it tries again next frame.
    this.fish.JumpOutFromWater(between(this.random, HEIGHT));
    if (this.fish.jumping) this.wait = between(this.random, EVERY);
  }
}

function between(random: () => number, [least, most]: readonly [number, number]): number {
  return least + (most - least) * random();
}
