import { Box, Check, ChevronRight, Folder, FolderOpen, Search, X } from 'lucide-react';
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { Input } from './ui/input';

// A folder tree to pick one leaf from, with a search box above it, like a
// document tree. Folders open and close; typing keeps only the leaves whose
// name or folder holds the text, and opens their folders while it does.
// Keys: the arrows move and open or close folders, Enter picks, and typing in
// the tree goes on in the search box. Enter in the search box picks the first
// match. It fills the height its parent gives it, and the tree scrolls under
// the search box.

export interface TreeNode {
  name: string; // shown, and what the search matches
  value?: string; // a leaf's value
  children?: TreeNode[]; // a folder's contents
}

interface Row {
  key: string; // a leaf's value, or a folder's path ("/Tree")
  name: string;
  depth: number;
  parent: string | null; // the key of the folder it is in
  value?: string; // leaves only
  count?: number; // folders only: how many leaves are inside
  open?: boolean; // folders only
}

export function SearchTree({
  label,
  nodes,
  value,
  disabled = false,
  onChange,
}: {
  label: string; // names the tree and its search box, for screen readers
  nodes: TreeNode[];
  value: string;
  disabled?: boolean; // greyed out
  onChange: (value: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(() => new Set(foldersAbove(nodes, value)));
  const [focused, setFocused] = useState<string | null>(null);
  const items = useRef(new Map<string, HTMLElement>());
  const search = useRef<HTMLInputElement>(null);

  // The folders of a newly picked leaf open, and the leaf scrolls into view.
  useEffect(() => {
    setOpen((old) => {
      const shut = foldersAbove(nodes, value).filter((key) => !old.has(key));
      return shut.length > 0 ? new Set([...old, ...shut]) : old;
    });
  }, [nodes, value]);
  useEffect(() => {
    // Braces matter: newer browsers return a promise from scrollIntoView, and
    // an effect may only return a cleanup function.
    items.current.get(value)?.scrollIntoView({ block: 'nearest' });
  }, [value]);

  const text = query.trim().toLowerCase();
  const rows = useMemo(() => rowsOf(nodes, '', 0, null, open, text), [nodes, open, text]);
  // The one row reached with Tab: the last one focused, else the picked leaf.
  const active = rows.find((r) => r.key === focused) ?? rows.find((r) => r.value === value) ?? rows[0];

  const focus = (row: Row | undefined) => {
    if (!row) return;
    setFocused(row.key);
    items.current.get(row.key)?.focus();
  };
  // While searching every folder with a match stays open.
  const toggle = (row: Row, opening = !row.open) => {
    if (text) return;
    setOpen((old) => {
      const next = new Set(old);
      if (opening) next.add(row.key);
      else next.delete(row.key);
      return next;
    });
  };
  const pick = (row: Row) => (row.value !== undefined ? onChange(row.value) : toggle(row));

  const onRowKey = (e: KeyboardEvent, row: Row) => {
    const i = rows.indexOf(row);
    switch (e.key) {
      case 'ArrowDown':
        focus(rows[i + 1]);
        break;
      case 'ArrowUp':
        if (i === 0) search.current?.focus();
        else focus(rows[i - 1]);
        break;
      case 'Home':
        focus(rows[0]);
        break;
      case 'End':
        focus(rows.at(-1));
        break;
      case 'ArrowRight': // open a folder, then go into it
        if (row.open === false) toggle(row, true);
        else if (row.open) focus(rows[i + 1]);
        break;
      case 'ArrowLeft': // close a folder, or go up to the folder it is in
        if (row.open && !text) toggle(row, false);
        else focus(rows.find((r) => r.key === row.parent));
        break;
      case 'Enter':
      case ' ':
        pick(row);
        break;
      default:
        // A letter typed in the tree starts a search.
        if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) search.current?.focus();
        return;
    }
    e.preventDefault();
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-1.5">
      <div className="relative">
        <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        {/* The browser's own clear button is hidden for one in the panel's colours. */}
        <Input
          ref={search}
          type="search"
          className="pr-8 pl-8 [&::-webkit-search-cancel-button]:hidden"
          placeholder={`Search ${label.toLowerCase()}…`}
          aria-label={`Search ${label.toLowerCase()}`}
          value={query}
          disabled={disabled}
          onChange={(e) => setQuery(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'ArrowDown') {
              e.preventDefault();
              focus(rows[0]);
            } else if (e.key === 'Enter') {
              const first = rows.find((r) => r.value !== undefined);
              if (first) onChange(first.value!);
            } else if (e.key === 'Escape' && query) {
              e.preventDefault();
              setQuery('');
            }
          }}
        />
        {query && !disabled && (
          <button
            type="button"
            aria-label="Clear the search"
            className="absolute top-1/2 right-2 -translate-y-1/2 rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
            onClick={() => {
              setQuery('');
              search.current?.focus();
            }}
          >
            <X className="size-4" />
          </button>
        )}
      </div>
      <div role="tree" aria-label={label} className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 py-0.5 scrollbar-thin">
        {rows.map((row) => {
          const leaf = row.value !== undefined;
          const picked = leaf && row.value === value;
          return (
            <div
              key={row.key}
              ref={(el) => {
                if (el) items.current.set(row.key, el);
                else items.current.delete(row.key);
              }}
              role="treeitem"
              aria-level={row.depth + 1}
              aria-expanded={row.open}
              aria-selected={leaf ? picked : undefined}
              aria-disabled={disabled || undefined}
              tabIndex={row === active && !disabled ? 0 : -1}
              style={{ paddingLeft: `${0.25 + row.depth}rem` }}
              className="flex h-7 cursor-pointer items-center gap-1.5 rounded-md pr-2 outline-none select-none hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/50 aria-disabled:pointer-events-none aria-disabled:opacity-50 aria-selected:bg-accent aria-selected:font-medium"
              onClick={() => {
                setFocused(row.key);
                pick(row);
              }}
              onKeyDown={(e) => onRowKey(e, row)}
            >
              {leaf ? (
                <>
                  <span className="size-4 shrink-0" />
                  <Box className="size-4 shrink-0 text-muted-foreground" />
                </>
              ) : (
                <>
                  <ChevronRight
                    className={`size-4 shrink-0 text-muted-foreground transition-transform ${row.open ? 'rotate-90' : ''}`}
                  />
                  {row.open ? (
                    <FolderOpen className="size-4 shrink-0 text-muted-foreground" />
                  ) : (
                    <Folder className="size-4 shrink-0 text-muted-foreground" />
                  )}
                </>
              )}
              <span className="truncate">{row.name}</span>
              {row.count !== undefined && <span className="ml-auto text-xs text-muted-foreground tabular-nums">{row.count}</span>}
              {picked && <Check className="ml-auto size-4 shrink-0" />}
            </div>
          );
        })}
        {rows.length === 0 && <p className="px-2 py-1.5 text-muted-foreground">Nothing matches “{query.trim()}”.</p>}
      </div>
    </div>
  );
}

