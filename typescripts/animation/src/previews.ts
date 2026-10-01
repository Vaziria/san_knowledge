import * as THREE from 'three';
import { demoStopper, type Behaviour } from './behaviours';
import { Floor } from './environtments/Floor/Floor';
import { ForestLake } from './environtments/ForestLake/ForestLake';
import { Grass } from './environtments/Grass/Grass';
import { deckPoint, FAR_SHORE } from './environtments/Lake/DockSite';
import { Lake, type LakeOptions } from './environtments/Lake/Lake';
import type { Animal } from './figures/animals/Animal';
import { Antelope } from './figures/animals/Antelope';
import { Bear } from './figures/animals/Bear';
import { Bird } from './figures/animals/Bird';
import { Cat } from './figures/animals/Cat';
import { Coyote } from './figures/animals/Coyote';
import { Crocodile } from './figures/animals/Crocodile';
import { Deer } from './figures/animals/Deer';
import { Fox } from './figures/animals/Fox';
import { Frog } from './figures/animals/Frog';
import { Lion } from './figures/animals/Lion';
import { Monkey } from './figures/animals/Monkey';
import { Ox } from './figures/animals/Ox';
import type { AnimalOptions } from './figures/animals/parts';
import { Penguin } from './figures/animals/Penguin';
import { Pig } from './figures/animals/Pig';
import { Raccoon } from './figures/animals/Raccoon';
import { Snake } from './figures/animals/Snake';
import { Squirrel } from './figures/animals/Squirrel';
import { Tiger } from './figures/animals/Tiger';
import { Wolf } from './figures/animals/Wolf';
import { Boat } from './figures/Boat/Boat';
import { DIRECTIONS, Fish, type Direction, type FishKind } from './figures/Fish/Fish';
import { FishingRod } from './figures/FishingRod/FishingRod';
import { forestAnimalPreviews } from './figures/ForestLake/animals/demos';
import { Barrel } from './figures/ForestLake/Barrel';
import { BroadleafTree } from './figures/ForestLake/BroadleafTree';
import { Bridge } from './figures/ForestLake/Bridge';
import { Bush } from './figures/ForestLake/Bush';
import { Campfire } from './figures/ForestLake/Campfire';
import { CaveEntrance } from './figures/ForestLake/CaveEntrance';
import { CherryTree } from './figures/ForestLake/CherryTree';
import { Crate } from './figures/ForestLake/Crate';
import { DeadTree } from './figures/ForestLake/DeadTree';
import { FallenLeaves } from './figures/ForestLake/FallenLeaves';
import { Fence } from './figures/ForestLake/Fence';
import { FlowerCluster } from './figures/ForestLake/FlowerCluster';
import { ForestGate } from './figures/ForestLake/ForestGate';
import { ForestPine } from './figures/ForestLake/ForestPine';
import { GrassPatch } from './figures/ForestLake/GrassPatch';
import { LampPost } from './figures/ForestLake/LampPost';
import { LargeRock } from './figures/ForestLake/LargeRock';
import { LilyPad } from './figures/ForestLake/LilyPad';
import { MossyLog } from './figures/ForestLake/MossyLog';
import { Mushrooms } from './figures/ForestLake/Mushrooms';
import type { Season } from './figures/ForestLake/parts';
import { Pier } from './figures/ForestLake/Pier';
import { Reeds } from './figures/ForestLake/Reeds';
import { Rowboat } from './figures/ForestLake/Rowboat';
import { Ruins } from './figures/ForestLake/Ruins';
import { SignPost } from './figures/ForestLake/SignPost';
import { SpaceshipWreck } from './figures/ForestLake/SpaceshipWreck/SpaceshipWreck';
import { StonePathVariations } from './figures/ForestLake/StonePath/StonePath';
import { SmallRock } from './figures/ForestLake/SmallRock';
import { Stump } from './figures/ForestLake/Stump';
import { Tent } from './figures/ForestLake/Tent';
import { WatchTower } from './figures/ForestLake/WatchTower';
import { BermudaGrass } from './figures/Grass/BermudaGrass';
import { ChivesGrass } from './figures/Grass/ChivesGrass';
import { CockFootGrass } from './figures/Grass/CockFootGrass';
import { MeadowFoxtailGrass } from './figures/Grass/MeadowFoxtailGrass';
import type { GrassOptions } from './figures/Grass/parts';
import { RedFescueGrass } from './figures/Grass/RedFescueGrass';
import { RosemaryGrass } from './figures/Grass/RosemaryGrass';
import { TimothyGrass } from './figures/Grass/TimothyGrass';
import { MidiPiano } from './figures/MidiPiano';
import { Cliff } from './figures/objects/Cliff';
import { Cloud } from './figures/objects/Cloud';
import { Firefly } from './figures/objects/Firefly';
import { Fog } from './figures/objects/Fog';
import { Dock } from './figures/objects/Dock';
import { Log } from './figures/objects/Log';
import { MudPit } from './figures/objects/MudPit';
import { CedarTree } from './figures/Tree/CedarTree';
import { ChestnutTree } from './figures/Tree/ChestnutTree';
import { CrystalTree } from './figures/Tree/CrystalTree';
import { ElmTree } from './figures/Tree/ElmTree';
import { MapleTree } from './figures/Tree/MapleTree';
import { OakTree } from './figures/Tree/OakTree';
import type { TreeOptions } from './figures/Tree/parts';
import { PineTree } from './figures/Tree/PineTree';
import { SpruceTree } from './figures/Tree/SpruceTree';
import { WillowTree } from './figures/Tree/WillowTree';
import type { Stepwise } from './stepwise';
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
  // The space the figures move about in, for the sun's shadow to cover, when
  // it is more than the figure fills when shown (a story's meeting, whose
  // animals come and go). Without it, the shadow fits the figure.
  bounds?: THREE.Box3;
  // The shadow follows what the camera looks at, at the size of `bounds`,
  // rather than staying round the figure's spot: for figures all over the
  // land (the lake meeting's roaming animals), which one shadow over all of
  // it would blur.
  followShadow?: boolean;
  // For a story that moves the camera itself (the lake meeting follows
  // whoever speaks): where the camera should be now. The stage follows it,
  // gliding there or cutting at once, while the camera's view is the
  // figure's own. Dragging the view, picking a direction or walking hands the
  // camera to the viewer, until the story claims it back (Shot.claim).
  shot?(): Shot;
  // For a story whose environment changes while it plays (the forest lake
  // meeting's season, every minute): the stage builds the next one ahead, a
  // piece at a time between frames, and swaps it in under the figure when
  // the story says, without building the figure again (Stage.ts).
  changes?: EnvironmentChanges;
}

