import type { Policy } from '../types'

/**
 * Eine Zeile für Version und Revision einer Richtlinie (UX-22).
 *
 * Eine Richtlinie hat zwei Nummern, und die Oberfläche hat sie früher drei Mal
 * verschieden genannt (Karte „v2" + „1.0", Detail „v1.0 · Revision 2", Historie
 * „v1"). Jetzt gilt überall:
 *   - Version  = `version`, die Bezeichnung, die der Verantwortliche vergibt („1.0").
 *   - Revision = `version_num`, der Zähler, den das Backend bei jedem Speichern
 *     hochzählt; die Versionshistorie (`ck_policy_versions.version`) zählt dieselbe Nummer.
 */
export function policyVersionLine(
  policy: Pick<Policy, 'version' | 'version_num' | 'category'>,
  t: (key: string, opts?: Record<string, unknown>) => string,
  withCategory = true,
): string {
  return [
    policy.version ? t('vaktcomply.policyDetail.versionLabel', { version: policy.version }) : '',
    t('vaktcomply.policyDetail.revisionLabel', { n: policy.version_num }),
    withCategory ? policy.category : '',
  ].filter(Boolean).join(' · ')
}
