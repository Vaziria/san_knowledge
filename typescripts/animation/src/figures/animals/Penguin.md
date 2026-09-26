# Penguin

## Animation Behavior.
1. `Flap()`
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
<title>Low-poly penguin — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #e0901f;
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
  <h1>Low-poly penguin</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="wag" aria-pressed="true">Waddle</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates for every part: x = right, y = up, z = forward
   (the penguin faces +z). All parts use the same units.
   ================================================================= */

const COLORS = {
  black:  [0.10, 0.11, 0.14],   // back, head, flippers
  white:  [0.96, 0.96, 0.95],   // belly
  yellow: [0.98, 0.72, 0.22],   // neck patches
  orange: [0.95, 0.50, 0.12],   // beak stripe and feet
  eyeRim: [0.90, 0.90, 0.92],
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
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => COLORS.black } = {}) {
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
   PART 1 — BODY + HEAD in one loft, standing UP.
   The path now goes up the y axis, so for each ring:
     rx = width (left–right), ry = depth (front–back),
     and the ring's first corner points to the BACK (-z).
   ================================================================= */
const buildBody = () => loft([
  [0.15,  0.00, 0.55, 0.50],   // bottom
  [0.50,  0.00, 0.95, 0.85],
  [1.20,  0.02, 1.05, 0.95],   // round belly
  [2.00,  0.00, 0.90, 0.80],   // chest
  [2.60, -0.02, 0.62, 0.58],   // neck
  [3.00,  0.03, 0.62, 0.60],   // head
  [3.35,  0.05, 0.52, 0.50],
], {
  sides: 8,
  startCap: 'flat', endCap: [3.62, 0.03],
  colorFor: (i, c, ring) => {
    const front = c.z - ring.z;                              // + = toward the front
    if (i === 3 && Math.abs(c.x) > 0.3 && front > -0.2) return COLORS.yellow;  // neck patches
    if (i <= 3 && front > 0.25) return COLORS.white;         // white belly
    return COLORS.black;
  },
});

// beak: black on top, orange stripe underneath
const buildBeak = () => loft([
  [ 0.00, 0.00, 0.15, 0.12],
  [-0.04, 0.28, 0.10, 0.08],
  [-0.10, 0.50, 0.05, 0.05],
], { startCap: 'flat', endCap: [-0.15, 0.62],
     colorFor: (i, c, ring) => (c.y < ring.y ? COLORS.orange : COLORS.black) });

// flipper: very thin side to side, wide front to back, hanging down
const buildFlipper = () => loft([
  [ 0.00,  0.00, 0.08, 0.26],
  [-0.60, -0.05, 0.07, 0.30],
  [-1.20, -0.10, 0.05, 0.20],
], { startCap: 'flat', endCap: [-1.55, -0.15],
     colorFor: (i, c) => (c.x < -0.02 ? COLORS.white : COLORS.black) });   // white on the inside

// flat orange foot pointing forward
const buildFoot = () => loft([
  [0, -0.10, 0.12, 0.06],
  [0,  0.20, 0.22, 0.05],
  [0,  0.42, 0.27, 0.04],
], { startCap: 'flat', endCap: [0, 0.55], colorFor: () => COLORS.orange });

// short stubby tail
const buildTail = () => loft([
  [ 0.00,  0.00, 0.18, 0.07],
  [-0.05, -0.28, 0.10, 0.04],
], { startCap: 'flat', endCap: [-0.08, -0.42] });

// small eye: light rim with a black centre (flat disc facing +x)
function buildEye() {
  const tris = [], N = 6, R = 0.09;
  const ring = (r, x) => [...Array(N).keys()].map(k => {
    const a = (2 * Math.PI * k) / N;
    return [x, Math.cos(a) * r, Math.sin(a) * r];
  });
  const outer = ring(R, 0), inner = ring(R * 0.6, 0.02), centre = [0.03, 0, 0];
  for (let k = 0; k < N; k++) {
    const k2 = (k + 1) % N;
    tris.push({ a: outer[k], b: outer[k2], c: inner[k2], color: COLORS.eyeRim });
    tris.push({ a: outer[k], b: inner[k2], c: inner[k], color: COLORS.eyeRim });
    tris.push({ a: centre, b: inner[k], c: inner[k2], color: COLORS.black });
  }
  return trisToGeometry(tris);
}

/* =================================================================
   ASSEMBLE
   "upper" holds everything except the feet, so the whole body can
   rock from side to side on top of the feet when it waddles.
   ================================================================= */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.6 });
