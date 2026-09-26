import { figureParts } from '../parts';
import { previews } from '../previews';
import type { TreeNode } from './SearchTree';

// The Figures tab's tree: each figure in the folder it lives in under
// src/figures, like a document tree. Every figure has its spec (<Figure>.md)
// next to its code, so the specs' paths give the folders. Vite can only list
// files by importing them; this glob is lazy and never called, so nothing is
// loaded. A figure with parts (parts.ts) opens to list them, to show one on
// its own.
const specs = Object.keys(import.meta.glob('../figures/**/*.md', { query: '?raw', import: 'default' }));

// "oak-tree" -> "OakTree": its class, and the name of its file and spec.
function className(figure: string): string {
  return figure
    .split('-')
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join('');
}

// The spec of a figure not named like it, below src/figures: the fish's
// kinds are the Fish figure's, which takes a kind (Fish/Fish.md), so they
// go in its folder, named like none of them, as the trees go in Tree/.
const SPECS: Record<string, string> = { salmon: 'Fish/Fish', piranha: 'Fish/Fish', clownfish: 'Fish/Fish' };

// The folders a figure is in below src/figures: ["Tree"] for Tree/OakTree.md.
// A folder named like its figure (Boat/Boat.md) holds only that
// figure's parts, so the figure goes beside it. A figure without a spec yet
// goes at the top.
function foldersOf(figure: string): string[] {
  const name = className(figure);
  const spec = specs.find((path) => path.endsWith(`/${Object.hasOwn(SPECS, figure) ? SPECS[figure] : name}.md`));
  const folders = spec ? spec.split('/').slice(2, -1) : []; // "../figures/Tree/OakTree.md"
  if (folders.at(-1) === name) folders.pop();
  return folders;
}

interface Folder {
  folders: Map<string, Folder>;
  figures: string[];
}

// Folders first, by name, then figures in the previews table's order.
function build(): TreeNode[] {
  const root: Folder = { folders: new Map(), figures: [] };
  for (const figure of Object.keys(previews)) {
    let folder = root;
    for (const name of foldersOf(figure)) {
      let inner = folder.folders.get(name);
      if (!inner) folder.folders.set(name, (inner = { folders: new Map(), figures: [] }));
      folder = inner;
    }
    folder.figures.push(figure);
  }
  const nodes = (folder: Folder): TreeNode[] => [
    ...[...folder.folders]
      .sort(([a], [b]) => a.localeCompare(b, 'en', { sensitivity: 'base' }))
      .map(([name, inner]) => ({ name: name[0].toUpperCase() + name.slice(1), children: nodes(inner) })),
    ...folder.figures.map((figure) => ({
      name: figure,
      value: figure,
      children: figureParts[figure]?.map((part) => ({ name: part.name, value: treeValue(figure, part.name) })),
    })),
  ];
  return nodes(root);
}

export const figureTree = build();

// A row's value: the figure, or "figure/part" for a part of it. Figures'
// and parts' names hold no "/".
export function treeValue(figure: string, part: string | null): string {
  return part === null ? figure : `${figure}/${part}`;
}

export function fromTreeValue(value: string): { figure: string; part: string | null } {
  const [figure, part] = value.split('/');
  return { figure, part: part ?? null };
}
