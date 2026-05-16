import { useNavigate } from 'react-router-dom'
import { FileText, FileSearch, Handshake, AlertTriangle, ChevronRight, Clock, Users } from 'lucide-react'
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
        title="SecPrivacy"
        description="DSGVO-Dokumentation: Verarbeitungen, DSFAs, Verträge und Datenpannen."
      />

      <div className="flex-1 p-6 space-y-8">
        {/* 72h Alert */}
        {urgentBreaches.length > 0 && (
          <button
            onClick={() => navigate('/secprivacy/breach')}
            className="w-full flex items-start gap-3 p-4 bg-red-500/10 border border-red-500/30 rounded-lg text-left hover:border-red-500/50 transition-colors"
          >
            <Clock className="w-5 h-5 text-red-500 shrink-0 mt-0.5" />
            <div>
              <p className="text-sm font-semibold text-red-500">
                {urgentBreaches.length} Datenpanne{urgentBreaches.length > 1 ? 'n' : ''} — 72h-Meldepflicht läuft
              </p>
              <p className="text-xs text-secondary mt-0.5">
                Datenpannen müssen innerhalb von 72h nach Bekanntwerden an die Aufsichtsbehörde gemeldet werden (Art. 33 DSGVO).
              </p>
            </div>
          </button>
        )}

        {/* KPI Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
          <StatCard
            icon={FileText}
            label="Verarbeitungen (VVT)"
            value={activeVVT.length}
            sub="aktive Einträge"
            onClick={() => navigate('/secprivacy/vvt')}
            accent={activeVVT.length > 0 ? 'green' : 'default'}
          />
          <StatCard
            icon={FileSearch}
            label="Datenschutz-Folgenabschätzungen"
            value={dpias?.length ?? 0}
            sub={activeDPIAs.length > 0 ? `${activeDPIAs.length} in Bearbeitung` : 'alle abgeschlossen'}
            onClick={() => navigate('/secprivacy/dpia')}
            accent={activeDPIAs.length > 0 ? 'yellow' : 'default'}
          />
          <StatCard
            icon={Handshake}
            label="AV-Verträge (AVV)"
            value={activeAVVs.length}
            sub={expiredAVVs.length > 0 ? `${expiredAVVs.length} abgelaufen` : 'alle gültig'}
            onClick={() => navigate('/secprivacy/avv')}
            accent={expiredAVVs.length > 0 ? 'red' : 'default'}
          />
          <StatCard
            icon={AlertTriangle}
            label="Datenpannen"
            value={openBreaches.length}
            sub={urgentBreaches.length > 0 ? `${urgentBreaches.length} innerhalb 72h` : 'keine aktiven'}
            onClick={() => navigate('/secprivacy/breach')}
            accent={urgentBreaches.length > 0 ? 'red' : openBreaches.length > 0 ? 'yellow' : 'green'}
          />
          <StatCard
            icon={Users}
            label="Datenschutzanfragen (DSR)"
            value={openDSRs.length}
            sub={overdueDSRs.length > 0 ? `${overdueDSRs.length} überfällig` : 'keine überfälligen'}
            onClick={() => navigate('/secprivacy/dsr')}
            accent={overdueDSRs.length > 0 ? 'red' : openDSRs.length > 0 ? 'yellow' : 'green'}
          />
        </div>

        {/* Module Cards */}
        <div>
          <h2 className="text-sm font-semibold text-secondary uppercase tracking-wider mb-3">
            Bereiche
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {[
              {
                icon: FileText,
                title: 'Verzeichnis von Verarbeitungstätigkeiten (VVT)',
                desc: 'Art. 30 DSGVO — Alle Datenverarbeitungen Ihrer Organisation dokumentieren.',
                path: '/secprivacy/vvt',
              },
              {
                icon: FileSearch,
                title: 'Datenschutz-Folgenabschätzung (DSFA)',
                desc: 'Art. 35 DSGVO — Risikoanalyse für hochriskante Verarbeitungen.',
                path: '/secprivacy/dpia',
              },
              {
                icon: Handshake,
                title: 'Auftragsverarbeiter-Verträge (AVV)',
                desc: 'Art. 28 DSGVO — Verträge mit Dienstleistern verwalten und Ablaufdaten überwachen.',
                path: '/secprivacy/avv',
              },
              {
                icon: AlertTriangle,
                title: 'Datenpannen-Register',
                desc: 'Art. 33/34 DSGVO — Datenpannen dokumentieren und die 72h-Meldepflicht einhalten.',
                path: '/secprivacy/breach',
              },
              {
                icon: Users,
                title: 'Datenschutzanfragen (DSR)',
                desc: 'Art. 15–21 DSGVO — Betroffenenrechte verwalten und die 30-Tage-Frist einhalten.',
                path: '/secprivacy/dsr',
              },
            ].map(({ icon: Icon, title, desc, path }) => (
              <button
                key={path}
                onClick={() => navigate(path)}
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
