import * as THREE from 'three';
import { Barrel } from './Barrel';
import { Crate } from './Crate';
import { addBeam, addBlob, addIcicle, addPole, between, color, grain, matte, mesh, PALETTE, seededRandom, Shape, shade, snowOn, vary, woodColors, type Season } from './parts';

export interface WatchTowerOptions {
  season?: Season; // spring by default; snow on its roof and platform, icicles on its eaves, in winter
  seed?: number;
}

const PLATFORM = 4.4; // m, the platform's top
const EAVES = 6.35; // m, the roof's edge
const APEX = 7.6; // m, the roof's point
const FOOT = 1.35; // m from the middle to each leg at the ground
const HEAD = 1.0; // and at the roof

// The forest lake's watch tower (WatchTower.md; from the reference's "Watch
// Tower"): four wooden legs splaying out to the ground, braced crosswise on
// every side, carrying a platform of planks walled with a railing of
// upright boards, under a pointed roof of brown shingles with a flag pole
// on its tip, the flag an orange-red swallowtail waving in the wind
// (update()). A ladder leans against its front (+z), and a barrel and a
// crate stand on the platform. In winter snow lies on its roof, its
// platform and its beams, and icicles hang from its eaves.
export class WatchTower extends THREE.Group {
  static readonly PLATFORM = PLATFORM;

  readonly frame: THREE.Mesh; // the legs and braces
  readonly deck: THREE.Mesh; // the platform, railing and ladder
  readonly roof: THREE.Mesh; // with the flag pole
  readonly flag: THREE.Mesh;
  private readonly flagRest: Float32Array;
  private time = 0;

