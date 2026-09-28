import * as THREE from 'three';
import { between, color, FALLEN_LEAVES, PALETTE, seededRandom, twoSided, type Season } from '../../figures/ForestLake/parts';
import { Rowboat } from '../../figures/ForestLake/Rowboat';
import { Backdrop, horizonOf } from './Backdrop';
import { Cliffs } from './Cliffs';
import { Landmarks } from './Landmarks';
import { CAVE_SCALE, CLEARING, LANDING, MAIN_LIP, SMALL_LIP, SPOTS } from './layout';
import { Scatter } from './Scatter';
import { Terrain, type River } from './Terrain';
import { Water, waveHeight, type Fall } from './Water';

export interface ForestLakeOptions {
  season?: Season; // spring by default
}

// The sky's colour and the lights a season brings, in place of the theme's.
export interface ForestLakeLight {
  background: number;
  sky: number;
  ground: number;
  light: number;
  ambientIntensity: number;
  lightIntensity: number;
}

// The forest_lake environment (ForestLake.md), built as the user's
// reference images ("Spring - Forest Lake" and the forest lake in all four
// seasons) show it, with its own look: its land and lake (Terrain.ts,
// Water.ts), cliffs (Cliffs.ts), sky and mountains (Backdrop.ts), what
// grows and lies on it (Scatter.ts) and its landmarks (Landmarks.ts), laid
// out by the map in layout.ts, in one of four seasons: spring's cherry
// blossom with petals falling round the trees by the landing, summer's deep
// green, autumn's orange and red with leaves falling, winter's snow, the
// lake frozen but for a hole of open water, and snow falling. It brings its
// own sky colour and lights (light), each season's, rather than the
// theme's.
export class ForestLake extends THREE.Group {
  // Where figures stand: the south shore, looking north across the lake.
  static readonly LANDING = new THREE.Vector3(LANDING[0], 0, LANDING[1]);
  // Where a figure in the water goes: open water north of the landing (in
  // winter a hole kept open in the ice).
  static readonly OPEN_WATER = new THREE.Vector3(-2.5, 0, 3.5);

  // The sky's colour and the lights in a season: a bright, warm sun, and a
  // sky light pale blue from above and from below the ground's colour,
  // softened so what is pink or orange keeps its colour in the shade (a
  // little green off the grass, gold off autumn's leaves, white off the
  // snow).
  static light(season: Season = 'spring'): ForestLakeLight {
    const background = horizonOf(season).getHex();
    switch (season) {
      case 'summer':
        return { background, sky: 0xe2f1ff, ground: 0xb2c794, light: 0xfff3da, ambientIntensity: 1.6, lightIntensity: 2.75 };
      case 'autumn':
        return { background, sky: 0xeef0ec, ground: 0xd6bf98, light: 0xffe5bf, ambientIntensity: 1.6, lightIntensity: 2.6 };
      case 'winter':
        return { background, sky: 0xe9f1fc, ground: 0xe1e9f2, light: 0xf5f8ff, ambientIntensity: 1.75, lightIntensity: 2.2 };
      default:
        return { background, sky: 0xe6f3ff, ground: 0xbccb9c, light: 0xfff1d8, ambientIntensity: 1.6, lightIntensity: 2.65 };
    }
  }
  // Spring's, for what shows the forest lake without a season.
  static readonly LIGHT = ForestLake.light('spring');

  readonly season: Season;
  readonly light: ForestLakeLight;
  readonly terrain: Terrain;
  readonly water: Water;
  readonly landmarks: Landmarks;
  readonly scatter: Scatter;
  readonly cliffs: Cliffs;
  readonly backdrop: Backdrop;
  readonly falling: Falling | Snowfall | null;
  readonly ground: THREE.Mesh;
  // The haze the land fades into with distance.
  readonly fog: THREE.Fog;

