import { timestampDate } from '@bufbuild/protobuf/wkt';
import { Badge, DataList, Heading, Stack, Text } from '@chakra-ui/react';
import { useEffect, useState } from 'react';
import { tower } from '../api';
import type { GetStatusResponse } from '../gen/san/tower/v1/tower_pb';

// Home is the tower's front page. For now it only shows that the tower is up;
// what it watches comes later.
export function Home() {
  const [status, setStatus] = useState<GetStatusResponse>();
  const [error, setError] = useState<string>();

  useEffect(() => {
    tower.getStatus({}).then(setStatus, (e: unknown) => setError(String(e)));
  }, []);

  return (
    <Stack gap="4" maxW="xl">
      <Heading size="lg">Home</Heading>
      {error ? (
        <Text color="fg.error">The tower does not answer: {error}</Text>
      ) : !status ? (
        <Text color="fg.muted">Asking the tower...</Text>
      ) : (
        <DataList.Root orientation="horizontal">
          <DataList.Item>
            <DataList.ItemLabel>Tower</DataList.ItemLabel>
            <DataList.ItemValue>
              <Badge colorPalette="green">Up</Badge>
            </DataList.ItemValue>
          </DataList.Item>
          <DataList.Item>
            <DataList.ItemLabel>Version</DataList.ItemLabel>
            <DataList.ItemValue>{status.version}</DataList.ItemValue>
          </DataList.Item>
          <DataList.Item>
            <DataList.ItemLabel>Host</DataList.ItemLabel>
            <DataList.ItemValue>{status.hostname}</DataList.ItemValue>
          </DataList.Item>
          {status.startedAt && (
            <DataList.Item>
              <DataList.ItemLabel>Started</DataList.ItemLabel>
              <DataList.ItemValue>{timestampDate(status.startedAt).toLocaleString()}</DataList.ItemValue>
            </DataList.Item>
          )}
        </DataList.Root>
      )}
    </Stack>
  );
}
