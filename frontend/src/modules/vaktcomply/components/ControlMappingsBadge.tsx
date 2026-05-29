import { Link2 } from 'lucide-react'
import { useControlMappings } from '../hooks/useControlMappings'
import type { ControlMapping } from '../hooks/useControlMappings'

// Colour scheme per mapping type
const MAPPING_TYPE_STYLE: Record<ControlMapping['mapping_type'], string> = {
  equivalent:  'bg-green-500/10 text-green-700 border-green-500/25',
  partial:     'bg-yellow-500/10 text-yellow-700 border-yellow-500/25',
  informative: 'bg-blue-500/10 text-blue-600 border-blue-500/25',
}

interface ControlMappingsBadgeProps {
  /** UUID of the control whose cross-framework mappings should be shown. */
  controlId: string | undefined
}

/**
 * Inline component that renders "Auch abgedeckt in: <badge> <badge> …" for a given control.
 * Renders nothing while data is loading or when no mappings exist.
 */
export function ControlMappingsBadge({ controlId }: ControlMappingsBadgeProps) {
  const { data: mappings, isLoading } = useControlMappings(controlId)

  if (isLoading || !mappings || mappings.length === 0) return null

  return (
    <div className="flex items-center gap-2 flex-wrap text-xs">
      <span className="flex items-center gap-1 text-secondary shrink-0">
        <Link2 className="w-3.5 h-3.5" />
        Auch abgedeckt in:
      </span>
      {mappings.map((m) => (
        <span
          key={m.id}
          title={`${m.target_framework_name}: ${m.target_control_title} (${m.mapping_type})`}
          className={`inline-flex items-center rounded border px-2 py-0.5 font-mono font-medium ${MAPPING_TYPE_STYLE[m.mapping_type]}`}
        >
          {m.target_framework} {m.target_control_code}
        </span>
      ))}
    </div>
  )
}
