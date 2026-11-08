import { Box, Button, Dialog, HStack, IconButton, Input, NativeSelect, Portal, Stack, Text } from '@chakra-ui/react';
import { ArrowDownIcon, ArrowUpIcon, PlusIcon, Trash2Icon } from 'lucide-react';
import { useState } from 'react';
import { rowHolds, type Column, type Row, type Setting } from './catalog';

// An edited row: its values, and the row it was when the editor opened
// (-1 for a new one), so the step's outputs made from it can follow a
// rename (StepNode).
export type EditedRow = { values: Row; from: number; key: number };

// TableEditor edits a table setting (Setting.columns: a Telegram Form's
// Questions, a Telegram Command's Commands) row by row: each row a card of
// its columns, a select where the column has options, the columns whose when
// does not hold for the row left out; rows are added, moved and removed.
// Save hands back the rows, empty ones left out.
export function TableEditor({
  setting,
  rows: start,
  onSave,
  onClose,
}: {
  setting: Setting;
  rows: Row[];
  onSave: (rows: EditedRow[]) => void;
  onClose: () => void;
}) {
  const [rows, setRows] = useState<EditedRow[]>(() => start.map((values, i) => ({ values, from: i, key: i })));
  const [next, setNext] = useState(start.length);
  const columns = setting.columns;
  const set = (key: number, name: string, value: string) =>
    setRows((rs) => rs.map((r) => (r.key === key ? { ...r, values: { ...r.values, [name]: value } } : r)));
  const move = (i: number, by: number) =>
    setRows((rs) => {
      const out = [...rs];
      const [r] = out.splice(i, 1);
      out.splice(i + by, 0, r);
      return out;
    });
  const add = () => {
    setRows((rs) => [...rs, { values: {}, from: -1, key: next }]);
    setNext((n) => n + 1);
  };
  const save = () => {
    // A select left as it showed is its first option, written out.
    const filled = rows.map((r) => ({
      ...r,
      values: Object.fromEntries(
        columns
          .map((c): [string, string] => [c.name, (r.values[c.name] ?? '').trim() || (c.options[0] ?? '')])
          .filter(([, v]) => v !== ''),
      ),
    }));
    // A row with only its selects' first options is empty.
    onSave(filled.filter((r) => columns.some((c) => c.options.length === 0 && r.values[c.name])));
    onClose();
  };
  return (
    <Dialog.Root open size="xl" placement="center" scrollBehavior="inside" onOpenChange={(e) => !e.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content mx="4">
            <Dialog.Header>
              <Dialog.Title>{setting.name}</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap="3">
                {rows.length === 0 && <Text color="fg.muted">No rows yet: add one.</Text>}
                {rows.map((r, i) => (
                  <Box key={r.key} borderWidth="1px" rounded="md" p="3">
                    <HStack mb="2">
                      <Text fontWeight="semibold" flex="1" truncate>
                        {i + 1}. {r.values[columns[0]?.name ?? ''] || 'new'}
                      </Text>
                      <IconButton aria-label="Up" title="Up" size="xs" variant="ghost" disabled={i === 0} onClick={() => move(i, -1)}>
                        <ArrowUpIcon />
                      </IconButton>
                      <IconButton
                        aria-label="Down"
                        title="Down"
                        size="xs"
                        variant="ghost"
                        disabled={i === rows.length - 1}
                        onClick={() => move(i, 1)}
                      >
                        <ArrowDownIcon />
                      </IconButton>
                      <IconButton
                        aria-label="Remove"
                        title="Remove"
                        size="xs"
                        variant="ghost"
                        colorPalette="red"
                        onClick={() => setRows((rs) => rs.filter((x) => x.key !== r.key))}
                      >
                        <Trash2Icon />
                      </IconButton>
                    </HStack>
                    <Box display="grid" gap="2" gridTemplateColumns="repeat(auto-fill, minmax(10rem, 1fr))">
                      {columns
                        .filter((c) => rowHolds(c.when, r.values, columns))
                        .map((c, ci) => (
                          <Cell
                            key={c.name}
                            column={c}
                            wide={ci === 1 && c.options.length === 0}
                            value={r.values[c.name] ?? ''}
                            onChange={(v) => set(r.key, c.name, v)}
                          />
                        ))}
                    </Box>
                  </Box>
                ))}
                <Button size="sm" variant="outline" alignSelf="flex-start" onClick={add}>
                  <PlusIcon />
                  Add
                </Button>
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button colorPalette="blue" onClick={save}>
                Save
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}

// Cell is one column of a row: its name, its box or select, and its help
// as the box's tooltip and placeholder.
function Cell({
  column,
  wide,
  value,
  onChange,
}: {
  column: Column;
  wide: boolean;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <Stack gap="0.5" gridColumn={wide ? 'span 2' : undefined} title={column.help || undefined}>
      <Text textStyle="xs" color="fg.muted">
        {column.name}
      </Text>
      {column.options.length > 0 ? (
        <NativeSelect.Root size="sm">
          <NativeSelect.Field value={value || column.options[0]} onChange={(e) => onChange(e.target.value)}>
            {(column.options.includes(value) || !value ? column.options : [value, ...column.options]).map((o) => (
              <option key={o} value={o}>
                {o}
              </option>
            ))}
          </NativeSelect.Field>
          <NativeSelect.Indicator />
        </NativeSelect.Root>
      ) : (
        <Input size="sm" placeholder={column.placeholder} value={value} onChange={(e) => onChange(e.target.value)} />
      )}
      {column.help && (
        <Text textStyle="2xs" color="fg.subtle" lineClamp={2}>
          {column.help}
        </Text>
      )}
    </Stack>
  );
}
