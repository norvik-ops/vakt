import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Monitor, Trash2, LogOut, AlertTriangle, CheckCircle2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { apiFetch } from '../api/client'
import { SkeletonTable } from '../shared/components/SkeletonLoaders'
import { formatLocale } from '../shared/utils/locale'

// ─── Types ────────────────────────────────────────────────────────────────────

type Session = {
  id: string
  device_hint?: string
  last_used: string
  created_at: string
  expires_at: string
  is_current?: boolean
}

// ─── Hooks ────────────────────────────────────────────────────────────────────

function useSessions() {
  return useQuery<Session[]>({
    queryKey: ['sessions'],
    queryFn: () => apiFetch<Session[]>('/auth/sessions'),
    retry: false,
  })
}

function useRevokeSession() {
  const qc = useQueryClient()
  return useMutation<unknown, Error, string>({
    mutationFn: (id) => apiFetch<unknown>(`/auth/sessions/${id}`, { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['sessions'] }),
  })
}

// Revoke-all-OTHERS: Backend liest X-Vakt-Session-Id aus dem apiFetch-Header
// und behält die aktuelle Session. Wird vom "Andere abmelden"-Button verwendet.
function useRevokeOtherSessions() {
  const qc = useQueryClient()
  return useMutation<unknown, Error>({
    mutationFn: () => apiFetch<unknown>('/auth/sessions', { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['sessions'] }),
  })
}

// Panic-Button: explizit OHNE die aktuelle Session ausnehmen. Frontend muss
// nach dem Call zwingend redirecten — der eigene Token ist gleich invalide.
async function panicRevokeAll(): Promise<void> {
  await fetch('/api/v1/auth/sessions', {
    method: 'DELETE',
    credentials: 'include',
    headers: { 'X-CSRF-Token': document.cookie.match(/csrf_token=([^;]+)/)?.[1] ?? '' },
  })
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat(formatLocale(), {
      dateStyle: 'medium',
      timeStyle: 'short',
    }).format(new Date(iso))
  } catch {
    return iso
  }
}

function useParseUserAgent() {
  const { t } = useTranslation()
  return function parseUserAgent(ua?: string): string {
    if (!ua) return t('settings.sessionsPage.unknownDevice')
    if (ua.includes('Firefox')) return 'Firefox'
    if (ua.includes('Chrome')) return 'Chrome'
    if (ua.includes('Safari')) return 'Safari'
    if (ua.includes('Edge')) return 'Edge'
    return ua.slice(0, 60)
  }
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function SessionsPage() {
  const { t } = useTranslation()
  const parseUserAgent = useParseUserAgent()
  const { data: sessions, isLoading, isError } = useSessions()
  const revoke = useRevokeSession()
  const revokeOthers = useRevokeOtherSessions()
  const [panicConfirm, setPanicConfirm] = useState(false)
  const [panicRunning, setPanicRunning] = useState(false)

  const handlePanic = async () => {
    if (!panicConfirm) {
      setPanicConfirm(true)
      return
    }
    setPanicRunning(true)
    try {
      await panicRevokeAll()
    } finally {
      // Egal ob OK oder Fehler — wir leiten weiter, das eigene Token ist tot.
      window.location.href = '/login'
    }
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={t('settings.sessionsPage.title')}
        description={t('settings.sessionsPage.description')}
      />

      <Card className="p-0 overflow-hidden">
        {/* Table header */}
        <div className="grid grid-cols-[auto_1fr_1fr_1fr_auto] gap-x-4 px-4 py-2.5 border-b border-border bg-muted/30">
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">·</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.sessionsPage.colDevice')}</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Zuletzt aktiv</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.sessionsPage.colExpiry')}</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.sessionsPage.colActions')}</span>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="px-4 py-4">
            <SkeletonTable rows={3} cols={4} />
          </div>
        )}

        {/* Error */}
        {isError && (
          <div className="px-4 py-8 text-center text-sm text-destructive">
            {t('settings.sessionsPage.loadError')}
          </div>
        )}

        {/* Empty */}
        {!isLoading && !isError && sessions?.length === 0 && (
          <div className="px-4 py-8 text-center text-sm text-secondary">
            {t('settings.sessionsPage.noSessions')}
          </div>
        )}

        {/* Rows */}
        {sessions?.map((session) => (
          <div
            key={session.id}
            className={`grid grid-cols-[auto_1fr_1fr_1fr_auto] gap-x-4 items-center px-4 py-3 border-b border-border last:border-0 ${session.is_current ? 'bg-brand/5' : ''}`}
          >
            <div className="flex items-center">
              {session.is_current ? (
                <span title="Diese Session" className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold uppercase bg-brand/10 text-brand">
                  <CheckCircle2 className="w-3 h-3" />
                  Diese hier
                </span>
              ) : (
                <span className="text-[10px] text-secondary">·</span>
              )}
            </div>
            <div className="flex items-center gap-2 min-w-0">
              <Monitor className="w-4 h-4 text-secondary shrink-0" />
              <span className="text-sm text-primary truncate">
                {parseUserAgent(session.device_hint)}
              </span>
            </div>
            <span className="text-sm text-secondary">
              {formatDate(session.last_used)}
            </span>
            <span className="text-sm text-secondary">
              {formatDate(session.expires_at)}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive hover:bg-destructive/10"
              disabled={revoke.isPending || session.is_current}
              title={session.is_current ? 'Diese Session abzumelden geht über Logout — nicht hier' : undefined}
              onClick={() => revoke.mutate(session.id)}
            >
              <Trash2 className="w-4 h-4" />
              <span className="sr-only">{t('settings.sessionsPage.revokeSession')}</span>
            </Button>
          </div>
        ))}
      </Card>

      {/* Actions: Andere abmelden + Panic-Button */}
      {sessions && sessions.length > 0 && (
        <div className="flex justify-between items-center gap-3">
          <Button
            variant={panicConfirm ? 'destructive' : 'outline'}
            disabled={panicRunning}
            onClick={() => { void handlePanic() }}
            className={panicConfirm ? '' : 'border-destructive/40 text-destructive hover:bg-destructive/10'}
          >
            <AlertTriangle className="mr-2 h-4 w-4" />
            {panicRunning
              ? 'Beende alle Sessions…'
              : panicConfirm
                ? 'Sicher? Klick zum Bestätigen — auch diese hier wird beendet'
                : 'Panic: Alle Sessions abmelden (inkl. dieser)'}
          </Button>
          <Button
            variant="destructive"
            disabled={revokeOthers.isPending || (sessions.filter(s => !s.is_current).length === 0)}
            onClick={() => revokeOthers.mutate()}
          >
            <LogOut className="mr-2 h-4 w-4" />
            {revokeOthers.isPending ? t('settings.sessionsPage.revokingAll') : 'Andere abmelden'}
          </Button>
        </div>
      )}
    </div>
  )
}
