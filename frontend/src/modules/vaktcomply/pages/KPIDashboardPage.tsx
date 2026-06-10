import { FileDown } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Button } from '../../../components/ui/button'
import { Spinner } from '../../../components/Spinner'
import { toast } from '../../../shared/hooks/useToast'
import { useKPIDashboard } from '../hooks/useKPIDashboard'
import type { KPISnapshot } from '../types'

// ── KPI configuration ──────────────────────────────────────────────────────

interface KPIConfig {
  key: keyof KPISnapshot
  label: string
  unit: string
  warn: number
  goal?: number
  higher_is_better: boolean
}

const kpiConfig: KPIConfig[] = [
  { key: 'kpi_compliance_score', label: 'Compliance-Score', unit: '%', warn: 70, goal: 85, higher_is_better: true },
  { key: 'kpi_open_critical_controls', label: 'Krit. offene Kontrollen', unit: '', warn: 10, higher_is_better: false },
  { key: 'kpi_open_high_risks', label: 'Offene Hochrisiken', unit: '', warn: 15, higher_is_better: false },
  { key: 'kpi_residual_risk_avg', label: 'Ø Residualrisiko', unit: '', warn: 8, higher_is_better: false },
  { key: 'kpi_open_incidents', label: 'Offene Incidents', unit: '', warn: 5, higher_is_better: false },
  { key: 'kpi_incident_mttr_days', label: 'MTTR Incidents', unit: ' Tage', warn: 30, higher_is_better: false },
  { key: 'kpi_evidence_coverage', label: 'Evidence-Abdeckung', unit: '%', warn: 60, goal: 80, higher_is_better: true },
  { key: 'kpi_expiring_evidence_count', label: 'Ablaufende Evidenz', unit: '', warn: 10, higher_is_better: false },
  { key: 'kpi_finding_sla_compliance', label: 'SLA-Einhaltung', unit: '%', warn: 80, higher_is_better: true },
  { key: 'kpi_open_major_ncs', label: 'Offene Major NCs', unit: '', warn: 3, higher_is_better: false },
  { key: 'kpi_suppliers_overdue_pct', label: 'Lieferanten überfällig', unit: '%', warn: 20, higher_is_better: false },
  { key: 'kpi_phishing_click_rate', label: 'Phishing Click-Rate', unit: '%', warn: 15, higher_is_better: false },
]

// ── Color logic ────────────────────────────────────────────────────────────

type StatusColor = 'green' | 'yellow' | 'red' | 'gray'

function getStatusColor(cfg: KPIConfig, value: number | undefined): StatusColor {
  if (value === undefined || value === null) return 'gray'
  if (cfg.higher_is_better) {
    const goal = cfg.goal ?? cfg.warn
    if (value >= goal) return 'green'
    if (value >= cfg.warn) return 'yellow'
    return 'red'
  } else {
    if (value <= cfg.warn) return 'green'
    if (value <= cfg.warn * 1.5) return 'yellow'
    return 'red'
  }
}

const colorClasses: Record<StatusColor, { border: string; text: string; badge: string }> = {
  green: {
    border: 'border-green-500/40',
    text: 'text-green-400',
    badge: 'bg-green-500/20 text-green-400',
  },
  yellow: {
    border: 'border-amber-500/40',
    text: 'text-amber-400',
    badge: 'bg-amber-500/20 text-amber-400',
  },
  red: {
    border: 'border-red-500/40',
    text: 'text-red-400',
    badge: 'bg-red-500/20 text-red-400',
  },
  gray: {
    border: 'border-border',
    text: 'text-muted-foreground',
    badge: 'bg-muted text-muted-foreground',
  },
}

// ── KPI Card ───────────────────────────────────────────────────────────────

function KPICard({ cfg, snapshot }: { cfg: KPIConfig; snapshot: KPISnapshot | undefined }) {
  const raw = snapshot ? (snapshot[cfg.key] as number | undefined) : undefined
  const status = getStatusColor(cfg, raw)
  const cls = colorClasses[status]

  const displayValue =
    raw !== undefined && raw !== null
      ? `${raw % 1 === 0 ? raw.toString() : raw.toFixed(1)}${cfg.unit}`
      : 'N/A'

  const statusLabel: Record<StatusColor, string> = {
    green: 'Gut',
    yellow: 'Warnung',
    red: 'Kritisch',
    gray: 'Keine Daten',
  }

  return (
    <Card className={`border ${cls.border}`}>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{cfg.label}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className={`text-2xl font-bold ${cls.text}`}>{displayValue}</div>
        <span className={`mt-1 inline-block rounded-full px-2 py-0.5 text-xs font-medium ${cls.badge}`}>
          {statusLabel[status]}
        </span>
      </CardContent>
    </Card>
  )
}

// ── Main Page ──────────────────────────────────────────────────────────────

export default function KPIDashboardPage() {
  const { data, isLoading, isError } = useKPIDashboard()

  function handleExportPDF() {
    toast('PDF-Export demnächst verfügbar', 'info')
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Spinner />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="p-6 text-red-400">
        Fehler beim Laden des ISMS KPI-Dashboards.
      </div>
    )
  }

  const current = data?.current

  return (
    <div className="space-y-6">
      <PageHeader
        title="ISMS KPI-Dashboard"
        description="Überblick über die 12 wichtigsten ISMS-Kennzahlen für ISO 27001-Auditbereitschaft."
        actions={
          <Button variant="outline" size="sm" onClick={handleExportPDF}>
            <FileDown className="mr-2 h-4 w-4" />
            KPI-Report exportieren
          </Button>
        }
      />

      {current ? (
        <p className="text-sm text-muted-foreground">
          Letztes Update: {current.snapshot_date} (täglich 06:00 Uhr)
        </p>
      ) : (
        <p className="text-sm text-muted-foreground">
          Noch keine Snapshots vorhanden. Der erste Snapshot wird täglich um 06:00 Uhr berechnet.
        </p>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {kpiConfig.map((cfg) => (
          <KPICard key={cfg.key} cfg={cfg} snapshot={current} />
        ))}
      </div>
    </div>
  )
}
