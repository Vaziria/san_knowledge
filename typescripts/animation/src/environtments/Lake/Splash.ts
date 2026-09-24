import * as THREE from 'three';
import { gloss, type Theme } from '../../theme';

// Splashes on the lake's water. Where something breaks the surface (a fish's
// leap), the water throws up drops that fly under gravity and fall back in,
// and two rings of ripples spread from the spot, riding the waves, and fade.
// Leaving the water, it drags most of the drops up after it; coming in, it
// throws them up in a crown around the spot. The faster it breaks the
// surface, the more drops, the higher and wider they fly, and the faster the
// rings spread.
//
// Drops are drawn far bigger than real, 2 to 5 cm across, and the rings a few
// centimeters wide, because the painterly filter averages away anything a
// few pixels wide. Both are the water's colour lightened toward the theme's
// light neutral, like foam; the drops are glossy, like anything wet. Neither
// casts a shadow.

const GRAVITY = 9.81;
const MAX_DROPS = 256; // in the air at once, across all splashes
const MAX_RINGS = 8; // spreading at once

// A splash's size is 1 for something breaking the surface at SIZE_SPEED (a
// fish leaping 30 cm), and grows or shrinks with its speed within these limits.
const SIZE_SPEED = 3; // m/s
const SIZE = { from: 0.3, to: 2.5 };
const FOAM = 0.6; // how far the colour goes from the water's toward the light neutral

// Drops, at size 1. Their speeds grow with the square root of the size.
const DROPS = 50;
const SPREAD = 0.06; // m: they start up to this far from the spot, around the body breaking through
const UP = { from: 0.8, to: 2.2 }; // m/s, in a crown: from 3 to 25 cm high
const OUT = { from: 0.2, to: 0.8 }; // m/s, away from the spot, in a crown
const PLUME = 0.7; // share of the drops dragged up after something leaving the water
const DRAGGED = { from: 0.3, to: 0.85 }; // a dragged drop's speed, as a share of the thing's: it trails behind
const SCATTER = 0.25; // m/s: dragged drops stray up to this much each way
const DROP_RADIUS = { from: 0.01, to: 0.025 };
const STRETCH = 0.2; // a drop is longer along its path by this x its speed (seconds)

// Rings: the first starts at once and the second RING_GAP later, from about
// the size of a fish's body, widening as they spread and fade.
const RING_START = 0.08; // m, radius
const RING_SPEED = 0.3; // m/s at size 1, growing with the square root of the size
const RING_WIDTH = { from: 0.03, to: 0.07 }; // m, over its life
const RING_LIFE = 1.6; // seconds
const RING_GAP = 0.3; // seconds
const RING_OPACITY = 0.8;
const RING_LIFT = 0.004; // m above the water, so the water doesn't cover it
const RING_SEGMENTS = 64;

const between = (range: { from: number; to: number }) => range.from + (range.to - range.from) * Math.random();

interface Drop {
  position: THREE.Vector3;
  velocity: THREE.Vector3;
  radius: number;
}

interface Ring {
  mesh: THREE.Mesh<THREE.BufferGeometry, THREE.MeshStandardMaterial>;
  x: number; // its middle
  z: number;
  age: number; // seconds; below 0 it has not started, from RING_LIFE it is free
  size: number;
}

// Reused by update().
const up = new THREE.Vector3(0, 1, 0);
const along = new THREE.Vector3();
const turn = new THREE.Quaternion();
const scale = new THREE.Vector3();
const matrix = new THREE.Matrix4();

export class Splashes extends THREE.Group {
  private readonly heightAt: (x: number, z: number) => number;
  private readonly drops: Drop[] = [];
  private readonly dropMesh: THREE.InstancedMesh;
  private readonly rings: Ring[] = [];

  // heightAt(x, z) is the water's height now, in this group's coordinates.
  constructor(theme: Theme, heightAt: (x: number, z: number) => number) {
    super();
    this.name = 'splashes';
    this.heightAt = heightAt;
    const foam = new THREE.Color(theme.scene.water).lerp(new THREE.Color(theme.colors.light), FOAM);

    this.dropMesh = new THREE.InstancedMesh(new THREE.IcosahedronGeometry(1, 1), gloss(foam), MAX_DROPS);
    this.dropMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.dropMesh.count = 0;
    this.dropMesh.frustumCulled = false; // its drops can be anywhere on the lake
    this.add(this.dropMesh);

    for (let i = 0; i < MAX_RINGS; i++) {
      const material = new THREE.MeshStandardMaterial({ color: foam, roughness: 0.9, transparent: true, depthWrite: false });
      const mesh = new THREE.Mesh(ringGeometry(), material);
      mesh.visible = false;
      mesh.frustumCulled = false; // its points move every frame
      mesh.renderOrder = 2; // after the water, which is see-through too
      this.add(mesh);
      this.rings.push({ mesh, x: 0, z: 0, age: RING_LIFE, size: 1 });
    }
  }

