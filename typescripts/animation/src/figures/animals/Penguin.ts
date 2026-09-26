import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { Animal, type Gait, type Motion } from './Animal';
import { createMaterials, lofted, mesh, mix, paintLoft, palette, polygons, type AnimalOptions, type Side } from './parts';
import { eyeFaces, PENGUIN_MODEL as MODEL, type PenguinColor, type PenguinLoft } from './penguinModel';

// A penguin, low poly like folded paper, in the shape the user modelled
// (penguinModel.ts): 55 cm tall, a round body and head in one, a white belly
// with yellow patches either side of the neck, a short beak black above an
// orange stripe, thin flippers white on the inside, flat orange feet turned
// out a little, a stubby tail, and small eyes, a light rim round a black
// middle. Every face is flat and of one colour (the black, white, orange and
// yellow of felt: body, light, trim, and trim toward glow); the eyes' black
// middles are glossy.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// penguin, where the model's page stands it: its lowest point (the heels) on
// the ground and the boxes round its parts centred front to back, so the
// model's own origin is 1.5 mm above it and 4.5 mm ahead. Its parts are the
// body (with the head, beak and eyes: the model makes the body and head one
// loft), two flippers, two feet and the tail, each a group with its origin at
// its pivot: the model's origin (the body), a shoulder, a point in the foot
// 1.5 cm ahead of its heel, the root of the tail.
//
// Animation behaviour follows its spec, Penguin.md: Flap(), Jump(),
// Hold(figure), Walk(), Run() and Speech(text), plus Stop() to end a walk or
// run. It is an animal (Animal.ts), so Jump(), Hold(), Walk(), Run(), Stop()
// and Speech() are the animals'; Flap() is its own. Call update(delta) once
// per frame.
//
// It waddles as the model does: everything but the feet (the model's `upper`,
// here the torso) rocks from side to side over them, about the model's origin
// on the ground between the feet, the foot bearing no weight lifts, and the
// flippers go out a little further with each step. Walking and running it
// moves itself forward the way it faces, at `speed`, stepping: each foot is
// down for half a stride and slides back under the body as the body moves on,
// so a planted foot stays put on the ground (19 cm a second walking, 52
// running). It leans forward a little walking and more running, bobs up over
// the planted foot, turns with the stepping one and wags its tail; running,
// its flippers go out and back. Flap() beats the flippers out three times. Its
// jump is a 12 cm hop, squashing as it crouches and lands, flippers lifting
// in the air; it holds a figure in front of its belly, the flippers wrapped
// round it, scaled down to 30 cm (its largest side), turned so its longest
// level side lies across.
//
// Beyond the model: its eyes are flat discs, which the model draws from both
// sides, so here each has its faces turned both ways, and neither casts a
// shadow (a disc on the head, it would only shadow the face it lies on). The
// model turns its left eye the same way as its right (rotation.y -0.5 on both,
// the left one mirrored in x), which set the left disc 52 degrees into the
// head, all but hidden from in front; here the left eye is the right one
// mirrored, as the model's mirroring meant.

const { unit } = MODEL;

// Gaits: a stride is how far a planted foot slides back under the body; each
// foot is down for half a stride (duty 0.5), the right then the left, so the
// speed is stride x cadence / duty.
const WALK: Gait = { stride: 0.06, cadence: 1.6, duty: 0.5 }; // 0.19 m/s
const RUN: Gait = { stride: 0.1, cadence: 2.6, duty: 0.5 }; // 0.52 m/s

