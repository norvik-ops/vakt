import { create } from 'zustand'
import { persist } from 'zustand/middleware'

export type Theme = 'dark' | 'light' | 'system'

interface ThemeStore {
  theme: Theme
  toggle: () => void
  setTheme: (theme: Theme) => void
  apply: () => void
}

export const useThemeStore = create<ThemeStore>()(
  persist(
    (set, get) => ({
      theme: 'system',
      toggle: () => {
        // Cycle: dark → light → system → dark
        const cycle: Theme[] = ['dark', 'light', 'system']
        const current = get().theme
        const next = cycle[(cycle.indexOf(current) + 1) % cycle.length]
        set({ theme: next })
        applyTheme(next)
      },
      setTheme: (theme: Theme) => {
        set({ theme })
        applyTheme(theme)
      },
      apply: () => applyTheme(get().theme),
    }),
    { name: 'vakt-theme' },
  ),
)

function applyTheme(theme: Theme) {
  const root = document.documentElement
  const isDark =
    theme === 'dark' ||
    (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  root.classList.toggle('dark', isDark)
}

// Re-apply when the OS color scheme changes (e.g. user toggles system dark mode while app is open).
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  const theme = useThemeStore.getState().theme
  if (theme === 'system') applyTheme('system')
})
