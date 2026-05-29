import React, { useState, type RefObject } from 'react'
import { useNavigate } from 'react-router-dom'
import { X } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'
import { Badge } from '../../../components/ui/badge'
import { useFocusTrap } from '../../../shared/hooks/useFocusTrap'

export interface HeatmapRisk {
  id: string
  title: string
  likelihood: number
  impact: number
  treatment_option?: string
  status?: string
}

interface Props {
  risks: HeatmapRisk[]
}

// Color for a cell based on its score (likelihood × impact)
function cellColor(likelihood: number, impact: number): string {
  const score = likelihood * impact
  if (score >= 20) return 'bg-red-900/80'
  if (score >= 15) return 'bg-red-600/70'
  if (score >= 10) return 'bg-orange-500/70'
  if (score >= 5)  return 'bg-yellow-500/70'
  return 'bg-green-600/60'
}

const LIKELIHOOD_LABELS = ['Sehr selten', 'Selten', 'Möglich', 'Wahrscheinl.', 'Sehr häufig']
const IMPACT_LABELS     = ['Minimal', 'Gering', 'Mittel', 'Hoch', 'Katastrophal']

const STATUS_LABELS: Record<string, string> = {
  open: 'Offen',
  in_review: 'In Prüfung',
  accepted: 'Akzeptiert',
  closed: 'Geschlossen',
  mitigated: 'Gemindert',
}

function statusVariant(status: string | undefined): 'default' | 'secondary' | 'success' | 'destructive' {
  if (status === 'open') return 'destructive'
  if (status === 'mitigated' || status === 'closed') return 'success'
  if (status === 'accepted') return 'secondary'
  return 'secondary'
}

// ─── Cell detail panel (slide-over style dialog) ──────────────────────────────

interface CellPanelProps {
  likelihood: number
  impact: number
  risks: HeatmapRisk[]
  onClose: () => void
}

function CellPanel({ likelihood, impact, risks, onClose }: CellPanelProps) {
  const navigate = useNavigate()
  const trapRef = useFocusTrap<HTMLDivElement>(true, onClose)

  return (
    <>
      {/* Backdrop */}
      <div
        className="fixed inset-0 z-40 bg-black/40"
        onClick={onClose}
        aria-hidden="true"
      />
      {/* Slide-over panel */}
      <div
        ref={trapRef as RefObject<HTMLDivElement>}
        role="dialog"
        aria-modal="true"
        aria-label={`Risiken: Wahrscheinlichkeit ${likelihood} / Auswirkung ${impact}`}
        className="fixed right-0 top-0 bottom-0 z-50 w-full max-w-sm bg-surface border-l border-border shadow-2xl flex flex-col"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
          <div>
            <h2 className="text-sm font-semibold text-primary">Risiken in dieser Zelle</h2>
            <p className="text-[11px] text-secondary mt-0.5">
              Wahrscheinlichkeit: <span className="font-medium">{LIKELIHOOD_LABELS[likelihood - 1]}</span>
              {' '}·{' '}
              Auswirkung: <span className="font-medium">{IMPACT_LABELS[impact - 1]}</span>
              {' '}·{' '}
              Score: <span className="font-medium">{likelihood * impact}</span>
            </p>
          </div>
          <button
            onClick={onClose}
            aria-label="Panel schließen"
            className="p-1.5 rounded-md text-secondary hover:text-primary hover:bg-muted/50 transition-colors"
          >
            <X className="w-4 h-4" aria-hidden="true" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-5">
          {risks.length === 0 ? (
            <p className="text-sm text-secondary">Keine Risiken in dieser Zelle.</p>
          ) : (
            <ol className="space-y-2">
              {risks.map((risk) => (
                <li key={risk.id}>
                  <button
                    className="w-full text-left p-3 rounded-lg border border-border bg-bg hover:border-brand/50 hover:bg-surface transition-colors"
                    onClick={() => { navigate(`/vaktcomply/risks/${risk.id}`); onClose() }}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <p className="text-sm font-medium text-primary leading-snug">{risk.title}</p>
                      {risk.status && (
                        <Badge variant={statusVariant(risk.status)} className="text-[10px] shrink-0">
                          {STATUS_LABELS[risk.status] ?? risk.status}
                        </Badge>
                      )}
                    </div>
                  </button>
                </li>
              ))}
            </ol>
          )}
        </div>
      </div>
    </>
  )
}

// ─── Main heatmap ──────────────────────────────────────────────────────────────

