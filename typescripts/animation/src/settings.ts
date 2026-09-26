import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';
import { isCameraDirection, type CameraDirection } from './camera';
import { hasPart } from './parts';
import { defaultFigure, environments, figureEnvironment, previews, TIMES_OF_DAY, type TimeOfDay } from './previews';
import { stories } from './stories';
import { terrains } from './terrains';

// The preview's settings, in one store shared by the panel (React) and the
// stage (Three.js). The panel reads them with useSettings and changes them
// with the setters below; main.tsx applies every change to the stage. Each
// setting is kept in the URL under its own name, so a reload keeps it.

export interface Kuwahara {
  on: boolean;
  radius: number; // CSS pixels, 1..KUWAHARA_MAX_RADIUS; kept while off
}

export interface Settings {
  story: string | null; // a key of stories, or null to show the picked figure
  terrain: string | null; // a key of terrains, shown on its own instead of the figure (terrains.ts), or null to show the figure
  figure: string; // a key of previews
  part: string | null; // one of the figure's parts to show on its own (parts.ts), or null for the whole figure
  environment: string | null; // a key of environments, or null for the figure's own
  kuwahara: Kuwahara;
  camera: CameraDirection; // which way the camera looks at the figure (camera.ts)
  walkSpeed: number; // m/s: how fast the camera walks (camera "walk")
  eyeHeight: number; // m: how high over the ground the walking camera looks from
  time: TimeOfDay; // day and night taking turns (cycle), or held at one; only the lake has night
}

export const KUWAHARA_DEFAULT_RADIUS = 5;
export const KUWAHARA_MAX_RADIUS = 12; // CSS pixels; up to 24 device pixels at a pixel ratio of 2
// A person's walk and eyes, and how far they go either way.
export const WALK_SPEED = { least: 0.5, most: 10, usual: 1.4 }; // m/s
export const EYE_HEIGHT = { least: 0.2, most: 10, usual: 1.6 }; // m

// ?story=<name> plays a story, none by default.
// ?terrain=<name> shows a terrain on its own instead of the figure, none by
// default.
// ?figure=<name> picks the figure, the one being worked on by default.
// ?part=<name> shows only that part of it, the whole figure by default.
// ?environment=<name> picks the environment, the figure's own by default.
// ?kuwahara=<radius in CSS pixels> sets the filter's strength, 0 = off.
// ?camera=<direction> looks at the figure from that side (camera.ts), from
// the figure's own view by default; walk walks it.
// ?walkSpeed=<m/s> and ?eyeHeight=<m> are for walking, 1.4 and 1.6 by default.
// ?time=day or night holds the lake at that time of day; cycle, the
// default, lets them take turns.
function fromUrl(params: URLSearchParams): Settings {
  const story = params.get('story') ?? '';
  const terrain = params.get('terrain') ?? '';
  const picked = params.get('figure') ?? '';
  const figure = Object.hasOwn(previews, picked) ? picked : defaultFigure;
  const part = params.get('part') ?? '';
  const environment = params.get('environment') ?? '';
  const radius = Number(params.get('kuwahara') ?? KUWAHARA_DEFAULT_RADIUS);
  const camera = params.get('camera') ?? '';
  const time = params.get('time') ?? '';
  return {
    story: Object.hasOwn(stories, story) ? story : null,
    terrain: Object.hasOwn(terrains, terrain) ? terrain : null,
    figure,
    part: hasPart(figure, part) ? part : null,
    environment: Object.hasOwn(environments, environment) ? environment : null,
    kuwahara:
      radius > 0
        ? { on: true, radius: Math.min(KUWAHARA_MAX_RADIUS, Math.max(1, Math.round(radius))) }
        : { on: false, radius: KUWAHARA_DEFAULT_RADIUS },
    camera: isCameraDirection(camera) && camera !== 'free' ? camera : 'figure',
    walkSpeed: tenths(params.get('walkSpeed'), WALK_SPEED),
    eyeHeight: tenths(params.get('eyeHeight'), EYE_HEIGHT),
    time: TIMES_OF_DAY.find((t) => t === time) ?? 'cycle',
  };
}

