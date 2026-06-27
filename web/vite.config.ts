import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// base './' 让产物用相对路径，便于在任意路径下被 Go embed 托管。
// dev 时把 /api 代理到本地后端（含 WebSocket）。
export default defineConfig({
  plugins: [react()],
  base: './',
  build: { outDir: 'dist', emptyOutDir: true },
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', ws: true, changeOrigin: true },
    },
  },
})
