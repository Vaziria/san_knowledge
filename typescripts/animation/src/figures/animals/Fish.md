# Fish

# Shape Reference (Salmon & Piranha)
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Low-poly piranha and salmon — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #c9453a;
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
  <h1>Low-poly piranha and salmon</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="swim" aria-pressed="true">Swim</button>
  <span class="sep" aria-hidden="true"></span>
  <button data-show="both" aria-pressed="true">Both</button>
  <button data-show="piranha" aria-pressed="false">Piranha</button>
  <button data-show="salmon" aria-pressed="false">Salmon</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates: x = right, y = up, z = forward (fish face +z).
   Both fish are made by ONE factory function, createFish(spec).
   The spec describes the shape, colours, fins and swimming style;
   the factory builds a body BufferGeometry that is rewritten every
   frame (like the snake and clownfish) so the tail can sway.
   ================================================================= */

/* -----------------------------------------------------------------
   HELPERS
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

// repeatable "random" number between 0 and 1 (used for spots)
const hash = n => { const v = Math.sin(n * 127.1) * 43758.5453; return v - Math.floor(v); };

// fin outline: triangle (sharp peak) or round (smooth hump); u runs 0 → 1 from back to front
const finShape = (type, peak = 0.6) => u =>
  type === 'round' ? Math.sin(Math.PI * u)
                   : (u < peak ? u / peak : (1 - u) / (1 - peak));

/* -----------------------------------------------------------------
   Tail fin: a forked flat sheet, drawn standing up and pointing -z
   ----------------------------------------------------------------- */
function buildTailFin({ size = 1, fork = 0.58, inner, edge }) {
  const s = size;
  const P = {
    rootT: [0, 0.22 * s, 0], rootB: [0, -0.22 * s, 0],
    iT: [0, 0.60 * s, -0.72 * s], iB: [0, -0.60 * s, -0.72 * s], iN: [0, 0, -fork * 0.78 * s],
    tT: [0, 0.80 * s, -0.98 * s], tB: [0, -0.80 * s, -0.98 * s], notch: [0, 0, -fork * s],
  };
  const T = [
    ['rootT','rootB','iN',inner], ['rootT','iN','iT',inner], ['rootB','iB','iN',inner],
    ['iT','iN','notch',edge], ['iT','notch','tT',edge], ['iB','notch','iN',edge],
    ['iB','tB','notch',edge], ['rootT','iT','tT',edge], ['rootB','tB','iB',edge],
  ];
  return trisToGeometry(T.map(([a, b, c, col]) => ({ a: P[a], b: P[b], c: P[c], color: col })));
}

// small paddle-shaped side fin
function buildPectoralFin(color, size = 1) {
  const s = size;
  const P = {
    rt: [0, 0.10 * s, 0], rb: [0, -0.10 * s, 0],
    a: [0.12 * s, 0.18 * s, -0.40 * s], b: [0.22 * s, 0, -0.55 * s], c: [0.12 * s, -0.16 * s, -0.40 * s],
  };
  const T = [['rt','rb','b'], ['rt','b','a'], ['rb','c','b']];
  return trisToGeometry(T.map(([a, b, c]) => ({ a: P[a], b: P[b], c: P[c], color })));
}

// eye: coloured ring (iris) with a black pupil
function buildEye({ iris, radius = 0.13 }) {
  const tris = [], N = 6, BLACK = [0.04, 0.04, 0.05];
  const ring = (r, x) => [...Array(N).keys()].map(k => {
    const a = (2 * Math.PI * k) / N;
    return [x, Math.cos(a) * r, Math.sin(a) * r];
  });
  const outer = ring(radius, 0), inner = ring(radius * 0.55, 0.025), centre = [0.04, 0, 0];
  for (let k = 0; k < N; k++) {
    const k2 = (k + 1) % N;
    tris.push({ a: outer[k], b: outer[k2], c: inner[k2], color: iris });
    tris.push({ a: outer[k], b: inner[k2], c: inner[k], color: iris });
    tris.push({ a: centre, b: inner[k], c: inner[k2], color: BLACK });
  }
  return trisToGeometry(tris);
}

/* =================================================================
   THE FACTORY
   ================================================================= */
