import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

// The build is embedded in the san_tower binary
// (golang/apps/san_tower/internal/web, go:embed). File names are fixed rather
// than hashed: the server sends no-cache, and a rebuild overwrites the same
// files instead of piling up old ones. The output dir is not emptied, so
// dist/.gitkeep (which lets go:embed compile before the first build) survives.
export default defineConfig(({ mode }) => ({
  plugins: [react()],
  // Absolute, not './': a route such as /a/b must still load /assets/index.js.
  base: '/',
  build: {
    outDir: '../../golang/apps/san_tower/internal/web/dist',
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
  // The dev server (`san_tower dev`, or `npm run dev`) serves the UI with hot
  // reload and passes the services on to the tower at SAN_TOWER_SERVER, which
  // `san_tower dev` sets to the tower it runs. Every service the tower serves
  // is under /san.: its own (/san.tower.v1.*) and the broker's
  // (/san.message.v1.MessageService). The UI's routes never start with /san.
  server: {
    proxy: { '^/san\\.': loadEnv(mode, '.', 'SAN_TOWER_').SAN_TOWER_SERVER || 'http://127.0.0.1:7477' },
  },
}));
