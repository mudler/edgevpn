import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = process.env.EDGEVPN_URL || 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  // Absolute base. A relative base ('./') breaks every deep link: at
  // /app/nodes the browser resolves "./assets/index.js" to
  // /app/assets/index.js, which the SPA fallback answers with index.html
  // as text/html, so the module is rejected and the page renders blank.
  // The router already hardcodes a /app basename, so there is no subpath
  // portability to preserve here.
  base: '/',
  server: {
    port: 3000,
    proxy: {
      '/api': backendUrl,
      '/debug': backendUrl,
    },
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
})