  constructor(options: WatchTowerOptions = {}) {
    super();
    const random = seededRandom(options.seed ?? 201);
    const up = new THREE.Vector3(0, 1, 0);
    const spread = (y: number) => FOOT + ((HEAD - FOOT) * (y + 0.3)) / (EAVES + 0.3);
    const corner = (sx: number, sz: number, y: number) => new THREE.Vector3(sx * spread(y), y, sz * spread(y));
    const corners: [number, number][] = [
      [-1, 1],
      [1, 1],
      [1, -1],
      [-1, -1],
    ];

    // The legs, and on each side girts across and braces crosswise.
    const frame = new Shape();
    const legs = woodColors(PALETTE.post);
    for (const [sx, sz] of corners) {
      addBeam(frame, corner(sx, sz, -0.3), corner(sx, sz, EAVES + 0.05), new THREE.Vector3(1, 0, 0), 0.2, 0.2, random, {
        colors: legs,
        cracks: grain(random, [0, 1, 2, 3], [1, 2]),
        caps: [false, true],
      });
    }
    for (let k = 0; k < 4; k++) {
      const [ax, az] = corners[k];
      const [bx, bz] = corners[(k + 1) % 4];
      const out = new THREE.Vector3(ax + bx, 0, az + bz).normalize();
      const nudge = (p: THREE.Vector3) => p.addScaledVector(out, 0.12);
      for (const y of [0.45, 2.4, PLATFORM - 0.25]) {
        addBeam(frame, nudge(corner(ax, az, y)), nudge(corner(bx, bz, y)), up, 0.13, 0.08, random, { colors: legs });
      }
      for (const [y0, y1] of [
        [0.45, 2.4],
        [2.4, PLATFORM - 0.25],
      ]) {
        const lift = 0.02;
        addBeam(frame, nudge(corner(ax, az, y0)).addScaledVector(out, lift), nudge(corner(bx, bz, y1)).addScaledVector(out, lift), out, 0.07, 0.12, random, { colors: woodColors(PALETTE.wood) });
        addBeam(frame, nudge(corner(bx, bz, y0)).addScaledVector(out, lift * 2), nudge(corner(ax, az, y1)).addScaledVector(out, lift * 2), out, 0.07, 0.12, random, { colors: woodColors(PALETTE.wood) });
      }
    }

    // The platform: joists, planks across them, and a railing of upright
    // boards round it, open at the front for the ladder; the ladder.
    const deck = new Shape();
    const edge = 1.48;
    for (const z of [-1.15, 0, 1.15]) {
      addBeam(deck, new THREE.Vector3(-edge, PLATFORM - 0.14, z), new THREE.Vector3(edge, PLATFORM - 0.14, z), up, 0.16, 0.12, random, { colors: woodColors(PALETTE.woodDark), cracks: [] });
    }
    const planks = woodColors(PALETTE.wood);
    const count = 12;
    for (let k = 0; k < count; k++) {
      const x = -edge + 0.12 + ((2 * edge - 0.24) * k) / (count - 1);
      addBeam(deck, new THREE.Vector3(x, PLATFORM - 0.0225, -edge - 0.02), new THREE.Vector3(x, PLATFORM - 0.0225, edge + 0.02), up, 0.045, 0.23, random, {
        colors: { ...planks, side: vary(planks.side, random, 1.5) },
        cracks: grain(random, [1], [0, 1]),
      });
    }
    const railTop = PLATFORM + 0.95;
    const gate = 0.36; // half the opening at the front
    for (let k = 0; k < 4; k++) {
      const [ax, az] = corners[k];
      const [bx, bz] = corners[(k + 1) % 4];
      const a = new THREE.Vector3(ax * edge, 0, az * edge);
      const b = new THREE.Vector3(bx * edge, 0, bz * edge);
      const out = new THREE.Vector3(ax + bx, 0, az + bz).normalize();
      const along = b.clone().sub(a).normalize();
      const front = out.z > 0.9;
      const boards = 16;
      for (let j = 0; j < boards; j++) {
        const at = a.clone().lerp(b, (j + 0.5) / boards);
        if (front && Math.abs(at.x) < gate) continue;
        const tall = railTop - PLATFORM + between(random, -0.04, 0.03);
        addBeam(deck, at.clone().setY(PLATFORM - 0.05), at.clone().setY(PLATFORM + tall), out, 0.03, 0.15, random, {
          colors: { ...planks, side: vary(planks.side, random, 1.6) },
          slant: [between(random, -0.3, 0.3), 0],
          cracks: grain(random, [2], [0, 1]),
          caps: [false, true],
        });
      }
      for (const y of [PLATFORM + 0.12, railTop - 0.1]) {
        const inset = out.clone().multiplyScalar(-0.06);
        if (front) {
          addBeam(deck, a.clone().add(inset).setY(y), new THREE.Vector3(-gate, y, a.z).add(inset), up, 0.1, 0.07, random, { colors: woodColors(PALETTE.post) });
          addBeam(deck, new THREE.Vector3(gate, y, a.z).add(inset), b.clone().add(inset).setY(y), up, 0.1, 0.07, random, { colors: woodColors(PALETTE.post) });
        } else {
          addBeam(deck, a.clone().add(inset).setY(y).addScaledVector(along, -0.05), b.clone().add(inset).setY(y).addScaledVector(along, 0.05), up, 0.1, 0.07, random, { colors: woodColors(PALETTE.post) });
        }
      }
    }
    const ladderFoot = 2.15;
    for (const x of [-0.27, 0.27]) {
      addBeam(deck, new THREE.Vector3(x, -0.1, ladderFoot), new THREE.Vector3(x, railTop, edge + 0.05), new THREE.Vector3(0, 0, 1), 0.07, 0.09, random, { colors: woodColors(PALETTE.post) });
    }
    for (let y = 0.3; y < PLATFORM + 0.6; y += 0.32) {
      const t = (y + 0.1) / (railTop + 0.1);
      const z = ladderFoot + (edge + 0.05 - ladderFoot) * t;
      addBeam(deck, new THREE.Vector3(-0.3, y, z), new THREE.Vector3(0.3, y, z), up, 0.05, 0.06, random, { colors: woodColors(PALETTE.wood), cracks: [] });
    }

    // The roof: four faces of shingles in overlapping rows, over a dark
    // underside, edged with boards; the flag pole on its tip.
    const roof = new Shape();
    const eave = 1.85;
    const apex = new THREE.Vector3(0, APEX, 0);
    const shingles = color(0x8a4f2b);
    const rows = 6;
    for (let k = 0; k < 4; k++) {
      const [ax, az] = corners[k];
      const [bx, bz] = corners[(k + 1) % 4];
      const a = new THREE.Vector3(ax * eave, EAVES, az * eave);
      const b = new THREE.Vector3(bx * eave, EAVES, bz * eave);
      const out = new THREE.Vector3(ax + bx, 0, az + bz).normalize();
      for (let r = 0; r < rows; r++) {
        const t0 = r / rows;
        const t1 = (r + 1) / rows;
        // Each row's foot hangs a little lower and further out than the top
        // of the row under it, so the rows overlap like shingles.
        const lip = out.clone().multiplyScalar(0.035).add(new THREE.Vector3(0, -0.03, 0));
        const a0 = a.clone().lerp(apex, t0).add(lip);
        const b0 = b.clone().lerp(apex, t0).add(lip);
        const a1 = a.clone().lerp(apex, t1);
        const b1 = b.clone().lerp(apex, t1);
        const tone = vary(shade(shingles, r % 2 ? 1.06 : 0.94), random, 0.8);
        // The corners run counter-clockwise seen from above, so (a0, b0,
        // b1, a1) faces out and up.
        roof.quad(a0, b0, b1, a1, tone);
        // The row's foot, a thin edge facing out and down.
        const a0in = a0.clone().sub(lip).add(new THREE.Vector3(0, -0.04, 0));
        const b0in = b0.clone().sub(lip).add(new THREE.Vector3(0, -0.04, 0));
        roof.quad(a0, a0in, b0in, b0, shade(tone, 0.7));
      }
      // The underside, dark.
      roof.triangle(a, new THREE.Vector3(0, EAVES + 0.5, 0), b, shade(shingles, 0.45));
      addBeam(roof, a.clone().add(new THREE.Vector3(0, -0.06, 0)), b.clone().add(new THREE.Vector3(0, -0.06, 0)), out, 0.04, 0.12, random, { colors: woodColors(PALETTE.woodDark), cracks: [] });
    }
    const poleTop = APEX + 1.45;
    addPole(roof, new THREE.Vector3(0, APEX - 0.3, 0), new THREE.Vector3(0, poleTop, 0), 0.035, random, 0.8, PALETTE.post, 6);
    addBlob(roof, new THREE.Vector3(0, poleTop + 0.03, 0), new THREE.Vector3(0.05, 0.05, 0.05), 0, 0.1, random, () => color(PALETTE.iron));

    // The flag: a swallowtail, its cloth a grid waved by update().
    const cols = 9;
    const flagRows = 4;
    const flagLength = 0.95;
    const flagHeight = 0.5;
    const positions: number[] = [];
    const colors: number[] = [];
    const cloth = color(PALETTE.flag);
    for (let j = 0; j <= flagRows; j++) {
      for (let i = 0; i <= cols; i++) {
        const u = i / cols;
        const v = j / flagRows;
        // The tail's notch: the middle of the fly end is cut back.
        const notch = 0.28 * Math.max(0, u - 0.6) / 0.4 * (1 - Math.abs(v - 0.5) * 2);
        positions.push(0.03 + flagLength * (u - notch), poleTop - 0.12 - flagHeight * v, 0);
        const c = vary(cloth, random, 0.5).multiplyScalar(1.06 - 0.12 * u);
        colors.push(c.r, c.g, c.b);
      }
    }
    const index: number[] = [];
    for (let j = 0; j < flagRows; j++) {
      for (let i = 0; i < cols; i++) {
        const a = j * (cols + 1) + i;
        index.push(a, a + cols + 1, a + 1, a + 1, a + cols + 1, a + cols + 2);
      }
    }
    const flagGeometry = new THREE.BufferGeometry();
    flagGeometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    flagGeometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
    flagGeometry.setIndex(index);
    flagGeometry.computeVertexNormals();
    this.flagRest = new Float32Array(positions);
    const flagMaterial = new THREE.MeshStandardMaterial({ vertexColors: true, roughness: 0.85, side: THREE.DoubleSide });
    flagMaterial.name = 'flag';

    this.frame = mesh(frame.geometry(), matte());
    this.deck = mesh(deck.geometry(), matte());
    this.roof = mesh(roof.geometry(), matte());
    this.flag = mesh(flagGeometry, flagMaterial);
    this.add(this.frame, this.deck, this.roof, this.flag);

    // A barrel and a crate on the platform.
    const barrel = new Barrel({ height: 0.72, seed: 7, season: options.season });
    barrel.position.set(-0.85, PLATFORM, -0.8);
    const crate = new Crate({ size: 0.5, seed: 8, season: options.season });
    crate.position.set(0.8, PLATFORM, -0.85);
    crate.rotation.y = 0.3;
    this.add(barrel, crate);
    if (options.season === 'winter') {
      snowOn(this.frame, random);
      snowOn(this.deck, random);
      snowOn(this.roof, random, 0.2, 0.45);
      // Icicles along the eaves.
      const ice = new Shape();
      for (let k = 0; k < 4; k++) {
        const [ax, az] = corners[k];
        const [bx, bz] = corners[(k + 1) % 4];
        for (let j = 0; j < 7; j++) {
          if (random() < 0.3) continue;
          const t = (j + 0.5) / 7;
          const at = new THREE.Vector3(ax * eave + (bx - ax) * eave * t, EAVES - 0.08, az * eave + (bz - az) * eave * t);
          addIcicle(ice, at, between(random, 0.12, 0.3), 0.025, random);
        }
      }
      this.add(mesh(ice.geometry(), matte(), false));
    }
  }

  // The flag waves: waves run along it from the pole, bigger toward its
  // free end, with a slower swell under them.
  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    const position = this.flag.geometry.getAttribute('position') as THREE.BufferAttribute;
    for (let i = 0; i < position.count; i++) {
      const x = this.flagRest[i * 3];
      const y = this.flagRest[i * 3 + 1];
      const reach = Math.max(0, x - 0.03) / 0.95;
      const wave = Math.sin(x * 7 - t * 6.5 + y * 1.5) * 0.09 + Math.sin(x * 3.1 - t * 2.3) * 0.05;
      position.setXYZ(i, x - reach * 0.04 * (1 + Math.sin(t * 2.3)), y - reach * reach * 0.05, wave * reach);
    }
    position.needsUpdate = true;
    this.flag.geometry.computeVertexNormals();
  }
}
