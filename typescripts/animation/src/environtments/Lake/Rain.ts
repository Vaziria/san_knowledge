import * as THREE from 'three';
import { keepSharp } from '../../effects/kuwahara';
import { seededRandom } from '../../figures/Tree/parts';
import type { Theme } from '../../theme';
import type { Wind } from '../wind';
import { shoreRadius } from './Ground';

// Rain at the lake, as the user asked ("add randomly rain in lake
// environtment"): streaks falling round the camera, leaning downwind, and
// small rings where drops land on the water, all over the lake. How hard it
// rains comes from the weather (Weather.ts: showers now and then); at 0
// nothing is drawn.
//
// - The streaks are one instanced mesh of thin quads, each moved on the
//   graphics card: it falls at its own speed and drifts with the wind, in a
//   box that goes with the camera and wraps round, as the sky dome does, so
//   the walking camera walks through rain too. Each faces the camera about
//   its own length, and none falls below the water's level. A harder shower
//   shows more of them. They are drawn far thicker than real (1.2 cm across,
//   40 cm long) and kept out of the Kuwahara filter (keepSharp), which
//   otherwise averages away anything a few pixels wide; lit from above, so
//   they dim with the daylight at night.
// - The rings are another instanced mesh, each ring spreading from 3 to 22 cm
//   and fading over 0.9 s, then starting again somewhere else on the water.
//   They ride the waves (heightAt), 4 mm above the water, and are kept out of
//   the filter too: a few centimeters wide, as the splash rings are
//   (Splash.ts), they were averaged away. Kept sharp they are opaque, so they
//   write depth, and the see-through water drawn after them leaves them be.
//
// Both are the water's colour lightened toward the theme's light neutral,
// like the splash drops. Neither casts a shadow. Units are meters, in the
// lake's coordinates; y = 0 is the still water.

const STREAKS = {
  count: 3000, // in the box at the heaviest
  box: new THREE.Vector3(24, 14, 24), // m, round the camera
  speed: [6.5, 8.5] as const, // m/s, falling
  drift: 3, // m/s downwind in a full wind
  calm: 0.4, // m/s downwind even in still air, so the rain never falls quite straight
  length: 0.4, // m
  width: 0.012, // m
};
const RINGS = {
  count: 280, // spreading at once at the heaviest
  life: 0.9, // s
  radius: [0.03, 0.22] as const, // m, as it spreads
  width: 0.35, // share of its radius the ring is wide
  lift: 0.004, // m above the water, so the water doesn't cover it
  edge: 0.3, // m kept in from the shore
  segments: 20,
};
const FOAM = 0.6; // how far the colour goes from the water's toward the light neutral
const SEED = 19;

interface RainUniforms {
  [name: string]: THREE.IUniform;
  rainTime: { value: number };
  rainCamera: { value: THREE.Vector3 };
  rainWind: { value: THREE.Vector2 };
  rainLevel: { value: number };
}

export class Rain extends THREE.Group {
  readonly streaks: THREE.InstancedMesh;
  readonly rings: THREE.InstancedMesh;
  private readonly uniforms: RainUniforms;
  private readonly heightAt: (x: number, z: number) => number;
  private readonly random = seededRandom(SEED);
  private readonly spots: { x: number; z: number; age: number }[] = [];
  private readonly foam: THREE.Color;
  private readonly water: THREE.Color;
  private time = 0;
  private frames = 0;

  constructor(theme: Theme, heightAt: (x: number, z: number) => number) {
    super();
    this.name = 'rain';
    this.heightAt = heightAt;
    this.water = new THREE.Color(theme.scene.water);
    this.foam = this.water.clone().lerp(new THREE.Color(theme.colors.light), FOAM);
    this.uniforms = {
      rainTime: { value: 0 },
      rainCamera: { value: new THREE.Vector3() },
      rainWind: { value: new THREE.Vector2() },
      rainLevel: { value: 0 },
    };

    // The streaks: where each starts in the box, the share of the heaviest
    // shower it needs to show, and its speed.
    const random = this.random;
    const seeds = new Float32Array(STREAKS.count * 4);
    const speeds = new Float32Array(STREAKS.count);
    for (let i = 0; i < STREAKS.count; i++) {
      seeds.set([random(), random(), random(), random()], i * 4);
      speeds[i] = STREAKS.speed[0] + (STREAKS.speed[1] - STREAKS.speed[0]) * random();
    }
    const quad = new THREE.PlaneGeometry(1, 1);
    quad.setAttribute('rainSeed', new THREE.InstancedBufferAttribute(seeds, 4));
    quad.setAttribute('rainSpeed', new THREE.InstancedBufferAttribute(speeds, 1));
    const streak = new THREE.MeshLambertMaterial({ color: this.foam });
    fallOnGpu(streak, this.uniforms);
    keepSharp(streak);
    this.streaks = new THREE.InstancedMesh(quad, streak, STREAKS.count);
    this.streaks.name = 'streaks';
    this.streaks.frustumCulled = false; // placed on the graphics card, round the camera
    this.streaks.castShadow = false;
    this.streaks.receiveShadow = false;
    this.streaks.onBeforeRender = (_renderer, _scene, camera) => {
      this.uniforms.rainCamera.value.setFromMatrixPosition(camera.matrixWorld);
      this.worldToLocal(this.uniforms.rainCamera.value);
    };

    // The rings: flat, one ring shape scaled to each, coloured per instance
    // as it fades from foam into the water.
    const ring = new THREE.RingGeometry(1 - RINGS.width, 1, RINGS.segments);
    ring.rotateX(-Math.PI / 2);
    const ringMaterial = new THREE.MeshStandardMaterial({ roughness: 0.4 });
    keepSharp(ringMaterial);
    this.rings = new THREE.InstancedMesh(ring, ringMaterial, RINGS.count);
    this.rings.name = 'rings';
    this.rings.castShadow = false;
    this.rings.receiveShadow = false;
    this.rings.frustumCulled = false; // spread over the whole lake
    for (let i = 0; i < RINGS.count; i++) {
      this.spots.push({ ...this.spot(), age: RINGS.life * random() });
      this.rings.setColorAt(i, this.water);
    }
    this.rings.count = 0;
    this.add(this.streaks, this.rings);
    this.visible = false;
  }

