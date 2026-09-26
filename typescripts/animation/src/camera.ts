import * as THREE from 'three';
import type { Preview } from './previews';

// The camera's direction, the Camera tab's View menu and ?camera=<name>:
// which way the camera looks at the shown figure. "figure" is the view the
// figure's preview starts from; the eight around it are named from the
// figure's side (it faces +z, and its left is -x); "top" looks straight down,
// its front toward the bottom of the screen. "walk" walks the camera round
// the scene at eye height, from where it is (the stage's walking, steered by
// walk.ts). "free" is wherever the view was dragged: the stage reports it
// (Stage.onDirectionLost), so picking a direction again after dragging away
// from it moves the camera back.
//
// A direction keeps the preview's target, its distance and, round the sides,
// the height it looks from, so a figure is framed as its preview frames it:
// "front" of a tree stands as far back as the tree's own view. The stage
// glides the camera there (Stage.setCameraDirection).

export const CAMERA_DIRECTIONS = [
  { value: 'figure', label: "figure's own" },
  { value: 'front', label: 'front' },
  { value: 'front-right', label: 'front right' },
  { value: 'right', label: 'right' },
  { value: 'back-right', label: 'back right' },
  { value: 'back', label: 'back' },
  { value: 'back-left', label: 'back left' },
  { value: 'left', label: 'left' },
  { value: 'front-left', label: 'front left' },
  { value: 'top', label: 'top' },
  { value: 'walk', label: 'walk' },
  { value: 'free', label: 'free (dragged)' },
] as const;

export type CameraDirection = (typeof CAMERA_DIRECTIONS)[number]['value'];

export function isCameraDirection(value: string): value is CameraDirection {
  return CAMERA_DIRECTIONS.some((d) => d.value === value);
}

// Round the figure: radians from +z (in front) toward +x (its right), as
// THREE.Spherical measures theta.
const AROUND: Partial<Record<CameraDirection, number>> = {
  front: 0,
  'front-right': Math.PI / 4,
  right: Math.PI / 2,
  'back-right': (3 * Math.PI) / 4,
  back: Math.PI,
  'back-left': (-3 * Math.PI) / 4,
  left: -Math.PI / 2,
  'front-left': -Math.PI / 4,
};

// How high the side views look from, as the angle down from straight up
// (THREE.Spherical's phi): the preview's own, kept between these, so the
// ground shows but the figure is still seen from the side.
const SIDE_PHI = { least: THREE.MathUtils.degToRad(30), most: THREE.MathUtils.degToRad(83) };
// Straight down, all but: a camera looking exactly down has no "up" to keep.
const TOP_PHI = THREE.MathUtils.degToRad(0.5);

export interface CameraView {
  target: THREE.Vector3;
  offset: THREE.Spherical; // the camera, from the target
}

// Where a direction puts the camera for a preview. "free" and "walk" have no
// place of their own, so they are the preview's view, where the camera goes
// for a new figure (and starts walking from).
export function cameraView(direction: CameraDirection, preview: Preview): CameraView {
  const target = new THREE.Vector3(...preview.target);
  const offset = new THREE.Spherical().setFromVector3(new THREE.Vector3(...preview.camera).sub(target));
  if (direction === 'top') {
    offset.theta = 0;
    offset.phi = TOP_PHI;
  } else {
    const around = AROUND[direction];
    if (around !== undefined) {
      offset.theta = around;
      offset.phi = THREE.MathUtils.clamp(offset.phi, SIDE_PHI.least, SIDE_PHI.most);
    }
  }
  return { target, offset };
}
