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
    floor: Color;
    sky: Color; // hemisphere light from above
    ground: Color; // hemisphere light bounced off the floor
    water: Color; // a lake's surface; the lakebed fades toward it with depth
    grass: Color; // a lawn's blades (the ground between them is a shade darker), and any foliage, such as leaves
    stone: Color; // rocks lying about, such as on a lakeside; must stand out from the floor
    light: Color; // key (sun) light
    ambientIntensity: number;
    lightIntensity: number;
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
    accent: Color; // every control (knobs, pads): the brand's accent, or the complement if it has none
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

export const pastel: Theme = {
  name: 'pastel',
  scene: {
    background: 0xf7eef3,
    floor: 0xefdde7,
    sky: 0xffffff,
    ground: 0xf2c6d6,
    water: 0xe6adc4, // a step deeper than ground, same pink
    grass: 0xeecbd9, // a pink lawn between floor and ground; mint stays for controls
    stone: 0xcfa9ba, // a greyed pink, a step below ground
    light: 0xfff4ec,
    ambientIntensity: 1.2,
    lightIntensity: 2.2,
  },
  colors: {
    body: 0xffb7c5,
    trim: 0xff8fa8,
    light: 0xfff8ee,
    dark: 0x6b4e8c,
    ink: 0x2b2233,
    marker: 0x2b2233,
    metal: 0xe8c26a,
    wood: 0xebcfa8, // pale maple, beside the gold metal
    accent: 0x8ee3c8, // mint, opposite pink
  },
  finish: { roughness: 0.45, clearcoat: 1, clearcoatRoughness: 0.15 },
};

export const studio: Theme = {
  name: 'studio',
  scene: {
    background: 0x1a1a1c,
    floor: 0x2a2a2d,
    sky: 0xffffff,
    ground: 0x333333,
    water: 0x202329, // a cool dark grey, a step below the floor
    grass: 0x323533, // a dark grey lawn, a step above the floor
    stone: 0x4a4a4e, // mid grey, lighter than the floor so rocks show
    light: 0xffffff,
    ambientIntensity: 0.8,
    lightIntensity: 2.5,
  },
  colors: {
    body: 0x111111,
    trim: 0x2c2c31,
    light: 0xf5f3ee,
    dark: 0x151515,
    ink: 0xf5f3ee,
    marker: 0xffffff,
    metal: 0xb08d3c,
    wood: 0x6b4a33, // dark walnut
    accent: 0xff5a1f,
  },
  finish: { roughness: 0.3, clearcoat: 0.6, clearcoatRoughness: 0.1 },
};

// One violet hue (~260°) in steps from 50 (palest) to 900 (darkest), so every
// purple in the scene belongs to the same family. Gold metal and the yellow
// accent sit opposite violet on the colour wheel and give it warmth.
const violet = {
  50: 0xf4f0fb,
  100: 0xe7dff6,
  200: 0xcbb8f0,
  400: 0x9b7be0,
  600: 0x6d4bc4,
  800: 0x3b2a66,
  900: 0x24193f,
};

export const purple: Theme = {
  name: 'purple',
  scene: {
    background: violet[50],
    floor: violet[100],
    sky: 0xffffff,
    ground: violet[200],
    water: violet[200],
    grass: violet[200],
    stone: 0xa597c9, // a greyed violet between 200 and 400
    light: 0xfbf8ff,
    ambientIntensity: 1.1,
    lightIntensity: 2.3,
  },
  colors: {
    body: violet[400],
    trim: violet[600],
    light: 0xfbf8ff,
    dark: violet[800],
    ink: violet[900],
    marker: violet[900],
    metal: 0xe0c068,
    wood: 0xd4ae78, // honey, beside the gold metal and the yellow accent
    accent: 0xffc94d, // yellow, opposite violet
  },
  finish: { roughness: 0.45, clearcoat: 1, clearcoatRoughness: 0.15 },
};

