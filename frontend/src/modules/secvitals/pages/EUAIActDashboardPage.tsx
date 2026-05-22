import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Bot, Download, AlertTriangle, CheckCircle, FileText } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { apiFetch } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import { RISK_CLASS_CSS, RISK_CLASS_LABELS } from '../components/aiRiskClassConfig'
import { formatLocale } from '../../../shared/utils/locale'

interface EUAIActISOMappingEntry {
  eu_ai_act_article: string
  eu_ai_act_topic: string
  iso27001_control: string
  iso27001_title: string
}

interface EUAIActDashboard {
  total_systems: number
  systems_by_risk_class: Record<string, number>
  systems_by_status: Record<string, number>
  systems_without_documentation: number
  high_risk_deadline: string
  high_risk_deadline_days_left: number
  iso27001_mappings: EUAIActISOMappingEntry[]
}

const RISK_ORDER: Array<keyof typeof RISK_CLASS_LABELS> = ['unacceptable', 'high', 'limited', 'minimal']
const STATUS_LABELS: Record<string, string> = {
  under_review: 'In Prüfung',
  approved: 'Genehmigt',
  compliant: 'Konform',
  prohibited: 'Verboten',
  decommissioned: 'Stillgelegt',
}

export default function EUAIActDashboardPage() {
  const { data: dashboard, isLoading, isError, error } = useQuery<EUAIActDashboard>({
    queryKey: ['secvitals', 'eu-ai-act', 'dashboard'],
    queryFn: () => apiFetch<EUAIActDashboard>('/secvitals/eu-ai-act/dashboard'),
    staleTime: 5 * 60 * 1000,
  })
  const [downloadError, setDownloadError] = useState<string | null>(null)

  async function handleDownloadPDF() {
    try {
      const res = await fetch('/api/v1/secvitals/eu-ai-act/report-pdf', {
        credentials: 'include',
      })
      if (!res.ok) {
        setDownloadError('PDF konnte nicht erstellt werden. Bitte erneut versuchen.')
        return
      }
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'eu-ai-act-report.pdf'
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch {
      setDownloadError('PDF konnte nicht erstellt werden.')
    }
  }

  const daysLeft = dashboard?.high_risk_deadline_days_left ?? 0
  const deadlineColor = daysLeft < 90 ? 'text-red-400' : daysLeft < 180 ? 'text-orange-400' : 'text-green-400'

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="EU AI Act Dashboard"
        description="Compliance-Status Ihrer KI-Systeme nach EU-Verordnung 2024/1689."
        actions={
          <Button onClick={() => { setDownloadError(null); void handleDownloadPDF() }} data-testid="export-report-pdf-btn">
            <Download className="w-4 h-4 mr-1" />
            Compliance-Report PDF
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {downloadError && (
          <p className="text-sm text-red-400 bg-red-500/10 rounded-lg px-4 py-2">{downloadError}</p>
        )}
        {isLoading && (
          <div className="flex items-center justify-center h-48">
            <Spinner size="lg" color="primary" />
          </div>
        )}
        {isError && <ProGate error={error}><div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">Fehler beim Laden des Dashboards.</div></ProGate>}

        {dashboard && (
          <>
            {/* Top KPIs */}
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
              <Card data-testid="kpi-total-systems">
                <CardContent className="pt-5">
                  <p className="text-xs text-muted-foreground mb-1">KI-Systeme gesamt</p>
                  <p className="text-3xl font-bold">{dashboard.total_systems}</p>
                </CardContent>
              </Card>

              <Card data-testid="kpi-without-docs">
                <CardContent className="pt-5">
                  <div className="flex items-start gap-2">
                    <div>
                      <p className="text-xs text-muted-foreground mb-1">Ohne Dossier</p>
                      <p className="text-3xl font-bold">{dashboard.systems_without_documentation}</p>
                    </div>
                    {dashboard.systems_without_documentation > 0 && (
                      <AlertTriangle className="w-4 h-4 text-orange-400 mt-1" />
                    )}
                  </div>
                </CardContent>
              </Card>

              <Card data-testid="kpi-high-risk-count">
                <CardContent className="pt-5">
                  <p className="text-xs text-muted-foreground mb-1">Hochrisiko-Systeme</p>
                  <p className="text-3xl font-bold">{dashboard.systems_by_risk_class['high'] ?? 0}</p>
                </CardContent>
              </Card>

              <Card data-testid="kpi-deadline">
                <CardContent className="pt-5">
                  <p className="text-xs text-muted-foreground mb-1">Frist Hochrisiko</p>
                  <p className={`text-xl font-bold ${deadlineColor}`}>
                    {new Date(dashboard.high_risk_deadline).toLocaleDateString(formatLocale())}
                  </p>
                  <p className="text-xs text-muted-foreground mt-0.5">{daysLeft} Tage</p>
                </CardContent>
              </Card>
            </div>

            {/* Risk class breakdown */}
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    <Bot className="w-4 h-4" />
                    Systeme nach Risikoklasse
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2" data-testid="risk-class-breakdown">
                  {RISK_ORDER.map((rc) => {
                    const count = dashboard.systems_by_risk_class[rc] ?? 0
                    if (count === 0) return null
                    return (
                      <div key={rc} className="flex items-center justify-between">
                        <Badge className={RISK_CLASS_CSS[rc] ?? ''}>
                          {RISK_CLASS_LABELS[rc]}
                        </Badge>
                        <span className="font-semibold text-sm">{count}</span>
                      </div>
                    )
                  })}
                  {Object.values(dashboard.systems_by_risk_class).every((v) => v === 0) && (
                    <p className="text-sm text-muted-foreground">Keine klassifizierten Systeme</p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    <CheckCircle className="w-4 h-4" />
                    Systeme nach Status
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2" data-testid="status-breakdown">
                  {Object.entries(dashboard.systems_by_status).map(([status, count]) => (
                    <div key={status} className="flex items-center justify-between text-sm">
                      <span className="text-muted-foreground">{STATUS_LABELS[status] ?? status}</span>
                      <span className="font-semibold">{count}</span>
                    </div>
                  ))}
                </CardContent>
              </Card>
            </div>

            {/* ISO 27001 mapping table */}
            {dashboard.iso27001_mappings.length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm flex items-center gap-2">
                    <FileText className="w-4 h-4" />
                    Mapping: EU AI Act ↔ ISO 27001
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <p className="text-xs text-muted-foreground mb-3">
                    Diese Controls überschneiden sich — eine ISO-27001-konforme Organisation hat damit bereits wesentliche EU AI Act Anforderungen erfüllt.
                  </p>
                  <div className="overflow-x-auto" data-testid="iso-mapping-table">
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b border-border">
                          <th className="text-left py-2 pr-3 text-muted-foreground">Artikel</th>
                          <th className="text-left py-2 pr-3 text-muted-foreground">EU AI Act Anforderung</th>
                          <th className="text-left py-2 pr-3 text-muted-foreground">ISO 27001</th>
                          <th className="text-left py-2 text-muted-foreground">ISO Titel</th>
                        </tr>
                      </thead>
                      <tbody>
                        {dashboard.iso27001_mappings.map((m, i) => (
                          <tr key={i} className="border-b border-border/50">
                            <td className="py-1.5 pr-3 font-medium text-primary/70">{m.eu_ai_act_article}</td>
                            <td className="py-1.5 pr-3">{m.eu_ai_act_topic}</td>
                            <td className="py-1.5 pr-3 font-mono text-primary/70">{m.iso27001_control}</td>
                            <td className="py-1.5">{m.iso27001_title}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </CardContent>
              </Card>
            )}
          </>
        )}
      </div>
    </div>
  )
}
