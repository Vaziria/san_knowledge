import * as THREE from 'three';
import { BermudaGrass } from '../../figures/Grass/BermudaGrass';
import { ChivesGrass } from '../../figures/Grass/ChivesGrass';
import { CockFootGrass } from '../../figures/Grass/CockFootGrass';
import { MeadowFoxtailGrass } from '../../figures/Grass/MeadowFoxtailGrass';
import { between, seededRandom, type GrassOptions } from '../../figures/Grass/parts';
import { RedFescueGrass } from '../../figures/Grass/RedFescueGrass';
import { RosemaryGrass } from '../../figures/Grass/RosemaryGrass';
import { TimothyGrass } from '../../figures/Grass/TimothyGrass';
import type { Theme } from '../../theme';
import { materialsOf, sway, type Wind } from '../wind';
import { BANK_WIDTH, groundHeight, shoreRadius } from './Ground';

// A meadow on the uneven land round the lake: patches of the grass figures'
// kinds (figures/Grass), each patch one kind, as grass spreads, from low mats
// of Bermuda grass and fine tufts of fescue to tall timothy, foxtail and
// cock's-foot, with now and then a clump of chives or a rosemary bush. Every
// plant has its own size, from about half its kind's to a third bigger, is a
// little taller and slimmer or shorter and broader, and is turned its own
// way and shaded a little lighter or darker. Plants stand on the ground where
// they grow, leaning a little with its slope (a creeping mat lies along it).
// They are placed by a seeded random generator, so the lake is the same on
// every load.
//
// None grow near `keepClear` (where standing figures go) or right against a
// tree's trunk (`trunks`), but more patches ring keepClear (AROUND), where
// the preview cameras look, and the rest lie thicker toward the water. Plants
// within NEAR of keepClear, which the cameras see up close, are grown with
// most of their leaves (NEAR_DETAIL); the rest with far fewer and coarser
// (FAR_DETAIL), since from across the water they are a few pixels. Between
// them the land is covered by the sward (Sward.ts).
//
// Each kind is grown once for each detail and drawn as instances (one
// InstancedMesh per mesh of the plant), so hundreds of plants cost only
// what a few do to build, and a few dozen draw calls. They take shadows but
// cast none, which would add a shadow pass over all of them. They bend in the
// wind (../wind.ts), a meter up 22 cm in a full wind.

interface Patch {
  kind: new (options: GrassOptions) => THREE.Object3D;
  weight: number; // how often a patch is of this kind
  plants: readonly [number, number]; // how many in a patch
  spread: number; // m, a patch's radius
  follow: number; // how far a plant leans with the ground's slope: 1 lies along it
}

const KINDS: Patch[] = [
  { kind: BermudaGrass, weight: 1.5, plants: [2, 4], spread: 1.2, follow: 1 },
  { kind: RedFescueGrass, weight: 2.5, plants: [4, 8], spread: 0.9, follow: 0.4 },
  { kind: TimothyGrass, weight: 2.5, plants: [4, 7], spread: 1, follow: 0.3 },
  { kind: MeadowFoxtailGrass, weight: 2.5, plants: [4, 7], spread: 1, follow: 0.3 },
  { kind: CockFootGrass, weight: 2.5, plants: [3, 5], spread: 1.1, follow: 0.3 },
  { kind: ChivesGrass, weight: 0.8, plants: [2, 4], spread: 0.5, follow: 0.3 },
  { kind: RosemaryGrass, weight: 0.4, plants: [1, 2], spread: 0.8, follow: 0.2 },
];
const PATCHES = 50; // over the land
const AROUND = 32; // more, round keepClear
const FROM = 0.4; // m past the top of the bank where the meadow starts
const SPREAD = 15; // m beyond that over which patches lie, more of them near the water
const THICKER = 2; // how much thicker toward the water: the power of the random share of SPREAD
const SIZE = [0.55, 1.35] as const; // of the kind's size, for a patch
const EACH = [0.8, 1.2] as const; // of the patch's size, for a plant in it
const STRETCH = 0.15; // each is up to this share taller and slimmer, or shorter and broader
const TINT = [0.88, 1.08] as const; // each is a little lighter or darker
const CLEAR = 2.6; // m kept free round keepClear
const TRUNK = 1.2; // m kept free round a tree's trunk
const NEAR = 7; // m from keepClear: closer plants are grown with NEAR_DETAIL, and the AROUND patches lie within it
const NEAR_DETAIL = 0.4; // the share of their leaves and stems plants grow there
const FAR_DETAIL = 0.2; // and the rest, coarsened too (see coarsen() in figures/Grass/parts.ts)
const BEND = { sway: 0.22, reach: 1 };
const SEED = 11;

