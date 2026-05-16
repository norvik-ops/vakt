import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useDemoMode } from '../hooks/useDemoMode'

interface VersionCheckResponse {
  current: string
  latest: string
  update_available: boolean
}

async function fetchVersionCheck(): Promise<VersionCheckResponse> {
  const res = await fetch('/api/v1/version/check')
  if (!res.ok) throw new Error('version check failed')
  return res.json()
}

export function VersionBanner() {
  const demoMode = useDemoMode()
  const [dismissed, setDismissed] = useState(false)

  const { data } = useQuery<VersionCheckResponse>({
    queryKey: ['version-check'],
    queryFn: fetchVersionCheck,
    staleTime: 60 * 60 * 1000, // 1 hour
    retry: false,
  })

  if (demoMode || dismissed || !data?.update_available) {
    return null
  }

  return (
    <div className="bg-amber-50 border-b border-amber-200 px-4 py-2 flex items-center justify-between text-sm shrink-0">
      <span className="text-amber-800">
        Neue Version verfügbar: v{data.latest} —{' '}
        <a
          href="https://github.com/sechealth-app/sechealth/releases"
          target="_blank"
          rel="noopener noreferrer"
          className="underline hover:text-amber-900 font-medium"
        >
          Jetzt aktualisieren
        </a>
      </span>
      <button
        onClick={() => setDismissed(true)}
        className="text-amber-600 hover:text-amber-800 ml-4"
      >
        ✕
      </button>
    </div>
  )
}
