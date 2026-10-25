import { Button, Dialog, List, Portal, Stack, Text } from '@chakra-ui/react';
import { errorText, workflows } from '../api';
import type { WorkflowSummary } from '../gen/san/tower/v1/workflow_pb';

// Export and import of workflows (the user's ask, 2026-10-26): the tower
// writes and reads the file (ExportWorkflows, ImportWorkflows); the page
// only saves it as a download, or reads the files picked.

// download saves text as a file in the browser's downloads.
function download(fileName: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/json' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// exportWorkflows downloads a file of the workflows ids (every one when
// empty), and gives its name.
export async function exportWorkflows(ids: number[]) {
  const r = await workflows.exportWorkflows({ ids });
  download(r.fileName, r.json);
  return r.fileName;
}

// Imported is what importing some files made: the workflows, what did not
// carry over as it was, and the files the tower refused.
export type Imported = { workflows: WorkflowSummary[]; notes: string[]; errors: string[] };

// importFiles imports each file in turn; one refused does not stop the
// others.
export async function importFiles(files: File[]): Promise<Imported> {
  const out: Imported = { workflows: [], notes: [], errors: [] };
  for (const f of files) {
    try {
      const r = await workflows.importWorkflows({ json: await f.text() });
      out.workflows.push(...r.workflows);
      out.notes.push(...r.notes);
    } catch (err) {
      out.errors.push(`${f.name}: ${errorText(err)}`);
    }
  }
  return out;
}

// ImportResult says what an import made, each workflow a link to its
// editor, and what to check: the notes, and the files refused.
export function ImportResult({
  result,
  onOpen,
  onClose,
}: {
  result: Imported;
  onOpen: (id: number) => void;
  onClose: () => void;
}) {
  const n = result.workflows.length;
  return (
    <Dialog.Root open placement="center" onOpenChange={(e) => !e.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content mx="4">
            <Dialog.Header>
              <Dialog.Title>
                {n === 0 ? 'Nothing imported' : `Imported ${n} workflow${n === 1 ? '' : 's'}`}
              </Dialog.Title>
            </Dialog.Header>
            <Dialog.Body display="flex" flexDirection="column" gap="4">
              {result.errors.map((e) => (
                <Text key={e} color="fg.error">
                  {e}
                </Text>
              ))}
              {n > 0 && (
                <Stack gap="1">
                  {result.workflows.map((w) => (
                    <Button
                      key={w.id}
                      variant="plain"
                      colorPalette="blue"
                      size="sm"
                      px="0"
                      h="auto"
                      justifyContent="flex-start"
                      onClick={() => onOpen(w.id)}
                    >
                      {w.name}
                      <Text as="span" color="fg.muted" fontWeight="normal">
                        {w.stepCount} steps
                      </Text>
                    </Button>
                  ))}
                  <Text textStyle="sm" color="fg.muted">
                    They are switched off: check them, then switch them on.
                  </Text>
                </Stack>
              )}
              {result.notes.length > 0 && (
                <Stack gap="1">
                  <Text fontWeight="semibold">Check these</Text>
                  <List.Root textStyle="sm" gap="1" ps="4">
                    {result.notes.map((note) => (
                      <List.Item key={note}>{note}</List.Item>
                    ))}
                  </List.Root>
                </Stack>
              )}
            </Dialog.Body>
            <Dialog.Footer>
              <Button onClick={onClose}>Close</Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
