import * as THREE from 'three';
import { barkTexture } from './textures/bark';
import { woodGrain } from './textures/woodGrain';

type Color = THREE.ColorRepresentation;

// A theme names colours by role, not by object, so a brand palette can be
// swapped in without touching the figures. Figures and the scene take every
// colour from a theme; no figure hardcodes a colour.
export interface Theme {
  name: string;
  scene: {
    background: Color;
    zenith: Color; // the sky straight overhead: a sky deepens to it from background at the horizon
    floor: Color;
    sky: Color; // hemisphere light from above
    ground: Color; // hemisphere light bounced off the floor
    water: Color; // a lake's surface; the lakebed fades toward it with depth
    grass: Color; // a lawn's blades (the ground between them is a shade darker), and any foliage, such as leaves
    autumn: Color; // leaves turned in autumn, such as an autumn maple's; must stand out from grass
    flower: Color; // blossoms, such as a chive's flower heads; must stand out from grass
    stone: Color; // rocks lying about, such as on a lakeside; must stand out from the floor
    sand: Color; // loose sand lying on the land, such as a beach (environtments/terrains/SandTerrain.ts); must stand out from the floor
    light: Color; // key (sun) light
    ambientIntensity: number;
    lightIntensity: number;
    // The same at night, for an environment with day and night (the lake):
    // the sky at the horizon and overhead, and dimmer, cooler lights, the key
    // light turned moonlight. sceneAt() mixes them with the day's at dusk and
    // dawn. Dim, not black: a figure must still read.
    night: {
      background: Color;
      zenith: Color;
      sky: Color;
      ground: Color;
      light: Color;
      ambientIntensity: number;
      lightIntensity: number;
    };
  };
  colors: {
    body: Color; // main surface, the brand's primary colour
    trim: Color; // secondary surface: feet, rims, panels
    light: Color; // light neutral: white keys
    dark: Color; // dark neutral: black keys
    ink: Color; // small details drawn on body (printed marks, labels); must contrast with body
    marker: Color; // indicators on knobs and wheels; must contrast with accent
    metal: Color; // metal parts: plates, pedals
    wood: Color; // bare or varnished wood: planks, benches, a rudder; a natural material like metal
    fur: Color; // an animal's coat; each animal mixes it with trim, light and dark into its own (a fox toward trim, a wolf toward grey)
    accent: Color; // every control (knobs, pads): the brand's accent, or the complement if it has none
    glow: Color; // something that gives off light, such as a firefly's lantern: emissive, so it shows exactly this colour
  };
  finish: {
    roughness: number;
    clearcoat: number;
    clearcoatRoughness: number;
    // Fabric fuzz for felt and plush: a soft glow at grazing angles.
    sheen?: number;
    sheenRoughness?: number;
    sheenColor?: Color;
  };
}

// A felt toy in natural penguin colours, from the penguin reference image:
// black felt, off-white belly and face, orange beak and feet, on warm grey.
export const felt: Theme = {
  name: 'felt',
  scene: {
    background: 0xbdb8b3,
    zenith: 0x93b3d3, // muted sky-blue felt over the warm grey horizon
    floor: 0xc4bfba,
    sky: 0xffffff,
    ground: 0x9d9893,
    water: 0x93a4ab, // muted grey-blue, a natural lake in felt
    grass: 0x7c8f5a, // muted sage green, natural grass in felt
    autumn: 0xc07a3e, // muted rust-orange felt, like a maple in autumn
    flower: 0xa98bbd, // muted lilac felt, like chive flowers
    stone: 0x8f8a85, // warm grey stone, darker than the floor
    sand: 0xd8c6a2, // muted sand-beige felt, warmer and a little lighter than the floor
    light: 0xfff6ee,
    ambientIntensity: 1.2,
    lightIntensity: 2.2,
    night: {
      background: 0x2b3242, // dusky blue-grey felt at the horizon
      zenith: 0x141b2e, // deep navy felt overhead
      sky: 0x9fb2d9,
      ground: 0x3b3a40,
      light: 0xc9d6f2, // pale blue moonlight
      ambientIntensity: 0.45,
      lightIntensity: 0.75,
    },
  },
  colors: {
    body: 0x1f1f22,
    trim: 0xf08a1c,
    light: 0xf2efe8,
    dark: 0x0b0b0c,
    ink: 0xf2efe8,
    marker: 0xf2efe8,
    metal: 0x9a9a9a,
    wood: 0xb07d4f, // natural wood
    fur: 0x9a6b45, // warm brown felt, a natural coat
    accent: 0xf08a1c,
    glow: 0xf7d64a, // warm yellow felt, like a firefly's lantern
  },
  finish: {
    roughness: 1,
    clearcoat: 0,
    clearcoatRoughness: 1,
    sheen: 0.3,
    sheenRoughness: 0.9,
    sheenColor: 0xffffff,
  },
};

