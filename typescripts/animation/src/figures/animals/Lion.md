# Lion

Draft for the user to correct: Claude wrote this title and the behaviours on 2026-09-26, building the lion from the shape reference below, which is the user's. It has the other animals' behaviours, numbered as theirs are.

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
<title>Low-poly lion — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #b7792e;
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
  .sep { width: 1px; background: var(--muted); opacity: .35; margin: 4px 4px; }
</style>
</head>
<body>
<div id="stage"></div>
<header>
  <h1>Low-poly lion</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="wag" aria-pressed="true">Swish tail</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates for every part: x = right, y = up, z = forward
   (the lion faces +z). All parts use the same units.
   ================================================================= */

const COLORS = {
  tawny:     [0.83, 0.62, 0.35],   // main golden fur
  cream:     [0.94, 0.85, 0.67],   // muzzle, chin, belly
  nose:      [0.36, 0.21, 0.18],
  eye:       [0.88, 0.62, 0.15],   // amber eyes
  earBack:   [0.40, 0.27, 0.16],
  maneDark:  [0.40, 0.22, 0.09],
  mane:      [0.55, 0.33, 0.14],
  maneLight: [0.68, 0.45, 0.20],
  tuft:      [0.30, 0.18, 0.09],   // dark tail tip
};

// repeatable "random" number from a seed (used to mix mane colours)
const rand = n => { const v = Math.sin(n * 91.7) * 43758.55; return v - Math.floor(v); };

/* -----------------------------------------------------------------
   HELPER 1: turn a list of triangles into a BufferGeometry.
   Each triangle: { a, b, c, color, ref }
   "ref" is a point inside the shape; it is used to flip the
   triangle so its front side always faces outward.
   ----------------------------------------------------------------- */
function trisToGeometry(tris) {
  const positions = [], colors = [];
  const A = new THREE.Vector3(), B = new THREE.Vector3(), C = new THREE.Vector3();
  const n = new THREE.Vector3(), mid = new THREE.Vector3(), tmp = new THREE.Vector3();

  for (const t of tris) {
    let a = t.a, b = t.b, c = t.c;
    if (t.ref) {
      A.fromArray(a); B.fromArray(b); C.fromArray(c);
      n.subVectors(B, A).cross(tmp.subVectors(C, A));
      mid.copy(A).add(B).add(C).divideScalar(3).sub(new THREE.Vector3().fromArray(t.ref));
      if (n.dot(mid) < 0) [b, c] = [c, b];       // wrong way round → swap
    }
    positions.push(...a, ...b, ...c);
    colors.push(...t.color, ...t.color, ...t.color);
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  g.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  g.computeVertexNormals();                      // non-indexed → flat facets
  return g;
}

/* -----------------------------------------------------------------
   HELPER 2: "loft" — builds a tube from rings placed along a path.
   sections: [y, z, radiusX, radiusY]  (the path lies in the y–z plane)
   caps: 'flat' closes with a flat lid, [y, z] closes to a point.
   colorFor(segmentIndex, triangleCentre, ringCentre) picks a colour
   for each triangle, so one tube can have several colours.
   ----------------------------------------------------------------- */
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => COLORS.tawny } = {}) {
  const X = new THREE.Vector3(1, 0, 0);
  const centers = sections.map(s => new THREE.Vector3(0, s[0], s[1]));
  const rings = sections.map((s, i) => {
    const prev = centers[Math.max(i - 1, 0)], next = centers[Math.min(i + 1, centers.length - 1)];
    const tangent = next.clone().sub(prev).normalize();
    const up = new THREE.Vector3().crossVectors(tangent, X).normalize();
    const ring = [];
    for (let k = 0; k < sides; k++) {
      const ang = Math.PI / 2 + (2 * Math.PI * k) / sides;
      ring.push(centers[i].clone()
        .addScaledVector(X, Math.cos(ang) * s[2])
        .addScaledVector(up, Math.sin(ang) * s[3]).toArray());
    }
    return ring;
  });

  const centreOf = (a, b, c) => new THREE.Vector3(
    (a[0] + b[0] + c[0]) / 3, (a[1] + b[1] + c[1]) / 3, (a[2] + b[2] + c[2]) / 3);

  const tris = [];
  for (let i = 0; i < rings.length - 1; i++) {
    const refV = centers[i].clone().add(centers[i + 1]).multiplyScalar(0.5);
    const ref = refV.toArray();
    for (let k = 0; k < sides; k++) {
      const a = rings[i][k], b = rings[i][(k + 1) % sides];
      const c = rings[i + 1][k], d = rings[i + 1][(k + 1) % sides];
      tris.push({ a, b, c: d, ref, color: colorFor(i, centreOf(a, b, d), refV) });
      tris.push({ a, b: d, c, ref, color: colorFor(i, centreOf(a, d, c), refV) });
    }
  }
  const addCap = (cap, ring, center, inner, segIndex) => {
    if (!cap) return;
    const tip = cap === 'flat' ? center.toArray() : [0, cap[0], cap[1]];
    for (let k = 0; k < sides; k++) {
      const a = ring[k], b = ring[(k + 1) % sides];
      tris.push({ a, b, c: tip, ref: inner.toArray(), color: colorFor(segIndex, centreOf(a, b, tip), center) });
    }
  };
  const last = rings.length - 1;
  addCap(startCap, rings[0], centers[0], centers[1], 0);
  addCap(endCap, rings[last], centers[last], centers[last - 1], last - 1);
  return trisToGeometry(tris);
}

