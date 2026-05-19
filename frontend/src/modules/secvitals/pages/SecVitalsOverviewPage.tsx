import { useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import {
  ShieldCheck, ShieldAlert, Siren, BookOpen, ClipboardList,
  ChevronRight, TrendingUp, Shield, FileText, DownloadCloud,
  Settings, ArrowUp, ArrowDown, RotateCcw,
} from 'lucide-react'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer,
} from 'recharts'
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
import { useScoreHistory } from '../hooks/useScoreHistory'
import { FeatureLockedError, getAuthToken } from '../../../api/client'
import { ProGate } from '../../../shared/components/ProGate'
import { cn } from '../../../lib/utils'
import { ExpiringEvidenceWidget } from '../components/ExpiringEvidenceWidget'
import { AIAdvisor } from '../components/AIAdvisor'

// ── Item 59: Score Trend Chart ────────────────────────────────────────────────
function ScoreTrendChart({ data }: { data: Array<{ date: string; score: number }> }) {
  if (data.length < 2) return null

  const formatted = data.map((d) => ({
    ...d,
    label: new Date(d.date).toLocaleDateString('de-DE', { month: 'short', day: 'numeric' }),
  }))

  return (
    <div className="bg-white border border-border rounded-lg p-4">
      <h3 className="text-sm font-semibold text-gray-700 mb-3">Compliance-Score Verlauf</h3>
      <ResponsiveContainer width="100%" height={160}>
        <LineChart data={formatted}>
          <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
          <XAxis dataKey="label" tick={{ fontSize: 11 }} />
          <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} unit="%" />
          <Tooltip formatter={(v: number) => [`${v}%`, 'Score']} />
          <Line
            type="monotone"
            dataKey="score"
            stroke="#3b82f6"
            strokeWidth={2}
            dot={{ r: 3 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

// ── Item 64: Score Benchmark Widget ──────────────────────────────────────────
function ScoreBenchmark({ currentScore }: { currentScore: number }) {
  const benchmarks = [
    { label: 'Typischer KMU-Einstieg', score: 30 },
    { label: 'Nach 6 Monaten aktiver Nutzung', score: 60 },
    { label: 'ISO 27001 zertifiziert', score: 85 },
  ]

  return (
    <div className="bg-white border border-border rounded-lg p-4">
      <h3 className="text-sm font-semibold text-gray-700 mb-1">Score-Orientierung</h3>
      <p className="text-xs text-gray-400 mb-3">Ungefähre Richtwerte zur Einordnung Ihres Fortschritts</p>
      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-500 w-44 flex-shrink-0">Ihr aktueller Score</span>
          <div className="flex-1 bg-gray-100 rounded-full h-3">
            <div
              className="h-3 rounded-full bg-blue-500 transition-all"
              style={{ width: `${currentScore}%` }}
            />
          </div>
          <span className="text-xs font-bold w-10 text-right">{currentScore}%</span>
        </div>
        {benchmarks.map((b) => {
          const diff = currentScore - b.score
          return (
            <div key={b.label} className="flex items-center gap-3">
              <span className="text-xs text-gray-400 w-44 flex-shrink-0">{b.label}</span>
              <div className="flex-1 bg-gray-100 rounded-full h-3">
                <div
                  className="h-3 rounded-full bg-gray-300 transition-all"
                  style={{ width: `${b.score}%` }}
                />
              </div>
              <span
                className={`text-xs font-semibold w-10 text-right ${diff >= 0 ? 'text-green-600' : 'text-red-500'}`}
              >
                {b.score}%
              </span>
            </div>
          )
        })}
      </div>
      <p className="text-xs text-gray-400 mt-3">
        * Richtwerte zur Orientierung — keine verifizierten Branchen-Benchmarks.
      </p>
    </div>
  )
}

// ── Item 61: Dashboard Widget Ordering ───────────────────────────────────────
const DEFAULT_ORDER = ['kpis', 'evidence', 'advisor', 'trend', 'benchmark', 'areas']

const WIDGET_LABELS: Record<string, string> = {
  kpis: 'KPI-Übersicht',
  evidence: 'Ablaufende Nachweise',
  advisor: 'KI-Berater',
  trend: 'Score-Verlauf',
  benchmark: 'Score-Vergleich',
  areas: 'Bereiche',
}

function useDashboardOrder() {
  const [order, setOrder] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('vakt_dashboard_order')
      if (saved) {
        const parsed = JSON.parse(saved) as string[]
        // Merge: keep saved order but add any new default widgets
        const merged = parsed.filter((id) => DEFAULT_ORDER.includes(id))
        const missing = DEFAULT_ORDER.filter((id) => !merged.includes(id))
        return [...merged, ...missing]
      }
    } catch {
      // ignore
    }
    return DEFAULT_ORDER
  })

  function moveUp(index: number) {
    if (index === 0) return
    const next = [...order]
    ;[next[index - 1], next[index]] = [next[index], next[index - 1]]
    setOrder(next)
    localStorage.setItem('vakt_dashboard_order', JSON.stringify(next))
  }

  function moveDown(index: number) {
    if (index === order.length - 1) return
    const next = [...order]
    ;[next[index], next[index + 1]] = [next[index + 1], next[index]]
    setOrder(next)
    localStorage.setItem('vakt_dashboard_order', JSON.stringify(next))
  }

  function reset() {
    setOrder(DEFAULT_ORDER)
    localStorage.removeItem('vakt_dashboard_order')
  }

  return { order, moveUp, moveDown, reset }
}

