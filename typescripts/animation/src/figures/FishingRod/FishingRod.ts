import * as THREE from 'three';
import { defaultTheme, type Theme } from '../../theme';
import { createMaterials, LINE_RADIUS, mesh } from './parts';
import { lineTie, Pole, POLE_LENGTH } from './Pole';

// A traditional fishing rod: a pole cut from a tree branch (ranting kayu),
// 2 m long, with a broken side twig and a few leaves still on it, and the
// fishing line (an 8 mm cord) tied just below its tip and hanging from
// there. No reel, no guides: the user found the spinning rod too modern.
// Every colour comes from the theme: bark and the pale cut ends are its wood
// colour, the leaves its grass colour, the line its dark colour.
//
// Units are meters. The pole lies along +z with its tip forward. Unlike a
// figure that stands, its origin is not on the floor but where the hand holds
// it: on the pole's axis 30 cm from the butt. That is the pivot for swinging
// it (rotation.x tips it up or down), and where Hold() puts it in a hand.
//
// The spec, FishingRod.md, has no behaviour yet. update() keeps the hanging
// line straight down, whatever way the rod is held; call it once per frame.

const LINE_DROP = 0.6; // how far the line hangs from the tip

export interface FishingRodOptions {
  theme?: Theme;
}

export class FishingRod extends THREE.Group {
  static readonly LENGTH = POLE_LENGTH;

  readonly pole: Pole;
  // The line hanging from the tip; its origin is where it is tied, and it
  // hangs down its -y. It is too thin to throw a useful shadow.
  readonly hangingLine = new THREE.Group();

  constructor(options: FishingRodOptions = {}) {
    super();
    this.name = 'fishing rod';
    const m = createMaterials(options.theme ?? defaultTheme);

    this.pole = new Pole(m);

    this.hangingLine.position.copy(lineTie());
    const dropGeo = new THREE.CylinderGeometry(LINE_RADIUS, LINE_RADIUS, LINE_DROP, 6);
    dropGeo.translate(0, -LINE_DROP / 2, 0);
    const drop = mesh(dropGeo, m.line);
    drop.castShadow = false;
    this.hangingLine.add(drop);

    this.add(this.pole, this.hangingLine);
  }

  // Keeps the hanging line pointing straight down in the world.
  update(): void {
    const parent = this.hangingLine.parent;
    if (!parent) return;
    parent.getWorldQuaternion(this.hangingLine.quaternion).invert();
  }
}
