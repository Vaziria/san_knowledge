import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { oscBridge } from './osc-bridge.ts';
import { streamBridge } from './stream-bridge.ts';

export default defineConfig({
  plugins: [react(), tailwindcss(), oscBridge(), streamBridge()],
  // `@/` is src/, as set in tsconfig.json; the panel's shadcn/ui components use it.
  resolve: { tsconfigPaths: true },
  server: {
    port: 8087,
    strictPort: true,
  },
});
