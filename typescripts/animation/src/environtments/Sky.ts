import * as THREE from 'three';
import { Cloud } from '../figures/objects/Cloud';
import { between, seededRandom } from '../figures/Tree/parts';
import { SUN } from '../sun';
import type { Theme } from '../theme';
import type { Wind } from './wind';

// The sky over an environment, as the user asked ("add sky in lake"): a dome
// that deepens from the haze at the horizon to the theme's zenith colour
// overhead, the sun where the stage's key light shines from (../sun.ts), and
// fair-weather clouds (the cloud object, figures/objects/Cloud.ts) drifting
// across it far off.
//
// - The dome is drawn behind everything, wherever the camera goes: it moves
//   with the camera, is drawn first, on the far plane, and writes no depth.
//   At the horizon it is exactly the theme's background, the colour the land
//   fades into at its end (haze.ts) and the fog's, so the land still meets
//   the sky with no line. It deepens to `zenith` over the first 30° or so,
//   and round the sun it glows in the key light's colour.
// - Clouds: sixty, 150-320 m wide, 220-340 m up and up to 2.5 km out,
//   each one of a handful of cloud shapes, stretched and turned on its own.
//   Each shape is built 2 m wide and scaled up, so a cloud is the smallest
//   cloud's 21 k triangles, not the default's 47 k, and only those in view
//   are drawn. They drift downwind, and one that drifts out past 2.5 km comes
//   back in on the far side. Far clouds fade into the sky behind them, as far
//   things do, and are gone by then, so they come and go unseen near the
//   horizon. They cover about 8% of the sky, as fair-weather cumulus do.
//   From the landing the cliffs stand 10-25° high, hiding the far, low
//   clouds, so it is the big ones a few hundred meters off that show above
//   them: at under half this size the sky looked empty.
// - In fog (the scene's, which the lake's weather thickens and thins) the
//   sky greys over into the fog's colour, clouds and sun with it.
// - At night (the lake's, Lake/DayNight.ts) setDaylight() darkens the dome
//   and the clouds' skylight to the night sky's colours, and the sun fades
//   out over the first half of dusk and back in over the last half of dawn.
//   No moon and no stars. Without it, the sky is the day's. Under a shower's
//   cloud (its `overcast`) the sky greys over as in fog and the sun hides.
//
// Placed by a seeded random generator, so it is the same on every load.
// Units are meters and the origin is the environment's middle, y = 0 its
// ground's level (the lake's water).

export interface SkyOptions {
  theme: Theme;
  fog?: THREE.FogExp2; // the scene's fog: the sky greys over as it thickens
  wind?: Wind; // the clouds drift the way it blows; toward +x without it
  seed?: number;
}

const DOME = {
  // How fast it deepens from the horizon: 1 - (1 - sin(elevation))^spread of
  // the way to the zenith colour, 30% at 5° up, 54% at 10° and 94% at 30°.
  spread: 4,
};
const SUN_DISC = {
  radius: THREE.MathUtils.degToRad(1.8), // far bigger than the real sun's 0.27°, so it shows
  soft: THREE.MathUtils.degToRad(0.4), // its edge blurs over this
  glow: 0.35, // how far the sky round it turns toward the light's colour
  tight: 24, // how closely the glow hugs it: a power of the cosine (70% as strong 10° off, a quarter 20° off)
  set: 0.5, // how far into the night (DayNight.night) it is gone
};
const CLOUDS = {
  count: 60,
  shapes: 5, // cloud objects built, each drawn by several clouds
  build: { width: 2, height: 1, depth: 1.25 }, // m each shape is built at (the default's proportions), then scaled up
  width: [150, 320] as const, // m
  tall: [0.4, 0.6] as const, // times its width
  deep: [0.55, 0.8] as const, // times its width
  base: [220, 340] as const, // m above y = 0
  reach: 2500, // m from the middle: past it a cloud comes back in on the far side
  apart: 1.1, // clouds start at least this many times their half widths apart
  drift: 6, // m/s downwind
  haze: 0.15, // how far even the nearest is faded toward the sky behind it
  // The sky lights a cloud from all round, its underside too, which the
  // stage's lights leave dark (the ground's bounce): this much of the
  // zenith's colour is added to it.
  skylight: 0.5,
  fade: [1000, 2400] as const, // m from the camera over which clouds fade into the sky
};
// The sky greys over as much as the fog greys a thing this far off.
const FOG_REACH = 30; // m
const OVERCAST_GREY = 0.75; // how far a full shower's cloud greys the sky over
const SEED = 23;

// Colours the dome and the clouds share, updated once a frame.
interface SkyUniforms {
  [name: string]: THREE.IUniform;
  skyHorizon: { value: THREE.Color };
  skyZenith: { value: THREE.Color };
  skySun: { value: THREE.Vector3 };
  skySunColor: { value: THREE.Color };
  skyFogColor: { value: THREE.Color };
  skyFog: { value: number };
  skySunShow: { value: number }; // 1 by day, 0 once the sun has set
}

