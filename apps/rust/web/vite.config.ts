import { defineConfig } from 'vite'

// Vite dev server proxies /api to the Rust backend so the SPA can be
// developed with hot reload while talking to a real API. In
// production the Rust server serves the built SPA from web/dist via
// ServeDir, and /api hits the same process.
export default defineConfig({
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.VITE_API_TARGET ?? 'http://localhost:9003',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
