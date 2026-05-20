import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Monitor, Trash2, LogOut } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { apiFetch } from '../api/client'
import { SkeletonTable } from '../shared/components/SkeletonLoaders'

// ─── Types ────────────────────────────────────────────────────────────────────

type Session = {
  id: string
  device_hint?: string
  last_used: string
  created_at: string
  expires_at: string
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

function useRevokeAllSessions() {
  const qc = useQueryClient()
  return useMutation<unknown, Error>({
    mutationFn: () => apiFetch<unknown>('/auth/sessions', { method: 'DELETE' }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['sessions'] }),
  })
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatDate(iso: string): string {
  try {
    return new Intl.DateTimeFormat('de-DE', {
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
  const revokeAll = useRevokeAllSessions()

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title={t('settings.sessionsPage.title')}
        description={t('settings.sessionsPage.description')}
      />

      <Card className="p-0 overflow-hidden">
        {/* Table header */}
        <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-x-4 px-4 py-2.5 border-b border-border bg-muted/30">
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.sessionsPage.colDevice')}</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">{t('settings.sessionsPage.colCreated')}</span>
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
            className="grid grid-cols-[1fr_1fr_1fr_auto] gap-x-4 items-center px-4 py-3 border-b border-border last:border-0"
          >
            <div className="flex items-center gap-2 min-w-0">
              <Monitor className="w-4 h-4 text-secondary shrink-0" />
              <span className="text-sm text-primary truncate">
                {parseUserAgent(session.device_hint)}
              </span>
            </div>
            <span className="text-sm text-secondary">
              {formatDate(session.created_at)}
            </span>
            <span className="text-sm text-secondary">
              {formatDate(session.expires_at)}
            </span>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive hover:bg-destructive/10"
              disabled={revoke.isPending}
              onClick={() => revoke.mutate(session.id)}
            >
              <Trash2 className="w-4 h-4" />
              <span className="sr-only">{t('settings.sessionsPage.revokeSession')}</span>
            </Button>
          </div>
        ))}
      </Card>

      {/* Revoke all */}
      {sessions && sessions.length > 0 && (
        <div className="flex justify-end">
          <Button
            variant="destructive"
            disabled={revokeAll.isPending}
            onClick={() => revokeAll.mutate()}
          >
            <LogOut className="mr-2 h-4 w-4" />
            {revokeAll.isPending ? t('settings.sessionsPage.revokingAll') : t('settings.sessionsPage.revokeAll')}
          </Button>
        </div>
      )}
    </div>
  )
}
