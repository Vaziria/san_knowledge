import * as THREE from 'three';

// Kuwahara filter: a painterly post-process that flattens areas into patches
// of colour while keeping edges sharp. For each pixel it looks at the four
// square quadrants around it (each radius + 1 pixels wide) and takes the
// average colour of the quadrant whose colours vary least. Flat colours pass
// through unchanged, so brand colours on flat areas stay exact.
//
// `radius` is a uniform in device pixels (1 to MAX_KUWAHARA_RADIUS), so it
// can change every frame without recompiling. Use it in a ShaderPass, and keep
// `resolution` at the render target's size in pixels.
//
// Thin details that must stay legible (a fishing line) are averaged away:
// anything narrower than the radius vanishes into its surroundings. A material
// marked with keepSharp() writes SHARP_ALPHA into the picture's alpha instead
// of 1, and the filter passes those pixels through unchanged, while everything
// around them stays painterly. They are still drawn in the scene with depth,
// so they hide behind nearer things as usual. The canvas has no alpha, so
// nothing else ever sees the mark.

// Samples grow with (2 * radius + 1)^2, so the radius is capped.
export const MAX_KUWAHARA_RADIUS = 24;

const SHARP_ALPHA = 0.5;

// Marks a material's pixels to be kept out of the filter. It stays an opaque
// surface, but drawn without blending, so its opacity (SHARP_ALPHA) is
// written as the pixel's alpha.
export function keepSharp(material: THREE.Material): void {
  material.transparent = false;
  material.blending = THREE.NoBlending;
  material.opacity = SHARP_ALPHA;
}

export function clampKuwaharaRadius(radius: number): number {
  return Math.min(MAX_KUWAHARA_RADIUS, Math.max(1, Math.round(radius)));
}

export function createKuwaharaShader(radius: number) {
  return {
    name: 'KuwaharaShader',
    uniforms: {
      tDiffuse: { value: null as THREE.Texture | null },
      resolution: { value: new THREE.Vector2(1, 1) },
      radius: { value: clampKuwaharaRadius(radius) },
    },
    vertexShader: /* glsl */ `
      varying vec2 vUv;
      void main() {
        vUv = uv;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: /* glsl */ `
      uniform sampler2D tDiffuse;
      uniform vec2 resolution;
      uniform int radius;
      varying vec2 vUv;

      void main() {
        // A pixel marked sharp (alpha below 1, see keepSharp) passes through.
        // Anti-aliased edges, blended with what lies behind, count too.
        vec4 center = texture2D(tDiffuse, vUv);
        if (center.a < 0.9) {
          gl_FragColor = vec4(center.rgb, 1.0);
          return;
        }

        vec2 texel = 1.0 / resolution;
        vec3 mean[4];
        vec3 squares[4];
        for (int k = 0; k < 4; k++) {
          mean[k] = vec3(0.0);
          squares[k] = vec3(0.0);
        }

        // Quadrants: 0 lower left, 1 lower right, 2 upper left, 3 upper right.
        // The center row and column belong to the quadrants on both sides.
        for (int j = -radius; j <= radius; j++) {
          for (int i = -radius; i <= radius; i++) {
            vec3 c = texture2D(tDiffuse, vUv + vec2(float(i), float(j)) * texel).rgb;
            vec3 c2 = c * c;
            if (i <= 0 && j <= 0) { mean[0] += c; squares[0] += c2; }
            if (i >= 0 && j <= 0) { mean[1] += c; squares[1] += c2; }
            if (i <= 0 && j >= 0) { mean[2] += c; squares[2] += c2; }
            if (i >= 0 && j >= 0) { mean[3] += c; squares[3] += c2; }
          }
        }

        float count = float((radius + 1) * (radius + 1));
        float lowest = 1e20;
        vec3 color = vec3(0.0);
        for (int k = 0; k < 4; k++) {
          vec3 m = mean[k] / count;
          vec3 variance = abs(squares[k] / count - m * m);
          float v = variance.r + variance.g + variance.b;
          if (v < lowest) {
            lowest = v;
            color = m;
          }
        }
        gl_FragColor = vec4(color, center.a);
      }
    `,
  };
}
