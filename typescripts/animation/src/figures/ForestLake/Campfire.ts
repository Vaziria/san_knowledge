import * as THREE from 'three';
import { addBlob, addLathe, addPole, addRock, addTube, between, color, Flame, glowing, matte, mesh, PALETTE, seededRandom, Shape, shade, snowOn, vary, type Season } from './parts';

export interface CampfireOptions {
  // A warm light that flickers with the fire, lighting what is round it;
  // true by default.
  light?: boolean;
  // A tripod of poles over it with an iron pot hanging from it, as the
  // reference's camp has; false by default.
  pot?: boolean;
  season?: Season; // spring by default; snow on its stones in winter
  seed?: number;
}

const SPARKS = 26;

// The forest lake's campfire (Campfire.md; from the reference's camp area):
// a ring of grey stones round a bed of ash, split logs leaning together
// into a cone, charred at their tops, glowing embers between them, and a
// flame rising out of them that flickers, throws up sparks and lights what
// is round it (update()). Over the camp's fire a pot hangs from a tripod
// of poles. In winter snow lies on its stones, melted where the fire is.
export class Campfire extends THREE.Group {
  readonly stones: THREE.Mesh;
  readonly logs: THREE.Mesh; // with the ash bed
  readonly embers: THREE.Mesh;
  readonly flame: Flame;
  readonly sparks: THREE.Points;
  readonly light: THREE.PointLight | null;
  readonly tripod: THREE.Mesh | null = null; // and its pot
  private readonly spark: { age: number; life: number; speed: number; turn: number; out: number }[];
  private readonly random: () => number;
  private time = 0;

