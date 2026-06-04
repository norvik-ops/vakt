import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import sri from 'vite-plugin-subresource-integrity'

export default defineConfig({
  plugins: [
    react(),
    // Subresource Integrity (SRI) for the production bundle. Audit response
    // F[1]: closes the supply-chain path that strict CSP alone cannot block —
    // if dist/assets/index-*.js is swapped at the CDN/proxy layer, browsers
    // will refuse to execute it because the hash in index.html no longer
    // matches. SHA-384 is the default and matches modern browser support.
    sri(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['logo.svg', 'manifest.json'],
      manifest: {
        name: 'Vakt — Security & Compliance',
        short_name: 'Vakt',
        description: 'Self-hosted Security and Compliance Platform',
        theme_color: '#6366f1',
        background_color: '#0f172a',
        display: 'standalone',
        start_url: '/',
        icons: [
          {
            src: '/logo.svg',
            sizes: 'any',
            type: 'image/svg+xml',
            purpose: 'any maskable',
          },
        ],
        lang: 'de',
        categories: ['business', 'productivity'],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
        runtimeCaching: [
          {
            urlPattern: /^\/api\/v1\/(license|health)/,
            handler: 'StaleWhileRevalidate',
            options: {
              cacheName: 'api-cache',
              expiration: { maxEntries: 10, maxAgeSeconds: 300 },
            },
          },
        ],
        navigateFallback: '/index.html',
        navigateFallbackDenylist: [/^\/api\//],
      },
      devOptions: {
        enabled: false, // don't run SW in dev
      },
    }),
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
  build: {},
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test-setup.ts'],
    exclude: ['**/node_modules/**', '**/e2e/**'],
  },
})
