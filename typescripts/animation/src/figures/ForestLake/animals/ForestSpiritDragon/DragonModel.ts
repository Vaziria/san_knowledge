import * as THREE from 'three';
import { GLTFLoader, type GLTF } from 'three/addons/loaders/GLTFLoader.js';
import { clone } from 'three/addons/utils/SkeletonUtils.js';
import { ease, smooth } from '../parts';

// The forest spirit dragon drawn with the user's own model, for the forest
// lake meeting, as the user asked ("export to glb and add to forest lake
// meeting", 2026-10-03, choosing to keep everything the meeting's dragon does
// and draw it with their model). The model is a Meshy model the user rigged
// and animated in Blender with Claude: public/models/forest_spirit_dragon.glb,
// with its clips (Dragon_Idle, _Walk, _Run, _Fly, _Speak, _Jump). It is the
// one thing drawn from a model file rather than built in code (rules.md,
// Project rule 7).
//
// It moves nothing itself. The ForestSpiritDragon it stands in for still
// walks, flies, takes off, lands and roars, and follow() picks the clips from
// what that is doing, each frame, easing from one to the next:
// - standing, Dragon_Idle; walking, Dragon_Walk, played as fast as its feet
//   must go back under it for the dragon's speed, so they stay planted;
// - crouching and springing to take off, the start of Dragon_Jump; in the
//   air, Dragon_Fly, its wings beating as fast as the figure's would (its
//   cruise, climb, glide, spring and braking beats); coming down to land, its
//   legs reaching for the ground over the last REACH m; touching down, the end
//   of Dragon_Jump, sinking into its legs and standing up again.
// Its body is tipped as the figure's is (pitched climbing, banked into its
// turns). Its roar and its talking are its head and jaw moved over whatever
// clip plays.
//
// Meters, y up, facing +z, its origin on the ground under its middle, as
// ForestSpiritDragon's: the model, 0.3 m tall, is scaled to HEIGHT to the top
// of its head.

const FILE = 'models/forest_spirit_dragon.glb';
const HEIGHT = 4.5; // m to the top of its head standing, as ForestSpiritDragon.md has it
const FPS = 24; // the clips' frames a second, from Blender
const PIVOT = 1.7; // m up where its body tips, as ForestSpiritDragon's body (BODY_AT)

// Its walk clip, in the model's own size: each foot down 0.65 of a 1 s
// stride, going 0.036 back under it (the clip as made in Blender).
const WALK_CLIP = { stride: 0.036, duty: 0.65, cycle: 1 };
const FLAPS = 2; // wingbeats in its 1 s fly clip
// Frames of Dragon_Jump: taking off, crouching and springing until its feet
// leave the ground, over the figure's crouch and spring (0.5 s and 0.1 s);
// landing, from touching down through sinking into its legs to standing,
// over the figure's settle (0.9 s).
const TAKEOFF = { from: 0, to: 13, time: 0.6 };
const SETTLE = { from: 23, to: 37, time: 0.9 };
const REACH = 3; // m over the ground from which its legs reach down, landing
const BLEND = 6; // how fast it eases from clip to clip, a second
// Its wingbeats a second, as the figure's (ForestSpiritDragon's BEATS).
const BEATS = { cruise: 0.85, climb: 1.2, glide: 0.4, spring: 1.45, brake: 1.5 };
const CLIMB = 3.5; // m/s the figure climbs at most (ForestSpiritDragon.CLIMB)

// Roaring, timed as the figure's roar: drawing its head in, then thrusting
// it up with its jaws wide, and closing them (rad: its head and chest nosed
// down, its jaw open).
const ROAR = { gather: 0.6, roar: 1.8, close: 0.8 };
const ROAR_HEAD = { gather: 0.15, open: -0.35 };
const ROAR_CHEST = -0.08;
const ROAR_JAW = 0.5;
const ROAR_SHAKE = 0.05;
// Talking: its jaw (rad) through a 2 s phrase, as its speak clip opens it.
const TALK: readonly (readonly [number, number])[] = [
  [0, 0], [0.125, 0.21], [0.25, 0.05], [0.375, 0.26], [0.5, 0.07], [0.625, 0.16], [0.75, 0.03], [0.917, 0.3],
  [1.083, 0.09], [1.208, 0.19], [1.333, 0.03], [1.5, 0.24], [1.667, 0.07], [1.792, 0.14], [2, 0],
];
const TALK_EASE = 6; // how fast it starts and stops talking, a second

