import { Box, Button, Flex, HStack, Input, Text } from '@chakra-ui/react';
import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type MouseEvent, type ReactNode } from 'react';
import { api, errorText, todos as fetchTodos, type Todo, type TodoColumn, type TodoEdit, type TodoStatus } from './api';
import { communityColor } from './model';
import { Mono, Segmented, toast, useApp } from './ui';

// The board: every checkbox item and every tasks/ file as a card (kb/todo.go).
// Cards live in the markdown; each change here edits the file, then the server syncs.
// The server filters the cards and pages each column (kb.TodoPage).

const COLUMNS: { status: TodoStatus; text: string; hint: string }[] = [
  { status: 'backlog', text: 'Backlog', hint: 'checkbox [ ] · tasks/backlog/' },
  { status: 'ready', text: 'Ready', hint: 'checkbox [>] · tasks/' },
  { status: 'in_progress', text: 'In progress', hint: 'checkbox [/] · tasks/doing/' },
  { status: 'review', text: 'Review', hint: 'checkbox [?] · tasks/blocked/' },
  { status: 'done', text: 'Done', hint: 'checkbox [x] · tasks/done/' },
];

type Kind = 'all' | Todo['kind'];

// A label keeps its colour everywhere: its name picks one of the palette slots.
function labelSlot(label: string) {
  let h = 0;
  for (const c of label) h = (h * 31 + c.charCodeAt(0)) >>> 0;
  return h % 8;
}

function LabelChip({ label, onClick, onRemove }: { label: string; onClick?: (ev: MouseEvent) => void; onRemove?: () => void }) {
  const { pal } = useApp();
  const color = communityColor(pal, labelSlot(label));
  return (
    <HStack as="span" gap="4px" fontSize="11px" fontWeight="600" px="7px" py="1px" rounded="full" borderWidth="1px" borderColor={color}
      bg={`color-mix(in srgb, ${color} 16%, transparent)`} cursor={onClick ? 'pointer' : undefined} title={onClick ? `Show only #${label}` : undefined} onClick={onClick}>
      #{label}
      {onRemove && (
        <Box as="button" aria-label={`Remove label ${label}`} color="fg.muted" lineHeight="1" onClick={(ev: MouseEvent) => { ev.stopPropagation(); onRemove(); }}>×</Box>
      )}
    </HStack>
  );
}

function Progress({ done, total }: { done: number; total: number }) {
  return (
    <HStack mt="8px" gap="8px" title={`${done} of ${total} subtasks done`}>
      <Box flex="1" h="4px" bg="bg.emphasized" rounded="full" overflow="hidden">
        <Box h="100%" w={`${Math.round((done / total) * 100)}%`} bg="accent.solid" />
      </Box>
      <Text fontSize="11px" color="fg.muted" fontVariantNumeric="tabular-nums">{done}/{total}</Text>
    </HStack>
  );
}

