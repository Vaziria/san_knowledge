import * as THREE from 'three';
import { addBar, addBeam, addExtrusion, addFlower, addLantern, addTube, addTuft, between, color, frame, glowing, grain, halo, matte, mesh, PALETTE, palette, seededRandom, Shape, shade, snowOn, woodColors, type BarColors, type Season } from './parts';

export interface SignPostOptions {
  // The way each board points, from the top down: right (+x) or left (-x).
  // Two boards, both pointing right, by default, as the asset sheet draws it.
  boards?: readonly ('right' | 'left')[];
  dressing?: boolean; // flowers and grass round its foot, as the asset sheet has; true by default
  // A lantern hanging from an arm at its top, out to its left (-x), as the
  // reference's close views draw it; false by default.
  lantern?: boolean;
  season?: Season; // spring by default; snow on it in winter
  seed?: number;
}

// The forest lake's sign post (SignPost.md), as its asset sheet draws it: a
// square wooden post with arrow-shaped boards nailed across its front, each
// a little askew, with flowers and grass round its foot; some carry a
// lantern on an arm at their top. The lantern flickers (update()). In
// winter snow lies on the boards' and the post's tops.
export class SignPost extends THREE.Group {
  readonly post: THREE.Mesh; // with its arm
  readonly boards: THREE.Mesh; // with their nails
  readonly dressing: THREE.Mesh;
  readonly glass: THREE.Mesh | null; // the lantern's, with its iron in `boards`
  private readonly glow: THREE.Sprite | null;
  private time = 0;

  constructor(options: SignPostOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 121);
    const boards = options.boards ?? ['right', 'right'];
    const up = new THREE.Vector3(0, 1, 0);
    const height = options.lantern ? 1.95 : 1.62;
    const P = palette(options.season);

    const wood = new Shape();
    addBar(wood, frame(new THREE.Vector3(0, -0.25, 0), up, new THREE.Vector3(0, 0, 1)), {
      length: height + 0.25,
      width: 0.12,
      height: 0.12,
      chamfer: 0.02,
      bevel: 0.014,
      segments: 3,
      rough: 0.003,
      slant: [0.06, 0.04],
      cracks: grain(random, [0, 1, 2, 3], [1, 2], 0.2),
      caps: [false, true],
      colors: woodColors(PALETTE.post),
      random,
    });

    // The boards: arrows cut from planks, nailed to the post's front.
    const signs = new Shape();
    const face = shade(color(PALETTE.wood), 1.12);
    const side = color(PALETTE.post);
    const nails: BarColors = { side: color(PALETTE.nail), groove: color(PALETTE.nail), end: color(PALETTE.nail), core: color(PALETTE.nail) };
    const top = options.lantern ? 1.62 : height;
    boards.forEach((way, k) => {
      const y = top - 0.26 - k * 0.27;
      const length = between(random, 0.6, 0.7);
      const tail = -0.15;
      const h = 0.09;
      const outline = [
        new THREE.Vector2(tail, -h),
        new THREE.Vector2(length - 0.12, -h),
        new THREE.Vector2(length, 0),
        new THREE.Vector2(length - 0.12, h),
        new THREE.Vector2(tail, h),
      ].map((p) => (way === 'left' ? new THREE.Vector2(-p.x, p.y) : p));
      const matrix = new THREE.Matrix4().makeRotationZ(between(random, -0.06, 0.06)).setPosition(0, y, 0.06 + 0.021);
      addExtrusion(signs, outline, 0.04, matrix, { face: shade(face, between(random, 0.94, 1.04)), side });
      for (const dx of [-0.035, 0.035]) {
        // From inside the board (its face is 2 cm out), standing 4 mm proud.
        const head = new THREE.Vector3(dx, (random() - 0.5) * 0.04, 0.012).applyMatrix4(matrix);
        addBar(signs, frame(head, new THREE.Vector3(0, 0, 1), up), { length: 0.012, width: 0.016, height: 0.016, chamfer: 0.004, bevel: 0.002, segments: 1, caps: [false, true], colors: nails, random });
      }
    });

    // The lantern on its arm, braced, out to the left.
    let glassShape: Shape | null = null;
    this.glow = null;
    if (options.lantern) {
      const armY = height - 0.12;
      addBeam(wood, new THREE.Vector3(0.05, armY, 0), new THREE.Vector3(-0.62, armY, 0), up, 0.1, 0.085, random);
      addBeam(wood, new THREE.Vector3(-0.05, armY - 0.38, 0), new THREE.Vector3(-0.38, armY - 0.05, 0), new THREE.Vector3(1, 1, 0), 0.07, 0.065, random);
      glassShape = new Shape();
      const glow = halo(PALETTE.glass, 1.3, 0.75);
      glow.position.copy(addLantern(signs, glassShape, new THREE.Vector3(-0.52, armY - 0.05, 0), random));
      this.glow = glow;
    }

    const green = new Shape();
    if (options.dressing ?? true) {
      addTuft(green, new THREE.Vector3(0, 0, 0), 11, 0.22, random, 0.08, 0, P);
      for (const [x, z, petal] of [
        [-0.28, 0.12, P.petalLight],
        [0.26, 0.18, P.petalPink],
        [0.2, -0.15, P.petalYellow],
      ] as const) {
        const foot = new THREE.Vector3(x, -0.01, z);
        const head = foot.clone().add(new THREE.Vector3(0, between(random, 0.16, 0.26), 0));
        addTube(green, [foot, head], [0.005, 0.004], { sides: 3, random, paint: () => color(P.grass) });
        addFlower(green, head, new THREE.Vector3(x, 1.5, z), 6, 0.045, color(petal), color(P.flowerHeart), random);
        addTuft(green, foot, 5, 0.14, random, 0, 0, P);
      }
    }

    this.post = mesh(wood.geometry(), matte());
    this.boards = mesh(signs.geometry(), matte());
    this.dressing = mesh(green.geometry(), matte());
    this.glass = glassShape ? mesh(glassShape.geometry(), glowing(), false) : null;
    this.add(this.post, this.boards, this.dressing);
    if (this.glass && this.glow) this.add(this.glass, this.glow);
    if (options.season === 'winter') snowOn(this, random);
  }

  // The lantern's flame flickers.
  update(delta: number): void {
    if (!this.glass || !this.glow) return;
    this.time += delta;
    const t = this.time;
    const flicker = 1 + 0.06 * Math.sin(t * 7.1 + 1) + 0.04 * Math.sin(t * 16.3) + 0.03 * Math.sin(t * 29.9 + 2);
    (this.glass.material as THREE.MeshBasicMaterial).color.setScalar(flicker);
    this.glow.material.opacity = 0.75 * flicker;
  }
}
