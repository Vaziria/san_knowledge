import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';
import { BANK_WIDTH, DEPTH, Ground, groundHeight, LAKE_RADIUS, LAND_HEIGHT, shoreRadius } from './Ground';
import { Stones } from './Stones';
import { Water } from './Water';

// A small lake, about 18 m across and 1.4 m deep in the middle, with a low
// bank and flat land around it to the horizon, put together from its ground,
// its water and clumps of stones along the lakeside. Every colour comes from
// the theme: the land is the floor colour, the water theme.scene.water and
// the stones theme.scene.stone. Units are meters, y is up, and
// the origin is the center of the lake on its still water surface, so y = 0
// is the water level and the land is LAND_HEIGHT above it.
//
// One of the environments the preview can show a figure in (previews.ts),
// like the plain floor. The spec, Lake.md, has no
// behaviour yet; the water moves in small waves on its own (update), and
// surfaceAt() gives the water's height anywhere, so a floating figure can ride
// the waves. Anything hollow that floats must keep its inside dry (see
// ../water.ts).

export interface LakeOptions {
  theme?: Theme;
}

export class Lake extends THREE.Group {
  static readonly RADIUS = LAKE_RADIUS; // average distance from the center to the shore
  static readonly DEPTH = DEPTH;
  static readonly LAND_HEIGHT = LAND_HEIGHT;
  // Where a figure stands on the lake's land: on the +z shore, 1.5 m past the
  // top of the bank, so a figure up to about 3 m across stands on flat land
  // with the water behind it (toward -z), where the preview cameras look.
  static readonly LANDING = new THREE.Vector3(0, LAND_HEIGHT, shoreRadius(Math.PI / 2) + BANK_WIDTH + 1.5);

  readonly ground: Ground;
  readonly water: Water;
  readonly stones: Stones;

  constructor(options: LakeOptions = {}) {
    super();
    this.name = 'lake';
    const theme = options.theme ?? defaultTheme;
    this.ground = new Ground(theme);
    this.water = new Water(theme);
    this.stones = new Stones(theme, Lake.LANDING); // none where figures stand
    this.add(this.ground, this.water, this.stones);
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

  // Moves the waves on; call once per frame.
  update(delta: number): void {
    this.water.update(delta);
  }
}
