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
        brand: '#6366f1',
        'brand-hover': '#818cf8',
        // semantic tokens — map to CSS variables
        bg:        'var(--color-bg)',
        surface:   'var(--color-surface)',
        surface2:  'var(--color-surface2)',
        border:    'var(--color-border)',
        border2:   'var(--color-border2)',
        primary:   'var(--color-text)',
        secondary: 'var(--color-text2)',
        muted:     'var(--color-text3)',
      },
      boxShadow: {
        brand: '0 0 24px rgba(99,102,241,0.35)',
      },
    },
  },
  plugins: [],
}
