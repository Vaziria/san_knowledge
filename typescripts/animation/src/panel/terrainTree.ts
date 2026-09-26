import { terrains } from '../terrains';
import { specTree } from './specTree';

// The Terrains tab's tree: each terrain in the folder it lives in under
// src/environtments/terrains, found by its spec (specTree.ts), as the
// Figures tab finds the figures. This glob is lazy and never called, so
// nothing is loaded.
const specs = Object.keys(import.meta.glob('../environtments/terrains/**/*.md', { query: '?raw', import: 'default' }));

export const terrainTree = specTree(Object.keys(terrains), specs, '../environtments/terrains/');
