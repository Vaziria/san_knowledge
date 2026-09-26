# 9 new animal draft

# Shape reference.

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Low-poly animal park — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #3a6ff0;
    box-sizing: border-box;
    padding-top: env(safe-area-inset-top, 0px);
    padding-bottom: env(safe-area-inset-bottom, 0px);
  }
  @media (prefers-color-scheme: dark) {
    :root:not([data-theme="light"]) {
      --bg: #1c1f24; --ink: #e9ebef; --muted: #9096a1;
      --chip: #2a2e35cc; --chip-on: #e9ebef; --chip-on-ink: #1c1f24;
    }
  }
  :root[data-theme="dark"] {
    --bg: #1c1f24; --ink: #e9ebef; --muted: #9096a1;
    --chip: #2a2e35cc; --chip-on: #e9ebef; --chip-on-ink: #1c1f24;
  }
  html { scroll-padding-top: env(safe-area-inset-top, 0px); height: 100%; }
  *, *::before, *::after { box-sizing: inherit; }
  body {
    margin: 0; height: 100%; overflow: hidden;
    background: var(--bg); color: var(--ink);
    font-family: "Manrope", system-ui, -apple-system, "Segoe UI", sans-serif;
  }
  #stage { position: fixed; inset: 0; }
  canvas { display: block; }
  header { position: fixed; top: env(safe-area-inset-top, 0px); left: 0; right: 0; padding: 20px 24px; pointer-events: none; }
  h1 { margin: 0; font-size: 1.15rem; font-weight: 600; letter-spacing: -0.01em; }
  header p { margin: 4px 0 0; font-size: 0.85rem; color: var(--muted); }
  nav {
    position: fixed; left: 50%; transform: translateX(-50%);
    bottom: calc(20px + env(safe-area-inset-bottom, 0px));
    display: flex; gap: 6px; flex-wrap: wrap; justify-content: center;
    width: max-content; max-width: calc(100% - 24px);
  }
  button {
    font: inherit; font-size: 0.85rem; font-weight: 600;
    border: 0; border-radius: 999px; padding: 9px 16px; cursor: pointer;
    background: var(--chip); color: var(--ink);
    backdrop-filter: blur(8px); -webkit-backdrop-filter: blur(8px);
  }
  button[aria-pressed="true"] { background: var(--chip-on); color: var(--chip-on-ink); }
  button:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
  #picker {
    display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px;
    pointer-events: auto; max-width: 760px;
  }
  #picker button { padding: 7px 13px; font-size: 0.8rem; }
  .sep { width: 1px; background: var(--muted); opacity: .35; margin: 4px 4px; }
</style>
</head>
<body>
<div id="stage"></div>
<header>
  <h1>Low-poly animal park</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
  <div id="picker" role="group" aria-label="Choose an animal"></div>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="animate" aria-pressed="true">Animate</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   LOW-POLY ANIMAL PARK
   Coordinates for every animal: x = right, y = up, z = forward.
   Every animal is made from the same few helpers:
     trisToGeometry()  list of triangles  → BufferGeometry
     loft()            rings along a path → tube BufferGeometry
     buildHead()       the wolf head from the start, stretched and
                       recoloured to become any animal's head
   Each animal is then just a description ("spec") at the bottom.
   ================================================================= */

// repeatable "random" number from a seed
const rand = n => { const v = Math.sin(n * 91.7 + 3.1) * 43758.55; return v - Math.floor(v); };
const stripe = (v, width = 0.5) => Math.sin(v) > width;

/* -----------------------------------------------------------------
   HELPER 1: triangles → BufferGeometry
   "ref" (optional) is a point inside the shape used to make every
   triangle face outward.
   ----------------------------------------------------------------- */
function trisToGeometry(tris) {
  const positions = [], colors = [];
  const A = new THREE.Vector3(), B = new THREE.Vector3(), C = new THREE.Vector3();
  const n = new THREE.Vector3(), mid = new THREE.Vector3(), tmp = new THREE.Vector3(), R = new THREE.Vector3();
  for (const t of tris) {
    let a = t.a, b = t.b, c = t.c;
    if (t.ref) {
      A.fromArray(a); B.fromArray(b); C.fromArray(c);
      n.subVectors(B, A).cross(tmp.subVectors(C, A));
      mid.copy(A).add(B).add(C).divideScalar(3).sub(R.fromArray(t.ref));
      if (n.dot(mid) < 0) [b, c] = [c, b];
    }
    positions.push(...a, ...b, ...c);
    colors.push(...t.color, ...t.color, ...t.color);
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  g.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  g.computeVertexNormals();
  return g;
}

/* -----------------------------------------------------------------
   HELPER 2: loft — a tube of rings along a path in the y–z plane.
   sections: [y, z, radiusX, radiusY]
   caps: 'flat' or [y, z] (closes to a point)
   colorFor(segment, triangleCentre, ringCentre) → [r, g, b]
   ----------------------------------------------------------------- */
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => [0.8, 0.8, 0.8] } = {}) {
  const X = new THREE.Vector3(1, 0, 0);
  const centers = sections.map(s => new THREE.Vector3(0, s[0], s[1]));
  const rings = sections.map((s, i) => {
    const prev = centers[Math.max(i - 1, 0)], next = centers[Math.min(i + 1, centers.length - 1)];
    const tangent = next.clone().sub(prev).normalize();
    const up = new THREE.Vector3().crossVectors(tangent, X).normalize();
    return [...Array(sides).keys()].map(k => {
      const ang = Math.PI / 2 + (2 * Math.PI * k) / sides;
      return centers[i].clone().addScaledVector(X, Math.cos(ang) * s[2]).addScaledVector(up, Math.sin(ang) * s[3]).toArray();
    });
  });
  const centreOf = (a, b, c) => new THREE.Vector3((a[0] + b[0] + c[0]) / 3, (a[1] + b[1] + c[1]) / 3, (a[2] + b[2] + c[2]) / 3);
  const tris = [];
  for (let i = 0; i < rings.length - 1; i++) {
    const refV = centers[i].clone().add(centers[i + 1]).multiplyScalar(0.5), ref = refV.toArray();
    for (let k = 0; k < sides; k++) {
      const a = rings[i][k], b = rings[i][(k + 1) % sides], c = rings[i + 1][k], d = rings[i + 1][(k + 1) % sides];
      tris.push({ a, b, c: d, ref, color: colorFor(i, centreOf(a, b, d), refV) });
      tris.push({ a, b: d, c, ref, color: colorFor(i, centreOf(a, d, c), refV) });
    }
  }
  const addCap = (cap, ring, center, inner, seg) => {
    if (!cap) return;
    const tip = cap === 'flat' ? center.toArray() : [0, cap[0], cap[1]];
    for (let k = 0; k < sides; k++) {
      const a = ring[k], b = ring[(k + 1) % sides];
      tris.push({ a, b, c: tip, ref: inner.toArray(), color: colorFor(seg, centreOf(a, b, tip), center) });
    }
  };
  const last = rings.length - 1;
  addCap(startCap, rings[0], centers[0], centers[1], 0);
  addCap(endCap, rings[last], centers[last], centers[last - 1], last - 1);
  return trisToGeometry(tris);
}

/* -----------------------------------------------------------------
   HELPER 3: one head for every animal.
   The base points are the wolf mask from the very first model.
   Each face has a REGION name, so an animal can colour "muzzle",
   "nose", "earIn" … without touching the geometry.
   ----------------------------------------------------------------- */
