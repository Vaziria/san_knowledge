import * as THREE from 'three';
import { between, randomUnit } from './parts';

// The wood of a tree, grown in code: its trunk and branches as a skeleton of
// nodes, each a short step from its parent, and the limbs and twig tips read
// from it. Trees grow by space colonization (Runions et al., 2007): points
// are scattered through the shape the crown should have, and branches grow
// toward them step by step, each point dropping out once a branch reaches it.
// So the crown fills its shape the way real branches share out the light.
// Each kind of tree gives its habit (its growth form): the trunk and the
// first big limbs, drawn by hand, the crown's shape, how far branches sense
// the points, and how they bend. The spruce builds its tiers of branches
// itself instead (SpruceTree.ts).

export interface Limb {
  points: THREE.Vector3[]; // its axis, from where it leaves its parent (or the ground) to its tip
  radii: number[]; // its radius at each point
}

export interface Tip {
  position: THREE.Vector3;
  direction: THREE.Vector3; // the way the twig grows, a unit vector
}

export interface SkeletonNode {
  position: THREE.Vector3;
  parent: number; // -1 for the foot of the trunk
  children: number[];
  radius: number;
}

export interface Skeleton {
  nodes: SkeletonNode[]; // [0] is the foot of the trunk; a node comes after its parent
  limbs: Limb[]; // [0] is the trunk
  tips: Tip[];
}

export interface Habit {
  trunk: THREE.Vector3[]; // the trunk's axis, from the ground up to where it ends or splits
  limbs?: THREE.Vector3[][]; // big limbs drawn by hand, each starting on the trunk
  crown: (p: THREE.Vector3) => boolean; // whether a point is inside the crown
  box: THREE.Box3; // a box around the crown, to scatter the points in
  points: number; // how many points to scatter
  step: number; // m a branch grows in a step
  reach: number; // m from which a point draws the nearest branch toward it
  kill: number; // m: a point drops out once a branch comes this close
  tropism?: (position: THREE.Vector3) => THREE.Vector3; // added to each step's direction: up for rising branches, down for drooping ones
  wander?: number; // a random turn added to each step, for crooked wood
  radius: number; // the trunk's radius at its foot, before its roots swell it
  pipe?: number; // the pipe model's exponent, see grow()
}

const MAX_STEPS = 300;
const SMOOTHING = 2; // passes that ease the kinks out of the grown branches

