import * as THREE from 'three';
import { finGeometry, mesh, tubeGeometry, type FishMaterials } from './parts';

// The body: head and trunk in one smooth, side-flattened shape, deepest a
// little ahead of the middle, with a rounded snout in front and a slim tail
// base (the caudal peduncle) behind. It is dark on the back and light on the
// belly, like most fish, and carries the fins that do not move on their own
// (the dorsal fin along the back and the anal fin under the tail end), the
// eyes and the mouth.
//
// Its origin is the rear end of its center line; the snout is BODY_LENGTH
// along +z. The shape is a function of two numbers: u runs along the body from
// the tail base (0) to the snout (1), and an angle runs around a section from
// the right side (+x) over the back.

export const BODY_LENGTH = 0.235; // tail base to snout
const BODY_DEPTH = 0.1; // back to belly, at the deepest
const BODY_WIDTH = 0.045; // side to side, at the deepest
const PEDUNCLE_DEPTH = 0.032; // the slim tail base
const PEDUNCLE_WIDTH = 0.016;
const DEEPEST = 0.55; // where the body is deepest, as u
const SNOUT = 0.55; // how the head narrows to the snout: 1 comes to a point, lower is blunter
const END = 0.035; // the rounded end that closes the tail base, as u
const BACK_SHARE = 0.56; // of the depth above the center line: the back arches more than the belly
const SECTION_POWER = 2.4; // 2 is an oval section, higher is flatter-sided
export const BODY_TOP = BACK_SHARE * BODY_DEPTH; // the top of the back above the center line
// Where the back colour gives way to the belly colour, as the sine of the
// angle around a section (1 the top of the back, -1 the bottom of the belly).
const BELLY_LINE = { from: -0.3, to: 0.35 };

const ROWS = 72; // along the body
const COLUMNS = 56; // around a section

// Fins along the middle of the back and belly: where they start and end (as
// u), how tall they stand, and how far their free edge sweeps back.
const DORSAL = { from: 0.34, to: 0.76, height: 0.036, sweep: 0.02 };
const ANAL = { from: 0.17, to: 0.36, height: 0.022, sweep: 0.012 };
const FIN_SINK = 0.004; // a fin's base sits this deep in the body
const FIN_STEPS = 24;

const EYE_U = 0.86;
const EYE_Y = 0.012; // above the center line
const EYEBALL_RADIUS = 0.011;
const PUPIL_RADIUS = 0.0065;
const SHINE_RADIUS = 0.002;
const MOUTH_RADII = new THREE.Vector3(0.007, 0.0022, 0.004);
const MOUTH_Y = -0.003;

// The size of a section at u, from the tail base's size to the full size.
function sectionSize(u: number, tailBase: number, full: number): number {
  let size: number;
  if (u >= DEEPEST) {
    const t = (u - DEEPEST) / (1 - DEEPEST);
    size = full * Math.pow(Math.max(0, 1 - t * t), SNOUT);
  } else {
    const t = (DEEPEST - u) / DEEPEST;
    size = tailBase + (full - tailBase) * Math.cos((Math.PI / 2) * t);
  }
  if (u < END) size *= Math.sqrt(Math.max(0, 1 - (1 - u / END) ** 2));
  return size;
}

export function depthAt(u: number): number {
  return sectionSize(u, PEDUNCLE_DEPTH, BODY_DEPTH);
}

export function widthAt(u: number): number {
  return sectionSize(u, PEDUNCLE_WIDTH, BODY_WIDTH);
}

// The top of the back and the bottom of the belly at u, above the center line.
export function backY(u: number): number {
  return BACK_SHARE * depthAt(u);
}

export function bellyY(u: number): number {
  return -(1 - BACK_SHARE) * depthAt(u);
}

// How far the side of the body is from the middle at u and height y.
export function sideX(u: number, y: number): number {
  const reach = y >= 0 ? backY(u) : -bellyY(u);
  if (reach <= 0) return 0;
  const r = Math.min(1, Math.abs(y) / reach);
  return (widthAt(u) / 2) * Math.pow(1 - Math.pow(r, SECTION_POWER), 1 / SECTION_POWER);
}