// getresolved.id brand, from wargasipil/getresolved@dev: branding/guidelines/brand.html
// and the tokens in frontend/src/shared/index.css. Rules: docs/3d_modelling/brand_getresolved.md.
// Indigo is the primary, green is the accent the brand chose (not the colour-wheel
// complement), ink is the dark, and surfaces are neutral cool greys, not tinted.
const brand = {
  indigo: 0x4f46e5,
  indigoDeep: 0x4338ca,
  indigoLight: 0x818cf8,
  green: 0x10b981,
  greenLight: 0x34d399,
  ink: 0x0f172a,
  slate: 0x64748b,
};

export const getresolved: Theme = {
  name: 'getresolved',
  scene: {
    background: 0xf6f8fb, // --bg
    floor: 0xeef2f7, // --band, a step deeper than --bg
    sky: 0xffffff,
    ground: 0xe8edf3, // --line
    water: 0xc5cee1, // slate 300 #CBD5E1 with 5% indigo, the most tint the brand allows
    grass: 0xe2e8f0, // slate 200: green is only for controls, so a neutral lawn
    stone: 0x94a3b8, // slate 400 (dark --muted), a neutral grey
    light: 0xffffff,
    ambientIntensity: 1.1,
    lightIntensity: 2.3,
  },
  colors: {
    body: brand.indigo, // --primary
    trim: brand.indigoDeep, // --primary-hover, "depth"
    light: 0xffffff, // --surface
    dark: brand.ink,
    ink: 0xffffff, // white on indigo, as in the mark's ring
    marker: brand.ink, // on green; white on green is too faint
    metal: brand.slate,
    wood: 0xcbd5e1, // slate 300: the brand has no brown, so weathered grey wood, neutral like its surfaces
    accent: brand.green, // --accent, the resolve point
  },
  // Clean and modern, no heavy gloss: the brand forbids added effects.
  finish: { roughness: 0.5, clearcoat: 0.4, clearcoatRoughness: 0.3 },
};

// The brand's dark mode swaps primary and accent to their light variants.
export const getresolvedDark: Theme = {
  name: 'getresolved-dark',
  scene: {
    background: 0x0b1020, // dark --bg
    floor: 0x111a30, // dark --surface
    sky: 0xe2e8f0,
    ground: 0x18223c, // dark --surface-2
    water: 0x22304d, // dark --line
    grass: 0x18223c, // dark --surface-2, a neutral lawn as in the light theme
    stone: 0x475569, // slate 600 (--muted text), a neutral grey above the dark floor
    light: 0xffffff,
    ambientIntensity: 0.9,
    lightIntensity: 2.3,
  },
  colors: {
    body: brand.indigoLight, // dark --primary
    trim: brand.indigo,
    light: 0xffffff,
    dark: brand.ink,
    ink: brand.ink, // white is too faint on indigo-light
    marker: brand.ink,
    metal: 0x94a3b8, // dark --muted
    wood: brand.slate, // weathered grey wood, as in the light theme
    accent: brand.greenLight, // dark --accent
  },
  finish: { roughness: 0.5, clearcoat: 0.4, clearcoatRoughness: 0.3 },
};

// A felt toy in natural penguin colours, from the penguin reference image:
// black felt, off-white belly and face, orange beak and feet, on warm grey.
export const felt: Theme = {
  name: 'felt',
  scene: {
    background: 0xbdb8b3,
    floor: 0xc4bfba,
    sky: 0xffffff,
    ground: 0x9d9893,
    water: 0x93a4ab, // muted grey-blue, a natural lake in felt
    grass: 0x7c8f5a, // muted sage green, natural grass in felt
    stone: 0x8f8a85, // warm grey stone, darker than the floor
    light: 0xfff6ee,
    ambientIntensity: 1.2,
    lightIntensity: 2.2,
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
    accent: 0xf08a1c,
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

export const themes: Record<string, Theme> = {
  getresolved,
  'getresolved-dark': getresolvedDark,
  felt,
  pastel,
  purple,
  studio,
};
// Every figure starts in felt; pick another in the panel or with ?theme=.
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
// so a lit face shows its hex value times its shading. Measured on the brand
// indigo #4F46E5: none renders #4941D3, Neutral #3224CE (it darkens the low
// channels, oversaturating the colour).
export function applyRendererTheme(renderer: THREE.WebGLRenderer): void {
  renderer.outputColorSpace = THREE.SRGBColorSpace;
  renderer.toneMapping = THREE.NoToneMapping;
}
