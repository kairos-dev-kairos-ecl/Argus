import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/v1':  { target: 'http://localhost:8080', changeOrigin: true },
      '/ws':  { target: 'ws://localhost:8080',   ws: true, changeOrigin: true,
                rewrite: (path: string) => path.replace('/ws/signals', '/v1/signals/stream') },
    }
  },
  build: {
    outDir: 'dist',
    sourcemap: true
  }
})
