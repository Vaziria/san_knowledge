import * as THREE from 'three';
import { Dock } from '../../figures/objects/Dock';
import { defaultTheme, type Theme } from '../../theme';
import { Border } from '../Border';
import { Sky } from '../Sky';
import { Boulders } from './Boulders';
import { DayNight } from './DayNight';
import { deckPoint, dockSite, inApproach, inTheWay, nearDeck, onDeck, type Deck, type DockPlace } from './DockSite';
import { Fireflies } from './Fireflies';
import { CLEAR, DEPTH, Ground, groundHeight, LAKE_RADIUS, LAND_HEIGHT, LAND_RADIUS, LANDING } from './Ground';
import { Logs, type LogsPlace } from './Logs';
import { Meadow } from './Meadow';
import { Rain } from './Rain';
import { MudPits } from './MudPits';
import { Stones } from './Stones';
import { Swamps } from './Swamps';
import { Sward } from './Sward';
import { Trees } from './Trees';
import { Water } from './Water';
import { Weather } from './Weather';

// A small lake, about 18 m across and 1.4 m deep in the middle, with a low
// bank and uneven land around it to the horizon, humped near the lake and
// rolling into low hills far out, put together from its ground (earth
// textured with grit, pebbles and cracks), its water, clumps of stones along
// the lakeside, four big boulders, nine trees of the tree figures' kinds
// around it and two more, a maple and an oak, in autumn colours, and grass
// covering the land: a sward of short tufts, and a meadow of the grass
// figures' kinds growing out of it in patches of varied sizes. Every colour
// comes from the theme: the bank is the floor colour and the land beyond it a
// lawn in the grass colour, the water theme.scene.water, the stones and
// boulders theme.scene.stone, the trees' bark and leaves its wood and grass
// colours (the autumn trees' leaves its autumn colour), and the sward and
// meadow its grass colour. Five mud pits lie about the land (MudPits.ts),
// the grass keeping off them. Units
// are meters, y is up, and the origin is the center of the lake on its still
// water surface, so y = 0 is the water level; the top of the bank is
// LAND_HEIGHT above it, and groundAt() gives the uneven land's height.
//
// One of the environments the preview can show a figure in (previews.ts),
// like the plain floor. The spec, Lake.md, has no behaviour yet. On its own
// the water moves in small waves (update), and surfaceAt() gives the water's
// height anywhere, so a floating figure can ride the waves. Now and then fog
// rolls in (`weather.fog`, which the stage puts in the scene) and windy
// spells come and go, swaying the grass and the trees' crowns and roughening
// the water (Weather.ts). Anything hollow that floats must keep its inside
// dry (see ../water.ts).
//
// Where the land ends, 45 m out, the border (../Border.ts) hides the edge:
// high cliffs along parts of it, mist drifting round it and tall grass up to
// it, as the user asked (Lake.md), and the land fades toward the sky's colour
// over its last meters (Ground.ts).
//
// Over it all is the sky (../Sky.ts), as the user asked: deepening from the
// haze at the horizon to the theme's zenith colour, with the sun where the
// light comes from and clouds drifting with the wind, greying over in fog.
//
// Day and night take turns (DayNight.ts), as the user asked ("in lake add
// day and night, and if night, add firefly"), or hold at one (the Time of
// day setting). At night the sky, the fog, the land's fade at its end and
// the border's mist and cliffs darken to the theme's night colours, the sun
// sets, the water's glint dulls, and the stage dims its lights to moonlight
// (Environment.dayNight in previews.ts). Twenty fireflies come out at dusk
// over the grass round the landing and go at dawn (Fireflies.ts).
//
// Showers come now and then (Weather.ts), as the user asked ("add randomly
// rain in lake environtment"): rain falls round the camera and rings the
// water (Rain.ts), the sky greys over and hides the sun, the light dims a
// step, the water's glint dulls, and the fireflies shelter.
//
// A lake built with a dock (LakeOptions.dock: the lake meeting's, as the user
// asked, "in lake meeting add dock in the lake") has one running out from
// the bank (figures/objects/Dock.ts) where DockSite.ts finds room for it, and
// the grass keeps off it. Its deck is ground to walk on: deckAt() gives its
// height.
//
// A lake built with logs (LakeOptions.logs: the lake meeting's, as the user
// asked, "in lake metting, add random log in lake") has four floating in the
// water (Logs.ts), riding the waves and drifting a little downwind, clear of
// the shore, the boulders, the dock and each other.
//
// Swamps lie at the lakeside (Swamps.ts), as the user asked ("add swamps in
// lakeside"): up to three of the swamp terrain, dug into the land past the
// top of the bank, on the far shore where the preview cameras look and either
// side of the landing. They are sited after everything else, which keeps its
// place; the stones and the grass inside them go. groundAt() is the land as
// dug for them.

