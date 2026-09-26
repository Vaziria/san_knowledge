import * as THREE from 'three';
import { defaultTheme, surface, type Theme } from '../../theme';
import { blob, createMaterials, mesh, mix, palette, plain, solid, type Section, type Shade } from '../animals/parts';
import { seededRandom } from '../Tree/parts';

// A firefly, low poly, after the user's reference (a Three.js scene of flat-
// shaded icospheres): about 2 cm long, a plain black head in front, an orange
// thorax behind it, the biggest part, three dark brown segments of abdomen
// tapering back, and the lantern at its tail, which glows and flashes. Two
// pairs of see-through wings rise from the top of the thorax and lie back
// over the abdomen in a narrow V, the hind pair 0.75 the size of the front
// pair. Six thin legs, three a side, reach out under the thorax and abdomen,
// and two antennae curve up and forward from the head. Every part is a few
// flat faces (the bear's faceted look, from the animals' parts), and the
// wings are flat outlines.
//
// An object, not an animal, as the user asked ("move firefly to object, it
// can't talk"): its spec, Firefly.md, names no behaviour, so it has no
// speech bubble and nothing for the panel. It only moves on its own
// (update(delta), its idle motion, like the fog's drift): it hovers about its
// own length up, bobbing gently and turning a little from side to side, its
// wings beating about their roots, and its lantern flashes about every 2.5 s,
// lighting what is near it with a small light in its glow colour.
//
// For fireflies flying about (the lake's at night, environtments/Lake/
// Fireflies.ts), `speed` sends it forward along its +z at that many meters a
// second, leaning into it with its wings beating faster, as the animals go
// forward; turn it (rotation.y) to steer. At 0, the default, it hovers in
// place. Its options give it its own flash timing and no light.
//
// Units are meters, it faces +z, and the origin is on the ground under the
// middle of its body. Its parts are the body (head, thorax, abdomen and
// lantern, with the antennae), four wings and six legs, each a group with
// its origin at its root.

// The reference's unit: 3.2 of them from the lantern's back to the head's
// front, and a firefly is about 2 cm long.
const UNIT = 0.02 / 3.2;
// The reference's x (forward) of the middle of the body, which goes over
// the origin.
const MIDDLE = -0.275;

// A point given in the reference's units, forward, up and out to the right
// from the body's axis, in meters in the firefly's axes.
function at(forward: number, up: number, out = 0): THREE.Vector3 {
  return new THREE.Vector3(out * UNIT, up * UNIT, (forward - MIDDLE) * UNIT);
}

// Balls along the body, in the reference's units: where each sits (forward,
// up) and its radius.
const HEAD = { at: [0.9, 0.15], radius: 0.42 } as const;
const THORAX = { at: [0.35, 0.08], radius: 0.48 } as const;
const ABDOMEN = [
  { at: [-0.15, 0], radius: 0.43 },
  { at: [-0.65, 0], radius: 0.43 },
  { at: [-1.05, 0], radius: 0.4 },
] as const;
const LANTERN = { at: [-1.42, 0], radius: 0.45 } as const;
const BROWN = 0.12; // how far the abdomen, legs and antennae are mixed from dark toward fur

// A wing's outline from its root, back along the body and up, in the
// reference's units. Each pair rises from the top of the thorax, leaning out
// from upright by `tilt` and turned in toward the other side by `splay`, and
// beats `beat` radians either way about its root while it hovers.
const WING_OUTLINE: [back: number, up: number][] = [
  [0, 0],
  [0.45, 0.08],
  [1.25, 0.7],
  [1.65, 0.35],
  [1.15, -0.05],
  [0.35, -0.1],
];
const WINGS = [
  { at: [0.2, 0.35, 0.12], size: 1, splay: 0.1, beat: 0.25 },
  { at: [0.2, 0.22, 0.12], size: 0.75, splay: 0.3, beat: 0.18 },
] as const;
const WING_TILT = 0.35; // radians
const WING_OPACITY = 0.38;

// The legs: three a side, each rooted inside the body at `x` forward, and
// reaching out and back along `path` from there (reference units: back, up,
// out). They are drawn about three times as thick as a real firefly's
// (0.2 mm across the reference's), since the Kuwahara filter wipes out
// anything a few pixels wide: at 0.4 mm they had all but gone even close up.
const LEG_ROOTS = [0.35, -0.1, -0.55];
const LEG_PATH: [back: number, up: number, out: number][] = [
  [0, 0, 0],
  [0.25, -0.2, 0.3],
  [0.5, -0.1, 0.55],
];
const LEG_OUT = 0.25; // reference units out from the axis to a leg's root
const LEG_RADIUS = 0.0006; // m
// The antennae, from inside the head up and forward (reference units:
// forward, up, out), thicker than real like the legs.
const ANTENNA_PATH: [forward: number, up: number, out: number][] = [
  [1.05, 0.35, 0.15],
  [1.35, 0.65, 0.25],
  [1.7, 0.85, 0.4],
];
const ANTENNA_RADIUS = 0.00045; // m

