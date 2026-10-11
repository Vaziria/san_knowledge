import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The build is embedded in the knowledge binary (internal/ownview, go:embed).
// File names are fixed rather than hashed: the server sends no-cache, and a
// rebuild overwrites the same files instead of piling up old ones. The output
// dir is not emptied, so dist/.gitkeep (which lets go:embed compile before the
// first build) survives.
export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: '../internal/ownview/dist',
    emptyOutDir: false,
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name].js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name][extname]',
      },
    },
  },
  // `npm run dev` serves the UI with hot reload; the API comes from a running `knowledge view`.
  server: { proxy: { '/api': 'http://127.0.0.1:7474' } },
});
