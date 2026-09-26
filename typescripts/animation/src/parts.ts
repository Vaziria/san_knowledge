import * as THREE from 'three';
import { Bear } from './figures/animals/Bear';
import { Bird } from './figures/animals/Bird';
import { Cat } from './figures/animals/Cat';
import { Deer } from './figures/animals/Deer';
import { Fox } from './figures/animals/Fox';
import { Frog } from './figures/animals/Frog';
import { Lion } from './figures/animals/Lion';
import { Penguin } from './figures/animals/Penguin';
import { Snake } from './figures/animals/Snake';
import { Wolf } from './figures/animals/Wolf';
import { Boat } from './figures/Boat/Boat';
import { Fish } from './figures/Fish/Fish';
import { FishingRod } from './figures/FishingRod/FishingRod';
import { BermudaGrass } from './figures/Grass/BermudaGrass';
import { ChivesGrass } from './figures/Grass/ChivesGrass';
import { CockFootGrass } from './figures/Grass/CockFootGrass';
import { MeadowFoxtailGrass } from './figures/Grass/MeadowFoxtailGrass';
import { RedFescueGrass } from './figures/Grass/RedFescueGrass';
import { RosemaryGrass } from './figures/Grass/RosemaryGrass';
import { TimothyGrass } from './figures/Grass/TimothyGrass';
import { Cliff } from './figures/objects/Cliff';
import { Firefly } from './figures/objects/Firefly';
import { Log } from './figures/objects/Log';
import { MudPit } from './figures/objects/MudPit';
import { CedarTree } from './figures/Tree/CedarTree';
import { ChestnutTree } from './figures/Tree/ChestnutTree';
import { ElmTree } from './figures/Tree/ElmTree';
import { MapleTree } from './figures/Tree/MapleTree';
import { OakTree } from './figures/Tree/OakTree';
import { PineTree } from './figures/Tree/PineTree';
import { SpruceTree } from './figures/Tree/SpruceTree';
import { WillowTree } from './figures/Tree/WillowTree';
import { previews, type Environment, type Preview } from './previews';
import type { Theme } from './theme';

// A figure's parts, to preview one on its own: the Figures tree lists them
// under their figure and ?part=<name> picks one (settings.ts). A part is what
// the figure exposes as a field (rules.md, Project rule 4): penguin.leftFlipper,
// a four-legged animal's leftFrontLeg, a tree's crown. The tree lists them
// without building the figures, so each figure's parts are written here, in
// the order the tree shows them. Every field of the figure's own that holds
// an Object3D (or a list of them) must be named: a renamed or new part is a
// type error until it is listed. A figure with no parts, or only one (the
// cloud's body), has no entry.

export interface Part {
  name: string; // lowercase with dashes, as the tree and ?part= write it
  objects(figure: THREE.Object3D): THREE.Object3D[]; // what it is made of, in the built figure
}

// The fields a figure has beyond a THREE.Group's that hold an Object3D or a
// list of them.
type PartField<F> = {
  [K in Exclude<keyof F, keyof THREE.Group>]: F[K] extends THREE.Object3D | readonly THREE.Object3D[] ? K : never;
}[Exclude<keyof F, keyof THREE.Group>];

// For each such field: true for one part named after the field ("leftWing"
// is "left-wing"), or, for a list, the names of the parts it holds, each an
// equal run of it in order (the frog's hind legs: a group per segment, the
// left leg's first); false for a field that holds no part.
type PartTable<F> = {
  [K in PartField<F>]: F[K] extends readonly THREE.Object3D[] ? boolean | readonly string[] : boolean;
};

