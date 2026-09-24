import * as THREE from 'three';

// Wind that sways plants: grass blades, meadow plants, a tree's crown. An
// environment owns one Wind and changes it every frame (the lake's weather,
// Lake/Weather.ts); sway() makes a material bend in it, on the graphics card,
// so thousands of instanced tufts cost nothing more to move.
//
// A plant bends from its foot (its mesh's or instance's origin): a point
// moves downwind by `sway` meters times the square of its height over
// `reach`, so the foot stays put and the tip moves most. Gusts run across the
// land downwind, so neighbouring plants bend together, and a quicker flutter
// shakes each one a little on its own. Only the drawing moves: shadows and
// anything that reads the geometry see the plant standing still.

export interface Wind {
  time: { value: number }; // s, for the gusts' and flutter's motion
  strength: { value: number }; // 0 still air, 1 a strong wind
  direction: { value: THREE.Vector2 }; // the way it blows over the ground (x, z), a unit vector
}

export function createWind(): Wind {
  return { time: { value: 0 }, strength: { value: 0 }, direction: { value: new THREE.Vector2(1, 0) } };
}

// How a plant bends: `sway` m at `reach` m up, in a full wind.
export interface Bend {
  sway: number;
  reach: number;
}

const DECLARE = /* glsl */ `
uniform float windTime;
uniform float windStrength;
uniform vec2 windDirection;
uniform float windSway;
uniform float windReach;
`;

const BEND = /* glsl */ `
#include <begin_vertex>
{
  #ifdef USE_INSTANCING
    mat4 windModel = modelMatrix * instanceMatrix;
  #else
    mat4 windModel = modelMatrix;
  #endif
  vec3 windAt = (windModel * vec4(transformed, 1.0)).xyz;
  vec3 windFoot = (windModel * vec4(0.0, 0.0, 0.0, 1.0)).xyz;
  float windRise = clamp((windAt.y - windFoot.y) / windReach, 0.0, 1.5);
  float windGust = 0.7 + 0.3 * sin(dot(windAt.xz, windDirection) * 0.9 - windTime * 2.6);
  float windFlutter = 0.25 * sin(windTime * 7.3 + dot(windAt.xz, vec2(3.1, 2.3)));
  vec2 windPush = windStrength * (windDirection * (windGust + windFlutter) + vec2(-windDirection.y, windDirection.x) * 0.5 * windFlutter);
  vec3 windOffset = vec3(windPush.x, 0.0, windPush.y) * windSway * windRise * windRise;
  windOffset.y = -0.3 * length(windOffset.xz) * windRise; // bent over, the tip sinks a little
  transformed += inverse(mat3(windModel)) * windOffset;
}
`;

// Makes a material's meshes bend in the wind (see the top of this file).
export function sway(material: THREE.Material, wind: Wind, bend: Bend): void {
  material.onBeforeCompile = (shader) => {
    shader.uniforms.windTime = wind.time;
    shader.uniforms.windStrength = wind.strength;
    shader.uniforms.windDirection = wind.direction;
    shader.uniforms.windSway = { value: bend.sway };
    shader.uniforms.windReach = { value: bend.reach };
    shader.vertexShader = DECLARE + shader.vertexShader.replace('#include <begin_vertex>', BEND);
  };
  material.customProgramCacheKey = () => 'wind';
  material.needsUpdate = true;
}

// Every material of the meshes under a root, once each.
export function materialsOf(root: THREE.Object3D): THREE.Material[] {
  const materials = new Set<THREE.Material>();
  root.traverse((object) => {
    if (object instanceof THREE.Mesh) for (const m of [object.material].flat()) materials.add(m);
  });
  return [...materials];
}
