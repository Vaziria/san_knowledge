# Frog Figure.

Draft for the user to correct: Claude wrote it on 2026-09-25 from the frog task and the code, and brought it in line with the shape reference below (the user's) on 2026-09-26. A low poly pond frog 8.6 cm long sitting and 5.4 cm to the top of its eyes: a squat green body and head in one, tilted up toward the front, with darker green spots on its back, a pale belly and throat, bulging gold eyes with black pupils on top of its head, short front legs ending in flat hands, and long hind legs folded at its sides, with webbed feet. It hops rather than walks, and between hops it sits still.

## Animation Behavior.
2. `Jump()`, one high leap, the hind legs flung out straight and folded again on landing.

    Example:
    `frog.Jump()`
3. `Hold(Figure figure)`, it carries the figure in its mouth, scaled down to about its own length.

    Example:
    `frog.Hold(fish)`
4. `Walk()`, a string of small hops forward, until `Run()` or `Stop()`.

    Example:
    `frog.Walk()`
5. `Run()`, long, quick hops forward.

    Example:
    `frog.Run()`
6. `Speech(text string)`, its show Speech Bubble and show text, when text to long, its split to multiple Bubble that show and hide sequentially

    Example:
    `frog.Speech("Ribbit!")`
7. `Stop()`, it stops hopping and sits.

    Example:
    `frog.Stop()`


# Shape reference
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Low-poly frog — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #3f8a2c;
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
  <h1>Low-poly frog</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="wag" aria-pressed="true">Hop</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates for every part: x = right, y = up, z = forward
   (the frog faces +z). All parts use the same units.
   ================================================================= */

const COLORS = {
  green:     [0.36, 0.62, 0.22],   // back and legs
  darkGreen: [0.20, 0.40, 0.14],   // spots, feet
  cream:     [0.91, 0.87, 0.60],   // belly and throat
  gold:      [0.95, 0.74, 0.22],   // eyes
  black:     [0.07, 0.07, 0.07],   // pupils
};

// a repeatable "random" number from a position — used to scatter spots
const noise = c => { const v = Math.sin(c.x * 12.99 + c.y * 4.1 + c.z * 78.23) * 43758.55; return v - Math.floor(v); };

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
function loft(sections, { sides = 6, startCap = null, endCap = null, colorFor = () => COLORS.green } = {}) {
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
   PART 1 — BODY + HEAD in one piece (a frog has no neck).
   It tilts up toward the front, like a frog sitting.
   ================================================================= */
const buildBody = () => loft([
  [0.40, -1.60, 0.50, 0.40],   // rear
  [0.60, -1.20, 1.10, 0.75],
  [0.85, -0.30, 1.30, 0.85],   // widest part of the belly
  [1.05,  0.60, 1.25, 0.75],   // shoulders
  [1.10,  1.30, 1.05, 0.55],   // wide flat head
  [1.00,  1.85, 0.60, 0.30],   // snout
], {
  startCap: [0.35, -1.80], endCap: [0.95, 2.05],
  colorFor: (i, c, ring) =>
    c.y < ring.y - 0.15 ? COLORS.cream :              // pale underside
    noise(c) < 0.3     ? COLORS.darkGreen : COLORS.green,   // spots on top
});

/* =================================================================
   PART 2 — BULGING EYE (a short tube pointing up)
   ================================================================= */
const buildEye = () => loft([
  [0.00, 0.00, 0.30, 0.30],
  [0.18, 0.00, 0.33, 0.33],
  [0.36, 0.00, 0.24, 0.24],
], {
  startCap: 'flat', endCap: [0.44, 0.00],
  colorFor: (i, c) =>
    i === 0                  ? COLORS.green :        // eyelid
    c.z > 0.2 && c.y > 0.22 ? COLORS.black : COLORS.gold,
});

/* =================================================================
   PART 3 — FRONT LEG (short, ends in a flat hand)
   ================================================================= */
const buildFrontLeg = () => loft([
  [ 0.00, 0.00, 0.22, 0.25],
  [-0.35, 0.12, 0.17, 0.19],
  [-0.62, 0.20, 0.13, 0.15],
  [-0.72, 0.38, 0.24, 0.07],   // flat hand
  [-0.74, 0.60, 0.30, 0.05],
], {
  startCap: 'flat', endCap: [-0.75, 0.75],
  colorFor: i => (i >= 3 ? COLORS.darkGreen : COLORS.green),
});

/* =================================================================
   PART 4 — BACK LEG, folded like a sitting frog.
   Three lofts: thigh goes forward, shin comes back beside it,
   and the long webbed foot lies flat on the ground.
   ================================================================= */
function buildBackLeg() {
  const leg = new THREE.Group();
  const thigh = loft([
    [ 0.00, 0.00, 0.38, 0.42],
    [-0.15, 0.50, 0.34, 0.36],
    [-0.25, 0.95, 0.26, 0.26],   // knee
  ], {
    startCap: 'flat', endCap: [-0.27, 1.10],
    colorFor: (i, c, ring) => (c.y < ring.y - 0.1 ? COLORS.cream : COLORS.green),
  });
  const shin = loft([
    [-0.25, 1.00, 0.22, 0.24],
    [-0.33, 0.45, 0.20, 0.22],
    [-0.40,-0.05, 0.16, 0.18],   // ankle
  ], { startCap: [-0.20, 1.15], endCap: 'flat' });
  const foot = loft([
    [-0.42,-0.10, 0.15, 0.10],
    [-0.47, 0.40, 0.22, 0.06],
    [-0.47, 0.90, 0.40, 0.05],   // wide webbed toes
  ], { endCap: [-0.47, 1.15], startCap: 'flat', colorFor: () => COLORS.darkGreen });

  leg.add(new THREE.Mesh(thigh, solidMat));
  const shinMesh = new THREE.Mesh(shin, solidMat); shinMesh.position.x = 0.38; leg.add(shinMesh);
  const footMesh = new THREE.Mesh(foot, solidMat); footMesh.position.x = 0.50; leg.add(footMesh);
  leg.traverse(o => { if (o.isMesh) o.castShadow = true; });
  return leg;
}

/* =================================================================
   ASSEMBLE — one THREE.Group, each part placed where it joins
   ================================================================= */
const solidMat = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, roughness: 0.55 });

