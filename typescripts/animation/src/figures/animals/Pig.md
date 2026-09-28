# Pig

Draft for the user to correct: Claude wrote this title and the behaviours on 2026-09-26, building the pig from the shape reference below, which is the user's: their "9 new animal draft" (the low-poly animal park), split into one page per animal, with only the pig's part of it, its own title and heading, and showing the pig first. It has the other animals' behaviours, numbered as theirs are.

## Animation Behavior.
2. `Jump()`
3. `Hold(Figure figure)`
4. `Walk()`
5. `Run()`
6. `Speech(text string)`, its show Speech Bubble and show text, when text to long, its split to multiple Bubble that show and hide sequentially


# Shape Reference
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Low-poly pig — Three.js BufferGeometry</title>
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
  <h1>Low-poly pig</h1>
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

showAnimal('pig');

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