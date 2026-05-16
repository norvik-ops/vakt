import { useNavigate, Link } from 'react-router-dom'
import {
  ShieldCheck, ShieldAlert, Siren, BookOpen, ClipboardList,
  ChevronRight, TrendingUp, Shield, FileText,
} from 'lucide-react'
import { PageHeader } from '../../../shared/components/PageHeader'
import { Button } from '../../../components/ui/button'
import { useFrameworks, useTISAXReport } from '../hooks/useFrameworks'
import { useRisks } from '../hooks/useRisks'
import { useIncidents } from '../hooks/useIncidents'
import { usePolicies } from '../hooks/usePolicies'
import { useAuditRecords } from '../hooks/useAudits'
import { useDORADashboard } from '../hooks/useDORADashboard'
import { useAIStatus } from '../hooks/useAIAdvisor'
import { useAuditReport } from '../hooks/useAuditReport'
import { FeatureLockedError } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import { cn } from '../../../lib/utils'
import { ExpiringEvidenceWidget } from '../components/ExpiringEvidenceWidget'
import { AIAdvisor } from '../components/AIAdvisor'

// ── TISAX tile inner component ───────────────────────────────────────────────
function TISAXTile({ tisaxFrameworkId }: { tisaxFrameworkId: string }) {
  const { data: report } = useTISAXReport(tisaxFrameworkId)
  const readinessPct = report?.tisax_maturity?.readiness_percent

  return (
    <Link
      to={`/secvitals/frameworks/${tisaxFrameworkId}/tisax`}
      className="group flex items-start gap-4 p-4 bg-surface border border-border rounded-lg text-left hover:border-brand/50 transition-all duration-150"
      data-testid="tisax-tile"
    >
      <div className="p-2 rounded-lg bg-surface2 text-brand shrink-0">
        <Shield className="w-4 h-4" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-primary group-hover:text-brand transition-colors">
          TISAX® Label-Vorbereitung
        </div>
        <div className="text-xs text-secondary mt-0.5 leading-relaxed">
          VDA ISA Bereitschaftsgrad und Label-Assessment verwalten.
        </div>
        {readinessPct !== undefined && (
          <div className={cn(
            'text-xs font-medium mt-1',
            readinessPct >= 80 ? 'text-green-500' :
            readinessPct >= 50 ? 'text-yellow-500' : 'text-red-500',
          )}>
            {readinessPct.toFixed(0)}% Bereitschaft
          </div>
        )}
      </div>
    </Link>
  )
}

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

