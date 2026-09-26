import * as THREE from 'three';
import { keepSharp } from '../../effects/kuwahara';
import { Firefly } from '../../figures/objects/Firefly';
import { between, seededRandom } from '../../figures/Tree/parts';
import type { Theme } from '../../theme';
import { LANDING, shoreRadius } from './Ground';

// Fireflies over the lake at night, as the user asked ("if night, add
// firefly"): twenty of the firefly object (figures/objects/Firefly.ts),
// wandering over the grass along the shore either side of Lake.LANDING and
// in front of it, toward the water, where the preview cameras look (they
// stand on the landing's far side, looking over it at the lake), 0.3-2 m
// above the ground, and a few over the water's edge.
// They come out at dusk and go at dawn (`night`, from DayNight.ts), their
// lanterns fading in and out, and by day they are hidden and nothing of them
// is drawn or moved. They are built once, with the lake: adding and removing
// them would change nothing in the number of lights, since they have none,
// but would build them again every dusk.
//
// - Each has its own home and wanders within ROAM of it: flying forward at
//   its `speed`, steered by turning it (rotation.y) as the animals' demo
//   steers them, it
//   turns now and then, rises and sinks, and turns back home once it strays.
//   None comes within KEEP_OUT of the landing, where the figure stands.
// - They fly at 0.25-0.45 m/s: the 4 cm/s the firefly first walked at, as an
//   animal, looks frozen from cameras meters away.
// - Each flashes on its own timing (FireflyOptions.seed), not with the others.
// - No lights (FireflyOptions.light): a light costs every lit surface in the
//   scene, and the lake is the heaviest scene there is.
// - Seen from the lake's cameras a firefly is a few pixels, and the Kuwahara
//   filter averages away anything narrower than its radius (rules.md,
//   Effects). So the lantern is kept out of the filter (keepSharp), and a
//   soft halo in the glow colour, 16 cm across, shines round it,
//   brighter at each flash. The halo adds light without changing the
//   picture's alpha, which the filter reads to keep the lantern sharp.
//
// Placed and timed by a seeded random generator, so the lake is the same on
// every load. Units are meters, in the lake's coordinates; `heightAt` gives
// the height they fly over, the ground's or the water's.

const SWARM = {
  count: 20,
  water: 4, // of them over the water's edge
  around: 1.1, // radians either side of the landing, round the lake, where homes are
  land: [0.6, 4] as const, // m past the shore, for homes over the grass
  reach: [2.4, 7] as const, // m from the landing to a home
  behind: 0.3, // m past the landing (+z, toward the cameras) a home may be
  edge: [-1.4, 0.2] as const, // m past the shore, for homes over the water's edge
  apart: 1.2, // m at least between two homes
  high: [0.5, 1.6] as const, // m above the ground, the middle of the heights it keeps to
  highWater: [0.4, 0.9] as const, // the same over the water
  lowest: 0.3, // m above the ground
  highest: 2,
  rise: { amount: 0.45, period: [7, 16] as const }, // m it rises and sinks either way, over s
  speed: [0.25, 0.45] as const, // m/s
  turn: { rate: 1.1, period: [3, 9] as const }, // radians a second it wanders round, changing over s
  turnBack: 0.6, // share of ROAM from home where it starts turning back
  wary: 0.8, // m outside KEEP_OUT where it starts turning away from the landing
  steer: 3, // radians a second it turns for each radian it faces away from where it heads
  home: 3.5, // radians a second at most it turns back or away: a circle a quarter meter across at its fastest
};
const ROAM = 1.2; // m from its home it wanders
const KEEP_OUT = 1.8; // m kept clear round the landing, where the figure stands
const LIT = [0.2, 0.9] as const; // how far into the night their lanterns fade in over
const SHELTER = 0.5; // how hard it rains when they have all gone out
const HALO = { size: 0.16, dim: 0.45, flash: 1 }; // m across; its opacity between flashes, and added at one
const SEED = 41;

