import { useNavigate } from 'react-router-dom'
import { ArrowLeft, CheckCircle2, XCircle } from 'lucide-react'
import { Spinner } from '../../../components/Spinner'
import { Button } from '../../../components/ui/button'
import { Badge } from '../../../components/ui/badge'
import { PageHeader } from '../../../shared/components/PageHeader'
import { useMappingCoverage } from '../hooks/useMappingCoverage'
import type { FrameworkPairCoverage } from '../types'

function PairRow({ pair }: { pair: FrameworkPairCoverage }) {
  return (
    <div className="flex items-center justify-between px-4 py-3 bg-surface border border-border rounded-lg">
      <div className="flex items-center gap-3">
        {pair.is_mapped ? (
          <CheckCircle2 className="w-4 h-4 text-green-500 shrink-0" />
        ) : (
          <XCircle className="w-4 h-4 text-red-400 shrink-0" />
        )}
        <span className="text-sm font-medium">
          {pair.framework_a_name}
          <span className="text-secondary mx-2">↔</span>
          {pair.framework_b_name}
        </span>
      </div>
      <div className="flex items-center gap-2">
        {pair.is_mapped ? (
          <Badge variant="success" className="text-xs">
            {pair.mapping_count} Mappings
          </Badge>
        ) : (
          <Badge variant="outline" className="text-xs text-secondary">
            Kein Mapping
          </Badge>
        )}
      </div>
    </div>
  )
}

export default function MappingCoveragePage() {
  const navigate = useNavigate()
  const { data, isLoading, isError } = useMappingCoverage()

  const pct = data?.coverage_pct ?? 0
  const mapped = data?.mapped_pairs ?? 0
  const total = data?.total_meaningful_pairs ?? 0

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="Cross-Framework Mapping Coverage"
        description="Zeigt welche Framework-Paare durch Kontroll-Mappings abgedeckt sind."
        actions={
          <Button variant="outline" size="sm" onClick={() => { navigate(-1); }}>
            <ArrowLeft className="w-4 h-4 mr-1" />
            Zurück
          </Button>
        }
      />

      <div className="flex-1 p-6 space-y-6">
        {isError && (
          <div className="text-sm text-red-400 p-4 bg-red-500/10 rounded-lg">
            Fehler beim Laden der Mapping-Übersicht.
          </div>
        )}

        {!isLoading && data && (
          <div className="flex items-center gap-4 flex-wrap">
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <span className="text-xs text-secondary">Abgedeckt:</span>
              <Badge variant="success" className="text-xs">{mapped} / {total}</Badge>
            </div>
            <div className="flex items-center gap-2 px-3 py-1.5 bg-surface border border-border rounded-md">
              <span className="text-xs text-secondary">Coverage:</span>
              <span className={`text-sm font-semibold ${pct >= 80 ? 'text-green-500' : pct >= 50 ? 'text-yellow-500' : 'text-red-400'}`}>
                {pct.toFixed(1)}%
              </span>
            </div>
          </div>
        )}

        {isLoading ? (
          <div className="flex items-center justify-center h-32">
            <Spinner size="md" />
          </div>
        ) : !data || data.pairs.length === 0 ? (
          <div className="flex items-center gap-3 p-4 bg-surface border border-border rounded-lg text-sm text-secondary">
            <span className="text-lg">ℹ</span>
            <span>Keine Framework-Paare vorhanden. Aktiviere mindestens zwei Frameworks.</span>
          </div>
        ) : (
          <div className="space-y-2">
            {data.pairs.map((pair) => (
              <PairRow
                key={`${pair.framework_a_name}--${pair.framework_b_name}`}
                pair={pair}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
