import { ChakraProvider, defaultSystem } from '@chakra-ui/react';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import { router } from './routes';
import { Toaster } from './ui';

// Follow the system's light or dark mode: Chakra's dark styles key off the
// .dark class.
document.documentElement.classList.toggle('dark', matchMedia('(prefers-color-scheme: dark)').matches);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ChakraProvider value={defaultSystem}>
      <RouterProvider router={router} />
      <Toaster />
    </ChakraProvider>
  </StrictMode>,
);
