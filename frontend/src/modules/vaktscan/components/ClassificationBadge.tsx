import { useTranslation } from 'react-i18next'
import { Badge } from '../../../components/ui/badge'
import type { ClassificationLevel } from '../types'

const CLASSIFICATION_CLASS: Record<ClassificationLevel, string> = {
  public: 'bg-green-500/20 text-green-400 border-green-500/30',
  internal: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  confidential: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
  restricted: 'bg-red-500/20 text-red-400 border-red-500/30',
}

const CLASSIFICATION_LABELS: Record<ClassificationLevel, string> = {
  public: 'Öffentlich',
  internal: 'Intern',
  confidential: 'Vertraulich',
  restricted: 'Streng Vertraulich',
}

export function ClassificationBadge({
  level,
  className = '',
}: {
  level: ClassificationLevel
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <Badge className={`text-xs ${CLASSIFICATION_CLASS[level]} ${className}`}>
      {t('vaktscan.classification.' + level, CLASSIFICATION_LABELS[level])}
    </Badge>
  )
}

export { CLASSIFICATION_LABELS, CLASSIFICATION_CLASS }
