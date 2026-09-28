import * as THREE from 'three';
import { faceFrame, joint } from './ExplorerBody';
import { decal, into, lay, oval, slab, strip, U } from './ExplorerKit';
import { blend, rigidMesh, Sculpt, tone, type Palette } from './parts';

// The explorer's face (Explorer.ts): its eyes, brows and mouth, laid onto
// the skin of its head as the sheet paints them, one mesh for each feature
// that changes, so that an expression shows some and hides others and
// turns the brows. Built where the face stands in the figure, then moved
// into the head's space.
//
// The sheet's six expressions, named from what they show: `happy` (eyes
// open, a closed smile), `laugh` (eyes shut in arcs, mouth wide open),
// `surprised` (one eye squeezed shut, brows up, mouth open in an O),
// `angry` (brows down at the middle, a frown), `wink` (one eye shut under a
// lowered brow, mouth flat) and `smirk` (a lopsided smile, eyes glancing
// aside). The shut eye is on the figure's left (-x), as the sheet draws it.

type Eye = 'open' | 'shut' | 'arc';
type Mouth = 'smile' | 'laugh' | 'o' | 'frown' | 'flat' | 'smirk';

interface Expression {
  eyes: readonly [Eye, Eye]; // left (-x), right
  brows: readonly [readonly [number, number], readonly [number, number]]; // px up, rad turned (its inner end up > 0 on the right brow)
  mouth: Mouth;
  look?: number; // px the irises move along x
}

export const EXPRESSIONS: Readonly<Record<string, Expression>> = {
  happy: { eyes: ['open', 'open'], brows: [[0, 0], [0, 0]], mouth: 'smile' },
  laugh: { eyes: ['arc', 'arc'], brows: [[2, 0.05], [2, -0.05]], mouth: 'laugh' },
  surprised: { eyes: ['shut', 'open'], brows: [[3, 0.1], [4, -0.14]], mouth: 'o' },
  angry: { eyes: ['open', 'open'], brows: [[-2, -0.38], [-2, 0.38]], mouth: 'frown' },
  wink: { eyes: ['shut', 'open'], brows: [[-3, -0.22], [0.5, 0.1]], mouth: 'flat' },
  smirk: { eyes: ['open', 'open'], brows: [[1.5, 0.06], [-0.5, 0.04]], mouth: 'smirk', look: -2.2 },
};

const EYE_X = 21; // px either side of the middle
const EYE_Y = 330; // px up
const BROW_Y = 351;
const MOUTH_Y = 305;

export class ExplorerFace {
  readonly group = new THREE.Group();
  private readonly whites: THREE.Mesh[] = [];
  private readonly irises: THREE.Mesh[] = [];
  private readonly shut: THREE.Mesh[] = [];
  private readonly arcs: THREE.Mesh[] = [];
  private readonly brows: THREE.Group[] = [];
  private readonly mouths = new Map<Mouth, THREE.Mesh>();
  private shown = '';

