import * as THREE from 'three';
import { demoStopper, type Behaviour } from './behaviours';
import { Floor } from './environtments/Floor/Floor';
import { Grass } from './environtments/Grass/Grass';
import { Lake } from './environtments/Lake/Lake';
import { Boat } from './figures/Boat/Boat';
import { DIRECTIONS, Fish, type Direction } from './figures/Fish/Fish';
import { FishingRod } from './figures/FishingRod/FishingRod';
import { MidiPiano } from './figures/MidiPiano';
import { Penguin } from './figures/Penguin/Penguin';
import type { Theme } from './theme';

// One figure is previewed at a time, in one environment: the figure, where
// the camera looks from and at, and its demo animation. The panel's Figure
// menu and ?figure=<name> pick one. Each entry builds its preview for the
// environment it is shown in.
export interface Preview {
  figure: THREE.Object3D;
  // Objects the demo uses that are not always inside the figure, such as the
  // keyboard the penguin only holds now and then; freed with the figure.
  props?: THREE.Object3D[];
  // Where the figure goes in its environment: on land (the default), or in
  // the water when the environment has any.
  place?: 'land' | 'water';
  camera: [number, number, number];
  target: [number, number, number];
  update(elapsed: number, delta: number): void; // elapsed: seconds since it was shown
  // Called when it is replaced, to stop listening to outside input such as
  // OSC messages (a story).
  dispose?(): void;
  // For a figure you can play by clicking or touching it: presses the part
  // under the pointer (the mesh hit) and returns the function that lets it
  // go, or returns null when that part doesn't press, so dragging there
  // turns the view as usual.
  press?(part: THREE.Object3D): (() => void) | null;
  // The figure's behaviours as its spec names them, for the panel's buttons
  // (see behaviours.ts). Running one stops the demo, so the figure does only
  // what it is told; the panel's Restart demo builds the preview again.
  behaviours?: Behaviour[];
}

