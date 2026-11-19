import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { reloadOnceAfterChunkError, type ChunkReloadResult } from '../chunkReload'

// Nach dieser Wartezeit erscheint auch im Zustand „wird geladen“ der Knopf.
// Grund: Der automatische Reload kann ausbleiben, etwa wenn der Nutzer die
// beforeunload-Rückfrage (useUnsavedChanges) abbricht — ohne Knopf hinge die
// Seite dann für immer.
const RELOAD_BUTTON_DELAY_MS = 3_000

// Anzeige für einen Chunk-Ladefehler (siehe shared/chunkReload.ts): zuerst
// „neue Version wird geladen“ und genau ein automatischer Reload; kommt der
// Fehler danach wieder, ein verständlicher Hinweis mit „Neu laden“-Knopf.
export function ChunkLoadFallback() {
  const { t } = useTranslation()
  const [result, setResult] = useState<ChunkReloadResult | null>(null)
  const [showButton, setShowButton] = useState(false)

  useEffect(() => {
    setResult(reloadOnceAfterChunkError())
    const timer = window.setTimeout(() => { setShowButton(true) }, RELOAD_BUTTON_DELAY_MS)
    return () => { window.clearTimeout(timer) }
  }, [])

  const reloadButton = (
    <button
      onClick={() => { window.location.reload() }}
      className="px-4 py-2 rounded bg-brand text-white text-sm hover:bg-brand-hover transition-colors"
    >
      {t('errors.chunk.reload')}
    </button>
  )

  if (result !== 'gave-up') {
    return (
      <div role="status" className="flex flex-col items-center justify-center gap-4 min-h-[16rem] p-8 text-sm text-secondary">
        <p>{t('errors.chunk.reloading')}</p>
        {showButton && reloadButton}
      </div>
    )
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-bg p-8">
      <div role="alert" className="max-w-md w-full rounded-lg border border-border bg-surface p-8 text-center">
        <h1 className="text-xl font-semibold mb-2">{t('errors.chunk.title')}</h1>
        <p className="text-sm text-secondary mb-4">{t('errors.chunk.text')}</p>
        {reloadButton}
      </div>
    </div>
  )
}
