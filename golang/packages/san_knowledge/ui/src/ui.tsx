import { Button, Toaster as ChakraToaster, Group, Portal, Text, Toast, createToaster, type TextProps } from '@chakra-ui/react';
import { createContext, useContext, type ReactNode } from 'react';
import type { KNode } from './api';
import { typeInfo, type ColorBy, type Graph } from './model';
import type { Palette } from './theme';

// What most components need; App provides it.
export interface AppContext {
  graph: Graph;
  pal: Palette;
  colorBy: ColorBy;
  colorOf: (n: KNode) => string;
  ai: boolean;
  version: number; // bumps on every reload, so open panels refetch
  select: (key: string | null, center?: boolean) => void;
  reload: () => Promise<void>;
  openDialog: (d: DialogRequest) => void;
  runFetch: (url: string) => void;
  filterKeyword: (keyword: string) => void;
  expanded: Set<string>; // nodes whose docs/sections the graph shows
  toggleExpand: (key: string) => void;
}

export type DialogRequest =
  | { kind: 'edit'; node: KNode }
  | { kind: 'domain' }
  | { kind: 'assign'; key: string }
  | { kind: 'delete'; node: KNode; members: number }
  | { kind: 'fetch' };

export const Ctx = createContext<AppContext | null>(null);

export function useApp(): AppContext {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error('useApp outside App');
  return ctx;
}

export const toaster = createToaster({ placement: 'bottom', max: 3 });

export function toast(message: string, isError = false) {
  toaster.create({ title: message, type: isError ? 'error' : 'info', duration: isError ? 6000 : 2600 });
}

export function Toaster() {
  return (
    <Portal>
      <ChakraToaster toaster={toaster} insetInline={{ mdDown: '4' }}>
        {(t) => (
          <Toast.Root width={{ md: 'sm' }}>
            <Toast.Title>{t.title}</Toast.Title>
            <Toast.CloseTrigger />
          </Toast.Root>
        )}
      </ChakraToaster>
    </Portal>
  );
}

export function Swatch({ type, color, size = 12 }: { type: string; color: string; size?: number }) {
  const shape = typeInfo(type).shape;
  return (
    <svg width={size} height={size} viewBox="0 0 12 12" aria-hidden="true" style={{ flex: 'none' }}>
      {shape === 'round-hexagon' ? <path d="M3 1h6l3 5-3 5H3L0 6z" fill={color} />
        : shape === 'round-rectangle' ? <path d="M2 1h8a2 2 0 012 2v6a2 2 0 01-2 2H2a2 2 0 01-2-2V3a2 2 0 012-2z" fill={color} />
        : <circle cx={6} cy={6} r={5.5} fill={color} />}
    </svg>
  );
}

export const SectionTitle = ({ children }: { children: ReactNode }) => (
  <Text as="h3" fontSize="11px" textTransform="uppercase" letterSpacing="0.06em" color="fg.subtle" fontWeight="600" mt="18px" mb="6px">
    {children}
  </Text>
);

export const Mono = (props: TextProps) => <Text fontFamily="mono" fontSize="12px" color="fg.muted" wordBreak="break-all" {...props} />;

export const Footnote = (props: TextProps) => <Text color="fg.subtle" fontSize="12px" mt="18px" {...props} />;

// Two or more buttons acting as one choice.
export function Segmented<T extends string>({ label, value, options, onChange, size = 'sm' }: {
  label: string;
  value: T;
  options: { value: T; text: string; title?: string }[];
  onChange: (v: T) => void;
  size?: 'xs' | 'sm';
}) {
  return (
    <Group attached role="group" aria-label={label}>
      {options.map((o) => (
        <Button key={o.value} size={size} variant={value === o.value ? 'subtle' : 'outline'} fontWeight={value === o.value ? '600' : '400'}
          aria-pressed={value === o.value} title={o.title} onClick={() => onChange(o.value)}>
          {o.text}
        </Button>
      ))}
    </Group>
  );
}

// "needs summary": dashed like the node border in the graph.
export const PendingBadge = ({ children }: { children: ReactNode }) => (
  <Text as="span" display="inline-flex" alignItems="center" whiteSpace="nowrap" fontSize="11px" fontWeight="600" px="8px" py="1px" rounded="full" borderWidth="1px" borderStyle="dashed" borderColor="fg.subtle" color="fg.muted">
    {children}
  </Text>
);
