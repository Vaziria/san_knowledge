import * as THREE from 'three';
import { defaultTheme, gloss } from '../../theme';
import { Animal, type AnimalShape, type Motion } from './Animal';
import { FROG_MODEL, type FrogColor, type FrogLoft, type Placement } from './frogModel';
import { createMaterials, lofted, mesh, mix, paintLoft, palette, polygons, type AnimalOptions, type Lofted } from './parts';

// A common pond frog, low poly like folded paper, in the shape the user
// modelled (frogModel.ts, from Frog.md's shape reference): 8.6 cm from its
// rump to the tip of its snout, 5.4 cm to the top of its eyes and 11.7 cm
// across its hind feet. Its body and head are one piece, tilted up toward the
// front (a frog has no neck), with bulging eyes on top, short front legs
// ending in flat hands, and long hind legs folded at its sides: the thigh
// forward, the shin back beside it, the long webbed foot flat. Every face is
// flat and of one colour, as the model colours it: green on top (the theme's
// grass) with dark green spots, a cream belly and throat, dark green hands and
// feet, and glossy gold eyes with a black pupil.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of it: the model stands it on its lowest point and centres it front
// to back. Its parts are the body (with the head and eyes), two front legs,
// each one piece turning at its shoulder, and two hind legs, each three pieces
// turning at the joints where the model's lofts meet: the hip, the knee and
// the ankle. Sitting still, it is the model, face for face. Animation
// behaviour follows its spec, Frog.md: Jump(), Hold(figure), Walk(), Run() and
// Speech(text), plus Stop() (Animal.ts).
//
// It hops rather than walks: Walk() is a string of small hops and Run() long,
// quick ones. Between hops it sits still on the ground, so its feet never
// slide, however the preview steers it: each hop starts where its body sits
// and which way it faces, and while it sits the rig holds it there against
// the animal's steady forward motion and turning; in the air it carries the
// body on to where the figure has got to. Its hind legs fling out straight
// behind it and fold again as it lands, each piece turning about the upright
// through its joint, so it keeps the height it has sitting and the webbed
// foot stays flat, and its front legs swing forward. Jump() is one high leap,
// the legs flung out the same way, with no crouch before it: its legs are
// rigid pieces and it sits on the lowest corners of its shins, so any sinking
// of the body to crouch would push them into the ground. It carries a figure
// up to its own length in its mouth, at the tip of its snout.

// Hops: a gait is how far a hop carries it, how many hops a second, and duty
// 1 (it glides, as far as Animal is concerned: the speed is stride x cadence).
// Each hop it sits for `sit` of the time, then flies `height` high.
const WALK = { stride: 0.055, cadence: 1.5, duty: 1 };
const RUN = { stride: 0.16, cadence: 2.1, duty: 1 };
const SIT = { walk: 0.55, run: 0.3 };
const HOP_HEIGHT = { walk: 0.018, run: 0.04 };
const HOP_REACH = 0.45; // how far the legs fling out in a small hop, as a share of all the way
const HOP_PITCH = { walk: 0.15, run: 0.35 }; // radians it points nose up leaving the ground, and down landing
const JUMP = 0.12; // m, one high leap
const HOLD = 0.09; // m, the largest figure it carries without scaling it down
// It pitches about the middle of its body: the middle of the body loft's
// widest section (the model's "widest part of the belly").
const MIDDLE = 2;

// Legs flung out: each piece of a hind leg turns about the upright through
// its joint until it points straight back, this many radians out from it
// (thigh, shin, foot); a front leg swings forward about its shoulder.
const SPLAY = [0.1, 0.06, 0.04] as const;
const REACH = 0.7; // radians

// Its speech bubble's tail, this far above the top of its eyes. The bubble is
// half the penguin's, as the bird's: the lake meeting's close-up comes no
// nearer than 0.6 m, and there words any smaller than this were a few pixels
// high in the 720p stream. Its preview's camera stands 45 cm off.
const BUBBLE_ABOVE = 0.022;
const BUBBLE_SCALE = 0.5;

const UP = new THREE.Vector3(0, 1, 0);
const RIGHT = new THREE.Vector3(1, 0, 0);

