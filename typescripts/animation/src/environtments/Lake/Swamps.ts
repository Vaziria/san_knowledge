import * as THREE from 'three';
import { between, seededRandom } from '../../figures/Tree/parts';
import type { Theme } from '../../theme';
import { ovalInside, ovalMask, type Footprint } from '../terrains/parts';
import { SwampTerrain } from '../terrains/SwampTerrain';
import type { Wind } from '../wind';
import { BANK_WIDTH, groundHeight, LANDING, lawnColor, shoreRadius, type Cover } from './Ground';

// Swamps at the lakeside, as the user asked ("add swamps in lakeside"): up to
// three of the swamp terrain (../terrains/SwampTerrain.ts) on the land just
// past the top of the bank, each lying along the shore, 5-8 m long and
// 4.4-5.5 m across. Still, murky water with duckweed on it, hummocks topped
// with sedge, cattails in the shallows, and wet mud, then mossy peat, round
// the water, fading into the lawn at the edge.
//
// Where: one on the far shore to the southwest, where the preview cameras
// look across the water, and one either side of the landing on the near
// shore (PLACES). For each, seeded candidates are tried along its stretch of
// shore. Of those that fit, the one needing the least digging is kept, so
// the swamps settle into the land's own hollows. A stretch with no room for
// one goes without.
//
// - None in the landing's clearing (the lake meeting's is wider), on the
//   bank, on or against a mud pit, a dock or the way onto it, or against
//   another swamp. A tree or a boulder may stand at a swamp's undug rim, its
//   plants keeping off them, but its dug middle keeps a meter off a tree's
//   foot and half a meter off a boulder: a swamp nestles against them. They
//   are sited after all of those, which keep their places; the lake takes
//   away the stones and the grass inside them (Lake.ts).
// - The land is dug for the water (dig()), since a terrain's water needs a
//   hollow (rules.md, Terrains rule 1). The dig is a smooth hollow from
//   DIG.rim m in from the outline, full depth DIG.slope m further in. The
//   land's own humps still show through as shallows and islands.
// - The water stands DIG.lip below the lowest land round the undug rim, so
//   the rim holds it in all round. It is never higher than DIG.level over
//   the lake, so the lake meeting's animals, which take ground under 12 cm
//   for water (story/lake_meeting/Land.ts), keep out of the pools. The middle
//   is dug about DIG.depth below the water, and a site needing more than
//   DIG.most of digging is not taken.
// - The pools lie below the lake's level: the lake leaves its own water out
//   over the land (Water.ts), so it doesn't show in them, and its ground out
//   under them (Ground.ts).
//
// Units are meters, in the lake's coordinates.

const PLACES = [
  { around: [-2.75, -1.95] as const }, // the far shore, southwest: radians round the lake from +x toward +z
  { around: [2.15, 2.6] as const }, // the near shore, left of the landing
  { around: [0.4, 0.9] as const }, // and right of it
];
const SITE = {
  length: [5, 8] as const, // m along the shore
  width: [4.4, 5.5] as const, // m across: wide enough to be dug fully round the middle (fit())
  edge: [0.3, 2] as const, // m past the top of the bank to its edge nearest the lake
  tries: 150, // candidates for each place: at 80 the far shore, hemmed in by a boulder, found none
  grid: 0.3, // m between the points checked
};
// m kept: from the top of the bank, the landing's clearing, a mud pit or a
// dock (`blocked`) and another swamp, from anywhere in a swamp; from a
// tree's foot, at its rim and where it is dug; and from a boulder, where it
// is dug.
const ROOM = { bank: 0.2, clearing: 0.8, blocked: 0.5, apart: 1.5, trunk: { rim: 0.3, dug: 1 }, boulder: 0.5 };
const PLANT_GAP = 0.1; // m its plants keep from a tree's foot or a boulder
const DIG = { rim: 0.7, slope: 0.9, depth: 0.18, lip: 0.04, level: 0.1, most: 0.3 };
const SEED = 37;

interface Site {
  x: number; // its middle
  z: number;
  turn: number; // radians about y: its own x runs along the shore, and its z toward the lake
  size: Footprint;
  seed: number;
  level: number; // m its still water stands over the lake's
  depth: number; // m the land is dug where it is dug fully
  inside: (u: number, v: number) => number; // m in from its outline, in its own coordinates
  mask: (u: number, v: number) => number; // how much of the ground there it covers
  reach: number; // m from its middle to the corners of its footprint
}

type Circle = { x: number; z: number; radius: number };

export interface SwampsOptions {
  clear: number; // m round the landing kept clear (LakeOptions.clear)
  trunks: Circle[]; // the trees' feet (Lake.ts footRadius())
  boulders: Circle[]; // the boulders' footprints
  blocked: (x: number, z: number, gap: number) => boolean; // whether a mud pit, a dock or the way onto one is within `gap` m
}