const BASE_V = {
  C_back: [0, 1.45, -1.20], C_forehead: [0, 1.70, 0.05], C_brow: [0, 0.55, 0.90],
  C_stop: [0, 0.35, 1.10], C_snoutTip: [0, 0.12, 2.40], C_noseBot: [0, -0.42, 2.50],
  C_chin: [0, -0.72, 1.20], C_throat: [0, -0.70, 0.25],
  topPlate: [0.45, 1.62, 0.10], temple: [0.78, 1.20, 0.30], brow: [0.40, 0.55, 0.85],
  browOuter: [0.82, 0.78, 0.60],
  eyeIn: [0.38, 0.32, 0.93], eyeOut: [0.74, 0.38, 0.68], eyeLow: [0.52, 0.10, 0.84], eyeDeep: [0.56, 0.30, 0.55],
  cheek: [1.22, 0.60, 0.00], cheekLow: [1.00, -0.20, 0.30], jaw: [0.60, -0.45, 0.90],
  back: [0.90, 1.00, -1.00], backLow: [1.10, -0.10, -0.70],
  snoutBase: [0.30, 0.30, 1.15], snoutTip: [0.25, 0.05, 2.35], noseBot: [0.22, -0.40, 2.45], snoutBot: [0.30, -0.60, 1.30],
  earFront: [0.50, 1.55, 0.15], earOut: [1.25, 0.95, 0.05], earBack: [0.75, 1.40, -0.55],
  earTip: [1.10, 2.55, -0.20], earInner: [0.90, 1.45, -0.15],
};
const BASE_F = [
  ['C_back','C_forehead','topPlate','top'], ['C_back','topPlate','back','top'],
  ['C_forehead','brow','topPlate','top'], ['C_forehead','C_brow','brow','top'],
  ['topPlate','brow','temple','top'], ['temple','brow','browOuter','top'],
  ['topPlate','temple','back','top'], ['back','temple','cheek','side'],
  ['temple','browOuter','cheek','side'],
  ['brow','eyeOut','browOuter','brow'], ['brow','eyeIn','eyeOut','brow'],
  ['C_brow','eyeIn','brow','top'], ['browOuter','eyeOut','cheek','cheek'],
  ['eyeOut','cheekLow','cheek','cheek'], ['eyeOut','eyeLow','cheekLow','mask'],
  ['eyeIn','snoutBase','eyeLow','mask'],
  ['eyeIn','eyeLow','eyeDeep','eye'], ['eyeLow','eyeOut','eyeDeep','eye'], ['eyeOut','eyeIn','eyeDeep','eye'],
  ['C_brow','snoutBase','eyeIn','bridge'], ['C_brow','C_stop','snoutBase','bridge'],
  ['eyeLow','jaw','cheekLow','cheekLow'], ['eyeLow','snoutBase','jaw','muzzle'],
  ['snoutBase','snoutBot','jaw','muzzle'],
  ['C_stop','snoutTip','snoutBase','snoutTop'], ['C_stop','C_snoutTip','snoutTip','snoutTop'],
  ['snoutBase','noseBot','snoutBot','muzzle'], ['snoutBase','snoutTip','noseBot','muzzle'],
  ['C_snoutTip','noseBot','snoutTip','nose'], ['C_snoutTip','C_noseBot','noseBot','nose'],
  ['C_noseBot','snoutBot','noseBot','chin'], ['C_noseBot','C_chin','snoutBot','chin'],
  ['C_chin','jaw','snoutBot','chin'], ['C_chin','C_throat','jaw','throat'],
  ['C_throat','backLow','jaw','throat'], ['jaw','backLow','cheekLow','cheekLow'],
  ['cheek','cheekLow','backLow','side'], ['cheek','backLow','back','side'],
  ['earFront','earTip','earInner','earIn'], ['earInner','earTip','earOut','earIn'],
  ['earFront','earInner','earOut','earOut'], ['earOut','earTip','earBack','earBack'],
  ['earBack','earTip','earFront','earBack'],
];
// regions that fall back to another region's colour if the animal doesn't set them
const FALLBACK = { side: 'top', brow: 'top', cheek: 'side', mask: 'cheek', bridge: 'top', cheekLow: 'cheek',
  muzzle: 'cheekLow', snoutTop: 'bridge', chin: 'muzzle', throat: 'chin', earIn: 'top', earOut: 'top', earBack: 'top',
  nose: 'top', eye: 'top' };

function buildHead(o) {
  const w = o.w ?? 1, h = o.h ?? 1, sl = o.snout ?? 1, sw = o.snoutW ?? 1, drop = o.snoutDrop ?? 0;
  // stretch the wolf head: whole head by w/h, and only the snout part by snout/snoutW/snoutDrop
  const morph = ([x, y, z]) => {
    const t = Math.min(1, Math.max(0, (z - 0.8) / 0.6));
    const nx = x * w * (1 + (sw - 1) * t);
    let ny = y * h, nz = z;
    if (z > 1.0) { ny += drop * (z - 1.0) / 1.5; nz = 1.0 + (z - 1.0) * sl; }
    return [nx, ny, nz];
  };
  const V = {};
  for (const k in BASE_V) V[k] = morph(BASE_V[k]);

  // ears: scale around their base, move, and optionally put the tip somewhere new
  const ear = { scale: 1, show: true, offset: [0, 0, 0], ...o.ear };
  const base = ['earFront', 'earOut', 'earBack'].map(k => V[k]);
  const c = [0, 1, 2].map(j => (base[0][j] + base[1][j] + base[2][j]) / 3);
  for (const k of ['earFront', 'earOut', 'earBack', 'earTip']) V[k] = V[k].map((v, j) => c[j] + (v - c[j]) * ear.scale + ear.offset[j]);
  if (ear.tip) V.earTip = ear.tip;
  const bc = [0, 1, 2].map(j => (V.earFront[j] + V.earOut[j] + V.earBack[j]) / 3);
  V.earInner = bc.map((v, j) => v + (V.earTip[j] - v) * 0.35 + (j === 2 ? -0.1 : 0));   // dent in the front of the ear

  const colorOf = (region, centre) => {
    const custom = o.faceColor && o.faceColor(region, centre);
    if (custom) return custom;
    let r = region;
    while (!o.colors[r] && FALLBACK[r]) r = FALLBACK[r];
    return o.colors[r] || o.colors.top;
  };
  const mirror = p => [-p[0], p[1], p[2]];
  const tris = [];
  for (const [a, b, cc, region] of BASE_F) {
    if (!ear.show && region.startsWith('ear')) continue;
    const centre = [0, 1, 2].map(j => (V[a][j] + V[b][j] + V[cc][j]) / 3);
    const color = colorOf(region, centre);
    tris.push({ a: V[a], b: V[b], c: V[cc], color });
    tris.push({ a: mirror(V[a]), b: mirror(V[cc]), c: mirror(V[b]), color });
  }
  return trisToGeometry(tris);
}

/* -----------------------------------------------------------------
   Small extra-part helpers
   ----------------------------------------------------------------- */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.8 });
const shellMat = solidMat.clone(); shellMat.side = THREE.DoubleSide;   // open or flat parts

function mesh(geometry, material = solidMat) {
  const m = new THREE.Mesh(geometry, material);
  m.castShadow = true;
  return m;
}

// a left + right pair of the same part; the left one is mirrored with scale.x = -1
function addPair(parent, geometry, [x, y, z], rotation = [0, 0, 0], material = shellMat) {
  for (const side of [1, -1]) {
    const holder = new THREE.Group();
    const m = mesh(geometry, material);
    m.rotation.set(...rotation);
    holder.add(m);
    holder.position.set(x * side, y, z);
    if (side < 0) holder.scale.x = -1;
    parent.add(holder);
  }
}

