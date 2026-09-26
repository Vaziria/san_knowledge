import * as THREE from 'three';
import { CedarTree } from '../../figures/Tree/CedarTree';
import { ChestnutTree } from '../../figures/Tree/ChestnutTree';
import { ElmTree } from '../../figures/Tree/ElmTree';
import { MapleTree } from '../../figures/Tree/MapleTree';
import { OakTree } from '../../figures/Tree/OakTree';
import { seededRandom, type TreeOptions } from '../../figures/Tree/parts';
import { PineTree } from '../../figures/Tree/PineTree';
import { SpruceTree } from '../../figures/Tree/SpruceTree';
import { WillowTree } from '../../figures/Tree/WillowTree';
import type { Theme } from '../../theme';
import { materialsOf, sway, type Wind } from '../wind';
import type { Boulders } from './Boulders';
import { BANK_WIDTH, groundHeight, shoreRadius } from './Ground';

// Trees round the lake: nine of them, of the tree figures' kinds
// (figures/Tree), each grown from its own seed, so no two are alike, standing
// on the land beyond the bank. The kinds are dealt from shuffled decks
// of one of each, one deck after another, so every kind comes up once before
// any comes up again: with nine trees, every kind and one more. Each
// tree gets its own size, from a young tree half the figure's size to an old
// one a quarter bigger, and is a little taller and slimmer or shorter and
// broader than its kind. They are placed by a seeded random generator, so the
// lake is the same on every load.
//
// Two more stand in autumn colours, as the user asked: a maple and an oak,
// the kinds best known for it, their leaves in the theme's autumn colour
// (`autumn` in TreeOptions). They are placed after the nine, the same way, so
// the nine stand where they did.
//
// None stand near `keepClear` (where standing figures go) or on the land
// beyond it, toward +z: that is where the preview cameras stand, looking
// across the water, so the trees are on the far shore and the sides, in the
// view instead of in its way. None stand on or against the boulders.

const COUNT = 9;
const AUTUMN: (new (options: TreeOptions) => THREE.Object3D)[] = [MapleTree, OakTree];
const KINDS: (new (options: TreeOptions) => THREE.Object3D)[] = [
  OakTree,
  CedarTree,
  MapleTree,
  PineTree,
  ChestnutTree,
  SpruceTree,
  ElmTree,
  WillowTree,
];
const SHORE_GAP = 1; // m from the top of the bank to the nearest trunk
const SPREAD = 16; // m beyond that over which trees stand, more of them near the water
const SPACING = 8; // m at least between two trunks
const CLEAR = 9; // m kept free round keepClear
const VIEW = 32; // m: no trees this far out from keepClear on its +z side
const VIEW_FROM = -4; // m along z from keepClear where that side begins
const BOULDER_GAP = 2; // m kept free round a boulder
const SCALE = [0.5, 1.25] as const; // of the figures' size: from young trees to old giants
const STRETCH = 0.12; // each is up to this share taller and slimmer, or shorter and broader
const CROWN_BEND = { sway: 0.35, reach: 15 }; // a crown's top sways this far in a full wind; the wood stands still
const SEED = 7;

export class Trees extends THREE.Group {
  constructor(theme: Theme, keepClear: THREE.Vector3, wind: Wind, boulders: Boulders) {
    super();
    this.name = 'trees';
    const random = seededRandom(SEED);
    // Enough decks of one of each kind to deal COUNT trees, each deck
    // shuffled on its own (Fisher-Yates).
    const deck = Array.from({ length: Math.ceil(COUNT / KINDS.length) }, () => {
      const kinds = [...KINDS];
      for (let i = kinds.length - 1; i > 0; i--) {
        const j = Math.floor(random() * (i + 1));
        [kinds[i], kinds[j]] = [kinds[j], kinds[i]];
      }
      return kinds;
    }).flat();
    const spots: THREE.Vector3[] = [];
    const total = COUNT + AUTUMN.length;
    for (let attempt = 0; spots.length < total && attempt < total * 200; attempt++) {
      const angle = 2 * Math.PI * random();
      const r = shoreRadius(angle) + BANK_WIDTH + SHORE_GAP + SPREAD * random() ** 1.5;
      const spot = new THREE.Vector3(r * Math.cos(angle), 0, r * Math.sin(angle));
      spot.y = groundHeight(spot.x, spot.z); // on the uneven land
      const fromClear = Math.hypot(spot.x - keepClear.x, spot.z - keepClear.z);
      if (fromClear < CLEAR) continue;
      if (spot.z - keepClear.z > VIEW_FROM && fromClear < VIEW) continue;
      if (spots.some((s) => s.distanceTo(spot) < SPACING)) continue;
      if (boulders.near(spot.x, spot.z, BOULDER_GAP)) continue;
      spots.push(spot);

      const autumn = spots.length > COUNT;
      const Kind = autumn ? AUTUMN[spots.length - 1 - COUNT] : deck[spots.length - 1];
      const tree = new Kind({ theme, seed: Math.floor(random() * 2 ** 31), autumn });
      const crown = tree.getObjectByName('crown');
      if (crown) for (const material of materialsOf(crown)) sway(material, wind, CROWN_BEND);
      tree.position.copy(spot);
      tree.rotation.y = 2 * Math.PI * random();
      const size = SCALE[0] + (SCALE[1] - SCALE[0]) * random();
      const stretch = 1 + STRETCH * (2 * random() - 1);
      tree.scale.set(size / Math.sqrt(stretch), size * stretch, size / Math.sqrt(stretch));
      this.add(tree);
    }
  }
}
