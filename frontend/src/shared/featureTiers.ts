import { frameworkDisplayName } from '../modules/vaktcomply/utils/frameworkCatalogue'

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

/**
 * Anzeigenamen der Lizenz-Merkmale für die Lizenzkarte (UX-05).
 *
 * Ein Lizenzschlüssel trägt Schlüssel wie `bsi_grundschutz` oder `siem_export`.
 * Die gehören nicht in die Oberfläche. Merkmale, die ein Framework freischalten,
 * heißen wie das Framework im Katalog (frameworkDisplayName); alle anderen stehen
 * hier. featureTiers.test.ts liest die Feature-Konstanten aus license.go und wird
 * rot, wenn ein Merkmal ohne Anzeigenamen dazukommt.
 */
const FEATURE_LABELS: Record<string, string> = {
  bsi_grundschutz: 'BSI IT-Grundschutz',
  // ai_advisor schaltet seit v0.6.x nur noch eine eigene KI-Server-Adresse frei
  // (admin/handler_org.go, base_url_override); der KI-Berater selbst ist Community.
  ai_advisor: 'Eigener KI-Server',
  audit_pdf: 'Audit-Berichte als PDF',
  sso: 'Single Sign-on (OIDC)',
  api_access: 'API-Zugang',
  granular_permissions: 'Modulrechte je Benutzer',
  supplier_portal: 'Lieferantenportal',
  nis2_reporting: 'NIS2-Meldewesen',
  saml_auth: 'SAML-Anmeldung',
  scim_provisioning: 'Benutzerabgleich per SCIM',
  siem_export: 'SIEM-Export',
  agent_write_tools: 'KI-Assistent mit Schreibrechten',
  multi_framework: 'Weitere Frameworks (ISO 27017/27018, DSGVO-TOM, CIS, KRITIS, C5)',
  // Merkmale der archivierten Module (2026-10-23). Ältere Schlüssel führen sie
  // noch; sie schalten nichts mehr frei.
  vaktaware_advanced: 'Vakt Aware Pro (Modul archiviert)',
  vaktscan_advanced: 'Vakt Scan Pro (Modul archiviert)',
  vaktvault_advanced: 'Vakt Vault Pro (Modul archiviert)',
  vaktprivacy_advanced: 'Vakt Privacy Pro (Modul archiviert)',
}

/** Merkmale, die genau ein Framework freischalten; Anzeigename aus dem Framework-Katalog. */
const FRAMEWORK_FEATURES: readonly string[] = ['tisax', 'dora', 'eu_ai_act', 'cra', 'iso_42001']

/** Anzeigename eines Lizenz-Merkmals; unbekannte Schlüssel bleiben, wie sie sind. */
export function featureLabel(feature: string): string {
  if (FRAMEWORK_FEATURES.includes(feature)) return frameworkDisplayName(feature)
  return FEATURE_LABELS[feature] ?? feature
}
