import * as THREE from 'three';
import { keepSharp } from '../../effects/kuwahara';
import { defaultTheme, surface, type Theme } from '../../theme';
import { buildCrystalTree, type CrystalColors } from './crystalTreeModel';
import { mesh } from './parts';

// The crystal tree (CrystalTree.md): the user's own low poly model, built as
// its page builds it (crystalTreeModel.ts, the same tree corner for corner).
// A fantasy tree 5.6 m tall: an S-curved trunk swelling into five roots at
// the ground, five main branches forking twice, star-like clusters of
// crystal shards, blue to violet, at their ends and one on top of the crown,
// crystals hanging from the outer clusters on strings, and its patch of
// ground: a grassy disk 6.2 m across with rocks, crystals growing from it
// and tufts of grass, sparkles drifting round it all. The crystals glow in
// their own colours and light the tree round them, violet from the crown
// and blue beside it, as on the page.
//
// Units are meters, y is up and the origin is on the ground in the middle of
// the disk, by the foot of the trunk (the disk's soil edge reaches 16 cm
// below it). The tree grows from a seed, the page's own (7) unless told
// otherwise, and is the same every time.
//
// The spec gives the shape only, no behaviour, so the tree plays its page's
// own animation in update() (the hanging crystals swing, the sparkles drift
// round and twinkle), and its page's Glow slider is SetGlow(amount).

// The page's seed, and its glow (its slider's 30 of 100).
const SEED = 7;
const GLOW = 0.3;

// The page lights its crystals' glow with two point lights; three.js now
// measures a light's intensity in candela, falling off with the square of the
// distance, where the page's r128 fell off in a straight line and was
// brighter by pi. At the 1-3 m the lights reach the branches from, this many
// times the page's intensity lights them about as much.
const LIGHT_SCALE = 4;

export interface CrystalTreeOptions {
  theme?: Theme;
  // The seed the tree grows from: the same seed always grows the same tree
  // (the page's "New tree" draws a new one), 7 by default, as on the page.
  seed?: number;
  // How strongly its crystals glow, 0 to 1: the page's Glow slider, 0.3.
  glow?: number;
  // Its patch of ground: the grassy disk with its rocks, crystals and grass
  // (true by default). Without it the tree stands on whatever ground it is
  // put on.
  ground?: boolean;
  // The two coloured lights its crystals shed on it (true by default).
  lights?: boolean;
}

// The page's colours mapped onto the theme, as display (sRGB) values, the
// way the model works them. Felt is muted: it has no vivid blue or violet,
// so the crystals are its sky blue (zenith) and its lilac (flower), greyer
// than the page's, darkened to be as light as the page's blue and violet
// (0.50 and 0.49 in luminance).
function crystalColors(theme: Theme): CrystalColors {
  const display = (c: THREE.ColorRepresentation) => new THREE.Color(c).convertLinearToSRGB();
  const wood = display(theme.colors.wood);
  const light = display(theme.colors.light);
  const zenith = display(theme.scene.zenith);
  const flower = display(theme.scene.flower);
  const grass = display(theme.scene.grass);
  // Warm wood, half way to the autumn colour: the page's orange bark.
  const barkHigh = wood.clone().lerp(display(theme.scene.autumn), 0.5).multiplyScalar(1.1);
  return {
    barkLow: barkHigh.clone().multiplyScalar(0.53), // as dark as the page's is beside its light bark
    barkHigh,
    crystalBlue: zenith.clone().multiplyScalar(0.75),
    crystalViolet: flower.clone().multiplyScalar(0.83),
    tipBlue: zenith.clone().lerp(light, 0.6),
    tipViolet: flower.clone().lerp(light, 0.6),
    lilac: flower.clone().multiplyScalar(0.9),
    stone: display(theme.scene.stone).multiplyScalar(0.62), // the page's rocks are dark
    grassLow: grass,
    grassHigh: grass.clone().lerp(display(theme.scene.sand), 0.4), // the page's paler, yellower green
    soil: wood.clone().multiplyScalar(0.42),
    blade: grass.clone().multiplyScalar(0.9),
  };
}

// The sparkles' soft round dot, as the page draws it on a canvas: opaque in
// the middle, 0.6 a third of the way out, clear at the rim. Made once.
let sparkleMap: THREE.DataTexture | null = null;
function sparkleTexture(): THREE.DataTexture {
  if (!sparkleMap) {
    const n = 64;
    const data = new Uint8Array(n * n * 4);
    for (let y = 0; y < n; y++) {
      for (let x = 0; x < n; x++) {
        const d = Math.min(1, Math.hypot(x + 0.5 - n / 2, y + 0.5 - n / 2) / (n / 2));
        const alpha = d < 0.3 ? 1 - (0.4 * d) / 0.3 : 0.6 * (1 - (d - 0.3) / 0.7);
        data.set([255, 255, 255, Math.round(255 * alpha)], (y * n + x) * 4);
      }
    }
    sparkleMap = new THREE.DataTexture(data, n, n);
    sparkleMap.magFilter = THREE.LinearFilter;
    sparkleMap.minFilter = THREE.LinearFilter;
    sparkleMap.needsUpdate = true;
  }
  return sparkleMap;
}

