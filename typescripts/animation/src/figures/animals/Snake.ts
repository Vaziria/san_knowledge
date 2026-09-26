import * as THREE from 'three';
import { keepSharp } from '../../effects/kuwahara';
import { defaultTheme, gloss, surface } from '../../theme';
import { Animal, type AnimalShape, type Motion } from './Animal';
import { createMaterials, mesh, mix, palette, polygons, type AnimalOptions, type Palette } from './parts';
import { EYE, FLAT, HEAD, LENGTH, NECK_COLOR, SEGMENTS, SIDES, TAIL_COLOR, TAIL_TIP, TONGUE, WAVE, fadeAt, liftAt, panelColor, radiusAt, type Paint } from './snakeModel';

// A green snake, low poly like folded paper, in the shape the user modelled
// (snakeModel.ts): its body 1.4 m long, 1.54 m from its snout to the tip of
// its tail as it lies, and 10.7 cm thick. One tube of 48 six-sided rings, a
// little flatter than round: a thin tail thickening into the body, then a
// slightly thinner neck rising to hold the head 22 to 32 cm off the ground;
// a flat wedge of a head with small yellow eyes, each with a dark slit, and
// a thin forked tongue that flicks out for half a second every 2.5 s while
// it moves. Every face is flat and of one colour: green scales (the grass
// colour) with a dark zig-zag down the back, small dark blotches on the
// sides, a pale yellow belly and a dark tail tip.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts are the body (the tube, reshaped every
// frame) and the head, a group with its origin where the model puts it,
// past the neck's end, holding the eyes, the tongue and the mouth.
// Animation behaviour follows its spec, Snake.md: Jump(), Hold(figure),
// Walk(), Run() and Speech(text), plus Stop() (Animal.ts).
//
// It has no legs, so it walks and runs by slithering (lateral undulation):
// its body lies along a wave fixed on the ground, the model's wave, and as
// it moves forward every part follows the same wave, so nothing slides
// sideways, like feet that stay planted; the head end swings less, as the
// model's does. (The model's wave instead runs along a body that stays in
// one place, which would slide sideways if it moved.) Turning, the wave lies
// along the curve it is walking, so on a circle nothing slides either.
// Walking is a slow glide on the model's wave, running a quicker one with a
// tighter wave. Standing still it keeps its last curves. Its jump springs
// the whole body off the ground, bowed up in the middle, and it lifts its
// head a little higher to carry what it holds. It carries a figure up to
// 12 cm across in its mouth.

const UNIT = 0.14; // m per unit of the model: its body (LENGTH, 10 units) is 1.4 m, as the snake was
const MIDDLE = LENGTH / 2; // the body's middle (in the model's s), over the origin
const REST_PHASE = WAVE.number * MIDDLE; // the wave's phase at the middle when it is built: the model's pose at its start

// The wave by gait, as shares of the model's (its number, and its height to
// either side): walking and standing the model's own; running quicker and
// tighter. It changes only while the snake moves, so standing still it
// keeps its last curves.
const RUN_WAVE = { number: 1 / 0.75, height: 0.7 };
const WAVE_EASE = 3; // how fast the wave changes between gaits, per second of moving

// Turning: the body lies along an arc of the curve it walks, taken from how
// fast it turns for how far it goes, eased over BEND.ease m of going, and no
// tighter than BEND.most (1/m). A turn of more than PUT in one frame is the
// snake being put somewhere, not steered.
const BEND = { most: 1 / 0.6, ease: 0.4 };
const PUT = 0.5; // rad

const WALK = { stride: 0.3, cadence: 1, duty: 1 }; // m/s: it glides, so the "stride" is its speed
const RUN = { stride: 0.9, cadence: 1, duty: 1 };
const JUMP = 0.2;
const BOW = 0.5; // share of the jump the middle of the body bows up
const HOLD = 0.12;
const HOLD_RAISE = 0.15; // the neck lifts this much higher, as a share of its lift, while it holds something
const MOUTH = new THREE.Vector3(0, -0.08, 0.66).multiplyScalar(UNIT); // in the head: where a held figure goes, under the snout
const BUBBLE_ABOVE = 0.1; // m above the head
const BUBBLE_SCALE = 1.2; // times the penguin's: the camera stands back to take in its length, though it is low
const REACH = 1; // m from the origin the body never goes beyond, for its bounding sphere

