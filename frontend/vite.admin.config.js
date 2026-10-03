import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

export default defineConfig({
  root: resolve(import.meta.dirname, 'admin'),
  plugins: [vue()],
  build: {
    outDir: resolve(import.meta.dirname, '../admin/dist'),
    emptyOutDir: true,
    sourcemap: false
  },
  server: {
    port: 5174,
    proxy: { '/api': 'http://127.0.0.1:1801' }
  }
})
