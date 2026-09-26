# Bird

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
<title>Low-poly bird — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #3f6fd8;
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
  <h1>Low-poly bird</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="wag" aria-pressed="false">Fly</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates for every part: x = right, y = up, z = forward
   (the bird faces +z). All parts use the same units.
   ================================================================= */

const COLORS = {
  blue:     [0.25, 0.46, 0.86],   // head, back, wings
  darkBlue: [0.14, 0.27, 0.60],   // flight feathers, tail
  orange:   [0.94, 0.55, 0.24],   // chest and throat
  white:    [0.95, 0.93, 0.90],   // belly
  beak:     [0.26, 0.24, 0.22],
  black:    [0.05, 0.05, 0.05],   // eyes
  leg:      [0.42, 0.34, 0.30],
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
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => COLORS.blue } = {}) {
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
   PART 1 — BODY + HEAD in one loft (a bird's neck is hidden in feathers)
   From the tail end, through a round belly, up to the round head.
   ================================================================= */
const buildBody = () => loft([
  [0.90, -1.50, 0.18, 0.15],   // base of the tail
  [0.75, -1.10, 0.45, 0.40],
  [0.75, -0.40, 0.75, 0.75],   // round belly
  [0.95,  0.30, 0.72, 0.80],   // chest
  [1.35,  0.75, 0.50, 0.55],   // neck
  [1.75,  0.95, 0.52, 0.52],   // back of the head
  [1.90,  1.35, 0.45, 0.45],
  [1.85,  1.65, 0.25, 0.25],   // face
], {
  sides: 7,
  startCap: [0.95, -1.65], endCap: [1.82, 1.75],
  colorFor: (i, c, ring) => {
    const below = c.y < ring.y - 0.15;
    if (below && i >= 2 && i <= 4 && c.z > -0.1) return COLORS.orange;  // chest + throat
    if (below && i <= 2)                          return COLORS.white;   // belly
    return COLORS.blue;
  },
});

// short pointed beak
const buildBeak = () => loft([
  [ 0.00, 0.00, 0.13, 0.10],
  [-0.03, 0.20, 0.08, 0.06],
], { startCap: 'flat', endCap: [-0.06, 0.42], colorFor: () => COLORS.beak });

// tiny eye: a short double-pointed shape
const buildEye = () => loft([
  [0, -0.04, 0.09, 0.09],
  [0,  0.04, 0.09, 0.09],
], { startCap: [0, -0.10], endCap: [0, 0.10], colorFor: () => COLORS.black });

/* =================================================================
   PART 2 — WING. Written by hand as a flat, slightly ridged shape,
   drawn "spread out" along +x. Rotating it backward folds it.
   ================================================================= */
function buildWing() {
  const P = {
    rootF: [0.0, 0.00,  0.35], rootB: [0.0, 0.00, -0.45],
    ridge: [0.8, 0.12,  0.10],                                // small bump on top
    midF:  [1.0, 0.05,  0.40], midB:  [1.0, 0.00, -0.70],
    f1:    [1.6, 0.00, -0.85], f2:    [2.0, 0.00, -0.65],     // feather tips
    tip:   [2.1, 0.00, -0.30],
  };
  const T = [
    ['rootF', 'ridge', 'midF', 'blue'], ['rootF', 'rootB', 'ridge', 'blue'],
    ['rootB', 'midB', 'ridge', 'blue'], ['ridge', 'midB', 'midF', 'blue'],
    ['midF', 'midB', 'f1', 'darkBlue'], ['midF', 'f1', 'f2', 'darkBlue'],
    ['midF', 'f2', 'tip', 'darkBlue'],
  ];
  return trisToGeometry(T.map(([a, b, c, col]) => ({ a: P[a], b: P[b], c: P[c], color: COLORS[col] })));
}

// fan-shaped tail feathers
function buildTailFeathers() {
  const P = {
    root: [0, 0, 0], ridge: [0, 0.08, -0.6],
    a: [-0.38, -0.05, -1.10], b: [-0.13, 0, -1.25], c: [0.13, 0, -1.25], d: [0.38, -0.05, -1.10],
  };
  const T = [['root','a','ridge'], ['ridge','a','b'], ['ridge','b','c'], ['ridge','c','d'], ['root','ridge','d']];
  return trisToGeometry(T.map(([a, b, c]) => ({ a: P[a], b: P[b], c: P[c], color: COLORS.darkBlue })));
}

/* =================================================================
   PART 3 — LEG with 3 toes forward and 1 back
   ================================================================= */
function buildLeg() {
  const leg = new THREE.Group();
  const opts = { sides: 5, colorFor: () => COLORS.leg };
  const shank = loft([[0, 0, 0.10, 0.12], [-0.35, 0.05, 0.06, 0.06], [-0.78, -0.02, 0.04, 0.04]],
                     { ...opts, startCap: 'flat', endCap: 'flat' });
  leg.add(new THREE.Mesh(shank, solidMat));
  const toeGeo = loft([[-0.80, 0.00, 0.035, 0.03], [-0.80, 0.28, 0.025, 0.02]],
                      { ...opts, startCap: 'flat', endCap: [-0.80, 0.36] });
  for (const angle of [-0.45, 0, 0.45, Math.PI]) {         // three forward, one back
    const toe = new THREE.Mesh(toeGeo, solidMat);
    toe.rotation.y = angle;
    toe.scale.setScalar(angle === Math.PI ? 0.8 : 1);
    leg.add(toe);
  }
  leg.traverse(o => { if (o.isMesh) o.castShadow = true; });
  return leg;
}

/* =================================================================
   ASSEMBLE — one THREE.Group, each part placed where it joins
   ================================================================= */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.7 });
