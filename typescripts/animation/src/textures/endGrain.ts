import * as THREE from 'three';
import { hash, noise } from './noise';

// End grain made in code: what a log or a branch shows where it is sawn
// across. Growth rings run round the pith, a little wavy and crowding closer
// together toward the bark, each a pale band of spring wood darkening into
// summer wood; the heartwood in the middle is a shade darker than the sapwood
// round it; a few drying cracks (checks) run in from the bark; and a rim of
// bark runs round it all. It is grey, so it shades the theme's wood colour:
// pale inside the rim, like fresh wood, and the rim about as dark as the bark
// texture (textures/bark.ts).
//
// The tile is one disk: the pith in the middle (0.5, 0.5) and the outside of
// the bark on the circle of radius 0.5 round it. Give a cut face texture
// coordinates that put each point at its share of the way from the pith out
// to the bark, in its direction from the pith (cutGeometry() in
// figures/objects/Log.ts), and the rings follow the face's outline, whatever
// its shape and wherever its pith.

const SIZE = 512; // pixels a side
const RINGS = 22; // growth rings from the pith to the bark
const CROWD = 1.35; // above 1, the rings crowd together toward the bark, where the tree grew slower
const YEARS = { rings: 0.8, cells: 6 }; // some years grew more than others: rings bunch and spread by up to `rings` rings, `cells` times from the pith out
const WAVE = { depth: 0.06, cells: 4 }; // the rings wander in and out by up to this share of the radius, `cells` times round
const EARLY = 0.97; // spring wood
const LATE = { from: 0.5, to: 0.85, shade: 0.22 }; // summer wood darkens each ring from `from` to `to` of the way across it
const HEART = { edge: 0.6, shade: 0.1 }; // heartwood within this share of the radius
const PITH = { radius: 0.02, shade: 0.4 };
const RAYS = 0.04; // faint streaks running out from the pith
const GRIT = 0.04; // and fine speckle all over
// The bark: its inner edge at `edge` of the radius, wandering by `wander`;
// the inner bark (the first `inside` of it) `light`, the outer bark `dark`
// and roughened by `rough`.
const BARK = { edge: 0.92, wander: 0.012, inside: 0.3, light: 0.64, dark: 0.46, rough: 0.2 };
const RIM_STEPS = 2048; // steps round the bark's inner edge, worked out once
const CAMBIUM = 0.8; // the thin line between the wood and the bark, times what is there
// Drying cracks, running in from the bark by `reach` of the radius, `width`
// pixels wide at the bark and closing to nothing at their inner end, and
// wandering a little as they go (`wander` of a turn either way).
const CHECKS = { count: 4, reach: [0.25, 0.7] as const, width: 4, wander: 0.004, shade: 0.3 };
const SEED = 71;

// How far `a` is round from `b`, both shares of a turn: -0.5..0.5.
function offTurn(a: number, b: number): number {
  return ((((a - b) % 1) + 1.5) % 1) - 0.5;
}

let cached: THREE.DataTexture | null = null;

