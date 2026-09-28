import * as THREE from 'three';
import { demoStopper, type Behaviour } from '../../../behaviours';
import type { Environment, Preview } from '../../../previews';
import type { Theme } from '../../../theme';
import { addBlade, addFlower, addMoss, addRock, addTube, color, matte, mesh, PALETTE, seededRandom, Shape, twoSided, vary } from '../parts';
import { Butterfly } from './Butterfly';
import { Duck } from './Duck';
import { Explorer } from './Explorer';
import type { ForestAnimal } from './ForestAnimal';
import { ForestDeer } from './ForestDeer';
import { FireflyLeaf, ForestFirefly } from './ForestFirefly';
import { ForestFish } from './ForestFish';
import { ForestFox } from './ForestFox';
import { ForestFrog } from './ForestFrog';
import { ForestSquirrel } from './ForestSquirrel';
import { ForestWolf } from './ForestWolf';
import type { FourLegged } from './FourLegged';
import { Otter } from './Otter';
import { Rabbit } from './Rabbit';
import { WildBoar } from './WildBoar';

// The previews of the forest lake's animals (ForestLake/animals/), for the
// `previews` table in src/previews.ts: each plays the poses of its reference
// sheet in a loop, and its behaviour buttons are its sheet's poses, named as
// its spec writes them, and SetColor for its sheet's colour variations.

// A step of a demo: what starts it, and when it is done (seconds since it
// began).
export interface Step<T> {
  start(figure: T): void;
  done(figure: T, seconds: number): boolean;
}

// Each named behaviour a button, run on the figure, and SetColor with a menu
// of its colours. Running any of them first stops the demo.
function buttons(figure: ForestAnimal, names: readonly string[], stop: () => void): Behaviour[] {
  const act = demoStopper(stop);
  const run = figure as unknown as Record<string, () => void>;
  return [
    ...names.map((name) => ({ name, run: act(() => run[name].call(figure)) })),
    { name: 'SetColor', params: [{ name: 'variation', value: figure.color, options: figure.colors }], run: act((variation) => figure.SetColor(variation)) },
  ];
}

// A four-legged animal's demo, in a loop: it stands, plays its own poses
// (`poses`), then walks half a lap and runs a lap of a circle `radius` m
// round, stopping on the far side of it, and back where it began the next
// time round, as the other animals' demos do. It moves itself forward; the
// preview steers it, turning it by speed / radius. The camera stands back
// far enough to see the whole circle, three-quarters from the side it
// starts on, and looks a little right of the circle's middle, clear of the
// panel.
function fourLegged<T extends FourLegged>(build: () => T, radius: number, poses: readonly Step<T>[], actions: readonly string[]) {
  return (_theme: Theme, _environment: Environment): Preview => {
    const figure = build();
    const box = new THREE.Box3().setFromObject(figure);
    const size = box.getSize(new THREE.Vector3());
    const lap = 2 * Math.PI;
    figure.position.set(-radius, 0, 0); // on the circle, facing +z along it
    let finish = 0;
    const steps: Step<T>[] = [
      { start: (f) => f.Idle(), done: (_f, s) => s > 2.5 },
      ...poses,
      {
        start: (f) => {
          // Counted from the nearest half lap, so small misses don't add up.
          finish = (Math.round((2 * f.rotation.y) / lap) / 2 + 1.5) * lap;
          f.Walk();
        },
        done: (f) => f.rotation.y > finish - lap,
      },
      // Stops before the finish by about the angle it covers slowing down.
      { start: (f) => f.Run(), done: (f) => f.rotation.y > finish - f.speed / (3.5 * radius) },
      { start: (f) => f.Idle(), done: (f) => f.speed < 0.002 },
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    const reach = radius + Math.max(size.x, size.z) / 2;
    const distance = 1.5 * reach + 1.1 * size.y;
    const view = new THREE.Vector3(-0.75, 0.35, 0.55).normalize().multiplyScalar(distance);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.4 * radius);
    return {
      figure,
      camera: [view.x + aim.x, view.y + 0.3 * size.y, view.z + aim.z],
      target: [aim.x, 0.3 * size.y, aim.z],
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        figure.rotation.y += (figure.speed / radius) * delta;
        figure.update(delta);
      },
      behaviours: buttons(figure, ['Idle', 'Walk', 'Run', ...actions], stop),
    };
  };
}

// A trunk for the squirrel to climb: a tapering length of bark with roots
// flaring at its foot, in the forest lake's colours, `height` m tall.
function trunk(height: number): THREE.Mesh {
  const random = seededRandom(31);
  const shape = new Shape();
  const bark = color(PALETTE.bark);
  const dark = color(PALETTE.barkDark);
  const steps = 7;
  const path = Array.from({ length: steps + 1 }, (_, i) => new THREE.Vector3(0, -0.05 + ((height + 0.05) * i) / steps, 0));
  const radii = path.map((_, i) => (i === 0 ? 0.16 : i === 1 ? 0.12 : 0.1 - (0.02 * i) / steps));
  const shades = Array.from({ length: 8 }, () => vary(bark, random, 2));
  addTube(shape, path, radii, { sides: 8, rough: 0.06, random, paint: (r, i) => (r === 0 ? dark : shades[i]), caps: ['open', 'rings'] });
  return mesh(shape.geometry(), matte());
}

