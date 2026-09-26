# Snake

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
<title>Low-poly snake — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #4f8a2c;
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
  <h1>Low-poly snake</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="slither" aria-pressed="true">Slither</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates: x = right, y = up, z = forward (the snake faces +z).
   The body is ONE BufferGeometry whose vertex positions are
   rewritten every frame, so the snake can slither.
   ================================================================= */

const COLORS = {
  green: [0.36, 0.56, 0.20],   // main scales
  dark:  [0.14, 0.27, 0.10],   // zig-zag pattern
  belly: [0.89, 0.85, 0.56],   // pale belly scales
  eye:   [0.95, 0.80, 0.15],   // yellow eyes
  red:   [0.85, 0.12, 0.15],   // tongue
};

/* -----------------------------------------------------------------
   HELPER: list of triangles → BufferGeometry (same as the other animals)
   ----------------------------------------------------------------- */
function trisToGeometry(tris) {
  const positions = [], colors = [];
  for (const t of tris) {
    positions.push(...t.a, ...t.b, ...t.c);
    colors.push(...t.color, ...t.color, ...t.color);
  }
  const g = new THREE.BufferGeometry();
  g.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
  g.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
  g.computeVertexNormals();
  return g;
}

/* =================================================================
   PART 1 — THE BODY (a tube along a wavy 3D path)
   s runs from 0 (tail tip) to LENGTH (neck).
   ================================================================= */
const SEGMENTS = 48;       // rings along the body
const SIDES = 6;           // corners per ring
const LENGTH = 10;

// how thick the body is at position s
function radiusAt(s) {
  if (s < 3)   return 0.06 + 0.128 * s;          // thin tail, getting thicker
  if (s > 8.6) return 0.44 - (s - 8.6) * 0.09;   // slightly thinner neck
  return 0.44;
}

// where the middle of the body is at position s and time t
function centerAt(s, t, out) {
  const fade = s > 8 ? Math.max(0.15, 1 - (s - 8) * 0.45) : 1;   // head end wiggles less
  const x = 0.9 * fade * Math.sin(1.15 * s - t * 2.2);            // the S-shaped wave
  let y = radiusAt(s) * 0.85;                                     // resting on the ground
  if (s > 8.4) y += Math.pow(s - 8.4, 2) * 0.55;                  // neck lifts up
  return out.set(x, y, s - 6);
}

// number of triangles: tube sides + tail cap + neck cap
const BODY_TRIS = (SEGMENTS - 1) * SIDES * 2 + SIDES * 2;
const bodyPositions = new Float32Array(BODY_TRIS * 9);
const bodyColors    = new Float32Array(BODY_TRIS * 9);

const bodyGeo = new THREE.BufferGeometry();
bodyGeo.setAttribute('position', new THREE.BufferAttribute(bodyPositions, 3));
bodyGeo.setAttribute('color',    new THREE.BufferAttribute(bodyColors, 3));

// pick the colour of each side panel once (pattern depends only on ring + side)
//   panels 2 and 3 are underneath → belly
//   panels 0 and 5 are on top → alternating zig-zag
//   panels 1 and 4 are the sides → small side blotches
function panelColor(i, k) {
  if (k === 2 || k === 3) return COLORS.belly;
  const step = i % 4;
  if (k === 0) return step < 2 ? COLORS.dark : COLORS.green;
  if (k === 5) return step >= 2 ? COLORS.dark : COLORS.green;
  return step === 1 ? COLORS.dark : COLORS.green;
}
(function fillColors() {
  let p = 0;
  const put = col => { for (let v = 0; v < 3; v++) { bodyColors.set(col, p); p += 3; } };
  for (let i = 0; i < SEGMENTS - 1; i++)
    for (let k = 0; k < SIDES; k++) { put(panelColor(i, k)); put(panelColor(i, k)); }
  for (let k = 0; k < SIDES; k++) put(COLORS.dark);    // tail tip
  for (let k = 0; k < SIDES; k++) put(COLORS.green);   // neck cap (hidden by the head)
})();

// reusable vectors so we don't create garbage every frame
const centers = Array.from({ length: SEGMENTS }, () => new THREE.Vector3());
const rings = Array.from({ length: SEGMENTS }, () => Array.from({ length: SIDES }, () => new THREE.Vector3()));
const UP = new THREE.Vector3(0, 1, 0);
const tangent = new THREE.Vector3(), side = new THREE.Vector3(), up = new THREE.Vector3();
const tailTip = new THREE.Vector3();