  // `skin` is the bare skin of the head to lay the face onto; `keep`
  // registers each mesh to be painted with the figure's colours.
  constructor(palette: Palette, skin: Sculpt, random: () => number, materials: { coat: THREE.Material; gloss: THREE.Material }, keep: (mesh: THREE.Mesh) => THREE.Mesh) {
    const head = joint('head');
    const f = faceFrame();
    const mesh = (s: Sculpt, material: THREE.Material) => {
      const m = keep(rigidMesh(into(s.geometry(palette), head), material, false));
      this.group.add(m);
      return m;
    };
    for (const side of [-1, 1]) {
      const cx = side * EYE_X;
      // The white, with a thick dark lash line over its top that flicks
      // out at its outer corner.
      const white = new Sculpt(random, 0.01);
      decal(white, skin, f, oval(cx, EYE_Y, 9.6, 10.8, 12), tone('sclera'), 0.001);
      const lash: [number, number][] = [];
      for (let i = 0; i <= 8; i++) {
        const a = Math.PI * (1.12 - (1.24 * i) / 8); // from the left side over the top to the right
        lash.push([cx + 10.2 * Math.cos(a), EYE_Y + 0.8 + 11 * Math.sin(a)]);
      }
      const outer = side < 0 ? 0 : lash.length - 1;
      const [ou, ov] = lash[outer];
      if (side < 0) lash.unshift([ou - 2.6, ov + 1.8]);
      else lash.push([ou + 2.6, ov + 1.8]);
      // Thickest toward its outer end, as the sheet paints it.
      strip(white, skin, f, lash, (share) => {
        const toOuter = side < 0 ? 1 - share : share;
        return 1.6 + 2.6 * Math.sin(Math.PI * Math.min(1, 0.15 + toOuter * 0.95));
      }, tone('lash'), 0.0017);
      this.whites.push(mesh(white, materials.coat));

      // The iris, dark brown and big, lighter at its foot, a darker pupil,
      // and a white glint up to the right (a small one down to the left).
      const iris = new Sculpt(random, 0.01);
      const ix = cx - side * 1.1;
      decal(iris, skin, f, oval(ix, EYE_Y - 0.2, 7.9, 10.3, 10), tone('iris'), 0.0013);
      decal(iris, skin, f, oval(ix, EYE_Y - 6.4, 5.4, 2.6, 6), blend('iris', 'shine', 0.28), 0.0016);
      decal(iris, skin, f, oval(ix, EYE_Y + 0.3, 4.7, 6.2, 8), tone('pupil'), 0.0019);
      decal(iris, skin, f, oval(ix + 2.8, EYE_Y + 3.9, 2.6, 2.8, 5), tone('shine'), 0.0023);
      this.irises.push(mesh(iris, materials.gloss));

      // Shut: a lid's dark line curving down a little, a lash at its outer
      // end.
      const shut = new Sculpt(random, 0.01);
      const line: [number, number][] = [];
      for (let i = 0; i <= 6; i++) {
        const t = -1 + (2 * i) / 6;
        line.push([cx + 9 * t, EYE_Y - 1.5 + 1.8 * t * t]);
      }
      if (side < 0) line.unshift([cx - 11, EYE_Y + 1.6]);
      else line.push([cx + 11, EYE_Y + 1.6]);
      strip(shut, skin, f, line, () => 2.4, tone('lash'), 0.0012);
      this.shut.push(mesh(shut, materials.coat));

      // Laughing: shut in an arc, like ^.
      const arc = new Sculpt(random, 0.01);
      const bow: [number, number][] = [];
      for (let i = 0; i <= 8; i++) {
        const a = Math.PI * (1.1 - (1.2 * i) / 8);
        bow.push([cx + 7.8 * Math.cos(a), EYE_Y - 3 + 6.5 * Math.sin(a)]);
      }
      strip(arc, skin, f, bow, () => 2.6, tone('lash'), 0.0012);
      this.arcs.push(mesh(arc, materials.coat));

      // The brow: a thick dark bar, arched a little, thicker at its inner
      // end, standing proud of the skin so it can turn.
      const brow = new Sculpt(random, 0.01);
      const bar: [number, number][] = [];
      for (let i = 0; i <= 6; i++) {
        const t = -1 + (2 * i) / 6; // inner end first
        bar.push([side * (21 + 11 * t), BROW_Y + 1.4 * (1 - t * t)]);
      }
      slab(brow, skin, f, bar, (share) => 4.4 - 1.4 * share, tone('brow'), tone('brow', 0.8), 0.0008, 0.0014);
      const pivot = lay(skin, f, side * 21, BROW_Y, 0.0008).sub(head);
      const group = new THREE.Group();
      group.position.copy(pivot);
      group.userData.rest = pivot.y;
      const browMesh = keep(rigidMesh(into(brow.geometry(palette), head).translate(-pivot.x, -pivot.y, -pivot.z), materials.coat, false));
      group.add(browMesh);
      this.group.add(group);
      this.brows.push(group);
    }

    // The mouths.
    const smile = new Sculpt(random, 0.01);
    strip(smile, skin, f, curve(-10, 10, 8, (u) => MOUTH_Y - 1 + 2.6 * (u / 10) ** 2), (s) => 2 - 0.8 * Math.abs(2 * s - 1), tone('mouth'), 0.001);
    this.mouths.set('smile', mesh(smile, materials.coat));

    const laugh = new Sculpt(random, 0.01);
    const top = curve(-10.5, 10.5, 8, (u) => MOUTH_Y + 2 - 0.8 * (1 - (u / 10.5) ** 2));
    const open: [number, number][] = [...top];
    for (let i = 1; i < 8; i++) {
      const a = (Math.PI * i) / 8;
      open.push([10.5 * Math.cos(a), MOUTH_Y + 1.6 - 10 * Math.sin(a)]);
    }
    decal(laugh, skin, f, open, tone('mouthIn'), 0.001);
    strip(laugh, skin, f, curve(-8.5, 8.5, 6, (u) => MOUTH_Y + 0.6 - 0.6 * (1 - (u / 8.5) ** 2)), () => 2.6, tone('teeth'), 0.0015);
    decal(laugh, skin, f, oval(0, MOUTH_Y - 5.6, 5.4, 2.6, 10), tone('tongue'), 0.0015);
    this.mouths.set('laugh', mesh(laugh, materials.coat));

    const o = new Sculpt(random, 0.01);
    decal(o, skin, f, oval(0, MOUTH_Y - 2.5, 5.2, 5.8, 12), tone('mouthIn'), 0.001);
    strip(o, skin, f, curve(-3.6, 3.6, 4, () => MOUTH_Y + 1.8), () => 1.8, tone('teeth'), 0.0015);
    decal(o, skin, f, oval(0, MOUTH_Y - 6, 3.2, 1.3, 8), tone('tongue'), 0.0015);
    this.mouths.set('o', mesh(o, materials.coat));

    const frown = new Sculpt(random, 0.01);
    strip(frown, skin, f, curve(-6.5, 6.5, 6, (u) => MOUTH_Y + 0.2 - 2 * (u / 6.5) ** 2), (s) => 1.9 - 0.6 * Math.abs(2 * s - 1), tone('mouth'), 0.001);
    this.mouths.set('frown', mesh(frown, materials.coat));

    const flat = new Sculpt(random, 0.01);
    strip(flat, skin, f, curve(-6, 6, 4, () => MOUTH_Y - 0.5), (s) => 1.7 - 0.5 * Math.abs(2 * s - 1), tone('mouth'), 0.001);
    this.mouths.set('flat', mesh(flat, materials.coat));

    const smirk = new Sculpt(random, 0.01);
    strip(smirk, skin, f, curve(-9, 8.5, 8, (u) => MOUTH_Y - 1 + 0.16 * u + 1.6 * (u / 9) ** 2), (s) => 2 - 0.8 * Math.abs(2 * s - 1), tone('mouth'), 0.001);
    decal(smirk, skin, f, oval(1, MOUTH_Y - 3.2, 3.6, 1.2, 8), blend('mouth', 'blush', 0.55), 0.0009);
    this.mouths.set('smirk', mesh(smirk, materials.coat));

    this.show('happy', false);
  }

