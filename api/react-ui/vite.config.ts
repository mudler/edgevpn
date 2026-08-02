import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendUrl = process.env.EDGEVPN_URL || 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  // Relative base so generated URLs resolve against the referencing file
  // rather than the origin root. Keeps the door open for reverse-proxy
  // subpath support later without a routing rewrite.
  base: './',
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
