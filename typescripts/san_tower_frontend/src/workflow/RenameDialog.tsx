import { Input } from '@chakra-ui/react';
import { useState } from 'react';
import { FormDialog, FormField } from '../ui';

// RenameDialog asks for a workflow's new name. onRename saves it; the dialog
// shows the tower's error (a name in use, too long) and stays open.
export function RenameDialog({
  name,
  onClose,
  onRename,
}: {
  name: string;
  onClose: () => void;
  onRename: (name: string) => Promise<void>;
}) {
  const [next, setNext] = useState(name);
  return (
    <FormDialog
      title={`Rename workflow ${name}`}
      submitText="Rename"
      onClose={onClose}
      onSubmit={async () => {
        if (next.trim() !== name) await onRename(next.trim());
      }}
    >
      <FormField label="Name" help="Unique, at most 100 characters.">
        <Input
          autoFocus
          value={next}
          onChange={(e) => setNext(e.currentTarget.value)}
          onFocus={(e) => e.currentTarget.select()}
        />
      </FormField>
    </FormDialog>
  );
}
