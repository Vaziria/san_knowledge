import * as THREE from 'three';
import { addBlob, addHull, between, seededRandom, Shape, type Season } from '../../figures/ForestLake/parts';
import { SUN } from '../../sun';

// The forest lake's sky and what stands against it (ForestLake.md), as the
// reference paints them: a bright blue sky, pale at the horizon and deep
// overhead, with the sun's glow; big white clouds, blue-grey underneath,
// drifting slowly; forested hills in a ring round the valley, and beyond
// them snow-capped mountains, tallest to the north, all hazed blue with
// distance; and land under them out to the horizon. None of it is lit or
// fogged by the scene: its light and haze are painted in. Each season
// colours it as the reference's seasons do: the hills deep green in summer,
// orange and brown in autumn under a warmer haze, and in winter white with
// snow, the mountains snowy to their feet, under a paler sky.
interface SkyColors {
  horizon: number; // the haze everything fades into
  middle: number;
  zenith: number;
  forest: number; // the hills' forest
  trees: number | null; // dark trees showing through the hills' snow, in winter
  mountain: number; // the mountains' lower slopes
  ground: number; // the land out to the horizon
  snowLine: number; // how far up the mountains the snow starts, 0..1
  cloudUnder: number;
}
const SKY: Record<Season, SkyColors> = {
  spring: { horizon: 0xd4ecf9, middle: 0x86c4ef, zenith: 0x3f93dd, forest: 0x4f8a5a, trees: null, mountain: 0x5f8b76, ground: 0x6f9e6b, snowLine: 0.62, cloudUnder: 0xbfd3e6 },
  summer: { horizon: 0xcfe9fb, middle: 0x7cbef0, zenith: 0x2f86d8, forest: 0x3c7a47, trees: null, mountain: 0x557f68, ground: 0x5b8f58, snowLine: 0.74, cloudUnder: 0xc2d6ea },
  autumn: { horizon: 0xebe4d3, middle: 0x9cc6e6, zenith: 0x4a8dcf, forest: 0xac7440, trees: null, mountain: 0x8a7a5e, ground: 0xa88a55, snowLine: 0.55, cloudUnder: 0xd8d0c6 },
  winter: { horizon: 0xe3edf6, middle: 0xb4d3ec, zenith: 0x6ea4d6, forest: 0xdce5ed, trees: 0x557466, mountain: 0xb3c1ce, ground: 0xe6edf3, snowLine: 0.18, cloudUnder: 0xc3cbd6 },
};

// The haze at the horizon in a season: the sky's colour there, the fog's
// and the background's.
export function horizonOf(season: Season): THREE.Color {
  return new THREE.Color(SKY[season].horizon);
}
// Spring's, for what has no season.
export const HORIZON = horizonOf('spring');

export class Backdrop extends THREE.Group {
  readonly clouds: THREE.Group;
  private readonly drift: { cloud: THREE.Object3D; speed: number }[] = [];
  private readonly colors: SkyColors;

  constructor(season: Season = 'spring') {
    super();
    const random = seededRandom(501);
    this.colors = SKY[season];
    this.add(sky(this.colors));
    this.add(this.hills(random));
    this.clouds = new THREE.Group();
    this.add(this.clouds);
    this.addClouds(random);
  }

