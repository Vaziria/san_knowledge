import * as THREE from 'three';
import { defaultTheme, metal, surface, type Theme } from '../theme';

// A grand piano built from primitives. Units are meters, the floor is y = 0,
// the keyboard faces +z and runs along x (bass at -x). Every colour comes from
// the theme.

const CASE_WIDTH = 1.5;
const CASE_BOTTOM = 0.68;
const CASE_HEIGHT = 0.3;
const CASE_TOP = CASE_BOTTOM + CASE_HEIGHT;

const KEY_Y = 0.73;
const WHITE_WIDTH = 0.0235;
const WHITE_LENGTH = 0.15;
const WHITE_HEIGHT = 0.022;
const BLACK_WIDTH = 0.0125;
const BLACK_LENGTH = 0.095;
const BLACK_HEIGHT = 0.02;
const KEY_PRESS_ANGLE = 0.06;
const KEY_SPEED = 25;

const LID_ANGLE = 0.55;

const BLACK_NOTES = new Set([1, 3, 6, 8, 10]);

function createMaterials(theme: Theme) {
  const c = theme.colors;
  return {
    body: surface(theme, c.body),
    whiteKey: surface(theme, c.light),
    blackKey: surface(theme, c.dark),
    metal: metal(theme),
  };
}

// Top view of the case: front edge at y = 0, tail toward +y (becomes -z).
function caseShape(): THREE.Shape {
  const half = CASE_WIDTH / 2;
  const s = new THREE.Shape();
  s.moveTo(-half, 0);
  s.lineTo(half, 0);
  s.lineTo(half, 0.45);
  s.bezierCurveTo(half, 0.85, 0.2, 0.9, 0.05, 1.15);
  s.bezierCurveTo(-0.15, 1.45, -0.4, 1.8, -half, 1.7);
  s.lineTo(-half, 0);
  return s;
}

// Extrudes the shape upward, so shape y maps to -z.
function extrudeUp(shape: THREE.Shape, depth: number): THREE.ExtrudeGeometry {
  const geo = new THREE.ExtrudeGeometry(shape, { depth, bevelEnabled: false, curveSegments: 48 });
  geo.rotateX(-Math.PI / 2);
  return geo;
}

function mesh(geo: THREE.BufferGeometry, mat: THREE.Material | THREE.Material[]): THREE.Mesh {
  const m = new THREE.Mesh(geo, mat);
  m.castShadow = true;
  m.receiveShadow = true;
  return m;
}

export class Piano extends THREE.Group {
  static readonly LOWEST = 21; // A0
  static readonly HIGHEST = 108; // C8

  readonly lid = new THREE.Group();
  private readonly keys: THREE.Object3D[] = [];
  private readonly keyTargets: number[] = [];
  private readonly m: ReturnType<typeof createMaterials>;

  constructor(options: { theme?: Theme } = {}) {
    super();
    this.name = 'piano';
    this.m = createMaterials(options.theme ?? defaultTheme);
    this.buildCase();
    this.buildLid();
    this.buildKeyboard();
    this.buildLegs();
    this.buildPedals();
    this.buildMusicStand();
  }

  isBlack(midi: number): boolean {
    return BLACK_NOTES.has(midi % 12);
  }

  setKeyPressed(midi: number, pressed: boolean): void {
    const i = midi - Piano.LOWEST;
    if (i < 0 || i >= this.keys.length) return;
    this.keyTargets[i] = pressed ? KEY_PRESS_ANGLE : 0;
  }

  releaseAll(): void {
    this.keyTargets.fill(0);
  }

  // Eases keys toward their pressed/released angle; call once per frame.
  update(delta: number): void {
    const t = 1 - Math.exp(-KEY_SPEED * delta);
    this.keys.forEach((key, i) => {
      key.rotation.x += (this.keyTargets[i] - key.rotation.x) * t;
    });
  }

  private buildCase(): void {
    // Extrude groups: 0 = caps (top shows as the harp plate), 1 = sides.
    const body = mesh(extrudeUp(caseShape(), CASE_HEIGHT), [this.m.metal, this.m.body]);
    body.position.y = CASE_BOTTOM;
    this.add(body);
  }

