import {
  Button,
  Dialog,
  Field,
  Portal,
  Text,
  Toast,
  Toaster as ChakraToaster,
  createToaster,
} from '@chakra-ui/react';
import { useState, type FormEvent, type ReactNode } from 'react';
import { errorText } from './api';

export const toaster = createToaster({ placement: 'bottom-end', max: 3 });

export function toast(message: string, isError = false) {
  toaster.create({ title: message, type: isError ? 'error' : 'success', duration: isError ? 6000 : 2600 });
}

export function Toaster() {
  return (
    <Portal>
      <ChakraToaster toaster={toaster} insetInline={{ mdDown: '4' }}>
        {(t) => (
          <Toast.Root width={{ md: 'sm' }}>
            <Toast.Title>{t.title}</Toast.Title>
            <Toast.CloseTrigger />
          </Toast.Root>
        )}
      </ChakraToaster>
    </Portal>
  );
}

export function FormField({ label, help, children }: { label: string; help?: string; children: ReactNode }) {
  return (
    <Field.Root>
      <Field.Label>{label}</Field.Label>
      {children}
      {help && <Field.HelperText>{help}</Field.HelperText>}
    </Field.Root>
  );
}

// FormDialog is a form in a modal: it shows the error from onSubmit, and
// closes when it succeeds.
export function FormDialog({
  title,
  submitText,
  danger,
  onSubmit,
  onClose,
  children,
}: {
  title: string;
  submitText: string;
  danger?: boolean;
  onSubmit: () => Promise<void>;
  onClose: () => void;
  children: ReactNode;
}) {
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async (ev: FormEvent) => {
    ev.preventDefault();
    setPending(true);
    try {
      await onSubmit();
      onClose();
    } catch (err) {
      setError(errorText(err));
      setPending(false);
    }
  };
  return (
    <Dialog.Root open placement="center" onOpenChange={(e) => !e.open && onClose()}>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content mx="4">
            <form onSubmit={submit}>
              <Dialog.Header>
                <Dialog.Title>{title}</Dialog.Title>
              </Dialog.Header>
              <Dialog.Body display="flex" flexDirection="column" gap="4">
                {error && <Text color="fg.error">{error}</Text>}
                {children}
              </Dialog.Body>
              <Dialog.Footer>
                <Button variant="ghost" onClick={onClose}>
                  Cancel
                </Button>
                <Button type="submit" colorPalette={danger ? 'red' : 'blue'} loading={pending}>
                  {submitText}
                </Button>
              </Dialog.Footer>
            </form>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
