import * as THREE from 'three';
import { keepSharp } from '../../effects/kuwahara';
import { blob, capsule, mesh, plain, polygons, solid, type AnimalMaterials, type Shade } from './parts';

// A four-legged animal's head, looking along +z: a rounded skull, a snout
// narrowing to the nose, glossy eyes with a catchlight, and ears of one of
// three kinds (pointed, round, or broad and set out to the sides), with
// antlers for a deer. Or a head modelled by hand, corner by corner and face
// by face, low poly like folded paper (a HeadModel: the wolf's, bear's,
// cat's, deer's, fox's and lion's, from the user's models), with what the
// model adds to it (a lion's mane, a cat's whiskers, a deer's antlers). The
// origin is where the neck joins it, at the back of the skull; the animal
// turns the head about it. `mouth` is under the snout, where a held figure
// goes.

export interface HeadShape {
  length: number; // m, back of the skull to the tip of the nose
  width: number; // m, the skull
  height: number; // m, the skull
  snout: number; // the snout's length, as a share of the head's
  snoutWidth: number; // its width where it leaves the skull, as a share of the skull's
  snoutHeight: number; // and its height
  snoutTaper?: number; // its width and height at the nose, as shares of those where it leaves the skull (0.55 when left out; a bear's blunt muzzle is fuller)
  ears: { kind: 'pointed' | 'round' | 'broad'; height: number; width: number; out: number; back: number };
  eye: number; // m, an eye's radius
  eyeAt?: { x: number; y: number; z: number }; // where the eyes sit, as shares of the skull's width and height and the head's length (EYE when left out)
  pitch: number; // radians it tips down at rest
  antlers?: { height: number; spread: number; radius: number };
}

// A head modelled by hand, in the head's own axes. Its points are its
// corners on the middle (x = 0) and on its +x side, which is mirrored to -x.
// Each face gives its colour, then its corners anticlockwise seen from
// outside, and must be convex: `side` faces are on the +x side, each mirrored
// to -x; `middle` faces lie across the middle and name their corners on the
// -x side with a leading '-'. Together they close the surface. What the model
// adds to the head (`extras`) is built with it and turns with it.
export interface HeadModel<Name extends string = string> {
  unit: number; // m per unit of the points: 0.01 for centimeters
  points: Record<Name, Point>;
  origin?: Point; // where the neck joins it, which it turns about ([0, 0, 0] when left out)
  side: readonly ModelFace<Name>[];
  middle: readonly ModelFace<Name>[];
  mouth: Point; // where a held figure goes
  pitch: number; // radians it tips down at rest
  extras?: readonly Extra[];
  top?: number; // how high what it adds reaches (a deer's antlers, a lion's mane), in its units, when that is above its points
}

export type Point = readonly [x: number, y: number, z: number];

// A face's colour is named as the user's model names it ('brown', 'tan', a
// cat's 'stripe'), one of the animal's colours: flat and of one colour, on
// the coat. An 'eye' face is glossy, in the animal's eye colour.
export type ModelFace<Name extends string = string> = readonly [color: string, ...(Name | `-${Name}`)[]];

// The animal's colours, by name: the coat's and those its model names.
export interface HeadColors {
  coat: THREE.Color;
  muzzle: THREE.Color; // the underside of the snout and the jaw, the inside of the ears
  horn: THREE.Color; // antlers
  [name: string]: THREE.Color;
}

// Something a model adds to its head (a mane, whiskers, antlers), built in
// the head's resting axes in meters: `point` places a point given in the
// head's units, as the head's own corners are placed, and `unit` is meters
// per unit.
export interface Dress {
  m: AnimalMaterials;
  colors: HeadColors;
  unit: number;
  point(p: Point): THREE.Vector3;
}
export type Extra = (dress: Dress) => THREE.Object3D;

// How high the head reaches above its origin at rest (the tips of its ears
// or antlers) and how long it is, for where the speech bubble goes.
export function headSize(shape: HeadShape | HeadModel): { top: number; length: number } {
  if (!('points' in shape)) return { top: Math.max(shape.height * 0.6 + shape.ears.height, shape.antlers?.height ?? 0), length: shape.length };
  const [, oy, oz] = shape.origin ?? [0, 0, 0];
  const points = Object.values(shape.points).map(([, y, z]) => [y - oy, z - oz]);
  if (shape.top !== undefined) points.push([shape.top - oy, -oz]);
  const [cos, sin] = [Math.cos(shape.pitch), Math.sin(shape.pitch)];
  return {
    top: shape.unit * Math.max(...points.map(([y, z]) => y * cos - z * sin)),
    length: shape.unit * Math.max(...points.map(([, z]) => z)),
  };
}

const EAR_ROOT = { x: 0.3, y: 0.42, z: 0.22 }; // where the ears sit, as shares of the skull's width, height and the head's length
const EYE = { x: 0.3, y: 0.2, z: 0.5 };
const SHINE = 0.35; // the catchlight's radius, as a share of the eye's

export class Head extends THREE.Group {
  readonly mouth = new THREE.Group();

  constructor(m: AnimalMaterials, shape: HeadShape | HeadModel, colors: HeadColors) {
    super();
    this.name = 'head';
    // The head's resting tilt, so its own rotation is 0 at rest.
    const rest = new THREE.Group();
    rest.rotation.x = shape.pitch;
    this.add(rest);
    if ('points' in shape) this.modelled(rest, m, shape, colors);
    else this.rounded(rest, m, shape, colors);
    rest.add(this.mouth);
  }

