// Noise shared by the textures made in code (wood grain, bark).

// A repeatable pseudo-random number in 0..1 for a lattice point.
export function hash(x: number, y: number, seed: number): number {
  let h = (x * 374761393 + y * 668265263 + seed * 2147483647) | 0;
  h = Math.imul(h ^ (h >>> 13), 1274126177);
  return ((h ^ (h >>> 16)) >>> 0) / 4294967295;
}

// Smooth value noise in 0..1. It repeats every `periodX` cells along x, and
// every `periodY` cells along y when given, so a texture's opposite edges
// match and it tiles without a seam.
export function noise(x: number, y: number, periodX: number, seed: number, periodY?: number): number {
  const ix = Math.floor(x);
  const iy = Math.floor(y);
  const fx = x - ix;
  const fy = y - iy;
  const sx = fx * fx * (3 - 2 * fx);
  const sy = fy * fy * (3 - 2 * fy);
  const wrap = (i: number, period?: number) => (period ? ((i % period) + period) % period : i);
  const x0 = wrap(ix, periodX);
  const x1 = wrap(ix + 1, periodX);
  const y0 = wrap(iy, periodY);
  const y1 = wrap(iy + 1, periodY);
  const a = hash(x0, y0, seed);
  const b = hash(x1, y0, seed);
  const c = hash(x0, y1, seed);
  const d = hash(x1, y1, seed);
  return a + (b - a) * sx + (c - a) * sy + (a - b - c + d) * sx * sy;
}