type Clip = 'idle' | 'walk' | 'fly' | 'takeoff' | 'land';
const CLIPS: readonly Clip[] = ['idle', 'walk', 'fly', 'takeoff', 'land'];

// What the dragon it stands in for is doing, each frame.
export interface DragonDoing {
  doing: 'idle' | 'walk' | 'takeoff' | 'fly' | 'land';
  flying: boolean;
  speed: number; // m/s along its heading
  climb: number; // m/s up
  height: number; // m over the ground under it
  walkSpeed: number; // m/s of its full walk
  roaring: boolean;
  talking: boolean;
  pitch: number; // rad its body is tipped about x, and about z (banked)
  bank: number;
}

let loading: Promise<GLTF> | null = null;

export class DragonModel extends THREE.Group {
  private readonly mixer: THREE.AnimationMixer;
  private readonly actions: Record<Clip, THREE.AnimationAction>;
  private readonly weights: Record<Clip, number> = { idle: 1, walk: 0, fly: 0, takeoff: 0, land: 0 };
  private readonly times: Record<Clip, number> = { idle: 0, walk: 0, fly: 0, takeoff: 0, land: 0 };
  private readonly tip = new THREE.Group(); // tipped as its body is
  private readonly lifted = new THREE.Group(); // the model, its feet on the ground
  private readonly rest: number; // m the model is raised to stand on the ground
  private readonly flyLift: number; // m its fly clip holds its body over its standing height
  private readonly walkSpeed: number; // m/s its walk clip goes at, played as made
  private readonly head: THREE.Object3D;
  private readonly jaw: THREE.Object3D;
  private readonly chest: THREE.Object3D;
  private readonly axes: { head: THREE.Vector3; turn: THREE.Vector3; jaw: THREE.Vector3; chest: THREE.Vector3 };
  private readonly crownAt: THREE.Vector3; // the top of its head, in its head bone's space
  private last: DragonDoing['doing'] = 'idle';
  private time = 0;
  private beat = BEATS.cruise;
  private settle = -1; // s since it touched down, or -1
  private takeoff = 0; // s into a take-off
  private roarTime = -1;
  private talk = 0; // 0..1, talking
  private talkTime = 0;

  // The model for one dragon, once its file has loaded; null in Node (the
  // meeting's checks) or if it couldn't load, when the figure built in code
  // is shown instead. The file is fetched once a page.
  static async load(): Promise<DragonModel | null> {
    if (typeof document === 'undefined') return null;
    loading ??= new GLTFLoader().loadAsync(new URL(FILE, document.baseURI).href);
    try {
      return new DragonModel(await loading);
    } catch (error) {
      console.warn('The forest spirit dragon model did not load; the dragon built in code is shown.', error);
      loading = null;
      return null;
    }
  }