// a row of little upright spikes along the top of a loft (crocodile scutes)
function ridge(sections, count, height, color, rows = [-0.12, 0.12]) {
  const tris = [];
  const at = u => {
    const f = u * (sections.length - 1), i = Math.min(Math.floor(f), sections.length - 2), k = f - i;
    return [0, 1, 2, 3].map(j => sections[i][j] + (sections[i + 1][j] - sections[i][j]) * k);
  };
  for (let j = 0; j < count; j++) {
    const [y, z, rx, ry] = at((j + 0.5) / count);
    for (const off of rows) {
      const x = off * rx * 4;
      tris.push({ a: [x, y + ry * 0.85, z + 0.13], b: [x, y + ry * 0.85, z - 0.13], c: [x, y + ry + height, z - 0.04], color });
    }
  }
  return trisToGeometry(tris);
}

// flat round eye (disc facing +x): coloured ring and dark pupil
function eyeDisc(iris, radius = 0.12, pupil = [0.04, 0.04, 0.04]) {
  const tris = [], N = 6;
  const ring = (r, x) => [...Array(N).keys()].map(k => [x, Math.cos(2 * Math.PI * k / N) * r, Math.sin(2 * Math.PI * k / N) * r]);
  const outer = ring(radius, 0), inner = ring(radius * 0.5, 0.02);
  for (let k = 0; k < N; k++) {
    const k2 = (k + 1) % N;
    tris.push({ a: outer[k], b: outer[k2], c: inner[k2], color: iris });
    tris.push({ a: outer[k], b: inner[k2], c: inner[k], color: iris });
    tris.push({ a: [0.03, 0, 0], b: inner[k], c: inner[k2], color: pupil });
  }
  return trisToGeometry(tris);
}

/* -----------------------------------------------------------------
   THE ANIMAL BUILDER — turns a spec into a THREE.Group
   ----------------------------------------------------------------- */
function makeAnimal(spec) {
  const g = new THREE.Group();
  g.name = spec.name;

  const body = mesh(loft(spec.body.sections, spec.body.opts));
  body.name = 'body';
  g.add(body);

  // most animals use the stretched wolf head; an animal can supply its own head builder instead
  const head = spec.head.build ? spec.head.build() : mesh(buildHead(spec.head), shellMat);
  head.name = 'head';
  head.position.set(...spec.head.pos);
  head.scale.setScalar(spec.head.scale);
  if (spec.head.extras) spec.head.extras(head);
  g.add(head);

  const legs = [];
  for (const [key, which] of [['frontLeg', 'front'], ['backLeg', 'back']]) {
    const L = spec[key];
    const geo = loft(L.sections, L.opts);
    for (const side of [1, -1]) {
      const m = mesh(geo);
      m.name = `${which}Leg${side > 0 ? 'R' : 'L'}`;
      m.position.set(L.pos[0] * side, L.pos[1], L.pos[2]);
      m.rotation.z = (L.splay || 0) * side;      // crocodile legs point outward
      if (L.scale) m.scale.setScalar(L.scale);
      g.add(m); legs.push(m);
    }
  }

  const tail = mesh(loft(spec.tail.sections, spec.tail.opts));
  tail.name = 'tail';
  tail.position.set(...spec.tail.pos);
  if (spec.tail.extras) spec.tail.extras(tail);
  g.add(tail);

  if (spec.extras) spec.extras(g);

  // stand on the ground
  const box = new THREE.Box3().setFromObject(g);
  g.position.y = -box.min.y;
  return { group: g, head, tail, legs, spec };
}

/* =================================================================
   THE ANIMALS — each one is only data + a few colour rules
   head.pos / head.scale place the head over the neck.
   ================================================================= */
const SPECS = {};

/* ---------------- CROCODILE ---------------- */
{
  const C = { green: [0.33, 0.42, 0.22], dark: [0.19, 0.26, 0.13], belly: [0.80, 0.76, 0.52],
              eye: [0.92, 0.80, 0.20], tooth: [0.97, 0.96, 0.88] };
  const tailSections = [[0, 0, 0.50, 0.40], [0, -1.2, 0.42, 0.35], [-0.05, -2.4, 0.32, 0.30],
                        [-0.10, -3.6, 0.22, 0.22], [-0.12, -4.6, 0.13, 0.14]];
  SPECS.crocodile = {
    name: 'crocodile', label: 'Crocodile',
    head: {
      w: 0.75, h: 0.5, snout: 2.3, snoutW: 2.6, snoutDrop: 0.05, ear: { show: false },
      colors: { top: C.green, snoutTop: C.dark, muzzle: C.green, chin: C.belly, throat: C.belly,
                cheekLow: C.belly, nose: C.dark, eye: C.green },
      pos: [0, 0.40, 3.20], scale: 0.7,
      extras(head) {
        // teeth sticking down along both sides of the long snout
        const tris = [];
        for (let z = 1.6; z < 4.0; z += 0.26)
          for (const s of [1, -1]) {
            const x = s * (0.60 - (z - 1.35) * 0.045);
            tris.push({ a: [x, -0.06, z - 0.06], b: [x, -0.06, z + 0.06], c: [x * 0.98, -0.24, z], color: C.tooth });
          }
        head.add(mesh(trisToGeometry(tris), shellMat));
        // eyes on top of the head, on little bumps
        const bump = loft([[0, -0.15, 0.14, 0.10], [0, 0.15, 0.14, 0.10]],
                          { startCap: [0, -0.28], endCap: [0, 0.28], colorFor: () => C.green });
        addPair(head, bump, [0.34, 0.78, 0.45], [0, 0, 0], solidMat);
        addPair(head, eyeDisc(C.eye, 0.1), [0.45, 0.84, 0.47], [0, -0.3, 0.6]);
      },
    },
    body: {
      sections: [[0.45, -3.0, 0.55, 0.40], [0.50, -2.2, 1.05, 0.60], [0.50, -0.8, 1.20, 0.65],
                 [0.50, 0.6, 1.15, 0.62], [0.50, 1.8, 0.90, 0.52], [0.48, 2.5, 0.70, 0.45]],
      opts: { startCap: 'flat', endCap: 'flat',
              colorFor: (i, c, r) => c.y < r.y - 0.3 ? C.belly : (c.y > r.y + 0.3 && i % 2 ? C.dark : C.green) },
    },
    frontLeg: {
      sections: [[0, 0, 0.28, 0.30], [-0.45, 0.05, 0.22, 0.24], [-0.80, 0.12, 0.18, 0.20],
                 [-0.95, 0.30, 0.26, 0.10], [-0.98, 0.50, 0.30, 0.06]],
      opts: { startCap: 'flat', endCap: [-1.0, 0.65], colorFor: i => (i >= 3 ? C.dark : C.green) },
      pos: [0.95, 0.45, 1.6], splay: 0.5,
    },
    backLeg: {
      sections: [[0, 0, 0.30, 0.32], [-0.45, 0.05, 0.24, 0.26], [-0.80, 0.12, 0.19, 0.21],
                 [-0.95, 0.30, 0.28, 0.10], [-0.98, 0.55, 0.32, 0.06]],
      opts: { startCap: 'flat', endCap: [-1.0, 0.72], colorFor: i => (i >= 3 ? C.dark : C.green) },
      pos: [1.0, 0.45, -1.9], splay: 0.5, scale: 1.1,
    },
    tail: {
      sections: tailSections,
      opts: { startCap: 'flat', endCap: [-0.14, -5.4],
              colorFor: (i, c, r) => c.y < r.y - 0.2 ? C.belly : (Math.floor(-c.z * 1.6) % 2 ? C.dark : C.green) },
      pos: [0, 0.45, -2.95],
      extras: tail => tail.add(mesh(ridge(tailSections, 16, 0.18, C.dark), shellMat)),
    },
    extras(g) {   // two rows of scutes along the back
      g.add(mesh(ridge([[0.5, -2.9, 0.3, 0.6], [0.5, 1.6, 0.3, 0.5]], 14, 0.12, C.dark, [-0.25, 0.25]), shellMat));
    },
    anim: { tailAxis: 'y', tailAmount: 0.3, tailSpeed: 1.2, headAmount: 0.06 },
  };
}

