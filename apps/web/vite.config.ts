import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  base: '/assets/web/',
  plugins: [react()],
  server: {
    proxy: {
      '/iam/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/internal': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/logout': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    globals: true,
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx']
  }
})
