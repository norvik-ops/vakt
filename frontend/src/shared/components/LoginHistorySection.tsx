import { useQuery } from '@tanstack/react-query'
import { History, Check, X, ShieldAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../../api/client'
import { formatLocale } from '../utils/locale'

// Sprint 22 / S22-11: Login-History-Section für die AccountSettingsPage.
// Konsumiert GET /api/v1/account/login-history (Sprint 22 Backend).
//
// Zeigt die letzten 50 Login-Versuche des Users mit IP, UA-Excerpt, Source,
// Result-Badge. Failed-Logins fett markiert, damit User verdächtige
// Versuche schnell erkennen kann.

interface LoginEntry {
  ts: string
  ip?: string
  user_agent?: string
  source: string
  result: string
}

const sourceLabels: Record<string, string> = {
  password: 'Passwort',
  oidc: 'SSO (OIDC)',
  saml: 'SAML',
  register: 'Setup',
  magic_link: 'Magic-Link',
  api_key: 'API-Key',
}

function uaShort(ua: string): string {
  if (!ua) return ''
  // Sehr einfache Heuristik — voll-parsen würde ua-parser-js benötigen.
  if (ua.includes('Firefox')) return 'Firefox'
  if (ua.includes('Edg/')) return 'Edge'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Safari')) return 'Safari'
  if (ua.includes('curl')) return 'curl'
  return ua.length > 30 ? ua.slice(0, 30) + '…' : ua
}

export function LoginHistorySection() {
  const { t: _t } = useTranslation()
  const { data, isLoading } = useQuery<LoginEntry[]>({
    queryKey: ['account', 'login-history'],
    queryFn: () => apiFetch<LoginEntry[]>('/account/login-history'),
    staleTime: 60_000,
  })

  return (
    <section className="rounded-xl border border-border bg-surface p-5 space-y-3">
      <div className="flex items-center gap-2">
        <History className="w-4 h-4 text-brand shrink-0" />
        <h2 className="text-sm font-semibold text-primary">Login-Historie</h2>
        <span className="text-xs text-secondary ml-auto">letzte 50 Versuche · 90 Tage Aufbewahrung</span>
      </div>

      {isLoading && (
        <p className="text-xs text-secondary">Lade…</p>
      )}

      {data && data.length === 0 && (
        <p className="text-xs text-secondary">Noch keine Einträge.</p>
      )}

      {data && data.length > 0 && (
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead className="text-left text-secondary border-b border-border">
              <tr>
                <th className="py-2 pr-3 font-medium">Zeitpunkt</th>
                <th className="py-2 pr-3 font-medium">Quelle</th>
                <th className="py-2 pr-3 font-medium">Browser</th>
                <th className="py-2 pr-3 font-medium">IP</th>
                <th className="py-2 font-medium">Ergebnis</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {data.map((e, i) => {
                const isFail = e.result !== 'ok'
                return (
                  <tr key={i} className={isFail ? 'font-semibold' : ''}>
                    <td className="py-2 pr-3 text-primary">
                      {new Date(e.ts).toLocaleString(formatLocale(), {
                        day: '2-digit', month: '2-digit', year: 'numeric',
                        hour: '2-digit', minute: '2-digit',
                      })}
                    </td>
                    <td className="py-2 pr-3 text-secondary">{sourceLabels[e.source] ?? e.source}</td>
                    <td className="py-2 pr-3 text-secondary">{uaShort(e.user_agent ?? '')}</td>
                    <td className="py-2 pr-3 font-mono text-secondary">{e.ip || '—'}</td>
                    <td className="py-2">
                      {e.result === 'ok' ? (
                        <span className="inline-flex items-center gap-1 text-severity-low">
                          <Check className="w-3 h-3" /> OK
                        </span>
                      ) : e.result === 'mfa_failed' ? (
                        <span className="inline-flex items-center gap-1 text-severity-medium">
                          <ShieldAlert className="w-3 h-3" /> MFA fehlgeschlagen
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-severity-critical">
                          <X className="w-3 h-3" /> {e.result}
                        </span>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      <p className="text-[11px] text-muted">
        Wenn du einen Login-Versuch nicht zuordnen kannst: sofort Passwort wechseln und MFA aktivieren.
      </p>
    </section>
  )
}
