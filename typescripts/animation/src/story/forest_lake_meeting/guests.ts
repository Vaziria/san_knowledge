import * as THREE from 'three';
import { Duck } from '../../figures/ForestLake/animals/Duck';
import { Explorer } from '../../figures/ForestLake/animals/Explorer';
import type { ForestAnimal } from '../../figures/ForestLake/animals/ForestAnimal';
import { ForestDeer } from '../../figures/ForestLake/animals/ForestDeer';
import { ForestFox } from '../../figures/ForestLake/animals/ForestFox';
import { ForestFrog } from '../../figures/ForestLake/animals/ForestFrog';
import { ForestSquirrel } from '../../figures/ForestLake/animals/ForestSquirrel';
import { ForestWolf } from '../../figures/ForestLake/animals/ForestWolf';
import { Otter } from '../../figures/ForestLake/animals/Otter';
import { Rabbit } from '../../figures/ForestLake/animals/Rabbit';
import { WildBoar } from '../../figures/ForestLake/animals/WildBoar';
import type { Season } from '../../figures/ForestLake/parts';
import { SpeechBubble } from '../../speech/SpeechBubble';
import type { Theme } from '../../theme';
import type { Figure } from '../lake_meeting/kinds';
import type { Command, Kind } from './events';

// The figures of the forest lake meeting (forest_lake_meeting.md): the
// forest lake's own animals (figures/ForestLake/animals), as their sheets
// made them, and the explorer, the host. Each is given what the meeting asks
// of a figure (lake_meeting/kinds.ts: Figure), and nothing of its own is
// changed:
//
// - It says what its viewer writes in a speech bubble headed by their name,
//   just over its head, as big as the lake meeting's animals' for its size.
// - Walk() and Run() start it going only when it isn't already, since the
//   meeting asks every frame (the frog's hop would start over each time); a
//   jump or a flap under way ends first. Stop() stands it still when it is
//   going, and leaves a pose it holds (a fox sitting, talking).
// - Jump() is a hop where it stands, so it never lands anywhere the meeting
//   didn't put it: the four-legged ones spring straight up (their rig's leap,
//   carrying them nowhere), the frog leaps as its sheet has it (a few
//   centimeters on), the duck flaps its wings, the explorer jumps.
// - Its tricks are its sheet's poses (events.ts, TRICKS): the wolf howls, the
//   duck flaps, the fox sits or looks up at the sky, the deer looks back, the
//   boar and the rabbit eat, the otter stands up, the explorer waves; each
//   lasts SECONDS. Standing at a spot, it now and then does one of them
//   rather than hop (Pastime).
// - Its colour is one of its sheet's variations, the same for a viewer on
//   every page (picked from their id), in its season's coat where it has one
//   (a snow wolf in winter).

// What the forest animals the meeting uses have in common.
interface Forest extends ForestAnimal {
  readonly doing: string;
  Idle(): void;
  Walk(): void;
  Run(): void;
  Hop(): void;
  update(delta: number): void;
}

// What a guest does that it doesn't do on its own.
export interface Guest extends Figure {
  readonly kind: Kind | 'explorer';
  // How long one of its tricks takes, s, or 0 when it has none.
  trickLength(command: Command): number;
  // Does one of its tricks: how long it takes, s, or 0 when it has none.
  Trick(command: Command): number;
  // What it does now and then standing at a spot: a trick of its own, or a hop.
  Pastime(random: () => number): void;
}

// The four-legged ones' hop: their rig's leap, straight up.
class Wolf extends ForestWolf {
  Hop(): void {
    this.leap(0.32, 0);
  }
}
class Fox extends ForestFox {
  Hop(): void {
    this.leap(0.26, 0);
  }
}
class Deer extends ForestDeer {
  Hop(): void {
    this.leap(0.4, 0);
  }
}
class Boar extends WildBoar {
  Hop(): void {
    this.leap(0.26, 0);
  }
}
class Hare extends Rabbit {
  Hop(): void {
    this.leap(0.3, 0, 0.14);
  }
}
class Squirrel extends ForestSquirrel {
  Hop(): void {
    if (!this.climbing) this.leap(0.18, 0, 0.12);
  }
}
class Swimmer extends Otter {
  Hop(): void {
    this.leap(0.22, 0);
  }
}
// The frog hops as it walks, and can't run: running, it hops on. (Its own
// Jump() and the explorer's, not the meeting's, which hops.)
class Frog extends ForestFrog {
  Hop(): void {
    super.Jump();
  }
  Run(): void {
    this.Walk();
  }
}
class Drake extends Duck {
  Hop(): void {
    this.FlapWings();
  }
}
class Host extends Explorer {
  Hop(): void {
    super.Jump();
  }
}

interface Trick<T> {
  act(animal: T): void;
  seconds: number;
}

interface Manner<T extends Forest> {
  make: (new (options: { color?: string; seed?: number }) => T) & { readonly COLORS: readonly string[] };
  tricks?: Partial<Record<Command, Trick<T>>>;
  pastime?: number; // the chance that what it does standing at a spot is a trick, not a hop
}