// Hovering and flying.
const HOVER = 0.022; // m the body's axis is off the ground: its lowest point about its own length up
const BOB = { height: 0.12 * UNIT, rate: 1.5 }; // m up and down, radians a second (the reference's)
const SWAY = { angle: 0.12, rate: 0.5 }; // radians it turns either way while it hovers, radians a second (the reference's)
const BEAT_RATE = { hover: 18, flying: 24 }; // radians a second (the reference's 18 hovering)
const LEAN = 0.12; // radians nose down, flying
const EASE = 4; // how fast it sets off and slows to a hover, per second

// The lantern's flash, as the reference's: pow(max(0, sin(2.5 t)), 8), a
// sharp flash about every 2.5 s. Its glow (emissive intensity) and the
// small light at it (candela, and the meters it reaches) rise with it.
// three.js stops a light's inverse square at 10 cm (1 / max(d², 0.01)), so
// at a firefly's size the light is as strong at its body as on the ground
// below, and only its reach makes a pool: at the flash it lights the
// ground under it about half as much as the sun does, fading out 4 cm away.
const FLASH = { rate: 2.5, sharpness: 8 };
const GLOW = { dim: 0.35, flash: 1.4 };
const LIGHT = { dim: 0.002, flash: 0.016, reach: 0.04 };
const OWN_RATE = 0.12; // with a seed, it flashes up to this share faster or slower

export interface FireflyOptions {
  theme?: Theme;
  // For many fireflies flying together (the lake's at night); left out, a
  // firefly is the one in its preview.
  // Its own timing: where it is in its flash, bob and sway, and a flash a
  // little faster or slower, so no two flash together. Without it, the
  // first flash comes 0.63 s after it is made, every 2.5 s after.
  seed?: number;
  // m/s it flies forward at (`speed`); 0, hovering in place, by default.
  speed?: number;
  // false: no light at its lantern. Every light costs every lit surface in
  // the scene, and a change in the number of lights recompiles them all.
  light?: boolean;
}

const ahead = new THREE.Vector3(); // reused by update()

export class Firefly extends THREE.Group {
  // m its body's axis flies above its origin, hovering: place it by that.
  static readonly HOVER = HOVER;

  readonly body = new THREE.Group();
  readonly lantern: THREE.Mesh;
  readonly glow: THREE.PointLight | null; // null with light: false
  readonly wings: THREE.Group[] = []; // front left, front right, hind left, hind right
  readonly legs: THREE.Group[] = []; // front to back, left then right
  // m/s it flies forward at, along its +z; it eases to it from a hover, and
  // back. Turn it (rotation.y) to steer.
  speed: number;
  // How brightly its lantern glows, 0 to 1 (and its light, if it has one):
  // the lake's fireflies fade in at dusk and out at dawn.
  lit = 1;

  // Bobs, sways and leans; the body inside it.
  private readonly rig = new THREE.Group();
  private readonly glowMaterial: THREE.MeshPhysicalMaterial;
  private clock = 0; // s it has been flying
  private beat = 0; // radians through its wing beat
  private flying = 0; // 0 hovering, 1 flying at its speed; eased
  private flashRate = FLASH.rate;
  private flashNow = 0;
  private readonly wingPairs: (typeof WINGS)[number][] = [];