function updateBody(t) {
  // 1) centre points along the path
  for (let i = 0; i < SEGMENTS; i++) centerAt((i / (SEGMENTS - 1)) * LENGTH, t, centers[i]);

  // 2) a ring of points around each centre
  for (let i = 0; i < SEGMENTS; i++) {
    const a = centers[Math.max(i - 1, 0)], b = centers[Math.min(i + 1, SEGMENTS - 1)];
    tangent.subVectors(b, a).normalize();
    side.crossVectors(tangent, UP).normalize();
    up.crossVectors(side, tangent).normalize();
    const r = radiusAt((i / (SEGMENTS - 1)) * LENGTH);
    for (let k = 0; k < SIDES; k++) {
      const ang = Math.PI / 2 + (2 * Math.PI * k) / SIDES;
      rings[i][k].copy(centers[i])
        .addScaledVector(side, Math.cos(ang) * r)
        .addScaledVector(up, Math.sin(ang) * r * 0.85);   // a little flatter than round
    }
  }

  // 3) write every triangle into the position array
  let p = 0;
  const put = v => { bodyPositions[p++] = v.x; bodyPositions[p++] = v.y; bodyPositions[p++] = v.z; };
  for (let i = 0; i < SEGMENTS - 1; i++) {
    for (let k = 0; k < SIDES; k++) {
      const a = rings[i][k], b = rings[i][(k + 1) % SIDES];
      const c = rings[i + 1][k], d = rings[i + 1][(k + 1) % SIDES];
      put(a); put(b); put(d);
      put(a); put(d); put(c);
    }
  }
  tangent.subVectors(centers[1], centers[0]).normalize();
  tailTip.copy(centers[0]).addScaledVector(tangent, -0.3);
  for (let k = 0; k < SIDES; k++) { put(rings[0][(k + 1) % SIDES]); put(rings[0][k]); put(tailTip); }
  const last = SEGMENTS - 1;
  for (let k = 0; k < SIDES; k++) { put(rings[last][k]); put(rings[last][(k + 1) % SIDES]); put(centers[last]); }

  // 4) tell Three.js the numbers changed
  bodyGeo.attributes.position.needsUpdate = true;
  bodyGeo.computeBoundingSphere();
}

/* =================================================================
   PART 2 — HEAD (right half + mirror), a flat wedge shape
   ================================================================= */
function buildHead() {
  const V = {
    C_topBack: [0, 0.25, -0.35], C_topFront: [0, 0.18, 0.55], C_snout: [0, 0.05, 0.75],
    C_chin: [0, -0.18, 0.55], C_bottomBack: [0, -0.22, -0.35],
    brow: [0.28, 0.20, 0.20], cheek: [0.42, 0.00, 0.00], back: [0.30, 0.02, -0.40],
    snoutSide: [0.15, 0.06, 0.65], jaw: [0.30, -0.15, 0.20], jawBack: [0.25, -0.18, -0.35],
  };
  const F = [
    ['C_topBack','C_topFront','brow','green'], ['C_topBack','brow','back','dark'],
    ['C_topFront','snoutSide','brow','green'], ['C_topFront','C_snout','snoutSide','green'],
    ['brow','snoutSide','cheek','green'], ['brow','cheek','back','dark'],
    ['snoutSide','jaw','cheek','belly'], ['cheek','jaw','jawBack','belly'],
    ['cheek','jawBack','back','green'],
    ['C_snout','snoutSide','jaw','belly'], ['C_snout','jaw','C_chin','belly'],
    ['C_chin','jaw','jawBack','belly'], ['C_chin','jawBack','C_bottomBack','belly'],
  ];
  const mirror = p => [-p[0], p[1], p[2]];
  const tris = [];
  for (const [a, b, c, col] of F) {
    tris.push({ a: V[a], b: V[b], c: V[c], color: COLORS[col] });
    tris.push({ a: mirror(V[a]), b: mirror(V[c]), c: mirror(V[b]), color: COLORS[col] });
  }
  return trisToGeometry(tris);
}

// small yellow eye with a dark slit
function buildEye() {
  const P = { top: [0, 0.07, 0], front: [0, 0, 0.08], bottom: [0, -0.07, 0], back: [0, 0, -0.08], out: [0.04, 0, 0] };
  const tris = [
    { a: P.top, b: P.front, c: P.out, color: COLORS.eye },
    { a: P.front, b: P.bottom, c: P.out, color: COLORS.eye },
    { a: P.bottom, b: P.back, c: P.out, color: COLORS.dark },
    { a: P.back, b: P.top, c: P.out, color: COLORS.dark },
  ];
  return trisToGeometry(tris);
}

