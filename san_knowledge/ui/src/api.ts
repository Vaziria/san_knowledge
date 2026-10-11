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

export async function explain(query: string): Promise<{ explanation: Explanation; markdown: string }> {
  type Raw = { explanation: { query: string; matches: (Omit<ContextMatch, 'domains' | 'related'> & { domains: string[] | null; related: RelatedContext[] | null })[] | null }; markdown: string };
  const data = await api<Raw>('GET', `/api/explain?q=${q(query)}&limit=8&depth=1`);
  const matches = (data.explanation.matches ?? []).map((m) => ({ ...m, domains: m.domains ?? [], related: m.related ?? [] }));
  return { explanation: { query: data.explanation.query, matches }, markdown: data.markdown };
}