function Card({ card, open, onToggle, onEdit, onFilterLabel }: {
  card: Todo;
  open: boolean;
  onToggle: () => void;
  onEdit: (e: TodoEdit) => void;
  onFilterLabel: (label: string) => void;
}) {
  const { pal, select } = useApp();
  const [subtask, setSubtask] = useState('');
  const [label, setLabel] = useState('');
  const ref = { kind: card.kind, loc: card.loc, line: card.line, raw: card.raw };
  const setLabels = (labels: string[]) => onEdit({ op: 'labels', ...ref, labels });

  return (
    <Box bg="bg.panel" borderWidth="1px" borderColor={open ? 'accent.solid' : 'border'} rounded="md" p="10px" shadow="xs"
      draggable={!open} cursor={open ? 'default' : 'grab'}
      onDragStart={(ev: DragEvent) => { ev.dataTransfer.setData('text/plain', card.id); ev.dataTransfer.effectAllowed = 'move'; }}>
      <Box cursor="pointer" role="button" tabIndex={0} aria-expanded={open} onClick={onToggle} onKeyDown={(ev) => { if (ev.key === 'Enter') onToggle(); }}>
        <Text fontSize="13px" fontWeight="500" lineHeight="1.35">{card.title || '(no text)'}</Text>
        {card.labels.length > 0 && (
          <HStack gap="4px" wrap="wrap" mt="6px">
            {card.labels.map((l) => <LabelChip key={l} label={l} onClick={(ev) => { ev.stopPropagation(); onFilterLabel(l); }} />)}
          </HStack>
        )}
        {card.subtasks.length > 0 && <Progress done={card.done} total={card.subtasks.length} />}
        <Text fontSize="11px" color="fg.subtle" mt="6px">
          {card.kind === 'task' ? 'Task · ' : ''}{card.doc}{card.section ? ` · ${card.section}` : ''}
        </Text>
        <Mono fontSize="11px">{card.kind === 'task' ? card.loc : `${card.loc}:${card.line}`}</Mono>
        {card.after && <Text fontSize="11px" color="fg.subtle">After {card.after}</Text>}
      </Box>

      {open && (
        <Box mt="8px" pt="8px" borderTopWidth="1px" borderColor="border" fontSize="12px">
          {card.subtasks.map((s) => (
            <HStack as="label" key={s.line} gap="6px" pl={`${s.depth * 16}px`} py="2px" alignItems="flex-start" cursor="pointer">
              <input type="checkbox" checked={s.done} style={{ accentColor: pal.s1, marginTop: '3px' }}
                onChange={(ev) => onEdit({ op: 'check', kind: card.kind, loc: card.loc, line: s.line, raw: s.raw, done: ev.target.checked })} />
              <Text as="span" color={s.done ? 'fg.subtle' : 'fg'} textDecoration={s.done ? 'line-through' : undefined}>{s.title || '(no text)'}</Text>
            </HStack>
          ))}
          <form onSubmit={(ev) => { ev.preventDefault(); if (subtask.trim()) { onEdit({ op: 'subtask', ...ref, text: subtask }); setSubtask(''); } }}>
            <Input size="xs" mt="6px" placeholder="Add a subtask, then Enter" aria-label="Add a subtask" value={subtask} onChange={(ev) => setSubtask(ev.target.value)} />
          </form>
          <HStack gap="4px" wrap="wrap" mt="8px">
            {card.labels.map((l) => <LabelChip key={l} label={l} onRemove={() => setLabels(card.labels.filter((x) => x !== l))} />)}
          </HStack>
          <form onSubmit={(ev) => { ev.preventDefault(); if (label.trim()) { setLabels([...card.labels, label]); setLabel(''); } }}>
            <Input size="xs" mt="6px" placeholder="Add a label, then Enter" aria-label="Add a label" value={label} onChange={(ev) => setLabel(ev.target.value)} />
          </form>
          <HStack mt="8px" gap="8px" justify="space-between">
            <HStack as="label" gap="6px" color="fg.muted" whiteSpace="nowrap">
              Move to
              <select value={card.status} aria-label="Move to column" style={{ background: 'var(--chakra-colors-bg-panel)', color: 'var(--chakra-colors-fg)' }}
                onChange={(ev) => onEdit({ op: 'move', ...ref, to: ev.target.value as TodoStatus })}>
                {COLUMNS.map((c) => <option key={c.status} value={c.status}>{c.text}</option>)}
              </select>
            </HStack>
            <Button size="xs" variant="ghost" onClick={() => select(card.key)}>Show in Node Detail</Button>
          </HStack>
        </Box>
      )}
    </Box>
  );
}

// Cards per column per request; scrolling to a column's end loads the next page.
const PAGE = 20;

type Columns = Record<TodoStatus, TodoColumn>;
type Board = { columns: Columns; total: number };

const emptyColumn = (status: TodoStatus): TodoColumn => ({ status, total: 0, todos: [] });
const emptyColumns = () => Object.fromEntries(COLUMNS.map((c) => [c.status, emptyColumn(c.status)])) as Columns;

// Shows a move or a tick at once; the reload after the write confirms it.
function applyEdit(columns: Columns, e: TodoEdit): Columns {
  for (const col of Object.values(columns)) {
    const i = col.todos.findIndex((c) => c.loc === e.loc
      && (e.op === 'check' ? c.subtasks.some((s) => s.line === e.line) : c.kind === 'task' || c.line === e.line));
    if (i < 0) continue;
    const card = col.todos[i];
    if (e.op === 'move' && e.to && e.to !== card.status) {
      const to = columns[e.to];
      return {
        ...columns,
        [col.status]: { ...col, total: col.total - 1, todos: col.todos.filter((_, j) => j !== i) },
        [e.to]: { ...to, total: to.total + 1, todos: [{ ...card, status: e.to }, ...to.todos] },
      };
    }
    if (e.op === 'check') {
      const subtasks = card.subtasks.map((s) => (s.line === e.line ? { ...s, done: !!e.done } : s));
      const todos = col.todos.map((c, j) => (j === i ? { ...c, subtasks, done: subtasks.filter((s) => s.done).length } : c));
      return { ...columns, [col.status]: { ...col, todos } };
    }
    return columns;
  }
  return columns;
}