  constructor(options: FireflyOptions = {}) {
    super();
    this.name = 'firefly';
    const theme = options.theme ?? defaultTheme;
    this.speed = options.speed ?? 0;
    if (options.seed !== undefined) {
      const random = seededRandom(options.seed);
      this.clock = ((2 * Math.PI) / FLASH.rate) * random();
      this.flashRate = FLASH.rate * (1 + OWN_RATE * (2 * random() - 1));
    }
    this.rig.rotation.order = 'YXZ'; // it turns (sway), then leans
    const m = createMaterials(theme, 'faceted');
    const p = palette(theme);
    const brown = plain(mix(p.dark, p.fur, BROWN));

    // The body: a string of faceted balls, and the lantern in its own
    // material, since it glows and flashes on its own.
    const ball = (part: { at: readonly number[]; radius: number }, shade: Shade) =>
      blob(at(part.at[0], part.at[1]), new THREE.Vector3().setScalar(part.radius * UNIT), shade, {}, undefined, m.look);
    this.body.add(mesh(ball(HEAD, plain(p.dark)), m.coat));
    this.body.add(mesh(ball(THORAX, plain(p.trim)), m.coat));
    for (const segment of ABDOMEN) this.body.add(mesh(ball(segment, brown), m.coat));
    this.glowMaterial = surface(theme, theme.colors.glow);
    this.glowMaterial.emissive.set(theme.colors.glow);
    this.lantern = mesh(ball(LANTERN, plain(p.light)), this.glowMaterial);
    this.body.add(this.lantern);
    this.glow = options.light === false ? null : new THREE.PointLight(theme.colors.glow, LIGHT.dim, LIGHT.reach, 2); // casts no shadow
    if (this.glow) {
      this.glow.position.copy(at(LANTERN.at[0], LANTERN.at[1]));
      this.body.add(this.glow);
    }

    // Antennae, thin tubes curving up and forward from the head.
    const thin = (radius: number) => (): Section => ({ width: radius, top: radius, bottom: radius });
    for (const side of [-1, 1] as const) {
      const path = ANTENNA_PATH.map(([ahead, up, out]) => at(ahead, up, side * out));
      this.body.add(mesh(solid(path, thin(ANTENNA_RADIUS), brown, 10, 18, m.look), m.coat));
    }

    // Wings: flat, see-through outlines, one shape for all four. They cast
    // no shadow, which would be as dark as a solid wing's.
    const wing = surface(theme, theme.colors.light);
    wing.transparent = true;
    wing.opacity = WING_OPACITY;
    wing.side = THREE.DoubleSide;
    wing.depthWrite = false;
    const outline = new THREE.ShapeGeometry(new THREE.Shape(WING_OUTLINE.map(([back, up]) => new THREE.Vector2(-back * UNIT, up * UNIT))));
    outline.rotateY(-Math.PI / 2); // drawn back along -x, now back along -z, upright along the body
    for (const pair of WINGS) {
      for (const side of [-1, 1] as const) {
        const root = new THREE.Group();
        root.name = `${pair.size < 1 ? 'hind' : 'front'} ${side < 0 ? 'left' : 'right'} wing`;
        root.position.copy(at(pair.at[0], pair.at[1], side * pair.at[2]));
        root.scale.setScalar(pair.size);
        root.rotation.order = 'ZYX'; // beats in its own plane, then turns in and leans out
        root.add(new THREE.Mesh(outline, wing));
        this.body.add(root);
        this.wings.push(root);
        this.wingPairs.push(pair);
      }
    }

    // Legs, each a group at its root.
    for (const x of LEG_ROOTS) {
      for (const side of [-1, 1] as const) {
        const leg = new THREE.Group();
        leg.position.copy(at(x, 0, side * LEG_OUT));
        const path = LEG_PATH.map(([back, up, out]) => new THREE.Vector3(side * out * UNIT, up * UNIT, -back * UNIT));
        leg.add(mesh(solid(path, thin(LEG_RADIUS), brown, 10, 18, m.look), m.coat));
        this.body.add(leg);
        this.legs.push(leg);
      }
    }

    this.rig.add(this.body);
    this.add(this.rig);
    this.update(0);
  }

  // Its idle motion, and flying forward at its speed; call once per frame.
  update(delta: number): void {
    this.flying += ((this.speed > 0 ? 1 : 0) - this.flying) * (1 - Math.exp(-EASE * delta));
    ahead.set(0, 0, 1).applyQuaternion(this.quaternion).setY(0);
    if (ahead.lengthSq() > 0) this.position.addScaledVector(ahead.normalize(), this.speed * this.flying * delta);

    this.clock += delta;
    const t = this.clock;

    // Hovering it bobs and turns a little from side to side; flying, it
    // leans into it.
    this.rig.position.y = HOVER + BOB.height * Math.sin(BOB.rate * t);
    this.rig.rotation.y = SWAY.angle * (1 - this.flying) * Math.sin(SWAY.rate * t);
    this.rig.rotation.x = LEAN * this.flying;

    // Wings beat about their roots, faster flying, and both sides together.
    this.beat = (this.beat + THREE.MathUtils.lerp(BEAT_RATE.hover, BEAT_RATE.flying, this.flying) * delta) % (2 * Math.PI);
    const swing = Math.sin(this.beat);
    this.wings.forEach((wing, i) => {
      const side = i % 2 === 0 ? -1 : 1;
      const pair = this.wingPairs[i];
      wing.rotation.set(pair.beat * swing, side * pair.splay, -side * WING_TILT);
    });

    // The lantern flashes.
    const flash = (this.flashNow = Math.pow(Math.max(0, Math.sin(this.flashRate * t)), FLASH.sharpness));
    this.glowMaterial.emissiveIntensity = this.lit * (GLOW.dim + GLOW.flash * flash);
    if (this.glow) this.glow.intensity = this.lit * (LIGHT.dim + LIGHT.flash * flash);
  }

  // How far into a flash its lantern is now, 0 between flashes, 1 at the
  // height of one (before `lit`).
  get flash(): number {
    return this.flashNow;
  }
}