const STONE_GAP = 0.1; // m a scattered stone keeps from a boulder or a mud pit
const TRUNK_GAP = 0.5; // and from a trunk's middle
const BIG_STONE = 0.12; // m across the middle: a stone this big is walked round (obstacles())
const FOOT = 0.8; // m up a trunk that counts as its foot, roots and all
const GAP_OUT = 0.3; // m between two points of a trunk's foot, out from its middle, past which lie only branch tips

// How far a tree's wood reaches from its middle near the ground, measured
// from its geometry: up to FOOT above where it stands (every branch is in the
// same mesh as the trunk), out from the middle until a gap of more than
// GAP_OUT, past which lie only branch tips hanging low. Mostly the trunk's
// flaring foot (0.3–0.9 m); a spruce's lowest branches sweep the ground
// 4.5 m out, and something walking goes round them too.
function footRadius(tree: THREE.Object3D): number {
  const wood = tree.getObjectByName('trunk');
  if (!wood) return 0;
  tree.updateMatrixWorld(true);
  const inverse = new THREE.Matrix4().copy(tree.matrixWorld).invert();
  const point = new THREE.Vector3();
  const out: number[] = [];
  wood.traverse((part) => {
    if (!(part instanceof THREE.Mesh)) return;
    const position = (part.geometry as THREE.BufferGeometry).getAttribute('position');
    const toTree = new THREE.Matrix4().multiplyMatrices(inverse, part.matrixWorld);
    for (let i = 0; i < position.count; i++) {
      point.fromBufferAttribute(position, i).applyMatrix4(toTree);
      if (point.y * tree.scale.y <= FOOT) out.push(Math.hypot(point.x * tree.scale.x, point.z * tree.scale.z));
    }
  });
  out.sort((a, b) => a - b);
  let radius = out[0] ?? 0;
  for (let i = 1; i < out.length && out[i] - out[i - 1] <= GAP_OUT; i++) radius = out[i];
  return radius;
}

export interface LakeOptions {
  theme?: Theme;
  // m round the landing kept level, mown short and free of stones and meadow
  // plants: room for one figure by default (Ground.ts CLEAR, 2.2 m), more
  // for the lake meeting's animals. The trees and boulders stand further off
  // either way.
  clear?: number;
  // A dock out from the bank, found room for from `dock.angle` along the
  // shore (DockSite.ts); none when left out.
  dock?: DockPlace;
  // Logs floating in the water, kept off the shore, the boulders, the dock,
  // whatever `logs.keepOff` says and each other (Logs.ts); none when left
  // out.
  logs?: LogsPlace;
}

const GRASS_ROOM = 0.15; // m round a dock's deck the grass keeps off
const STEP_ON = 0.3; // m of a dock's deck over which the height to walk on eases from the land's to the deck's
// How much of the ground a swamp must cover to take away a stone there (its
// middle), a sward tuft, or a meadow plant: the tall meadow plants go first,
// and the sward thins into the swamp's own short grass, which thins out
// toward its edge.
const SWAMP_COVER = { stone: 0.5, sward: 0.35, meadow: 0.1 };
const SWAMP_STONE_GAP = 0.05; // m a swamp's plants keep from a stone left at its edge

export class Lake extends THREE.Group {
  static readonly RADIUS = LAKE_RADIUS; // average distance from the center to the shore
  static readonly DEPTH = DEPTH;
  static readonly LAND_HEIGHT = LAND_HEIGHT;
  // Where a figure stands on the lake's land: on the +z shore, 1.5 m past the
  // top of the bank, where the land is kept level, so a figure up to about 3 m
  // across stands on flat land with the water behind it (toward -z), where the
  // preview cameras look.
  static readonly LANDING = LANDING;