// The waddle, walking and running (pose()). Walking it is the model's: it
// rocks 0.12 rad, a stepping foot lifts 0.12 units (18 mm), and the flippers
// go out 0.12 rad and up to 0.25 more with each step. Where the model has
// nothing to say, the numbers are the penguin's before it: the lean, bob and
// twist, and the run (lifting its feet 1.6 times as high as walking, as it
// did, its flippers out and back).
interface Waddle {
  rock: number; // radians it rocks from side to side, over the planted foot
  lift: number; // m a stepping foot rises
  out: number; // radians the flippers go out beyond their rest
  flap: number; // radians more they go out at the height of each step
  back: number; // radians the flippers swing back
  lean: number; // radians it leans forward
  bob: number; // m it rises over the planted foot
  twist: number; // radians it turns with the stepping foot
}
const WADDLE: Record<'walk' | 'run', Waddle> = {
  walk: {
    rock: MODEL.waddle.rock,
    lift: MODEL.waddle.lift * unit,
    out: MODEL.waddle.flippers,
    flap: MODEL.waddle.flap,
    back: 0,
    lean: 0.05,
    bob: 0.008,
    twist: 0.08,
  },
  run: { rock: 0.1, lift: 1.6 * MODEL.waddle.lift * unit, out: 0.55, flap: MODEL.waddle.flap, back: 0.4, lean: 0.28, bob: 0.015, twist: 0.06 },
};
const TAIL_WAG = 0.25; // radians the tail swings, against the body's twist

// Flap: the flippers beat out and back FLAP.count times.
const FLAP = { duration: 0.9, count: 3, angle: 1.1 }; // s, beats, radians out at the top of a beat

// Jump: a hop JUMP m high, as long as gravity keeps it up there (Animal.ts);
// it squashes by up to SQUASH of its height crouching and landing, and lifts
// its flippers JUMP_FLIPPERS rad at the top.
const JUMP = 0.12;
const SQUASH = 0.12;
const JUMP_FLIPPERS = 0.5;

// Hold: the held figure's middle, in front of the belly (a figure is scaled
// down to `size`, its largest side), and the flippers wrapped forward (`forward`,
// rotation.x) and in toward the body (`in`) round it.
const HOLD = { at: new THREE.Vector3(0, 0.175, 0.24), size: 0.3, forward: -0.9, in: 0.35 };

// Speech: the bubble's tail points down at the top of the head (0.55 m up)
// from 3.5 cm above it, at the size it has always had (the animals' bubbles
// are measured against it).
const BUBBLE = new THREE.Vector3(0, 0.585, 0.01);

// A flat face in meters, and its colour by the model's name for it.
interface Face {
  corners: THREE.Vector3[];
  color: PenguinColor;
}

// A lofted part's faces, in its own axes and in meters, coloured as the model
// colours them: in the model's own units, before they are scaled.
function loftFaces(part: PenguinLoft): Face[] {
  const surface = lofted(part);
  const colors = paintLoft(surface, part.paint);
  return surface.faces.map((face, f) => ({ corners: face.map((i) => surface.points[i].clone().multiplyScalar(unit)), color: colors[f] }));
}

// Faces turned `turn` radians about y and moved to `at` (in the model's
// units), as the model places a part.
function placed(faces: Face[], at: readonly number[], turn = 0): Face[] {
  const matrix = new THREE.Matrix4().makeRotationY(turn).setPosition(at[0] * unit, at[1] * unit, at[2] * unit);
  return faces.map(({ corners, color }) => ({ corners: corners.map((p) => p.clone().applyMatrix4(matrix)), color }));
}

// Faces mirrored to the other side of x = 0, their corners in the other
// order so that they still face out.
function mirrored(faces: Face[]): Face[] {
  return faces.map(({ corners, color }) => ({ corners: corners.map((p) => new THREE.Vector3(-p.x, p.y, p.z)).reverse(), color }));
}

// Faces, and the same faces turned round, so a thin shell shows from both
// sides.
function bothSides(faces: Face[]): Face[] {
  return [...faces, ...faces.map(({ corners, color }) => ({ corners: [...corners].reverse(), color }))];
}

export class Penguin extends Animal {
  // Each part is a group with its origin at its pivot, ready to animate.
  readonly body = new THREE.Group();
  readonly tail = new THREE.Group();
  readonly leftFlipper: THREE.Group;
  readonly rightFlipper: THREE.Group;
  readonly leftFoot: THREE.Group;
  readonly rightFoot: THREE.Group;