  constructor(options: ForestLakeOptions = {}) {
    super();
    const season = (this.season = options.season ?? 'spring');
    this.light = ForestLake.light(season);
    this.fog = new THREE.Fog(horizonOf(season), 70, 460);
    const terrain = (this.terrain = new Terrain(season));
    ForestLake.LANDING.y = terrain.heightAt(LANDING[0], LANDING[1]);
    const landmarks = (this.landmarks = new Landmarks(terrain, season));

    // The waterfalls, from each river's lip out the way it runs.
    const falls: Fall[] = terrain.rivers.map((river) => fallOf(river, terrain));
    const pools = falls.map((f) => ({ x: f.x + f.out.x * 1.8, z: f.z + f.out.y * 1.8, width: f.width }));
    const cliffs = (this.cliffs = new Cliffs(
      terrain,
      [
        { x: MAIN_LIP.x, z: MAIN_LIP.z, radius: MAIN_LIP.width / 2 + 0.9 },
        { x: SMALL_LIP.x, z: SMALL_LIP.z, radius: SMALL_LIP.width / 2 + 0.8 },
        { x: SPOTS.cave.x + 0.8, z: SPOTS.cave.z, radius: 4.4 * CAVE_SCALE, above: 4.3 * CAVE_SCALE },
      ],
      pools,
      season,
    ));

    const keep = [
      ...landmarks.keep,
      ...cliffs.boulders.map((b) => ({ x: b.x, z: b.z, radius: b.radius + 0.3 })),
      ...pools.map((p) => ({ x: p.x, z: p.z, radius: p.width + 1.5 })),
      { x: LANDING[0], z: LANDING[1], radius: CLEARING },
      // The open water a floating figure goes to.
      { x: ForestLake.OPEN_WATER.x, z: ForestLake.OPEN_WATER.z, radius: 3.5 },
    ];
    const scatter = (this.scatter = new Scatter(terrain, keep, season, cliffs.ledges));
    this.water = new Water(
      terrain,
      [...cliffs.froth, ...scatter.froth, ...landmarks.froth, ...pools.map((p) => ({ x: p.x, z: p.z, radius: p.width + 1.6, strength: 1 }))],
      falls,
      season,
      { x: ForestLake.OPEN_WATER.x, z: ForestLake.OPEN_WATER.z, radius: 3.2 },
    );
    this.ground = terrain.mesh([...scatter.shades, ...landmarks.shades, ...cliffs.shades]);
    scatter.build();
    this.backdrop = new Backdrop(season);
    const trees = [...landmarks.cherries, ...scatter.shedding];
    this.falling = season === 'spring' ? new Falling(trees, terrain, 'petals') : season === 'autumn' ? new Falling(trees, terrain, 'leaves') : season === 'winter' ? new Snowfall(terrain) : null;
    this.add(this.backdrop, this.ground, cliffs, scatter, landmarks, this.water);
    if (this.falling) this.add(this.falling);
  }

  // The ground to stand on: the land, the water over it (or its ice), a
  // deck, or the cave's floor.
  groundAt(x: number, z: number): number {
    const y = this.landAt(x, z);
    const water = this.terrain.waterAt(x, z);
    return Number.isNaN(water) ? y : Math.max(y, water);
  }

  // The same, but for the water: under it the bed (the forest lake
  // meeting's animals keep off the water, forest_lake_meeting/).
  landAt(x: number, z: number): number {
    let y = this.terrain.heightAt(x, z);
    if (Math.abs(z - SPOTS.cave.z) < 1.4 * CAVE_SCALE && x > SPOTS.cave.x - 0.2 - 0.9 * (CAVE_SCALE - 1) && x < SPOTS.cave.x + 0.7 + 3.3 * CAVE_SCALE) y = Math.max(y, this.terrain.caveFloor);
    for (const deck of this.landmarks.decks) {
      const top = deck(x, z);
      if (!Number.isNaN(top)) y = Math.max(y, top);
    }
    return y;
  }

  // The lake's surface over a point now, with its waves (still where it
  // is ice).
  surfaceAt(x: number, z: number): number {
    return this.water.surfaceAt(x, z);
  }

  update(delta: number): void {
    this.water.update(delta);
    this.backdrop.update(delta);
    this.landmarks.update(delta);
    this.falling?.update(delta);
    // The moored boat rides the waves, easing toward them; in winter it
    // lies frozen in the ice.
    const { boat, boatRest } = this.landmarks;
    if (this.water.iceAt(boatRest.x, boatRest.z) > 0.5) return;
    const t = 1 - Math.exp(-3 * delta);
    // Its bow is its +x, turned by its turn about y.
    const ahead = new THREE.Vector3(Math.cos(boatRest.turn), 0, -Math.sin(boatRest.turn)).multiplyScalar(0.9);
    const bow = this.surfaceAt(boatRest.x + ahead.x, boatRest.z + ahead.z);
    const stern = this.surfaceAt(boatRest.x - ahead.x, boatRest.z - ahead.z);
    boat.position.y += ((bow + stern) / 2 - Rowboat.DRAFT - boat.position.y) * t;
    boat.rotation.z += (Math.atan2(bow - stern, 1.8) - boat.rotation.z) * t;
  }
}

