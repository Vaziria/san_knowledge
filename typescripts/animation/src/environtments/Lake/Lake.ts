import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';
import { DEPTH, Ground, groundHeight, LAKE_RADIUS, LAND_HEIGHT, LANDING } from './Ground';
import { Meadow } from './Meadow';
import { Stones } from './Stones';
import { Sward } from './Sward';
import { Trees } from './Trees';
import { Water } from './Water';
import { Weather } from './Weather';

// A small lake, about 18 m across and 1.4 m deep in the middle, with a low
// bank and uneven land around it to the horizon, humped near the lake and
// rolling into low hills far out, put together from its ground, its water,
// clumps of stones along the lakeside, nine trees of the tree figures' kinds
// around it, and grass covering the land: a sward of short tufts, and a
// meadow of the grass figures' kinds growing out of it in patches of varied
// sizes. Every colour comes from the theme: the bank is the floor
// colour and the land beyond it a lawn in the grass colour, the water
// theme.scene.water, the stones theme.scene.stone, the trees' bark and leaves
// its wood and grass colours, and the sward and meadow its grass colour. Units
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

export interface LakeOptions {
  theme?: Theme;
}

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
  readonly trees: Trees;
  readonly sward: Sward;
  readonly meadow: Meadow;
  readonly weather: Weather;

  constructor(options: LakeOptions = {}) {
    super();
    this.name = 'lake';
    const theme = options.theme ?? defaultTheme;
    this.weather = new Weather(theme);
    const wind = this.weather.wind;
    this.ground = new Ground(theme);
    this.water = new Water(theme);
    this.stones = new Stones(theme, Lake.LANDING); // none where figures stand
    this.trees = new Trees(theme, Lake.LANDING, wind); // none where figures stand, nor where the cameras look from
    const trunks = this.trees.children.map((tree) => tree.position);
    this.sward = new Sward(theme, Lake.LANDING, trunks, wind); // mown short where figures stand
    this.meadow = new Meadow(theme, Lake.LANDING, trunks, wind); // none where figures stand, nor against the trees
    this.add(this.ground, this.water, this.stones, this.trees, this.sward, this.meadow);
  }

  // The water's height at a point of the lake (lake coordinates), now.
  surfaceAt(x: number, z: number): number {
    return this.water.heightAt(x, z);
  }

  // The ground's height at a point of the lake: the bed under the water, the
  // bank or the land.
  groundAt(x: number, z: number): number {
    return groundHeight(x, z);
  }

  // Moves the weather and the waves on; call once per frame.
  update(delta: number): void {
    this.weather.update(delta);
    this.water.setWind(this.weather.wind.strength.value, this.weather.wind.direction.value);
    this.water.update(delta);
  }
}
