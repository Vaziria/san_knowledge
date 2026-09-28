import * as THREE from 'three';
import { mergeGeometries } from 'three/addons/utils/BufferGeometryUtils.js';
import { Barrel } from './Barrel';
import { Crate } from './Crate';
import { addIcicle, addLathe, addPole, addRock, addTube, between, color, Flame, glowing, matte, mesh, PALETTE, palette, prelight, rockPaint, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface CaveEntranceOptions {
  // A warm light at its mouth that flickers with the torches, lighting the
  // ground in front of it; true by default.
  light?: boolean;
  season?: Season; // spring by default; snow on its rocks and icicles over its mouth in winter
  seed?: number;
}

const HALF = 1.3; // m, half the mouth's width
const WALL = 1.35; // m, how high its walls rise before its roof curves over
const DEEP = 3.3; // m, how far in its back wall is

// The forest lake's cave entrance (CaveEntrance.md; from the reference's
// "Cave Entrance"): an arched mouth into a rocky hill, framed by big
// lavender-grey boulders, opening to the front (+z). Inside, the rock walls
// glow orange in the light of a torch on an iron stand, with a crate and a
// barrel against the walls; two torches burn on the rocks either side of
// the mouth, and a sandy floor runs out of it. The torches flicker
// (update()). In winter snow lies on its rocks and icicles hang over its
// mouth, as the reference's winter draws it; the floor outside is trodden
// snow.
export class CaveEntrance extends THREE.Group {
  readonly rocks: THREE.Mesh; // the hill, the boulders and the wall torches' sticks
  readonly hollow: THREE.Mesh; // the inside, its floor, stand and things, lit by the torch
  readonly floor: THREE.Mesh; // the sandy floor outside
  readonly flames: Flame[];
  readonly light: THREE.PointLight | null;
  private time = 0;

  constructor(options: CaveEntranceOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 221);
    const season = options.season ?? 'spring';
    const P = palette(season);

    // The tunnel's rings, from the mouth in: each a cross-section from its
    // right foot up, over its roof and down to its left foot, roughened.
    const stations: [z: number, scale: number, rough: number][] = [
      [0.7, 1.1, 0.03],
      [0.2, 1.0, 0.08],
      [-0.6, 0.97, 0.1],
      [-1.4, 0.93, 0.1],
      [-2.2, 0.87, 0.1],
      [-2.9, 0.78, 0.08],
      [-DEEP, 0.64, 0.05],
    ];
    const section = (z: number, scale: number, rough: number, grow = 0) => {
      const hw = HALF * scale + grow;
      const wall = WALL * scale + grow * 0.5;
      const points: [number, number][] = [
        [hw, 0],
        [hw, wall * 0.5],
        [hw, wall],
      ];
      for (let k = 1; k < 7; k++) {
        const a = (Math.PI * k) / 7;
        points.push([Math.cos(a) * hw, wall + Math.sin(a) * (HALF * scale + grow)]);
      }
      points.push([-hw, wall], [-hw, wall * 0.5], [-hw, 0]);
      return points.map(([x, y], i) => {
        const foot = i === 0 || i === points.length - 1;
        return new THREE.Vector3(x + (random() - 0.5) * 2 * rough, foot ? 0 : y + (random() - 0.5) * 2 * rough, z + (random() - 0.5) * rough);
      });
    };
    const inner = stations.map(([z, scale, rough]) => section(z, scale, rough));

    // The inside: rock walls facing in, the back wall, the floor, the torch
    // stand, a crate and a barrel, all lit by the torch alone.
    const hollow = new Shape();
    const paintRock = rockPaint(random, 2, P);
    const count = inner[0].length;
    // Dark rock of one tone throughout, which the torch's light baked in below
    // brightens where it faces the torch.
    const level = new THREE.Vector3(0, 0.3, 0);
    for (let r = 0; r + 1 < inner.length; r++) {
      const [p, q] = [inner[r], inner[r + 1]];
      for (let i = 0; i + 1 < count; i++) hollow.quad(p[i], p[i + 1], q[i + 1], q[i], shade(paintRock(level), 0.62));
    }
    const back = inner[inner.length - 1];
    const backMiddle = new THREE.Vector3(0, WALL * 0.55, -DEEP - 0.2);
    for (let i = 0; i + 1 < count; i++) hollow.triangle(backMiddle, back[i], back[i + 1], shade(vary(color(PALETTE.rock), random, 1.5), 0.62));
    const sand = color(PALETTE.path);
    const outside = color(P.path);
    for (let r = 0; r + 1 < inner.length; r++) {
      const [p, q] = [inner[r], inner[r + 1]];
      const [pr, pl, qr, ql] = [p[0], p[count - 1], q[0], q[count - 1]];
      hollow.quad(pl.clone().setY(0.01), pr.clone().setY(0.01), qr.clone().setY(0.01), ql.clone().setY(0.01), vary(sand, random, 1));
    }
    const stand = new THREE.Vector3(0.3, 0, -2.05);
    const iron = PALETTE.iron;
    addPole(hollow, stand, stand.clone().add(new THREE.Vector3(0, 1.0, 0)), 0.028, random, 0.8, iron, 5);
    for (let k = 0; k < 3; k++) {
      const a = (2 * Math.PI * k) / 3 + 0.4;
      addTube(hollow, [stand.clone().add(new THREE.Vector3(0, 0.3, 0)), stand.clone().add(new THREE.Vector3(Math.cos(a) * 0.28, 0, Math.sin(a) * 0.28))], [0.02, 0.016], { sides: 4, random, paint: () => color(iron) });
    }
    addLathe(
      hollow,
      [
        [0.02, 0.97],
        [0.12, 1.03],
        [0.15, 1.1],
        [0.12, 1.1],
        [0.02, 1.05],
      ],
      8,
      new THREE.Matrix4().makeTranslation(stand.x, stand.y, stand.z),
      () => color(iron),
    );
    const inside = hollow.geometry();
    const props = [new Crate({ size: 0.55, seed: 3 }), new Barrel({ height: 0.72, seed: 4 })];
    props[0].position.set(-0.78, 0, -1.35);
    props[0].rotation.y = 0.25;
    props[1].position.set(0.88, 0, -1.1);
    const geometries = [inside];
    for (const prop of props) {
      prop.updateMatrixWorld(true);
      prop.traverse((object) => {
        if (object instanceof THREE.Mesh) geometries.push((object.geometry as THREE.BufferGeometry).clone().applyMatrix4(object.matrixWorld));
      });
    }
    const merged = mergeGeometries(geometries) ?? inside;
    const torch = stand.clone().add(new THREE.Vector3(0, 1.3, 0));
    prelight(merged, torch, new THREE.Color(2.7, 1.35, 0.5), 1.7, new THREE.Color(0.14, 0.09, 0.07));

    // The hill round it: the same rings, grown outward, facing out, closed
    // at the back, with a lip round the mouth; boulders framing the mouth
    // and heaped over it.
    const rocks = new Shape();
    const outer = stations.map(([z, scale, rough]) => section(z, scale, rough * 2.5, 0.5));
    for (let r = 0; r + 1 < outer.length; r++) {
      const [p, q] = [outer[r], outer[r + 1]];
      for (let i = 0; i + 1 < count; i++) {
        const n = new THREE.Vector3().crossVectors(q[i + 1].clone().sub(p[i]), p[i + 1].clone().sub(p[i])).normalize();
        rocks.quad(p[i], q[i], q[i + 1], p[i + 1], paintRock(n));
      }
    }
    const outerBack = outer[outer.length - 1];
    const outerMiddle = new THREE.Vector3(0, WALL * 0.6, -DEEP - 0.9);
    for (let i = 0; i + 1 < count; i++) rocks.triangle(outerMiddle, outerBack[i + 1], outerBack[i], paintRock(new THREE.Vector3(0, 0.2, -1)));
    for (let i = 0; i + 1 < count; i++) {
      const [a, b, c, d] = [inner[0][i], inner[0][i + 1], outer[0][i + 1], outer[0][i]];
      rocks.quad(a, d, c, b, paintRock(new THREE.Vector3(0, 0.3, 1).normalize()));
    }
    const boulders: [x: number, y: number, z: number, sx: number, sy: number, sz: number][] = [
      [-2.45, 0, 0.45, 1.7, 2.7, 1.9],
      [2.5, 0, 0.4, 1.8, 2.5, 1.9],
      [-0.35, 2.98, 0.35, 2.5, 1.35, 1.6],
      [0.75, 2.95, -1.4, 3.2, 1.8, 2.8],
      [-2.75, 0, -2.2, 2.1, 3.0, 2.5],
      [2.8, 0, -2.0, 2.1, 2.8, 2.5],
      [-1.75, 0, 1.45, 0.7, 0.55, 0.6],
      [1.8, 0, 1.55, 0.55, 0.45, 0.5],
      [2.35, 0, 1.2, 0.35, 0.3, 0.35],
    ];
    for (const [x, y, z, sx, sy, sz] of boulders) addRock(rocks, new THREE.Vector3(x, y, z), new THREE.Vector3(sx, sy, sz), random() * Math.PI, random, 0.8, 18, P);
    // The wall torches' sticks, leaning out from the rocks by the mouth.
    const flames: Flame[] = [];
    for (const side of [-1, 1]) {
      const foot = new THREE.Vector3(side * 1.62, 1.25, 0.95);
      const top = new THREE.Vector3(side * 1.75, 1.78, 1.1);
      addTube(rocks, [foot, top], [0.035, 0.03], { sides: 5, random, paint: (_, i) => vary(color(PALETTE.bark), random, i % 2 ? 1 : 0.5), caps: ['open', shade(color(PALETTE.barkDark), 0.6)] });
      const flame = new Flame(0.26, random);
      flame.position.copy(top);
      flames.push(flame);
    }
    const inner0 = new Flame(0.36, random);
    inner0.position.copy(stand).add(new THREE.Vector3(0, 1.08, 0));
    flames.push(inner0);

    // The sandy floor running out of the mouth, widening.
    const ground = new Shape();
    const mouthRight = inner[0][0];
    const mouthLeft = inner[0][count - 1];
    const rows = 4;
    for (let k = 0; k < rows; k++) {
      const t0 = k / rows;
      const t1 = (k + 1) / rows;
      const at = (t: number, side: number) =>
        new THREE.Vector3((side > 0 ? mouthRight.x : mouthLeft.x) * (1 + t * 0.6) + side * between(random, -0.04, 0.04), 0.01, 0.7 + t * 2.1);
      ground.quad(at(t0, -1), at(t1, -1), at(t1, 1), at(t0, 1), vary(outside, random, 1));
    }

    this.rocks = mesh(rocks.geometry(), matte());
    this.hollow = mesh(merged, glowing(), false);
    this.floor = mesh(ground.geometry(), matte());
    this.flames = flames;
    this.add(this.rocks, this.hollow, this.floor, ...flames);
    if (season === 'winter') {
      snowOn(this.rocks, random, 0.3, 0.65);
      // Icicles hanging from the arch over its mouth.
      const ice = new Shape();
      for (let i = 3; i <= 8; i++) {
        for (const shift of [-0.12, 0.12]) {
          if (random() < 0.25) continue;
          const at = inner[0][i].clone().lerp(inner[0][i + (i < 8 ? 1 : -1)], Math.abs(shift) * 2).add(new THREE.Vector3(0, -0.02, 0.12));
          addIcicle(ice, at, between(random, 0.18, 0.45), 0.035, random);
        }
      }
      this.add(mesh(ice.geometry(), matte(), false));
    }
    this.light = null;
    if (options.light ?? true) {
      this.light = new THREE.PointLight(0xff8c3c, 2.5, 7, 2);
      this.light.position.set(0, 1.4, 0.6);
      this.add(this.light);
    }
  }

  update(delta: number): void {
    this.time += delta;
    for (const flame of this.flames) flame.update(delta);
    if (this.light) this.light.intensity = 2.5 * (1 + 0.1 * Math.sin(this.time * 11) + 0.06 * Math.sin(this.time * 27 + 1));
  }
}
