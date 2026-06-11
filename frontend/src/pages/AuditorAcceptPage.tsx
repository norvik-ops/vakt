import { useState, useRef, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { ShieldCheck, Copy, AlertTriangle, ExternalLink } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'

// ---------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------

interface AcceptResponse {
  session_token: string
  message: string
}

async function acceptInvite(token: string): Promise<AcceptResponse> {
  const res = await fetch(`/api/v1/auditor/accept/${token}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  })
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: string }
    throw new Error(body.error ?? 'ACCEPT_FAILED')
  }
  return res.json() as Promise<AcceptResponse>
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function AuditorAcceptPage() {
  const { token } = useParams<{ token: string }>()
  const [sessionToken, setSessionToken] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const copiedTimerRef = useRef<ReturnType<typeof setTimeout>>()
  useEffect(() => () => { clearTimeout(copiedTimerRef.current); }, [])

  const accept = useMutation<AcceptResponse>({
    mutationFn: () => acceptInvite(token ?? ''),
    onSuccess: (data) => {
      setSessionToken(data.session_token)
    },
  })

  function handleCopy() {
    if (!sessionToken) return
    void navigator.clipboard.writeText(sessionToken).then(() => {
      setCopied(true)
      copiedTimerRef.current = setTimeout(() => { setCopied(false); }, 2000)
    })
  }

  if (sessionToken) {
    return (
      <div className="min-h-screen bg-bg flex items-center justify-center p-6">
        <div className="w-full max-w-lg space-y-6">
          <div className="flex flex-col items-center gap-3 text-center">
            <ShieldCheck className="w-12 h-12 text-brand" />
            <h1 className="text-2xl font-bold text-primary">Auditor-Zugang aktiviert</h1>
            <p className="text-secondary text-sm">
              Dein Zugang ist jetzt aktiv. Verwende den folgenden Session-Token als
              Bearer-Token in deinen API-Anfragen.
            </p>
          </div>

          <div className="rounded-lg border border-border bg-surface p-4 space-y-3">
            <p className="text-xs font-semibold text-secondary uppercase tracking-wider">Session-Token</p>
            <div className="flex items-center gap-2">
              <Input
                readOnly
                value={sessionToken}
                className="font-mono text-xs"
              />
              <Button variant="outline" size="sm" onClick={handleCopy}>
                <Copy className="w-4 h-4 mr-1" />
                {copied ? 'Kopiert!' : 'Kopieren'}
              </Button>
            </div>
          </div>

          <Button
            className="w-full"
            onClick={() => {
              // Store token in sessionStorage and navigate to the auditor portal.
              sessionStorage.setItem('auditor_session_token', sessionToken)
              window.location.href = '/auditor/portal'
            }}
          >
            <ExternalLink className="w-4 h-4 mr-2" />
            Im Auditor-Portal öffnen
          </Button>

          <div className="rounded-lg border border-border bg-surface p-4 space-y-2 text-sm">
            <p className="font-semibold text-primary">API-Zugang</p>
            <p className="text-secondary">
              Alternativ: Token als <code className="bg-muted px-1 rounded text-xs">Authorization</code>-Header
              in API-Anfragen verwenden:
            </p>
            <pre className="bg-muted rounded p-3 text-xs overflow-x-auto">
              {`Authorization: Bearer ${sessionToken}`}
            </pre>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-bg flex items-center justify-center p-6">
      <div className="w-full max-w-md space-y-6 text-center">
        <div className="flex flex-col items-center gap-3">
          <ShieldCheck className="w-12 h-12 text-brand" />
          <h1 className="text-2xl font-bold text-primary">Auditor-Einladung</h1>
          <p className="text-secondary text-sm">
            Du wurdest eingeladen, als externer Auditor auf die Compliance-Dokumentation
            dieser Organisation zuzugreifen.
          </p>
        </div>

        {accept.isError && (
          <div className="flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            <AlertTriangle className="w-4 h-4 shrink-0" />
            <span>
              {accept.error.message === 'invalid or expired invite token'
                ? 'Der Einladungs-Link ist ungueltig oder bereits abgelaufen.'
                : 'Einladung konnte nicht aktiviert werden. Bitte versuche es erneut.'}
            </span>
          </div>
        )}

        <Button
          className="w-full"
          size="lg"
          onClick={() => { accept.mutate(); }}
          disabled={accept.isPending || !token}
        >
          {accept.isPending ? 'Aktiviere...' : 'Zugang aktivieren'}
        </Button>
      </div>
    </div>
  )
}
