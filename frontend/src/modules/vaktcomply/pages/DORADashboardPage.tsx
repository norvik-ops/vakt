import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'
import { ShieldAlert, AlertTriangle, Clock, Building2, FlaskConical, FileDown } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { cn } from '../../../lib/utils'
import { useDORADashboard } from '../hooks/useDORADashboard'
import { ProGate } from '../../../shared/components/ProGate'

function readinessColorClass(pct: number): string {
  if (pct >= 80) return 'text-green-500'
  if (pct >= 50) return 'text-yellow-500'
  return 'text-red-500'
}

function readinessBgClass(pct: number): string {
  if (pct >= 80) return 'bg-green-500/20 border-green-500/30'
  if (pct >= 50) return 'bg-yellow-500/20 border-yellow-500/30'
  return 'bg-red-500/20 border-red-500/30'
}

function useFormatCountdown() {
  const { t } = useTranslation()
  return (deadlineAt: string): string => {
    const now = new Date()
    const deadline = new Date(deadlineAt)
    const diffMs = deadline.getTime() - now.getTime()
    if (diffMs <= 0) return t('vaktcomply.doraDashboard.overdueLabel')
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
    if (diffHours < 24) return t('vaktcomply.doraDashboard.hoursLeft', { hours: diffHours.toString() })
    const diffDays = Math.floor(diffHours / 24)
    return t('vaktcomply.doraDashboard.daysLeft', { days: diffDays.toString() })
  }
}