// thin forked tongue made of flat triangles
function buildTongue() {
  const w = 0.018;
  const tris = [
    { a: [-w, 0, 0], b: [w, 0, 0], c: [0, 0, 0.40], color: COLORS.red },          // stem
    { a: [-w, 0, 0.30], b: [0, 0, 0.36], c: [-0.08, 0, 0.55], color: COLORS.red }, // left fork
    { a: [0, 0, 0.36], b: [w, 0, 0.30], c: [0.08, 0, 0.55], color: COLORS.red },   // right fork
  ];
  return trisToGeometry(tris);
}

/* =================================================================
   ASSEMBLE
   ================================================================= */
const mat = new THREE.MeshStandardMaterial({
  vertexColors: true, flatShading: true, roughness: 0.45, side: THREE.DoubleSide,
});

const snake = new THREE.Group();
snake.name = 'snake';

const body = new THREE.Mesh(bodyGeo, mat);
body.name = 'body';
body.castShadow = true;
body.frustumCulled = false;       // the shape changes every frame
snake.add(body);

const head = new THREE.Mesh(buildHead(), mat);
head.name = 'head';
head.castShadow = true;
snake.add(head);

const eyeGeo = buildEye();
const eyeR = new THREE.Mesh(eyeGeo, mat); eyeR.position.set( 0.33, 0.13, 0.30);
const eyeL = new THREE.Mesh(eyeGeo, mat); eyeL.position.set(-0.33, 0.13, 0.30); eyeL.scale.x = -1;
head.add(eyeR, eyeL);

const tongue = new THREE.Mesh(buildTongue(), mat);
tongue.position.set(0, -0.02, 0.68);
head.add(tongue);

// place the head at the front of the body, pointing along the neck
const headTarget = new THREE.Vector3();
function updateHead() {
  const a = centers[SEGMENTS - 2], b = centers[SEGMENTS - 1];
  tangent.subVectors(b, a).normalize();
  head.position.copy(b).addScaledVector(tangent, 0.25);
  headTarget.copy(head.position).add(tangent.setY(tangent.y * 0.3));   // keep the head fairly level
  head.lookAt(headTarget);                                              // +z points forward
}

updateBody(0);
updateHead();

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
scene.add(snake);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -8, right: 8, top: 8, bottom: -8, near: 1, far: 40 });
scene.add(key);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

// edge overlay: a wireframe copy that shares each geometry, so it follows the animation
const wireMat = new THREE.MeshBasicMaterial({ color: 0x3a6ff0, wireframe: true });
const wires = [];
snake.traverse(o => {
  if (!o.isMesh || o.userData.isWire) return;
  const w = new THREE.Mesh(o.geometry, wireMat);
  w.userData.isWire = true; w.visible = false; w.frustumCulled = false;
  o.add(w); wires.push(w);
});

let triCount = 0;
snake.traverse(o => { if (o.isMesh && !o.userData.isWire) triCount += o.geometry.attributes.position.count / 3; });
document.getElementById('stats').textContent =
  `${SEGMENTS} rings, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 4;
controls.maxDistance = 30;
controls.maxPolarAngle = Math.PI * 0.49;
controls.autoRotateSpeed = 2;
controls.target.set(0, 0.6, 0);

const VIEWS = {
  angle: new THREE.Vector3(8, 6.5, 10),
  front: new THREE.Vector3(0, 2.2, 14),
  side:  new THREE.Vector3(-15, 2.2, 0),
  top:   new THREE.Vector3(0, 16, 0.01),
};
let target = VIEWS.angle.clone();
camera.position.copy(target);
let moving = false;
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

window.addEventListener('resize', () => {
  camera.aspect = stage.clientWidth / stage.clientHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(stage.clientWidth, stage.clientHeight);
});

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
  const on = !wires[0].visible;
  wires.forEach(w => (w.visible = on));
  edgesBtn.setAttribute('aria-pressed', String(on));
});

let slithering = !reduceMotion;
const slitherBtn = document.getElementById('slither');
slitherBtn.setAttribute('aria-pressed', String(slithering));
slitherBtn.addEventListener('click', () => {
  slithering = !slithering;
  slitherBtn.setAttribute('aria-pressed', String(slithering));
});

/* =================================================================
   RENDER LOOP
   ================================================================= */
const clock = new THREE.Clock();
let waveTime = 0;
(function loop() {
  requestAnimationFrame(loop);
  const dt = Math.min(clock.getDelta(), 0.05);
  const t = clock.elapsedTime;

  if (slithering) {
    waveTime += dt;
    updateBody(waveTime);        // rewrite the body's vertex positions
    updateHead();
    // tongue flicks out for half a second every 2.5 seconds
    const phase = t % 2.5;
    const out = phase < 0.5 ? Math.sin((phase / 0.5) * Math.PI) : 0;
    tongue.scale.set(1, 1, Math.max(out, 0.001));
    tongue.visible = out > 0.02;
  } else {
    tongue.visible = false;
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