function createFish(spec) {
  const { profile, noseTip, rings: RINGS = 36, sides: SIDES = 8 } = spec;
  const zTail = profile[0][0], zNose = profile[profile.length - 1][0];
  const ringZ = i => zTail + (zNose - zTail) * (i / (RINGS - 1));

  // blend between profile rows: returns [rx, ry, yOffset]
  const sizeAt = z => {
    for (let i = 0; i < profile.length - 1; i++) {
      const a = profile[i], b = profile[i + 1];
      if (z <= b[0]) {
        const f = (z - a[0]) / (b[0] - a[0]);
        return [1, 2, 3].map(j => a[j] + (b[j] - a[j]) * f);
      }
    }
    return profile[profile.length - 1].slice(1);
  };

  // swimming: the head end stays still, the tail end swings
  const { amplitude, speed, wavelength, stillFrom } = spec.swim;
  const sideOffset = (z, t) => {
    const amount = Math.max(0, (stillFrom - z) / (stillFrom - zTail));
    return amplitude * amount * amount * Math.sin(wavelength * z - t * speed);
  };

  // which rings each fin grows from
  const fins = spec.fins.map(f => ({
    ...f,
    list: [...Array(RINGS).keys()].filter(i => ringZ(i) >= f.from && ringZ(i) <= f.to),
    corner: f.side === 'top' ? 0 : SIDES / 2,
    dir: f.side === 'top' ? 1 : -1,
  }));

  const triCount = (RINGS - 1) * SIDES * 2 + SIDES * 2 +
                   fins.reduce((n, f) => n + (f.list.length - 1) * 4, 0);
  const positions = new Float32Array(triCount * 9);
  const colors = new Float32Array(triCount * 9);
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
  geo.setAttribute('color', new THREE.BufferAttribute(colors, 3));

  // colours: fixed, written once in the same order as the positions
  {
    let p = 0;
    const put = col => { for (let v = 0; v < 3; v++) { colors.set(col, p); p += 3; } };
    for (let i = 0; i < RINGS - 1; i++) {
      const z = (ringZ(i) + ringZ(i + 1)) / 2;
      for (let k = 0; k < SIDES; k++) {
        const height = Math.sin(Math.PI / 2 + (2 * Math.PI * (k + 0.5)) / SIDES);   // +1 top … -1 belly
        const col = spec.colorAt(z, height, hash(i * 13.7 + k * 3.1));
        put(col); put(col);
      }
    }
    for (let k = 0; k < SIDES * 2; k++) put(spec.capColor);
    for (const f of fins)
      for (let j = 0; j < f.list.length - 1; j++) { put(f.inner); put(f.inner); put(f.edge); put(f.edge); }
  }

  // reusable vectors
  const centers = Array.from({ length: RINGS }, () => new THREE.Vector3());
  const ups = Array.from({ length: RINGS }, () => new THREE.Vector3());
  const ringPts = Array.from({ length: RINGS }, () => Array.from({ length: SIDES }, () => new THREE.Vector3()));
  const UP = new THREE.Vector3(0, 1, 0), tangent = new THREE.Vector3(), side = new THREE.Vector3();
  const A = new THREE.Vector3(), B = new THREE.Vector3(), C = new THREE.Vector3(), D = new THREE.Vector3();
  const tip = new THREE.Vector3(0, noseTip[1], noseTip[0]);

  function updateBody(t) {
    for (let i = 0; i < RINGS; i++) {
      const z = ringZ(i);
      centers[i].set(sideOffset(z, t), sizeAt(z)[2], z);
    }
    for (let i = 0; i < RINGS; i++) {
      tangent.subVectors(centers[Math.min(i + 1, RINGS - 1)], centers[Math.max(i - 1, 0)]).normalize();
      side.crossVectors(tangent, UP).normalize();
      ups[i].crossVectors(side, tangent).normalize();
      const [rx, ry] = sizeAt(ringZ(i));
      for (let k = 0; k < SIDES; k++) {
        const ang = Math.PI / 2 + (2 * Math.PI * k) / SIDES;
        ringPts[i][k].copy(centers[i])
          .addScaledVector(side, Math.cos(ang) * rx)
          .addScaledVector(ups[i], Math.sin(ang) * ry);
      }
    }
    let p = 0;
    const put = v => { positions[p++] = v.x; positions[p++] = v.y; positions[p++] = v.z; };
    for (let i = 0; i < RINGS - 1; i++)
      for (let k = 0; k < SIDES; k++) {
        const a = ringPts[i][k], b = ringPts[i][(k + 1) % SIDES];
        const c = ringPts[i + 1][k], d = ringPts[i + 1][(k + 1) % SIDES];
        put(a); put(b); put(d); put(a); put(d); put(c);
      }
    const last = RINGS - 1;
    for (let k = 0; k < SIDES; k++) { put(ringPts[last][k]); put(ringPts[last][(k + 1) % SIDES]); put(tip); }
    for (let k = 0; k < SIDES; k++) { put(ringPts[0][(k + 1) % SIDES]); put(ringPts[0][k]); put(centers[0]); }

    for (const f of fins) {
      for (let j = 0; j < f.list.length - 1; j++) {
        const i0 = f.list[j], i1 = f.list[j + 1];
        const u = i => (ringZ(i) - f.from) / (f.to - f.from);
        const h0 = f.height * f.shape(u(i0)) * f.dir, h1 = f.height * f.shape(u(i1)) * f.dir;
        const b0 = ringPts[i0][f.corner], b1 = ringPts[i1][f.corner];
        A.copy(b0).addScaledVector(ups[i0], h0 * 0.8); B.copy(b1).addScaledVector(ups[i1], h1 * 0.8);
        C.copy(b0).addScaledVector(ups[i0], h0);       D.copy(b1).addScaledVector(ups[i1], h1);
        put(b0); put(b1); put(B); put(b0); put(B); put(A);
        put(A); put(B); put(D); put(A); put(D); put(C);
      }
    }
    geo.attributes.position.needsUpdate = true;
    geo.computeBoundingSphere();
  }

  /* ---- build the meshes ---- */
  const group = new THREE.Group();
  group.name = spec.name;

  const body = new THREE.Mesh(geo, fishMat);
  body.name = 'body'; body.castShadow = true; body.frustumCulled = false;
  group.add(body);

  const tailFin = new THREE.Mesh(buildTailFin(spec.tail), fishMat);
  tailFin.name = 'tailFin'; tailFin.castShadow = true;
  group.add(tailFin);

  const [pz, py] = spec.pectoral.at;
  const px = sizeAt(pz)[0] * 0.92;
  const pecGeo = buildPectoralFin(spec.pectoral.color, spec.pectoral.size);
  const finR = new THREE.Mesh(pecGeo, fishMat); finR.position.set( px, py, pz);
  const finL = new THREE.Mesh(pecGeo, fishMat); finL.position.set(-px, py, pz); finL.scale.x = -1;
  group.add(finR, finL);

  const [ez, ey] = spec.eye.at;
  const [erx, ery, eoff] = sizeAt(ez);
  const ex = erx * Math.sqrt(Math.max(0, 1 - ((ey - eoff) / ery) ** 2)) * 0.97;   // on the oval's surface
  const eyeGeo = buildEye(spec.eye);
  const eyeR = new THREE.Mesh(eyeGeo, fishMat); eyeR.position.set( ex, ey, ez);
  const eyeL = new THREE.Mesh(eyeGeo, fishMat); eyeL.position.set(-ex, ey, ez); eyeL.scale.x = -1;
  group.add(eyeR, eyeL);

  if (spec.extras) spec.extras(group, sizeAt);

  const finTarget = new THREE.Vector3();
  function update(t) {
    updateBody(t);
    tangent.subVectors(centers[1], centers[0]).normalize();
    tailFin.position.copy(centers[0]);
    tailFin.lookAt(finTarget.copy(centers[0]).add(tangent));
    tailFin.rotateY(-0.35 * Math.sin(wavelength * zTail - t * speed));
    const flap = 0.55 + Math.sin(t * spec.pectoral.flapSpeed) * 0.3;
    finR.rotation.y = flap; finL.rotation.y = -flap;
  }
  update(0);
  return { group, update };
}

