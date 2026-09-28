import * as THREE from 'three';
import { Barrel } from '../../figures/ForestLake/Barrel';
import { Bridge } from '../../figures/ForestLake/Bridge';
import { Bush } from '../../figures/ForestLake/Bush';
import { Campfire } from '../../figures/ForestLake/Campfire';
import { CaveEntrance } from '../../figures/ForestLake/CaveEntrance';
import { CherryTree } from '../../figures/ForestLake/CherryTree';
import { Crate } from '../../figures/ForestLake/Crate';
import { Fence } from '../../figures/ForestLake/Fence';
import { FlowerCluster } from '../../figures/ForestLake/FlowerCluster';
import { ForestGate } from '../../figures/ForestLake/ForestGate';
import { ForestPine } from '../../figures/ForestLake/ForestPine';
import { LampPost } from '../../figures/ForestLake/LampPost';
import { MossyLog } from '../../figures/ForestLake/MossyLog';
import { addBlock, Flame, matte, merge, mesh, palette, seededRandom, Shape, snowOn, stonePaint, type Season } from '../../figures/ForestLake/parts';
import { Pier } from '../../figures/ForestLake/Pier';
import { Rowboat } from '../../figures/ForestLake/Rowboat';
import { Ruins } from '../../figures/ForestLake/Ruins';
import { SignPost } from '../../figures/ForestLake/SignPost';
import { Tent } from '../../figures/ForestLake/Tent';
import { WatchTower } from '../../figures/ForestLake/WatchTower';
import { CAVE_SCALE, LANDING, RUINS_SCALE, SPOTS } from './layout';
import type { Keep } from './Scatter';
import type { Shade, Terrain } from './Terrain';
import type { Froth } from './Water';

const UP = new THREE.Vector3(0, 1, 0);

// The forest lake's landmarks (ForestLake.md), where the reference's
// overview has them: the pier on the southwest shore with its lamp and the
// rowboat moored beside it, a shorter pier on the east shore and a jetty
// off the island; the arched bridge over the outlet above its cascade and a
// footbridge over the river above the waterfall; the watch tower on the
// heights; the cave in the east cliffs with a campfire before it; the ruins
// on their terrace to the west; the camp to the southwest, its tent, its
// campfire with a pot over it, log seats, crates and barrels behind a
// fence; the forest entrance's gate on the way in from the south; lamp
// posts, sign posts and fences along the paths; and on the island's plinth
// a big cherry tree, a pine, a bush and flowers, fenced round, lit by two
// double lamps, with steps up from its jetty. Each is its season's. What
// never moves is merged into a few meshes; the flames, lights, glows, the
// flag and the boat stay live.
export class Landmarks extends THREE.Group {
  readonly keep: Keep[] = [];
  readonly shades: Shade[] = [];
  readonly froth: Froth[] = [];
  readonly obstacles: { x: number; z: number; radius: number }[] = [];
  readonly boat: Rowboat;
  readonly boatRest: { x: number; z: number; turn: number };
  // What the walking camera can stand on: each deck's height under a point,
  // or NaN off it.
  readonly decks: ((x: number, z: number) => number)[] = [];
  // The piers: each landward end's middle, the way it runs out (radians from
  // +x toward +z), its size, and its deck's height over the water.
  readonly piers: { name: 'pier' | 'eastPier' | 'jetty'; x: number; z: number; heading: number; length: number; width: number; height: number }[] = [];
  // Where the cherry trees they plant stand (for their falling petals or
  // leaves).
  readonly cherries: { x: number; y: number; z: number; height: number }[] = [];
  // What a camera can't be in or see through, and what walking goes round:
  // all of it that never moves, merged, and the boat (the forest lake
  // meeting's, forest_lake_meeting/).
  readonly solid: THREE.Object3D[] = [];
  private readonly moving: { update(delta: number): void }[] = [];