interface Wanderer {
  firefly: Firefly;
  halo: THREE.Sprite;
  home: THREE.Vector2;
  high: number; // m above the ground, the middle of its heights
  rise: { rate: number; phase: number };
  turn: { rates: [number, number]; phases: [number, number] };
}

export class Fireflies extends THREE.Group {
  private readonly wanderers: Wanderer[] = [];
  private readonly heightAt: (x: number, z: number) => number;
  private time = 0;
  private frames = 0;

  constructor(theme: Theme, heightAt: (x: number, z: number) => number) {
    super();
    this.name = 'fireflies';
    this.heightAt = heightAt;
    this.visible = false; // until dusk
    const random = seededRandom(SEED);
    const texture = haloTexture();
    const homes: THREE.Vector2[] = [];
    const landing = new THREE.Vector2(LANDING.x, LANDING.z);
    const round = Math.atan2(LANDING.z, LANDING.x);

    for (let attempt = 0; this.wanderers.length < SWARM.count && attempt < 200 * SWARM.count; attempt++) {
      const overWater = this.wanderers.length < SWARM.water;
      const angle = round + between(random, -SWARM.around, SWARM.around);
      const [nearest, furthest] = overWater ? SWARM.edge : SWARM.land;
      const out = shoreRadius(angle) + between(random, nearest, furthest);
      const home = new THREE.Vector2(out * Math.cos(angle), out * Math.sin(angle));
      const fromLanding = home.distanceTo(landing);
      if (fromLanding < SWARM.reach[0] || fromLanding > SWARM.reach[1] || home.y > landing.y + SWARM.behind) continue;
      if (homes.some((h) => h.distanceTo(home) < SWARM.apart)) continue;
      homes.push(home);

      const firefly = new Firefly({
        theme,
        seed: Math.floor(random() * 1e6),
        speed: between(random, ...SWARM.speed),
        light: false,
      });
      firefly.traverse((object) => (object.castShadow = false)); // too small and far to shadow anything
      const lantern = firefly.lantern;
      keepSharp(lantern.material as THREE.Material);
      lantern.geometry.computeBoundingSphere();
      const halo = new THREE.Sprite(
        new THREE.SpriteMaterial({
          map: texture,
          color: theme.colors.glow,
          transparent: true,
          depthWrite: false,
          fog: false,
          // Adds its light, and leaves the picture's alpha as it was.
          blending: THREE.CustomBlending,
          blendEquation: THREE.AddEquation,
          blendSrc: THREE.SrcAlphaFactor,
          blendDst: THREE.OneFactor,
          blendSrcAlpha: THREE.ZeroFactor,
          blendDstAlpha: THREE.OneFactor,
        }),
      );
      halo.position.copy(lantern.geometry.boundingSphere!.center);
      halo.scale.setScalar(HALO.size);
      firefly.body.add(halo);

      firefly.position.set(home.x, 0, home.y);
      firefly.rotation.y = 2 * Math.PI * random();
      const [low, top] = overWater ? SWARM.highWater : SWARM.high;
      const high = between(random, low, top);
      const rate = (period: readonly [number, number]) => (2 * Math.PI) / between(random, ...period);
      const wanderer: Wanderer = {
        firefly,
        halo,
        home,
        high,
        rise: { rate: rate(SWARM.rise.period), phase: 2 * Math.PI * random() },
        turn: { rates: [rate(SWARM.turn.period), rate(SWARM.turn.period)], phases: [2 * Math.PI * random(), 2 * Math.PI * random()] },
      };
      firefly.position.y = this.flyingHeight(wanderer, 0);
      this.wanderers.push(wanderer);
      this.add(firefly);
    }
  }