  // Throws up a splash where something breaks the surface at (x, z), moving
  // at `velocity` meters a second.
  splash(x: number, z: number, velocity: THREE.Vector3): void {
    const size = THREE.MathUtils.clamp(velocity.length() / SIZE_SPEED, SIZE.from, SIZE.to);
    const fling = Math.sqrt(size);
    const y = this.heightAt(x, z);
    const count = Math.min(Math.round(DROPS * size), MAX_DROPS - this.drops.length);
    const dragged = velocity.y > 0 ? Math.round(PLUME * count) : 0;
    const stray = () => SCATTER * fling * (2 * Math.random() - 1);
    for (let i = 0; i < count; i++) {
      const angle = 2 * Math.PI * Math.random();
      const dx = Math.cos(angle);
      const dz = Math.sin(angle);
      const start = SPREAD * Math.sqrt(Math.random()); // spread evenly over the disc
      const drop = { position: new THREE.Vector3(x + dx * start, y, z + dz * start), velocity: new THREE.Vector3(), radius: between(DROP_RADIUS) };
      if (i < dragged) {
        drop.velocity.copy(velocity).multiplyScalar(between(DRAGGED)).add(new THREE.Vector3(stray(), stray(), stray()));
      } else {
        const out = between(OUT) * fling;
        drop.velocity.set(dx * out, between(UP) * fling, dz * out);
      }
      this.drops.push(drop);
    }
    // Each ring takes a free one, or else the one that has spread longest.
    for (const delay of [0, RING_GAP]) {
      const ring = this.rings.reduce((oldest, r) => (r.age > oldest.age ? r : oldest));
      Object.assign(ring, { x, z, age: -delay, size });
      ring.mesh.visible = false;
    }
  }

  // Moves the splashes on by delta seconds; call once per frame.
  update(delta: number): void {
    // Drops fly until they fall back into the water, stretched along their path.
    for (let i = this.drops.length - 1; i >= 0; i--) {
      const drop = this.drops[i];
      drop.velocity.y -= GRAVITY * delta;
      drop.position.addScaledVector(drop.velocity, delta);
      if (drop.velocity.y < 0 && drop.position.y < this.heightAt(drop.position.x, drop.position.z)) {
        this.drops[i] = this.drops[this.drops.length - 1];
        this.drops.pop();
      }
    }
    this.drops.forEach((drop, i) => {
      const speed = drop.velocity.length();
      turn.setFromUnitVectors(up, along.copy(drop.velocity).divideScalar(speed || 1));
      scale.set(drop.radius, drop.radius * (1 + STRETCH * speed), drop.radius);
      this.dropMesh.setMatrixAt(i, matrix.compose(drop.position, turn, scale));
    });
    this.dropMesh.count = this.drops.length;
    this.dropMesh.instanceMatrix.needsUpdate = true;

    for (const ring of this.rings) {
      if (ring.age >= RING_LIFE) continue;
      ring.age += delta;
      ring.mesh.visible = ring.age >= 0 && ring.age < RING_LIFE;
      if (ring.mesh.visible) this.placeRing(ring);
    }
  }

  // Lays a ring on the water around its middle, as wide and as faded as its
  // age makes it.
  private placeRing(ring: Ring): void {
    const t = ring.age / RING_LIFE;
    const radius = RING_START + RING_SPEED * Math.sqrt(ring.size) * ring.age;
    const half = (RING_WIDTH.from + (RING_WIDTH.to - RING_WIDTH.from) * t) / 2;
    const position = ring.mesh.geometry.attributes.position;
    for (let i = 0; i <= RING_SEGMENTS; i++) {
      const angle = (i / RING_SEGMENTS) * 2 * Math.PI;
      for (const [k, r] of [
        [0, radius - half],
        [1, radius + half],
      ]) {
        const x = ring.x + Math.cos(angle) * r;
        const z = ring.z + Math.sin(angle) * r;
        position.setXYZ(2 * i + k, x, this.heightAt(x, z) + RING_LIFT, z);
      }
    }
    position.needsUpdate = true;
    ring.mesh.material.opacity = RING_OPACITY * (1 - t) ** 2;
  }
}

// A flat ring of RING_SEGMENTS quads, facing up: point 2i is on its inner
// edge and 2i + 1 on its outer edge, at the i-th angle from +x toward +z.
// placeRing() sets where the points are.
function ringGeometry(): THREE.BufferGeometry {
  const count = 2 * (RING_SEGMENTS + 1);
  const geo = new THREE.BufferGeometry();
  geo.setAttribute('position', new THREE.Float32BufferAttribute(new Float32Array(3 * count), 3));
  geo.setAttribute('normal', new THREE.Float32BufferAttribute(Array.from({ length: count }, () => [0, 1, 0]).flat(), 3));
  const indices: number[] = [];
  for (let i = 0; i < RING_SEGMENTS; i++) {
    const inner = 2 * i;
    indices.push(inner, inner + 2, inner + 1, inner + 1, inner + 2, inner + 3);
  }
  geo.setIndex(indices);
  return geo;
}
