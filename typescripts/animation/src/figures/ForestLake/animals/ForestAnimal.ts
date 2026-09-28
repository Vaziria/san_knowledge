import * as THREE from 'three';
import { paint, type Palette } from './parts';

export interface ForestAnimalOptions {
  // One of its sheet's colour variations (its `variations`), the sheet's
  // default when left out.
  color?: string;
  seed?: number;
}

// What every forest lake animal shares: its colour variations, and moving
// itself. Each animal builds its body into `rig` and moves it in update().
//
// A walking or swimming animal moves itself: forward along its own +z at
// `speed` meters a second, so turn it (rotation.y) to steer. Poses that
// stay in place never move it.
export abstract class ForestAnimal extends THREE.Group {
  protected readonly rig = new THREE.Group();
  private readonly painted: THREE.Mesh[] = [];
  private current: string;
  protected speedNow = 0;
  // Up the world's y as well as forward, for an animal that climbs.
  protected climbNow = 0;

  protected constructor(
    readonly variations: Readonly<Record<string, Palette>>,
    options: ForestAnimalOptions,
  ) {
    super();
    const names = Object.keys(variations);
    this.current = options.color && Object.hasOwn(variations, options.color) ? options.color : names[0];
    this.add(this.rig);
  }

  // The names of its colour variations, the default first.
  get colors(): string[] {
    return Object.keys(this.variations);
  }

  get color(): string {
    return this.current;
  }

  protected get palette(): Palette {
    return this.variations[this.current];
  }

  // Forward speed in meters a second; 0 when still. Turning it by speed /
  // radius radians a second walks it round a circle of that radius.
  get speed(): number {
    return this.speedNow;
  }

  // Paints it in another of its sheet's colour variations. An unknown name
  // throws, naming the ones it has.
  SetColor(variation: string): void {
    if (!Object.hasOwn(this.variations, variation)) throw new Error(`"${variation}" is not one of its colours: ${this.colors.join(', ')}`);
    if (variation === this.current) return;
    this.current = variation;
    for (const mesh of this.painted) paint(mesh.geometry, this.palette);
  }

  // Keeps a mesh painted in the colour variation shown.
  protected painting<T extends THREE.Mesh>(mesh: T): T {
    this.painted.push(mesh);
    return mesh;
  }

  // Moves it along its heading at its speed (and up at its climbing speed).
  protected advance(delta: number): void {
    const heading = this.rotation.y;
    this.position.x += Math.sin(heading) * this.speedNow * delta;
    this.position.z += Math.cos(heading) * this.speedNow * delta;
    this.position.y += this.climbNow * delta;
  }

  abstract update(delta: number): void;
}