// The squirrel's demo, in a loop: it sits with its acorn, climbs the trunk
// before it (ForestSquirrel.TRUNK ahead), comes down and sits facing away,
// leaps away from the tree, sits, runs a lap of a small circle, then walks
// a half lap round and up to a second tree beside the first, facing it as
// it faced the first. The next loop climbs that one and comes back round
// the other way, so it goes to and fro between the two, and neither tree
// is ever in its way. It moves itself forward; the preview steers it,
// turning it by speed / radius, one way or the other.
function squirrel(): (theme: Theme, environment: Environment) => Preview {
  return () => {
    const figure = new ForestSquirrel();
    const rho = 0.3; // m, the circles' radius
    const leap = 0.45; // m a jump carries it
    const gap = ForestSquirrel.TRUNK;
    const trees = [trunk(1.1), trunk(1.1)];
    // Their faces where it climbs, TRUNK ahead of where it starts each
    // climb: the first where it begins, the second two radii to its right.
    trees[0].position.set(0, 0, gap + 0.092);
    trees[1].position.set(-2 * rho, 0, gap + 0.092);
    trees[1].rotation.y = 1.3;
    let turn = 1; // which way it turns this loop
    let turned = 0; // rad it has turned in this step
    let goal = 0; // rad it turns in this step
    let from = new THREE.Vector3();
    const steps: Step<ForestSquirrel>[] = [
      { start: (f) => f.Idle(), done: (_f, s) => s > 3 },
      { start: (f) => f.Climb(), done: (f, s) => s > 0.5 && !f.climbing },
      { start: (f) => f.Idle(), done: (_f, s) => s > 2 },
      { start: (f) => f.Jump(), done: (f, s) => s > 0.4 && !f.jumping },
      { start: (f) => f.Idle(), done: (_f, s) => s > 1.2 },
      { start: (f) => ((turned = 0), (goal = 2 * Math.PI), f.Run()), done: () => turned >= goal },
      { start: (f) => ((turned = 0), (goal = Math.PI), f.Walk()), done: () => turned >= goal },
      // Short of the tree by as far as it goes slowing to a stop.
      { start: (f) => ((from = f.position.clone()), (goal = 0), f.Walk()), done: (f) => f.position.distanceTo(from) >= leap - 0.045 },
      {
        start: (f) => {
          f.Idle();
          turn = -turn;
        },
        done: (f) => f.speed < 0.002,
      },
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    // Steering: the laps turn it, the walk up to the tree goes straight.
    const steering = () => step === 5 || step === 6;
    const bounds = new THREE.Box3(new THREE.Vector3(-3 * rho - 0.2, 0, -leap - 2 * rho - 0.1), new THREE.Vector3(rho + 0.2, 1.1, gap + 0.25));
    const centre = bounds.getCenter(new THREE.Vector3());
    // From behind it as it climbs (it climbs the trunks' far faces), a
    // little to the side of the first tree, so the second is not in the way.
    const view = new THREE.Vector3(0.62, 0.42, -0.85).normalize().multiplyScalar(2.2);
    return {
      figure,
      props: trees,
      bounds,
      camera: [centre.x + view.x, 0.25 + view.y, centre.z + view.z],
      target: [centre.x + 0.15, 0.25, centre.z],
      update(elapsed, delta) {
        // The trees stand in the scene beside it, not in it.
        for (const tree of trees) if (!tree.parent && figure.parent) figure.parent.add(tree);
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        if (demo && steering()) {
          // Never past the step's turn, so each lap ends heading exactly
          // as it should.
          const angle = Math.min((figure.speed / rho) * delta, goal - turned);
          figure.rotation.y += turn * angle;
          turned += angle;
        }
        figure.update(delta);
      },
      dispose() {
        for (const tree of trees) tree.removeFromParent();
      },
      behaviours: buttons(figure, ['Idle', 'Walk', 'Run', 'Jump', 'Climb'], stop),
    };
  };
}

// Until it is back at rest after a pose that ends by itself.
const idleAgain = <T extends FourLegged>(f: T, s: number) => s > 0.5 && f.doing === 'idle';

// Where the firefly's leaf lies from the preview's middle (m): under where
// its landings end, a little past the middle of its circle, along its turn's
// last glide.
const FIREFLY_LEAF_AT = new THREE.Vector3(-0.005, 0, 0.014);

// The firefly's demo, in a loop, as its sheet's poses go: it idles and
// hovers over a leaf, flies out and once round a circle about the leaf,
// turns in and lands on the leaf, rests there with its wings folded and
// takes off again. The circle is twice as wide as its turn
// (ForestFirefly.TURN_RADIUS), so its Turn() from anywhere on the circle
// ends over the leaf. It moves itself forward; the preview steers it by
// turning it: half a circle out from the leaf onto the circle, then round
// it, keeping to it. Idling and hovering it turns back to face +z, so every
// loop goes the same way. Flying on its own (the Fly button) it keeps round
// the circle. The leaf is the preview's, not the firefly's: it goes into the
// scenery, and the firefly lands on it through its surfaceAt (the Land
// button too, when it is over the leaf; elsewhere it lands on the ground).
// The camera looks at the leaf from close by, three-quarters from the front
// on the side it flies out from: the firefly is 2 cm long. The sun's shadow
// covers the circle and the leaf (`bounds`), not only the firefly.
function forestFirefly(): (theme: Theme, environment: Environment) => Preview {
  return (_theme: Theme, environment: Environment): Preview => {
    const figure = new ForestFirefly();
    const radius = 2 * ForestFirefly.TURN_RADIUS;
    const leaf = new FireflyLeaf();
    leaf.position.copy(environment.land).add(FIREFLY_LEAF_AT);
    environment.scenery.add(leaf);
    figure.surfaceAt = (x, z) => leaf.heightAt(x, z) ?? 0;

    const wrap = (a: number) => Math.atan2(Math.sin(a), Math.cos(a));
    const around = () => Math.atan2(figure.position.x, figure.position.z); // its heading from the middle
    let steer: 'none' | 'out' | 'round' = 'none';
    let until = 0; // its heading at the end of the flight out
    let travelled = 0; // radians it has gone round the circle
    let last = 0;
    const steps: Step<ForestFirefly>[] = [
      {
        start: (f) => {
          steer = 'none';
          f.rotation.y = wrap(f.rotation.y); // the same heading, fewer laps counted
          f.Idle();
        },
        done: (_f, s) => s > 3,
      },
      { start: (f) => f.Hover(), done: (_f, s) => s > 2.5 },
      {
        start: (f) => {
          steer = 'out';
          until = f.rotation.y + Math.PI;
          f.Fly();
        },
        done: (f) => f.rotation.y >= until,
      },
      {
        start: () => {
          steer = 'round';
          travelled = 0;
          last = around();
        },
        done: () => travelled >= 2 * Math.PI,
      },
      {
        start: (f) => {
          steer = 'none';
          f.Turn();
        },
        done: (f) => f.doing === 'hover',
      },
      { start: (f) => f.Land(), done: (f, s) => s > 0.5 && f.doing === 'idle' },
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      steer = 'none';
    };
    // Where it goes: round the circle, its wings out, and the leaf.
    const reach = radius + 0.02;
    const bounds = new THREE.Box3(new THREE.Vector3(-reach, 0, -reach), new THREE.Vector3(reach, ForestFirefly.HOVER + 0.02, reach));
    leaf.geometry.computeBoundingBox();
    if (leaf.geometry.boundingBox) bounds.union(leaf.geometry.boundingBox.clone().translate(FIREFLY_LEAF_AT));
    const view = new THREE.Vector3(-0.75, 0.42, 0.55).normalize().multiplyScalar(0.15);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.3 * radius).setY(0.024);
    return {
      figure,
      bounds,
      camera: [aim.x + view.x, aim.y + view.y, aim.z + view.z],
      target: [aim.x, aim.y, aim.z],
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        if (steer === 'out') figure.rotation.y += (figure.speed / (radius / 2)) * delta;
        else if (steer === 'round' || (!demo && figure.doing === 'fly')) {
          // Round the circle, turned back onto it when off it.
          const a = around();
          travelled += wrap(a - last);
          last = a;
          const off = Math.hypot(figure.position.x, figure.position.z) - radius;
          const want = a + Math.PI / 2 + THREE.MathUtils.clamp((1.5 * off) / radius, -0.8, 0.8);
          figure.rotation.y += (figure.speed / radius + 6 * wrap(want - figure.rotation.y)) * delta;
        } else if (demo && (figure.doing === 'idle' || figure.doing === 'hover')) {
          figure.rotation.y += wrap(-figure.rotation.y) * (1 - Math.exp(-1.5 * delta));
        }
        figure.update(delta);
      },
      behaviours: buttons(figure, ['Idle', 'Hover', 'Fly', 'Turn', 'Land'], stop),
    };
  };
}

