import { Badge, ChakraProvider, defaultSystem } from '@chakra-ui/react';
import { type ReactNode, StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import type { Status } from './settings';

// mount renders a page (the popup, the side panel) in Chakra, following the
// system's light or dark mode: Chakra's dark styles key off the .dark class.
export function mount(page: ReactNode) {
  document.documentElement.classList.toggle('dark', matchMedia('(prefers-color-scheme: dark)').matches);
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ChakraProvider value={defaultSystem}>{page}</ChakraProvider>
    </StrictMode>,
  );
}

export function StatusBadge({ status }: { status?: Status }) {
  switch (status?.state) {
    case 'connected':
      return <Badge colorPalette="green">Connected</Badge>;
    case 'connecting':
      return <Badge colorPalette="yellow">Connecting</Badge>;
    default:
      return <Badge colorPalette="red">Not connected</Badge>;
  }
}
