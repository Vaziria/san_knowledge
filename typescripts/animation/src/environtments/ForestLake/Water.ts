import * as THREE from 'three';
import { addBlob, between, color, halo, matte, mesh, PALETTE, seededRandom, Shape, type Season } from '../../figures/ForestLake/parts';
import { SUN } from '../../sun';
import { waterSurface } from '../water';
import { CASCADE } from './layout';
import { smoothstep, type River, type Terrain } from './Terrain';

// The waves on the forest lake's open water, in meters: small, three
// crossing swells. The same sum is drawn by the water's shader (WAVES
// below), so what floats rides exactly what is drawn.
export function waveHeight(x: number, z: number, t: number): number {
  return (
    0.022 * Math.sin(0.42 * x + 0.25 * z + 1.1 * t) +
    0.016 * Math.sin(-0.2 * x + 0.61 * z + 1.4 * t + 1.7) +
    0.009 * Math.sin(0.9 * x - 0.7 * z + 2.3 * t + 0.4)
  );
}

const WAVES = /* glsl */ `
float waves(vec2 p, float t) {
  return 0.022 * sin(0.42 * p.x + 0.25 * p.y + 1.1 * t)
       + 0.016 * sin(-0.2 * p.x + 0.61 * p.y + 1.4 * t + 1.7)
       + 0.009 * sin(0.9 * p.x - 0.7 * p.y + 2.3 * t + 0.4);
}
vec2 waveSlope(vec2 p, float t) {
  float a = 0.022 * cos(0.42 * p.x + 0.25 * p.y + 1.1 * t);
  float b = 0.016 * cos(-0.2 * p.x + 0.61 * p.y + 1.4 * t + 1.7);
  float c = 0.009 * cos(0.9 * p.x - 0.7 * p.y + 2.3 * t + 0.4);
  return vec2(0.42 * a - 0.2 * b + 0.9 * c, 0.25 * a + 0.61 * b - 0.7 * c);
}
`;

const NOISE = /* glsl */ `
vec2 hash2(vec2 p) {
  p = vec2(dot(p, vec2(127.1, 311.7)), dot(p, vec2(269.5, 183.3)));
  return fract(sin(p) * 43758.5453);
}
float hash1(vec2 p) {
  return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453);
}
float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash1(i), hash1(i + vec2(1.0, 0.0)), u.x), mix(hash1(i + vec2(0.0, 1.0)), hash1(i + vec2(1.0, 1.0)), u.x), u.y);
}
// The nearest and second-nearest of a field of wandering points (Worley).
vec2 cells(vec2 p, float t) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  float f1 = 8.0;
  float f2 = 8.0;
  for (int y = -1; y <= 1; y++) {
    for (int x = -1; x <= 1; x++) {
      vec2 g = vec2(float(x), float(y));
      vec2 o = hash2(i + g);
      o = 0.5 + 0.42 * sin(t + 6.2831 * o);
      float d = length(g + o - f);
      if (d < f1) { f2 = f1; f1 = d; } else if (d < f2) { f2 = d; }
    }
  }
  return vec2(f1, f2);
}
// Still cells, with an id for the nearest (0..1): plates of ice.
vec3 plates(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  float f1 = 8.0;
  float f2 = 8.0;
  float id = 0.0;
  for (int y = -1; y <= 1; y++) {
    for (int x = -1; x <= 1; x++) {
      vec2 g = vec2(float(x), float(y));
      vec2 o = 0.15 + 0.7 * hash2(i + g);
      float d = length(g + o - f);
      if (d < f1) { f2 = f1; f1 = d; id = hash1(i + g); } else if (d < f2) { f2 = d; }
    }
  }
  return vec3(f1, f2, id);
}
`;

// The water's colours in each season, as the reference's seasons paint it:
// clear turquoise over blue in spring, a deeper blue in summer, a darker,
// cooler blue in autumn, and in winter, where it is not ice, dark and cold.
const WATER: Record<Season, { shallow: number; deep: number }> = {
  spring: { shallow: 0x25cbdc, deep: 0x1278cc },
  summer: { shallow: 0x1fbad9, deep: 0x0b62bd },
  autumn: { shallow: 0x2aaec2, deep: 0x0d5aa3 },
  winter: { shallow: 0x3aa6c8, deep: 0x125a92 },
};