  constructor(terrain: Terrain, season: Season = 'spring') {
    super();
    const random = seededRandom(701);
    const still: THREE.Object3D[] = [];
    const place = <T extends THREE.Object3D>(object: T, x: number, y: number, z: number, turn = 0, scale = 1): T => {
      object.position.set(x, y, z);
      object.rotation.y = turn;
      object.scale.setScalar(scale);
      still.push(object);
      return object;
    };
    const ground = (x: number, z: number) => terrain.heightAt(x, z);
    const P = palette(season);

    // The piers, each running out along its heading from its landward end.
    const pierAt = (name: 'pier' | 'eastPier' | 'jetty', options: { width?: number; height?: number; lamp?: boolean }) => {
      const spot = SPOTS[name];
      const turn = -spot.heading;
      const width = options.width ?? 1.9;
      this.piers.push({ name, x: spot.x, z: spot.z, heading: spot.heading, length: spot.length, width, height: options.height ?? 0.5 });
      const lampSpot = new THREE.Vector3(0.35, 0, -(width / 2 + 0.22)).applyAxisAngle(UP, turn).add(new THREE.Vector3(spot.x, 0, spot.z));
      const pier = new Pier({ length: spot.length, width, height: options.height ?? 0.5, depth: 3.2, lamp: options.lamp ?? true, shore: ground(lampSpot.x, lampSpot.z), seed: Math.floor(random() * 1000), season });
      place(pier, spot.x, 0, spot.z, turn);
      this.moving.push(pier);
      const inverse = new THREE.Matrix4();
      pier.updateMatrix();
      inverse.copy(pier.matrix).invert();
      const local = new THREE.Vector3();
      this.decks.push((x, z) => {
        local.set(x, 0, z).applyMatrix4(inverse);
        return pier.deckAt(local.x, local.z);
      });
      for (let d = 0; d <= spot.length; d += 1.5) {
        const x = spot.x + Math.cos(spot.heading) * d;
        const z = spot.z + Math.sin(spot.heading) * d;
        this.keep.push({ x, z, radius: width * 0.9 });
        if (ground(x, z) < 0) this.froth.push({ x, z, radius: 0.9, strength: 0.3 });
      }
      return pier;
    };
    pierAt('pier', {});
    pierAt('eastPier', {});
    pierAt('jetty', { width: 1.4, height: 0.45, lamp: false });

    // The rowboat, moored beside the pier; it rides the waves (update()),
    // or in winter lies frozen in the ice.
    this.boat = new Rowboat({ seed: 5, season });
    this.boat.keepDry();
    this.boatRest = { x: SPOTS.boat.x, z: SPOTS.boat.z, turn: -SPOTS.boat.heading };
    this.boat.position.set(SPOTS.boat.x, -Rowboat.DRAFT, SPOTS.boat.z);
    this.boat.rotation.y = this.boatRest.turn;
    this.add(this.boat);
    this.keep.push({ x: SPOTS.boat.x, z: SPOTS.boat.z, radius: 2.2 });

    // The bridge over the outlet, arching, from path end to path end.
    const [from, to] = [SPOTS.bridge.from, SPOTS.bridge.to];
    const span = Math.hypot(to[0] - from[0], to[1] - from[1]);
    const bridge = new Bridge({ length: span + 0.6, width: 1.7, arch: 0.45, seed: 3, season });
    const bridgeY = Math.max(ground(...from), ground(...to)) - 0.05;
    const bridgeTurn = -Math.atan2(to[1] - from[1], to[0] - from[0]);
    place(bridge, (from[0] + to[0]) / 2, bridgeY, (from[1] + to[1]) / 2, bridgeTurn);
    this.deckOf(bridge);
    for (let t = 0; t <= 1; t += 0.2) this.keep.push({ x: from[0] + (to[0] - from[0]) * t, z: from[1] + (to[1] - from[1]) * t, radius: 1.6 });
    // The footbridge over the river above the main waterfall.
    const foot = SPOTS.footbridge;
    const footbridge = new Bridge({ length: 6.8, width: 1.5, arch: 0.3, seed: 4, season });
    place(footbridge, foot.x, Math.max(ground(foot.x - 3.4, foot.z), ground(foot.x + 3.4, foot.z)) - 0.05, foot.z, 0);
    this.deckOf(footbridge);
    this.keep.push({ x: foot.x, z: foot.z, radius: 4 });

    // The watch tower, its ladder toward the path up to it.
    const tower = new WatchTower({ seed: 2, season });
    const towerY = ground(SPOTS.tower.x, SPOTS.tower.z);
    place(tower, SPOTS.tower.x, towerY, SPOTS.tower.z, Math.atan2(26.2 - SPOTS.tower.x, -18.5 - SPOTS.tower.z));
    this.moving.push(tower);
    this.keep.push({ x: SPOTS.tower.x, z: SPOTS.tower.z, radius: 3.6 });
    this.shades.push({ x: SPOTS.tower.x, y: towerY + 6.8, z: SPOTS.tower.z, radius: 2.2, foot: 2.6 }, { x: SPOTS.tower.x, y: towerY + 4.6, z: SPOTS.tower.z, radius: 1.8 });
    this.obstacles.push({ x: SPOTS.tower.x, z: SPOTS.tower.z, radius: 1.8 });

    // The cave, its mouth facing west out of the east cliffs, as big as the
    // reference draws it, and a campfire before it.
    const cave = new CaveEntrance({ seed: 3, season });
    const caveY = ground(SPOTS.cave.x - 2.2, SPOTS.cave.z);
    place(cave, SPOTS.cave.x + 0.7, caveY, SPOTS.cave.z, -Math.PI / 2, CAVE_SCALE);
    this.moving.push(cave);
    this.keep.push({ x: SPOTS.cave.x + 1, z: SPOTS.cave.z, radius: 4.6 * CAVE_SCALE, bare: true });
    const caveFire = new Campfire({ seed: 6, light: false, season });
    place(caveFire, SPOTS.cave.x - 3.6, ground(SPOTS.cave.x - 3.6, SPOTS.cave.z + 2.6), SPOTS.cave.z + 2.6);
    this.moving.push(caveFire);
    this.keep.push({ x: SPOTS.cave.x - 3.6, z: SPOTS.cave.z + 2.6, radius: 1.4, bare: true });

    // The ruins on their terrace, the arch facing the lake.
    place(new Ruins({ seed: 5, season }), SPOTS.ruins.x, ground(SPOTS.ruins.x, SPOTS.ruins.z) - 0.02, SPOTS.ruins.z, Math.PI / 2, RUINS_SCALE);
    this.keep.push({ x: SPOTS.ruins.x, z: SPOTS.ruins.z, radius: 5.2 * RUINS_SCALE }, { x: SPOTS.ruins.x + 2.6 * RUINS_SCALE, z: SPOTS.ruins.z, radius: 3.2 * RUINS_SCALE });
    this.shades.push({ x: SPOTS.ruins.x, y: ground(SPOTS.ruins.x, SPOTS.ruins.z) + 2.2 * RUINS_SCALE, z: SPOTS.ruins.z, radius: 1.6 * RUINS_SCALE });

    // The camp: the tent facing the fire, a pot over the fire, log seats
    // round it, crates and barrels by the tent, a lamp at its way in, a
    // fence round its back.
    const camp = SPOTS.camp;
    const campY = ground(camp.x, camp.z);
    const fire = SPOTS.campfire;
    this.moving.push(place(new Campfire({ seed: 7, pot: true, season }), fire.x, campY - 0.02, fire.z));
    const tentTurn = Math.atan2(fire.x - SPOTS.tent.x, fire.z - SPOTS.tent.z);
    place(new Tent({ seed: 2, season }), SPOTS.tent.x, campY, SPOTS.tent.z, tentTurn);
    for (const [angle, length] of [
      [0.5, 1.6],
      [2.3, 1.9],
      [4.2, 1.7],
    ]) {
      const x = fire.x + Math.cos(angle) * 1.75;
      const z = fire.z + Math.sin(angle) * 1.75;
      place(new MossyLog({ seed: Math.floor(angle * 10), length, radius: 0.19, dressing: false, mushrooms: false, moss: false, season }), x, campY - 0.05, z, -angle + Math.PI / 2);
    }
    const byTent = new THREE.Vector3(1.9, 0, 0.4).applyAxisAngle(UP, tentTurn).add(new THREE.Vector3(SPOTS.tent.x, campY, SPOTS.tent.z));
    place(new Crate({ seed: 11, season }), byTent.x, campY, byTent.z, tentTurn + 0.2);
    place(new Crate({ seed: 12, size: 0.5, season }), byTent.x + 0.05, campY + 0.6, byTent.z + 0.05, tentTurn - 0.3);
    place(new Crate({ seed: 13, size: 0.55, season }), byTent.x + 0.75, campY, byTent.z - 0.55, tentTurn + 0.6);
    const barrelAt = new THREE.Vector3(-1.9, 0, 0.6).applyAxisAngle(UP, tentTurn).add(new THREE.Vector3(SPOTS.tent.x, campY, SPOTS.tent.z));
    place(new Barrel({ seed: 14, season }), barrelAt.x, campY, barrelAt.z);
    place(new Barrel({ seed: 15, height: 0.75, season }), barrelAt.x - 0.62, campY, barrelAt.z + 0.35);
    const campLamp = place(new LampPost({ seed: 16, season }), camp.x + 4.6, ground(camp.x + 4.6, camp.z - 1.4), camp.z - 1.4, Math.PI * 0.9);
    this.moving.push(campLamp);
    const arc: [number, number][] = [];
    for (let a = 1.2; a <= 3.9; a += 1.15 / (camp.radius + 0.9)) arc.push([camp.x + Math.cos(a) * (camp.radius + 0.9), camp.z + Math.sin(a) * (camp.radius + 0.9)]);
    place(new Fence({ posts: arc, groundAt: ground, side: -1, seed: 17, season }), 0, 0, 0);
    this.keep.push({ x: camp.x, z: camp.z, radius: camp.radius + 1.3 });
    this.shades.push({ x: SPOTS.tent.x, y: campY + 0.9, z: SPOTS.tent.z, radius: 1.3, foot: 2 });

    // The forest entrance: the gate over the way in from the south, a fence
    // running off into the forest from each of its posts.
    {
      const { line, width } = terrain.paths[SPOTS.gate.path];
      const at = line.at(SPOTS.gate.along);
      const gateTurn = Math.atan2(at.dx, at.dz);
      const gate = new ForestGate({ width: width + 1.1, seed: 5, season });
      place(gate, at.x, ground(at.x, at.z) - 0.02, at.z, gateTurn);
      this.moving.push(gate);
      this.keep.push({ x: at.x, z: at.z, radius: 2.6 });
      const across = new THREE.Vector3(1, 0, 0).applyAxisAngle(UP, gateTurn);
      for (const side of [-1, 1]) {
        const posts: [number, number][] = [];
        for (let d = (width + 1.1) / 2 + 0.25; d < (width + 1.1) / 2 + 6; d += 1.2) posts.push([at.x + across.x * side * d, at.z + across.z * side * d]);
        place(new Fence({ posts, groundAt: ground, side: 1, seed: 40 + side, season }), 0, 0, 0);
        for (const [x, z] of posts) this.keep.push({ x, z, radius: 0.7 });
      }
    }

    // Lamp posts along the paths, their lanterns hanging over them, every
    // 13 m or so, first on one side then the other; fences along stretches
    // of them; sign posts where they meet.
    terrain.paths.forEach(({ line, width }, p) => {
      let side = p % 2 ? 1 : -1;
      for (let along = 4; along < line.length - 2; along += 13 + random() * 3) {
        const at = line.at(along);
        const off = side * (width / 2 + 0.6);
        const x = at.x - at.dz * off;
        const z = at.z + at.dx * off;
        side = -side;
        if (this.near(x, z, 2.2) || this.inView(x, z) || terrain.lake.distance(x, z) < 1) continue;
        // Its arm reaches back over the path.
        const turn = Math.atan2(at.dx * off, -at.dz * off) + Math.PI / 2;
        const lamp = place(new LampPost({ seed: Math.floor(random() * 1000), season }), x, ground(x, z), z, turn);
        this.moving.push(lamp);
        this.keep.push({ x, z, radius: 0.8 });
      }
    });
    const fenceRun = (p: number, from: number, to: number, side: number, seed: number) => {
      const { line, width } = terrain.paths[p];
      const posts: [number, number][] = [];
      for (let along = from; along <= to; along += 1.25) {
        const at = line.at(along);
        const off = side * (width / 2 + 0.75);
        const x = at.x - at.dz * off;
        const z = at.z + at.dx * off;
        if (this.inView(x, z) || this.near(x, z, 1)) {
          if (posts.length > 1) place(new Fence({ posts: [...posts], groundAt: ground, side: side > 0 ? -1 : 1, seed: seed + posts.length, season }), 0, 0, 0);
          posts.length = 0;
          continue;
        }
        posts.push([x, z]);
      }
      if (posts.length > 1) place(new Fence({ posts, groundAt: ground, side: side > 0 ? -1 : 1, seed, season }), 0, 0, 0);
    };
    fenceRun(0, 1, 10, -1, 21); // the lake side of the forest path, east of the camp
    fenceRun(0, 1, 7, 1, 22);
    fenceRun(0, 28, 33, 1, 23);
    fenceRun(2, 2, 12, 1, 24); // the path to the ruins
    fenceRun(1, 1, 6, -1, 25); // up from the bridge
    fenceRun(5, 12, 20, 1, 26); // the way in, past the gate
    fenceRun(5, 12, 20, -1, 27);
    const signs: [number, number, number, ('left' | 'right')[], boolean][] = [
      [camp.x + 5.8, camp.z - 3.4, 0.4, ['right', 'left'], true],
      [SPOTS.bridge.from[0] - 1.4, SPOTS.bridge.from[1] + 1.8, -0.3, ['right', 'right'], true],
      [-23.2, 6.2, 1.2, ['left', 'right'], false],
      [4.1, 19.1, -0.6, ['left', 'right'], false],
    ];
    for (const [x, z, turn, boards, lantern] of signs) {
      const sign = place(new SignPost({ seed: Math.floor(x * 10), boards, lantern, season }), x, ground(x, z), z, turn);
      this.moving.push(sign);
      this.keep.push({ x, z, radius: 0.7 });
    }

    // The island, on its plinth of rock: its big cherry tree in the middle,
    // a pine behind it, a bush and flowers, a fence round its top on either
    // side, a double lamp each side of the tree, a sign, and steps up from
    // its jetty.
    const hero = { x: 6.6, z: -3.4, height: 7.2 };
    place(new CherryTree({ seed: 9, height: hero.height, season }), hero.x, ground(hero.x, hero.z), hero.z, 0.6);
    this.cherries.push({ x: hero.x, y: ground(hero.x, hero.z), z: hero.z, height: hero.height });
    this.shades.push({ x: hero.x, y: ground(hero.x, hero.z) + hero.height * 0.68, z: hero.z, radius: hero.height * 0.4, foot: 2 });
    place(new ForestPine({ seed: 8, height: 5.6, season }), 4.1, ground(4.1, -5.9), -5.9, 0.3);
    this.shades.push({ x: 4.1, y: ground(4.1, -5.9) + 2.4, z: -5.9, radius: 1.4, foot: 1 });
    place(new Bush({ seed: 7, season }), 9.4, ground(9.4, -2.1), -2.1, 1.1);
    for (const [x, z, seed, lanterns] of [
      [3.7, -1.9, 31, 2],
      [9.7, -5.3, 32, 2],
    ] as const) {
      // Its crossbar runs round the tree, a lantern either side.
      const lamp = place(new LampPost({ seed, lanterns, height: 2.3, season }), x, ground(x, z), z, Math.atan2(z - hero.z, -(x - hero.x)) + Math.PI / 2);
      this.moving.push(lamp);
      this.keep.push({ x, z, radius: 1.1 });
    }
    for (const [x, z, seed, kind] of [
      [5.1, -0.3, 51, 'pink'],
      [8.2, -0.8, 52, 'mixed'],
      [8.9, -4.6, 53, 'pink'],
      [4.6, -4.4, 54, 'mixed'],
    ] as const) {
      place(new FlowerCluster({ seed, kind, season }), x, ground(x, z), z, seed);
    }
    const islandSign = place(new SignPost({ seed: 33, boards: ['left', 'right'], dressing: false, season }), 7.9, ground(7.9, 0.1), 0.1, -0.4);
    this.moving.push(islandSign);
    // The fence round the island's top, in two runs, open to the jetty in
    // the south and on the far side.
    const center = new THREE.Vector2(6.7, -3.1);
    const rim = terrain.island.points;
    const run = (from: number, to: number, seed: number) => {
      const posts: [number, number][] = [];
      for (let i = 0; i < rim.length; i++) {
        const [x, z] = rim[i];
        const angle = Math.atan2(z - center.y, x - center.x);
        let a = angle - from;
        a = ((a % (2 * Math.PI)) + 2 * Math.PI) % (2 * Math.PI);
        if (a > to - from) continue;
        const d = Math.hypot(x - center.x, z - center.y);
        const inset = Math.max(0, d - 1.35) / d;
        if (i % 2 === 0) posts.push([center.x + (x - center.x) * inset, center.y + (z - center.y) * inset]);
      }
      posts.sort((p, q) => ((Math.atan2(p[1] - center.y, p[0] - center.x) - from + 4 * Math.PI) % (2 * Math.PI)) - ((Math.atan2(q[1] - center.y, q[0] - center.x) - from + 4 * Math.PI) % (2 * Math.PI)));
      if (posts.length > 1) place(new Fence({ posts, groundAt: ground, side: 1, seed, season }), 0, 0, 0);
    };
    run(-2.6, -1.1, 61); // round the north-west
    run(-0.5, 0.95, 62); // round the east
    // Steps of dressed stone from the jetty's head up onto the island.
    {
      const steps = new Shape();
      const paint = stonePaint(random, 0.8, 0.2, P);
      const jetty = SPOTS.jetty;
      const top = ground(jetty.x, jetty.z - 1.5);
      for (let k = 0; k < 3; k++) {
        const y = 0.45 + ((top - 0.45) * (k + 1)) / 3;
        addBlock(steps, new THREE.Vector3(jetty.x, (y - 0.3) / 2 - 0.02, jetty.z - 0.22 - k * 0.42), new THREE.Vector3(1.5, y + 0.3, 0.44), paint, random, { chamfer: 0.04, rough: 0.012 });
      }
      const stepsMesh = mesh(steps.geometry(), matte());
      if (season === 'winter') snowOn(stepsMesh, random, 0.5, 0.85);
      still.push(stepsMesh);
      this.decks.push((x, z) => {
        if (Math.abs(x - jetty.x) > 0.75) return NaN;
        for (let k = 0; k < 3; k++) {
          const zc = jetty.z - 0.22 - k * 0.42;
          if (Math.abs(z - zc) <= 0.22) return 0.45 + ((top - 0.45) * (k + 1)) / 3;
        }
        return NaN;
      });
    }
    for (const [x, z, r] of [
      [hero.x, hero.z, 1],
      [4.1, -5.9, 0.8],
      [9.4, -2.1, 0.9],
      [7.9, 0.1, 0.6],
    ]) this.keep.push({ x, z, radius: r });
    // The island's top is kept clear of the scatter's trees and bushes.
    this.keep.push({ x: center.x, z: center.y, radius: 3.4 });
    // A cherry tree each side of the landing's view, framing the lake.
    for (const [x, z, seed, height] of [
      [LANDING[0] - 6.8, LANDING[1] - 1.2, 41, 5.6],
      [LANDING[0] + 7.4, LANDING[1] - 0.2, 42, 5.2],
    ]) {
      place(new CherryTree({ seed, height, season }), x, ground(x, z), z, random() * Math.PI);
      this.cherries.push({ x, y: ground(x, z), z, height });
      this.shades.push({ x, y: ground(x, z) + height * 0.68, z, radius: height * 0.4, foot: 1.6 });
      this.keep.push({ x, z, radius: 2.2 });
    }

    // What moves or shines is taken out to stay live; the rest is merged.
    const live = new THREE.Group();
    for (const root of still) {
      root.updateMatrixWorld(true);
      const lively: THREE.Object3D[] = [];
      root.traverse((object) => {
        if (object instanceof Flame || object instanceof THREE.Sprite || object instanceof THREE.Points || object instanceof THREE.Light) lively.push(object);
        else if (object instanceof THREE.Mesh && (object.material as THREE.Material).name === 'flag') lively.push(object);
      });
      for (const object of lively) {
        if (lively.some((other) => other !== object && isInside(object, other))) continue;
        object.matrixWorld.decompose(object.position, object.quaternion, object.scale);
        object.removeFromParent();
        live.add(object);
      }
    }
    const merged = merge(still, true);
    this.add(merged, live);
    this.solid.push(merged, this.boat);
  }

