import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';
import { Border } from '../Border';
import { hazeEdge } from '../haze';
import { Sky } from '../Sky';

// A lawn: short grass, thick in the middle where a figure stands and thinning
// out, on ground a shade darker than the blades that runs to the horizon. The
// colour comes from the theme (theme.scene.grass). Units are meters, y is up,
// and the origin is the middle of the lawn, on the ground.
//
// The blades are one instanced mesh: one bent, pointed blade, placed, turned,
// sized and shaded at random for each blade, from a fixed seed so the lawn is
// the same every time. Blades take shadows but cast none, which would cost a
// shadow pass over thousands of blades.
//
// Where the ground ends, 40 m out, the border (../Border.ts) hides the edge,
// as on the lake: high cliffs along parts of it, mist drifting round it and
// tall grass up to it (Grass.md), and the ground fades into the sky's colour
// over its last meters (../haze.ts).
//
// Over it is the sky (../Sky.ts), as on the lake, with the sun and clouds.
// The lawn has no weather, so its clouds drift toward +x and its sky never
// greys over. Call update(delta) once a frame for the mist and the clouds.

const GROUND_RADIUS = 40;
const LAWN_RADIUS = 4; // blades grow this far from the middle
const BLADES = 30000;
const BLADE_HEIGHT = [0.02, 0.04] as const; // shortest and tallest: a mown lawn, below a keyboard's keys
const BLADE_SHADE = [0.8, 1.05] as const; // darkest and lightest blade, times the grass colour
const GROUND_SHADE = 0.75; // the ground between the blades, times the grass colour
const SEED = 7;

// The shape of one blade, 1 unit tall; each blade is scaled to its height.
const SEGMENTS = 3; // along the blade
const WIDTH = 0.12; // at the base, as a share of the height
const LEAN = 0.35; // how far the tip leans over, as a share of the height
const BASE_SHADE = 0.7; // the base is darker than the tip, which has the grass colour

export interface GrassOptions {
  theme?: Theme;
}

export class Grass extends THREE.Group {
  readonly ground: THREE.Mesh;
  readonly border: Border;
  readonly sky: Sky;

  constructor(options: GrassOptions = {}) {
    super();
    this.name = 'grass';
    const theme = options.theme ?? defaultTheme;
    const color = new THREE.Color(theme.scene.grass);

    // Laid down in its geometry rather than turned, so its own x and z are
    // the lawn's, which the haze measures from the middle.
    const material = new THREE.MeshStandardMaterial({ color: color.clone().multiplyScalar(GROUND_SHADE), roughness: 1 });
    hazeEdge(material, theme.scene.background, GROUND_RADIUS);
    const ground = new THREE.Mesh(new THREE.CircleGeometry(GROUND_RADIUS, 64).rotateX(-Math.PI / 2), material);
    ground.receiveShadow = true;
    this.ground = ground;
    this.border = new Border({ theme, radius: GROUND_RADIUS });
    this.sky = new Sky({ theme });

    const blades = new THREE.InstancedMesh(
      bladeGeometry(),
      new THREE.MeshStandardMaterial({ color, roughness: 0.9, vertexColors: true }),
      BLADES,
    );
    blades.receiveShadow = true;
    const random = seededRandom(SEED);
    const matrix = new THREE.Matrix4();
    const position = new THREE.Vector3();
    const turn = new THREE.Quaternion();
    const up = new THREE.Vector3(0, 1, 0);
    const size = new THREE.Vector3();
    const shade = new THREE.Color();
    for (let i = 0; i < BLADES; i++) {
      // A radius picked evenly (not by area) puts more blades near the middle.
      const r = LAWN_RADIUS * random();
      const angle = 2 * Math.PI * random();
      position.set(r * Math.cos(angle), 0, r * Math.sin(angle));
      turn.setFromAxisAngle(up, 2 * Math.PI * random());
      size.setScalar(THREE.MathUtils.lerp(BLADE_HEIGHT[0], BLADE_HEIGHT[1], random()));
      blades.setMatrixAt(i, matrix.compose(position, turn, size));
      blades.setColorAt(i, shade.setScalar(THREE.MathUtils.lerp(BLADE_SHADE[0], BLADE_SHADE[1], random())));
    }

    this.add(ground, blades, this.border, this.sky);
  }

  // Moves the border's mist and the clouds on; call once per frame.
  update(delta: number): void {
    this.border.update(delta);
    this.sky.update(delta);
  }
}

// One blade, 1 unit tall, narrowing from its base to a point and leaning over
// toward +z as it rises. Both sides are front faces with normals pointing up,
// so a blade is lit like the ground whichever side you see (a double-sided
// material would light the back from below). Its shade runs from BASE_SHADE
// at the base to 1 at the tip.
function bladeGeometry(): THREE.BufferGeometry {
  const positions: number[] = [];
  const colors: number[] = [];
  for (let i = 0; i <= SEGMENTS; i++) {
    const t = i / SEGMENTS;
    const half = (WIDTH / 2) * (1 - t);
    const z = LEAN * t * t;
    const shade = THREE.MathUtils.lerp(BASE_SHADE, 1, t);
    if (i < SEGMENTS) {
      positions.push(-half, t, z, half, t, z);
      colors.push(shade, shade, shade, shade, shade, shade);
    } else {
      positions.push(0, t, z); // the tip
      colors.push(shade, shade, shade);
    }
  }

  // Two triangles between each pair of levels, and one up to the tip, facing
  // +z; then the same triangles wound the other way, facing -z.
  const indices: number[] = [];
  for (let i = 0; i < SEGMENTS - 1; i++) {
    const a = 2 * i; // left and right of this level; the next level's are a + 2 and a + 3
    indices.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
  }
  const last = 2 * (SEGMENTS - 1);
  indices.push(last, last + 1, 2 * SEGMENTS);
  const front = indices.length;
  for (let i = 0; i < front; i += 3) indices.push(indices[i], indices[i + 2], indices[i + 1]);

  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  const up = positions.map((_, i) => (i % 3 === 1 ? 1 : 0));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(up, 3));
  geo.setIndex(indices);
  return geo;
}

// A seeded random generator (mulberry32): numbers from 0 to 1, the same
// sequence for the same seed.
function seededRandom(seed: number): () => number {
  return () => {
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
