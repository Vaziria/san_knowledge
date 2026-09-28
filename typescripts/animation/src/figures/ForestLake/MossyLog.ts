import * as THREE from 'three';
import { addFallenLeaves, addMoss, addMushroom, addTube, addTuft, between, color, matte, mesh, palette, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface MossyLogOptions {
  length?: number; // m; 1.7 by default
  radius?: number; // m; 0.21 by default
  // A tuft of grass beside it and moss at its foot, as the asset sheet has;
  // true by default.
  dressing?: boolean;
  // A few red-capped mushrooms growing on it, as the reference's log has;
  // true by default.
  mushrooms?: boolean;
  // Moss along its top; true by default (a log sat on by the camp's fire
  // has none).
  moss?: boolean;
  season?: Season; // spring by default
  seed?: number;
}

// The forest lake's log (MossyLog.md; the sheet's "Log"), as its asset sheet
// draws it: a fallen trunk lying along x, its orange-brown bark split into
// long dark cracks, moss growing along its top, a stub where a branch was,
// a few red mushrooms on its side, and both ends sawn, showing pale growth
// rings round a darker heart. As the reference's seasons show it: its moss
// olive and leaves fallen on and round it in autumn, snow along its top in
// winter.
export class MossyLog extends THREE.Group {
  readonly wood: THREE.Mesh; // with its mushrooms
  readonly dressing: THREE.Mesh;

  constructor(options: MossyLogOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 91);
    const length = options.length ?? 1.7;
    const radius = options.radius ?? 0.21;
    const season = options.season ?? 'spring';
    const P = palette(season);

    const wood = new Shape();
    const bark = color(P.bark);
    const crack = color(P.barkDark);
    const moss = color(P.moss);
    const sides = 10;
    const segments = 8;
    // Side i faces (i + 0.5) / sides of the way round from straight up (the
    // tube's first normal is +y). Moss grows in a strip along its top over
    // most of its length, ragged at its edges, and the bark is cracked here
    // and there.
    const from = 1 + Math.floor(random() * 2);
    const to = segments - 1 - Math.floor(random() * 2);
    const faces = Array.from({ length: segments }, (_, r) =>
      Array.from({ length: sides }, (_, i) => {
        const up = Math.cos(((i + 0.5) / sides) * Math.PI * 2);
        const along = r >= from && r < to;
        if ((options.moss ?? true) && along && (up > 0.55 || (up > 0.2 && random() < 0.5))) return vary(moss, random, 2);
        if (random() < 0.24) return vary(crack, random, 1);
        return vary(i % 2 ? bark : shade(bark, 0.88), random, 1.4);
      }),
    );
    const path = Array.from({ length: segments + 1 }, (_, k) => {
      const t = k / segments;
      return new THREE.Vector3(-length / 2 + length * t, radius * 0.92 + Math.sin(t * Math.PI) * radius * 0.08, Math.sin(t * Math.PI * 1.3) * 0.04);
    });
    const radii = path.map((_, k) => radius * (1.04 - 0.1 * (k / segments)) * between(random, 0.96, 1.04));
    addTube(wood, path, radii, { sides, rough: 0.05, random, paint: (r, i) => faces[r][i], caps: ['rings', 'rings'] });
    // A stub where a branch was cut off.
    const at = path[Math.floor(segments * 0.62)].clone();
    const out = new THREE.Vector3(0.25, 0.8, 0.55).normalize();
    addTube(wood, [at.clone().addScaledVector(out, radius * 0.6), at.clone().addScaledVector(out, radius * 1.45)], [radius * 0.3, radius * 0.24], {
      sides: 6,
      random,
      paint: () => vary(bark, random, 1),
      caps: ['open', 'grain'],
    });
    // Red mushrooms growing out of its front side, bending up.
    if ((options.mushrooms ?? true) && season !== 'winter') {
      const count = 2 + Math.floor(random() * 2);
      for (let k = 0; k < count; k++) {
        const p = path[Math.floor(segments * between(random, 0.2, 0.5))];
        const a = between(random, 0.95, 1.35); // radians round from straight up, toward +z: its side
        const foot = p.clone().add(new THREE.Vector3(between(random, -0.08, 0.08), Math.cos(a) * radius * 0.95, Math.sin(a) * radius * 0.95));
        addMushroom(wood, foot, between(random, 0.05, 0.08) * (radius / 0.21), between(random, 0.07, 0.1) * (radius / 0.21), 'red', random, Math.PI / 2, a * 0.55);
      }
    }

    const green = new Shape();
    if (options.dressing ?? true) {
      addTuft(green, new THREE.Vector3(length * 0.2, 0, radius * 1.6), 9, 0.3, random, 0, 0, P);
      addMoss(green, new THREE.Vector3(-length * 0.25, 0, -radius * 1.2), 0.22, random, P);
      if (season === 'autumn') addFallenLeaves(green, new THREE.Vector3(0, 0, 0), length * 0.7, 14, random);
    }
    if (season === 'autumn') {
      for (let k = 0; k < 5; k++) {
        const p = path[1 + Math.floor(random() * (segments - 1))];
        addFallenLeaves(wood, p.clone().setY(p.y + radii[0] * 0.92), 0.05, 1, random);
      }
    }

    this.wood = mesh(wood.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.add(this.wood, this.dressing);
    if (season === 'winter') snowOn(this, random, 0.3, 0.65);
  }
}