// The forest lake's water (ForestLake.md), as the reference paints it:
// clear turquoise in the shallows, showing the sandy bed, deepening to
// blue; bright lines of light wandering over it in cells; streaks carried
// along where it flows; white foam at the shore, round rocks and at the
// waterfalls' feet; the sky in it at a glance, and the sun glinting off
// its waves. Each vertex says how deep the water over it is (depth), which
// way and how fast it flows (flow), how much foam is on it (foam) and, in
// winter, how much of it is frozen (ice): plates of pale ice, each its own
// shade, white cracks between them, frosted white toward the shore, the
// plates breaking up where the ice ends at the open water.
function waterMaterial(season: Season): THREE.ShaderMaterial {
  const material = new THREE.ShaderMaterial({
    uniforms: THREE.UniformsUtils.merge([
      THREE.UniformsLib.fog,
      {
        time: { value: 0 },
        shallow: { value: new THREE.Color(WATER[season].shallow) },
        deep: { value: new THREE.Color(WATER[season].deep) },
        iceLight: { value: new THREE.Color(0xe2f4fb) },
        iceDeep: { value: new THREE.Color(0x9fd0ea) },
        frost: { value: new THREE.Color(PALETTE.snow) },
        foamColor: { value: new THREE.Color(0xf4fbff) },
        sky: { value: new THREE.Color(0xd2ecff) },
        sunColor: { value: new THREE.Color(0xfff4dc) },
        sunDirection: { value: SUN.clone().normalize() },
        dull: { value: 0 },
      },
    ]),
    vertexShader: /* glsl */ `
      #include <common>
      #include <fog_pars_vertex>
      uniform float time;
      attribute float depth;
      attribute vec2 flow;
      attribute float foam;
      attribute float ice;
      varying float vDepth;
      varying vec2 vFlow;
      varying float vFoam;
      varying float vIce;
      varying vec3 vLocal;
      varying vec3 vWorld;
      varying vec2 vSlope;
      ${WAVES}
      void main() {
        vec3 p = position;
        // Waves on open, still water; hardly any in the shallows or where it
        // flows, and none under ice.
        float calm = smoothstep(0.02, 0.5, depth) * (1.0 - min(1.0, length(flow) * 2.0)) * (1.0 - ice);
        vIce = ice;
        p.y += waves(p.xz, time) * calm;
        vSlope = waveSlope(p.xz, time) * calm;
        vDepth = depth;
        vFlow = flow;
        vFoam = foam;
        vLocal = p;
        vec4 world = modelMatrix * vec4(p, 1.0);
        vWorld = world.xyz;
        vec4 mvPosition = viewMatrix * world;
        gl_Position = projectionMatrix * mvPosition;
        #include <fog_vertex>
      }
    `,
    fragmentShader: /* glsl */ `
      #include <common>
      #include <fog_pars_fragment>
      uniform float time;
      uniform vec3 shallow;
      uniform vec3 deep;
      uniform vec3 foamColor;
      uniform vec3 sky;
      uniform vec3 sunColor;
      uniform vec3 sunDirection;
      uniform vec3 iceLight;
      uniform vec3 iceDeep;
      uniform vec3 frost;
      varying float vDepth;
      varying vec2 vFlow;
      varying float vFoam;
      varying float vIce;
      varying vec3 vLocal;
      varying vec3 vWorld;
      varying vec2 vSlope;
      ${NOISE}
      void main() {
        float d = max(vDepth, 0.0);
        vec3 tint = mix(shallow, deep, smoothstep(0.15, 2.6, d));
        // Cells of light, drifting, carried along where it flows.
        float speed = length(vFlow);
        vec2 c = cells(vLocal.xz * 0.6 - vFlow * time * 0.6, time * 0.55);
        float edge = 1.0 - smoothstep(0.0, 0.1, c.y - c.x);
        tint += vec3(0.42, 0.66, 0.72) * edge * (0.12 + 0.2 * (1.0 - smoothstep(0.3, 2.5, d)));
        // Streaks along running water.
        if (speed > 0.001) {
          vec2 dir = vFlow / speed;
          vec2 across = vec2(-dir.y, dir.x);
          float along = dot(vLocal.xz, dir) - time * speed * 1.4;
          float streak = noise(vec2(dot(vLocal.xz, across) * 3.5, along * 0.9));
          tint = mix(tint, tint * 1.2 + 0.12, smoothstep(0.62, 0.9, streak) * min(1.0, speed * 1.4));
        }
        // Foam: at the edge of the water, and where each vertex asks for it.
        float froth = max(vFoam, 1.0 - smoothstep(0.0, 0.13, d));
        float breakup = noise(vLocal.xz * 2.6 + vec2(time * 0.35, -time * 0.25) - vFlow * time);
        froth = smoothstep(0.38, 0.72, froth * (0.55 + 0.9 * breakup));
        // The sky at a glance, the sun's glint off the waves.
        vec3 normal = normalize(vec3(-vSlope.x * 7.0, 1.0, -vSlope.y * 7.0));
        vec3 view = normalize(cameraPosition - vWorld);
        float fresnel = pow(1.0 - max(dot(normal, view), 0.0), 3.0);
        tint = mix(tint, sky, fresnel * 0.22);
        float glint = pow(max(dot(normal, normalize(sunDirection + view)), 0.0), 180.0);
        tint += sunColor * glint * 0.9;
        vec3 outColor = mix(tint, foamColor, froth);
        // As see-through as the lake's water at most, so what swims in it shows.
        float alpha = mix(0.45, 0.72, smoothstep(0.05, 2.2, d));
        alpha = max(max(alpha, froth), fresnel * 0.9);
        // Ice: plates about 2.5 m across, the ones with the higher ids
        // reaching further out where the ice ends.
        if (vIce > 0.001) {
          vec3 pl = plates(vLocal.xz * 0.4);
          float crack = 1.0 - smoothstep(0.0, 0.07, pl.y - pl.x);
          float fine = 1.0 - smoothstep(0.0, 0.05, abs(noise(vLocal.xz * 1.7) - 0.5));
          vec3 iceColor = mix(iceDeep, iceLight, 0.35 + 0.65 * pl.z);
          iceColor = mix(iceColor, frost, smoothstep(0.7, 0.0, d) * 0.85 + fine * 0.18);
          iceColor = mix(iceColor, vec3(1.0), crack * 0.8);
          iceColor += sky * fresnel * 0.25 + sunColor * glint * 0.4;
          float here = step(1.0 - vIce, 0.25 + 0.75 * pl.z) * (1.0 - smoothstep(0.02, 0.06, pl.y - pl.x) * (1.0 - vIce) * 0.9);
          here = max(here, step(0.999, vIce));
          outColor = mix(outColor, iceColor, here);
          alpha = mix(alpha, 0.97, here);
        }
        gl_FragColor = vec4(outColor, alpha);
        #include <tonemapping_fragment>
        #include <colorspace_fragment>
        #include <fog_fragment>
      }
    `,
    transparent: true,
    depthWrite: false,
    fog: true,
  });
  material.name = 'water';
  return material;
}