export default function SecVitalsOverviewPage() {
  const navigate = useNavigate()
  const { data: frameworks } = useFrameworks()
  const { data: aiStatus } = useAIStatus()
  const { generate: generateAuditReport, isGenerating: isGeneratingReport, error: reportError } = useAuditReport()
  const tisaxFramework = frameworks?.find((f) => f.name === 'TISAX')
  const { data: risks } = useRisks()
  const { data: incidents } = useIncidents()
  const { data: policies } = usePolicies()
  const { data: audits } = useAuditRecords()
  const { data: doraResult } = useDORADashboard()

  const openRisks = risks?.filter((r) => r.status === 'open') ?? []
  const highRisks = openRisks.filter((r) => r.risk_score >= 12)
  const openIncidents = incidents?.filter((i) => i.status === 'open' || i.status === 'investigating') ?? []
  const criticalIncidents = openIncidents.filter((i) => i.severity === 'critical' || i.severity === 'high')

  const today = new Date()
  const overdueReviews = policies?.filter((p) => {
    if (!p.review_date) return false
    return new Date(p.review_date) < today && p.status === 'active'
  }) ?? []

  const plannedAudits = audits?.filter((a) => a.status === 'planned' || a.status === 'in_progress') ?? []

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vakt Comply"
        description="Compliance, Risiken & Governance auf einen Blick."
        actions={
          <div className="flex flex-col items-end gap-1">
            <Button
              size="sm"
              onClick={generateAuditReport}
              disabled={isGeneratingReport}
              className="h-8 text-xs gap-1.5"
            >
              {isGeneratingReport ? (
                <>
                  <div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
                  Wird erstellt…
                </>
              ) : (
                <>
                  <FileText className="w-3.5 h-3.5" />
                  Audit-Bericht generieren
                </>
              )}
            </Button>
            <ProGate error={reportError instanceof FeatureLockedError ? reportError : null}>{''}</ProGate>
            {reportError instanceof Error && !(reportError instanceof FeatureLockedError) && (
              <p className="text-[10px] text-red-500">{reportError.message}</p>
            )}
          </div>
        }
      />

      <div className="flex-1 p-6 space-y-8">
        {/* KPI Grid */}
        <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
          <StatCard
            icon={ShieldCheck}
            label="Frameworks"
            value={frameworks?.length ?? 0}
            sub="aktiviert"
            onClick={() => navigate('/secvitals/frameworks')}
            accent={frameworks?.length ? 'green' : 'default'}
          />
          <StatCard
            icon={ShieldAlert}
            label="Offene Risiken"
            value={openRisks.length}
            sub={highRisks.length > 0 ? `${highRisks.length} kritisch/hoch` : 'keine kritischen'}
            onClick={() => navigate('/secvitals/risks')}
            accent={highRisks.length > 0 ? 'red' : openRisks.length > 0 ? 'yellow' : 'green'}
          />
          <StatCard
            icon={Siren}
            label="Offene Vorfälle"
            value={openIncidents.length}
            sub={criticalIncidents.length > 0 ? `${criticalIncidents.length} kritisch` : 'keine kritischen'}
            onClick={() => navigate('/secvitals/incidents')}
            accent={criticalIncidents.length > 0 ? 'red' : openIncidents.length > 0 ? 'yellow' : 'green'}
          />
          <StatCard
            icon={BookOpen}
            label="Richtlinien"
            value={policies?.length ?? 0}
            sub={overdueReviews.length > 0 ? `${overdueReviews.length} überfällig` : 'alle aktuell'}
            onClick={() => navigate('/secvitals/policies')}
            accent={overdueReviews.length > 0 ? 'yellow' : 'default'}
          />
          <StatCard
            icon={ClipboardList}
            label="Audits"
            value={plannedAudits.length}
            sub="aktiv / geplant"
            onClick={() => navigate('/secvitals/audits')}
          />
        </div>

        {/* Expiring Evidence Alert Widget */}
        <ExpiringEvidenceWidget />

        {/* KI-Compliance-Berater */}
        <AIAdvisor aiAvailable={aiStatus?.available ?? false} />

        {/* Quick Actions */}
        <div>
          <h2 className="text-sm font-semibold text-secondary uppercase tracking-wider mb-3">
            Bereiche
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {[
              {
                icon: ShieldCheck,
                title: 'Compliance Frameworks',
                desc: 'NIS2, ISO 27001, BSI-Grundschutz aktivieren und Controls verwalten.',
                path: '/secvitals/frameworks',
              },
              {
                icon: ShieldAlert,
                title: 'Risikoregister',
                desc: 'Risiken bewerten, priorisieren und Behandlungsmaßnahmen festhalten.',
                path: '/secvitals/risks',
              },
              {
                icon: Siren,
                title: 'Vorfallsregister',
                desc: 'Sicherheitsvorfälle dokumentieren und nachverfolgen.',
                path: '/secvitals/incidents',
              },
              {
                icon: BookOpen,
                title: 'Richtlinienverwaltung',
                desc: 'Sicherheitsrichtlinien erstellen, versionieren und Überprüfungen planen.',
                path: '/secvitals/policies',
              },
              {
                icon: ClipboardList,
                title: 'Interne Audits',
                desc: 'Interne Auditberichte dokumentieren und Maßnahmen verfolgen.',
                path: '/secvitals/audits',
              },
              {
                icon: TrendingUp,
                title: 'Bereitschafts-Score',
                desc: 'Gesamtbild des Compliance-Reifegrads über alle Frameworks.',
                path: '/secvitals/frameworks',
              },
            ].map(({ icon: Icon, title, desc, path }) => (
              <button
                key={path + title}
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

            {/* DORA Dashboard Tile */}
            <Link
              to="/secvitals/dora/dashboard"
              className="group flex items-start gap-4 p-4 bg-surface border border-border rounded-lg text-left hover:border-brand/50 transition-all duration-150"
            >
              <div className="p-2 rounded-lg bg-surface2 text-brand shrink-0">
                <Shield className="w-4 h-4" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-primary group-hover:text-brand transition-colors">
                  DORA Dashboard
                </div>
                <div className="text-xs text-secondary mt-0.5 leading-relaxed">
                  DORA-Bereitschaft, Meldepflichten und Resilienztests im Überblick.
                </div>
                {doraResult && !doraResult.notEnabled && doraResult.data && (
                  <div className={cn(
                    'text-xs font-medium mt-1',
                    doraResult.data.readiness_pct >= 80 ? 'text-green-500' :
                    doraResult.data.readiness_pct >= 50 ? 'text-yellow-500' : 'text-red-500',
                  )}>
                    {doraResult.data.readiness_pct.toFixed(0)}% Bereitschaft
                  </div>
                )}
                {doraResult?.notEnabled && (
                  <div className="text-xs text-secondary mt-1 italic">DORA nicht aktiviert</div>
                )}
              </div>
            </Link>

            {/* TISAX Tile — only shown when TISAX framework is enabled */}
            {tisaxFramework && (
              <TISAXTile tisaxFrameworkId={tisaxFramework.id} />
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
