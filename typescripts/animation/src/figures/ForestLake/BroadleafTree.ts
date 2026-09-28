import * as THREE from 'three';
import { addBlob, addTube, between, color, matte, mesh, palette, seededRandom, Shape, shade, snowOn, vary, type Palette, type Season } from './parts';

// An autumn crown's colour, as the reference's autumn mixes them.
export type AutumnTone = 'orange' | 'red' | 'yellow';

export interface BroadleafTreeOptions {
  height?: number; // m, to the top of the crown; 6 by default
  season?: Season; // spring by default
  tone?: AutumnTone; // its colour in autumn; orange by default
  // Fewer facets, for trees seen from afar (most of the forest lake's
  // forest): its clumps plainer, none standing out of them, fewer twigs in
  // winter. False by default.
  simple?: boolean;
  seed?: number;
}

// The forest lake's broadleaf tree (BroadleafTree.md; the asset sheets'
// "Tree (Broadleaf)"): a straight orange-brown trunk flaring into its roots,
// forking into branches that rise into a big round crown of leafy clumps,
// light where the sun catches them and dark green in its shade underneath.
// As the reference's seasons show it: fresh light green in spring, deep
// green in summer, orange, red or yellow in autumn, and in winter bare,
// its twigs holding clumps of snow and snow lying along its branches.
export class BroadleafTree extends THREE.Group {
  readonly trunk: THREE.Mesh; // with its branches
  readonly crown: THREE.Mesh;

  constructor(options: BroadleafTreeOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 231);
    const height = options.height ?? 6;
    const s = height / 6;
    const winter = options.season === 'winter';
    const P = leafTone(palette(options.season), options.season === 'autumn' ? (options.tone ?? 'orange') : null);

    // The trunk, nearly straight, with its roots.
    const wood = new Shape();
    const bark = color(P.bark);
    const barkDark = color(P.barkDark);
    const sides = options.simple ? 5 : 7;
    const shades = Array.from({ length: sides }, (_, i) => vary(i % 2 ? bark : bark.clone().lerp(barkDark, 0.3), random, 1.2));
    const paint = (_: number, i: number) => shades[i % sides];
    const lean = new THREE.Vector3(between(random, -0.05, 0.05), 1, between(random, -0.05, 0.05)).normalize();
    const fork = lean.clone().multiplyScalar(2.35 * s);
    const r = 0.19 * s;
    addTube(wood, [new THREE.Vector3(0, -0.2, 0), new THREE.Vector3(0, 0.05, 0), lean.clone().multiplyScalar(1.1 * s), fork], [r * 1.65, r * 1.35, r * 1.02, r * 0.84], { sides, rough: 0.06, random, paint });
    for (let k = 0; k < (options.simple ? 2 : 4); k++) {
      const angle = ((k + random() * 0.6) / 4) * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      addTube(
        wood,
        [out.clone().multiplyScalar(r * 0.5).setY(r * 1.1), out.clone().multiplyScalar(r * 1.6).setY(0.03), out.clone().multiplyScalar(r * 2.5).setY(-0.1)],
        [r * 0.52, r * 0.34, r * 0.13],
        { sides: 5, random, paint },
      );
    }

