import * as THREE from 'three';
import { seededRandom, type Season } from '../parts';
import { Joints } from './Joints';
import { Flights, layChips, layStones, plantGeometries, stepsOf, stoneGeometries, straight, VARIATIONS, type Step, type Stretch, type Variation } from './parts';
import { Stones } from './Stones';
import { Verge } from './Verge';

export type { Variation } from './parts';

export interface StonePathOptions {
  variation?: Variation; // regular by default
  length?: number; // m along it, 4.2 by default
  width?: number; // m across, 1.6 by default
  season?: Season; // spring by default
  seed?: number;
  // Laid along ways of its own instead of one straight strip (the forest
  // lake's paths): each stretch its course, width and variation.
  stretches?: readonly Stretch[];
  // How much light reaches the ground under a point (the forest lake's baked
  // light, 0.6..1), tinting what stands there; 1 by default.
  light?: (x: number, z: number) => number;
}

// A stone path (StonePath.md), as the user's reference sheet draws it: flat,
// chunky flagstones in the forest lake's look, in one of the sheet's five
// variations:
// - regular: stones laid close, grass in the joints and tufts along the
//   edges;
// - mossy: wider joints full of moss and plants, moss on some stones;
// - dirty: smaller stones spaced out in brown dirt, pebbles between;
// - ruined: broken stones, tilted, some missing, rubble round them;
// - stairs: oblong blocks, two to a step, grass at the sides.
// In the season's colours: green in spring and summer (more flowers in
// spring), gold grass and fallen leaves in autumn, snow in the joints and
// banked along the sides in winter, a little on the stones' tops.
//
// Meters. A strip `length` long along +z from the origin, `width` across,
// on flat ground (stairs on a slope rising 1 in 3 toward -z); or laid along
// the forest lake's paths (`stretches`), on their ground. A flagstone's top
// is at most 5 cm over the ground (TOP_MOST), so what walks there walks over
// it; a stair's treads are ground (stepAt).
//
// Its parts: the stones (drawn instanced, a few shapes of each kind), what
// fills between them (the joints) and the verge along its sides. Nothing
// moves; it has no behaviours.
export class StonePath extends THREE.Group {
  static readonly WIDTH = 1.6;
  static readonly LENGTH = 4.2;
  static readonly STAIR_SLOPE = 1 / 3; // the preview's stairs climb this

  readonly stones: Stones;
  readonly joints: Joints;
  readonly verge: Verge;
  // Its stairs' steps, for what stands on them, each flight's in order up
  // its course.
  readonly flights: Flights;

  constructor(options: StonePathOptions = {}) {
    super();
    const season = options.season ?? 'spring';
    const seed = options.seed ?? 19;
    const variation = options.variation ?? 'regular';
    const length = options.length ?? StonePath.LENGTH;
    const stretches: readonly Stretch[] = options.stretches ?? [
      {
        course: straight(length, variation === 'stairs' ? (_x, z) => (length - z) * StonePath.STAIR_SLOPE : undefined),
        width: options.width ?? StonePath.WIDTH,
        variation,
      },
    ];
    const light = options.light ?? (() => 1);
    const random = seededRandom(seed * 7919 + 17);
    const laid = stretches.map((stretch) => {
      const stones = layStones(stretch.course, stretch.width, stretch.variation, random, stretch.wear, stretch);
      const chips = layChips(stretch.course, stretch.width, stretch.variation, stones, random, stretch.wear, stretch);
      return { stretch, stones: [...stones, ...chips] };
    });
    const plants = plantGeometries(season, seed);
    this.stones = new Stones(
      laid.flatMap((l) => l.stones),
      stoneGeometries(season, seed),
      light,
      random,
    );
    this.joints = new Joints(laid, plants, season, light, random);
    this.verge = new Verge(laid, plants, season, light, random);
    this.add(this.stones, this.joints, this.verge);

    this.flights = new Flights(stretches.filter((s) => s.variation === 'stairs').map((s) => stepsOf(s.course, s.width, s.widthAt)));
  }

  get steps(): readonly Step[] {
    return this.flights.steps;
  }

  // The tread's height under a point of it, for standing on its stairs (the
  // walking camera); NaN off them.
  stepAt(x: number, z: number): number {
    return this.flights.stepAt(x, z);
  }

  // A ramp over its stairs instead: the height of a slope through the
  // middles of their risers, within half a riser of each tread, easing down
  // to `ground` over RAMP_EDGE m at its sides and over its first and last
  // steps, so it has no edge. For walking up them over a grid half a meter
  // to a cell (the forest lake meeting's land), where a tread's edge would
  // count as ground too steep to stand on. NaN off them.
  rampAt(x: number, z: number, ground: (x: number, z: number) => number): number {
    return this.flights.rampAt(x, z, ground);
  }
}

// The sheet's Variation row, for the preview: the five variations side by
// side, GAP m apart, each a strip StonePath.LENGTH long running toward +z,
// centred on the origin. Its parts are theirs, taken together.
export class StonePathVariations extends THREE.Group {
  static readonly GAP = 0.7;
  readonly paths: StonePath[];
  readonly stones: THREE.Object3D[];
  readonly joints: THREE.Object3D[];
  readonly verge: THREE.Object3D[];

  constructor(options: { season?: Season; seed?: number } = {}) {
    super();
    const step = StonePath.WIDTH + StonePathVariations.GAP;
    this.paths = VARIATIONS.map((variation, i) => {
      const path = new StonePath({ variation, season: options.season, seed: (options.seed ?? 19) + i });
      path.position.set((i - (VARIATIONS.length - 1) / 2) * step, 0, -StonePath.LENGTH / 2);
      this.add(path);
      return path;
    });
    this.stones = this.paths.map((p) => p.stones);
    this.joints = this.paths.map((p) => p.joints);
    this.verge = this.paths.map((p) => p.verge);
  }
}
