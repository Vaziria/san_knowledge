import * as THREE from 'three';
import type { FishModel, Point, ProfileRow } from './models';
import { hash } from './models';
import { LiveFaces, mesh, sheet, triangles, type FishMaterials } from './parts';

// The body, built as the user's models build theirs (models.ts): rings of
// the model's corners from the tail end to the nose, each an oval square to
// the line through the rings' middles, joined by flat faces, closed to the
// nose tip in front and flat at the tail end; the fins that grow from the
// rings' top and bottom corners (dorsal, adipose, anal, pelvic), each an
// inner strip and its edge; the eyes; and the piranha's underbite jaw and
// teeth. Like the models, it bends by moving its rings' middles sideways and
// building the rings again (bend()), so the fins bend with it; the head in
// front of the model's `stillFrom` stays still.
//
// Its origin is the model's origin, the pivot the fish turns about, and it
// is built in meters, the model's units times its `unit`. The model draws
// every face from both sides; here the body's faces are wound to face out
// and drawn from outside only, and the fins are thin sheets drawn from both
// sides (parts.ts).

const UP = new THREE.Vector3(0, 1, 0);
const SIDE = new THREE.Vector3(); // scratch

// The profile's half-width, half-height and y-offset at z, read between its
// rows by straight lines, as the models read it.
export function sizeAt(profile: readonly ProfileRow[], z: number): [rx: number, ry: number, yOffset: number] {
  for (let i = 0; i < profile.length - 1; i++) {
    const a = profile[i];
    const b = profile[i + 1];
    if (z <= b[0]) {
      const f = (z - a[0]) / (b[0] - a[0]);
      const ay = a[3] ?? 0;
      const by = b[3] ?? 0;
      return [a[1] + (b[1] - a[1]) * f, a[2] + (b[2] - a[2]) * f, ay + (by - ay) * f];
    }
  }
  const last = profile[profile.length - 1];
  return [last[1], last[2], last[3] ?? 0];
}

export class Body extends THREE.Group {
  // The middle of the tail end, where the tail fin grows, and the way the
  // body runs there, toward the head: in meters, as the body is bent now.
  readonly tailRoot = new THREE.Vector3();
  readonly tailAxis = new THREE.Vector3();
  // How far the top of the back is above the origin, at rest, in meters.
  readonly top: number;

  private readonly model: FishModel<string>;
  private readonly zTail: number;
  private readonly ringZ: number[];
  private readonly sizes: [number, number, number][];
  private readonly centers: THREE.Vector3[];
  private readonly ups: THREE.Vector3[];
  private readonly ringPoints: THREE.Vector3[][];
  // Each ring's side and up at rest, as the model builds them.
  private readonly restSides: THREE.Vector3[];
  private readonly restUps: THREE.Vector3[];
  // For each fin, the rings it grows from, and on each the point where its
  // inner strip ends and its tip.
  private readonly finRings: { rings: number[]; corner: number; dir: number; height: (z: number) => number; inner: THREE.Vector3[]; outer: THREE.Vector3[] }[];
  private readonly skin: LiveFaces;
  private readonly fins: LiveFaces;