const RiskHeatmap: React.FC<Props> = ({ risks }) => {
  const [selectedCell, setSelectedCell] = useState<{ likelihood: number; impact: number } | null>(null)

  // Build a map: key = `${likelihood}-${impact}` → risks in that cell
  const cellMap = new Map<string, HeatmapRisk[]>()
  for (const risk of risks) {
    const key = `${risk.likelihood}-${risk.impact}`
    const cell = cellMap.get(key) ?? []
    cell.push(risk)
    cellMap.set(key, cell)
  }

  const selectedRisks = selectedCell
    ? (cellMap.get(`${selectedCell.likelihood}-${selectedCell.impact}`) ?? [])
    : []

  return (
    <>
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm font-semibold">Risiko-Heatmap (5×5)</CardTitle>
          <p className="text-xs text-muted-foreground">
            X = Wahrscheinlichkeit, Y = Auswirkung. Jeder Punkt steht für ein Risiko.
            Risiken mit Strategie "Akzeptieren" werden transparent dargestellt.
            Klicke auf eine Zelle für Details.
          </p>
        </CardHeader>
        <CardContent>
          <div className="flex gap-3">
            {/* Y-axis label */}
            <div className="flex items-center justify-center" style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)', minWidth: 28 }}>
              <span className="text-xs text-muted-foreground whitespace-nowrap">Auswirkung</span>
            </div>

            <div className="flex-1">
              {/* Grid: impact rows from 5 (top) to 1 (bottom) */}
              <div className="grid gap-1" style={{ gridTemplateRows: 'repeat(5, 1fr)' }}>
                {[5, 4, 3, 2, 1].map((impact) => (
                  <div key={impact} className="flex items-center gap-1">
                    {/* Y-axis label */}
                    <div className="w-20 text-right pr-1 shrink-0">
                      <span className="text-xs text-muted-foreground leading-none">{IMPACT_LABELS[impact - 1]}</span>
                    </div>
                    {/* Cells for each likelihood value */}
                    <div className="flex gap-1 flex-1">
                      {[1, 2, 3, 4, 5].map((likelihood) => {
                        const key = `${likelihood}-${impact}`
                        const cellRisks = cellMap.get(key) ?? []
                        const bg = cellColor(likelihood, impact)
                        const isSelected =
                          selectedCell?.likelihood === likelihood &&
                          selectedCell?.impact === impact
                        return (
                          <button
                            key={likelihood}
                            onClick={() => { setSelectedCell({ likelihood, impact }); }}
                            aria-label={`Wahrscheinlichkeit ${LIKELIHOOD_LABELS[likelihood - 1]}, Auswirkung ${IMPACT_LABELS[impact - 1]}, Score ${likelihood * impact}, ${cellRisks.length} Risiko${cellRisks.length === 1 ? '' : 'en'}`}
                            className={`flex-1 min-h-[52px] rounded-md ${bg} relative flex flex-wrap content-start gap-0.5 p-1 transition-all hover:ring-2 hover:ring-white/50 focus:outline-none focus:ring-2 focus:ring-white/70 ${isSelected ? 'ring-2 ring-white/80' : ''}`}
                            title={`W:${likelihood} A:${impact} — Score ${likelihood * impact} · ${cellRisks.length} Risiken`}
                          >
                            {cellRisks.map((risk) => (
                              <span
                                key={risk.id}
                                className={`inline-flex w-4 h-4 rounded-full bg-white border border-white/40 shrink-0 ${
                                  risk.treatment_option === 'accept' ? 'opacity-40' : 'opacity-90'
                                }`}
                                aria-hidden="true"
                              />
                            ))}
                            {cellRisks.length > 4 && (
                              <span className="text-[9px] text-white font-semibold leading-none self-end ml-auto">
                                +{cellRisks.length - 4}
                              </span>
                            )}
                          </button>
                        )
                      })}
                    </div>
                  </div>
                ))}
              </div>

              {/* X-axis labels */}
              <div className="flex gap-1 mt-1 ml-[84px]">
                {[1, 2, 3, 4, 5].map((l) => (
                  <div key={l} className="flex-1 text-center">
                    <span className="text-[10px] text-muted-foreground leading-none">{LIKELIHOOD_LABELS[l - 1]}</span>
                  </div>
                ))}
              </div>
              <div className="text-center mt-1">
                <span className="text-xs text-muted-foreground">Wahrscheinlichkeit</span>
              </div>
            </div>
          </div>

          {/* Legend */}
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 mt-3 text-xs text-muted-foreground">
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-green-600/60 inline-block" /> Score 1–4 (Niedrig)
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-yellow-500/70 inline-block" /> Score 5–9 (Mittel)
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-orange-500/70 inline-block" /> Score 10–14 (Erhöht)
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-red-600/70 inline-block" /> Score 15–19 (Hoch)
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-3 h-3 rounded-sm bg-red-900/80 inline-block" /> Score 20–25 (Kritisch)
            </span>
            <span className="flex items-center gap-1.5">
              <span className="w-4 h-4 rounded-full bg-white border border-white/40 opacity-40 inline-block" /> Akzeptiert (transparent)
            </span>
          </div>
        </CardContent>
      </Card>

      {selectedCell && (
        <CellPanel
          likelihood={selectedCell.likelihood}
          impact={selectedCell.impact}
          risks={selectedRisks}
          onClose={() => { setSelectedCell(null); }}
        />
      )}
    </>
  )
}

export default RiskHeatmap
