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
import type { Environment, Preview, TimeOfDay } from './previews';
import { SUN } from './sun';
import { applyRendererTheme, sceneAt, type SceneAt, type Theme } from './theme';
import type { Move } from './walk';

// The Three.js side of the preview: renderer, camera, orbit controls, lights,
// post-processing and the animation loop. It shows one preview at a time, in
// one environment (the floor, a lake). It changes only through show(),
// setCameraDirection(), setWalk(), setMoves(), setKuwahara(), setTimeOfDay()
// and setDrawing(), which do nothing when their value is unchanged, so they can
// be called on every settings change. React never touches it. The camera
// turns round the figure (the orbit controls, and gliding to a picked
// direction), or walks round the scene at eye height (camera "walk"). A story
// can move it itself (Preview.shot): while the view is the figure's own, the
// camera follows the story's shots. Dragging the view, picking a direction or
// walking hands it to the viewer, until the story claims it back (claim()).
export class Stage {
  // The stencil buffer keeps a lake's water out of a floating boat's hull
  // (environtments/water.ts).
  private readonly renderer = new THREE.WebGLRenderer({ antialias: true, stencil: true });
  private readonly scene = new THREE.Scene();
  // Sees 3 km: the lake's clouds drift up to 2.5 km off (environtments/Sky.ts).
  // The near plane follows the camera in close (fitNear()).
  private readonly camera = new THREE.PerspectiveCamera(50, window.innerWidth / window.innerHeight, NEAR, 3000);
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
  private walker: Walker | null = null; // where the camera walks, while it walks
  private moves = new Set<Move>(); // held down to walk (walk.ts)
  private walkSpeed = 1.4; // m/s
  private eyeHeight = 1.6; // m over the ground
  private looking: { pointer: number; x: number; y: number } | null = null; // a drag turning the walking view
  private holding = false; // the orbit controls are held (a drag), from their start to their end
  private claimed = false; // the story claimed the camera back, and it waits for the view to be let go
  private shadowFrom = new THREE.Vector3(); // the sun from the shadow's middle (fitShadow)
  private time: TimeOfDay = 'cycle'; // the Time of day setting, for an environment with night
  private daylight: SceneAt | null = null; // the sky and lights now, at night mixed toward the theme's night

  // Called when the view is dragged away from the picked camera direction,
  // so the settings can say it is free (main.tsx).
  onDirectionLost: (() => void) | null = null;
  // Called when a story claims the camera back from the viewer, so the
  // settings can say the view is the figure's own again (main.tsx).
  onDirectionTaken: (() => void) | null = null;
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
    this.controls.addEventListener('start', () => {
      this.holding = true;
      this.stopGlide();
      // A story that moves the camera itself lets go of it once it is
      // taken hold of; its view is then free, until it is picked again.
      if (this.directed) {
        this.direction = 'free';
        this.onDirectionLost?.();
      }
    });
    this.controls.addEventListener('end', () => (this.holding = false));
    this.controls.addEventListener('change', () => this.checkDirection());

    // Pressing a part of the figure you can play (a piano key) plays it
    // instead of turning the view. The capture phase runs this before the
    // orbit controls' own listener, so it can keep the press from them.
    const canvas = renderer.domElement;
    canvas.addEventListener('pointerdown', (event) => this.pointerDown(event), { capture: true });
    canvas.addEventListener('pointerup', (event) => this.pointerUp(event));
    canvas.addEventListener('pointercancel', (event) => this.pointerUp(event));
    // While walking, a drag turns the view and the wheel steps; the orbit
    // controls rest.
    canvas.addEventListener('pointerdown', (event) => this.lookStart(event));
    canvas.addEventListener('pointermove', (event) => this.lookMove(event));
    canvas.addEventListener('wheel', (event) => this.wheel(event), { passive: false });

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

  // One frame: the environment and the demo move on, the camera glides or
  // walks, and the scene is drawn.
  private frame(time: number): void {
    const { renderer, scene, camera } = this;
    this.timer.update(time);
    // The first frame's time can be earlier than the timer's start while the
    // page is busy loading; a negative delta would run a demo backwards.
    const delta = Math.max(0, this.timer.getDelta());
    this.elapsed += delta;
    this.environment?.update(delta); // first, so a floating figure rides this frame's waves
    this.applyDaylight();
    this.preview?.update(this.elapsed, delta);
    if (this.preview?.shot && !this.directed) this.claim();
    if (this.walker) {
      this.walk(delta);
    } else {
      if (this.directed) this.follow(delta);
      else if (this.goal) this.glide(delta);
      this.controls.update();
    }
    if (this.preview?.followShadow) this.moveShadow();
    this.fitNear();
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
    environment.dayNight?.set(this.time); // before its first frame, so it is built into it
    this.environment = environment;
    this.preview = preview;
    this.claimed = false; // the old story's
    this.fitShadow(preview.bounds ?? new THREE.Box3().setFromObject(preview.figure));
    this.elapsed = 0;
    behaviours.setState({ behaviours: preview.behaviours ?? [] }); // for the panel's buttons

    const s = theme.scene;
    this.scene.background = new THREE.Color(s.background);
    this.hemisphere.color.set(s.sky);
    this.hemisphere.groundColor.set(s.ground);
    this.hemisphere.intensity = s.ambientIntensity;
    this.light.color.set(s.light);
    this.light.intensity = s.lightIntensity;
    this.applyDaylight();

    // A walking camera walks on where it is.
    if (newFigure && !this.walker) {
      this.stopGlide();
      this.place(cameraView(this.direction, preview));
      if (this.direction === 'walk') this.startWalking();
    }
  }