const TRIANGLES = (SEGMENTS - 1) * SIDES * 2 + SIDES * 2; // tube sides, the tail's point and the neck's end
const ROUND = Array.from({ length: SIDES }, (_, k) => Math.PI / 2 + (2 * Math.PI * k) / SIDES); // corner k's angle round its ring, from the top

// Each ring's place along the body (s), its radius (m), how far it swings
// with the wave, how high it rests and how high the neck lifts it (m), and
// how far it bows up in a jump (a share).
const RINGS = Array.from({ length: SEGMENTS }, (_, i) => {
  const s = (i / (SEGMENTS - 1)) * LENGTH;
  return { s, radius: radiusAt(s) * UNIT, fade: fadeAt(s), rest: radiusAt(s) * FLAT * UNIT, lift: liftAt(s) * UNIT, bow: Math.sin((Math.PI * s) / LENGTH) };
});

// The body's pose: the wave's phase at the middle (it grows as the snake
// goes forward, so the wave stays where it is on the ground), the wave's
// number and height (per unit of the model and in m), the curve its middle
// follows (1/m, > 0 toward +x), how much higher the neck lifts, and how far
// the middle bows up (m).
interface Curves {
  phase: number;
  number: number;
  height: number;
  curve: number;
  raise: number;
  bow: number;
}

const REST: Curves = { phase: REST_PHASE, number: WAVE.number, height: WAVE.height * UNIT, curve: 0, raise: 0, bow: 0 };

// Reused by head(), pose(), reshape() and tongueGeometry().
const UP = new THREE.Vector3(0, 1, 0);
const tangent = new THREE.Vector3();
const side = new THREE.Vector3();
const up = new THREE.Vector3();
const basis = new THREE.Matrix4();
const e1 = new THREE.Vector3();
const e2 = new THREE.Vector3();
const forward = new THREE.Vector3();

// The middle of each ring, in the rig's axes. Straight, ring i is where the
// model puts it, z = (s - MIDDLE) units along the body, swinging to the side
// on the wave; turning, the line it swings about bends round an arc of
// `curve` through the origin, and it swings square to the arc.
function spine(centers: THREE.Vector3[], c: Curves): void {
  for (let i = 0; i < SEGMENTS; i++) {
    const ring = RINGS[i];
    const u = (ring.s - MIDDLE) * UNIT; // m along the arc from the middle
    const swing = c.height * ring.fade * Math.sin(c.phase + c.number * (ring.s - MIDDLE));
    const a = c.curve * u; // how far the arc has turned by then
    const x = Math.abs(c.curve) < 1e-9 ? 0 : (2 * Math.sin(a / 2) ** 2) / c.curve;
    const z = Math.abs(c.curve) < 1e-9 ? u : Math.sin(a) / c.curve;
    const y = ring.rest + ring.lift * (1 + c.raise) + c.bow * ring.bow;
    centers[i].set(x + Math.cos(a) * swing, y, z - Math.sin(a) * swing);
  }
}

// Where the head goes, past the end of the neck along it, and how it turns:
// pointing along the neck, but kept fairly level, as the model does.
function head(centers: THREE.Vector3[], position: THREE.Vector3, quaternion: THREE.Quaternion): void {
  const last = centers[SEGMENTS - 1];
  tangent.subVectors(last, centers[SEGMENTS - 2]).normalize();
  position.copy(last).addScaledVector(tangent, HEAD.ahead * UNIT);
  e1.set(tangent.x, tangent.y * HEAD.level, tangent.z).normalize(); // its +z
  e2.crossVectors(UP, e1).normalize(); // its +x
  up.crossVectors(e1, e2);
  quaternion.setFromRotationMatrix(basis.makeBasis(e2, up, e1));
}

// Where the head is at rest, for its speech bubble.
const HEAD_REST = (() => {
  const centers = RINGS.map(() => new THREE.Vector3());
  spine(centers, REST);
  const position = new THREE.Vector3();
  head(centers, position, new THREE.Quaternion());
  return position;
})();
const BUBBLE = HEAD_REST.clone().add(new THREE.Vector3(0, BUBBLE_ABOVE, 0));

