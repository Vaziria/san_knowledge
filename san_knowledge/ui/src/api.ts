// The view server's JSON API (internal/ownview). Go encodes empty slices as
// null, so list fields are optional here and normalized where they are read.

export interface KNode {
  key: string;
  node_type: string;
  title: string;
  summary?: string;
  keyword?: string[];
  loc?: string;
  line_loc?: number;
  level?: number;
  needs_summary?: boolean;
  uri?: string;
  last_fetched?: string;
  author?: string;
  created?: string;
  updated?: string;
  props?: Record<string, unknown>;
}

export interface Link {
  from: string;
  rel: string;
  to: string;
  author?: string;
}

export interface GraphResponse {
  nodes: KNode[] | null;
  links: Link[] | null;
  communities: Record<string, number> | null;
}

export interface NodeResponse {
  node: KNode;
  out: Link[] | null;
  in: Link[] | null;
  text?: string;
}

export interface RelatedContext {
  key: string;
  node_type: string;
  title: string;
  summary?: string;
  depth: number;
}

export interface ContextMatch {
  node: KNode;
  score: number;
  excerpt?: string;
  domains: string[];
  related: RelatedContext[];
}

export interface Explanation {
  query: string;
  matches: ContextMatch[];
}

export interface SyncReport {
  docs_added: string[] | null;
  docs_updated: string[] | null;
  docs_removed: string[] | null;
  docs_moved?: string[] | null;
  sections_added: number;
  sections_updated: number;
  sections_removed: number;
  needs_summary: number;
}

export interface AIReport {
  docs: string[] | null;
  nodes_summarized: number;
  domains_created: string[] | null;
  failed: Record<string, string> | null;
}

export interface SavedWeb {
  loc: string;
  created: boolean;
  changed: boolean;
}

// The board (kb/todo.go): cards read from checkbox items and tasks/ files.
export type TodoStatus = 'backlog' | 'ready' | 'in_progress' | 'review' | 'done';

export interface Subtask {
  line: number;
  depth: number;
  title: string;
  done: boolean;
  raw: string;
}

export interface Todo {
  id: string;
  kind: 'checkbox' | 'task';
  status: TodoStatus;
  title: string;
  labels: string[];
  loc: string;
  line: number;
  key: string; // graph node: the item's section, or its doc
  doc: string;
  section?: string;
  after?: string;
  raw: string;
  subtasks: Subtask[];
  done: number;
}

export interface TodoEdit {
  op: 'move' | 'labels' | 'subtask' | 'check';
  kind: Todo['kind'];
  loc: string;
  line?: number;
  raw?: string;
  to?: TodoStatus;
  labels?: string[];
  text?: string;
  done?: boolean;
}

export async function api<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText);
  return data as T;
}

export const q = encodeURIComponent;

export const errorText = (err: unknown) => (err instanceof Error ? err.message : String(err));

// One page of a column: `total` counts the column's cards that pass the filters.
export interface TodoColumn {
  status: TodoStatus;
  total: number;
  todos: Todo[];
}

export interface TodoQuery {
  status?: TodoStatus; // one column; all of them when left out
  kind?: Todo['kind'];
  labels?: string[];
  q?: string;
  offset?: number;
  limit?: number; // cards per column
}

// The board, filtered and paged per column on the server. `total` is every card, before the filters.
export async function todos(query: TodoQuery = {}): Promise<{ statuses: TodoStatus[]; total: number; columns: TodoColumn[] }> {
  type RawTodo = Omit<Todo, 'labels' | 'subtasks'> & { labels: string[] | null; subtasks: Subtask[] | null };
  type Raw = { statuses: TodoStatus[]; total: number; columns: (Omit<TodoColumn, 'todos'> & { todos: RawTodo[] | null })[] | null };
  const params = new URLSearchParams();
  if (query.status) params.set('status', query.status);
  if (query.kind) params.set('kind', query.kind);
  for (const l of query.labels ?? []) params.append('label', l);
  if (query.q) params.set('q', query.q);
  if (query.offset) params.set('offset', String(query.offset));
  if (query.limit) params.set('limit', String(query.limit));
  const data = await api<Raw>('GET', `/api/todos?${params}`);
  const columns = (data.columns ?? []).map((c) => ({ ...c, todos: (c.todos ?? []).map((t) => ({ ...t, labels: t.labels ?? [], subtasks: t.subtasks ?? [] })) }));
  return { statuses: data.statuses, total: data.total, columns };
}

export async function explain(query: string): Promise<{ explanation: Explanation; markdown: string }> {
  type Raw = { explanation: { query: string; matches: (Omit<ContextMatch, 'domains' | 'related'> & { domains: string[] | null; related: RelatedContext[] | null })[] | null }; markdown: string };
  const data = await api<Raw>('GET', `/api/explain?q=${q(query)}&limit=8&depth=1`);
  const matches = (data.explanation.matches ?? []).map((m) => ({ ...m, domains: m.domains ?? [], related: m.related ?? [] }));
  return { explanation: { query: data.explanation.query, matches }, markdown: data.markdown };
}