// The fish's demo, in a loop, as its sheet's poses go. In water (the lake)
// it hovers at its depth, opens and closes its mouth, swims off, leaps out
// of the water and dives back in ahead, hovers, turns round (its sharp
// turn, its body bent into a C), swims back to where it began, hovers and
// turns round again, so every loop starts where the last one did, facing
// the same way. The water splashes where the leap breaks its surface. It
// swims straight: the loop never steers it. Once a button stops the demo it
// is steered round a circle whenever it moves, so swimming on its own it
// stays in view. Without water it lies on the floor, resting on its fins,
// and plays the poses that stay in place (a leap there is a flop). In the
// water the camera looks back down its run from ahead of it on its right,
// close and from above (the lake's water hides much of what is under it):
// the run foreshortened, it and the leap stay in view, clear of the panel.
function forestFish(): (theme: Theme, environment: Environment) => Preview {
  return (_theme: Theme, environment: Environment): Preview => {
    const figure = new ForestFish();
    const radius = 0.8; // m, the circle it is steered round on its own
    const water = environment.water;
    // Back to hovering after a pose that ends by itself.
    const idleAgain = (f: ForestFish, s: number) => s > 0.3 && f.doing === 'idle' && !f.jumping;
    // Stopped, after swimming or a leap.
    const still = (f: ForestFish, s: number) => s > 0.5 && f.speed < 0.005;
    const wrap = (a: number) => Math.atan2(Math.sin(a), Math.cos(a));
    let steps: Step<ForestFish>[];
    let place: 'land' | 'water' = 'land';
    let camera: [number, number, number];
    let target: [number, number, number];
    let bounds: THREE.Box3 | undefined;
    if (water) {
      place = 'water';
      figure.addEventListener('splash', ({ x, z, velocity }) => water.splash(x, z, velocity));
      // Each loop begins here, facing +z: its run (a swim and a leap, about
      // 2.2 m) then ends as far past the middle as it began before it.
      const start = -1.1;
      figure.position.set(0, figure.swimY, start);
      steps = [
        {
          start: (f) => {
            f.rotation.y = wrap(f.rotation.y); // the same heading, fewer turns counted
            f.Idle();
          },
          done: (_f, s) => s > 2.5,
        },
        { start: (f) => f.OpenMouth(), done: idleAgain },
        { start: (f) => f.Swim(), done: (_f, s) => s > 1.2 },
        { start: (f) => f.Jump(), done: (f, s) => s > 0.2 && !f.jumping },
        { start: (f) => f.Idle(), done: still },
        { start: (f) => f.Turn(180), done: idleAgain },
        // Back where it began, short of it by about as far as it glides
        // slowing to a stop (its speed over how fast it slows, 2.2 a second).
        { start: (f) => f.Swim(), done: (f, s) => s > 0.5 && f.position.z <= start + f.speed / 2.2 },
        { start: (f) => f.Idle(), done: still },
        { start: (f) => f.Turn(180), done: idleAgain },
      ];
      camera = [1.05, 1.0, 1.75];
      target = [0.15, -0.1, -0.05];
      bounds = new THREE.Box3(new THREE.Vector3(-0.5, -0.5, -1.4), new THREE.Vector3(0.5, 0.7, 1.4));
    } else {
      steps = [
        { start: (f) => f.Idle(), done: (_f, s) => s > 3 },
        { start: (f) => f.OpenMouth(), done: idleAgain },
        { start: (f) => f.Idle(), done: (_f, s) => s > 1.5 },
        { start: (f) => f.Turn(180), done: idleAgain },
        { start: (f) => f.Jump(), done: idleAgain },
        { start: (f) => f.Turn(180), done: idleAgain },
      ];
      camera = [0.6, 0.3, 0.5];
      target = [0, 0.1, 0.06];
    }
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    return {
      figure,
      place,
      camera,
      target,
      bounds,
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        // On its own, round a circle whenever it moves (a leap holds its
        // heading until it is back in the water).
        if (!demo) figure.rotation.y += (figure.speed / radius) * delta;
        figure.update(delta);
      },
      behaviours: buttons(figure, ['Idle', 'Swim', 'Turn', 'OpenMouth', 'Jump'], stop),
    };
  };
}

