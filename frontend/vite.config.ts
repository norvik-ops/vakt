import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import sri from 'vite-plugin-subresource-integrity'
import { pwaOptions } from './src/pwaOptions'

export default defineConfig({
  plugins: [
    react(),
    // Subresource Integrity (SRI) for the production bundle. Audit response
    // F[1]: closes the supply-chain path that strict CSP alone cannot block —
    // if dist/assets/index-*.js is swapped at the CDN/proxy layer, browsers
    // will refuse to execute it because the hash in index.html no longer
    // matches. SHA-384 is the default and matches modern browser support.
    sri(),
    // Service Worker: Konfiguration und Begründung in src/pwaOptions.ts.
    VitePWA(pwaOptions),
  ],
  server: {
    port: 5173,
    watch: {
      usePolling: true,
    },
    proxy: {
      '/api': {
        target: process.env.BACKEND_URL ?? 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react':  ['react', 'react-dom', 'react-router-dom'],
          'vendor-query':  ['@tanstack/react-query'],
          'vendor-ui':     [
            '@radix-ui/react-dialog',
            '@radix-ui/react-dropdown-menu',
            '@radix-ui/react-label',
            '@radix-ui/react-progress',
            '@radix-ui/react-select',
            '@radix-ui/react-separator',
            '@radix-ui/react-slot',
            '@radix-ui/react-switch',
            '@radix-ui/react-tabs',
            '@radix-ui/react-toast',
            'class-variance-authority',
            'clsx',
            'cmdk',
            'tailwind-merge',
          ],
          'vendor-charts': ['recharts'],
          'vendor-i18n':   ['i18next', 'react-i18next'],
        },
      },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    // e2e/ und kernwege/ sind Playwright-Suiten (eigener Runner), keine Vitest-Tests.
    exclude: ['**/node_modules/**', '**/e2e/**', '**/kernwege/**'],
  },
})