const UP = new THREE.Vector3(0, 1, 0);

// The way the ground faces at a point.
function groundNormal(x: number, z: number): THREE.Vector3 {
  const e = 0.05;
  const dx = (groundHeight(x + e, z) - groundHeight(x - e, z)) / (2 * e);
  const dz = (groundHeight(x, z + e) - groundHeight(x, z - e)) / (2 * e);
  return new THREE.Vector3(-dx, 1, -dz).normalize();
}

export class Meadow extends THREE.Group {
  constructor(theme: Theme, keepClear: THREE.Vector3, trunks: THREE.Vector3[], wind: Wind) {
    super();
    this.name = 'meadow';
    const random = seededRandom(SEED);
    const total = KINDS.reduce((sum, k) => sum + k.weight, 0);
    const pick = () => {
      let w = total * random();
      return KINDS.find((k) => (w -= k.weight) < 0) ?? KINDS[KINDS.length - 1];
    };

    // Each plant as where it stands, how it is turned and sized, and its
    // shade, listed by kind and by whether it is near.
    type Plant = { matrix: THREE.Matrix4; tint: number };
    const planted = new Map<Patch, { near: Plant[]; far: Plant[] }>(KINDS.map((k) => [k, { near: [], far: [] }]));
    const turn = new THREE.Quaternion();
    const lean = new THREE.Quaternion();
    for (let p = 0; p < AROUND + PATCHES; p++) {
      const patch = pick();
      const angle = 2 * Math.PI * random();
      let middle: THREE.Vector2;
      if (p < AROUND) {
        const out = between(random, CLEAR + 0.5 * patch.spread, NEAR);
        middle = new THREE.Vector2(keepClear.x + out * Math.cos(angle), keepClear.z + out * Math.sin(angle));
      } else {
        const r = shoreRadius(angle) + BANK_WIDTH + FROM + SPREAD * random() ** THICKER;
        middle = new THREE.Vector2(r * Math.cos(angle), r * Math.sin(angle));
      }
      const size = between(random, ...SIZE);
      const shade = between(random, ...TINT);
      const count = Math.round(between(random, ...patch.plants));
      for (let k = 0; k < count; k++) {
        const round = 2 * Math.PI * random();
        const out = patch.spread * Math.sqrt(random());
        const x = middle.x + out * Math.cos(round);
        const z = middle.y + out * Math.sin(round);
        const fromClear = Math.hypot(x - keepClear.x, z - keepClear.z);
        if (fromClear < CLEAR) continue;
        if (Math.hypot(x, z) - shoreRadius(Math.atan2(z, x)) - BANK_WIDTH < FROM) continue; // down the bank
        if (trunks.some((t) => Math.hypot(x - t.x, z - t.z) < TRUNK)) continue;

        const s = size * between(random, ...EACH);
        const stretch = 1 + STRETCH * (2 * random() - 1);
        lean.setFromUnitVectors(UP, groundNormal(x, z));
        lean.slerp(new THREE.Quaternion(), 1 - patch.follow);
        turn.setFromAxisAngle(UP, 2 * Math.PI * random());
        const matrix = new THREE.Matrix4().compose(
          new THREE.Vector3(x, groundHeight(x, z), z),
          lean.clone().multiply(turn),
          new THREE.Vector3(s / Math.sqrt(stretch), s * stretch, s / Math.sqrt(stretch)),
        );
        const plant = { matrix, tint: shade * between(random, 0.95, 1.05) };
        planted.get(patch)![fromClear < NEAR ? 'near' : 'far'].push(plant);
      }
    }

    // Grow each kind once for each detail it is seen at, and draw its meshes
    // as instances, one for each plant.
    const color = new THREE.Color();
    for (const [patch, { near, far }] of planted) {
      for (const [plants, detail] of [
        [near, NEAR_DETAIL],
        [far, FAR_DETAIL],
      ] as const) {
        if (plants.length === 0) continue;
        const grown = new patch.kind({ theme, detail });
        grown.updateMatrixWorld(true);
        for (const material of materialsOf(grown)) sway(material, wind, BEND);
        grown.traverse((object) => {
          if (!(object instanceof THREE.Mesh)) return;
          const instances = new THREE.InstancedMesh(object.geometry, object.material, plants.length);
          plants.forEach((plant, i) => {
            instances.setMatrixAt(i, plant.matrix.clone().multiply(object.matrixWorld));
            instances.setColorAt(i, color.setScalar(plant.tint));
          });
          instances.receiveShadow = true;
          this.add(instances);
        });
      }
    }
  }
}