// The model's colours, mixed from the theme's.
function paints(p: Palette): Record<Paint, THREE.Color> {
  const green = p.grass.clone(); // its scales
  return {
    green,
    dark: mix(green, p.dark, 0.8), // the zig-zag, the blotches, the tail's tip, the eyes' slits
    belly: mix(mix(p.light, p.glow, 0.6), green, 0.15), // pale yellow-cream
    eye: p.glow.clone(), // yellow
    red: mix(p.trim, p.dark, 0.15), // the tongue: the theme has no red, so the trim's orange, deepened
  };
}

export class Snake extends Animal {
  readonly body: THREE.Mesh;
  readonly head = new THREE.Group();

  private readonly tongue: THREE.Mesh;
  private readonly positions: THREE.BufferAttribute;
  private readonly normals: THREE.BufferAttribute;
  private readonly centers = RINGS.map(() => new THREE.Vector3());
  private readonly corners = Array.from({ length: SEGMENTS * SIDES }, () => new THREE.Vector3());
  private readonly tip = new THREE.Vector3();
  private readonly curves: Curves = { ...REST };
  private heading: number | null = null; // which way it faced last frame, to tell how fast it turns
  private flick = 0; // s into the tongue's cycle
  private flicking = false; // whether the tongue flicks out this cycle

