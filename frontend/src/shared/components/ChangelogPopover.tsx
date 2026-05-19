import { useState } from 'react'
import { Sparkles, X } from 'lucide-react'
import changelog from '../data/changelog.json'

interface ChangelogEntry {
  type: 'feat' | 'improvement' | 'fix'
  text: string
}

interface ChangelogVersion {
  version: string
  date: string
  entries: ChangelogEntry[]
}

const STORAGE_KEY = 'vakt_last_seen_changelog'
const LATEST_VERSION = (changelog as ChangelogVersion[])[0]?.version ?? '0.0.0'

function useChangelogState() {
  const [lastSeen, setLastSeen] = useState(() =>
    localStorage.getItem(STORAGE_KEY) ?? ''
  )
  const hasNew = lastSeen !== LATEST_VERSION

  function markSeen() {
    localStorage.setItem(STORAGE_KEY, LATEST_VERSION)
    setLastSeen(LATEST_VERSION)
  }

  return { hasNew, markSeen }
}

const TYPE_LABELS: Record<ChangelogEntry['type'], { label: string; color: string }> = {
  feat: { label: 'Neu', color: 'bg-blue-100 text-blue-700' },
  improvement: { label: 'Verbesserung', color: 'bg-purple-100 text-purple-700' },
  fix: { label: 'Behoben', color: 'bg-green-100 text-green-700' },
}

export function ChangelogPopover() {
  const [open, setOpen] = useState(false)
  const { hasNew, markSeen } = useChangelogState()

  function handleOpen() {
    setOpen(true)
    markSeen()
  }

  return (
    <div className="relative">
      <button
        onClick={handleOpen}
        className="relative p-2 rounded-lg hover:bg-muted/50 transition-colors"
        title="Neue Features"
        aria-label="Changelog öffnen"
      >
        <Sparkles className="h-4 w-4 text-secondary" aria-hidden="true" />
        {hasNew && (
          <span className="absolute top-1 right-1 w-2 h-2 bg-blue-500 rounded-full" aria-hidden="true" />
        )}
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} aria-hidden="true" />
          <div className="absolute right-0 bottom-10 z-50 w-80 bg-surface border border-border rounded-xl shadow-lg overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-surface2">
              <h3 className="font-semibold text-sm">Neue Funktionen</h3>
              <button
                onClick={() => setOpen(false)}
                className="p-1 hover:bg-muted/50 rounded"
                aria-label="Changelog schließen"
              >
                <X className="h-4 w-4 text-secondary" aria-hidden="true" />
              </button>
            </div>
            <div className="overflow-y-auto max-h-96">
              {(changelog as ChangelogVersion[]).map((version) => (
                <div key={version.version} className="px-4 py-3 border-b border-border last:border-b-0">
                  <div className="flex items-center gap-2 mb-2">
                    <span className="font-mono text-xs font-bold text-primary">v{version.version}</span>
                    <span className="text-xs text-secondary">
                      {new Date(version.date).toLocaleDateString('de-DE')}
                    </span>
                  </div>
                  <ul className="space-y-1.5">
                    {version.entries.map((entry, i) => {
                      const badge = TYPE_LABELS[entry.type] ?? TYPE_LABELS.improvement
                      return (
                        <li key={i} className="flex gap-2 items-start">
                          <span
                            className={`text-xs px-1.5 py-0.5 rounded font-medium flex-shrink-0 mt-0.5 ${badge.color}`}
                          >
                            {badge.label}
                          </span>
                          <span className="text-xs text-secondary">{entry.text}</span>
                        </li>
                      )
                    })}
                  </ul>
                </div>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
