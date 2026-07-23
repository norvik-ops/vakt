import { BrainCircuit, X, AlertTriangle, Info, ChevronRight } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAIInsights, useDismissInsight } from '../hooks/useAIInsights'
import { AIDisclaimer } from '../../../shared/components/AIDisclaimer'
import type { AIInsight } from '../types'

function urgencyIcon(urgency: AIInsight['urgency']) {
  if (urgency === 1) return <AlertTriangle className="w-3.5 h-3.5 text-red-400 shrink-0" />
  if (urgency === 2) return <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0" />
  return <Info className="w-3.5 h-3.5 text-blue-400 shrink-0" />
}

function insightLink(insight: AIInsight): string | null {
  if (insight.control_id) return `/vaktcomply/controls/${insight.control_id}`
  if (insight.risk_id) return `/vaktcomply/risks/${insight.risk_id}`
  if (insight.finding_id) return `/vaktscan/findings/${insight.finding_id}`
  return null
}

export function AIInsightsFeed() {
  const { data, isLoading } = useAIInsights()
  const dismiss = useDismissInsight()
  const navigate = useNavigate()

  const items = data ?? []

  if (isLoading) return null
  if (items.length === 0) return null

  return (
    <div className="rounded-xl border border-border bg-surface p-5 space-y-3">
      <div className="flex items-center gap-2">
        <BrainCircuit className="w-4 h-4 text-brand shrink-0" />
        <h2 className="text-sm font-semibold text-primary">KI-Insights</h2>
        <span className="ml-auto text-xs text-secondary">{items.length} Hinweise</span>
      </div>
      <AIDisclaimer />

      <ul className="space-y-2">
        {items.map((item) => {
          const link = insightLink(item)
          return (
            <li key={item.id} className="flex items-start gap-2 text-xs text-primary">
              {urgencyIcon(item.urgency)}
              <div className="flex-1 min-w-0">
                <p className="font-medium truncate">{item.title}</p>
                <p className="text-secondary line-clamp-2 mt-0.5">{item.message}</p>
              </div>
              <div className="flex items-center gap-1 shrink-0">
                {link && (
                  <button
                    onClick={() => { navigate(link) }}
                    className="p-1 text-secondary hover:text-brand transition-colors"
                    aria-label="Öffnen"
                  >
                    <ChevronRight className="w-3.5 h-3.5" />
                  </button>
                )}
                <button
                  onClick={() => { dismiss.mutate(item.id) }}
                  disabled={dismiss.isPending}
                  className="p-1 text-secondary hover:text-red-400 transition-colors"
                  aria-label="Verwerfen"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
