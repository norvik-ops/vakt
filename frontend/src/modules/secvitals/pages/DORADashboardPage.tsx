import { useState } from 'react'
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

function formatCountdown(deadlineAt: string): string {
  const now = new Date()
  const deadline = new Date(deadlineAt)
  const diffMs = deadline.getTime() - now.getTime()
  if (diffMs <= 0) return 'Überfällig'
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  if (diffHours < 24) return `${diffHours.toString()}h verbleibend`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays.toString()}d verbleibend`
}

export default function DORADashboardPage() {
  const { data: result, isLoading, isError, error } = useDORADashboard()
  const [pdfError, setPdfError] = useState<string | null>(null)

  async function handleDownloadPDF() {
    const res = await fetch('/api/v1/secvitals/dora/report-pdf', {
      credentials: 'include',
    })
    if (!res.ok) {
      setPdfError('PDF-Export fehlgeschlagen. Bitte versuchen Sie es erneut.')
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
    return <ProGate error={error}><div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg m-6">Fehler beim Laden des Dashboards.</div></ProGate>
  }

  if (result?.notEnabled) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title="DORA Dashboard"
          description="Digital Operational Resilience Act — Bereitschaftsübersicht"
        />
        <div className="flex-1 p-6">
          <div
            className="flex items-start gap-3 p-4 rounded-lg bg-yellow-500/10 border border-yellow-500/30"
            data-testid="dora-not-enabled-banner"
          >
            <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-primary">DORA ist noch nicht aktiviert</p>
              <p className="text-xs text-secondary mt-1">
                Aktivieren Sie das DORA-Framework, um das Dashboard zu nutzen.
              </p>
              <Link
                to="/secvitals/frameworks"
                className="text-xs text-brand underline mt-2 inline-block"
              >
                Zu den Frameworks →
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
      Keine Dashboard-Daten verfügbar.
    </div>
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="DORA Dashboard"
        description="Digital Operational Resilience Act — Bereitschaftsübersicht"
        actions={
          <div className="flex flex-col items-end gap-1">
            <Button variant="outline" size="sm" onClick={() => { setPdfError(null); void handleDownloadPDF() }}>
              <FileDown className="w-4 h-4 mr-2" />
              PDF exportieren
            </Button>
            {pdfError && <p className="text-xs text-red-500">{pdfError}</p>}
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {/* 5-Tile Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-4">

          {/* Tile 1: Bereitschaftsgrad */}
          <Card data-testid="tile-readiness">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <ShieldAlert className="w-4 h-4" />
                Bereitschaftsgrad
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
                  ? 'Gut'
                  : dashboard.readiness_pct >= 50
                    ? 'Mittel'
                    : 'Kritisch'}
              </div>
            </CardContent>
          </Card>

          {/* Tile 2: Offene Critical Controls */}
          <Card data-testid="tile-critical-controls">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <AlertTriangle className="w-4 h-4" />
                Kritische Controls
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
                {dashboard.open_critical_controls > 0 ? 'Offen' : 'Alle erfüllt'}
              </Badge>
            </CardContent>
          </Card>

          {/* Tile 3: Nächste Meldepflicht */}
          <Card data-testid="tile-next-deadline">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <Clock className="w-4 h-4" />
                Nächste Meldepflicht
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
                  <div className="text-sm text-green-500 font-medium">Keine offenen Fristen</div>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Tile 4: Abgelaufene Verträge */}
          <Card data-testid="tile-expired-suppliers">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <Building2 className="w-4 h-4" />
                Abgelaufene Verträge
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="text-3xl font-bold text-primary">{dashboard.expired_suppliers}</div>
              <Link to="/secvitals/suppliers" className="text-xs text-brand underline mt-1 inline-block">
                Zur Lieferantenliste →
              </Link>
            </CardContent>
          </Card>

          {/* Tile 5: TLPT-Status */}
          <Card data-testid="tile-tlpt-status">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium text-secondary flex items-center gap-2">
                <FlaskConical className="w-4 h-4" />
                TLPT-Status
              </CardTitle>
            </CardHeader>
            <CardContent>
              {dashboard.tlpt_overdue_warning ? (
                <Badge
                  variant="destructive"
                  className="text-xs"
                  data-testid="tlpt-warning-badge"
                >
                  Kein TLPT in 3 Jahren
                </Badge>
              ) : (
                <Badge variant="success" className="text-xs" data-testid="tlpt-ok-badge">
                  Aktuell
                </Badge>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
