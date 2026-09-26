import * as THREE from 'three';
import { onDeck, type Deck } from '../../environtments/Lake/DockSite';

// The lake's land as the meeting's animals see it (Meeting.ts): where one may
// stand, spots picked at random, and the way from one place to another. It is
// a grid of CELL m squares over the lake, each with its clearance: how far it
// is to the nearest place no animal may be. Those are the water and the wet
// foot of the bank, ground too steep, the land's end and the border's tall
// grass and mist, and what the lake says stands on its land (Lake.obstacles():
// trunks, boulders, big stones). An animal of radius r stands where the
// clearance is at least r + GAP. A way is found over the grid (A*), so it goes
// round the lake and round what stands in it, never across the water, then
// straightened wherever a straight line keeps the clearance. Other animals are
// passed in as circles to keep off.
//
// A dock's deck (task 09) is ground too, and its clearance is measured
// exactly, from the point itself to the deck's sides and far end (its
// landward end joins the land): the grid's cells are half a meter, and taking
// the worst point of each, as elsewhere, a 3 m deck had room for nothing
// bigger than a cat.
//
// Coordinates are the meeting's (its origin at the landing); `middle` is the
// lake's middle in them.

export interface Circle {
  x: number;
  z: number;
  radius: number;
}

export interface LandOptions {
  ground(x: number, z: number): number; // m above the still water; below 0 under it
  middle: THREE.Vector2; // the lake's middle
  reach: number; // m from the middle to the land's end
  obstacles: Circle[];
  decks?: Deck[]; // docks' decks, in the same coordinates; `ground` gives their height
}

const OUT = 2; // m from a deck's landward end a spot on it is at least: out over the water

const CELL = 0.5; // m
const WET = 0.12; // m above the still water: the bank lower than this counts as water
const STEEP = 0.35; // m of rise a meter: steeper ground is kept off
const END = 7; // m in from the land's end: the border's tall grass and mist are there
export const GAP = 0.25; // m kept between an animal and anything

export class Land {
  private readonly size: number; // cells across
  private readonly corner: THREE.Vector2; // of the grid, at its lowest x and z
  private readonly clearance: Float32Array; // m from each cell's middle to the nearest place no animal may be
  private readonly middle: THREE.Vector2;
  private readonly reach: number;
  private readonly decks: Deck[];
  private readonly deckCells: Float32Array; // a cell whose middle is on a deck: its exact clearance there; NaN elsewhere

  constructor({ ground, middle, reach, obstacles, decks = [] }: LandOptions) {
    this.middle = middle.clone();
    this.reach = reach - END;
    this.decks = decks;
    const size = (this.size = Math.ceil((2 * reach) / CELL));
    this.corner = new THREE.Vector2(middle.x - (size * CELL) / 2, middle.y - (size * CELL) / 2);

    // The ground at each cell's middle, then what can't be stood on.
    const heights = new Float32Array(size * size);
    for (let j = 0; j < size; j++) for (let i = 0; i < size; i++) heights[j * size + i] = ground(this.x(i), this.z(j));
    const far = new Float32Array(size * size).fill(Infinity); // in cells, to the nearest cell that can't be stood on
    for (let j = 0; j < size; j++) {
      for (let i = 0; i < size; i++) {
        const k = j * size + i;
        const out = Math.hypot(this.x(i) - middle.x, this.z(j) - middle.y) > this.reach;
        const edge = i === 0 || j === 0 || i === size - 1 || j === size - 1;
        const steep = !edge && Math.hypot(heights[k + 1] - heights[k - 1], heights[k + size] - heights[k - size]) / (2 * CELL) > STEEP;
        if (out || edge || heights[k] < WET || steep) far[k] = 0;
      }
    }
    // How far each cell is from those (a two-pass chamfer distance: straight
    // steps 1, diagonal steps √2).
    const d = Math.SQRT2;
    for (let j = 1; j < size - 1; j++) {
      for (let i = 1; i < size - 1; i++) {
        const k = j * size + i;
        far[k] = Math.min(far[k], far[k - 1] + 1, far[k - size] + 1, far[k - size - 1] + d, far[k - size + 1] + d);
      }
    }
    for (let j = size - 2; j > 0; j--) {
      for (let i = size - 2; i > 0; i--) {
        const k = j * size + i;
        far[k] = Math.min(far[k], far[k + 1] + 1, far[k + size] + 1, far[k + size + 1] + d, far[k + size - 1] + d);
      }
    }
    // In meters, to the edge of the nearest such cell, and no further than
    // the nearest trunk, boulder or big stone; then less half the cell's
    // diagonal, so that it holds for every point of the cell, not only its
    // middle.
    const clearance = (this.clearance = new Float32Array(size * size));
    for (let j = 0; j < size; j++) {
      for (let i = 0; i < size; i++) {
        const k = j * size + i;
        let most = Math.max(0, (far[k] - 0.5) * CELL);
        for (const o of obstacles) most = Math.min(most, Math.hypot(this.x(i) - o.x, this.z(j) - o.z) - o.radius);
        clearance[k] = Math.max(0, most - (CELL * Math.SQRT2) / 2);
      }
    }
    this.deckCells = new Float32Array(size * size).fill(NaN);
    if (decks.length) {
      for (let j = 0; j < size; j++) {
        for (let i = 0; i < size; i++) {
          const room = this.deckRoom(this.x(i), this.z(j));
          if (room >= 0) this.deckCells[j * size + i] = room;
        }
      }
    }
  }

