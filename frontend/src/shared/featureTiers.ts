/**
 * Lizenz-Merkmale, die Vakt in keinem Tarif verkauft (`features.UnsoldFeatures`,
 * backend/internal/shared/platform/features/tiers.go).
 *
 * Diese Merkmale gaten echte Routen, aber kein ausgestellter Schlüssel trägt sie.
 * Eine Sperre auf ihnen darf deshalb keinen Kauf anbieten: Ein Pro-Kunde, der auf
 * der DORA-Seite „Jetzt upgraden“ liest, soll noch einmal zahlen — und bekäme
 * danach wieder dieselbe Sperre.
 *
 * Quelle der Wahrheit bleibt der Go-Code. featureTiers.test.ts liest tiers.go und
 * license.go und wird rot, wenn diese Liste davon abweicht.
 */
export const UNSOLD_FEATURES: readonly string[] = ['tisax', 'dora', 'iso_42001']

export function isUnsoldFeature(feature: string | undefined | null): boolean {
  return feature != null && UNSOLD_FEATURES.includes(feature)
}
