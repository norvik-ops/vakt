import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { FileText, FileSearch, Handshake, AlertTriangle, ChevronRight, Clock, Users, Sparkles } from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { useVVT } from '../hooks/useVVT'
import { useDPIAs } from '../hooks/useDPIAs'
import { useAVVs } from '../hooks/useAVVs'
import { useBreaches } from '../hooks/useBreaches'
import { useDSRs } from '../hooks/useDSRs'
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

export default function SecPrivacyOverviewPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: vvt } = useVVT()
  const { data: dpias } = useDPIAs()
  const { data: avvs } = useAVVs()
  const { data: breaches } = useBreaches()
  const { data: dsrs } = useDSRs()

  const activeVVT = vvt?.filter((v) => v.status === 'active') ?? []
  const activeDPIAs = dpias?.filter((d) => d.status !== 'approved') ?? []
  const activeAVVs = avvs ?? []
  const expiredAVVs = activeAVVs.filter((a) => a.status === 'expired')

  const openBreaches = breaches?.filter((b) => b.status === 'open') ?? []
  const urgentBreaches = openBreaches.filter((b) => {
    const discovered = new Date(b.discovered_at)
    const hoursSince = (Date.now() - discovered.getTime()) / 36e5
    return hoursSince < 72
  })

  const openDSRs = dsrs?.filter((d) => d.status === 'open' || d.status === 'in_progress') ?? []
  const overdueDSRs = openDSRs.filter((d) => d.due_date && new Date(d.due_date) < new Date())

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vakt Privacy"
        description={t('vaktprivacy.overviewPage.description')}
      />

      <div className="flex-1 p-6 space-y-8">
        {/* 72h Alert */}
        {urgentBreaches.length > 0 && (
          <button
            onClick={() => { navigate('/vaktprivacy/breach'); }}
            className="w-full flex items-start gap-3 p-4 bg-red-500/10 border border-red-500/30 rounded-lg text-left hover:border-red-500/50 transition-colors"
          >
            <Clock className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
            <div>
              <p className="text-sm font-semibold text-red-500">
                {t(urgentBreaches.length > 1 ? 'vaktprivacy.overviewPage.urgentBreachAlertPlural' : 'vaktprivacy.overviewPage.urgentBreachAlert', { count: urgentBreaches.length })}
              </p>
              <p className="text-xs text-secondary mt-0.5">
                {t('vaktprivacy.overviewPage.urgentBreachDesc')}
              </p>
            </div>
          </button>
        )}

        {/* KPI Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
          <StatCard
            icon={FileText}
            label={t('vaktprivacy.overviewPage.statVVT')}
            value={activeVVT.length}
            sub={t('vaktprivacy.overviewPage.statVVTSub')}
            onClick={() => { navigate('/vaktprivacy/vvt'); }}
            accent={activeVVT.length > 0 ? 'green' : 'default'}
          />
          <StatCard
            icon={FileSearch}
            label={t('vaktprivacy.overviewPage.statDPIA')}
            value={dpias?.length ?? 0}
            sub={activeDPIAs.length > 0 ? `${String(activeDPIAs.length)} ${t('vaktprivacy.overviewPage.statDPIASubPending')}` : t('vaktprivacy.overviewPage.statDPIASubDone')}
            onClick={() => { navigate('/vaktprivacy/dpia'); }}
            accent={activeDPIAs.length > 0 ? 'yellow' : 'default'}
          />
          <StatCard
            icon={Handshake}
            label={t('vaktprivacy.overviewPage.statAVV')}
            value={activeAVVs.length}
            sub={expiredAVVs.length > 0 ? `${String(expiredAVVs.length)} ${t('vaktprivacy.overviewPage.statAVVSubExpired')}` : t('vaktprivacy.overviewPage.statAVVSubValid')}
            onClick={() => { navigate('/vaktprivacy/avv'); }}
            accent={expiredAVVs.length > 0 ? 'red' : 'default'}
          />
          <StatCard
            icon={AlertTriangle}
            label={t('vaktprivacy.overviewPage.statBreach')}
            value={openBreaches.length}
            sub={urgentBreaches.length > 0 ? `${urgentBreaches.length} ${t('vaktprivacy.overviewPage.statBreachSubUrgent')}` : t('vaktprivacy.overviewPage.statBreachSubNone')}
            onClick={() => { navigate('/vaktprivacy/breach'); }}
            accent={urgentBreaches.length > 0 ? 'red' : openBreaches.length > 0 ? 'yellow' : 'green'}
          />
          <StatCard
            icon={Users}
            label={t('vaktprivacy.overviewPage.statDSR')}
            value={openDSRs.length}
            sub={overdueDSRs.length > 0 ? `${overdueDSRs.length} ${t('vaktprivacy.overviewPage.statDSRSubOverdue')}` : t('vaktprivacy.overviewPage.statDSRSubNone')}
            onClick={() => { navigate('/vaktprivacy/dsr'); }}
            accent={overdueDSRs.length > 0 ? 'red' : openDSRs.length > 0 ? 'yellow' : 'green'}
          />
        </div>

        {/* Module Cards */}
        <div>
          <h2 className="text-sm font-semibold text-secondary uppercase tracking-wider mb-3">
            {t('vaktprivacy.overviewPage.sectionAreas')}
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[
              {
                icon: FileText,
                title: t('vaktprivacy.overviewPage.moduleVVTTitle'),
                desc: t('vaktprivacy.overviewPage.moduleVVTDesc'),
                path: '/vaktprivacy/vvt',
                pro: false,
              },
              {
                icon: FileSearch,
                title: t('vaktprivacy.overviewPage.moduleDSFATitle'),
                desc: t('vaktprivacy.overviewPage.moduleDSFADesc'),
                path: '/vaktprivacy/dpia',
                pro: true,
              },
              {
                icon: Handshake,
                title: t('vaktprivacy.overviewPage.moduleAVVTitle'),
                desc: t('vaktprivacy.overviewPage.moduleAVVDesc'),
                path: '/vaktprivacy/avv',
                pro: false,
              },
              {
                icon: AlertTriangle,
                title: t('vaktprivacy.overviewPage.moduleBreachTitle'),
                desc: t('vaktprivacy.overviewPage.moduleBreachDesc'),
                path: '/vaktprivacy/breach',
                pro: false,
              },
              {
                icon: Users,
                title: t('vaktprivacy.overviewPage.moduleDSRTitle'),
                desc: t('vaktprivacy.overviewPage.moduleDSRDesc'),
                path: '/vaktprivacy/dsr',
                pro: false,
              },
            ].map(({ icon: Icon, title, desc, path, pro }) => (
              <button
                key={path}
                onClick={() => { navigate(path); }}
                className="group flex items-start gap-4 p-4 bg-surface border border-border rounded-lg text-left hover:border-brand/50 transition-all duration-150"
              >
                <div className="p-2 rounded-lg bg-surface2 text-brand shrink-0">
                  <Icon className="w-4 h-4" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-primary group-hover:text-brand transition-colors">
                      {title}
                    </span>
                    {pro && (
                      <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-brand/10 text-brand border border-brand/20 shrink-0">
                        <Sparkles className="w-2.5 h-2.5" />
                        Pro
                      </span>
                    )}
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