  // How far a point on a deck is from its sides and far end, or -1 off every
  // deck.
  private deckRoom(x: number, z: number): number {
    for (const deck of this.decks) {
      const { across, along } = onDeck(deck, x, z);
      if (Math.abs(across) <= deck.width / 2 && along >= 0 && along <= deck.length) return Math.min(deck.width / 2 - Math.abs(across), deck.length - along);
    }
    return -1;
  }

  // A spot out on a dock's deck (OUT m or more from its landward end) where
  // an animal `radius` m round can stand, clear of `others`, or null: for a
  // walk out on the dock now and then (Roam.ts).
  deckSpot(random: () => number, radius: number, others: Circle[]): THREE.Vector2 | null {
    if (!this.decks.length) return null;
    const deck = this.decks[Math.floor(random() * this.decks.length)];
    const across = deck.width / 2 - radius - GAP;
    const last = deck.length - radius - GAP;
    if (across < 0 || last < OUT) return null;
    for (let attempt = 0; attempt < 60; attempt++) {
      const a = (2 * random() - 1) * across;
      const along = OUT + (last - OUT) * random();
      const c = Math.cos(deck.heading);
      const s = Math.sin(deck.heading);
      const x = deck.x + a * c + along * s;
      const z = deck.z - a * s + along * c;
      if (this.free(x, z, radius, others)) return new THREE.Vector2(x, z);
    }
    return null;
  }

  // How far a point is from the nearest place no animal may be (the nearest
  // cell's; on a deck, its own).
  clearanceAt(x: number, z: number): number {
    const room = this.decks.length ? this.deckRoom(x, z) : -1;
    if (room >= 0) return room;
    const i = Math.floor((x - this.corner.x) / CELL);
    const j = Math.floor((z - this.corner.y) / CELL);
    if (i < 0 || j < 0 || i >= this.size || j >= this.size) return 0;
    return this.clearance[j * this.size + i];
  }

  // Whether an animal `radius` m round can stand at a point, clear of the
  // land's own obstacles and of `others`.
  free(x: number, z: number, radius: number, others: Circle[] = []): boolean {
    if (this.clearanceAt(x, z) < radius + GAP) return false;
    return others.every((o) => Math.hypot(x - o.x, z - o.z) >= radius + o.radius + GAP);
  }

  // A spot where an animal `radius` m round can stand, clear of `others`:
  // anywhere on the land, evenly by area, or within `within` m of `near`.
  // Null when none is found.
  spot(random: () => number, radius: number, others: Circle[], near?: THREE.Vector2, within = 0): THREE.Vector2 | null {
    for (let attempt = 0; attempt < 400; attempt++) {
      let x: number;
      let z: number;
      if (near) {
        const r = within * Math.sqrt(random());
        const a = 2 * Math.PI * random();
        x = near.x + r * Math.cos(a);
        z = near.y + r * Math.sin(a);
      } else {
        x = this.middle.x + this.reach * (2 * random() - 1);
        z = this.middle.y + this.reach * (2 * random() - 1);
      }
      if (this.free(x, z, radius, others)) return new THREE.Vector2(x, z);
    }
    return null;
  }