// Which material each of the model's colours is drawn with: the skin's on
// the coat (its colour per face), and the eye's gold and pupil glossy.
const MATERIAL: Record<FrogColor, 'coat' | 'iris' | 'pupil'> = { green: 'coat', darkGreen: 'coat', cream: 'coat', gold: 'iris', black: 'pupil' };

// A piece of the frog: a loft of the model, its surface, and the matrix from
// the loft's own units and axes to the frog's, in meters, standing where the
// model stands it.
interface Piece {
  loft: FrogLoft;
  surface: Lofted;
  matrix: THREE.Matrix4;
}

// A hind leg: its pieces (the thigh, shin and foot) and their groups, the
// joints each turns at when it sits (the hip, the knee and the ankle; then
// the middle of the toes), and how far each turns about the upright to fling
// it out straight.
interface HindLeg {
  pieces: Piece[];
  joints: THREE.Vector3[];
  fling: number[];
  segments: THREE.Group[];
}

// Where the model puts a part, as three.js places it (Object3D.updateMatrix).
function matrixOf(p: Placement): THREE.Matrix4 {
  return new THREE.Matrix4().compose(new THREE.Vector3(...p.at), new THREE.Quaternion().setFromEuler(new THREE.Euler(...p.turn)), new THREE.Vector3(...p.scale));
}

// The middle of a ring of one of a hind leg's lofts, in the leg's axes (the
// loft moved `x` along them, as the model moves it); a ring below 0 counts
// back from the last.
function middle(loft: FrogLoft & { x: number }, ring: number): THREE.Vector3 {
  const [y, z] = loft.sections[ring < 0 ? loft.sections.length + ring : ring];
  return new THREE.Vector3(loft.x, y, z);
}

// The frog's pieces and joints, where the model puts them, in meters.
function build() {
  const { unit } = FROG_MODEL;
  const piece = (loft: FrogLoft, matrix = new THREE.Matrix4()): Piece => ({ loft, surface: lofted(loft), matrix });
  const legOf = (p: Placement) => FROG_MODEL.hind.map((segment) => piece(segment, matrixOf(p).multiply(new THREE.Matrix4().makeTranslation(segment.x, 0, 0))));
  const body = piece(FROG_MODEL.body);
  const eyes = FROG_MODEL.eyes.map((p) => piece(FROG_MODEL.eye, matrixOf(p)));
  const fronts = FROG_MODEL.fronts.map((p) => piece(FROG_MODEL.front, matrixOf(p)));
  const hinds = FROG_MODEL.hinds.map(legOf);
  const all = [body, ...eyes, ...fronts, ...hinds.flat()];

  // Stood on the ground and centred front to back as the model does it, by
  // the box round the pieces' own boxes, each turned as its piece is (as
  // three.js's Box3.setFromObject measures), then made meters.
  const box = new THREE.Box3();
  const each = new THREE.Box3();
  for (const p of all) box.union(each.setFromPoints(p.surface.points).applyMatrix4(p.matrix));
  const stand = new THREE.Matrix4().makeScale(unit, unit, unit).multiply(new THREE.Matrix4().makeTranslation(0, -box.min.y, -(box.min.z + box.max.z) / 2));
  for (const p of all) p.matrix.premultiply(stand);

  // A hind leg's joints, where its lofts meet: the hip where the thigh
  // starts, the knee between the thigh's last ring and the shin's first, the
  // ankle between the shin's last and the foot's first; and the middle of the
  // toes, the foot's last ring. Each piece flings out the short way round to
  // point back, SPLAY out from straight back: the thigh and the foot, which
  // point forward, swing out and round.
  const [thigh, shin, foot] = FROG_MODEL.hind;
  const legs: Omit<HindLeg, 'segments'>[] = FROG_MODEL.hinds.map((p, k) => {
    const leg = stand.clone().multiply(matrixOf(p));
    const side = Math.sign(p.at[0]);
    const joints = [middle(thigh, 0), middle(thigh, -1).lerp(middle(shin, 0), 0.5), middle(shin, -1).lerp(middle(foot, 0), 0.5), middle(foot, -1)].map((j) => j.applyMatrix4(leg));
    const fling = SPLAY.map((splay, i) => {
      const way = joints[i + 1].clone().sub(joints[i]);
      const turn = side * (Math.PI - splay) - Math.atan2(way.x, way.z);
      return THREE.MathUtils.euclideanModulo(turn + Math.PI, 2 * Math.PI) - Math.PI;
    });
    return { pieces: hinds[k], joints, fling };
  });

  const at = (x: number, y: number, z: number) => new THREE.Vector3(x, y, z).applyMatrix4(stand);
  const [eyeX, eyeY, eyeZ] = FROG_MODEL.eyes[1].at;
  const top = Math.max(...eyes.flatMap((e) => e.surface.points.map((q) => q.clone().applyMatrix4(e.matrix).y)));
  const snout = FROG_MODEL.body.end;
  const [middleY, middleZ] = FROG_MODEL.body.sections[MIDDLE];
  return {
    body,
    eyes,
    fronts,
    legs,
    shoulders: fronts.map((p) => new THREE.Vector3().applyMatrix4(p.matrix)), // where each front loft starts
    bubble: new THREE.Vector3(0, top + BUBBLE_ABOVE, at(eyeX, eyeY, eyeZ).z),
    mouth: at(0, snout[0], snout[1]), // the tip of the snout
    pitchAt: at(0, middleY, middleZ),
  };
}

