import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { Pass } from 'three/addons/postprocessing/Pass.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { ShaderPass } from 'three/addons/postprocessing/ShaderPass.js';
import { behaviours } from './behaviours';
import { cameraView, type CameraDirection, type CameraView } from './camera';
import { clampKuwaharaRadius, createKuwaharaShader } from './effects/kuwahara';
import { drawOverlay } from './overlay';
import type { Environment, Preview } from './previews';
import { applyRendererTheme, type Theme } from './theme';

// The Three.js side of the preview: renderer, camera, orbit controls, lights,
// post-processing and the animation loop. It shows one preview at a time, in
// one environment (the floor, a lake). It changes only through show(),
// setCameraDirection(), setKuwahara() and setDrawing(), which do nothing when
// their value is unchanged, so they can be called on every settings change.
// React never touches it.
export class Stage {
  // The stencil buffer keeps a lake's water out of a floating boat's hull
  // (environtments/water.ts).
  private readonly renderer = new THREE.WebGLRenderer({ antialias: true, stencil: true });
  private readonly scene = new THREE.Scene();
  private readonly camera = new THREE.PerspectiveCamera(50, window.innerWidth / window.innerHeight, 0.1, 100);
  private readonly controls: OrbitControls;
  private readonly hemisphere = new THREE.HemisphereLight();
  private readonly light = new THREE.DirectionalLight();
  private readonly composer: EffectComposer;
  private readonly kuwahara: ShaderPass;
  private readonly timer = new THREE.Timer();
  private readonly raycaster = new THREE.Raycaster();
  private readonly pressed = new Map<number, () => void>(); // pointer id -> lets go of what it presses

  private create: ((theme: Theme, environment: Environment) => Preview) | null = null;
  private createEnvironment: ((theme: Theme) => Environment) | null = null;
  private theme: Theme | null = null;
  private preview: Preview | null = null;
  private environment: Environment | null = null;
  private elapsed = 0; // seconds since the preview was shown
  private kuwaharaRadius = 0; // CSS pixels; 0 = off
  private direction: CameraDirection = 'figure';
  private goal: CameraView | null = null; // where the camera is gliding to, if anywhere
  private readonly fixedSize: Size | null; // drawn at this size, not the window's
  private drawing = true;

  // Called when the view is dragged away from the picked camera direction,
  // so the settings can say it is free (main.tsx).
  onDirectionLost: (() => void) | null = null;
  // Called after each frame is drawn, while the picture is on the canvas
  // (streamRenderer.ts takes a copy for the panel).
  onFrame: (() => void) | null = null;

  // Draws at the window's size, sharp on high-density screens, or at a fixed
  // size one pixel to a pixel: the stream's, in the dev server's hidden
  // browser (streamRenderer.ts).
  constructor(parent: HTMLElement, fixedSize?: Size) {
    const { renderer, scene, camera } = this;
    this.fixedSize = fixedSize ?? null;
    renderer.setPixelRatio(fixedSize ? 1 : Math.min(window.devicePixelRatio, 2));
    const { width, height } = this.viewSize();
    renderer.setSize(width, height);
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    applyRendererTheme(renderer);
    parent.appendChild(renderer.domElement);

    this.controls = new OrbitControls(camera, renderer.domElement);
    this.controls.enableDamping = true;
    // Taking hold of the view (a drag, the wheel, a pinch) stops a glide to a
    // picked direction; once it has turned away from it, the view is free.
    this.controls.addEventListener('start', () => this.stopGlide());
    this.controls.addEventListener('change', () => this.checkDirection());

    // Pressing a part of the figure you can play (a piano key) plays it
    // instead of turning the view. The capture phase runs this before the
    // orbit controls' own listener, so it can keep the press from them.
    const canvas = renderer.domElement;
    canvas.addEventListener('pointerdown', (event) => this.pointerDown(event), { capture: true });
    canvas.addEventListener('pointerup', (event) => this.pointerUp(event));
    canvas.addEventListener('pointercancel', (event) => this.pointerUp(event));

    scene.add(this.hemisphere);
    this.light.castShadow = true;
    this.light.shadow.mapSize.set(2048, 2048);
    scene.add(this.light); // placed and fitted to each figure by fitShadow()

    // The painterly Kuwahara filter. The scene renders into a multisampled
    // target, the filter runs on it, and OutputPass converts to sRGB as the
    // renderer would on its own. When the filter is off, the scene renders
    // directly.
    const target = new THREE.WebGLRenderTarget(1, 1, { type: THREE.HalfFloatType, samples: 4, stencilBuffer: true });
    this.composer = new EffectComposer(renderer, target);
    this.composer.addPass(new RenderPass(scene, camera));
    this.kuwahara = new ShaderPass(createKuwaharaShader(1));
    this.composer.addPass(this.kuwahara);
    this.composer.addPass(new OverlayPass(scene, camera)); // after the filter, so bubble text stays sharp
    this.composer.addPass(new OutputPass());

    this.resize();
    if (!fixedSize) window.addEventListener('resize', () => this.resize());
    behaviours.setState({ restart: () => this.restart() });

    renderer.setAnimationLoop((time) => this.frame(time));
  }

