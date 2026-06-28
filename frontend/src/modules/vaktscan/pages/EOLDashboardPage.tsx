import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PackageX, RefreshCw } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { PageHeader } from '../../../shared/components/PageHeader'
import { EmptyState } from '../../../shared/components/EmptyState'
import { ProGate } from '../../../shared/components/ProGate'
import { Badge } from '../../../components/ui/badge'
import { Button } from '../../../components/ui/button'
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '../../../components/ui/table'
import { useEOLDashboard } from '../hooks/useSBOM'
import type { ComponentSummary } from '../hooks/useSBOM'

/** Possible EOL status filter tabs. */
type FilterTab = 'all' | 'eol' | 'supported' | 'unknown'

/** Maps EOL status to a Badge variant for consistent colouring. */
const eolVariant: Record<
  ComponentSummary['eol_status'],
  React.ComponentProps<typeof Badge>['variant']
> = {
  supported: 'default',
  eol: 'destructive',
  unknown: 'secondary',
}

/** i18n key lookup for an EOL status value. */
const eolStatusKey: Record<ComponentSummary['eol_status'], string> = {
  supported: 'vaktscan.eolPage.statusSupported',
  eol: 'vaktscan.eolPage.statusEol',
  unknown: 'vaktscan.eolPage.statusUnknown',
}

/**
 * EOL Dashboard — shows all software components discovered via SBOM scans
 * and their end-of-life status as reported by endoflife.date.
 *
 * Use the "EOL-Scan aktualisieren" button to trigger a fresh Syft SBOM scan
 * for the currently selected asset (placeholder — asset selection is scoped
 * to the component level in AssetDetailPage; here we refresh all data).
 */
export default function EOLDashboardPage() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<FilterTab>('all')
  // Trigger SBOM is asset-scoped; we expose a global refresh here that re-fetches
  // the EOL dashboard query. For per-asset triggering, use the asset detail page.
  const { data, isLoading, error, refetch } = useEOLDashboard(false)

  const all = data?.data ?? []

  const filtered = (() => {
    if (activeTab === 'eol') return all.filter((c) => c.eol_status === 'eol')
    if (activeTab === 'supported') return all.filter((c) => c.eol_status === 'supported')
    if (activeTab === 'unknown') return all.filter((c) => c.eol_status === 'unknown')
    return all
  })()

  const eolCount = all.filter((c) => c.eol_status === 'eol').length
  const supportedCount = all.filter((c) => c.eol_status === 'supported').length
  const unknownCount = all.filter((c) => c.eol_status === 'unknown').length

  const tabs: { key: FilterTab; label: string; count: number }[] = [
    { key: 'all', label: t('vaktscan.eolPage.tabAll'), count: all.length },
    { key: 'eol', label: t('vaktscan.eolPage.statusEol'), count: eolCount },
    { key: 'supported', label: t('vaktscan.eolPage.statusSupported'), count: supportedCount },
    { key: 'unknown', label: t('vaktscan.eolPage.statusUnknown'), count: unknownCount },
  ]

  if (isLoading) {
    return (
      <div className="flex flex-col h-full">
        <PageHeader
          title="EOL-Dashboard"
          description={t('vaktscan.eolPage.description')}
        />
        <div className="flex justify-center py-16">
          <Spinner size="md" />
        </div>
      </div>
    )
  }

  return (
    <ProGate error={error}>
      <div className="flex flex-col h-full">
        <PageHeader
          title="EOL-Dashboard"
          description={t('vaktscan.eolPage.description')}
          actions={
            <Button
              variant="outline"
              size="sm"
              onClick={() => { void refetch(); }}
              disabled={isLoading}
            >
              <RefreshCw className={`w-4 h-4 mr-2 ${isLoading ? 'animate-spin' : ''}`} />
              {t('vaktscan.eolPage.refresh')}
            </Button>
          }
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
                  <span
                    className={`ml-1.5 text-xs px-1.5 py-0.5 rounded-full ${
                      activeTab === tab.key
                        ? 'bg-brand/10 text-brand'
                        : 'bg-surface2 text-secondary'
                    }`}
                  >
                    {tab.count}
                  </span>
                )}
              </button>
            ))}
          </div>

          {error && (
            <p className="text-sm text-red-600 p-4">{t('vaktscan.eolPage.error', { msg: (error).message })}</p>
          )}

          {!error && filtered.length === 0 && (
            <EmptyState
              icon={PackageX}
              title={t('vaktscan.eolPage.emptyTitle')}
              description={
                activeTab === 'all'
                  ? t('vaktscan.eolPage.emptyDescAll')
                  : t('vaktscan.eolPage.emptyDescFilter')
              }
              action={
                activeTab === 'all' ? (
                  <Button size="sm" onClick={() => { navigate('/vaktscan/assets'); }}>
                    {t('vaktscan.eolPage.configureAssets')}
                  </Button>
                ) : undefined
              }
            />
          )}

          {!error && filtered.length > 0 && (
            <div className="rounded-md border border-border bg-surface overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('common.name')}</TableHead>
                    <TableHead>{t('vaktscan.eolPage.colVersion')}</TableHead>
                    <TableHead>PURL</TableHead>
                    <TableHead>{t('vaktscan.eolPage.colEolStatus')}</TableHead>
                    <TableHead>{t('vaktscan.eolPage.colEolDate')}</TableHead>
                    <TableHead>{t('vaktscan.eolPage.colAssetId')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.map((comp) => (
                    <TableRow key={comp.id}>
                      <TableCell className="font-medium text-sm">{comp.name}</TableCell>
                      <TableCell className="text-sm tabular-nums text-secondary">
                        {comp.version}
                      </TableCell>
                      <TableCell className="text-xs text-secondary max-w-[200px] truncate">
                        {comp.purl ?? '—'}
                      </TableCell>
                      <TableCell>
                        <Badge variant={eolVariant[comp.eol_status]}>
                          {t(eolStatusKey[comp.eol_status])}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-sm tabular-nums text-secondary">
                        {comp.eol_date ?? '—'}
                      </TableCell>
                      <TableCell className="text-xs text-secondary font-mono">
                        {comp.asset_id}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </div>
      </div>
    </ProGate>
  )
}