function part(name, geometry, [x, y, z], rot = [0, 0, 0]) {
  const m = geometry.isObject3D ? geometry : new THREE.Mesh(geometry, solidMat);
  m.name = name;
  m.position.set(x, y, z);
  m.rotation.set(...rot);
  m.castShadow = true;
  return m;
}

const frog = new THREE.Group();
frog.name = 'frog';

const eyeGeo = buildEye();
const frontLegGeo = buildFrontLeg();

frog.add(part('body', buildBody(), [0, 0, 0]));
frog.add(part('eyeL', eyeGeo, [-0.55, 1.55, 1.15]));
frog.add(part('eyeR', eyeGeo, [ 0.55, 1.55, 1.15]));
frog.add(part('frontLegL', frontLegGeo, [-0.85, 0.70, 0.90], [0, 0,  0.35]));  // splay outward
frog.add(part('frontLegR', frontLegGeo, [ 0.85, 0.70, 0.90], [0, 0, -0.35]));
const backR = part('backLegR', buildBackLeg(), [ 1.00, 0.40, -1.15], [0,  0.45, 0]);
const backL = part('backLegL', buildBackLeg(), [-1.00, 0.40, -1.15], [0, -0.45, 0]);
backR.scale.set( 1.4, 1.4, 1.4);  // big powerful jumping legs
backL.scale.set(-1.4, 1.4, 1.4);  // negative x mirrors the right leg into a left one
frog.add(backR, backL);

// stand the frog on y = 0 and centre it front-to-back
const box = new THREE.Box3().setFromObject(frog);
frog.position.y = -box.min.y;
frog.position.z = -(box.min.z + box.max.z) / 2;

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
scene.add(frog);

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
frog.traverse(o => {
  if (!o.isMesh) return;
  const l = new THREE.LineSegments(new THREE.EdgesGeometry(o.geometry, 1),
                                   new THREE.LineBasicMaterial({ color: 0x3a6ff0 }));
  l.visible = false; o.add(l); edgeLines.push(l);
});

let triCount = 0, partCount = 0;
frog.traverse(o => { if (o.isMesh) { partCount++; triCount += o.geometry.attributes.position.count / 3; } });
document.getElementById('stats').textContent =
  `${partCount} meshes, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 4;
controls.maxDistance = 25;
controls.maxPolarAngle = Math.PI * 0.49;   // don't go under the floor
controls.autoRotateSpeed = 2;
const center = new THREE.Vector3(0, 1.0, 0);
controls.target.copy(center);

const VIEWS = {
  angle: new THREE.Vector3(6.5, 4.5, 8),
  front: new THREE.Vector3(0, 1.6, 11.5),
  side:  new THREE.Vector3(-11.5, 1.6, 0),
  top:   new THREE.Vector3(0, 12.5, 0.01),
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

const baseY = frog.position.y;
let wagging = !reduceMotion;   // "wagging" = hopping for the frog
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
  // hop: jump for the first 40% of every 1.6 s, then rest
  const phase = (t % 1.6) / 1.6;
  const up = wagging && phase < 0.4 ? Math.sin((phase / 0.4) * Math.PI) : 0;
  frog.position.y = baseY + up * 1.1;
  frog.rotation.x = -up * 0.25;                          // nose tips up mid-air
  backR.rotation.x = backL.rotation.x = up * 0.6;        // legs swing back
  frog.scale.set(1, 1 - (wagging && phase > 0.4 && phase < 0.5 ? 0.08 : 0), 1); // small squash on landing
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
