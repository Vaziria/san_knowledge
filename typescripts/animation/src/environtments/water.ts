import * as THREE from 'three';

// Keeping water out of a boat.
//
// A water surface is a see-through sheet at the waterline, so it runs straight
// through anything that floats: the inside of a hull sits below the waterline
// and would look flooded. A floating figure marks the surfaces that must look
// dry with keepDry(). Where one of them is the surface you see, it writes DRY
// into the stencil buffer, and a water surface made with waterSurface() is not
// drawn there. Only the inside of a hull (and whatever stands inside it) needs
// this; the outside of a hull must stay unmarked, or its part under the
// waterline would look out of the water.
//
// The renderer and any render target it draws into need a stencil buffer
// (WebGLRenderer { stencil: true }, WebGLRenderTarget { stencilBuffer: true }).

const DRY = 1;

// Drawn after the other solid meshes, so a dry surface marks only the pixels
// where nothing else stands in front of it. Water is see-through, so it is
// drawn after all solid meshes anyway.
const DRY_RENDER_ORDER = 1;

// Marks the given materials as dry, and draws every mesh under root that uses
// one of them after the other solid meshes.
export function keepDry(root: THREE.Object3D, materials: THREE.Material[]): void {
  for (const material of materials) {
    material.stencilWrite = true;
    material.stencilRef = DRY;
    material.stencilFunc = THREE.AlwaysStencilFunc;
    material.stencilZPass = THREE.ReplaceStencilOp;
  }
  root.traverse((object) => {
    if (object instanceof THREE.Mesh && [object.material].flat().some((m) => materials.includes(m))) {
      object.renderOrder = DRY_RENDER_ORDER;
    }
  });
}

// Makes a material a water surface: it is not drawn where a dry surface is.
export function waterSurface(material: THREE.Material): void {
  material.stencilWrite = true; // turns the stencil test on
  material.stencilWriteMask = 0; // but leaves the buffer as it is
  material.stencilRef = DRY;
  material.stencilFunc = THREE.NotEqualStencilFunc;
}