/* =================================================================
   SHARED MATERIAL
   ================================================================= */
const fishMat = new THREE.MeshStandardMaterial({
  vertexColors: true, flatShading: true, roughness: 0.35, metalness: 0.15, side: THREE.DoubleSide,
});

/* =================================================================
   PIRANHA — short, very deep body, blunt head, big underbite with teeth
   ================================================================= */
const P_COL = {
  back:   [0.30, 0.34, 0.37],
  silver: [0.62, 0.66, 0.68],
  speck:  [0.45, 0.49, 0.52],
  red:    [0.86, 0.24, 0.14],
  fin:    [0.28, 0.30, 0.32],
  finRed: [0.80, 0.25, 0.16],
  black:  [0.08, 0.08, 0.09],
  tooth:  [0.97, 0.96, 0.90],
};

const piranha = createFish({
  name: 'piranha',
  //        z     rx    ry    y-offset
  profile: [[-1.60, 0.08, 0.22,  0.00],
            [-1.20, 0.20, 0.55,  0.00],
            [-0.50, 0.38, 1.00, -0.05],
            [ 0.20, 0.46, 1.15, -0.10],   // very deep body
            [ 0.90, 0.42, 0.95, -0.12],
            [ 1.40, 0.34, 0.70, -0.15],
            [ 1.75, 0.24, 0.45, -0.20]],  // blunt face
  noseTip: [1.88, -0.22],
  swim: { amplitude: 0.28, speed: 8, wavelength: 2.4, stillFrom: 0.6 },
  colorAt: (z, h, rnd) =>
    h < -0.35 ? P_COL.red :                       // red belly
    h >  0.55 ? P_COL.back :                      // dark back
    rnd < 0.25 ? P_COL.speck : P_COL.silver,      // speckled silver sides
  capColor: P_COL.silver,
  fins: [
    { side: 'top',    from: -0.95, to:  0.15, height: 0.50, shape: finShape('tri', 0.75), inner: P_COL.fin,    edge: P_COL.black },
    { side: 'top',    from: -1.45, to: -1.22, height: 0.14, shape: finShape('round'),     inner: P_COL.fin,    edge: P_COL.fin },  // adipose fin
    { side: 'bottom', from: -1.35, to: -0.35, height: 0.48, shape: finShape('tri', 0.8),  inner: P_COL.finRed, edge: P_COL.black },
  ],
  tail: { size: 0.95, fork: 0.45, inner: P_COL.fin, edge: P_COL.black },
  pectoral: { at: [1.05, -0.55], size: 0.9, color: P_COL.finRed, flapSpeed: 7 },
  eye: { at: [1.35, 0.12], iris: [0.85, 0.20, 0.10], radius: 0.13 },

  // underbite jaw + two rows of triangular teeth
  extras(group, sizeAt) {
    const tris = [];
    const jaw = {
      tl: [0.20, -0.30, 1.30], tr: [-0.20, -0.30, 1.30],   // back of the jaw (top)
      fl: [0.17, -0.33, 2.02], fr: [-0.17, -0.33, 2.02],   // front of the jaw (top)
      bl: [0.18, -0.62, 1.35], br: [-0.18, -0.62, 1.35],   // bottom back
      cf: [0.00, -0.56, 1.95],                              // chin
    };
    const q = (a, b, c, col) => tris.push({ a: jaw[a], b: jaw[b], c: jaw[c], color: col });
    q('fl','fr','cf', P_COL.silver);                     // front
    q('tl','fl','cf', P_COL.silver); q('tl','cf','bl', P_COL.red);   // right side
    q('tr','cf','fr', P_COL.silver); q('tr','br','cf', P_COL.red);   // left side
    q('bl','cf','br', P_COL.red);                        // underneath
    q('tl','tr','fr', P_COL.black); q('tl','fr','fl', P_COL.black);  // inside the mouth

    // lower teeth point up along the jaw edge, upper teeth point down under the snout
    const tooth = (x, y, z, dir, w = 0.055, h = 0.13) =>
      tris.push({ a: [x, y, z - w], b: [x, y, z + w], c: [x * 0.9, y + h * dir, z], color: P_COL.tooth });
    for (let j = 0; j < 5; j++) {
      const z = 1.45 + j * 0.13, x = 0.19 - j * 0.004;
      tooth( x, -0.32, z,  1); tooth(-x, -0.32, z,  1);           // lower row
      tooth( x * 0.95, -0.27, z + 0.06, -1, 0.05, 0.10);           // upper row
      tooth(-x * 0.95, -0.27, z + 0.06, -1, 0.05, 0.10);
    }
    tooth(0.08, -0.33, 2.02, 1, 0.04, 0.12); tooth(-0.08, -0.33, 2.02, 1, 0.04, 0.12);
    const jawMesh = new THREE.Mesh(trisToGeometry(tris), fishMat);
    jawMesh.name = 'jaw'; jawMesh.castShadow = true;
    group.add(jawMesh);
  },
});