// A number from the URL, to a tenth and within its range, or the usual one.
function tenths(text: string | null, range: { least: number; most: number; usual: number }): number {
  const value = Number(text);
  if (text === null || text === '' || !Number.isFinite(value)) return range.usual;
  return Math.min(range.most, Math.max(range.least, Math.round(value * 10) / 10));
}

// How each setting is written to its query parameter; null removes it.
const toParam: { [K in keyof Settings]: (value: Settings[K]) => string | null } = {
  story: (story) => story,
  terrain: (terrain) => terrain,
  figure: (figure) => figure,
  part: (part) => part,
  environment: (environment) => environment,
  kuwahara: ({ on, radius }) => String(on ? radius : 0),
  // A dragged view can't be written down, so free leaves no parameter, like
  // the figure's own view.
  camera: (camera) => (camera === 'figure' || camera === 'free' ? null : camera),
  walkSpeed: (speed) => String(speed),
  eyeHeight: (height) => String(height),
  time: (time) => (time === 'cycle' ? null : time),
};

export const settings = createStore<Settings>()(() => fromUrl(new URLSearchParams(location.search)));

// A part the figure doesn't have shows the whole figure, and leaves the URL.
{
  const params = new URLSearchParams(location.search);
  if (params.has('part') && settings.getState().part === null) {
    params.delete('part');
    history.replaceState(null, '', `?${params}${location.hash}`);
  }
}

// Only settings that change are written, so the rest keep following their
// defaults: a figure that was never picked still follows the one being
// worked on.
settings.subscribe((state, previous) => {
  const params = new URLSearchParams(location.search);
  for (const key of Object.keys(toParam) as (keyof Settings)[]) {
    if (state[key] !== previous[key]) writeParam(params, key, state[key]);
  }
  history.replaceState(null, '', `?${params}${location.hash}`);
});

function writeParam<K extends keyof Settings>(params: URLSearchParams, key: K, value: Settings[K]): void {
  const text = toParam[key](value);
  if (text === null) params.delete(key);
  else params.set(key, text);
}

// The settings as a query string, every one written, and back: the dev
// server's hidden browser draws the stream from them (stream.ts).
export function settingsQuery(s: Settings): string {
  const params = new URLSearchParams();
  for (const key of Object.keys(toParam) as (keyof Settings)[]) writeParam(params, key, s[key]);
  return params.toString();
}

export function settingsFromQuery(query: string): Settings {
  return fromUrl(new URLSearchParams(query));
}

// The part of the settings a component uses; it re-renders when that changes.
export function useSettings<T>(select: (settings: Settings) => T): T {
  return useStore(settings, select);
}

// The environment to show: the one picked, or the figure's own until then. A
// pick stays when the figure changes.
export function environmentOf(s: Settings): string {
  return s.environment ?? figureEnvironment(s.figure);
}

// A new figure or story brings the camera to its own view from the picked
// direction; a dragged (free) view has no direction to keep, so the camera
// goes to the figure's own view, and the menu says so. A walking camera
// walks on from where it is.
function unfree(camera: CameraDirection): CameraDirection {
  return camera === 'free' ? 'figure' : camera;
}

export function setStory(story: string | null): void {
  settings.setState((s) => ({ story, camera: unfree(s.camera) }));
}

// A part of it, or null for the whole figure. Picking a figure shows it
// again in place of a terrain.
export function setFigure(figure: string, part: string | null = null): void {
  settings.setState((s) => ({ figure, part, terrain: null, camera: unfree(s.camera) }));
}

// Shows a terrain on its own in place of the figure, which keeps its pick
// until a figure is picked again.
export function setTerrain(terrain: string): void {
  settings.setState((s) => ({ terrain, camera: unfree(s.camera) }));
}

export function setEnvironment(environment: string): void {
  settings.setState({ environment });
}

export function setKuwahara(change: Partial<Kuwahara>): void {
  settings.setState((s) => ({ kuwahara: { ...s.kuwahara, ...change } }));
}

export function setCamera(camera: CameraDirection): void {
  settings.setState({ camera });
}

export function setWalkSpeed(walkSpeed: number): void {
  settings.setState({ walkSpeed });
}

export function setEyeHeight(eyeHeight: number): void {
  settings.setState({ eyeHeight });
}

export function setTime(time: TimeOfDay): void {
  settings.setState({ time });
}