// A column's cards. Its end has a "Load more" button that presses itself when
// it scrolls into view. A new filter (`start`) scrolls back to the top.
function ColumnCards({ col, start, onMore, children }: { col: TodoColumn; start: unknown; onMore: (status: TodoStatus) => void; children: ReactNode }) {
  const scroll = useRef<HTMLDivElement>(null);
  const end = useRef<HTMLButtonElement>(null);
  const count = col.todos.length;
  const more = count < col.total;
  useEffect(() => { scroll.current?.scrollTo({ top: 0 }); }, [start]);
  // A new observer per page, so a column whose page does not reach its end keeps loading.
  useEffect(() => {
    if (!more || !end.current) return;
    const io = new IntersectionObserver((entries) => { if (entries.some((en) => en.isIntersecting)) onMore(col.status); },
      { root: scroll.current, rootMargin: '0px 0px 300px 0px' });
    io.observe(end.current);
    return () => io.disconnect();
  }, [more, count, col.status, onMore]);
  return (
    <Flex ref={scroll} direction="column" gap="8px" px="8px" pb="8px" overflowY="auto" minH="0">
      {children}
      {more && (
        <Button ref={end} size="xs" variant="ghost" flexShrink={0} color="fg.muted" onClick={() => onMore(col.status)}>
          Load more ({col.total - count} left)
        </Button>
      )}
    </Flex>
  );
}

