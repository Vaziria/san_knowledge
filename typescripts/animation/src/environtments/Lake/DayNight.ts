import * as THREE from 'three';
import type { TimeOfDay } from '../../previews';
import { sceneAt, type SceneAt, type Theme } from '../../theme';

// Day and night at the lake, as the user asked ("in lake add day and night").
// On their own they take turns: 6 minutes of day, a minute of dusk, 4 of
// night and a minute of dawn, dusk and dawn easing from one to the other.
// The first dusk comes a minute after the lake is shown, so night is seen
// without a long wait, as the first fog is. The Time of day setting
// (?time=, previews.ts) can hold it at day or at night instead; a change
// eases there over a few seconds, so nothing snaps, except before the first
// frame, when the lake is built straight into it.
//
// `night` says how far into the night it is (0 by day, 1 at night). Under a
// shower's cloud (`overcast`, which the lake sets from its weather) the light
// dims a step more toward the night's: `dim` is how far, and `scene` what the
// sky's colours and the lights are now (sceneAt() in theme.ts, the theme's day
// values mixed with its night ones by `dim`). The stage
// dims its lights and darkens the background by it (Environment.dayNight),
// and the lake colours its sky, fog, haze and mist the same way (Lake.ts).

const DAY = 360; // s
const DUSK = 60;
const NIGHT = 240;
const DAWN = 60;
const FIRST_DUSK = 60; // s after the lake is shown
const EASE = 1.2; // s: a picked day or night eases in over about three times this
const OVERCAST = 0.3; // how far a full shower's cloud dims the light toward the night's

// How far into the night the cycle is, `time` seconds after the lake is
// shown: 0 by day, 1 at night, easing between at dusk and dawn.
export function cycleNight(time: number): number {
  const t = THREE.MathUtils.euclideanModulo(time + DAY - FIRST_DUSK, DAY + DUSK + NIGHT + DAWN);
  if (t < DAY) return 0;
  if (t < DAY + DUSK) return THREE.MathUtils.smootherstep(t, DAY, DAY + DUSK);
  if (t < DAY + DUSK + NIGHT) return 1;
  return 1 - THREE.MathUtils.smootherstep(t, DAY + DUSK + NIGHT, DAY + DUSK + NIGHT + DAWN);
}

export class DayNight {
  night = 0;
  overcast = 0; // 0 clear, 1 a full shower's cloud; the lake sets it every frame
  dim = 0; // how far the light is toward the night's: the night, and a step more when overcast
  readonly scene: SceneAt;
  private readonly theme: Theme;
  private time = 0;
  private mode: TimeOfDay = 'cycle';
  private started = false;

  constructor(theme: Theme) {
    this.theme = theme;
    this.scene = sceneAt(theme, 0);
  }

  // Follows the Time of day setting: the cycle, or held at day or night.
  set(mode: TimeOfDay): void {
    this.mode = mode;
    if (this.started) return;
    this.night = this.target();
    this.dim = this.night + (1 - this.night) * OVERCAST * this.overcast;
    sceneAt(this.theme, this.dim, this.scene);
  }

  // Moves the time of day on by delta seconds; call once per frame.
  update(delta: number): void {
    this.started = true;
    this.time += delta;
    this.night += (this.target() - this.night) * (1 - Math.exp(-delta / EASE));
    this.dim = this.night + (1 - this.night) * OVERCAST * this.overcast;
    sceneAt(this.theme, this.dim, this.scene);
  }

  private target(): number {
    if (this.mode === 'day') return 0;
    if (this.mode === 'night') return 1;
    return cycleNight(this.time);
  }
}