// Each kind's tricks, from its sheet's poses, and how long each lasts.
const MANNERS: { [K in Kind | 'explorer']: Manner<Forest> } = {
  wolf: manner({ make: Wolf, tricks: { howl: { act: (w) => w.Howl(), seconds: 4.8 } }, pastime: 0.3 }),
  deer: manner({ make: Deer, tricks: { look: { act: (d) => d.LookBack(), seconds: 4.5 } }, pastime: 0.45 }),
  fox: manner({
    make: Fox,
    tricks: { sit: { act: (f) => f.Sit(), seconds: 6 }, look: { act: (f) => f.LookUp(), seconds: 5.2 } },
    pastime: 0.5,
  }),
  boar: manner({ make: Boar, tricks: { eat: { act: (b) => b.Eat(), seconds: 6 } }, pastime: 0.5 }),
  rabbit: manner({ make: Hare, tricks: { eat: { act: (r) => r.Eat(), seconds: 6 } }, pastime: 0.5 }),
  squirrel: manner({ make: Squirrel }),
  duck: manner({ make: Drake, tricks: { flap: { act: (d) => d.FlapWings(), seconds: 3 } }, pastime: 0.3 }),
  otter: manner({ make: Swimmer, tricks: { stand: { act: (o) => o.StandUp(), seconds: 4.8 } }, pastime: 0.4 }),
  frog: manner({ make: Frog }),
  explorer: manner({ make: Host, tricks: { wave: { act: (e) => e.Wave(), seconds: 2.6 } }, pastime: 0.35 }),
};

function manner<T extends Forest>(m: Manner<T>): Manner<Forest> {
  return m as unknown as Manner<Forest>;
}

// What it does that a walk or a run must wait for, and that Stop() leaves.
const BUSY = new Set(['jump', 'flap', 'takeoff']);
const PER = 0.55; // m over the ground a bubble of the lake's penguin's size stands, as the lake meeting's animals'
const SCALE = [0.5, 4] as const; // the bubble's size at least and at most, times the penguin's

// Kinds whose sheets have coats for a season, worn only in it.
const SEASONAL: Readonly<Record<string, Season>> = { snow: 'winter', winter: 'winter', autumn: 'autumn' };

// A figure of the forest lake meeting: its kind (the explorer for the
// host), the member's id for its colour, and the season.
export function makeGuest(kind: Kind | 'explorer', id: string, season: Season, theme: Theme): Guest {
  const m = MANNERS[kind];
  const worn = m.make.COLORS.filter((c) => !SEASONAL[c] || SEASONAL[c] === season);
  const inSeason = worn.filter((c) => SEASONAL[c] === season);
  const pick = hash(id);
  // In its season's coat half the time, where it has one.
  const color = inSeason.length && pick % 2 === 0 ? inSeason[(pick >>> 1) % inSeason.length] : worn[(pick >>> 1) % worn.length];
  const Base = m.make as new (...args: any[]) => Forest;

  class Meets extends Base implements Guest {
    readonly kind = kind;
    private readonly bubble: SpeechBubble;

    constructor() {
      super({ color });
      this.rotation.order = 'YXZ'; // heading first, then leaning with the ground's slope
      // Just over its head: the top of it as it stands, three quarters of
      // the way to its front.
      this.update(0);
      this.updateMatrixWorld(true);
      const box = new THREE.Box3().setFromObject(this);
      this.bubble = new SpeechBubble(theme);
      this.bubble.position.set(0, box.max.y + 0.05, THREE.MathUtils.lerp(box.min.z, box.max.z, kind === 'explorer' ? 0.5 : 0.75));
      this.bubble.scale.setScalar(THREE.MathUtils.clamp(this.bubble.position.y / PER, SCALE[0], SCALE[1]));
      this.add(this.bubble);
    }

    get speaking(): boolean {
      return this.bubble.speaking;
    }

    get speechBubble(): THREE.Object3D {
      return this.bubble;
    }

    Speech(text: string, speaker = ''): void {
      this.bubble.say(text, speaker);
    }

    Walk(): void {
      if (!BUSY.has(this.doing) && this.doing !== 'walk') super.Walk();
    }

    Run(): void {
      if (!BUSY.has(this.doing) && this.doing !== 'run') super.Run();
    }

    Stop(): void {
      if (this.doing === 'walk' || this.doing === 'run') this.Idle();
    }

    Jump(): void {
      this.Hop();
    }

    trickLength(command: Command): number {
      return m.tricks?.[command]?.seconds ?? 0;
    }

    Trick(command: Command): number {
      const trick = m.tricks?.[command];
      if (!trick) return 0;
      trick.act(this);
      return trick.seconds;
    }

    Pastime(random: () => number): void {
      const tricks = Object.values(m.tricks ?? {});
      if (tricks.length && random() < (m.pastime ?? 0)) tricks[Math.floor(random() * tricks.length)]!.act(this);
      else this.Hop();
    }

    update(delta: number): void {
      super.update(delta);
      this.bubble?.update(delta);
    }
  }
  return new Meets();
}

// Whether a figure is one of the forest lake meeting's.
export function isGuest(figure: Figure): figure is Guest {
  return 'Trick' in figure;
}

// A number from a member's id, the same on every page.
function hash(text: string): number {
  let h = 2166136261;
  for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 16777619);
  return h >>> 0;
}