// The texture, made once and shared by every material that uses it.
export function endGrain(): THREE.DataTexture {
  if (cached) return cached;
  const checks = Array.from({ length: CHECKS.count }, (_, k) => ({
    around: (k + 0.6 * hash(k, 1, SEED)) / CHECKS.count, // spread round, a share of a turn
    inner: 1 - (CHECKS.reach[0] + (CHECKS.reach[1] - CHECKS.reach[0]) * hash(k, 2, SEED)),
  }));
  // The bark's inner edge, all the way round.
  const rims = Array.from({ length: RIM_STEPS }, (_, i) => BARK.edge + BARK.wander * (2 * noise((i / RIM_STEPS) * 24, 0.5, 24, SEED + 1) - 1));
  const half = SIZE / 2;
  const data = new Uint8Array(SIZE * SIZE * 4);
  for (let y = 0; y < SIZE; y++) {
    for (let x = 0; x < SIZE; x++) {
      const dx = x + 0.5 - half;
      const dy = y + 0.5 - half;
      const s = Math.hypot(dx, dy) / half; // 0 at the pith, 1 at the bark's outside
      const around = (Math.atan2(dy, dx) / (2 * Math.PI) + 1) % 1; // a share of a turn
      const rim = rims[Math.floor(around * RIM_STEPS) % RIM_STEPS];
      let v: number;
      if (s < rim) {
        // The rings, wavier away from the pith.
        const wavy = s + WAVE.depth * Math.min(1, s / 0.3) * (2 * noise(around * WAVE.cells, s * 3, WAVE.cells, SEED + 2) - 1);
        // Which ring, counted from the pith. Toward the pith, where the rings
        // are few, the years vary less, so no ring folds back on itself.
        const g = RINGS * Math.max(0, wavy) ** CROWD + YEARS.rings * Math.min(1, s / 0.4) * (2 * noise(wavy * YEARS.cells, 0.5, 0, SEED + 7) - 1);
        const ring = Math.floor(g);
        const strength = 0.6 + 0.8 * hash(ring, 3, SEED); // some rings stand out more
        v = EARLY - LATE.shade * strength * THREE.MathUtils.smoothstep(g - ring, LATE.from, LATE.to);
        v *= 1 - HEART.shade * (1 - THREE.MathUtils.smoothstep(wavy, HEART.edge - 0.04, HEART.edge + 0.04));
        v -= RAYS * (noise(around * 160, s * 2, 160, SEED + 3) - 0.5);
        v *= 1 - PITH.shade * (1 - THREE.MathUtils.smoothstep(s, 0.6 * PITH.radius, PITH.radius));
        v *= 1 - (1 - CAMBIUM) * (1 - THREE.MathUtils.smoothstep(rim - s, 0, 3 / half));
      } else {
        const t = Math.min(1, (s - rim) / (1 - rim));
        v = THREE.MathUtils.lerp(BARK.light, BARK.dark,THREE.MathUtils.smoothstep(t, BARK.inside - 0.1, BARK.inside + 0.1));
        v += BARK.rough * (noise(around * 90, s * 25, 90, SEED + 4) - 0.5);
      }
      v -= GRIT * (hash(x, y, SEED + 5) - 0.5);
      // The cracks, from the bark in, widening outward. Only pixels near
      // one need its wander worked out.
      for (let k = 0; k < checks.length; k++) {
        const check = checks[k];
        if (s <= check.inner || s > 1) continue;
        const width = CHECKS.width * Math.min(1, (s - check.inner) / (1 - check.inner)) ** 0.7;
        const pixels = 2 * Math.PI * s * half; // round the circle through this pixel
        if (Math.abs(offTurn(around, check.around)) * pixels > width / 2 + 1 + CHECKS.wander * pixels) continue;
        const at = check.around + CHECKS.wander * (2 * noise(s * 6, k * 5, 0, SEED + 6) - 1);
        const off = Math.abs(offTurn(around, at)) * pixels; // from the crack's middle
        v *= THREE.MathUtils.lerp(CHECKS.shade, 1, THREE.MathUtils.smoothstep(off, width / 2 - 1, width / 2 + 1));
      }
      const byte = Math.round(255 * Math.min(1, Math.max(0, v)));
      const i = (y * SIZE + x) * 4;
      data[i] = data[i + 1] = data[i + 2] = byte;
      data[i + 3] = 255;
    }
  }

  const texture = new THREE.DataTexture(data, SIZE, SIZE, THREE.RGBAFormat);
  texture.colorSpace = THREE.SRGBColorSpace;
  texture.magFilter = THREE.LinearFilter;
  texture.minFilter = THREE.LinearMipmapLinearFilter;
  texture.generateMipmaps = true;
  texture.anisotropy = 8; // keeps the rings sharp on an end seen at a slant
  texture.needsUpdate = true;
  cached = texture;
  return texture;
}
