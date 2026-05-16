import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

interface DemoStartResponse {
  admin_email: string
  analyst_email: string
}

export default function DemoLanding() {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleStart() {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/v1/demo/start', { method: 'POST' })
      if (res.status === 429) {
        setError('Demo-Kapazität ausgelastet. Bitte in einigen Minuten erneut versuchen.')
        return
      }
      if (!res.ok) {
        setError('Demo konnte nicht gestartet werden. Bitte versuche es erneut.')
        return
      }
      const data = await res.json() as DemoStartResponse
      navigate('/login', {
        replace: true,
        state: { demoEmails: { admin: data.admin_email, analyst: data.analyst_email } },
      })
    } catch {
      setError('Demo konnte nicht gestartet werden. Bitte versuche es erneut.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen bg-[#0f0f1a] flex flex-col items-center justify-center px-4">
      <div className="w-full max-w-md text-center">
        <div className="flex justify-center mb-8">
          <img src="/logo.svg" alt="Vakt" className="w-16 h-16" />
        </div>

        <h1 className="text-3xl font-bold text-white mb-2">Vakt Demo</h1>
        <p className="text-slate-300 text-sm mb-2">
          Compliance-Dokumentation für NIS2, ISO 27001 und DSGVO — in deiner eigenen Infrastruktur.
        </p>
        <p className="text-slate-400 mb-8">
          Deine persönliche Demo-Umgebung mit realistischen Beispieldaten —
          bereit in Sekunden, automatisch gelöscht nach 4 Stunden.
        </p>

        <div className="bg-white/[.05] border border-white/[.1] rounded-xl p-6 mb-6 text-left space-y-3">
          <Feature text="Alle 5 Module vorausgefüllt mit echten Szenarien" />
          <Feature text="Vollständig isoliert — deine eigene Instanz" />
          <Feature text="Kein Account nötig, kein Passwort" />
          <Feature text="Automatisch bereinigt nach 4 Stunden" />
        </div>

        {error && (
          <p className="text-red-400 text-sm mb-4">{error}</p>
        )}

        <button
          type="button"
          onClick={() => void handleStart()}
          disabled={loading}
          className="w-full bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold py-3 px-6 rounded-lg transition-colors"
        >
          {loading ? 'Demo wird gestartet…' : 'Demo starten'}
        </button>

        <div className="mt-4 space-y-2">
          <a
            href="https://sec.norvikops.de"
            target="_blank"
            rel="noopener noreferrer"
            className="block text-indigo-400 hover:underline text-sm"
          >
            Mehr über Vakt erfahren →
          </a>
          <p className="text-slate-500 text-xs">
            Für den produktiven Einsatz:{' '}
            <a
              href="https://github.com/norvik-ops/vakt"
              target="_blank"
              rel="noopener noreferrer"
              className="text-indigo-400 hover:underline"
            >
              Selbst hosten auf GitHub
            </a>
          </p>
        </div>
      </div>
    </div>
  )
}

function Feature({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-3 text-sm text-slate-300">
      <span className="text-green-400 shrink-0">✓</span>
      {text}
    </div>
  )
}
