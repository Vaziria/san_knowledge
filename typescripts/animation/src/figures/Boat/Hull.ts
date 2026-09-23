import * as THREE from 'three';
import { grainUVs, gridGeometry, mesh, type BoatMaterials } from './parts';

// The hull of a small wooden dinghy, 2.4 m long and 1.1 m wide: an open,
// round-bilged shell of planks with a flat transom at the stern, a pointed
// bow whose keel rises a little, a painted gunwale rim along the top edge and
// a bench across the middle. Its origin is on the floor under the middle of the keel, and
// the bow points +z.
//
// The shape is a function of two numbers: u runs along the hull from the
// stern (0) to the bow (1), and t runs around a cross-section from the port
// gunwale (-1) through the keel (0) to the starboard gunwale (1).

export const HULL_LENGTH = 2.4;
const HULL_BEAM = 1.1; // widest
const WIDEST = 0.45; // where the hull is widest, as u
const TRANSOM = 0.72; // stern width as a share of the beam
const SHEER = 0.45; // gunwale height amidships
const SHEER_RISE = 0.1; // the gunwale rises toward the bow
const BOW_RISE = 0.14; // the keel rises toward the bow
const STERN_RISE = 0.04; // and a little toward the stern
// Section shape: 2 is a round bottom, higher is flatter with fuller sides.
const SECTION_POWER = 2.6;

const ROWS = 48; // along the hull
const COLUMNS = 40; // around a section
const RIM_RADIUS = 0.022;

const BENCH_U = 0.42;
const BENCH_Y = 0.3;
const BENCH_DEPTH = 0.22;
const BENCH_THICKNESS = 0.03;
const BENCH_CLEARANCE = 0.004; // total gap left between the bench's ends and the hull

export function hullZ(u: number): number {
  return (u - 0.5) * HULL_LENGTH;
}

// Half the hull's width at its gunwale.
function halfBeam(u: number): number {
  if (u < WIDEST) return (HULL_BEAM / 2) * (TRANSOM + (1 - TRANSOM) * Math.sin(((Math.PI / 2) * u) / WIDEST));
  return (HULL_BEAM / 2) * Math.pow(Math.cos(((Math.PI / 2) * (u - WIDEST)) / (1 - WIDEST)), 0.75);
}

export function sheerHeight(u: number): number {
  return SHEER + SHEER_RISE * u * u;
}

export function keelHeight(u: number): number {
  const bow = Math.max(0, (u - 0.55) / 0.45);
  const stern = Math.max(0, (0.25 - u) / 0.25);
  return BOW_RISE * bow * bow + STERN_RISE * stern * stern;
}

// A superellipse section: the gunwale at t = ±1, the keel at t = 0.
function hullPoint(u: number, t: number, target: THREE.Vector3): THREE.Vector3 {
  const a = (t * Math.PI) / 2;
  const e = 2 / SECTION_POWER;
  const top = sheerHeight(u);
  const x = halfBeam(u) * Math.sign(a) * Math.pow(Math.abs(Math.sin(a)), e);
  const y = top - (top - keelHeight(u)) * Math.pow(Math.abs(Math.cos(a)), e);
  return target.set(x, y, hullZ(u));
}

// Half the inside width of the hull at height y, at u.
function halfWidthAt(u: number, y: number): number {
  const e = 2 / SECTION_POWER;
  const top = sheerHeight(u);
  const c = THREE.MathUtils.clamp((top - y) / (top - keelHeight(u)), 0, 1);
  return halfBeam(u) * Math.pow(Math.sin(Math.acos(Math.pow(c, 1 / e))), e);
}

// The inside of a thin shell: the same geometry drawn from behind. Only the
// outside casts the shell's shadow; if both did, each side would shadow the
// other and the surfaces would fill with stripes (shadow acne).
function insideOf(geo: THREE.BufferGeometry, mat: THREE.Material): THREE.Mesh {
  const inside = mesh(geo, mat);
  inside.castShadow = false;
  return inside;
}

// The distance around the section at u, from gunwale to gunwale.
function girth(u: number): number {
  const a = hullPoint(u, -1, new THREE.Vector3());
  const b = new THREE.Vector3();
  let length = 0;
  for (let j = 1; j <= COLUMNS; j++) {
    hullPoint(u, (j / COLUMNS) * 2 - 1, b);
    length += a.distanceTo(b);
    a.copy(b);
  }
  return length;
}

export class Hull extends THREE.Group {
  constructor(m: BoatMaterials) {
    super();
    this.name = 'hull';

    // Rows run stern to bow and columns port to starboard, so the triangles'
    // front faces point out of the hull. The inside is the same surface drawn
    // from behind. The wood's grain runs along the hull (u in meters) and its
    // planks lie side by side around it (v), sized to the widest section: a
    // hull keeps the same number of planks from end to end, so they narrow
    // toward the bow and stern as a real boat's strakes do.
    const planking = girth(WIDEST);
    const shell = gridGeometry(
      ROWS,
      COLUMNS,
      (i, j) => hullPoint(i / ROWS, (j / COLUMNS) * 2 - 1, new THREE.Vector3()),
      (i, j) => [hullZ(i / ROWS), (j / COLUMNS) * planking],
    );
    this.add(mesh(shell, m.hull), insideOf(shell, m.inside));

    // The transom closes the stern: its outline is the stern section, and it
    // is turned to face aft so its front is the outside. Its texture
    // coordinates are the outline's, in meters, so its boards run across the
    // stern.
    const outline = new THREE.Shape();
    const p = new THREE.Vector3();
    for (let j = 0; j <= COLUMNS; j++) {
      hullPoint(0, (j / COLUMNS) * 2 - 1, p);
      if (j === 0) outline.moveTo(p.x, p.y);
      else outline.lineTo(p.x, p.y);
    }
    outline.closePath();
    const transom = new THREE.ShapeGeometry(outline, 24);
    transom.rotateY(Math.PI);
    transom.translate(0, 0, hullZ(0));
    this.add(mesh(transom, m.hull), insideOf(transom, m.inside));

    // The gunwale rim runs up the starboard side to the bow, back down the
    // port side, and across the top of the transom.
    const rim: THREE.Vector3[] = [];
    const steps = 24;
    for (let k = 0; k <= steps; k++) rim.push(hullPoint(k / steps, 1, new THREE.Vector3()));
    for (let k = steps - 1; k >= 0; k--) rim.push(hullPoint(k / steps, -1, new THREE.Vector3()));
    const rimCurve = new THREE.CatmullRomCurve3(rim, true, 'centripetal');
    this.add(mesh(new THREE.TubeGeometry(rimCurve, 240, RIM_RADIUS, 10, true), m.trim));

    // One bench across the widest part. The hull narrows downward and toward
    // both ends, so the bench is as wide as the hull at its tightest corner
    // (bottom face, front or back edge); any wider and its corners poke out.
    const halfDepth = BENCH_DEPTH / 2 / HULL_LENGTH; // as u
    const underside = BENCH_Y - BENCH_THICKNESS / 2;
    const benchWidth =
      2 * Math.min(halfWidthAt(BENCH_U - halfDepth, underside), halfWidthAt(BENCH_U + halfDepth, underside)) -
      BENCH_CLEARANCE;
    const benchGeo = new THREE.BoxGeometry(benchWidth, BENCH_THICKNESS, BENCH_DEPTH);
    grainUVs(benchGeo);
    const bench = mesh(benchGeo, m.timber);
    bench.position.set(0, BENCH_Y, hullZ(BENCH_U));
    this.add(bench);
  }
}
