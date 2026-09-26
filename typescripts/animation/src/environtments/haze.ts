import * as THREE from 'three';

// Fades a material toward the sky's colour with its distance from the
// environment's middle, as far land fades into haze: nothing within `from`
// meters, `amount` at `to` and beyond (1: exactly the sky's colour). The
// land ends at its border, and faded all the way it meets the sky (the
// scene's background) with no line, from any side and height (Border.ts).
//
// The distance is measured in the object's own coordinates, or an
// instance's, which must be the environment's (the ground, the border's
// grass): x and z from its middle. The fade is mixed in after the lighting,
// before the colour is converted for the screen, so a fully faded surface is
// the sky's colour exactly, as the background is drawn, whatever the light.
// Any onBeforeCompile the material has (the soil, the wind) still runs.
//
// The sky's colour is a uniform. Given as a THREE.Color it is used as it is,
// so changing that colour later changes the fade too: the lake's sky darkens
// at night (Lake/DayNight.ts). Given any other way it stays as it was.

// How far in from its end the land starts fading into the sky.
export const EDGE_HAZE = 14; // m

// Fades a material that stands on the land toward the sky over the last
// EDGE_HAZE meters before `radius`, where the land ends: all the way for the
// ground itself, part of the way for what grows on it (`amount`).
export function hazeEdge(material: THREE.Material, sky: THREE.ColorRepresentation, radius: number, amount = 1): void {
  fadeToSky(material, { sky, from: radius - EDGE_HAZE, to: radius, amount });
}

export interface Haze {
  sky: THREE.ColorRepresentation; // the scene's background; a THREE.Color is followed as it changes
  from: number; // m from the middle where the fade begins
  to: number; // m where it is full
  amount?: number; // how far toward the sky's colour at `to`; 1 by default
}

export function fadeToSky(material: THREE.Material, haze: Haze): void {
  // Linear, as the renderer clears to it before converting.
  const sky = { value: haze.sky instanceof THREE.Color ? haze.sky : new THREE.Color(haze.sky) };
  const amount = haze.amount ?? 1;
  const glsl = (n: number) => n.toFixed(5);
  const before = material.onBeforeCompile;
  const key = material.customProgramCacheKey;
  material.onBeforeCompile = (shader, renderer) => {
    before.call(material, shader, renderer);
    shader.uniforms.skyHaze = sky;
    // Where the point is, passed on so the fade is worked out for each pixel,
    // however far apart the vertices are.
    shader.vertexShader = shader.vertexShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vSkyAt;')
      .replace(
        '#include <project_vertex>',
        /* glsl */ `#include <project_vertex>
        #ifdef USE_INSTANCING
          vSkyAt = ( instanceMatrix * vec4( transformed, 1.0 ) ).xz;
        #else
          vSkyAt = transformed.xz;
        #endif`,
      );
    shader.fragmentShader = shader.fragmentShader
      .replace('#include <common>', '#include <common>\nvarying vec2 vSkyAt;\nuniform vec3 skyHaze;')
      .replace(
        '#include <colorspace_fragment>',
        /* glsl */ `gl_FragColor.rgb = mix( gl_FragColor.rgb, skyHaze,
          ${glsl(amount)} * smoothstep( ${glsl(haze.from)}, ${glsl(haze.to)}, length( vSkyAt ) ) );
        #include <colorspace_fragment>`,
      );
  };
  material.customProgramCacheKey = () => `${key.call(material)}|sky-${haze.from}-${haze.to}-${amount}`;
  material.needsUpdate = true;
}