  constructor(m: FishMaterials, model: FishModel<string>, colors: Record<string, THREE.Color>) {
    super();
    this.name = 'body';
    this.model = model;
    const { profile, rings: RINGS, sides: SIDES, unit } = model;
    const zTail = (this.zTail = profile[0][0]);
    const zNose = profile[profile.length - 1][0];
    this.ringZ = Array.from({ length: RINGS }, (_, i) => zTail + (zNose - zTail) * (i / (RINGS - 1)));
    this.sizes = this.ringZ.map((z) => sizeAt(profile, z));
    this.centers = Array.from({ length: RINGS }, () => new THREE.Vector3());
    this.ups = Array.from({ length: RINGS }, () => new THREE.Vector3());
    this.ringPoints = Array.from({ length: RINGS }, () => Array.from({ length: SIDES }, () => new THREE.Vector3()));
    const tip = new THREE.Vector3(0, model.noseTip[1], model.noseTip[0]);

    // Each ring at rest square to the line through the middles either side
    // of it, its first corner on top, as the models build them.
    const tangent = new THREE.Vector3();
    this.ringZ.forEach((z, i) => this.centers[i].set(0, this.sizes[i][2], z));
    this.restSides = [];
    this.restUps = [];
    this.centers.forEach((_, i) => {
      tangent.subVectors(this.centers[Math.min(i + 1, RINGS - 1)], this.centers[Math.max(i - 1, 0)]).normalize();
      const side = new THREE.Vector3().crossVectors(tangent, UP).normalize();
      this.restSides.push(side);
      this.restUps.push(new THREE.Vector3().crossVectors(side, tangent).normalize());
    });

    // The fins: the rings each grows from, from its top or bottom corner.
    this.finRings = model.fins.map((f) => {
      const rings = this.ringZ.map((_, i) => i).filter((i) => this.ringZ[i] >= f.from && this.ringZ[i] <= f.to);
      return {
        rings,
        corner: f.side === 'top' ? 0 : SIDES / 2,
        dir: f.side === 'top' ? 1 : -1,
        height: f.height,
        inner: rings.map(() => new THREE.Vector3()),
        outer: rings.map(() => new THREE.Vector3()),
      };
    });
    this.place(0, 0, 0);

    // The body's faces, wound to face out (the model's face inward), each of
    // the model's colour for its band and height round the ring; then the
    // nose and the tail end.
    const faces: [THREE.Vector3, THREE.Vector3, THREE.Vector3][] = [];
    const faceColors: THREE.Color[] = [];
    const ring = this.ringPoints;
    for (let i = 0; i < RINGS - 1; i++) {
      const z = (this.ringZ[i] + this.ringZ[i + 1]) / 2;
      for (let k = 0; k < SIDES; k++) {
        const height = Math.sin(Math.PI / 2 + (2 * Math.PI * (k + 0.5)) / SIDES); // +1 top ... -1 belly
        const color = colors[model.colorAt(z, height, hash(i * 13.7 + k * 3.1))];
        const [a, b] = [ring[i][k], ring[i][(k + 1) % SIDES]];
        const [c, d] = [ring[i + 1][k], ring[i + 1][(k + 1) % SIDES]];
        faces.push([a, d, b], [a, c, d]);
        faceColors.push(color, color);
      }
    }
    const last = RINGS - 1;
    for (let k = 0; k < SIDES; k++) {
      faces.push([ring[last][(k + 1) % SIDES], ring[last][k], tip]);
      faces.push([ring[0][k], ring[0][(k + 1) % SIDES], this.centers[0]]);
      faceColors.push(colors[model.cap], colors[model.cap]);
    }
    this.skin = new LiveFaces(faces, faceColors, unit);
    this.add(mesh(this.skin.geometry, m.skin));

    // The fins: from the body to 80% of the fin's height, its inner colour,
    // and from there to its tip, its edge colour; a sheet, seen from both
    // sides, wound as the model winds it.
    const finFaces: [THREE.Vector3, THREE.Vector3, THREE.Vector3][] = [];
    const finColors: THREE.Color[] = [];
    this.finRings.forEach((f, n) => {
      const { inner, edge } = model.fins[n];
      for (let j = 0; j < f.rings.length - 1; j++) {
        const [b0, b1] = [ring[f.rings[j]][f.corner], ring[f.rings[j + 1]][f.corner]];
        const [A, B, C, D] = [f.inner[j], f.inner[j + 1], f.outer[j], f.outer[j + 1]];
        finFaces.push([b0, b1, B], [b0, B, A], [A, B, D], [A, D, C]);
        finColors.push(colors[inner], colors[inner], colors[edge], colors[edge]);
      }
    });
    this.fins = new LiveFaces(finFaces, finColors, unit);
    this.add(...sheet(this.fins.geometry, m));

    // Bounds that hold any pose: at rest, and more than it ever bends (twice
    // the model's sway, and as far again as its swaying part is long, where a
    // turn bends it a quarter of that).
    const reach = (model.swim.amplitude * 2 + (model.swim.stillFrom - zTail)) * unit;
    for (const geo of [this.skin.geometry, this.fins.geometry]) {
      geo.computeBoundingSphere();
      geo.boundingSphere!.radius += reach;
    }
    this.top = Math.max(...this.ringPoints.map((r) => r[0].y)) * unit;

    this.addEyes(m, colors);
    if (model.mouth) this.addMouth(m, colors);
  }

  // Bends the body sideways, as the model sways it: from `stillFrom` back,
  // more and more toward the tail (the square of the share of the way), a
  // wave of the model's wavelength, `share` of the model's amplitude, at
  // `phase` (at the tail end its offset is -sin(phase) of that), plus `turn`
  // (the model's units at the tail end, > 0 toward +x). Then builds the rings
  // round the bent line again, and the fins on them.
  bend(share: number, phase: number, turn: number): void {
    this.place(share, phase, turn);
    this.skin.update();
    this.fins.update();
  }

