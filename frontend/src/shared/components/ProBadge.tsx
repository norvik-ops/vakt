import { useTranslation } from 'react-i18next'

/**
 * Marker für eine Seite, die mit einer Community-Lizenz nicht nutzbar ist.
 *
 * `tier` ist nicht Kosmetik. TISAX, DORA und ISO 42001 gaten echte Routen, liegen aber
 * in KEINEM verkäuflichen Tarif (`features.UnsoldFeatures`, backend/internal/shared/
 * platform/features/tiers.go). Ein „Pro"-Badge auf diesen Seiten verspricht, dass eine
 * Pro-Lizenz sie freischaltet — sie antwortet dort 402, und zwar für jeden zahlenden
 * Kunden. Ein früherer Entwurf beschriftete sie mit dem am 2026-08-08 abgeschafften
 * dritten Tarifnamen — den konnte ohnehin kein Signierpfad je ausstellen.
 *
 * Die Beschriftung ist bewusst der bereits vorhandene Katalog-Schlüssel und keine neue
 * Formulierung: die Frameworks-Seite zeigt für dieselbe Sache schon „Nicht im Angebot".
 * Zwei Wortlaute für einen Sachverhalt sind die nächste Drift-Quelle.
 *
 * Quelle der Wahrheit ist die Tier-Zuordnung im Backend; scripts/check_feature_tiers.py
 * hält Badge und Code zusammen.
 */
export function ProBadge({ tier = 'pro' }: { tier?: 'pro' | 'unsold' }) {
  const { t } = useTranslation()
  return (
    <span className="ml-auto text-[10px] font-semibold text-amber-600 bg-amber-50 border border-amber-200 rounded px-1 py-0.5 leading-none dark:bg-amber-950/40 dark:border-amber-700 dark:text-amber-400">
      {tier === 'unsold'
        ? t('vaktcomply.frameworksPage.frameworkStatusNotOffered')
        : 'Pro'}
    </span>
  )
}