export function grow(habit: Habit, random: () => number): Skeleton {
  const nodes: SkeletonNode[] = [];
  const add = (position: THREE.Vector3, parent: number) => {
    nodes.push({ position, parent, children: [], radius: 0 });
    if (parent >= 0) nodes[parent].children.push(nodes.length - 1);
    return nodes.length - 1;
  };

  // The hand-drawn wood: nodes every step along the trunk, then along each
  // limb, which grows from the trunk node nearest its start.
  const along = (points: THREE.Vector3[], parent: number, skipFirst: boolean) => {
    const axis = new THREE.CatmullRomCurve3(points);
    const count = Math.max(1, Math.round(axis.getLength() / habit.step));
    for (let i = skipFirst ? 1 : 0; i <= count; i++) parent = add(axis.getPointAt(i / count), parent);
  };
  along(habit.trunk, -1, false);
  const trunkCount = nodes.length;
  for (const limb of habit.limbs ?? []) {
    let from = 0;
    for (let k = 1; k < trunkCount; k++) {
      if (nodes[k].position.distanceTo(limb[0]) < nodes[from].position.distanceTo(limb[0])) from = k;
    }
    along([nodes[from].position, ...limb.slice(1)], from, true);
  }
  const drawn = nodes.length;

  // The points, scattered evenly through the crown.
  const points: THREE.Vector3[] = [];
  const size = habit.box.getSize(new THREE.Vector3());
  for (let tries = 0; points.length < habit.points && tries < habit.points * 1000; tries++) {
    const p = new THREE.Vector3(random(), random(), random()).multiply(size).add(habit.box.min);
    if (habit.crown(p)) points.push(p);
  }

  // Each living point keeps its nearest node. measure() checks the living
  // points against the nodes from `first` on, so each step only compares
  // them with the nodes it added.
  const nearest = new Int32Array(points.length).fill(-1);
  const distance = new Float64Array(points.length).fill(Infinity);
  const alive = new Uint8Array(points.length).fill(1);
  const measure = (first: number) => {
    for (let a = 0; a < points.length; a++) {
      if (!alive[a]) continue;
      for (let k = first; k < nodes.length; k++) {
        const d = points[a].distanceTo(nodes[k].position);
        if (d < habit.kill) {
          alive[a] = 0;
          break;
        }
        if (d < distance[a]) {
          distance[a] = d;
          nearest[a] = k;
        }
      }
    }
  };
  measure(0);

  // Each step, every node that points draw grows a shoot toward them.
  const pull = new Map<number, THREE.Vector3>();
  for (let s = 0; s < MAX_STEPS; s++) {
    pull.clear();
    for (let a = 0; a < points.length; a++) {
      if (!alive[a] || distance[a] > habit.reach) continue;
      const toward = points[a].clone().sub(nodes[nearest[a]].position).normalize();
      const sum = pull.get(nearest[a]);
      if (sum) sum.add(toward);
      else pull.set(nearest[a], toward);
    }
    const first = nodes.length;
    for (const [k, sum] of pull) {
      if (sum.lengthSq() < 1e-6) continue; // drawn equally both ways
      const from = nodes[k].position;
      const direction = sum.normalize();
      if (habit.tropism) direction.add(habit.tropism(from));
      if (habit.wander) direction.addScaledVector(randomUnit(random), habit.wander);
      const position = from.clone().addScaledVector(direction.normalize(), habit.step);
      // Drawn the same way again, it would grow a twin of its last shoot.
      if (nodes[k].children.some((c) => nodes[c].position.distanceTo(position) < 0.3 * habit.step)) continue;
      add(position, k);
    }
    if (nodes.length === first) break;
    measure(first);
  }

  // Radii by the pipe model: a branch's cross-section feeds its children's,
  // r^pipe = sum of the children's r^pipe, from the twig tips down to the
  // trunk, then scaled to the trunk's radius. A child always comes after its
  // parent, so walking backwards meets every child first.
  const pipe = habit.pipe ?? 2.5;
  for (let i = nodes.length - 1; i >= 0; i--) {
    const node = nodes[i];
    node.radius =
      node.children.length === 0 ? 1 : Math.pow(node.children.reduce((sum, c) => sum + nodes[c].radius ** pipe, 0), 1 / pipe);
  }
  const scale = habit.radius / nodes[0].radius;
  for (const node of nodes) node.radius *= scale;

  // Ease the kinks out of the grown branches: each node moves halfway toward
  // the middle of its parent and children. The hand-drawn wood stays.
  const middle = new THREE.Vector3();
  for (let pass = 0; pass < SMOOTHING; pass++) {
    for (let i = drawn; i < nodes.length; i++) {
      const node = nodes[i];
      if (node.children.length === 0) continue;
      middle.set(0, 0, 0);
      for (const c of node.children) middle.add(nodes[c].position);
      middle.divideScalar(node.children.length).add(nodes[node.parent].position).multiplyScalar(0.5);
      node.position.lerp(middle, 0.5);
    }
  }

  // Limbs: from the foot of the trunk, each limb follows its thickest child
  // on; every other child starts a limb of its own, beginning at the parent's
  // axis so it grows out of the wood.
  const limbs: Limb[] = [];
  const starts = [0];
  while (starts.length > 0) {
    let i = starts.pop()!;
    const parent = nodes[i].parent;
    const limb: Limb = { points: [], radii: [] };
    if (parent >= 0) {
      limb.points.push(nodes[parent].position);
      limb.radii.push(nodes[i].radius);
    }
    for (;;) {
      const node = nodes[i];
      limb.points.push(node.position);
      limb.radii.push(node.radius);
      if (node.children.length === 0) break;
      const thickest = node.children.reduce((a, b) => (nodes[b].radius > nodes[a].radius ? b : a));
      for (const c of node.children) if (c !== thickest) starts.push(c);
      i = thickest;
    }
    limbs.push(limb);
  }

  const tips: Tip[] = [];
  for (const node of nodes) {
    if (node.children.length > 0 || node.parent < 0) continue;
    const direction = node.position.clone().sub(nodes[node.parent].position).normalize();
    tips.push({ position: node.position, direction });
  }
  return { nodes, limbs, tips };
}

// Big limbs fanning out from `from`, evenly round the trunk with a little
// randomness: each rises `rise` meters while it spreads `spread` meters out.
// `bend` says how it curves: low rises steeply first and spreads at the top
// (a vase), high spreads first and rises at the end (an oak's wide limbs).
export function fan(
  from: THREE.Vector3,
  count: number,
  spread: number,
  rise: number,
  bend: number,
  random: () => number,
): THREE.Vector3[][] {
  const start = 2 * Math.PI * random();
  return Array.from({ length: count }, (_, i) => {
    const angle = start + ((i + between(random, -0.25, 0.25)) / count) * 2 * Math.PI;
    const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
    const s = spread * between(random, 0.8, 1.2);
    const r = rise * between(random, 0.8, 1.2);
    const mid = from.clone().addScaledVector(out, s * bend * 0.6).setY(from.y + r * (1 - bend * 0.6));
    const end = from.clone().addScaledVector(out, s).setY(from.y + r);
    return [from.clone(), mid, end];
  });
}

// How far p is inside an ellipsoid: under 1 inside, 1 on its surface.
export function ellipsoid(p: THREE.Vector3, center: THREE.Vector3, radii: THREE.Vector3): number {
  const x = (p.x - center.x) / radii.x;
  const y = (p.y - center.y) / radii.y;
  const z = (p.z - center.z) / radii.z;
  return Math.sqrt(x * x + y * y + z * z);
}

// A crown's outline is never a clean shape: this makes a factor round 1 that
// swells and dents it by up to `amount`, in a few lobes round the trunk that
// shift with height, so a crown shape can be made irregular.
export function wobble(amount: number, random: () => number): (p: THREE.Vector3) => number {
  const phases = [0, 1, 2].map(() => 2 * Math.PI * random());
  return (p) => {
    const angle = Math.atan2(p.z, p.x);
    return (
      1 +
      amount *
        (0.5 * Math.sin(3 * angle + phases[0] + 0.3 * p.y) +
          0.3 * Math.sin(5 * angle + phases[1] - 0.2 * p.y) +
          0.2 * Math.sin(2 * angle + phases[2]))
    );
  };
}
