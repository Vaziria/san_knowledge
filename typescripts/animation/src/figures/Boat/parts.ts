import * as THREE from 'three';
import { surface, wood, type Theme } from '../../theme';

// Helpers shared by the boat's parts.

// Materials for every part, built once per boat from the theme. The boat is
// built of wood: planked inside and out, with a wooden bench, rudder and
// tiller, all in the theme's wood colour with the wood-grain texture. The
// hull is a thin shell: its outside and inside are the same surface, drawn
// from each side. The gunwale rim is painted in the trim colour.
export function createMaterials(theme: Theme) {
  const c = theme.colors;
  const inside = wood(theme);
  inside.side = THREE.BackSide;
  return {
    hull: wood(theme), // outside of the hull and transom
    inside, // inside of the hull and transom
    timber: wood(theme), // bench, rudder blade, tiller
    trim: surface(theme, c.trim), // gunwale rim
    grip: surface(theme, c.accent), // the tiller's grip, the one control
  };
}

export type BoatMaterials = ReturnType<typeof createMaterials>;

export function mesh(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

// A surface from a grid of points: rows along one direction, columns along
// the other. Each quad is split into two triangles wound so that their front
// faces the side given by (column direction) x (row direction). `uv` gives
// each point's texture coordinates, when the surface is textured.
export function gridGeometry(
  rows: number,
  columns: number,
  point: (row: number, column: number) => THREE.Vector3,
  uv?: (row: number, column: number) => [u: number, v: number],
): THREE.BufferGeometry {
  const positions: number[] = [];
  const uvs: number[] = [];
  for (let i = 0; i <= rows; i++) {
    for (let j = 0; j <= columns; j++) {
      const p = point(i, j);
      positions.push(p.x, p.y, p.z);
      if (uv) uvs.push(...uv(i, j));
    }
  }
  const at = (i: number, j: number) => i * (columns + 1) + j;
  const indices: number[] = [];
  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < columns; j++) {
      indices.push(at(i, j), at(i, j + 1), at(i + 1, j));
      indices.push(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  if (uv) geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}

// Texture coordinates in meters for a solid wooden part (a board, a blade, a
// rod), with the grain along its longest side: each point's u is its position
// along that side, and its v its position along whichever other side lies
// flattest in its face.
export function grainUVs(geo: THREE.BufferGeometry): void {
  geo.computeBoundingBox();
  const size = geo.boundingBox!.getSize(new THREE.Vector3()).toArray();
  const along = size.indexOf(Math.max(...size));
  const [a, b] = [0, 1, 2].filter((axis) => axis !== along);
  const position = geo.attributes.position;
  const normal = geo.attributes.normal;
  const uvs = new Float32Array(position.count * 2);
  for (let i = 0; i < position.count; i++) {
    const across = Math.abs(normal.getComponent(i, a)) <= Math.abs(normal.getComponent(i, b)) ? a : b;
    uvs[2 * i] = position.getComponent(i, along);
    uvs[2 * i + 1] = position.getComponent(i, across);
  }
  geo.setAttribute('uv', new THREE.BufferAttribute(uvs, 2));
}
