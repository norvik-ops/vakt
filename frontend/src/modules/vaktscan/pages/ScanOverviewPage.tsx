import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Bug, ShieldAlert, Clock, BarChart2, PackageX, Lock,
  Server, ChevronRight, CheckCircle2, AlertTriangle,
} from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { useAssets } from '../hooks/useAssets'
import { useFindings } from '../hooks/useFindings'
import { useScannerStatus } from '../hooks/useScannerStatus'
import { cn } from '../../../lib/utils'

interface StatCardProps {
  icon: React.ElementType
  label: string
  value: number | string
  sub?: string
  onClick: () => void
  accent?: 'default' | 'red' | 'yellow' | 'green'
}

function StatCard({ icon: Icon, label, value, sub, onClick, accent = 'default' }: StatCardProps) {
  const accentColors = {
    default: 'text-brand',
    red: 'text-red-500',
    yellow: 'text-yellow-500',
    green: 'text-green-500',
  }
  return (
    <button
      onClick={onClick}
      className="group flex flex-col gap-3 p-5 bg-surface border border-border rounded-xl text-left hover:border-brand/50 transition-all duration-150"
    >
      <div className="flex items-center justify-between">
        <div className={cn('p-2 rounded-lg bg-surface2', accentColors[accent])}>
          <Icon className="w-5 h-5" />
        </div>
        <ChevronRight className="w-4 h-4 text-secondary opacity-0 group-hover:opacity-100 transition-opacity" />
      </div>
      <div>
        <div className={cn('text-2xl font-bold', accentColors[accent])}>{value}</div>
        <div className="text-sm font-medium text-primary mt-0.5">{label}</div>
        {sub && <div className="text-xs text-secondary mt-0.5">{sub}</div>}
      </div>
    </button>
  )
}

export default function ScanOverviewPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: assets } = useAssets(1, 200)
  const { data: findings } = useFindings({ status: 'open' }, 1, 200)
  const { data: scannerStatus } = useScannerStatus()

  const allAssets = assets ?? []
  const criticalAssets = allAssets.filter((a) => a.criticality === 'critical')

  const openFindings = findings?.data ?? []
  const criticalFindings = openFindings.filter((f) => f.severity === 'critical')

  const scannerConfigured = scannerStatus
    ? scannerStatus.trivy || scannerStatus.nuclei || scannerStatus.openvas
    : null

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vakt Scan"
        description={t('vaktscan.overviewPage.description')}
      />

      <div className="flex-1 p-6 space-y-8">
        {/* Scanner warning */}
        {scannerConfigured === false && (
          <button
            onClick={() => { navigate('/settings?tab=scanner'); }}
            className="w-full flex items-start gap-3 p-4 bg-amber-500/10 border border-amber-500/30 rounded-lg text-left hover:border-amber-500/50 transition-colors"
          >
            <AlertTriangle className="w-5 h-5 text-amber-500 shrink-0 mt-0.5" />
            <div>
              <p className="text-sm font-semibold text-amber-600 dark:text-amber-400">
                {t('vaktscan.overviewPage.noScannerTitle')}
              </p>
              <p className="text-xs text-secondary mt-0.5">
                {t('vaktscan.overviewPage.noScannerDesc')}
              </p>
            </div>
          </button>
        )}

        {scannerConfigured === true && (
          <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
            <CheckCircle2 className="w-4 h-4 shrink-0" />
            <span>{t('vaktscan.overviewPage.scannerReady')}</span>
          </div>
        )}

        {/* KPI Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard
            icon={Server}
            label={t('vaktscan.overviewPage.statAssets')}
            value={allAssets.length}
            sub={allAssets.length > 0 ? t('vaktscan.overviewPage.statAssetsCriticalSub', { count: criticalAssets.length }) : t('vaktscan.overviewPage.statAssetsNoSub')}
            onClick={() => { navigate('/vaktscan/assets'); }}
            accent={allAssets.length > 0 ? 'green' : 'default'}
          />
          <StatCard
            icon={ShieldAlert}
            label={t('vaktscan.overviewPage.statCriticalAssets')}
            value={criticalAssets.length}
            sub={t('vaktscan.overviewPage.statCriticalAssetsSub')}
            onClick={() => { navigate('/vaktscan/assets'); }}
            accent={criticalAssets.length > 0 ? 'red' : 'green'}
          />
          <StatCard
            icon={Bug}
            label={t('vaktscan.overviewPage.statCriticalFindings')}
            value={criticalFindings.length}
            sub={t('vaktscan.overviewPage.statCriticalFindingsSub')}
            onClick={() => { navigate('/vaktscan/findings'); }}
            accent={criticalFindings.length > 0 ? 'red' : 'green'}
          />
          <StatCard
            icon={AlertTriangle}
            label={t('vaktscan.overviewPage.statOpenFindings')}
            value={openFindings.length}
            sub={criticalFindings.length > 0 ? t('vaktscan.overviewPage.statOpenFindingsCritical', { count: criticalFindings.length }) : t('vaktscan.overviewPage.statOpenFindingsNoCritical')}
            onClick={() => { navigate('/vaktscan/findings'); }}
            accent={criticalFindings.length > 0 ? 'red' : openFindings.length > 0 ? 'yellow' : 'green'}
          />
        </div>

        {/* Bereiche */}
        <div>
          <h2 className="text-sm font-semibold text-secondary uppercase tracking-wider mb-3">
            {t('vaktscan.overviewPage.sections')}
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {[
              {
                icon: Server,
                title: 'Assets',
                desc: t('vaktscan.overviewPage.areaAssetsDesc'),
                path: '/vaktscan/assets',
              },
              {
                icon: Bug,
                title: 'Findings',
                desc: t('vaktscan.overviewPage.areaFindingsDesc'),
                path: '/vaktscan/findings',
              },
              {
                icon: Clock,
                title: 'SLA-Dashboard',
                desc: t('vaktscan.overviewPage.areaSlaDesc'),
                path: '/vaktscan/sla',
              },
              {
                icon: BarChart2,
                title: t('vaktscan.overviewPage.areaReports'),
                desc: t('vaktscan.overviewPage.areaReportsDesc'),
                path: '/vaktscan/reports',
              },
              {
                icon: PackageX,
                title: 'EOL-Dashboard',
                desc: t('vaktscan.overviewPage.areaEolDesc'),
                path: '/vaktscan/eol',
              },
              {
                icon: Lock,
                title: t('vaktscan.overviewPage.areaCerts'),
                desc: t('vaktscan.overviewPage.areaCertsDesc'),
                path: '/vaktscan/certificates',
              },
            ].map(({ icon: Icon, title, desc, path }) => (
              <button
                key={path}
                onClick={() => { navigate(path); }}
                className="group flex items-start gap-4 p-4 bg-surface border border-border rounded-lg text-left hover:border-brand/50 transition-all duration-150"
              >
                <div className="p-2 rounded-lg bg-surface2 text-brand shrink-0">
                  <Icon className="w-4 h-4" />
                </div>
                <div className="min-w-0">
                  <div className="text-sm font-medium text-primary group-hover:text-brand transition-colors">
                    {title}
                  </div>
                  <div className="text-xs text-secondary mt-0.5 leading-relaxed">{desc}</div>
                </div>
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