  // Shows an expression; `blink` shuts the open eyes for a moment.
  show(name: string, blink: boolean): void {
    const key = `${name}:${blink}`;
    if (key === this.shown) return;
    this.shown = key;
    const e = EXPRESSIONS[name] ?? EXPRESSIONS.happy;
    for (let i = 0; i < 2; i++) {
      const eye = e.eyes[i] === 'open' && blink ? 'shut' : e.eyes[i];
      this.whites[i].visible = eye === 'open';
      this.irises[i].visible = eye === 'open';
      // Glancing aside moves the iris across the eye, and out a little as
      // it goes: toward the middle of the face the skin comes forward.
      this.irises[i].position.x = (e.look ?? 0) * U;
      this.irises[i].position.z = Math.abs(e.look ?? 0) * 0.65 * U;
      this.shut[i].visible = eye === 'shut';
      this.arcs[i].visible = eye === 'arc';
      const [up, turn] = e.brows[i];
      this.brows[i].position.y = (this.brows[i].userData.rest as number) + up * U;
      this.brows[i].rotation.z = turn;
    }
    for (const [mouth, m] of this.mouths) m.visible = mouth === e.mouth;
  }
}

// Points along u from `a` to `b`, `n` steps, at heights v(u).
function curve(a: number, b: number, n: number, v: (u: number) => number): [number, number][] {
  return Array.from({ length: n + 1 }, (_, i) => {
    const u = a + ((b - a) * i) / n;
    return [u, v(u)] as [number, number];
  });
}