  // Hills and mountains, and the land out to the horizon, in two rings.
  private hills(random: () => number): THREE.Mesh {
    const shape = new Shape();
    const sun = SUN.clone().normalize();
    const horizon = new THREE.Color(this.colors.horizon);
    const haze = (c: THREE.Color, distance: number) => c.lerp(horizon, Math.min(0.78, 0.18 + distance / 1900));
    const trees = this.colors.trees === null ? null : new THREE.Color(this.colors.trees);
    const peak = (x: number, z: number, radius: number, height: number, snowy: boolean, forest: THREE.Color) => {
      const points: THREE.Vector3[] = [];
      const base = 9 + Math.floor(random() * 3);
      for (let k = 0; k < base; k++) {
        const a = (k / base) * Math.PI * 2 + random() * 0.4;
        const r = radius * between(random, 0.75, 1.15);
        points.push(new THREE.Vector3(x + Math.cos(a) * r, -10, z + Math.sin(a) * r));
      }
      // A ridge of shoulders up to the summit.
      for (let k = 0; k < 5; k++) {
        const a = random() * Math.PI * 2;
        const f = between(random, 0.35, 0.7);
        const r = radius * (1 - f) * between(random, 0.6, 1.0);
        points.push(new THREE.Vector3(x + Math.cos(a) * r, height * f * between(random, 0.85, 1.1), z + Math.sin(a) * r));
      }
      points.push(new THREE.Vector3(x + between(random, -0.1, 0.1) * radius, height, z + between(random, -0.1, 0.1) * radius));
      const distance = Math.hypot(x, z);
      const snow = new THREE.Color(0xf6f9fd);
      const rock = new THREE.Color(0x8d89a6);
      const snowLine = height * between(random, this.colors.snowLine - 0.06, this.colors.snowLine + 0.06);
      addHull(shape, points, (normal, middle) => {
        const lit = 0.7 + 0.32 * Math.max(0, normal.dot(sun)) + 0.08 * normal.y;
        const high = middle.y / height;
        let c: THREE.Color;
        if (snowy && middle.y > snowLine + (random() - 0.5) * height * 0.12) c = snow.clone();
        else if (snowy && high > 0.3) c = rock.clone().lerp(forest, 0.2);
        else if (!snowy && trees && random() < 0.35) c = trees.clone();
        else c = forest.clone().lerp(rock, snowy ? 0.25 : 0.1);
        return haze(c.multiplyScalar(lit), distance);
      });
    };
    // Forested hills close round the valley, all the way round.
    const forest = new THREE.Color(this.colors.forest);
    for (let k = 0; k < 26; k++) {
      const a = (k / 26) * Math.PI * 2 + random() * 0.15;
      const d = between(random, 360, 520);
      peak(Math.cos(a) * d, Math.sin(a) * d, between(random, 110, 190), between(random, 55, 120), false, forest.clone().offsetHSL(0, 0, (random() - 0.5) * 0.06));
    }
    // Snowy mountains beyond, tallest to the north (-z), behind the
    // waterfall, lower and fewer to the south.
    for (let k = 0; k < 22; k++) {
      const a = (k / 22) * Math.PI * 2 + random() * 0.2;
      const north = Math.max(0, -Math.sin(a));
      const d = between(random, 720, 1050);
      const height = between(random, 180, 260) + north * between(random, 150, 260);
      peak(Math.cos(a) * d, Math.sin(a) * d, height * between(random, 0.75, 1.0), height, true, new THREE.Color(this.colors.mountain));
    }
    // The land out to the horizon, under the hills, hazed.
    const ground = new THREE.Color(this.colors.ground);
    const rings = [280, 450, 700, 1100, 1700];
    const around = 48;
    for (let r = 0; r + 1 < rings.length; r++) {
      for (let k = 0; k < around; k++) {
        const a0 = (k / around) * Math.PI * 2;
        const a1 = ((k + 1) / around) * Math.PI * 2;
        const [r0, r1] = [rings[r], rings[r + 1]];
        const point = (radius: number, a: number) => new THREE.Vector3(Math.cos(a) * radius, 6 + (radius > 300 ? 4 : 0), Math.sin(a) * radius);
        const c0 = haze(ground.clone(), r0);
        const c1 = haze(ground.clone(), r1);
        // Counter-clockwise seen from above: decreasing angle.
        shape.quad(point(r0, a1), point(r1, a1), point(r1, a0), point(r0, a0), c0, c1, c1, c0);
      }
    }
    const material = new THREE.MeshBasicMaterial({ vertexColors: true, fog: false });
    material.name = 'backdrop';
    const hills = new THREE.Mesh(shape.geometry(), material);
    hills.frustumCulled = false;
    hills.renderOrder = -5;
    return hills;
  }