function partsOf<F extends THREE.Object3D>(Kind: abstract new (...args: never[]) => F, table: PartTable<F>): Part[] {
  const parts: Part[] = [];
  for (const [field, how] of Object.entries(table) as [string, boolean | readonly string[]][]) {
    if (how === false) continue;
    const all = (figure: THREE.Object3D): THREE.Object3D[] => {
      if (!(figure instanceof Kind)) throw new Error(`${figure.name || figure.type} is not a ${Kind.name}`);
      return [(figure as unknown as Record<string, THREE.Object3D | THREE.Object3D[]>)[field]].flat();
    };
    if (how === true) {
      parts.push({ name: dashed(field), objects: all });
      continue;
    }
    how.forEach((name, i) =>
      parts.push({
        name,
        objects(figure) {
          const list = all(figure);
          const run = list.length / how.length;
          return list.slice(i * run, (i + 1) * run);
        },
      }),
    );
  }
  return parts;
}

// "leftFrontLeg" -> "left-front-leg".
function dashed(field: string): string {
  return field.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`);
}

const QUADRUPED = {
  head: true,
  body: true,
  tail: true,
  leftFrontLeg: true,
  rightFrontLeg: true,
  leftHindLeg: true,
  rightHindLeg: true,
  speechBubble: false,
} as const;
const TREE = { crown: true, trunk: true } as const;
const GRASS = { leaves: true, stems: true } as const;
const FISH = { body: true, tail: true, leftFin: true, rightFin: true } as const;

export const figureParts: Record<string, Part[]> = {
  boat: partsOf(Boat, { hull: true, rudder: true }),
  salmon: partsOf(Fish, FISH),
  piranha: partsOf(Fish, FISH),
  clownfish: partsOf(Fish, FISH),
  'fishing-rod': partsOf(FishingRod, { pole: true, hangingLine: true }),
  penguin: partsOf(Penguin, { body: true, leftFlipper: true, rightFlipper: true, leftFoot: true, rightFoot: true, tail: true, speechBubble: false }),
  cat: partsOf(Cat, QUADRUPED),
  wolf: partsOf(Wolf, QUADRUPED),
  deer: partsOf(Deer, QUADRUPED),
  bird: partsOf(Bird, { head: true, body: true, leftWing: true, rightWing: true, tail: true, leftLeg: true, rightLeg: true, speechBubble: false }),
  bear: partsOf(Bear, QUADRUPED),
  fox: partsOf(Fox, QUADRUPED),
  snake: partsOf(Snake, { head: true, body: true, speechBubble: false }),
  frog: partsOf(Frog, {
    body: true,
    frontLegs: ['left-front-leg', 'right-front-leg'],
    hindLegs: ['left-hind-leg', 'right-hind-leg'],
    speechBubble: false,
  }),
  lion: partsOf(Lion, QUADRUPED),
  'oak-tree': partsOf(OakTree, TREE),
  'cedar-tree': partsOf(CedarTree, TREE),
  'maple-tree': partsOf(MapleTree, TREE),
  'pine-tree': partsOf(PineTree, TREE),
  'chestnut-tree': partsOf(ChestnutTree, TREE),
  'spruce-tree': partsOf(SpruceTree, TREE),
  'elm-tree': partsOf(ElmTree, TREE),
  'willow-tree': partsOf(WillowTree, TREE),
  'cock-foot-grass': partsOf(CockFootGrass, GRASS),
  'bermuda-grass': partsOf(BermudaGrass, GRASS),
  'timothy-grass': partsOf(TimothyGrass, GRASS),
  'meadow-foxtail-grass': partsOf(MeadowFoxtailGrass, GRASS),
  'red-fescue-grass': partsOf(RedFescueGrass, GRASS),
  'chives-grass': partsOf(ChivesGrass, GRASS),
  'rosemary-grass': partsOf(RosemaryGrass, GRASS),
  cliff: partsOf(Cliff, { rock: true, scree: true }),
  log: partsOf(Log, { bark: true, cuts: true }),
  'mud-pit': partsOf(MudPit, { mud: true, puddles: true }),
  firefly: partsOf(Firefly, {
    body: true,
    lantern: true,
    wings: ['front-left-wing', 'front-right-wing', 'hind-left-wing', 'hind-right-wing'],
    legs: ['front-left-leg', 'front-right-leg', 'middle-left-leg', 'middle-right-leg', 'hind-left-leg', 'hind-right-leg'],
  }),
};

export function hasPart(figure: string, part: string): boolean {
  return figureParts[figure]?.some((p) => p.name === part) ?? false;
}

// The preview of one part of a figure, in place of the figure's own: the
// figure is built as its preview builds it, in the pose it is first shown in,
// and everything but the part is hidden, the parts attached to it too (the
// bird's body holds its head, wings, tail and legs). It stands still: no
// demo and no behaviours, which are the whole figure's. The camera looks at
// the part from the way the figure's preview looks at the figure, from as
// far as fits the part, and the sun's shadow fits the part. The same function
// for the same part every time, so the stage sees it unchanged.
export function partPreview(figure: string, part: string): (theme: Theme, environment: Environment) => Preview {
  const key = `${figure}/${part}`;
  let create = made.get(key);
  if (!create) {
    create = (theme, environment) => showPart(previews[figure](theme, environment), figure, part);
    made.set(key, create);
  }
  return create;
}

const made = new Map<string, (theme: Theme, environment: Environment) => Preview>();

// How far the camera stands from the part: this many times the radius of the
// ball round it, which fits the ball in the camera's 50° height with room
// round it.
const FIT = 2.7;
// The camera looks this share of that radius to the right of the part's
// middle, which puts the part left of the middle of the screen, clear of the
// panel, as the figures' own views do.
const ASIDE = 0.3;

function showPart(preview: Preview, figure: string, name: string): Preview {
  const parts = figureParts[figure];
  const part = parts.find((p) => p.name === name);
  if (!part) throw new Error(`${figure} has no part ${name}`);
  const root = preview.figure;
  preview.update(0, 0); // the pose it is first shown in
  root.updateMatrixWorld(true);

  // What shows: the part, less any other part inside it. Lights are not
  // hidden: they show nothing themselves (the firefly's glow).
  const shown = new Set<THREE.Object3D>();
  for (const object of part.objects(root)) object.traverse((o) => shown.add(o));
  for (const other of parts) {
    if (other === part) continue;
    for (const object of other.objects(root)) {
      if (shown.has(object)) object.traverse((o) => shown.delete(o));
    }
  }
  // Taken off every layer rather than made invisible, which would hide what
  // is inside it too: the part itself, when it hangs from a hidden mesh.
  root.traverse((o) => {
    if (!shown.has(o) && !(o instanceof THREE.Light)) o.layers.disableAll();
  });

  const box = boxOf(shown);
  if (box.isEmpty()) return { ...preview, update() {}, behaviours: [] };
  const sphere = box.getBoundingSphere(new THREE.Sphere());
  const target = new THREE.Vector3(...preview.target);
  const way = new THREE.Vector3(...preview.camera).sub(target).normalize();
  const right = new THREE.Vector3(0, 1, 0).cross(way).normalize(); // the screen's right, seen along -way
  const aim = sphere.center.clone().addScaledVector(right, ASIDE * sphere.radius);
  const camera = aim.clone().addScaledVector(way, FIT * sphere.radius);
  return {
    figure: root,
    props: preview.props,
    place: preview.place,
    camera: camera.toArray(),
    target: aim.toArray(),
    update() {},
    dispose: preview.dispose,
    bounds: box,
  };
}

// The box round what shows, in the figure's space (it stands at the origin).
function boxOf(objects: Iterable<THREE.Object3D>): THREE.Box3 {
  const box = new THREE.Box3();
  const each = new THREE.Box3();
  for (const object of objects) {
    if (object instanceof THREE.InstancedMesh) {
      object.computeBoundingBox();
      box.union(each.copy(object.boundingBox!).applyMatrix4(object.matrixWorld));
    } else if (object instanceof THREE.Mesh || object instanceof THREE.Line || object instanceof THREE.Points) {
      const geometry = object.geometry as THREE.BufferGeometry;
      if (!geometry.getAttribute('position')) continue;
      geometry.computeBoundingBox();
      box.union(each.copy(geometry.boundingBox!).applyMatrix4(object.matrixWorld));
    }
  }
  return box;
}
