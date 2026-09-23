import * as THREE from 'three';
import { gloss, surface, type Theme } from '../../theme';

// Helpers shared by the fish's parts.

const FIN_THICKNESS = 0.0025;

// Materials for every part, built once per fish from the theme. The body is
// painted per vertex, from the back colour to the belly colour, so its
// material is white and takes the colours from the vertices.
export function createMaterials(theme: Theme) {
  const c = theme.colors;
  const skin = surface(theme, 0xffffff);
  skin.vertexColors = true;
  return {
    skin,
    back: new THREE.Color(c.body), // the back, down to the sides
    belly: new THREE.Color(c.light), // the belly, up to the sides
    fin: surface(theme, c.trim), // every fin and the tail
    white: gloss(c.light), // eyeballs and their highlights
    pupil: gloss(c.dark), // eyes are glossy whatever the finish
    mouth: surface(theme, c.dark),
  };
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

// A thin fin from an outline drawn from the side: the outline's x runs along
// the fish (+z, toward the snout) and its y is up. The fin stands in the fish's
// middle plane (x = 0), FIN_THICKNESS thick.
export function finGeometry(outline: THREE.Shape): THREE.BufferGeometry {
  const geo = new THREE.ExtrudeGeometry(outline, { depth: FIN_THICKNESS, bevelEnabled: false, curveSegments: 16 });
  geo.translate(0, 0, -FIN_THICKNESS / 2);
  geo.rotateY(-Math.PI / 2); // the outline's x becomes +z, its thickness x
  return geo;
}

// A closed tube from a grid of points: rows along the tube, columns around it.
// The last column joins the first, so the tube has no seam. Each quad is split
// into two triangles wound so that their front faces the side given by
// (column direction) x (row direction).
export function tubeGeometry(rows: number, columns: number, point: (row: number, column: number) => THREE.Vector3): THREE.BufferGeometry {
  const positions: number[] = [];
  for (let i = 0; i <= rows; i++) {
    for (let j = 0; j < columns; j++) {
      const p = point(i, j);
      positions.push(p.x, p.y, p.z);
    }
  }
  const at = (i: number, j: number) => i * columns + (j % columns);
  const indices: number[] = [];
  for (let i = 0; i < rows; i++) {
    for (let j = 0; j < columns; j++) {
      indices.push(at(i, j), at(i, j + 1), at(i + 1, j));
      indices.push(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
    }
  }
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setIndex(indices);
  geo.computeVertexNormals();
  return geo;
}