// A superellipse section, taller above the center line than below it.
function bodyPoint(u: number, angle: number, target: THREE.Vector3): THREE.Vector3 {
  const e = 2 / SECTION_POWER;
  const c = Math.cos(angle);
  const s = Math.sin(angle);
  const x = (widthAt(u) / 2) * Math.sign(c) * Math.pow(Math.abs(c), e);
  const y = (s >= 0 ? backY(u) : -bellyY(u)) * Math.sign(s) * Math.pow(Math.abs(s), e);
  return target.set(x, y, u * BODY_LENGTH);
}

// A fin standing on the back (up = 1) or hanging from the belly (up = -1),
// following the body's outline along its base. It rises steeply at the
// front, is tallest past its middle, and its free edge sweeps back.
function fin(spec: typeof DORSAL, edge: (u: number) => number, up: 1 | -1): THREE.Shape {
  const along = (s: number) => spec.to - s * (spec.to - spec.from); // s: 0 front .. 1 rear
  const outline = new THREE.Shape();
  for (let k = 0; k <= FIN_STEPS; k++) {
    const u = along(k / FIN_STEPS);
    const z = u * BODY_LENGTH;
    const y = edge(u) - up * FIN_SINK;
    if (k === 0) outline.moveTo(z, y);
    else outline.lineTo(z, y);
  }
  for (let k = FIN_STEPS - 1; k > 0; k--) {
    const s = k / FIN_STEPS;
    const u = along(s);
    const h = spec.height * Math.pow(Math.sin(Math.PI * s), 0.6) * (0.7 + 0.3 * s);
    outline.lineTo(u * BODY_LENGTH - spec.sweep * (h / spec.height), edge(u) + up * h);
  }
  outline.closePath();
  return outline;
}

export class Body extends THREE.Group {
  constructor(m: FishMaterials) {
    super();
    this.name = 'body';

    // Rows run tail to snout and columns from the right side over the back,
    // so the triangles' front faces point out of the body.
    const angleOf = (j: number) => (j / COLUMNS) * 2 * Math.PI;
    const geo = tubeGeometry(ROWS, COLUMNS, (i, j) => bodyPoint(i / ROWS, angleOf(j), new THREE.Vector3()));
    const colors: number[] = [];
    const color = new THREE.Color();
    for (let i = 0; i <= ROWS; i++) {
      for (let j = 0; j < COLUMNS; j++) {
        const t = THREE.MathUtils.smoothstep(Math.sin(angleOf(j)), BELLY_LINE.from, BELLY_LINE.to);
        color.copy(m.belly).lerp(m.back, t);
        colors.push(color.r, color.g, color.b);
      }
    }
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    this.add(mesh(geo, m.skin));

    this.add(mesh(finGeometry(fin(DORSAL, backY, 1)), m.fin));
    this.add(mesh(finGeometry(fin(ANAL, bellyY, -1)), m.fin));

    // Each eye is a white eyeball set into the side of the head with a dark
    // pupil and a highlight on it, turned to face out of the body's surface
    // where the head narrows toward the snout.
    const eyeZ = EYE_U * BODY_LENGTH;
    const du = 0.01;
    const slope = (sideX(EYE_U + du, EYE_Y) - sideX(EYE_U - du, EYE_Y)) / (2 * du * BODY_LENGTH);
    const eyeballGeo = new THREE.SphereGeometry(EYEBALL_RADIUS, 32, 16);
    const pupilGeo = new THREE.SphereGeometry(PUPIL_RADIUS, 24, 12);
    const shineGeo = new THREE.SphereGeometry(SHINE_RADIUS, 12, 8);
    for (const side of [-1, 1]) {
      const eye = new THREE.Group();
      eye.position.set(side * sideX(EYE_U, EYE_Y), EYE_Y, eyeZ);
      eye.rotation.y = side * Math.atan(slope);

      const eyeball = mesh(eyeballGeo, m.white);
      eyeball.scale.x = 0.4;
      const pupil = mesh(pupilGeo, m.pupil);
      pupil.scale.x = 0.6;
      pupil.position.x = side * 0.003;
      const shine = mesh(shineGeo, m.white);
      shine.position.set(side * 0.0068, 0.0025, 0.0022);
      eye.add(eyeball, pupil, shine);
      this.add(eye);
    }

    const mouth = mesh(new THREE.SphereGeometry(1, 16, 8), m.mouth);
    mouth.scale.copy(MOUTH_RADII);
    mouth.position.set(0, MOUTH_Y, BODY_LENGTH - 0.002);
    this.add(mouth);
  }
}