const glsl = (n: number) => n.toFixed(6);

// The sky's colour one way, shared by the dome and the clouds fading into it.
// Colours are linear, as three.js keeps them, and converted for the screen
// after (colorspace_fragment), as the background is.
const SKY_GLSL = /* glsl */ `
uniform vec3 skyHorizon;
uniform vec3 skyZenith;
uniform vec3 skySun;
uniform vec3 skySunColor;
uniform vec3 skyFogColor;
uniform float skyFog;
uniform float skySunShow;

// The clear sky one way (a unit vector): the horizon's colour deepening to
// the zenith's as it rises, and the sun with its glow.
vec3 skyClear( vec3 direction ) {
  float up = max( direction.y, 0.0 );
  vec3 sky = mix( skyHorizon, skyZenith, 1.0 - pow( 1.0 - up, ${glsl(DOME.spread)} ) );
  float toward = max( dot( direction, skySun ), 0.0 );
  sky = mix( sky, skySunColor, skySunShow * ${glsl(SUN_DISC.glow)} * pow( toward, ${glsl(SUN_DISC.tight)} ) );
  return mix( sky, skySunColor, skySunShow * smoothstep( ${glsl(Math.cos(SUN_DISC.radius + SUN_DISC.soft / 2))}, ${glsl(Math.cos(SUN_DISC.radius - SUN_DISC.soft / 2))}, toward ) );
}

// A colour of the sky, greyed over by the fog.
vec3 skyFogged( vec3 color ) {
  return mix( color, skyFogColor, skyFog );
}
`;

const DOME_VERTEX = /* glsl */ `
varying vec3 vSkyDirection;

void main() {
  vSkyDirection = ( modelMatrix * vec4( position, 0.0 ) ).xyz;
  gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
  gl_Position.z = gl_Position.w; // on the far plane, behind everything
}
`;

const DOME_FRAGMENT = /* glsl */ `
varying vec3 vSkyDirection;

void main() {
  gl_FragColor = vec4( skyFogged( skyClear( normalize( vSkyDirection ) ) ), 1.0 );
  #include <colorspace_fragment>
}
`;

export class Sky extends THREE.Group {
  readonly dome: THREE.Mesh;
  readonly clouds = new THREE.Group();
  private readonly uniforms: SkyUniforms;
  private readonly cloudMaterial: THREE.MeshStandardMaterial;
  private readonly fog: THREE.FogExp2 | null;
  private readonly wind: Wind | null;
  private overcast = 0;

  constructor(options: SkyOptions) {
    super();
    this.name = 'sky';
    const { theme } = options;
    this.fog = options.fog ?? null;
    this.wind = options.wind ?? null;
    const random = seededRandom(options.seed ?? SEED);
    this.uniforms = {
      skyHorizon: { value: new THREE.Color(theme.scene.background) },
      skyZenith: { value: new THREE.Color(theme.scene.zenith) },
      skySun: { value: SUN.clone().normalize() },
      skySunColor: { value: new THREE.Color(theme.scene.light) },
      // The fog's own colour, so a change to it shows here too.
      skyFogColor: { value: this.fog?.color ?? new THREE.Color(theme.scene.background) },
      skyFog: { value: 0 },
      skySunShow: { value: 1 },
    };

    const material = new THREE.ShaderMaterial({
      uniforms: this.uniforms,
      vertexShader: DOME_VERTEX,
      fragmentShader: SKY_GLSL + DOME_FRAGMENT,
      side: THREE.BackSide,
      depthTest: false,
      depthWrite: false,
    });
    // Any size does: it is drawn on the far plane, round the camera.
    this.dome = new THREE.Mesh(new THREE.SphereGeometry(1, 32, 16), material);
    this.dome.name = 'dome';
    this.dome.frustumCulled = false;
    this.dome.renderOrder = -1000; // first, so everything is drawn over it
    this.dome.onBeforeRender = (_renderer, _scene, camera) => this.dome.matrixWorld.copyPosition(camera.matrixWorld);

    this.clouds.name = 'clouds';
    this.cloudMaterial = placeClouds(this.clouds, theme, this.uniforms, random);
    this.add(this.dome, this.clouds);
  }

  // The sky's colours at a time of day (Lake/DayNight.ts): at the horizon
  // and overhead now, and how far into the night it is, which sets the sun;
  // and how overcast it is under a shower (0 clear, 1 full), which greys the
  // sky over and hides the sun. The clouds' skylight follows the colour
  // overhead.
  setDaylight(horizon: THREE.Color, zenith: THREE.Color, night: number, overcast = 0): void {
    this.overcast = overcast;
    this.uniforms.skyHorizon.value.copy(horizon);
    this.uniforms.skyZenith.value.copy(zenith);
    this.uniforms.skySunShow.value = (1 - THREE.MathUtils.smoothstep(night, 0, SUN_DISC.set)) * (1 - overcast);
    this.cloudMaterial.emissive.copy(zenith).multiplyScalar(CLOUDS.skylight);
  }

