import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [svelte()],
  build: {
    // Embedded into the Go gateway binary (see gateway/web/web.go).
    // public/.gitkeep is copied over so the tracked placeholder survives emptyOutDir.
    outDir: '../gateway/web/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/ws': { target: 'http://localhost:8080', ws: true },
      '/healthz': { target: 'http://localhost:8080' },
    },
  },
})
