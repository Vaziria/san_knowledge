import { HStack, Table } from '@chakra-ui/react';
import { ChevronDownIcon, ChevronsUpDownIcon, ChevronUpIcon } from 'lucide-react';
import { useMemo, useState } from 'react';

// Sortable table columns for the Backtest Analysis pages (the user,
// 2026-11-15: "Add sort column in backtest analysis"). A click on a header
// sorts by it; a second click flips the order. Every sortable header shows
// a faint up/down mark so it is clear it can be clicked.

export type SortValue = number | string;
export type SortState<K extends string> = { key: K; desc: boolean };

export function compareValues(x: SortValue, y: SortValue) {
  if (typeof x === 'string' || typeof y === 'string') return String(x).localeCompare(String(y), undefined, { numeric: true });
  if (Number.isNaN(x)) return Number.isNaN(y) ? 0 : 1;
  if (Number.isNaN(y)) return -1;
  return x - y;
}

// sortRows returns a sorted copy; rows with the same value keep their order.
export function sortRows<T, K extends string>(rows: T[], sort: SortState<K>, value: (row: T, key: K) => SortValue) {
  return rows
    .map((r, i) => [r, i] as const)
    .sort(([a, i], [b, j]) => {
      const c = compareValues(value(a, sort.key), value(b, sort.key));
      return (sort.desc ? -c : c) || i - j;
    })
    .map(([r]) => r);
}

// useSort keeps a table's sort state and gives the sorted rows and the
// header cells. numeric keys start descending (largest first), text and
// time keys ascending.
export function useSort<T, K extends string>(
  rows: T[],
  initial: SortState<NoInfer<K>>,
  value: (row: T, key: K) => SortValue,
  descFirst: (key: K) => boolean = () => false,
) {
  const [sort, setSort] = useState<SortState<K>>(initial);
  const sorted = useMemo(() => sortRows(rows, sort, value), [rows, sort, value]);
  const click = (key: K) => setSort((s) => ({ key, desc: s.key === key ? !s.desc : descFirst(key) }));
  const header = (key: K, label: string, end = false, size = 12) => (
    <SortHeader key={key} label={label} end={end} size={size} active={sort.key === key} desc={sort.desc} onClick={() => click(key)} />
  );
  return { sorted, sort, setSort, header };
}

export function SortHeader({
  label,
  end,
  active,
  desc,
  size = 12,
  onClick,
  w,
}: {
  label: string;
  end?: boolean;
  active: boolean;
  desc: boolean;
  size?: number;
  onClick: () => void;
  w?: string;
}) {
  return (
    <Table.ColumnHeader
      textAlign={end ? 'end' : undefined}
      cursor="pointer"
      userSelect="none"
      whiteSpace="nowrap"
      w={w}
      onClick={onClick}
      title={`Sort by ${label}`}
      aria-sort={active ? (desc ? 'descending' : 'ascending') : 'none'}
      data-testid="sort-header"
    >
      <HStack gap="1" justify={end ? 'flex-end' : undefined}>
        {label}
        {active ? (
          desc ? (
            <ChevronDownIcon size={size} />
          ) : (
            <ChevronUpIcon size={size} />
          )
        ) : (
          <ChevronsUpDownIcon size={size} opacity={0.35} />
        )}
      </HStack>
    </Table.ColumnHeader>
  );
}
