import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';

// A plain floor, 20 m square, in the theme's floor colour: the surroundings a
// figure stands on when it needs nothing else. It has no water, so a figure
// that goes in water, such as the boat, rests on it. Units are meters, y is
// up, and the origin is the center of the floor, on its surface.

const SIZE = 20;

export interface FloorOptions {
  theme?: Theme;
}

export class Floor extends THREE.Group {
  constructor(options: FloorOptions = {}) {
    super();
    this.name = 'floor';
    const theme = options.theme ?? defaultTheme;
    const floor = new THREE.Mesh(
      new THREE.PlaneGeometry(SIZE, SIZE),
      new THREE.MeshStandardMaterial({ color: theme.scene.floor, roughness: 0.9 }),
    );
    floor.rotation.x = -Math.PI / 2;
    floor.receiveShadow = true;
    this.add(floor);
  }
}
