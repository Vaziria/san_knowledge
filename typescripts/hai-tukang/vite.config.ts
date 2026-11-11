import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// The dev server passes the Connect services (/haitukang.v1.*) and the
// health check on to the Go backend; the app's own paths never start so.
// HAI_TUKANG_API is where it runs (`hai-tukang dev run` sets it). The
// port is fixed (5173 belongs to another project): if 5175 is taken, Vite
// stops instead of moving to another port.
export default defineConfig(({ mode }) => {
  const api = loadEnv(mode, '.', 'HAI_TUKANG_').HAI_TUKANG_API || 'http://127.0.0.1:8087'
  return {
    plugins: [react()],
    server: {
      port: 5175,
      strictPort: true,
      proxy: {
        '^/haitukang\\.': api,
        '/healthz': api,
      },
    },
  }
})