  // Big clouds of white puffs, blue-grey underneath, in a ring well out,
  // more of them over the mountains to the north.
  private addClouds(random: () => number): void {
    const material = new THREE.MeshBasicMaterial({ vertexColors: true, fog: false });
    material.name = 'clouds';
    const top = new THREE.Color(0xffffff);
    const side = new THREE.Color(0xeef5fc);
    const under = new THREE.Color(this.colors.cloudUnder);
    for (let k = 0; k < 26; k++) {
      const shape = new Shape();
      const size = between(random, 22, 42);
      const puffs = 5 + Math.floor(random() * 4);
      for (let p = 0; p < puffs; p++) {
        const along = (p / (puffs - 1) - 0.5) * size * 2.6;
        const r = size * between(random, 0.55, 1) * (1 - Math.abs(along) / (size * 2.2));
        const center = new THREE.Vector3(along, r * 0.35 + between(random, -0.1, 0.25) * size, between(random, -0.3, 0.3) * size);
        addBlob(shape, center, new THREE.Vector3(r, r * 0.78, r * 0.9), 1, 0.18, random, (n) => {
          if (n.y < -0.25) return under.clone();
          return n.y > 0.35 ? top.clone() : side.clone().lerp(under, Math.max(0, -n.y) * 1.5);
        });
      }
      const cloud = new THREE.Mesh(shape.geometry(), material);
      const a = random() < 0.6 ? between(random, Math.PI * 1.05, Math.PI * 1.95) : random() * Math.PI * 2;
      const d = between(random, 480, 1150);
      cloud.position.set(Math.cos(a) * d, between(random, 150, 330), Math.sin(a) * d);
      cloud.rotation.y = random() * Math.PI;
      cloud.frustumCulled = false;
      cloud.renderOrder = -4;
      this.clouds.add(cloud);
      this.drift.push({ cloud, speed: between(random, 0.8, 2) });
    }
  }

  // The clouds drift east, wrapping round from the far west.
  update(delta: number): void {
    for (const { cloud, speed } of this.drift) {
      cloud.position.x += speed * delta;
      if (cloud.position.x > 1300) cloud.position.x -= 2600;
    }
  }
}

// The sky: a dome shaded from the haze at the horizon to deep blue
// overhead, with the sun's disc and its glow; drawn behind everything.
function sky(colors: SkyColors): THREE.Mesh {
  const material = new THREE.ShaderMaterial({
    uniforms: {
      horizon: { value: new THREE.Color(colors.horizon) },
      middle: { value: new THREE.Color(colors.middle) },
      zenith: { value: new THREE.Color(colors.zenith) },
      sun: { value: SUN.clone().normalize() },
    },
    vertexShader: /* glsl */ `
      varying vec3 vDirection;
      void main() {
        vDirection = normalize(position);
        vec4 clip = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        gl_Position = clip.xyww; // on the far plane, behind everything
      }
    `,
    fragmentShader: /* glsl */ `
      uniform vec3 horizon;
      uniform vec3 middle;
      uniform vec3 zenith;
      uniform vec3 sun;
      varying vec3 vDirection;
      void main() {
        vec3 d = normalize(vDirection);
        float h = d.y;
        vec3 outColor = mix(horizon, middle, smoothstep(0.0, 0.22, h));
        outColor = mix(outColor, zenith, smoothstep(0.18, 0.85, h));
        float s = max(dot(d, sun), 0.0);
        outColor += vec3(1.0, 0.96, 0.86) * (pow(s, 900.0) * 1.2 + pow(s, 14.0) * 0.1);
        gl_FragColor = vec4(outColor, 1.0);
        #include <colorspace_fragment>
      }
    `,
    side: THREE.BackSide,
    depthWrite: false,
    fog: false,
  });
  material.name = 'sky';
  const dome = new THREE.Mesh(new THREE.SphereGeometry(2400, 48, 24), material);
  dome.frustumCulled = false;
  dome.renderOrder = -10;
  return dome;
}