export interface EnvironmentChanges {
  // The environment to build next, a piece at a time, or null for none: the
  // same function until it is swapped in, so the stage goes on building it.
  next(): ((theme: Theme) => Stepwise<Environment>) | null;
  // What the story measures from it before it is shown (the meeting's land
  // and sight), a piece at a time, returning what swaps it in on its side.
  // The stage has already placed its scenery where it will stand.
  prepare(environment: Environment): Stepwise<() => void>;
  // Whether to swap it in now, once it is built, measured and compiled.
  due(): boolean;
  // The settings picked another environment for it while it plays (the
  // Season setting): whether the story takes it this way, making it the next
  // and due at once, rather than being built again with it.
  pick(create: (theme: Theme) => Environment): boolean;
}

export interface Shot {
  camera: THREE.Vector3;
  target: THREE.Vector3;
  cut: boolean; // jump there at once instead of gliding
  // Something new to show (the lake meeting's next comment or command): the
  // stage takes the camera back from the viewer, so it isn't missed.
  claim: boolean;
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
  // The fish, in its three kinds (Fish.ts), each swimming its square with a
  // leap in it (fish() below).
  salmon: fish('salmon'),
  piranha: fish('piranha'),
  clownfish: fish('clownfish'),
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
    const slowing = 0.41; // radians it turns while slowing from a run to a stop (an animal's gait eases at 4 a second)
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
  // The animals share the penguin's behaviours without Flap(), and a demo
  // like its own round a circle sized to each (animal() below).
  cat: animal(Cat, 'cat', 0.5),
  wolf: animal(Wolf, 'wolf', 1.3),
  deer: animal(Deer, 'deer', 1.6),
  bird: animal(Bird, 'bird', 0.2),
  bear: animal(Bear, 'bear', 1.6),
  fox: animal(Fox, 'fox', 0.85),
  snake: animal(Snake, 'snake', 0.8),
  frog: animal(Frog, 'frog', 0.2),
  lion: animal(Lion, 'lion', 1.6),
  // The nine from the user's animal park (animalPark.ts).
  crocodile: animal(Crocodile, 'crocodile', 2.2),
  ox: animal(Ox, 'ox', 1.8),
  antelope: animal(Antelope, 'antelope', 1.2),
  tiger: animal(Tiger, 'tiger', 1.6),
  coyote: animal(Coyote, 'coyote', 0.9),
  raccoon: animal(Raccoon, 'raccoon', 0.6),
  monkey: animal(Monkey, 'monkey', 0.5),
  pig: animal(Pig, 'pig', 1.1),
  squirrel: animal(Squirrel, 'squirrel', 0.25),
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
  // The trees' specs have no behaviour yet, so each stands still (tree()
  // below).
  'oak-tree': tree(OakTree),
  'cedar-tree': tree(CedarTree),
  'maple-tree': tree(MapleTree),
  'pine-tree': tree(PineTree),
  'chestnut-tree': tree(ChestnutTree),
  'spruce-tree': tree(SpruceTree),
  'elm-tree': tree(ElmTree),
  'willow-tree': tree(WillowTree),
  // The crystal tree, the user's own model, plays its page's animation;
  // its spec gives no behaviour, so its one button is its page's Glow
  // slider (crystalTree() below).
  'crystal-tree': crystalTree,
  // The grasses' specs have no behaviour yet, so each stands still (plant()
  // below).
  'cock-foot-grass': plant(CockFootGrass),
  'bermuda-grass': plant(BermudaGrass),
  'timothy-grass': plant(TimothyGrass),
  'meadow-foxtail-grass': plant(MeadowFoxtailGrass),
  'red-fescue-grass': plant(RedFescueGrass),
  'chives-grass': plant(ChivesGrass),
  'rosemary-grass': plant(RosemaryGrass),
  // The objects' specs name no behaviour. The cliff, the cloud and the log
  // stay still; the fog drifts and swirls on its own, its idle motion like the
  // lake's waves. Each is seen whole, from in front and a little to the right.
  cliff: (theme) => ({ figure: new Cliff({ theme }), camera: [8, 3.4, 10.5], target: [0, 2.6, 0], update() {} }),
  // The cloud doesn't float by itself, so the preview lifts it, and looks up
  // at it from about eye height, where its grey underside shows.
  cloud: (theme) => {
    const cloud = new Cloud({ theme });
    cloud.position.y = 3; // m
    return { figure: cloud, camera: [5, 1.8, 12], target: [0, 4.6, 0], update() {} };
  },
  fog: (theme) => {
    const fog = new Fog({ theme });
    return {
      figure: fog,
      camera: [3.5, 2.6, 8],
      target: [0, 0.5, 0],
      update(_elapsed, delta) {
        fog.update(delta);
      },
    };
  },
  // The log is seen from a little above too, so the sawn end at its thin end
  // shows. The camera looks right of its middle, which puts it left of the
  // middle of the screen, clear of the panel.
  log: (theme) => ({ figure: new Log({ theme }), camera: [2.85, 1.4, 2.65], target: [0.9, 0.2, 0], update() {} }),
  // The mud pit lies still on the floor: on the lawn its 2-4 cm blades would
  // grow through it. Seen from a little above, so the rim and the puddles show.
  'mud-pit': (theme) => ({ figure: new MudPit({ theme }), camera: [1.9, 1.5, 2.3], target: [0, 0, 0], update() {} }),
  // The dock stands at the lake where the lake meeting has it: from the far
  // bank out over the water, its posts down to the bed, the grass cleared
  // from under it, and its deck ground for the walking camera. It is seen
  // from beside it, a little above. In another environment it stands on the
  // ground, its posts going down into it.
  dock: (theme, environment) => {
    const lake = environment.scenery instanceof Lake ? environment.scenery : null;
    const deck = lake?.siteDock({ angle: FAR_SHORE }) ?? null;
    if (!lake || !deck) return { figure: new Dock({ theme }), camera: [4.6, 2.2, 5.6], target: [0, 0.2, 3], update() {} };
    const land = environment.land;
    const dock = lake.buildDock(theme, deck);
    dock.position.sub(land);
    // From over the water past its far end, a little to its side, looking
    // back at it and the bank: from the land beside it, a tree hung in front.
    const middle = deckPoint(deck, 0, deck.length * 0.45);
    const out = deckPoint(deck, -3.5, deck.length + 4);
    return { figure: dock, camera: [out.x - land.x, 2.6 - land.y, out.z - land.z], target: [middle.x - land.x, 0.3 - land.y, middle.z - land.z], update() {} };
  },
  // The firefly hovers in place, bobbing and flashing on its own, seen from
  // close up, three-quarters from the front: it is 2 cm long. It stays on
  // the floor, where a lawn's 2-4 cm blades would hide it.
  firefly: (theme) => {
    const firefly = new Firefly({ theme });
    return {
      figure: firefly,
      camera: [-0.046, 0.044, 0.056],
      target: [0.004, 0.022, 0.001],
      update(_elapsed, delta) {
        firefly.update(delta);
      },
    };
  },
  // The forest lake's fence (ForestLake/Fence.md): three posts and two
  // sections, standing still, seen from in front and to its left, a little
  // above, as its asset sheet shows it. Its colours are the forest lake's
  // own, not the theme's, in the season of the forest lake it is shown in.
  fence: (_theme, environment) => ({ figure: new Fence({ season: environment.season }), camera: [-1.6, 1.3, 2.7], target: [0.15, 0.42, 0], update() {} }),
  // The rest of the forest lake's assets (figures/ForestLake/), in the
  // asset sheet's order, then the reference's landmarks, then what the
  // other references add (the broadleaf tree, reeds, mushrooms, fallen
  // leaves, the bare winter tree, the forest entrance's gate).
  'forest-pine': forestAsset((season) => new ForestPine({ season })),
  'cherry-tree': forestAsset((season) => new CherryTree({ season })),
  'small-rock': forestAsset((season) => new SmallRock({ season })),
  'large-rock': forestAsset((season) => new LargeRock({ season })),
  bush: forestAsset((season) => new Bush({ season })),
  'grass-patch': forestAsset((season) => new GrassPatch({ season })),
  'flower-cluster': forestAsset((season) => new FlowerCluster({ season })),
  'lily-pad': floating((season) => new LilyPad({ season }), 0),
  'mossy-log': forestAsset((season) => new MossyLog({ season })),
  stump: forestAsset((season) => new Stump({ season })),
  'lamp-post': forestAsset((season) => new LampPost({ season })),
  'sign-post': forestAsset((season) => new SignPost({ season, lantern: true })),
  tent: forestAsset((season) => new Tent({ season })),
  rowboat: floating((season) => new Rowboat({ season }), Rowboat.DRAFT),
  crate: forestAsset((season) => new Crate({ season })),
  barrel: forestAsset((season) => new Barrel({ season })),
  bridge: forestAsset((season) => new Bridge({ season })),
  pier: forestAsset((season) => new Pier({ season })),
  campfire: forestAsset((season) => new Campfire({ season, pot: true })),
  'watch-tower': forestAsset((season) => new WatchTower({ season })),
  ruins: forestAsset((season) => new Ruins({ season })),
  'cave-entrance': forestAsset((season) => new CaveEntrance({ season }), [0.45, 0.3, 0.85]),
  'broadleaf-tree': forestAsset((season) => new BroadleafTree({ season })),
  reeds: forestAsset((season) => new Reeds({ season })),
  mushrooms: forestAsset((season) => new Mushrooms({ season })),
  'fallen-leaves': forestAsset(() => new FallenLeaves()),
  'dead-tree': forestAsset((season) => new DeadTree({ season: season === 'spring' ? 'winter' : season })),
  'forest-gate': forestAsset((season) => new ForestGate({ season })),
  // The spaceship wreck, from the user's reference sheets
  // (ForestLake/SpaceshipWreck/SpaceshipWreck.md), in its clearing by the
  // camp in the forest lake.
  'spaceship-wreck': forestAsset((season) => new SpaceshipWreck({ season })),
  // The stone path, from the user's reference sheet
  // (ForestLake/StonePath/StonePath.md): its five variations side by side,
  // as the sheet's Variation row lays them out, seen from above in front.
  'stone-path': forestAsset((season) => new StonePathVariations({ season }), [0, 0.95, 0.62]),
  // The forest lake's animals, from the user's reference sheets, each
  // playing its sheet's poses (figures/ForestLake/animals/demos.ts).
  ...forestAnimalPreviews,
};

