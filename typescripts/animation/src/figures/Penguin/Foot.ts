import * as THREE from 'three';
import { mesh, type PenguinMaterials, type Side } from './parts';

// One foot: a flat webbed plate with a scalloped front edge, three toes raised
// on top of it, and a claw at each toe tip. Its origin is the heel, on the
// floor, the pivot it lifts and turns around. At rest the toes turn outward.

const FOOT_SCALE = 1.3; // sizes the whole foot: web, toes and claws
const FOOT_TURN = 0.25; // toes turned outward at rest, radians
const WEB_THICKNESS = 0.006;
const WEB_BEVEL = 0.002;
const TOE_RADIUS = 0.0055;
const TOE_BASE = new THREE.Vector2(0, 0.005); // where the toes start, on the plate
// Toe tips in the plate's plane (x across, y forward), left to right.
const TOE_TIPS = [new THREE.Vector2(-0.03, 0.052), new THREE.Vector2(0, 0.066), new THREE.Vector2(0.03, 0.052)];
const CLAW_RADIUS = 0.0035;
const CLAW_LENGTH = 0.013;

// Outline of the webbed foot seen from above: a round heel widening to three
// toe tips, with the web curving back between the toes.
function webShape(): THREE.Shape {
  const [left, middle, right] = TOE_TIPS;
  const s = new THREE.Shape();
  s.moveTo(-0.014, -0.018);
  s.quadraticCurveTo(0, -0.032, 0.014, -0.018);
  s.lineTo(right.x + 0.004, right.y - 0.02);
  s.quadraticCurveTo(right.x + 0.012, right.y, right.x, right.y + 0.004);
  s.quadraticCurveTo(0.016, 0.04, middle.x + 0.008, middle.y - 0.006);
  s.quadraticCurveTo(middle.x, middle.y + 0.006, middle.x - 0.008, middle.y - 0.006);
  s.quadraticCurveTo(-0.016, 0.04, left.x, left.y + 0.004);
  s.quadraticCurveTo(left.x - 0.012, left.y, left.x - 0.004, left.y - 0.02);
  s.lineTo(-0.014, -0.018);
  return s;
}

export class Foot extends THREE.Group {
  constructor(m: PenguinMaterials, side: Side) {
    super();
    this.name = side < 0 ? 'left-foot' : 'right-foot';

    // Scaled and turned in an inner group, so the foot's own transform is
    // zero at rest and free for animation.
    const shape = new THREE.Group();
    shape.scale.setScalar(FOOT_SCALE);
    shape.rotation.y = side * FOOT_TURN;
    this.add(shape);

    // The web is extruded downward from the outline, then lifted onto the floor.
    const webGeo = new THREE.ExtrudeGeometry(webShape(), {
      depth: WEB_THICKNESS,
      bevelEnabled: true,
      bevelThickness: WEB_BEVEL,
      bevelSize: WEB_BEVEL,
      bevelSegments: 3,
      curveSegments: 24,
    });
    webGeo.rotateX(Math.PI / 2); // outline y -> forward (+z)
    webGeo.translate(0, WEB_THICKNESS + WEB_BEVEL, 0);
    shape.add(mesh(webGeo, m.beak));
    const top = WEB_THICKNESS + 2 * WEB_BEVEL;

    const clawGeo = new THREE.ConeGeometry(CLAW_RADIUS, CLAW_LENGTH, 12);
    clawGeo.rotateX(Math.PI / 2); // point forward

    // Each toe is a capsule lying along the line from the base to its tip,
    // half sunk into the web, with a claw sticking out past the tip.
    for (const tip of TOE_TIPS) {
      const length = tip.distanceTo(TOE_BASE);
      const toe = new THREE.Group();
      toe.position.set(TOE_BASE.x, top, TOE_BASE.y);
      toe.rotation.y = Math.atan2(tip.x - TOE_BASE.x, tip.y - TOE_BASE.y);

      const toeGeo = new THREE.CapsuleGeometry(TOE_RADIUS, length - 2 * TOE_RADIUS, 6, 12);
      toeGeo.rotateX(Math.PI / 2);
      const bone = mesh(toeGeo, m.beak);
      bone.position.z = length / 2;
      toe.add(bone);

      const claw = mesh(clawGeo, m.claw);
      claw.position.set(0, 0, length + CLAW_LENGTH / 2 - 0.002);
      toe.add(claw);

      shape.add(toe);
    }
  }
}