function DashboardOrderPopover({
  order,
  moveUp,
  moveDown,
  reset,
  onClose,
}: {
  order: string[]
  moveUp: (i: number) => void
  moveDown: (i: number) => void
  reset: () => void
  onClose: () => void
}) {
  return (
    <div className="absolute right-0 top-10 z-50 w-64 rounded-lg border border-border bg-white shadow-lg p-3">
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-semibold text-gray-700">Widgets anpassen</span>
        <button
          onClick={reset}
          className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600"
          title="Zurücksetzen"
        >
          <RotateCcw className="w-3 h-3" />
          Zurücksetzen
        </button>
      </div>
      <ul className="space-y-1">
        {order.map((id, i) => (
          <li key={id} className="flex items-center gap-2 rounded p-1 hover:bg-gray-50">
            <span className="flex-1 text-xs text-gray-600">{WIDGET_LABELS[id] ?? id}</span>
            <button
              disabled={i === 0}
              onClick={() => moveUp(i)}
              className="p-0.5 disabled:opacity-30 hover:text-brand"
            >
              <ArrowUp className="w-3 h-3" />
            </button>
            <button
              disabled={i === order.length - 1}
              onClick={() => moveDown(i)}
              className="p-0.5 disabled:opacity-30 hover:text-brand"
            >
              <ArrowDown className="w-3 h-3" />
            </button>
          </li>
        ))}
      </ul>
      <p className="text-[10px] text-gray-400 mt-2 text-center">Reihenfolge gespeichert</p>
      <button
        onClick={onClose}
        className="mt-2 w-full text-xs text-center text-gray-500 hover:text-gray-700"
      >
        Schließen
      </button>
    </div>
  )
}

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
  const { data: scoreHistory } = useScoreHistory(30)

  const [isGeneratingExec, setIsGeneratingExec] = useState(false)
  const [execError, setExecError] = useState<unknown>(null)
  const [showOrderPopover, setShowOrderPopover] = useState(false)
  const { order, moveUp, moveDown, reset } = useDashboardOrder()

  async function downloadExecutiveSummary() {
    setIsGeneratingExec(true)
    setExecError(null)
    try {
      const token = getAuthToken() ?? ''
      const res = await fetch('/api/v1/secvitals/reports/executive-summary', {
        headers: { Authorization: `Bearer ${token}` },
      })
      if (res.status === 402) throw new FeatureLockedError('audit-pdf')
      if (!res.ok) throw new Error('Executive Summary konnte nicht erstellt werden.')
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `executive-summary-${new Date().toISOString().slice(0, 10)}.pdf`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setExecError(err)
    } finally {
      setIsGeneratingExec(false)
    }
  }

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

  // Current compliance score: latest score-history entry, or 0
  const currentScore = scoreHistory && scoreHistory.length > 0
    ? Math.round(scoreHistory[scoreHistory.length - 1].score)
    : 0

  // Widget map — each entry renders a React node
  const widgetMap: Record<string, React.ReactNode> = {
    kpis: (
      <div key="kpis" className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-5 gap-4">
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
    ),
    evidence: <ExpiringEvidenceWidget key="evidence" />,
    advisor: <AIAdvisor key="advisor" aiAvailable={aiStatus?.available ?? false} />,
    trend: <ScoreTrendChart key="trend" data={scoreHistory ?? []} />,
    benchmark: currentScore > 0 ? <ScoreBenchmark key="benchmark" currentScore={currentScore} /> : null,
    areas: (
      <div key="areas">
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
    ),
  }

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Vakt Comply"
        description="Compliance, Risiken & Governance auf einen Blick."
        actions={
          <div className="flex flex-col items-end gap-1">
            <div className="flex items-center gap-2">
              {/* Item 61: Anpassen button */}
              <div className="relative">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setShowOrderPopover((v) => !v)}
                  className="h-8 text-xs gap-1.5"
                >
                  <Settings className="w-3.5 h-3.5" />
                  Anpassen
                </Button>
                {showOrderPopover && (
                  <DashboardOrderPopover
                    order={order}
                    moveUp={moveUp}
                    moveDown={moveDown}
                    reset={reset}
                    onClose={() => setShowOrderPopover(false)}
                  />
                )}
              </div>
              <Button
                size="sm"
                variant="outline"
                onClick={downloadExecutiveSummary}
                disabled={isGeneratingExec}
                className="h-8 text-xs gap-1.5"
              >
                {isGeneratingExec ? (
                  <>
                    <div className="w-3 h-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
                    Wird erstellt…
                  </>
                ) : (
                  <>
                    <DownloadCloud className="w-3.5 h-3.5" />
                    Executive Summary
                  </>
                )}
              </Button>
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
            </div>
            <ProGate error={reportError instanceof FeatureLockedError ? reportError : null}>{''}</ProGate>
            {reportError instanceof Error && !(reportError instanceof FeatureLockedError) && (
              <p className="text-[10px] text-red-500">{reportError.message}</p>
            )}
            {execError instanceof FeatureLockedError && (
              <ProGate error={execError}>{''}</ProGate>
            )}
            {execError instanceof Error && !(execError instanceof FeatureLockedError) && (
              <p className="text-[10px] text-red-500">{execError.message}</p>
            )}
          </div>
        }
      />

      {/* Item 61: Render widgets in user-defined order */}
      <div className="flex-1 p-6 space-y-8">
        {order.map((id) => widgetMap[id] ?? null)}
      </div>
    </div>
  )
}
