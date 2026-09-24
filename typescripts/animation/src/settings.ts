import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';
import { isCameraDirection, type CameraDirection } from './camera';
import { defaultFigure, environments, figureEnvironment, previews } from './previews';
import { stories } from './stories';
import { defaultTheme, themes } from './theme';

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
  figure: string; // a key of previews
  environment: string | null; // a key of environments, or null for the figure's own
  theme: string; // a key of themes
  kuwahara: Kuwahara;
  camera: CameraDirection; // which way the camera looks at the figure (camera.ts)
}

export const KUWAHARA_DEFAULT_RADIUS = 5;
export const KUWAHARA_MAX_RADIUS = 12; // CSS pixels; up to 24 device pixels at a pixel ratio of 2

// ?story=<name> plays a story, none by default.
// ?figure=<name> picks the figure, the one being worked on by default.
// ?environment=<name> picks the environment, the figure's own by default.
// ?theme=<name> picks the theme, defaultTheme (felt) by default.
// ?kuwahara=<radius in CSS pixels> sets the filter's strength, 0 = off.
// ?camera=<direction> looks at the figure from that side (camera.ts), from
// the figure's own view by default.
function fromUrl(params: URLSearchParams): Settings {
  const story = params.get('story') ?? '';
  const figure = params.get('figure') ?? '';
  const environment = params.get('environment') ?? '';
  const theme = params.get('theme') ?? '';
  const radius = Number(params.get('kuwahara') ?? KUWAHARA_DEFAULT_RADIUS);
  const camera = params.get('camera') ?? '';
  return {
    story: Object.hasOwn(stories, story) ? story : null,
    figure: Object.hasOwn(previews, figure) ? figure : defaultFigure,
    environment: Object.hasOwn(environments, environment) ? environment : null,
    theme: Object.hasOwn(themes, theme) ? theme : defaultTheme.name,
    kuwahara:
      radius > 0
        ? { on: true, radius: Math.min(KUWAHARA_MAX_RADIUS, Math.max(1, Math.round(radius))) }
        : { on: false, radius: KUWAHARA_DEFAULT_RADIUS },
    camera: isCameraDirection(camera) && camera !== 'free' ? camera : 'figure',
  };
}

// How each setting is written to its query parameter; null removes it.
const toParam: { [K in keyof Settings]: (value: Settings[K]) => string | null } = {
  story: (story) => story,
  figure: (figure) => figure,
  environment: (environment) => environment,
  theme: (theme) => theme,
  kuwahara: ({ on, radius }) => String(on ? radius : 0),
  // A dragged view can't be written down, so free leaves no parameter, like
  // the figure's own view.
  camera: (camera) => (camera === 'figure' || camera === 'free' ? null : camera),
};

export const settings = createStore<Settings>()(() => fromUrl(new URLSearchParams(location.search)));

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
// goes to the figure's own view, and the menu says so.
function unfree(camera: CameraDirection): CameraDirection {
  return camera === 'free' ? 'figure' : camera;
}

export function setStory(story: string | null): void {
  settings.setState((s) => ({ story, camera: unfree(s.camera) }));
}

export function setFigure(figure: string): void {
  settings.setState((s) => ({ figure, camera: unfree(s.camera) }));
}

export function setEnvironment(environment: string): void {
  settings.setState({ environment });
}

export function setTheme(theme: string): void {
  settings.setState({ theme });
}

export function setKuwahara(change: Partial<Kuwahara>): void {
  settings.setState((s) => ({ kuwahara: { ...s.kuwahara, ...change } }));
}

export function setCamera(camera: CameraDirection): void {
  settings.setState({ camera });
}