  // One frame: the environment and the demo move on, the camera glides, and
  // the scene is drawn.
  private frame(time: number): void {
    const { renderer, scene, camera } = this;
    this.timer.update(time);
    // The first frame's time can be earlier than the timer's start while the
    // page is busy loading; a negative delta would run a demo backwards.
    const delta = Math.max(0, this.timer.getDelta());
    this.elapsed += delta;
    this.environment?.update(delta); // first, so a floating figure rides this frame's waves
    this.preview?.update(this.elapsed, delta);
    if (this.goal) this.glide(delta);
    this.controls.update();
    if (this.kuwaharaRadius > 0) {
      this.composer.render();
    } else {
      renderer.render(scene, camera);
      drawOverlay(renderer, scene, camera);
    }
    this.onFrame?.();
  }

  // Draws the scene, or stops drawing it and hides the canvas: with the
  // Stream tab's "stream only", the dev server draws the stream and this page
  // doesn't (main.tsx). Figures are still built while it rests, so the panel
  // lists their behaviours, and the demo goes on from where it rested.
  setDrawing(on: boolean): void {
    if (on === this.drawing) return;
    this.drawing = on;
    this.renderer.domElement.style.visibility = on ? '' : 'hidden';
    if (on) this.timer.reset(); // not a jump by the time it rested
    this.renderer.setAnimationLoop(on ? (time) => this.frame(time) : null);
  }

  // Shows a preview in an environment and a theme. Figures and environments
  // build their materials from the theme, and a preview is built for its
  // environment, so any change rebuilds both and restarts the demo. The camera
  // goes to the preview's view only for a new figure, from the picked camera
  // direction; a new environment or theme keeps the view.
  show(
    create: (theme: Theme, environment: Environment) => Preview,
    createEnvironment: (theme: Theme) => Environment,
    theme: Theme,
  ): void {
    if (create === this.create && createEnvironment === this.createEnvironment && theme === this.theme) return;
    const newFigure = create !== this.create;
    this.create = create;
    this.createEnvironment = createEnvironment;
    this.theme = theme;

    for (const release of this.pressed.values()) release();
    this.pressed.clear();
    if (this.preview && this.environment) {
      this.preview.dispose?.();
      this.scene.remove(this.preview.figure, this.environment.scenery);
      dispose(this.preview.figure, ...(this.preview.props ?? []), this.environment.scenery);
    }
    const environment = createEnvironment(theme);
    const preview = create(theme, environment);
    // The figure stays at the origin: the scenery moves so that the figure's
    // spot, on land or in the water, is there.
    const spot = preview.place === 'water' && environment.water ? environment.water.at : environment.land;
    environment.scenery.position.copy(spot).negate();
    this.scene.add(environment.scenery, preview.figure);
    this.scene.fog = environment.fog ?? null; // the lake's comes and goes; the other environments have none
    this.environment = environment;
    this.preview = preview;
    this.fitShadow(preview.figure);
    this.elapsed = 0;
    behaviours.setState({ behaviours: preview.behaviours ?? [] }); // for the panel's buttons

    const s = theme.scene;
    this.scene.background = new THREE.Color(s.background);
    this.hemisphere.color.set(s.sky);
    this.hemisphere.groundColor.set(s.ground);
    this.hemisphere.intensity = s.ambientIntensity;
    this.light.color.set(s.light);
    this.light.intensity = s.lightIntensity;

    if (newFigure) {
      this.stopGlide();
      this.place(cameraView(this.direction, preview));
    }
  }

  // Which way the camera looks at the figure (camera.ts). A new direction
  // glides the camera there, round the preview's target, over about half a
  // second; "free" leaves it where it is. Before anything is shown it only
  // keeps the direction, so the first figure appears from it at once.
  setCameraDirection(direction: CameraDirection): void {
    if (direction === this.direction) return;
    this.direction = direction;
    if (direction === 'free' || !this.preview) return;
    this.goal = cameraView(direction, this.preview);
    // Without damping, an update applies what is left of a drag at once, so
    // the drag's own glide doesn't carry on against this one.
    this.controls.enableDamping = false;
    this.controls.update();
  }

