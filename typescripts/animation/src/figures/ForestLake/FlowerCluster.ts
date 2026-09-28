import * as THREE from 'three';
import { addBerry, addBlade, addBlob, addFlower, addTube, between, color, matte, mesh, palette, seededRandom, Shape, vary, type Palette, type Season } from './parts';

// Which flowers grow in a cluster: the asset sheet's two, pink (pinks,
// white and lilac) and mixed (blue, orange, pink and yellow), as the
// palette's petal colours, which each season changes (summer's whiter and
// yellower, autumn's orange, red and gold).
export const FLOWER_KINDS = {
  pink: ['petalPink', 'petalLight', 'petalWhite', 'petalLilac', 'petalPink'],
  mixed: ['petalBlue', 'petalOrange', 'petalOrange', 'petalPink', 'petalLight', 'petalYellow'],
} as const satisfies Record<string, readonly (keyof Palette)[]>;
export type FlowerKind = keyof typeof FLOWER_KINDS;

export interface FlowerClusterOptions {
  kind?: FlowerKind; // pink by default
  flowers?: number; // 6 by default
  height?: number; // m, the tallest stems; 0.42 by default
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's flower cluster (FlowerCluster.md), as its asset sheet
// draws it: a few flowers on slender stems of different heights, each head a
// ring of broad petals round a raised yellow middle, among long leaves. As
// the reference's seasons show them: white and yellow in summer, orange, red
// and gold in autumn, and in winter bare sprigs of red berries standing out
// of a drift of snow.
export class FlowerCluster extends THREE.Group {
  readonly stems: THREE.Mesh; // with their leaves
  readonly heads: THREE.Mesh; // or berries

  constructor(options: FlowerClusterOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 71);
    const P = palette(options.season);
    const winter = options.season === 'winter';
    const colors = FLOWER_KINDS[options.kind ?? 'pink'].map((key) => P[key]);
    const count = options.flowers ?? 6;
    const height = options.height ?? 0.42;

    const green = new Shape();
    const heads = new Shape();
    const stem = color(winter ? P.barkDark : P.grass);
    const dark = color(P.grassFoot);
    const tip = color(P.grassTip);
    for (let k = 0; k < count; k++) {
      const angle = ((k + random() * 0.8) / count) * Math.PI * 2;
      const d = between(random, 0.02, 0.16);
      const foot = new THREE.Vector3(Math.cos(angle) * d, -0.01, Math.sin(angle) * d);
      const tall = height * between(random, 0.45, 1);
      const lean = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle)).multiplyScalar(tall * between(random, 0.08, 0.25));
      const head = foot.clone().add(new THREE.Vector3(0, tall, 0)).add(lean);
      const bend = foot.clone().lerp(head, 0.55).addScaledVector(lean, -0.35);
      addTube(green, [foot, bend, head], [0.0075, 0.0065, 0.0055], { sides: 3, random, paint: () => vary(stem, random, 1) });
      const petal = vary(color(colors[Math.floor(random() * colors.length)]), random, 0.8);
      if (winter) {
        // A bunch of berries at the sprig's tip, and one or two on a twig.
        for (let j = 0; j < 5; j++) addBerry(heads, head.clone().add(new THREE.Vector3(between(random, -0.025, 0.025), between(random, -0.02, 0.015), between(random, -0.025, 0.025))), 0.017, petal);
        const twig = foot.clone().lerp(head, 0.62);
        const twigTip = twig.clone().add(new THREE.Vector3(lean.z * 0.6 + 0.02, 0.05, -lean.x * 0.6));
        addTube(green, [twig, twigTip], [0.004, 0.003], { sides: 3, random, paint: () => vary(stem, random, 1) });
        addBerry(heads, twigTip, 0.015, petal);
        continue;
      }
      const facing = lean.clone().normalize().multiplyScalar(0.45).add(new THREE.Vector3(0, 1, 0));
      addFlower(heads, head, facing, 5 + Math.floor(random() * 4), between(random, 0.06, 0.078), petal, color(P.flowerHeart), random, 0.18);
      // Two or three long leaves at its foot.
      const leaves = 2 + Math.floor(random() * 2);
      for (let j = 0; j < leaves; j++) {
        const a = random() * Math.PI * 2;
        const out = new THREE.Vector3(Math.cos(a), 0, Math.sin(a));
        addBlade(green, foot, out, new THREE.Vector3(-out.z, 0, out.x), between(random, 0.12, 0.22), between(random, 0.5, 0.9), 0.04, vary(dark, random), vary(color(P.grass), random), vary(tip, random));
      }
    }
    // In winter, a drift of snow round their feet.
    if (winter) {
      const snow = color(P.snow);
      const blue = color(P.snowShade);
      addBlob(green, new THREE.Vector3(0, -0.02, 0), new THREE.Vector3(0.24, 0.07, 0.22), 1, 0.2, random, (n) => vary(n.y > 0.3 ? snow : blue, random, 0.5));
    }

    this.stems = mesh(green.geometry(), matte());
    this.heads = mesh(heads.geometry(), matte());
    this.add(this.stems, this.heads);
  }
}
