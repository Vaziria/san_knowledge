import { Heading, Link, Stack, Text } from '@chakra-ui/react';
import { Link as RouterLink, useLocation } from 'react-router';

export function NotFound() {
  const { pathname } = useLocation();
  return (
    <Stack gap="2">
      <Heading size="lg">Not found</Heading>
      <Text>There is no page at {pathname}.</Text>
      <Link asChild colorPalette="blue">
        <RouterLink to="/">Back home</RouterLink>
      </Link>
    </Stack>
  );
}
