import React from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '../../../components/ui/card'

export interface HeatmapRisk {
  id: string
  title: string
  likelihood: number
  impact: number
  treatment_option?: string
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

const RiskHeatmap: React.FC<Props> = ({ risks }) => {
  // Build a map: key = `${likelihood}-${impact}` → risks in that cell
  const cellMap = new Map<string, HeatmapRisk[]>()
  for (const risk of risks) {
    const key = `${risk.likelihood}-${risk.impact}`
    const cell = cellMap.get(key) ?? []
    cell.push(risk)
    cellMap.set(key, cell)
  }

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-semibold">Risiko-Heatmap (5×5)</CardTitle>
        <p className="text-xs text-muted-foreground">
          X = Wahrscheinlichkeit, Y = Auswirkung. Jeder Punkt steht für ein Risiko.
          Risiken mit Strategie "Akzeptieren" werden transparent dargestellt.
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
                      return (
                        <div
                          key={likelihood}
                          className={`flex-1 min-h-[52px] rounded-md ${bg} relative flex flex-wrap content-start gap-0.5 p-1`}
                          title={`W:${likelihood} A:${impact} — Score ${likelihood * impact}`}
                        >
                          {cellRisks.map((risk) => (
                            <span
                              key={risk.id}
                              className={`inline-flex w-4 h-4 rounded-full bg-white border border-white/40 shrink-0 cursor-default ${
                                risk.treatment_option === 'accept' ? 'opacity-40' : 'opacity-90'
                              }`}
                              title={risk.title}
                            />
                          ))}
                          {cellRisks.length > 4 && (
                            <span className="text-[9px] text-white font-semibold leading-none self-end ml-auto">
                              +{cellRisks.length - 4}
                            </span>
                          )}
                        </div>
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
  )
}

export default RiskHeatmap
