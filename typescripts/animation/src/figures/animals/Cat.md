# Cat

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
<title>Low-poly cat — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #6c8a3c;
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
  <h1>Low-poly cat</h1>
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
   (the cat faces +z). All parts use the same units.
   ================================================================= */

const COLORS = {
  grey:   [0.62, 0.62, 0.64],   // main fur
  stripe: [0.30, 0.30, 0.33],   // tabby stripes
  white:  [0.95, 0.94, 0.92],   // chest, muzzle, paws
  pink:   [0.93, 0.62, 0.64],   // nose, inside ears
  eye:    [0.45, 0.78, 0.30],   // green eyes
};

// tabby stripe test: bands that repeat along one direction
const isStripe = (value, width = 0.35) => Math.sin(value) > width;

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
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => COLORS.grey } = {}) {
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
   Round face, short muzzle, big forward-facing eyes, tall pointed ears.
   ================================================================= */
function buildHead() {
  const V = {
    C_back: [0, 1.30, -0.90], C_forehead: [0, 1.55, 0.20], C_brow: [0, 0.95, 0.95],
    C_stop: [0, 0.60, 1.20], C_snoutTip: [0, 0.40, 1.60], C_noseBot: [0, 0.10, 1.65],
    C_chin: [0, -0.45, 1.25], C_throat: [0, -0.70, 0.20],
    topPlate: [0.50, 1.50, 0.20], temple: [0.90, 1.15, 0.30], brow: [0.42, 0.95, 0.90],
    browOuter: [0.90, 0.95, 0.60],
    eyeIn: [0.25, 0.72, 1.12], eyeOut: [0.75, 0.80, 0.85], eyeLow: [0.50, 0.45, 1.05],
    eyeDeep: [0.48, 0.70, 0.85],
    cheek: [1.20, 0.45, 0.10], cheekLow: [1.00, -0.20, 0.50], jaw: [0.45, -0.35, 1.05],
    back: [0.85, 0.95, -0.80], backLow: [1.00, -0.20, -0.50],
    snoutBase: [0.30, 0.45, 1.30], snoutTip: [0.28, 0.30, 1.58], noseBot: [0.20, 0.02, 1.62],
    snoutBot: [0.25, -0.30, 1.30],
    earFront: [0.35, 1.50, 0.35], earOut: [1.05, 1.20, 0.25], earBack: [0.70, 1.45, -0.30],
    earTip: [0.95, 2.35, 0.05], earInner: [0.72, 1.60, 0.15],
  };
  const F = [
    // forehead (with the classic tabby "M" stripes)
    ['C_back','C_forehead','topPlate','stripe'], ['C_back','topPlate','back','grey'],
    ['C_forehead','brow','topPlate','grey'], ['C_forehead','C_brow','brow','stripe'],
    ['topPlate','brow','temple','stripe'], ['temple','brow','browOuter','grey'],
    ['topPlate','temple','back','grey'], ['back','temple','cheek','stripe'],
    ['temple','browOuter','cheek','grey'],
    // around the eye
    ['brow','eyeOut','browOuter','grey'], ['brow','eyeIn','eyeOut','grey'],
    ['C_brow','eyeIn','brow','grey'], ['browOuter','eyeOut','cheek','stripe'],
    ['eyeOut','cheekLow','cheek','grey'], ['eyeOut','eyeLow','cheekLow','grey'],
    ['eyeIn','snoutBase','eyeLow','white'],
    ['eyeIn','eyeLow','eyeDeep','eye'], ['eyeLow','eyeOut','eyeDeep','eye'],
    ['eyeOut','eyeIn','eyeDeep','eye'],
    // bridge
    ['C_brow','snoutBase','eyeIn','grey'], ['C_brow','C_stop','snoutBase','grey'],
    // white muzzle and chin
    ['eyeLow','jaw','cheekLow','white'], ['eyeLow','snoutBase','jaw','white'],
    ['snoutBase','snoutBot','jaw','white'],
    ['C_stop','snoutTip','snoutBase','white'], ['C_stop','C_snoutTip','snoutTip','white'],
    ['snoutBase','noseBot','snoutBot','white'], ['snoutBase','snoutTip','noseBot','white'],
    // small pink nose
    ['C_snoutTip','noseBot','snoutTip','pink'], ['C_snoutTip','C_noseBot','noseBot','pink'],
    // underside
    ['C_noseBot','snoutBot','noseBot','white'], ['C_noseBot','C_chin','snoutBot','white'],
    ['C_chin','jaw','snoutBot','white'], ['C_chin','C_throat','jaw','white'],
    ['C_throat','backLow','jaw','white'], ['jaw','backLow','cheekLow','grey'],
    // side of head
    ['cheek','cheekLow','backLow','stripe'], ['cheek','backLow','back','grey'],
    // tall pointed ears, pink inside
    ['earFront','earTip','earInner','pink'], ['earInner','earTip','earOut','pink'],
    ['earFront','earInner','earOut','grey'], ['earOut','earTip','earBack','grey'],
    ['earBack','earTip','earFront','grey'],
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
   WHISKERS — a LineSegments made from a plain BufferGeometry.
   Every 2 points in the "position" array make one line.
   ================================================================= */
function buildWhiskers() {
  const pts = [];
  for (const side of [1, -1]) {
    for (const [y0, y1, z1] of [[0.28, 0.45, 1.25], [0.20, 0.20, 1.30], [0.12, -0.05, 1.20]]) {
      pts.push(0.30 * side, y0, 1.45,   1.45 * side, y1, z1);   // start → end
    }
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(pts, 3));
  return new THREE.LineSegments(g, new THREE.LineBasicMaterial({ color: 0xf4f4f4 }));
}

/* =================================================================
   PARTS 2–5 — body, front leg, back leg, tail  [y, z, rx, ry]
   ================================================================= */
// slim body: white chest, tabby stripes across the back and sides
const buildBody = () => loft([
  [0.35, -2.30, 0.40, 0.40],
  [0.45, -1.85, 0.72, 0.75],   // hips
  [0.50, -0.70, 0.68, 0.70],
  [0.40,  0.50, 0.72, 0.80],   // chest
  [0.50,  1.50, 0.68, 0.78],   // shoulders
  [1.00,  2.10, 0.48, 0.52],
  [1.45,  2.40, 0.42, 0.45],   // neck
], {
  startCap: [0.35, -2.55], endCap: 'flat',
  colorFor: (i, c, ring) =>
    c.y < ring.y - 0.3 && c.z > 0.3             ? COLORS.white :
    c.y > ring.y - 0.2 && isStripe(c.z * 5)     ? COLORS.stripe : COLORS.grey,
});

// legs with stripes and white paws ("socks")
const buildFrontLeg = () => loft([
  [ 0.00, 0.00, 0.28, 0.40],
  [-0.80,-0.05, 0.20, 0.28],
  [-1.50, 0.05, 0.13, 0.15],
  [-2.20, 0.05, 0.12, 0.13],
  [-2.40, 0.12, 0.16, 0.15],
  [-2.50, 0.25, 0.20, 0.13],
  [-2.55, 0.42, 0.19, 0.10],   // small round paw
], { startCap: 'flat', endCap: [-2.57, 0.55],
     colorFor: (i, c) => i >= 4 ? COLORS.white : isStripe(c.y * 6, 0.5) ? COLORS.stripe : COLORS.grey });

const buildBackLeg = () => loft([
  [ 0.00,  0.00, 0.36, 0.62],   // thigh
  [-0.50,  0.15, 0.30, 0.50],
  [-0.95,  0.28, 0.18, 0.24],   // knee
  [-1.40,  0.00, 0.14, 0.17],
  [-1.80, -0.30, 0.12, 0.15],   // hock
  [-2.25, -0.18, 0.11, 0.13],
  [-2.45, -0.05, 0.15, 0.15],
  [-2.53,  0.12, 0.20, 0.13],
  [-2.57,  0.35, 0.19, 0.10],   // paw
], { startCap: 'flat', endCap: [-2.59, 0.50],
     colorFor: (i, c) => i >= 6 ? COLORS.white : isStripe(c.y * 6, 0.5) ? COLORS.stripe : COLORS.grey });

// long tail that rises and curls forward, ringed with stripes
const buildTail = () => loft([
  [0.00,  0.00, 0.14, 0.14],
  [0.30, -0.45, 0.14, 0.14],
  [0.90, -0.80, 0.13, 0.13],
  [1.60, -0.95, 0.13, 0.13],
  [2.30, -0.85, 0.12, 0.12],
  [2.80, -0.55, 0.11, 0.11],
  [3.05, -0.20, 0.10, 0.10],
], { startCap: 'flat', endCap: [3.10, 0.00],
     colorFor: i => (i % 2 === 1 || i === 5 ? COLORS.stripe : COLORS.grey) });

/* =================================================================
   ASSEMBLE — one THREE.Group, each part placed where it joins
   ================================================================= */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.8 });
const shellMat = solidMat.clone(); shellMat.side = THREE.DoubleSide;   // head is an open shell

function part(name, geometry, material, [x, y, z], scale = 1) {
  const m = new THREE.Mesh(geometry, material);
  m.name = name;
  m.position.set(x, y, z);
  m.scale.setScalar(scale);
  m.castShadow = true;
  return m;
}

const cat = new THREE.Group();
cat.name = 'cat';

const frontLegGeo = buildFrontLeg();
const backLegGeo  = buildBackLeg();

cat.add(part('body', buildBody(), solidMat, [0, 0, 0]));
const head = part('head', buildHead(), shellMat, [0, 1.35, 3.09], 0.82);
head.add(buildWhiskers());          // whiskers move with the head
cat.add(head);
cat.add(part('frontLegL', frontLegGeo, solidMat, [-0.42, 0.2,  1.20]));
cat.add(part('frontLegR', frontLegGeo, solidMat, [ 0.42, 0.2,  1.20]));
cat.add(part('backLegL',  backLegGeo,  solidMat, [-0.45, 0.2, -1.75]));
cat.add(part('backLegR',  backLegGeo,  solidMat, [ 0.45, 0.2, -1.75]));
const tail = part('tail', buildTail(), solidMat, [0, 0.70, -2.40]);
cat.add(tail);

// stand the cat on y = 0 and centre it front-to-back
const box = new THREE.Box3().setFromObject(cat);
cat.position.y = -box.min.y;
cat.position.z = -(box.min.z + box.max.z) / 2;

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
scene.add(cat);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -9, right: 9, top: 9, bottom: -9, near: 1, far: 40 });
scene.add(key);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

const edgeLines = [];
cat.traverse(o => {
  if (!o.isMesh || o.isLineSegments) return;
  const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1),
                                   new THREE.LineBasicMaterial({ color: 0x3a6ff0 }));
  l.visible = false; o.add(l); edgeLines.push(l);
});

let triCount = 0;
cat.traverse(o => { if (o.isMesh) triCount += o.geometry.attributes.position.count / 3; });
document.getElementById('stats').textContent =
  `7 parts + whiskers, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 8;
controls.maxDistance = 45;
controls.maxPolarAngle = Math.PI * 0.49;   // don't go under the floor
controls.autoRotateSpeed = 2;
const center = new THREE.Vector3(0, 2.6, 0);
controls.target.copy(center);

const VIEWS = {
  angle: new THREE.Vector3(11, 7, 13),
  front: new THREE.Vector3(0, 3.0, 20),
  side:  new THREE.Vector3(-20, 3.0, 0),
  top:   new THREE.Vector3(0, 22, 0.01),
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
  const wagTarget = wagging ? Math.sin(t * 1.8) * 0.45 : 0;     // slow, lazy swish
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