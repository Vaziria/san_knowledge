# Firefly figure, low poly

The user asked (2026-09-25): "create firefly animal figure with lowpoly genre, this is for reference", with the Three.js code under [Reference](#reference) below. That line is the user's; the rest of this file was written by Claude from it. Choices marked **(default)** are decided, not open questions: build them as written unless the user has changed this file.

Read `docs/3d_modelling/rules.md` first, as for every figure, and follow it wherever this file is silent.

## What to build

A firefly among the animals: `src/figures/animals/Firefly.ts` with its spec `Firefly.md` beside it, plus part files if it needs them (the bird has `Wing.ts`). The figure tree then puts it under Animals by itself.

Its shape, parts and motion follow the reference:

- **Parts:** a black head in front; an orange thorax behind it, the biggest part; three dark brown abdomen segments tapering back; and the glowing lantern at the tail. Two pairs of see-through wings from the top of the thorax, lying back over the abdomen, the hind pair 0.75 the size of the front pair. Six thin legs, three a side, under the thorax and abdomen. Two antennae from the head, curving up and forward. The reference's head has no eyes: keep it a plain black ball.
- **Low poly (the user's "lowpoly genre"):** build it faceted, like the bear, with the animals' faceted look: `createMaterials(theme, 'faceted')`, then `blob()` and `solid()` with `m.look` (`src/figures/animals/parts.ts`; rules.md, Animals rule 13). The reference's `IcosahedronGeometry(r, 1)` with flat shading is what a faceted `blob()` already is, with its facets a little uneven. The wings are flat outlines, like the reference's `Shape`s.
- **Motion while it does nothing:** as in the reference, it hovers, bobbing gently and swaying a little from side to side. Its wings beat up and down about their roots (the reference: 18 rad/s, ±0.25 rad for the front pair and ±0.18 for the hind pair). Its lantern flashes: the reference's `pow(max(0, sin(2.5 t)), 8)`, a sharp flash about every 2.5 s. A small point light at the lantern flashes with it, in the glow colour, casting no shadow. Adding or removing a light makes three.js recompile the scene's materials, so check for a hitch when the firefly is shown.

## Size, axes and colours

- **Real size (default),** like every figure: about 2 cm long. The reference is about 3.2 units from the lantern to the head, so one unit is about 6 mm. Draw thin parts thicker than real if the Kuwahara filter wipes them out (rules.md: Grasses rule 4, Building shapes rule 11).
- **Axes:** the reference faces +x. The figure faces +z, with y up and its origin on the ground under the middle of its body (rules.md, Units, axes and origin).
- **Colours come from the theme, never hardcoded:**
  - the head: `dark`;
  - the thorax: `trim`, which is orange in felt, like the reference;
  - the abdomen, legs and antennae: a dark brown mixed from `dark` and `fur`;
  - the wings: `light`, see-through (the reference's opacity is 0.38).
- **A new `glow` role for the lantern:** no role has its colour, so add `glow` to `Theme.colors` in every theme (rules.md, Colour and theme rule 2). Make it a warm yellow in felt, like the reference's. In the brand themes, use a colour the brand's palette allows (`docs/3d_modelling/brand_getresolved.md`). The lantern's material is emissive in `glow` and belongs to this firefly alone, since it flashes on its own.
- The reference's near-black background belongs to its scene, not to the firefly: don't add a theme for it. Check the glow in `studio`, the dark theme.

## Behaviours (default)

It is an animal, so it shares the animals' behaviours through `Animal.ts`, as the bird does with its own body: `Jump()`, `Hold(figure)`, `Walk()`, `Run()` and `Speech(text)`, plus `Stop()`. A firefly flies rather than walks:

- Standing still, it hovers a little above the ground (about its own length) with the motion above.
- `Walk()` flies forward slowly and `Run()` flies fast, leaning into it with its wings beating faster. It moves itself at `speed` along its +z, as every animal does (rules.md, Moving parts rule 7).
- `Stop()` slows it back to a hover.
- `Jump()` darts up and drops back to its hover.
- `Hold(figure)` carries a figure in its legs under its body, scaled down to about its own length.
- `Speech(text)` shows the speech bubble. Set `bubbleScale` so its words read in its own preview, as the bird and the snake do (Animals rule 9).

Write `Firefly.md` in the user's spec layout (rules.md, Figure specs rule 6) with these behaviours, and say at its top that it is a draft for the user to correct.

## Preview

Add `firefly: animal(Firefly, 'firefly', <radius>)` to the `previews` table in `src/previews.ts`, with a circle sized to the firefly. The demo and the Behaviours buttons come with `animal()`.

Other sessions are also changing `src/previews.ts`, `src/theme.ts` and `src/figures/animals/Animal.ts`. Change only the lines you need, and never restore any of them from a copy.

## Checks

- `npm run typecheck` in `typescripts/animation`.
- Screenshots with headless Edge on a spare port, never 8087 (rules.md, Checking the result). Get close enough to see the facets, and shoot with the Kuwahara filter on and off, in `felt` and in `studio`. With the filter on, look at the wings and the flash.
- Numeric checks: its box is about 2 cm long, the demo's circle closes loop after loop, and nothing is NaN.
- If you change shared animal code (`Animal.ts`, `parts.ts`, `Quadruped.ts` and so on), the other animals must stay exactly as they were. Record every mesh's world matrix through a scripted demo before and after the change and compare them, as the bear work did (Animals rule 13).
- Add what you learned to rules.md (Animals), then call `knowledge_sync`.
- Don't commit.

## Reference

The user's reference, verbatim:

```js
import * as THREE from "three";
import { OrbitControls } from "three/addons/controls/OrbitControls.js";

const scene = new THREE.Scene();
scene.background = new THREE.Color(0x080b0d);

const camera = new THREE.PerspectiveCamera(
  45,
  window.innerWidth / window.innerHeight,
  0.1,
  100
);

camera.position.set(3.5, 2, 5);

const renderer = new THREE.WebGLRenderer({
  antialias: true
});

renderer.setSize(
  window.innerWidth,
  window.innerHeight
);

renderer.setPixelRatio(
  Math.min(window.devicePixelRatio, 2)
);

renderer.outputColorSpace = THREE.SRGBColorSpace;

document.body.appendChild(renderer.domElement);


// --------------------------------------------------
// Controls
// --------------------------------------------------

const controls = new OrbitControls(
  camera,
  renderer.domElement
);

controls.enableDamping = true;
controls.target.set(0, 0, 0);


// --------------------------------------------------
// Lighting
// --------------------------------------------------

scene.add(
  new THREE.AmbientLight(
    0xffffff,
    1.5
  )
);

const light = new THREE.DirectionalLight(
  0xffffff,
  3
);

light.position.set(4, 5, 5);

scene.add(light);


// --------------------------------------------------
// Firefly
// --------------------------------------------------

const firefly = new THREE.Group();

scene.add(firefly);


// --------------------------------------------------
// Materials
// --------------------------------------------------

const black = new THREE.MeshStandardMaterial({
  color: 0x111318,
  roughness: 0.8,
  flatShading: true
});

const body = new THREE.MeshStandardMaterial({
  color: 0x29251f,
  roughness: 0.8,
  flatShading: true
});

const orange = new THREE.MeshStandardMaterial({
  color: 0xd9532b,
  roughness: 0.7,
  flatShading: true
});

const glow = new THREE.MeshStandardMaterial({
  color: 0xffff44,
  emissive: 0xffff00,
  emissiveIntensity: 8,
  flatShading: true
});

const wingMaterial = new THREE.MeshBasicMaterial({
  color: 0xaaa68c,
  transparent: true,
  opacity: 0.38,
  side: THREE.DoubleSide
});


// --------------------------------------------------
// Helper: low poly sphere
// --------------------------------------------------

function ico(
  radius,
  material,
  position
) {

  const geometry =
    new THREE.IcosahedronGeometry(
      radius,
      1
    );

  const mesh =
    new THREE.Mesh(
      geometry,
      material
    );

  mesh.position.copy(position);

  firefly.add(mesh);

  return mesh;
}


// --------------------------------------------------
// Head
// --------------------------------------------------

ico(
  0.42,
  black,
  new THREE.Vector3(0.9, 0.15, 0)
);


// --------------------------------------------------
// Thorax
// --------------------------------------------------

ico(
  0.48,
  orange,
  new THREE.Vector3(0.35, 0.08, 0)
);


// --------------------------------------------------
// Abdomen
// --------------------------------------------------

ico(
  0.43,
  body,
  new THREE.Vector3(-0.15, 0, 0)
);

ico(
  0.43,
  body,
  new THREE.Vector3(-0.65, 0, 0)
);

ico(
  0.40,
  body,
  new THREE.Vector3(-1.05, 0, 0)
);

const glowingPart = ico(
  0.45,
  glow,
  new THREE.Vector3(-1.42, 0, 0)
);


// --------------------------------------------------
// Glow light
// --------------------------------------------------

const fireLight =
  new THREE.PointLight(
    0xffff33,
    8,
    4
  );

fireLight.position.set(
  -1.45,
  0,
  0
);

firefly.add(fireLight);


// --------------------------------------------------
// Wings
// --------------------------------------------------

function createWing(
  side,
  y
) {

  const shape =
    new THREE.Shape();

  shape.moveTo(0, 0);

  shape.lineTo(
    -0.45,
    0.08
  );

  shape.lineTo(
    -1.25,
    0.7
  );

  shape.lineTo(
    -1.65,
    0.35
  );

  shape.lineTo(
    -1.15,
    -0.05
  );

  shape.lineTo(
    -0.35,
    -0.1
  );

  shape.closePath();

  const geometry =
    new THREE.ShapeGeometry(shape);

  const wing =
    new THREE.Mesh(
      geometry,
      wingMaterial
    );

  wing.position.set(
    0.2,
    0.35,
    side * 0.12
  );

  wing.rotation.x =
    side * 0.35;

  wing.rotation.y = y;

  firefly.add(wing);

  return wing;
}

const wingLeft =
  createWing(-1, 0.1);

const wingRight =
  createWing(1, -0.1);


// second pair

const wingLeft2 =
  createWing(-1, 0.3);

const wingRight2 =
  createWing(1, -0.3);

wingLeft2.scale.set(
  0.75,
  0.75,
  0.75
);

wingRight2.scale.set(
  0.75,
  0.75,
  0.75
);

wingLeft2.position.y = 0.22;
wingRight2.position.y = 0.22;


// --------------------------------------------------
// Legs
// --------------------------------------------------

function leg(
  side,
  x
) {

  const points = [
    new THREE.Vector3(
      x,
      0,
      side * 0.25
    ),

    new THREE.Vector3(
      x - 0.25,
      -0.2,
      side * 0.55
    ),

    new THREE.Vector3(
      x - 0.5,
      -0.1,
      side * 0.8
    )
  ];

  const curve =
    new THREE.CatmullRomCurve3(points);

  const geometry =
    new THREE.TubeGeometry(
      curve,
      4,
      0.035,
      4,
      false
    );

  const mesh =
    new THREE.Mesh(
      geometry,
      body
    );

  firefly.add(mesh);
}

for (const side of [-1, 1]) {

  leg(side, 0.35);
  leg(side, -0.1);
  leg(side, -0.55);

}


// --------------------------------------------------
// Antennae
// --------------------------------------------------

function antenna(side) {

  const points = [

    new THREE.Vector3(
      1.05,
      0.35,
      side * 0.15
    ),

    new THREE.Vector3(
      1.35,
      0.65,
      side * 0.25
    ),

    new THREE.Vector3(
      1.7,
      0.85,
      side * 0.4
    )
  ];

  const curve =
    new THREE.CatmullRomCurve3(points);

  const geometry =
    new THREE.TubeGeometry(
      curve,
      5,
      0.025,
      4,
      false
    );

  firefly.add(
    new THREE.Mesh(
      geometry,
      body
    )
  );
}

antenna(-1);
antenna(1);


// --------------------------------------------------
// Animation
// --------------------------------------------------

const clock = new THREE.Clock();

function animate() {

  requestAnimationFrame(animate);

  const t =
    clock.getElapsedTime();

  // floating
  firefly.position.y =
    Math.sin(t * 1.5) * 0.12;

  // body movement
  firefly.rotation.y =
    Math.sin(t * 0.5) * 0.12;

  // wings
  wingLeft.rotation.z =
    Math.sin(t * 18) * 0.25;

  wingRight.rotation.z =
    -Math.sin(t * 18) * 0.25;

  wingLeft2.rotation.z =
    Math.sin(t * 18) * 0.18;

  wingRight2.rotation.z =
    -Math.sin(t * 18) * 0.18;

  // blinking
  const blink =
    Math.pow(
      Math.max(
        0,
        Math.sin(t * 2.5)
      ),
      8
    );

  fireLight.intensity =
    2 + blink * 10;

  glowingPart.material.emissiveIntensity =
    2 + blink * 8;

  controls.update();

  renderer.render(
    scene,
    camera
  );
}

animate();


// --------------------------------------------------
// Resize
// --------------------------------------------------

window.addEventListener(
  "resize",
  () => {

    camera.aspect =
      window.innerWidth /
      window.innerHeight;

    camera.updateProjectionMatrix();

    renderer.setSize(
      window.innerWidth,
      window.innerHeight
    );
  }
);
```

Taken: 2026-09-25 16:23

## Done

**What changed**

- `src/figures/animals/Firefly.ts`: a new figure that extends `Animal` directly, as the bird and the snake do. It is built faceted from `createMaterials(theme, 'faceted')`, `blob()` and `solid()` with `m.look`.
  - **Parts:** a black head, an orange (`trim`) thorax, three dark brown segments of abdomen (`dark` mixed 12% toward `fur`), and the lantern in its own emissive `glow` material with a point light that casts no shadow. There are four see-through wings (one `ShapeGeometry` of the reference's outline, `light` at opacity 0.38), six legs and two antennae.
  - **Size:** 2.0 cm from the lantern to the head, 2.2 cm with the antennae. It is 832 triangles in 18 meshes and builds in about 15 ms.
  - **Hovering:** its origin is on the ground and its body flies 2.2 cm up. It bobs and sways as in the reference, the wings beat at 18 rad/s (±0.25 front, ±0.18 hind), and the lantern flashes with `pow(max(0, sin(2.5 t)), 8)`.
  - **Behaviours:**
    - `Walk()` flies at 4 cm/s and `Run()` at 12 cm/s, leaning 0.35 rad and beating faster.
    - `Stop()` slows back to a hover.
    - `Jump()` darts up 5 cm.
    - `Hold()` carries the figure under the body in its legs, along the body and head first, scaled down to 2 cm.
    - `Speech()`'s bubble is 0.08 of the penguin's.
- `Firefly.md`: the spec in the user's layout, marked at its top as a draft for you to correct.
- `src/theme.ts`: a new `glow` role in every theme:
  - felt `#F7D64A`, a warm yellow;
  - studio `#FFD23F`;
  - pastel `#FBE39A`, beside its gold metal;
  - purple `#FFE38A`, a paler step of its yellow accent;
  - getresolved `#818CF8` and getresolved-dark `#A5B4FC`. The brand has no yellow and keeps green for controls, so these are its light indigos, recorded in `brand_getresolved.md`.
- `src/previews.ts`: `firefly: animal(Firefly, 'firefly', 0.03)`, plus two small fixes in `animal()` that every animal's demo shares:
  - The laps are now counted from the nearest **half** lap. Counted from whole laps, every loop after the first walked only 0.03 of a lap, and the circle crept inward loop after loop (the cat 3% of its radius a loop, the bird 2%, the firefly 1.2%). Now each loop walks half a lap and runs one, stopping on the far side of the circle, then back where it began, and the circle stays put (Node check over 100 s, every animal). **The other animals' demos change from their second loop on;** their first loop is as before.
  - The view is lifted by how far the animal's box is off the ground, and stands back for its top, so a hovering animal is framed where it flies. For the animals whose feet reach the ground both are exactly what they were. The snake's box sits 0.2 mm up, so its camera moves by that much.
- `src/Stage.ts`: the near plane (a fixed 10 cm) cut away a 2 cm figure seen close up. `fitNear()` makes it a tenth of the camera's distance to its target, between 2 mm and 10 cm, and 10 cm while walking. Views from 1 m or more are unchanged, and the bird's preview (near plane now 6 cm) drew the same to within one level.
- `docs/3d_modelling/rules.md`:
  - Animals: the firefly in the list, rule 10 (the demo and its framing), rule 12 (cost), and a new rule 14 (the firefly).
  - The `glow` role in the colour table.
  - Checking the result, rule 7: start a test page's clock from the page's own time, and reach the stage by patching `Stage.prototype.frame`.
  - Knowledge synced.

**How it was checked**

- `npm run typecheck` passes.
- **Numeric checks in Node** (Vite `ssrLoadModule`, 90 s of the demo at 60 fps):
  - the box is 22.3 mm long (the body alone 19.9 mm);
  - hovering, its lowest point is 19.2 mm up;
  - it flashes every 2.51 s;
  - no matrix entry is NaN or infinite;
  - where each loop stops alternates between 0.981 and 0.997 of the radius, with no drift.
- **Screenshots** in headless Edge on the GPU, port 8131, with frames driven through the remote-debugging protocol:
  - close up (6 cm) and from the preview's own camera;
  - filter off and on, in felt, studio, getresolved and getresolved-dark;
  - at a flash and between flashes, holding the fish, and running.

  The facets show. Legs and antennae survive the filter as strokes, and the wings as soft planes. At a flash the lantern turns pale yellow-white and lights a warm pool on the ground; in studio it also catches the wings and the facets facing it.
- **The first try failed.** The first light lit nothing, because three.js stops a light's inverse square within 10 cm. It now relies on its 4 cm reach.
- **Hitch** (RTX 3050, fresh shader cache). A point light is a new light count, so the whole scene recompiles:
  - the first time the firefly is shown the page stalls 0.37 s on the floor (other animals 0.07 s) and 1.8 s on the lake (other animals 0.8–0.87 s);
  - after that it costs what any animal does (0.08 s and 0.85 s), since the browser keeps the compiled programs.
- No shared animal code (`Animal.ts`, `parts.ts`, `Quadruped.ts`, …) was changed, so the world-matrix regression run was not needed.

**Left for you**

- **Wing beat:** the reference beats the left wings against the right ones, one side up while the other is down. I made both sides beat together, as an insect's wings do. Say if you want the reference's version.
- **Thorax colour:** it is `trim`, which is orange only in felt. In studio it is dark grey, so there only the lantern stands out.
- **Leg thickness:** legs are 1.2 mm across and antennae 0.9 mm, about three times real, so they survive the Kuwahara filter.
- **Flight speeds** of 4 and 12 cm/s and the 5 cm dart are my choices, as is carrying a held figure head first along the body.
- **First-show stall:** if the stall the first time the firefly is shown matters, the stage could keep a dark point light in the scene at all times, or compile the next scene in the background (`renderer.compileAsync`) before showing it.
- **Other animals' demos:** from their second loop on they now walk half a lap again, since the half-lap fix changes them.
- **Summaries:** the knowledge sync left 10 nodes needing a summary. Running `bin\knowledge.exe sync` writes them.