// One of the forest lake's assets (figures/ForestLake/), standing still
// unless it moves on its own (a flame, a flag, a lantern), seen whole from
// in front and to its left, a little above, as the asset sheet shows them.
// Their colours are the forest lake's own, not the theme's, in the season
// of the forest lake they are shown in (spring anywhere else).
function forestAsset(build: (season: Season) => THREE.Object3D & { update?(delta: number): void }, from: [number, number, number] = [-0.55, 0.42, 0.72]) {
  return (_theme: Theme, environment: Environment): Preview => {
    const figure = build(environment.season ?? 'spring');
    const box = new THREE.Box3().setFromObject(figure);
    const center = box.getCenter(new THREE.Vector3());
    const radius = box.getSize(new THREE.Vector3()).length() / 2;
    // Far enough for its bounding sphere to fill most of the 50° view.
    const camera = center.clone().addScaledVector(new THREE.Vector3(...from).normalize(), (radius / Math.sin(THREE.MathUtils.degToRad(25))) * 0.92);
    return {
      figure,
      camera: [camera.x, camera.y, camera.z],
      target: [center.x, center.y, center.z],
      update(_elapsed, delta) {
        figure.update?.(delta);
      },
    };
  };
}

// An asset that floats (the rowboat, the lily pads): in an environment with
// water it sits in it, `draft` m deep, rising and falling with the waves
// under it and tipping with them, easing toward them as a hull lags the
// water it sits in; without water it rests on the ground.
function floating(build: (season: Season) => THREE.Object3D & { keepDry?(): void }, draft: number) {
  return (theme: Theme, environment: Environment): Preview => {
    const preview = forestAsset(build)(theme, environment);
    const figure = preview.figure as THREE.Object3D & { keepDry?(): void };
    const water = environment.water;
    if (!water) return preview;
    figure.keepDry?.();
    figure.position.y = -draft;
    const box = new THREE.Box3().setFromObject(figure);
    const reach = Math.max(0.2, (box.max.x - box.min.x) * 0.3);
    const beam = Math.max(0.2, (box.max.z - box.min.z) * 0.3);
    return {
      ...preview,
      place: 'water',
      update(_elapsed, delta) {
        const ahead = water.heightAt(reach, 0);
        const astern = water.heightAt(-reach, 0);
        const port = water.heightAt(0, -beam);
        const starboard = water.heightAt(0, beam);
        const t = 1 - Math.exp(-3 * delta);
        figure.position.y += ((ahead + astern + port + starboard) / 4 - draft - figure.position.y) * t;
        figure.rotation.z += (Math.atan2(ahead - astern, 2 * reach) - figure.rotation.z) * t;
        figure.rotation.x += (Math.atan2(port - starboard, 2 * beam) - figure.rotation.x) * t;
      },
    };
  };
}

