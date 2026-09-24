import * as THREE from 'three';
import { defaultTheme } from '../../theme';
import { between, seededRandom, sizeOf, wobble, type ObjectOptions, type Size } from './parts';

// A bank of low-lying fog, as an object standing in a scene (not the scene's
// own fog, which the lake's weather thickens and thins): about 8 m wide,
// 1.5 m tall and 5 m deep. It is many soft, see-through puffs layered
// together, thicker in the middle and near the ground, thinning out toward
// its edges and its top. Its colour is the theme's background lightened
// toward its light neutral, a little lighter or darker from puff to puff.
//
// It drifts and swirls slowly on its own (its idle motion, like the lake's
// waves; the spec, Fog.md, is a draft and names no behaviour): the puffs
// drift slowly along +x, each circling round its place as it goes, and
// swell, shrink, thin, thicken and tilt a little, each at its own pace. A
// puff that drifts out of one side comes back in at the other, thinned out
// to nothing at both, so it loops forever. Call update(delta) once a frame;
// it goes by the time passed, so it moves at the same speed at any frame
// rate.
//
// Each puff is a soft oval patch, wider than it is tall, that always faces
// the camera (turned on the graphics card, see puffMaterial()), all of them
// one instanced mesh. They are big and many, because the painterly filter
// averages away sparse, small things. They cast no shadows, write no depth,
// and fade out over the last few centimeters above the ground, so they don't
// end in a hard line where they cut into it.
//
// Units are meters, y is up, the front faces +z and the origin is on the
// ground at the middle of the bank. It is built from a seed and sized to a
// width, height and depth (see ObjectOptions in parts.ts): each puff and its
// wandering stay inside that box, so the same seed always gives the same
// fog, moving the same way.

const SIZE: Size = { width: 8, height: 1.5, depth: 5 };
const SEED = 6;

const PUFFS = 240; // for a bank of the default footprint; more or fewer with its area
const MAX_PUFFS = 2000;
const RADIUS = [0.45, 1.05] as const; // m tall (half), for a bank of the default height; sparse, small puffs would vanish in the painterly filter
const STRETCH = [1.4, 2.2] as const; // a puff is this many times as wide as it is tall, as seen: fog lies in flat layers
const LOW = 0.35; // a puff's middle is at least this share of its height above the ground
const OPACITY = [0.1, 0.22] as const; // of a puff at the ground in the middle of the bank
const THIN_TOP = 0.5; // a puff at the top is this share as thick as one at the ground
// The bank thins out toward its sides: a puff is thickest while its middle
// is within `from` of the way to the edge of its range, and fades to
// nothing at the edge (along x, where it drifts, and along z).
const EDGE = { x: 0.6, z: 0.5 };
const DRIFT = 0.08; // m/s along +x
const SWIRL = { radius: [0.15, 0.45] as const, period: [18, 40] as const }; // m, s: each puff circles round its place
const BOB = { height: [0.03, 0.1] as const, period: [9, 20] as const }; // m, s: and rises and sinks
const GROW = { amount: 0.15, period: [10, 24] as const }; // it swells and shrinks by this share of its size
const FADE = { amount: 0.4, period: [7, 16] as const }; // and thins by up to this share and thickens again
// Each puff tilts slowly back and forth as seen, by up to `angle` radians
// either way; turning right round, a wide puff would stand on end.
const SWAY = { angle: 0.08, period: [15, 30] as const };
const LIGHTEN = 0.3; // how far the colour goes from the theme's background toward its light neutral
const SHADE = [0.96, 1.03] as const; // each puff a little lighter or darker
const GROUND_FADE = 0.12; // m above the ground over which the puffs fade in
const TEXTURE = { size: 64, lumps: 3, lumpiness: 0.35 }; // pixels across; lumps across it

interface Puff {
  x: number; // its place when the bank is shown, before it drifts and swirls
  y: number;
  z: number;
  range: number; // it drifts from -range to range along x, then starts again
  radius: number; // half its height, as seen
  stretch: number; // its width for its height
  opacity: number;
  swirl: { radius: number; rate: number; phase: number }; // rate: rad/s, either way
  bob: { height: number; rate: number; phase: number };
  grow: { rate: number; phase: number };
  fade: { rate: number; phase: number };
  sway: { rate: number; phase: number };
}

// Reused by update().
const matrix = new THREE.Matrix4();
const place = new THREE.Vector3();
const unturned = new THREE.Quaternion();
const scale = new THREE.Vector3();

export class Fog extends THREE.Group {
  static readonly SIZE: Size = SIZE;
  readonly puffs: THREE.InstancedMesh;
  private readonly plan: Puff[];
  private readonly opacity: THREE.InstancedBufferAttribute;
  private readonly turn: THREE.InstancedBufferAttribute;
  private time = 0;

