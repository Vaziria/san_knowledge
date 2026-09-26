import { Dock, DOCK_SIZE, RAMP } from '../../figures/objects/Dock';
import { BANK_WIDTH, shoreRadius } from './Ground';

// Where a dock goes on the lake (Lake.ts, LakeOptions.dock; the lake
// meeting's): its landward end on the land LAND_IN m past the top of the
// bank, running straight out toward the lake's middle, at the first place
// along the shore, from the angle asked for, where all of it is clear:
//
// - off the boulders, the mud pits and the trees' feet, and whatever the one
//   asking says to keep off (the meeting's fish, its loop and its leaps);
// - its deck just over the highest land under its level part, and no higher
//   than HEIGHT.most; its ramp (Dock.ts, RAMP) coming down to the ground at
//   its landward end, so that an animal walks on from the land with no step.
//
// The stones in its way are cleared, as its builders would clear its site
// (inTheWay(); Lake.siteDock takes them away): a stone where one of its posts
// stands, or rising into its beams or, where the land is higher than those,
// its planks. With some 200 stones along the shore, there was nowhere a
// 3 m dock could cross the waterline without one. Nothing else on the lake
// moves for it. The lake's coordinates: its middle at the origin, y = 0 the
// still water.

// Straight across the lake from the landing, on its far shore, where the
// preview cameras and the lake meeting's look: where a dock is looked for
// first.
export const FAR_SHORE = -Math.PI / 2;

export interface DockPlace {
  angle: number; // radians from +x toward +z: where along the shore to start looking
  keepOff?: (x: number, z: number) => boolean; // a point of the lake it must not be over
}

// The deck, in the lake's coordinates: its landward end's middle, the way it
// runs out (as rotation.y turns +z), its size, and its top's height over the
// still water.
export interface Deck {
  x: number;
  z: number;
  heading: number;
  width: number;
  length: number;
  height: number;
  foot: number; // its top's height at its landward end, rising to `height` over RAMP m (Dock.ts)
}

export interface DockSiteOptions {
  place: DockPlace;
  groundAt(x: number, z: number): number;
  // Whether a boulder, a mud pit or a tree's foot is within `gap` m of a point.
  blocked(x: number, z: number, gap: number): boolean;
}

const SEARCH = { step: 0.02, most: Math.PI }; // radians between the angles tried, and the furthest either side of the one asked for
// Radians it may turn from running straight out toward the lake's middle, to
// fit between boulders, first the smaller turns.
const TURNS = [0, 0.12, -0.12, 0.24, -0.24, 0.35, -0.35];
const LAND_IN = 1.2; // m past the top of the bank the landward end is: room for a big animal to step on from land, clear of the bank's wet foot
const HEIGHT = { least: 0.36, most: 0.55, over: 0.01 }; // m: its deck's top over the still water, and its planks' underside over the highest land under its level part
const FOOT = 0.015; // m its ramp's foot stands over the ground at the middle of its landward end: the planks' edge
const ROOM = { side: 0.3, post: 0.1 }; // m kept from a boulder, pit or tree's foot, and between a post and a stone
const POST_RADIUS = 0.085; // m, the thickest post
const SAMPLE = 0.25; // m between the points checked, along and across
// The way onto it from the land: APPROACH.length m behind its landward end,
// and APPROACH.side m past its sides, clear of the boulders, mud pits and
// trees and on dry land, and its big stones cleared (inApproach()): with a
// boulder up the bank behind it, the lake meeting's wolf and deer, 1.7 m
// long, could not get onto the first dock found.
export const APPROACH = { length: 2.5, side: 0, dry: 0.12 }; // dry: m over the water, as the meeting's land counts it (Land.ts WET); the land's hollows dip to 8 cm

export function dockSite(options: DockSiteOptions): Deck | null {
  const steps = Math.round(SEARCH.most / SEARCH.step);
  for (let i = 0; i <= steps; i++) {
    for (const sign of i === 0 ? [1] : [1, -1]) {
      for (const turn of TURNS) {
        const deck = tryAt(options.place.angle + sign * i * SEARCH.step, turn, options);
        if (deck) return deck;
      }
    }
  }
  return null;
}