// The rows shown, in order: open folders show what is inside them. While
// searching (text) only matching leaves and the folders holding them show.
function rowsOf(nodes: TreeNode[], path: string, depth: number, parent: string | null, open: Set<string>, text: string): Row[] {
  const rows: Row[] = [];
  for (const node of nodes) {
    const key = `${path}/${node.name}`;
    if (node.children) {
      const inside = rowsOf(node.children, key, depth + 1, key, open, text);
      if (text && inside.length === 0) continue;
      const isOpen = text !== '' || open.has(key);
      rows.push({ key, name: node.name, depth, parent, count: leaves(node), open: isOpen });
      if (isOpen) rows.push(...inside);
    } else if (!text || key.toLowerCase().includes(text)) {
      rows.push({ key: node.value!, name: node.name, depth, parent, value: node.value });
    }
  }
  return rows;
}

function leaves(node: TreeNode): number {
  return node.children ? node.children.reduce((n, child) => n + leaves(child), 0) : 1;
}

// The keys of the folders holding the leaf with this value, outermost first.
function foldersAbove(nodes: TreeNode[], value: string, path = ''): string[] {
  for (const node of nodes) {
    const key = `${path}/${node.name}`;
    if (node.value === value) return [];
    if (node.children) {
      const inner = foldersAbove(node.children, value, key);
      if (inner.length > 0 || node.children.some((child) => child.value === value)) return [key, ...inner];
    }
  }
  return [];
}