export class Frog extends Animal {
  readonly body = new THREE.Group();
  readonly hindLegs: THREE.Group[] = []; // left, right: the thigh, shin and foot each, their origins at their joints
  readonly frontLegs: THREE.Group[] = []; // left, right, their origins at the shoulders

  private readonly legs: HindLeg[] = [];
  private readonly pitchAt: THREE.Vector3;
  private readonly sitAt = new THREE.Vector3(); // where its body sits between hops, in its parent's axes
  private sitHeading = 0;
  private lastPhase = 1;
  private readonly joint = new THREE.Vector3(); // reused every frame
  private readonly way = new THREE.Vector3();

  constructor(options: AnimalOptions = {}) {
    const theme = options.theme ?? defaultTheme;
    const frog = build();
    const shape: AnimalShape = { walk: WALK, run: RUN, jump: JUMP, hold: HOLD, bubble: frog.bubble, bubbleScale: BUBBLE_SCALE };
    super(theme, shape);
    this.name = 'frog';
    this.pitchAt = frog.pitchAt;

    // The model's colours from the theme: its green the grass, its dark
    // green (the spots, hands and feet) that far toward the dark, its cream
    // the light toward sand and the glow, its gold the glow toward the trim
    // (orange), and its black the dark.
    const p = palette(theme);
    const colors: Record<FrogColor, THREE.Color> = {
      green: p.grass,
      darkGreen: mix(p.grass, p.dark, 0.6),
      cream: mix(mix(p.light, p.sand, 0.5), p.glow, 0.45),
      gold: mix(p.glow, p.trim, 0.4),
      black: p.dark,
    };
    const m = createMaterials(theme);
    const materials = { coat: m.coat, iris: gloss(colors.gold), pupil: m.eye };

    // A piece's faces into `group`, whose origin is `origin` in the frog's
    // axes: flat, each of the colour the model gives it, a mirrored piece's
    // corners the other way round so they still face out.
    const add = (group: THREE.Group, piece: Piece, origin = new THREE.Vector3()) => {
      const place = new THREE.Matrix4().makeTranslation(-origin.x, -origin.y, -origin.z).multiply(piece.matrix);
      const points = piece.surface.points.map((q) => q.clone().applyMatrix4(place));
      const mirrored = place.determinant() < 0;
      const painted = paintLoft(piece.surface, piece.loft.paint);
      for (const kind of ['coat', 'iris', 'pupil'] as const) {
        const faces = piece.surface.faces.flatMap((face, f) => (MATERIAL[painted[f]] === kind ? [{ corners: (mirrored ? [...face].reverse() : face).map((i) => points[i]), color: colors[painted[f]] }] : []));
        if (faces.length === 0) continue;
        const coat = kind === 'coat' ? (_n: THREE.Vector3, f: number) => faces[f].color : undefined;
        group.add(mesh(polygons(faces.map((face) => face.corners), coat), materials[kind]));
      }
    };

    // The body and head, and the eyes on top; the held figure in its mouth.
    add(this.body, frog.body);
    for (const eye of frog.eyes) add(this.body, eye);
    this.holder.position.copy(frog.mouth);
    this.body.add(this.holder);

    // The legs, a group for each piece, its origin at its joint, placed every
    // frame; the left leg first.
    frog.fronts.forEach((piece, k) => {
      const leg = new THREE.Group();
      leg.position.copy(frog.shoulders[k]);
      add(leg, piece, frog.shoulders[k]);
      this.body.add(leg);
      this.frontLegs.push(leg);
    });
    for (const leg of frog.legs) {
      const segments = leg.pieces.map((piece, i) => {
        const segment = new THREE.Group();
        segment.position.copy(leg.joints[i]);
        add(segment, piece, leg.joints[i]);
        this.body.add(segment);
        return segment;
      });
      this.legs.push({ ...leg, segments });
      this.hindLegs.push(...segments);
    }

    this.rig.add(this.body);
    this.sitAt.copy(this.position);
    this.update(0);
  }