  private constructor(gltf: GLTF) {
    super();
    const model = clone(gltf.scene);
    // Meshy's material comes fully metallic and lit by its own colour; it is
    // a plain matte surface of its texture here, like the meeting's felt.
    model.traverse((o) => {
      if (!(o instanceof THREE.Mesh)) return;
      const made = o.material as THREE.MeshStandardMaterial;
      o.material = new THREE.MeshStandardMaterial({ map: made.map, roughness: 0.9, metalness: 0 });
      o.castShadow = true;
      o.receiveShadow = true;
      o.frustumCulled = false; // skinned: its bounds are its rest pose's
    });
    const bone = (name: string) => {
      const found = model.getObjectByName(name);
      if (!found) throw new Error(`The dragon model has no bone ${name}`);
      return found;
    };
    this.head = bone('head');
    this.jaw = bone('jaw');
    this.chest = bone('chest');
    const hips = bone('Hips');

    // Its size and where it stands, from its rest pose.
    model.updateMatrixWorld(true);
    const box = new THREE.Box3().setFromObject(model);
    const scale = HEIGHT / (box.max.y - box.min.y);
    // Each bone's turn about the model's own x (nose down) and y, in the
    // bone's space: its rest turn in the model undone.
    const axis = (o: THREE.Object3D, along: THREE.Vector3) => along.clone().applyQuaternion(o.getWorldQuaternion(new THREE.Quaternion()).invert());
    const X = new THREE.Vector3(1, 0, 0);
    this.axes = { head: axis(this.head, X), turn: axis(this.head, new THREE.Vector3(0, 1, 0)), jaw: axis(this.jaw, X), chest: axis(this.chest, X) };
    const headAt = this.head.getWorldPosition(new THREE.Vector3());
    this.crownAt = this.head.worldToLocal(new THREE.Vector3(headAt.x, box.max.y, headAt.z));

    this.mixer = new THREE.AnimationMixer(model);
    const clip = (name: string) => {
      const found = gltf.animations.find((a) => a.name === name);
      if (!found) throw new Error(`The dragon model has no clip ${name}`);
      return found;
    };
    const jump = clip('Dragon_Jump');
    const clips: Record<Clip, THREE.AnimationClip> = {
      idle: clip('Dragon_Idle'),
      walk: clip('Dragon_Walk'),
      fly: clip('Dragon_Fly'),
      takeoff: THREE.AnimationUtils.subclip(jump, 'takeoff', TAKEOFF.from, TAKEOFF.to, FPS),
      land: THREE.AnimationUtils.subclip(jump, 'land', SETTLE.from, SETTLE.to, FPS),
    };
    this.actions = Object.fromEntries(CLIPS.map((c) => [c, this.mixer.clipAction(clips[c]).play()])) as Record<Clip, THREE.AnimationAction>;

    // How far over its standing height its fly clip holds its body (it
    // hovers in place): taken off again, so it flies where the figure does.
    const hipsHeight = (c: Clip, times: number[]) => {
      let sum = 0;
      for (const t of times) {
        this.pose({ [c]: 1 }, { [c]: t });
        model.updateMatrixWorld(true);
        sum += hips.getWorldPosition(new THREE.Vector3()).y;
      }
      return sum / times.length;
    };
    const beats = Array.from({ length: 8 }, (_, i) => (i / 8) * clips.fly.duration);
    this.flyLift = (hipsHeight('fly', beats) - hipsHeight('idle', [0])) * scale;
    this.walkSpeed = (WALK_CLIP.stride / (WALK_CLIP.duty * WALK_CLIP.cycle)) * scale;

    model.scale.setScalar(scale);
    this.rest = -box.min.y * scale;
    this.lifted.add(model);
    this.tip.add(this.lifted);
    this.tip.position.y = PIVOT;
    this.lifted.position.y = this.rest - PIVOT;
    this.add(this.tip);
    this.pose(this.weights, this.times);
  }

  // The top of its head, in the world, wherever its neck and clip put it.
  crown(target: THREE.Vector3): THREE.Vector3 {
    this.head.updateWorldMatrix(true, false);
    return this.head.localToWorld(target.copy(this.crownAt));
  }

