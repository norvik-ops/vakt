import type { VitePWAOptions } from 'vite-plugin-pwa'

// Service-Worker-Konfiguration (vite-plugin-pwa / Workbox), eigene Datei, damit
// pwaOptions.test.ts sie prüfen kann.
//
// Warum index.html NICHT im Precache liegt: Ein Service Worker, der Seitenaufrufe
// aus dem Precache beantwortet, liefert nach einem Deploy beim nächsten Öffnen
// noch die ALTE index.html und damit die alten Chunk-Namen. Der neue Service
// Worker übernimmt Sekunden später (skipWaiting + clientsClaim, Standard bei
// registerType 'autoUpdate') und räumt die alten Chunks aus dem Precache; der
// Server hat sie ohnehin nicht mehr. Der offene Tab zeigt den alten Stand und
// scheitert beim ersten Lazy-Import („Importing a module script failed.“).
// Seitenaufrufe gehen deshalb zuerst ins Netz (index.html kommt mit no-store);
// nur offline greift die zuletzt geladene Fassung aus dem Cache.
export const pwaOptions: Partial<VitePWAOptions> = {
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
      { src: '/pwa-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
      { src: '/pwa-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
      // maskable braucht eine eigene Datei: vollflächiger Grund statt abgerundetem
      // Badge, Glyph in der 80%-Safe-Zone — sonst schneidet die Launcher-Maske hinein.
      { src: '/pwa-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
    ],
    lang: 'de',
    categories: ['business', 'productivity'],
  },
  workbox: {
    // Kein html: siehe Kopfkommentar. navigateFallback ausdrücklich null —
    // undefined setzt vite-plugin-pwa auf 'index.html', dann bediente der
    // Precache die Seitenaufrufe wieder aus der alten index.html.
    globPatterns: ['**/*.{js,css,svg,png,woff2}'],
    navigateFallback: null,
    skipWaiting: true,
    clientsClaim: true,
    cleanupOutdatedCaches: true,
    runtimeCaching: [
      {
        // Seitenaufrufe der SPA: Netz zuerst, ohne Timeout — ein Timeout würde
        // bei langsamer Verbindung wieder den alten Stand ausliefern.
        // /api/ bleibt draußen (Downloads, SSO-Rückrufe dürfen nie aus dem Cache kommen).
        urlPattern: ({ request, url }) => request.mode === 'navigate' && !url.pathname.startsWith('/api/'),
        handler: 'NetworkFirst',
        options: {
          cacheName: 'pages',
          cacheableResponse: { statuses: [200] },
          expiration: { maxEntries: 20 },
        },
      },
      {
        urlPattern: /^\/api\/v1\/(license|health)/,
        handler: 'StaleWhileRevalidate',
        options: {
          cacheName: 'api-cache',
          expiration: { maxEntries: 10, maxAgeSeconds: 300 },
        },
      },
    ],
  },
  devOptions: {
    enabled: false, // don't run SW in dev
  },
}