  constructor(options: ObjectOptions = {}) {
    super();
    this.name = 'fog';
    const theme = options.theme ?? defaultTheme;
    const random = seededRandom(options.seed ?? SEED);
    const size = sizeOf(options, SIZE);
    this.plan = planPuffs(size, random);

    const count = this.plan.length;
    const geo = new THREE.PlaneGeometry(2, 2); // corners at ±1: a puff's radius is its size
    this.opacity = new THREE.InstancedBufferAttribute(new Float32Array(count), 1).setUsage(THREE.DynamicDrawUsage);
    this.turn = new THREE.InstancedBufferAttribute(new Float32Array(count), 1).setUsage(THREE.DynamicDrawUsage);
    geo.setAttribute('puffOpacity', this.opacity);
    geo.setAttribute('puffTurn', this.turn);

    const color = new THREE.Color(theme.scene.background).lerp(new THREE.Color(theme.colors.light), LIGHTEN);
    this.puffs = new THREE.InstancedMesh(geo, puffMaterial(color), count);
    this.puffs.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    const shade = new THREE.Color();
    for (let i = 0; i < count; i++) this.puffs.setColorAt(i, shade.setScalar(between(random, ...SHADE)));
    this.puffs.castShadow = false;
    this.puffs.receiveShadow = false;
    this.puffs.frustumCulled = false; // its puffs move every frame
    // Its bounds are the box the puffs stay in, not their flat patches.
    this.puffs.boundingBox = new THREE.Box3(
      new THREE.Vector3(-size.width / 2, 0, -size.depth / 2),
      new THREE.Vector3(size.width / 2, size.height, size.depth / 2),
    );
    this.puffs.boundingSphere = this.puffs.boundingBox.getBoundingSphere(new THREE.Sphere());
    this.add(this.puffs);
    this.update(0);
  }

  // Moves the fog on by `delta` seconds: each puff drifts, circles, rises
  // and sinks, swells, thins and tilts as it goes.
  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    this.plan.forEach((p, i) => {
      const swirl = p.swirl.phase + p.swirl.rate * t;
      // Drifting along x, starting again at -range once past range.
      const along = p.x + p.range + DRIFT * t + p.swirl.radius * Math.cos(swirl);
      const x = -p.range + (((along % (2 * p.range)) + 2 * p.range) % (2 * p.range));
      const z = p.z + p.swirl.radius * Math.sin(swirl);
      const y = p.y + p.bob.height * Math.sin(p.bob.phase + p.bob.rate * t);
      const radius = p.radius * (1 + GROW.amount * Math.sin(p.grow.phase + p.grow.rate * t));
      const thin = 1 - FADE.amount * (0.5 + 0.5 * Math.sin(p.fade.phase + p.fade.rate * t));
      const edge = 1 - THREE.MathUtils.smoothstep(Math.abs(x) / p.range, EDGE.x, 1);
      this.opacity.setX(i, p.opacity * thin * edge);
      this.turn.setX(i, SWAY.angle * Math.sin(p.sway.phase + p.sway.rate * t));
      this.puffs.setMatrixAt(i, matrix.compose(place.set(x, y, z), unturned, scale.set(p.stretch * radius, radius, radius)));
    });
    this.opacity.needsUpdate = true;
    this.turn.needsUpdate = true;
    this.puffs.instanceMatrix.needsUpdate = true;
  }
}

// Where each puff is and how it moves, all inside the box of `size`: swollen,
// seen from any side and tilted as far as it sways, plus its swirl and bob,
// it never reaches past the box's sides or top. Big puffs keep to the middle
// and most puffs lie low; puffs are thinner toward the top and toward the
// front and back.
function planPuffs(size: Size, random: () => number): Puff[] {
  const { width, height, depth } = size;
  const count = Math.min(MAX_PUFFS, Math.round((PUFFS * width * depth) / (SIZE.width * SIZE.depth)));
  const swell = 1 + GROW.amount;
  const rate = (period: readonly [number, number]) => (2 * Math.PI) / between(random, ...period);
  const puffs: Puff[] = [];
  for (let i = 0; i < count; i++) {
    const swirl = between(random, ...SWIRL.radius);
    const bob = between(random, ...BOB.height);
    const stretch = between(random, ...STRETCH);
    // How far it reaches up and to the side, for each meter of its radius.
    const tall = swell * (Math.cos(SWAY.angle) + stretch * Math.sin(SWAY.angle));
    const wide = swell * stretch;
    // As big as wanted, but small enough to fit the box with its wandering.
    const radius = Math.max(
      0.01,
      Math.min(
        (between(random, ...RADIUS) * height) / SIZE.height,
        (height - 2 * bob) / (tall + LOW),
        (width / 2 - swirl) / wide - 0.05,
        (depth / 2 - swirl) / wide - 0.05,
      ),
    );
    const low = LOW * radius + bob;
    const high = Math.max(low, height - tall * radius - bob);
    const y = low + (high - low) * random() ** 2.5;
    const range = Math.max(0.05, width / 2 - wide * radius - swirl);
    const across = Math.max(0, depth / 2 - wide * radius - swirl);
    const z = across * (random() < 0.5 ? -1 : 1) * random() ** 1.15;
    const sides = 1 - THREE.MathUtils.smoothstep(across > 0 ? Math.abs(z) / across : 0, EDGE.z, 1);
    const up = height > 0 ? y / height : 0;
    puffs.push({
      x: between(random, -range, range),
      y,
      z,
      range,
      radius,
      stretch,
      opacity: between(random, ...OPACITY) * THREE.MathUtils.lerp(1, THIN_TOP, up) * sides,
      swirl: { radius: swirl, rate: rate(SWIRL.period) * (random() < 0.5 ? -1 : 1), phase: 2 * Math.PI * random() },
      bob: { height: bob, rate: rate(BOB.period), phase: 2 * Math.PI * random() },
      grow: { rate: rate(GROW.period), phase: 2 * Math.PI * random() },
      fade: { rate: rate(FADE.period), phase: 2 * Math.PI * random() },
      sway: { rate: rate(SWAY.period), phase: 2 * Math.PI * random() },
    });
  }
  return puffs;
}

