import * as THREE from 'three';
import { addBar, addBeam, addLantern, addTuft, color, frame, glowing, grain, halo, matte, mesh, PALETTE, palette, seededRandom, Shape, snowOn, woodColors, type BarColors, type Season } from './parts';

export interface LampPostOptions {
  height?: number; // m, the post over the ground; 2.55 by default
  tufts?: boolean; // grass round its foot, as the asset sheet has; true by default
  // One lantern on an arm out to one side, as the asset sheet draws it, or
  // two, one each side of a crossbar (the reference's island lamps); 1 by
  // default.
  lanterns?: 1 | 2;
  season?: Season; // spring by default; snow on it in winter
  seed?: number;
}

// The forest lake's lamp post (LampPost.md), as its asset sheet draws it: a
// square wooden post with an iron band round its foot, an arm out to one
// side (+x) near its top held by a diagonal brace, and a lantern hanging
// from the arm's end on an iron hook: a square iron frame round glowing
// yellow glass, under a pointed iron roof with a ring on top. With two
// lanterns the arm is a crossbar, braced both ways, a lantern at each end.
// The lanterns flicker a little (update()). In winter snow lies on its arm,
// its post's top and its lanterns' roofs.
export class LampPost extends THREE.Group {
  static readonly REACH = 0.78; // m from the post's middle to the lantern's

  readonly post: THREE.Mesh; // with its arm and brace
  readonly lantern: THREE.Mesh; // the iron: frame, roof, hook
  readonly glass: THREE.Mesh;
  readonly glow: THREE.Sprite;
  readonly tufts: THREE.Mesh;
  private readonly glows: THREE.Sprite[] = [];
  private time: number;

  constructor(options: LampPostOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 111);
    const height = options.height ?? 2.55;
    const P = palette(options.season);
    const sides = options.lanterns === 2 ? [1, -1] : [1];
    this.time = random() * 10;
    const up = new THREE.Vector3(0, 1, 0);

    // The post, its arm (or crossbar) and the braces under it.
    const wood = new Shape();
    addBar(wood, frame(new THREE.Vector3(0, -0.3, 0), up, new THREE.Vector3(0, 0, 1)), {
      length: height + 0.3,
      width: 0.15,
      height: 0.15,
      chamfer: 0.025,
      bevel: 0.016,
      segments: 4,
      rough: 0.004,
      slant: [0.05, -0.04],
      cracks: grain(random, [0, 1, 2, 3], [1, 2], 0.2),
      caps: [false, true],
      colors: woodColors(PALETTE.post),
      random,
    });
    const armY = height - 0.15;
    const reach = LampPost.REACH + 0.1;
    addBeam(wood, new THREE.Vector3(sides.length > 1 ? -reach : -0.06, armY, 0), new THREE.Vector3(reach, armY, 0), up, 0.12, 0.1, random);
    for (const side of sides) addBeam(wood, new THREE.Vector3(side * 0.07, armY - 0.5, 0), new THREE.Vector3(side * 0.5, armY - 0.06, 0), new THREE.Vector3(-side, 1, 0), 0.09, 0.075, random);
    // An iron band round the post's foot.
    const irons: BarColors = { side: color(PALETTE.iron), groove: color(PALETTE.iron), end: color(PALETTE.iron), core: color(PALETTE.iron) };
    addBar(wood, frame(new THREE.Vector3(0, 0.3, 0), up, new THREE.Vector3(0, 0, 1)), { length: 0.07, width: 0.165, height: 0.165, chamfer: 0.01, bevel: 0.005, segments: 1, colors: irons, random });

    // The lanterns, hanging from the arm's ends, each with its glow.
    const iron = new Shape();
    const glass = new Shape();
    for (const side of sides) {
      const glow = halo(PALETTE.glass, 1.5, 0.8);
      glow.position.copy(addLantern(iron, glass, new THREE.Vector3(side * LampPost.REACH, armY - 0.06, 0), random));
      this.glows.push(glow);
    }

    // Grass round its foot.
    const green = new Shape();
    if (options.tufts ?? true) addTuft(green, new THREE.Vector3(0, 0, 0), 12, 0.26, random, 0.09, 0, P);

    this.post = mesh(wood.geometry(), matte());
    this.lantern = mesh(iron.geometry(), matte());
    this.glass = mesh(glass.geometry(), glowing(), false);
    this.glow = this.glows[0];
    this.tufts = mesh(green.geometry(), matte());
    this.add(this.post, this.lantern, this.glass, ...this.glows, this.tufts);
    if (options.season === 'winter') snowOn(this, random);
  }

  // The flame inside flickers: the glass and its glow brighten and dim a
  // little, never quite together.
  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    const flicker = 1 + 0.06 * Math.sin(t * 7.3) + 0.04 * Math.sin(t * 17.1 + 2) + 0.03 * Math.sin(t * 31.7);
    (this.glass.material as THREE.MeshBasicMaterial).color.setScalar(flicker);
    for (const glow of this.glows) glow.material.opacity = 0.8 * flicker;
  }
}