  // Drifts the clouds on by delta seconds and greys the sky by the fog as it
  // is now; call once per frame.
  update(delta: number): void {
    const density = this.fog?.density ?? 0;
    this.uniforms.skyFog.value = Math.max(1 - Math.exp(-((density * FOG_REACH) ** 2)), OVERCAST_GREY * this.overcast);

    const way = this.wind?.direction.value ?? EAST;
    const step = CLOUDS.drift * delta;
    for (const cloud of this.clouds.children) {
      const at = cloud.position;
      at.x += way.x * step;
      at.z += way.y * step;
      // Out past the edge downwind: back in on the far side, as far out.
      const along = at.x * way.x + at.z * way.y;
      if (along > 0 && at.x * at.x + at.z * at.z > CLOUDS.reach * CLOUDS.reach) {
        at.x -= 2 * along * way.x;
        at.z -= 2 * along * way.y;
      }
    }
  }
}

const EAST = new THREE.Vector2(1, 0);

// Clouds scattered over the sky, apart from each other, each a shape
// stretched to its own size and turned its own way.
// Returns their one material.
function placeClouds(group: THREE.Group, theme: Theme, uniforms: SkyUniforms, random: () => number): THREE.MeshStandardMaterial {
  const shapes: Cloud[] = [];
  for (let i = 0; i < CLOUDS.shapes; i++) shapes.push(new Cloud({ theme, seed: Math.floor(random() * 1e6), ...CLOUDS.build }));
  // One material for all: the cloud object's own, lit by the sky and fading
  // into it.
  const material = shapes[0].body.material as THREE.MeshStandardMaterial;
  for (const shape of shapes.slice(1)) (shape.body.material as THREE.Material).dispose();
  material.emissive.set(theme.scene.zenith).multiplyScalar(CLOUDS.skylight);
  fadeIntoSky(material, uniforms);

  const placed: { x: number; z: number; half: number }[] = [];
  for (let attempt = 0; placed.length < CLOUDS.count && attempt < 50 * CLOUDS.count; attempt++) {
    const r = CLOUDS.reach * Math.sqrt(random());
    const angle = 2 * Math.PI * random();
    const width = between(random, ...CLOUDS.width);
    const height = width * between(random, ...CLOUDS.tall);
    const depth = width * between(random, ...CLOUDS.deep);
    const base = between(random, ...CLOUDS.base);
    const turn = 2 * Math.PI * random();
    const shape = shapes[Math.floor(random() * shapes.length)];
    const x = r * Math.cos(angle);
    const z = r * Math.sin(angle);
    if (placed.some((p) => Math.hypot(p.x - x, p.z - z) < CLOUDS.apart * (p.half + width / 2))) continue;
    placed.push({ x, z, half: width / 2 });

    const cloud = new THREE.Mesh(shape.body.geometry, material);
    cloud.name = 'cloud';
    cloud.position.set(x, base, z);
    cloud.rotation.y = turn;
    cloud.scale.set(width / CLOUDS.build.width, height / CLOUDS.build.height, depth / CLOUDS.build.depth);
    cloud.castShadow = false; // far outside the sun's shadow, which fits the figure
    cloud.receiveShadow = false;
    group.add(cloud);
  }
  return material;
}

// Fades a cloud into the sky behind it, the further off the more (CLOUDS.haze
// to all the way over CLOUDS.fade), and greys it over with the sky in fog. It
// takes no fog of its own: the scene's fog would hide it long before the sky.
// Mixed in after the lighting, before the colour is converted for the screen,
// as the land's haze is (haze.ts).
function fadeIntoSky(material: THREE.MeshStandardMaterial, uniforms: SkyUniforms): void {
  material.fog = false;
  const before = material.onBeforeCompile;
  const key = material.customProgramCacheKey;
  material.onBeforeCompile = (shader, renderer) => {
    before.call(material, shader, renderer);
    Object.assign(shader.uniforms, uniforms);
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec3 vSkyAt;')
      .replace('#include <project_vertex>', '#include <project_vertex>\nvSkyAt = ( modelMatrix * vec4( transformed, 1.0 ) ).xyz;');
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', `#include <common>\nvarying vec3 vSkyAt;\n${SKY_GLSL}`)
      .replace(
        '#include <colorspace_fragment>',
        /* glsl */ `{
          vec3 skyWay = vSkyAt - cameraPosition;
          float skyFar = length( skyWay );
          float skyFade = max( ${glsl(CLOUDS.haze)}, smoothstep( ${glsl(CLOUDS.fade[0])}, ${glsl(CLOUDS.fade[1])}, skyFar ) );
          gl_FragColor.rgb = skyFogged( mix( gl_FragColor.rgb, skyClear( skyWay / skyFar ), skyFade ) );
        }
        #include <colorspace_fragment>`,
      );
  };
  material.customProgramCacheKey = () => `${key.call(material)}|sky-cloud`;
  material.needsUpdate = true;
}
