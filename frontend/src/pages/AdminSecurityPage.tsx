import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ShieldAlert, LockOpen, RefreshCw, AlertTriangle, Lock } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { apiFetch } from '../api/client'
import { PageHeader } from '../shared/components/PageHeader'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Skeleton } from '../components/ui/skeleton'
import { useToast } from '../shared/hooks/useToast'
import { useFormatDate } from '../shared/hooks/useFormatDate'

// ─── Types ────────────────────────────────────────────────────────────────────

interface LockedAccount {
  email: string
  locked_at: string
  locked_until: string
}

interface RecentFailure {
  email: string
  ip?: string
  at: string
  count: number
}

interface SecurityEventsData {
  locked_accounts: LockedAccount[]
  recent_failures: RecentFailure[]
  total_locked: number
  failures_last_24h: number
}

// ─── Helpers ──────────────────────────────────────────────────────────────────
// S13-27: vorher hardcoded 'de-DE'. Jetzt nutzt formatDateTime den `useFormatDate`-Hook
// und respektiert die aktive i18n-Locale.

function useFormatTimeRelative() {
  const { t } = useTranslation()
  const { formatDateTime } = useFormatDate()
  return function formatTimeRelative(iso: string) {
    const diff = Date.now() - new Date(iso).getTime()
    const mins = Math.floor(diff / 60_000)
    if (mins < 1) return t('admin.security.justNow')
    if (mins < 60) return t('admin.security.minutesAgo', { count: mins })
    const hrs = Math.floor(mins / 60)
    if (hrs < 24) return t('admin.security.hoursAgo', { count: hrs })
    return formatDateTime(iso, {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit',
    })
  }
}

// ─── Summary Card ─────────────────────────────────────────────────────────────

function SummaryCard({
  icon: Icon,
  label,
  value,
  accent,
}: {
  icon: React.ElementType
  label: string
  value: number | string
  accent?: boolean
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm font-medium text-secondary">
          <Icon className={`w-4 h-4 shrink-0 ${accent ? 'text-red-500' : ''}`} />
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className={`text-[28px] font-bold leading-none ${accent ? 'text-red-600 dark:text-red-400' : 'text-primary'}`}>
          {value.toString()}
        </p>
      </CardContent>
    </Card>
  )
}

// ─── Skeleton ─────────────────────────────────────────────────────────────────