  // The nearest spot to a point, within `within` m, where an animal `radius`
  // m round can stand clear of `others`, or null.
  nearestFree(point: THREE.Vector2, radius: number, others: Circle[], within: number): THREE.Vector2 | null {
    for (let d = 0; d <= within; d += 0.1) {
      const around = d === 0 ? 1 : Math.ceil((2 * Math.PI * d) / 0.1);
      for (let k = 0; k < around; k++) {
        const a = (2 * Math.PI * k) / around;
        const x = point.x + d * Math.cos(a);
        const z = point.y + d * Math.sin(a);
        if (this.free(x, z, radius, others)) return new THREE.Vector2(x, z);
      }
    }
    return null;
  }

  // Where the land ends in the direction of a point, seen from the lake's
  // middle: a spot there for an animal to walk off to.
  edgeToward(point: THREE.Vector2, radius: number, random: () => number): THREE.Vector2 | null {
    const heading = Math.atan2(point.y - this.middle.y, point.x - this.middle.x);
    for (let attempt = 0; attempt < 60; attempt++) {
      const a = heading + (random() - 0.5) * (0.3 + attempt * 0.05);
      const r = this.reach - radius - GAP - 2 * random();
      const x = this.middle.x + r * Math.cos(a);
      const z = this.middle.y + r * Math.sin(a);
      if (this.free(x, z, radius)) return new THREE.Vector2(x, z);
    }
    return null;
  }

  // The way for an animal `radius` m round from one point to another, keeping
  // `margin` m more than it needs from everything, and off `others`: the
  // points to walk through, the last one `to`. Null when there is none.
  path(from: THREE.Vector2, to: THREE.Vector2, radius: number, margin: number, others: Circle[] = []): THREE.Vector2[] | null {
    const need = radius + GAP + margin;
    const size = this.size;
    const passable = (k: number) => {
      const deck = this.deckCells[k];
      if ((Number.isNaN(deck) ? this.clearance[k] : deck) < need) return false;
      const x = this.x(k % size);
      const z = this.z(Math.floor(k / size));
      return others.every((o) => Math.hypot(x - o.x, z - o.z) >= radius + o.radius + GAP);
    };
    const start = this.nearestPassable(this.cell(from), passable);
    const goal = this.nearestPassable(this.cell(to), passable);
    if (start < 0 || goal < 0) return null;

    // A* over the cells, eight ways round.
    const gx = goal % size;
    const gz = Math.floor(goal / size);
    const cost = new Float32Array(size * size).fill(Infinity);
    const came = new Int32Array(size * size).fill(-1);
    const done = new Uint8Array(size * size);
    const open = new Heap();
    const guess = (k: number) => {
      const dx = Math.abs((k % size) - gx);
      const dz = Math.abs(Math.floor(k / size) - gz);
      return Math.max(dx, dz) + (Math.SQRT2 - 1) * Math.min(dx, dz);
    };
    cost[start] = 0;
    open.push(start, guess(start));
    let found = false;
    while (open.size > 0) {
      const k = open.pop();
      if (done[k]) continue;
      if (k === goal) {
        found = true;
        break;
      }
      done[k] = 1;
      const i = k % size;
      const j = Math.floor(k / size);
      for (let dj = -1; dj <= 1; dj++) {
        for (let di = -1; di <= 1; di++) {
          if (di === 0 && dj === 0) continue;
          const ni = i + di;
          const nj = j + dj;
          if (ni < 0 || nj < 0 || ni >= size || nj >= size) continue;
          const n = nj * size + ni;
          if (done[n] || !passable(n)) continue;
          // No cutting a corner past a cell that can't be passed; but from
          // one cell on a deck to another, the step stays on the deck's room,
          // which is a rectangle: along a deck at a slant to the grid, the
          // room a big animal needs is a band too narrow for the corner rule.
          const onDecks = !Number.isNaN(this.deckCells[k]) && !Number.isNaN(this.deckCells[n]);
          if (di !== 0 && dj !== 0 && !onDecks && (!passable(j * size + ni) || !passable(nj * size + i))) continue;
          const step = cost[k] + (di !== 0 && dj !== 0 ? Math.SQRT2 : 1);
          if (step < cost[n]) {
            cost[n] = step;
            came[n] = k;
            open.push(n, step + guess(n));
          }
        }
      }
    }
    if (!found) return null;

    const cells: THREE.Vector2[] = [];
    for (let k = goal; k >= 0; k = came[k]) cells.push(new THREE.Vector2(this.x(k % size), this.z(Math.floor(k / size))));
    cells.reverse();
    cells[0] = from.clone();
    cells.push(to.clone());
    // Straightened: from each point, on to the furthest one in a clear line.
    const way: THREE.Vector2[] = [];
    let at = 0;
    while (at < cells.length - 1) {
      let next = cells.length - 1;
      while (next > at + 1 && !this.clearLine(cells[at], cells[next], need, radius, others)) next--;
      way.push(cells[next]);
      at = next;
    }
    return way;
  }