  follow(s: DragonDoing, delta: number): void {
    const dt = Math.max(0, Math.min(delta, 0.1));
    this.time += dt;
    if (s.doing !== this.last) {
      if (s.doing === 'takeoff') this.takeoff = 0;
      if (this.last === 'land' && !s.flying) this.settle = 0;
      this.last = s.doing;
    }
    if (s.doing === 'takeoff') this.takeoff += dt;
    if (this.settle >= 0) {
      this.settle += dt;
      if (this.settle > SETTLE.time) this.settle = -1;
    }

    // ------------------------------------------------ which clips
    const want: Record<Clip, number> = { idle: 0, walk: 0, fly: 0, takeoff: 0, land: 0 };
    if (s.doing === 'takeoff' && !s.flying) want.takeoff = 1;
    else if (s.flying) {
      const reach = s.doing === 'land' ? 1 - smooth(s.height / REACH) : 0;
      want.fly = 1 - reach;
      want.idle = reach;
    } else if (this.settle >= 0) want.land = 1;
    else {
      const walking = Math.min(1, s.speed / s.walkSpeed);
      want.walk = walking;
      want.idle = 1 - walking;
    }
    let sum = 0;
    for (const c of CLIPS) sum += this.weights[c] = ease(this.weights[c], want[c], BLEND, dt);
    for (const c of CLIPS) this.weights[c] /= sum;

    // ------------------------------------------------ how far through each
    const t = this.times;
    const length = (c: Clip) => this.actions[c].getClip().duration;
    let beat = BEATS.cruise;
    if (s.doing === 'takeoff') beat = BEATS.spring;
    else if (s.doing === 'land') beat = s.height < REACH + 1 ? BEATS.brake : BEATS.glide;
    else if (s.climb > 0.3 * CLIMB) beat = BEATS.climb;
    else if (s.climb < -0.2 * CLIMB || s.roaring) beat = BEATS.glide;
    this.beat = ease(this.beat, beat, 2, dt);
    t.idle = (t.idle + dt) % length('idle');
    t.walk = (t.walk + (dt * s.speed) / this.walkSpeed) % length('walk');
    t.fly = (t.fly + (dt * this.beat * length('fly')) / FLAPS) % length('fly');
    t.takeoff = Math.min(this.takeoff / TAKEOFF.time, 0.999) * length('takeoff');
    t.land = Math.min(Math.max(this.settle, 0) / SETTLE.time, 0.999) * length('land');
    this.pose(this.weights, t);

    // ------------------------------------------------ over the clips
    // Its roar and its talking, on its head and jaw.
    if (!s.roaring) this.roarTime = -1;
    else this.roarTime = this.roarTime < 0 ? 0 : this.roarTime + dt;
    const { gather, open } = roarShape(this.roarTime);
    this.talk = ease(this.talk, s.talking ? 1 : 0, TALK_EASE, dt);
    this.talkTime = s.talking || this.talk > 0.01 ? (this.talkTime + dt) % 2 : 0;
    const speech = this.talk * (1 - open) * talking(this.talkTime);
    const turn = (o: THREE.Object3D, along: THREE.Vector3, angle: number) => {
      if (angle) o.quaternion.multiply(_turn.setFromAxisAngle(along, angle));
    };
    turn(this.head, this.axes.head, ROAR_HEAD.gather * gather + ROAR_HEAD.open * open + 0.04 * speech);
    turn(this.head, this.axes.turn, ROAR_SHAKE * open * Math.sin(this.time * 38));
    turn(this.chest, this.axes.chest, ROAR_CHEST * open);
    turn(this.jaw, this.axes.jaw, 0.1 * gather + ROAR_JAW * open + speech);

    // ------------------------------------------------ where its body is
    // Flying, tipped as the figure's body (pitched climbing, banked into its
    // turns), and held where the figure flies rather than over it, as its fly
    // clip hovers. Not on the ground: the figure's body rolls over its feet
    // as it walks, and the model's clips carry its body themselves (rolled
    // about its body, 1.7 m up, its planted feet slid 5–9 cm).
    const air = this.weights.fly;
    this.tip.rotation.set(s.pitch * air, 0, s.bank * air);
    this.lifted.position.y = this.rest - PIVOT - this.flyLift * this.weights.fly;
  }

  // Poses the model with these clips' weights at these times.
  private pose(weights: Partial<Record<Clip, number>>, times: Partial<Record<Clip, number>>): void {
    for (const c of CLIPS) {
      const action = this.actions[c];
      action.setEffectiveWeight(weights[c] ?? 0);
      action.time = times[c] ?? 0;
    }
    this.mixer.update(0);
  }
}

const _turn = new THREE.Quaternion();

// How far through a roar it is (s, or -1): gathering itself, its head drawn
// in, and roaring, its head up and jaws wide; as ForestSpiritDragon's.
function roarShape(t: number): { gather: number; open: number } {
  if (t < 0) return { gather: 0, open: 0 };
  const gather = smooth(t / ROAR.gather) * (1 - smooth((t - ROAR.gather) / 0.35));
  const open = smooth((t - ROAR.gather + 0.1) / 0.35) * (1 - smooth((t - ROAR.gather - ROAR.roar) / ROAR.close));
  return { gather, open };
}

// Its jaw talking, `t` s into a phrase.
function talking(t: number): number {
  for (let i = 1; i < TALK.length; i++) {
    const [t1, a1] = TALK[i];
    if (t <= t1) {
      const [t0, a0] = TALK[i - 1];
      return a0 + (a1 - a0) * smooth((t - t0) / (t1 - t0));
    }
  }
  return 0;
}