/* ---------------- OX ---------------- */
{
  const C = { brown: [0.42, 0.26, 0.15], dark: [0.20, 0.13, 0.08], muzzle: [0.80, 0.70, 0.60],
              nose: [0.55, 0.42, 0.42], horn: [0.90, 0.85, 0.72], hornTip: [0.30, 0.27, 0.22] };
  const hoof = n => (i => (i >= n ? C.dark : C.brown));
  SPECS.ox = {
    name: 'ox', label: 'Ox',
    head: {
      w: 1.1, h: 1.0, snout: 0.75, snoutW: 1.6, snoutDrop: -0.3,
      ear: { scale: 0.9, tip: [1.75, 1.0, -0.4] },
      colors: { top: C.brown, muzzle: C.muzzle, snoutTop: C.brown, nose: C.nose, chin: C.muzzle,
                throat: C.brown, eye: C.dark, earIn: [0.72, 0.56, 0.46] },
      pos: [0, 0.30, 4.10], scale: 0.95,
      extras(head) {
        // horns: out to the side, then curving up (drawn along z, turned to point along x)
        const L = 1.9;   // horn length  (1 = the old small horns)
        const T = 1.6;   // horn thickness
        const horn = loft([
          [0.00 * L, 0.00 * L, 0.16 * T, 0.16 * T],
          [0.03 * L, 0.35 * L, 0.14 * T, 0.14 * T],
          [0.10 * L, 0.70 * L, 0.12 * T, 0.12 * T],
          [0.28 * L, 1.00 * L, 0.10 * T, 0.10 * T],
          [0.55 * L, 1.22 * L, 0.07 * T, 0.07 * T],
        ], { startCap: 'flat', endCap: [0.85 * L, 1.32 * L], colorFor: i => (i >= 3 ? C.hornTip : C.horn) });
        addPair(head, horn, [0.50, 1.45, -0.20], [0, Math.PI / 2, 0]);
      },
    },
    body: {
      sections: [[0.40, -2.8, 0.60, 0.60], [0.50, -2.3, 1.25, 1.25], [0.55, -1.0, 1.30, 1.30],
                 [0.50, 0.4, 1.40, 1.45], [0.80, 1.6, 1.35, 1.50], [0.70, 2.5, 1.00, 1.10], [0.50, 3.1, 0.80, 0.85]],
      opts: { startCap: [0.45, -3.1], endCap: 'flat', colorFor: () => C.brown },
    },
    frontLeg: {
      sections: [[0, 0, 0.45, 0.60], [-0.9, -0.05, 0.34, 0.45], [-1.7, 0.05, 0.22, 0.26], [-2.4, 0.05, 0.20, 0.22],
                 [-2.65, 0.10, 0.22, 0.23], [-2.85, 0.18, 0.25, 0.25], [-3.0, 0.22, 0.26, 0.20]],
      opts: { startCap: 'flat', endCap: 'flat', colorFor: hoof(5) }, pos: [0.75, 0.2, 1.6],
    },
    backLeg: {
      sections: [[0, 0, 0.55, 0.85], [-0.7, 0.10, 0.45, 0.65], [-1.3, 0.20, 0.28, 0.32], [-1.8, -0.10, 0.22, 0.26],
                 [-2.2, -0.30, 0.20, 0.24], [-2.65, -0.20, 0.19, 0.22], [-2.85, -0.10, 0.24, 0.25], [-3.0, -0.05, 0.26, 0.20]],
      opts: { startCap: 'flat', endCap: 'flat', colorFor: hoof(6) }, pos: [0.8, 0.2, -2.1],
    },
    tail: {
      sections: [[0, 0, 0.12, 0.12], [-0.4, -0.30, 0.08, 0.08], [-1.2, -0.40, 0.07, 0.07], [-2.0, -0.40, 0.06, 0.06],
                 [-2.3, -0.38, 0.14, 0.14], [-2.6, -0.35, 0.15, 0.15]],
      opts: { startCap: 'flat', endCap: [-2.85, -0.33], colorFor: i => (i >= 3 ? C.dark : C.brown) },
      pos: [0, 1.3, -2.9],
    },
    anim: { tailAxis: 'y', tailAmount: 0.25, tailSpeed: 2, headAmount: 0.12 },
  };
}

/* ---------------- ANTELOPE (gazelle) ---------------- */
{
  const C = { tan: [0.80, 0.56, 0.30], white: [0.95, 0.93, 0.88], dark: [0.24, 0.16, 0.10],
              horn: [0.18, 0.16, 0.14], ring: [0.34, 0.31, 0.27], black: [0.08, 0.07, 0.07] };
  SPECS.antelope = {
    name: 'antelope', label: 'Antelope',
    head: {
      w: 0.72, h: 0.8, snout: 1.1, snoutW: 0.8, snoutDrop: -0.1,
      ear: { scale: 1.1, tip: [1.25, 2.0, -0.55] },
      colors: { top: C.tan, brow: C.white, mask: C.dark, cheekLow: C.white, muzzle: C.tan, chin: C.white,
                throat: C.white, nose: C.black, eye: C.black, earIn: C.white },
      pos: [0, 2.53, 3.50], scale: 0.72,
      extras(head) {
        // long ringed horns sweeping up and back
        const secs = [];
        for (let j = 0; j <= 8; j++) {
          const u = j / 8;
          secs.push([u * 2.4, -0.7 * Math.sin(u * 2.2), 0.12 - u * 0.08, 0.12 - u * 0.08]);
        }
        const horn = loft(secs, { startCap: 'flat', endCap: [2.65, -0.45], colorFor: i => (i % 2 ? C.ring : C.horn) });
        addPair(head, horn, [0.28, 1.45, 0.15], [0, 0, -0.18]);
      },
    },
    body: {
      sections: [[0.35, -2.2, 0.40, 0.40], [0.40, -1.75, 0.72, 0.78], [0.45, -0.65, 0.66, 0.66], [0.35, 0.45, 0.72, 0.85],
                 [0.50, 1.45, 0.62, 0.80], [1.20, 2.10, 0.42, 0.46], [2.00, 2.50, 0.33, 0.37], [2.60, 2.75, 0.30, 0.34]],
      opts: {
        startCap: [0.40, -2.45], endCap: 'flat',
        colorFor: (i, c, r) => {
          const d = c.y - r.y;
          if (i >= 1 && i <= 3 && d < -0.45) return C.white;                 // belly
          if (i >= 1 && i <= 3 && d < -0.22) return C.dark;                  // dark side stripe
          if (i >= 5 && c.z > r.z + 0.15) return C.white;                    // throat
          return C.tan;
        },
      },
    },
    frontLeg: {
      sections: [[0, 0, 0.28, 0.44], [-0.8, -0.05, 0.20, 0.30], [-1.55, 0.05, 0.12, 0.15], [-1.9, 0.05, 0.10, 0.12],
                 [-3.1, 0.10, 0.075, 0.09], [-3.35, 0.12, 0.10, 0.10], [-3.6, 0.18, 0.11, 0.12], [-3.72, 0.24, 0.12, 0.08]],
      opts: { startCap: 'flat', endCap: [-3.75, 0.30], colorFor: i => (i >= 6 ? C.black : C.tan) }, pos: [0.38, 0.2, 1.15],
    },
    backLeg: {
      sections: [[0, 0, 0.36, 0.62], [-0.6, 0.13, 0.30, 0.48], [-1.1, 0.18, 0.17, 0.22], [-1.6, -0.15, 0.13, 0.17],
                 [-2.05, -0.42, 0.10, 0.14], [-3.0, -0.30, 0.075, 0.09], [-3.3, -0.20, 0.10, 0.10], [-3.55, -0.12, 0.11, 0.12],
                 [-3.72, -0.06, 0.12, 0.08]],
      opts: { startCap: 'flat', endCap: [-3.75, 0], colorFor: i => (i >= 7 ? C.black : C.tan) }, pos: [0.40, 0.2, -1.65],
    },
    tail: {
      sections: [[0, 0, 0.10, 0.08], [-0.15, -0.30, 0.09, 0.07], [-0.35, -0.50, 0.06, 0.05]],
      opts: { startCap: 'flat', endCap: [-0.5, -0.6], colorFor: () => C.black }, pos: [0, 0.8, -2.25],
    },
    anim: { tailAxis: 'y', tailAmount: 0.5, tailSpeed: 6, headAmount: 0.2 },
  };
}