/* =================================================================
   SALMON — long, streamlined body, silver sides with a pink stripe,
   black spots on the back and a small adipose fin near the tail
   ================================================================= */
const S_COL = {
  back:   [0.30, 0.42, 0.48],
  silver: [0.82, 0.84, 0.86],
  pink:   [0.93, 0.58, 0.58],
  belly:  [0.96, 0.95, 0.93],
  spot:   [0.10, 0.12, 0.14],
  fin:    [0.42, 0.50, 0.54],
  edge:   [0.25, 0.32, 0.36],
};

const salmon = createFish({
  name: 'salmon',
  profile: [[-2.50, 0.08, 0.20,  0.05],
            [-2.10, 0.16, 0.35,  0.05],   // narrow tail stem
            [-1.20, 0.35, 0.60,  0.02],
            [-0.20, 0.45, 0.72,  0.00],   // thickest part
            [ 0.80, 0.42, 0.66, -0.02],
            [ 1.60, 0.33, 0.50, -0.05],
            [ 2.10, 0.22, 0.33, -0.08],
            [ 2.40, 0.10, 0.16, -0.12]],  // pointed snout
  noseTip: [2.55, -0.14],
  swim: { amplitude: 0.45, speed: 6, wavelength: 1.5, stillFrom: 1.2 },
  colorAt: (z, h, rnd) =>
    h >  0.55 ? (rnd < 0.3 ? S_COL.spot : S_COL.back) :      // spotted back
    h >  0.15 ? (rnd < 0.15 ? S_COL.spot : S_COL.silver) :   // a few spots on upper sides
    h > -0.20 ? S_COL.pink :                                 // pink stripe
    h > -0.55 ? S_COL.silver : S_COL.belly,                  // white belly
  capColor: S_COL.silver,
  fins: [
    { side: 'top',    from: -0.30, to:  0.65, height: 0.50, shape: finShape('tri', 0.7), inner: S_COL.fin, edge: S_COL.edge },
    { side: 'top',    from: -1.95, to: -1.65, height: 0.16, shape: finShape('round'),   inner: S_COL.fin, edge: S_COL.fin },  // adipose fin
    { side: 'bottom', from: -1.65, to: -1.05, height: 0.34, shape: finShape('tri', 0.7), inner: S_COL.fin, edge: S_COL.edge },
    { side: 'bottom', from: -0.10, to:  0.30, height: 0.22, shape: finShape('tri', 0.7), inner: S_COL.fin, edge: S_COL.fin },   // pelvic fin
  ],
  tail: { size: 1.2, fork: 0.35, inner: S_COL.fin, edge: S_COL.edge },
  pectoral: { at: [1.55, -0.30], size: 1.0, color: S_COL.fin, flapSpeed: 5 },
  eye: { at: [1.95, 0.08], iris: [0.80, 0.80, 0.78], radius: 0.11 },
});

const fishes = new THREE.Group();
fishes.add(piranha.group, salmon.group);
piranha.group.position.set(-2.4, 2.2, 0.6);
salmon.group.position.set( 2.4, 2.2, -0.2);

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
scene.add(fishes);