function PageSkeleton() {
  return (
    <div className="p-6 space-y-6">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {Array.from({ length: 2 }).map((_, i) => (
          <Card key={i}>
            <CardHeader><Skeleton className="h-4 w-32" /></CardHeader>
            <CardContent><Skeleton className="h-8 w-16" /></CardContent>
          </Card>
        ))}
      </div>
      <Skeleton className="h-48 w-full rounded-md" />
      <Skeleton className="h-48 w-full rounded-md" />
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function AdminSecurityPage() {
  const { t } = useTranslation()
  const formatTimeRelative = useFormatTimeRelative()
  const { formatDateTime } = useFormatDate()
  const queryClient = useQueryClient()
  const { toast } = useToast()

  const { data, isLoading, isError, error, refetch, isFetching } = useQuery<SecurityEventsData>({
    queryKey: ['admin', 'security-events'],
    queryFn: () => apiFetch<SecurityEventsData>('/admin/security-events'),
    refetchInterval: 60_000, // auto-refresh every 60 seconds
  })

  const unlockMutation = useMutation({
    mutationFn: (email: string) =>
      apiFetch<void>(`/admin/accounts/${encodeURIComponent(email)}/unlock`, {
        method: 'DELETE',
      }),
    onSuccess: (_data, email) => {
      toast({ title: t('admin.security.unlockSuccess', { email }), variant: 'default' })
      void queryClient.invalidateQueries({ queryKey: ['admin', 'security-events'] })
    },
    onError: (e: Error) => {
      toast({ title: t('admin.security.unlockError'), description: e.message, variant: 'destructive' })
    },
  })

  const lockedAccounts = data?.locked_accounts ?? []
  const recentFailures = data?.recent_failures ?? []

  return (
    <div>
      <PageHeader
        title={t('admin.security.title')}
        description={t('admin.security.description')}
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            <RefreshCw className={`w-3.5 h-3.5 mr-1.5 ${isFetching ? 'animate-spin' : ''}`} />
            {t('admin.security.refresh')}
          </Button>
        }
      />

      {isLoading && <PageSkeleton />}

      {isError && (
        <div className="m-6 rounded-md bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-300">
          {t('admin.security.loadError', { message: error instanceof Error ? error.message : t('admin.security.unknownError') })}
        </div>
      )}

      {data && (
        <div className="p-6 space-y-6">
          {/* Summary Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <SummaryCard
              icon={Lock}
              label={t('admin.security.lockedAccounts')}
              value={data.total_locked}
              accent={data.total_locked > 0}
            />
            <SummaryCard
              icon={AlertTriangle}
              label={t('admin.security.loginFailures24h')}
              value={data.failures_last_24h}
              accent={data.failures_last_24h >= 10}
            />
          </div>

          {/* Locked Accounts Table */}
          <div>
            <h2 className="text-sm font-semibold text-primary mb-3 flex items-center gap-2">
              <Lock className="w-4 h-4 shrink-0" />
              {t('admin.security.lockedAccounts')}
            </h2>
            <Card>
              <CardContent className="p-0">
                {lockedAccounts.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-10 text-secondary gap-2">
                    <ShieldAlert className="w-7 h-7 opacity-30" />
                    <p className="text-sm">{t('admin.security.noLockedAccounts')}</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border text-secondary text-[12px] uppercase tracking-wider">
                          <th className="px-4 py-3 text-left font-medium">{t('admin.security.colEmail')}</th>
                          <th className="px-4 py-3 text-left font-medium">{t('admin.security.colLockedSince')}</th>
                          <th className="px-4 py-3 text-left font-medium">{t('admin.security.colLockedUntil')}</th>
                          <th className="px-4 py-3 text-right font-medium">{t('admin.security.colAction')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {lockedAccounts.map((acc) => (
                          <tr key={acc.email} className="hover:bg-surface/50 transition-colors">
                            <td className="px-4 py-3 font-medium text-primary">{acc.email}</td>
                            <td className="px-4 py-3 text-secondary">
                              {formatTimeRelative(acc.locked_at)}
                            </td>
                            <td className="px-4 py-3 text-secondary">
                              {formatDateTime(acc.locked_until, {
                                day: '2-digit', month: '2-digit', year: 'numeric',
                                hour: '2-digit', minute: '2-digit',
                              })}
                            </td>
                            <td className="px-4 py-3 text-right">
                              <Button
                                variant="outline"
                                size="sm"
                                disabled={unlockMutation.isPending}
                                onClick={() => unlockMutation.mutate(acc.email)}
                              >
                                <LockOpen className="w-3 h-3 mr-1.5" />
                                {t('admin.security.unlock')}
                              </Button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Recent Failures Table */}
          <div>
            <h2 className="text-sm font-semibold text-primary mb-3 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 shrink-0" />
              {t('admin.security.failuresSection')}
            </h2>
            <Card>
              <CardContent className="p-0">
                {recentFailures.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-10 text-secondary gap-2">
                    <ShieldAlert className="w-7 h-7 opacity-30" />
                    <p className="text-sm">{t('admin.security.noFailures')}</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border text-secondary text-[12px] uppercase tracking-wider">
                          <th className="px-4 py-3 text-left font-medium">{t('admin.security.colEmail')}</th>
                          <th className="px-4 py-3 text-left font-medium">{t('admin.security.colIp')}</th>
                          <th className="px-4 py-3 text-left font-medium">{t('admin.security.colLastAttempt')}</th>
                          <th className="px-4 py-3 text-right font-medium">{t('admin.security.colCount')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {recentFailures.map((f, idx) => (
                          <tr key={`${f.email}-${f.ip ?? ''}-${idx.toString()}`} className="hover:bg-surface/50 transition-colors">
                            <td className="px-4 py-3 font-medium text-primary">{f.email || '—'}</td>
                            <td className="px-4 py-3 text-secondary font-mono text-[12px]">
                              {f.ip ?? '—'}
                            </td>
                            <td className="px-4 py-3 text-secondary">
                              {formatTimeRelative(f.at)}
                            </td>
                            <td className="px-4 py-3 text-right">
                              <span
                                className={`font-semibold ${
                                  f.count >= 5
                                    ? 'text-red-600 dark:text-red-400'
                                    : 'text-primary'
                                }`}
                              >
                                {f.count.toString()}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </div>
  )
}