  readonly ground: Ground;
  readonly water: Water;
  readonly stones: Stones;
  readonly boulders: Boulders;
  readonly trees: Trees;
  readonly sward: Sward;
  readonly meadow: Meadow;
  readonly mudPits: MudPits;
  readonly border: Border;
  readonly sky: Sky;
  readonly weather: Weather;
  readonly dayNight: DayNight;
  readonly fireflies: Fireflies;
  readonly rain: Rain;
  // The sky's colour at the horizon now, which the fog, the land's fade and
  // the border follow; it darkens at night.
  private readonly horizon: THREE.Color;
  readonly clearing: number; // m round the landing kept level and clear (LakeOptions.clear)
  readonly dock: Dock | null; // LakeOptions.dock's, when there was room for it
  private site: Deck | null = null; // a dock's deck, its own or one siteDock() found room for
  readonly logs: Logs | null; // LakeOptions.logs'
  readonly swamps: Swamps;

  constructor(options: LakeOptions = {}) {
    super();
    this.name = 'lake';
    const theme = options.theme ?? defaultTheme;
    const clear = (this.clearing = options.clear ?? CLEAR);
    this.weather = new Weather(theme);
    this.dayNight = new DayNight(theme);
    const horizon = (this.horizon = new THREE.Color(theme.scene.background));
    const wind = this.weather.wind;
    this.water = new Water(theme, (x, z) => groundHeight(x, z, clear)); // none over the land the waves never reach, nor over the swamps dug into it
    this.stones = new Stones(theme, Lake.LANDING, clear); // the first 75, none where figures stand; the rest are scattered below
    this.boulders = new Boulders(theme); // placed by hand, away from where figures stand
    this.trees = new Trees(theme, Lake.LANDING, wind, this.boulders); // none where figures stand, nor where the cameras look from
    const trunks = this.trees.children.map((tree) => tree.position);
    // After the trees, stones and boulders, which keep their places; none in the clearing, nor against those.
    this.mudPits = new MudPits(theme, clear, trunks, this.boulders, this.stones.spots);
    // The rest of the stones, off the boulders, the trunks and the pits, which keep their places.
    this.stones.scatter(
      (x, z, radius) =>
        this.boulders.near(x, z, radius + STONE_GAP) || trunks.some((t) => Math.hypot(x - t.x, z - t.z) < radius + TRUNK_GAP) || this.mudPits.near(x, z, radius + STONE_GAP),
    );
    // The grass keeps off the boulders, the pits and the stones alike: among 220 stones, the big ones had tufts growing up through them.
    const keepOff = {
      near: (x: number, z: number, gap: number) => this.boulders.near(x, z, gap) || this.mudPits.near(x, z, gap) || this.stones.near(x, z, gap),
    };
    this.sward = new Sward(theme, Lake.LANDING, trunks, keepOff, wind, clear); // mown short where figures stand
    this.meadow = new Meadow(theme, Lake.LANDING, trunks, keepOff, wind, clear); // none where figures stand, nor against the trees, boulders or pits
    // A dock, after all of that, which keeps its place; the grass under it goes.
    const deck = options.dock ? this.siteDock(options.dock) : null;
    this.dock = deck ? this.buildDock(theme, deck) : null;
    if (this.dock) this.add(this.dock);
    // Logs floating in the water, after the dock, which they keep clear of.
    this.logs = options.logs
      ? new Logs({
          theme,
          place: options.logs,
          bedAt: (x, z) => this.groundAt(x, z),
          near: (x, z, gap) => this.boulders.near(x, z, gap) || (deck !== null && nearDeck(deck, x, z, gap)),
        })
      : null;
    if (this.logs) this.add(this.logs);
    // Swamps at the lakeside, after all of that, which keeps its place; the
    // stones and the grass inside them go. The ground is built last, dug for them.
    this.swamps = new Swamps({
      clear,
      trunks: this.trees.children.map((tree) => ({ x: tree.position.x, z: tree.position.z, radius: footRadius(tree) })),
      boulders: this.boulders.footprints,
      blocked: (x, z, gap) => this.mudPits.near(x, z, gap) || (deck !== null && (nearDeck(deck, x, z, gap) || inApproach(deck, x, z, gap))),
    });
    this.stones.takeAway((stone) => this.swamps.covers(stone.x, stone.z) > SWAMP_COVER.stone);
    this.swamps.build(theme, wind, (x, z) => this.stones.near(x, z, SWAMP_STONE_GAP));
    this.clearGrass(
      (x, z) => this.swamps.covers(x, z) > SWAMP_COVER.sward,
      (x, z) => this.swamps.covers(x, z) > SWAMP_COVER.meadow,
    );
    this.ground = new Ground(theme, clear, horizon, this.swamps);
    this.border = new Border({ theme, radius: LAND_RADIUS, heightAt: groundHeight, wind, sky: horizon });
    this.sky = new Sky({ theme, fog: this.weather.fog, wind });
    this.fireflies = new Fireflies(theme, (x, z) => Math.max(groundHeight(x, z, clear), 0)); // over the ground, or the still water
    this.rain = new Rain(theme, (x, z) => this.water.heightAt(x, z));
    this.add(this.ground, this.water, this.stones, this.boulders, this.trees, this.mudPits, this.sward, this.meadow, this.swamps, this.border, this.sky, this.fireflies, this.rain);
  }