scene.add(new THREE.HemisphereLight(0xffffff, 0x444a55, 0.55));
const key = new THREE.DirectionalLight(0xffffff, 0.85);
key.position.set(6, 12, 8);
key.castShadow = true;
key.shadow.mapSize.set(2048, 2048);
Object.assign(key.shadow.camera, { left: -7, right: 7, top: 7, bottom: -7, near: 1, far: 40 });
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
fishes.traverse(o => {
  if (!o.isMesh || o.userData.isWire) return;
  const w = new THREE.Mesh(o.geometry, wireMat);
  w.userData.isWire = true; w.visible = false; w.frustumCulled = false;
  o.add(w); wires.push(w);
});

let triCount = 0;
fishes.traverse(o => { if (o.isMesh && !o.userData.isWire) triCount += o.geometry.attributes.position.count / 3; });
document.getElementById('stats').textContent =
  `2 fish, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 3;
controls.maxDistance = 28;
controls.maxPolarAngle = Math.PI * 0.49;
controls.autoRotateSpeed = 2;
controls.target.set(0, 2.2, 0);

const VIEWS = {
  angle: new THREE.Vector3(8, 5, 11),
  front: new THREE.Vector3(0, 2.6, 13),
  side:  new THREE.Vector3(-13, 2.6, 0),
  top:   new THREE.Vector3(0, 15, 0.01),
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

let swimming = !reduceMotion;
const swimBtn = document.getElementById('swim');
swimBtn.setAttribute('aria-pressed', String(swimming));
swimBtn.addEventListener('click', () => {
  swimming = !swimming;
  swimBtn.setAttribute('aria-pressed', String(swimming));
});

// show both fish side by side, or one at a time in the centre
const LAYOUT = {
  both:    { piranha: [-2.4, 0.6], salmon: [2.4, -0.2] },
  piranha: { piranha: [0, 0.3] },
  salmon:  { salmon: [0, 0] },
};
function showFish(mode) {
  for (const f of [piranha, salmon]) {
    const spot = LAYOUT[mode][f.group.name];
    f.group.visible = !!spot;
    if (spot) { f.group.position.x = spot[0]; f.group.position.z = spot[1]; }
  }
  document.querySelectorAll('[data-show]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.show === mode)));
}
document.querySelectorAll('[data-show]').forEach(b => b.addEventListener('click', () => showFish(b.dataset.show)));

/* =================================================================
   RENDER LOOP
   ================================================================= */
const clock = new THREE.Clock();
let waveTime = 0;
(function loop() {
  requestAnimationFrame(loop);
  const dt = Math.min(clock.getDelta(), 0.05);
  const t = clock.elapsedTime;

  if (swimming) {
    waveTime += dt;
    piranha.update(waveTime);        // rewrite each body's vertex positions
    salmon.update(waveTime);
    piranha.group.position.y = 2.2 + Math.sin(t * 1.4) * 0.12;
    salmon.group.position.y  = 2.2 + Math.sin(t * 1.1 + 1) * 0.12;
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



# Shape Reference (Nemo)
```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Low-poly fish — Three.js BufferGeometry</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;600&display=swap" rel="stylesheet">
<style>
  :root {
    --bg: #e4e6ea; --ink: #22252b; --muted: #666b75;
    --chip: #ffffffcc; --chip-on: #22252b; --chip-on-ink: #f2f3f5; --ring: #e8761a;
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
  <h1>Low-poly fish</h1>
  <p id="stats">Drag to orbit, scroll or pinch to zoom</p>
</header>
<nav aria-label="Viewer controls">
  <button data-view="angle" aria-pressed="true">3/4</button>
  <button data-view="front" aria-pressed="false">Front</button>
  <button data-view="side" aria-pressed="false">Side</button>
  <button data-view="top" aria-pressed="false">Top</button>
  <span class="sep" aria-hidden="true"></span>
  <button id="swim" aria-pressed="true">Swim</button>
  <button id="edges" aria-pressed="false">Show edges</button>
  <button id="spin" aria-pressed="false">Spin</button>
</nav>

<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/build/three.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/three@0.128.0/examples/js/controls/OrbitControls.js"></script>
<script>
/* =================================================================
   Coordinates: x = right, y = up, z = forward (the fish faces +z).
   Like the snake, the body is ONE BufferGeometry that is rewritten
   every frame, so the back half of the fish can sway while it swims.
   The dorsal and anal fins are part of that same geometry, so they
   bend together with the body.
   ================================================================= */

const COLORS = {
  orange: [1.00, 0.45, 0.08],
  white:  [0.97, 0.97, 0.95],
  black:  [0.07, 0.07, 0.08],
  fin:    [1.00, 0.62, 0.28],   // fins are a lighter, see-through-looking orange
};

/* -----------------------------------------------------------------
   HELPER: list of triangles → BufferGeometry (for the static parts)
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
   PART 1 — BODY PROFILE
   The fish is tall and thin, so each ring is an oval: narrow (rx)
   and tall (ry). This table is the shape from tail to nose: [z, rx, ry]
   ================================================================= */
const PROFILE = [
  [-1.70, 0.10, 0.28],   // narrow tail stem
  [-1.30, 0.20, 0.50],
  [-0.70, 0.38, 0.85],
  [ 0.00, 0.50, 1.00],   // deepest part of the body
  [ 0.70, 0.48, 0.90],
  [ 1.20, 0.38, 0.70],
  [ 1.60, 0.22, 0.42],
  [ 1.85, 0.08, 0.18],   // blunt nose
];
const NOSE_TIP = 1.95;

// smoothly read rx and ry at any z by blending the two nearest rows
function sizeAt(z) {
  for (let i = 0; i < PROFILE.length - 1; i++) {
    const [z0, x0, y0] = PROFILE[i], [z1, x1, y1] = PROFILE[i + 1];
    if (z <= z1) {
      const f = (z - z0) / (z1 - z0);
      return [x0 + (x1 - x0) * f, y0 + (y1 - y0) * f];
    }
  }
  return PROFILE[PROFILE.length - 1].slice(1);
}

const RINGS = 36;
const SIDES = 8;           // 0 = top, 4 = bottom
const Z_TAIL = PROFILE[0][0], Z_NOSE = PROFILE[PROFILE.length - 1][0];
const ringZ = i => Z_TAIL + (Z_NOSE - Z_TAIL) * (i / (RINGS - 1));

// swimming: the front stays still, the back swings more and more
function sideOffset(z, t) {
  const amount = Math.max(0, (0.8 - z) / 2.5);
  return 0.4 * amount * amount * Math.sin(2.2 * z - t * 7);
}

/* -----------------------------------------------------------------
   Clownfish stripes: three white bands with thin black edges.
   ----------------------------------------------------------------- */
const BANDS = [[1.00, 0.17], [0.00, 0.20], [-1.45, 0.13]];   // [centre z, half width]
function bandColor(z) {
  for (const [c, w] of BANDS) {
    const d = Math.abs(z - c) - w;
    if (d < 0) return COLORS.white;
    if (d < 0.1) return COLORS.black;
  }
  return COLORS.orange;
}

/* -----------------------------------------------------------------
   Fins that grow out of the top and bottom of the body.
   ----------------------------------------------------------------- */
const DORSAL = { from: -1.25, to: 0.90, height: z => 0.55 * Math.sin(Math.PI * (z + 1.25) / 2.15) };
const ANAL   = { from: -1.35, to: -0.40, height: z => 0.40 * Math.sin(Math.PI * (z + 1.35) / 0.95) };
const ringsIn = fin => [...Array(RINGS).keys()].filter(i => ringZ(i) >= fin.from && ringZ(i) <= fin.to);
const dorsalRings = ringsIn(DORSAL), analRings = ringsIn(ANAL);

// count triangles once so the arrays can be made the right size
const TRI_COUNT =
  (RINGS - 1) * SIDES * 2 +      // body tube
  SIDES * 2 +                    // nose cap + tail cap
  (dorsalRings.length - 1) * 4 + // dorsal fin (2 strips: orange + black edge)
  (analRings.length - 1) * 4;    // anal fin

const positions = new Float32Array(TRI_COUNT * 9);
const colors    = new Float32Array(TRI_COUNT * 9);
const bodyGeo = new THREE.BufferGeometry();
bodyGeo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
bodyGeo.setAttribute('color',    new THREE.BufferAttribute(colors, 3));

/* -----------------------------------------------------------------
   Colours never change, so fill them once — in exactly the same
   order that updateBody() writes the triangles.
   ----------------------------------------------------------------- */
(function fillColors() {
  let p = 0;
  const put = col => { for (let v = 0; v < 3; v++) { colors.set(col, p); p += 3; } };
  for (let i = 0; i < RINGS - 1; i++) {
    const col = bandColor((ringZ(i) + ringZ(i + 1)) / 2);
    for (let k = 0; k < SIDES; k++) { put(col); put(col); }
  }
  for (let k = 0; k < SIDES; k++) put(COLORS.orange);   // nose
  for (let k = 0; k < SIDES; k++) put(COLORS.orange);   // tail cap
  for (const fin of [dorsalRings, analRings])
    for (let j = 0; j < fin.length - 1; j++) {
      put(COLORS.fin); put(COLORS.fin);                  // inner part of the fin
      put(COLORS.black);  put(COLORS.black);             // black edge
    }
})();

// reusable vectors (no new objects every frame)
const centers = Array.from({ length: RINGS }, () => new THREE.Vector3());
const ups     = Array.from({ length: RINGS }, () => new THREE.Vector3());
const rings   = Array.from({ length: RINGS }, () => Array.from({ length: SIDES }, () => new THREE.Vector3()));
const UP = new THREE.Vector3(0, 1, 0);
const tangent = new THREE.Vector3(), side = new THREE.Vector3();
const tmpA = new THREE.Vector3(), tmpB = new THREE.Vector3(), tmpC = new THREE.Vector3(), tmpD = new THREE.Vector3();
const noseTip = new THREE.Vector3();

function updateBody(t) {
  // 1) centre of every ring
  for (let i = 0; i < RINGS; i++) centers[i].set(sideOffset(ringZ(i), t), 0, ringZ(i));

  // 2) oval ring of points around each centre
  for (let i = 0; i < RINGS; i++) {
    const a = centers[Math.max(i - 1, 0)], b = centers[Math.min(i + 1, RINGS - 1)];
    tangent.subVectors(b, a).normalize();
    side.crossVectors(tangent, UP).normalize();
    ups[i].crossVectors(side, tangent).normalize();
    const [rx, ry] = sizeAt(ringZ(i));
    for (let k = 0; k < SIDES; k++) {
      const ang = Math.PI / 2 + (2 * Math.PI * k) / SIDES;
      rings[i][k].copy(centers[i])
        .addScaledVector(side, Math.cos(ang) * rx)
        .addScaledVector(ups[i], Math.sin(ang) * ry);
    }
  }

  // 3) write triangles
  let p = 0;
  const put = v => { positions[p++] = v.x; positions[p++] = v.y; positions[p++] = v.z; };
  for (let i = 0; i < RINGS - 1; i++)
    for (let k = 0; k < SIDES; k++) {
      const a = rings[i][k], b = rings[i][(k + 1) % SIDES];
      const c = rings[i + 1][k], d = rings[i + 1][(k + 1) % SIDES];
      put(a); put(b); put(d);  put(a); put(d); put(c);
    }
  const last = RINGS - 1;
  noseTip.set(0, -0.02, NOSE_TIP);
  for (let k = 0; k < SIDES; k++) { put(rings[last][k]); put(rings[last][(k + 1) % SIDES]); put(noseTip); }
  for (let k = 0; k < SIDES; k++) { put(rings[0][(k + 1) % SIDES]); put(rings[0][k]); put(centers[0]); }

  // fins: a strip from the body edge outward; the last 20% is the black edge
  const writeFin = (list, fin, cornerIndex, dir) => {
    for (let j = 0; j < list.length - 1; j++) {
      const i0 = list[j], i1 = list[j + 1];
      const h0 = fin.height(ringZ(i0)) * dir, h1 = fin.height(ringZ(i1)) * dir;
      const b0 = rings[i0][cornerIndex], b1 = rings[i1][cornerIndex];
      tmpA.copy(b0).addScaledVector(ups[i0], h0 * 0.8);   // inner tip
      tmpB.copy(b1).addScaledVector(ups[i1], h1 * 0.8);
      tmpC.copy(b0).addScaledVector(ups[i0], h0);          // outer tip
      tmpD.copy(b1).addScaledVector(ups[i1], h1);
      put(b0); put(b1); put(tmpB);   put(b0); put(tmpB); put(tmpA);     // orange
      put(tmpA); put(tmpB); put(tmpD); put(tmpA); put(tmpD); put(tmpC); // black edge
    }
  };
  writeFin(dorsalRings, DORSAL, 0, 1);            // corner 0 = top of the ring
  writeFin(analRings,   ANAL,   SIDES / 2, -1);   // corner 4 = bottom of the ring

  bodyGeo.attributes.position.needsUpdate = true;
  bodyGeo.computeBoundingSphere();
}

/* =================================================================
   PART 2 — TAIL FIN (flat sheet, follows the end of the body)
   Drawn standing up (y) and pointing backward (-z).
   ================================================================= */
function buildTailFin() {
  const P = {
    rootT: [0, 0.25, 0], rootB: [0, -0.25, 0],
    iT: [0, 0.62, -0.72], iB: [0, -0.62, -0.72], iN: [0, 0, -0.45],   // inner outline
    tT: [0, 0.82, -0.98], tB: [0, -0.82, -0.98], notch: [0, 0, -0.58], // outer edge
  };
  const T = [
    ['rootT','rootB','iN','fin'], ['rootT','iN','iT','fin'], ['rootB','iB','iN','fin'],
    ['iT','iN','notch','black'], ['iT','notch','tT','black'],
    ['iB','notch','iN','black'], ['iB','tB','notch','black'],
    ['rootT','iT','tT','black'], ['rootB','tB','iB','black'],
  ];
  return trisToGeometry(T.map(([a, b, c, col]) => ({ a: P[a], b: P[b], c: P[c], color: COLORS[col] })));
}

/* =================================================================
   PART 3 — PECTORAL FIN (small paddle on each side) and EYE
   ================================================================= */
function buildPectoralFin() {
  const P = {
    rt: [0, 0.12, 0], rb: [0, -0.12, 0],
    a: [0.15, 0.25, -0.35], b: [0.28, 0, -0.50], c: [0.15, -0.22, -0.35],
    a2: [0.20, 0.30, -0.42], b2: [0.34, 0, -0.60], c2: [0.20, -0.27, -0.42],
  };
  const T = [
    ['rt','rb','b','fin'], ['rt','b','a','fin'], ['rb','c','b','fin'],
    ['a','b','b2','black'], ['a','b2','a2','black'], ['b','c','c2','black'], ['b','c2','b2','black'],
  ];
  return trisToGeometry(T.map(([a, b, c, col]) => ({ a: P[a], b: P[b], c: P[c], color: COLORS[col] })));
}

function buildEye() {
  const tris = [], N = 6, R_OUT = 0.15, R_IN = 0.085;
  const ring = (r, x) => [...Array(N).keys()].map(k => {
    const a = (2 * Math.PI * k) / N;
    return [x, Math.cos(a) * r, Math.sin(a) * r];
  });
  const outer = ring(R_OUT, 0.0), inner = ring(R_IN, 0.03), centre = [0.045, 0, 0];
  for (let k = 0; k < N; k++) {
    const k2 = (k + 1) % N;
    tris.push({ a: outer[k], b: outer[k2], c: inner[k2], color: COLORS.white });
    tris.push({ a: outer[k], b: inner[k2], c: inner[k], color: COLORS.white });
    tris.push({ a: centre, b: inner[k], c: inner[k2], color: COLORS.black });
  }
  return trisToGeometry(tris);
}

/* =================================================================
   ASSEMBLE
   ================================================================= */
const mat = new THREE.MeshStandardMaterial({
  vertexColors: true, flatShading: true, roughness: 0.4, side: THREE.DoubleSide,
});

const fish = new THREE.Group();
fish.name = 'fish';

const body = new THREE.Mesh(bodyGeo, mat);
body.name = 'body';
body.castShadow = true;
body.frustumCulled = false;       // the shape changes every frame
fish.add(body);

const tailFin = new THREE.Mesh(buildTailFin(), mat);
tailFin.name = 'tailFin';
tailFin.castShadow = true;
fish.add(tailFin);

const pectoralGeo = buildPectoralFin();
const finR = new THREE.Mesh(pectoralGeo, mat); finR.position.set( 0.46, -0.25, 0.55);
const finL = new THREE.Mesh(pectoralGeo, mat); finL.position.set(-0.46, -0.25, 0.55); finL.scale.x = -1;
finR.name = 'finR'; finL.name = 'finL';
fish.add(finR, finL);

const eyeGeo = buildEye();
const eyeR = new THREE.Mesh(eyeGeo, mat); eyeR.position.set( 0.31, 0.25, 1.30);
const eyeL = new THREE.Mesh(eyeGeo, mat); eyeL.position.set(-0.31, 0.25, 1.30); eyeL.scale.x = -1;
fish.add(eyeR, eyeL);

// keep the tail fin on the end of the swaying body
const finTarget = new THREE.Vector3();
function updateTailFin(t) {
  tangent.subVectors(centers[1], centers[0]).normalize();      // points from tail toward head
  tailFin.position.copy(centers[0]);
  tailFin.lookAt(finTarget.copy(centers[0]).add(tangent));
  tailFin.rotateY(Math.sin(2.2 * Z_TAIL - t * 7) * -0.35);     // extra flick at the very end
}

function updatePectorals(t) {
  const flap = 0.5 + Math.sin(t * 6) * 0.3;
  finR.rotation.y =  flap;
  finL.rotation.y = -flap;
}

updateBody(0);
updateTailFin(0);
updatePectorals(0);
fish.position.y = 2.2;            // float above the ground

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
scene.add(fish);

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

// edge overlay: a wireframe copy that shares each geometry, so it follows the animation
const wireMat = new THREE.MeshBasicMaterial({ color: 0x3a6ff0, wireframe: true });
const wires = [];
fish.traverse(o => {
  if (!o.isMesh || o.userData.isWire) return;
  const w = new THREE.Mesh(o.geometry, wireMat);
  w.userData.isWire = true; w.visible = false; w.frustumCulled = false;
  o.add(w); wires.push(w);
});

let triCount = 0;
fish.traverse(o => { if (o.isMesh && !o.userData.isWire) triCount += o.geometry.attributes.position.count / 3; });
document.getElementById('stats').textContent =
  `${RINGS} rings, ${triCount} triangles. Drag to orbit, scroll or pinch to zoom.`;

/* =================================================================
   CAMERA + CONTROLS
   ================================================================= */
const camera = new THREE.PerspectiveCamera(35, stage.clientWidth / stage.clientHeight, 0.1, 200);
const controls = new THREE.OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;
controls.enablePan = false;
controls.minDistance = 3;
controls.maxDistance = 22;
controls.maxPolarAngle = Math.PI * 0.49;
controls.autoRotateSpeed = 2;
controls.target.set(0, 2.2, 0);

const VIEWS = {
  angle: new THREE.Vector3(6, 4.2, 7),
  front: new THREE.Vector3(0, 2.6, 10),
  side:  new THREE.Vector3(-10, 2.6, 0),
  top:   new THREE.Vector3(0, 12, 0.01),
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

let swimming = !reduceMotion;
const swimBtn = document.getElementById('swim');
swimBtn.setAttribute('aria-pressed', String(swimming));
swimBtn.addEventListener('click', () => {
  swimming = !swimming;
  swimBtn.setAttribute('aria-pressed', String(swimming));
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

  if (swimming) {
    waveTime += dt;
    updateBody(waveTime);          // rewrite the body's vertex positions
    updateTailFin(waveTime);
    updatePectorals(t);
    fish.position.y = 2.2 + Math.sin(t * 1.3) * 0.12;   // gentle bobbing
    fish.rotation.z = Math.sin(t * 1.1) * 0.03;
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