    // The branches, rising from the fork up and out into the crown, each
    // ending in a clump of leaves; the crown's heart fills the middle. In
    // winter each branch ends in a few twigs instead, holding snow.
    const heart = fork.clone().add(new THREE.Vector3(0, 1.75 * s, 0));
    const clumps: { at: THREE.Vector3; size: number }[] = [{ at: heart, size: 1.35 * s }];
    const branches = 5 + Math.floor(random() * 2);
    for (let k = 0; k < branches; k++) {
      const angle = ((k + between(random, -0.25, 0.25)) / branches) * Math.PI * 2;
      const out = new THREE.Vector3(Math.cos(angle), 0, Math.sin(angle));
      const rise = between(random, 1.2, 2.0) * s;
      const reach = between(random, 1.0, 1.35) * s;
      const mid = fork.clone().addScaledVector(out, reach * 0.4).add(new THREE.Vector3(0, rise * 0.55, 0));
      const end = fork.clone().addScaledVector(out, reach).add(new THREE.Vector3(0, rise, 0));
      addTube(wood, [fork.clone().addScaledVector(out, r * 0.2), mid, end], [r * 0.6, r * 0.4, r * 0.2], { sides: 6, random, paint });
      clumps.push({ at: end.clone().addScaledVector(out, 0.25 * s).add(new THREE.Vector3(0, 0.2 * s, 0)), size: between(random, 0.9, 1.15) * s });
      if (winter) {
        for (let j = 0; j < (options.simple ? 1 : 3); j++) {
          const twig = new THREE.Vector3(between(random, -0.5, 0.5), between(random, 0.5, 1), between(random, -0.5, 0.5)).add(out.clone().multiplyScalar(0.6)).normalize();
          const tip = end.clone().addScaledVector(twig, between(random, 0.5, 0.85) * s);
          addTube(wood, [end, end.clone().lerp(tip, 0.5), tip], [r * 0.18, r * 0.12, r * 0.05], { sides: 4, random, paint });
          clumps.push({ at: tip, size: between(random, 0.28, 0.4) * s });
        }
      }
    }
    clumps.push({ at: heart.clone().add(new THREE.Vector3(between(random, -0.25, 0.25) * s, 1.15 * s, between(random, -0.25, 0.25) * s)), size: 0.95 * s });

    // The leaves: light where they face the sky, a warm light here and
    // there, dark underneath, each face its own shade, each clump with
    // smaller clumps standing out of it so the crown is lumpy.
    const leafy = new Shape();
    const light = color(P.leafLight);
    const mid = color(P.leaf);
    const dark = color(P.leafDark);
    const snow = color(P.snow);
    const snowShade = color(P.snowShade);
    const leaves = (n: THREE.Vector3) => {
      if (winter) return vary(n.y > -0.1 ? snowShade.clone().lerp(snow, Math.min(1, (n.y + 0.1) * 1.6)) : snowShade, random, 0.6);
      if (random() < 0.08) return vary(shade(light, 1.06), random, 0.8);
      const base = n.y > 0 ? mid.clone().lerp(light, Math.min(1, n.y * 1.15)) : mid.clone().lerp(dark, Math.min(1, -n.y * 1.3 + 0.15));
      return vary(base, random, 1.8);
    };
    for (const [k, { at, size }] of clumps.entries()) {
      if (winter && k === 0) continue; // the heart is bare in winter
      const radii = new THREE.Vector3(size, size * between(random, 0.78, 0.9), size).multiplyScalar(winter ? 0.55 : 0.86);
      if (winter) radii.y *= 0.55;
      addBlob(leafy, at, radii, winter || (options.simple && k > 0) ? 0 : 1, 0.22, random, leaves);
      const chunks = winter || options.simple ? 0 : 2 + Math.floor(random() * 2);
      for (let j = 0; j < chunks; j++) {
        const dir = new THREE.Vector3(between(random, -1, 1), between(random, -0.35, 1), between(random, -1, 1)).normalize();
        const c = at.clone().add(dir.clone().multiply(radii).multiplyScalar(between(random, 0.82, 0.98)));
        const chunk = size * between(random, 0.26, 0.4);
        addBlob(leafy, c, new THREE.Vector3(chunk, chunk * 0.85, chunk), 0, 0.2, random, leaves);
      }
    }

    this.trunk = mesh(wood.geometry(), matte());
    this.crown = mesh(leafy.geometry(), matte());
    this.add(this.trunk, this.crown);
    if (winter) snowOn(this.trunk, random, 0.35, 0.7);
  }
}

// The season's leaf colours, in an autumn tone when one is given.
function leafTone(P: Palette, tone: AutumnTone | null): Palette {
  if (tone === 'red') return { ...P, leafLight: 0xf4733f, leaf: 0xd73b2b, leafDark: 0x8e231d };
  if (tone === 'yellow') return { ...P, leafLight: 0xffe07c, leaf: 0xf2b42f, leafDark: 0xb0781d };
  return P;
}