/* ---------------- TIGER ---------------- */
{
  const C = { orange: [0.93, 0.52, 0.13], black: [0.10, 0.08, 0.07], white: [0.96, 0.94, 0.88],
              nose: [0.82, 0.52, 0.47], eye: [0.86, 0.70, 0.20] };
  const striped = (v, base) => (stripe(v, 0.72) ? C.black : base);   // narrow black bands
  SPECS.tiger = {
    name: 'tiger', label: 'Tiger',
    head: {
      w: 1.05, h: 1.0, snout: 0.55, snoutW: 1.45, snoutDrop: -0.15,
      ear: { scale: 0.75, tip: [0.95, 1.95, -0.05] },
      colors: { top: C.orange, brow: C.white, muzzle: C.white, snoutTop: C.orange, chin: C.white,
                cheekLow: C.white, nose: C.nose, eye: C.eye, earIn: C.white, earBack: C.black },
      // stripes on the forehead and the sides of the head
      faceColor: (region, c) => (region === 'top' || region === 'side') && stripe(c[0] * 7 + c[2] * 2, 0.7) ? C.black : null,
      pos: [0, 1.25, 3.64], scale: 0.85,
    },
    body: {
      sections: [[0.40, -2.6, 0.55, 0.55], [0.50, -2.05, 0.92, 0.98], [0.55, -0.8, 0.85, 0.88], [0.45, 0.5, 0.95, 1.05],
                 [0.60, 1.6, 0.92, 1.08], [1.10, 2.35, 0.70, 0.78], [1.50, 2.75, 0.60, 0.66]],
      opts: {
        startCap: [0.40, -2.85], endCap: 'flat',
        colorFor: (i, c, r) => (c.y < r.y - 0.55 ? C.white : striped(c.z * 3.2 + c.y * 0.8, C.orange)),
      },
    },
    frontLeg: {
      sections: [[0, 0, 0.40, 0.58], [-0.9, -0.05, 0.30, 0.43], [-1.7, 0.05, 0.22, 0.26], [-2.5, 0.05, 0.20, 0.23],
                 [-2.75, 0.12, 0.24, 0.24], [-2.9, 0.30, 0.31, 0.19], [-2.95, 0.55, 0.31, 0.14]],
      opts: { startCap: 'flat', endCap: [-2.97, 0.72], colorFor: (i, c) => (i < 4 ? striped(c.y * 5, C.orange) : C.orange) },
      pos: [0.56, 0.2, 1.4],
    },
    backLeg: {
      sections: [[0, 0, 0.50, 0.82], [-0.6, 0.15, 0.42, 0.68], [-1.1, 0.30, 0.27, 0.32], [-1.6, 0, 0.21, 0.25],
                 [-2.05, -0.32, 0.18, 0.22], [-2.55, -0.20, 0.17, 0.20], [-2.78, -0.05, 0.23, 0.23],
                 [-2.9, 0.15, 0.31, 0.19], [-2.95, 0.42, 0.31, 0.14]],
      opts: { startCap: 'flat', endCap: [-2.97, 0.62], colorFor: (i, c) => (i < 6 ? striped(c.y * 5, C.orange) : C.orange) },
      pos: [0.58, 0.2, -2.0],
    },
    tail: {
      sections: [[0, 0, 0.16, 0.16], [0.05, -0.5, 0.14, 0.14], [-0.35, -1.0, 0.13, 0.13], [-0.95, -1.35, 0.12, 0.12],
                 [-1.6, -1.5, 0.11, 0.11], [-2.1, -1.35, 0.10, 0.10], [-2.45, -1.05, 0.09, 0.09]],
      opts: { startCap: 'flat', endCap: [-2.65, -0.85], colorFor: i => (i >= 5 || i % 2 ? C.black : C.orange) },
      pos: [0, 0.9, -2.7],
    },
    anim: { tailAxis: 'y', tailAmount: 0.35, tailSpeed: 1.6, headAmount: 0.2 },
  };
}

/* ---------------- COYOTE ---------------- */
{
  const C = { fur: [0.70, 0.58, 0.42], grey: [0.55, 0.50, 0.45], cream: [0.93, 0.88, 0.78],
              black: [0.10, 0.09, 0.08], eye: [0.85, 0.65, 0.20] };
  SPECS.coyote = {
    name: 'coyote', label: 'Coyote',
    head: {
      w: 0.85, h: 0.9, snout: 1.1, snoutW: 0.85,
      ear: { scale: 1.25, tip: [1.15, 2.85, -0.25] },
      colors: { top: C.grey, side: C.fur, snoutTop: C.fur, muzzle: C.cream, cheekLow: C.cream, chin: C.cream,
                nose: C.black, eye: C.eye, earIn: C.cream, earBack: C.fur },
      pos: [0, 1.26, 3.66], scale: 0.72,
    },
    body: {
      sections: [[0.25, -2.5, 0.45, 0.45], [0.30, -1.95, 0.82, 0.88], [0.45, -0.8, 0.70, 0.72], [0.25, 0.55, 0.85, 1.0],
                 [0.35, 1.7, 0.80, 0.98], [0.85, 2.5, 0.55, 0.60], [1.45, 2.9, 0.45, 0.50]],
      opts: { startCap: [0.25, -2.75], endCap: 'flat',
              colorFor: (i, c, r) => (c.y < r.y - 0.5 ? C.cream : c.y > r.y + 0.5 ? C.grey : C.fur) },
    },
    frontLeg: {
      sections: [[0, 0, 0.32, 0.48], [-0.9, -0.05, 0.24, 0.34], [-1.6, 0.05, 0.16, 0.19], [-2.5, 0.05, 0.14, 0.16],
                 [-2.78, 0.12, 0.18, 0.19], [-2.96, 0.30, 0.26, 0.18], [-3.05, 0.55, 0.27, 0.13]],
      opts: { startCap: 'flat', endCap: [-3.08, 0.75], colorFor: () => C.fur }, pos: [0.5, 0.2, 1.45],
    },
    backLeg: {
      sections: [[0, 0, 0.40, 0.66], [-0.55, 0.15, 0.35, 0.55], [-1.05, 0.30, 0.22, 0.28], [-1.55, 0, 0.17, 0.20],
                 [-1.95, -0.30, 0.14, 0.18], [-2.5, -0.18, 0.13, 0.15], [-2.8, -0.05, 0.18, 0.19],
                 [-2.96, 0.15, 0.26, 0.18], [-3.05, 0.43, 0.27, 0.13]],
      opts: { startCap: 'flat', endCap: [-3.08, 0.65], colorFor: () => C.fur }, pos: [0.5, 0.2, -1.85],
    },
    tail: {   // bushy tail hanging low, black tip
      sections: [[0, 0, 0.18, 0.18], [-0.2, -0.5, 0.30, 0.28], [-0.7, -0.95, 0.36, 0.34], [-1.3, -1.2, 0.34, 0.32], [-1.85, -1.3, 0.25, 0.24]],
      opts: { startCap: 'flat', endCap: [-2.25, -1.3], colorFor: i => (i >= 3 ? C.black : C.fur) },
      pos: [0, 0.8, -2.55],
    },
    anim: { tailAxis: 'y', tailAmount: 0.3, tailSpeed: 3, headAmount: 0.22 },
  };
}

