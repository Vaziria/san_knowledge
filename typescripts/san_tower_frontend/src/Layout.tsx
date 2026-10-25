import { Box, Button, Flex, Heading, IconButton, Image, Stack } from '@chakra-ui/react';
import { PanelLeftCloseIcon, PanelLeftOpenIcon, type LucideIcon } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { NavLink, Outlet } from 'react-router';

export type Page = { path: string; label: string; icon: LucideIcon; element: ReactNode };

// collapsedKey keeps, per browser, whether the sidebar is collapsed.
const collapsedKey = 'san-tower-sidebar-collapsed';

function loadCollapsed() {
  try {
    return localStorage.getItem(collapsedKey) === '1';
  } catch {
    return false;
  }
}

function saveCollapsed(collapsed: boolean) {
  try {
    localStorage.setItem(collapsedKey, collapsed ? '1' : '0');
  } catch {
    // Storage blocked: the sidebar just starts expanded next time.
  }
}

// Layout is the frame every page shares: the menu in a sidebar on the left,
// and the current page beside it. The sidebar collapses to its icons.
export function Layout({ pages }: { pages: Page[] }) {
  const [collapsed, setCollapsed] = useState(loadCollapsed);
  const toggle = () => {
    setCollapsed(!collapsed);
    saveCollapsed(!collapsed);
  };

  return (
    <Flex minH="100vh" bg="bg" color="fg">
      <Stack
        as="aside"
        w={collapsed ? '16' : '56'}
        transition="width 0.15s"
        flexShrink={0}
        h="100vh"
        position="sticky"
        top="0"
        overflowX="hidden"
        overflowY="auto"
        gap="4"
        p="3"
        bg="bg.subtle"
        borderEndWidth="1px"
      >
        <Flex align="center" justify="space-between" direction={collapsed ? 'column' : 'row'} gap="2">
          <Flex align="center" gap="2" px={collapsed ? '0' : '2.5'}>
            <Image src="/logo.svg" alt="San Tower" boxSize="7" />
            {!collapsed && (
              <Heading size="md" whiteSpace="nowrap">
                San Tower
              </Heading>
            )}
          </Flex>
          <IconButton
            aria-label={collapsed ? 'Expand the menu' : 'Collapse the menu'}
            title={collapsed ? 'Expand the menu' : 'Collapse the menu'}
            variant="ghost"
            size="sm"
            color="fg.muted"
            onClick={toggle}
          >
            {collapsed ? <PanelLeftOpenIcon /> : <PanelLeftCloseIcon />}
          </IconButton>
        </Flex>
        <Stack as="nav" gap="1">
          {pages.map((p) => (
            <Button
              key={p.path}
              asChild
              variant="ghost"
              justifyContent={collapsed ? 'center' : 'flex-start'}
              px={collapsed ? '0' : undefined}
              whiteSpace="nowrap"
              color="fg.muted"
              _currentPage={{ bg: 'bg.emphasized', color: 'fg', fontWeight: 'semibold' }}
            >
              <NavLink to={p.path} end={p.path === '/'} title={collapsed ? p.label : undefined} aria-label={p.label}>
                <p.icon />
                {!collapsed && p.label}
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