  private place(share: number, phase: number, turn: number): void {
    const { model, zTail, ringZ, sizes, centers, ups, ringPoints } = this;
    const { amplitude, wavelength, stillFrom } = model.swim;
    const RINGS = ringZ.length;
    const SIDES = model.sides;
    for (let i = 0; i < RINGS; i++) {
      const z = ringZ[i];
      const amount = Math.max(0, (stillFrom - z) / (stillFrom - zTail));
      centers[i].set(amount * amount * (amplitude * share * Math.sin(wavelength * (z - zTail) - phase) + turn), sizes[i][2], z);
    }
    // Each ring is its rest ring turned about the upright by the way the bent
    // line through the middles either side of it heads: its side is then the
    // model's (level, square to the line), and its up keeps its height, so
    // bending sideways moves nothing up or down. (The model builds the ring
    // square to the bent line, which where the body slopes up or down, as the
    // piranha's does, sinks its lowest point 0.03 mm as it sways.)
    const side = SIDE;
    for (let i = 0; i < RINGS; i++) {
      const a = centers[Math.max(i - 1, 0)];
      const b = centers[Math.min(i + 1, RINGS - 1)];
      const heading = Math.atan2(b.x - a.x, b.z - a.z);
      const [cos, sin] = [Math.cos(heading), Math.sin(heading)];
      const [s, u] = [this.restSides[i], this.restUps[i]];
      side.set(s.x * cos + s.z * sin, s.y, s.z * cos - s.x * sin);
      ups[i].set(u.x * cos + u.z * sin, u.y, u.z * cos - u.x * sin);
      const [rx, ry] = sizes[i];
      for (let k = 0; k < SIDES; k++) {
        const angle = Math.PI / 2 + (2 * Math.PI * k) / SIDES;
        ringPoints[i][k]
          .copy(centers[i])
          .addScaledVector(side, Math.cos(angle) * rx)
          .addScaledVector(ups[i], Math.sin(angle) * ry);
      }
    }
    for (const f of this.finRings) {
      f.rings.forEach((i, j) => {
        const h = f.height(ringZ[i]) * f.dir;
        f.inner[j].copy(ringPoints[i][f.corner]).addScaledVector(ups[i], h * 0.8);
        f.outer[j].copy(ringPoints[i][f.corner]).addScaledVector(ups[i], h);
      });
    }
    this.tailRoot.copy(centers[0]).multiplyScalar(model.unit);
    this.tailAxis.subVectors(centers[1], centers[0]).normalize();
  }

  // Each eye, a coloured ring (the iris) round a black pupil that stands a
  // little out of it, facing out of the side of the head, the left the right
  // one's mirror image. It is drawn from the front only: its back lies
  // against the body.
  private addEyes(m: FishMaterials, colors: Record<string, THREE.Color>): void {
    const { eye, unit, profile } = this.model;
    const N = 6;
    const ring = (r: number, x: number) =>
      Array.from({ length: N }, (_, k): Point => {
        const a = (2 * Math.PI * k) / N;
        return [x, Math.cos(a) * r, Math.sin(a) * r];
      });
    const outer = ring(eye.radius, 0);
    const inner = ring(eye.pupil, eye.pupilX);
    const points: Record<string, Point> = { centre: [eye.centreX, 0, 0] };
    const faces: [string, string, string, string][] = [];
    for (let k = 0; k < N; k++) {
      const k2 = (k + 1) % N;
      points[`o${k}`] = outer[k];
      points[`i${k}`] = inner[k];
      faces.push([`o${k}`, `o${k2}`, `i${k2}`, eye.iris], [`o${k}`, `i${k2}`, `i${k}`, eye.iris], ['centre', `i${k}`, `i${k2}`, eye.black]);
    }
    const geo = triangles(points, faces, colors, unit);
    // Where the factory puts it: on the ring's oval there, at 0.97.
    const [erx, ery, eoff] = sizeAt(profile, eye.at.z);
    const x = eye.at.x ?? erx * Math.sqrt(Math.max(0, 1 - ((eye.at.y - eoff) / ery) ** 2)) * 0.97;
    for (const side of [-1, 1]) {
      const e = mesh(geo, m.eye);
      e.name = side < 0 ? 'left eye' : 'right eye';
      e.position.set(side * x, eye.at.y, eye.at.z).multiplyScalar(unit);
      e.scale.x = side;
      this.add(e);
    }
  }

  // The piranha's underbite jaw, closed but for its back inside the head,
  // its faces turned to face out of it, and its two rows of teeth, each a
  // thin sheet.
  private addMouth(m: FishMaterials, colors: Record<string, THREE.Color>): void {
    const { mouth, unit } = this.model;
    if (!mouth) return;
    const middle = new THREE.Vector3();
    const names = Object.keys(mouth.points);
    for (const name of names) middle.add(new THREE.Vector3(...mouth.points[name]));
    middle.divideScalar(names.length);
    const p = (name: string) => new THREE.Vector3(...mouth.points[name]);
    const faces = mouth.faces.map(([a, b, c, color]) => {
      const normal = p(b).sub(p(a)).cross(p(c).sub(p(a)));
      const out = p(a).add(p(b)).add(p(c)).divideScalar(3).sub(middle);
      return (normal.dot(out) < 0 ? [a, c, b, color] : [a, b, c, color]) as [string, string, string, string];
    });
    const jaw = mesh(triangles(mouth.points, faces, colors, unit), m.skin);
    jaw.name = 'jaw';
    this.add(jaw);

    const points: Record<string, Point> = {};
    const teeth = mouth.teeth.map((corners, t) => {
      corners.forEach((corner, k) => (points[`${t}.${k}`] = corner));
      return [`${t}.0`, `${t}.1`, `${t}.2`, mouth.tooth] as const;
    });
    const [front, back] = sheet(triangles(points, teeth, colors, unit), m);
    front.name = back.name = 'teeth';
    this.add(front, back);
  }
}