export const previews: Record<string, (theme: Theme, environment: Environment) => Preview> = {
  // The boat floats when the environment has water. Its spec has no animation
  // behaviour yet, so the waves are its only movement: it rises and falls with
  // the water under it and tips with the slope between bow and stern and
  // between its sides, easing toward them because a hull lags the water it
  // sits in. Without water it rests on its keel.
  boat: (theme, environment) => {
    const boat = new Boat({ theme });
    const water = environment.water;
    const reach = 0.9; // it feels the water this far ahead of and behind its middle
    const beam = 0.4; // and this far to either side
    const response = 3; // how quickly it follows the water, per second
    if (water) boat.position.y = -Boat.DRAFT;

    return {
      figure: boat,
      place: 'water',
      camera: [2.6, 1.6, 3.0],
      target: [0, 0.3, 0],
      update(_elapsed, delta) {
        if (!water) return;
        const ahead = water.heightAt(0, reach);
        const astern = water.heightAt(0, -reach);
        const port = water.heightAt(-beam, 0);
        const starboard = water.heightAt(beam, 0);
        const t = 1 - Math.exp(-response * delta);
        boat.position.y += ((ahead + astern + port + starboard) / 4 - Boat.DRAFT - boat.position.y) * t;
        boat.rotation.x += (Math.atan2(astern - ahead, 2 * reach) - boat.rotation.x) * t; // > 0 dips the bow
        boat.rotation.z += (Math.atan2(starboard - port, 2 * beam) - boat.rotation.z) * t; // > 0 lifts starboard
      },
    };
  },
  // In water the fish swims a square, in a loop: right along the surface,
  // diving on the way back, left at depth, and rising on the way forward.
  // Each leg is a timed swim and the turns at the corners are the fish's own,
  // so the square stays in place. Without water it rests on the floor.
  fish: (theme, environment) => {
    const fish = new Fish({ theme });
    if (!environment.water) {
      return { figure: fish, camera: [0.35, 0.22, 0.45], target: [0, 0.06, 0], update() {} };
    }

    const leg = 4; // seconds a side
    const legs = [
      () => fish.SwimOnSurface('right'),
      () => fish.SwimOnDepth('back'),
      () => fish.SwimOnDepth('left'),
      () => fish.SwimOnSurface('forward'),
    ];
    const half = (Fish.SWIM_SPEED * leg) / 2; // half a side at full speed
    fish.position.set(-half, fish.surfaceY, half); // the corner where it starts right
    fish.rotation.y = Math.PI / 2; // facing right
    let current = -1;
    let demo = true;
    const act = demoStopper(() => (demo = false));
    const direction = { name: 'direction', value: 'right', options: DIRECTIONS };

    return {
      figure: fish,
      place: 'water',
      camera: [1.8, 1.4, 2.4],
      target: [0, -0.2, 0],
      update(elapsed, delta) {
        const now = Math.floor(elapsed / leg) % legs.length;
        if (demo && now !== current) {
          current = now;
          legs[current]();
        }
        fish.update(delta);
      },
      // Swimming needs water, so these are offered only on the lake.
      behaviours: [
        { name: 'SwimOnSurface', params: [direction], run: act((d) => fish.SwimOnSurface(d as Direction)) },
        { name: 'SwimOnDepth', params: [direction], run: act((d) => fish.SwimOnDepth(d as Direction)) },
        { name: 'Stop', run: act(() => fish.Stop()) },
      ],
    };
  },
  // The rod's spec has no behaviour yet, so it is held still, as if by an
  // unseen hand 1 m up, tip raised. update() keeps its line hanging straight.
  'fishing-rod': (theme) => {
    const rod = new FishingRod({ theme });
    rod.position.set(0, 1, -0.5);
    rod.rotation.x = -0.5; // < 0 raises the tip
    return {
      figure: rod,
      camera: [1.9, 1.5, 2.0],
      target: [0, 1.1, 0.3],
      update() {
        rod.update();
      },
    };
  },
  penguin: (theme) => {
    const penguin = new Penguin({ theme });
    const keyboard = new MidiPiano({ theme });
    // What the Hold button can hand it.
    const holdable: Record<string, THREE.Object3D> = {
      'midi-piano': keyboard,
      fish: new Fish({ theme }),
      'fishing-rod': new FishingRod({ theme }),
    };

    // Demo, in a loop: say what it will do (in two bubbles), flap, jump, hold
    // a MIDI keyboard, then walk half a lap and run one and a half laps of a
    // circle, stopping back where it began. The penguin moves itself forward;
    // the preview steers it, turning it by speed / radius so it keeps to the
    // circle at any speed, also when a behaviour button makes it walk. Each
    // step runs until its test passes, since a lap takes as long as the gait
    // makes it.
    const radius = 0.22;
    const lap = 2 * Math.PI;
    const slowing = 0.26; // radians it turns while slowing from a run to a stop
    penguin.position.set(-radius, 0, 0); // on the circle, facing +z along it
    let finish = 0; // heading (rotation.y) at the end of the two laps
    const steps: [start: () => void, done: (seconds: number) => boolean][] = [
      [() => penguin.Speech("Hi! I'm a penguin. Watch me flap, jump, hold a keyboard, then walk and run around."), () => true],
      [() => penguin.Flap(), (s) => s > 1.5],
      [() => penguin.Jump(), (s) => s > 1.5],
      [() => penguin.Hold(keyboard), (s) => s > 3],
      [() => penguin.Hold(null), (s) => s > 0.5],
      [
        () => {
          // Counted from the nearest whole lap, so small misses don't add up.
          finish = (Math.round(penguin.rotation.y / lap) + 2) * lap;
          penguin.Walk();
        },
        () => penguin.rotation.y > finish - 1.5 * lap,
      ],
      [() => penguin.Run(), () => penguin.rotation.y > finish - slowing],
      [() => penguin.Stop(), () => penguin.speed < 0.002],
    ];
    let step = -1;
    let stepStart = 0;

    // The first behaviour button ends the demo where it is: the penguin stops
    // and puts down what it holds, then does what the button says.
    let demo = true;
    const act = demoStopper(() => {
      if (!demo) return;
      demo = false;
      penguin.Stop();
      penguin.Hold(null);
    });

    return {
      figure: penguin,
      props: Object.values(holdable),
      camera: [0.6, 0.7, 1.35],
      target: [0, 0.3, 0],
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step][1](elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step][0]();
        }
        penguin.rotation.y += (penguin.speed / radius) * delta;
        penguin.update(delta);
      },
      behaviours: [
        { name: 'Flap', run: act(() => penguin.Flap()) },
        { name: 'Jump', run: act(() => penguin.Jump()) },
        {
          name: 'Hold',
          params: [{ name: 'figure', value: 'midi-piano', options: [...Object.keys(holdable), 'nothing'] }],
          run: act((figure) => penguin.Hold(holdable[figure] ?? null)),
        },
        { name: 'Walk', run: act(() => penguin.Walk()) },
        { name: 'Run', run: act(() => penguin.Run()) },
        { name: 'Stop', run: act(() => penguin.Stop()) },
        {
          name: 'Speech',
          params: [{ name: 'text', value: 'Hello! This text is too long for one bubble, so I say it in several, one after another.' }],
          run: act((text) => penguin.Speech(text)),
        },
      ],
    };
  },
  'midi-piano': (theme) => {
    const midiPiano = new MidiPiano({ theme });

    // Demo: loop a chord progression with PlayChord, and a melody over it
    // with PlayNote, four notes per chord.
    const progression = ['C', 'G', 'Am', 'F'];
    const melody = [
      ['G5', 'E5', 'C5', 'E5'],
      ['B5', 'G5', 'D5', 'G5'],
      ['A5', 'E5', 'C5', 'E5'],
      ['A5', 'F5', 'C5', 'F5'],
    ];
    const noteLength = 0.2;
    let lastStep = -1;
    let demo = true;
    const act = demoStopper(() => (demo = false));

    return {
      figure: midiPiano,
      camera: [0.15, 0.4, 0.55],
      target: [0, 0.02, 0],
      update(elapsed, delta) {
        if (demo) {
          const step = Math.floor(elapsed / noteLength) % (progression.length * 4);
          if (step !== lastStep) {
            const bar = Math.floor(step / 4);
            if (step % 4 === 0) midiPiano.PlayChord(progression[bar]);
            midiPiano.PlayNote(melody[bar][step % 4]);
            lastStep = step;
          }
          for (let i = 0; i < MidiPiano.KNOB_COUNT; i++) {
            midiPiano.setKnob(i, 0.5 + 0.5 * Math.sin(elapsed + i * 0.8));
          }
        }
        midiPiano.update(delta);
      },
      // Chord names such as C, F#m, Bb7 or Gmaj7; notes such as E, F# or A4.
      behaviours: [
        { name: 'PlayChord', params: [{ name: 'chord', value: 'Am' }], run: act((chord) => midiPiano.PlayChord(chord)) },
        { name: 'PlayNote', params: [{ name: 'note', value: 'E5' }], run: act((note) => midiPiano.PlayNote(note)) },
      ],
    };
  },
};