export function BoardView() {
  const { version, reload } = useApp();
  const [board, setBoard] = useState<Board | null>(null);
  const [search, setSearch] = useState('');
  const [query, setQuery] = useState(''); // the search, once typing pauses
  const [labels, setLabels] = useState<string[]>([]);
  const [kind, setKind] = useState<Kind>('all');
  const [open, setOpen] = useState<string | null>(null);
  const [over, setOver] = useState<TodoStatus | null>(null);
  const [saving, setSaving] = useState(false);

  const boardNow = useRef<Board | null>(null);
  useEffect(() => { boardNow.current = board; }, [board]);
  const gen = useRef(0); // bumped by each full load; a page that arrives after a newer load is dropped
  const loadingMore = useRef(new Set<TodoStatus>());

  useEffect(() => {
    const t = setTimeout(() => setQuery(search.trim()), 250);
    return () => clearTimeout(t);
  }, [search]);
  const filter = useMemo(() => ({ kind: kind === 'all' ? undefined : kind, labels, q: query }), [kind, labels, query]);

  // As many cards as the longest column has loaded, so a reload keeps what was scrolled in.
  const loaded = () => Math.max(PAGE, ...Object.values(boardNow.current?.columns ?? {}).map((c) => c.todos.length));

  // Loads every column from its first card.
  const loadAll = useCallback(async (limit: number) => {
    const g = ++gen.current;
    try {
      const page = await fetchTodos({ ...filter, limit });
      if (g !== gen.current) return;
      const columns = emptyColumns();
      for (const c of page.columns) columns[c.status] = c;
      setBoard({ columns, total: page.total });
    } catch (err) {
      if (g === gen.current) toast(errorText(err), true);
    }
  }, [filter]);

  // Loads the next page of one column.
  const loadMore = useCallback(async (status: TodoStatus) => {
    const col = boardNow.current?.columns[status];
    if (!col || col.todos.length >= col.total || loadingMore.current.has(status)) return;
    loadingMore.current.add(status);
    const g = gen.current;
    try {
      const next = (await fetchTodos({ ...filter, status, offset: col.todos.length, limit: PAGE })).columns[0];
      if (g !== gen.current || !next) return;
      setBoard((b) => {
        if (!b) return b;
        const have = b.columns[status];
        const ids = new Set(have.todos.map((t) => t.id));
        const todos = [...have.todos, ...next.todos.filter((t) => !ids.has(t.id))];
        return { ...b, columns: { ...b.columns, [status]: { ...have, total: next.total, todos } } };
      });
    } catch (err) {
      toast(errorText(err), true);
    } finally {
      loadingMore.current.delete(status);
    }
  }, [filter]);

  // A new filter starts every column at its first page; a reload (version) keeps what was scrolled in.
  const lastFilter = useRef(filter);
  useEffect(() => {
    const keep = lastFilter.current === filter;
    lastFilter.current = filter;
    loadAll(keep ? loaded() : PAGE);
  }, [filter, version, loadAll]);

  // Writes one change into the card's file. The server syncs, and the graph
  // reload that follows reloads the board too.
  const edit = useCallback(async (e: TodoEdit) => {
    setBoard((b) => b && { ...b, columns: applyEdit(b.columns, e) });
    setSaving(true);
    try {
      await api('POST', '/api/todos', e);
    } catch (err) {
      toast(errorText(err), true);
    } finally {
      setSaving(false);
      reload().catch(() => loadAll(loaded()));
    }
  }, [loadAll, reload]);

  const drop = (status: TodoStatus) => (ev: DragEvent) => {
    ev.preventDefault();
    setOver(null);
    const id = ev.dataTransfer.getData('text/plain');
    const card = board && Object.values(board.columns).flatMap((c) => c.todos).find((c) => c.id === id);
    if (!card || card.status === status) return;
    edit({ op: 'move', kind: card.kind, loc: card.loc, line: card.line, raw: card.raw, to: status });
  };

  const shown = board ? Object.values(board.columns).reduce((n, c) => n + c.total, 0) : 0;

  return (
    <Flex direction="column" h="100%" minH="0">
      <HStack px="16px" py="10px" gap="10px" wrap="wrap" borderBottomWidth="1px" borderColor="border" bg="bg.panel">
        <Input type="search" size="sm" maxW="340px" bg="bg" placeholder="Search tasks, #labels, docs" aria-label="Search tasks"
          value={search} onChange={(ev) => setSearch(ev.target.value)} />
        <Segmented label="Card kind" size="xs" value={kind} onChange={setKind}
          options={[{ value: 'all', text: 'All' }, { value: 'task', text: 'Task files' }, { value: 'checkbox', text: 'Checklists' }]} />
        {labels.map((l) => <LabelChip key={l} label={l} onRemove={() => setLabels((ls) => ls.filter((x) => x !== l))} />)}
        <Text fontSize="12px" color="fg.subtle" ml="auto" fontVariantNumeric="tabular-nums">
          {board === null ? 'Loading…' : `${shown} of ${board.total} cards · drag to move`}{saving ? ' · saving…' : ''}
        </Text>
      </HStack>
      <Flex flex="1" minH="0" gap="12px" p="12px" overflowX="auto">
        {COLUMNS.map((info) => {
          const col = board?.columns[info.status] ?? emptyColumn(info.status);
          const target = over === info.status;
          return (
            <Flex key={info.status} as="section" aria-label={info.text} direction="column" flex="1 0 210px" maxW="380px" minH="0" rounded="md" borderWidth="1px"
              borderColor={target ? 'accent.solid' : 'border'} bg={target ? 'bg.emphasized' : 'bg.subtle'}
              onDragOver={(ev: DragEvent) => { ev.preventDefault(); ev.dataTransfer.dropEffect = 'move'; setOver(info.status); }}
              onDragLeave={(ev: DragEvent) => { if (!ev.currentTarget.contains(ev.relatedTarget as Node | null)) setOver(null); }}
              onDrop={drop(info.status)}>
              <HStack px="12px" py="8px" justify="space-between" title={info.hint}>
                <Text fontSize="13px" fontWeight="600">{info.text}</Text>
                <Text fontSize="12px" color="fg.subtle" fontVariantNumeric="tabular-nums">{col.total}</Text>
              </HStack>
              <ColumnCards col={col} start={filter} onMore={loadMore}>
                {col.todos.map((c) => (
                  <Card key={c.id} card={c} open={open === c.id} onToggle={() => setOpen((o) => (o === c.id ? null : c.id))} onEdit={edit}
                    onFilterLabel={(l) => setLabels((ls) => (ls.includes(l) ? ls : [...ls, l]))} />
                ))}
              </ColumnCards>
            </Flex>
          );
        })}
      </Flex>
    </Flex>
  );
}