  // Whether a straight line keeps `need` m from the land's obstacles and off
  // `others` all along.
  clearLine(a: THREE.Vector2, b: THREE.Vector2, need: number, radius: number, others: Circle[] = []): boolean {
    const steps = Math.max(1, Math.ceil(a.distanceTo(b) / (CELL / 2)));
    for (let s = 1; s < steps; s++) {
      const x = a.x + ((b.x - a.x) * s) / steps;
      const z = a.y + ((b.y - a.y) * s) / steps;
      if (this.clearanceAt(x, z) < need) return false;
      if (others.some((o) => Math.hypot(x - o.x, z - o.z) < radius + o.radius + GAP)) return false;
    }
    return true;
  }

  private x(i: number): number {
    return this.corner.x + (i + 0.5) * CELL;
  }

  private z(j: number): number {
    return this.corner.y + (j + 0.5) * CELL;
  }

  private cell(point: THREE.Vector2): number {
    const i = THREE.MathUtils.clamp(Math.floor((point.x - this.corner.x) / CELL), 0, this.size - 1);
    const j = THREE.MathUtils.clamp(Math.floor((point.y - this.corner.y) / CELL), 0, this.size - 1);
    return j * this.size + i;
  }

  // The cell itself if it can be passed, or the nearest one that can within
  // two meters (an animal standing a little closer to something than a way
  // may pass), or -1.
  private nearestPassable(k: number, passable: (k: number) => boolean): number {
    if (passable(k)) return k;
    const size = this.size;
    const i = k % size;
    const j = Math.floor(k / size);
    const most = Math.ceil(2 / CELL);
    let best = -1;
    let bestDistance = Infinity;
    for (let dj = -most; dj <= most; dj++) {
      for (let di = -most; di <= most; di++) {
        const ni = i + di;
        const nj = j + dj;
        if (ni < 0 || nj < 0 || ni >= size || nj >= size) continue;
        const distance = Math.hypot(di, dj);
        if (distance < bestDistance && passable(nj * size + ni)) {
          best = nj * size + ni;
          bestDistance = distance;
        }
      }
    }
    return best;
  }
}

// A binary min-heap of cells by their estimated cost.
class Heap {
  private readonly keys: number[] = [];
  private readonly costs: number[] = [];

  get size(): number {
    return this.keys.length;
  }

  push(key: number, cost: number): void {
    const { keys, costs } = this;
    let at = keys.length;
    keys.push(key);
    costs.push(cost);
    while (at > 0) {
      const up = (at - 1) >> 1;
      if (costs[up] <= cost) break;
      keys[at] = keys[up];
      costs[at] = costs[up];
      at = up;
    }
    keys[at] = key;
    costs[at] = cost;
  }

  pop(): number {
    const { keys, costs } = this;
    const top = keys[0];
    const key = keys.pop()!;
    const cost = costs.pop()!;
    const n = keys.length;
    if (n === 0) return top;
    let at = 0;
    for (;;) {
      const left = 2 * at + 1;
      if (left >= n) break;
      const right = left + 1;
      const child = right < n && costs[right] < costs[left] ? right : left;
      if (costs[child] >= cost) break;
      keys[at] = keys[child];
      costs[at] = costs[child];
      at = child;
    }
    keys[at] = key;
    costs[at] = cost;
    return top;
  }
}