// A river's waterfall: from its lip, the way its last stretch runs, as
// wide as it is, falling to the lake.
function fallOf(river: River, terrain: Terrain): Fall {
  const points = river.line.points;
  const [a, b] = [points[points.length - 2], points[points.length - 1]];
  const out = new THREE.Vector2(b[0] - a[0], b[1] - a[1]).normalize();
  return { x: b[0], z: b[1], level: terrain.riverLevel(river, river.line.length), width: river.lip.width, out, bottom: 0 };
}

// Petals falling round the cherry trees by the landing and on the island
// in spring, or in autumn leaves falling round them and the broadleaf
// trees near it: each drifts down from somewhere in a crown, turning over
// and swaying on the breeze, lies a while where it lands, and falls again.
class Falling extends THREE.InstancedMesh {
  private readonly state: { position: THREE.Vector3; spin: THREE.Vector3; turn: number; sway: number; age: number; resting: number; tree: number }[] = [];
  private readonly random = seededRandom(801);
  private readonly dummy = new THREE.Object3D();

  constructor(
    private readonly trees: readonly { x: number; y: number; z: number; height: number }[],
    private readonly terrain: Terrain,
    kind: 'petals' | 'leaves',
  ) {
    const shape = new THREE.BufferGeometry();
    const positions: number[] = [];
    const colors: number[] = [];
    if (kind === 'petals') {
      // A small pink petal: a pointed oval, 3 cm long.
      const points = [
        [0, 0, 0],
        [0.008, 0, 0.01],
        [0.011, 0, 0.022],
        [0, 0, 0.03],
        [-0.011, 0, 0.022],
        [-0.008, 0, 0.01],
      ];
      const light = color(PALETTE.blossomLight);
      const mid = color(PALETTE.blossom);
      for (let i = 1; i + 1 < points.length; i++) {
        for (const [k, c] of [
          [0, mid],
          [i, light],
          [i + 1, light],
        ] as const) {
          positions.push(...points[k]);
          colors.push(c.r, c.g, c.b);
        }
      }
    } else {
      // A leaf, 7 cm across: five pointed lobes round its middle, white
      // here, coloured copy by copy.
      const outline = Array.from({ length: 10 }, (_, i) => {
        const a = (2 * Math.PI * i) / 10;
        const r = i % 2 === 0 ? 0.035 : 0.016;
        return [Math.cos(a) * r, 0, Math.sin(a) * r];
      });
      for (let i = 0; i < 10; i++) {
        for (const [p, f] of [
          [[0, 0.004, 0], 0.8],
          [outline[i], 1],
          [outline[(i + 1) % 10], 1],
        ] as const) {
          positions.push(...p);
          colors.push(f, f, f);
        }
      }
    }
    shape.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    shape.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    shape.computeVertexNormals();
    super(shape, twoSided(), kind === 'petals' ? 180 : 160);
    this.frustumCulled = false;
    for (let i = 0; i < this.count; i++) {
      const state = { position: new THREE.Vector3(), spin: new THREE.Vector3(), turn: 0, sway: 0, age: 0, resting: 0, tree: 0 };
      this.respawn(state, true);
      this.state.push(state);
      if (kind === 'leaves') this.setColorAt(i, color(FALLEN_LEAVES[Math.floor(this.random() * FALLEN_LEAVES.length)]));
    }
    this.update(0);
  }

  private respawn(s: (typeof this.state)[number], anywhere: boolean): void {
    const random = this.random;
    if (!this.trees.length) return;
    s.tree = Math.floor(random() * this.trees.length);
    const tree = this.trees[s.tree];
    const angle = random() * Math.PI * 2;
    const r = Math.sqrt(random()) * tree.height * 0.42;
    s.position.set(tree.x + Math.cos(angle) * r, tree.y + tree.height * between(random, 0.5, 0.85), tree.z + Math.sin(angle) * r);
    if (anywhere) s.position.y -= random() * tree.height * 0.6;
    s.spin.set(random() * 6, random() * 6, random() * 6);
    s.turn = random() * Math.PI * 2;
    s.sway = random() * Math.PI * 2;
    s.age = 0;
    s.resting = 0;
  }

