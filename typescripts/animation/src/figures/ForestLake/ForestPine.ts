import * as THREE from 'three';
import { addTier, addTube, between, color, matte, mesh, palette, seededRandom, Shape, snowOn, vary, type Palette, type Season } from './parts';

// How green a pine is, for a forest of them not all alike: the season's
// green, a darker, bluer one, a lighter, yellower one, or gold (a larch in
// autumn, as the reference's autumn has among its pines).
export type PineTone = 'green' | 'dark' | 'light' | 'gold';

export interface ForestPineOptions {
  height?: number; // m, to the tip; 7 by default
  tiers?: number; // 4 by default, as the asset sheet draws it
  // Fewer facets, for pines seen only from afar (the forest lake's far
  // hills): six points to a tier, no shoulder, no roots. False by default.
  simple?: boolean;
  season?: Season; // spring by default; snow on its tiers in winter
  tone?: PineTone; // green by default
  seed?: number;
}

// The forest lake's pine (ForestPine.md), as its asset sheet draws it: tiers
// of green needles stacked into a cone, each with a skirt of drooping
// points, the lowest widest and the top one a narrow spire, lit yellow-green
// on top and dark underneath, on a short orange-brown trunk that flares into
// its roots. It keeps its needles all year; in winter snow lies on its tiers.
export class ForestPine extends THREE.Group {
  readonly trunk: THREE.Mesh;
  readonly crown: THREE.Mesh;

  constructor(options: ForestPineOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 11);
    const height = options.height ?? 7;
    const tiers = options.tiers ?? 4;
    const P = pineTone(palette(options.season), options.tone ?? 'green');

    // The trunk, flaring at its foot, with a few roots going into the ground.
    const wood = new Shape();
    const bark = color(P.bark);
    const barkDark = color(P.barkDark);
    const sides = options.simple ? 5 : 7;
    const shades = Array.from({ length: sides }, (_, i) => vary(i % 2 ? bark : bark.clone().lerp(barkDark, 0.35), random, 1));
    const lean = new THREE.Vector3(between(random, -0.02, 0.02), 1, between(random, -0.02, 0.02)).normalize();
    const top = lean.clone().multiplyScalar(height * 0.45);
    const r = height * 0.05;
    addTube(wood, [new THREE.Vector3(0, -0.2, 0), new THREE.Vector3(0, 0.02, 0), lean.clone().multiplyScalar(height * 0.12), top], [r * 1.9, r * 1.5, r, r * 0.6], {
      sides,
      rough: 0.06,
      random,
      paint: (_, i) => shades[i],
    });
    const roots = options.simple ? 0 : 3 + Math.floor(random() * 2);
    for (let k = 0; k < roots; k++) {
      const angle = ((k + random() * 0.5) / roots) * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      addTube(
        wood,
        [out.clone().multiplyScalar(r * 0.5).setY(r * 0.9), out.clone().multiplyScalar(r * 1.9).setY(0.02), out.clone().multiplyScalar(r * 2.9).setY(-0.12)],
        [r * 0.5, r * 0.32, r * 0.12],
        { sides: 5, random, paint: (_, i) => shades[i % sides] },
      );
    }

    // The tiers, from the widest at the foot of the crown to the spire:
    // big, overlapping, their points drooping.
    const needles = new Shape();
    for (let k = 0; k < tiers; k++) {
      const t = tiers > 1 ? k / (tiers - 1) : 0;
      const radius = height * (0.33 - 0.21 * t) * between(random, 0.94, 1.06);
      const base = lean.clone().multiplyScalar(height * (0.17 + 0.57 * t));
      const tall = height * (0.37 - 0.1 * t);
      addTier(needles, base, radius, tall, options.simple ? 6 : k === tiers - 1 ? 7 : 8, radius * 0.2, random, !options.simple, P);
    }

    this.trunk = mesh(wood.geometry(), matte());
    this.crown = mesh(needles.geometry(), matte());
    this.add(this.trunk, this.crown);
    if (options.season === 'winter') snowOn(this, random, 0.28, 0.6);
  }
}

// The season's pine colours in a tone.
function pineTone(P: Palette, tone: PineTone): Palette {
  if (tone === 'green') return P;
  if (tone === 'gold') return { ...P, pineLight: 0xffc64d, pine: 0xe38f28, pineDark: 0x9c5418 };
  const dark = tone === 'dark';
  const shift = (hex: number) =>
    color(hex)
      .offsetHSL(dark ? 0.02 : -0.018, dark ? -0.04 : 0.05, dark ? -0.035 : 0.035)
      .getHex();
  return { ...P, pineLight: shift(P.pineLight), pine: shift(P.pine), pineDark: shift(P.pineDark) };
}