  constructor(options: CampfireOptions = {}) {
    super();
    const random = (this.random = seededRandom(options.seed ?? 191));

    // The ring of stones.
    const stones = new Shape();
    const ring = 10;
    for (let k = 0; k < ring; k++) {
      const angle = ((k + between(random, -0.15, 0.15)) / ring) * Math.PI * 2;
      const size = between(random, 0.2, 0.27);
      addRock(stones, new THREE.Vector3(Math.cos(angle) * 0.5, 0, Math.sin(angle) * 0.5), new THREE.Vector3(size, size * 0.75, size * 0.85), angle, random, 2, 10);
    }

    // The ash bed and the logs leaning together over it, charred where the
    // fire reaches them.
    const wood = new Shape();
    const ash = color(0x4d4038);
    const bed = Array.from({ length: 10 }, (_, i) => {
      const angle = -(2 * Math.PI * i) / 10;
      const r = between(random, 0.36, 0.42);
      return new THREE.Vector3(Math.cos(angle) * r, 0.012, Math.sin(angle) * r);
    });
    const middle = new THREE.Vector3(0, 0.03, 0);
    for (let i = 0; i < 10; i++) wood.triangle(middle, bed[i], bed[(i + 1) % 10], shade(ash, 0.7), vary(ash, random), vary(ash, random));
    const bark = color(PALETTE.bark);
    const char = color(0x2e2420);
    const logs = 5;
    for (let k = 0; k < logs; k++) {
      const angle = ((k + between(random, -0.1, 0.1)) / logs) * Math.PI * 2;
      const foot = new THREE.Vector3(Math.cos(angle) * 0.36, 0.03, Math.sin(angle) * 0.36);
      const head = new THREE.Vector3(Math.cos(angle + 0.3) * 0.05, between(random, 0.4, 0.48), Math.sin(angle + 0.3) * 0.05);
      const path = [foot, foot.clone().lerp(head, 0.5), head];
      addTube(wood, path, [0.052, 0.048, 0.04], {
        sides: 6,
        random,
        paint: (r, i) => (r === 1 ? vary(char, random) : vary(i % 2 ? bark : shade(bark, 0.85), random)),
        caps: ['rings', char],
      });
    }

    // Embers glowing between the logs.
    const embers = new Shape();
    const hot = color(PALETTE.flameEdge);
    const glow = color(PALETTE.flame);
    for (let k = 0; k < 9; k++) {
      const angle = random() * Math.PI * 2;
      const d = between(random, 0, 0.22);
      const size = between(random, 0.035, 0.06);
      addBlob(embers, new THREE.Vector3(Math.cos(angle) * d, 0.03, Math.sin(angle) * d), new THREE.Vector3(size, size * 0.6, size), 0, 0.2, random, () => (random() < 0.5 ? hot.clone() : glow.clone()));
    }

    this.stones = mesh(stones.geometry(), matte());
    this.logs = mesh(wood.geometry(), matte());
    this.embers = mesh(embers.geometry(), glowing(), false);
    this.flame = new Flame(0.75, random, 5);
    this.flame.position.y = 0.03;
    this.add(this.stones, this.logs, this.embers, this.flame);
    if (options.season === 'winter') snowOn(this.stones, random, 0.45, 0.8);

    // The tripod: three poles leaning together over the fire, lashed at
    // the top, a chain down from there and a black iron pot on it.
    if (options.pot) {
      const tripod = new Shape();
      const apex = new THREE.Vector3(0, 1.32, 0);
      for (let k = 0; k < 3; k++) {
        const a = (k / 3) * Math.PI * 2 + 0.4;
        const foot = new THREE.Vector3(Math.cos(a) * 0.78, -0.05, Math.sin(a) * 0.78);
        const past = apex.clone().sub(foot).normalize().multiplyScalar(0.16).add(apex);
        addPole(tripod, foot, past, 0.025, random, 0.85, PALETTE.post, 5);
      }
      const iron = color(0x2d2a2a);
      const potTop = 0.62;
      addTube(tripod, [apex.clone().add(new THREE.Vector3(0, -0.02, 0)), new THREE.Vector3(0, potTop + 0.2, 0)], [0.008, 0.008], { sides: 4, random, paint: () => color(PALETTE.iron) });
      // The pot's handle, and the pot: round-bellied, a rim at its top.
      const handle = Array.from({ length: 7 }, (_, i) => {
        const a = Math.PI * (i / 6);
        return new THREE.Vector3(Math.cos(a) * 0.15, potTop + 0.02 + Math.sin(a) * 0.18, 0);
      });
      addTube(tripod, handle, handle.map(() => 0.006), { sides: 4, random, paint: () => iron });
      addLathe(
        tripod,
        [
          [0, potTop - 0.21],
          [0.1, potTop - 0.2],
          [0.16, potTop - 0.12],
          [0.155, potTop - 0.02],
          [0.17, potTop],
          [0.14, potTop + 0.005],
          [0, potTop - 0.03],
        ],
        9,
        new THREE.Matrix4(),
        (r, i) => shade(iron, r === 4 ? 1.35 : i % 2 ? 1 : 0.85),
      );
      this.tripod = mesh(tripod.geometry(), matte());
      this.add(this.tripod);
      if (options.season === 'winter') snowOn(this.tripod, random, 0.5, 0.85);
    }

    // Sparks, rising from the fire and dying away.
    this.spark = Array.from({ length: SPARKS }, () => ({ age: random() * 2, life: between(random, 1.2, 2.2), speed: between(random, 0.5, 0.9), turn: random() * Math.PI * 2, out: between(random, 0.05, 0.25) }));
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(SPARKS * 3), 3));
    geometry.setAttribute('color', new THREE.Float32BufferAttribute(new Float32Array(SPARKS * 3), 3));
    const material = new THREE.PointsMaterial({ size: 0.045, vertexColors: true, blending: THREE.AdditiveBlending, depthWrite: false, transparent: true });
    this.sparks = new THREE.Points(geometry, material);
    this.sparks.frustumCulled = false;
    this.add(this.sparks);

    this.light = null;
    if (options.light ?? true) {
      this.light = new THREE.PointLight(0xff9a45, 2.2, 8, 2);
      this.light.position.set(0, 1.1, 0);
      this.add(this.light);
    }
    this.update(0);
  }

  update(delta: number): void {
    this.time += delta;
    this.flame.update(delta);
    const t = this.time;
    const position = this.sparks.geometry.getAttribute('position') as THREE.BufferAttribute;
    const tint = this.sparks.geometry.getAttribute('color') as THREE.BufferAttribute;
    const hot = new THREE.Color(PALETTE.flame);
    this.spark.forEach((s, i) => {
      s.age += delta;
      if (s.age > s.life) {
        s.age = 0;
        s.life = between(this.random, 1.2, 2.2);
        s.speed = between(this.random, 0.5, 0.9);
        s.turn = this.random() * Math.PI * 2;
        s.out = between(this.random, 0.05, 0.25);
      }
      const a = s.age;
      const angle = s.turn + a * 2.2;
      const spread = s.out * Math.min(1, a * 0.8);
      position.setXYZ(i, Math.cos(angle) * spread, 0.35 + a * s.speed, Math.sin(angle) * spread);
      const fade = Math.max(0, 1 - a / s.life);
      tint.setXYZ(i, hot.r * fade * 1.4, hot.g * fade * 1.2, hot.b * fade);
    });
    position.needsUpdate = true;
    tint.needsUpdate = true;
    if (this.light) this.light.intensity = 2.2 * (1 + 0.12 * Math.sin(t * 11) + 0.08 * Math.sin(t * 23 + 1.3) + 0.05 * Math.sin(t * 37));
  }
}
