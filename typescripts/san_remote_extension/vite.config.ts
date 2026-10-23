import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// dist/ is the unpacked extension: public/manifest.json, popup.html, the
// debug side panel and the service worker. The manifest names background.js,
// so entry names are fixed, not hashed.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      input: {
        popup: 'popup.html',
        sidepanel: 'sidepanel.html',
        background: 'src/background/index.ts',
      },
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name][extname]',
      },
    },
  },
});