  // The water's height at a point of the lake (lake coordinates), now.
  surfaceAt(x: number, z: number): number {
    return this.water.heightAt(x, z);
  }

  // The ground's height at a point of the lake: the bed under the water, the
  // bank or the land, dug where a swamp lies (not yet while the constructor
  // sites a dock or floats logs, before the swamps).
  groundAt(x: number, z: number): number {
    return groundHeight(x, z, this.clearing) - (this.swamps !== undefined ? this.swamps.dig(x, z) : 0);
  }

  // The deck of a dock, if there is one: its own (LakeOptions.dock) or one
  // siteDock() found room for.
  get deck(): Deck | null {
    return this.site;
  }

  // The height of a dock's deck at a point of the lake, where there is one;
  // -Infinity elsewhere. The ground to walk on is the higher of this and the
  // land (Environment.ground in previews.ts). Over its first STEP_ON m it
  // eases from the land's own height to the deck's, so walking on has no
  // step: the land is uneven across the deck's landward end, up to 7 cm off
  // its ramp within a stride of its middle.
  deckAt(x: number, z: number): number {
    const deck = this.site;
    if (!deck) return -Infinity;
    const { across, along } = onDeck(deck, x, z);
    if (Math.abs(across) > deck.width / 2 || along < 0 || along > deck.length) return -Infinity;
    const top = Dock.topAt(along, deck.height, deck.foot);
    return along < STEP_ON ? THREE.MathUtils.lerp(this.groundAt(x, z), top, along / STEP_ON) : top;
  }

  // Finds room for a dock from `place` along the shore (DockSite.ts), clear
  // of the boulders, mud pits and trees, which keep their places, and of the
  // swamps once there are any; clears the stones in its way and the grass
  // under it; and keeps its deck for deckAt(). Null when there is no room.
  // The dock's own preview asks for this on a lake built without one.
  siteDock(place: DockPlace): Deck | null {
    const trunks = this.trees.children.map((tree) => ({ x: tree.position.x, z: tree.position.z, radius: footRadius(tree) }));
    const groundAt = (x: number, z: number) => this.groundAt(x, z);
    const deck = dockSite({
      place,
      groundAt,
      blocked: (x, z, gap) =>
        this.boulders.near(x, z, gap) ||
        this.mudPits.near(x, z, gap) ||
        trunks.some((t) => Math.hypot(x - t.x, z - t.z) < t.radius + gap) ||
        (this.swamps !== undefined && this.swamps.near(x, z, gap)),
    });
    if (!deck) return null;
    const position = new THREE.Vector3();
    const turn = new THREE.Quaternion();
    const scale = new THREE.Vector3();
    this.stones.takeAway((stone) => {
      // The big stones on the way onto it too, which the animals walk round (obstacles()).
      if (stone.radius >= BIG_STONE && inApproach(deck, stone.x, stone.z, stone.radius)) return true;
      stone.matrix.decompose(position, turn, scale);
      return inTheWay(deck, { x: stone.x, z: stone.z, radius: stone.radius, top: position.y + scale.y }, groundAt);
    });
    this.site = deck;
    this.clearGrass((x, z) => {
      const { across, along } = onDeck(deck, x, z);
      return Math.abs(across) <= deck.width / 2 + GRASS_ROOM && along >= -GRASS_ROOM && along <= deck.length + GRASS_ROOM;
    });
    return deck;
  }

