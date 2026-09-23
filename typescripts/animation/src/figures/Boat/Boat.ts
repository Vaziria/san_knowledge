import * as THREE from 'three';
import { keepDry } from '../../environtments/water';
import { defaultTheme, type Theme } from '../../theme';
import { Hull, hullZ, sheerHeight } from './Hull';
import { createMaterials } from './parts';
import { Rudder } from './Rudder';

// A small dinghy, 2.4 m long and 1.1 m wide, put together from its parts: the
// hull and the rudder (with its tiller). It has no sail and no mast: the user
// removed both (first the sail, which they called the boat's flag, then the
// pole it hung from). It is built of wood: planks, bench, rudder and tiller
// in the theme's wood colour with a wood-grain texture, and a gunwale rim
// painted in its trim colour. Every colour comes from the theme. Units are
// meters, the bow points +z, and the origin is on the floor under the middle
// of the keel.
//
// The spec, Boat.md, has no animation behaviour yet. The rudder is pivoted
// ready for it: it turns on the transom.

const RUDDER_GAP = 0.03; // the rudder's pivot sits just aft of the transom

export interface BoatOptions {
  theme?: Theme;
}

export class Boat extends THREE.Group {
  // How deep the keel sits below the waterline when afloat with no one aboard.
  static readonly DRAFT = 0.1;

  // Each part is its own group with its origin at its pivot, ready to animate.
  readonly hull: Hull;
  readonly rudder: Rudder;

  constructor(options: BoatOptions = {}) {
    super();
    this.name = 'boat';
    const m = createMaterials(options.theme ?? defaultTheme);

    this.hull = new Hull(m);

    this.rudder = new Rudder(m);
    this.rudder.position.set(0, sheerHeight(0), hullZ(0) - RUDDER_GAP);

    this.add(this.hull, this.rudder);

    // Afloat, the water's surface runs through the hull; the inside of the
    // hull must not look flooded.
    keepDry(this, [m.inside]);
  }
}
