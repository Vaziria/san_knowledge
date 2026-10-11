import { HStack, Table, Text } from '@chakra-ui/react';
import type { KNode } from './api';
import { domainsFor, locOf, titleOf, TYPE_ORDER, typeInfo, typeSwatchColor } from './model';
import { Mono, PendingBadge, Swatch, useApp } from './ui';

export type SortCol = 'title' | 'type' | 'loc' | 'domain' | 'keyword' | 'status';
export interface Sort { col: SortCol; dir: 1 | -1 }

const COLUMNS: { col: SortCol; text: string }[] = [
  { col: 'title', text: 'Title' }, { col: 'type', text: 'Type' }, { col: 'loc', text: 'Location' },
  { col: 'domain', text: 'Domain' }, { col: 'keyword', text: 'Keywords' }, { col: 'status', text: 'Summary' },
];

export function TableView({ visible, selected, sort, setSort }: {
  visible: (n: KNode) => boolean;
  selected: string | null;
  sort: Sort;
  setSort: (s: Sort) => void;
}) {
  const { graph, pal, colorBy, select } = useApp();
  const domainText = (n: KNode) => domainsFor(graph, n).map((k) => titleOf(graph, k)).join(', ');
  const sortValue = (n: KNode): string | number => {
    switch (sort.col) {
      case 'title': return (n.title || n.key).toLowerCase();
      case 'type': return TYPE_ORDER.indexOf(n.node_type);
      case 'loc': return `${n.loc ?? ''}:${String(n.line_loc ?? 0).padStart(6, '0')}`;
      case 'domain': return domainText(n);
      case 'keyword': return (n.keyword ?? []).join(',');
      case 'status': return n.needs_summary ? 0 : n.summary ? 2 : 1;
    }
  };
  const rows = graph.nodes.filter(visible).map((n) => ({ n, v: sortValue(n) }))
    .sort((a, b) => (a.v > b.v ? sort.dir : a.v < b.v ? -sort.dir : 0));

  return (
    <Table.Root size="sm" stickyHeader interactive fontSize="13px">
      <Table.Header>
        <Table.Row bg="bg.panel">
          {COLUMNS.map(({ col, text }) => {
            const active = sort.col === col;
            return (
              <Table.ColumnHeader key={col} cursor="pointer" userSelect="none" color="fg.subtle" fontSize="12px" fontWeight="600" px="12px" py="8px"
                aria-sort={active ? (sort.dir > 0 ? 'ascending' : 'descending') : 'none'}
                onClick={() => setSort({ col, dir: active ? (-sort.dir as 1 | -1) : 1 })}>
                {text}{active ? (sort.dir > 0 ? ' ▲' : ' ▼') : ''}
              </Table.ColumnHeader>
            );
          })}
        </Table.Row>
      </Table.Header>
      <Table.Body>
        {rows.map(({ n }) => (
          <Table.Row key={n.key} cursor="pointer" tabIndex={0} bg={n.key === selected ? 'bg.subtle' : undefined}
            boxShadow={n.key === selected ? 'inset 3px 0 0 var(--chakra-colors-accent-solid)' : undefined}
            onClick={() => select(n.key)} onKeyDown={(ev) => { if (ev.key === 'Enter') select(n.key); }}>
            <Table.Cell px="12px" verticalAlign="top">
              <div>{n.title || n.key}</div>
              {n.node_type === 'domain' && <Mono color="fg.subtle">{n.key}</Mono>}
            </Table.Cell>
            <Table.Cell px="12px" verticalAlign="top">
              <HStack gap="6px" whiteSpace="nowrap"><Swatch type={n.node_type} color={typeSwatchColor(pal, colorBy, n.node_type)} />{typeInfo(n.node_type).label}</HStack>
            </Table.Cell>
            <Table.Cell px="12px" verticalAlign="top"><Mono>{locOf(n)}</Mono></Table.Cell>
            <Table.Cell px="12px" verticalAlign="top" color="fg.subtle">{domainText(n)}</Table.Cell>
            <Table.Cell px="12px" verticalAlign="top" color="fg.subtle">{(n.keyword ?? []).join(', ')}</Table.Cell>
            <Table.Cell px="12px" verticalAlign="top">
              {n.needs_summary ? <PendingBadge>needs summary</PendingBadge> : <Text as="span" color="fg.subtle">{n.summary ? 'done' : '—'}</Text>}
            </Table.Cell>
          </Table.Row>
        ))}
      </Table.Body>
    </Table.Root>
  );
}