/* =================================================================
   PART 1 — HEAD (right half + mirror)
   Like the cat, but bigger and broader, with a long wide muzzle
   and small rounded ears.
   ================================================================= */
function buildHead() {
  const V = {
    C_back: [0, 1.30, -0.90], C_forehead: [0, 1.50, 0.20], C_brow: [0, 0.95, 0.95],
    C_stop: [0, 0.70, 1.25], C_snoutTip: [0, 0.45, 1.95], C_noseBot: [0, 0.10, 2.02],
    C_chin: [0, -0.60, 1.55], C_throat: [0, -0.80, 0.20],
    topPlate: [0.50, 1.45, 0.20], temple: [0.95, 1.10, 0.30], brow: [0.42, 0.95, 0.90],
    browOuter: [0.92, 0.90, 0.60],
    eyeIn: [0.32, 0.72, 1.10], eyeOut: [0.72, 0.78, 0.85], eyeLow: [0.50, 0.50, 1.02],
    eyeDeep: [0.50, 0.70, 0.85],
    cheek: [1.20, 0.40, 0.10], cheekLow: [1.00, -0.35, 0.60], jaw: [0.50, -0.50, 1.35],
    back: [0.85, 0.95, -0.80], backLow: [1.00, -0.30, -0.50],
    snoutBase: [0.42, 0.55, 1.30], snoutTip: [0.38, 0.35, 1.90], noseBot: [0.28, 0.00, 1.98],
    snoutBot: [0.35, -0.40, 1.60],
    earFront: [0.55, 1.40, 0.20], earOut: [1.00, 1.20, 0.10], earBack: [0.78, 1.35, -0.25],
    earTip: [0.95, 1.85, 0.00], earInner: [0.80, 1.50, 0.05],
  };
  const F = [
    // forehead
    ['C_back','C_forehead','topPlate','tawny'], ['C_back','topPlate','back','tawny'],
    ['C_forehead','brow','topPlate','tawny'], ['C_forehead','C_brow','brow','tawny'],
    ['topPlate','brow','temple','tawny'], ['temple','brow','browOuter','tawny'],
    ['topPlate','temple','back','tawny'], ['back','temple','cheek','tawny'],
    ['temple','browOuter','cheek','tawny'],
    // around the eye
    ['brow','eyeOut','browOuter','tawny'], ['brow','eyeIn','eyeOut','tawny'],
    ['C_brow','eyeIn','brow','tawny'], ['browOuter','eyeOut','cheek','tawny'],
    ['eyeOut','cheekLow','cheek','tawny'], ['eyeOut','eyeLow','cheekLow','tawny'],
    ['eyeIn','snoutBase','eyeLow','tawny'],
    ['eyeIn','eyeLow','eyeDeep','eye'], ['eyeLow','eyeOut','eyeDeep','eye'],
    ['eyeOut','eyeIn','eyeDeep','eye'],
    // bridge of the nose
    ['C_brow','snoutBase','eyeIn','tawny'], ['C_brow','C_stop','snoutBase','tawny'],
    // cheeks and muzzle
    ['eyeLow','jaw','cheekLow','tawny'], ['eyeLow','snoutBase','jaw','cream'],
    ['snoutBase','snoutBot','jaw','cream'],
    ['C_stop','snoutTip','snoutBase','tawny'], ['C_stop','C_snoutTip','snoutTip','tawny'],
    ['snoutBase','noseBot','snoutBot','cream'], ['snoutBase','snoutTip','noseBot','cream'],
    // broad nose
    ['C_snoutTip','noseBot','snoutTip','nose'], ['C_snoutTip','C_noseBot','noseBot','nose'],
    // chin and throat
    ['C_noseBot','snoutBot','noseBot','cream'], ['C_noseBot','C_chin','snoutBot','cream'],
    ['C_chin','jaw','snoutBot','cream'], ['C_chin','C_throat','jaw','cream'],
    ['C_throat','backLow','jaw','tawny'], ['jaw','backLow','cheekLow','tawny'],
    // side of the head
    ['cheek','cheekLow','backLow','tawny'], ['cheek','backLow','back','tawny'],
    // small round ears
    ['earFront','earTip','earInner','cream'], ['earInner','earTip','earOut','cream'],
    ['earFront','earInner','earOut','tawny'], ['earOut','earTip','earBack','earBack'],
    ['earBack','earTip','earFront','earBack'],
  ];
  const mirror = p => [-p[0], p[1], p[2]];
  const tris = [];
  for (const [a, b, c, col] of F) {
    tris.push({ a: V[a], b: V[b], c: V[c], color: COLORS[col] });
    tris.push({ a: mirror(V[a]), b: mirror(V[c]), c: mirror(V[b]), color: COLORS[col] });
  }
  return trisToGeometry(tris);
}

