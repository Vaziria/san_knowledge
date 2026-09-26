import * as THREE from 'three';
import { gloss, surface, type Theme } from '../../theme';
import { palette, polygons } from '../animals/parts';
import type { Point, Triangle } from './models';

// Helpers shared by the fish's parts.

// Materials for every part, built once per fish from the theme. Every face
// is flat and of one colour, its vertices' colour (models.ts maps each
// model's colours onto the theme), so the materials are white and take the
// colours from the vertices. Closed surfaces (the body, the piranha's jaw)
// are drawn from outside only. A thin sheet (a fin, the tail fin, a tooth)
// is drawn as two meshes of the same geometry, its front (`sheet`) and its
// back (`sheetBack`), and only the front casts a shadow (rules.md, Building
// shapes 9), from whichever of its sides faces the sun: a sheet's front turned
// to the sun would otherwise cast none, since a front-sided material casts
// from its back faces. Eyes are glossy, as on any material.
export function createMaterials(theme: Theme) {
  const skin = surface(theme, 0xffffff);
  skin.vertexColors = true;
  const sheet = skin.clone();
  sheet.shadowSide = THREE.DoubleSide;
  const sheetBack = skin.clone();
  sheetBack.side = THREE.BackSide;
  const eye = gloss(0xffffff);
  eye.vertexColors = true;
  return { skin, sheet, sheetBack, eye, palette: palette(theme) };
}

export type FishMaterials = ReturnType<typeof createMaterials>;

// -1 for the fish's left (-x), 1 for its right (+x).
export type Side = -1 | 1;

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// A thin sheet: its front, which casts the shadow, and its back.
export function sheet(geo: THREE.BufferGeometry, m: FishMaterials): [front: THREE.Mesh, back: THREE.Mesh] {
  const back = mesh(geo, m.sheetBack);
  back.castShadow = false;
  return [mesh(geo, m.sheet), back];
}

// Flat triangles from a model's named points (in its units) and triangles,
// each of its colour, in meters: `unit` m a unit. A triangle's corners go
// anticlockwise seen from its front.
export function triangles<C extends string>(points: Readonly<Record<string, Point>>, faces: readonly Triangle<C>[], colors: Record<C, THREE.Color>, unit: number): THREE.BufferGeometry {
  const at = (name: string) => new THREE.Vector3(...points[name]).multiplyScalar(unit);
  return polygons(
    faces.map(([a, b, c]) => [at(a), at(b), at(c)]),
    (_normal, face) => colors[faces[face][3]],
  );
}

// A surface whose corners move (the body, bent every frame): flat triangles
// between points that are moved in place, in a model's units, each triangle
// of one colour, drawn `scale` m a unit. update() writes where the corners
// are now, and each face's own normal. The geometry's bounding box is dropped
// then, so a Box3 of the fish measures the pose of the moment; its bounding
// sphere is set by its owner to hold any pose.
export class LiveFaces {
  readonly geometry = new THREE.BufferGeometry();
  private readonly corners: THREE.Vector3[];
  private readonly scale: number;

  constructor(faces: readonly (readonly [THREE.Vector3, THREE.Vector3, THREE.Vector3])[], colors: readonly THREE.Color[], scale: number) {
    this.corners = faces.flat();
    this.scale = scale;
    const count = this.corners.length;
    const color = new Float32Array(count * 3);
    colors.forEach((c, f) => {
      for (let k = 0; k < 3; k++) color.set([c.r, c.g, c.b], (3 * f + k) * 3);
    });
    this.geometry.setAttribute('position', new THREE.BufferAttribute(new Float32Array(count * 3), 3));
    this.geometry.setAttribute('normal', new THREE.BufferAttribute(new Float32Array(count * 3), 3));
    this.geometry.setAttribute('color', new THREE.BufferAttribute(color, 3));
    this.update();
  }

  update(): void {
    const position = this.geometry.getAttribute('position') as THREE.BufferAttribute;
    const normal = this.geometry.getAttribute('normal') as THREE.BufferAttribute;
    const p = position.array as Float32Array;
    const n = normal.array as Float32Array;
    const { corners, scale } = this;
    for (let i = 0; i < corners.length; i += 3) {
      const a = corners[i];
      const b = corners[i + 1];
      const c = corners[i + 2];
      // (b - a) x (c - a), the face's normal; a face with no area (a fin's
      // strip where it stands out 0) keeps the one it had.
      const ux = b.x - a.x, uy = b.y - a.y, uz = b.z - a.z; // prettier-ignore
      const vx = c.x - a.x, vy = c.y - a.y, vz = c.z - a.z; // prettier-ignore
      let nx = uy * vz - uz * vy;
      let ny = uz * vx - ux * vz;
      let nz = ux * vy - uy * vx;
      const length = Math.hypot(nx, ny, nz);
      if (length > 1e-12) {
        nx /= length;
        ny /= length;
        nz /= length;
      } else if (n[i * 3] || n[i * 3 + 1] || n[i * 3 + 2]) {
        [nx, ny, nz] = [n[i * 3], n[i * 3 + 1], n[i * 3 + 2]];
      } else {
        [nx, ny, nz] = [0, 1, 0];
      }
      for (let k = 0; k < 3; k++) {
        const v = corners[i + k];
        const o = (i + k) * 3;
        p[o] = v.x * scale;
        p[o + 1] = v.y * scale;
        p[o + 2] = v.z * scale;
        n[o] = nx;
        n[o + 1] = ny;
        n[o + 2] = nz;
      }
    }
    position.needsUpdate = true;
    normal.needsUpdate = true;
    this.geometry.boundingBox = null;
  }
}
