import { useState, useEffect } from 'react'
import { Download, X } from 'lucide-react'

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

const DISMISSED_KEY = 'vakt_pwa_dismissed'

export function PWAInstallPrompt() {
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null)
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    // Don't show if already installed (running in standalone mode)
    const isStandalone =
      window.matchMedia('(display-mode: standalone)').matches ||
      ('standalone' in window.navigator && (window.navigator as { standalone?: boolean }).standalone === true)

    if (isStandalone) return

    // Don't show if user already dismissed
    if (localStorage.getItem(DISMISSED_KEY)) return

    function handleBeforeInstallPrompt(e: Event) {
      e.preventDefault()
      setDeferredPrompt(e as BeforeInstallPromptEvent)
      setVisible(true)
    }

    window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
    return () => window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
  }, [])

  async function handleInstall() {
    if (!deferredPrompt) return
    await deferredPrompt.prompt()
    const { outcome } = await deferredPrompt.userChoice
    if (outcome === 'accepted') {
      setVisible(false)
    }
    setDeferredPrompt(null)
  }

  function handleDismiss() {
    localStorage.setItem(DISMISSED_KEY, '1')
    setVisible(false)
  }

  if (!visible) return null

  return (
    <div
      role="banner"
      className="fixed bottom-4 left-1/2 -translate-x-1/2 z-50 w-[calc(100%-2rem)] max-w-sm
        bg-surface border border-border rounded-xl shadow-lg px-4 py-3
        flex items-center gap-3 animate-in slide-in-from-bottom-4 duration-300"
    >
      <div className="flex-shrink-0 w-9 h-9 rounded-lg bg-brand/10 flex items-center justify-center">
        <Download className="w-5 h-5 text-brand" />
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-primary leading-tight">Vakt als App installieren</p>
        <p className="text-xs text-secondary truncate">Schnellzugriff ohne Browser</p>
      </div>
      <button
        onClick={handleInstall}
        className="flex-shrink-0 rounded-lg bg-brand px-3 py-1.5 text-xs font-semibold text-white
          hover:bg-brand/90 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand"
      >
        Installieren
      </button>
      <button
        onClick={handleDismiss}
        aria-label="Schließen"
        className="flex-shrink-0 text-secondary hover:text-primary transition-colors p-1 rounded"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}