  // Puts the camera at a view at once.
  private place(view: CameraView): void {
    this.controls.target.copy(view.target);
    this.camera.position.setFromSpherical(view.offset).add(view.target);
    this.controls.update();
  }

  // Moves the camera a step toward the goal: round the target the short way,
  // up or down, nearer or further, easing in as it arrives, so it takes about
  // the same time at any frame rate.
  private glide(delta: number): void {
    const goal = this.goal!;
    const t = 1 - Math.exp(-GLIDE * delta);
    const target = this.controls.target;
    const now = new THREE.Spherical().setFromVector3(this.camera.position.clone().sub(target));
    const turn = shortest(goal.offset.theta - now.theta);
    now.theta += turn * t;
    now.phi += (goal.offset.phi - now.phi) * t;
    now.radius += (goal.offset.radius - now.radius) * t;
    target.lerp(goal.target, t);
    const close =
      Math.abs(turn) < ARRIVED &&
      Math.abs(goal.offset.phi - now.phi) < ARRIVED &&
      Math.abs(goal.offset.radius - now.radius) < ARRIVED * goal.offset.radius &&
      target.distanceTo(goal.target) < ARRIVED * goal.offset.radius;
    if (close) {
      this.place(goal);
      this.stopGlide();
    } else {
      this.camera.position.setFromSpherical(now).add(target);
    }
  }

  private stopGlide(): void {
    this.goal = null;
    this.controls.enableDamping = true;
  }

  // After the view moves: if it has been turned or tipped away from the
  // picked direction (by a drag; zooming and panning keep the direction), the
  // direction is lost and the view is free.
  private checkDirection(): void {
    if (this.goal || this.direction === 'free' || !this.preview) return;
    const view = cameraView(this.direction, this.preview);
    const now = new THREE.Spherical().setFromVector3(this.camera.position.clone().sub(this.controls.target));
    if (Math.abs(shortest(view.offset.theta - now.theta)) < LOST && Math.abs(view.offset.phi - now.phi) < LOST) return;
    this.direction = 'free';
    this.onDirectionLost?.();
  }

  // Builds the shown preview again, restarting its demo after the panel's
  // behaviour buttons stopped it. The view stays where it is.
  restart(): void {
    const { create, createEnvironment, theme } = this;
    if (!create || !createEnvironment || !theme) return;
    this.theme = null; // so show() doesn't skip it as unchanged
    this.show(create, createEnvironment, theme);
  }

  // Fits the sun's shadow to the figure, from small (a keyboard) to tall (a
  // mast): the shadow camera covers the figure's footprint and the shadow a
  // tall part throws (about 0.65 x its height at this sun angle), and the sun
  // sits far enough out to be above the figure's top. At least ±0.52 m, so a
  // walking penguin stays inside it. The offsets that keep surfaces from
  // shadowing themselves in stripes (acne) grow with it, because each shadow
  // texel then covers more of the figure.
  private fitShadow(figure: THREE.Object3D): void {
    const bounds = new THREE.Box3().setFromObject(figure);
    const footprint = Math.max(...[bounds.min.x, bounds.max.x, bounds.min.z, bounds.max.z].map(Math.abs));
    const reach = 1.15 * Math.max(0.45, footprint, 0.65 * bounds.max.y);
    const { light } = this;
    const shadow = light.shadow.camera;
    light.position.set(0.6, 1, 0.6).multiplyScalar(Math.max(1, 2 * bounds.max.y));
    shadow.left = shadow.bottom = -reach;
    shadow.right = shadow.top = reach;
    shadow.far = light.position.length() + 2 * reach;
    shadow.updateProjectionMatrix();
    light.shadow.bias = -0.0005;
    light.shadow.normalBias = 0.01 * reach;
  }

  // A press on the figure's first part under the pointer. A part the preview
  // presses stays pressed until this pointer lifts; any other press is left
  // to the orbit controls.
  private pointerDown(event: PointerEvent): void {
    if (!this.preview?.press || event.button !== 0) return;
    const canvas = this.renderer.domElement;
    const box = canvas.getBoundingClientRect();
    const pointer = new THREE.Vector2(
      ((event.clientX - box.left) / box.width) * 2 - 1,
      -((event.clientY - box.top) / box.height) * 2 + 1,
    );
    this.raycaster.setFromCamera(pointer, this.camera);
    const hit = this.raycaster.intersectObject(this.preview.figure)[0];
    const release = hit ? this.preview.press(hit.object) : null;
    if (!release) return;
    event.stopImmediatePropagation();
    canvas.setPointerCapture(event.pointerId); // so its release arrives here, wherever it lifts
    this.pressed.set(event.pointerId, release);
  }