// The duck's demo, in a loop, as its sheet's poses go. On the lake it floats
// (Idle), swims a lap of a circle `radius` m round (Swim), beats its wings
// (FlapWings), then takes off from the water, flies about once round the
// circle and lands on the water again (TakeOff); it rides the waves, and its
// splashes break the water's surface. Elsewhere it stands (Idle), beats its
// wings (FlapWings), waddles half a lap of the circle (Walk) and runs a lap
// (Run), stopping on the far side of it, then takes off, flies about once
// round and lands (TakeOff). Swimming needs water, so Swim is offered only on
// the lake; there Walk swims and Run paddles hard. It moves itself forward;
// the preview steers it, turning it by speed / radius. The camera stands
// back far enough to see the whole circle and its flight over it.
function duck(): (theme: Theme, environment: Environment) => Preview {
  return (_theme, environment) => {
    const figure = new Duck();
    const water = environment.water;
    const radius = 1.2; // m: a flight takes it about once round
    const lap = 2 * Math.PI;
    figure.position.set(-radius, 0, 0); // on the circle, facing +z along it
    if (water) {
      figure.afloat = true;
      figure.onSplash = (x, z, velocity) => water.splash(x, z, velocity);
    }
    let finish = 0;
    // `laps` on from the nearest half lap, so small misses don't add up.
    const lapsOn = (f: Duck, laps: number) => {
      finish = (Math.round((2 * f.rotation.y) / lap) / 2 + laps) * lap;
    };
    // Until it is back at rest after a pose that ends by itself.
    const idleAgain = (f: Duck, s: number) => s > 0.5 && f.doing === 'idle';
    // Each going step stops short of its finish by about the angle it covers
    // slowing down: afloat its speed eases away at 2.5 a second, on its feet
    // at 3.5.
    const steps: Step<Duck>[] = water
      ? [
          { start: (f) => f.Idle(), done: (_f, s) => s > 3 },
          {
            start: (f) => {
              lapsOn(f, 1);
              f.Swim();
            },
            done: (f) => f.rotation.y > finish - f.speed / (2.5 * radius),
          },
          { start: (f) => f.Idle(), done: (f, s) => s > 1.5 && f.speed < 0.01 },
          { start: (f) => f.FlapWings(), done: idleAgain },
          { start: (f) => f.Idle(), done: (_f, s) => s > 1 },
          { start: (f) => f.TakeOff(), done: idleAgain },
          { start: (f) => f.Idle(), done: (_f, s) => s > 2 },
        ]
      : [
          { start: (f) => f.Idle(), done: (_f, s) => s > 2.5 },
          { start: (f) => f.FlapWings(), done: idleAgain },
          { start: (f) => f.Idle(), done: (_f, s) => s > 1 },
          {
            start: (f) => {
              lapsOn(f, 1.5);
              f.Walk();
            },
            done: (f) => f.rotation.y > finish - lap,
          },
          { start: (f) => f.Run(), done: (f) => f.rotation.y > finish - f.speed / (3.5 * radius) },
          { start: (f) => f.Idle(), done: (f) => f.speed < 0.002 },
          { start: (f) => f.TakeOff(), done: idleAgain },
          { start: (f) => f.Idle(), done: (_f, s) => s > 1.5 },
        ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    const size = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3());
    const reach = radius + Math.max(size.x, size.z) / 2;
    const high = Duck.FLY_HEIGHT + size.y;
    // The circle and the air over it, for the sun's shadow to cover.
    const bounds = new THREE.Box3(new THREE.Vector3(-reach, -Duck.SWIM_DEPTH, -reach), new THREE.Vector3(reach, high, reach));
    const distance = 1.5 * reach + 1.1 * high;
    const view = new THREE.Vector3(-0.75, 0.35, 0.55).normalize().multiplyScalar(distance);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.4 * radius);
    const middle = 0.3 * high;
    return {
      figure,
      place: water ? 'water' : 'land',
      bounds,
      camera: [view.x + aim.x, view.y + middle, view.z + aim.z],
      target: [aim.x, middle, aim.z],
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        figure.rotation.y += (figure.speed / radius) * delta;
        // Riding the waves under it.
        if (water) figure.surface = water.heightAt(figure.position.x, figure.position.z);
        figure.update(delta);
      },
      behaviours: buttons(figure, water ? ['Idle', 'Walk', 'Run', 'Swim', 'FlapWings', 'TakeOff'] : ['Idle', 'Walk', 'Run', 'FlapWings', 'TakeOff'], stop),
    };
  };
}