  // A dock on a deck siteDock() found, in the lake's coordinates, its posts
  // down to the bed.
  buildDock(theme: Theme, deck: Deck): Dock {
    const dock = new Dock({
      theme,
      width: deck.width,
      length: deck.length,
      height: deck.height,
      foot: deck.foot,
      bedAt: (x, z) => {
        const at = deckPoint(deck, x, z);
        return this.groundAt(at.x, at.z);
      },
    });
    dock.position.set(deck.x, 0, deck.z);
    dock.rotation.y = deck.heading;
    return dock;
  }

  // Takes the sward's tufts and the meadow's plants away where `sward` and
  // `meadow` say (points of the lake): from under a dock's deck and GRASS_ROOM
  // m round it, where they would come up through its planks, and from the
  // swamps. Every other plant keeps its place.
  private clearGrass(sward: (x: number, z: number) => boolean, meadow = sward): void {
    this.updateMatrixWorld(true);
    const toLake = this.matrixWorld.clone().invert();
    const matrix = new THREE.Matrix4();
    const none = new THREE.Matrix4().makeScale(0, 0, 0);
    const at = new THREE.Vector3();
    for (const [group, gone] of [
      [this.sward, sward],
      [this.meadow, meadow],
    ] as const) {
      group.traverse((object) => {
        if (!(object instanceof THREE.InstancedMesh)) return;
        const toLakeMesh = new THREE.Matrix4().multiplyMatrices(toLake, object.matrixWorld);
        let changed = false;
        for (let i = 0; i < object.count; i++) {
          object.getMatrixAt(i, matrix);
          at.setFromMatrixPosition(matrix).applyMatrix4(toLakeMesh);
          if (!gone(at.x, at.z)) continue;
          object.setMatrixAt(i, none);
          changed = true;
        }
        if (changed) object.instanceMatrix.needsUpdate = true;
      });
    }
  }

  // What stands on the land that something walking there must go round, as
  // circles on the ground (lake coordinates): the trees (their wood near the
  // ground, footRadius()), the boulders and the big stones. Mud pits and
  // small stones can be walked over, and the swamps' pools lie under 12 cm,
  // which the meeting's land takes for water (Swamps.ts). The lake meeting's
  // animals ask for it (story/lake_meeting/Land.ts), so whatever is added to
  // the land later belongs here too.
  obstacles(): { x: number; z: number; radius: number }[] {
    return [
      ...this.trees.children.map((tree) => ({ x: tree.position.x, z: tree.position.z, radius: footRadius(tree) })),
      ...this.boulders.footprints,
      ...this.stones.placed.filter((stone) => stone.radius >= BIG_STONE).map(({ x, z, radius }) => ({ x, z, radius })),
    ];
  }

  // Moves the time of day, the weather, the waves, the floating logs, the
  // mist, the clouds and the fireflies on; call once per frame.
  update(delta: number): void {
    this.weather.update(delta);
    const rain = this.weather.rain;
    this.dayNight.overcast = rain; // a shower's cloud dims the light a step
    this.dayNight.update(delta);
    const { night, scene } = this.dayNight;
    this.horizon.copy(scene.background);
    this.weather.fog.color.copy(scene.background);
    this.water.setNight(night, rain);
    this.water.setWind(this.weather.wind.strength.value, this.weather.wind.direction.value);
    this.water.update(delta);
    this.swamps.setNight(night, rain);
    this.swamps.update(delta);
    this.logs?.update(delta, this.water, this.weather.wind);
    this.border.update(delta, night);
    this.sky.setDaylight(scene.background, scene.zenith, night, rain);
    this.sky.update(delta);
    this.fireflies.update(delta, night, rain);
    this.rain.update(delta, rain, this.weather.wind);
  }
}
