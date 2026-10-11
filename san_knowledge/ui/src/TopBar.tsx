import { Box, Button, Flex, Heading, HStack, IconButton, Input, Text } from '@chakra-ui/react';
import type { RefObject } from 'react';
import type { Mode, Palette } from './theme';
import { Segmented, useApp } from './ui';

export type ViewMode = 'graph' | 'table';

// A small graph in the node encoding: a domain hexagon linked to a doc and its
// sections. index.html carries the same drawing (light colours) as the favicon.
function Logo({ pal }: { pal: Palette }) {
  return (
    <svg width="30" height="30" viewBox="0 0 32 32" aria-hidden="true" style={{ flex: 'none' }}>
      <rect x="0.5" y="0.5" width="31" height="31" rx="8" fill={pal.surface2} stroke={pal.border} />
      <g stroke={pal.ink2} strokeWidth="1.5" strokeLinecap="round" opacity="0.6">
        <path d="M12 13 22 8.5M12 13 21.5 22M12 13 8.5 23.5M22 8.5 21.5 22" />
      </g>
      <path d="M17 13 14.5 17.3h-5L7 13l2.5-4.3h5z" fill={pal.s1} />
      <rect x="19" y="5.5" width="6.5" height="6.5" rx="1.8" fill={pal.s2} />
      <circle cx="21.5" cy="22" r="3.2" fill={pal.s3} />
      <circle cx="8.5" cy="23.5" r="2.4" fill={pal.s3} />
    </svg>
  );
}

export function TopBar({ counts, mode, setMode, colorMode, toggleColorMode, query, setQuery, searchRef, onExplain }: {
  counts: string;
  mode: ViewMode;
  setMode: (m: ViewMode) => void;
  colorMode: Mode;
  toggleColorMode: () => void;
  query: string;
  setQuery: (s: string) => void;
  searchRef: RefObject<HTMLInputElement | null>;
  onExplain: () => void;
}) {
  const { pal } = useApp();
  return (
    <Flex as="header" align="center" gap={{ base: '10px', md: '16px' }} wrap={{ base: 'wrap', md: 'nowrap' }} px="16px" py="10px" bg="bg.panel" borderBottomWidth="1px" borderColor="border">
      <HStack gap="10px" minW="0" flex={{ base: '1', md: 'none' }}>
        <Logo pal={pal} />
        <Box minW="0">
          <Heading as="h1" fontSize="15px" fontWeight="650" letterSpacing="-0.01em" lineHeight="1.3">Knowledge Graph</Heading>
          <Text fontSize="12px" color="fg.subtle" fontVariantNumeric="tabular-nums" hideBelow="sm">{counts}</Text>
        </Box>
      </HStack>
      <Flex as="form" role="search" gap="8px" flex="1" maxW={{ md: '620px' }} ml={{ md: 'auto' }} order={{ base: 3, md: 0 }} flexBasis={{ base: '100%', md: 'auto' }}
        onSubmit={(e) => { e.preventDefault(); onExplain(); }}>
        <Input ref={searchRef} type="search" size="sm" flex="1" minW="0" bg="bg" autoComplete="off" aria-label="Explain query"
          placeholder="Ask or search, e.g. rural internet use" value={query} onChange={(e) => setQuery(e.target.value)} />
        <Button type="submit" size="sm" colorPalette="accent">Explain</Button>
      </Flex>
      <HStack gap="8px" order={{ base: 2, md: 0 }}>
        <Segmented label="View mode" value={mode} onChange={setMode} options={[{ value: 'graph', text: 'Graph' }, { value: 'table', text: 'Table' }]} />
        <IconButton size="sm" variant="outline" aria-label="Toggle theme" title={`Switch to ${colorMode === 'dark' ? 'light' : 'dark'} theme`} onClick={toggleColorMode}>◐</IconButton>
      </HStack>
    </Flex>
  );
}