// The frog's demo, in a loop, as its sheet's poses go. On the lake it floats
// at the surface, bobbing on the waves, swims half a lap of a circle, floats
// again and leaps out of the water and dives back in, splashing as it kicks
// and wherever it breaks the surface. On dry ground it sits, hops a quarter
// of a lap, sits, leaps, and sits again. Each loop goes on round the circle
// from where the last ended. It moves itself forward; the preview steers it,
// turning it by speed / radius, so it keeps to the circle at any speed, also
// when a button makes it walk, swim or leap. The camera stands back far
// enough to see the whole circle, three-quarters from the side it starts
// on, and looks a little right of the circle's middle, clear of the panel.
function forestFrog(): (theme: Theme, environment: Environment) => Preview {
  return (_theme: Theme, environment: Environment): Preview => {
    const figure = new ForestFrog();
    const water = environment.water;
    const radius = 0.2; // m
    figure.inWater = Boolean(water);
    if (water) figure.onSplash = (x, z, velocity) => water.splash(x, z, velocity);
    figure.position.set(-radius, 0, 0); // on the circle, facing +z along it
    const size = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3());
    let finish = 0; // its heading when the step going round ends
    const round = (angle: number, f: ForestFrog) => (finish = f.rotation.y + angle);
    const idleAgain = (f: ForestFrog, s: number) => s > 0.5 && f.doing === 'idle';
    const steps: Step<ForestFrog>[] = water
      ? [
          { start: (f) => f.Idle(), done: (_f, s) => s > 3 },
          { start: (f) => (round(Math.PI, f), f.Swim()), done: (f) => f.rotation.y >= finish },
          { start: (f) => f.Idle(), done: (_f, s) => s > 2 },
          { start: (f) => f.Jump(), done: idleAgain },
        ]
      : [
          { start: (f) => f.Idle(), done: (_f, s) => s > 3 },
          { start: (f) => (round(Math.PI / 2, f), f.Walk()), done: (f) => f.rotation.y >= finish },
          // A hop under way ends first.
          { start: (f) => f.Idle(), done: (f, s) => s > 1.5 && f.doing === 'idle' },
          { start: (f) => f.Jump(), done: idleAgain },
        ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    const reach = radius + Math.max(size.x, size.z) / 2;
    const distance = 1.5 * reach + 1.1 * size.y;
    const view = new THREE.Vector3(-0.75, 0.35, 0.55).normalize().multiplyScalar(distance);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.4 * radius);
    return {
      figure,
      place: 'water',
      camera: [view.x + aim.x, view.y + 0.3 * size.y, view.z + aim.z],
      target: [aim.x, 0.3 * size.y, aim.z],
      // The circle, and as high as it leaps.
      bounds: new THREE.Box3(new THREE.Vector3(-reach, 0, -reach), new THREE.Vector3(reach, 0.15, reach)),
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        figure.rotation.y += (figure.speed / radius) * delta;
        // Afloat, its origin rides the water's surface.
        if (water) figure.position.y = water.heightAt(figure.position.x, figure.position.z);
        figure.update(delta);
      },
      behaviours: buttons(figure, ['Idle', 'Walk', 'Jump', 'Swim'], stop),
    };
  };
}

// The butterfly's flower, as its sheet's Land on Flower draws it: a white
// head of seven petals round a raised yellow middle, facing up on a slender
// stem among three long leaves, in the forest lake's colours. The middle of
// its head is `height` m up.
function butterflyFlower(height: number): THREE.Group {
  const random = seededRandom(43);
  const green = new Shape();
  const head = new Shape();
  const foot = new THREE.Vector3(0, -0.01, 0);
  const top = new THREE.Vector3(0, height, 0);
  addTube(green, [foot, new THREE.Vector3(0.006, height * 0.55, 0.004), top], [0.0075, 0.0065, 0.0055], {
    sides: 3,
    random,
    paint: () => vary(color(PALETTE.grass), random, 1),
  });
  addFlower(head, top, new THREE.Vector3(0, 1, 0), 7, 0.075, color(PALETTE.petalWhite), color(PALETTE.flowerHeart), random, 0.2);
  for (let j = 0; j < 3; j++) {
    const a = (2 * Math.PI * (j + 0.3 * random())) / 3;
    const out = new THREE.Vector3(Math.cos(a), 0, Math.sin(a));
    const across = new THREE.Vector3(-out.z, 0, out.x);
    const colors = [PALETTE.grassFoot, PALETTE.grass, PALETTE.grassTip].map((c) => vary(color(c), random)) as [THREE.Color, THREE.Color, THREE.Color];
    addBlade(green, foot, out, across, 0.13 + 0.07 * random(), 0.5 + 0.4 * random(), 0.04, ...colors);
  }
  const flower = new THREE.Group();
  flower.add(mesh(green.geometry(), matte()), mesh(head.geometry(), matte()));
  return flower;
}

const BUTTERFLY_FLOWER = 0.16; // m, the middle of the flower's head
const BUTTERFLY_PERCH = BUTTERFLY_FLOWER + 0.0055; // m, its raised middle where the butterfly's feet stand