// A falling sheet of water: pale blue streaked with white, streaks running
// down it, white where it tips over its lip and where it lands, a little
// see-through. Unlit. In winter (`frozen`) it has nearly stopped: slow,
// its streaks long icicles, paler and bluer.
function fallMaterial(frozen: boolean): THREE.ShaderMaterial {
  const material = new THREE.ShaderMaterial({
    uniforms: THREE.UniformsUtils.merge([THREE.UniformsLib.fog, { time: { value: 0 }, frozen: { value: frozen ? 1 : 0 } }]),
    vertexShader: /* glsl */ `
      #include <common>
      #include <fog_pars_vertex>
      varying vec2 vUv;
      void main() {
        vUv = uv;
        vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
        gl_Position = projectionMatrix * mvPosition;
        #include <fog_vertex>
      }
    `,
    fragmentShader: /* glsl */ `
      #include <common>
      #include <fog_pars_fragment>
      uniform float time;
      uniform float frozen;
      varying vec2 vUv;
      ${NOISE}
      void main() {
        // uv: x across the fall, y down it (0 at its lip).
        float flow = time * mix(1.0, 0.04, frozen);
        float streak = noise(vec2(vUv.x * 26.0, vUv.y * mix(2.2, 0.8, frozen) - flow * 2.4));
        float fine = noise(vec2(vUv.x * 60.0, vUv.y * mix(5.0, 1.5, frozen) - flow * 3.4));
        vec3 blue = mix(vec3(0.45, 0.75, 0.92), vec3(0.62, 0.84, 0.95), frozen);
        vec3 white = vec3(0.96, 0.99, 1.0);
        float lit = smoothstep(0.45, 0.8, streak * 0.7 + fine * 0.45);
        float ends = max(1.0 - smoothstep(0.0, 0.07, vUv.y), smoothstep(0.78, 1.0, vUv.y));
        vec3 outColor = mix(blue, white, max(lit, ends));
        float sides = smoothstep(0.0, 0.06, vUv.x) * smoothstep(1.0, 0.94, vUv.x);
        gl_FragColor = vec4(outColor, (0.82 + 0.15 * lit) * sides);
        #include <tonemapping_fragment>
        #include <colorspace_fragment>
        #include <fog_fragment>
      }
    `,
    transparent: true,
    depthWrite: false,
    side: THREE.DoubleSide,
    fog: true,
  });
  material.name = 'waterfall';
  return material;
}

