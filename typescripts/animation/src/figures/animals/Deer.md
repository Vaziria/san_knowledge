# Deer

## Animation Behavior.
2. `Jump()`
3. `Hold(Figure figure)`
4. `Walk()`
5. `Run()`
6. `Speech(text string)`, its show Speech Bubble and show text, when text to long, its split to multiple Bubble that show and hide sequentially


# Shape reference
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Low-poly deer — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #a8672e;
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
  <h1>Low-poly deer</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="wag" aria-pressed="true">Flick tail</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates for every part: x = right, y = up, z = forward
   (the deer faces +z). All parts use the same units.
   ================================================================= */

const COLORS = {
  fur:    [0.66, 0.43, 0.24],   // warm tan-brown coat
  white:  [0.94, 0.91, 0.85],   // belly, throat, chin, under the tail
  black:  [0.10, 0.08, 0.07],   // nose, eyes, hooves
  antler: [0.86, 0.79, 0.63],   // bone colour
  inner:  [0.85, 0.70, 0.60],   // inside the ears
};

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
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => COLORS.fur } = {}) {
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
   Long narrow face, eyes on the sides, big ears that stick out sideways.
   ================================================================= */
function buildHead() {
  const V = {
    C_back: [0, 1.30, -1.00], C_forehead: [0, 1.45, 0.15], C_brow: [0, 0.90, 1.00],
    C_stop: [0, 0.70, 1.30], C_snoutTip: [0, 0.25, 2.50], C_noseBot: [0, -0.20, 2.60],
    C_chin: [0, -0.55, 1.80], C_throat: [0, -0.75, 0.20],
    topPlate: [0.40, 1.40, 0.15], temple: [0.70, 1.05, 0.25], brow: [0.40, 0.85, 0.90],
    browOuter: [0.72, 0.80, 0.55],
    eyeIn: [0.50, 0.58, 0.80], eyeOut: [0.78, 0.62, 0.38], eyeLow: [0.64, 0.36, 0.62],
    eyeDeep: [0.55, 0.55, 0.45],
    cheek: [0.95, 0.50, -0.10], cheekLow: [0.80, -0.25, 0.30], jaw: [0.40, -0.45, 1.30],
    back: [0.75, 0.95, -0.90], backLow: [0.85, -0.20, -0.60],
    snoutBase: [0.35, 0.60, 1.30], snoutTip: [0.25, 0.20, 2.45], noseBot: [0.22, -0.18, 2.55],
    snoutBot: [0.28, -0.45, 1.60],
    earFront: [0.55, 1.25, 0.00], earOut: [0.85, 1.00, -0.15], earBack: [0.60, 1.20, -0.50],
    earTip: [1.85, 1.80, -0.45], earInner: [1.10, 1.42, -0.20],
  };
  const F = [
    // forehead
    ['C_back','C_forehead','topPlate','fur'], ['C_back','topPlate','back','fur'],
    ['C_forehead','brow','topPlate','fur'], ['C_forehead','C_brow','brow','fur'],
    ['topPlate','brow','temple','fur'], ['temple','brow','browOuter','fur'],
    ['topPlate','temple','back','fur'], ['back','temple','cheek','fur'],
    ['temple','browOuter','cheek','fur'],
    // around the eye
    ['brow','eyeOut','browOuter','fur'], ['brow','eyeIn','eyeOut','fur'],
    ['C_brow','eyeIn','brow','fur'], ['browOuter','eyeOut','cheek','fur'],
    ['eyeOut','cheekLow','cheek','fur'], ['eyeOut','eyeLow','cheekLow','fur'],
    ['eyeIn','snoutBase','eyeLow','fur'],
    ['eyeIn','eyeLow','eyeDeep','black'], ['eyeLow','eyeOut','eyeDeep','black'],
    ['eyeOut','eyeIn','eyeDeep','black'],
    // bridge
    ['C_brow','snoutBase','eyeIn','fur'], ['C_brow','C_stop','snoutBase','fur'],
    // cheek to jaw
    ['eyeLow','jaw','cheekLow','fur'], ['eyeLow','snoutBase','jaw','fur'],
    ['snoutBase','snoutBot','jaw','fur'],
    // long snout
    ['C_stop','snoutTip','snoutBase','fur'], ['C_stop','C_snoutTip','snoutTip','fur'],
    ['snoutBase','noseBot','snoutBot','white'], ['snoutBase','snoutTip','noseBot','fur'],
    // nose
    ['C_snoutTip','noseBot','snoutTip','black'], ['C_snoutTip','C_noseBot','noseBot','black'],
    // white chin and throat
    ['C_noseBot','snoutBot','noseBot','white'], ['C_noseBot','C_chin','snoutBot','white'],
    ['C_chin','jaw','snoutBot','white'], ['C_chin','C_throat','jaw','white'],
    ['C_throat','backLow','jaw','white'], ['jaw','backLow','cheekLow','fur'],
    // side of head
    ['cheek','cheekLow','backLow','fur'], ['cheek','backLow','back','fur'],
    // big sideways ears
    ['earFront','earTip','earInner','inner'], ['earInner','earTip','earOut','inner'],
    ['earFront','earInner','earOut','fur'], ['earOut','earTip','earBack','fur'],
    ['earBack','earTip','earFront','fur'],
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
   PART 2 — ANTLER (one side). A curved main beam plus three tines,
   each a thin loft that ends in a point.
   ================================================================= */
function buildAntler() {
  const opts = { sides: 5, startCap: 'flat', colorFor: () => COLORS.antler };
  const group = new THREE.Group();
  const pieces = [
    // main beam: up, back, then curving forward
    loft([[0.00, 0.00, 0.10, 0.10], [0.60, -0.25, 0.09, 0.09], [1.20, -0.30, 0.08, 0.08],
          [1.70, -0.10, 0.065, 0.065], [2.00, 0.25, 0.05, 0.05]], { ...opts, endCap: [2.15, 0.50] }),
    // brow tine (low, pointing forward)
    loft([[0.25, -0.10, 0.06, 0.06], [0.45, 0.20, 0.04, 0.04]], { ...opts, endCap: [0.55, 0.40] }),
    // middle tine (straight up)
    loft([[0.60, -0.25, 0.06, 0.06], [1.10, 0.05, 0.045, 0.045]], { ...opts, endCap: [1.40, 0.20] }),
    // back tine
    loft([[1.20, -0.30, 0.055, 0.055], [1.65, -0.35, 0.04, 0.04]], { ...opts, endCap: [1.95, -0.30] }),
  ];
  pieces.forEach(g => { const m = new THREE.Mesh(g, solidMat); m.castShadow = true; group.add(m); });
  return group;
}

/* =================================================================
   PARTS 3–6 — body, front leg, back leg, tail  [y, z, rx, ry]
   ================================================================= */
// slim body and a long neck rising at the front
const buildBody = () => loft([
  [0.35, -2.40, 0.45, 0.45],
  [0.40, -1.90, 0.85, 0.90],   // hips
  [0.45, -0.70, 0.80, 0.80],
  [0.35,  0.50, 0.85, 1.00],   // chest
  [0.50,  1.60, 0.75, 0.95],   // shoulders
  [1.30,  2.30, 0.50, 0.55],   // base of the neck
  [2.30,  2.75, 0.40, 0.45],
  [3.10,  3.05, 0.36, 0.40],   // top of the neck
], {
  startCap: [0.40, -2.65], endCap: 'flat',
  colorFor: (i, c, ring) =>
    (i >= 1 && i <= 3 && c.y < ring.y - 0.6) ? COLORS.white :     // belly
    (i >= 5 && c.z > ring.z + 0.2)           ? COLORS.white :      // front of the neck
    COLORS.fur,
});

// long thin legs with black hooves
const buildFrontLeg = () => loft([
  [ 0.00, 0.00, 0.32, 0.50],
  [-0.90,-0.05, 0.24, 0.35],
  [-1.70, 0.05, 0.14, 0.17],   // "knee" (really the wrist)
  [-2.10, 0.05, 0.12, 0.14],
  [-3.40, 0.10, 0.09, 0.11],
  [-3.70, 0.12, 0.12, 0.12],   // fetlock
  [-4.00, 0.20, 0.13, 0.14],
  [-4.15, 0.28, 0.14, 0.10],   // hoof
], { startCap: 'flat', endCap: [-4.18, 0.34], colorFor: i => (i >= 6 ? COLORS.black : COLORS.fur) });

const buildBackLeg = () => loft([
  [ 0.00,  0.00, 0.42, 0.72],   // thigh
  [-0.70,  0.15, 0.35, 0.55],
  [-1.30,  0.20, 0.20, 0.26],   // knee
  [-1.90, -0.20, 0.15, 0.20],
  [-2.40, -0.50, 0.12, 0.16],   // hock (sharp backward point)
  [-3.40, -0.35, 0.09, 0.11],
  [-3.75, -0.25, 0.12, 0.12],
  [-4.00, -0.15, 0.13, 0.14],
  [-4.15, -0.08, 0.14, 0.10],   // hoof
], { startCap: 'flat', endCap: [-4.18, 0.00], colorFor: i => (i >= 7 ? COLORS.black : COLORS.fur) });

// short tail, white underneath
const buildTail = () => loft([
  [ 0.00,  0.00, 0.18, 0.14],
  [ 0.10, -0.35, 0.22, 0.16],
  [-0.10, -0.70, 0.16, 0.12],
], { startCap: 'flat', endCap: [-0.25, -0.85],
     colorFor: (i, c, ring) => (c.y < ring.y ? COLORS.white : COLORS.fur) });

/* =================================================================
   ASSEMBLE — one THREE.Group, each part placed where it joins
   ================================================================= */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.85 });
const shellMat = solidMat.clone(); shellMat.side = THREE.DoubleSide;   // head is an open shell

function part(name, geometry, material, [x, y, z], scale = 1) {
  const m = geometry.isObject3D ? geometry : new THREE.Mesh(geometry, material);
  m.name = name;
  m.position.set(x, y, z);
  m.scale.setScalar(scale);
  m.castShadow = true;
  return m;
}

const deer = new THREE.Group();
deer.name = 'deer';

const frontLegGeo = buildFrontLeg();
const backLegGeo  = buildBackLeg();

deer.add(part('body', buildBody(), solidMat, [0, 0, 0]));
deer.add(part('head', buildHead(), shellMat, [0, 3.00, 3.80], 0.8));
deer.add(part('frontLegL', frontLegGeo, solidMat, [-0.45, 0.2,  1.30]));
deer.add(part('frontLegR', frontLegGeo, solidMat, [ 0.45, 0.2,  1.30]));
deer.add(part('backLegL',  backLegGeo,  solidMat, [-0.48, 0.2, -1.80]));
deer.add(part('backLegR',  backLegGeo,  solidMat, [ 0.48, 0.2, -1.80]));

// antlers sit on top of the head and lean outward
const antlerR = part('antlerR', buildAntler(), null, [ 0.30, 4.05, 3.85]);
antlerR.rotation.z = -0.45;
const antlerL = part('antlerL', buildAntler(), null, [-0.30, 4.05, 3.85]);
antlerL.rotation.z = 0.45;
antlerL.scale.x = -1;             // mirror the right antler
deer.add(antlerR, antlerL);

const tail = part('tail', buildTail(), solidMat, [0, 0.80, -2.45]);
deer.add(tail);

// stand the deer on y = 0 and centre it front-to-back
const box = new THREE.Box3().setFromObject(deer);
deer.position.y = -box.min.y;
deer.position.z = -(box.min.z + box.max.z) / 2;

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
scene.add(deer);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -11, right: 11, top: 11, bottom: -11, near: 1, far: 50 });
scene.add(key);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

const edgeLines = [];
deer.traverse(o => {
  if (!o.isMesh) return;
  const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1),
                                   new THREE.LineBasicMaterial({ color: 0x3a6ff0 }));
  l.visible = false; o.add(l); edgeLines.push(l);
});

let triCount = 0, partCount = 0;
deer.traverse(o => { if (o.isMesh) { partCount++; triCount += o.geometry.attributes.position.count / 3; } });
document.getElementById('stats').textContent =
  `${partCount} meshes, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

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
const center = new THREE.Vector3(0, 4.2, 0);
controls.target.copy(center);

const VIEWS = {
  angle: new THREE.Vector3(15, 9, 18),
  front: new THREE.Vector3(0, 4.8, 28),
  side:  new THREE.Vector3(-28, 4.8, 0),
  top:   new THREE.Vector3(0, 30, 0.01),
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
  const wagTarget = wagging ? Math.max(0, Math.sin(t * 4)) * 0.7 : 0;
  tail.rotation.x += (-wagTarget - tail.rotation.x) * 0.2;   // flick up around its base
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