  update(delta: number): void {
    for (let i = 0; i < this.state.length; i++) {
      const s = this.state[i];
      s.age += delta;
      if (s.resting > 0) {
        s.resting -= delta;
        if (s.resting <= 0) this.respawn(s, false);
      } else {
        // Down slowly, drifting east on the breeze, swaying.
        s.position.x += (0.25 + 0.35 * Math.sin(s.age * 1.3 + s.sway)) * delta;
        s.position.z += 0.3 * Math.cos(s.age * 1.1 + s.sway) * delta;
        s.position.y -= (0.32 + 0.12 * Math.sin(s.age * 2.1 + s.sway)) * delta;
        const floor = Math.max(this.terrain.heightAt(s.position.x, s.position.z), Number.isNaN(this.terrain.waterAt(s.position.x, s.position.z)) ? -Infinity : waveHeight(s.position.x, s.position.z, s.age)) + 0.01;
        if (s.position.y < floor) {
          s.position.y = floor;
          s.resting = between(this.random, 2, 6);
        }
      }
      const tumbling = s.resting > 0 ? 0 : s.age;
      this.dummy.position.copy(s.position);
      this.dummy.rotation.set(s.spin.x * tumbling, s.turn + s.spin.y * tumbling * 0.3, s.spin.z * tumbling);
      this.dummy.updateMatrix();
      this.setMatrixAt(i, this.dummy.matrix);
    }
    this.instanceMatrix.needsUpdate = true;
  }
}

// Snow falling in winter, round the landing and over the lake: soft white
// flakes drifting down slowly, swaying, each starting again high above
// once it reaches the ground or the ice.
const FLAKES = 1800;
class Snowfall extends THREE.Points {
  private readonly flakes: { x: number; y: number; z: number; speed: number; sway: number; age: number }[] = [];
  private readonly random = seededRandom(811);
  private static readonly REACH = 38; // m round the landing, each way
  private static readonly HIGH = 18; // m over the ground it starts from

  constructor(private readonly terrain: Terrain) {
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(FLAKES * 3), 3));
    const material = new THREE.PointsMaterial({ color: 0xffffff, size: 0.075, map: flakeTexture(), transparent: true, depthWrite: false, opacity: 0.95 });
    super(geometry, material);
    this.frustumCulled = false;
    for (let i = 0; i < FLAKES; i++) {
      const flake = { x: 0, y: 0, z: 0, speed: 0, sway: 0, age: 0 };
      this.start(flake, true);
      this.flakes.push(flake);
    }
    this.update(0);
  }

  private start(flake: { x: number; y: number; z: number; speed: number; sway: number; age: number }, anywhere: boolean): void {
    const random = this.random;
    flake.x = LANDING[0] + between(random, -1, 1) * Snowfall.REACH;
    flake.z = LANDING[1] - 10 + between(random, -1, 1) * Snowfall.REACH;
    const ground = Math.max(0, this.terrain.heightAt(flake.x, flake.z));
    flake.y = ground + (anywhere ? random() : 1) * Snowfall.HIGH;
    flake.speed = between(random, 0.55, 0.95);
    flake.sway = random() * Math.PI * 2;
    flake.age = 0;
  }

  update(delta: number): void {
    const position = this.geometry.getAttribute('position') as THREE.BufferAttribute;
    for (let i = 0; i < this.flakes.length; i++) {
      const f = this.flakes[i];
      f.age += delta;
      f.y -= f.speed * delta;
      f.x += (0.18 + 0.22 * Math.sin(f.age * 0.9 + f.sway)) * delta;
      f.z += 0.16 * Math.cos(f.age * 0.7 + f.sway) * delta;
      if (f.y < Math.max(0, this.terrain.heightAt(f.x, f.z))) this.start(f, false);
      position.setXYZ(i, f.x, f.y, f.z);
    }
    position.needsUpdate = true;
  }
}

// A soft round flake: white, fading out to its edge.
let flake: THREE.DataTexture | null = null;
function flakeTexture(): THREE.DataTexture {
  if (flake) return flake;
  const n = 32;
  const data = new Uint8Array(n * n * 4);
  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      const d = Math.hypot(x + 0.5 - n / 2, y + 0.5 - n / 2) / (n / 2);
      const a = Math.max(0, 1 - d * d) ** 1.5;
      data.set([255, 255, 255, Math.round(255 * a)], (y * n + x) * 4);
    }
  }
  flake = new THREE.DataTexture(data, n, n);
  flake.needsUpdate = true;
  return flake;
}