function createMaterials(theme: Theme) {
  // The bark and the ground in the theme's finish, coloured face by face.
  const earth = surface(theme, 0xffffff);
  earth.vertexColors = true;
  earth.flatShading = true;
  earth.side = THREE.DoubleSide; // as the page: a few of its faces are wound inward
  // The crystals glossy whatever the theme's finish (as eyes are), and
  // glowing in each face's own colour: the page's emissive, times the
  // vertex colour.
  const crystal = new THREE.MeshStandardMaterial({
    vertexColors: true,
    flatShading: true,
    roughness: 0.28,
    metalness: 0.12,
    emissive: 0xffffff,
    side: THREE.DoubleSide,
  });
  crystal.onBeforeCompile = (shader) => {
    shader.fragmentShader = shader.fragmentShader.replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>\n totalEmissiveRadiance *= vColor.rgb;');
  };
  crystal.customProgramCacheKey = () => 'crystal-tree';
  // The strings: pale, a pixel wide, kept out of the painterly filter,
  // which would wipe them out (so opaque, where the page's are half clear).
  const string = new THREE.LineBasicMaterial({ color: new THREE.Color(theme.colors.light).lerp(new THREE.Color(theme.scene.zenith), 0.35) });
  keepSharp(string);
  const sparkle = new THREE.PointsMaterial({
    color: new THREE.Color(theme.colors.light).lerp(new THREE.Color(theme.scene.zenith), 0.3),
    size: 0.07,
    map: sparkleTexture(),
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  });
  return { earth, crystal, string, sparkle };
}

export class CrystalTree extends THREE.Group {
  // The glows the panel offers (SetGlow), the page's default among them.
  static readonly GLOWS = ['0', '0.15', '0.3', '0.5', '0.75', '1'];

  readonly trunk: THREE.Mesh; // the trunk, roots and branches
  readonly crystals: THREE.Mesh; // the clusters on the branches and on top of the crown
  readonly pendants: THREE.Group; // the crystals hanging on strings
  readonly ground: THREE.Group; // the grassy disk with its rocks, crystals and grass (empty without it)
  readonly sparkles: THREE.Points;
  // Triangles, counted as the page counts them.
  readonly triangles: number;

  private readonly swings: { pivot: THREE.Group; phase: number; speed: number }[] = [];
  private readonly glowLights: { light: THREE.PointLight; base: number; gain: number }[] = [];
  private readonly crystalMaterial: THREE.MeshStandardMaterial;
  private readonly sparkleMaterial: THREE.PointsMaterial;
  private glow = GLOW;
  private time = 0;

  constructor(options: CrystalTreeOptions = {}) {
    super();
    this.name = 'crystal tree';
    const theme = options.theme ?? defaultTheme;
    const m = createMaterials(theme);
    this.crystalMaterial = m.crystal;
    this.sparkleMaterial = m.sparkle;
    const model = buildCrystalTree(options.seed ?? SEED, crystalColors(theme));
    this.triangles = model.triangles;

    this.trunk = mesh(model.wood, m.earth);
    this.trunk.name = 'trunk';
    this.crystals = mesh(model.crystals, m.crystal);
    this.crystals.name = 'crystals';

    // Each hanging crystal swings about the top of its string.
    this.pendants = new THREE.Group();
    this.pendants.name = 'pendants';
    for (const p of model.pendants) {
      const pivot = new THREE.Group();
      pivot.position.copy(p.at);
      const line = new THREE.BufferGeometry();
      line.setAttribute('position', new THREE.Float32BufferAttribute([0, 0, 0, 0, -p.length, 0], 3));
      pivot.add(new THREE.LineSegments(line, m.string), mesh(p.geometry, m.crystal));
      this.pendants.add(pivot);
      this.swings.push({ pivot, phase: p.phase, speed: p.speed });
    }

    this.ground = new THREE.Group();
    this.ground.name = 'ground';
    if (options.ground !== false) this.ground.add(mesh(model.ground, m.earth), mesh(model.groundCrystals, m.crystal));

    const sparkles = new THREE.BufferGeometry();
    sparkles.setAttribute('position', new THREE.BufferAttribute(model.sparkles, 3));
    this.sparkles = new THREE.Points(sparkles, m.sparkle);
    this.sparkles.name = 'sparkles';

    this.add(this.trunk, this.crystals, this.pendants, this.ground, this.sparkles);

    // The page's two lights: violet in the crown, blue beside it.
    if (options.lights !== false) {
      const lights = [
        { color: theme.scene.flower, at: new THREE.Vector3(0, 4.2, 0), base: 0.4, gain: 1.3 },
        { color: theme.scene.zenith, at: new THREE.Vector3(1.6, 3.2, 1.8), base: 0.3, gain: 1.0 },
      ];
      for (const l of lights) {
        const light = new THREE.PointLight(l.color, 0, 8, 1);
        light.position.copy(l.at);
        this.add(light);
        this.glowLights.push({ light, base: l.base, gain: l.gain });
      }
    }

    this.SetGlow(options.glow ?? GLOW);
    this.update(0);
  }

  // How strongly the crystals glow, 0 to 1: the page's Glow slider (0.3 at
  // first). It brightens their glow, their lights and the sparkles; the
  // page's bloom has no counterpart here. Not in the spec, which gives no
  // behaviour.
  SetGlow(amount: number): void {
    const glow = THREE.MathUtils.clamp(Number.isFinite(amount) ? amount : GLOW, 0, 1);
    this.glow = glow;
    this.crystalMaterial.emissiveIntensity = 0.05 + glow * 0.5;
    for (const l of this.glowLights) l.light.intensity = (l.base + glow * l.gain) * LIGHT_SCALE;
  }

  // Moves it on `delta` seconds, as the page's loop does: the hanging
  // crystals swing a little each its own way, and the sparkles turn slowly
  // round the tree, twinkling together.
  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    for (const s of this.swings) {
      s.pivot.rotation.z = Math.sin(t * s.speed + s.phase) * 0.08;
      s.pivot.rotation.x = Math.cos(t * s.speed * 0.8 + s.phase) * 0.06;
    }
    this.sparkles.rotation.y = t * 0.05;
    this.sparkleMaterial.opacity = (0.35 + Math.sin(t * 2) * 0.12) * (0.4 + this.glow);
  }
}
