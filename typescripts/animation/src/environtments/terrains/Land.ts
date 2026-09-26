import * as THREE from 'three';
import { SOIL_MEAN, soilTexture } from '../../textures/soil';
import { defaultTheme, type Theme } from '../../theme';
import { hazeEdge } from '../haze';
import { surfaceMaterial } from './parts';

// Bare land for terrains to lie on: earth in the theme's floor colour, the
// soil texture over it (grit, pebbles and cracks, as on the lake's bank),
// shaped by `heightAt` and reaching `radius` m from its middle, where it
// fades into the sky's colour over its last meters (../haze.ts), so it meets
// the sky with no line. The Terrains tab shows each terrain lying on it
// (src/terrains.ts), and any environment can put it under its terrains.
//
// It is one mesh: rings round the middle, every 25 cm out to 12 m, where the
// terrains lie and the preview cameras look, then further apart, cut into
// spokes. Units are meters, y is up, and the origin is its middle; y = 0 is
// the level `heightAt` measures from.

export interface LandOptions {
  theme?: Theme;
  radius?: number; // m from the middle to where it ends
  heightAt?: (x: number, z: number) => number; // flat (0) when left out
  sky?: THREE.ColorRepresentation; // the colour it fades into at its end; the theme's background by default
}

const RADIUS = 40;
const NEAR = 12; // m out to which the rings are close together
const NEAR_STEP = 0.25; // m between them there
const GROWTH = 1.12; // how much further apart each ring beyond is than the last
const SPOKES = 240;
const BUMP = 0.012; // m, the soil's bump height, as on the lake's ground

export class Land extends THREE.Group {
  readonly ground: THREE.Mesh;

  constructor(options: LandOptions = {}) {
    super();
    this.name = 'land';
    const theme = options.theme ?? defaultTheme;
    const radius = options.radius ?? RADIUS;
    const heightAt = options.heightAt ?? (() => 0);

    const rings: number[] = [];
    for (let r = 0; r < NEAR; r += NEAR_STEP) rings.push(r);
    for (let step = NEAR_STEP, r = NEAR; r < radius; step *= GROWTH, r += step) rings.push(r);
    rings.push(radius);

    const color = new THREE.Color(theme.scene.floor);
    const positions: number[] = [];
    const colors: number[] = [];
    const uvs: number[] = [];
    for (const r of rings) {
      for (let j = 0; j < SPOKES; j++) {
        const angle = (2 * Math.PI * j) / SPOKES;
        const x = r * Math.cos(angle);
        const z = r * Math.sin(angle);
        positions.push(x, heightAt(x, z), z);
        colors.push(color.r, color.g, color.b);
        uvs.push(x, z); // the soil texture, laid flat in meters
      }
    }
    // Each quad between two rings and two spokes as two triangles facing up;
    // the last spoke joins the first.
    const at = (i: number, j: number) => i * SPOKES + (j % SPOKES);
    const indices: number[] = [];
    for (let i = 0; i < rings.length - 1; i++) {
      for (let j = 0; j < SPOKES; j++) {
        indices.push(at(i, j), at(i, j + 1), at(i + 1, j));
        indices.push(at(i, j + 1), at(i + 1, j + 1), at(i + 1, j));
      }
    }
    const count = positions.length / 3;
    const geo = new THREE.BufferGeometry();
    geo.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geo.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    geo.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    geo.setAttribute('soil', new THREE.Float32BufferAttribute(new Float32Array(count).fill(1), 1));
    geo.setAttribute('wet', new THREE.Float32BufferAttribute(new Float32Array(count), 1));
    geo.setIndex(indices);
    geo.computeVertexNormals();

    const material = surfaceMaterial({ texture: soilTexture(), mean: SOIL_MEAN, bump: BUMP, roughness: 0.9 });
    hazeEdge(material, options.sky ?? theme.scene.background, radius);
    this.ground = new THREE.Mesh(geo, material);
    this.ground.name = 'ground';
    this.ground.receiveShadow = true;
    this.add(this.ground);
  }
}