/* ---------------- RACCOON ---------------- */
{
  const C = { grey: [0.55, 0.54, 0.52], light: [0.82, 0.80, 0.76], dark: [0.13, 0.12, 0.12], white: [0.95, 0.94, 0.92] };
  const tail = [];
  for (let j = 0; j <= 8; j++) { const u = j / 8; tail.push([0.05 * Math.sin(u * 3) - u * 0.55, -u * 2.3, 0.2 + Math.sin(u * Math.PI) * 0.15, 0.2 + Math.sin(u * Math.PI) * 0.15]); }
  SPECS.raccoon = {
    name: 'raccoon', label: 'Raccoon',
    head: {
      w: 1.0, h: 0.9, snout: 0.8, snoutW: 0.7, snoutDrop: -0.05,
      ear: { scale: 0.75, tip: [1.0, 2.0, -0.2] },
      colors: { top: C.grey, brow: C.white, mask: C.dark, cheek: C.light, cheekLow: C.dark, bridge: C.dark,
                snoutTop: C.dark, muzzle: C.white, chin: C.white, throat: C.light, nose: C.dark,
                eye: [0.02, 0.02, 0.02], earIn: C.white, earBack: C.dark },
      pos: [0, 0.48, 2.5], scale: 0.62,
    },
    body: {
      sections: [[0.50, -1.9, 0.50, 0.50], [0.60, -1.45, 0.95, 0.95], [0.65, -0.5, 1.00, 1.00],
                 [0.50, 0.5, 0.92, 0.95], [0.45, 1.3, 0.78, 0.82], [0.60, 1.85, 0.55, 0.58]],
      opts: { startCap: [0.5, -2.1], endCap: 'flat', colorFor: (i, c, r) => (c.y < r.y - 0.5 ? C.light : C.grey) },
    },
    frontLeg: {
      sections: [[0, 0, 0.30, 0.40], [-0.6, 0, 0.22, 0.28], [-1.1, 0.05, 0.16, 0.18], [-1.35, 0.12, 0.18, 0.18],
                 [-1.5, 0.30, 0.22, 0.12], [-1.53, 0.50, 0.22, 0.09]],
      opts: { startCap: 'flat', endCap: [-1.55, 0.62], colorFor: i => (i >= 2 ? C.dark : C.grey) }, pos: [0.55, 0.2, 1.2],
    },
    backLeg: {
      sections: [[0, 0, 0.38, 0.60], [-0.5, 0.10, 0.30, 0.45], [-0.95, 0.15, 0.20, 0.24], [-1.25, 0, 0.17, 0.19],
                 [-1.45, 0.15, 0.22, 0.14], [-1.53, 0.45, 0.22, 0.09]],
      opts: { startCap: 'flat', endCap: [-1.55, 0.62], colorFor: i => (i >= 3 ? C.dark : C.grey) }, pos: [0.6, 0.3, -1.3],
    },
    tail: {   // bushy tail with dark rings
      sections: tail,
      opts: { startCap: 'flat', endCap: [-0.62, -2.5], colorFor: i => (i % 2 || i >= 7 ? C.dark : C.light) },
      pos: [0, 0.75, -1.95],
    },
    anim: { tailAxis: 'y', tailAmount: 0.3, tailSpeed: 2.5, headAmount: 0.25 },
  };
}

/* ---------------- MONKEY (walking on all fours, like a macaque or capuchin) ----------------
   Primates are built differently from dogs and cats, so the monkey does NOT use the
   stretched wolf head. It gets its own head: a round skull, a flat face, eyes that
   look forward, a brow ridge, a short muzzle and round ears low on the sides.
   Its arms bend backward at the elbow and it walks on flat palms and soles.       */
{
  const C = { fur: [0.45, 0.30, 0.18], dark: [0.33, 0.21, 0.12], light: [0.68, 0.53, 0.37],
              face: [0.93, 0.74, 0.60], nose: [0.70, 0.48, 0.40], mouth: [0.40, 0.22, 0.18],
              eye: [0.55, 0.36, 0.16] };

  function buildMonkeyHead() {
    const head = new THREE.Group();

    // 1) round skull: a loft along z that ends in a flat face
    const skull = loft([
      [ 0.10, -0.75, 0.30, 0.30],
      [ 0.05, -0.50, 0.62, 0.62],
      [ 0.00, -0.05, 0.75, 0.72],   // widest (the braincase is large)
      [ 0.00,  0.35, 0.72, 0.70],
      [-0.05,  0.60, 0.60, 0.60],   // flat face plane
    ], {
      sides: 8, startCap: [0.10, -0.85], endCap: 'flat',
      colorFor: (i, c) =>
        c.z > 0.45 && c.y < 0.38 ? C.face :                 // bare face (mask shape)
        c.z > 0.2 && c.y < -0.25 ? C.face : C.fur,          // bare cheeks under the face
    });
    head.add(mesh(skull, shellMat));

    // 2) short muzzle sticking out below the eyes
    const muzzle = loft([
      [-0.30, 0.45, 0.32, 0.26],
      [-0.33, 0.75, 0.28, 0.22],
      [-0.35, 0.90, 0.22, 0.17],
    ], { startCap: 'flat', endCap: 'flat',
         colorFor: (i, c, r) => (i === 1 && c.z > 0.82 && c.y > r.y ? C.nose : c.y < r.y - 0.12 && c.z > 0.8 ? C.mouth : C.face) });
    head.add(mesh(muzzle, shellMat));

    // 3) brow ridge: a slanted band of fur above the eyes
    const B = { l: [0.50, 0.30, 0.52], m: [0, 0.26, 0.70], lt: [0.50, 0.40, 0.46], mt: [0, 0.37, 0.64] };
    const brow = [];
    for (const s of [1, -1]) {
      const f = p => [p[0] * s, p[1], p[2]];
      brow.push({ a: f(B.l), b: f(B.m), c: f(B.mt), color: C.dark });
      brow.push({ a: f(B.l), b: f(B.mt), c: f(B.lt), color: C.dark });
    }
    head.add(mesh(trisToGeometry(brow), shellMat));

    // 4) eyes close together, facing FORWARD (a disc facing +x turned to face +z)
    addPair(head, eyeDisc(C.eye, 0.11), [0.24, 0.12, 0.66], [0, -Math.PI / 2, 0]);

    // 5) round flat ears low on the sides, at eye level
    addPair(head, eyeDisc(C.dark, 0.24, C.face), [0.74, 0.05, -0.10], [0, -0.35, 0]);
    return head;
  }

  SPECS.monkey = {
    name: 'monkey', label: 'Monkey',
    head: { build: buildMonkeyHead, pos: [0, 1.30, 2.45], scale: 0.9 },
    body: {   // deep chest, narrow waist, broad shoulders, flat back
      sections: [[0.45, -1.7, 0.45, 0.45], [0.55, -1.3, 0.70, 0.72], [0.60, -0.4, 0.60, 0.62],
                 [0.62, 0.6, 0.75, 0.82], [0.75, 1.4, 0.78, 0.78], [0.95, 1.9, 0.42, 0.45]],
      opts: { startCap: [0.45, -1.9], endCap: 'flat', colorFor: (i, c, r) => (c.y < r.y - 0.5 ? C.light : C.fur) },
    },
    frontLeg: {   // arm: elbow bends BACKWARD, hand lies flat with fingers forward
      sections: [[0, 0, 0.20, 0.24], [-0.75, -0.18, 0.16, 0.18], [-1.45, 0.02, 0.13, 0.14],
                 [-1.62, 0.10, 0.15, 0.10], [-1.70, 0.30, 0.17, 0.05], [-1.71, 0.50, 0.15, 0.04]],
      opts: { startCap: 'flat', endCap: [-1.72, 0.62], colorFor: i => (i >= 3 ? C.face : C.fur) },
      pos: [0.62, 0.40, 1.35],        // shoulders at the SIDES of the chest
    },
    backLeg: {    // leg: knee forward, then the whole sole flat on the ground, long toes
      sections: [[0, 0, 0.30, 0.42], [-0.60, 0.30, 0.20, 0.24], [-1.25, 0.05, 0.14, 0.16], [-1.40, 0, 0.14, 0.12],
                 [-1.52, 0.30, 0.16, 0.06], [-1.53, 0.62, 0.16, 0.05], [-1.54, 0.85, 0.12, 0.04]],
      opts: { startCap: 'flat', endCap: [-1.55, 0.95], colorFor: i => (i >= 3 ? C.face : C.fur) },
      pos: [0.50, 0.40, -1.30], scale: 1.1,
    },
    tail: {   // long tail rising up and curling at the end
      sections: [[0, 0, 0.12, 0.12], [0.4, -0.4, 0.11, 0.11], [1.1, -0.7, 0.10, 0.10], [1.9, -0.8, 0.09, 0.09],
                 [2.6, -0.65, 0.08, 0.08], [3.05, -0.3, 0.07, 0.07], [3.15, 0.10, 0.06, 0.06], [2.95, 0.35, 0.05, 0.05]],
      opts: { startCap: 'flat', endCap: [2.75, 0.3], colorFor: () => C.fur }, pos: [0, 0.8, -1.75],
    },
    anim: { tailAxis: 'y', tailAmount: 0.4, tailSpeed: 1.5, headAmount: 0.35 },
  };
}

