import * as THREE from 'three';
import { between, seededRandom } from '../../figures/Grass/parts';
import type { Theme } from '../../theme';
import { createWind, type Wind } from '../wind';

// The lake's weather, which changes now and then on its own: fog rolls in
// over the lake, hangs for a while and clears, and windy spells come and go,
// gusting while they last, between stretches of still air with only a
// breath of wind. Fog and wind keep their own times, so they sometimes come
// together. The first fog and the first wind come soon after the lake is
// shown, so they are seen without a long wait; after that the times are
// drawn from a seeded random generator, the same on every load.
//
// The fog is the scene's fog (the stage puts it in the scene, see
// Environment.fog in previews.ts), in the theme's background colour, so the
// far shore and the hills fade into the sky while a figure a few meters from
// the camera stays clear. The wind (../wind.ts) sways the sward, the meadow
// and the trees' crowns, and the water (Water.ts) roughens in it. Both ease
// in and out, so nothing snaps.

const FOG = {
  first: 25, // s until the first fog
  lasts: [20, 35] as const, // s it hangs
  gap: [40, 90] as const, // s of clear air between fogs
  density: [0.05, 0.09] as const, // of the scene's exponential fog: at 0.07 the far shore, 20 m off, is 86% hidden and a figure 2 m off 2%
  ease: 4, // s: it rolls in and clears over about three times this
};
const WIND = {
  first: 8, // s until the first windy spell
  lasts: [12, 25] as const, // s a spell lasts
  gap: [15, 40] as const, // s of still air between spells
  level: [0.55, 1] as const, // how strong a spell is
  breath: 0.06, // how strong the air is between spells
  gusts: 0.45, // how far gusts rise and fall within a spell, as a share of its strength
  ease: 1.5, // s it takes to follow a change, about a third of the way
  prevailing: -0.6, // radians from +x toward +z it mostly blows toward
  veer: 0.8, // radians a spell may blow either side of that
  turn: 6, // s to swing round to a new spell's direction, about a third of the way
};
const SEED = 3;

export class Weather {
  readonly fog: THREE.FogExp2;
  readonly wind: Wind = createWind();
  private readonly random = seededRandom(SEED);
  private time = 0;
  private fogOn = false;
  private fogNext = FOG.first;
  private fogTarget = 0;
  private windOn = false;
  private windNext = WIND.first;
  private windLevel = WIND.breath;
  private readonly heading = new THREE.Vector2(Math.cos(WIND.prevailing), Math.sin(WIND.prevailing));

  constructor(theme: Theme) {
    this.fog = new THREE.FogExp2(new THREE.Color(theme.scene.background), 0);
    this.wind.direction.value.copy(this.heading);
    this.wind.strength.value = WIND.breath;
  }

  // Moves the weather on by delta seconds; call once per frame.
  update(delta: number): void {
    this.time += delta;
    const random = this.random;

    if (this.time >= this.fogNext) {
      this.fogOn = !this.fogOn;
      const [shortest, longest] = this.fogOn ? FOG.lasts : FOG.gap;
      this.fogNext = this.time + between(random, shortest, longest);
      this.fogTarget = this.fogOn ? between(random, ...FOG.density) : 0;
    }
    this.fog.density += (this.fogTarget - this.fog.density) * (1 - Math.exp(-delta / FOG.ease));

    if (this.time >= this.windNext) {
      this.windOn = !this.windOn;
      const [shortest, longest] = this.windOn ? WIND.lasts : WIND.gap;
      this.windNext = this.time + between(random, shortest, longest);
      this.windLevel = this.windOn ? between(random, ...WIND.level) : WIND.breath;
      if (this.windOn) {
        const angle = WIND.prevailing + between(random, -WIND.veer, WIND.veer);
        this.heading.set(Math.cos(angle), Math.sin(angle));
      }
    }
    // Gusts: a slow swell and a quicker one, together never quite regular.
    const t = this.time;
    const gust = 0.5 + 0.3 * Math.sin(1.1 * t) + 0.2 * Math.sin(2.9 * t + 1.3);
    const target = this.windLevel * (1 - WIND.gusts + WIND.gusts * gust);
    const strength = this.wind.strength;
    strength.value += (target - strength.value) * (1 - Math.exp(-delta / WIND.ease));
    this.wind.direction.value.lerp(this.heading, 1 - Math.exp(-delta / WIND.turn)).normalize();
    this.wind.time.value = t;
  }
}