export class Swamps extends THREE.Group implements Cover {
  readonly terrains: SwampTerrain[] = [];
  private readonly sites: Site[] = [];
  private readonly clearing: number; // m round the landing kept clear
  private readonly trunks: Circle[];
  private readonly boulders: Circle[];
  private readonly blocked: SwampsOptions['blocked'];

  // Finds the swamps' sites and digs the land for them; build() puts the
  // terrains there.
  constructor({ clear, trunks, boulders, blocked }: SwampsOptions) {
    super();
    this.name = 'swamps';
    this.clearing = clear;
    this.trunks = trunks;
    this.boulders = boulders;
    this.blocked = blocked;
    const random = seededRandom(SEED);
    for (const place of PLACES) {
      let best: Site | null = null;
      for (let t = 0; t < SITE.tries; t++) {
        const angle = between(random, place.around[0], place.around[1]);
        const size = { width: between(random, ...SITE.length), depth: between(random, ...SITE.width) };
        const edge = between(random, ...SITE.edge);
        const seed = Math.floor(random() * 1e6);
        const out = shoreRadius(angle) + BANK_WIDTH + edge + size.depth / 2;
        const site = this.fit(out * Math.cos(angle), out * Math.sin(angle), -angle - Math.PI / 2, size, seed);
        if (site && (!best || site.depth < best.depth)) best = site;
      }
      if (best) this.sites.push(best);
    }
  }

  // The swamps themselves, on their sites: their plants keep off the trees'
  // feet, the boulders, and wherever `keepOff` says (the stones left at their
  // edges).
  build(theme: Theme, wind: Wind, keepOff: (x: number, z: number) => boolean): void {
    const off = (x: number, z: number) => keepOff(x, z) || [...this.trunks, ...this.boulders].some((c) => Math.hypot(x - c.x, z - c.z) < c.radius + PLANT_GAP);
    for (const site of this.sites) {
      const lake = (u: number, v: number) => toLake(site, u, v);
      const terrain = new SwampTerrain({
        theme,
        wind,
        seed: site.seed,
        width: site.size.width,
        depth: site.size.depth,
        heightAt: (u, v) => {
          const p = lake(u, v);
          return groundHeight(p.x, p.z, this.clearing) - this.dig(p.x, p.z) - site.level;
        },
        mask: site.mask,
        keepOff: (u, v) => {
          const p = lake(u, v);
          return off(p.x, p.z);
        },
        rim: lawnColor(theme),
      });
      terrain.position.set(site.x, site.level, site.z);
      terrain.rotation.y = site.turn;
      this.terrains.push(terrain);
      this.add(terrain);
    }
  }

  // How many meters the land is dug at a point of the lake.
  dig(x: number, z: number): number {
    const site = this.siteAt(x, z);
    if (!site) return 0;
    const { u, v } = toSite(site, x, z);
    const t = THREE.MathUtils.clamp((site.inside(u, v) - DIG.rim) / DIG.slope, 0, 1);
    return (site.depth * (1 - Math.cos(Math.PI * t))) / 2;
  }

  // How much of the ground at a point of the lake a swamp covers, 0 to 1.
  covers(x: number, z: number): number {
    const site = this.siteAt(x, z);
    if (!site) return 0;
    const { u, v } = toSite(site, x, z);
    return site.mask(u, v);
  }

  // Whether a point of the lake is within `gap` m of a swamp's outline, or
  // inside it.
  near(x: number, z: number, gap: number): boolean {
    return this.sites.some((site) => {
      if (Math.hypot(x - site.x, z - site.z) > site.reach + gap) return false;
      const { u, v } = toSite(site, x, z);
      return site.inside(u, v) > -gap;
    });
  }

  // The swamps' places, sizes and water levels, for checking them.
  get spots(): { x: number; z: number; turn: number; width: number; depth: number; level: number; dug: number }[] {
    return this.sites.map((s) => ({ x: s.x, z: s.z, turn: s.turn, width: s.size.width, depth: s.size.depth, level: s.level, dug: s.depth }));
  }

  // How far into the night it is and how hard it rains, which dull the
  // water's glint; call before update() each frame.
  setNight(night: number, rain: number): void {
    for (const terrain of this.terrains) terrain.setNight(night, rain);
  }

  // Moves the swamps' waves on; call once per frame.
  update(delta: number): void {
    for (const terrain of this.terrains) terrain.update(delta);
  }