/* ---------------- PIG ---------------- */
{
  const C = { pink: [0.96, 0.72, 0.72], darkPink: [0.86, 0.56, 0.59], snout: [0.93, 0.60, 0.64],
              nostril: [0.50, 0.28, 0.33], eye: [0.12, 0.08, 0.08], hoof: [0.45, 0.35, 0.33] };
  const SNOUT = 0.55;   // how long the snout is (the wolf's is 1.0)
  SPECS.pig = {
    name: 'pig', label: 'Pig',
    head: {
      w: 1.0, h: 0.95, snout: SNOUT, snoutW: 1.6, snoutDrop: -0.2,
      ear: { scale: 1.3, tip: [1.15, 1.5, 0.85] },   // big floppy ears flopping forward
      colors: { top: C.pink, nose: C.snout, chin: C.pink, eye: C.eye, earIn: C.darkPink },
      pos: [0, 0.49, 2.69], scale: 0.75,
      extras(head) {
        // two nostrils on the flat end of the snout; the end sits at z = 1 + 1.45 × SNOUT
        const z = 1 + 1.45 * SNOUT + 0.04;
        const tris = [];
        for (const x of [0.13, -0.13])
          tris.push({ a: [x - 0.07, -0.27, z], b: [x + 0.07, -0.27, z], c: [x, -0.42, z + 0.01], color: C.nostril });
        head.add(mesh(trisToGeometry(tris), shellMat));
      },
    },
    body: {   // round barrel
      sections: [[0.50, -2.0, 0.55, 0.55], [0.55, -1.6, 1.05, 1.00], [0.55, -0.5, 1.20, 1.12], [0.50, 0.6, 1.15, 1.10],
                 [0.55, 1.4, 0.95, 0.95], [0.60, 1.9, 0.72, 0.72]],
      opts: { startCap: [0.5, -2.2], endCap: 'flat', colorFor: (i, c, r) => (c.y < r.y - 0.75 ? C.darkPink : C.pink) },
    },
    frontLeg: {
      sections: [[0, 0, 0.30, 0.36], [-0.6, 0, 0.22, 0.26], [-1.0, 0.02, 0.18, 0.20], [-1.2, 0.05, 0.19, 0.19], [-1.35, 0.08, 0.20, 0.16]],
      opts: { startCap: 'flat', endCap: 'flat', colorFor: i => (i >= 3 ? C.hoof : C.pink) }, pos: [0.6, 0.1, 1.15],
    },
    backLeg: {
      sections: [[0, 0, 0.40, 0.55], [-0.5, 0.08, 0.28, 0.35], [-0.9, -0.05, 0.19, 0.21], [-1.2, 0, 0.19, 0.19], [-1.35, 0.04, 0.20, 0.16]],
      opts: { startCap: 'flat', endCap: 'flat', colorFor: i => (i >= 3 ? C.hoof : C.pink) }, pos: [0.62, 0.1, -1.25],
    },
    tail: {   // little curly tail
      sections: [[0, 0, 0.06, 0.06], [0.15, -0.2, 0.05, 0.05], [0.35, -0.15, 0.05, 0.05], [0.40, 0, 0.045, 0.045],
                 [0.28, 0.08, 0.04, 0.04], [0.20, -0.02, 0.035, 0.035]],
      opts: { startCap: 'flat', endCap: [0.22, -0.1], colorFor: () => C.pink }, pos: [0, 0.9, -2.05],
    },
    anim: { tailAxis: 'z', tailAmount: 0.6, tailSpeed: 8, headAmount: 0.15 },
  };
}

