import * as THREE from 'three';
import { blob, capsule, mesh, plain, solid, type AnimalMaterials, type Shade } from './parts';

// A four-legged animal's head, looking along +z: a rounded skull, a snout
// narrowing to the nose, glossy eyes with a catchlight, and ears of one of
// three kinds (pointed, round, or broad and set out to the sides), with
// antlers for a deer. The origin is where the neck joins it, at the back of
// the skull; the animal turns the head about it. `mouth` is under the snout,
// where a held figure goes.

export interface HeadShape {
  length: number; // m, back of the skull to the tip of the nose
  width: number; // m, the skull
  height: number; // m, the skull
  snout: number; // the snout's length, as a share of the head's
  snoutWidth: number; // its width where it leaves the skull, as a share of the skull's
  snoutHeight: number; // and its height
  ears: { kind: 'pointed' | 'round' | 'broad'; height: number; width: number; out: number; back: number };
  eye: number; // m, an eye's radius
  pitch: number; // radians it tips down at rest
  antlers?: { height: number; spread: number; radius: number };
}

export interface HeadColors {
  coat: THREE.Color;
  muzzle: THREE.Color; // the underside of the snout and the jaw, the inside of the ears
  horn: THREE.Color; // antlers
}

const EAR_ROOT = { x: 0.3, y: 0.42, z: 0.22 }; // where the ears sit, as shares of the skull's width, height and the head's length
const EYE = { x: 0.3, y: 0.2, z: 0.5 };
const SHINE = 0.35; // the catchlight's radius, as a share of the eye's

export class Head extends THREE.Group {
  readonly mouth = new THREE.Group();

  constructor(m: AnimalMaterials, shape: HeadShape, colors: HeadColors) {
    super();
    this.name = 'head';
    // The head's resting tilt, so its own rotation is 0 at rest.
    const rest = new THREE.Group();
    rest.rotation.x = shape.pitch;
    this.add(rest);
    const { length, width, height } = shape;
    const c = new THREE.Color();
    // The coat, turning to the muzzle colour under the snout and jaw.
    const face: Shade = (_u, out) => c.copy(colors.coat).lerp(colors.muzzle, THREE.MathUtils.smoothstep(-out.y, 0.1, 0.55));

    // The skull.
    const skullLength = length * (1 - shape.snout) * 1.15;
    rest.add(mesh(blob(new THREE.Vector3(0, height * 0.12, skullLength * 0.5), new THREE.Vector3(width / 2, height / 2, skullLength / 2), face), m.coat));

    // The snout: from inside the skull out to the nose, narrowing and a
    // little lower than the skull's middle.
    const snoutLength = length * shape.snout;
    const from = new THREE.Vector3(0, -height * 0.02, length - snoutLength * 1.35);
    const to = new THREE.Vector3(0, -height * 0.12, length);
    rest.add(
      mesh(
        solid(
          [from, from.clone().lerp(to, 0.5), to],
          (u) => {
            const taper = THREE.MathUtils.lerp(1, 0.55, u) * capsule(3)(u);
            return {
              width: ((shape.snoutWidth * width) / 2) * taper,
              top: ((shape.snoutHeight * height) / 2) * taper * 0.9,
              bottom: ((shape.snoutHeight * height) / 2) * taper,
            };
          },
          face,
          10,
          14,
        ),
        m.coat,
      ),
    );
    // The nose, dark, on the end of the snout.
    const nose = shape.snoutWidth * width * 0.2;
    rest.add(mesh(blob(new THREE.Vector3(0, to.y + nose * 0.4, length - nose * 0.4), new THREE.Vector3(nose, nose * 0.75, nose * 0.8), plain(colors.coat)), m.dark));

    // Eyes, with a catchlight up and in front.
    for (const side of [-1, 1]) {
      const at = new THREE.Vector3(side * width * EYE.x, height * EYE.y, length * EYE.z);
      const eye = mesh(new THREE.SphereGeometry(shape.eye, 16, 12), m.eye);
      eye.position.copy(at);
      const shine = mesh(new THREE.SphereGeometry(shape.eye * SHINE, 8, 6), m.shine);
      shine.position.copy(at).add(new THREE.Vector3(side * -0.2, 0.45, 0.75).multiplyScalar(shape.eye));
      rest.add(eye, shine);
    }

    // Ears, their fronts in the muzzle colour.
    const ear: Shade = (_u, out) => c.copy(colors.coat).lerp(colors.muzzle, THREE.MathUtils.smoothstep(out.z, 0.2, 0.7));
    const { ears } = shape;
    for (const side of [-1, 1]) {
      const root = new THREE.Vector3(side * width * EAR_ROOT.x, height * EAR_ROOT.y, length * EAR_ROOT.z);
      let geo: THREE.BufferGeometry;
      if (ears.kind === 'round') {
        geo = blob(new THREE.Vector3(0, ears.height * 0.35, 0), new THREE.Vector3(ears.width / 2, ears.height / 2, ears.width * 0.22), ear);
      } else {
        const taper = ears.kind === 'pointed' ? { top: 0.9 } : { top: 0.45, bottom: 0.5 };
        geo = blob(new THREE.Vector3(0, ears.height * 0.45, 0), new THREE.Vector3(ears.width / 2, ears.height / 2, ears.width * 0.16), ear, taper);
      }
      const e = mesh(geo, m.coat);
      e.position.copy(root);
      e.rotation.set(-ears.back, 0, -side * ears.out);
      rest.add(e);
    }

    // Antlers: a beam each side sweeping up, out and forward, with tines
    // rising from it.
    if (shape.antlers) {
      const { height: h, spread, radius } = shape.antlers;
      const horn = plain(colors.horn);
      const tube = (points: THREE.Vector3[], r: number) =>
        mesh(
          solid(points, (u) => {
            const w = r * THREE.MathUtils.lerp(1, 0.45, u) * capsule(6)(u);
            return { width: w, top: w, bottom: w };
          }, horn, 10, 8),
          m.coat,
        );
      for (const side of [-1, 1]) {
        const base = new THREE.Vector3(side * width * 0.22, height * 0.45, length * 0.3);
        const beam = [
          base,
          base.clone().add(new THREE.Vector3(side * spread * 0.45, h * 0.35, -h * 0.12)),
          base.clone().add(new THREE.Vector3(side * spread * 0.9, h * 0.7, -h * 0.02)),
          base.clone().add(new THREE.Vector3(side * spread, h, h * 0.25)),
        ];
        rest.add(tube(beam, radius));
        for (const [k, rise] of [[1, 0.4], [2, 0.35]] as const) {
          const from = beam[k];
          rest.add(tube([from, from.clone().add(new THREE.Vector3(side * spread * 0.05, h * rise * 0.6, h * 0.08)), from.clone().add(new THREE.Vector3(side * spread * 0.1, h * rise, h * 0.05))], radius * 0.7));
        }
      }
    }

    this.mouth.position.set(0, to.y - (shape.snoutHeight * height) / 2, length * 0.82);
    rest.add(this.mouth);
  }
}