  // The Time of day setting: day and night taking turns, or held at one, in
  // an environment that has night (the lake); the others stay in day.
  setTimeOfDay(time: TimeOfDay): void {
    if (time === this.time) return;
    this.time = time;
    this.environment?.dayNight?.set(time);
  }

  // Dims the lights and darkens the background by how far the environment's
  // light is toward night (Environment.dayNight: the night, and a shower's
  // cloud), mixing the theme's day and night values (sceneAt()). An environment without night keeps the day show()
  // set.
  private applyDaylight(): void {
    const dim = this.environment?.dayNight?.dim;
    if (dim === undefined || !this.theme) return;
    const s = (this.daylight = sceneAt(this.theme, dim, this.daylight ?? undefined));
    if (this.scene.background instanceof THREE.Color) this.scene.background.copy(s.background);
    this.hemisphere.color.copy(s.sky);
    this.hemisphere.groundColor.copy(s.ground);
    this.hemisphere.intensity = s.ambientIntensity;
    this.light.color.copy(s.light);
    this.light.intensity = s.lightIntensity;
  }

  // Which way the camera looks at the figure (camera.ts). A new direction
  // glides the camera there, round the preview's target, over about half a
  // second; "free" leaves it where it is. Before anything is shown it only
  // keeps the direction, so the first figure appears from it at once.
  setCameraDirection(direction: CameraDirection): void {
    if (direction === this.direction) return;
    // The walk ends while the direction still says walk, so the orbit
    // controls taking over isn't taken for a drag away from the new one.
    if (direction !== 'walk') this.stopWalking();
    this.direction = direction;
    if (direction === 'walk') {
      if (this.preview) this.startWalking();
      return;
    }
    if (direction === 'free' || !this.preview || this.directed) return; // a story's own view: it follows the story (follow())
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

  // The near plane is a tenth of the way to what the camera turns round, and
  // at most 10 cm: so a figure a few centimeters long (the firefly) can be
  // seen from close up without being cut away, while every view from a meter
  // or more, and a walking camera, keeps 10 cm and the depth precision the
  // 3 km far plane needs.
  private fitNear(): void {
    const distance = this.walker ? Infinity : this.camera.position.distanceTo(this.controls.target);
    const near = THREE.MathUtils.clamp(distance / 10, NEAREST, NEAR);
    if (near === this.camera.near) return;
    this.camera.near = near;
    this.camera.updateProjectionMatrix();
  }

  // Whether the shown story moves the camera itself now: it has shots, and
  // the view is the figure's own (not a picked direction, free or walking).
  private get directed(): boolean {
    return this.direction === 'figure' && !this.walker && this.preview?.shot !== undefined;
  }

  // Moves the camera toward the story's shot this frame, easing there as a
  // glide does, or at once for a cut. The shot moves with what it shows (an
  // animal walking in), and the camera keeps following it.
  private follow(delta: number): void {
    const shot = this.preview!.shot!();
    const t = shot.cut ? 1 : 1 - Math.exp(-SHOT_EASE * delta);
    this.camera.position.lerp(shot.camera, t);
    this.controls.target.lerp(shot.target, t);
  }

  // While the viewer has the camera (dragged, a picked direction, walking),
  // the story's shot can claim it back when it has something new to show (a
  // comment in the lake meeting), so nothing is missed. It waits while the
  // view is held (a drag, or turning the walking view); then the view is the
  // figure's own again, the settings are told, and the camera follows the
  // story. From over CLAIM_CUT m off it cuts there: gliding across the lake
  // would take longer than a turn waits, through whatever stands between.
  private claim(): void {
    const shot = this.preview!.shot!();
    if (shot.claim) this.claimed = true;
    if (!this.claimed || this.holding || this.looking) return;
    this.claimed = false;
    this.stopWalking();
    this.stopGlide();
    this.direction = 'figure';
    // What is left of a drag's own glide is spent at once, so it doesn't
    // carry on against the story's.
    this.controls.enableDamping = false;
    this.controls.update();
    this.controls.enableDamping = true;
    this.onDirectionTaken?.();
    if (shot.cut || this.camera.position.distanceTo(shot.camera) > CLAIM_CUT) {
      this.camera.position.copy(shot.camera);
      this.controls.target.copy(shot.target);
    }
  }

  // How fast the walking camera goes (m/s) and how high over the ground its
  // eyes are (m).
  setWalk(speed: number, eyeHeight: number): void {
    this.walkSpeed = speed;
    this.eyeHeight = eyeHeight;
  }

  // What is held down to walk (walk.ts).
  setMoves(moves: readonly Move[]): void {
    this.moves = new Set(moves);
  }

  // Walks from where the camera is, the way it looks, at eye height over the
  // ground or the water; the camera glides down there (walk()).
  private startWalking(): void {
    const { camera, controls } = this;
    const look = controls.target.clone().sub(camera.position);
    const position = camera.position.clone();
    this.keepOnGround(position);
    position.y = this.groundAt(position.x, position.z) + this.eyeHeight;
    // Still looking at what it looked at, from the eyes' height.
    const pitch = Math.atan2(controls.target.y - position.y, Math.hypot(look.x, look.z));
    this.walker = { position, yaw: Math.atan2(-look.x, -look.z), pitch: upOrDown(pitch) };
    this.goal = null;
    this.looking = null;
    controls.enabled = false;
  }

  // Back to turning round a point: the one a few meters ahead of the camera.
  private stopWalking(): void {
    if (!this.walker) return;
    this.walker = null;
    this.looking = null;
    const ahead = new THREE.Vector3(0, 0, -TURN_ROUND).applyQuaternion(this.camera.quaternion);
    this.controls.target.copy(this.camera.position).add(ahead);
    this.controls.enabled = true;
    this.controls.update();
  }

  // A frame of walking: it turns, looks up or down and moves by what is held,
  // on the ground, and the camera follows closely, which also smooths the
  // ground's bumps and the glide down from where it was.
  private walk(delta: number): void {
    const walker = this.walker!;
    const held = (move: Move) => (this.moves.has(move) ? 1 : 0);
    walker.yaw += (held('turnLeft') - held('turnRight')) * TURN_SPEED * delta;
    walker.pitch = upOrDown(walker.pitch + (held('lookUp') - held('lookDown')) * LOOK_SPEED * delta);
    const ahead = held('forward') - held('back');
    const aside = held('right') - held('left');
    if (ahead || aside) {
      const step = (this.walkSpeed * (held('run') ? RUN : 1) * delta) / Math.hypot(ahead, aside);
      this.stepWalker(ahead * step, aside * step);
    }
    walker.position.y = this.groundAt(walker.position.x, walker.position.z) + this.eyeHeight;
    this.camera.position.lerp(walker.position, 1 - Math.exp(-FOLLOW * delta));
    const heading = new THREE.Quaternion().setFromEuler(new THREE.Euler(walker.pitch, walker.yaw, 0, 'YXZ'));
    this.camera.quaternion.slerp(heading, 1 - Math.exp(-FOLLOW_TURN * delta));
  }

  // Moves the walker ahead (the way it faces) and aside (to its right), in
  // meters, keeping it on the ground.
  private stepWalker(ahead: number, aside: number): void {
    const { position, yaw } = this.walker!;
    position.x += -Math.sin(yaw) * ahead + Math.cos(yaw) * aside;
    position.z += -Math.cos(yaw) * ahead - Math.sin(yaw) * aside;
    this.keepOnGround(position);
  }

  // The height the walking camera stands at, under a point: the ground's or
  // the still water's (Environment.ground), or the scenery's level.
  private groundAt(x: number, z: number): number {
    const environment = this.environment;
    if (!environment) return 0;
    const at = environment.scenery.position; // moved so that the figure's spot is at the origin
    return (environment.ground?.heightAt(x - at.x, z - at.z) ?? 0) + at.y;
  }

  // Keeps a point within the ground's reach of the scenery's middle.
  private keepOnGround(point: THREE.Vector3): void {
    const environment = this.environment;
    const reach = environment?.ground?.reach;
    if (!environment || reach === undefined) return;
    const at = environment.scenery.position;
    const x = point.x - at.x;
    const z = point.z - at.z;
    const far = Math.hypot(x, z);
    if (far <= reach) return;
    point.x = at.x + (x * reach) / far;
    point.z = at.z + (z * reach) / far;
  }

  // Dragging the walking view turns it, the way dragging a photo sphere
  // does: the ground under the pointer stays under it.
  private lookStart(event: PointerEvent): void {
    if (!this.walker || this.looking || !event.isPrimary) return;
    this.looking = { pointer: event.pointerId, x: event.clientX, y: event.clientY };
    this.renderer.domElement.setPointerCapture(event.pointerId);
  }

  private lookMove(event: PointerEvent): void {
    const { looking, walker } = this;
    if (!looking || !walker || event.pointerId !== looking.pointer) return;
    const perPixel = THREE.MathUtils.degToRad(this.camera.fov) / Math.max(1, this.renderer.domElement.clientHeight);
    walker.yaw += (event.clientX - looking.x) * perPixel;
    walker.pitch = upOrDown(walker.pitch + (event.clientY - looking.y) * perPixel);
    looking.x = event.clientX;
    looking.y = event.clientY;
  }

  // The wheel steps the walking camera ahead or back.
  private wheel(event: WheelEvent): void {
    if (!this.walker || event.deltaY === 0) return;
    event.preventDefault();
    this.stepWalker(-Math.sign(event.deltaY) * WHEEL_STEP, 0);
  }

  // After the view moves: if it has been turned or tipped away from the
  // picked direction (by a drag; zooming and panning keep the direction), the
  // direction is lost and the view is free.
  private checkDirection(): void {
    if (this.goal || this.direction === 'free' || this.direction === 'walk' || !this.preview || this.directed) return;
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
  private fitShadow(bounds: THREE.Box3): void {
    const footprint = Math.max(...[bounds.min.x, bounds.max.x, bounds.min.z, bounds.max.z].map(Math.abs));
    const reach = 1.15 * Math.max(0.45, footprint, 0.65 * bounds.max.y);
    const { light } = this;
    const shadow = light.shadow.camera;
    light.position.copy(SUN).multiplyScalar(Math.max(1, 2 * bounds.max.y));
    shadow.left = shadow.bottom = -reach;
    shadow.right = shadow.top = reach;
    shadow.far = light.position.length() + 2 * reach;
    shadow.updateProjectionMatrix();
    light.shadow.bias = -0.0005;
    light.shadow.normalBias = 0.01 * reach;
    this.shadowFrom = light.position.clone(); // from the shadow's middle, which moveShadow() moves
    light.target.position.set(0, 0, 0);
    light.target.updateMatrixWorld();
  }

  // For a preview whose shadow follows the view (Preview.followShadow): the
  // shadow, at the size fitShadow() gave it, moves to what the camera looks
  // at (a meter ahead of a walking camera, on the ground), in whole steps of
  // its texels so that its edges don't shimmer as the camera moves.
  private moveShadow(): void {
    const focus = new THREE.Vector3();
    if (this.walker) this.camera.getWorldDirection(focus).setY(0).normalize().multiplyScalar(3).add(this.camera.position);
    else focus.copy(this.controls.target);
    const shadow = this.light.shadow.camera;
    const texel = (shadow.right - shadow.left) / this.light.shadow.mapSize.x;
    focus.set(Math.round(focus.x / texel) * texel, 0, Math.round(focus.z / texel) * texel);
    this.light.target.position.copy(focus);
    this.light.target.updateMatrixWorld();
    this.light.position.copy(focus).add(this.shadowFrom);
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
    if (this.looking?.pointer === event.pointerId) this.looking = null;
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

// The walking camera: where its eyes are, and which way it looks: radians
// round y (0 looks along -z, and more turns left) and up from level.
interface Walker {
  position: THREE.Vector3;
  yaw: number;
  pitch: number;
}

const TURN_SPEED = THREE.MathUtils.degToRad(90); // a second, turning with a key or button
const LOOK_SPEED = THREE.MathUtils.degToRad(60); // a second, looking up or down with a key or button
const MOST_PITCH = THREE.MathUtils.degToRad(80); // up or down from level
const RUN = 2.5; // times the speed, running
const FOLLOW = 10; // how fast the camera catches up with the walker's place, per second
const FOLLOW_TURN = 20; // and with the way it looks
const WHEEL_STEP = 0.5; // m, a notch of the wheel
const TURN_ROUND = 4; // m ahead of the camera: the point it turns round after walking

function upOrDown(pitch: number): number {
  return THREE.MathUtils.clamp(pitch, -MOST_PITCH, MOST_PITCH);
}

const NEAR = 0.1; // m, the camera's near plane from a meter or more away
const NEAREST = 0.002; // m, the nearest it comes in close
const GLIDE = 7; // how fast the camera glides to a picked direction, per second
const SHOT_EASE = 2.5; // how fast it follows a story's shot, per second: most of the way in a second
const CLAIM_CUT = 6; // m: a camera a story claims back from further off than this cuts to its shot
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