  // Keeps a bridge's deck under the walking camera.
  private deckOf(bridge: Bridge): void {
    bridge.updateMatrix();
    const inverse = bridge.matrix.clone().invert();
    const local = new THREE.Vector3();
    this.decks.push((x, z) => {
      local.set(x, 0, z).applyMatrix4(inverse);
      return bridge.deckAt(local.x, local.z) + bridge.position.y;
    });
  }

  // Whether a point is within `radius` of a place already kept clear.
  private near(x: number, z: number, radius: number): boolean {
    return this.keep.some((k) => Math.hypot(x - k.x, z - k.z) < k.radius + radius);
  }

  // Whether a point is in the landing's view south, kept open.
  private inView(x: number, z: number): boolean {
    return z > LANDING[1] - 2 && z < LANDING[1] + 48 && Math.abs(x - LANDING[0]) < 3.5 + Math.max(0, z - LANDING[1]) * 0.4;
  }

  // The flames flicker, the lanterns glow, the flag waves.
  update(delta: number): void {
    for (const thing of this.moving) thing.update(delta);
  }
}

// Whether `object` is somewhere under `other`.
function isInside(object: THREE.Object3D, other: THREE.Object3D): boolean {
  for (let parent = object.parent; parent; parent = parent.parent) if (parent === other) return true;
  return false;
}