// A tree standing still, seen whole: trees are real size, 10 to 20 m, so the
// camera stands back from the middle of the tree far enough to take in its
// height and its width, a little above eye level.
function tree(Kind: new (options: TreeOptions) => THREE.Object3D) {
  return (theme: Theme): Preview => {
    const figure = new Kind({ theme });
    return { figure, ...treeView(figure), update() {} };
  };
}

// Where the camera stands to see a tree whole (tree() above).
function treeView(figure: THREE.Object3D): Pick<Preview, 'camera' | 'target'> {
  const size = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3());
  const width = Math.max(size.x, size.z);
  const distance = Math.max(1.3 * size.y, 0.9 * width) + 0.3 * width;
  const middle = 0.45 * size.y;
  const view = new THREE.Vector3(0.55, 0.12, 0.83).normalize().multiplyScalar(distance);
  return { camera: [view.x, middle + view.y, view.z], target: [0, middle, 0] };
}

// The crystal tree, seen whole as the trees are, its hanging crystals
// swinging and its sparkles drifting, with the page's Glow slider as its
// one behaviour.
function crystalTree(theme: Theme): Preview {
  const figure = new CrystalTree({ theme });
  return {
    figure,
    ...treeView(figure),
    update: (_elapsed, delta) => figure.update(delta),
    behaviours: [{ name: 'SetGlow', params: [{ name: 'amount', value: '0.3', options: CrystalTree.GLOWS }], run: (amount) => figure.SetGlow(Number(amount)) }],
  };
}