// The butterfly's demo, in a loop, as its sheet's poses go: it hovers over
// a flower, its wings beating between Flap Up and Flap Down (Idle), flies
// once round a circle of 26 cm radius that starts and ends at the flower
// (Fly (Side)), and lands on the flower (Land on Flower): it slows to a stop
// over it, settles, rests with its wings opening and closing slowly, and
// rises back to hovering by itself. It calls Land short of the flower by as
// far as it goes on slowing down (its speed eases by Butterfly.SLOWING a
// second), reckoned frame by frame, and counts laps from where it is round
// the circle, from the nearest whole one, so small misses don't add up. It
// moves itself forward; the preview steers it round the circle, turning it
// by speed / radius, so the Fly button flies it round too. The flower is a
// prop beside it; the Land button lands it on the flower when it is over it
// (its `perch`), on the ground elsewhere. The camera looks at the circle
// from three-quarters in front, the flower on the near side: the butterfly
// is 10 cm across.
function butterfly(): (theme: Theme, environment: Environment) => Preview {
  return () => {
    const figure = new Butterfly();
    const radius = 0.26;
    const lap = 2 * Math.PI;
    const flower = butterflyFlower(BUTTERFLY_FLOWER);
    flower.position.set(-radius, 0, 0);
    figure.position.set(-radius, 0, 0); // over the flower, on the circle, facing +z along it
    // How far round the circle it is from the flower (rad, counted on by its
    // heading).
    const around = (b: Butterfly) => {
      const a = Math.atan2(b.position.z, -b.position.x);
      return a + lap * Math.round((b.rotation.y - a) / lap);
    };
    // How far it goes on once told to Land, flying at `speed`: the eased
    // speeds it moves at, frame by frame, till it stops.
    const coast = (speed: number, delta: number) => {
      const q = Math.exp(-Butterfly.SLOWING * delta);
      return (speed * delta * q) / (1 - q);
    };
    let frame = 1 / 60; // s, the last frame's delta, to time the landing by
    let finish = 0;
    const steps: Step<Butterfly>[] = [
      { start: (b) => b.Idle(), done: (_b, s) => s > 3 },
      {
        start: (b) => {
          finish = (Math.round(around(b) / lap) + 1) * lap;
          b.Fly();
        },
        // Within half a frame's flight of where it should be told.
        done: (b) => (finish - around(b)) * radius <= coast(b.speed, frame) + (b.speed * frame) / 2,
      },
      { start: (b) => b.Land(BUTTERFLY_PERCH), done: (b, s) => s > 1 && b.doing === 'idle' },
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      demo = false;
    };
    const bounds = new THREE.Box3(new THREE.Vector3(-radius - 0.1, 0, -radius - 0.1), new THREE.Vector3(radius + 0.1, Butterfly.HOVER + 0.06, radius + 0.1));
    const view = new THREE.Vector3(-0.75, 0.4, 0.55).normalize().multiplyScalar(0.85);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.3 * radius).setY(0.16);
    return {
      figure,
      props: [flower],
      bounds,
      camera: [aim.x + view.x, aim.y + view.y, aim.z + view.z],
      target: [aim.x, aim.y, aim.z],
      update(elapsed, delta) {
        // The flower stands in the scene beside it, not in it.
        if (!flower.parent && figure.parent) figure.parent.add(flower);
        if (delta > 0) frame = delta;
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        // Round the circle, and while it moves, turned back onto it when off
        // it (never turned at rest, which would slide its feet).
        figure.rotation.y += (figure.speed / radius) * delta;
        if (figure.speed > 0.01) {
          const off = Math.hypot(figure.position.x, figure.position.z) - radius;
          const want = around(figure) + THREE.MathUtils.clamp((1.5 * off) / radius, -0.8, 0.8);
          figure.rotation.y += 6 * (want - figure.rotation.y) * delta;
        }
        // Over the flower, Land lands on it.
        figure.perch = Math.hypot(figure.position.x - flower.position.x, figure.position.z - flower.position.z) < 0.03 ? BUTTERFLY_PERCH : 0;
        figure.update(delta);
      },
      dispose() {
        flower.removeFromParent();
      },
      behaviours: buttons(figure, ['Idle', 'Fly', 'Land'], stop),
    };
  };
}

// A fallen log for the explorer to sit on: a length of bark lying across,
// sawn ends, a tuft of moss, its top Explorer.SEAT high, half sunk in the
// ground. Built lying along x round its middle.
function explorerLog(): THREE.Mesh {
  const random = seededRandom(47);
  const shape = new Shape();
  const r = 0.12;
  const y = Explorer.SEAT - r;
  const path = Array.from({ length: 5 }, (_, i) => new THREE.Vector3(-0.42 + (0.84 * i) / 4, y, 0));
  const bark = color(PALETTE.bark);
  addTube(shape, path, path.map((_, i) => r * (1 + 0.05 * Math.sin(i * 2.3))), { sides: 9, rough: 0.05, random, paint: () => vary(bark, random, 2), caps: ['grain', 'grain'] });
  addMoss(shape, new THREE.Vector3(0.16, Explorer.SEAT + 0.002, -0.04), 0.05, random);
  addMoss(shape, new THREE.Vector3(-0.5, 0, 0.1), 0.08, random);
  return mesh(shape.geometry(), matte());
}

// A sapling springing up between two mossy stones, for the explorer to
// touch: a thin green stem and two leaves reaching up and out at the height
// it touches (Explorer.TOUCH), a smaller one lower down.
function explorerSapling(): THREE.Mesh {
  const random = seededRandom(53);
  const shape = new Shape();
  const top = Explorer.TOUCH.y - 0.03;
  const stem = [0, 0.33, 0.66, 1].map((f) => new THREE.Vector3(0.006 * Math.sin(f * 3), f * top, 0.004 * f));
  addTube(shape, stem, [0.009, 0.008, 0.006, 0.004], { sides: 5, random, paint: () => vary(color(PALETTE.leafDark), random, 2), caps: ['open', color(PALETTE.leafDark)] });
  const leaf = (at: THREE.Vector3, turn: number, rise: number, length: number, width: number) => {
    const dir = new THREE.Vector3(Math.sin(turn) * Math.cos(rise), Math.sin(rise), Math.cos(turn) * Math.cos(rise));
    const across = new THREE.Vector3(Math.cos(turn), 0, -Math.sin(turn)).multiplyScalar(width);
    const tip = at.clone().addScaledVector(dir, length);
    const mid = at.clone().addScaledVector(dir, length * 0.45);
    const fresh = color(PALETTE.leafLight);
    const deep = color(PALETTE.leaf);
    shape.triangle(at, mid.clone().add(across), tip, deep, fresh, fresh);
    shape.triangle(at, tip, mid.clone().sub(across), deep, fresh, fresh);
  };
  const crown = stem[3];
  leaf(crown, 0.9, 0.55, 0.085, 0.024);
  leaf(crown, -2.2, 0.75, 0.075, 0.022);
  leaf(stem[1].clone().setY(top * 0.45), 2.6, 0.35, 0.05, 0.016);
  addRock(shape, new THREE.Vector3(-0.08, 0, 0.02), new THREE.Vector3(0.11, 0.08, 0.1), 0.4, random, 1);
  addRock(shape, new THREE.Vector3(0.09, 0, -0.03), new THREE.Vector3(0.13, 0.1, 0.11), 1.2, random, 1);
  addMoss(shape, new THREE.Vector3(0, 0, 0.01), 0.06, random);
  return mesh(shape.geometry(), twoSided());
}

