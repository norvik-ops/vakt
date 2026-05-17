import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Clock, AlertTriangle } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Badge } from '../../../components/ui/badge'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useSLADashboard } from '../hooks/useAssets'
import type { SLAEntry } from '../types'

/** Active filter tab on the SLA dashboard. */
type FilterTab = 'all' | 'overdue' | 'at_risk'

/** Maps finding severity to the shadcn Badge variant for consistent colouring. */
const severityVariant: Record<SLAEntry['severity'], React.ComponentProps<typeof Badge>['variant']> = {
  info: 'secondary',
  low: 'outline',
  medium: 'warning',
  high: 'outline',
  critical: 'destructive',
}

/**
 * Computes the percentage of SLA time consumed for a finding.
 * Returns 100 when `sla_days` is 0 (no SLA configured) to treat it as immediately
 * overdue. The result is capped at 999 so overdue rows still display a meaningful number.
 */
function slaPercent(entry: SLAEntry): number {
  if (entry.sla_days === 0) return 100
  return Math.min(Math.round((entry.days_open / entry.sla_days) * 100), 999)
}

/**
 * Horizontal progress bar visualising SLA consumption for a single finding.
 * Colour thresholds: green below 50%, amber 50–90%, red above 90% or already overdue.
 */
function ProgressBar({ entry }: { entry: SLAEntry }) {
  const pct = slaPercent(entry)
  const clampedPct = Math.min(pct, 100)

  let barColor = 'bg-green-500'
  if (entry.overdue || pct > 90) {
    barColor = 'bg-red-500'
  } else if (pct >= 50) {
    barColor = 'bg-yellow-500'
  }

  return (
    <div className="flex items-center gap-2 min-w-[100px]">
      <div className="flex-1 h-1.5 rounded-full bg-border overflow-hidden">
        <div
          className={`h-full rounded-full transition-all ${barColor}`}
          style={{ width: `${clampedPct}%` }}
        />
      </div>
      <span className={`text-xs tabular-nums shrink-0 ${entry.overdue ? 'text-red-500 font-semibold' : 'text-secondary'}`}>
        {pct}%
      </span>
    </div>
  )
}

/**
 * Dashboard page listing all open SecPulse findings alongside their SLA progress bars.
 *
 * Three filter tabs segment findings: "Alle" (all open), "Überfällig" (past SLA deadline),
 * and "Gefährdet" (≥ 50% of SLA time consumed but not yet overdue). Clicking a row
 * navigates to the individual finding detail page.
 */
export default function SLADashboardPage() {
  const navigate = useNavigate()
  const { data: entries, isLoading, error } = useSLADashboard()
  const [activeTab, setActiveTab] = useState<FilterTab>('all')

  const all = entries ?? []

  const filtered = (() => {
    if (activeTab === 'overdue') return all.filter((e) => e.overdue)
    if (activeTab === 'at_risk') return all.filter((e) => !e.overdue && slaPercent(e) >= 50)
    return all
  })()

  const overdueCount = all.filter((e) => e.overdue).length
  const atRiskCount = all.filter((e) => !e.overdue && slaPercent(e) >= 50).length

  const tabs: { key: FilterTab; label: string; count: number }[] = [
    { key: 'all', label: 'Alle', count: all.length },
    { key: 'overdue', label: 'Überfällig', count: overdueCount },
    { key: 'at_risk', label: 'Gefährdet (>50%)', count: atRiskCount },
  ]

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="SLA-Dashboard"
        description="Überfällige und gefährdete Findings nach SLA."
      />

      <div className="flex-1 p-6 space-y-4">
        {/* Filter Tabs */}
        <div className="flex gap-1 border-b border-border">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
                activeTab === tab.key
                  ? 'border-brand text-brand'
                  : 'border-transparent text-secondary hover:text-primary'
              }`}
            >
              {tab.label}
              {tab.count > 0 && (
                <span className={`ml-1.5 text-xs px-1.5 py-0.5 rounded-full ${
                  activeTab === tab.key ? 'bg-brand/10 text-brand' : 'bg-surface2 text-secondary'
                }`}>
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>

        {isLoading && (
          <div className="flex justify-center py-16">
            <div className="w-5 h-5 border-2 border-brand border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {error && (
          <p className="text-sm text-red-600 p-4">Fehler: {error.message}</p>
        )}

        {!isLoading && !error && filtered.length === 0 && (
          <EmptyState
            icon={activeTab === 'overdue' ? AlertTriangle : Clock}
            title={activeTab === 'overdue' ? 'Keine überfälligen Findings' : 'Keine Findings in diesem Filter'}
            description={
              activeTab === 'all'
                ? 'Alle Findings liegen im SLA-Rahmen oder es gibt keine offenen Findings.'
                : activeTab === 'overdue'
                  ? 'Alle offenen Findings befinden sich noch im SLA-Zeitfenster.'
                  : 'Keine Findings haben mehr als 50% ihrer SLA-Zeit verbraucht.'
            }
          />
        )}

        {!isLoading && !error && filtered.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Asset</TableHead>
                  <TableHead>Finding</TableHead>
                  <TableHead>Severity</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Offen (Tage)</TableHead>
                  <TableHead className="text-right">SLA (Tage)</TableHead>
                  <TableHead>Fortschritt</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((entry) => (
                  <TableRow
                    key={entry.finding_id}
                    className="cursor-pointer hover:bg-surface2"
                    onClick={() => navigate(`/secpulse/findings/${entry.finding_id}`)}
                  >
                    <TableCell className="text-sm text-secondary">{entry.asset_name}</TableCell>
                    <TableCell>
                      <span className="font-medium text-sm">{entry.finding_title}</span>
                    </TableCell>
                    <TableCell>
                      <Badge variant={severityVariant[entry.severity]}>{entry.severity}</Badge>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-secondary">{entry.status}</span>
                    </TableCell>
                    <TableCell className="text-right tabular-nums text-sm">
                      {entry.overdue ? (
                        <span className="text-red-500 font-semibold">{entry.days_open}</span>
                      ) : (
                        entry.days_open
                      )}
                    </TableCell>
                    <TableCell className="text-right tabular-nums text-sm text-secondary">
                      {entry.sla_days}
                    </TableCell>
                    <TableCell>
                      <ProgressBar entry={entry} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
    </div>
  )
}