// A grass or herb standing still, seen whole and a little from above: they
// are real size, from 20 cm to a meter tall, so the camera stands back from
// the middle of the plant far enough to take in its height and its width.
function plant(Kind: new (options: GrassOptions) => THREE.Object3D) {
  return (theme: Theme): Preview => {
    const figure = new Kind({ theme });
    const size = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3());
    const width = Math.max(size.x, size.z);
    const distance = Math.max(1.2 * size.y, 1.0 * width) + 0.25 * width;
    const middle = 0.45 * size.y;
    const view = new THREE.Vector3(0.55, 0.35, 0.8).normalize().multiplyScalar(distance);
    return { figure, camera: [view.x, middle + view.y, view.z], target: [0, middle, 0], update() {} };
  };
}

// A fish of one kind. In water it swims a square, in a loop: right along
// the surface, leaping out of the water on the way, diving on the way back,
// left at depth, and rising on the way forward. Each side ends where the fish
// crosses the square's edge and the turns at the corners are the fish's own,
// so the square stays in place although the leap carries the fish faster
// than it swims. The water splashes where the leap breaks its surface. The
// square, the leap and the camera are the salmon's times the kind's size
// (Fish.size), as its own depths and speeds are, so every kind fills the
// view alike. Without water it rests on the floor.
function fish(kind: FishKind) {
  return (theme: Theme, environment: Environment): Preview => {
    const fish = new Fish({ theme, kind });
    const s = fish.size;
    const water = environment.water;
    if (!water) {
      const middle = new THREE.Box3().setFromObject(fish).getCenter(new THREE.Vector3()).y;
      return { figure: fish, camera: [0.35 * s, middle + 0.16 * s, 0.45 * s], target: [0, middle, 0], update() {} };
    }
    fish.addEventListener('splash', ({ x, z, velocity }) => water.splash(x, z, velocity));

    const half = 1 * s; // half a side of the square, m
    const leapAt = -0.7 * s; // swimming right, it leaps once past this x
    const leap = Math.round(30 * s) / 100; // m, how far the leap clears the water: 0.3 for the salmon
    const steps: [start: () => void, done: () => boolean][] = [
      [() => fish.SwimOnSurface('right'), () => fish.position.x > leapAt],
      [() => fish.JumpOutFromWater(leap), () => !fish.jumping && fish.position.x > half],
      [() => fish.SwimOnDepth('back'), () => fish.position.z < -half],
      [() => fish.SwimOnDepth('left'), () => fish.position.x < -half],
      [() => fish.SwimOnSurface('forward'), () => fish.position.z > half],
    ];
    fish.position.set(-half, fish.surfaceY, half); // the corner where it starts right
    fish.rotation.y = Math.PI / 2; // facing right
    let step = -1;
    let demo = true;
    const act = demoStopper(() => (demo = false));
    const direction = { name: 'direction', value: 'right', options: DIRECTIONS };

    return {
      figure: fish,
      place: 'water',
      camera: [1.8 * s, 1.4 * s, 2.4 * s],
      target: [0, -0.2 * s, 0],
      update(_elapsed, delta) {
        if (demo && (step < 0 || steps[step][1]())) {
          step = (step + 1) % steps.length;
          steps[step][0]();
        }
        fish.update(delta);
      },
      // Swimming and leaping need water, so these are offered only on the lake.
      behaviours: [
        { name: 'SwimOnSurface', params: [direction], run: act((d) => fish.SwimOnSurface(d as Direction)) },
        { name: 'SwimOnDepth', params: [direction], run: act((d) => fish.SwimOnDepth(d as Direction)) },
        {
          name: 'JumpOutFromWater',
          params: [{ name: 'height', value: String(leap) }],
          run: act((text) => {
            const height = Number(text);
            if (text.trim() === '' || !Number.isFinite(height)) throw new Error(`height "${text}" is not a number of meters`);
            fish.JumpOutFromWater(height);
          }),
        },
        { name: 'Stop', run: act(() => fish.Stop()) },
      ],
    };
  };
}

