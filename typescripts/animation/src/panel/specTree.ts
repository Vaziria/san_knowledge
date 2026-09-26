import type { TreeNode } from './SearchTree';

// A tab's tree of things to show (the figures, the terrains): each in the
// folder it lives in below `root`, like a document tree. Every one has its
// spec (<Name>.md) next to its code, so the specs' paths give the folders:
// `specs` are their paths as import.meta.glob lists them, which each tree
// takes from its own folder (Vite can only list files by importing them; a
// lazy glob that is never called loads nothing).

// "oak-tree" -> "OakTree": its class, and the name of its file and spec.
function className(name: string): string {
  return name
    .split('-')
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join('');
}

// The folders a thing is in below `root`: ["Tree"] for Tree/OakTree.md. A
// folder named like its thing (Boat/Boat.md) holds only that thing's
// parts, so the thing goes beside it. One without a spec yet goes at the top.
function foldersOf(name: string, specs: string[], root: string): string[] {
  const file = className(name);
  const spec = specs.find((path) => path.startsWith(root) && path.endsWith(`/${file}.md`));
  const folders = spec ? spec.slice(root.length).split('/').slice(0, -1) : [];
  if (folders.at(-1) === file) folders.pop();
  return folders;
}

interface Folder {
  folders: Map<string, Folder>;
  names: string[];
}

// Folders first, by name, then the things in the order of `names` (their
// table's).
export function specTree(names: string[], specs: string[], root: string): TreeNode[] {
  const top: Folder = { folders: new Map(), names: [] };
  for (const name of names) {
    let folder = top;
    for (const inner of foldersOf(name, specs, root)) {
      let next = folder.folders.get(inner);
      if (!next) folder.folders.set(inner, (next = { folders: new Map(), names: [] }));
      folder = next;
    }
    folder.names.push(name);
  }
  const nodes = (folder: Folder): TreeNode[] => [
    ...[...folder.folders]
      .sort(([a], [b]) => a.localeCompare(b, 'en', { sensitivity: 'base' }))
      .map(([name, inner]) => ({ name: name[0].toUpperCase() + name.slice(1), children: nodes(inner) })),
    ...folder.names.map((name) => ({ name, value: name })),
  ];
  return nodes(top);
}
