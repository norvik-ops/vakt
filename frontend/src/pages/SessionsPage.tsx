import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Monitor, Trash2, LogOut } from 'lucide-react'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Card } from '../components/ui/card'
import { apiFetch } from '../api/client'

// ─── Types ────────────────────────────────────────────────────────────────────

type Session = {
  id: string
  user_agent?: string
  ip_address?: string
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

function parseUserAgent(ua?: string): string {
  if (!ua) return 'Unbekanntes Gerät'
  if (ua.includes('Firefox')) return 'Firefox'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Safari')) return 'Safari'
  if (ua.includes('Edge')) return 'Edge'
  return ua.slice(0, 60)
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function SessionsPage() {
  const { data: sessions, isLoading, isError } = useSessions()
  const revoke = useRevokeSession()
  const revokeAll = useRevokeAllSessions()

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Aktive Sitzungen"
        description="Verwalte alle aktiven Anmeldungen deines Kontos."
      />

      <Card className="p-0 overflow-hidden">
        {/* Table header */}
        <div className="grid grid-cols-[1fr_1fr_1fr_1fr_auto] gap-x-4 px-4 py-2.5 border-b border-border bg-muted/30">
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Gerät / Browser</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">IP-Adresse</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Erstellt am</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Ablauf</span>
          <span className="text-[11px] font-semibold text-secondary uppercase tracking-wide">Aktionen</span>
        </div>

        {/* Loading */}
        {isLoading && (
          <div className="px-4 py-8 text-center text-sm text-secondary">
            Lade Sitzungen...
          </div>
        )}

        {/* Error */}
        {isError && (
          <div className="px-4 py-8 text-center text-sm text-destructive">
            Sitzungen konnten nicht geladen werden.
          </div>
        )}

        {/* Empty */}
        {!isLoading && !isError && sessions?.length === 0 && (
          <div className="px-4 py-8 text-center text-sm text-secondary">
            Keine aktiven Sitzungen gefunden.
          </div>
        )}

        {/* Rows */}
        {sessions?.map((session) => (
          <div
            key={session.id}
            className="grid grid-cols-[1fr_1fr_1fr_1fr_auto] gap-x-4 items-center px-4 py-3 border-b border-border last:border-0"
          >
            <div className="flex items-center gap-2 min-w-0">
              <Monitor className="w-4 h-4 text-secondary shrink-0" />
              <span className="text-sm text-primary truncate">
                {parseUserAgent(session.user_agent)}
              </span>
            </div>
            <span className="text-sm text-secondary font-mono">
              {session.ip_address ?? '–'}
            </span>
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
              <span className="sr-only">Widerrufen</span>
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
            {revokeAll.isPending ? 'Wird beendet...' : 'Alle anderen Sitzungen beenden'}
          </Button>
        </div>
      )}
    </div>
  )
}