// An animal's demo, in a loop, like the penguin's: it says what it will do,
// jumps, carries a fish in its mouth for a while and puts it down, then
// walks half a lap and runs a lap of a circle `radius` meters round,
// stopping on the far side, and back where it began the next time round.
// The animal moves itself forward; the preview
// steers it, turning it by speed / radius so it keeps to the circle at any
// speed, also when a behaviour button makes it walk. The camera stands back
// far enough to see the whole circle, on the side it starts. The first
// behaviour button ends the demo where it is: the animal stops and puts down
// what it holds.
function animal(Kind: new (options: AnimalOptions) => Animal, name: string, radius: number) {
  return (theme: Theme): Preview => {
    const figure = new Kind({ theme });
    const fish = new Fish({ theme });
    // What the Hold button can hand it.
    const holdable: Record<string, THREE.Object3D> = {
      fish,
      'fishing-rod': new FishingRod({ theme }),
      'midi-piano': new MidiPiano({ theme }),
    };
    const box = new THREE.Box3().setFromObject(figure);
    const size = box.getSize(new THREE.Vector3());
    // An animal off the ground would be framed where it is: the view is
    // lifted by its height off the ground and stands back to take in its top
    // (the firefly's, when it was an animal). For the animals on the ground
    // both are what they were.
    const lift = Math.max(0, box.min.y);
    const top = Math.max(size.y, box.max.y);
    const lap = 2 * Math.PI;
    figure.position.set(-radius, 0, 0); // on the circle, facing +z along it
    let finish = 0; // heading (rotation.y) at the end of the laps
    const article = /^[aeiou]/.test(name) ? 'an' : 'a'; // an ox, an antelope
    const steps: [start: () => void, done: (seconds: number) => boolean][] = [
      [() => figure.Speech(`Hi! I'm ${article} ${name}. Watch me jump, carry a fish, then walk and run around.`), () => true],
      [() => figure.Jump(), (s) => s > 2],
      [() => figure.Hold(fish), (s) => s > 3],
      [() => figure.Hold(null), (s) => s > 0.5],
      [
        () => {
          // Counted from the nearest half lap, so small misses don't add up:
          // a loop is a lap and a half, so it stops on the far side of the
          // circle, then back where it began. Rounded to whole laps, every
          // loop after the first walked 0.03 of a lap, and the circle crept
          // inward, a few percent of its radius a loop.
          finish = (Math.round((2 * figure.rotation.y) / lap) / 2 + 1.5) * lap;
          figure.Walk();
        },
        () => figure.rotation.y > finish - lap,
      ],
      // Stops before the finish by about the angle it covers slowing down.
      [() => figure.Run(), () => figure.rotation.y > finish - figure.speed / (4 * radius)],
      [() => figure.Stop(), () => figure.speed < 0.002],
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const act = demoStopper(() => {
      if (!demo) return;
      demo = false;
      figure.Stop();
      figure.Hold(null);
    });

    const reach = radius + Math.max(size.x, size.z) / 2;
    const distance = 1.5 * reach + 1.1 * top;
    // From the side of the circle it starts on (-x), so it is near the camera
    // and seen three-quarters from the side.
    const view = new THREE.Vector3(-0.75, 0.35, 0.55).normalize().multiplyScalar(distance);
    // It looks a little to the right of the circle's middle, which puts the
    // circle left of the middle of the screen, clear of the panel.
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.4 * radius);
    const preview: Preview = {
      figure,
      props: Object.values(holdable),
      camera: [view.x + aim.x, lift + view.y + 0.3 * size.y, view.z + aim.z],
      target: [aim.x, lift + 0.3 * size.y, aim.z],
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step][1](elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step][0]();
        }
        figure.rotation.y += (figure.speed / radius) * delta;
        figure.update(delta);
      },
      behaviours: [
        { name: 'Jump', run: act(() => figure.Jump()) },
        {
          name: 'Hold',
          params: [{ name: 'figure', value: 'fish', options: [...Object.keys(holdable), 'nothing'] }],
          run: act((what) => figure.Hold(holdable[what] ?? null)),
        },
        { name: 'Walk', run: act(() => figure.Walk()) },
        { name: 'Run', run: act(() => figure.Run()) },
        { name: 'Stop', run: act(() => figure.Stop()) },
        {
          name: 'Speech',
          params: [{ name: 'text', value: 'Hello! This text is too long for one bubble, so I say it in several, one after another.' }],
          run: act((text) => figure.Speech(text)),
        },
      ],
    };
    return preview;
  };
}

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
    // Throws up a splash where something breaks the surface (measured from
    // `at`), moving at `velocity` meters a second.
    splash(x: number, z: number, velocity: THREE.Vector3): void;
  };
  update(delta: number): void; // moves it on, such as the waves; called every frame
  // The scene's fog while it is shown, which it thickens and thins itself
  // (the lake's weather); none when left out.
  fog?: THREE.Fog | THREE.FogExp2;
  // Day and night, for an environment that has them (the lake): how far into
  // the night it is now (0 by day, 1 at night), how far its light is toward
  // the night's (`dim`: the night, and a step more under a shower's cloud),
  // by which the stage dims its lights and darkens the background every frame
  // (sceneAt() in theme.ts), and set(), which hands it the Time of day
  // setting. Without it, always day.
  dayNight?: { readonly night: number; readonly dim: number; set(time: TimeOfDay): void };
  // Where the walking camera goes (camera "walk"): the height it stands at,
  // the ground's or the still water's, and how far from the scenery's middle
  // the ground reaches, in the scenery's coordinates. Without it, flat ground
  // at the scenery's level, everywhere.
  ground?: { heightAt(x: number, z: number): number; reach: number };
  // Its own sky colour and lights, used in place of the theme's, for an
  // environment built as a reference shows it rather than in the theme's
  // look (the forest lake). Without it, the theme's.
  light?: EnvironmentLight;
  // Its season, for one that has seasons (the forest lake): its own assets
  // shown in it take the same season. Without it, spring.
  season?: Season;
}