/* =================================================================
   PART 2 — THE MANE
   A shaggy collar around the face, made of two layers of spikes.
   Each layer goes around the head in a circle:
     inner edge (close to the face) → spiky outer edge → back edge
   Longer spikes underneath make the mane hang down over the chest.
   ================================================================= */
function buildMane() {
  const tris = [];
  const CENTER_Y = 0.25;
  const layers = [
    // count, inner radius, inner z, spike length, valley length, back z, twist
    { n: 16, rIn: 1.20, zIn:  0.15, rTip: 2.00, rValley: 1.55, zOut: -0.40, zBack: -1.20, twist: 0 },
    { n: 16, rIn: 1.15, zIn: -0.25, rTip: 2.25, rValley: 1.70, zOut: -0.85, zBack: -1.55, twist: 0.5 },
  ];
  const pick = seed => {
    const r = rand(seed);
    return r < 0.33 ? COLORS.maneDark : r < 0.7 ? COLORS.mane : COLORS.maneLight;
  };
  layers.forEach((L, li) => {
    // point on an oval around the head: a little taller than wide, longer at the bottom
    const at = (angle, radius, z) => {
      const droop = 1 + 0.3 * Math.max(0, -Math.sin(angle));        // chest mane hangs lower
      return [Math.cos(angle) * radius, CENTER_Y + Math.sin(angle) * radius * 1.08 * droop, z];
    };
    for (let k = 0; k < L.n; k++) {
      const a0 = ((k + L.twist) / L.n) * Math.PI * 2;
      const a1 = ((k + 1 + L.twist) / L.n) * Math.PI * 2;
      const am = (a0 + a1) / 2;
      const in0 = at(a0, L.rIn, L.zIn), in1 = at(a1, L.rIn, L.zIn);
      const v0 = at(a0, L.rValley, L.zOut), v1 = at(a1, L.rValley, L.zOut);
      const tip = at(am, L.rTip * (0.9 + rand(k + li * 50) * 0.25), L.zOut - 0.15);
      const b0 = at(a0, L.rIn * 0.9, L.zBack), b1 = at(a1, L.rIn * 0.9, L.zBack);
      const seed = k * 7 + li * 100;
      // front side of the collar
      tris.push({ a: in0, b: in1, c: tip, color: pick(seed + 1) });
      tris.push({ a: in0, b: tip, c: v0, color: pick(seed + 2) });
      tris.push({ a: in1, b: v1, c: tip, color: pick(seed + 3) });
      // back side of the collar
      tris.push({ a: b0, b: tip, c: b1, color: pick(seed + 4) });
      tris.push({ a: b0, b: v0, c: tip, color: pick(seed + 5) });
      tris.push({ a: b1, b: tip, c: v1, color: pick(seed + 6) });
    }
  });
  return trisToGeometry(tris);
}