  private pointerUp(event: PointerEvent): void {
    this.pressed.get(event.pointerId)?.();
    this.pressed.delete(event.pointerId);
  }

  // The canvas the scene is drawn on, for streaming it (stream.ts). The panel
  // is not on it.
  get canvas(): HTMLCanvasElement {
    return this.renderer.domElement;
  }

  // What draws it: the WebGL renderer, as "ANGLE (NVIDIA, NVIDIA GeForce RTX
  // 3050 …, D3D11)", or SwiftShader's name when it is drawn in software.
  get graphics(): string {
    const gl = this.renderer.getContext();
    const info = gl.getExtension('WEBGL_debug_renderer_info');
    return String(gl.getParameter(info ? info.UNMASKED_RENDERER_WEBGL : gl.RENDERER));
  }

  // The Kuwahara radius in CSS pixels; 0 turns the filter off. The radius is
  // a shader uniform, so it changes live without recompiling.
  setKuwahara(radius: number): void {
    this.kuwaharaRadius = radius;
    this.applyKuwaharaRadius();
  }

  // Radius in device pixels, so the patches look the same size on any screen.
  private applyKuwaharaRadius(): void {
    if (this.kuwaharaRadius <= 0) return;
    this.kuwahara.uniforms.radius.value = clampKuwaharaRadius(this.kuwaharaRadius * this.renderer.getPixelRatio());
  }

  private viewSize(): Size {
    return this.fixedSize ?? { width: window.innerWidth, height: window.innerHeight };
  }

  private resize(): void {
    const { width, height } = this.viewSize();
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height);
    this.composer.setPixelRatio(this.renderer.getPixelRatio());
    this.composer.setSize(width, height);
    this.renderer.getDrawingBufferSize(this.kuwahara.uniforms.resolution.value);
    this.applyKuwaharaRadius();
  }
}

export interface Size {
  width: number;
  height: number; // CSS pixels
}

const GLIDE = 7; // how fast the camera glides to a picked direction, per second
const ARRIVED = 1e-3; // radians (and shares of the distance) from the goal where the glide ends
const LOST = THREE.MathUtils.degToRad(0.5); // radians turned or tipped away that leave a direction

// An angle's difference the short way round, between -pi and pi.
function shortest(angle: number): number {
  return THREE.MathUtils.euclideanModulo(angle + Math.PI, 2 * Math.PI) - Math.PI;
}

// Draws the overlay layer (speech bubbles) over the picture the passes before
// it made, before OutputPass converts it to sRGB.
class OverlayPass extends Pass {
  private readonly scene: THREE.Scene;
  private readonly camera: THREE.Camera;

  constructor(scene: THREE.Scene, camera: THREE.Camera) {
    super();
    this.scene = scene;
    this.camera = camera;
    this.needsSwap = false;
  }

  render(renderer: THREE.WebGLRenderer, _writeBuffer: THREE.WebGLRenderTarget, readBuffer: THREE.WebGLRenderTarget): void {
    renderer.setRenderTarget(this.renderToScreen ? null : readBuffer);
    drawOverlay(renderer, this.scene, this.camera);
  }
}

// Frees the GPU memory of a figure, its props and its scenery once they are no
// longer shown. Meshes share geometries and materials, so each one is freed
// once, and so is an instanced mesh's own list of instances (the lawn, the
// meadow). Textures stay, since some are shared (the wood grain), except a
// sprite's own drawing (a speech bubble).
function dispose(...roots: THREE.Object3D[]): void {
  const resources = new Set<{ dispose(): void }>();
  for (const root of roots) {
    root.traverse((object) => {
      if (object instanceof THREE.Sprite) {
        resources.add(object.material);
        if (object.material.map) resources.add(object.material.map);
      }
      if (!(object instanceof THREE.Mesh)) return;
      if (object instanceof THREE.InstancedMesh) resources.add(object); // its instances' matrices and colours
      resources.add(object.geometry);
      for (const material of [object.material].flat()) resources.add(material);
    });
  }
  for (const resource of resources) resource.dispose();
}
