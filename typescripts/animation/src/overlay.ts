import * as THREE from 'three';

// The overlay layer: objects on it (speech bubbles) are drawn after the scene
// and its effects, on top of everything, so the Kuwahara filter never smears
// their text. The stage draws the scene's layer 0 first, runs the effects,
// then draws this layer with drawOverlay(). Put an object on it with
// object.layers.set(OVERLAY_LAYER) and give its material depthTest: false,
// since it is drawn over a finished picture. The raycaster only sees layer 0,
// so overlay objects never catch a press.
export const OVERLAY_LAYER = 1;

// Draws only the overlay layer of the scene into the current render target,
// over what is already there: no clearing, no background, and the shadow maps
// are not drawn again. The background is taken away while it draws, because a
// colour background clears the picture even with autoClear off.
export function drawOverlay(renderer: THREE.WebGLRenderer, scene: THREE.Scene, camera: THREE.Camera): void {
  const layers = camera.layers.mask;
  const background = scene.background;
  const autoClear = renderer.autoClear;
  const shadows = renderer.shadowMap.autoUpdate;
  camera.layers.set(OVERLAY_LAYER);
  scene.background = null;
  renderer.autoClear = false;
  renderer.shadowMap.autoUpdate = false;
  renderer.render(scene, camera);
  camera.layers.mask = layers;
  scene.background = background;
  renderer.autoClear = autoClear;
  renderer.shadowMap.autoUpdate = shadows;
}