// The explorer's demo, in a loop, as its sheet's Animation Preview runs:
// it stands, waves, walks half a lap and runs a lap of a circle 1.1 m round
// (stopping on the far side of it, and back where it began the next time
// round, as the animals' demos do), jumps, kneels to touch a sapling, and
// sits on a log, then stands; each pose with one of its sheet's faces. It
// moves itself forward; the preview steers it, turning it by speed /
// radius. A sapling springs up before it whenever it kneels to touch
// (Interact), and a log rolls up behind it whenever it sits, the buttons'
// too; each shrinks away after. Its tools and faces are behaviour buttons
// with menus.
function explorer(): (theme: Theme, environment: Environment) => Preview {
  return () => {
    const figure = new Explorer();
    const radius = 1.1;
    const lap = 2 * Math.PI;
    figure.position.set(-radius, 0, 0); // on the circle, facing +z along it
    const box = new THREE.Box3().setFromObject(figure);
    const size = box.getSize(new THREE.Vector3());

    // The props: a sapling to touch and a log to sit on, each grown in
    // where the figure is when it starts (placed in its own space), shrunk
    // away when it is done.
    const sapling = explorerSapling();
    const log = explorerLog();
    const props = [
      { object: sapling as THREE.Object3D, at: new THREE.Vector3(Explorer.TOUCH.x, 0, Explorer.TOUCH.z + 0.02), wanted: (doing: string) => doing === 'interact', scale: 0, placed: false },
      { object: log as THREE.Object3D, at: new THREE.Vector3(0, 0, -0.21), wanted: (doing: string) => doing === 'sit' || doing === 'rise', scale: 0, placed: false },
    ];
    for (const p of props) p.object.visible = false;

    let finish = 0;
    const idleAgain = (f: Explorer, s: number) => s > 0.5 && f.doing === 'idle';
    const steps: Step<Explorer>[] = [
      { start: (f) => (f.SetExpression('happy'), f.Idle()), done: (_f, s) => s > 2.5 },
      { start: (f) => (f.SetExpression('laugh'), f.Wave()), done: idleAgain },
      {
        start: (f) => {
          f.SetExpression('happy');
          // Counted from the nearest half lap, so small misses don't add up.
          finish = (Math.round((2 * f.rotation.y) / lap) / 2 + 1.5) * lap;
          f.Walk();
        },
        done: (f) => f.rotation.y > finish - lap,
      },
      // Stops before the finish by about the angle it covers slowing down.
      { start: (f) => (f.SetExpression('smirk'), f.Run()), done: (f) => f.rotation.y > finish - f.speed / (3.5 * radius) },
      { start: (f) => (f.SetExpression('happy'), f.Idle()), done: (f, s) => s > 1 && f.speed < 0.002 },
      { start: (f) => (f.SetExpression('surprised'), f.Jump()), done: idleAgain },
      { start: (f) => (f.SetExpression('happy'), f.Interact()), done: idleAgain },
      { start: (f) => (f.SetExpression('wink'), f.Sit()), done: (_f, s) => s > 4.5 },
      { start: (f) => (f.SetExpression('happy'), f.Idle()), done: (f, s) => s > 1.5 && f.doing === 'idle' },
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    const act = demoStopper(stop);

    const reach = radius + Math.max(size.x, size.z) / 2;
    const distance = 1.25 * reach + size.y;
    const view = new THREE.Vector3(-0.75, 0.35, 0.55).normalize().multiplyScalar(distance);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.4 * radius);
    return {
      figure,
      props: props.map((p) => p.object),
      camera: [view.x + aim.x, view.y + 0.4 * size.y, view.z + aim.z],
      target: [aim.x, 0.4 * size.y, aim.z],
      update(elapsed, delta) {
        // The props stand in the scene beside it, not in it.
        for (const p of props) if (!p.object.parent && figure.parent) figure.parent.add(p.object);
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        figure.rotation.y += (figure.speed / radius) * delta;
        figure.update(delta);
        // A prop is placed once the figure has stopped to use it, then grows
        // in; it shrinks away once the figure is done with it.
        for (const p of props) {
          const wanted = p.wanted(figure.doing);
          if (wanted && !p.placed && figure.speed < 0.01) {
            p.placed = true;
            const h = figure.rotation.y;
            p.object.position.set(figure.position.x + Math.cos(h) * p.at.x + Math.sin(h) * p.at.z, 0, figure.position.z - Math.sin(h) * p.at.x + Math.cos(h) * p.at.z);
            p.object.rotation.y = h;
          }
          if (!wanted && p.scale < 0.01) p.placed = false;
          p.scale += ((wanted && p.placed ? 1 : 0) - p.scale) * (1 - Math.exp(-8 * delta));
          p.object.scale.setScalar(Math.max(1e-3, p.scale));
          p.object.visible = p.placed && p.scale > 0.01;
        }
      },
      dispose() {
        for (const p of props) p.object.removeFromParent();
      },
      behaviours: [
        ...buttons(figure, ['Idle', 'Walk', 'Run', 'Jump', 'Interact', 'Wave', 'Sit'], stop),
        { name: 'SetExpression', params: [{ name: 'name', value: 'happy', options: [...Explorer.EXPRESSIONS] }], run: act((name) => figure.SetExpression(name)) },
        { name: 'Hold', params: [{ name: 'tool', value: 'nothing', options: [...Explorer.TOOLS] }], run: act((tool) => figure.Hold(tool)) },
      ],
    };
  };
}