// A point of the dock (across it, along it) in the lake's coordinates.
export function deckPoint(deck: Pick<Deck, 'x' | 'z' | 'heading'>, across: number, along: number): { x: number; z: number } {
  const c = Math.cos(deck.heading);
  const s = Math.sin(deck.heading);
  return { x: deck.x + across * c + along * s, z: deck.z - across * s + along * c };
}

// A point of the lake in the dock's own coordinates: across it and along it.
export function onDeck(deck: Deck, x: number, z: number): { across: number; along: number } {
  const dx = x - deck.x;
  const dz = z - deck.z;
  const c = Math.cos(deck.heading);
  const s = Math.sin(deck.heading);
  return { across: dx * c - dz * s, along: dx * s + dz * c };
}

// Whether a stone (its top's height over the still water) is in a dock's
// way: where one of its posts stands, or rising into its beams, or, where
// the land is higher than those and they are bedded in it, its planks.
export function inTheWay(deck: Deck, stone: { x: number; z: number; radius: number; top: number }, groundAt: (x: number, z: number) => number): boolean {
  const { width, length } = deck;
  const posts = Dock.postSpots(width, length).map(({ x, z }) => deckPoint(deck, x, z));
  if (posts.some((p) => Math.hypot(p.x - stone.x, p.z - stone.z) < stone.radius + POST_RADIUS + ROOM.post)) return true;
  const { across, along } = onDeck(deck, stone.x, stone.z);
  const under = Math.abs(across) < width / 2 + stone.radius && along > -stone.radius && along < length + stone.radius;
  const { planks, beams } = Dock.undersides(Dock.topAt(along, deck.height, deck.foot));
  return under && stone.top > (groundAt(stone.x, stone.z) > beams ? planks : beams) - 0.01;
}

// Whether a point of the lake is within `gap` m of a dock's deck, under
// which its posts stand (the lake's floating logs keep off it).
export function nearDeck(deck: Deck, x: number, z: number, gap: number): boolean {
  const { across, along } = onDeck(deck, x, z);
  return Math.hypot(Math.max(0, Math.abs(across) - deck.width / 2), Math.max(0, -along, along - deck.length)) < gap;
}

// Whether a point of the lake is on the way onto a dock from the land
// (APPROACH), or within `pad` m of it.
export function inApproach(deck: Deck, x: number, z: number, pad = 0): boolean {
  const { across, along } = onDeck(deck, x, z);
  return Math.abs(across) < deck.width / 2 + APPROACH.side + pad && along < pad && along > -APPROACH.length - pad;
}

function tryAt(angle: number, turn: number, { place, groundAt, blocked }: DockSiteOptions): Deck | null {
  const { width, length } = DOCK_SIZE;
  const r = shoreRadius(angle) + BANK_WIDTH + LAND_IN;
  // Out toward the lake's middle, its +z being -(cos, sin) of the angle,
  // turned `turn` from that.
  const site = { x: r * Math.cos(angle), z: r * Math.sin(angle), heading: Math.atan2(-Math.cos(angle), -Math.sin(angle)) + turn };
  let land = -Infinity; // the highest land under its level part
  for (let along = 0; along <= length + 1e-6; along += SAMPLE) {
    for (let across = -width / 2; across <= width / 2 + 1e-6; across += SAMPLE) {
      const p = deckPoint(site, across, along);
      if (blocked(p.x, p.z, ROOM.side) || place.keepOff?.(p.x, p.z)) return null;
      if (along >= RAMP) land = Math.max(land, groundAt(p.x, p.z));
    }
  }
  // The way onto it: clear, and dry land.
  for (let along = -APPROACH.length; along < 0; along += SAMPLE) {
    for (let across = -width / 2 - APPROACH.side; across <= width / 2 + APPROACH.side + 1e-6; across += SAMPLE) {
      const p = deckPoint(site, across, along);
      if (blocked(p.x, p.z, ROOM.side) || groundAt(p.x, p.z) < APPROACH.dry) return null;
    }
  }
  const thick = HEIGHT.least - Dock.undersides(HEIGHT.least).planks;
  const height = Math.max(HEIGHT.least, land + thick + HEIGHT.over);
  if (height > HEIGHT.most) return null;
  return { ...site, width, length, height, foot: groundAt(site.x, site.z) + FOOT };
}