export type EnvironmentLight = Pick<Theme['scene'], 'background' | 'sky' | 'ground' | 'light' | 'ambientIntensity' | 'lightIntensity'>;

// The Time of day setting (?time=): day and night taking turns on their own,
// or held at one of them. Only an environment with day and night (the lake)
// follows it; the others stay in day.
export const TIMES_OF_DAY = ['cycle', 'day', 'night'] as const;
export type TimeOfDay = (typeof TIMES_OF_DAY)[number];

export const environments: Record<string, (theme: Theme) => Environment> = {
  floor: (theme) => {
    const floor = new Floor({ theme });
    return { scenery: floor, land: new THREE.Vector3(), update() {}, ground: { heightAt: () => 0, reach: reachOf(floor) } };
  },
  lake: (theme) => lakeEnvironment(theme),
  // The forest lake, in the season the Season setting picks (environmentIn()
  // below); this is its spring.
  forest_lake: () => forestLakeEnvironment('spring'),
  grass: (theme) => {
    const grass = new Grass({ theme });
    return {
      scenery: grass,
      land: new THREE.Vector3(),
      update: (delta) => grass.update(delta),
      // The ground's, not the border's, whose cliffs and mist stand past it.
      ground: { heightAt: () => 0, reach: reachOf(grass.ground) },
    };
  },
};