const thinMat = solidMat.clone(); thinMat.side = THREE.DoubleSide;   // wings and tail are flat sheets

function part(name, geometry, material, [x, y, z]) {
  const m = geometry.isObject3D ? geometry : new THREE.Mesh(geometry, material);
  m.name = name;
  m.position.set(x, y, z);
  m.castShadow = true;
  return m;
}

const bird = new THREE.Group();
bird.name = 'bird';

const eyeGeo = buildEye();
const wingGeo = buildWing();

bird.add(part('body', buildBody(), solidMat, [0, 0, 0]));
bird.add(part('beak', buildBeak(), solidMat, [0, 1.80, 1.68]));
const eyeL = part('eyeL', eyeGeo, solidMat, [-0.40, 1.95, 1.40]); eyeL.rotation.y = Math.PI / 2;
const eyeR = part('eyeR', eyeGeo, solidMat, [ 0.40, 1.95, 1.40]); eyeR.rotation.y = Math.PI / 2;
bird.add(eyeL, eyeR);

const tailFeathers = part('tailFeathers', buildTailFeathers(), thinMat, [0, 0.95, -1.40]);
tailFeathers.rotation.x = -0.2;             // tilt the tail up a little
bird.add(tailFeathers);

// wings: the left one is the right one mirrored with scale.x = -1
const wingR = part('wingR', wingGeo, thinMat, [ 0.66, 1.20, 0.30]);
const wingL = part('wingL', wingGeo, thinMat, [-0.66, 1.20, 0.30]);
wingL.scale.x = -1;
wingR.rotation.order = wingL.rotation.order = 'YXZ';   // flap (z), then twist (x), then swing (y)
bird.add(wingR, wingL);

bird.add(part('legL', buildLeg(), null, [-0.30, 0.30, -0.20]));
bird.add(part('legR', buildLeg(), null, [ 0.30, 0.30, -0.20]));

// folded vs spread wing poses
//   x = twist so the wing stands up flat against the body
//   y = swing backward along the body
//   z = lift (used for flapping)
const FOLDED = { x: -1.50, y: 1.62, z: -0.18 };
const SPREAD = { x:  0.00, y: 0.15, z:  0.00 };
function poseWings(spread, flap) {
  const mix = k => FOLDED[k] + (SPREAD[k] - FOLDED[k]) * spread;
  const x = mix('x'), y = mix('y'), z = mix('z') + flap;
  wingR.rotation.set(x,  y,  z);
  wingL.rotation.set(x, -y, -z);   // mirrored side: y and z flip, x stays
}
poseWings(0, 0);

// stand the bird on y = 0 and centre it front-to-back
const box = new THREE.Box3().setFromObject(bird);
bird.position.y = -box.min.y;
bird.position.z = -(box.min.z + box.max.z) / 2;

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
scene.add(bird);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -4, right: 4, top: 4, bottom: -4, near: 1, far: 40 });
scene.add(key);
const rim = new THREE.DirectionalLight(0xb8c4ff, 0.35);
rim.position.set(-8, 4, -6);
scene.add(rim);

const ground = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.ShadowMaterial({ opacity: 0.22 }));
ground.rotation.x = -Math.PI / 2;
ground.receiveShadow = true;
scene.add(ground);

const edgeLines = [];
bird.traverse(o => {
  if (!o.isMesh) return;
  const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1),
                                   new THREE.LineBasicMaterial({ color: 0x3a6ff0 }));
  l.visible = false; o.add(l); edgeLines.push(l);
});

let triCount = 0, partCount = 0;
bird.traverse(o => { if (o.isMesh) { partCount++; triCount += o.geometry.attributes.position.count / 3; } });
document.getElementById('stats').textContent =
  `${partCount} meshes, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 3;
controls.maxDistance = 20;
controls.maxPolarAngle = Math.PI * 0.49;   // don't go under the floor
controls.autoRotateSpeed = 2;
const center = new THREE.Vector3(0, 1.3, 0);
controls.target.copy(center);

const VIEWS = {
  angle: new THREE.Vector3(5.5, 3.8, 7),
  front: new THREE.Vector3(0, 1.6, 10),
  side:  new THREE.Vector3(-10, 1.6, 0),
  top:   new THREE.Vector3(0, 11, 0.01),
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

const baseY = bird.position.y;
let wagging = false;   // "wagging" = flying for the bird
let spread = 0;
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
  spread += ((wagging ? 1 : 0) - spread) * 0.08;            // open or fold the wings smoothly
  const flap = reduceMotion ? 0 : Math.sin(t * 12) * 0.8 * spread;
  poseWings(spread, flap);
  bird.position.y = baseY + spread * (0.9 + Math.sin(t * 12) * 0.08);   // hover while flapping
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