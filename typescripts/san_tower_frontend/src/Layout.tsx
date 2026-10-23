import { Box, Button, Flex, Heading, Stack } from '@chakra-ui/react';
import type { ReactNode } from 'react';
import { NavLink, Outlet } from 'react-router';

export type Page = { path: string; label: string; element: ReactNode };

// Layout is the frame every page shares: the menu in a sidebar on the left,
// and the current page beside it.
export function Layout({ pages }: { pages: Page[] }) {
  return (
    <Flex minH="100vh" bg="bg" color="fg">
      <Stack
        as="aside"
        w="56"
        flexShrink={0}
        h="100vh"
        position="sticky"
        top="0"
        overflowY="auto"
        gap="4"
        p="3"
        bg="bg.subtle"
        borderEndWidth="1px"
      >
        <Heading size="md" px="3" pt="2">
          San Tower
        </Heading>
        <Stack as="nav" gap="1">
          {pages.map((p) => (
            <Button
              key={p.path}
              asChild
              variant="ghost"
              justifyContent="flex-start"
              color="fg.muted"
              _currentPage={{ bg: 'bg.emphasized', color: 'fg', fontWeight: 'semibold' }}
            >
              <NavLink to={p.path} end>
                {p.label}
              </NavLink>
            </Button>
          ))}
        </Stack>
      </Stack>
      <Box as="main" flex="1" minW="0" p="6">
        <Outlet />
      </Box>
    </Flex>
  );
}