// Where foam gathers on the water: round a point, fading out by `radius`.
export interface Froth {
  x: number;
  z: number;
  radius: number;
  strength: number;
}

// A waterfall: where it tips over (its lip, the level of the water there
// and which way it runs out), how wide, and how far it falls.
export interface Fall {
  x: number;
  z: number;
  level: number;
  width: number;
  out: THREE.Vector2; // the way it runs over the lip, level
  bottom: number; // the water's level where it lands
}

// The forest lake's water: the lake with the stream's first stretch as one
// sheet, the rivers over the heights and the stream past its cascade as
// ribbons along them, the waterfalls, the foam and mist at their feet, and
// the splashes something breaking the surface throws up (splash()).
export class Water extends THREE.Group {
  private readonly material: THREE.ShaderMaterial;
  private readonly fallMaterial: THREE.ShaderMaterial;
  // In winter the water is ice, but for a hole of open water kept round
  // `hole` (where a floating figure goes).
  readonly frozen: boolean;
  readonly lake: THREE.Mesh;
  private readonly foam: { mesh: THREE.Mesh; phase: number }[] = [];
  private readonly mist: { sprite: THREE.Sprite; age: number; life: number; from: THREE.Vector3; drift: THREE.Vector3 }[] = [];
  private readonly rings: { mesh: THREE.Mesh; age: number }[] = [];
  private readonly drops: THREE.Points;
  private readonly dropState: { position: THREE.Vector3; velocity: THREE.Vector3; age: number }[] = [];
  private time = 0;
  private readonly random = seededRandom(301);