  constructor(options: AnimalOptions = {}) {
    const theme = options.theme ?? defaultTheme;
    const shape: AnimalShape = { walk: WALK, run: RUN, jump: JUMP, hold: HOLD, bubble: BUBBLE, bubbleScale: BUBBLE_SCALE };
    super(theme, shape);
    this.name = 'snake';
    const m = createMaterials(theme);
    const colors = paints(palette(theme));

    // The body: flat faces, each of one colour, which stays with it as it
    // moves; their corners and normals are set every frame (reshape()).
    const vertices = TRIANGLES * 3;
    this.positions = new THREE.BufferAttribute(new Float32Array(vertices * 3), 3).setUsage(THREE.DynamicDrawUsage);
    this.normals = new THREE.BufferAttribute(new Float32Array(vertices * 3), 3).setUsage(THREE.DynamicDrawUsage);
    const paint: Paint[] = [];
    for (let i = 0; i < SEGMENTS - 1; i++) for (let k = 0; k < SIDES; k++) paint.push(panelColor(i, k), panelColor(i, k));
    for (let k = 0; k < SIDES; k++) paint.push(TAIL_COLOR);
    for (let k = 0; k < SIDES; k++) paint.push(NECK_COLOR);
    const color = new Float32Array(vertices * 3);
    paint.forEach((name, t) => {
      for (let v = 0; v < 3; v++) colors[name].toArray(color, (3 * t + v) * 3);
    });
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', this.positions);
    geometry.setAttribute('normal', this.normals);
    geometry.setAttribute('color', new THREE.BufferAttribute(color, 3));
    // It changes shape every frame, so it isn't culled (its bounds would go
    // stale), and its sphere is one no pose goes beyond.
    geometry.boundingSphere = new THREE.Sphere(new THREE.Vector3(), REACH);
    this.body = mesh(geometry, m.coat);
    this.body.frustumCulled = false;

    // The head: the model's faces, the left half mirrored from the right.
    const point = (p: readonly number[], x: number) => new THREE.Vector3(p[0] * x, p[1], p[2]).multiplyScalar(UNIT);
    const faces: THREE.Vector3[][] = [];
    const facePaint: Paint[] = [];
    for (const [a, b, c, name] of [...HEAD.faces, ...HEAD.back]) {
      const [pa, pb, pc] = [HEAD.points[a], HEAD.points[b], HEAD.points[c]];
      faces.push([point(pa, 1), point(pb, 1), point(pc, 1)], [point(pa, -1), point(pc, -1), point(pb, -1)]);
      facePaint.push(name, name);
    }
    this.head.add(mesh(polygons(faces, (_n, f) => colors[facePaint[f]]), m.coat));

    // The eyes, glossy: yellow in front, the dark slit behind, closed at
    // their base. They are small (2 cm), and at the preview's distance the
    // painterly filter wiped them out, so they are kept out of it.
    const eyeFaces: THREE.Vector3[][] = [];
    const eyePaint: Paint[] = [];
    for (const x of [1, -1]) {
      const corner = (name: keyof typeof EYE.points) => point([EYE.at[0] + EYE.points[name][0], EYE.at[1] + EYE.points[name][1], EYE.at[2] + EYE.points[name][2]], x);
      const turned = <T>(list: readonly T[]) => (x > 0 ? [...list] : [...list].reverse());
      for (const [a, b, c, name] of EYE.faces) {
        eyeFaces.push(turned([a, b, c]).map(corner));
        eyePaint.push(name);
      }
      eyeFaces.push(turned(EYE.base).map(corner));
      eyePaint.push('dark');
    }
    const eyes = gloss(0xffffff);
    eyes.vertexColors = true;
    keepSharp(eyes);
    this.head.add(mesh(polygons(eyeFaces, (_n, f) => colors[eyePaint[f]]), eyes));

    // The tongue: the model's three flat triangles, given a thickness as
    // deep as its stem is wide, so it shows edge on, and kept out of the
    // painterly filter, which would average it away.
    const tongue = surface(theme, colors.red);
    keepSharp(tongue);
    this.tongue = new THREE.Mesh(tongueGeometry(), tongue);
    this.tongue.position.fromArray(TONGUE.at).multiplyScalar(UNIT);
    this.tongue.visible = false;
    this.head.add(this.tongue);

    this.holder.position.copy(MOUTH);
    this.head.add(this.holder);
    this.rig.add(this.body, this.head);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    const c = this.curves;
    const moved = this.speed * mo.delta; // m it went forward this frame

    // The wave by gait, changing only while it moves.
    const ease = 1 - Math.exp(-WAVE_EASE * mo.delta * mo.moving);
    c.number += (WAVE.number * THREE.MathUtils.lerp(1, RUN_WAVE.number, mo.running) - c.number) * ease;
    c.height += (WAVE.height * UNIT * THREE.MathUtils.lerp(1, RUN_WAVE.height, mo.running) - c.height) * ease;
    // Going forward along the wave, which stays on the ground.
    c.phase += (c.number / UNIT) * moved;

    // The curve it walks: how far it turned for how far it went.
    forward.set(0, 0, 1).applyQuaternion(this.quaternion);
    const heading = Math.atan2(forward.x, forward.z);
    if (this.heading !== null && moved > 1e-6) {
      const turn = THREE.MathUtils.euclideanModulo(heading - this.heading + Math.PI, 2 * Math.PI) - Math.PI;
      if (Math.abs(turn) < PUT) c.curve += (THREE.MathUtils.clamp(turn / moved, -BEND.most, BEND.most) - c.curve) * (1 - Math.exp(-moved / BEND.ease));
    }
    this.heading = heading;

    // Off the ground in a jump, bowed up in the middle; the neck higher
    // while it carries something.
    this.rig.position.y = mo.height;
    c.bow = mo.height > 0 ? BOW * mo.height : 0;
    c.raise = HOLD_RAISE * mo.holding;
    spine(this.centers, c);
    this.reshape();
    head(this.centers, this.head.position, this.head.quaternion);

    // The tongue flicks out for a moment every few seconds while it moves
    // (but not past what it carries).
    this.flick += mo.delta;
    if (this.flick >= TONGUE.every) {
      this.flick %= TONGUE.every;
      this.flicking = mo.moving > 0.5;
    }
    const out = this.flick < TONGUE.out ? Math.sin((this.flick / TONGUE.out) * Math.PI) : 0;
    this.tongue.scale.z = Math.max(out, 0.001);
    this.tongue.visible = this.flicking && out > 0.02 && mo.holding < 0.5;

    this.bubbleShift.subVectors(this.head.position, HEAD_REST);
  }

