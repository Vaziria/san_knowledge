import { Combobox, useFilter, useListCollection } from '@chakra-ui/react';
import { useEffect } from 'react';
import type { LiveApproach } from '../gen/san/tower/v1/live_trading_pb';

// ApproachPicker is the New session dialog's approach field: a combobox
// that searches the approaches as you type (the user, 2026-11-15: "make
// select approach can be search"). It matches any part of the name or
// its timeframes, ignoring case.
export function ApproachPicker({
  approaches,
  value,
  onPick,
}: {
  approaches: LiveApproach[];
  value: string;
  onPick: (a: LiveApproach) => void;
}) {
  const { contains } = useFilter({ sensitivity: 'base' });
  const { collection, filter, set } = useListCollection<LiveApproach>({
    initialItems: approaches,
    itemToString: (a) => `${a.name} ${a.timeframes.join('/')}`,
    itemToValue: (a) => a.name,
    filter: contains,
  });
  useEffect(() => set(approaches), [approaches, set]);
  return (
    <Combobox.Root
      size="sm"
      collection={collection}
      value={value ? [value] : []}
      defaultInputValue={value}
      openOnClick
      selectionBehavior="replace"
      onInputValueChange={(e) => filter(e.inputValue)}
      onValueChange={(e) => {
        const a = approaches.find((x) => x.name === e.value[0]);
        if (a) onPick(a);
      }}
      onOpenChange={(e) => e.open && filter('')}
    >
      <Combobox.Control>
        <Combobox.Input aria-label="Approach" placeholder="Search approach…" data-testid="approach-search" />
        <Combobox.IndicatorGroup>
          <Combobox.Trigger />
        </Combobox.IndicatorGroup>
      </Combobox.Control>
      <Combobox.Positioner>
        <Combobox.Content maxH="xs" overflowY="auto">
          <Combobox.Empty>No approach matches</Combobox.Empty>
          {collection.items.map((a) => (
            <Combobox.Item item={a} key={a.name} data-testid="approach-option">
              <Combobox.ItemText>
                {a.name} <span style={{ opacity: 0.6 }}>({a.timeframes.join('/')})</span>
              </Combobox.ItemText>
              <Combobox.ItemIndicator />
            </Combobox.Item>
          ))}
        </Combobox.Content>
      </Combobox.Positioner>
    </Combobox.Root>
  );
}