  // Everything but the feet, the model's `upper`: it rocks, leans, bobs and
  // twists over the feet, and carries what it holds. Its origin is the
  // model's, where the model stands it (`stand`).
  private readonly torso = new THREE.Group();
  private readonly stand = new THREE.Vector3();
  private flapElapsed = FLAP.duration; // not flapping

  constructor(options: AnimalOptions = {}) {
    const theme = options.theme ?? defaultTheme;
    super(theme, { walk: WALK, run: RUN, jump: JUMP, hold: HOLD.size, bubble: BUBBLE, bubbleScale: 1 });
    this.name = 'penguin';
    const m = createMaterials(theme);
    const p = palette(theme);
    // The model's colours from the theme: its black the body colour, its
    // white and the eyes' rims the light one, its orange the trim, and its
    // yellow the trim halfway to the glow's warm yellow.
    const colors: Record<PenguinColor, THREE.Color> = {
      black: p.body,
      white: p.light,
      yellow: mix(p.trim, p.glow, 0.5),
      orange: p.trim,
      eyeRim: p.light,
    };
    const coat = (faces: Face[]) =>
      mesh(
        polygons(
          faces.map((f) => f.corners),
          (_normal, i) => colors[faces[i].color],
        ),
        m.coat,
      );

    // The body and head, with the beak.
    this.body.name = 'body';
    this.body.add(coat([...loftFaces(MODEL.body), ...placed(loftFaces(MODEL.beak), MODEL.beak.at)]));
    // The eyes, their faces turned both ways: the rims in the coat, the
    // middles glossy.
    const eye = eyeFaces().map(({ corners, color }) => ({ corners: corners.map((c) => new THREE.Vector3(...c).multiplyScalar(unit)), color }));
    const right = placed(eye, MODEL.eye.at, MODEL.eye.turn);
    const eyes = bothSides([...right, ...mirrored(right)]);
    const rims = coat(eyes.filter((f) => f.color === 'eyeRim'));
    const middles = mesh(polygons(eyes.filter((f) => f.color === 'black').map((f) => f.corners)), m.eye);
    rims.castShadow = middles.castShadow = false;
    this.body.add(rims, middles);

    this.tail.name = 'tail';
    this.tail.position.fromArray(MODEL.tail.at).multiplyScalar(unit);
    this.tail.add(coat(loftFaces(MODEL.tail)));

    // Flippers: the right one as modelled, the left one mirrored, each
    // resting away from the body in an inner group, so its own rotation is 0
    // at rest.
    const flipper = loftFaces(MODEL.flipper);
    const makeFlipper = (side: Side) => {
      const part = new THREE.Group();
      part.name = side < 0 ? 'left-flipper' : 'right-flipper';
      const [x, y, z] = MODEL.flipper.at;
      part.position.set(side * x, y, z).multiplyScalar(unit);
      const rest = new THREE.Group();
      rest.rotation.z = side * MODEL.flipper.rest;
      rest.add(coat(side < 0 ? mirrored(flipper) : flipper));
      part.add(rest);
      return part;
    };
    this.leftFlipper = makeFlipper(-1);
    this.rightFlipper = makeFlipper(1);

    // Feet: the same loft both sides, turned out in an inner group.
    const foot = loftFaces(MODEL.foot);
    const makeFoot = (side: Side) => {
      const part = new THREE.Group();
      part.name = side < 0 ? 'left-foot' : 'right-foot';
      const [x, y, z] = MODEL.foot.at;
      part.position.set(side * x, y, z).multiplyScalar(unit);
      const turn = new THREE.Group();
      turn.rotation.y = side * MODEL.foot.turn;
      turn.add(coat(foot));
      part.add(turn);
      return part;
    };
    this.leftFoot = makeFoot(-1);
    this.rightFoot = makeFoot(1);

    this.torso.add(this.body, this.tail, this.leftFlipper, this.rightFlipper, this.holder);

    // Stand it as the model does: its lowest point on the ground and the
    // boxes round its parts centred front to back.
    const box = new THREE.Box3().setFromObject(this.torso);
    for (const part of [this.leftFoot, this.rightFoot]) box.union(new THREE.Box3().setFromObject(part));
    this.stand.set(0, -box.min.y, -(box.min.z + box.max.z) / 2);
    this.holder.position.copy(HOLD.at).sub(this.stand);

    this.rig.add(this.torso, this.leftFoot, this.rightFoot);
    this.update(0);
  }

