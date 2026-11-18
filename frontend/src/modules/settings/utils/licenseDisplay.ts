/** Felder der Lizenzantwort, die über die Plakette „Auto-Renewal aktiv" entscheiden. */
export interface AutoRenewalFields {
  is_pro: boolean
  demo: boolean
  auto_renewal_enabled: boolean
  /** Ein Verlängerungs-Token existiert (im Schlüssel oder über VAKT_LICENSE_TOKEN). */
  renewal_token_set?: boolean
}

/**
 * UX-04: „Auto-Renewal aktiv" nur, wenn es etwas zu verlängern gibt (Pro-Schlüssel,
 * keine Demo) und womit (Token). Der Config-Schalter `auto_renewal_enabled` allein
 * steht auch auf Community-Instanzen ohne Schlüssel an und sagt darüber nichts.
 */
export function showAutoRenewalBadge(lic: AutoRenewalFields | undefined): boolean {
  return !!lic && lic.is_pro && !lic.demo && lic.auto_renewal_enabled && lic.renewal_token_set === true
}
