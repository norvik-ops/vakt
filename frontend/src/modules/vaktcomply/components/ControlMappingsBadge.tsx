import { Link2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { useControlMappings } from '../hooks/useControlMappings'
import { frameworkDisplayName } from '../utils/frameworkCatalogue'
import { dedupeMappings, mappingChipLabel, MAPPING_TYPES, type MappingType } from '../utils/controlMappings'

// Colour scheme per mapping type
const MAPPING_TYPE_STYLE: Record<MappingType, string> = {
  equivalent:  'bg-green-500/10 text-green-700 border-green-500/25 dark:text-green-400',
  partial:     'bg-yellow-500/10 text-yellow-700 border-yellow-500/25 dark:text-yellow-400',
  informative: 'bg-blue-500/10 text-blue-600 border-blue-500/25 dark:text-blue-400',
}

const MAPPING_TYPE_DOT: Record<MappingType, string> = {
  equivalent:  'bg-green-500',
  partial:     'bg-yellow-500',
  informative: 'bg-blue-500',
}

// Explizite t()-Aufrufe statt dynamischer Schlüssel, damit der i18n-Sweep sie sieht.
function typeLabel(t: TFunction, type: MappingType): string {
  if (type === 'equivalent') return t('vaktcomply.controlMappings.typeEquivalent')
  if (type === 'partial') return t('vaktcomply.controlMappings.typePartial')
  return t('vaktcomply.controlMappings.typeInformative')
}

function typeHint(t: TFunction, type: MappingType): string {
  if (type === 'equivalent') return t('vaktcomply.controlMappings.hintEquivalent')
  if (type === 'partial') return t('vaktcomply.controlMappings.hintPartial')
  return t('vaktcomply.controlMappings.hintInformative')
}

interface ControlMappingsBadgeProps {
  /** UUID of the control whose cross-framework mappings should be shown. */
  controlId: string | undefined
}

/**
 * Inline component that renders "Auch abgedeckt in: <badge> <badge> …" for a given control,
 * plus a legend for the chip colours (UX-18). One chip per target control.
 * Renders nothing while data is loading or when no mappings exist.
 */
export function ControlMappingsBadge({ controlId }: ControlMappingsBadgeProps) {
  const { t } = useTranslation()
  const { data: mappings, isLoading } = useControlMappings(controlId)

  if (isLoading || !mappings || mappings.length === 0) return null

  const unique = dedupeMappings(mappings)
  const typesPresent = MAPPING_TYPES.filter((type) => unique.some((m) => m.mapping_type === type))

  return (
    <div className="space-y-1.5 text-xs" data-testid="control-mappings">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="flex items-center gap-1 text-secondary shrink-0">
          <Link2 className="w-3.5 h-3.5" aria-hidden="true" />
          {t('vaktcomply.controlMappings.alsoCovered')}
        </span>
        {unique.map((m) => (
          <span
            key={`${m.target_framework}-${m.target_control_code}`}
            data-testid="control-mapping-chip"
            title={`${frameworkDisplayName(m.target_framework_name)}: ${m.target_control_title} (${typeLabel(t, m.mapping_type)})`}
            className={`inline-flex items-center rounded border px-2 py-0.5 font-mono font-medium ${MAPPING_TYPE_STYLE[m.mapping_type]}`}
          >
            {mappingChipLabel(m)}
          </span>
        ))}
      </div>
      <ul
        className="flex items-center gap-x-3 gap-y-1 flex-wrap text-[11px] text-secondary"
        aria-label={t('vaktcomply.controlMappings.legend')}
        data-testid="control-mappings-legend"
      >
        {typesPresent.map((type) => (
          <li key={type} className="flex items-center gap-1">
            <span className={`inline-block w-2 h-2 rounded-full ${MAPPING_TYPE_DOT[type]}`} aria-hidden="true" />
            <span className="font-medium">{typeLabel(t, type)}</span>
            <span>— {typeHint(t, type)}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