// A soft, round, slightly lumpy patch: white, its opacity falling smoothly
// from the middle to nothing at the edge. Made once and shared by every fog.
let puffTexture: THREE.DataTexture | null = null;

function softPuff(): THREE.DataTexture {
  if (puffTexture) return puffTexture;
  const n = TEXTURE.size;
  const data = new Uint8Array(4 * n * n);
  for (let j = 0; j < n; j++) {
    for (let i = 0; i < n; i++) {
      const u = (2 * (i + 0.5)) / n - 1;
      const v = (2 * (j + 0.5)) / n - 1;
      const d2 = Math.min(1, u * u + v * v);
      const lump = 1 - TEXTURE.lumpiness * (0.5 + 0.5 * wobble(TEXTURE.lumps * (u + 1), TEXTURE.lumps * (v + 1), SEED));
      const k = 4 * (j * n + i);
      data[k] = data[k + 1] = data[k + 2] = 255;
      data[k + 3] = Math.round(255 * (1 - d2) * (1 - d2) * lump);
    }
  }
  puffTexture = new THREE.DataTexture(data, n, n);
  puffTexture.magFilter = THREE.LinearFilter;
  puffTexture.minFilter = THREE.LinearMipmapLinearFilter;
  puffTexture.generateMipmaps = true;
  puffTexture.needsUpdate = true;
  return puffTexture;
}

// The puffs' material: unlit (fog glows evenly in the light it scatters), in
// the fog's colour times each puff's shade, see-through, and writing no
// depth, so puffs behind one another all show. Its vertex shader turns each
// puff's square to face the camera, sized and turned by the puff, and its
// fragment shader thins each puff by its own opacity and fades it out near
// the ground (the fog's y = 0). It takes the scene's fog like anything else.
function puffMaterial(color: THREE.Color): THREE.MeshBasicMaterial {
  const material = new THREE.MeshBasicMaterial({ color, map: softPuff(), transparent: true, depthWrite: false });
  material.onBeforeCompile = (shader) => {
    shader.vertexShader = shader.vertexShader
      .replace(
        '#include <common>',
        /* glsl */ `#include <common>
        attribute float puffOpacity;
        attribute float puffTurn;
        varying float vPuffOpacity;
        varying float vPuffHeight;`,
      )
      .replace(
        '#include <project_vertex>',
        /* glsl */ `
        vec4 mvPosition = modelViewMatrix * vec4( instanceMatrix[ 3 ].xyz, 1.0 );
        vec2 puffCorner = transformed.xy * vec2( length( instanceMatrix[ 0 ].xyz ), length( instanceMatrix[ 1 ].xyz ) );
        float puffCos = cos( puffTurn );
        float puffSin = sin( puffTurn );
        mvPosition.xy += vec2( puffCos * puffCorner.x - puffSin * puffCorner.y, puffSin * puffCorner.x + puffCos * puffCorner.y );
        gl_Position = projectionMatrix * mvPosition;
        vPuffOpacity = puffOpacity;
        // The corner's height above the fog's origin: back from the view to
        // the world (the view matrix only turns and moves).
        vPuffHeight = ( ( mvPosition.xyz - viewMatrix[ 3 ].xyz ) * mat3( viewMatrix ) ).y - modelMatrix[ 3 ].y;`,
      );
    shader.fragmentShader = shader.fragmentShader
      .replace(
        '#include <common>',
        /* glsl */ `#include <common>
        varying float vPuffOpacity;
        varying float vPuffHeight;`,
      )
      .replace(
        '#include <alphatest_fragment>',
        /* glsl */ `diffuseColor.a *= vPuffOpacity * smoothstep( 0.0, ${GROUND_FADE.toFixed(3)}, vPuffHeight );
        #include <alphatest_fragment>`,
      );
  };
  material.customProgramCacheKey = () => 'fog-puff';
  return material;
}