export default function DORADashboardPage() {
  const { t } = useTranslation()
  const { data: result, isLoading, isError, error } = useDORADashboard()
  const [pdfError, setPdfError] = useState<string | null>(null)
  const formatCountdown = useFormatCountdown()

  async function handleDownloadPDF() {
    const res = await fetch('/api/v1/vaktcomply/dora/report-pdf', {
      credentials: 'include',
    })
    if (!res.ok) {
      setPdfError(t('vaktcomply.doraDashboard.pdfError'))
      return
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'dora-bericht.pdf'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-brand" />
      </div>
    )
  }

  if (isError) {
    return <ProGate error={error}><div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg m-6">{t('vaktcomply.doraDashboard.loadError')}</div></ProGate>
  }

  if (result?.notEnabled) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title={t('vaktcomply.doraDashboard.title')}
          description={t('vaktcomply.doraDashboard.description')}
        />
        <div className="flex-1 p-6">
          <div
            className="flex items-start gap-3 p-4 rounded-lg bg-yellow-500/10 border border-yellow-500/30"
            data-testid="dora-not-enabled-banner"
          >
            <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-primary">{t('vaktcomply.doraDashboard.notEnabledTitle')}</p>
              <p className="text-xs text-secondary mt-1">
                {t('vaktcomply.doraDashboard.notEnabledDescription')}
              </p>
              <Link
                to="/vaktcomply/frameworks"
                className="text-xs text-brand underline mt-2 inline-block"
              >
                {t('vaktcomply.doraDashboard.toFrameworks')}
              </Link>
            </div>
          </div>
        </div>
      </div>
    )
  }

  const dashboard = result?.data
  if (!dashboard) return (
    <div className="p-6 text-sm text-muted-foreground">
      {t('vaktcomply.doraDashboard.noData')}
    </div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t('vaktcomply.doraDashboard.title')}
        description={t('vaktcomply.doraDashboard.description')}
        actions={
          <div className="flex flex-col items-end gap-1">
            <Button variant="outline" size="sm" onClick={() => { setPdfError(null); void handleDownloadPDF() }}>
              <FileDown className="w-4 h-4 mr-2" />
              {t('vaktcomply.doraDashboard.pdfExport')}
            </Button>
            {pdfError && <p className="text-xs text-red-500">{pdfError}</p>}
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* 6-Tile Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">

          {/* Tile 1: Bereitschaftsgrad */}
          <Card data-testid="tile-readiness">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <ShieldAlert className="w-4 h-4" />
                {t('vaktcomply.doraDashboard.tileReadiness')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div
                className={cn('text-3xl font-bold', readinessColorClass(dashboard.readiness_pct))}
                data-testid="readiness-value"
              >
                {dashboard.readiness_pct.toFixed(0)}%
              </div>
              <div
                className={cn(
                  'mt-2 text-xs px-2 py-0.5 rounded-full border inline-block',
                  readinessBgClass(dashboard.readiness_pct),
                )}
              >
                {dashboard.readiness_pct >= 80
                  ? t('vaktcomply.doraDashboard.tileReadinessGood')
                  : dashboard.readiness_pct >= 50
                    ? t('vaktcomply.doraDashboard.tileReadinessMedium')
                    : t('vaktcomply.doraDashboard.tileReadinessCritical')}
              </div>
            </CardContent>
          </Card>

          {/* Tile 2: Offene Critical Controls */}
          <Card data-testid="tile-critical-controls">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <AlertTriangle className="w-4 h-4" />
                {t('vaktcomply.doraDashboard.tileCriticalControls')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-primary">
                {dashboard.open_critical_controls}
              </div>
              <Badge
                variant={dashboard.open_critical_controls > 0 ? 'destructive' : 'success'}
                className="mt-2 text-xs"
              >
                {dashboard.open_critical_controls > 0 ? t('vaktcomply.doraDashboard.open') : t('vaktcomply.doraDashboard.allFulfilled')}
              </Badge>
            </CardContent>
          </Card>

          {/* Tile 3: Nächste Meldepflicht */}
          <Card data-testid="tile-next-deadline">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <Clock className="w-4 h-4" />
                {t('vaktcomply.doraDashboard.tileNextDeadline')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {dashboard.next_deadline ? (
                <div>
                  <p className="text-sm font-medium text-primary truncate" title={dashboard.next_deadline.title}>
                    {dashboard.next_deadline.title}
                  </p>
                  <Badge variant="outline" className="mt-1 text-xs" data-testid="deadline-type-badge">
                    {dashboard.next_deadline.deadline_type}
                  </Badge>
                  <p className="text-xs text-secondary mt-1">
                    {formatCountdown(dashboard.next_deadline.deadline_at)}
                  </p>
                </div>
              ) : (
                <div>
                  <div className="text-sm text-green-500 font-medium">{t('vaktcomply.doraDashboard.noDeadlines')}</div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Tile 4: Abgelaufene Verträge */}
          <Card data-testid="tile-expired-suppliers">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <Building2 className="w-4 h-4" />
                {t('vaktcomply.doraDashboard.tileExpiredSuppliers')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-primary">{dashboard.expired_suppliers}</div>
              <Link to="/vaktcomply/suppliers" className="text-xs text-brand underline mt-1 inline-block">
                {t('vaktcomply.doraDashboard.toSuppliers')}
              </Link>
            </CardContent>
          </Card>

          {/* Tile 5: TLPT-Status */}
          <Card data-testid="tile-tlpt-status">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <FlaskConical className="w-4 h-4" />
                {t('vaktcomply.doraDashboard.tileTlpt')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {dashboard.tlpt_overdue_warning ? (
                <Badge
                  variant="destructive"
                  className="text-xs"
                  data-testid="tlpt-warning-badge"
                >
                  {t('vaktcomply.doraDashboard.tlptOverdue')}
                </Badge>
              ) : (
                <Badge variant="success" className="text-xs" data-testid="tlpt-ok-badge">
                  {t('vaktcomply.doraDashboard.tlptOk')}
                </Badge>
              )}
            </CardContent>
          </Card>

          {/* Tile 6: IKT-Drittanbieter (S38-3) */}
          <Card data-testid="tile-third-parties">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <Building2 className="w-4 h-4" />
                {t('vaktcomply.doraDashboard.tileThirdParties')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-primary" data-testid="third-party-count">
                {dashboard.third_party_count}
              </div>
              {dashboard.missing_exit_strategies > 0 ? (
                <Badge variant="destructive" className="mt-2 text-xs">
                  {t('vaktcomply.doraDashboard.missingExitPlan', { count: dashboard.missing_exit_strategies })}
                </Badge>
              ) : dashboard.critical_third_parties > 0 ? (
                <Badge variant="success" className="mt-2 text-xs">
                  {t('vaktcomply.doraDashboard.criticalWithExitPlan', { count: dashboard.critical_third_parties })}
                </Badge>
              ) : null}
              <Link to="/vaktcomply/dora/third-parties" className="text-xs text-brand underline mt-1 inline-block">
                {t('vaktcomply.doraDashboard.toRegister')}
              </Link>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