// The figure being worked on, shown when none is picked.
export const defaultFigure = 'penguin';

// The surroundings a figure is shown in, picked in the panel's Environment
// menu or with ?environment=<name>; the menu lists them in this order. The
// stage moves the scenery so that the figure's spot is at the origin: `land`
// for a figure that stands, `water.at` for one whose place is the water. So
// figures, cameras and shadows stay where they are in every environment.
export interface Environment {
  scenery: THREE.Object3D;
  land: THREE.Vector3; // where a figure stands, in the scenery's coordinates
  water?: {
    at: THREE.Vector3; // where a figure in the water goes, on the still surface
    heightAt(x: number, z: number): number; // the water's height now, measured from `at`
  };
  update(delta: number): void; // moves it on, such as the waves; called every frame
}

export const environments: Record<string, (theme: Theme) => Environment> = {
  floor: (theme) => ({ scenery: new Floor({ theme }), land: new THREE.Vector3(), update() {} }),
  lake: (theme) => {
    const lake = new Lake({ theme });
    return {
      scenery: lake,
      land: Lake.LANDING.clone(),
      water: { at: new THREE.Vector3(), heightAt: (x, z) => lake.surfaceAt(x, z) },
      update: (delta) => lake.update(delta),
    };
  },
  grass: (theme) => ({ scenery: new Grass({ theme }), land: new THREE.Vector3(), update() {} }),
};

// The environment a figure starts in until one is picked: the boat and the
// fish on the lake, the rest on the floor.
const figureEnvironments: Record<string, string> = { boat: 'lake', fish: 'lake' };

export function figureEnvironment(figure: string): string {
  return figureEnvironments[figure] ?? 'floor';
}
