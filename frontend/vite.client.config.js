import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

export default defineConfig({
  root: resolve(import.meta.dirname, 'client'),
  publicDir: resolve(import.meta.dirname, 'client/public'),
  plugins: [vue()],
  build: {
    outDir: resolve(import.meta.dirname, '../web/client/dist'),
    emptyOutDir: true,
    sourcemap: false
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:1802',
      '/ws': { target: 'ws://127.0.0.1:1802', ws: true }
    }
  }
})