  // Moves the rain on by delta seconds: `level` is how hard it rains (0 dry,
  // up to 1), and the streaks drift with the wind. Call once per frame.
  update(delta: number, level: number, wind: Wind): void {
    // Drawn in the first frame whatever the weather, every streak folded
    // and one ring shrunk to nothing, so their materials compile with the
    // lake's rather than when the first shower comes, as the fireflies'.
    const first = this.frames++ === 0;
    this.visible = level > 0.001 || first;
    if (level <= 0.001) {
      this.rings.count = first ? 1 : 0;
      if (first) this.rings.setMatrixAt(0, new THREE.Matrix4().makeScale(0, 0, 0));
      return;
    }
    this.time += delta;
    const u = this.uniforms;
    u.rainTime.value = this.time;
    u.rainLevel.value = level;
    u.rainWind.value.copy(wind.direction.value).multiplyScalar(STREAKS.calm + STREAKS.drift * wind.strength.value);

    // Rings: each spreads and fades, then starts again somewhere else.
    const count = Math.round(RINGS.count * level);
    this.rings.count = count;
    const matrix = new THREE.Matrix4();
    const place = new THREE.Vector3();
    const size = new THREE.Vector3();
    const color = new THREE.Color();
    const still = new THREE.Quaternion();
    for (let i = 0; i < count; i++) {
      const s = this.spots[i];
      s.age += delta;
      if (s.age > RINGS.life) Object.assign(s, this.spot(), { age: s.age % RINGS.life });
      const t = s.age / RINGS.life;
      const radius = THREE.MathUtils.lerp(RINGS.radius[0], RINGS.radius[1], Math.sqrt(t));
      this.rings.setMatrixAt(i, matrix.compose(place.set(s.x, this.heightAt(s.x, s.z) + RINGS.lift, s.z), still, size.set(radius, 1, radius)));
      this.rings.setColorAt(i, color.copy(this.foam).lerp(this.water, t * t));
    }
    this.rings.instanceMatrix.needsUpdate = true;
    if (this.rings.instanceColor) this.rings.instanceColor.needsUpdate = true;
  }

  // A spot on the water, away from the shore.
  private spot(): { x: number; z: number } {
    const angle = 2 * Math.PI * this.random();
    const r = (shoreRadius(angle) - RINGS.edge) * Math.sqrt(this.random());
    return { x: r * Math.cos(angle), z: r * Math.sin(angle) };
  }
}

// Places each streak on the graphics card: falling from its start at its own
// speed and drifting with the wind, wrapped into the box round the camera, and
// turned to face the camera about its length. A streak the shower is too
// light for, or one below the water's level, is folded to nothing.
function fallOnGpu(material: THREE.Material, uniforms: RainUniforms): void {
  const glsl = (n: number) => n.toFixed(4);
  const box = STREAKS.box;
  material.onBeforeCompile = (shader) => {
    Object.assign(shader.uniforms, uniforms);
    shader.vertexShader = shader.vertexShader
      .replace(
        '#include <common>',
        /* glsl */ `#include <common>
        attribute vec4 rainSeed;
        attribute float rainSpeed;
        uniform float rainTime;
        uniform vec3 rainCamera;
        uniform vec2 rainWind;
        uniform float rainLevel;`,
      )
      .replace('#include <beginnormal_vertex>', 'vec3 objectNormal = vec3( 0.0, 1.0, 0.0 ); // lit from above, whichever way it faces')
      .replace(
        '#include <begin_vertex>',
        /* glsl */ `
        vec3 rainBox = vec3( ${glsl(box.x)}, ${glsl(box.y)}, ${glsl(box.z)} );
        vec3 rainVelocity = vec3( rainWind.x, -rainSpeed, rainWind.y );
        vec3 rainFrom = rainCamera - 0.5 * rainBox;
        vec3 rainAt = rainFrom + mod( rainSeed.xyz * rainBox + rainVelocity * rainTime - rainFrom, rainBox );
        vec3 rainAlong = normalize( rainVelocity );
        vec3 rainSide = normalize( cross( rainAlong, rainCamera - rainAt ) );
        float rainShown = step( rainSeed.w, rainLevel ) * step( 0.0, rainAt.y );
        vec3 transformed = rainAt + ( position.y * ${glsl(STREAKS.length)} * rainAlong + position.x * ${glsl(STREAKS.width)} * rainSide ) * rainShown;`,
      );
  };
  material.customProgramCacheKey = () => 'rain-streak';
}
