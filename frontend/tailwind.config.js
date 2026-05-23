/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        brand: '#4f46e5',
        'brand-hover': '#6366f1',
        // semantic tokens — map to CSS variables
        bg:        'var(--color-bg)',
        surface:   'var(--color-surface)',
        surface2:  'var(--color-surface2)',
        border:    'var(--color-border)',
        border2:   'var(--color-border2)',
        primary:   'var(--color-text)',
        secondary: 'var(--color-text2)',
        muted:     'var(--color-text3)',
        // Sprint 16 S16-1: semantische Severity- und Status-Farben.
        // Ersetzen die bisherigen `bg-[#hexhex]`-Bracket-Notations und
        // ermöglichen ein späteres Whitelabel-Theme via CSS-Variable.
        severity: {
          critical: '#ef4444',          // text/icon
          'critical-bg': '#7f1d1d',     // dunkler bg
          high:     '#f97316',          // text/icon
          'high-bg': '#7c2d12',         // dunkler bg
          medium:   '#f59e0b',          // text/icon
          'medium-bg': '#78350f',       // dunkler bg
          low:      '#22c55e',          // text/icon
          'low-bg': '#14532d',          // dunkler bg
          info:     '#93c5fd',          // text/icon
          'info-bg': '#1e3a5f',         // dunkler bg
        },
      },
      boxShadow: {
        brand: '0 0 24px rgba(79,70,229,0.35)',
      },
    },
  },
  plugins: [],
}
