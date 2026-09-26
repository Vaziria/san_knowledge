import * as THREE from 'three';
import type { Theme } from '../../theme';
import type { Kind } from './events';
import { disposeFigure, makeFigure, type Figure } from './kinds';

// One animal at the lake meeting (Meeting.ts): the host's Bear, a viewer's
// animal or a bot. It walks or runs along a path of points, as it roams the
// lake's land (Roam.ts). It steers itself toward each point, turning at most
// so fast, and moves itself forward (the figures walk by themselves), so its
// feet keep to its steps. It stays wherever it stops, with the heading it
// arrived with. It stands on the uneven ground, leaning with its slope.

const WALK_TURN = 2.2; // radians a second, turning toward the next point while walking
const RUN_TURN = 3.2; // and running
const SLOPE_EASE = 6; // how fast it leans with the ground's slope, per second
const REACHED = 0.15; // m from a point that counts as there, or less by its stopping distance
const STOPPING = 0.25; // s: it stops by about its speed times this after Stop()
const CORNER = 0.1; // s: a corner on its way counts as reached this far before it, at its speed
const SLOW_BEFORE = 2.5; // m before a sharp corner that a runner slows to a walk
const SHARP = 0.7; // radians of turn at a corner that is sharp

export interface Step {
  to: THREE.Vector3;
  gait: 'walk' | 'run';
}

export class Member {
  readonly id: string;
  name: string;
  kind: Kind | 'bear';
  readonly bot: boolean;
  figure: Figure;
  length = 0; // m, nose to tail
  width = 0; // m, side to side
  height = 0; // m, standing
  radius = 0; // m round its middle it takes up on the ground: half its length or width, whichever is more
  private path: Step[] = [];
  private onArrive: (() => void) | null = null;

  constructor(id: string, name: string, kind: Kind | 'bear', bot: boolean, theme: Theme) {
    this.id = id;
    this.name = name;
    this.kind = kind;
    this.bot = bot;
    this.figure = this.build(kind, theme);
  }

  // Builds its figure, measuring it as it stands.
  private build(kind: Kind | 'bear', theme: Theme): Figure {
    const figure = makeFigure(kind, theme);
    figure.updateMatrixWorld(true);
    const size = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3());
    this.length = size.z;
    this.width = size.x;
    this.height = size.y;
    this.radius = Math.max(size.x, size.z) / 2;
    return figure;
  }

  // Becomes another kind in its place: the new figure stands where the old
  // one did, the way it faced. Returns the old one, to be taken away.
  change(kind: Kind, theme: Theme): Figure {
    const old = this.figure;
    this.kind = kind;
    this.figure = this.build(kind, theme);
    this.figure.position.copy(old.position);
    this.figure.rotation.copy(old.rotation);
    return old;
  }

  // Walks the steps in turn; `arrive` runs once the last is reached.
  walk(steps: Step[], arrive: (() => void) | null = null): void {
    this.path = steps;
    this.onArrive = arrive;
  }

  // Stands still where it is.
  stop(): void {
    this.path = [];
    this.onArrive = null;
    this.figure.Stop();
  }

  // Where it is on the ground.
  get at(): THREE.Vector2 {
    return new THREE.Vector2(this.figure.position.x, this.figure.position.z);
  }

  // The rest of its way: the points it still has to walk through.
  get ahead(): readonly Step[] {
    return this.path;
  }

  // Whether it is on its way somewhere.
  get moving(): boolean {
    return this.path.length > 0;
  }

  update(delta: number, ground: (x: number, z: number) => number): void {
    const figure = this.figure;
    const step = this.path[0];
    if (step) {
      const dx = step.to.x - figure.position.x;
      const dz = step.to.z - figure.position.z;
      const distance = Math.hypot(dx, dz);
      // It stops by the last point, easing to a stand; a corner on the way
      // it turns close to, so as not to cut it (what the way goes round is
      // on its inside).
      const next = this.path[1];
      const reached = next ? Math.max(REACHED, figure.speed * CORNER) : Math.max(REACHED, figure.speed * STOPPING);
      // Running into a sharp corner, or facing well away from its next point
      // (a new way), it walks until it faces the right way, turning in a
      // tighter circle.
      let gait = step.gait;
      if (gait === 'run' && Math.abs(shortest(Math.atan2(dx, dz) - figure.rotation.y)) > SHARP) gait = 'walk';
      if (gait === 'run' && next && distance < SLOW_BEFORE) {
        const turn = shortest(Math.atan2(next.to.x - step.to.x, next.to.z - step.to.z) - Math.atan2(dx, dz));
        if (Math.abs(turn) > SHARP) gait = 'walk';
      }
      if (distance < reached) {
        this.path.shift();
        if (this.path.length === 0) {
          figure.Stop();
          const arrive = this.onArrive;
          this.onArrive = null;
          arrive?.();
        }
      } else {
        if (gait === 'run') figure.Run();
        else figure.Walk();
        const turn = shortest(Math.atan2(dx, dz) - figure.rotation.y);
        const most = (gait === 'run' ? RUN_TURN : WALK_TURN) * delta;
        figure.rotation.y += THREE.MathUtils.clamp(turn, -most, most);
      }
    }
    figure.update(delta);

    // On the ground, leaning with its slope along and across the way it faces.
    const { x, z } = figure.position;
    figure.position.y = ground(x, z);
    const e = 0.2;
    const heading = figure.rotation.y;
    const ahead = new THREE.Vector2(Math.sin(heading), Math.cos(heading));
    const along = (ground(x + ahead.x * e, z + ahead.y * e) - ground(x - ahead.x * e, z - ahead.y * e)) / (2 * e);
    const across = (ground(x + ahead.y * e, z - ahead.x * e) - ground(x - ahead.y * e, z + ahead.x * e)) / (2 * e);
    const lean = 1 - Math.exp(-SLOPE_EASE * delta);
    figure.rotation.x += (-Math.atan(along) - figure.rotation.x) * lean;
    figure.rotation.z += (Math.atan(across) - figure.rotation.z) * lean;
  }

  dispose(): void {
    this.figure.removeFromParent();
    disposeFigure(this.figure);
  }
}

// An angle's difference the short way round, between -pi and pi.
export function shortest(angle: number): number {
  return THREE.MathUtils.euclideanModulo(angle + Math.PI, 2 * Math.PI) - Math.PI;
}
