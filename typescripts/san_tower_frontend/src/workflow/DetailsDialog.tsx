import { Field, Input, Switch, Textarea } from '@chakra-ui/react';
import { useState } from 'react';
import { FormDialog, FormField } from '../ui';

// Details are a workflow's settings: its name, its description (the user,
// 2026-10-27: a description for each workflow), and whether its runs go one
// at a time (the user, 2026-10-27: X links sent in a burst failed together).
export type Details = { name: string; description: string; oneAtATime: boolean };

// DetailsDialog edits a workflow's settings. onSave saves them; the dialog
// shows the tower's error (a name in use, too long) and stays open.
export function DetailsDialog({
  details,
  onClose,
  onSave,
}: {
  details: Details;
  onClose: () => void;
  onSave: (next: Details) => Promise<void>;
}) {
  const [name, setName] = useState(details.name);
  const [description, setDescription] = useState(details.description);
  const [oneAtATime, setOneAtATime] = useState(details.oneAtATime);
  return (
    <FormDialog
      title={`Settings of ${details.name}`}
      submitText="Save"
      onClose={onClose}
      onSubmit={async () => {
        const next = { name: name.trim(), description: description.trim(), oneAtATime };
        if (next.name !== details.name || next.description !== details.description || next.oneAtATime !== details.oneAtATime) {
          await onSave(next);
        }
      }}
    >
      <FormField label="Name" help="Unique, at most 100 characters.">
        <Input
          autoFocus
          value={name}
          onChange={(e) => setName(e.currentTarget.value)}
          onFocus={(e) => e.currentTarget.select()}
        />
      </FormField>
      <DescriptionField value={description} onChange={setDescription} />
      <Field.Root>
        <Switch.Root checked={oneAtATime} onCheckedChange={(e) => setOneAtATime(e.checked)}>
          <Switch.HiddenInput />
          <Switch.Control>
            <Switch.Thumb />
          </Switch.Control>
          <Switch.Label>One run at a time</Switch.Label>
        </Switch.Root>
        <Field.HelperText>
          A run waits until the runs before it have ended, in the order they came. For workflows that drive the browser
          or schedule posts, so messages sent in a burst don't run together.
        </Field.HelperText>
      </Field.Root>
    </FormDialog>
  );
}

// DescriptionField is a workflow's description box.
export function DescriptionField({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <FormField label="Description" help="What it is for, at most 2000 characters; the list shows it and the search finds it.">
      <Textarea
        rows={3}
        autoresize
        maxH="60"
        value={value}
        onChange={(e) => onChange(e.currentTarget.value)}
        placeholder="Builds the tower and sends it to Lucky on WhatsApp."
      />
    </FormField>
  );
}