const flatMat = solidMat.clone(); flatMat.side = THREE.DoubleSide;   // eyes are flat discs

function part(name, geometry, material, [x, y, z]) {
  const m = new THREE.Mesh(geometry, material);
  m.name = name;
  m.position.set(x, y, z);
  m.castShadow = true;
  return m;
}

const penguin = new THREE.Group();
penguin.name = 'penguin';
const upper = new THREE.Group();
upper.name = 'upper';
penguin.add(upper);

upper.add(part('body', buildBody(), solidMat, [0, 0, 0]));
upper.add(part('beak', buildBeak(), solidMat, [0, 3.12, 0.46]));
upper.add(part('tail', buildTail(), solidMat, [0, 0.35, -0.72]));

const eyeGeo = buildEye();
const eyeR = part('eyeR', eyeGeo, flatMat, [ 0.47, 3.25, 0.25]); eyeR.rotation.y = -0.5;
const eyeL = part('eyeL', eyeGeo, flatMat, [-0.47, 3.25, 0.25]); eyeL.rotation.y = -0.5; eyeL.scale.x = -1;
upper.add(eyeR, eyeL);

const flipperGeo = buildFlipper();
const flipperR = part('flipperR', flipperGeo, solidMat, [ 0.88, 2.15, -0.05]);
const flipperL = part('flipperL', flipperGeo, solidMat, [-0.88, 2.15, -0.05]);
flipperL.scale.x = -1;          // mirror the right flipper
upper.add(flipperR, flipperL);

const footGeo = buildFoot();
const footR = part('footR', footGeo, solidMat, [ 0.35, 0.05, 0.30]); footR.rotation.y =  0.25;
const footL = part('footL', footGeo, solidMat, [-0.35, 0.05, 0.30]); footL.rotation.y = -0.25;
penguin.add(footR, footL);

// flippers rest slightly away from the body
const FLIPPER_REST = 0.22;
function poseFlippers(extra) {
  flipperR.rotation.z =  FLIPPER_REST + extra;
  flipperL.rotation.z = -FLIPPER_REST - extra;
}
poseFlippers(0);

// stand the penguin on y = 0 and centre it front-to-back
const box = new THREE.Box3().setFromObject(penguin);
penguin.position.y = -box.min.y;
penguin.position.z = -(box.min.z + box.max.z) / 2;

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
scene.add(penguin);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -5, right: 5, top: 5, bottom: -5, near: 1, far: 40 });
scene.add(key);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

const edgeLines = [];
penguin.traverse(o => {
  if (!o.isMesh) return;
  const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1),
                                   new THREE.LineBasicMaterial({ color: 0x3a6ff0 }));
  l.visible = false; o.add(l); edgeLines.push(l);
});

let triCount = 0, partCount = 0;
penguin.traverse(o => { if (o.isMesh) { partCount++; triCount += o.geometry.attributes.position.count / 3; } });
document.getElementById('stats').textContent =
  `${partCount} parts, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 3;
controls.maxDistance = 25;
controls.maxPolarAngle = Math.PI * 0.49;   // don't go under the floor
controls.autoRotateSpeed = 2;
const center = new THREE.Vector3(0, 1.8, 0);
controls.target.copy(center);

const VIEWS = {
  angle: new THREE.Vector3(5.5, 4, 7.5),
  front: new THREE.Vector3(0, 2.2, 11),
  side:  new THREE.Vector3(-11, 2.2, 0),
  top:   new THREE.Vector3(0, 12, 0.01),
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
  // waddle: rock side to side, lift the foot that has no weight on it, flap a little
  const w = wagging ? Math.sin(t * 4) : 0;
  upper.rotation.z += (w * 0.12 - upper.rotation.z) * 0.2;
  footR.position.y = 0.05 + Math.max(0,  w) * 0.12;
  footL.position.y = 0.05 + Math.max(0, -w) * 0.12;
  poseFlippers(wagging ? 0.12 + Math.abs(w) * 0.25 : 0);
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