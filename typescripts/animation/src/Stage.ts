import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { EffectComposer } from 'three/addons/postprocessing/EffectComposer.js';
import { OutputPass } from 'three/addons/postprocessing/OutputPass.js';
import { Pass } from 'three/addons/postprocessing/Pass.js';
import { RenderPass } from 'three/addons/postprocessing/RenderPass.js';
import { ShaderPass } from 'three/addons/postprocessing/ShaderPass.js';
import { behaviours } from './behaviours';
import { clampKuwaharaRadius, createKuwaharaShader } from './effects/kuwahara';
import { drawOverlay } from './overlay';
import type { Environment, Preview } from './previews';
import { applyRendererTheme, type Theme } from './theme';

// The Three.js side of the preview: renderer, camera, orbit controls, lights,
// post-processing and the animation loop. It shows one preview at a time, in
// one environment (the floor, a lake). It changes only through show() and
// setKuwahara(), which do nothing when their value is unchanged, so they can
// be called on every settings change. React never touches it.
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

  constructor(parent: HTMLElement) {
    const { renderer, scene, camera } = this;
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.setSize(window.innerWidth, window.innerHeight);
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    applyRendererTheme(renderer);
    parent.appendChild(renderer.domElement);

    this.controls = new OrbitControls(camera, renderer.domElement);
    this.controls.enableDamping = true;

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
    window.addEventListener('resize', () => this.resize());
    behaviours.setState({ restart: () => this.restart() });

    renderer.setAnimationLoop((time) => {
      this.timer.update(time);
      // The first frame's time can be earlier than the timer's start while the
      // page is busy loading; a negative delta would run a demo backwards.
      const delta = Math.max(0, this.timer.getDelta());
      this.elapsed += delta;
      this.environment?.update(delta); // first, so a floating figure rides this frame's waves
      this.preview?.update(this.elapsed, delta);
      this.controls.update();
      if (this.kuwaharaRadius > 0) {
        this.composer.render();
      } else {
        renderer.render(scene, camera);
        drawOverlay(renderer, scene, camera);
      }
    });
  }

  // Shows a preview in an environment and a theme. Figures and environments
  // build their materials from the theme, and a preview is built for its
  // environment, so any change rebuilds both and restarts the demo. The camera
  // goes to the preview's view only for a new figure; a new environment or
  // theme keeps the view.
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
      this.camera.position.set(...preview.camera);
      this.controls.target.set(...preview.target);
      this.controls.update();
    }
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

  private resize(): void {
    this.camera.aspect = window.innerWidth / window.innerHeight;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(window.innerWidth, window.innerHeight);
    this.composer.setPixelRatio(this.renderer.getPixelRatio());
    this.composer.setSize(window.innerWidth, window.innerHeight);
    this.renderer.getDrawingBufferSize(this.kuwahara.uniforms.resolution.value);
    this.applyKuwaharaRadius();
  }
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
// once. Textures stay, since some are shared (the wood grain), except a
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
      resources.add(object.geometry);
      for (const material of [object.material].flat()) resources.add(material);
    });
  }
  for (const resource of resources) resource.dispose();
}