/* ---------------- SQUIRREL ---------------- */
{
  const C = { red: [0.72, 0.38, 0.18], dark: [0.50, 0.26, 0.12], white: [0.95, 0.92, 0.86], eye: [0.04, 0.03, 0.03],
              nose: [0.30, 0.20, 0.18] };
  SPECS.squirrel = {
    name: 'squirrel', label: 'Squirrel',
    head: {
      w: 0.9, h: 1.0, snout: 0.7, snoutW: 0.8, snoutDrop: -0.1,
      ear: { scale: 0.8, tip: [0.8, 2.35, -0.1] },   // tall tufted ears
      colors: { top: C.red, brow: C.white, chin: C.white, throat: C.white, cheekLow: C.white, muzzle: C.red,
                nose: C.nose, eye: C.eye, earIn: C.white, earBack: C.dark },
      pos: [0, 0.94, 1.98], scale: 0.55,
    },
    body: {
      sections: [[0.50, -1.3, 0.50, 0.50], [0.60, -0.95, 0.85, 0.85], [0.60, -0.2, 0.72, 0.75], [0.60, 0.55, 0.65, 0.70],
                 [0.75, 1.1, 0.52, 0.55], [0.95, 1.4, 0.42, 0.45]],
      opts: { startCap: [0.5, -1.5], endCap: 'flat', colorFor: (i, c, r) => (c.y < r.y - 0.35 && c.z > -0.7 ? C.white : C.red) },
    },
    frontLeg: {
      sections: [[0, 0, 0.18, 0.22], [-0.4, 0.05, 0.13, 0.15], [-0.75, 0.10, 0.10, 0.11], [-0.9, 0.18, 0.12, 0.08], [-0.93, 0.32, 0.11, 0.05]],
      opts: { startCap: 'flat', endCap: [-0.94, 0.40], colorFor: () => C.red }, pos: [0.4, 0.3, 0.9],
    },
    backLeg: {   // big haunches and long feet
      sections: [[0, 0, 0.42, 0.60], [-0.35, 0.25, 0.33, 0.45], [-0.6, 0.35, 0.18, 0.20], [-0.8, 0.05, 0.12, 0.14],
                 [-0.95, 0.10, 0.14, 0.08], [-1.0, 0.50, 0.14, 0.05], [-1.01, 0.70, 0.12, 0.04]],
      opts: { startCap: 'flat', endCap: [-1.02, 0.8], colorFor: () => C.red }, pos: [0.52, 0.35, -0.8],
    },
    tail: {   // huge bushy tail curling up over the back
      sections: [[0, 0, 0.20, 0.20], [0.4, -0.3, 0.42, 0.40], [1.1, -0.5, 0.60, 0.55], [1.9, -0.45, 0.68, 0.62],
                 [2.6, -0.2, 0.66, 0.60], [3.0, 0.25, 0.52, 0.48], [3.1, 0.65, 0.34, 0.32]],
      opts: { startCap: 'flat', endCap: [2.95, 0.95], sides: 8,
              colorFor: (i, c) => (rand(c.x * 13 + c.y * 7 + c.z * 3) < 0.3 ? C.dark : C.red) },
      pos: [0, 0.6, -1.35],
    },
    anim: { tailAxis: 'y', tailAmount: 0.25, tailSpeed: 5, headAmount: 0.35 },
  };
}

/* =================================================================
   SCENE, LIGHTS, GROUND SHADOW
   ================================================================= */
const stage = document.getElementById('stage');
const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.setSize(stage.clientWidth, stage.clientHeight);
renderer.shadowMap.enabled = true;
renderer.shadowMap.type = THREE.PCFSoftShadowMap;
stage.appendChild(renderer.domElement);

const scene = new THREE.Scene();
scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
scene.add(key, key.target);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(80, 80), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 300);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.maxPolarAngle = Math.PI * 0.49;
controls.autoRotateSpeed = 2;

const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
const wireMat = new THREE.LineBasicMaterial({ color: 0x3a6ff0 });
let showEdges = false;

/* =================================================================
   SWITCHING ANIMALS — built the first time they are picked
   ================================================================= */
const built = {};
let current = null;
let fitRadius = 5;
const fitCenter = new THREE.Vector3();

const DIRS = {
  angle: new THREE.Vector3(0.55, 0.38, 0.72).normalize(),
  front: new THREE.Vector3(0, 0.15, 1).normalize(),
  side:  new THREE.Vector3(-1, 0.15, 0).normalize(),
  top:   new THREE.Vector3(0, 1, 0.001).normalize(),
};
let viewName = 'angle';
let target = new THREE.Vector3();
let moving = false;

function viewPosition(name) {
  return fitCenter.clone().addScaledVector(DIRS[name], fitRadius * 3.1);
}

function showAnimal(name) {
  if (current) scene.remove(current.group);
  if (!built[name]) {
    built[name] = makeAnimal(SPECS[name]);
    built[name].group.traverse(o => {      // edge lines (hidden until "Show edges")
      if (!o.isMesh) return;
      const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1), wireMat);
      l.visible = showEdges; o.add(l);
    });
  }
  current = built[name];
  current.group.traverse(o => { if (o.isLineSegments) o.visible = showEdges; });
  scene.add(current.group);

  // frame the camera and the shadow around this animal
  const sphere = new THREE.Box3().setFromObject(current.group).getBoundingSphere(new THREE.Sphere());
  fitCenter.copy(sphere.center);
  fitRadius = sphere.radius;
  controls.target.copy(fitCenter);
  controls.minDistance = fitRadius * 1.3;
  controls.maxDistance = fitRadius * 7;
  key.position.copy(fitCenter).add(new THREE.Vector3(0.6, 1.2, 0.8).multiplyScalar(fitRadius * 2));
  key.target.position.copy(fitCenter);
  Object.assign(key.shadow.camera, { left: -fitRadius * 1.3, right: fitRadius * 1.3, top: fitRadius * 1.3,
                                     bottom: -fitRadius * 1.3, near: 0.1, far: fitRadius * 8 });
  key.shadow.camera.updateProjectionMatrix();
  camera.position.copy(viewPosition(viewName));
  moving = false;

  let tris = 0;
  current.group.traverse(o => { if (o.isMesh) tris += o.geometry.attributes.position.count / 3; });
  document.getElementById('stats').textContent = `${SPECS[name].label}: ${tris} triangles. Drag to orbit, scroll or pinch to zoom.`;
  document.querySelectorAll('[data-animal]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.animal === name)));
}

/* =================================================================
   BUTTONS
   ================================================================= */
const picker = document.getElementById('picker');
for (const name in SPECS) {
  const b = document.createElement('button');
  b.textContent = SPECS[name].label;
  b.dataset.animal = name;
  b.setAttribute('aria-pressed', 'false');
  b.addEventListener('click', () => showAnimal(name));
  picker.appendChild(b);
}

document.querySelectorAll('[data-view]').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('[data-view]').forEach(b => b.setAttribute('aria-pressed', 'false'));
    btn.setAttribute('aria-pressed', 'true');
    viewName = btn.dataset.view;
    target = viewPosition(viewName);
    setSpin(false);
    if (reduceMotion) camera.position.copy(target); else moving = true;
  });
});
controls.addEventListener('start', () => { moving = false; });

const spinBtn = document.getElementById('spin');
function setSpin(on) { controls.autoRotate = on; spinBtn.setAttribute('aria-pressed', String(on)); }
spinBtn.addEventListener('click', () => setSpin(!controls.autoRotate));

const edgesBtn = document.getElementById('edges');
edgesBtn.addEventListener('click', () => {
  showEdges = !showEdges;
  current.group.traverse(o => { if (o.isLineSegments) o.visible = showEdges; });
  edgesBtn.setAttribute('aria-pressed', String(showEdges));
});

let animating = !reduceMotion;
const animBtn = document.getElementById('animate');
animBtn.setAttribute('aria-pressed', String(animating));
animBtn.addEventListener('click', () => { animating = !animating; animBtn.setAttribute('aria-pressed', String(animating)); });

window.addEventListener('resize', () => {
  camera.aspect = stage.clientWidth / stage.clientHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(stage.clientWidth, stage.clientHeight);
});

showAnimal('crocodile');

/* =================================================================
   RENDER LOOP — tail sway and head look-around for whichever animal is shown
   ================================================================= */
const clock = new THREE.Clock();
(function loop() {
  requestAnimationFrame(loop);
  const t = clock.getElapsedTime();
  if (current) {
    const a = current.spec.anim;
    const wag = animating ? Math.sin(t * a.tailSpeed) * a.tailAmount : 0;
    const axis = a.tailAxis;
    current.tail.rotation[axis] += (wag - current.tail.rotation[axis]) * 0.15;
    const look = animating ? Math.sin(t * 0.7) * a.headAmount : 0;
    current.head.rotation.y += (look - current.head.rotation.y) * 0.1;
  }
  if (moving) {
    camera.position.lerp(target, 0.1);
    if (camera.position.distanceTo(target) < 0.02) moving = false;
  }
  controls.update();
  renderer.render(scene, camera);
})();
</script>
</body>
</html>
```