// The otter's demo, in a loop. In an environment with water (the lake) it
// floats in the open water (`place: 'water'`, its surface y = 0 of its
// parent, following the waves): it treads water, rears up out of it to look
// about, then swims a lap of a circle `radius` m round, diving under and
// coming back up on the way, the water splashing where it goes in and comes
// out, and slows to a stop where it began, facing as it began. Anywhere else
// it is on the ground and plays the four-legged animals' demo: it sits up
// (its Idle), stands up on its hind legs to look about, walks half a lap and
// bounds a lap of a circle, stopping on the far side of it and back where it
// began the next time round. It moves itself forward; the preview steers it,
// turning it by speed / radius. Its buttons are its sheet's poses; Swim and
// Dive need water, so they are offered only there.
function otter(): (theme: Theme, environment: Environment) => Preview {
  const land = fourLegged(() => new Otter(), 0.9, [{ start: (o) => o.StandUp(), done: idleAgain }], ['StandUp']);
  return (theme: Theme, environment: Environment): Preview => {
    const water = environment.water;
    if (!water) return land(theme, environment);
    const figure = new Otter();
    const radius = 1.1; // m, the lap it swims
    const lap = 2 * Math.PI;
    figure.afloat = true; // in the water from its first frame
    figure.onSplash = (x, z, velocity) => water.splash(x, z, velocity);
    figure.position.set(-radius, 0, 0); // on the circle, facing +z along it
    let finish = 0;
    const steps: Step<Otter>[] = [
      { start: (o) => o.Idle(), done: (_o, s) => s > 3 },
      { start: (o) => o.StandUp(), done: idleAgain },
      { start: (o) => o.Idle(), done: (_o, s) => s > 1.5 },
      {
        start: (o) => {
          // A whole lap on from where it is, counted from the nearest one,
          // so small misses don't add up.
          finish = (Math.round(o.rotation.y / lap) + 1) * lap;
          o.Swim();
        },
        done: (o) => o.rotation.y > finish - 0.75 * lap,
      },
      { start: (o) => o.Dive(), done: (o, s) => s > 0.5 && !o.diving },
      // Stops short of the finish by about the angle it covers slowing down.
      { start: (o) => o.Swim(), done: (o) => o.rotation.y > finish - o.speed / (2.2 * radius) },
      { start: (o) => o.Idle(), done: (o) => o.speed < 0.002 },
    ];
    let step = -1;
    let stepStart = 0;
    let demo = true;
    const stop = () => {
      if (!demo) return;
      demo = false;
      figure.Idle();
    };
    // The camera three-quarters from the side it starts on, far enough back
    // for the whole lap, looking a little right of its middle, clear of the
    // panel; the sun's shadow covers the lap.
    const size = new THREE.Box3().setFromObject(figure).getSize(new THREE.Vector3());
    const reach = radius + Math.max(size.x, size.z) / 2;
    const distance = 1.5 * reach + 1.1 * size.y;
    const view = new THREE.Vector3(-0.75, 0.4, 0.55).normalize().multiplyScalar(distance);
    const aim = new THREE.Vector3(0.55, 0, 0.75).normalize().multiplyScalar(0.4 * radius);
    const bounds = new THREE.Box3(new THREE.Vector3(-reach, -Otter.SINK - Otter.DIVE_DEPTH, -reach), new THREE.Vector3(reach, 0.5, reach));
    return {
      figure,
      place: 'water',
      bounds,
      camera: [view.x + aim.x, view.y + 0.1, view.z + aim.z],
      target: [aim.x, 0.05, aim.z],
      update(elapsed, delta) {
        if (demo && (step < 0 || steps[step].done(figure, elapsed - stepStart))) {
          step = (step + 1) % steps.length;
          stepStart = elapsed;
          steps[step].start(figure);
        }
        // Kept on its circle: turning a little more outside it and less
        // inside, so its laps never creep (they crept 2.4 mm a lap inward,
        // from the frames' steps).
        const off = Math.hypot(figure.position.x, figure.position.z) / radius - 1;
        figure.rotation.y += (figure.speed / radius) * delta * (1 + 3 * off);
        figure.surface = water.heightAt(figure.position.x, figure.position.z);
        figure.update(delta);
      },
      behaviours: buttons(figure, ['Idle', 'Walk', 'Run', 'Swim', 'Dive', 'StandUp'], stop),
    };
  };
}

export const forestAnimalPreviews: Record<string, (theme: Theme, environment: Environment) => Preview> = {
  'forest-wolf': fourLegged(() => new ForestWolf(), 1.6, [{ start: (w) => w.Howl(), done: idleAgain }], ['Howl']),
  'forest-deer': fourLegged(() => new ForestDeer(), 1.8, [{ start: (d) => d.LookBack(), done: idleAgain }], ['LookBack']),
  'forest-fox': fourLegged(
    () => new ForestFox(),
    0.9,
    [
      { start: (f) => f.Sit(), done: (_f, s) => s > 3 },
      { start: (f) => f.LookUp(), done: (f, s) => s > 0.5 && f.doing === 'sit' },
      { start: (f) => f.Idle(), done: (_f, s) => s > 1.5 },
    ],
    ['Sit', 'LookUp'],
  ),
  'wild-boar': fourLegged(
    () => new WildBoar(),
    1.4,
    [
      { start: (b) => b.Eat(), done: (_b, s) => s > 4 },
      { start: (b) => b.Idle(), done: (_b, s) => s > 1 },
    ],
    ['Charge', 'Eat'],
  ),
  rabbit: fourLegged(
    () => new Rabbit(),
    0.45,
    [
      { start: (r) => r.Eat(), done: (_r, s) => s > 3 },
      { start: (r) => r.Idle(), done: (_r, s) => s > 1 },
      { start: (r) => r.Jump(), done: idleAgain },
    ],
    ['Jump', 'Eat'],
  ),
  'forest-squirrel': squirrel(),
  duck: duck(),
  otter: otter(),
  'forest-fish': forestFish(),
  'forest-frog': forestFrog(),
  butterfly: butterfly(),
  'forest-firefly': forestFirefly(),
  explorer: explorer(),
};