  protected pose(mo: Motion): void {
    // A new hop starts where the last one landed: where the figure is now.
    // Standing still, it sits wherever it is put.
    if (mo.phase < this.lastPhase || mo.moving < 0.01) {
      this.sitAt.copy(this.position);
      this.sitHeading = this.rotation.y;
    }
    this.lastPhase = mo.phase;
    const sit = THREE.MathUtils.lerp(SIT.walk, SIT.run, mo.running);
    const flight = mo.phase < sit ? 0 : (mo.phase - sit) / (1 - sit); // 0..1 through the hop's flight
    const along = THREE.MathUtils.smootherstep(flight, 0, 1); // share of the way from where it sat
    const hopping = mo.moving;

    // Its body in its parent's axes: sitting where the hop began, then
    // carried to where the figure is now; turned the same way. Then into the
    // figure's own axes, for the rig.
    const body = this.sitAt.clone().lerp(this.position, along).sub(this.position);
    body.applyAxisAngle(UP, -this.rotation.y);
    const heading = THREE.MathUtils.lerp(this.sitHeading, this.rotation.y, along);
    const arc = hopping * THREE.MathUtils.lerp(HOP_HEIGHT.walk, HOP_HEIGHT.run, mo.running) * 4 * flight * (1 - flight);
    this.rig.position.set(body.x, mo.height + arc, body.z);
    this.rig.rotation.y = heading - this.rotation.y;
    // Nose up leaving the ground, down landing, in a hop and in a leap,
    // about the middle of its body.
    const tip = THREE.MathUtils.lerp(HOP_PITCH.walk, HOP_PITCH.run, mo.running);
    const pitch = hopping * tip * Math.sin(2 * Math.PI * flight) - HOP_PITCH.run * mo.air * mo.tuck;
    this.body.rotation.x = -pitch;
    this.body.position.copy(this.pitchAt).sub(this.way.copy(this.pitchAt).applyAxisAngle(RIGHT, -pitch));

    // Legs: folded while it sits, flung out straight in the air and folded
    // again for landing; only part of the way in a small hop, and once the
    // body is up.
    const reach = THREE.MathUtils.lerp(HOP_REACH, 1, mo.running);
    const out = Math.max(hopping * reach * Math.sin(Math.PI * flight) ** 2, mo.tuck);
    for (const leg of this.legs) this.fling(leg, out);
    for (const leg of this.frontLegs) leg.quaternion.setFromAxisAngle(RIGHT, -REACH * out);
  }

  // Puts a hind leg's pieces end to end from its hip, each turned `out` of
  // the way from how it sits to flung out: about the upright through its
  // joint, which is carried round by the pieces above it.
  private fling(leg: HindLeg, out: number): void {
    this.joint.copy(leg.joints[0]);
    leg.segments.forEach((segment, i) => {
      segment.position.copy(this.joint);
      segment.quaternion.setFromAxisAngle(UP, leg.fling[i] * out);
      this.joint.add(this.way.subVectors(leg.joints[i + 1], leg.joints[i]).applyQuaternion(segment.quaternion));
    });
  }
}