  // Puts the rings round the spine, as the model does (each square to the
  // line through its neighbours, its first corner on top, a little flatter
  // than round), and writes the faces, turned to face out.
  private reshape(): void {
    const { centers, corners } = this;
    for (let i = 0; i < SEGMENTS; i++) {
      tangent.subVectors(centers[Math.min(i + 1, SEGMENTS - 1)], centers[Math.max(i - 1, 0)]).normalize();
      side.crossVectors(tangent, UP).normalize();
      up.crossVectors(side, tangent).normalize();
      const r = RINGS[i].radius;
      for (let k = 0; k < SIDES; k++) {
        corners[i * SIDES + k]
          .copy(centers[i])
          .addScaledVector(side, Math.cos(ROUND[k]) * r)
          .addScaledVector(up, Math.sin(ROUND[k]) * r * FLAT);
      }
    }
    tangent.subVectors(centers[1], centers[0]).normalize();
    this.tip.copy(centers[0]).addScaledVector(tangent, -TAIL_TIP * UNIT);

    let t = 0;
    for (let i = 0; i < SEGMENTS - 1; i++) {
      const ring = i * SIDES;
      const next = ring + SIDES;
      for (let k = 0; k < SIDES; k++) {
        const k1 = (k + 1) % SIDES;
        t = this.face(t, corners[ring + k], corners[next + k1], corners[ring + k1]);
        t = this.face(t, corners[ring + k], corners[next + k], corners[next + k1]);
      }
    }
    const last = (SEGMENTS - 1) * SIDES;
    for (let k = 0; k < SIDES; k++) t = this.face(t, corners[k], corners[(k + 1) % SIDES], this.tip);
    for (let k = 0; k < SIDES; k++) t = this.face(t, corners[last + ((k + 1) % SIDES)], corners[last + k], centers[SEGMENTS - 1]);
    this.positions.needsUpdate = true;
    this.normals.needsUpdate = true;
    this.body.geometry.boundingBox = null; // measured again when next asked for, in the pose of the moment
  }

  // Writes triangle t, flat: its corners and its normal.
  private face(t: number, a: THREE.Vector3, b: THREE.Vector3, c: THREE.Vector3): number {
    const ux = b.x - a.x;
    const uy = b.y - a.y;
    const uz = b.z - a.z;
    const vx = c.x - a.x;
    const vy = c.y - a.y;
    const vz = c.z - a.z;
    let nx = uy * vz - uz * vy;
    let ny = uz * vx - ux * vz;
    let nz = ux * vy - uy * vx;
    const length = Math.hypot(nx, ny, nz) || 1;
    nx /= length;
    ny /= length;
    nz /= length;
    const p = this.positions.array;
    const n = this.normals.array;
    let i = t * 9;
    p[i] = a.x;
    p[i + 1] = a.y;
    p[i + 2] = a.z;
    p[i + 3] = b.x;
    p[i + 4] = b.y;
    p[i + 5] = b.z;
    p[i + 6] = c.x;
    p[i + 7] = c.y;
    p[i + 8] = c.z;
    for (let v = 0; v < 3; v++, i += 3) {
      n[i] = nx;
      n[i + 1] = ny;
      n[i + 2] = nz;
    }
    return t + 1;
  }
}

// The tongue: each of the model's triangles made a thin slab, as deep as the
// stem is wide, its faces facing out. Its root is at the origin, and it
// points along +z.
function tongueGeometry(): THREE.BufferGeometry {
  const depth = TONGUE.width * UNIT; // half its thickness
  const faces: THREE.Vector3[][] = [];
  for (const triangle of TONGUE.triangles) {
    const top = triangle.map(([x, y, z]) => new THREE.Vector3(x, y, z).multiplyScalar(UNIT).setY(depth));
    const bottom = top.map((p) => p.clone().setY(-depth));
    const middle = new THREE.Vector3();
    for (const p of top) middle.add(p);
    middle.divideScalar(3).setY(0);
    const slab = [top, [...bottom].reverse(), ...[0, 1, 2].map((k) => [top[k], bottom[k], bottom[(k + 1) % 3], top[(k + 1) % 3]])];
    for (const face of slab) {
      // Facing away from the slab's middle.
      e1.subVectors(face[1], face[0]).cross(e2.subVectors(face[2], face[0]));
      e2.copy(face[0]).add(face[1]).add(face[2]).divideScalar(3).sub(middle);
      faces.push(e1.dot(e2) < 0 ? [...face].reverse() : face);
    }
  }
  return polygons(faces);
}
