import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { oscBridge } from './osc-bridge.ts';

export default defineConfig({
  plugins: [react(), oscBridge()],
  server: {
    port: 8087,
    strictPort: true,
  },
});
