import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Clock, AlertTriangle } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../../../components/ui/table'
import { useSLADashboard } from '../hooks/useAssets'
import type { SLAEntry } from '../types'
import { findingSeverityVariant } from '../../../lib/statusMapping'

/** Active filter tab on the SLA dashboard. */
type FilterTab = 'all' | 'overdue' | 'at_risk'

const severityVariant = findingSeverityVariant

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
  const { t } = useTranslation()
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
    { key: 'all', label: t('vaktscan.slaPage.tabAll'), count: all.length },
    { key: 'overdue', label: t('vaktscan.slaPage.tabOverdue'), count: overdueCount },
    { key: 'at_risk', label: t('vaktscan.slaPage.tabAtRisk'), count: atRiskCount },
  ]

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="SLA-Dashboard"
        description={t('vaktscan.slaPage.description')}
      />

      <div className="flex-1 p-6 space-y-4">
        {/* Filter Tabs */}
        <div className="flex gap-1 border-b border-border">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => { setActiveTab(tab.key); }}
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
            <Spinner size="md" />
          </div>
        )}

        {error && (
          <p className="text-sm text-red-600 p-4">{t('vaktscan.slaPage.error', { msg: error.message })}</p>
        )}

        {!isLoading && !error && filtered.length === 0 && (
          <EmptyState
            icon={activeTab === 'overdue' ? AlertTriangle : Clock}
            title={activeTab === 'overdue' ? t('vaktscan.slaPage.emptyOverdueTitle') : t('vaktscan.slaPage.emptyFilterTitle')}
            description={
              activeTab === 'all'
                ? t('vaktscan.slaPage.emptyAllDesc')
                : activeTab === 'overdue'
                  ? t('vaktscan.slaPage.emptyOverdueDesc')
                  : t('vaktscan.slaPage.emptyAtRiskDesc')
            }
            action={
              activeTab === 'all' ? (
                <Button size="sm" onClick={() => { navigate('/vaktscan/assets'); }}>
                  {t('vaktscan.slaPage.showAssets')}
                </Button>
              ) : undefined
            }
          />
        )}

        {!isLoading && !error && filtered.length > 0 && (
          <div className="rounded-md border border-border bg-surface overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('vaktscan.slaPage.colAsset')}</TableHead>
                  <TableHead>{t('vaktscan.slaPage.colFinding')}</TableHead>
                  <TableHead>{t('vaktscan.slaPage.colSeverity')}</TableHead>
                  <TableHead>{t('common.status')}</TableHead>
                  <TableHead className="text-right">{t('vaktscan.slaPage.colOpenDays')}</TableHead>
                  <TableHead className="text-right">{t('vaktscan.slaPage.colSlaDays')}</TableHead>
                  <TableHead>{t('vaktscan.slaPage.colProgress')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.map((entry) => (
                  <TableRow
                    key={entry.finding_id}
                    className="cursor-pointer hover:bg-surface2"
                    onClick={() => { navigate(`/vaktscan/findings/${entry.finding_id}`); }}
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