  // A head modelled by hand: its faces, the -x side's mirrored from the +x
  // side's (and wound the other way round, so they face out too), in one
  // flat mesh for the coat, coloured face by face, and one for the eyes;
  // then what the model adds to it.
  private modelled(rest: THREE.Group, m: AnimalMaterials, model: HeadModel, colors: HeadColors) {
    const origin = new THREE.Vector3(...(model.origin ?? [0, 0, 0]));
    const at = ([x, y, z]: Point) => new THREE.Vector3(x, y, z).sub(origin).multiplyScalar(model.unit);
    const corner = (name: string) => {
      const [x, y, z] = model.points[name.replace(/^-/, '')];
      return at([name.startsWith('-') ? -x : x, y, z]);
    };
    const mirror = (name: string) => (name.startsWith('-') ? name.slice(1) : `-${name}`);
    const faces: ModelFace[] = [...model.middle, ...model.side, ...model.side.map(([color, ...corners]): ModelFace => [color, ...corners.map(mirror).reverse()])];
    const corners = (face: ModelFace) => face.slice(1).map(corner);
    const colorOf = (name: string) => {
      const color = colors[name];
      if (!color) throw new Error(`a head face names the colour "${name}", which the animal hasn't`);
      return color;
    };

    const coat = faces.filter(([color]) => color !== 'eye');
    rest.add(mesh(polygons(coat.map(corners), (_n, i) => colorOf(coat[i][0])), m.coat));
    rest.add(mesh(polygons(faces.filter(([color]) => color === 'eye').map(corners)), m.eye));
    this.mouth.position.copy(at(model.mouth));
    for (const extra of model.extras ?? []) rest.add(extra({ m, colors, unit: model.unit, point: at }));
  }

  private rounded(rest: THREE.Group, m: AnimalMaterials, shape: HeadShape, colors: HeadColors) {
    const { length, width, height } = shape;
    const c = new THREE.Color();
    // The coat, turning to the muzzle colour under the snout and jaw.
    const face: Shade = (_u, out) => c.copy(colors.coat).lerp(colors.muzzle, THREE.MathUtils.smoothstep(-out.y, 0.1, 0.55));

    // The skull.
    const skullLength = length * (1 - shape.snout) * 1.15;
    rest.add(mesh(blob(new THREE.Vector3(0, height * 0.12, skullLength * 0.5), new THREE.Vector3(width / 2, height / 2, skullLength / 2), face, {}, undefined, m.look), m.coat));

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
            const taper = THREE.MathUtils.lerp(1, shape.snoutTaper ?? 0.55, u) * capsule(3)(u);
            return {
              width: ((shape.snoutWidth * width) / 2) * taper,
              top: ((shape.snoutHeight * height) / 2) * taper * 0.9,
              bottom: ((shape.snoutHeight * height) / 2) * taper,
            };
          },
          face,
          10,
          14,
          m.look,
        ),
        m.coat,
      ),
    );
    // The nose, dark, on the end of the snout.
    const nose = shape.snoutWidth * width * 0.2;
    rest.add(mesh(blob(new THREE.Vector3(0, to.y + nose * 0.4, length - nose * 0.4), new THREE.Vector3(nose, nose * 0.75, nose * 0.8), plain(colors.coat), {}, undefined, m.look), m.dark));

    // Eyes, with a catchlight up and in front. Faceted, an eye is a few
    // flat faces like the rest, and a round catchlight would be out of
    // place, so it has none.
    for (const side of [-1, 1]) {
      const place = shape.eyeAt ?? EYE;
      const at = new THREE.Vector3(side * width * place.x, height * place.y, length * place.z);
      if (m.look === 'faceted') {
        rest.add(mesh(blob(at, new THREE.Vector3(shape.eye, shape.eye, shape.eye * 0.8), plain(colors.coat), {}, undefined, m.look), m.eye));
        continue;
      }
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
        geo = blob(new THREE.Vector3(0, ears.height * 0.35, 0), new THREE.Vector3(ears.width / 2, ears.height / 2, ears.width * 0.22), ear, {}, undefined, m.look);
      } else {
        const taper = ears.kind === 'pointed' ? { top: 0.9 } : { top: 0.45, bottom: 0.5 };
        geo = blob(new THREE.Vector3(0, ears.height * 0.45, 0), new THREE.Vector3(ears.width / 2, ears.height / 2, ears.width * 0.16), ear, taper, undefined, m.look);
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
          }, horn, 10, 8, m.look),
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
  }
}

// Whiskers, as the user's models draw them (a cat's, a lion's): straight
// lines from beside the nose out to their tips, given in the head's units on
// its right side and mirrored to the left, in the colour named. Like the
// models' LineSegments they are a pixel wide however far off they are, and
// they are kept out of the painterly filter (keepSharp), which would wipe
// out anything so thin. Lines take no light, so they don't dim at night.
export function whiskers(lines: readonly (readonly [from: Point, to: Point])[], color: string): Extra {
  return ({ colors, point }) => {
    const positions: number[] = [];
    for (const side of [1, -1]) {
      for (const ends of lines) {
        for (const [x, y, z] of ends) positions.push(...point([side * x, y, z]).toArray());
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    const material = new THREE.LineBasicMaterial({ color: colors[color] });
    keepSharp(material);
    const segments = new THREE.LineSegments(geometry, material);
    segments.name = 'whiskers';
    return segments;
  };
}