  private buildLid(): void {
    const half = CASE_WIDTH / 2;
    // Hinged along the straight bass side, raised on the treble side.
    const lid = mesh(extrudeUp(caseShape(), 0.02), this.m.body);
    lid.position.x = half;
    this.lid.add(lid);
    this.lid.position.set(-half, CASE_TOP, 0);
    this.lid.rotation.z = LID_ANGLE;
    this.add(this.lid);

    const reach = 1.1; // where the prop stick meets the lid, from the hinge
    const stickLength = reach * Math.sin(LID_ANGLE);
    const stick = mesh(new THREE.CylinderGeometry(0.008, 0.008, stickLength, 8), this.m.body);
    stick.position.set(-half + reach * Math.cos(LID_ANGLE), CASE_TOP + stickLength / 2, -0.5);
    this.add(stick);
  }

  private buildKeyboard(): void {
    const whiteCount = 52;
    const keyboardWidth = whiteCount * WHITE_WIDTH;
    const depth = WHITE_LENGTH + 0.02;

    const keybed = mesh(new THREE.BoxGeometry(CASE_WIDTH, 0.07, depth), this.m.body);
    keybed.position.set(0, KEY_Y - 0.035, depth / 2);
    this.add(keybed);

    const cheekWidth = (CASE_WIDTH - keyboardWidth) / 2;
    for (const side of [-1, 1]) {
      const cheek = mesh(new THREE.BoxGeometry(cheekWidth, 0.07, depth), this.m.body);
      cheek.position.set(side * (keyboardWidth + cheekWidth) / 2, KEY_Y + 0.035, depth / 2);
      this.add(cheek);
    }

    // Pivot at the back of each key so rotation.x > 0 dips the front.
    const whiteGeo = new THREE.BoxGeometry(WHITE_WIDTH - 0.001, WHITE_HEIGHT, WHITE_LENGTH);
    whiteGeo.translate(0, WHITE_HEIGHT / 2, WHITE_LENGTH / 2);
    const blackGeo = new THREE.BoxGeometry(BLACK_WIDTH, BLACK_HEIGHT, BLACK_LENGTH);
    blackGeo.translate(0, BLACK_HEIGHT / 2, BLACK_LENGTH / 2);

    const x0 = -keyboardWidth / 2 + WHITE_WIDTH / 2;
    let whiteIndex = 0;
    for (let midi = Piano.LOWEST; midi <= Piano.HIGHEST; midi++) {
      const pivot = new THREE.Group();
      if (this.isBlack(midi)) {
        pivot.position.set(x0 + (whiteIndex - 0.5) * WHITE_WIDTH, KEY_Y + WHITE_HEIGHT - 0.01, 0);
        pivot.add(mesh(blackGeo, this.m.blackKey));
      } else {
        pivot.position.set(x0 + whiteIndex * WHITE_WIDTH, KEY_Y, 0);
        pivot.add(mesh(whiteGeo, this.m.whiteKey));
        whiteIndex++;
      }
      pivot.name = `key-${midi}`;
      this.keys.push(pivot);
      this.keyTargets.push(0);
      this.add(pivot);
    }
  }

  private buildLegs(): void {
    const legGeo = new THREE.CylinderGeometry(0.05, 0.035, CASE_BOTTOM, 16);
    for (const [x, z] of [[-0.62, -0.12], [0.62, -0.12], [-0.5, -1.5]]) {
      const leg = mesh(legGeo, this.m.body);
      leg.position.set(x, CASE_BOTTOM / 2, z);
      this.add(leg);
    }
  }

  private buildPedals(): void {
    const z = -0.3;
    const postHeight = CASE_BOTTOM - 0.08;
    const post = mesh(new THREE.BoxGeometry(0.12, postHeight, 0.04), this.m.body);
    post.position.set(0, 0.08 + postHeight / 2, z);
    this.add(post);

    const box = mesh(new THREE.BoxGeometry(0.3, 0.06, 0.12), this.m.body);
    box.position.set(0, 0.05, z);
    this.add(box);

    const pedalGeo = new THREE.BoxGeometry(0.03, 0.012, 0.12);
    for (const x of [-0.07, 0, 0.07]) {
      const pedal = mesh(pedalGeo, this.m.metal);
      pedal.position.set(x, 0.06, z + 0.1);
      this.add(pedal);
    }
  }

  private buildMusicStand(): void {
    const stand = mesh(new THREE.BoxGeometry(0.7, 0.28, 0.015), this.m.body);
    stand.position.set(0, CASE_TOP + 0.14, -0.15);
    stand.rotation.x = -0.2;
    this.add(stand);
  }
}