// The Season setting (?season=): the season an environment with seasons
// (the forest lake) is shown in, spring by default. The others have none.
export { SEASONS, type Season } from './figures/ForestLake/parts';

// The environments with seasons (the forest lake), one in each season: the
// same function for the same season every time, so the stage sees it
// unchanged until the Season setting changes, and rebuilds it then.
const seasonal: Record<string, Record<Season, (theme: Theme) => Environment>> = {
  forest_lake: {
    spring: environments.forest_lake,
    summer: () => forestLakeEnvironment('summer'),
    autumn: () => forestLakeEnvironment('autumn'),
    winter: () => forestLakeEnvironment('winter'),
  },
};

// The environment `name` in `season`: that season's for one with seasons,
// the environment itself for the others.
export function environmentIn(name: string, season: Season): (theme: Theme) => Environment {
  return seasonal[name]?.[season] ?? environments[name];
}

// Whether the Season setting changes an environment.
export function hasSeasons(name: string): boolean {
  return Object.hasOwn(seasonal, name);
}

// The lake, as an environment. A story whose figures need more level room
// than one figure's builds it with a wider clearing (LakeOptions.clear).
export function lakeEnvironment(theme: Theme, options: Omit<LakeOptions, 'theme'> = {}): Environment {
  const lake = new Lake({ ...options, theme });
  return {
    scenery: lake,
    land: Lake.LANDING.clone(),
    water: {
      at: new THREE.Vector3(),
      heightAt: (x, z) => lake.surfaceAt(x, z),
      splash: (x, z, velocity) => lake.water.splash(x, z, velocity),
    },
    update: (delta) => lake.update(delta),
    fog: lake.weather.fog,
    dayNight: lake.dayNight,
    // Over the water, on its still surface (y = 0), and on a dock's deck.
    ground: { heightAt: (x, z) => Math.max(lake.groundAt(x, z), 0, lake.deckAt(x, z)), reach: reachOf(lake.ground) },
  };
}

// The forest lake (environtments/ForestLake/ForestLake.md), as the user's
// reference images show it, in one of its four seasons (the Season
// setting), in the season's own light rather than the theme's. Figures stand at its landing on the
// south shore; one in the water goes to the open water north of it (in
// winter a hole in the ice). The walking camera walks its land, its water
// (and ice) and its piers' and bridges' decks.
export function forestLakeEnvironment(season: Season = 'spring'): Environment {
  return forestLakeAround(new ForestLake({ season }));
}

// The same, built a piece at a time (ForestLake.build): the forest lake
// meeting builds the next season this way while it plays.
export function* forestLakeBuild(season: Season): Stepwise<Environment> {
  return forestLakeAround(yield* ForestLake.build({ season }));
}

function forestLakeAround(lake: ForestLake): Environment {
  const season = lake.season;
  const at = ForestLake.OPEN_WATER;
  return {
    scenery: lake,
    land: ForestLake.LANDING.clone(),
    water: {
      at: at.clone(),
      heightAt: (x, z) => lake.surfaceAt(x + at.x, z + at.z),
      splash: (x, z, velocity) => lake.water.splash(x + at.x, z + at.z, velocity),
    },
    update: (delta) => lake.update(delta),
    fog: lake.fog,
    light: lake.light,
    season,
    ground: { heightAt: (x, z) => lake.groundAt(x, z), reach: 81 },
  };
}

// How far a ground reaches from its middle: a meter short of its nearest
// edge, measured from its meshes.
function reachOf(ground: THREE.Object3D): number {
  const box = new THREE.Box3().setFromObject(ground);
  return Math.max(0, Math.min(-box.min.x, box.max.x, -box.min.z, box.max.z) - 1);
}

// The environment a figure starts in until one is picked: the boat and the
// fish (its three kinds) on the lake, trees on the grass, the rest on the
// floor.
const figureEnvironments: Record<string, string> = {
  boat: 'lake',
  salmon: 'lake',
  piranha: 'lake',
  clownfish: 'lake',
  ...Object.fromEntries(Object.keys(previews).filter((name) => name.endsWith('-tree')).map((name) => [name, 'grass'])),
  // Objects too big for the floor's 20 m square.
  cliff: 'grass',
  cloud: 'grass',
  fog: 'grass',
  dock: 'lake', // it stands in the water
  rowboat: 'forest_lake', // it floats, and is the forest lake's own
  'forest-spirit-dragon': 'grass', // it flies a circle 24 m across, past the floor's edges
};

export function figureEnvironment(figure: string): string {
  return figureEnvironments[figure] ?? 'floor';
}