  // The swamp whose footprint a point of the lake is in, if any: they never
  // overlap.
  private siteAt(x: number, z: number): Site | null {
    for (const site of this.sites) {
      if (Math.hypot(x - site.x, z - site.z) > site.reach) continue;
      const { u, v } = toSite(site, x, z);
      if (Math.abs(u) <= site.size.width / 2 && Math.abs(v) <= site.size.depth / 2) return site;
    }
    return null;
  }

  // A swamp at x, z turned `turn`, if it fits there (see the top of the
  // file), with its water level and how deep it is dug; null if it doesn't.
  private fit(x: number, z: number, turn: number, size: Footprint, seed: number): Site | null {
    // Cheap first, round its middle: the outline comes in to about 0.79 of
    // its box (the oval swells and dips 12% either way) and its middle may
    // lie a quarter meter off the box's, so everything within `within` of
    // the box's middle is inside it, and within `dug` is dug.
    const within = 0.39 * Math.min(size.width, size.depth) - 0.3;
    const dug = within - DIG.rim;
    if (Math.hypot(x - LANDING.x, z - LANDING.z) < this.clearing + ROOM.clearing + within) return null;
    if (this.blocked(x, z, ROOM.blocked + within) || this.near(x, z, ROOM.apart + within)) return null;
    if (this.trunks.some((t) => Math.hypot(x - t.x, z - t.z) < t.radius + ROOM.trunk.dug + dug)) return null;
    if (this.boulders.some((b) => Math.hypot(x - b.x, z - b.z) < b.radius + ROOM.boulder + dug)) return null;

    const inside = ovalInside(size, seed);
    const site = { x, z, turn };
    const rim: number[] = []; // the land round the undug rim
    const middle: number[] = []; // and where it is dug fully
    let top = -Infinity; // the point furthest in, near its middle
    let topU = 0;
    let topV = 0;
    for (let u = -size.width / 2; u <= size.width / 2; u += SITE.grid) {
      for (let v = -size.depth / 2; v <= size.depth / 2; v += SITE.grid) {
        const d = inside(u, v);
        if (d < 0) continue;
        const p = toLake(site, u, v);
        if (Math.hypot(p.x, p.z) - shoreRadius(Math.atan2(p.z, p.x)) - BANK_WIDTH < ROOM.bank) return null;
        if (Math.hypot(p.x - LANDING.x, p.z - LANDING.z) < this.clearing + ROOM.clearing) return null;
        if (this.blocked(p.x, p.z, ROOM.blocked) || this.near(p.x, p.z, ROOM.apart)) return null;
        const dugHere = d >= DIG.rim;
        const trunkRoom = dugHere ? ROOM.trunk.dug : ROOM.trunk.rim;
        if (this.trunks.some((t) => Math.hypot(p.x - t.x, p.z - t.z) < t.radius + trunkRoom)) return null;
        if (dugHere && this.boulders.some((b) => Math.hypot(p.x - b.x, p.z - b.z) < b.radius + ROOM.boulder)) return null;
        const y = groundHeight(p.x, p.z, this.clearing);
        if (!dugHere) rim.push(y);
        else if (d >= DIG.rim + DIG.slope) middle.push(y);
        if (d > top) {
          top = d;
          topU = u;
          topV = v;
        }
      }
    }
    if (rim.length === 0 || middle.length === 0) return null;
    // Dug fully all round its middle: inside() comes to a point there, which
    // a hollow still sloping would show.
    for (let k = 0; k < 16; k++) {
      const a = (2 * Math.PI * k) / 16;
      if (inside(topU + 0.4 * Math.cos(a), topV + 0.4 * Math.sin(a)) < DIG.rim + DIG.slope) return null;
    }
    const level = Math.min(Math.min(...rim) - DIG.lip, DIG.level);
    middle.sort((a, b) => a - b);
    const depth = Math.max(0, middle[Math.floor(middle.length / 2)] - (level - DIG.depth));
    if (depth > DIG.most) return null;
    return { x, z, turn, size, seed, level, depth, inside, mask: ovalMask(size, seed), reach: Math.hypot(size.width, size.depth) / 2 };
  }
}

// A point of a swamp (its own coordinates) in the lake's, and back.
function toLake(site: Pick<Site, 'x' | 'z' | 'turn'>, u: number, v: number): { x: number; z: number } {
  const c = Math.cos(site.turn);
  const s = Math.sin(site.turn);
  return { x: site.x + c * u + s * v, z: site.z - s * u + c * v };
}

function toSite(site: Pick<Site, 'x' | 'z' | 'turn'>, x: number, z: number): { u: number; v: number } {
  const c = Math.cos(site.turn);
  const s = Math.sin(site.turn);
  const dx = x - site.x;
  const dz = z - site.z;
  return { u: c * dx - s * dz, v: s * dx + c * dz };
}