  // Moves them on by delta seconds, `night` being how far into the night it
  // is (0 day, 1 night) and `rain` how hard it rains (0 dry, 1 hard): they
  // shelter from the rain, their lanterns fading out while it lasts. Call
  // once per frame. By day they rest, hidden.
  update(delta: number, night: number, rain = 0): void {
    // Drawn in the first frame whatever the time, so their materials are
    // compiled with the lake's: compiled at the first dusk, they froze the
    // page for 0.2 s. By day their lanterns and halos are dark then, and they
    // are gone the next frame.
    this.visible = night > 0 || this.frames++ === 0;
    if (!this.visible) return;
    this.time += delta;
    const t = this.time;
    const lit = THREE.MathUtils.smoothstep(night, LIT[0], LIT[1]) * (1 - THREE.MathUtils.smoothstep(rain, 0, SHELTER));
    const landing = new THREE.Vector2(LANDING.x, LANDING.z);
    const at = new THREE.Vector2();
    for (const w of this.wanderers) {
      const f = w.firefly;
      at.set(f.position.x, f.position.z);
      // Wandering round, in turns that come and go; then, more and more as
      // it nears the edge of its area, turning back home instead, and away
      // from the landing as it comes near that.
      const wander = SWARM.turn.rate * (0.65 * Math.sin(w.turn.rates[0] * t + w.turn.phases[0]) + 0.35 * Math.sin(w.turn.rates[1] * t + w.turn.phases[1]));
      const homeward = THREE.MathUtils.smoothstep(at.distanceTo(w.home), SWARM.turnBack * ROAM, ROAM);
      const shy = THREE.MathUtils.smoothstep(-at.distanceTo(landing), -(KEEP_OUT + SWARM.wary), -KEEP_OUT);
      const toward = (goal: number) => THREE.MathUtils.clamp(SWARM.steer * shortest(goal - f.rotation.y), -SWARM.home, SWARM.home);
      const home = toward(heading(at, w.home));
      const away = toward(heading(landing, at));
      f.rotation.y += (wander * (1 - Math.max(homeward, shy)) + home * homeward * (1 - shy) + away * shy) * delta;
      f.position.y = this.flyingHeight(w, t);
      f.lit = lit;
      f.update(delta);
      // Never into the figure's space, however it turned.
      at.set(f.position.x, f.position.z).sub(landing);
      if (at.length() < KEEP_OUT) {
        at.setLength(KEEP_OUT).add(landing);
        f.position.x = at.x;
        f.position.z = at.y;
      }
      (w.halo.material as THREE.SpriteMaterial).opacity = lit * (HALO.dim + HALO.flash * f.flash);
    }
  }

  // Where its origin goes for its body to fly at its height now over the
  // ground under it (the figure's body is Firefly.HOVER above its origin).
  private flyingHeight(w: Wanderer, t: number): number {
    const f = w.firefly;
    const above = THREE.MathUtils.clamp(w.high + SWARM.rise.amount * Math.sin(w.rise.rate * t + w.rise.phase), SWARM.lowest, SWARM.highest);
    return this.heightAt(f.position.x, f.position.z) + above - Firefly.HOVER;
  }
}

// The way (rotation.y) from one point to another, for a figure that faces +z.
function heading(from: THREE.Vector2, to: THREE.Vector2): number {
  return Math.atan2(to.x - from.x, to.y - from.y);
}

// An angle's difference the short way round, between -pi and pi.
function shortest(angle: number): number {
  return THREE.MathUtils.euclideanModulo(angle + Math.PI, 2 * Math.PI) - Math.PI;
}

// A soft round glow, white, brightest in the middle and gone at the edge,
// made once.
let halo: THREE.DataTexture | null = null;

function haloTexture(): THREE.DataTexture {
  if (halo) return halo;
  const n = 64;
  const data = new Uint8Array(4 * n * n);
  for (let j = 0; j < n; j++) {
    for (let i = 0; i < n; i++) {
      const u = (2 * (i + 0.5)) / n - 1;
      const v = (2 * (j + 0.5)) / n - 1;
      const d2 = Math.min(1, u * u + v * v);
      const k = 4 * (j * n + i);
      data[k] = data[k + 1] = data[k + 2] = 255;
      data[k + 3] = Math.round(255 * Math.pow(1 - d2, 2));
    }
  }
  halo = new THREE.DataTexture(data, n, n);
  halo.magFilter = THREE.LinearFilter;
  halo.minFilter = THREE.LinearMipmapLinearFilter;
  halo.generateMipmaps = true;
  halo.needsUpdate = true;
  return halo;
}
