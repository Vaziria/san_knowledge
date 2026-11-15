import { Badge, Box, Button, Dialog, HStack, Input, Portal, Stack, Text } from '@chakra-ui/react';
import { ArrowUpIcon, FileChartColumnIcon, FolderIcon, HardDriveIcon, HouseIcon } from 'lucide-react';
import { useEffect, useState } from 'react';
import { backtestAnalysis, errorText } from '../api';
import type { BacktestListFoldersResponse } from '../gen/san/tower/v1/backtest_analysis_pb';

// FolderPicker is the tower's own folder browser (ListFolders): a page
// cannot read a real path from the browser's file dialog. It walks this
// computer's folders, marks the ones that are reports, and Open picks the
// folder shown.
export function FolderPicker({ start, onPick, onClose }: { start: string; onPick: (path: string) => void; onClose: () => void }) {
  const [at, setAt] = useState(start);
  const [list, setList] = useState<BacktestListFoldersResponse>();
  const [error, setError] = useState('');
  const [typed, setTyped] = useState(start);

  useEffect(() => {
    let live = true;
    setError('');
    backtestAnalysis
      .listFolders({ path: at })
      .then((r) => {
        if (!live) return;
        setList(r);
        setTyped(r.path);
      })
      .catch((err) => live && setError(errorText(err)));
    return () => {
      live = false;
    };
  }, [at]);

  const top = list && !list.path;
  return (
    <Dialog.Root open placement="center" size="lg" onOpenChange={(e) => !e.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content mx="4" data-testid="folder-picker">
            <Dialog.Header>
              <Dialog.Title>Choose a folder</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body display="flex" flexDirection="column" gap="3">
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  setAt(typed);
                }}
              >
                <HStack>
                  <Input size="sm" value={typed} onChange={(e) => setTyped(e.currentTarget.value)} aria-label="Folder path" />
                  <Button size="sm" variant="outline" type="submit">
                    Go
                  </Button>
                </HStack>
              </form>
              <HStack gap="2">
                <Button size="xs" variant="outline" disabled={!list || top} onClick={() => setAt(list?.parent ?? '')} aria-label="Up">
                  <ArrowUpIcon /> Up
                </Button>
                <Button size="xs" variant="outline" onClick={() => setAt('')}>
                  <HardDriveIcon /> Drives
                </Button>
                {list?.home && (
                  <Button size="xs" variant="outline" onClick={() => setAt(list.home)}>
                    <HouseIcon /> Home
                  </Button>
                )}
                {list?.report && <Badge colorPalette="green">This folder is a report</Badge>}
              </HStack>
              {error && <Text color="fg.error">{error}</Text>}
              <Box borderWidth="1px" rounded="md" h="sm" overflowY="auto">
                {!list ? (
                  <Text p="3" color="fg.muted">
                    Loading...
                  </Text>
                ) : list.folders.length === 0 ? (
                  <Text p="3" color="fg.muted">
                    No folders here.
                  </Text>
                ) : (
                  <Stack gap="0">
                    {list.folders.map((f) => (
                      <HStack
                        key={f.path}
                        as="button"
                        px="3"
                        py="1.5"
                        textAlign="start"
                        _hover={{ bg: 'bg.muted' }}
                        onClick={() => setAt(f.path)}
                        data-testid="picker-folder"
                      >
                        {top ? <HardDriveIcon size={16} /> : f.report ? <FileChartColumnIcon size={16} /> : <FolderIcon size={16} />}
                        <Text textStyle="sm" truncate flex="1">
                          {f.name}
                        </Text>
                        {f.report && (
                          <Badge size="sm" colorPalette="green">
                            report
                          </Badge>
                        )}
                      </HStack>
                    ))}
                  </Stack>
                )}
              </Box>
            </Dialog.Body>
            <Dialog.Footer>
              <Button variant="ghost" onClick={onClose}>
                Cancel
              </Button>
              <Button colorPalette="blue" disabled={!list?.path} onClick={() => list?.path && onPick(list.path)}>
                Open this folder
              </Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