// Felt is the only theme: the user dropped the others (getresolved and its
// dark mode, pastel, purple, studio) on 2026-09-26, "just keep felt, we dont
// need other". Every figure and environment still takes it as its `theme`.
export const defaultTheme = felt;

// A surface in the theme's finish.
export function surface(theme: Theme, color: Color): THREE.MeshPhysicalMaterial {
  return new THREE.MeshPhysicalMaterial({ color, ...theme.finish });
}

// A glossy surface whatever the theme's finish, for parts that are shiny on
// any material, such as eyes.
export function gloss(color: Color): THREE.MeshPhysicalMaterial {
  return new THREE.MeshPhysicalMaterial({ color, roughness: 0.2, clearcoat: 1, clearcoatRoughness: 0.05 });
}

export function metal(theme: Theme): THREE.MeshPhysicalMaterial {
  return new THREE.MeshPhysicalMaterial({ color: theme.colors.metal, roughness: 0.35, metalness: 0.8 });
}

// Wood in the theme's finish, its planks and grain drawn by the wood-grain
// texture. The surface needs texture coordinates in meters, with u along the
// grain (see textures/woodGrain.ts).
export function wood(theme: Theme): THREE.MeshPhysicalMaterial {
  const material = surface(theme, theme.colors.wood);
  material.map = woodGrain();
  return material;
}

// Bark in the theme's finish: the wood colour, darkened and roughened into
// bark by the bark texture. The surface needs texture coordinates in meters,
// u along the branch and v one tile around it (see textures/bark.ts).
export function bark(theme: Theme): THREE.MeshPhysicalMaterial {
  const material = surface(theme, theme.colors.wood);
  material.map = barkTexture();
  return material;
}

// Colour-accurate output: hex colours are sRGB and no tone mapping is applied,
// so a lit face shows its hex value times its shading. Measured on the
// getresolved brand's indigo #4F46E5 (a theme since dropped): none renders
// #4941D3, Neutral #3224CE (it darkens the low channels, oversaturating the
// colour).
export function applyRendererTheme(renderer: THREE.WebGLRenderer): void {
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.NoToneMapping;
}

// The scene's sky and lights at a time of day: the theme's own by day (night
// 0), its night values at night (1), and mixed between them at dusk and dawn.
// Colours mix as three.js keeps them, linear, as light adds up.
export interface SceneAt {
  background: THREE.Color;
  zenith: THREE.Color;
  sky: THREE.Color;
  ground: THREE.Color;
  light: THREE.Color;
  ambientIntensity: number;
  lightIntensity: number;
}

const nightColor = new THREE.Color(); // reused by sceneAt()

export function sceneAt(theme: Theme, night: number, out?: SceneAt): SceneAt {
  const day = theme.scene;
  const dark = day.night;
  const at = out ?? { background: new THREE.Color(), zenith: new THREE.Color(), sky: new THREE.Color(), ground: new THREE.Color(), light: new THREE.Color(), ambientIntensity: 0, lightIntensity: 0 };
  for (const key of ['background', 'zenith', 'sky', 'ground', 'light'] as const) at[key].set(day[key]).lerp(nightColor.set(dark[key]), night);
  at.ambientIntensity = THREE.MathUtils.lerp(day.ambientIntensity, dark.ambientIntensity, night);
  at.lightIntensity = THREE.MathUtils.lerp(day.lightIntensity, dark.lightIntensity, night);
  return at;
}