/* =================================================================
   WHISKERS — LineSegments: every 2 points make one line
   ================================================================= */
function buildWhiskers() {
  const pts = [];
  for (const side of [1, -1]) {
    for (const [y0, y1, z1] of [[0.30, 0.45, 1.55], [0.20, 0.20, 1.60], [0.10, -0.05, 1.50]]) {
      pts.push(0.40 * side, y0, 1.80,   1.70 * side, y1, z1);
    }
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(pts, 3));
  return new THREE.LineSegments(g, new THREE.LineBasicMaterial({ color: 0xf6f0e4 }));
}

/* =================================================================
   PARTS 3–6 — body, front leg, back leg, tail  [y, z, rx, ry]
   ================================================================= */
// big muscular body with a lighter belly
const buildBody = () => loft([
  [0.40, -2.60, 0.55, 0.55],
  [0.50, -2.05, 0.95, 1.00],   // hips
  [0.55, -0.80, 0.90, 0.92],
  [0.45,  0.50, 1.00, 1.10],   // chest
  [0.60,  1.60, 0.98, 1.12],   // shoulders
  [1.15,  2.35, 0.72, 0.80],
  [1.60,  2.75, 0.62, 0.68],   // neck
], {
  startCap: [0.40, -2.85], endCap: 'flat',
  colorFor: (i, c, ring) => (c.y < ring.y - 0.55 ? COLORS.cream : COLORS.tawny),
});

// thick legs with big paws
const buildFrontLeg = () => loft([
  [ 0.00, 0.00, 0.42, 0.60],
  [-0.90,-0.05, 0.32, 0.45],
  [-1.70, 0.05, 0.24, 0.28],
  [-2.50, 0.05, 0.22, 0.25],
  [-2.75, 0.12, 0.26, 0.26],
  [-2.90, 0.30, 0.33, 0.20],
  [-2.95, 0.55, 0.33, 0.15],   // big paw
], { startCap: 'flat', endCap: [-2.97, 0.72] });

const buildBackLeg = () => loft([
  [ 0.00,  0.00, 0.52, 0.85],   // thigh
  [-0.60,  0.15, 0.44, 0.70],
  [-1.10,  0.30, 0.28, 0.34],   // knee
  [-1.60,  0.00, 0.22, 0.26],
  [-2.05, -0.32, 0.19, 0.23],   // hock
  [-2.55, -0.20, 0.18, 0.21],
  [-2.78, -0.05, 0.24, 0.24],
  [-2.90,  0.15, 0.32, 0.20],
  [-2.95,  0.42, 0.33, 0.15],   // paw
], { startCap: 'flat', endCap: [-2.97, 0.62] });

// long tail hanging down, ending in a dark tuft
const buildTail = () => loft([
  [ 0.00,  0.00, 0.15, 0.15],
  [ 0.05, -0.50, 0.13, 0.13],
  [-0.40, -1.00, 0.12, 0.12],
  [-1.10, -1.30, 0.11, 0.11],
  [-1.80, -1.40, 0.10, 0.10],
  [-2.20, -1.30, 0.20, 0.20],   // tuft starts
  [-2.50, -1.15, 0.22, 0.22],
], { startCap: 'flat', endCap: [-2.75, -1.00],
     colorFor: i => (i >= 4 ? COLORS.tuft : COLORS.tawny) });

/* =================================================================
   ASSEMBLE — one THREE.Group, each part placed where it joins
   ================================================================= */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.85 });
const shellMat = solidMat.clone(); shellMat.side = THREE.DoubleSide;   // head and mane are open shells