  constructor(
    private readonly terrain: Terrain,
    froth: readonly Froth[],
    falls: readonly Fall[],
    season: Season = 'spring',
    private readonly hole = { x: 0, z: 0, radius: 3.5 },
  ) {
    super();
    this.frozen = season === 'winter';
    this.material = waterMaterial(season);
    this.fallMaterial = fallMaterial(this.frozen);
    waterSurface(this.material);
    this.lake = this.buildLake(froth);
    this.lake.renderOrder = 1;
    this.add(this.lake);
    for (const river of terrain.rivers) this.add(this.ribbon(river.line.points, (along) => terrain.riverLevel(river, along), river.lip.width, 0.75, 0, river));
    const stream = terrain.stream;
    this.add(this.ribbon(stream.points, (along) => terrain.streamLevel(along), 3.1, 0.9, CASCADE.from - 0.3));
    for (const fall of falls) this.addFall(fall);

    // Droplets for splashes.
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(90 * 3), 3));
    this.drops = new THREE.Points(geometry, new THREE.PointsMaterial({ color: 0xeefaff, size: 0.05, transparent: true, opacity: 0.9, depthWrite: false }));
    this.drops.frustumCulled = false;
    this.drops.renderOrder = 3;
    this.add(this.drops);
  }

  // The lake, and the stream down to its cascade: a sheet 0.5 m to a cell,
  // over every cell with water in it, flat at y 0 (the shader adds the
  // waves), each vertex knowing the depth under it.
  private buildLake(froth: readonly Froth[]): THREE.Mesh {
    const { terrain } = this;
    const step = 0.5;
    const x0 = -24;
    const x1 = 24;
    const z0 = -34;
    const z1 = 20;
    const columns = Math.round((x1 - x0) / step);
    const rows = Math.round((z1 - z0) / step);
    const wet = (x: number, z: number) => {
      if (terrain.lake.inside(x, z) || terrain.lake.distance(x, z) < 1.2) return true;
      const near = terrain.stream.nearest(x, z);
      return near.distance < 2.6 && near.along < CASCADE.from;
    };
    const index = new Map<number, number>();
    const positions: number[] = [];
    const depths: number[] = [];
    const flows: number[] = [];
    const foams: number[] = [];
    const ices: number[] = [];
    const cells: number[] = [];
    const vertex = (i: number, j: number) => {
      const key = j * (columns + 1) + i;
      let k = index.get(key);
      if (k !== undefined) return k;
      k = positions.length / 3;
      index.set(key, k);
      const x = x0 + i * step;
      const z = z0 + j * step;
      positions.push(x, 0, z);
      depths.push(-terrain.heightAt(x, z));
      // Flowing out along the stream, faster the further from the lake.
      const near = terrain.stream.nearest(x, z);
      const pull = near.segment >= 0 && near.distance < 3 ? smoothstep(-1.5, 3, near.along) * smoothstep(3, 1.2, near.distance) : 0;
      const along = terrain.stream.at(near.along);
      flows.push(along.dx * pull * 0.9, along.dz * pull * 0.9);
      let foam = 0;
      for (const f of froth) foam = Math.max(foam, f.strength * (1 - smoothstep(f.radius * 0.35, f.radius, Math.hypot(x - f.x, z - f.z))));
      foams.push(foam);
      ices.push(this.iceAt(x, z));
      return k;
    };
    for (let j = 0; j < rows; j++) {
      for (let i = 0; i < columns; i++) {
        const x = x0 + (i + 0.5) * step;
        const z = z0 + (j + 0.5) * step;
        if (!wet(x, z)) continue;
        // Only where the ground somewhere in the cell is under the water.
        const lowest = Math.min(
          terrain.heightAt(x - step / 2, z - step / 2),
          terrain.heightAt(x + step / 2, z - step / 2),
          terrain.heightAt(x - step / 2, z + step / 2),
          terrain.heightAt(x + step / 2, z + step / 2),
        );
        if (lowest > 0.06) continue;
        const a = vertex(i, j);
        const b = vertex(i + 1, j);
        const c = vertex(i, j + 1);
        const d = vertex(i + 1, j + 1);
        cells.push(a, c, b, b, c, d);
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geometry.setAttribute('depth', new THREE.Float32BufferAttribute(depths, 1));
    geometry.setAttribute('flow', new THREE.Float32BufferAttribute(flows, 2));
    geometry.setAttribute('foam', new THREE.Float32BufferAttribute(foams, 1));
    geometry.setAttribute('ice', new THREE.Float32BufferAttribute(ices, 1));
    geometry.setIndex(cells);
    geometry.computeBoundingSphere();
    return new THREE.Mesh(geometry, this.material);
  }

  // A ribbon of running water along a line (a river, the stream), from
  // `start` m along it, at the water's level along it, `width` wide, flowing
  // at `speed` m/s; frothing where it drops fast (the cascade) and at its
  // lip (a river running to a waterfall).
  private ribbon(points: number[][], level: (along: number) => number, width: number, speed: number, start: number, river?: River): THREE.Mesh {
    const { terrain } = this;
    const stations: { x: number; z: number; dx: number; dz: number; along: number }[] = [];
    let along = 0;
    for (let i = 0; i < points.length; i++) {
      if (i > 0) along += Math.hypot(points[i][0] - points[i - 1][0], points[i][1] - points[i - 1][1]);
      if (along < start) continue;
      const a = points[Math.max(0, i - 1)];
      const b = points[Math.min(points.length - 1, i + 1)];
      const length = Math.hypot(b[0] - a[0], b[1] - a[1]) || 1;
      stations.push({ x: points[i][0], z: points[i][1], dx: (b[0] - a[0]) / length, dz: (b[1] - a[1]) / length, along });
    }
    const across = 7;
    const positions: number[] = [];
    const depths: number[] = [];
    const flows: number[] = [];
    const foams: number[] = [];
    const ices: number[] = [];
    const total = along;
    for (const s of stations) {
      const y = level(s.along);
      const drop = Math.abs(level(s.along + 0.5) - level(s.along - 0.5));
      const toLip = river ? total - s.along : Infinity;
      for (let k = 0; k < across; k++) {
        const offset = (k / (across - 1) - 0.5) * (width + 0.9);
        const x = s.x - s.dz * offset;
        const z = s.z + s.dx * offset;
        positions.push(x, y, z);
        depths.push(y - terrain.heightAt(x, z));
        const fast = speed * (1 + drop * 3);
        flows.push(s.dx * fast, s.dz * fast);
        foams.push(Math.max(smoothstep(0.05, 0.4, drop), 1 - smoothstep(0, 1.6, toLip)) * 0.9 * (this.frozen ? 0.2 : 1));
        ices.push(this.frozen ? 1 : 0);
      }
    }
    const index: number[] = [];
    for (let r = 0; r + 1 < stations.length; r++) {
      for (let k = 0; k + 1 < across; k++) {
        const a = r * across + k;
        const b = a + 1;
        const c = a + across;
        const d = c + 1;
        index.push(a, b, c, b, d, c);
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geometry.setAttribute('depth', new THREE.Float32BufferAttribute(depths, 1));
    geometry.setAttribute('flow', new THREE.Float32BufferAttribute(flows, 2));
    geometry.setAttribute('foam', new THREE.Float32BufferAttribute(foams, 1));
    geometry.setAttribute('ice', new THREE.Float32BufferAttribute(ices, 1));
    geometry.setIndex(index);
    geometry.computeBoundingSphere();
    const ribbon = new THREE.Mesh(geometry, this.material);
    ribbon.renderOrder = 1;
    return ribbon;
  }

  // A waterfall: a sheet over its lip and down, bending out as falling water
  // does, then white foam heaving where it lands, and mist rising.
  private addFall(fall: Fall): void {
    const out = fall.out.clone().normalize();
    const across = new THREE.Vector2(-out.y, out.x);
    const drop = fall.level - fall.bottom;
    const speed = 1.3; // m/s it leaves the lip at
    const time = Math.sqrt((2 * drop) / 9.8);
    const columns = 10;
    const rows = 18;
    const positions: number[] = [];
    const uvs: number[] = [];
    for (let r = 0; r <= rows; r++) {
      const v = r / rows;
      const t = time * v;
      const forward = speed * t + 0.15 * Math.sin(v * Math.PI);
      const y = fall.level - 4.9 * t * t;
      for (let c = 0; c <= columns; c++) {
        const u = c / columns;
        // A little wider as it falls, its edges ragged.
        const spread = (u - 0.5) * fall.width * (1 + v * 0.18);
        positions.push(fall.x + out.x * forward + across.x * spread, y, fall.z + out.y * forward + across.y * spread);
        uvs.push(u, v);
      }
    }
    const index: number[] = [];
    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < columns; c++) {
        const a = r * (columns + 1) + c;
        index.push(a, a + columns + 1, a + 1, a + 1, a + columns + 1, a + columns + 2);
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geometry.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2));
    geometry.setIndex(index);
    geometry.computeBoundingSphere();
    const sheet = new THREE.Mesh(geometry, this.fallMaterial);
    sheet.renderOrder = 2;
    this.add(sheet);

    // Foam heaving where it lands: white lumps, each swelling and settling
    // on its own time.
    const landing = new THREE.Vector3(fall.x + out.x * speed * time, fall.bottom, fall.z + out.y * speed * time);
    const random = this.random;
    const white = color(0xf6fbff);
    const blue = color(0xd4ecf7);
    const lumps = Math.round(6 + fall.width * 2.2);
    for (let k = 0; k < lumps; k++) {
      const shape = new Shape();
      const size = between(random, 0.35, 0.7) * Math.min(1.4, 0.6 + fall.width * 0.2);
      addBlob(shape, new THREE.Vector3(), new THREE.Vector3(size, size * 0.5, size), 1, 0.25, random, (n) => (n.y > 0.2 ? white.clone() : (this.frozen ? color(PALETTE.ice) : blue.clone())));
      const lump = mesh(shape.geometry(), matte(), false);
      const spread = (random() - 0.5) * fall.width * 1.3;
      const ahead = between(random, -0.3, 1.4);
      lump.position.set(landing.x + across.x * spread + out.x * ahead, fall.bottom - 0.05, landing.z + across.y * spread + out.y * ahead);
      this.foam.push({ mesh: lump, phase: random() * Math.PI * 2 });
      this.add(lump);
    }
    // Mist rising off it (none off a frozen fall).
    for (let k = 0; k < (this.frozen ? 0 : Math.round(4 + fall.width * 1.5)); k++) {
      const sprite = halo(0xffffff, 1, 0.5);
      sprite.material.blending = THREE.NormalBlending;
      sprite.renderOrder = 3;
      const from = landing.clone().add(new THREE.Vector3(across.x * (random() - 0.5) * fall.width, 0.2, across.y * (random() - 0.5) * fall.width));
      this.mist.push({ sprite, age: random() * 3, life: between(random, 2.5, 4), from, drift: new THREE.Vector3(out.x * 0.35, between(random, 0.35, 0.6), out.y * 0.35) });
      this.add(sprite);
    }
  }

  // The water's surface over a point of open water now (the lake), from the
  // same waves the shader draws; still where it is ice.
  surfaceAt(x: number, z: number): number {
    return waveHeight(x, z, this.time) * (1 - this.iceAt(x, z));
  }

  // How much of the water at a point is ice: in winter all of it, but for
  // the hole of open water, its edge broken over 1.8 m; none in the other
  // seasons.
  iceAt(x: number, z: number): number {
    if (!this.frozen) return 0;
    return smoothstep(this.hole.radius, this.hole.radius + 1.8, Math.hypot(x - this.hole.x, z - this.hole.z));
  }

  // Something breaks the surface at (x, z), moving at `velocity`: a ring
  // spreading out, and drops thrown up.
  splash(x: number, z: number, velocity: THREE.Vector3): void {
    const y = this.surfaceAt(x, z) + 0.01;
    const ring = new THREE.Mesh(new THREE.RingGeometry(0.8, 1, 24), new THREE.MeshBasicMaterial({ color: 0xf2fbff, transparent: true, opacity: 0.8, depthWrite: false }));
    ring.rotation.x = -Math.PI / 2;
    ring.position.set(x, y, z);
    ring.scale.setScalar(0.1);
    ring.renderOrder = 3;
    this.add(ring);
    this.rings.push({ mesh: ring, age: 0 });
    const strength = Math.min(1, velocity.length() / 3);
    for (let k = 0; k < 14; k++) {
      const angle = this.random() * Math.PI * 2;
      const out = between(this.random, 0.3, 0.9);
      const position = new THREE.Vector3(x, y, z);
      const push = new THREE.Vector3(Math.cos(angle) * out + velocity.x * 0.2, between(this.random, 1.2, 2.6) * (0.5 + strength), Math.sin(angle) * out + velocity.z * 0.2);
      if (this.dropState.length >= 90) this.dropState.shift();
      this.dropState.push({ position, velocity: push, age: 0 });
    }
  }

  update(delta: number): void {
    this.time += delta;
    const t = this.time;
    this.material.uniforms.time.value = t;
    this.fallMaterial.uniforms.time.value = t;
    for (const { mesh: lump, phase } of this.frozen ? [] : this.foam) {
      const swell = 1 + 0.18 * Math.sin(t * 3.1 + phase) + 0.08 * Math.sin(t * 7.3 + phase * 2);
      lump.scale.set(swell, 0.8 + 0.4 * Math.abs(Math.sin(t * 2.2 + phase)), swell);
      lump.rotation.y = phase + t * 0.2;
    }
    for (const m of this.mist) {
      m.age += delta;
      if (m.age > m.life) m.age -= m.life;
      const f = m.age / m.life;
      m.sprite.position.copy(m.from).addScaledVector(m.drift, m.age);
      m.sprite.scale.setScalar(1.2 + f * 2.6);
      m.sprite.material.opacity = 0.42 * Math.sin(f * Math.PI);
    }
    for (let i = this.rings.length - 1; i >= 0; i--) {
      const ring = this.rings[i];
      ring.age += delta;
      ring.mesh.scale.setScalar(0.1 + ring.age * 0.9);
      (ring.mesh.material as THREE.MeshBasicMaterial).opacity = Math.max(0, 0.8 * (1 - ring.age / 1.2));
      if (ring.age > 1.2) {
        this.remove(ring.mesh);
        ring.mesh.geometry.dispose();
        (ring.mesh.material as THREE.Material).dispose();
        this.rings.splice(i, 1);
      }
    }
    const positions = this.drops.geometry.getAttribute('position') as THREE.BufferAttribute;
    for (let i = 0; i < 90; i++) {
      const drop = this.dropState[i];
      if (!drop) {
        positions.setXYZ(i, 0, -100, 0);
        continue;
      }
      drop.age += delta;
      drop.velocity.y -= 9.8 * delta;
      drop.position.addScaledVector(drop.velocity, delta);
      if (drop.position.y < this.surfaceAt(drop.position.x, drop.position.z) - 0.05) drop.position.set(0, -100, 0);
      positions.setXYZ(i, drop.position.x, drop.position.y, drop.position.z);
    }
    positions.needsUpdate = true;
  }
}