  // Beats both flippers out and back three times. Calling it again restarts
  // the flap. The flippers don't flap while they hold a figure. Named as in
  // the spec, Penguin.md.
  Flap(): void {
    this.flapElapsed = 0;
  }

  protected pose(mo: Motion): void {
    // Flap: |sin| gives one beat out and back per half turn.
    let flap = 0;
    if (this.flapElapsed < FLAP.duration) {
      this.flapElapsed += mo.delta;
      const p = Math.min(1, this.flapElapsed / FLAP.duration);
      flap = FLAP.angle * Math.abs(Math.sin(Math.PI * FLAP.count * p));
    }

    // Jump: the rig rises, and squashes crouching and landing; its origin is
    // on the ground, so squashing keeps the feet down.
    const squash = SQUASH * mo.crouch;
    this.rig.position.y = mo.height;
    this.rig.scale.set(1 + squash / 2, 1 - squash, 1 + squash / 2);

    // The waddle, eased in as it starts walking: rocking over the planted
    // foot, as the model rocks its `upper`, leaning, bobbing and twisting.
    const waddle = (key: keyof Waddle) => THREE.MathUtils.lerp(WADDLE.walk[key], WADDLE.run[key], mo.running) * mo.moving;
    const wave = Math.sin(2 * Math.PI * mo.phase); // positive while the right foot is planted, negative while the left is
    this.torso.position.copy(this.stand);
    this.torso.position.y += waddle('bob') * Math.abs(wave);
    this.torso.rotation.set(waddle('lean'), waddle('twist') * wave, -waddle('rock') * wave); // rotation.z < 0 rocks it toward +x, the right foot
    this.tail.rotation.y = -TAIL_WAG * mo.moving * wave;

    // Feet: each planted for half a stride, sliding back under the moving
    // body, then lifted and brought forward, flat, as the model lifts the
    // foot that bears no weight. The left foot is half a stride behind the
    // right.
    const lift = waddle('lift');
    const [x, y, z] = MODEL.foot.at;
    for (const [foot, side, lag] of [
      [this.rightFoot, 1, 0],
      [this.leftFoot, -1, 0.5],
    ] as const) {
      const p = (mo.phase + lag) % 1;
      let ahead = mo.stride / 2 - mo.stride * (p / mo.duty);
      let up = 0;
      if (p >= mo.duty) {
        const s = (p - mo.duty) / (1 - mo.duty);
        ahead = -mo.stride / 2 + mo.stride * THREE.MathUtils.smoothstep(s, 0, 1);
        up = lift * Math.sin(Math.PI * s);
      }
      foot.position.set(side * x * unit, y * unit + up, z * unit + ahead).add(this.stand);
    }

    // Flippers: out and in is rotation.z (out is negative on the left,
    // positive on the right), forward and back rotation.x. Holding wins over
    // flapping, the lift in the air and the waddle.
    const out = flap + JUMP_FLIPPERS * (mo.height / JUMP) + waddle('out') + waddle('flap') * Math.abs(wave);
    const back = waddle('back');
    for (const [flipper, side] of [
      [this.leftFlipper, -1],
      [this.rightFlipper, 1],
    ] as const) {
      flipper.rotation.z = side * (out * (1 - mo.holding) - HOLD.in * mo.holding);
      flipper.rotation.x = HOLD.forward * mo.holding + back * (1 - mo.holding);
    }
  }
}