function part(name, geometry, material, [x, y, z], scale = 1) {
  const m = new THREE.Mesh(geometry, material);
  m.name = name;
  m.position.set(x, y, z);
  m.scale.setScalar(scale);
  m.castShadow = true;
  return m;
}

const lion = new THREE.Group();
lion.name = 'lion';

const frontLegGeo = buildFrontLeg();
const backLegGeo  = buildBackLeg();

lion.add(part('body', buildBody(), solidMat, [0, 0, 0]));
const head = part('head', buildHead(), shellMat, [0, 1.54, 3.46], 0.85);
head.add(part('mane', buildMane(), shellMat, [0, 0, 0]));   // mane moves with the head
head.add(buildWhiskers());
lion.add(head);
lion.add(part('frontLegL', frontLegGeo, solidMat, [-0.58, 0.2,  1.40]));
lion.add(part('frontLegR', frontLegGeo, solidMat, [ 0.58, 0.2,  1.40]));
lion.add(part('backLegL',  backLegGeo,  solidMat, [-0.60, 0.2, -2.00]));
lion.add(part('backLegR',  backLegGeo,  solidMat, [ 0.60, 0.2, -2.00]));
const tail = part('tail', buildTail(), solidMat, [0, 0.90, -2.70]);
lion.add(tail);

// stand the lion on y = 0 and centre it front-to-back
const box = new THREE.Box3().setFromObject(lion);
lion.position.y = -box.min.y;
lion.position.z = -(box.min.z + box.max.z) / 2;

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
scene.add(lion);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -10, right: 10, top: 10, bottom: -10, near: 1, far: 45 });
scene.add(key);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

const edgeLines = [];
lion.traverse(o => {
  if (!o.isMesh || o.isLineSegments) return;
  const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1),
                                   new THREE.LineBasicMaterial({ color: 0x3a6ff0 }));
  l.visible = false; o.add(l); edgeLines.push(l);
});

let triCount = 0;
lion.traverse(o => { if (o.isMesh) triCount += o.geometry.attributes.position.count / 3; });
document.getElementById('stats').textContent =
  `8 parts + whiskers, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 8;
controls.maxDistance = 55;
controls.maxPolarAngle = Math.PI * 0.49;   // don't go under the floor
controls.autoRotateSpeed = 2;
const center = new THREE.Vector3(0, 3.0, 0);
controls.target.copy(center);

const VIEWS = {
  angle: new THREE.Vector3(14, 8, 17),
  front: new THREE.Vector3(0, 3.6, 25),
  side:  new THREE.Vector3(-25, 3.6, 0),
  top:   new THREE.Vector3(0, 28, 0.01),
};
let target = VIEWS.angle.clone();
camera.position.copy(target);
let moving = false;
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

function fitToScreen() {
  camera.aspect = stage.clientWidth / stage.clientHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(stage.clientWidth, stage.clientHeight);
}
window.addEventListener('resize', fitToScreen);

/* =================================================================
   BUTTONS
   ================================================================= */
document.querySelectorAll('[data-view]').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('[data-view]').forEach(b => b.setAttribute('aria-pressed', 'false'));
    btn.setAttribute('aria-pressed', 'true');
    target = VIEWS[btn.dataset.view].clone();
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
  const on = !edgeLines[0].visible;
  edgeLines.forEach(l => (l.visible = on));
  edgesBtn.setAttribute('aria-pressed', String(on));
});

let wagging = !reduceMotion;
const wagBtn = document.getElementById('wag');
wagBtn.setAttribute('aria-pressed', String(wagging));
wagBtn.addEventListener('click', () => {
  wagging = !wagging;
  wagBtn.setAttribute('aria-pressed', String(wagging));
});

/* =================================================================
   RENDER LOOP
   ================================================================= */
const clock = new THREE.Clock();
(function loop() {
  requestAnimationFrame(loop);
  const t = clock.getElapsedTime();
  const wagTarget = wagging ? Math.sin(t * 1.4) * 0.35 : 0;     // slow, heavy swish
  tail.rotation.y += (wagTarget - tail.rotation.y) * 0.1;
  tail.rotation.z = tail.rotation.y * 0.4;                        // lean a little as it swings
  head.rotation.y = wagging ? Math.sin(t * 0.7) * 0.2 : head.rotation.y * 0.95;   // look around
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