import * as THREE from 'three';
import type { Theme } from '../../theme';
import { waterSurface } from '../water';
import { SHORE_MAX } from './Ground';
import { Splashes } from './Splash';

// The lake's water: a see-through sheet at the water level (y = 0) moving in
// small waves. It is a square a little wider than the lake; past the shore it
// runs under the land, which hides it.
//
// The waves are a few sine waves crossing each other. Each travels at the
// speed deep water gives its length, so longer waves travel faster. The same
// sum moves the sheet's points and answers heightAt(), so anything riding the
// waves follows exactly what is drawn. The sheet's normals come from the
// waves' slopes, so they need no recomputing from the triangles.
//
// splash() throws up a splash where something breaks the surface, such as a
// leaping fish (see Splash.ts); its ripples ride the same waves.
//
// In wind (setWind, from the lake's weather) the waves grow higher, a short
// ripple runs downwind across them, and the surface roughens so its glint
// dulls; heightAt() follows, so a floating figure rocks more.

const SIZE = 2 * (SHORE_MAX + 1);
const SEGMENTS = 128; // along each side: 18 cm squares, 6 per shortest wave
const GRAVITY = 9.81;
const OPACITY = 0.72; // how much of the lakebed the water hides
const ROUGHNESS = 0.12; // low: a calm surface with a clear sun glint
const WIND_WAVES = 1.2; // in a full wind the waves are this much higher again
const RIPPLE = { length: 0.9, height: 0.02 }; // m: the short wave a full wind raises, running downwind
const WIND_ROUGHNESS = 0.25; // how much rougher the surface is in a full wind

// [length (m), height from trough to crest (m), direction it travels (radians
// from +x toward +z), phase].
const WAVES: [length: number, height: number, direction: number, phase: number][] = [
  [3.2, 0.03, 0.5, 0],
  [1.9, 0.02, 1.4, 1.7],
  [1.1, 0.012, -0.5, 4.2],
];

interface Wave {
  kx: number; // wave number along x and z (radians per meter)
  kz: number;
  omega: number; // radians per second
  amplitude: number;
  phase: number;
}

const waves: Wave[] = WAVES.map(([length, height, direction, phase]) => {
  const k = (2 * Math.PI) / length;
  return {
    kx: k * Math.cos(direction),
    kz: k * Math.sin(direction),
    omega: Math.sqrt(GRAVITY * k),
    amplitude: height / 2,
    phase,
  };
});

export class Water extends THREE.Group {
  private time = 0; // seconds of wave motion so far
  private readonly geometry: THREE.PlaneGeometry;
  private readonly material: THREE.MeshStandardMaterial;
  private readonly splashes: Splashes;
  // The waves as they are now: the calm ones, higher in wind, and the ripple.
  private current: Wave[] = waves;

  constructor(theme: Theme) {
    super();
    this.name = 'water';

    this.geometry = new THREE.PlaneGeometry(SIZE, SIZE, SEGMENTS, SEGMENTS);
    this.geometry.rotateX(-Math.PI / 2);

    const material = (this.material = new THREE.MeshStandardMaterial({
      color: theme.scene.water,
      roughness: ROUGHNESS,
      transparent: true,
      opacity: OPACITY,
    }));
    waterSurface(material);

    const sheet = new THREE.Mesh(this.geometry, material);
    sheet.receiveShadow = true;
    this.add(sheet);
    this.shape();

    this.splashes = new Splashes(theme, (x, z) => this.heightAt(x, z));
    this.add(this.splashes);
  }

  // The water's height at a point of the lake, now.
  heightAt(x: number, z: number): number {
    let y = 0;
    for (const w of this.current) y += w.amplitude * Math.sin(w.kx * x + w.kz * z - w.omega * this.time + w.phase);
    return y;
  }

  // How strong the wind is (0 still, 1 strong) and the way it blows over the
  // water (x, z, a unit vector). Call before update() each frame.
  setWind(strength: number, direction: THREE.Vector2): void {
    const higher = 1 + WIND_WAVES * strength;
    const k = (2 * Math.PI) / RIPPLE.length;
    this.current = [
      ...waves.map((w) => ({ ...w, amplitude: w.amplitude * higher })),
      { kx: k * direction.x, kz: k * direction.y, omega: Math.sqrt(GRAVITY * k), amplitude: (RIPPLE.height / 2) * strength, phase: 0 },
    ];
    this.material.roughness = ROUGHNESS + WIND_ROUGHNESS * strength;
  }

  // Throws up a splash where something breaks the surface at a point of the
  // lake, moving at `velocity` meters a second.
  splash(x: number, z: number, velocity: THREE.Vector3): void {
    this.splashes.splash(x, z, velocity);
  }

  // Moves the waves and the splashes on by delta seconds.
  update(delta: number): void {
    this.time += delta;
    this.shape();
    this.splashes.update(delta);
  }

  // Sets every point of the sheet to the waves' height and its normal to
  // their slope: for y = f(x, z), the normal is (-df/dx, 1, -df/dz).
  private shape(): void {
    const position = this.geometry.attributes.position;
    const normal = this.geometry.attributes.normal;
    const n = new THREE.Vector3();
    for (let i = 0; i < position.count; i++) {
      const x = position.getX(i);
      const z = position.getZ(i);
      let y = 0;
      let dx = 0;
      let dz = 0;
      for (const w of this.current) {
        const a = w.kx * x + w.kz * z - w.omega * this.time + w.phase;
        y += w.amplitude * Math.sin(a);
        const slope = w.amplitude * Math.cos(a);
        dx += slope * w.kx;
        dz += slope * w.kz;
      }
      position.setY(i, y);
      n.set(-dx, 1, -dz).normalize();
      normal.setXYZ(i, n.x, n.y, n.z);
    }
    position.needsUpdate = true;
    normal.needsUpdate = true;
  }
}
